# Go parameters
GOCMD = go
GOTEST = $(GOCMD) test 

.PHONY: test
test:
	$(GOTEST) -race ./...

test-coverage:
	echo "" > $(COVERAGE_REPORT); \
	$(GOTEST) -coverprofile=$(COVERAGE_REPORT) -coverpkg=./... -covermode=$(COVERAGE_MODE) ./...
