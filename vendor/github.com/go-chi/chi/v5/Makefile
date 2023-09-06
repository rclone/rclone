.PHONY: all
all:
	@echo "**********************************************************"
	@echo "**                    chi build tool                    **"
	@echo "**********************************************************"


.PHONY: test
test:
	go clean -testcache && $(MAKE) test-router && $(MAKE) test-middleware

.PHONY: test-router
test-router:
	go test -race -v .

.PHONY: test-middleware
test-middleware:
	go test -race -v ./middleware

.PHONY: docs
docs:
	npx docsify-cli serve ./docs
