//go:build acceptance

// Package namecheap_test's acceptance suite exercises the real Namecheap API.
// It is excluded from the normal build by the "acceptance" build tag and only
// run via `make testacc`. It reads credentials from the environment and skips
// cleanly when they are absent, so it never fails a credential-less run.
// NAMECHEAP_USE_SANDBOX selects the endpoint: "true" targets the sandbox API,
// anything else targets production.
//
// Most tests exercise read-only commands, and the ones that change account
// state capture it first and restore it (defer). TestAcc_EmailType is the
// exception, by design: it wipes NAMECHEAP_TEST_DOMAIN's zone, switches its mail
// mode through every value (NONE, MX, MXE and the GMAIL, OX and FWD presets) and
// leaves the zone empty under NONE. make testacc and make update-fixtures both
// run it (-run TestAcc), so the domain must be disposable in every environment
// the suite is pointed at, production included.
//
// With -update-fixtures, the read-only responses are re-captured into
// ../namecheaptest/fixtures so CI can diff them against the committed corpus and
// surface server-shape drift as a reviewable diff. Fixture capture requires
// NAMECHEAP_USE_SANDBOX=true: the committed corpus is sandbox-shaped, and
// production responses must never overwrite it.
package namecheap_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
)

// updateFixtures, when set, re-captures the read-only sandbox responses into the
// committed fixture corpus so drift surfaces as a git diff.
var updateFixtures = flag.Bool("update-fixtures", false,
	"re-capture read-only sandbox responses into ../namecheaptest/fixtures")

const (
	envAPIUser    = "NAMECHEAP_API_USER"
	envAPIKey     = "NAMECHEAP_API_KEY"
	envClientIP   = "NAMECHEAP_CLIENT_IP"
	envUserName   = "NAMECHEAP_USER_NAME"
	envUseSandbox = "NAMECHEAP_USE_SANDBOX"
	// envAddFunds opts in to TestAcc_UsersAddFunds. Every run leaves a pending
	// credit-card funding request on the account; nothing is charged, because
	// the token is never submitted with card details.
	envAddFunds = "NAMECHEAP_ACC_ADD_FUNDS"
	// envUsersUpdate opts in to TestAcc_UsersUpdateRoundTrip, which writes the
	// account profile back to itself through users.update.
	envUsersUpdate = "NAMECHEAP_ACC_USERS_UPDATE"
	// envDomain names a dedicated, disposable test domain in the target account
	// used for the reversible DNS round-trip. The DNS test skips when it is unset.
	envDomain = "NAMECHEAP_TEST_DOMAIN"
	// envRenewDomain opts in to TestAcc_DomainsRenew, the one test that is
	// neither read-only nor reversible: it renews the named domain for a year
	// and spends account funds (sandbox test funds when NAMECHEAP_USE_SANDBOX
	// is true). It exists to prove the DateTime parser against the real
	// domains.renew response and is never set in CI.
	envRenewDomain = "NAMECHEAP_ACC_RENEW_DOMAIN"
)

// wipeZone empties the zone of domain under EmailType NONE, for a DNS test that
// must not depend on what an earlier run or another suite sharing the domain
// left there. Only the tests that need an empty zone call it.
func wipeZone(t *testing.T, client *namecheap.Client, domain string) {
	t.Helper()
	if err := clearZone(ctx(t), client, domain, namecheap.EmailTypeNone); err != nil {
		t.Fatalf("wipe the zone of %s before the test: %v", domain, err)
	}
}

// clearZone leaves the zone of domain with no host records, under emailType.
func clearZone(ctx context.Context, client *namecheap.Client, domain, emailType string) error {
	if _, err := client.DomainsDNS.SetHostsWithContext(ctx, &namecheap.DomainsDNSSetHostsArgs{
		Domain:    namecheap.String(domain),
		Records:   &[]namecheap.DomainsDNSHostRecord{},
		EmailType: namecheap.String(emailType),
	}); err != nil {
		return fmt.Errorf("clear the zone of %s under EmailType=%s: %w", domain, emailType, err)
	}
	return nil
}

// dateOnlyPadded is the form domains.getList and domains.getInfo have always
// used, confirmed on production on 2026-10-23. A change here is server-side
// drift worth a look.
var dateOnlyPadded = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)

// assertLiveDate checks that a live date field parsed and that its raw text
// still has the shape this endpoint has always used.
func assertLiveDate(t *testing.T, field string, dt *namecheap.DateTime, shape *regexp.Regexp) {
	t.Helper()
	if dt == nil || dt.IsZero() {
		t.Errorf("%s = %v, want a parsed date", field, dt)
		return
	}
	if err := dt.ParseError(); err != nil {
		t.Errorf("%s: %v", field, err)
	}
	if !shape.MatchString(dt.Raw) {
		t.Errorf("%s raw text = %q, want shape %s (server-side format drift?)", field, dt.Raw, shape)
	}
}

// assertLiveDateText is assertLiveDate for fields the SDK exposes as raw
// strings rather than DateTime. The text goes through the SDK parser first, so
// a value the shape accepts but the parser rejects (a day/month swap such as
// "14/12/2026") still fails.
func assertLiveDateText(t *testing.T, field string, raw *string, shape *regexp.Regexp) {
	t.Helper()
	if raw == nil {
		t.Errorf("%s is missing, want a date", field)
		return
	}
	assertLiveDate(t, field, parseLiveDateText(*raw), shape)
}

// parseLiveDateText runs raw API text through the SDK date parser. UnmarshalText
// never fails; the outcome is in the returned value's ParseError.
func parseLiveDateText(raw string) *namecheap.DateTime {
	var dt namecheap.DateTime
	_ = dt.UnmarshalText([]byte(raw))
	return &dt
}

