SHELL := /bin/bash

PACKAGE_NAME="github.com/pengsrc/go-shared"

DIRS_TO_CHECK=$(shell ls -d */ | grep -v "vendor")
PKGS_TO_CHECK=$(shell go list ./... | grep -vE "/vendor")

ifneq (${PKG},)
	DIRS_TO_CHECK="./${PKG}"
	PKGS_TO_CHECK="${PACKAGE_NAME}/${PKG}"
endif

.PHONY: help
help:
	@echo "Please use \`make <target>\` where <target> is one of"
	@echo "  check         to vet and lint"
	@echo "  test          to run test"
	@echo "  test-coverage to run test with coverage"

.PHONY: check
check: format vet lint

.PHONY: format
format:
	@gofmt -w .
	@echo "Done"

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
.PHONY: test
test:
	@echo "Run test"
	@go test -v ${PKGS_TO_CHECK}
	@echo "Done"

.PHONY: test-coverage
test-coverage:
	@echo "Run test with coverage"
	@for pkg in ${PKGS_TO_CHECK}; do \
		output="coverage$${pkg#${PACKAGE_NAME}}"; \
		mkdir -p $${output}; \
		go test -v -cover -coverprofile="$${output}/profile.out" $${pkg}; \
		if [[ -e "$${output}/profile.out" ]]; then \
			go tool cover -html="$${output}/profile.out" \
			              -o "$${output}/profile.html"; \
		fi; \
	 done
	@echo "Done"
