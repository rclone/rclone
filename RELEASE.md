# Release

This file describes how to make the various kinds of releases

## Extra required software for making a release

  * [gh the github cli](https://github.com/cli/cli) for uploading packages
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
  * git push --follow-tags origin
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

  * BASE_TAG=v1.XX          # e.g. v1.52
  * NEW_TAG=${BASE_TAG}.Y   # e.g. v1.52.1
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
  * git co master
  * `#` cherry pick the changes to the changelog - check the diff to make sure it is correct
  * git checkout ${BASE_TAG}-stable docs/content/changelog.md
  * git commit -a -v -m "Changelog updates from Version ${NEW_TAG}"
  * git push

## Making a manual build of docker

The rclone docker image should autobuild on via GitHub actions.  If it doesn't
or needs to be updated then rebuild like this.

See: https://github.com/ilteoood/docker_buildx/issues/19
See: https://github.com/ilteoood/docker_buildx/blob/master/scripts/install_buildx.sh

```
git co v1.54.1
docker pull golang
export DOCKER_CLI_EXPERIMENTAL=enabled
docker buildx create --name actions_builder --use
docker run --rm --privileged docker/binfmt:820fdd95a9972a5308930a2bdfb8573dd4447ad3
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
SUPPORTED_PLATFORMS=$(docker buildx inspect --bootstrap | grep 'Platforms:*.*' | cut -d : -f2,3)
echo "Supported platforms: $SUPPORTED_PLATFORMS"
docker buildx build --platform linux/amd64,linux/386,linux/arm64,linux/arm/v7 -t rclone/rclone:1.54.1 -t rclone/rclone:1.54 -t rclone/rclone:1 -t rclone/rclone:latest --push .
docker buildx stop actions_builder
```

### Old build for linux/amd64 only

```
docker pull golang
docker build --rm --ulimit memlock=67108864  -t rclone/rclone:1.52.0 -t rclone/rclone:1.52 -t rclone/rclone:1 -t rclone/rclone:latest .
docker push rclone/rclone:1.52.0
docker push rclone/rclone:1.52
docker push rclone/rclone:1
docker push rclone/rclone:latest
```