// liveClient builds a client for the live API from environment credentials,
// skipping the test when any required variable is missing. The endpoint is
// production unless NAMECHEAP_USE_SANDBOX is true.
func liveClient(t *testing.T) *namecheap.Client {
	t.Helper()
	apiUser := os.Getenv(envAPIUser)
	apiKey := os.Getenv(envAPIKey)
	clientIP := os.Getenv(envClientIP)
	if apiUser == "" || apiKey == "" || clientIP == "" {
		t.Skipf("live API credentials not set; skipping (need %s, %s, %s[, %s, %s])",
			envAPIUser, envAPIKey, envClientIP, envUserName, envDomain)
	}
	userName := os.Getenv(envUserName)
	if userName == "" {
		userName = apiUser
	}
	useSandbox, _ := strconv.ParseBool(os.Getenv(envUseSandbox))
	return namecheap.NewClient(&namecheap.ClientOptions{
		UserName:   userName,
		ApiUser:    apiUser,
		ApiKey:     apiKey,
		ClientIp:   clientIP,
		UseSandbox: useSandbox,
		// Conservative pacing: the live API throttles aggressively, and the
		// acceptance account is shared with other CI runs.
		RateLimit: &namecheap.RateLimitOptions{
			PerMinute:     10,
			MaxConcurrent: 1,
		},
		Retry: &namecheap.RetryOptions{
			MaxAttempts: 5,
			BaseDelay:   2 * time.Second,
			MaxDelay:    30 * time.Second,
		},
	})
}

// ctx returns a per-test context with a generous timeout for the live API.
// It must accommodate the full retry budget (RetryOptions.MaxElapsed defaults
// to 2 minutes), not just a single round trip.
func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return c
}

func TestAcc_DomainsCheck(t *testing.T) {
	const freeCandidate = "example-that-should-be-free-12345.com"
	client := liveClient(t)

	// Check the free candidate plus, when configured, the account's own test
	// domain — the one name guaranteed to be registered in this registry.
	domains := []string{freeCandidate}
	testDomain := os.Getenv(envDomain)
	if testDomain != "" {
		domains = append(domains, testDomain)
	}

	resp, err := client.Domains.CheckWithContext(ctx(t), domains...)
	if err != nil {
		t.Fatalf("domains.check: %v", err)
	}
	if resp == nil || resp.DomainCheckResults == nil {
		t.Fatal("domains.check: empty result")
	}

	availability := map[string]bool{}
	for _, r := range *resp.DomainCheckResults {
		if r.Domain == nil || r.IsAvailable == nil {
			t.Fatalf("domains.check: result missing Domain or Available attr: %+v", r)
		}
		availability[strings.ToLower(*r.Domain)] = *r.IsAvailable
	}
	if got, ok := availability[freeCandidate]; !ok || !got {
		t.Errorf("domains.check(%q) available = %v (present=%v), want true", freeCandidate, got, ok)
	}
	if testDomain != "" {
		if got, ok := availability[strings.ToLower(testDomain)]; !ok || got {
			t.Errorf("domains.check(%q) available = %v (present=%v), want false (registered test domain)", testDomain, got, ok)
		}
	}

	captureFixture(t, client, "domains_check", map[string]string{
		"Command":    "namecheap.domains.check",
		"DomainList": freeCandidate,
	}, nil)
}

