# CLAUDE.md — go-namecheap-sdk

## Project overview

Go SDK for the Namecheap API. Module path: `github.com/namecheap/go-namecheap-sdk/v2`. Requires Go 1.26.3+.

All source code lives in `namecheap/` package. Vendored dependencies in `vendor/`.

## Prerequisites

- Go 1.26.3+
- golangci-lint
- gopls (optional — improves navigation; not required)

## Skills

Invoke these golang-skills for specific task types (via the Skill tool):

| Task type | Skill |
|---|---|
| Naming identifiers, receivers, initialisms | `golang-skills:go-naming` |
| Error handling, wrapping, sentinel errors | `golang-skills:go-error-handling` |
| General style, formatting, nesting, clarity | `golang-skills:go-style-core` |
| Writing doc comments for exported symbols | `golang-skills:go-documentation` |
| Writing or reviewing tests | `golang-skills:go-testing` |
| Linting setup or golangci-lint config | `golang-skills:go-linting` |
| Code review against community standards | `golang-skills:go-code-review` |
| Variable/struct/const declarations | `golang-skills:go-declarations` |
| Interfaces and type assertions | `golang-skills:go-interfaces` |
| Generics | `golang-skills:go-generics` |
| Package layout and import organization | `golang-skills:go-packages` |
| Optional constructor parameters | `golang-skills:go-functional-options` |
| Goroutines, channels, mutexes | `golang-skills:go-concurrency` |
| context.Context propagation | `golang-skills:go-context` |
| Conditionals, loops, switch | `golang-skills:go-control-flow` |
| Slices, maps, collections | `golang-skills:go-data-structures` |
| Defensive coding, defer, cleanup | `golang-skills:go-defensive` |
| Function signatures and return values | `golang-skills:go-functions` |
| Structured logging with slog | `golang-skills:go-logging` |
| Performance, benchmarking | `golang-skills:go-performance` |

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

### Error handling

- Always check and return errors — never ignore them silently.
- Default to `%w` for wrapping so callers can use `errors.Is`/`errors.As`. Use `%v` only at system boundaries where leaking internal types would be wrong.
- Place error context before `%w`: `fmt.Errorf("parse domain %q: %w", name, err)`.
- Do not log and return the same error — choose one. Logging is the caller's responsibility.

### Naming

- All identifiers use MixedCaps (exported) or mixedCaps (unexported) — never underscores except in test files for generated names.
- Receiver names are 1–2 letter abbreviations derived from the type name and must be consistent across all methods of that type (e.g., `c` for `Client`, `s` for `DomainsService`).
- No `Get` prefix on getters: use `c.Name()` not `c.GetName()`.
- Initialisms keep consistent case: `userID`, `HTTPClient`, `apiURL` — never `userId`, `HttpClient`, `apiUrl`.
- Do not shadow built-ins (`len`, `cap`, `error`, `string`, etc.).

### Documentation

- Every exported top-level name (type, function, method, constant, variable) must have a doc comment.
- Doc comments start with the name of the thing being described: `// Client is the entry point…`, `// NewClient creates…`.
- Full sentences: capitalized, ending with a period.
- Do not add doc comments to unexported symbols unless the logic is non-obvious.

### Style

- Priority order: Clarity > Simplicity > Concision > Maintainability > Consistency.
- No rigid line length, but keep lines readable; break long lines at semantic boundaries (argument lists, binary operators), not arbitrarily.
- Prefer early returns over deep nesting — guard clauses at the top of functions.
- Existing `nolint` directives (`stylecheck`, `revive`) on `ClientOptions` fields are intentional — the API field names match Namecheap's API.
- Do not add new `nolint` directives without justification.

## Testing

- Tests are co-located: `*_test.go` next to source files.
- Use `github.com/stretchr/testify/assert` for assertions (existing pattern — do not switch to `cmp.Diff`).
- Tests use `httptest.NewServer` to mock HTTP responses with fake XML.
- `setupClient()` in `namecheap_test.go` creates a test client with dummy credentials.
- Race detection runs via `make test` (includes `go test -race`).
- Test sub-cases use `t.Run()` for clear naming.
- Prefer table-driven tests when multiple cases share identical logic.

### Test failure messages

Format: `FuncName(%v) = %v, want %v` — print got before want. Example:

```go
t.Errorf("ParseDomain(%q) = %v, want %v", input, got, want)
```

### Test helpers

- Call `t.Helper()` as the first statement in every test helper function so failure lines point to the caller.
- Use `t.Error` for non-fatal failures (test continues); use `t.Fatal` only when continuing is impossible (setup failure, chained assertions that would panic).

## Security

- Never hardcode real API keys, tokens, or credentials in code or tests.
- Tests use dummy values (`"user"`, `"token"`, `"10.10.10.10"`).
- Use `UseSandbox: true` for development/testing against the sandbox API.
- Do not commit `.env`, `.vault-token`, `*.pem`, `*.key`, or credential files.
