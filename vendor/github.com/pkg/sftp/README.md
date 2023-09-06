sftp
----

The `sftp` package provides support for file system operations on remote ssh
servers using the SFTP subsystem. It also implements an SFTP server for serving
files from the filesystem.

![CI Status](https://github.com/pkg/sftp/workflows/CI/badge.svg?branch=master&event=push) [![Go Reference](https://pkg.go.dev/badge/github.com/pkg/sftp.svg)](https://pkg.go.dev/github.com/pkg/sftp)

usage and examples
------------------

See [https://pkg.go.dev/github.com/pkg/sftp](https://pkg.go.dev/github.com/pkg/sftp) for
examples and usage.

The basic operation of the package mirrors the facilities of the
[os](http://golang.org/pkg/os) package.

The Walker interface for directory traversal is heavily inspired by Keith
Rarick's [fs](https://pkg.go.dev/github.com/kr/fs) package.

roadmap
-------

* There is way too much duplication in the Client methods. If there was an
  unmarshal(interface{}) method this would reduce a heap of the duplication.

contributing
------------

We welcome pull requests, bug fixes and issue reports.

Before proposing a large change, first please discuss your change by raising an
issue.

For API/code bugs, please include a small, self contained code example to
reproduce the issue. For pull requests, remember test coverage.

We try to handle issues and pull requests with a 0 open philosophy. That means
we will try to address the submission as soon as possible and will work toward
a resolution. If progress can no longer be made (eg. unreproducible bug) or
stops (eg. unresponsive submitter), we will close the bug.

Thanks.
