Required software for making a release
  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * [gox](https://github.com/mitchellh/gox) for cross compiling
    * Run `gox -build-toolchain`
    * This assumes you have your own source checkout
  * pandoc for making the html and man pages

Making a release
  * go get -u -f -v ./...
  * make test
  * make tag
  * edit docs/content/changelog.md
  * git commit -a -v
  * make retag
  * # Set the GOPATH for a gox enabled compiler - . ~/bin/go-cross - not required for go >= 1.5
  * make cross
  * make upload
  * make upload_website
  * git push --tags origin master
