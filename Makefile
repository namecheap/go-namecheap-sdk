.PHONY: default format check lint test test-unit test-race vendor

default: format check lint test

format:
	go fmt ./...

check:
	go vet ./...

test: test-unit test-race

test-unit:
	go test -v -cover -count=1 ./...

test-race:
	go test -race ./...

vendor:
	go mod vendor

# Make sure you have installed golangci-lint CLI with the same version
# that is used in github workflows
# https://golangci-lint.run/usage/install/#local-installation
lint:
	golangci-lint run
