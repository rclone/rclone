SHELL = /bin/bash
TAG := $(shell echo `git describe --tags`-`git rev-parse --abbrev-ref HEAD` | sed 's/-\([0-9]\)-/-0\1-/; s/-\(HEAD\|master\)$$//')
LAST_TAG := $(shell git describe --tags --abbrev=0)
NEW_TAG := $(shell echo $(LAST_TAG) | perl -lpe 's/v//; $$_ += 0.01; $$_ = sprintf("v%.2f", $$_)')
GO_VERSION := $(shell go version)
GO_FILES := $(shell go list ./... | grep -v /vendor/ )
GO_LATEST := $(findstring go1.8,$(GO_VERSION))
BETA_URL := https://beta.rclone.org/$(TAG)/

.PHONY: rclone

rclone:
	touch fs/version.go
	go install -v --ldflags "-s -X github.com/ncw/rclone/fs.Version=$(TAG)"
	cp -av `go env GOPATH`/bin/rclone .

vars:
	@echo SHELL="'$(SHELL)'"
	@echo TAG="'$(TAG)'"
	@echo LAST_TAG="'$(LAST_TAG)'"
	@echo NEW_TAG="'$(NEW_TAG)'"
	@echo GO_VERSION="'$(GO_VERSION)'"
	@echo GO_LATEST="'$(GO_LATEST)'"
	@echo BETA_URL="'$(BETA_URL)'"

# Full suite of integration tests
test:	rclone
	go test $(GO_FILES)
	cd fs && go run test_all.go

# Quick test
quicktest:
	RCLONE_CONFIG="/notfound" go test $(GO_FILES)
	RCLONE_CONFIG="/notfound" go test -cpu=2 -race $(GO_FILES)

# Do source code quality checks
check:	rclone
ifdef GO_LATEST
	go tool vet -printfuncs Debugf,Infof,Logf,Errorf . 2>&1 | grep -E -v vendor/ ; test $$? -eq 1
	errcheck $(GO_FILES)
	find . -name \*.go | grep -v /vendor/ | xargs goimports -d | grep . ; test $$? -eq 1
	go list ./... | grep -v /vendor/ | xargs -i golint {} | grep -E -v '(StorageUrl|CdnUrl)' ; test $$? -eq 1
else
	@echo Skipping tests as not on Go stable
endif

# Get the build dependencies
build_dep:
ifdef GO_LATEST
	go get -u github.com/kisielk/errcheck
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/inconshreveable/mousetrap
	go get -u github.com/tools/godep
endif

# Update dependencies
update:
	dep ensure -update -v

doc:	rclone.1 MANUAL.html MANUAL.txt

rclone.1:	MANUAL.md
	pandoc -s --from markdown --to man MANUAL.md -o rclone.1

MANUAL.md:	bin/make_manual.py docs/content/*.md commanddocs
	./bin/make_manual.py

MANUAL.html:	MANUAL.md
	pandoc -s --from markdown --to html MANUAL.md -o MANUAL.html

MANUAL.txt:	MANUAL.md
	pandoc -s --from markdown --to plain MANUAL.md -o MANUAL.txt

commanddocs: rclone
	rclone gendocs docs/content/commands/

install: rclone
	install -d ${DESTDIR}/usr/bin
	install -t ${DESTDIR}/usr/bin ${GOPATH}/bin/rclone

clean:
	go clean ./...
	find . -name \*~ | xargs -r rm -f
	rm -rf build docs/public
	rm -f rclone rclonetest/rclonetest

website:
	cd docs && hugo

upload_website:	website
	rclone -v sync docs/public memstore:www-rclone-org

upload:
	rclone -v copy build/ memstore:downloads-rclone-org

upload_github:
	./bin/upload-github $(TAG)

cross:	doc
	go run bin/cross-compile.go -release current $(TAG)

beta:
	go run bin/cross-compile.go $(TAG)β
	rclone -v copy build/ memstore:pub-rclone-org/$(TAG)β
	@echo Beta release ready at https://pub.rclone.org/$(TAG)%CE%B2/

travis_beta:
	git log $(LAST_TAG).. > /tmp/git-log.txt
	go run bin/cross-compile.go -release beta-latest -git-log /tmp/git-log.txt $(TAG)β
	rclone --config bin/travis.rclone.conf -v copy --exclude '*beta-latest*' build/ memstore:beta-rclone-org/$(TAG)
	rclone --config bin/travis.rclone.conf -v copy --include '*beta-latest*' build/ memstore:beta-rclone-org
	@echo Beta release ready at $(BETA_URL)

serve:	website
	cd docs && hugo server -v -w

tag:	doc
	@echo "Old tag is $(LAST_TAG)"
	@echo "New tag is $(NEW_TAG)"
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(NEW_TAG)\"\n" | gofmt > fs/version.go
	perl -lpe 's/VERSION/${NEW_TAG}/g; s/DATE/'`date -I`'/g;' docs/content/downloads.md.in > docs/content/downloads.md
	git tag $(NEW_TAG)
	@echo "Edit the new changelog in docs/content/changelog.md"
	@echo "  * $(NEW_TAG) -" `date -I` >> docs/content/changelog.md
	@git log $(LAST_TAG)..$(NEW_TAG) --oneline >> docs/content/changelog.md
	@echo "Then commit the changes"
	@echo git commit -m \"Version $(NEW_TAG)\" -a -v
	@echo "And finally run make retag before make cross etc"

retag:
	git tag -f $(LAST_TAG)

startdev:
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(LAST_TAG)-DEV\"\n" | gofmt > fs/version.go
	git commit -m "Start $(LAST_TAG)-DEV development" fs/version.go

gen_tests:
	cd fstest/fstests && go generate

winzip:
	zip -9 rclone-$(TAG).zip rclone.exe

