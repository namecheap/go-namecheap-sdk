.PHONY: default format check lint test test-unit test-unit-quiet test-race test-coverage testacc update-fixtures vendor

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

# testacc runs ONLY the build-tagged acceptance suite (TestAcc_*) against the
# live Namecheap API — no unit tests. It needs NAMECHEAP_API_USER/
# NAMECHEAP_API_KEY/NAMECHEAP_CLIENT_IP (and optionally NAMECHEAP_USER_NAME and
# a disposable NAMECHEAP_TEST_DOMAIN) in the environment; without them the
# suite skips cleanly. NAMECHEAP_USE_SANDBOX=true points it at the sandbox
# endpoint instead of production. It is never part of `make test`.
testacc:
	go test -tags acceptance -count=1 -v -run TestAcc ./namecheap/

# update-fixtures re-captures the read-only live responses into
# namecheaptest/fixtures so drift against the committed corpus surfaces as a diff.
# Requires the same credentials as testacc.
update-fixtures:
	go test -tags acceptance -count=1 -run TestAcc ./namecheap/ -update-fixtures

vendor:
	go mod vendor

# Make sure you have installed golangci-lint CLI with the same version
# that is used in github workflows
# https://golangci-lint.run/usage/install/#local-installation
lint:
	golangci-lint run
