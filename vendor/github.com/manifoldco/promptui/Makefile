HAS_GO_MOD=$(shell go help mod; echo $$?)
LINTERS=$(shell grep "// lint" tools.go | awk '{gsub(/\"/, "", $$1); print $$1}' | awk -F / '{print $$NF}') \
	gofmt \
	vet

ci: $(LINTERS) test

.PHONY: ci

#################################################
# Bootstrapping for base golang package and tool deps
#################################################

CMD_PKGS=$(shell grep '	"' tools.go | awk -F '"' '{print $$2}')

define VENDOR_BIN_TMPL
vendor/bin/$(notdir $(1)): vendor/$(1) | vendor
	go build -a -o $$@ ./vendor/$(1)
VENDOR_BINS += vendor/bin/$(notdir $(1))
vendor/$(1): vendor
endef

$(foreach cmd_pkg,$(CMD_PKGS),$(eval $(call VENDOR_BIN_TMPL,$(cmd_pkg))))

$(patsubst %,%-bin,$(filter-out gofmt vet,$(LINTERS))): %-bin: vendor/bin/%
gofmt-bin vet-bin:

ifeq ($(HAS_GO_MOD),0)
bootstrap:

vendor: go.sum
	GO111MODULE=on go mod vendor
else
bootstrap:
	which dep || go get -u github.com/golang/dep/cmd/dep

vendor: Gopkg.lock
	dep ensure -vendor-only
endif

mod-update:
	GO111MODULE=on go get -u -m
	GO111MODULE=on go mod tidy
	dep ensure -update

mod-tidy:
	GO111MODULE=on go mod tidy

.PHONY: $(CMD_PKGS)
.PHONY: mod-update mod-tidy

#################################################
# Test and linting
#################################################

test: vendor
	CGO_ENABLED=0 go test $$(go list ./... | grep -v generated)

$(LINTERS): %: vendor/bin/gometalinter %-bin vendor
	PATH=`pwd`/vendor/bin:$$PATH gometalinter --tests --disable-all --vendor \
		--deadline=5m -s data --enable $@ ./...

.PHONY: $(LINTERS) test
.PHONY: cover all-cover.txt
