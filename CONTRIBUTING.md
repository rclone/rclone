# Contributing to rclone #

This is a short guide on how to contribute things to rclone.

## Reporting a bug ##

If you've just got a question or aren't sure if you've found a bug
then please use the [rclone forum](https://forum.rclone.org/) instead
of filing an issue.

When filing an issue, please include the following information if
possible as well as a description of the problem.  Make sure you test
with the [latest beta of rclone](https://beta.rclone.org/):

  * Rclone version (e.g. output from `rclone -V`)
  * Which OS you are using and how many bits (e.g. Windows 7, 64 bit)
  * The command you were trying to run (e.g. `rclone copy /tmp remote:tmp`)
  * A log of the command with the `-vv` flag (e.g. output from `rclone -vv copy /tmp remote:tmp`)
    * if the log contains secrets then edit the file with a text editor first to obscure them

## Submitting a pull request ##

If you find a bug that you'd like to fix, or a new feature that you'd
like to implement then please submit a pull request via GitHub.

If it is a big feature then make an issue first so it can be discussed.

You'll need a Go environment set up with GOPATH set.  See [the Go
getting started docs](https://golang.org/doc/install) for more info.

First in your web browser press the fork button on [rclone's GitHub
page](https://github.com/rclone/rclone).

Now in your terminal

    git clone https://github.com/rclone/rclone.git
    cd rclone
    git remote rename origin upstream
    git remote add origin git@github.com:YOURUSER/rclone.git
    go build

Make a branch to add your new feature

    git checkout -b my-new-feature

And get hacking.

When ready - run the unit tests for the code you changed

    go test -v

Note that you may need to make a test remote, e.g. `TestSwift` for some
of the unit tests.

Note the top level Makefile targets

  * make check
  * make test

Both of these will be run by Travis when you make a pull request but
you can do this yourself locally too.  These require some extra go
packages which you can install with

  * make build_dep

Make sure you

  * Add [documentation](#writing-documentation) for a new feature.
  * Follow the [commit message guidelines](#commit-messages).
  * Add [unit tests](#testing) for a new feature
  * squash commits down to one per feature
  * rebase to master with `git rebase master`

When you are done with that

    git push -u origin my-new-feature

Go to the GitHub website and click [Create pull
request](https://help.github.com/articles/creating-a-pull-request/).

You patch will get reviewed and you might get asked to fix some stuff.

If so, then make the changes in the same branch, squash the commits (make multiple commits one commit) by running:
```
git log # See how many commits you want to squash
git reset --soft HEAD~2 # This squashes the 2 latest commits together.
git status # Check what will happen, if you made a mistake resetting, you can run git reset 'HEAD@{1}' to undo.
git commit # Add a new commit message.
git push --force # Push the squashed commit to your GitHub repo.
# For more, see Stack Overflow, Git docs, or generally Duck around the web. jtagcat also recommends wizardzines.com
```

## CI for your fork ##

rclone currently uses [GitHub Actions](https://github.com/rclone/rclone/actions) to build and test the project, which should be automatically available for your fork too from the `Actions` tab in your repository.

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

    cd backend/drive
    go test -v

You can then run the integration tests which test all of rclone's
operations.  Normally these get run against the local file system,
but they can be run against any of the remotes.

    cd fs/sync
    go test -v -remote TestDrive:
    go test -v -remote TestDrive: -fast-list

    cd fs/operations
    go test -v -remote TestDrive:

If you want to use the integration test framework to run these tests
altogether with an HTML report and test retries then from the
project root:

    go install github.com/rclone/rclone/fstest/test_all
    test_all -backend drive

If you want to run all the integration tests against all the remotes,
then change into the project root and run

    make test

This command is run daily on the integration test server. You can
find the results at https://pub.rclone.org/integration-tests/

## Code Organisation ##

Rclone code is organised into a small number of top level directories
with modules beneath.

  * backend - the rclone backends for interfacing to cloud providers - 
    * all - import this to load all the cloud providers
    * ...providers
  * bin - scripts for use while building or maintaining rclone
  * cmd - the rclone commands
    * all - import this to load all the commands
    * ...commands
  * docs - the documentation and website
    * content - adjust these docs only - everything else is autogenerated
      * command - these are auto generated - edit the corresponding .go file
  * fs - main rclone definitions - minimal amount of code
    * accounting - bandwidth limiting and statistics
    * asyncreader - an io.Reader which reads ahead
    * config - manage the config file and flags
    * driveletter - detect if a name is a drive letter
    * filter - implements include/exclude filtering
    * fserrors - rclone specific error handling
    * fshttp - http handling for rclone
    * fspath - path handling for rclone
    * hash - defines rclone's hash types and functions
    * list - list a remote
    * log - logging facilities
    * march - iterates directories in lock step
    * object - in memory Fs objects
    * operations - primitives for sync, e.g. Copy, Move
    * sync - sync directories
    * walk - walk a directory
  * fstest - provides integration test framework
    * fstests - integration tests for the backends
    * mockdir - mocks an fs.Directory
    * mockobject - mocks an fs.Object
    * test_all - Runs integration tests for everything
  * graphics - the images used in the website, etc.
  * lib - libraries used by the backend
    * atexit - register functions to run when rclone exits
    * dircache - directory ID to name caching
    * oauthutil - helpers for using oauth
    * pacer - retries with backoff and paces operations
    * readers - a selection of useful io.Readers
    * rest - a thin abstraction over net/http for REST
  * vfs - Virtual FileSystem layer for implementing rclone mount and similar

## Writing Documentation ##

If you are adding a new feature then please update the documentation.

If you add a new general flag (not for a backend), then document it in
`docs/content/docs.md` - the flags there are supposed to be in
alphabetical order.

If you add a new backend option/flag, then it should be documented in
the source file in the `Help:` field.  The first line of this is used
for the flag help, the remainder is shown to the user in `rclone
config` and is added to the docs with `make backenddocs`.

The only documentation you need to edit are the `docs/content/*.md`
files.  The `MANUAL.*`, `rclone.1`, web site, etc. are all auto generated
from those during the release process.  See the `make doc` and `make
website` targets in the Makefile if you are interested in how.  You
don't need to run these when adding a feature.

Documentation for rclone sub commands is with their code, e.g.
`cmd/ls/ls.go`.

Note that you can use [GitHub's online editor](https://help.github.com/en/github/managing-files-in-a-repository/editing-files-in-another-users-repository)
for small changes in the docs which makes it very easy.

## Making a release ##

There are separate instructions for making a release in the RELEASE.md
file.

## Commit messages ##

Please make the first line of your commit message a summary of the
change that a user (not a developer) of rclone would like to read, and
prefix it with the directory of the change followed by a colon.  The
changelog gets made by looking at just these first lines so make it
good!

If you have more to say about the commit, then enter a blank line and
carry on the description.  Remember to say why the change was needed -
the commit itself shows what was changed.

Writing more is better than less.  Comparing the behaviour before the
change to that after the change is very useful.  Imagine you are
writing to yourself in 12 months time when you've forgotten everything
about what you just did and you need to get up to speed quickly.

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

rclone uses the [go
modules](https://tip.golang.org/cmd/go/#hdr-Modules__module_versions__and_more)
support in go1.11 and later to manage its dependencies.

rclone can be built with modules outside of the `GOPATH`.

To add a dependency `github.com/ncw/new_dependency` see the
instructions below.  These will fetch the dependency and add it to
`go.mod` and `go.sum`.

    GO111MODULE=on go get github.com/ncw/new_dependency

You can add constraints on that package when doing `go get` (see the
go docs linked above), but don't unless you really need to.

Please check in the changes generated by `go mod` including `go.mod`
and `go.sum` in the same commit as your other changes.

## Updating a dependency ##

If you need to update a dependency then run

    GO111MODULE=on go get -u github.com/pkg/errors

Check in a single commit as above.

## Updating all the dependencies ##

In order to update all the dependencies then run `make update`.  This
just uses the go modules to update all the modules to their latest
stable release. Check in the changes in a single commit as above.

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

  * Create `backend/remote/remote.go` (copy this from a similar remote)
    * box is a good one to start from if you have a directory based remote
    * b2 is a good one to start from if you have a bucket based remote
  * Add your remote to the imports in `backend/all/all.go`
  * HTTP based remotes are easiest to maintain if they use rclone's rest module, but if there is a really good go SDK then use that instead.
  * Try to implement as many optional methods as possible as it makes the remote more usable.
  * Use lib/encoder to make sure we can encode any path name and `rclone info` to help determine the encodings needed
    * `rclone purge -v TestRemote:rclone-info`
    * `rclone test info --all --remote-encoding None -vv --write-json remote.json TestRemote:rclone-info`
    * `go run cmd/test/info/internal/build_csv/main.go -o remote.csv remote.json`
    * open `remote.csv` in a spreadsheet and examine

Unit tests

  * Create a config entry called `TestRemote` for the unit tests to use
  * Create a `backend/remote/remote_test.go` - copy and adjust your example remote
  * Make sure all tests pass with `go test -v`

Integration tests

  * Add your backend to `fstest/test_all/config.yaml`
      * Once you've done that then you can use the integration test framework from the project root:
      * go install ./...
      * test_all -backends remote

Or if you want to run the integration tests manually:

  * Make sure integration tests pass with
      * `cd fs/operations`
      * `go test -v -remote TestRemote:`
      * `cd fs/sync`
      * `go test -v -remote TestRemote:`
  * If your remote defines `ListR` check with this also
      * `go test -v -remote TestRemote: -fast-list`

See the [testing](#testing) section for more information on integration tests.

Add your fs to the docs - you'll need to pick an icon for it from
[fontawesome](http://fontawesome.io/icons/).  Keep lists of remotes in
alphabetical order of full name of remote (e.g. `drive` is ordered as
`Google Drive`) but with the local file system last.

  * `README.md` - main GitHub page
  * `docs/content/remote.md` - main docs page (note the backend options are automatically added to this file with `make backenddocs`)
    * make sure this has the `autogenerated options` comments in (see your reference backend docs)
    * update them with `make backenddocs` - revert any changes in other backends
  * `docs/content/overview.md` - overview docs
  * `docs/content/docs.md` - list of remotes in config section
  * `docs/content/_index.md` - front page of rclone.org
  * `docs/layouts/chrome/navbar.html` - add it to the website navigation
  * `bin/make_manual.py` - add the page to the `docs` constant

Once you've written the docs, run `make serve` and check they look OK
in the web browser and the links (internal and external) all work.

## Writing a plugin ##

New features (backends, commands) can also be added "out-of-tree", through Go plugins.
Changes will be kept in a dynamically loaded file instead of being compiled into the main binary.
This is useful if you can't merge your changes upstream or don't want to maintain a fork of rclone.

Usage

 - Naming
   - Plugins names must have the pattern `librcloneplugin_KIND_NAME.so`.
   - `KIND` should be one of `backend`, `command` or `bundle`.
   - Example: A plugin with backend support for PiFS would be called
     `librcloneplugin_backend_pifs.so`.
 - Loading
   - Supported on macOS & Linux as of now. ([Go issue for Windows support](https://github.com/golang/go/issues/19282))
   - Supported on rclone v1.50 or greater.
   - All plugins in the folder specified by variable `$RCLONE_PLUGIN_PATH` are loaded.
   - If this variable doesn't exist, plugin support is disabled.
   - Plugins must be compiled against the exact version of rclone to work.
     (The rclone used during building the plugin must be the same as the source of rclone)

Building

To turn your existing additions into a Go plugin, move them to an external repository
and change the top-level package name to `main`.

Check `rclone --version` and make sure that the plugin's rclone dependency and host Go version match.

Then, run `go build -buildmode=plugin -o PLUGIN_NAME.so .` to build the plugin.

[Go reference](https://godoc.org/github.com/rclone/rclone/lib/plugin)

[Minimal example](https://gist.github.com/terorie/21b517ee347828e899e1913efc1d684f)
