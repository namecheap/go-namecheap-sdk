# Complete domains.getInfo Response Mapping — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Map the full `DomainGetInfoResult` subtree (Whois privacy, domain dates, ownership, modification rights) per issue #165, deprecate the misnamed `DomainDNSGetListResult` field behind a `Result()` accessor, and fix two adjacent verified defects (FreeDNS-fallback nil panics, `ClassifyPrivacyStatus` inversion).

**Architecture:** Pure additive struct expansion in `namecheap/domains_get_info.go` decoded by `encoding/xml`; a one-function widening of `DateTime.UnmarshalText` to tolerate empty/padded element bodies; nil-guards in the getList→getInfo fallback. No changes to request building, `DoXML`, or method signatures.

**Tech Stack:** Go 1.26.3+, `encoding/xml`, `github.com/stretchr/testify/assert`, `httptest` mock servers.

**Spec:** `docs/superpowers/specs/2026-08-10-domains-get-info-design.md` (read it before starting any task).

## Global Constraints

- Every commit: `git commit -s` (DCO sign-off required). Conventional Commit subjects. No AI-attribution trailers of any kind.
- All new optional API fields are pointer-typed.
- Every exported symbol added or touched gets a doc comment starting with its own name, full sentences.
- Assertions use `testify/assert`; sub-cases use `t.Run` with `t.Parallel()`; helper style matches the file being edited.
- Do not add `nolint` directives except the specific contingency in Task 3 Step 6.
- XML struct tags must match the wire names character-for-character: element `Whoisguard` (lowercase g), attribute `WhoisGuardEmail` (capital G), element `Modificationrights` (lowercase r), element `SubscriptionId`. A wrong-case tag fails silently (nil field, no error).
- Verification gates, in order: `make format`, `make check`, `make lint`, `make test-unit-quiet`, `make test-race`.

---

### Task 1: DateTime empty-input tolerance

**Files:**
- Modify: `namecheap/datetime.go:14-21`
- Test: `namecheap/datetime_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `DateTime.UnmarshalText([]byte) error` — trims whitespace; empty/whitespace-only input yields the zero `time.Time` with nil error; non-empty input parses as `01/02/2006` exactly as before. Tasks 2's element-body `*DateTime` fields rely on this.

- [ ] **Step 1: Write the failing tests**

Append inside `TestDateTimeUnmarshalText` in `namecheap/datetime_test.go` (after the existing `invalid_date_string_returns_error` sub-test):

```go
	t.Run("empty_input_yields_zero_time", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte(""))
		assert.NoError(t, err)
		assert.True(t, dt.IsZero())
	})

	t.Run("whitespace_only_input_yields_zero_time", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("\n\t \n"))
		assert.NoError(t, err)
		assert.True(t, dt.IsZero())
	})

	t.Run("padded_date_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("\n\t11/26/2021\n"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 11, 26, 0, 0, 0, 0, time.UTC)}))
	})

	t.Run("plain_date_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("11/26/2021"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 11, 26, 0, 0, 0, 0, time.UTC)}))
	})

	t.Run("iso_timestamp_returns_error", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("0001-01-01T00:00:00"))
		assert.Error(t, err)
	})
```

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `go test ./namecheap/ -run TestDateTimeUnmarshalText -v`
Expected: `empty_input_yields_zero_time`, `whitespace_only_input_yields_zero_time`, `padded_date_parses` FAIL (parse errors); `plain_date_parses`, `iso_timestamp_returns_error`, `invalid_date_string_returns_error` PASS.

- [ ] **Step 3: Widen UnmarshalText**

Replace the `UnmarshalText` method in `namecheap/datetime.go` with:

```go
// UnmarshalText parses an API date in MM/DD/YYYY form, ignoring surrounding
// whitespace. Empty or whitespace-only input yields the zero time without
// error: the API omits or empties date elements for domains that lack them,
// and failing there would abort decoding of the entire response.
func (dt *DateTime) UnmarshalText(text []byte) (err error) {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		dt.Time = time.Time{}
		return nil
	}

	dt.Time, err = time.Parse("01/02/2006", trimmed)
	return err
}
```

Add `"strings"` to the imports in `namecheap/datetime.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./namecheap/ -run TestDateTime -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add namecheap/datetime.go namecheap/datetime_test.go
git commit -s -m "fix: tolerate empty and padded input in DateTime.UnmarshalText

