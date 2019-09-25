# Go Meta Linter
[![Build Status](https://travis-ci.org/alecthomas/gometalinter.svg)](https://travis-ci.org/alecthomas/gometalinter) [![Gitter chat](https://badges.gitter.im/alecthomas.svg)](https://gitter.im/alecthomas/Lobby)

<!-- MarkdownTOC -->

- [Installing](#installing)
- [Editor integration](#editor-integration)
- [Supported linters](#supported-linters)
- [Configuration file](#configuration-file)
    - [`Format` key](#format-key)
    - [Format Methods](#format-methods)
  - [Adding Custom linters](#adding-custom-linters)
- [Comment directives](#comment-directives)
- [Quickstart](#quickstart)
- [FAQ](#faq)
  - [Exit status](#exit-status)
  - [What's the best way to use `gometalinter` in CI?](#whats-the-best-way-to-use-gometalinter-in-ci)
  - [How do I make `gometalinter` work with Go 1.5 vendoring?](#how-do-i-make-gometalinter-work-with-go-15-vendoring)
  - [Why does `gometalinter --install` install a fork of gocyclo?](#why-does-gometalinter---install-install-a-fork-of-gocyclo)
  - [Many unexpected errors are being reported](#many-unexpected-errors-are-being-reported)
  - [Gometalinter is not working](#gometalinter-is-not-working)
    - [1. Update to the latest build of gometalinter and all linters](#1-update-to-the-latest-build-of-gometalinter-and-all-linters)
    - [2. Analyse the debug output](#2-analyse-the-debug-output)
    - [3. Report an issue.](#3-report-an-issue)
  - [How do I filter issues between two git refs?](#how-do-i-filter-issues-between-two-git-refs)
- [Checkstyle XML format](#checkstyle-xml-format)

<!-- /MarkdownTOC -->


The number of tools for statically checking Go source for errors and warnings
is impressive.

This is a tool that concurrently runs a whole bunch of those linters and
normalises their output to a standard format:

    <file>:<line>:[<column>]: <message> (<linter>)

eg.

    stutter.go:9::warning: unused global variable unusedGlobal (varcheck)
    stutter.go:12:6:warning: exported type MyStruct should have comment or be unexported (golint)

It is intended for use with editor/IDE integration.

## Installing

To install the latest stable release:

    curl https://git.io/vp6lP | sh

Alternatively you can install a specific version from the [releases](https://github.com/alecthomas/gometalinter/releases) list.

## Editor integration

- [SublimeLinter plugin](https://github.com/alecthomas/SublimeLinter-contrib-gometalinter).
- [Atom go-plus package](https://atom.io/packages/go-plus).
- [Emacs Flycheck checker](https://github.com/favadi/flycheck-gometalinter).
- [Go for Visual Studio Code](https://marketplace.visualstudio.com/items?itemName=lukehoban.Go).
- Vim/Neovim
    - [Neomake](https://github.com/neomake/neomake).
    - [Syntastic](https://github.com/scrooloose/syntastic/wiki/Go:---gometalinter) `let g:syntastic_go_checkers = ['gometalinter']`.
    - [ale](https://github.com/w0rp/ale) `let g:ale_linters = {'go': ['gometalinter']}`
    - [vim-go](https://github.com/fatih/vim-go) with the `:GoMetaLinter` command.

## Supported linters

- [go vet](https://golang.org/cmd/vet/) - Reports potential errors that otherwise compile.
- [go tool vet --shadow](https://golang.org/cmd/vet/#hdr-Shadowed_variables) - Reports variables that may have been unintentionally shadowed.
- [gotype](https://golang.org/x/tools/cmd/gotype) - Syntactic and semantic analysis similar to the Go compiler.
- [gotype -x](https://golang.org/x/tools/cmd/gotype) - Syntactic and semantic analysis in external test packages (similar to the Go compiler).
- [deadcode](https://github.com/tsenart/deadcode) - Finds unused code.
- [gocyclo](https://github.com/alecthomas/gocyclo) - Computes the cyclomatic complexity of functions.
- [golint](https://github.com/golang/lint) - Google's (mostly stylistic) linter.
- [varcheck](https://github.com/opennota/check) - Find unused global variables and constants.
- [structcheck](https://github.com/opennota/check) - Find unused struct fields.
- [maligned](https://github.com/mdempsky/maligned) -  Detect structs that would take less memory if their fields were sorted.
- [errcheck](https://github.com/kisielk/errcheck) - Check that error return values are used.
- [megacheck](https://github.com/dominikh/go-tools/tree/master/cmd/megacheck) - Run staticcheck, gosimple and unused, sharing work.
- [dupl](https://github.com/mibk/dupl) - Reports potentially duplicated code.
- [ineffassign](https://github.com/gordonklaus/ineffassign) - Detect when assignments to *existing* variables are not used.
- [interfacer](https://github.com/mvdan/interfacer) - Suggest narrower interfaces that can be used.
- [unconvert](https://github.com/mdempsky/unconvert) - Detect redundant type conversions.
- [goconst](https://github.com/jgautheron/goconst) - Finds repeated strings that could be replaced by a constant.
- [gosec](https://github.com/securego/gosec) - Inspects source code for security problems by scanning the Go AST.

Disabled by default (enable with `--enable=<linter>`):

- [testify](https://github.com/stretchr/testify) - Show location of failed testify assertions.
- [test](http://golang.org/pkg/testing/) - Show location of test failures from the stdlib testing module.
- [gofmt -s](https://golang.org/cmd/gofmt/) - Checks if the code is properly formatted and could not be further simplified.
- [goimports](https://godoc.org/golang.org/x/tools/cmd/goimports) - Checks missing or unreferenced package imports.
- [gosimple](https://github.com/dominikh/go-tools/tree/master/cmd/gosimple) - Report simplifications in code.
- [gochecknoinits](https://4d63.com/gochecknoinits) - Report init functions, to reduce side effects in code.
- [gochecknoglobals](https://4d63.com/gochecknoglobals) - Report global vars, to reduce side effects in code.
- [lll](https://github.com/walle/lll) - Report long lines (see `--line-length=N`).
- [misspell](https://github.com/client9/misspell) - Finds commonly misspelled English words.
- [nakedret](https://github.com/alexkohler/nakedret) - Finds naked returns.
- [unparam](https://github.com/mvdan/unparam) - Find unused function parameters.
- [unused](https://github.com/dominikh/go-tools/tree/master/cmd/unused) - Find unused variables.
- [safesql](https://github.com/stripe/safesql) - Finds potential SQL injection vulnerabilities.
- [staticcheck](https://github.com/dominikh/go-tools/tree/master/cmd/staticcheck) - Statically detect bugs, both obvious and subtle ones.

Additional linters can be added through the command line with `--linter=NAME:COMMAND:PATTERN` (see [below](#details)).

## Configuration file

gometalinter now supports a JSON configuration file called `.gometalinter.json` that can
be placed at the root of your project. The configuration file will be automatically loaded
from the working directory or any parent directory and can be overridden by passing
`--config=<file>` or ignored with `--no-config`. The format of this file is determined by
the `Config` struct in [config.go](https://github.com/alecthomas/gometalinter/blob/master/config.go).

The configuration file mostly corresponds to command-line flags, with the following exceptions:

- Linters defined in the configuration file will overlay existing definitions, not replace them.
- "Enable" defines the exact set of linters that will be enabled (default
  linters are disabled). `--help` displays the list of default linters with the exact names
  you must use.

Here is an example configuration file:

```json
{
  "Enable": ["deadcode", "unconvert"]
}
```

If a `.gometalinter.json` file is loaded, individual options can still be overridden by
passing command-line flags. All flags are parsed in order, meaning configuration passed
with the `--config` flag will override any command-line flags passed before and be
overridden by flags passed after.


#### `Format` key

The default `Format` key places the different fields of an `Issue` into a template. this
corresponds to the `--format` option command-line flag.

Default `Format`:
```
Format: "{{.Path}}:{{.Line}}:{{if .Col}}{{.Col}}{{end}}:{{.Severity}}: {{.Message}} ({{.Linter}})"
```

#### Format Methods

* `{{.Path.Relative}}` - equivalent to `{{.Path}}` which outputs a relative path to the file
* `{{.Path.Abs}}` - outputs an absolute path to the file

### Adding Custom linters

Linters can be added and customized from the config file using the `Linters` field.
Linters supports the following fields:

* `Command` - the path to the linter binary and any default arguments
* `Pattern` - a regular expression used to parse the linter output
* `IsFast` - if the linter should be run when the `--fast` flag is used
* `PartitionStrategy` - how paths args should be passed to the linter command:
  * `directories` - call the linter once with a list of all the directories
  * `files` - call the linter once with a list of all the files
  * `packages` - call the linter once with a list of all the package paths
  * `files-by-package` - call the linter once per package with a list of the
    files in the package.
  * `single-directory` - call the linter once per directory

The config for default linters can be overridden by using the name of the
linter.

Additional linters can be configured via the command line using the format
`NAME:COMMAND:PATTERN`.

Example:

```
$ gometalinter --linter='vet:go tool vet -printfuncs=Infof,Debugf,Warningf,Errorf:PATH:LINE:MESSAGE' .
```

## Comment directives

gometalinter supports suppression of linter messages via comment directives. The
form of the directive is:

```
// nolint[: <linter>[, <linter>, ...]]
```

Suppression works in the following way:

1. Line-level suppression

    A comment directive suppresses any linter messages on that line.

    eg. In this example any messages for `a := 10` will be suppressed and errcheck
    messages for `defer r.Close()` will also be suppressed.

    ```go
    a := 10 // nolint
    a = 2
    defer r.Close() // nolint: errcheck
    ```

2. Statement-level suppression

    A comment directive at the same indentation level as a statement it
    immediately precedes will also suppress any linter messages in that entire
    statement.

    eg. In this example all messages for `SomeFunc()` will be suppressed.

    ```go
    // nolint
    func SomeFunc() {
    }
    ```

Implementation details: gometalinter now performs parsing of Go source code,
to extract linter directives and associate them with line ranges. To avoid
unnecessary processing, parsing is on-demand: the first time a linter emits a
message for a file, that file is parsed for directives.

## Quickstart

Install gometalinter (see above).

Install all known linters:

```
$ gometalinter --install
Installing:
  structcheck
  maligned
  nakedret
  deadcode
  gocyclo
  ineffassign
  dupl
  golint
  gotype
  goimports
  errcheck
  varcheck
  interfacer
  goconst
  gosimple
  staticcheck
  unparam
  unused
  misspell
  lll
  gosec
  safesql
```

Run it:

```
$ cd example
$ gometalinter ./...
stutter.go:13::warning: unused struct field MyStruct.Unused (structcheck)
stutter.go:9::warning: unused global variable unusedGlobal (varcheck)
stutter.go:12:6:warning: exported type MyStruct should have comment or be unexported (golint)
stutter.go:16:6:warning: exported type PublicUndocumented should have comment or be unexported (golint)
stutter.go:8:1:warning: unusedGlobal is unused (deadcode)
stutter.go:12:1:warning: MyStruct is unused (deadcode)
stutter.go:16:1:warning: PublicUndocumented is unused (deadcode)
stutter.go:20:1:warning: duplicateDefer is unused (deadcode)
stutter.go:21:15:warning: error return value not checked (defer a.Close()) (errcheck)
stutter.go:22:15:warning: error return value not checked (defer a.Close()) (errcheck)
stutter.go:27:6:warning: error return value not checked (doit()           // test for errcheck) (errcheck)
stutter.go:29::error: unreachable code (vet)
stutter.go:26::error: missing argument for Printf("%d"): format reads arg 1, have only 0 args (vet)
```


Gometalinter also supports the commonly seen `<path>/...` recursive path
format. Note that this can be *very* slow, and you may need to increase the linter `--deadline` to allow linters to complete.

## FAQ

### Exit status

gometalinter sets two bits of the exit status to indicate different issues:

| Bit | Meaning
|-----|----------
| 0   | A linter generated an issue.
| 1   | An underlying error occurred; eg. a linter failed to execute. In this situation a warning will also be displayed.

eg. linter only = 1, underlying only = 2, linter + underlying = 3

### What's the best way to use `gometalinter` in CI?

There are two main problems running in a CI:

1. <s>Linters break, causing `gometalinter --install --update` to error</s> (this is no longer an issue as all linters are vendored).
2. `gometalinter` adds a new linter.

I have solved 1 by vendoring the linters.

For 2, the best option is to disable all linters, then explicitly enable the
ones you want:

    gometalinter --disable-all --enable=errcheck --enable=vet --enable=vetshadow ...

### How do I make `gometalinter` work with Go 1.5 vendoring?

`gometalinter` has a `--vendor` flag that just sets `GO15VENDOREXPERIMENT=1`, however the
underlying tools must support it. Ensure that all of the linters are up to date and built with Go 1.5
(`gometalinter --install --force`) then run `gometalinter --vendor .`. That should be it.

### Why does `gometalinter --install` install a fork of gocyclo?

I forked `gocyclo` because the upstream behaviour is to recursively check all
subdirectories even when just a single directory is specified. This made it
unusably slow when vendoring. The recursive behaviour can be achieved with
gometalinter by explicitly specifying `<path>/...`. There is a
[pull request](https://github.com/fzipp/gocyclo/pull/1) open.

### Many unexpected errors are being reported

If you see a whole bunch of errors being reported that you wouldn't expect,
such as compile errors, this typically means that something is wrong with your
Go environment. Try `go install` and fix any issues with your go installation,
then try gometalinter again.

### Gometalinter is not working

That's more of a statement than a question, but okay.

Sometimes gometalinter will not report issues that you think it should. There
are three things to try in that case:

#### 1. Update to the latest build of gometalinter and all linters

    curl https://git.io/vp6lP | sh

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

#### 3. Report an issue.

Failing all else, if the problem looks like a bug please file an issue and
include the output of `gometalinter --debug`.

### How do I filter issues between two git refs?

[revgrep](https://github.com/bradleyfalzon/revgrep) can be used to filter the output of `gometalinter`
to show issues on lines that have changed between two git refs, such as unstaged changes, changes in
`HEAD` vs `master` and between `master` and `origin/master`. See the project's documentation and `-help`
usage for more information.

```
go get -u github.com/bradleyfalzon/revgrep/...
gometalinter |& revgrep               # If unstaged changes or untracked files, those issues are shown.
gometalinter |& revgrep               # Else show issues in the last commit.
gometalinter |& revgrep master        # Show issues between master and HEAD (or any other reference).
gometalinter |& revgrep origin/master # Show issues that haven't been pushed.
```

## Checkstyle XML format

`gometalinter` supports [checkstyle](http://checkstyle.sourceforge.net/)
compatible XML output format. It is triggered with `--checkstyle` flag:

	gometalinter --checkstyle

Checkstyle format can be used to integrate gometalinter with Jenkins CI with the
help of [Checkstyle Plugin](https://wiki.jenkins-ci.org/display/JENKINS/Checkstyle+Plugin).
