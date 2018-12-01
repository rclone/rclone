TAGS ?= "sqlite"
GO_BIN ?= go

install:
	$(GO_BIN) install -tags ${TAGS} -v .
	make tidy

tidy:
ifeq ($(GO111MODULE),on)
	$(GO_BIN) mod tidy
else
	echo skipping go mod tidy
endif

deps:
	$(GO_BIN) get github.com/gobuffalo/release
	$(GO_BIN) get -tags ${TAGS} -t ./...
	make tidy

build:
	$(GO_BIN) build -v .
	make tidy

test:
	$(GO_BIN) test -tags ${TAGS} ./...
	make tidy

ci-deps:
	$(GO_BIN) get -tags ${TAGS} -t ./...

ci-test:
	$(GO_BIN) test -tags ${TAGS} -race ./...

lint:
	gometalinter --vendor ./... --deadline=1m --skip=internal
	make tidy

update:
	$(GO_BIN) get -u -tags ${TAGS}
	make tidy
	make test
	make install
	make tidy

release-test:
	$(GO_BIN) test -tags ${TAGS} -race ./...
	make tidy

release:
	make tidy
	release -y -f version.go
	make tidy
