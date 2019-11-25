# Release

This file describes how to make the various kinds of releases

## Extra required software for making a release

  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * pandoc for making the html and man pages

## Making a release

  * git status - make sure everything is checked in
  * Check travis & appveyor builds are green
  * make check
  * make test # see integration test server or run locally
  * make tag
  * edit docs/content/changelog.md
  * make tidy
  * make doc
  * git status - to check for new man pages - git add them
  * git commit -a -v -m "Version v1.XX.0"
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
 

## Making a point release

If rclone needs a point release due to some horrendous bug:

First make the release branch.  If this is a second point release then
this will be done already.

  * BASE_TAG=v1.XX          # eg v1.49
  * NEW_TAG=${BASE_TAG}.Y   # eg v1.49.1
  * echo $BASE_TAG $NEW_TAG # v1.49 v1.49.1
  * git branch ${BASE_TAG} ${BASE_TAG}-fixes

Now

  * git co ${BASE_TAG}-fixes
  * git cherry-pick any fixes
  * Test (see above)
  * make NEXT_VERSION=${NEW_TAG} tag
  * edit docs/content/changelog.md
  * make TAG=${NEW_TAG} doc
  * git commit -a -v -m "Version ${NEW_TAG}"
  * git tag -d ${NEW_TAG}
  * git tag -s -m "Version ${NEW_TAG}" ${NEW_TAG}
  * git push --tags -u origin ${BASE_TAG}-fixes
  * Wait for builds to complete
  * make BRANCH_PATH= TAG=${NEW_TAG} fetch_binaries
  * make TAG=${NEW_TAG} tarball
  * make TAG=${NEW_TAG} sign_upload
  * make TAG=${NEW_TAG} check_sign
  * make TAG=${NEW_TAG} upload
  * make TAG=${NEW_TAG} upload_website
  * make TAG=${NEW_TAG} upload_github
  * NB this overwrites the current beta so we need to do this
  * git co master
  * make VERSION=${NEW_TAG} startdev
  * # cherry pick the changes to the changelog and VERSION
  * git checkout ${BASE_TAG}-fixes VERSION docs/content/changelog.md
  * git commit --amend
  * git push
  * Announce!

## Making a manual build of docker

The rclone docker image should autobuild on docker hub.  If it doesn't
or needs to be updated then rebuild like this.

```
docker build -t rclone/rclone:1.49.1 -t rclone/rclone:1.49 -t rclone/rclone:1 -t rclone/rclone:latest .
docker push rclone/rclone:1.49.1
docker push rclone/rclone:1.49
docker push rclone/rclone:1
docker push rclone/rclone:latest
```
