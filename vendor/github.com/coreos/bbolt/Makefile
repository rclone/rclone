BRANCH=`git rev-parse --abbrev-ref HEAD`
COMMIT=`git rev-parse --short HEAD`
GOLDFLAGS="-X main.branch $(BRANCH) -X main.commit $(COMMIT)"

default: build

race:
	@go test -v -race -test.run="TestSimulate_(100op|1000op)"

# go get honnef.co/go/tools/simple
# go get honnef.co/go/tools/unused
fmt:
	gosimple ./...
	unused ./...
	gofmt -l -s -d $(find -name \*.go)


# go get github.com/kisielk/errcheck
errcheck:
	@errcheck -ignorepkg=bytes -ignore=os:Remove github.com/coreos/bbolt

test:
	go test -timeout 20m -v -coverprofile cover.out -covermode atomic
	# Note: gets "program not an importable package" in out of path builds
	go test -v ./cmd/bolt

.PHONY: race fmt errcheck test
