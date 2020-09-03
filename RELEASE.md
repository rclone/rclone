# Release

This file describes how to make the various kinds of releases

## Extra required software for making a release

  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * pandoc for making the html and man pages

## Making a release

  * git checkout master # see below for stable branch
  * git pull
  * git status - make sure everything is checked in
  * Check GitHub actions build for master is Green
  * make test # see integration test server or run locally
  * make tag
  * edit docs/content/changelog.md # make sure to remove duplicate logs from point releases
  * make tidy
  * make doc
  * git status - to check for new man pages - git add them
  * git commit -a -v -m "Version v1.XX.0"
  * make retag
  * git push --tags origin master
  * # Wait for the GitHub builds to complete then...
  * make fetch_binaries
  * make tarball
  * make vendorball
  * make sign_upload
  * make check_sign
  * make upload
  * make upload_website
  * make upload_github
  * make startdev # make startstable for stable branch
  * # announce with forum post, twitter post, patreon post

Early in the next release cycle update the dependencies

  * Review any pinned packages in go.mod and remove if possible
  * make update
  * git status
  * git add new files
  * git commit -a -v

## Making a point release

If rclone needs a point release due to some horrendous bug:

Set vars

  * BASE_TAG=v1.XX          # eg v1.52
  * NEW_TAG=${BASE_TAG}.Y   # eg v1.52.1
  * echo $BASE_TAG $NEW_TAG # v1.52 v1.52.1

First make the release branch.  If this is a second point release then
this will be done already.

  * git branch ${BASE_TAG} ${BASE_TAG}-stable
  * git co ${BASE_TAG}-stable
  * make startstable

Now

  * git co ${BASE_TAG}-stable
  * git cherry-pick any fixes
  * Do the steps as above
  * make startstable
  * NB this overwrites the current beta so we need to do this - FIXME is this true any more?
  * git co master
  * # cherry pick the changes to the changelog
  * git checkout ${BASE_TAG}-stable docs/content/changelog.md
  * git commit -a -v -m "Changelog updates from Version ${NEW_TAG}"
  * git push

## Making a manual build of docker

The rclone docker image should autobuild on via GitHub actions.  If it doesn't
or needs to be updated then rebuild like this.

```
docker pull golang
docker build --rm --ulimit memlock=67108864  -t rclone/rclone:1.52.0 -t rclone/rclone:1.52 -t rclone/rclone:1 -t rclone/rclone:latest .
docker push rclone/rclone:1.52.0
docker push rclone/rclone:1.52
docker push rclone/rclone:1
docker push rclone/rclone:latest
```
