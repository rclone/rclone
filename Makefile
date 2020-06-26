SHELL = bash
# Branch we are working on
BRANCH := $(or $(BUILD_SOURCEBRANCHNAME),$(lastword $(subst /, ,$(GITHUB_REF))),$(shell git rev-parse --abbrev-ref HEAD))
# Tag of the current commit, if any.  If this is not "" then we are building a release
RELEASE_TAG := $(shell git tag -l --points-at HEAD)
# Version of last release (may not be on this branch)
VERSION := $(shell cat VERSION)
# Last tag on this branch
LAST_TAG := $(shell git describe --tags --abbrev=0)
# If we are working on a release, override branch to master
ifdef RELEASE_TAG
	BRANCH := master
endif
TAG_BRANCH := -$(BRANCH)
BRANCH_PATH := branch/
# If building HEAD or master then unset TAG_BRANCH and BRANCH_PATH
ifeq ($(subst HEAD,,$(subst master,,$(BRANCH))),)
	TAG_BRANCH :=
	BRANCH_PATH :=
endif
# Make version suffix -DDD-gCCCCCCCC (D=commits since last relase, C=Commit) or blank
VERSION_SUFFIX := $(shell git describe --abbrev=8 --tags | perl -lpe 's/^v\d+\.\d+\.\d+//; s/^-(\d+)/"-".sprintf("%03d",$$1)/e;')
# TAG is current version + number of commits since last release + branch
TAG := $(VERSION)$(VERSION_SUFFIX)$(TAG_BRANCH)
NEXT_VERSION := $(shell echo $(VERSION) | perl -lpe 's/v//; $$_ += 0.01; $$_ = sprintf("v%.2f.0", $$_)')
ifndef RELEASE_TAG
	TAG := $(TAG)-beta
endif
GO_VERSION := $(shell go version)
GO_FILES := $(shell go list ./... | grep -v /vendor/ )
ifdef BETA_SUBDIR
	BETA_SUBDIR := /$(BETA_SUBDIR)
endif
BETA_PATH := $(BRANCH_PATH)$(TAG)$(BETA_SUBDIR)
BETA_URL := https://beta.rclone.org/$(BETA_PATH)/
BETA_UPLOAD_ROOT := memstore:beta-rclone-org
BETA_UPLOAD := $(BETA_UPLOAD_ROOT)/$(BETA_PATH)
# Pass in GOTAGS=xyz on the make command line to set build tags
ifdef GOTAGS
BUILDTAGS=-tags "$(GOTAGS)"
LINTTAGS=--build-tags "$(GOTAGS)"
endif

.PHONY: rclone test_all vars version

rclone:
	go build -v --ldflags "-s -X github.com/rclone/rclone/fs.Version=$(TAG)" $(BUILDTAGS)
	mkdir -p `go env GOPATH`/bin/
	cp -av rclone`go env GOEXE` `go env GOPATH`/bin/rclone`go env GOEXE`.new
	mv -v `go env GOPATH`/bin/rclone`go env GOEXE`.new `go env GOPATH`/bin/rclone`go env GOEXE`

test_all:
	go install --ldflags "-s -X github.com/rclone/rclone/fs.Version=$(TAG)" $(BUILDTAGS) github.com/rclone/rclone/fstest/test_all

vars:
	@echo SHELL="'$(SHELL)'"
	@echo BRANCH="'$(BRANCH)'"
	@echo TAG="'$(TAG)'"
	@echo VERSION="'$(VERSION)'"
	@echo NEXT_VERSION="'$(NEXT_VERSION)'"
	@echo GO_VERSION="'$(GO_VERSION)'"
	@echo BETA_URL="'$(BETA_URL)'"

btest:
	@echo "[$(TAG)]($(BETA_URL)) on branch [$(BRANCH)](https://github.com/rclone/rclone/tree/$(BRANCH)) (uploaded in 15-30 mins)" | xclip -r -sel clip
	@echo "Copied markdown of beta release to clip board"

