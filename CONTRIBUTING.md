# Contributing to Go Namecheap SDK

You're welcome to start a discussion about required features, file an issue or submit a work in progress (WIP) pull
request. Feel free to ask us for help. We'll do our best to guide you and help you to get on it.

## Prerequisites

- Go 1.26.3+
- [golangci-lint](https://golangci-lint.run/usage/install/#local-installation) — install the same version used in CI

## Development workflow

Run all checks before pushing:

```shell
make format          # gofmt
make check           # go vet
make lint            # golangci-lint (must be 0 issues)
make test-unit-quiet # unit tests, failures only
make test-race       # race detector
```

Or run the full suite at once:

```shell
make
```

## Pull request titles

PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `ci:`, `perf:`,
`build:`, `revert:`) — the PR Title Check workflow fails non-conforming PRs.
PRs are squash-merged, so the title becomes the commit message on `master`,
which [release-please](https://github.com/googleapis/release-please) parses to
compute the next version and changelog entry (`fix:` → patch, `feat:` → minor,
`feat!:`/`BREAKING CHANGE:` → major). See [RELEASE.md](RELEASE.md).

## DCO sign-off

This project requires a [Developer Certificate of Origin](https://developercertificate.org/) sign-off on every commit.
Add the following trailer to each commit message (use `git commit -s` or add it manually):

```
Signed-off-by: Your Name <your-email@example.com>
```

Pull requests with commits missing the sign-off will fail the DCO check in CI.

## Tests

Tests live next to the source files (`*_test.go`). Add or update tests for any code you change.
Use `httptest.NewServer` and `setupClient()` to mock HTTP responses — do not use real API credentials.

### Running unit tests

```shell
make test-unit-quiet  # failures only (fast)
make test             # verbose + race detector
```

### Acceptance tests (live API)

A build-tagged suite (`namecheap/acceptance_test.go`, tests named `TestAcc_*`)
runs read-only and reversible commands against the live API. `make testacc` runs
only these tests, and they are never part of `make test`:

```shell
export NAMECHEAP_API_USER=...     # API user
export NAMECHEAP_API_KEY=...      # API key
export NAMECHEAP_CLIENT_IP=...    # your whitelisted public IP
export NAMECHEAP_TEST_DOMAIN=...  # optional disposable domain for getInfo/DNS tests
export NAMECHEAP_USE_SANDBOX=true # optional; targets the sandbox endpoint
make testacc
```

Without credentials the suite skips cleanly. In CI it runs via
[acceptance.yml](.github/workflows/acceptance.yml) on an EC2 runner holding the
repo's whitelisted Elastic IP.

## Adding an auto-paging iterator to a new list endpoint

Paged list endpoints expose `ListAll` / `ListAllSlice` iterators (see issue #120)
built on the shared generic pager in `namecheap/pager.go`. Adding them to a new
paged endpoint is mechanical — you write one small fetch adapter, the pager does
the rest. Copy an existing `*_list_all.go` file (for example
`domains_list_all.go`) and change four things:

1. A `const <endpoint>MaxPageSize = 100` citing the documented maximum from
   `docs/namecheap-api-v2.md`.
2. A `ListAll(ctx, args) iter.Seq2[*ItemType, error]` method that returns
   `pageAll(ctx, <fetch closure>)`.
3. A `ListAllSlice(ctx, args) ([]*ItemType, error)` method that captures the
   total into a variable inside the closure and returns
   `collectAll(seq, &total)`.
4. A `fetch<Endpoint>Page` helper: copy the caller's args (never mutate them),
   set `Page` and default `PageSize` to the max when unset, call the existing
   `GetListWithContext`, and return `(ptrsOf(resp.<Items>), TotalItems, err)`.

The pager's semantics (laziness, clean early break, error-then-stop, context
cancellation) are table-tested once in `pager_test.go`; a new endpoint only needs
a small wiring test that its `ListAllSlice` returns the full multi-page set. Keep
the existing `GetListWithContext` method unchanged so page-level control is
preserved.

## Release

Releases are semi-automated with release-please: merged changes accumulate in a
long-lived release PR, and a maintainer merges it to tag and publish a new
version. See [RELEASE.md](RELEASE.md) for the full flow. If you need a release
with a specific fix, open an issue or contact us.