Element-body DateTime fields are being introduced for domains.getInfo;
encoding/xml hands element chardata to TextUnmarshaler untrimmed, and
an empty or padded date element previously aborted the whole decode."
```

---

### Task 2: getInfo struct expansion + full-response decode tests

**Files:**
- Modify: `namecheap/domains_get_info.go` (full rewrite of the type block; methods unchanged)
- Test: `namecheap/domains_get_info_test.go`

**Interfaces:**
- Consumes: Task 1's tolerant `DateTime.UnmarshalText`.
- Produces: expanded `DomainsGetInfoResult` plus new exported types `DomainDetails`, `WhoisGuard`, `WhoisGuardEmailDetails`, `ModificationRights`, `ModificationRight`; expanded `PremiumDnsSubscription` (`UseAutoRenew *bool`, `SubscriptionID *int`, `CreatedDate *string`, `ExpirationDate *string`) and `DnsDetails` (`HostCount *int`, `EmailType *string`, `DynamicDNSStatus *bool`, `IsFailover *bool`). Tasks 3 and 4 use these exact names.

- [ ] **Step 1: Write the failing tests**

In `namecheap/domains_get_info_test.go`, add two sub-tests inside `TestDomainsGetInfo` (after `request_command`). The first decodes the existing `fakeResponse` fixture; the second uses a new populated fixture. Also declare the new fixture const after `fakeResponse`:

```go
	fakePopulatedResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.getinfo</RequestedCommand>
			<CommandResponse Type="namecheap.domains.getInfo">
				<DomainGetInfoResult Status="Ok" ID="313319" DomainName="domain1.com" OwnerName="apisample" IsOwner="true" IsPremium="true">
					<DomainDetails>
						<CreatedDate>04/23/2016</CreatedDate>
						<ExpiredDate>04/23/2017</ExpiredDate>
						<NumYears>1</NumYears>
					</DomainDetails>
					<LockDetails />
					<Whoisguard Enabled="True">
						<ID>53536</ID>
						<ExpiredDate>04/23/2017</ExpiredDate>
						<EmailDetails WhoisGuardEmail="protect@whoisguard.com" ForwardedTo="example@gmail.com" LastAutoEmailChangeDate="05/20/2020" AutoEmailChangeFrequencyDays="3" />
					</Whoisguard>
					<PremiumDnsSubscription>
						<UseAutoRenew>true</UseAutoRenew>
						<SubscriptionId>4756</SubscriptionId>
						<CreatedDate>2016-04-23T00:00:00</CreatedDate>
						<ExpirationDate>2017-04-23T00:00:00</ExpirationDate>
						<IsActive>true</IsActive>
					</PremiumDnsSubscription>
					<DnsDetails ProviderType="CUSTOM" IsUsingOurDNS="false" HostCount="2" EmailType="FWD" DynamicDNSStatus="false" IsFailover="false">
						<Nameserver>dns1.name-servers.com</Nameserver>
						<Nameserver>dns2.name-servers.com</Nameserver>
					</DnsDetails>
					<Modificationrights All="false">
						<Rights Type="dns">OK</Rights>
						<Rights Type="eforward">OK</Rights>
						<Rights Type="rlock">OK</Rights>
						<Rights Type="parkpage">OK</Rights>
						<Rights Type="hosts">OK</Rights>
						<Rights Type="ddns">OK</Rights>
						<Rights Type="extend">OK</Rights>
						<Rights Type="nameserver">OK</Rights>
						<Rights Type="autorenew">OK</Rights>
						<Rights Type="whoisguard">OK</Rights>
						<Rights Type="premiumDns">OK</Rights>
						<Rights Type="dnsSec">OK</Rights>
					</Modificationrights>
				</DomainGetInfoResult>
			</CommandResponse>
			<Server>PHX01APIEXT12</Server>
			<GMTTimeDifference>--5:00</GMTTimeDifference>
			<ExecutionTime>0.013</ExecutionTime>
		</ApiResponse>
	`
```

Sub-test 1 — every struct non-nil and values from the real capture (`fakeResponse`):

