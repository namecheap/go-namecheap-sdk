.PHONY: default format check lint test test-unit test-unit-quiet test-race test-coverage vendor

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

vendor:
	go mod vendor

# Make sure you have installed golangci-lint CLI with the same version
# that is used in github workflows
# https://golangci-lint.run/usage/install/#local-installation
lint:
	golangci-lint run