version:
	@echo '$(TAG)'

# Full suite of integration tests
test:	rclone test_all
	-test_all 2>&1 | tee test_all.log
	@echo "Written logs in test_all.log"

# Quick test
quicktest:
	RCLONE_CONFIG="/notfound" go test $(BUILDTAGS) $(GO_FILES)

racequicktest:
	RCLONE_CONFIG="/notfound" go test $(BUILDTAGS) -cpu=2 -race $(GO_FILES)

# Do source code quality checks
check:	rclone
	@echo "-- START CODE QUALITY REPORT -------------------------------"
	@golangci-lint run $(LINTTAGS) ./...
	@echo "-- END CODE QUALITY REPORT ---------------------------------"

# Get the build dependencies
build_dep:
	go run bin/get-github-release.go -extract golangci-lint golangci/golangci-lint 'golangci-lint-.*\.tar\.gz'

# Get the release dependencies we only install on linux
release_dep_linux:
	go run bin/get-github-release.go -extract nfpm goreleaser/nfpm 'nfpm_.*_Linux_x86_64.tar.gz'
	go run bin/get-github-release.go -extract github-release aktau/github-release 'linux-amd64-github-release.tar.bz2'

# Get the release dependencies we only install on Windows
release_dep_windows:
	GO111MODULE=off GOOS="" GOARCH="" go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo

# Update dependencies
update:
	GO111MODULE=on go get -u ./...
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor

# Tidy the module dependencies
tidy:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor

doc:	rclone.1 MANUAL.html MANUAL.txt rcdocs commanddocs

rclone.1:	MANUAL.md
	pandoc -s --from markdown-smart --to man MANUAL.md -o rclone.1