```go
	t.Run("parses_all_result_fields", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetInfoWithContext(context.Background(), "horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domain info", err)
		}

		result := resp.DomainDNSGetListResult
		if result == nil {
			t.Fatal("GetInfoWithContext() result is nil, want populated DomainsGetInfoResult")
		}

		assert.Equal(t, 1706717, *result.ID)
		assert.Equal(t, "horse-family.com.ua", *result.DomainName)
		assert.Equal(t, "NCStaffvladlenf", *result.OwnerName)
		assert.Equal(t, false, *result.IsOwner)
		assert.Equal(t, false, *result.IsPremium)
		assert.Nil(t, result.Status)

		if assert.NotNil(t, result.DomainDetails) {
			assert.Equal(t, "11/26/2021", result.DomainDetails.CreatedDate.Format("01/02/2006"))
			assert.Nil(t, result.DomainDetails.ExpiredDate)
			assert.Equal(t, 0, *result.DomainDetails.NumYears)
		}

		if assert.NotNil(t, result.WhoisGuard) {
			assert.Equal(t, "NotAlloted", *result.WhoisGuard.Enabled)
			assert.Equal(t, 0, *result.WhoisGuard.ID)
			assert.Nil(t, result.WhoisGuard.ExpiredDate)
			assert.Nil(t, result.WhoisGuard.EmailDetails)
		}

		if assert.NotNil(t, result.PremiumDnsSubscription) {
			assert.Equal(t, false, *result.PremiumDnsSubscription.UseAutoRenew)
			assert.Equal(t, -1, *result.PremiumDnsSubscription.SubscriptionID)
			assert.Equal(t, "0001-01-01T00:00:00", *result.PremiumDnsSubscription.CreatedDate)
			assert.Equal(t, "0001-01-01T00:00:00", *result.PremiumDnsSubscription.ExpirationDate)
			assert.Equal(t, false, *result.PremiumDnsSubscription.IsActive)
		}

		if assert.NotNil(t, result.DnsDetails) {
			assert.Equal(t, "FreeDNS", *result.DnsDetails.ProviderType)
			assert.Equal(t, true, *result.DnsDetails.IsUsingOurDNS)
			assert.Equal(t, 0, *result.DnsDetails.HostCount)
			assert.Equal(t, "No Email Service", *result.DnsDetails.EmailType)
			assert.Equal(t, false, *result.DnsDetails.DynamicDNSStatus)
			assert.Equal(t, false, *result.DnsDetails.IsFailover)
		}

		if assert.NotNil(t, result.ModificationRights) {
			assert.Nil(t, result.ModificationRights.All)
			assert.Nil(t, result.ModificationRights.Rights)
		}
	})
```

Sub-test 2 — the populated path:

```go
	t.Run("parses_populated_privacy_response", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakePopulatedResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetInfoWithContext(context.Background(), "domain1.com")
		if err != nil {
			t.Fatal("Unable to get domain info", err)
		}

		result := resp.DomainDNSGetListResult
		if result == nil {
			t.Fatal("GetInfoWithContext() result is nil, want populated DomainsGetInfoResult")
		}

		assert.Equal(t, "Ok", *result.Status)
		assert.Equal(t, 313319, *result.ID)
		assert.Equal(t, true, *result.IsOwner)
		assert.Equal(t, true, *result.IsPremium)

		if assert.NotNil(t, result.DomainDetails) {
			assert.Equal(t, "04/23/2017", result.DomainDetails.ExpiredDate.Format("01/02/2006"))
			assert.Equal(t, 1, *result.DomainDetails.NumYears)
		}

		if assert.NotNil(t, result.WhoisGuard) {
			assert.Equal(t, "True", *result.WhoisGuard.Enabled)
			assert.Equal(t, 53536, *result.WhoisGuard.ID)
			assert.Equal(t, "04/23/2017", result.WhoisGuard.ExpiredDate.Format("01/02/2006"))
			if assert.NotNil(t, result.WhoisGuard.EmailDetails) {
				assert.Equal(t, "protect@whoisguard.com", *result.WhoisGuard.EmailDetails.WhoisGuardEmail)
				assert.Equal(t, "example@gmail.com", *result.WhoisGuard.EmailDetails.ForwardedTo)
				assert.Equal(t, "05/20/2020", *result.WhoisGuard.EmailDetails.LastAutoEmailChangeDate)
				assert.Equal(t, 3, *result.WhoisGuard.EmailDetails.AutoEmailChangeFrequencyDays)
			}
		}

		if assert.NotNil(t, result.PremiumDnsSubscription) {
			assert.Equal(t, true, *result.PremiumDnsSubscription.UseAutoRenew)
			assert.Equal(t, 4756, *result.PremiumDnsSubscription.SubscriptionID)
		}

		if assert.NotNil(t, result.ModificationRights) {
			assert.Equal(t, false, *result.ModificationRights.All)
			if assert.NotNil(t, result.ModificationRights.Rights) {
				rights := *result.ModificationRights.Rights
				assert.Len(t, rights, 12)
				assert.Equal(t, "dns", *rights[0].Type)
				assert.Equal(t, "OK", *rights[0].Value)
				assert.Equal(t, "rlock", *rights[2].Type)
			}
		}
	})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./namecheap/ -run TestDomainsGetInfo -v`
