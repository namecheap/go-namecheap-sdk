# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `context.Context` support across all client and service methods. New
  ctx-first `...WithContext` variants (`Client.NewRequestWithContext`,
  `Client.DoXMLWithContext`, and every service method such as
  `Domains.GetInfoWithContext`, `DomainsDNS.SetHostsWithContext`,
  `DomainsNS.CreateWithContext`, plus `syncretry.SyncRetry.DoContext`) thread a
  context through the request. Cancelling the context now aborts an in-flight
  HTTP request, a pending inter-retry sleep, and waiting on the internal retry
  lock (#110).

### Deprecated

- The existing non-context methods (`Client.NewRequest`, `Client.DoXML`,
  `syncretry.SyncRetry.Do`, and every service method such as `Domains.GetInfo`,
  `DomainsDNS.SetHosts`, `DomainsNS.Create`) are deprecated. They now delegate
  to their `...WithContext` counterparts with `context.Background()` and are
  slated for removal in v3 (#110).

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
