SHELL = bash

default: test

.PHONY: test
test:
	@echo "--> Running Tests ..."
	@go test -v -race ./...

vet:
	@echo "--> Vet Go sources ..."
	@go vet ./...
