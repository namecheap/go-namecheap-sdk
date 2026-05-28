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

## Release

We publish a new tagged release once significant changes accumulate. If you need a release with a specific fix,
open an issue or contact us.
