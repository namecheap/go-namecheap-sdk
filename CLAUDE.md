# CLAUDE.md — go-namecheap-sdk

## Project overview

Go SDK for the Namecheap API. Module path: `github.com/namecheap/go-namecheap-sdk/v2`. Requires Go 1.22+.

All source code lives in `namecheap/` package. Vendored dependencies in `vendor/`.

## Prerequisites

- Go 1.22+
- golangci-lint

## Verification (run in order)

```bash
make format          # go fmt ./...
make check           # go vet ./...
make lint            # golangci-lint run (16 linters enabled)
make test-unit-quiet # unit tests (only failures shown)
make test-race       # race detector
```

## General principles

- Write clean, idiomatic Go — follow Effective Go and Go Code Review Comments.
- Keep functions small, names descriptive, packages focused.
- Do not add unnecessary comments, docstrings, or type annotations to code you didn't change.
- Avoid over-engineering: no premature abstractions, no feature flags for one-time operations.

## Architecture

### Service pattern

The SDK uses a service-based architecture:

- `Client` is the entry point, created via `NewClient(*ClientOptions)`.
- Three service types embed the unexported `service` struct: `DomainsService`, `DomainsDNSService`, `DomainsNSService`.
- Services are initialized in `NewClient` via type conversion from `client.common`.

### Request/response flow

1. Service methods build a `map[string]string` body with `Command` and parameters.
2. `Client.DoXML()` sends a POST request with URL-encoded body, receives XML response.
3. Responses are decoded via `encoding/xml` into typed structs.
4. API errors (Status="ERROR") are parsed and returned as Go errors.

### Internal retry

`namecheap/internal/syncretry` — mutex-guarded retry with configurable delays. Retries on HTTP 405; other errors propagate immediately.

### Pointer-based optional fields

Optional API fields use pointer types. Helper constructors: `String()`, `Bool()`, `Int()`, `UInt8()`.

### Domain parsing

`ParseDomain()` wraps `publicsuffix.Parse` with regex validation. Returns SLD/TLD split needed for API calls.

### File organization

Each API method gets its own file pair: `domains_dns_get_hosts.go` + `domains_dns_get_hosts_test.go`. Service type definitions live in `domains.go`, `domains_dns.go`, `domains_ns.go`.

## Go conventions

- Always check and return errors — never ignore them silently.
- Use `fmt.Errorf` for error wrapping with context.
- Existing `nolint` directives (`stylecheck`, `revive`) on `ClientOptions` fields are intentional — the API field names match Namecheap's API.
- Do not add new `nolint` directives without justification.

## Testing

- Tests are co-located: `*_test.go` next to source files.
- Use `github.com/stretchr/testify/assert` for assertions.
- Tests use `httptest.NewServer` to mock HTTP responses with fake XML.
- `setupClient()` in `namecheap_test.go` creates a test client with dummy credentials.
- Race detection runs via `make test` (includes `go test -race`).
- Test sub-cases use `t.Run()` for clear naming.

## Security

- Never hardcode real API keys, tokens, or credentials in code or tests.
- Tests use dummy values (`"user"`, `"token"`, `"10.10.10.10"`).
- Use `UseSandbox: true` for development/testing against the sandbox API.
- Do not commit `.env`, `.vault-token`, `*.pem`, `*.key`, or credential files.

