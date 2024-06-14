SHELL = bash
# Branch we are working on
BRANCH := $(or $(BUILD_SOURCEBRANCHNAME),$(lastword $(subst /, ,$(GITHUB_REF))),$(shell git rev-parse --abbrev-ref HEAD))
# Tag of the current commit, if any.  If this is not "" then we are building a release
RELEASE_TAG := $(shell git tag -l --points-at HEAD)
# Version of last release (may not be on this branch)
VERSION := $(shell cat VERSION)
# Last tag on this branch
LAST_TAG := $(shell git describe --tags --abbrev=0)
# Next version
NEXT_VERSION := $(shell echo $(VERSION) | awk -F. -v OFS=. '{print $$1,$$2+1,0}')
NEXT_PATCH_VERSION := $(shell echo $(VERSION) | awk -F. -v OFS=. '{print $$1,$$2,$$3+1}')
# If we are working on a release, override branch to master
ifdef RELEASE_TAG
	BRANCH := master
	LAST_TAG := $(shell git describe --abbrev=0 --tags $(VERSION)^)
endif
TAG_BRANCH := .$(BRANCH)
BRANCH_PATH := branch/$(BRANCH)/
# If building HEAD or master then unset TAG_BRANCH and BRANCH_PATH
ifeq ($(subst HEAD,,$(subst master,,$(BRANCH))),)
	TAG_BRANCH :=
	BRANCH_PATH :=
endif
# Make version suffix -beta.NNNN.CCCCCCCC (N=Commit number, C=Commit)
VERSION_SUFFIX := -beta.$(shell git rev-list --count HEAD).$(shell git show --no-patch --no-notes --pretty='%h' HEAD)
# TAG is current version + commit number + commit + branch
TAG := $(VERSION)$(VERSION_SUFFIX)$(TAG_BRANCH)
ifdef RELEASE_TAG
	TAG := $(RELEASE_TAG)
endif
GO_VERSION := $(shell go version)
GO_OS := $(shell go env GOOS)
ifdef BETA_SUBDIR
	BETA_SUBDIR := /$(BETA_SUBDIR)
endif
BETA_PATH := $(BRANCH_PATH)$(TAG)$(BETA_SUBDIR)
BETA_URL := https://beta.rclone.org/$(BETA_PATH)/
BETA_UPLOAD_ROOT := beta.rclone.org:
BETA_UPLOAD := $(BETA_UPLOAD_ROOT)/$(BETA_PATH)
# Pass in GOTAGS=xyz on the make command line to set build tags
ifdef GOTAGS
BUILDTAGS=-tags "$(GOTAGS)"
LINTTAGS=--build-tags "$(GOTAGS)"
endif
LDFLAGS=--ldflags "-s -X github.com/rclone/rclone/fs.Version=$(TAG)"

.PHONY: rclone test_all vars version

rclone:
ifeq ($(GO_OS),windows)
	go run bin/resource_windows.go -version $(TAG) -syso resource_windows_`go env GOARCH`.syso
endif
	go build -v $(LDFLAGS) $(BUILDTAGS) $(BUILD_ARGS)
ifeq ($(GO_OS),windows)
	rm resource_windows_`go env GOARCH`.syso
endif
	mkdir -p `go env GOPATH`/bin/
	cp -av rclone`go env GOEXE` `go env GOPATH`/bin/rclone`go env GOEXE`.new
	mv -v `go env GOPATH`/bin/rclone`go env GOEXE`.new `go env GOPATH`/bin/rclone`go env GOEXE`

test_all:
	go install $(LDFLAGS) $(BUILDTAGS) $(BUILD_ARGS) github.com/rclone/rclone/fstest/test_all

vars:
	@echo SHELL="'$(SHELL)'"
	@echo BRANCH="'$(BRANCH)'"
	@echo TAG="'$(TAG)'"
	@echo VERSION="'$(VERSION)'"
	@echo GO_VERSION="'$(GO_VERSION)'"
	@echo BETA_URL="'$(BETA_URL)'"

btest:
	@echo "[$(TAG)]($(BETA_URL)) on branch [$(BRANCH)](https://github.com/rclone/rclone/tree/$(BRANCH)) (uploaded in 15-30 mins)" | xclip -r -sel clip
	@echo "Copied markdown of beta release to clip board"

btesth:
	@echo "<a href="$(BETA_URL)">$(TAG)</a> on branch <a href="https://github.com/rclone/rclone/tree/$(BRANCH)">$(BRANCH)</a> (uploaded in 15-30 mins)" | xclip -r -sel clip -t text/html
	@echo "Copied beta release in HTML to clip board"

version:
	@echo '$(TAG)'

# Full suite of integration tests
test:	rclone test_all
	-test_all 2>&1 | tee test_all.log
	@echo "Written logs in test_all.log"