Expected: compile errors (`result.Status undefined`, `undefined: WhoisGuard`, etc.). A compile failure is the failing state for this step.

- [ ] **Step 3: Rewrite the type block in `namecheap/domains_get_info.go`**

Replace everything from `type DomainsGetInfoResponse struct` through the end of `type DnsDetails struct` (lines 8–36) with the following. The two functions at the bottom of the file (`GetInfoWithContext`, `GetInfo`) are untouched.

```go
// DomainsGetInfoResponse is the raw XML envelope for the
// namecheap.domains.getInfo response.
type DomainsGetInfoResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsGetInfoCommandResponse `xml:"CommandResponse"`
}

// DomainsGetInfoCommandResponse wraps the getInfo result.
type DomainsGetInfoCommandResponse struct {
	// DomainDNSGetListResult holds the getInfo result.
	//
	// Deprecated: misnamed; use Result. The field will be renamed to
	// DomainGetInfoResult in v3.
	DomainDNSGetListResult *DomainsGetInfoResult `xml:"DomainGetInfoResult"`
}

// DomainsGetInfoResult is the detailed information about a single domain
// returned by namecheap.domains.getInfo.
//
// Registrar-lock state is reported here only indirectly (Status "Locked" and
// the ModificationRight with Type "rlock"); the authoritative check is
// DomainsService.GetRegistrarLockWithContext. The endpoint carries no
// domain-level auto-renew flag — that lives on namecheap.domains.getList
// (Domain.AutoRenew). The always-empty LockDetails element is deliberately
// not mapped.
type DomainsGetInfoResult struct {
	// Status is the domain status: "Ok", "Locked", or "Expired".
	Status *string `xml:"Status,attr"`
	// ID is the unique integer identifying the domain.
	ID         *int    `xml:"ID,attr"`
	DomainName *string `xml:"DomainName,attr"`
	// OwnerName is the user account under which the domain is registered.
	OwnerName *string `xml:"OwnerName,attr"`
	// IsOwner reports whether the API user is the domain owner.
	IsOwner   *bool `xml:"IsOwner,attr"`
	IsPremium *bool `xml:"IsPremium,attr"`

	DomainDetails          *DomainDetails          `xml:"DomainDetails"`
	WhoisGuard             *WhoisGuard             `xml:"Whoisguard"`
	PremiumDnsSubscription *PremiumDnsSubscription `xml:"PremiumDnsSubscription"` // nolint: stylecheck,revive
	DnsDetails             *DnsDetails             `xml:"DnsDetails"`             // nolint: stylecheck,revive
	ModificationRights     *ModificationRights     `xml:"Modificationrights"`
}

// DomainDetails carries the registration dates of the domain.
type DomainDetails struct {
	CreatedDate *DateTime `xml:"CreatedDate"`
	ExpiredDate *DateTime `xml:"ExpiredDate"`
	NumYears    *int      `xml:"NumYears"`
}

// WhoisGuard describes the Whois privacy protection attached to the domain.
// The type name follows the legacy Whoisguard wire element.
type WhoisGuard struct {
	// Enabled is tri-state: "True", "False", or "NotAlloted".
	Enabled *string `xml:"Enabled,attr"`
	// ID is the privacy subscription ID; 0 when Enabled is "NotAlloted".
	ID           *int                    `xml:"ID"`
	ExpiredDate  *DateTime               `xml:"ExpiredDate"`
	EmailDetails *WhoisGuardEmailDetails `xml:"EmailDetails"`
}

// WhoisGuardEmailDetails describes the privacy email forwarding
// configuration. Field names mirror the API's attribute names.
type WhoisGuardEmailDetails struct {
	WhoisGuardEmail *string `xml:"WhoisGuardEmail,attr"`
	ForwardedTo     *string `xml:"ForwardedTo,attr"`
	// LastAutoEmailChangeDate is empty or an MM/DD/YYYY date string.
	LastAutoEmailChangeDate      *string `xml:"LastAutoEmailChangeDate,attr"`
	AutoEmailChangeFrequencyDays *int    `xml:"AutoEmailChangeFrequencyDays,attr"`
}

// ModificationRights lists what the API user may modify on the domain.
// For permissioned (shared) domains All is false and Rights carries the
// granular per-feature rights.
type ModificationRights struct {
	All    *bool                `xml:"All,attr"`
	Rights *[]ModificationRight `xml:"Rights"`
}

// ModificationRight is one granular modification right, for example
// Type "rlock" with Value "OK". Observed Type values: dns, eforward, rlock,
// parkpage, hosts, ddns, extend, nameserver, autorenew, whoisguard,
// premiumDns, dnsSec.
type ModificationRight struct {
	Type  *string `xml:"Type,attr"`
	Value *string `xml:",chardata"`
}

// PremiumDnsSubscription describes the premium DNS subscription attached to
// the domain, if any.
type PremiumDnsSubscription struct { // nolint: stylecheck,revive
	// UseAutoRenew is the premium DNS subscription's own auto-renew flag,
	// not the domain's; getInfo exposes no domain-level auto-renew.
	UseAutoRenew *bool `xml:"UseAutoRenew"`
	// SubscriptionID is -1 when no premium DNS subscription exists.
	SubscriptionID *int `xml:"SubscriptionId"`
	// CreatedDate is an ISO-format string; "0001-01-01T00:00:00" means never.
	CreatedDate *string `xml:"CreatedDate"`
	// ExpirationDate is an ISO-format string; "0001-01-01T00:00:00" means never.
	ExpirationDate *string `xml:"ExpirationDate"`
	IsActive       *bool   `xml:"IsActive"`
}

// DnsDetails describes the DNS configuration of the domain.
type DnsDetails struct { // nolint: stylecheck,revive
	ProviderType     *string   `xml:"ProviderType,attr"`
	IsUsingOurDNS    *bool     `xml:"IsUsingOurDNS,attr"`
	HostCount        *int      `xml:"HostCount,attr"`
	EmailType        *string   `xml:"EmailType,attr"`
	DynamicDNSStatus *bool     `xml:"DynamicDNSStatus,attr"`
	IsFailover       *bool     `xml:"IsFailover,attr"`
	Nameservers      *[]string `xml:"Nameserver"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./namecheap/ -run TestDomainsGetInfo -v`
Expected: all PASS. Then run the whole package — the FreeDNS fallback test consumes these types: `go test ./namecheap/ -count=1`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add namecheap/domains_get_info.go namecheap/domains_get_info_test.go
git commit -s -m "fix: map the full domains.getInfo response

Adds Whois privacy (Whoisguard incl. EmailDetails), domain dates,
ownership attributes, modification rights (incl. granular Rights for
permissioned domains), and the missing PremiumDnsSubscription and
DnsDetails fields."
```

