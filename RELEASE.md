Extra required software for making a release
  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * pandoc for making the html and man pages

Making a release
  * git status - make sure everything is checked in
  * Check travis & appveyor builds are green
  * make check
  * make test # see integration test server or run locally
  * make tag
  * edit docs/content/changelog.md
  * make doc
  * git status - to check for new man pages - git add them
  * git commit -a -v -m "Version v1.XX"
  * make retag
  * git push --tags origin master
  * # Wait for the appveyor and travis builds to complete then...
  * make fetch_binaries
  * make tarball
  * make sign_upload
  * make check_sign
  * make upload
  * make upload_website
  * make upload_github
  * make startdev
  * # announce with forum post, twitter post, G+ post

Early in the next release cycle update the vendored dependencies
  * Review any pinned packages in go.mod and remove if possible
  * make update
  * git status
  * git add new files
  * git commit -a -v

If `make update` fails with errors like this:

```
# github.com/cpuguy83/go-md2man/md2man
../../../../pkg/mod/github.com/cpuguy83/go-md2man@v1.0.8/md2man/md2man.go:11:16: undefined: blackfriday.EXTENSION_NO_INTRA_EMPHASIS
../../../../pkg/mod/github.com/cpuguy83/go-md2man@v1.0.8/md2man/md2man.go:12:16: undefined: blackfriday.EXTENSION_TABLES
```

Can be fixed with

    * GO111MODULE=on go get -u github.com/russross/blackfriday@v1.5.2
    * GO111MODULE=on go mod tidy
    * GO111MODULE=on go mod vendor
 

Making a point release.  If rclone needs a point release due to some
horrendous bug, then
  * git branch v1.XX v1.XX-fixes
  * git cherry-pick any fixes
  * Test (see above)
  * make NEW_TAG=v1.XX.1 tag
  * edit docs/content/changelog.md
  * make TAG=v1.43.1 doc
  * git commit -a -v -m "Version v1.XX.1"
  * git tag -d -v1.XX.1
  * git tag -s -m "Version v1.XX.1" v1.XX.1
  * git push --tags -u origin v1.XX-fixes
  * make BRANCH_PATH= TAG=v1.43.1 fetch_binaries
  * make TAG=v1.43.1 tarball
  * make TAG=v1.43.1 sign_upload
  * make TAG=v1.43.1 check_sign
  * make TAG=v1.43.1 upload
  * make TAG=v1.43.1 upload_website
  * make TAG=v1.43.1 upload_github
  * NB this overwrites the current beta so after the release, rebuild the last travis build
  * Announce!
