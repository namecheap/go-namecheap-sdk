# Design: complete domains.getInfo response mapping

**Issue:** [#165](https://github.com/namecheap/go-namecheap-sdk/issues/165) —
`DomainsGetInfoResult` doesn't contain Whois (privacy) information.

**Branch:** `fix/domains-get-info-result`

## Problem

`DomainsGetInfoResult` maps only four fields of the `namecheap.domains.getInfo`
response (`DomainName`, `IsPremium`, `PremiumDnsSubscription`, `DnsDetails`).
Privacy (Whoisguard), domain dates, ownership attributes, and modification
rights are silently dropped. Additionally, the field on
`DomainsGetInfoCommandResponse` is misnamed `DomainDNSGetListResult` — a
copy-paste from the DNS-list endpoint.

## Decisions

1. **Full response mapping** — map every element and attribute of the
   documented response, not just the fields the issue names.
2. **Misnamed field: deprecate + accessor** — keep
   `DomainDNSGetListResult` working, mark it `Deprecated:` in its doc comment,
   and add a `DomainGetInfoResult()` method on
   `DomainsGetInfoCommandResponse` returning the same pointer. The field is
   renamed in v3. (A parallel correctly-named field is impossible: two struct
   fields cannot map the same XML element in `encoding/xml`.)
3. **Date typing follows the closest precedent** — `*DateTime` for
   `MM/DD/YYYY` values (as in `domains_get_list.go`), raw `*string` for
   `PremiumDnsSubscription`'s ISO-format timestamps (as in
   `domainprivacy_get_list.go`).

## Struct changes (all in `namecheap/domains_get_info.go`)

`DomainsGetInfoResult` gains attributes `Status *string`, `ID *int`,
`OwnerName *string`, `IsOwner *bool`, and child elements `DomainDetails`,
`WhoisGuard` (XML element `Whoisguard`), `ModificationRights` (XML element
`Modificationrights`).

New types:

```go
type DomainDetails struct {
	CreatedDate *DateTime `xml:"CreatedDate"`
	ExpiredDate *DateTime `xml:"ExpiredDate"`
	NumYears    *int      `xml:"NumYears"`
}

type WhoisGuard struct {
	// Enabled is tri-state: "True", "False", or "NotAlloted".
	Enabled      *string                 `xml:"Enabled,attr"`
	ID           *int                    `xml:"ID"`
	ExpiredDate  *DateTime               `xml:"ExpiredDate"`
	EmailDetails *WhoisGuardEmailDetails `xml:"EmailDetails"`
}

type WhoisGuardEmailDetails struct {
	WhoisGuardEmail              *string `xml:"WhoisGuardEmail,attr"`
	ForwardedTo                  *string `xml:"ForwardedTo,attr"`
	LastAutoEmailChangeDate      *string `xml:"LastAutoEmailChangeDate,attr"`
	AutoEmailChangeFrequencyDays *int    `xml:"AutoEmailChangeFrequencyDays,attr"`
}

type ModificationRights struct {
	All *bool `xml:"All,attr"`
}
```

Existing types gain fields:

- `PremiumDnsSubscription`: `UseAutoRenew *bool`, `SubscriptionID *int`
  (XML `SubscriptionId`), `CreatedDate *string`, `ExpirationDate *string`
  (ISO format, hence raw strings).
- `DnsDetails`: `HostCount *int`, `EmailType *string`,
  `DynamicDNSStatus *bool`, `IsFailover *bool` (all attributes).

### Constraints that shaped the field types

- `Whoisguard Enabled` must be `*string`, not `*bool`: live responses return
  `"NotAlloted"` for domains without an allotted privacy subscription; a bool
  field would fail unmarshaling of the entire response.
- `EmailDetails.LastAutoEmailChangeDate` must be `*string`: the API returns an
  empty attribute value, and an empty string fed to `DateTime.UnmarshalText`
  errors and aborts the whole decode.

### Deliberately not mapped

`LockDetails` stays unmapped: it is empty in both the documented example and
the captured production response, and registrar lock has a dedicated endpoint
(`GetRegistrarLockWithContext`). A doc comment on `DomainsGetInfoResult`
points there.

## What does not change

- Method signatures: `GetInfoWithContext`/`GetInfo` still return
  `*DomainsGetInfoCommandResponse`.
- Request building, `DoXML` pipeline, error handling.
- `DateTime` parsing behavior.

## Testing

Extend `namecheap/domains_get_info_test.go` (existing style: `httptest`
fixtures, testify asserts, `t.Run` sub-cases):

1. Assertions on all new fields against the existing real-world fixture
   (privacy `NotAlloted`, empty `LockDetails`, no `ExpiredDate`).
2. A second fixture covering the populated path: `Whoisguard Enabled="True"`
   with `ExpiredDate` and full `EmailDetails`, `Modificationrights
   All="true"`, `Status`/`ID`/`OwnerName`/`IsOwner` attributes, premium DNS
   fields, and `DnsDetails` extra attributes.
3. Accessor test: `DomainGetInfoResult()` returns the same pointer as the
   deprecated field.

Implementation follows TDD. Verification gates: `make format`, `make check`,
`make lint`, `make test-unit-quiet`, `make test-race`.

## Documentation and release

- Doc comments on every new exported type and non-obvious field, per project
  convention.
- PR titled `fix: ...` (patch release), body ends with `Fixes #165`.
