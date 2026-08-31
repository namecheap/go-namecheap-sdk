.PHONY: default format check lint test test-unit test-unit-quiet test-race test-coverage testacc update-fixtures vendor

# testify links a YAML parser solely for YAMLEq/YAMLEqf, which this repo never
# calls (`grep -rn YAMLEq` finds nothing outside vendor/). The build tag below
# drops gopkg.in/yaml.v3 -- 11,362 vendored lines -- from the test binaries:
#   go list -deps -test ./...                          -> gopkg.in/yaml.v3 present
#   go list -tags testify_yaml_fail -deps -test ./...  -> absent
# Only testify's own thin assert/yaml shim remains, compiled to its stub, so a
# YAMLEq assertion added later fails loudly at run time rather than silently
# linking the parser back in. This does NOT shrink vendor/ or the module graph:
# `go mod vendor` and `go mod tidy` evaluate every build tag regardless.
GO_TEST_TAGS := testify_yaml_fail

default: format check lint test

format:
	go fmt ./...

check:
	go vet ./...

test: test-unit test-race

test-unit:
	go test -tags '$(GO_TEST_TAGS)' -v -cover -count=1 -parallel=8 ./...

test-unit-quiet:
	go test -tags '$(GO_TEST_TAGS)' -cover -count=1 -parallel=8 ./...

test-race:
	go test -tags '$(GO_TEST_TAGS)' -race -parallel=8 ./...

test-coverage:
	go test -tags '$(GO_TEST_TAGS)' -coverprofile=coverage.out -count=1 -parallel=8 ./...
	go tool cover -func=coverage.out

# testacc runs ONLY the build-tagged acceptance suite (TestAcc_*) against the
# live Namecheap API — no unit tests. It needs NAMECHEAP_API_USER/
# NAMECHEAP_API_KEY/NAMECHEAP_CLIENT_IP (and optionally NAMECHEAP_USER_NAME and
# a disposable NAMECHEAP_TEST_DOMAIN) in the environment; without them the
# suite skips cleanly. NAMECHEAP_USE_SANDBOX=true points it at the sandbox
# endpoint instead of production. It is never part of `make test`.
testacc:
	go test -tags 'acceptance,$(GO_TEST_TAGS)' -count=1 -v -run TestAcc ./namecheap/

# update-fixtures re-captures the read-only live responses into
# namecheaptest/fixtures so drift against the committed corpus surfaces as a diff.
# Requires the same credentials as testacc.
update-fixtures:
	go test -tags 'acceptance,$(GO_TEST_TAGS)' -count=1 -run TestAcc ./namecheap/ -update-fixtures

vendor:
	go mod vendor

# Make sure you have installed golangci-lint CLI with the same version
# that is used in github workflows
# https://golangci-lint.run/usage/install/#local-installation
lint:
	golangci-lint run