func TestAcc_DomainsGetList(t *testing.T) {
	client := liveClient(t)
	args := &namecheap.DomainsGetListArgs{
		PageSize: namecheap.Int(10),
	}
	// When a test domain is configured, filter for it so the assertion is
	// independent of account size and listing order.
	testDomain := os.Getenv(envDomain)
	if testDomain != "" {
		args.SearchTerm = namecheap.String(testDomain)
	}

	resp, err := client.Domains.GetListWithContext(ctx(t), args)
	if err != nil {
		t.Fatalf("domains.getList: %v", err)
	}
	if resp == nil || resp.Domains == nil {
		t.Fatal("domains.getList: empty result")
	}

	if testDomain != "" {
		var found *namecheap.Domain
		for i, d := range *resp.Domains {
			if d.Name != nil && strings.EqualFold(*d.Name, testDomain) {
				found = &(*resp.Domains)[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("domains.getList(SearchTerm=%q) did not return the test domain", testDomain)
		}
		if found.ID == nil || *found.ID == "" {
			t.Errorf("domains.getList: test domain ID = %v, want non-empty", found.ID)
		}
		assertLiveDate(t, "domains.getList Created", found.Created, dateOnlyPadded)
		assertLiveDate(t, "domains.getList Expires", found.Expires, dateOnlyPadded)
	}

	captureFixture(t, client, "domains_getList", map[string]string{
		"Command":  "namecheap.domains.getList",
		"PageSize": "10",
	}, nil)
}

func TestAcc_DomainsGetInfo(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := liveClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping domains.getInfo", envDomain)
	}
	resp, err := client.Domains.GetInfoWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("domains.getInfo: %v", err)
	}
	if resp == nil {
		t.Fatal("domains.getInfo: nil command response")
	}
	result := resp.Result()
	if result == nil {
		t.Fatal("domains.getInfo: nil result")
	}

	// A decode abort surfaces as err above; these assertions catch the other
	// failure mode — a struct-tag mismatch leaving whole sections silently nil.
	if result.DomainName == nil || !strings.EqualFold(*result.DomainName, domain) {
		t.Errorf("getInfo DomainName = %v, want %q", result.DomainName, domain)
	}
	if result.ID == nil || *result.ID <= 0 {
		t.Errorf("getInfo ID = %v, want a positive integer", result.ID)
	}
	// Status is not present in every response; when present it must be non-empty.
	if result.Status != nil && *result.Status == "" {
		t.Error("getInfo Status is present but empty")
	}

	if result.DomainDetails == nil {
		t.Error("getInfo DomainDetails is nil, want populated")
	} else {
		assertLiveDate(t, "getInfo DomainDetails.CreatedDate", result.DomainDetails.CreatedDate, dateOnlyPadded)
		assertLiveDate(t, "getInfo DomainDetails.ExpiredDate", result.DomainDetails.ExpiredDate, dateOnlyPadded)
	}

	if result.WhoisGuard == nil {
		t.Error("getInfo WhoisGuard is nil, want populated")
	} else if result.WhoisGuard.Enabled == nil {
		t.Error("getInfo WhoisGuard.Enabled is nil, want tri-state value")
	} else if got := *result.WhoisGuard.Enabled; got != "True" && got != "False" && got != "NotAlloted" {
		t.Errorf("getInfo WhoisGuard.Enabled = %q, want True/False/NotAlloted", got)
	}
	// An allotted subscription (True or False) carries its own expiry, formatted
	// independently of whoisguard.getList; NotAlloted has none.
	if wg := result.WhoisGuard; wg != nil && wg.Enabled != nil && *wg.Enabled != "NotAlloted" {
		if wg.ExpiredDate != nil {
			t.Logf("getInfo Whoisguard.ExpiredDate raw text %q", wg.ExpiredDate.Raw)
		}
		assertLiveDate(t, "getInfo Whoisguard.ExpiredDate", wg.ExpiredDate, dateOnlyPadded)
		// LastAutoEmailChangeDate is a raw string, empty until the first
		// automatic email rotation; once set it has the same date-only shape
		// (docs/namecheap-api-v2.md, "Date formats").
		if ed := wg.EmailDetails; ed != nil {
			t.Logf("getInfo Whoisguard.EmailDetails.LastAutoEmailChangeDate raw text %s", attr(ed.LastAutoEmailChangeDate))
			if ed.LastAutoEmailChangeDate != nil && *ed.LastAutoEmailChangeDate != "" {
				assertLiveDateText(t, "getInfo Whoisguard.EmailDetails.LastAutoEmailChangeDate", ed.LastAutoEmailChangeDate, dateOnlyPadded)
			}
		}
	}

	if result.DnsDetails == nil {
		t.Error("getInfo DnsDetails is nil, want populated")
	} else {
		if result.DnsDetails.ProviderType == nil || *result.DnsDetails.ProviderType == "" {
			t.Errorf("getInfo DnsDetails.ProviderType = %v, want non-empty", result.DnsDetails.ProviderType)
		}
		if result.DnsDetails.Nameservers == nil || len(*result.DnsDetails.Nameservers) == 0 {
			t.Error("getInfo DnsDetails.Nameservers is empty, want at least one")
		}
	}

	if result.ModificationRights == nil {
		t.Error("getInfo ModificationRights is nil, want populated")
	}

	captureFixture(t, client, "domains_getInfo", map[string]string{
		"Command":    "namecheap.domains.getInfo",
		"DomainName": domain,
		"HostName":   domain,
	}, nil)
}

// TestAcc_DomainPrivacyGetList is the live-format check for whoisguard.getList
// dates. Created and Expires are raw strings in the SDK, so a format change
// would never fail decoding; only this check notices it. The raw values are
// logged as evidence of what the API returns. Status is checked against the
// vocabulary ClassifyPrivacyStatus understands for the same reason, and the
// test domain's subscription is cross-checked against domains.getInfo.
func TestAcc_DomainPrivacyGetList(t *testing.T) {
	client := liveClient(t)
	resp, err := client.DomainPrivacy.GetListWithContext(ctx(t), &namecheap.DomainPrivacyGetListArgs{
		PageSize: namecheap.Int(10),
	})
	if err != nil {
		t.Fatalf("whoisguard.getList: %v", err)
	}
	if resp == nil || resp.DomainPrivacyList == nil || len(*resp.DomainPrivacyList) == 0 {
		t.Fatal("whoisguard.getList: no subscriptions returned, want at least one to check dates on")
	}

	for i, e := range *resp.DomainPrivacyList {
		field := "whoisguard.getList ID=<nil>"
		if e.ID == nil {
			// The mutating privacy commands take this ID; a row without one is
			// a decode defect, not an account-state quirk.
			t.Errorf("whoisguard.getList row %d: ID is missing, want the subscription ID", i)
		} else {
			field = "whoisguard.getList ID=" + strconv.Itoa(*e.ID)
		}
		t.Logf("%s DomainName=%v Status=%v Created=%v Expires=%v",
			field, attr(e.DomainName), attr(e.Status), attr(e.Created), attr(e.Expires))
		if e.State() == namecheap.PrivacyStateUnknown {
			t.Errorf("%s Status = %s, want a value ClassifyPrivacyStatus recognises (server-side vocabulary drift?)",
				field, attr(e.Status))
		}
		assertLiveDateText(t, field+" Created", e.Created, dateOnlyPadded)
		assertLiveDateText(t, field+" Expires", e.Expires, dateOnlyPadded)
	}

	assertPrivacyListMatchesGetInfo(t, client)

	captureFixture(t, client, "whoisguard_getlist", map[string]string{
		"Command":  "namecheap.whoisguard.getlist",
		"PageSize": "10",
	}, nil)
}

