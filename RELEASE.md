# Release

This file describes how to make the various kinds of releases

## Extra required software for making a release

  * [gh the github cli](https://github.com/cli/cli) for uploading packages
  * pandoc for making the html and man pages

## Making a release

  * git checkout master # see below for stable branch
  * git pull # IMPORTANT
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
  * git push origin # without --follow-tags so it doesn't push the tag if it fails
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

## Update dependencies

Early in the next release cycle update the dependencies.

  * Review any pinned packages in go.mod and remove if possible
  * `make updatedirect`
  * `make GOTAGS=cmount`
  * `make compiletest`
  * Fix anything which doesn't compile at this point and commit changes here
  * `git commit -a -v -m "build: update all dependencies"`

If the `make updatedirect` upgrades the version of go in the `go.mod`
then go to manual mode. `go1.20` here is the lowest supported version
in the `go.mod`.

```
go list -m -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' all > /tmp/potential-upgrades
go get -d $(cat /tmp/potential-upgrades)
go mod tidy -go=1.20 -compat=1.20
```

If the `go mod tidy` fails use the output from it to remove the
package which can't be upgraded from `/tmp/potential-upgrades` when
done

```
git co go.mod go.sum
```

And try again.

Optionally upgrade the direct and indirect dependencies. This is very
likely to fail if the manual method was used abve - in that case
ignore it as it is too time consuming to fix.

  * `make update`
  * `make GOTAGS=cmount`
  * `make compiletest`
  * roll back any updates which didn't compile
  * `git commit -a -v --amend`
  * **NB** watch out for this changing the default go version in `go.mod`

Note that `make update` updates all direct and indirect dependencies
and there can occasionally be forwards compatibility problems with
doing that so it may be necessary to roll back dependencies to the
version specified by `make updatedirect` in order to get rclone to
build.

Once it compiles locally, push it on a test branch and commit fixes
until the tests pass.

## Tidy beta

At some point after the release run

    bin/tidy-beta v1.55

where the version number is that of a couple ago to remove old beta binaries.

## Making a point release

If rclone needs a point release due to some horrendous bug:

Set vars

  * BASE_TAG=v1.XX          # e.g. v1.52
  * NEW_TAG=${BASE_TAG}.Y   # e.g. v1.52.1
  * echo $BASE_TAG $NEW_TAG # v1.52 v1.52.1

First make the release branch.  If this is a second point release then
this will be done already.

  * git co -b ${BASE_TAG}-stable ${BASE_TAG}.0
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

## Sponsor logos

If updating the website note that the sponsor logos have been moved out of the main repository.

You will need to checkout `/docs/static/img/logos` from https://github.com/rclone/third-party-logos
which is a private repo containing artwork from sponsors.

## Update the website between releases

Create an update website branch based off the last release

    git co -b update-website

If the branch already exists, double check there are no commits that need saving.

Now reset the branch to the last release

    git reset --hard v1.64.0

Create the changes, check them in, test with `make serve` then

    make upload_test_website

Check out https://test.rclone.org and when happy

    make upload_website

Cherry pick any changes back to master and the stable branch if it is active.

## Making a manual build of docker

To do a basic build of rclone's docker image to debug builds locally:

```
docker buildx build --load -t rclone/rclone:testing --progress=plain .
docker run --rm rclone/rclone:testing version
```

To test the multipatform build

```
docker buildx build -t rclone/rclone:testing --progress=plain --platform linux/amd64,linux/386,linux/arm64,linux/arm/v7,linux/arm/v6 .
```

To make a full build then set the tags correctly and add `--push`

Note that you can't only build one architecture - you need to build them all.

```
docker buildx build --platform linux/amd64,linux/386,linux/arm64,linux/arm/v7,linux/arm/v6 -t rclone/rclone:1.54.1 -t rclone/rclone:1.54 -t rclone/rclone:1 -t rclone/rclone:latest --push .
```
