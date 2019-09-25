### Please only report errors with gometalinter itself

gometalinter relies on underlying linters to detect issues in source code.
If your issue seems to be related to an underlying linter, please report an
issue against that linter rather than gometalinter. For a full list of linters
and their repositories please see the [README](README.md).

### Do you want to upgrade a vendored linter?

Please send a PR. We use [GVT](https://github.com/FiloSottile/gvt). It should be as simple as:

```
go get github.com/FiloSottile/gvt
cd _linters
gvt update <linter>
git add <paths>
```

### Before you report an issue

Sometimes gometalinter will not report issues that you think it should. There
are three things to try in that case:

#### 1. Update to the latest build of gometalinter and all linters

    go get -u github.com/alecthomas/gometalinter
    gometalinter --install

If you're lucky, this will fix the problem.

#### 2. Analyse the debug output

If that doesn't help, the problem may be elsewhere (in no particular order):

1. Upstream linter has changed its output or semantics.
2. gometalinter is not invoking the tool correctly.
3. gometalinter regular expression matches are not correct for a linter.
4. Linter is exceeding the deadline.

To find out what's going on run in debug mode:

    gometalinter --debug

This will show all output from the linters and should indicate why it is
failing.

#### 3. Run linters manually

The output of `gometalinter --debug` should show the exact commands gometalinter
is running. Run these commands from the command line to determine if the linter
or gometaliner is at fault.

#### 4. Report an issue.

Failing all else, if the problem looks like a bug please file an issue and
include the output of `gometalinter --debug`
