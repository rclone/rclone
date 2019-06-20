SHELL = bash
BRANCH := $(or $(APPVEYOR_REPO_BRANCH),$(TRAVIS_BRANCH),$(shell git rev-parse --abbrev-ref HEAD))
LAST_TAG := $(shell git describe --tags --abbrev=0)
ifeq ($(BRANCH),$(LAST_TAG))
	BRANCH := master
endif
TAG_BRANCH := -$(BRANCH)
BRANCH_PATH := branch/
ifeq ($(subst HEAD,,$(subst master,,$(BRANCH))),)
	TAG_BRANCH :=
	BRANCH_PATH :=
endif
TAG := $(shell echo $$(git describe --abbrev=8 --tags | sed 's/-\([0-9]\)-/-00\1-/; s/-\([0-9][0-9]\)-/-0\1-/'))$(TAG_BRANCH)
NEW_TAG := $(shell echo $(LAST_TAG) | perl -lpe 's/v//; $$_ += 0.01; $$_ = sprintf("v%.2f.0", $$_)')
ifneq ($(TAG),$(LAST_TAG))
	TAG := $(TAG)-beta
endif
GO_VERSION := $(shell go version)
GO_FILES := $(shell go list ./... | grep -v /vendor/ )
BETA_PATH := $(BRANCH_PATH)$(TAG)
BETA_URL := https://beta.rclone.org/$(BETA_PATH)/
BETA_UPLOAD_ROOT := memstore:beta-rclone-org
BETA_UPLOAD := $(BETA_UPLOAD_ROOT)/$(BETA_PATH)
# Pass in GOTAGS=xyz on the make command line to set build tags
ifdef GOTAGS
BUILDTAGS=-tags "$(GOTAGS)"
LINTTAGS=--build-tags "$(GOTAGS)"
endif

.PHONY: rclone vars version

rclone:
	touch fs/version.go
	go install -v --ldflags "-s -X github.com/ncw/rclone/fs.Version=$(TAG)" $(BUILDTAGS)
	cp -av `go env GOPATH`/bin/rclone .

vars:
	@echo SHELL="'$(SHELL)'"
	@echo BRANCH="'$(BRANCH)'"
	@echo TAG="'$(TAG)'"
	@echo LAST_TAG="'$(LAST_TAG)'"
	@echo NEW_TAG="'$(NEW_TAG)'"
	@echo GO_VERSION="'$(GO_VERSION)'"
	@echo BETA_URL="'$(BETA_URL)'"

version:
	@echo '$(TAG)'

# Full suite of integration tests
test:	rclone
	go install --ldflags "-s -X github.com/ncw/rclone/fs.Version=$(TAG)" $(BUILDTAGS) github.com/ncw/rclone/fstest/test_all
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

# Get the release dependencies
release_dep:
	go get -u github.com/goreleaser/nfpm/...
	go get -u github.com/aktau/github-release

# Update dependencies
update:
	GO111MODULE=on go get -u ./...
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor

doc:	rclone.1 MANUAL.html MANUAL.txt rcdocs commanddocs

rclone.1:	MANUAL.md
	pandoc -s --from markdown --to man MANUAL.md -o rclone.1

MANUAL.md:	bin/make_manual.py docs/content/*.md commanddocs backenddocs
	./bin/make_manual.py

MANUAL.html:	MANUAL.md
	pandoc -s --from markdown --to html MANUAL.md -o MANUAL.html

MANUAL.txt:	MANUAL.md
	pandoc -s --from markdown --to plain MANUAL.md -o MANUAL.txt

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
	cd docs && hugo

upload_website:	website
	rclone -v sync docs/public memstore:www-rclone-org

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
	go run bin/cross-compile.go -parallel 8 -compile-only $(BUILDTAGS) $(TAG)

appveyor_upload:
	rclone --config bin/travis.rclone.conf -v copy --exclude '*beta-latest*' build/ $(BETA_UPLOAD)
ifndef BRANCH_PATH
	rclone --config bin/travis.rclone.conf -v copy --include '*beta-latest*' --include version.txt build/ $(BETA_UPLOAD_ROOT)
endif
	@echo Beta release ready at $(BETA_URL)

circleci_upload:
	./rclone --config bin/travis.rclone.conf -v copy build/ $(BETA_UPLOAD)/testbuilds
ifndef BRANCH_PATH
	./rclone --config bin/travis.rclone.conf -v copy build/ $(BETA_UPLOAD_ROOT)/test/testbuilds-latest
endif
	@echo Beta release ready at $(BETA_URL)/testbuilds

BUILD_FLAGS := -exclude "^(windows|darwin)/"
ifeq ($(TRAVIS_OS_NAME),osx)
	BUILD_FLAGS := -include "^darwin/" -cgo
endif
ifeq ($(TRAVIS_OS_NAME),windows)
# BUILD_FLAGS := -include "^windows/" -cgo
# 386 doesn't build yet
	BUILD_FLAGS := -include "^windows/amd64" -cgo
endif

travis_beta:
ifeq ($(TRAVIS_OS_NAME),linux)
	go run bin/get-github-release.go -extract nfpm goreleaser/nfpm 'nfpm_.*\.tar.gz'
endif
	git log $(LAST_TAG).. > /tmp/git-log.txt
	go run bin/cross-compile.go -release beta-latest -git-log /tmp/git-log.txt $(BUILD_FLAGS) -parallel 8 $(BUILDTAGS) $(TAG)
	rclone --config bin/travis.rclone.conf -v copy --exclude '*beta-latest*' build/ $(BETA_UPLOAD)
ifndef BRANCH_PATH
	rclone --config bin/travis.rclone.conf -v copy --include '*beta-latest*' --include version.txt build/ $(BETA_UPLOAD_ROOT)
endif
	@echo Beta release ready at $(BETA_URL)

# Fetch the binary builds from travis and appveyor
fetch_binaries:
	rclone -P sync --exclude "/testbuilds/**" --delete-excluded $(BETA_UPLOAD) build/

serve:	website
	cd docs && hugo server -v -w

tag:	doc
	@echo "Old tag is $(LAST_TAG)"
	@echo "New tag is $(NEW_TAG)"
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(NEW_TAG)\"\n" | gofmt > fs/version.go
	echo -n "$(NEW_TAG)" > docs/layouts/partials/version.html
	git tag -s -m "Version $(NEW_TAG)" $(NEW_TAG)
	bin/make_changelog.py $(LAST_TAG) $(NEW_TAG) > docs/content/changelog.md.new
	mv docs/content/changelog.md.new docs/content/changelog.md
	@echo "Edit the new changelog in docs/content/changelog.md"
	@echo "Then commit all the changes"
	@echo git commit -m \"Version $(NEW_TAG)\" -a -v
	@echo "And finally run make retag before make cross etc"

retag:
	git tag -f -s -m "Version $(LAST_TAG)" $(LAST_TAG)

startdev:
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(LAST_TAG)-DEV\"\n" | gofmt > fs/version.go
	git commit -m "Start $(LAST_TAG)-DEV development" fs/version.go

winzip:
	zip -9 rclone-$(TAG).zip rclone.exe