---

### Task 3: Deprecation notice + Result() accessor

**Files:**
- Modify: `namecheap/domains_get_info.go` (add one method after `DomainsGetInfoCommandResponse`)
- Test: `namecheap/domains_get_info_test.go`

**Interfaces:**
- Consumes: `DomainsGetInfoCommandResponse.DomainDNSGetListResult` (deprecated in Task 2's rewrite).
- Produces: `func (r *DomainsGetInfoCommandResponse) Result() *DomainsGetInfoResult`.

- [ ] **Step 1: Write the failing test**

Add inside `TestDomainsGetInfo`:

```go
	t.Run("result_accessor_returns_deprecated_field", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetInfoWithContext(context.Background(), "horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domain info", err)
		}

		assert.NotNil(t, resp.Result())
		assert.Same(t, resp.DomainDNSGetListResult, resp.Result())
	})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./namecheap/ -run TestDomainsGetInfo -v`
Expected: compile error `resp.Result undefined`.

- [ ] **Step 3: Add the accessor**

In `namecheap/domains_get_info.go`, immediately after the `DomainsGetInfoCommandResponse` type declaration:

```go
// Result returns the getInfo result under its correct name. It exists
// because the DomainDNSGetListResult field is misnamed; when the field is
// renamed to DomainGetInfoResult in v3, Result remains valid.
func (r *DomainsGetInfoCommandResponse) Result() *DomainsGetInfoResult {
	return r.DomainDNSGetListResult
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./namecheap/ -run TestDomainsGetInfo -v`
Expected: PASS.

- [ ] **Step 5: Run the linter**

Run: `make lint`
Expected: clean. staticcheck's SA1019 does not flag same-package uses of deprecated symbols, so no `nolint` should be needed.

- [ ] **Step 6: Contingency — only if Step 5 reports SA1019**

If (and only if) `make lint` flags the deprecated-field references in `domains_get_info.go`, the test file, or `domains_dns_get_list.go`, append to each flagged line exactly:

```go
//nolint:staticcheck // SA1019: the deprecated field is the accessor's backing store until the v3 rename.
```

Re-run `make lint` and confirm clean.

- [ ] **Step 7: Commit**

```bash
git add namecheap/domains_get_info.go namecheap/domains_get_info_test.go
git commit -s -m "fix: deprecate misnamed DomainDNSGetListResult behind Result accessor

The getInfo command response field is a copy-paste from the DNS-list
endpoint. A rename is breaking, and a parallel field mapping the same
XML element is a hard encoding/xml conflict error, so v2 keeps the
field deprecated and v3 renames it; Result() survives both."
```

---

### Task 4: Nil-guard the FreeDNS fallback

**Files:**
- Modify: `namecheap/domains_dns_get_list.go:63-79`
- Test: `namecheap/domains_dns_get_list_test.go`

**Interfaces:**
- Consumes: Task 2's `DomainsGetInfoResult` (fields `DomainName`, `DnsDetails`, `PremiumDnsSubscription`) and `DomainsDNSGetListCommandResponse`/`DomainDNSGetListResult` (existing, unchanged).
- Produces: no new API; the fallback becomes panic-free on sparse getInfo responses.

- [ ] **Step 1: Write the failing test**

Add inside `TestDomainsDNSGetList` in `namecheap/domains_dns_get_list_test.go`, after the `FreeDNS domain handling` sub-test. The getInfo body is the officially documented sparse response (self-closed `DnsDetails`, no `PremiumDnsSubscription`); today this test panics:

```go
	t.Run("freedns_fallback_sparse_getinfo_response", func(t *testing.T) {
		t.Parallel()
		fakeDNSGetListResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2019166">Domain name not found</Error>
				</Errors>
				<RequestedCommand>namecheap.domains.dns.getlist</RequestedCommand>
				<CommandResponse Type="namecheap.domains.dns.getList" />
			</ApiResponse>
		`

		fakeGetInfoResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<RequestedCommand>namecheap.domains.getinfo</RequestedCommand>
				<CommandResponse Type="namecheap.domains.getInfo">
					<DomainGetInfoResult Status="Ok" ID="313319" DomainName="domain1.com" OwnerName="apisample" IsOwner="true">
						<DnsDetails ProviderType="ENOM" />
					</DomainGetInfoResult>
				</CommandResponse>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			if query.Get("Command") == "namecheap.domains.dns.getList" {
				_, _ = writer.Write([]byte(fakeDNSGetListResponse))
			} else {
				_, _ = writer.Write([]byte(fakeGetInfoResponse))
			}
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetListWithContext(context.Background(), "domain1.com")
		if err != nil {
			t.Fatal("Unable to get DNS list", err)
		}

		if result.DomainDNSGetListResult == nil {
			t.Fatal("GetListWithContext() fallback result is nil, want synthesized DomainDNSGetListResult")
		}
		assert.Equal(t, "domain1.com", *result.DomainDNSGetListResult.Domain)
		assert.Equal(t, false, *result.DomainDNSGetListResult.IsUsingFreeDNS)
		assert.Nil(t, result.DomainDNSGetListResult.IsPremiumDNS)
		assert.Nil(t, result.DomainDNSGetListResult.IsUsingOurDNS)
		assert.Nil(t, result.DomainDNSGetListResult.Nameservers)
	})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./namecheap/ -run 'TestDomainsDNSGetList/freedns_fallback_sparse_getinfo_response' -v`
Expected: FAIL with a panic (`invalid memory address or nil pointer dereference`) from `domains_dns_get_list.go`.

- [ ] **Step 3: Guard the dereferences**

In `namecheap/domains_dns_get_list.go`, replace lines 69–79 (from `IsUsingFreeDNS := ...` through the `return &DomainsDNSGetListCommandResponse{...}, nil` block) with:

```go
		synthesized := &DomainDNSGetListResult{}
		if info := domainInfo.DomainDNSGetListResult; info != nil {
			synthesized.Domain = info.DomainName
			if info.DnsDetails != nil {
				synthesized.IsUsingOurDNS = info.DnsDetails.IsUsingOurDNS
				synthesized.Nameservers = info.DnsDetails.Nameservers
				if info.DnsDetails.ProviderType != nil {
					isUsingFreeDNS := *info.DnsDetails.ProviderType == "FreeDNS"
					synthesized.IsUsingFreeDNS = &isUsingFreeDNS
				}
			}
			if info.PremiumDnsSubscription != nil {
				synthesized.IsPremiumDNS = info.PremiumDnsSubscription.IsActive
			}
		}

		return &DomainsDNSGetListCommandResponse{
			DomainDNSGetListResult: synthesized,
		}, nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./namecheap/ -run TestDomainsDNSGetList -v`
Expected: all PASS, including the pre-existing `FreeDNS domain handling` sub-test (same synthesized values as before for populated responses).

- [ ] **Step 5: Commit**

```bash
git add namecheap/domains_dns_get_list.go namecheap/domains_dns_get_list_test.go
git commit -s -m "fix: nil-guard the FreeDNS fallback in domains.dns.getList