// assertPrivacyListMatchesGetInfo cross-checks the two endpoints that report
// the test domain's privacy subscription: the ID domains.getInfo returns must
// appear in whoisguard.getList, attached to that domain and with the same
// expiry, so one endpoint drifting alone is caught. It logs and returns when no
// test domain is configured or the domain has no allotted subscription. The
// list is paged lazily through ListAll because the subscription need not sit
// on the first page of a large account.
func assertPrivacyListMatchesGetInfo(t *testing.T, client *namecheap.Client) {
	t.Helper()
	domain := os.Getenv(envDomain)
	if domain == "" {
		t.Logf("%s not set; skipping the getInfo cross-check", envDomain)
		return
	}
	info, err := client.Domains.GetInfoWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("domains.getInfo(%q): %v", domain, err)
	}
	result := info.Result()
	if result == nil || result.WhoisGuard == nil || result.WhoisGuard.ID == nil || *result.WhoisGuard.ID == 0 {
		t.Logf("getInfo(%q) reports no allotted privacy subscription; skipping the getList cross-check", domain)
		return
	}
	wg := result.WhoisGuard
	want, ok := wg.ExpiredDate.Parsed()
	if !ok {
		t.Errorf("getInfo(%q) Whoisguard.ExpiredDate = %v, want a parsed date to compare with getList", domain, wg.ExpiredDate)
		return
	}

	for e, err := range client.DomainPrivacy.ListAll(ctx(t), nil) {
		if err != nil {
			t.Fatalf("whoisguard.getList (all pages): %v", err)
		}
		if e.ID == nil || *e.ID != *wg.ID {
			continue
		}
		if e.DomainName == nil || !strings.EqualFold(*e.DomainName, domain) {
			t.Errorf("whoisguard.getList ID=%d DomainName = %s, want %q (the domain getInfo attaches it to)",
				*wg.ID, attr(e.DomainName), domain)
		}
		got, ok := parseLiveDateText(deref(e.Expires)).Parsed()
		if !ok || !got.Equal(want) {
			t.Errorf("whoisguard.getList ID=%d Expires = %s, want %q (getInfo(%q) Whoisguard.ExpiredDate)",
				*wg.ID, attr(e.Expires), wg.ExpiredDate.Raw, domain)
		}
		return
	}
	t.Errorf("whoisguard.getList has no row with ID=%d, the subscription getInfo(%q) reports", *wg.ID, domain)
}

// deref returns the pointed-to string, or "" for nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// attr renders an optional field for logs in Go syntax, so a nil pointer, an
// empty string and a zero stay distinguishable: <nil>, "" and 0.
func attr[T any](p *T) string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%#v", *p)
}

// TestAcc_DomainsRenew proves the DateTime parser against a real domains.renew
// response, the one endpoint whose date carries a time component in a
// server-dependent format (captured 2026-10-26: "12/14/2034 1:01:07\u202fPM").
// It is opt-in through NAMECHEAP_ACC_RENEW_DOMAIN because it spends funds and
// adds a year to the domain; a decode failure here is exactly the
// charge-then-error outcome this parser exists to prevent.
func TestAcc_DomainsRenew(t *testing.T) {
	domain := os.Getenv(envRenewDomain)
	client := liveClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping domains.renew (spends funds, not reversible)", envRenewDomain)
	}

	before, err := client.Domains.GetInfoWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("domains.getInfo before renew: %v", err)
	}
	if before.Result() == nil || before.Result().DomainDetails == nil || before.Result().DomainDetails.ExpiredDate == nil {
		t.Fatal("domains.getInfo before renew: no ExpiredDate to compare against")
	}
	expiryBefore := before.Result().DomainDetails.ExpiredDate.Time

	resp, err := client.Domains.RenewWithContext(ctx(t), &namecheap.DomainsRenewArgs{DomainName: domain, Years: 1})
	if err != nil {
		t.Fatalf("domains.renew: %v (a successful charge may have been made)", err)
	}
	result := resp.DomainRenewResult
	if result == nil || result.Renew == nil || !*result.Renew {
		t.Fatalf("domains.renew: Renew = %v, want true", result)
	}
	if result.DomainDetails == nil || result.DomainDetails.ExpiredDate == nil {
		t.Fatal("domains.renew: DomainDetails.ExpiredDate missing")
	}
	got := result.DomainDetails.ExpiredDate
	t.Logf("domains.renew ExpiredDate raw text %q (bytes % x)", got.Raw, []byte(got.Raw))
	if err := got.ParseError(); err != nil {
		t.Errorf("domains.renew ExpiredDate: %v", err)
	}
	if got.Year() != expiryBefore.Year()+1 {
		t.Errorf("domains.renew ExpiredDate = %v, want year %d (expiry before: %v)", got.Time, expiryBefore.Year()+1, expiryBefore)
	}
}

func TestAcc_UsersGetBalances(t *testing.T) {
	client := liveClient(t)
	resp, err := client.Users.GetBalancesWithContext(ctx(t))
	if err != nil {
		t.Fatalf("users.getBalances: %v", err)
	}
	if resp == nil || resp.UserGetBalancesResult == nil {
		t.Fatal("users.getBalances: empty result")
	}
	if resp.UserGetBalancesResult.Currency == "" {
		t.Error("users.getBalances: Currency is empty, want e.g. USD")
	}
	captureFixture(t, client, "users_getBalances", map[string]string{
		"Command": "namecheap.users.getBalances",
	}, nil)
}

func TestAcc_UsersGetPricing(t *testing.T) {
	client := liveClient(t)
	resp, err := client.Users.GetPricingWithContext(ctx(t), &namecheap.UsersGetPricingArgs{
		ProductType: namecheap.String("DOMAIN"),
		ActionName:  namecheap.String("REGISTER"),
		ProductName: namecheap.String("com"),
	})
	if err != nil {
		t.Fatalf("users.getPricing: %v", err)
	}
	if resp == nil || resp.UserGetPricingResult == nil {
		t.Fatal("users.getPricing: empty result")
	}
	// The narrowed query (DOMAIN/REGISTER/com) must yield at least one positive
	// price tier somewhere in the type -> category -> product -> price tree.
	positiveTier := false
	for _, pt := range resp.UserGetPricingResult.ProductTypes {
		for _, cat := range pt.ProductCategories {
			for _, prod := range cat.Products {
				for _, price := range prod.Prices {
					if price.Price.IsPositive() {
						positiveTier = true
					}
				}
			}
		}
	}
	if !positiveTier {
		t.Error("users.getPricing(DOMAIN/REGISTER/com): no positive price tier in response")
	}
	captureFixture(t, client, "users_getPricing", map[string]string{
		"Command":     "namecheap.users.getPricing",
		"ProductType": "DOMAIN",
		"ActionName":  "REGISTER",
		"ProductName": "com",
	}, nil)
}

