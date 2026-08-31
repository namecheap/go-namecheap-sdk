# Security Policy

## Supported versions

Only the latest release of `github.com/namecheap/go-namecheap-sdk/v2` receives security fixes.

## Go toolchain

This module **requires go1.26.6 or newer** — that is the `go` directive in `go.mod`, and 1.26.6 is
the first Go 1.26 patch carrying the standard-library fixes for advisories reachable through this
SDK's own call paths, for example
[GO-2026-6088](https://pkg.go.dev/vuln/GO-2026-6088) (`encoding/xml`, reached from response
decoding) and [GO-2026-5026](https://pkg.go.dev/vuln/GO-2026-5026) (`net/http`, reached from the
request path used by every API call).

Standard-library fixes ship in the Go toolchain, not in this module: they reach your binaries only
when *you* build with a patched toolchain, so this is not something an SDK release can do for you.
The `go` directive is the one lever that does reach your build, which is why it is set to a patch
release rather than a bare `1.26` — and it is a floor, not a ceiling. Newer patches are always
preferred; the floor moves only when an advisory is found reachable from this SDK's own code.

This repository builds and tests itself with the version in the `toolchain` directive of `go.mod`,
and runs [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) over both modules on
every pull request. To check your own build:

```sh
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Use GitHub's [private vulnerability reporting](https://github.com/namecheap/go-namecheap-sdk/security/advisories/new) to report a vulnerability confidentially. We will acknowledge receipt within 5 business days and aim to release a fix within 30 days for confirmed issues.

Alternatively, email `opensource@namecheap.com` with the subject line `[SECURITY] go-namecheap-sdk`.