The fallback dereferenced the getInfo result, DnsDetails, ProviderType,
and PremiumDnsSubscription unchecked; the officially documented sparse
getInfo response (self-closed DnsDetails, no PremiumDnsSubscription)
panicked."
```

---

### Task 5: ClassifyPrivacyStatus inversion for "NotAlloted"

**Files:**
- Modify: `namecheap/domainprivacy.go:71-103`
- Test: `namecheap/domainprivacy_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `ClassifyPrivacyStatus("NotAlloted") == PrivacyStateFree`.

- [ ] **Step 1: Write the failing test cases**

`TestClassifyPrivacyStatus` in `namecheap/domainprivacy_test.go` is table-driven. Add these rows to its table (match the existing row shape — fields `status` and `want`):

```go
		{status: "NotAlloted", want: PrivacyStateFree},
		{status: "notalloted", want: PrivacyStateFree},
		{status: "NotAllotted", want: PrivacyStateFree},
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./namecheap/ -run TestClassifyPrivacyStatus -v`
Expected: FAIL — the new rows return `PrivacyStateAllotted` ("notalloted" contains "allot" and does not contain "unallot").

- [ ] **Step 3: Fix the classifier**

In `namecheap/domainprivacy.go`, change the FREE case of the switch in `ClassifyPrivacyStatus` from:

