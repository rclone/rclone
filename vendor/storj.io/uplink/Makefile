.PHONY: bump-dependencies
bump-dependencies:
	go get storj.io/common@master
	go mod tidy
	cd testsuite;\
		go get storj.io/common@master storj.io/storj@master storj.io/uplink@master;\
		go mod tidy