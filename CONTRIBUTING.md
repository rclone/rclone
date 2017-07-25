# Contributing to rclone #

This is a short guide on how to contribute things to rclone.

## Reporting a bug ##

If you've just got a question or aren't sure if you've found a bug
then please use the [rclone forum](https://forum.rclone.org/) instead
of filing an issue.

When filing an issue, please include the following information if
possible as well as a description of the problem.  Make sure you test
with the [latest beta of rclone](https://beta.rclone.org/):

  * Rclone version (eg output from `rclone -V`)
  * Which OS you are using and how many bits (eg Windows 7, 64 bit)
  * The command you were trying to run (eg `rclone copy /tmp remote:tmp`)
  * A log of the command with the `-vv` flag (eg output from `rclone -vv copy /tmp remote:tmp`)
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

Note that you may need to make a test remote, eg `TestSwift` for some
of the unit tests.

Note the top level Makefile targets

  * make check
  * make test

Both of these will be run by Travis when you make a pull request but
you can do this yourself locally too.  These require some extra go
packages which you can install with

  * make build_dep

Make sure you

  * Add documentation for a new feature (see below for where)
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

    go run test_all.go

## Writing Documentation ##

If you are adding a new feature then please update the documentation.

If you add a new flag, then if it is a general flag, document it in
`docs/content/docs.md` - the flags there are supposed to be in
alphabetical order.  If it is a remote specific flag, then document it
in `docs/content/remote.md`.

The only documentation you need to edit are the `docs/content/*.md`
files.  The MANUAL.*, rclone.1, web site etc are all auto generated
from those during the release process.  See the `make doc` and `make
website` targets in the Makefile if you are interested in how.  You
don't need to run these when adding a feature.

Documentation for rclone sub commands is with their code, eg
`cmd/ls/ls.go`.

## Making a release ##

There are separate instructions for making a release in the RELEASE.md
file.

## Commit messages ##

Please make the first line of your commit message a summary of the
change, and prefix it with the directory of the change followed by a
colon.  The changelog gets made by looking at just these first lines
so make it good!

If you have more to say about the commit, then enter a blank line and
carry on the description.  Remember to say why the change was needed -
the commit itself shows what was changed.

If the change fixes an issue then write `Fixes #1234` in the commit
message.  This can be on the subject line if it will fit.  If you
don't want to close the associated issue just put `#1234` and the
change will get linked into the issue.

Here is an example of a short commit message:

```
drive: add team drive support - fixes #885
```

And here is an example of a longer one:

```
mount: fix hang on errored upload

In certain circumstances if an upload failed then the mount could hang
indefinitely. This was fixed by closing the read pipe after the Put
completed.  This will cause the write side to return a pipe closed
error fixing the hang.

Fixes #1498
```

## Adding a dependency ##

rclone uses the [dep](https://github.com/golang/dep) tool to manage
its dependencies.  All code that rclone needs for building is stored
in the `vendor` directory for perfectly reproducable builds.

The `vendor` directory is entirely managed by the `dep` tool.

To add a new dependency

    dep ensure github.com/pkg/errors

You can add constraints on that package (see the `dep` documentation),
but don't unless you really need to.

Please check in the changes generated by dep including the `vendor`
directory and `Godep.toml` and `Godep.locl` in a single commit
separate from any other code changes.  Watch out for new files in
`vendor`.

## Updating a dependency ##

If you need to update a dependency then run

    dep ensure -update github.com/pkg/errors

Check in in a single commit as above.

## Updating all the dependencies ##

In order to update all the dependencies then run `make update`.  This
just runs `dep ensure -update`.  Check in the changes in a single
commit as above.

This should be done early in the release cycle to pick up new versions
of packages in time for them to get some testing.

## Updating a backend ##

If you update a backend then please run the unit tests and the
integration tests for that backend.

Assuming the backend is called `remote`, make create a config entry
called `TestRemote` for the tests to use.

Now `cd remote` and run `go test -v` to run the unit tests.

Then `cd fs` and run `go test -v -remote TestRemote:` to run the
integration tests.

The next section goes into more detail about the tests.

## Writing a new backend ##

Choose a name.  The docs here will use `remote` as an example.

Note that in rclone terminology a file system backend is called a
remote or an fs.

Research

  * Look at the interfaces defined in `fs/fs.go`
  * Study one or more of the existing remotes

Getting going

  * Create `remote/remote.go` (copy this from a similar remote)
    * onedrive is a good one to start from if you have a directory based remote
    * b2 is a good one to start from if you have a bucket based remote
  * Add your remote to the imports in `fs/all/all.go`
  * HTTP based remotes are easiest to maintain if they use rclone's rest module, but if there is a really good go SDK then use that instead.

Unit tests

  * Create a config entry called `TestRemote` for the unit tests to use
  * Add your fs to the end of `fstest/fstests/gen_tests.go`
  * generate `remote/remote_test.go` unit tests `cd fstest/fstests; go generate`
  * Make sure all tests pass with `go test -v`

Integration tests

  * Add your fs to `fs/test_all.go`
  * Make sure integration tests pass with
      * `cd fs`
      * `go test -v -remote TestRemote:`
  * If you are making a bucket based remote, then check with this also
      * `go test -v -remote TestRemote: -subdir`
  * And if your remote defines `ListR` this also
      * `go test -v -remote TestRemote: -fast-list`

Add your fs to the docs - you'll need to pick an icon for it from [fontawesome](http://fontawesome.io/icons/).  Keep lists of remotes in alphabetical order but with the local file system last.

  * `README.md` - main Github page
  * `docs/content/remote.md` - main docs page
  * `docs/content/overview.md` - overview docs
  * `docs/content/docs.md` - list of remotes in config section
  * `docs/content/about.md` - front page of rclone.org
  * `docs/layouts/chrome/navbar.html` - add it to the website navigation
  * `bin/make_manual.py` - add the page to the `docs` constant
  * `cmd/cmd.go` - the main help for rclone
