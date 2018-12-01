TAGS ?= "sqlite"
GO_BIN ?= go

install: deps
	echo "installing packr v2"
	packr2
	$(GO_BIN) install -v ./packr2

tidy:
ifeq ($(GO111MODULE),on)
	$(GO_BIN) mod tidy
else
	echo skipping go mod tidy
endif

deps:
	$(GO_BIN) get github.com/gobuffalo/release
	$(GO_BIN) get -tags ${TAGS} -t ./...
	$(GO_BIN) install -v ./packr2
	make tidy

build: deps
	packr2
	$(GO_BIN) build -v ./packr2
	make tidy

test:
	packr2
	$(GO_BIN) test -tags ${TAGS} ./...
	make tidy

lint:
	gometalinter --vendor ./... --deadline=1m --skip=internal

update:
	$(GO_BIN) get -u -tags ${TAGS} ./...
	make tidy
	make install
	make test
	make tidy

release-test:
	$(GO_BIN) test -tags ${TAGS} -race ./...

release:
	release -y -f version.go
	make tidy
