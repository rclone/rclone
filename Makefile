SHELL = /bin/bash
TAG := $(shell git describe --tags)
LAST_TAG := $(shell git describe --tags --abbrev=0)
NEW_TAG := $(shell echo $(LAST_TAG) | perl -lpe 's/v//; $$_ += 0.01; $$_ = sprintf("v%.2f", $$_)')

rclone:
	@go version
	go install -v ./...

test:	rclone
	go test ./...
	cd fs && ./test_all.sh

check:	rclone
	go vet ./...
	errcheck ./...
	goimports -d . | grep . ; test $$? -eq 1
	golint ./... | grep -E -v '(StorageUrl|CdnUrl)' ; test $$? -eq 1

doc:	rclone.1 MANUAL.html MANUAL.txt

rclone.1:	MANUAL.md
	pandoc -s --from markdown --to man MANUAL.md -o rclone.1

MANUAL.md:	make_manual.py docs/content/*.md
	./make_manual.py

MANUAL.html:	MANUAL.md
	pandoc -s --from markdown --to html MANUAL.md -o MANUAL.html

MANUAL.txt:	MANUAL.md
	pandoc -s --from markdown --to plain MANUAL.md -o MANUAL.txt

install: rclone
	install -d ${DESTDIR}/usr/bin
	install -t ${DESTDIR}/usr/bin ${GOPATH}/bin/rclone

clean:
	go clean ./...
	find . -name \*~ | xargs -r rm -f
	rm -rf build docs/public
	rm -f rclone rclonetest/rclonetest rclone.1 MANUAL.md MANUAL.html MANUAL.txt

website:
	cd docs && hugo

upload_website:	website
	rclone -v sync docs/public memstore:www-rclone-org

upload:
	rclone -v copy build/ memstore:downloads-rclone-org

upload_github:
	./upload_github $(TAG)

cross:	doc
	./cross-compile $(TAG)

beta:	doc
	./cross-compile $(TAG)β
	rm build/*-current-*
	rclone -v copy build/ memstore:pub-rclone-org/$(TAG)β
	@echo Beta release ready at http://pub.rclone.org/$(TAG)%CE%B2/

serve:
	cd docs && hugo server -v -w

tag:
	@echo "Old tag is $(LAST_TAG)"
	@echo "New tag is $(NEW_TAG)"
	echo -e "package fs\n\n// Version of rclone\nconst Version = \"$(NEW_TAG)\"\n" | gofmt > fs/version.go
	perl -lpe 's/VERSION/${NEW_TAG}/g; s/DATE/'`date -I`'/g;' docs/content/downloads.md.in > docs/content/downloads.md
	git tag $(NEW_TAG)
	@echo "Add this to changelog in docs/content/changelog.md"
	@echo "  * $(NEW_TAG) -" `date -I`
	@git log $(LAST_TAG)..$(NEW_TAG) --oneline
	@echo "Then commit the changes"
	@echo git commit -m \"Version $(NEW_TAG)\" -a -v
	@echo "And finally run make retag before make cross etc"

retag:
	git tag -f $(LAST_TAG)

gen_tests:
	cd fstest/fstests && go generate