// TestAcc_UsersAddressGetListAndInfo reads every address-book entry through
// getList and then getInfo, so an id the SDK refuses to send (the primary
// address is AddressId 0, #212) or a flag it fails to decode is caught on the
// real response. Only ids and names are logged: the stored address is personal
// data.
func TestAcc_UsersAddressGetListAndInfo(t *testing.T) {
	client := liveClient(t)
	list, err := client.UsersAddress.GetListWithContext(ctx(t))
	if err != nil {
		t.Fatalf("users.address.getList: %v", err)
	}
	if list == nil || list.AddressGetListResult == nil || len(*list.AddressGetListResult) == 0 {
		t.Fatal("users.address.getList: no entries, want at least the primary address")
	}

	defaults := 0
	defaultID := -1
	for _, e := range *list.AddressGetListResult {
		if e.AddressID == nil {
			t.Errorf("users.address.getList entry %s: AddressId is missing", attr(e.AddressName))
			continue
		}
		id := *e.AddressID
		t.Logf("users.address.getList AddressId=%d AddressName=%s IsDefault=%s", id, attr(e.AddressName), attr(e.IsDefault))
		if e.IsDefault == nil {
			t.Errorf("users.address.getList AddressId=%d: IsDefault is missing", id)
		} else if *e.IsDefault {
			defaults++
			defaultID = id
		}

		info, err := client.UsersAddress.GetInfoWithContext(ctx(t), id)
		if err != nil {
			t.Errorf("users.address.getInfo(%d): %v", id, err)
			continue
		}
		got := info.GetAddressInfoResult
		if got == nil {
			t.Errorf("users.address.getInfo(%d): no GetAddressInfoResult", id)
			continue
		}
		if got.AddressID == nil || *got.AddressID != id {
			t.Errorf("users.address.getInfo(%d) AddressID = %s, want %d", id, attr(got.AddressID), id)
		}
		switch {
		case got.DefaultYN == nil:
			t.Errorf("users.address.getInfo(%d) DefaultYN = nil, want the default flag", id)
		case e.IsDefault != nil && *got.DefaultYN != *e.IsDefault:
			t.Errorf("users.address.getInfo(%d) DefaultYN = %v, want %v (getList IsDefault)", id, *got.DefaultYN, *e.IsDefault)
		}
	}
	if defaults != 1 {
		t.Fatalf("users.address.getList: %d entries have IsDefault=true, want exactly one", defaults)
	}

	captureFixture(t, client, "users_address_getList", map[string]string{
		"Command": "namecheap.users.address.getList",
	}, &fixtureRedaction{
		Container: "AddressGetListResult",
		Keep:      []string{"List"},
		Attrs:     []string{"AddressId", "AddressName", "IsDefault"},
	})
	captureFixture(t, client, "users_address_getInfo", map[string]string{
		"Command":   "namecheap.users.address.getInfo",
		"AddressId": strconv.Itoa(defaultID),
	}, &fixtureRedaction{
		Container:    "GetAddressInfoResult",
		Keep:         []string{"AddressId", "AddressName", "Default_YN"},
		Placeholders: addressPlaceholders,
	})
}

// addressPlaceholders replace the account owner's personal data in the
// committed getInfo fixture with made-up values, so code that stubs the fixture
// gets usable data. Name, street, city, state, zip, country, phone and email
// are the values the sandbox returned for a dummy address on 2026-11-03;
// Country is replaced too, so the address stays consistent.
var addressPlaceholders = map[string]string{
	"UserName":            "sandboxuser",
	"FirstName":           "Test",
	"LastName":            "Probe",
	"JobTitle":            "Engineer",
	"Organization":        "Example Org",
	"Address1":            "1 Test Street",
	"Address2":            "Suite 100",
	"City":                "Phoenix",
	"StateProvince":       "AZ",
	"StateProvinceChoice": "S",
	"Zip":                 "85284",
	"Country":             "US",
	"Phone":               "+1.5555555555",
	"PhoneExt":            "100",
	"Fax":                 "+1.5555555556",
	"EmailAddress":        "user@example.com",
}

