# Design: complete domains.getInfo response mapping

**Issue:** [#165](https://github.com/namecheap/go-namecheap-sdk/issues/165) —
`DomainsGetInfoResult` doesn't contain Whois (privacy) information.

**Branch:** `fix/domains-get-info-result`

**Revision note:** validated against six independent captured/documented
getInfo responses, an `encoding/xml` behavior probe, and two independent
design reviews. Two decisions changed from the first draft and are flagged
inline: the accessor name (was `DomainGetInfoResult()`, now `Result()`) and
`DateTime` empty-input tolerance (was "no parser changes").

## Problem

`DomainsGetInfoResult` maps only four fields of the `namecheap.domains.getInfo`
response (`DomainName`, `IsPremium`, `PremiumDnsSubscription`, `DnsDetails`).
Privacy (Whoisguard), domain dates, ownership attributes, and modification
rights are silently dropped. Additionally, the field on
`DomainsGetInfoCommandResponse` is misnamed `DomainDNSGetListResult` — a
copy-paste from the DNS-list endpoint.

## Decisions

1. **Full mapping of the `DomainGetInfoResult` subtree** — every element and
   attribute observed across the documented example and five captured
   responses. Envelope-level nodes (`Server`, `GMTTimeDifference`,
   `ExecutionTime`, etc.) stay unmapped, consistent with the rest of the SDK.
2. **Misnamed field: deprecate + accessor named `Result()`** — keep
   `DomainDNSGetListResult` working, deprecate it, and add a
   `Result()` method on `DomainsGetInfoCommandResponse` returning the same
   pointer. *(Changed from `DomainGetInfoResult()`: a method may not share the
   name of a field on the same struct — it fails to compile — so an accessor
   named after the v3 field would have to be deleted in v3, breaking migrated
   consumers a second time. `Result()` survives the v3 rename.)* A parallel
   correctly-named field is impossible: two struct fields with the same XML
   tag make `xml.Unmarshal` return a hard runtime error (`field ... conflicts
   with field ...`) on every call.
3. **Date typing follows the closest precedent** — `*DateTime` for
   `MM/DD/YYYY` values (as in `domains_get_list.go`), raw `*string` for
   `PremiumDnsSubscription`'s ISO-format timestamps (as in
   `domainprivacy_get_list.go`; `0001-01-01T00:00:00` is a "never" sentinel
   and does not parse as `MM/DD/YYYY`).
