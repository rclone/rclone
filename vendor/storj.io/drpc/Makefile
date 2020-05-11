.DEFAULT_GOAL = check

.PHONY: check
check: generate build lint test docs

.PHONY: build
build:
	./scripts/run.sh go build ./...

.PHONY: docs
docs:
	./scripts/docs.sh

.PHONY: download
download:
	./scripts/run.sh go mod download

.PHONY: generate
generate:
	./scripts/run.sh go generate ./...

.PHONY: lint
lint:
	./scripts/run.sh staticcheck ./...
	./scripts/run.sh golangci-lint -j=2 run

.PHONY: tidy
tidy:
	./scripts/run.sh go mod tidy

.PHONY: test
test:
	./scripts/run.sh go test ./... -count=1