```go
	case strings.Contains(normalized, "free"), strings.Contains(normalized, "unallot"):
		return PrivacyStateFree
```

to:

```go
	case strings.Contains(normalized, "free"),
		strings.Contains(normalized, "unallot"),
		strings.Contains(normalized, "notallot"):
		return PrivacyStateFree
```

Update the corresponding doc-comment line from:

```go
//   - a description containing "free" or "unallot"     -> PrivacyStateFree;
```

to:

```go
//   - a description containing "free", "unallot" or
//     "notallot" (getInfo's Whoisguard "NotAlloted")   -> PrivacyStateFree;
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./namecheap/ -run TestClassifyPrivacyStatus -v`
Expected: all PASS, including every pre-existing row.

- [ ] **Step 5: Commit**

```bash
git add namecheap/domainprivacy.go namecheap/domainprivacy_test.go
git commit -s -m "fix: classify NotAlloted privacy status as free, not allotted

\"notalloted\" contains \"allot\" but not \"unallot\", so the keyword
matcher returned the inverse state for getInfo's Whoisguard vocabulary."
```

---

### Task 6: API reference doc — getInfo child elements

**Files:**
- Modify: `docs/namecheap-api-v2.md` (getInfo Response Parameters table, ~line 381)

**Interfaces:** none (documentation only).

