SHELL := /bin/bash

PREFIX=qingstor-sdk-go
VERSION=$(shell cat version.go | grep "Version\ =" | sed -e s/^.*\ //g | sed -e s/\"//g)
DIRS_TO_CHECK=$(shell ls -d */ | grep -vE "vendor|test")
PKGS_TO_CHECK=$(shell go list ./... | grep -v "/vendor/")
PKGS_TO_RELEASE=$(shell go list ./... | grep -vE "/vendor/|/test")
FILES_TO_RELEASE=$(shell find . -name "*.go" | grep -vE "/vendor/|/test|.*_test.go")
FILES_TO_RELEASE_WITH_VENDOR=$(shell find . -name "*.go" | grep -vE "/test|.*_test.go")

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
	@echo "  release           to build and release current version"
	@echo "  release-source    to pack the source code"
	@echo "  clean             to clean the coverage files"

.PHONY: all
all: check build unit release

.PHONY: check
check: vet lint format

.PHONY: format
format:
	@echo "go fmt, skipping vendor packages"
	@for pkg in ${PKGS_TO_CHECK}; do go fmt $${pkg}; done;
	@echo "ok"

.PHONY: vet
vet:
	@echo "Go tool vet, skipping vendor packages"
	@go tool vet -all ${DIRS_TO_CHECK}
	@echo "Done"

.PHONY: lint
lint:
	@echo "Golint, skipping vendor packages"
	@lint=$$(for pkg in ${PKGS_TO_CHECK}; do golint $${pkg}; done); \
	 lint=$$(echo "$${lint}"); \
	 if [[ -n $${lint} ]]; then echo "$${lint}"; exit 1; fi
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
	gofmt -w .
	@echo "Done"

.PHONY: build
build: format
	@echo "Build the SDK"
	go build ${PKGS_TO_RELEASE}
	@echo "Done"

.PHONY: test
test:
	@echo "Run test"
	go test -v ${PKGS_TO_RELEASE}
	@echo "Done"

.PHONY: test-coverage
test-coverage:
	@echo "Run test with coverage"
	for pkg in ${PKGS_TO_RELEASE}; do \
		output="coverage$${pkg#github.com/yunify/qingstor-sdk-go}"; \
		mkdir -p $${output}; \
		go test -v -cover -coverprofile="$${output}/profile.out" $${pkg}; \
		if [[ -e "$${output}/profile.out" ]]; then \
			go tool cover -html="$${output}/profile.out" -o "$${output}/profile.html"; \
		fi; \
	done
	@echo "Done"

.PHONY: test-race
test-race:
	@echo "Run test with race"
	go test -v -race -cpu=1,2,4 ${PKGS_TO_RELEASE}
	@echo "Done"

.PHONY: integration-test
integration-test:
	@echo "Run integration test"
	pushd "./test"; go test; popd
	@echo "Done"

.PHONY: release
release: release-source release-source-with-vendor

.PHONY: release-source
release-source:
	@echo "Pack the source code"
	mkdir -p "release"
	zip -FS "release/${PREFIX}-source-v${VERSION}.zip" ${FILES_TO_RELEASE}
	@echo "Done"

.PHONY: release-source-with-vendor
release-source-with-vendor:
	@echo "Pack the source code with vendor"
	mkdir -p "release"
	zip -FS "release/${PREFIX}-source-with-vendor-v${VERSION}.zip" ${FILES_TO_RELEASE_WITH_VENDOR}
	@echo "Done"

.PHONY: clean
clean:
	rm -rf $${PWD}/coverage
	@echo "Done"
