module github.com/namecheap/go-namecheap-sdk/v2

go 1.26.3

// Build and test this module with a patched toolchain. Every Go patch release
// after 1.26.3 carries standard-library security fixes that govulncheck finds
// reachable from this code (encoding/xml via decodeBody, encoding/asn1,
// net/http via doXMLWithCommand). `toolchain` applies only when this module is
// the main module, so it does not raise the minimum Go version for consumers
// -- that is still the `go` directive above. actions/setup-go reads this line
// in preference to `go`, so CI compiles against it too. Bump it when
// govulncheck reports a new standard-library advisory.
toolchain go1.26.7

require (
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/stretchr/testify v1.12.0
	github.com/weppos/publicsuffix-go v0.50.3
	golang.org/x/time v0.15.0
)

require (
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