// TestAcc_UsersAddFunds creates a credit-card add-funds request on the sandbox
// and reads it back through getAddFundsStatus. Nothing is charged: the token is
// never submitted with card details. It is opt-in through NAMECHEAP_ACC_ADD_FUNDS
// because every run leaves a pending funding request on the account. The write
// results were typed from the docs (#212 found AddressId where the docs wrote
// AddressID), so the token, URLs, transaction id and status are checked on the
// real response.
func TestAcc_UsersAddFunds(t *testing.T) {
	client := liveClient(t)
	if on, _ := strconv.ParseBool(os.Getenv(envAddFunds)); !on {
		t.Skipf("%s not true; skipping users.createaddfundsrequest (leaves a pending funding request)", envAddFunds)
	}
	requireSandbox(t, client, "users.createaddfundsrequest")
	args := &namecheap.UsersCreateAddFundsRequestArgs{
		PaymentType: namecheap.PaymentTypeCreditcard,
		Amount:      "10.00",
		ReturnURL:   "https://example.com/add-funds/return",
	}
	created, err := client.Users.CreateAddFundsRequestWithContext(ctx(t), args)
	if err != nil {
		t.Fatalf("users.createaddfundsrequest: %v", err)
	}

	// The capture runs before any assertion so a response the SDK cannot
	// decode is still written down. It is a second request with a token of
	// its own; the status fixture is captured for that token so the two
	// fixtures agree.
	body := captureFixture(t, client, "users_createaddfundsrequest", map[string]string{
		"Command":     "namecheap.users.createaddfundsrequest",
		"PaymentType": args.PaymentType,
		"Amount":      args.Amount.String(),
		"ReturnUrl":   args.ReturnURL,
	}, nil)
	captureToken := ""
	if m := tokenAttr.FindSubmatch(body); m != nil {
		captureToken = string(m[1])
	}
	if captureToken != "" {
		captureFixture(t, client, "users_getAddFundsStatus", map[string]string{
			"Command": "namecheap.users.getAddFundsStatus",
			"TokenId": captureToken,
		}, nil)
	}

	cr := created.CreateAddFundsRequestResult
	if cr == nil {
		t.Fatal("users.createaddfundsrequest: no CreateAddFundsRequestResult (result element named differently on the wire?)")
	}
	t.Logf("users.createaddfundsrequest TokenID=%s RedirectURL=%s ReturnURL=%s",
		attr(cr.TokenID), attr(cr.RedirectURL), attr(cr.ReturnURL))
	if cr.TokenID == nil || *cr.TokenID == "" {
		t.Error("users.createaddfundsrequest: TokenID is missing, want the token")
	}
	if cr.RedirectURL == nil || *cr.RedirectURL == "" {
		t.Error("users.createaddfundsrequest: RedirectURL is missing")
	}
	// The server sends the payment page URL here, not the ReturnUrl it was
	// given (namecheaptest/fixtures/users_createaddfundsrequest.xml).
	if cr.ReturnURL == nil || cr.RedirectURL == nil || *cr.ReturnURL != *cr.RedirectURL {
		t.Errorf("users.createaddfundsrequest ReturnURL = %s, want RedirectURL %s", attr(cr.ReturnURL), attr(cr.RedirectURL))
	}

	token := deref(cr.TokenID)
	if token == "" {
		token = captureToken
	}
	if token == "" {
		t.Fatal("users.getAddFundsStatus: no token to query")
	}
	status, err := client.Users.GetAddFundsStatusWithContext(ctx(t), token)
	if err != nil {
		t.Fatalf("users.getAddFundsStatus: %v", err)
	}
	sr := status.GetAddFundsStatusResult
	if sr == nil {
		t.Fatal("users.getAddFundsStatus: no GetAddFundsStatusResult (result element named differently on the wire?)")
	}
	t.Logf("users.getAddFundsStatus TransactionID=%s Amount=%s Status=%q", attr(sr.TransactionID), attr(sr.Amount), sr.Status)
	if sr.TransactionID == nil {
		t.Error("users.getAddFundsStatus: TransactionID is missing")
	}
	// The server drops the fractional part of a whole amount ("10" for 10.00),
	// so the comparison is numeric.
	if sr.Amount == nil || !sameAmount(*sr.Amount, args.Amount) {
		t.Errorf("users.getAddFundsStatus Amount = %s, want %s", attr(sr.Amount), args.Amount)
	}
	if sr.ReturnedTokenID == nil || *sr.ReturnedTokenID != token {
		t.Errorf("users.getAddFundsStatus ReturnedTokenID = %s, want %q", attr(sr.ReturnedTokenID), token)
	}
	switch sr.Status {
	case namecheap.AddFundsStatusCreated, namecheap.AddFundsStatusSubmitted, namecheap.AddFundsStatusCompleted,
		namecheap.AddFundsStatusFailed, namecheap.AddFundsStatusExpired:
	default:
		t.Errorf("users.getAddFundsStatus Status = %q, want one of the documented values", sr.Status)
	}
}

// sameAmount reports whether two decimal strings denote the same value.
func sameAmount(a, b namecheap.Amount) bool {
	x, okX := new(big.Rat).SetString(a.String())
	y, okY := new(big.Rat).SetString(b.String())
	return okX && okY && x.Cmp(y) == 0
}

// tokenAttr finds the add-funds token in a raw createaddfundsrequest response,
// whatever the attribute's spelling.
var tokenAttr = regexp.MustCompile(`Token[Ii][Dd]="([^"]+)"`)

// updateProbeFirstName is the FirstName TestAcc_UsersUpdateRoundTrip writes to
// the account profile for the length of the test.
const updateProbeFirstName = "SDKProbe"

