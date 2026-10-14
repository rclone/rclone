# Release

This file describes how to make the various kinds of releases

## Extra required software for making a release

- [gh the github cli](https://github.com/cli/cli) for uploading packages
- pandoc for making the html and man pages
- hugo for building the website
- golangci-lint and docker for `make check` (markdownlint runs in docker)
- a gpg key for signing the tag and the checksums
- rclone remotes `beta.rclone.org:`, `downloads.rclone.org:`,
  `www.rclone.org:` and `test-rclone-org:`
- the sponsor logos - see [Sponsor logos](#sponsor-logos)

## Making a release

- Enable the [release freeze rule](https://github.com/rclone/rclone/settings/rules/24751862)
- git checkout master # see below for stable branch
- git pull # IMPORTANT
- make fetch-gui-and-commit # bump the embedded GUI if rclone-web has a newer release
- git status - make sure everything is checked in
- Check GitHub actions build for master is Green
- Check the integration tests at <https://integration.rclone.org>
- cat VERSION # check this is the version being released
- make tag # signed tag on HEAD + new changelog entry + docs
- edit docs/content/changelog.md # entries already in point releases are
  removed automatically
- make tidy
- make doc # again to pick up the changelog edits
- git status - to check for new command docs in docs/content/commands - git add them
- git commit -a -v -m "Version v1.XX.0"
- make check # if this fails, fix, git commit --amend and run it again
- make retag # moves the tag onto the Version commit
- git push origin # without --follow-tags so it doesn't push the tag if it fails
- git push --follow-tags origin
- \# Wait for the GitHub builds to complete then...
- \# Don't commit anything until after make upload_github as these steps
  use the tag on HEAD to find the version
- make fetch_binaries
- make tarball
- make vendorball
- make sign_upload
- make check_sign
- make upload
- make upload_test_website # check sponsors are correct/unchanged
- make upload_website
- make upload_github # this triggers the docker plugin and winget workflows
- Check the docker image, docker plugin and winget GitHub actions succeeded
- make startdev # works out the next version for master or a stable branch
- git push
- Disable the release freeze rule
- Release any security advisories
  - request a CVE for each advisory if not done already
  - set the patched version on each advisory and publish it
  - when the CVEs are assigned replace `CVE-PENDING` in the changelog on
    master and the stable branch then `make upload_website`
- \# announce with forum post, twitter post, and post to announcements list

## Update dependencies

Early in the next release cycle update the dependencies.

- Active Pins
  - `github.com/gdamore/tcell/v2` v2.9.0 - awaiting fix for [tcell/issues/1124](https://github.com/gdamore/tcell/issues/1124)
    - See: [#9539](https://github.com/rclone/rclone/issues/9539)
- Review Active Pins in go.mod and remove if possible
- `make updatedirect`
- `make GOTAGS=cmount`
- `make compiletest`
- Fix anything which doesn't compile at this point and commit changes here
- `git commit -a -v -m "build: update all dependencies"`

If the `make updatedirect` upgrades the version of go in the `go.mod`

```text
go 1.22.0
```

then so this instead `make updatedirectnoupgrade`. This will try each
upgrade individually. Consider making an upstream bug report for any
`SKIPPING` lines as these only support the latest Go version not the
previous version too.

If `make updatedirect` or `make updatedirectnoupgrade` added a
`toolchain` directive then remove it. We don't want to force a
toolchain on our users. Linux packagers are often using a version of
Go that is a few versions out of date.

Optionally upgrade the direct and indirect dependencies. This is very
likely to fail if `make updatedirectnoupgrade` was used.

- `make update`
- `make GOTAGS=cmount`
- `make compiletest`
- roll back any updates which didn't compile
- `git commit -a -v --amend`
- **NB** watch out for this changing the default go version in `go.mod`

Note that `make update` updates all direct and indirect dependencies
and there can occasionally be forwards compatibility problems with
doing that so it may be necessary to roll back dependencies to the
version specified by `make updatedirect` in order to get rclone to
build.

Once it compiles locally, push it on a test branch and commit fixes
until the tests pass.

### Pseudo versions

Go makes pseudo versions for untagged repos and repos not at a tag.
The pseudo versions on repos that have been tagged before do not get
updated automatically so need manually checking. These can be found with

```console
grep -E '[0-9]{14}-[0-9a-f]{12}' go.mod | grep -v indirect | grep -v 'v0\.0\.0'
```

These will need to be updated manually using the `go get ...@branch`
syntax.

### Major versions

The above procedure will not upgrade major versions, so v2 to v3.
However this tool can show which major versions might need to be
upgraded:

```console
go run github.com/icholy/gomajor@latest list -major
```

Expect API breakage when updating major versions.

## Updating Go

When a new Go stable is released update to it. We support the current
stable Go and the previous release which is in line with the rest of
the Go ecosystem.

These files will need editing:

- `.github/workflows/build.yml` - change current and previous Go versions
- `docs/content/install.md` - change minimum Go version required
- `fs/versioncheck.go` - update minimum Go version required
- `go.mod` - update minimum Go version required

Check it builds

- `make GOTAGS=cmount`
- `make compiletest`

Assuming `go1.XX` is current and `go1.YY` is previous version:

Use `git grep go1.YY` and `git grep go1.YY` to look for opportunities
to remove build tags we no longer need.

Commit with message like this:

```text
build: update to go1.YY and make go1.YY the minimum required version
```

Send to CI and if it passes, merge.

### gofix

Updating the minimum required version of Go is a good opportunity to
run the `go fix` command to modernize Go usage.

This needs to be run for all architectures.

```console
GOOS=linux go fix -tags cmount ./...
GOOS=freebsd go fix -tags cmount ./...
GOOS=windows go fix -tags cmount ./...
GOOS=darwin go fix -tags cmount ./...
```

Examine the diff carefully.

Commit with message

```text
build: modernize Go code with go fix for go1.YY
```

## Tidy beta

At some point after the release run

```console
bin/tidy-beta v1.55
```

where the version number is that of a couple ago to remove old beta binaries.

## Making a point release

If rclone needs a point release due to some horrendous bug:

Set vars

- BASE_TAG=v1.XX          # e.g. v1.52
- NEW_TAG=${BASE_TAG}.Y   # e.g. v1.52.1
- echo $BASE_TAG $NEW_TAG # v1.52 v1.52.1

First make the release branch.  If this is a second point release then
this will be done already.

- git co -b ${BASE_TAG}-stable ${BASE_TAG}.0
- make startdev # sets the version to ${BASE_TAG}.1
- git push -u origin ${BASE_TAG}-stable

Now

- Enable release freeze rule
- git co master
- git pull --rebase
- make fetch-gui-and-commit # update the GUI on master first
- git push
- git co ${BASE_TAG}-stable
- git pull
- cat VERSION # check this is ${NEW_TAG} - if not run make startdev
- `git cherry-pick -x` any fixes, including the GUI update if there was one
  - for security releases consider updating the Go patch version in
    `.github/workflows/build.yml` and dependencies with CVE fixes
  - fixes may depend on commits which weren't picked so push to CI
    and check it is green before tagging
- Do the steps in "Making a release" above, except
  - stay on ${BASE_TAG}-stable - don't checkout master or run
    make fetch-gui-and-commit
  - check the GitHub actions build for ${BASE_TAG}-stable not master
  - run the integration tests on ${BASE_TAG}-stable as described below
  - the release freeze rule is already enabled - leave it until the end
- git co master
- git pull --rebase
- git checkout ${BASE_TAG}-stable docs/content/changelog.md
- git diff --cached # check only the ${NEW_TAG} entry has changed
- git commit -v -m "Changelog updates from Version ${NEW_TAG}" docs/content/changelog.md
- git push
- Disable release freeze rule

Point releases are only made for the latest release. Once v1.(XX+1).0
is out the ${BASE_TAG}-stable branch is abandoned.

### Running the integration tests on the stable branch

The integration tests at <https://integration.rclone.org> test master.
To test the stable branch push it to the testing server and run the
tests there in `screen` as they take a while.

```console
git push -f rclone@rclone-testing:private-rclone.git ${BASE_TAG}-stable
ssh rclone@rclone-testing
screen
cd ~/integration-test && go run integration-test.go -repo /home/rclone/private-rclone.git -branch v1.XX-stable
```

## Sponsor logos

If updating the website note that the sponsor logos have been moved out of the
main repository.

You will need to checkout `/docs/static/img/logos` from <https://github.com/rclone/third-party-logos>
which is a private repo containing artwork from sponsors.

## Update the website between releases

Create an update website branch based off the last release

```console
git co -b update-website
```

If the branch already exists, double check there are no commits that need saving.

Now reset the branch to the last release

```console
git reset --hard v1.64.0
```

Create the changes, check them in, test with `make serve` then

```console
make upload_test_website
```

Check out <https://test.rclone.org> and when happy

```console
make upload_website
```

Cherry pick any changes back to master and the stable branch if it is active.

## Making a manual build of docker

To do a basic build of rclone's docker image to debug builds locally:

```console
docker buildx build --load -t rclone/rclone:testing --progress=plain .
docker run --rm rclone/rclone:testing version
```

To test the multipatform build

```console
docker buildx build -t rclone/rclone:testing --progress=plain --platform linux/amd64,linux/386,linux/arm64,linux/arm/v7,linux/arm/v6 .
```

To make a full build then set the tags correctly and add `--push`

Note that you can't only build one architecture - you need to build them all.

```console
docker buildx build --platform linux/amd64,linux/386,linux/arm64,linux/arm/v7,linux/arm/v6 -t rclone/rclone:1.54.1 -t rclone/rclone:1.54 -t rclone/rclone:1 -t rclone/rclone:latest --push .
```