- [ ] **Step 1: Extend the Response Parameters table**

In `docs/namecheap-api-v2.md`, the getInfo "Response Parameters" table ends with the `IsPremium` row. Append these rows to that table:

```markdown
| DomainDetails | Child element: CreatedDate, ExpiredDate (MM/DD/YYYY), NumYears |
| LockDetails | Child element: observed empty in all captures; use namecheap.domains.getRegistrarLock for lock state |
| Whoisguard | Child element: Enabled attr ("True", "False", "NotAlloted"), ID (0 = not alloted), ExpiredDate, EmailDetails (WhoisGuardEmail, ForwardedTo, LastAutoEmailChangeDate — empty or MM/DD/YYYY, AutoEmailChangeFrequencyDays) |
| PremiumDnsSubscription | Child element: UseAutoRenew (the subscription's flag, not the domain's), SubscriptionId (-1 = none), CreatedDate/ExpirationDate (ISO; 0001-01-01T00:00:00 = never), IsActive |
| DnsDetails | Child element: ProviderType, IsUsingOurDNS, HostCount, EmailType, DynamicDNSStatus, IsFailover attrs; Nameserver children |
| Modificationrights | Child element: All attr; Rights children with Type attr (dns, eforward, rlock, parkpage, hosts, ddns, extend, nameserver, autorenew, whoisguard, premiumDns, dnsSec) and chardata value ("OK") |
```

Also append one line below the table:

```markdown
Note: getInfo has no domain-level auto-renew field; domain auto-renew and lock booleans are returned by `namecheap.domains.getList` (`AutoRenew`, `IsLocked`).
```

- [ ] **Step 2: Commit**

```bash
git add docs/namecheap-api-v2.md
git commit -s -m "docs: document getInfo child elements in the API reference"
```

---

### Task 7: Full verification gates

**Files:** none (verification only).

- [ ] **Step 1: Run every gate in order**

```bash
make format
make check
make lint
make test-unit-quiet
make test-race
```

Expected: all clean/green. `make format` must produce no diff (`git status --short` shows nothing unstaged); if it reformats anything, amend the offending commit or add a `style:` commit.

- [ ] **Step 2: Confirm the working tree is clean and the branch is coherent**

```bash
git status --short
git log --oneline origin/master..HEAD
```

Expected: no uncommitted changes; commit list = the spec/plan docs plus Tasks 1–6.

---

## Self-Review (completed at planning time)

- **Spec coverage:** Decision 1 → Task 2; Decision 2 → Tasks 2+3; Decision 3 → Task 2 (types); Decision 4 → Task 1; Adjacent fix 5 → Task 4; Adjacent fix 6 → Task 5; documentation section → Tasks 2 (doc comments) + 6 (api-v2.md); testing section → Tasks 1–5 test steps; release section → PR stage (after plan execution).
- **Placeholder scan:** none; Task 3 Step 6 is an explicit conditional with exact text, not a TBD.
- **Type consistency:** `Result()` (Tasks 2 doc comment, 3, 4 unaffected), `SubscriptionID`/`xml:"SubscriptionId"` (Tasks 2, 4), `ModificationRight(s)` (Task 2, referenced in 6), `DateTime` zero-tolerance (Task 1, consumed by Task 2 fixtures via absent elements only — empty-element behavior additionally covered in Task 1's unit tests).