// TestAcc_UsersUpdateRoundTrip proves which record users.update writes. It sets
// the profile's FirstName to updateProbeFirstName, reads the primary address
// (users.address.getInfo AddressId=0, #212) back, and restores the original
// values. Writing the same values back would pass whichever record
// users.update wrote, or none. FirstName is the field changed because it is
// required, so the restore always sends it: an optional field that was empty
// could not be cleared again, since the SDK omits empty optional parameters.
// It is opt-in through NAMECHEAP_ACC_USERS_UPDATE because it writes the shared
// account's profile. The profile is personal data: no value is logged, and a
// users.update capture that echoes one is refused before it is written.
func TestAcc_UsersUpdateRoundTrip(t *testing.T) {
	client := liveClient(t)
	if on, _ := strconv.ParseBool(os.Getenv(envUsersUpdate)); !on {
		t.Skipf("%s not true; skipping users.update (writes the account profile)", envUsersUpdate)
	}
	requireSandbox(t, client, "users.update")
	before := primaryAddress(t, client)
	original := profileArgs(before)
	t.Cleanup(func() {
		if _, err := client.Users.UpdateWithContext(context.Background(), original); err != nil {
			t.Errorf("users.update restore: %v; the profile FirstName stays %q", err, updateProbeFirstName)
			return
		}
		compareProfiles(t, "after the restore", before, primaryAddress(t, client))
	})

	probe := *original
	probe.FirstName = updateProbeFirstName
	resp, err := client.Users.UpdateWithContext(ctx(t), &probe)
	if err != nil {
		t.Fatalf("users.update: %v", err)
	}

	// The capture runs before any assertion so a response the SDK cannot
	// decode is still written down. It sends the probe a second time.
	captureFixture(t, client, "users_update", updateParams(&probe), &fixtureRedaction{
		Container: "UserUpdateResult",
		Attrs:     []string{"Success", "UserId"},
		Forbidden: []string{
			original.FirstName, original.LastName, original.JobTitle, original.Organization, original.Address1,
			original.Address2, original.City, original.Zip, original.EmailAddress, original.Phone, original.PhoneExt,
			original.Fax,
		},
	})

	ur := resp.UserUpdateResult
	if ur == nil {
		t.Fatal("users.update: no UserUpdateResult")
	}
	t.Logf("users.update Success=%s UserId=%s", attr(ur.Success), attr(ur.UserID))
	if ur.Success == nil || !*ur.Success {
		t.Errorf("users.update Success = %s, want true", attr(ur.Success))
	}

	after := primaryAddress(t, client)
	if deref(after.FirstName) != updateProbeFirstName {
		t.Errorf("users.address.getInfo(0) FirstName is not the %q users.update wrote: the update did not reach the primary address", updateProbeFirstName)
	}
	written := *after
	written.FirstName = before.FirstName
	compareProfiles(t, "after users.update (FirstName aside)", before, &written)
}

// profileArgs returns the users.update arguments that write a back unchanged.
func profileArgs(a *namecheap.UsersAddressGetInfoResult) *namecheap.UsersUpdateArgs {
	return &namecheap.UsersUpdateArgs{
		FirstName:     deref(a.FirstName),
		LastName:      deref(a.LastName),
		JobTitle:      deref(a.JobTitle),
		Organization:  deref(a.Organization),
		Address1:      deref(a.Address1),
		Address2:      deref(a.Address2),
		City:          deref(a.City),
		StateProvince: deref(a.StateProvince),
		Zip:           deref(a.Zip),
		Country:       deref(a.Country),
		EmailAddress:  deref(a.EmailAddress),
		Phone:         deref(a.Phone),
		PhoneExt:      deref(a.PhoneExt),
		Fax:           deref(a.Fax),
	}
}

// updateParams returns the request parameters UpdateWithContext sends for args,
// for captureFixture's direct POST.
func updateParams(args *namecheap.UsersUpdateArgs) map[string]string {
	params := map[string]string{
		"Command":       "namecheap.users.update",
		"FirstName":     args.FirstName,
		"LastName":      args.LastName,
		"Address1":      args.Address1,
		"City":          args.City,
		"StateProvince": args.StateProvince,
		"Zip":           args.Zip,
		"Country":       args.Country,
		"EmailAddress":  args.EmailAddress,
		"Phone":         args.Phone,
	}
	for k, v := range map[string]string{
		"JobTitle": args.JobTitle, "Organization": args.Organization, "Address2": args.Address2,
		"PhoneExt": args.PhoneExt, "Fax": args.Fax,
	} {
		if v != "" {
			params[k] = v
		}
	}
	return params
}

// compareProfiles reports every field of got that differs from want by name
// only: the values are personal data.
func compareProfiles(t *testing.T, when string, want, got *namecheap.UsersAddressGetInfoResult) {
	t.Helper()
	for _, f := range []struct {
		name      string
		want, got *string
	}{
		{"FirstName", want.FirstName, got.FirstName},
		{"LastName", want.LastName, got.LastName},
		{"JobTitle", want.JobTitle, got.JobTitle},
		{"Organization", want.Organization, got.Organization},
		{"Address1", want.Address1, got.Address1},
		{"Address2", want.Address2, got.Address2},
		{"City", want.City, got.City},
		{"StateProvince", want.StateProvince, got.StateProvince},
		{"StateProvinceChoice", want.StateProvinceChoice, got.StateProvinceChoice},
		{"Zip", want.Zip, got.Zip},
		{"Country", want.Country, got.Country},
		{"EmailAddress", want.EmailAddress, got.EmailAddress},
		{"Phone", want.Phone, got.Phone},
		{"PhoneExt", want.PhoneExt, got.PhoneExt},
		{"Fax", want.Fax, got.Fax},
		{"DefaultYN", boolPtrText(want.DefaultYN), boolPtrText(got.DefaultYN)},
	} {
		if deref(f.want) != deref(f.got) {
			t.Errorf("primary address %s: %s changed (values not logged: personal data)", when, f.name)
		}
	}
}

// requireSandbox fails a write test before its first request unless the client
// targets the sandbox. The opt-in variables choose which writes run, not where:
// under a production profile they would write to the real account.
func requireSandbox(t *testing.T, c *namecheap.Client, command string) {
	t.Helper()
	if !c.ClientOptions.UseSandbox {
		t.Fatalf("%s writes to the account; refusing to run it outside the sandbox (%s is not true)", command, envUseSandbox)
	}
}

// primaryAddress reads the account's primary address (AddressId 0).
func primaryAddress(t *testing.T, c *namecheap.Client) *namecheap.UsersAddressGetInfoResult {
	t.Helper()
	info, err := c.UsersAddress.GetInfoWithContext(ctx(t), 0)
	if err != nil {
		t.Fatalf("users.address.getInfo(0): %v", err)
	}
	if info.GetAddressInfoResult == nil {
		t.Fatal("users.address.getInfo(0): no GetAddressInfoResult")
	}
	return info.GetAddressInfoResult
}

