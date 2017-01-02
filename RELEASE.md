Required software for making a release
  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * [gox](https://github.com/mitchellh/gox) for cross compiling
    * Run `gox -build-toolchain`
    * This assumes you have your own source checkout
  * pandoc for making the html and man pages
  * errcheck - go get github.com/kisielk/errcheck
  * golint - go get github.com/golang/lint

Making a release
  * git status - make sure everything is checked in
  * Check travis & appveyor builds are green
  * make check
  * make test
  * make tag
  * edit docs/content/changelog.md
  * make doc
  * git status - to check for new man pages - git add them
  * git commit -a -v -m "Version v1.XX"
  * make retag
  * # Set the GOPATH for a current stable go compiler
  * make cross
  * make upload
  * make upload_website
  * git push --tags origin master
  * make upload_github

Early in the next release cycle update the vendored dependencies
  * make update
  * git status
  * git add new files
  * carry forward any patches to vendor stuff
  * git commit -a -v
