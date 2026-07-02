# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- New `SSLService` (`client.SSL`), context-first with the `WithContext` suffix,
  covering the full `namecheap.ssl.*` group — all 13 commands across three phases:
  inventory (`GetListWithContext`, `GetInfoWithContext`, `ParseCSRWithContext`),
  activation (`ActivateWithContext`, `GetApproverEmailListWithContext`,
  `ResendApproverEmailWithContext`, `EditDCVMethodWithContext`) and the
  charge-bearing money operations (`CreateWithContext`, `RenewWithContext`,
  `ReissueWithContext`, `PurchaseMoreSansWithContext`,
  `RevokeCertificateWithContext`, `ResendFulfillmentEmailWithContext`).
  Args/response structs match `docs/namecheap-api-v2.md` (ssl section, lines
  785-1101); the 13th command is `editDCVMethod` per the doc (the issue's
  "editDNSDSCRecords" is a typo) (#116).
- Typed `DCVMethod` enum (`DCVMethodHTTP`, `DCVMethodDNS`, `DCVMethodEmail`) with
  client-side, per-method required-field validation for `Activate` and
  `EditDCVMethod`: an invalid method cannot be expressed as a valid value, and
  email validation requires an `ApproverEmail`, reported together with every other
  missing field via `*InvalidArgumentsError` (table-tested method × missing-field).
  The doc names a `DCVMethod` parameter but enumerates no values, so the wire
  tokens are grounded in the documented Namecheap DCV flow (flagged in code and
  README) (#116).
- Grounded `CertStatus` type mirroring the documented certificate-status
  vocabulary (`ACTIVE`/`NEWPURCHASE`/`NEWRENEWAL`/`PURCHASED`/`PURCHASEERROR`/
  `CANCELLED`/`UNKNOWN`, doc lines 948-957) with `ClassifyStatus`, plus
  `SSLGetInfoResult.IsIssued()` (true only for `ACTIVE`) and
  `IsExpiringSoon(within)`. All expiry math lives in one tested, timezone-safe
  helper with an inclusive boundary (boundary-tested exactly at the threshold);
  the raw status string is exposed verbatim (#116).
- SSL money operations `Create`, `Renew`, `Reissue` and `PurchaseMoreSans` are
  **non-idempotent**: they use the same retry classification as `domains.create`
  (`doXML(..., false)`), so an ambiguous transport/server failure is never
  auto-retried (only the pre-execution HTTP 405 rate-limit signal is). Read,
  approver and revoke/resend calls are idempotent. A test asserts no re-fire after
  a simulated server error (call-count == 1) for `Create` and `Reissue` (#116).
- Multi-SAN (host-block) support on `Activate` and `EditDCVMethod`: N SAN entries
  serialize deterministically (indexed `SANDomainName[i]`/`SANDCVMethod[i]` for
  activate; comma-separated `DNSNames`/`DCVMethods` for editDCVMethod) and are
  round-trip tested with 3+ SANs across mixed DCV methods (#116).
- Key/CSR generation is documented as an explicit non-goal: the SDK transports the
  CSR string only and never generates or stores private keys (README points at
  `crypto/x509` for generating a CSR) (#116).
- New `DomainsTransferService` (`client.DomainsTransfer`), context-first with the
  `WithContext` suffix, covering the full `namecheap.domains.transfer.*` group:
  `CreateWithContext`, `GetStatusWithContext`, `UpdateStatusWithContext` and
  `GetListWithContext` (filtered/paged). Args/response structs match
  `docs/namecheap-api-v2.md` (lines 681-781) (#115).
- `Create` is charge-bearing and **non-idempotent**: it uses the same retry
  classification as `domains.create`, so an ambiguous transport/server failure is
  never auto-retried (only the pre-execution HTTP 405 rate-limit signal is);
  `GetStatus`/`UpdateStatus`/`GetList` are idempotent. A test asserts no duplicate
  submit after a simulated server error/timeout (call-count == 1) (#115).
- Grounded `TransferState` machine: constants `INPROGRESS`/`COMPLETED`/`CANCELLED`
  /`UNKNOWN` mirror the documented `getList` category vocabulary, with
  `ClassifyTransferStatus(status)`, `TransferState.IsTerminal()` and
  `TransferState.IsActionRequired()` plus a `TransferState()` helper on the
  getStatus response. The doc enumerates **no numeric `StatusID` codes**, so the
  raw `StatusID` (int) and `Status` (string) are exposed verbatim and no numeric
  constants are fabricated; classification is by case-insensitive keyword matching
  and is table-tested (#115).
- `WaitForCompletionWithContext(ctx, transferID, opts...)` polls `GetStatus` until
  the transfer is terminal, with a configurable interval (`WithPollInterval`,
  default 30s) and prompt mid-poll context cancellation (#115).
- EPP code redaction: `EPPCode` is added to the observability secret-key set, so
  the transfer authorization code is replaced with `***` on every hook and log
  record (same mechanism as `ApiKey`); a grep-all-observable-output test asserts
  the value appears zero times (#113, #115).
- Observability: request/response hooks `ClientOptions.OnRequest` and
  `OnResponse` (new `RequestInfo`/`ResponseInfo` types) that fire once per HTTP
  attempt — retries included — with a 1-based `Attempt`. Ordering is documented:
  the rate-limiter wait happens first, then `OnRequest` fires immediately before
  the HTTP send, then `OnResponse` after the attempt with status, duration, error
  code and whether a retry will follow. Panicking hooks are recovered (and logged
  when a `Logger` is set), never crashing the caller or aborting the request (#113).
- Mandatory redaction: `RequestInfo.Params` is always a redacted copy — the
  values of the secret keys `ApiKey`, `NewPassword`, `OldPassword` and
  `ResetCode` are replaced with `***` before reaching any hook or log record. The
  key set lives in one place and is trivial to extend; a test greps every string
  the hooks and slog emit and asserts the credential appears zero times (#113).
- `ClientOptions.Logger *slog.Logger`: opt-in structured logging on the same
  request path (stdlib `log/slog`, no new dependency). Request start and
  limiter-wait at `Debug`, success at `Info`, retryable failure/retry at `Warn`;
  records carry `command`, `attempt`, `duration`, `status` and `error_code` and
  only ever log redacted parameters. With no hooks and no logger, the
  observability path allocates nothing (benchmark-guarded) (#113).
- `Client.Stats()` returns a snapshot (deep copy) of cumulative counters:
  `RequestsByCommand`, `ErrorsByCode`, `Retries`, `TotalLimiterWait` and a
  best-effort `QuotaRemaining` estimate — enough to export Prometheus/OTel
  metrics without the SDK depending on either. Counters are race-safe under
  concurrent load (#113).
- New opt-in `otelnamecheap` submodule (its own `go.mod`, so the core SDK stays
  OpenTelemetry-free): `otelnamecheap.NewTransport` wraps an `http.RoundTripper`
  for `ClientOptions.Transport` and emits one client span per API-call attempt
  with command and HTTP-status attributes and an error status on failure. It
  reads the request body only to extract the command, never a secret (#113).
- New `UsersService` (`client.Users`), context-first with the `WithContext`
  suffix and no non-context wrappers: `GetPricingWithContext`,
  `GetBalancesWithContext`, `CreateAddFundsRequestWithContext`,
  `GetAddFundsStatusWithContext`, `ChangePasswordWithContext` and
  `UpdateWithContext`. Every struct field is cross-checked against
  `docs/namecheap-api-v2.md`; each method carries the Namecheap doc URL (#117).
- `GetPricing` models the deeply nested price sheet as navigable typed structs
  (`ProductType → ProductCategory → Product → Price` tiers) matching the doc's
  element/attribute names, plus the flattening helper
  `UsersGetPricingResult.PriceFor(action, productName, years)` for the common
  single-tier lookup and `Price.EffectivePrice()`, which resolves the documented
  precedence (server-resolved `Price` incl. promo/special → `YourPrice` →
  `RegularPrice`). All monetary values are `Amount` (exact decimal strings), never
  `float64`. Fixtures cover DOMAIN, SSLCERTIFICATE and WHOISGUARD (#117).
- `GetBalances` returns typed decimal-safe amounts (`Amount`) and currency (#117).
- Add-funds flow with `AddFundsStatus` constants (`CREATED`, `SUBMITTED`,
  `COMPLETED`, `FAILED`, `EXPIRED`). `CreateAddFundsRequest` is classified
  **non-idempotent** (charge-bearing): an ambiguous transport/server failure is
  never retried (only Namecheap's pre-execution HTTP 405 is), so it can never
  double-charge; reconcile via `GetAddFundsStatus` (#117).
- `ChangePassword` supports both the old-password and reset-code methods; password
  values are only ever placed in the outbound request parameters — never stored or
  logged. Hook-level redaction is deferred to the logging layer (#113) (#117).
- New `UsersAddressService` (`client.UsersAddress`) with full context-first CRUD:
  `CreateWithContext`, `UpdateWithContext`, `DeleteWithContext`,
  `GetInfoWithContext`, `GetListWithContext`, `SetDefaultWithContext` (#117).
- `ContactInfo` ↔ address-book adapter, bidirectional and tested for no field
  drift: `ContactInfo.ToAddressDetails`, `UsersAddressDetails.ToContactInfo` and
  `UsersAddressGetInfoResult.ToContactInfo`, so a stored address can feed the
  `domains.create` contact blocks. The two renamed correspondences
  (`PostalCode` ↔ `Zip`, `OrganizationName` ↔ `Organization`) are mapped
  explicitly (#117).
- The reseller account-creation surface (`users.create`, `users.login`,
  `users.resetPassword`) is deferred (planned, unscheduled) and documented in the
  README coverage matrix with the reseller-only rationale (#117).
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
