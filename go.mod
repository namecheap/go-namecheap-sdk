module github.com/namecheap/go-namecheap-sdk/v2

// The floor every consumer must meet. 1.26.6 is the first Go 1.26 patch that
// carries the standard-library fixes for advisories govulncheck finds
// reachable from this SDK's own call paths -- encoding/xml from decodeBody,
// encoding/asn1, net/http from doXMLWithCommand. A library cannot fix those
// for its consumers: standard-library fixes ship in the toolchain that
// compiles the final binary, so the only lever that reaches a consumer's build
// is this line. See SECURITY.md.
//
// Raising it is consumer-visible. A consumer on an older patch gets
// "requires go >= 1.26.6" from `go build` until they re-run `go get` /
// `go mod tidy` (which switches toolchain and rewrites their own `go` line),
// and one running GOTOOLCHAIN=local must install a newer Go by hand. Raise it
// only for a reachable advisory, and only in its own release.
go 1.26.6

// What this repository's own builds and CI use: the newest patch, chosen
// independently of the floor above. It applies only while this module is the
// main module, so it imposes nothing on consumers. actions/setup-go reads it
// in preference to `go`, and `go mod tidy` keeps it only while it is newer
// than the `go` line -- if the floor ever catches up, this line disappears.
toolchain go1.26.7

require (
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/stretchr/testify v1.12.1
	github.com/weppos/publicsuffix-go v0.50.3
	golang.org/x/time v0.15.0
)

require (
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)