MANUAL.md:	bin/make_manual.py docs/content/*.md commanddocs backenddocs
	./bin/make_manual.py

MANUAL.html:	MANUAL.md
	pandoc -s --from markdown-smart --to html MANUAL.md -o MANUAL.html

MANUAL.txt:	MANUAL.md
	pandoc -s --from markdown-smart --to plain MANUAL.md -o MANUAL.txt

commanddocs: rclone
	XDG_CACHE_HOME="" XDG_CONFIG_HOME="" HOME="\$$HOME" USER="\$$USER" rclone gendocs docs/content/

backenddocs: rclone bin/make_backend_docs.py
	XDG_CACHE_HOME="" XDG_CONFIG_HOME="" HOME="\$$HOME" USER="\$$USER" ./bin/make_backend_docs.py

rcdocs: rclone
	bin/make_rc_docs.sh

install: rclone
	install -d ${DESTDIR}/usr/bin
	install -t ${DESTDIR}/usr/bin ${GOPATH}/bin/rclone

clean:
	go clean ./...
	find . -name \*~ | xargs -r rm -f
	rm -rf build docs/public
	rm -f rclone fs/operations/operations.test fs/sync/sync.test fs/test_all.log test.log

website:
	rm -rf docs/public
	cd docs && hugo
	@if grep -R "raw HTML omitted" docs/public ; then echo "ERROR: found unescaped HTML - fix the markdown source" ; fi

upload_website:	website
	rclone -v sync docs/public memstore:www-rclone-org

upload_test_website:	website
	rclone -P sync docs/public test-rclone-org:

validate_website: website
	find docs/public -type f -name "*.html" | xargs tidy --mute-id yes -errors --gnu-emacs yes --drop-empty-elements no --warn-proprietary-attributes no --mute MISMATCHED_ATTRIBUTE_WARN

tarball:
	git archive -9 --format=tar.gz --prefix=rclone-$(TAG)/ -o build/rclone-$(TAG).tar.gz $(TAG)

sign_upload:
	cd build && md5sum rclone-v* | gpg --clearsign > MD5SUMS
	cd build && sha1sum rclone-v* | gpg --clearsign > SHA1SUMS
	cd build && sha256sum rclone-v* | gpg --clearsign > SHA256SUMS

check_sign:
	cd build && gpg --verify MD5SUMS && gpg --decrypt MD5SUMS | md5sum -c
	cd build && gpg --verify SHA1SUMS && gpg --decrypt SHA1SUMS | sha1sum -c
	cd build && gpg --verify SHA256SUMS && gpg --decrypt SHA256SUMS | sha256sum -c

upload:
	rclone -P copy build/ memstore:downloads-rclone-org/$(TAG)
	rclone lsf build --files-only --include '*.{zip,deb,rpm}' --include version.txt | xargs -i bash -c 'i={}; j="$$i"; [[ $$i =~ (.*)(-v[0-9\.]+-)(.*) ]] && j=$${BASH_REMATCH[1]}-current-$${BASH_REMATCH[3]}; rclone copyto -v "memstore:downloads-rclone-org/$(TAG)/$$i" "memstore:downloads-rclone-org/$$j"'

upload_github:
	./bin/upload-github $(TAG)

cross:	doc
	go run bin/cross-compile.go -release current $(BUILDTAGS) $(TAG)

beta:
	go run bin/cross-compile.go $(BUILDTAGS) $(TAG)
	rclone -v copy build/ memstore:pub-rclone-org/$(TAG)
	@echo Beta release ready at https://pub.rclone.org/$(TAG)/

log_since_last_release:
	git log $(LAST_TAG)..

compile_all:
	go run bin/cross-compile.go -compile-only $(BUILDTAGS) $(TAG)

ci_upload:
	sudo chown -R $$USER build
	find build -type l -delete
	gzip -r9v build
	./rclone --config bin/travis.rclone.conf -v copy build/ $(BETA_UPLOAD)/testbuilds
ifndef BRANCH_PATH
	./rclone --config bin/travis.rclone.conf -v copy build/ $(BETA_UPLOAD_ROOT)/test/testbuilds-latest
endif
	@echo Beta release ready at $(BETA_URL)/testbuilds

ci_beta:
	git log $(LAST_TAG).. > /tmp/git-log.txt
	go run bin/cross-compile.go -release beta-latest -git-log /tmp/git-log.txt $(BUILD_FLAGS) $(BUILDTAGS) $(TAG)
	rclone --config bin/travis.rclone.conf -v copy --exclude '*beta-latest*' build/ $(BETA_UPLOAD)
ifndef BRANCH_PATH
	rclone --config bin/travis.rclone.conf -v copy --include '*beta-latest*' --include version.txt build/ $(BETA_UPLOAD_ROOT)$(BETA_SUBDIR)
endif
	@echo Beta release ready at $(BETA_URL)

# Fetch the binary builds from GitHub actions
fetch_binaries:
	rclone -P sync --exclude "/testbuilds/**" --delete-excluded $(BETA_UPLOAD) build/

serve:	website
	cd docs && hugo server -v -w --disableFastRender

tag:	doc
	@echo "Old tag is $(VERSION)"
	@echo "New tag is $(NEXT_VERSION)"
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(NEXT_VERSION)\"\n" | gofmt > fs/version.go
	echo -n "$(NEXT_VERSION)" > docs/layouts/partials/version.html
	echo "$(NEXT_VERSION)" > VERSION
	git tag -s -m "Version $(NEXT_VERSION)" $(NEXT_VERSION)
	bin/make_changelog.py $(LAST_TAG) $(NEXT_VERSION) > docs/content/changelog.md.new
	mv docs/content/changelog.md.new docs/content/changelog.md
	@echo "Edit the new changelog in docs/content/changelog.md"
	@echo "Then commit all the changes"
	@echo git commit -m \"Version $(NEXT_VERSION)\" -a -v
	@echo "And finally run make retag before make cross etc"

retag:
	git tag -f -s -m "Version $(VERSION)" $(VERSION)

startdev:
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(VERSION)-DEV\"\n" | gofmt > fs/version.go
	git commit -m "Start $(VERSION)-DEV development" fs/version.go

winzip:
	zip -9 rclone-$(TAG).zip rclone.exe
