# Cross-platform FUSE library for Go

[![Travis CI](https://img.shields.io/travis/billziss-gh/cgofuse.svg?label=osx/linux)](https://travis-ci.org/billziss-gh/cgofuse)
[![AppVeyor](https://img.shields.io/appveyor/ci/billziss-gh/cgofuse.svg?label=windows)](https://ci.appveyor.com/project/billziss-gh/cgofuse)
[![CircleCI](https://img.shields.io/circleci/project/github/billziss-gh/cgofuse.svg?label=cross-build)](https://circleci.com/gh/billziss-gh/cgofuse)
[![GoDoc](https://godoc.org/github.com/billziss-gh/cgofuse/fuse?status.svg)](https://godoc.org/github.com/billziss-gh/cgofuse/fuse)

Cgofuse is a cross-platform FUSE library for Go. It is implemented using [cgo](https://golang.org/cmd/cgo/) and can be ported to any platform that has a FUSE implementation.

Cgofuse currently runs on **OSX**, **Linux** and **Windows** (using [WinFsp](https://github.com/billziss-gh/winfsp)).

## How to build

**OSX**
- Prerequisites: [OSXFUSE](https://osxfuse.github.io), [command line tools](https://developer.apple.com/library/content/technotes/tn2339/_index.html)
- Build:
    ```
    $ cd cgofuse
    $ go install -v ./fuse ./examples/memfs ./examples/passthrough
    ```

**Linux**
- Prerequisites: libfuse-dev, gcc
- Build:
    ```
    $ cd cgofuse
    $ go install -v ./fuse ./examples/memfs ./examples/passthrough
    ```
**Windows**
- Prerequisites: [WinFsp](https://github.com/billziss-gh/winfsp), gcc (e.g. from [Mingw-builds](http://mingw-w64.org/doku.php/download))
- Build:
    ```
    > cd cgofuse
    > set CPATH=C:\Program Files (x86)\WinFsp\inc\fuse
    > go install -v ./fuse ./examples/memfs
    ```

## How to cross-compile your project using xgo

You can easily cross-compile your project using [xgo](https://github.com/karalabe/xgo) and the [billziss/xgo-cgofuse](https://hub.docker.com/r/billziss/xgo-cgofuse/) docker image.

- Prerequisites: [docker](https://www.docker.com), [xgo](https://github.com/karalabe/xgo)
- Build:
    ```
    $ docker pull billziss/xgo-cgofuse
    $ go get -u github.com/karalabe/xgo
    $ cd YOUR-PROJECT-THAT-USES-CGOFUSE
    $ xgo --image=billziss/xgo-cgofuse \
        --targets=darwin/386,darwin/amd64,linux/386,linux/amd64,windows/386,windows/amd64 .
    ```

## How to use

User mode file systems are expected to implement `fuse.FileSystemInterface`. To make implementation simpler a file system can embed ("inherit") a `fuse.FileSystemBase` which provides default implementations for all operations. To mount a file system one must instantiate a `fuse.FileSystemHost` using `fuse.NewFileSystemHost`.

The full documentation is available at GoDoc.org: [package fuse](https://godoc.org/github.com/billziss-gh/cgofuse/fuse)

There are currently three example file systems:

- [Hellofs](examples/hellofs/hellofs.go) is an extremely simple file system. Runs on OSX, Linux and Windows.
- [Memfs](examples/memfs/memfs.go) is an in memory file system. Runs on OSX, Linux and Windows.
- [Passthrough](examples/passthrough/passthrough.go) is a file system that passes all operations to the underlying file system. Runs on OSX, Linux.

## How it is tested

Cgofuse is regularly built and tested on [Travis CI](https://travis-ci.org/billziss-gh/cgofuse) and [AppVeyor](https://ci.appveyor.com/project/billziss-gh/cgofuse). The following software is being used to test cgofuse.

**OSX/Linux**
- [fstest](https://github.com/billziss-gh/secfs.test/tree/master/fstest/ntfs-3g-pjd-fstest-8af5670)
- [fsx](https://github.com/billziss-gh/secfs.test/tree/master/fstools/src/fsx)

**Windows**
- [winfsp-tests](https://github.com/billziss-gh/winfsp/tree/master/tst/winfsp-tests)

## Contributors

- Bill Zissimopoulos \<billziss at navimatics.com>
- Nick Craig-Wood \<nick at craig-wood.com>
