# Contributing to rclone #

This is a short guide on how to contribute things to rclone.

## Reporting a bug ##

Bug reports are welcome.  Please when submitting add:

  * Rclone version (eg output from `rclone -V`)
  * Which OS you are using and how many bits (eg Windows 7, 64 bit)
  * The command you were trying to run (eg `rclone copy /tmp remote:tmp`)
  * A log of the command with the `-v` flag (eg output from `rclone -v copy /tmp remote:tmp`)
    * if the log contains secrets then edit the file with a text editor first to obscure them

## Submitting a pull request ##

If you find a bug that you'd like to fix, or a new feature that you'd
like to implement then please submit a pull request via Github.

If it is a big feature then make an issue first so it can be discussed.

You'll need a Go environment set up with GOPATH set.  See [the Go
getting started docs](https://golang.org/doc/install) for more info.

First in your web browser press the fork button on [rclone's Github
page](https://github.com/ncw/rclone).

Now in your terminal

    go get github.com/ncw/rclone
    cd $GOPATH/src/github.com/ncw/rclone
    git remote rename origin upstream
    git remote add origin git@github.com:YOURUSER/rclone.git

Make a branch to add your new feature

    git checkout -b my-new-feature

And get hacking.

When ready - run the unit tests for the code you changed

    go test -v

Note that you make need to make a test remote, eg `TestSwift` for some
of the unit tests.

Note the top level Makefile targets

  * make check
  * make test

Both of these will be run by Travis when you make a pull request but
you can do this yourself locally too.

Make sure you

  * Add documentation for a new feature
  * Add unit tests for a new feature
  * squash commits down to one per feature
  * rebase to master `git rebase master`

When you are done with that

  git push origin my-new-feature

Go to the Github website and click [Create pull
request](https://help.github.com/articles/creating-a-pull-request/).

You patch will get reviewed and you might get asked to fix some stuff.

If so, then make the changes in the same branch, squash the commits,
rebase it to master then push it to Github with `--force`.

## Testing ##

rclone's tests are run from the go testing framework, so at the top
level you can run this to run all the tests.

    go test -v ./...
    
rclone contains a mixture of unit tests and integration tests.
Because it is difficult (and in some respects pointless) to test cloud
storage systems by mocking all their interfaces, rclone unit tests can
run against any of the backends.  This is done by making specially
named remotes in the default config file.

If you wanted to test changes in the `drive` backend, then you would
need to make a remote called `TestDrive`.

You can then run the unit tests in the drive directory.  These tests
are skipped if `TestDrive:` isn't defined.

    cd drive
    go test -v

You can then run the integration tests which tests all of rclone's
operations.  Normally these get run against the local filing system,
but they can be run against any of the remotes.

    cd ../fs
    go test -v -remote TestDrive:
    go test -v -remote TestDrive: -subdir

If you want to run all the integration tests against all the remotes,
then run in that directory

    ./test_all.sh

## Making a release ##

There are separate instructions for making a release in the RELEASE.md
file - doing the first few steps is useful before making a
contribution.

  * go get -u -f -v ./...
  * make check
  * make test
  * make tag

## Writing a new backend ##

Choose a name.  The docs here will use `remote` as an example.

Note that in rclone terminology a file system backend is called a
remote or an fs.

Research

  * Look at the interfaces defined in `fs/fs.go`
  * Study one or more of the existing remotes

Getting going

  * Create `remote/remote.go` (copy this from a similar fs)
  * Add your fs to the imports in `rclone.go`

Unit tests

  * Create a config entry called `TestRemote` for the unit tests to use
  * Add your fs to the end of `fstest/fstests/gen_tests.go`
  * generate `remote/remote_test.go` unit tests `cd fstest/fstests; go generate`
  * Make sure all tests pass with `go test -v`

Integration tests

  * Add your fs to the imports in `fs/operations_test.go`
  * Add your fs to `fs/test_all.sh`
  * Make sure integration tests pass with
      * `cd fs`
      * `go test -v -remote TestRemote:` and
      * `go test -v -remote TestRemote: -subdir`

Add your fs to the docs

  * `README.md` - main Github page
  * `docs/content/remote.md` - main docs page
  * `docs/content/overview.md` - overview docs
  * `docs/content/about.md` - front page of rclone.org
  * `docs/layouts/chrome/navbar.html` - add it to the website navigation
  * `make_manual.py` - add the page to the `docs` constant