// boolPtrText renders an optional flag as an optional string for comparisons.
func boolPtrText(b *bool) *string {
	if b == nil {
		return nil
	}
	s := strconv.FormatBool(*b)
	return &s
}

// TestAcc_DNSRoundTrip exercises the get -> set -> restore reversible flow on
// a dedicated test domain. It captures the current host records and mail mode,
// writes them back unchanged, and restores them via defer, leaving the zone
// exactly as found.
func TestAcc_DNSRoundTrip(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := liveClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping DNS round-trip", envDomain)
	}

	got, err := client.DomainsDNS.GetHostsWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("dns.getHosts: %v", err)
	}
	captureFixture(t, client, "domains_dns_getHosts", map[string]string{
		"Command": "namecheap.domains.dns.getHosts",
	}, nil)

	if got.DomainDNSGetHostsResult == nil {
		t.Fatalf("dns.getHosts %s: response has no DomainDNSGetHostsResult", domain)
	}
	original := hostRecords(got)
	// The mode goes back with the records: SetHosts rejects MX records without
	// EmailType=MX. An empty attribute means the mode is unset, which setHosts
	// expresses by omitting the parameter, so it is mapped to nil, not sent as "".
	emailType := got.DomainDNSGetHostsResult.EmailType
	if emailType != nil && *emailType == "" {
		emailType = nil
	}

	// Always restore the captured state, even if the round-trip write fails.
	defer func() {
		if _, rerr := client.DomainsDNS.SetHostsWithContext(context.Background(), &namecheap.DomainsDNSSetHostsArgs{
			Domain:    namecheap.String(domain),
			Records:   &original,
			EmailType: emailType,
		}); rerr != nil {
			t.Errorf("dns.setHosts restore failed (zone may be modified): %v", rerr)
		}
	}()

	// Round-trip write: set the exact records we read, a no-op net change that
	// still exercises setHosts end-to-end.
	if _, err := client.DomainsDNS.SetHostsWithContext(ctx(t), &namecheap.DomainsDNSSetHostsArgs{
		Domain:    namecheap.String(domain),
		Records:   &original,
		EmailType: emailType,
	}); err != nil {
		t.Fatalf("dns.setHosts round-trip: %v", err)
	}

	// Read back and verify the zone matches what was written — the round trip
	// must be lossless, not merely error-free.
	reread, err := client.DomainsDNS.GetHostsWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("dns.getHosts read-back: %v", err)
	}
	after := hostRecords(reread)
	if len(after) != len(original) {
		t.Fatalf("dns round-trip: read back %d records, wrote %d", len(after), len(original))
	}
	want := map[string]int{}
	for _, r := range original {
		want[recordKey(r)]++
	}
	for _, r := range after {
		if want[recordKey(r)] == 0 {
			t.Errorf("dns round-trip: unexpected record after write: %s", recordKey(r))
			continue
		}
		want[recordKey(r)]--
	}
	for k, n := range want {
		if n > 0 {
			t.Errorf("dns round-trip: record lost by write: %s", k)
		}
	}
}

// recordKey canonicalizes a host record for set comparison, ignoring order.
func recordKey(r namecheap.DomainsDNSHostRecord) string {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	ttl := ""
	if r.TTL != nil {
		ttl = strconv.Itoa(*r.TTL)
	}
	return strings.ToLower(deref(r.HostName)) + "|" + deref(r.RecordType) + "|" + deref(r.Address) + "|" + ttl
}

// hostRecords maps the detailed host records of a getHosts response to the
// setHosts input shape through the exported RecordFromDetailed, so the round
// trip exercises the mapping callers use.
func hostRecords(resp *namecheap.DomainsDNSGetHostsCommandResponse) []namecheap.DomainsDNSHostRecord {
	if resp == nil || resp.DomainDNSGetHostsResult == nil || resp.DomainDNSGetHostsResult.Hosts == nil {
		return nil
	}
	hosts := *resp.DomainDNSGetHostsResult.Hosts
	records := make([]namecheap.DomainsDNSHostRecord, 0, len(hosts))
	for _, h := range hosts {
		records = append(records, namecheap.RecordFromDetailed(h))
	}
	return records
}

// captureFixture re-captures a raw sandbox response into the committed fixture
// corpus when -update-fixtures is set. It performs a direct POST (bypassing the
// SDK's response decoding) to obtain the verbatim XML body. A response that
// holds personal data passes a redaction (see fixtureRedaction); the rest of the
// body is written unchanged. It returns the body it wrote, or nil when
// -update-fixtures is not set and nothing was captured.
func captureFixture(t *testing.T, c *namecheap.Client, short string, params map[string]string, redaction *fixtureRedaction) []byte {
	t.Helper()
	if !*updateFixtures {
		return nil
	}
	if !c.ClientOptions.UseSandbox {
		t.Fatalf("capture %s: the client does not target the sandbox; fixtures are captured from the sandbox only", short)
	}
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("Username", c.ClientOptions.UserName)
	form.Set("ApiUser", c.ClientOptions.ApiUser)
	form.Set("ApiKey", c.ClientOptions.ApiKey)
	form.Set("ClientIp", c.ClientOptions.ClientIp)

	resp, err := http.Post(c.BaseURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("capture %s: %v", short, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("capture %s: read: %v", short, err)
	}
	if redaction != nil {
		body, err = redaction.apply(body)
		if err != nil {
			t.Fatalf("capture %s: %v; refusing to write personal data", short, err)
		}
	}
	path := filepath.Join("..", "namecheaptest", "fixtures", short+".xml")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("capture %s: write: %v", short, err)
	}
	t.Logf("captured fixture %s (%d bytes)", short, len(body))
	return body
}
