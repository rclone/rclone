SHELL := /bin/bash

PREFIX=qingstor-sdk-go
VERSION=$(shell cat version.go | grep "Version\ =" | sed -e s/^.*\ //g | sed -e s/\"//g)

.PHONY: help
help:
	@echo "Please use \`make <target>\` where <target> is one of"
	@echo "  all               to check, build, test and release this SDK"
	@echo "  check             to vet and lint the SDK"
	@echo "  update            to update git submodules"
	@echo "  generate          to generate service code"
	@echo "  build             to build the SDK"
	@echo "  test              to run test"
	@echo "  test-coverage     to run test with coverage"
	@echo "  test-race         to run test with race"
	@echo "  integration-test  to run integration test"
	@echo "  clean             to clean the coverage files"

.PHONY: all
all: check build test release

.PHONY: check
check: vet lint format

.PHONY: format
format:
	@echo "go fmt, skipping vendor packages"
	@go fmt ./...
	@echo "ok"

.PHONY: vet
vet:
	@echo "Go tool vet, skipping vendor packages"
	@go vet ./...
	@echo "Done"

.PHONY: lint
lint:
	@echo "Golint, skipping vendor packages"
	@golint ./...
	@echo "Done"

.PHONY: update
update:
	git submodule update --remote
	@echo "Done"

.PHONY: generate
generate:
	@if [[ ! -f "$$(which snips)" ]]; then \
		echo "ERROR: Command \"snips\" not found."; \
	fi
	snips -f="./specs/qingstor/2016-01-06/swagger/api_v2.0.json" -t="./template" -o="./service"
	snips -f="./specs/qingstor/2016-01-06/swagger/api_v2.0.json" -t="./interface" -o="./interface"
	gofmt -w .
	@echo "Done"

.PHONY: build
build: format
	@echo "Build the SDK"
	go build ./...
	@echo "Done"

.PHONY: test
test:
	@echo "Run test"
	go test -v ./...
	@echo "Done"

.PHONY: test-coverage
test-coverage:
	@echo "Run test with coverage"
	@go test -race -coverprofile=coverage.txt -covermode=atomic -v ./...
	@go tool cover -html="coverage.txt" -o "coverage.html"
	@echo "Done"

.PHONY: test-race
test-race:
	@echo "Run test with race"
	go test -v -race -cpu=1,2,4 ./...
	@echo "Done"

.PHONY: integration-test
integration-test:
	@echo "Run integration test"
	pushd "./test"; go test; popd
	@echo "Done"

.PHONY: clean
clean:
	rm -rf $${PWD}/coverage
	@echo "Done"