4. **`DateTime` gains empty-input tolerance** *(changed from "no parser
   changes")*: `UnmarshalText` trims surrounding whitespace and treats
   empty/whitespace-only input as the zero value instead of erroring. This
   design introduces the SDK's first element-body `*DateTime` fields
   (`DomainDetails.CreatedDate`/`ExpiredDate`, `WhoisGuard.ExpiredDate`); all
   seven existing uses are attributes. Probing shows `encoding/xml` hands
   element chardata to `TextUnmarshaler` untrimmed, and an empty element
   (`<ExpiredDate/>`) or padded text aborts the entire response decode —
   `GetInfo` would fail outright for such domains. The change is strictly
   widening: every input that parsed before still parses to the same value.

## Struct changes (all in `namecheap/domains_get_info.go`)

`DomainsGetInfoResult` gains attributes `Status *string` (observed values
`Ok`, `Locked`, `Expired`), `ID *int`, `OwnerName *string`, `IsOwner *bool`,
and child elements `DomainDetails`, `WhoisGuard` (XML element `Whoisguard` —
the type name tracks the wire element; newer privacy code calls this "the
legacy element"), `ModificationRights` (XML element `Modificationrights`).

New types:

```go
type DomainDetails struct {
	CreatedDate *DateTime `xml:"CreatedDate"`
	ExpiredDate *DateTime `xml:"ExpiredDate"`
	NumYears    *int      `xml:"NumYears"`
}

type WhoisGuard struct {
	// Enabled is tri-state: "True", "False", or "NotAlloted".
	// Do not feed it to ClassifyPrivacyStatus (see Decision 6).
	Enabled      *string                 `xml:"Enabled,attr"`
	ID           *int                    `xml:"ID"` // 0 when Enabled="NotAlloted"
	ExpiredDate  *DateTime               `xml:"ExpiredDate"`
	EmailDetails *WhoisGuardEmailDetails `xml:"EmailDetails"`
}

type WhoisGuardEmailDetails struct {
	// Field names mirror the API's attribute names verbatim, stutter included.
	WhoisGuardEmail *string `xml:"WhoisGuardEmail,attr"`
	ForwardedTo     *string `xml:"ForwardedTo,attr"`
	// LastAutoEmailChangeDate is empty or MM/DD/YYYY; consumers parse it.
	LastAutoEmailChangeDate      *string `xml:"LastAutoEmailChangeDate,attr"`
	AutoEmailChangeFrequencyDays *int    `xml:"AutoEmailChangeFrequencyDays,attr"`
}

type ModificationRights struct {
	All    *bool                `xml:"All,attr"`
	Rights *[]ModificationRight `xml:"Rights"`
}

// ModificationRight is one granular right, e.g. Type="rlock" Value="OK".
// Captured Types: dns, eforward, rlock, parkpage, hosts, ddns, extend,
// nameserver, autorenew, whoisguard, premiumDns, dnsSec.
type ModificationRight struct {
	Type  *string `xml:"Type,attr"`
	Value *string `xml:",chardata"` // only "OK" observed; not typed as bool
}
```

`Rights` is essential, not optional: a permissioned/shared domain returns
`All="false"` plus the granular list — dropping it reports "no rights" for
exactly the domains where the detail matters.

Existing types gain fields:

- `PremiumDnsSubscription`: `UseAutoRenew *bool` (**the premium DNS
  subscription's auto-renew, not the domain's** — doc comment must say so),
  `SubscriptionID *int` (XML `SubscriptionId`; `-1` sentinel = none, so it
  stays signed), `CreatedDate *string`, `ExpirationDate *string` (ISO
  format; `0001-01-01T00:00:00` sentinel = never).
- `DnsDetails`: `HostCount *int`, `EmailType *string`,
  `DynamicDNSStatus *bool`, `IsFailover *bool` (all attributes).

Every field is a pointer; captures show whole sections absent per domain
(`IsPremium`, `NumYears`, `EmailDetails`, `PremiumDnsSubscription` each
missing somewhere). Known divergence, kept deliberately: `ID *int` here vs
`Domain.ID *string` in getList (matches newer `DomainPrivacyGetListEntry.ID`).

### Constraints that shaped the field types (probe-verified)

- `Whoisguard Enabled` must be `*string`, not `*bool`: `"NotAlloted"` fails
  `strconv.ParseBool` and aborts the entire response decode.
- XML tag casing is load-bearing and fails **silently**: a wrong-case tag
  (`WhoisGuard` vs `Whoisguard`) leaves the field nil with no error. Tags are
  verified character-for-character against captures — note the asymmetry:
  element `Whoisguard` (lowercase g) vs attribute `WhoisGuardEmail`
  (capital G); `Modificationrights` (lowercase r); `SubscriptionId`.

### Deliberately not mapped

`LockDetails` stays unmapped: empty in all six sources. Lock information
getInfo *does* carry: `Status` (`Locked`) and the `ModificationRight` with
`Type="rlock"`. The doc comment on `DomainsGetInfoResult` names those and
points to `GetRegistrarLockWithContext` as the authoritative check.

## Adjacent fixes folded in (same types, verified defects)

5. **Nil-guard the FreeDNS fallback** — `domains_dns_get_list.go:69-75`
   dereferences `DomainDNSGetListResult`, `DnsDetails`, `ProviderType`, and
   `PremiumDnsSubscription` unchecked. The officially documented getInfo
   response (`<DnsDetails ProviderType="ENOM"/>`, no `PremiumDnsSubscription`)
   panics there today. Guard each pointer; absent data degrades to nil/false
   fields in the synthesized result. This call site is also the in-repo
   consumer of the deprecated field (likely origin of the copy-paste name) —
   it keeps using the field internally until the v3 rename, with the
   `//nolint:staticcheck` that requires.
6. **`ClassifyPrivacyStatus` inversion for the getInfo vocabulary** —
   `"NotAlloted"` normalizes to `"notalloted"`, which contains `"allot"` but
   not `"unallot"`, so the classifier returns `PrivacyStateAllotted` — the
   opposite of the truth — for the very value this change introduces. Add
   `"notallot"` to the FREE case (evaluated before the ALLOTTED case) in
   `domainprivacy.go`, and extend its doc comment.

## What does not change

- Method signatures: `GetInfoWithContext`/`GetInfo` still return
  `*DomainsGetInfoCommandResponse`.
- Request building, `DoXML` pipeline, error handling.
- `DateTime` parsing of non-empty values (Decision 4 widens empty/padded
  input only).

## Not available from this endpoint (issue expectations)

The issue asks about "renewal": getInfo has **no domain-level auto-renew or
renewal field** in any capture or the official response table. Renewal-
adjacent data here: `DomainDetails.ExpiredDate`, `Status="Expired"`,
`ModificationRight Type="autorenew"` (permission to change it, not its
value). Domain auto-renew and lock booleans live on `domains.getList`
(`Domain.AutoRenew`, `Domain.IsLocked`). The PR description and the
`DomainsGetInfoResult` doc comment state this so the issue closes honestly.

## Documentation

- Doc comments on every new exported type and non-obvious field.
- The five pre-existing exported types in the file currently have **no** doc
  comments (`DomainsGetInfoResponse`, `DomainsGetInfoCommandResponse`,
  `DomainsGetInfoResult`, `PremiumDnsSubscription`, `DnsDetails`); nothing in
  the lint gate catches that. Add doc comments to all five while touching
  them.
- Deprecated field format matches the file's existing convention: leading
  descriptive sentence, blank `//`, then `Deprecated: misnamed; use
  Result(). The field will be renamed to DomainGetInfoResult in v3.`
- Extend `docs/namecheap-api-v2.md` (getInfo section, ~line 379): it
  currently documents only the six attributes and no child elements.

## Testing

Extend `namecheap/domains_get_info_test.go` (existing style: `httptest`
fixtures, testify asserts, `t.Run` sub-cases). Three fixtures, all derived
from real captures rather than hand-written XML:

1. **Repo production fixture** (privacy `NotAlloted`, empty `LockDetails`,
   no `ExpiredDate`, `Modificationrights` empty): all new fields asserted.
2. **Managed/permissioned capture**: `Whoisguard Enabled="True"` with
   `ExpiredDate` and full `EmailDetails` (`LastAutoEmailChangeDate`
   populated), `Modificationrights All="false"` with the 12 `Rights`
   children, `Status`/`ID`/`OwnerName`/`IsOwner`, premium DNS and
   `DnsDetails` attributes.
3. **Sparse documented example** (`<DnsDetails ProviderType="ENOM"/>`, no
   `PremiumDnsSubscription`, no `NumYears`, no `EmailDetails`): decodes with
   nil optionals — and drives the FreeDNS-fallback nil-guard test in
   `domains_dns_get_list_test.go` (panics today).

Test-design rules: assert **non-nil per struct**, not just `err == nil` — a
tag-casing typo ships green otherwise. Accessor test asserts `Result()`
returns the same pointer as the deprecated field; that one comparison line
carries a justified `//nolint:staticcheck // SA1019: deprecated field is the
accessor's backing store`. `DateTime` tests cover empty element, padded
text, and `MM/DD/YYYY` (Decision 4). Classifier test covers `"NotAlloted"`.

Implementation follows TDD. Verification gates: `make format`, `make check`,
`make lint`, `make test-unit-quiet`, `make test-race`.

## Release

- PR titled `fix: ...` (patch release), body ends with `Fixes #165`.
- All changes are additive or nil-safe; no consumer-visible breakage. The v3
  rename plan (field → `DomainGetInfoResult`, accessor `Result()` retained)
  is recorded in the deprecation notice.
