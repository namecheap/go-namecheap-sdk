# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Record-level DNS helpers on `DomainsDNSService`, all context-first with the
  `WithContext` suffix and no non-context wrappers:
  `AddRecordsWithContext` (append, preserving all existing records),
  `DeleteRecordsWithContext` (remove selector-matched records, preserve the rest)
  and `UpsertRecordsWithContext` (replace exactly the selector-matched records).
  Each is a read-modify-write over `GetHosts`/`SetHosts` that preserves every
  settable field and the zone `EmailType`, then re-reads and verifies the result,
  returning `ErrConcurrentModification` on a detected lost-update race. They fix
  the `setHosts` "replaces everything" footgun (#49) and de-duplicate logic every
  consumer re-derives (#119).
- `RecordSelector` for exact-match record selection (HostName/RecordType/Address/
  MXPref; empty selector rejected as a typed `*InvalidArgumentsError`; `MatchAll`
  required for an intentional full wipe), and `WithRetryOnConflict(n)` to
  auto-retry the read-modify-write-verify cycle on conflict (#119).
- `PlanWithContext` plus the `RecordOp` constructors `AddOp`/`DeleteOp`/`UpsertOp`
  and the `RecordDiff` type: compute the add/remove/keep diff for a set of
  operations **without writing** (zero `setHosts` calls), for previewing changes
  (#119).
- `RecordFromDetailed` (maps a `GetHosts` `DomainsDNSHostRecordDetailed` to a
  `SetHosts` `DomainsDNSHostRecord`, converting/clamping `MXPref` and consciously
  dropping server-managed read-only fields), plus exported `NormalizeRecord` and
  `RecordsEqual` normalization/comparison helpers (TTL default 1799, hostname
  case, `@`/trailing-dot handling, record-type case). A `reflect`-based
  exhaustiveness test fails loudly if a new struct field is left unmapped (#119).
- `ErrConcurrentModification`, a typed, `errors.Is`-matchable sentinel returned
  when a record-level mutation detects that the zone changed between its read and
  its verifying re-read (#119).
- Eight new `Domains` API methods, all context-first with the `WithContext`
  suffix and no non-context wrappers (these are brand-new, charge-bearing or
  read-only surfaces with no legacy to preserve): `CreateWithContext`,
  `RenewWithContext`, `ReactivateWithContext`, `GetContactsWithContext`,
  `SetContactsWithContext`, `GetRegistrarLockWithContext`,
  `SetRegistrarLockWithContext` and `GetTldListWithContext`. Each returns a typed
  `*...CommandResponse` and carries the Namecheap doc URL (#114).
- Shared `ContactInfo` type used by both `create` and `setContacts` for the
  Registrant/Tech/Admin/AuxBilling blocks, with up-front validation that reports
  **all** missing required contact fields at once as a typed
  `*InvalidArgumentsError` (rather than failing on the first) (#114).
- `Amount`, a string-based money type. `ChargedAmount`, `PremiumPrice` and
  `EapFee` are exposed as `Amount` and parsed from the exact server string —
  money is never modeled as `float64`, avoiding decimal-rounding surprises (#114).
- Premium-domain money-safety guard on `create`/`renew`/`reactivate`: when
  `IsPremiumDomain` is true `PremiumPrice` is mandatory, and when it is false no
  premium pricing may be set. A violation returns an `*InvalidArgumentsError`
  before any charge-bearing request is sent, making an accidental premium
  purchase impossible without explicit acknowledgment (#114).
- Non-idempotent (charge-bearing) request classification. `create`, `renew` and
  `reactivate` are no longer retried on ambiguous transport/server failures that
  may already have executed (which could double-charge); only Namecheap's
  pre-execution HTTP 405 rate-limit signal is retried for them. All other calls
  remain idempotent and retry as before. Implemented by threading an idempotency
  flag through the transport (`Client.doXML`) and `shouldRetry` (#114).
- Typed API errors. API failures now surface as a machine-matchable
  `*APIError` (exposing `Number`, `Message` and `Command`) instead of a flat
  string, so callers can inspect the Namecheap error code via `errors.As`.
  Multi-error responses return an `errors.Join` of `*APIError` values.
  Documented, actionable codes are matchable via `errors.Is` against exported
  sentinels (`ErrDomainNotFound`, `ErrDomainNotAssociated`, `ErrDomainInvalid`,
  `ErrTooManyDomains`, `ErrPromotionCodeInvalid`, `ErrOrderNotFound`,
  `ErrAccessDenied`, `ErrServerError`; each code cross-checked against
  `docs/namecheap-api-v2.md`). Malformed responses return a `*ParseError` with
  a bounded body snippet, and `IsRetryable` classifies transient failures
  (server-side codes and transport timeouts) as retryable. Existing error
  strings are preserved verbatim (`"<message> (<number>)"` and
  `"unable to parse server response: ..."`) so string-matching consumers keep
  working (#111).
- `context.Context` support across all client and service methods. New
  ctx-first `...WithContext` variants (`Client.NewRequestWithContext`,
  `Client.DoXMLWithContext`, and every service method such as
  `Domains.GetInfoWithContext`, `DomainsDNS.SetHostsWithContext`,
  `DomainsNS.CreateWithContext`) thread a context through the request.
  Cancelling the context now aborts an in-flight HTTP request, a pending
  rate-limit or concurrency wait, and any inter-retry backoff sleep (#110).
- Client-side resilience layer: a token-bucket rate limiter, optional
  concurrency bound, and a context-aware exponential-backoff-with-jitter retry
  policy, all configurable via new `ClientOptions` fields. `RateLimitOptions`
  (`PerMinute`, `Disabled`, `MaxConcurrent`) paces requests against Namecheap's
  published quota (default 20/min, burst 20); `RetryOptions` (`MaxAttempts`,
  `MaxElapsed`, `BaseDelay`, `MaxDelay`) governs retries (defaults: 4 attempts,
  500ms base, 30s cap, 2m budget). New transport knobs `HTTPClient`,
  `Transport` (a `http.RoundTripper` override) and `UserAgent` (appended to the
  SDK's default UA) round out the injection points. Requests are now executed
  concurrently and only retried when `IsRetryable` (or the HTTP 405 rate-limit
  signal) says so (#112).

### Changed

- **Behavior change:** API calls are no longer globally serialized. The
  process-wide mutex that forced every request through a single lock is gone;
  requests now run concurrently, bounded by the token-bucket limiter and the
  optional `RateLimit.MaxConcurrent`. Consumers that relied on serialization can
  restore it with `RateLimit.MaxConcurrent: 1` (or a low `RateLimit.PerMinute`)
  (#112).
- A terminal retry failure now wraps the last underlying error as
  `after N attempts: <cause>` (reachable via `errors.Is`/`errors.As`) instead of
  the opaque, untyped `"API retry limit exceeded"` string, which discarded the
  cause (#112).

### Deprecated

- The existing non-context methods (`Client.NewRequest`, `Client.DoXML`, and
  every service method such as `Domains.GetInfo`, `DomainsDNS.SetHosts`,
  `DomainsNS.Create`) are deprecated. They now delegate to their
  `...WithContext` counterparts with `context.Background()` and are slated for
  removal in v3 (#110).

### Removed

- The internal `namecheap/internal/syncretry` package (the global-mutex retry
  loop) has been removed and replaced by the new resilience pipeline. It was an
  internal package, so this is not a public API change (#112).

## [2.5.1] - 2026-05-28

### Fixed

- `DomainsNSService.Delete` and `Update` returning wrong command-response type (#85, closes #75)
- `DoXML` body close: replace deferred close with explicit close after decode (#86, closes #74)
- `ParseDomain` recompiling regex on every call: move to package-level var (#86, closes #76)
- Error wrapping using `%s`/`%v` instead of `%w`, breaking `errors.Is`/`errors.As` (#86, closes #77)
- `DomainsDNSService.SetHosts` using value receiver instead of pointer receiver (#89, closes #78)
- `parseDomainsGetListArgs` and `parseDomainsDNSSetHostsArgs` returning `*map[string]string` unnecessarily (#89, closes #79)
- `DoXML` and `decodeBody` using `interface{}` instead of `any` (#86, closes #80)
- `Domain.String()` and related `String()` methods panicking on nil pointer fields (#84, closes #82)

### Added

- Doc comments on exported functions and package (#87, closes #81)
- Doc comments to `syncretry` package (#88, closes #83)
- Tests for `DateTime.String()` and `DateTime.Equal()` (previously 0% coverage)
- Nil-panic tests for all `String()` methods
- Tests for `DomainsNSService.Delete` and `Update` result parsing
- Error-wrapping test for `ParseDomain`

## [2.5.0] - 2026-05-28

### Added

- `DomainsDNS.GetEmailForwarding` and `DomainsDNS.SetEmailForwarding` methods (#72)
- `Domains.Check` method for domain availability checking (#69)
- CodeQL analysis workflow (#70)
- Trivy security scan with SBOM artifact (#60)
- Namecheap APIv2 method reference at `docs/namecheap-api-v2.md` (#73)

### Fixed

- Import path and package prefix in README usage example (#71)
- `it.com` and other TLD parsing issues via `github.com/weppos/publicsuffix-go` bump (#65)

### Changed

- Go updated to 1.26.3 (#68)
- `golangci-lint` upgraded from v1.60 to v2.8.0 (#63)
- GitHub Actions pinned to SHA hashes; Dependabot added (#57)
- `golang.org/x/net` bumped from 0.34.0 to 0.38.0 (#47)
- `github.com/stretchr/testify` bumped from 1.7.0 to 1.11.1 (#66)

### Security

- Updated `gopkg.in/yaml.v3` to 3.0.1 (CVE-2022-28948) (#59)

## [2.4.1] - 2025-05-26

### Changed

- Added stricter linters and fixed code to comply (#45)

## [2.4.0] - 2024-11-21

### Added

- `mailto` support for CAA IODEF records (#35)

### Changed

- `go mod tidy` dependency cleanup (#34)

## [2.3.0] - 2024-05-07

### Added

- `DomainsNS.Create` method for nameserver creation (#40)
- `DomainsNS.Update` method for nameserver updates (#42)
- `DomainsNS.Delete` method for nameserver deletion (#43)

## [2.2.0] - 2024-03-08

### Added

- `domains.ns.getInfo` API method support (#39)

### Changed

- CI Go version upgraded to 1.22
- Fixed all CI workflow errors and warnings

## [2.1.0] - 2021-11-29

### Added

- `namecheap.domains.getInfo` API support

### Fixed

- `namecheap.domains.dns.getList` failing for FreeDNS domains

## [2.0.2] - 2021-07-28

### Fixed

- API retry when minutely API limits are exceeded (#28)

## [2.0.1] - 2021-07-14

### Added

- `GMAIL` email type support for `DomainsDNS.SetHosts` (#26)
- Public constants for DNS record types and email type values (#27)

## [2.0.0] - 2021-07-07

### Added

- Smart validation for `namecheap.domains.dns.setHosts` API command (#25)

### Changed

- Complete rewrite of v1; v2 is not backward compatible with v1
- Method structure aligned with the official Namecheap API hierarchy
- Improved domain argument parsing (#13)

## [1.3.0] - 2021-06-04

### Changed

- Package migration

## [1.2.0] - 2021-02-11

### Added

- NS record type support
- Debug logging for new requests

### Changed

- Removed dependency on Terraform SDK
- Updated Travis CI config to use Go 1.15

## [1.1.0] - 2019-11-20

### Added

- Domain resolution support

### Changed

- Switched from `hashicorp/terraform` to `hashicorp/terraform-plugin-sdk`
- Upgraded to Go 1.13

[Unreleased]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.5.1...HEAD
[2.5.1]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.5.0...v2.5.1
[2.5.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.0.2...v2.1.0
[2.0.2]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/namecheap/go-namecheap-sdk/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/namecheap/go-namecheap-sdk/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/namecheap/go-namecheap-sdk/releases/tag/v1.1.0
