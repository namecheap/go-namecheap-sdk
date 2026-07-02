.PHONY: default format check lint test test-unit test-unit-quiet test-race test-coverage test-sandbox update-fixtures vendor

default: format check lint test

format:
	go fmt ./...

check:
	go vet ./...

test: test-unit test-race

test-unit:
	go test -v -cover -count=1 -parallel=8 ./...

test-unit-quiet:
	go test -cover -count=1 -parallel=8 ./...

test-race:
	go test -race -parallel=8 ./...

test-coverage:
	go test -coverprofile=coverage.out -count=1 -parallel=8 ./...
	go tool cover -func=coverage.out

# test-sandbox runs the build-tagged integration suite against the real Namecheap
# sandbox API. It needs NAMECHEAP_SANDBOX_APIUSER/APIKEY/CLIENTIP (and optionally
# USERNAME and a disposable NAMECHEAP_SANDBOX_DOMAIN) in the environment; without
# them the suite skips cleanly. It is never part of `make test`.
test-sandbox:
	go test -tags sandbox -count=1 -v ./...

# update-fixtures re-captures the read-only sandbox responses into
# namecheaptest/fixtures so drift against the committed corpus surfaces as a diff.
# Requires the same sandbox credentials as test-sandbox.
update-fixtures:
	go test -tags sandbox -count=1 -run TestSandbox ./namecheap/ -update-fixtures

vendor:
	go mod vendor

# Make sure you have installed golangci-lint CLI with the same version
# that is used in github workflows
# https://golangci-lint.run/usage/install/#local-installation
lint:
	golangci-lint run
