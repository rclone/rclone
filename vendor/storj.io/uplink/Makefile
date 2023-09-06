.PHONY: bump-dependencies
bump-dependencies:
	go get storj.io/common@main
	go mod tidy
	cd testsuite;\
		go get storj.io/common@main storj.io/storj@main storj.io/uplink@main;\
		go mod tidy;\