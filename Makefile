TAG := $(shell git describe --tags)
LAST_TAG := $(shell git describe --tags --abbrev=0)
NEW_TAG := $(shell echo $(LAST_TAG) | perl -lpe 's/v//; $$_ += 0.01; $$_ = sprintf("v%.2f", $$_)')

rclone:
	@go version
	go install -v ./...

test:	rclone
	go test ./...
	rclonetest/test.sh

doc:	rclone.1 README.html README.txt

rclone.1:	README.md
	pandoc -s --from markdown --to man README.md -o rclone.1

README.html:	README.md
	pandoc -s --from markdown_github --to html README.md -o README.html

README.txt:	README.md
	pandoc -s --from markdown_github --to plain README.md -o README.txt

install: rclone
	install -d ${DESTDIR}/usr/bin
	install -t ${DESTDIR}/usr/bin ${GOPATH}/bin/rclone

clean:
	go clean ./...
	find . -name \*~ | xargs -r rm -f
	rm -rf build docs/public
	rm -f rclone rclonetest/rclonetest rclone.1 README.html README.txt

website:
	cd docs && hugo

upload_website:	website
	rclone -v sync docs/public memstore:www-rclone-org

upload:
	rclone -v copy build/ memstore:downloads-rclone-org

cross:	doc
	./cross-compile $(TAG)

serve:
	cd docs && hugo server -v -w

tag:
	@echo "Old tag is $(LAST_TAG)"
	@echo "New tag is $(NEW_TAG)"
	echo -e "package fs\n const Version = \"$(NEW_TAG)\"\n" | gofmt > fs/version.go
	perl -lpe 's/VERSION/${NEW_TAG}/g; s/DATE/'`date -I`'/g;' docs/content/downloads.md.in > docs/content/downloads.md
	git tag $(NEW_TAG)
	@echo "Add this to changelog in README.md"
	@echo "  * $(NEW_TAG) -" `date -I`
	@git log $(LAST_TAG)..$(NEW_TAG) --oneline
	@echo "Then commit the changes"
	@echo git commit -m "Version $(NEW_TAG)" -a -v
	@echo "And finally run make retag before make cross etc"

retag:
	git tag -f $(LAST_TAG)

gen_tests:
	cd fstest/fstests && go run gen_tests.go