# Quick test
quicktest:
	RCLONE_CONFIG="/notfound" go test $(LDFLAGS) $(BUILDTAGS) ./...

racequicktest:
	RCLONE_CONFIG="/notfound" go test $(LDFLAGS) $(BUILDTAGS) -cpu=2 -race ./...

compiletest:
	RCLONE_CONFIG="/notfound" go test $(LDFLAGS) $(BUILDTAGS) -run XXX ./...

# Do source code quality checks
check:	rclone
	@echo "-- START CODE QUALITY REPORT -------------------------------"
	@golangci-lint run $(LINTTAGS) ./...
	@echo "-- END CODE QUALITY REPORT ---------------------------------"

# Get the build dependencies
build_dep:
	go run bin/get-github-release.go -use-api -extract golangci-lint golangci/golangci-lint 'golangci-lint-.*\.tar\.gz'

# Get the release dependencies we only install on linux
release_dep_linux:
	go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest

# Update dependencies
showupdates:
	@echo "*** Direct dependencies that could be updated ***"
	@GO111MODULE=on go list -u -f '{{if (and (not (or .Main .Indirect)) .Update)}}{{.Path}}: {{.Version}} -> {{.Update.Version}}{{end}}' -m all 2> /dev/null

# Update direct dependencies only
updatedirect:
	GO111MODULE=on go get -d $$(go list -m -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' all)
	GO111MODULE=on go mod tidy

# Update direct and indirect dependencies and test dependencies
update:
	GO111MODULE=on go get -d -u -t ./...
	GO111MODULE=on go mod tidy

# Tidy the module dependencies
tidy:
	GO111MODULE=on go mod tidy

doc:	rclone.1 MANUAL.html MANUAL.txt rcdocs commanddocs

rclone.1:	MANUAL.md
	pandoc -s --from markdown-smart --to man MANUAL.md -o rclone.1

MANUAL.md:	bin/make_manual.py docs/content/*.md commanddocs backenddocs rcdocs
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
	install ${GOPATH}/bin/rclone ${DESTDIR}/usr/bin

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
	rclone -v sync docs/public www.rclone.org:

upload_test_website:	website
	rclone -P sync docs/public test-rclone-org:

validate_website: website
	find docs/public -type f -name "*.html" | xargs tidy --mute-id yes -errors --gnu-emacs yes --drop-empty-elements no --warn-proprietary-attributes no --mute MISMATCHED_ATTRIBUTE_WARN

tarball:
	git archive -9 --format=tar.gz --prefix=rclone-$(TAG)/ -o build/rclone-$(TAG).tar.gz $(TAG)

vendorball:
	go mod vendor
	tar -zcf build/rclone-$(TAG)-vendor.tar.gz vendor
	rm -rf vendor

sign_upload:
	cd build && md5sum rclone-v* | gpg --clearsign > MD5SUMS
	cd build && sha1sum rclone-v* | gpg --clearsign > SHA1SUMS
	cd build && sha256sum rclone-v* | gpg --clearsign > SHA256SUMS

check_sign:
	cd build && gpg --verify MD5SUMS && gpg --decrypt MD5SUMS | md5sum -c
	cd build && gpg --verify SHA1SUMS && gpg --decrypt SHA1SUMS | sha1sum -c
	cd build && gpg --verify SHA256SUMS && gpg --decrypt SHA256SUMS | sha256sum -c

upload:
	rclone -P copy build/ downloads.rclone.org:/$(TAG)
	rclone lsf build --files-only --include '*.{zip,deb,rpm}' --include version.txt | xargs -i bash -c 'i={}; j="$$i"; [[ $$i =~ (.*)(-v[0-9\.]+-)(.*) ]] && j=$${BASH_REMATCH[1]}-current-$${BASH_REMATCH[3]}; rclone copyto -v "downloads.rclone.org:/$(TAG)/$$i" "downloads.rclone.org:/$$j"'

upload_github:
	./bin/upload-github $(TAG)

cross:	doc
	go run bin/cross-compile.go -release current $(BUILD_FLAGS) $(BUILDTAGS) $(BUILD_ARGS) $(TAG)

beta:
	go run bin/cross-compile.go $(BUILD_FLAGS) $(BUILDTAGS) $(BUILD_ARGS) $(TAG)
	rclone -v copy build/ pub.rclone.org:/$(TAG)
	@echo Beta release ready at https://pub.rclone.org/$(TAG)/

log_since_last_release:
	git log $(LAST_TAG)..

compile_all:
	go run bin/cross-compile.go -compile-only $(BUILD_FLAGS) $(BUILDTAGS) $(BUILD_ARGS) $(TAG)

ci_upload:
	sudo chown -R $$USER build
	find build -type l -delete
	gzip -r9v build
	./rclone --no-check-dest --config bin/ci.rclone.conf -v copy build/ $(BETA_UPLOAD)/testbuilds
ifeq ($(or $(BRANCH_PATH),$(RELEASE_TAG)),)
	./rclone --no-check-dest --config bin/ci.rclone.conf -v copy build/ $(BETA_UPLOAD_ROOT)/test/testbuilds-latest
endif
	@echo Beta release ready at $(BETA_URL)/testbuilds

ci_beta:
	git log $(LAST_TAG).. > /tmp/git-log.txt
	go run bin/cross-compile.go -release beta-latest -git-log /tmp/git-log.txt $(BUILD_FLAGS) $(BUILDTAGS) $(BUILD_ARGS) $(TAG)
	rclone --no-check-dest --config bin/ci.rclone.conf -v copy --exclude '*beta-latest*' build/ $(BETA_UPLOAD)
ifeq ($(or $(BRANCH_PATH),$(RELEASE_TAG)),)
	rclone --no-check-dest --config bin/ci.rclone.conf -v copy --include '*beta-latest*' --include version.txt build/ $(BETA_UPLOAD_ROOT)$(BETA_SUBDIR)
endif
	@echo Beta release ready at $(BETA_URL)

# Fetch the binary builds from GitHub actions
fetch_binaries:
	rclone -P sync --exclude "/testbuilds/**" --delete-excluded $(BETA_UPLOAD) build/

serve:	website
	cd docs && hugo server --logLevel info -w --disableFastRender

tag:	retag doc
	bin/make_changelog.py $(LAST_TAG) $(VERSION) > docs/content/changelog.md.new
	mv docs/content/changelog.md.new docs/content/changelog.md
	@echo "Edit the new changelog in docs/content/changelog.md"
	@echo "Then commit all the changes"
	@echo git commit -m \"Version $(VERSION)\" -a -v
	@echo "And finally run make retag before make cross, etc."

retag:
	@echo "Version is $(VERSION)"
	git tag -f -s -m "Version $(VERSION)" $(VERSION)

startdev:
	@echo "Version is $(VERSION)"
	@echo "Next version is $(NEXT_VERSION)"
	echo -e "package fs\n\n// VersionTag of rclone\nvar VersionTag = \"$(NEXT_VERSION)\"\n" | gofmt > fs/versiontag.go
	echo -n "$(NEXT_VERSION)" > docs/layouts/partials/version.html
	echo "$(NEXT_VERSION)" > VERSION
	git commit -m "Start $(NEXT_VERSION)-DEV development" fs/versiontag.go VERSION docs/layouts/partials/version.html

startstable:
	@echo "Version is $(VERSION)"
	@echo "Next stable version is $(NEXT_PATCH_VERSION)"
	echo -e "package fs\n\n// VersionTag of rclone\nvar VersionTag = \"$(NEXT_PATCH_VERSION)\"\n" | gofmt > fs/versiontag.go
	echo -n "$(NEXT_PATCH_VERSION)" > docs/layouts/partials/version.html
	echo "$(NEXT_PATCH_VERSION)" > VERSION
	git commit -m "Start $(NEXT_PATCH_VERSION)-DEV development" fs/versiontag.go VERSION docs/layouts/partials/version.html

winzip:
	zip -9 rclone-$(TAG).zip rclone.exe

# docker volume plugin
PLUGIN_USER ?= rclone
PLUGIN_TAG ?= latest
PLUGIN_BASE_TAG ?= latest
PLUGIN_ARCH ?= amd64
PLUGIN_IMAGE := $(PLUGIN_USER)/docker-volume-rclone:$(PLUGIN_TAG)
PLUGIN_BASE := $(PLUGIN_USER)/rclone:$(PLUGIN_BASE_TAG)
PLUGIN_BUILD_DIR := ./build/docker-plugin
PLUGIN_CONTRIB_DIR := ./contrib/docker-plugin/managed

docker-plugin-create:
	docker buildx inspect |grep -q /${PLUGIN_ARCH} || \
	docker run --rm --privileged tonistiigi/binfmt --install all
	rm -rf ${PLUGIN_BUILD_DIR}
	docker buildx build \
		--no-cache --pull \
		--build-arg BASE_IMAGE=${PLUGIN_BASE} \
		--platform linux/${PLUGIN_ARCH} \
		--output ${PLUGIN_BUILD_DIR}/rootfs \
		${PLUGIN_CONTRIB_DIR}
	cp ${PLUGIN_CONTRIB_DIR}/config.json ${PLUGIN_BUILD_DIR}
	docker plugin rm --force ${PLUGIN_IMAGE} 2>/dev/null || true
	docker plugin create ${PLUGIN_IMAGE} ${PLUGIN_BUILD_DIR}

docker-plugin-push:
	docker plugin push ${PLUGIN_IMAGE}
	docker plugin rm ${PLUGIN_IMAGE}

docker-plugin: docker-plugin-create docker-plugin-push
