[![GoDoc](https://godoc.org/github.com/pkg/xattr?status.svg)](http://godoc.org/github.com/pkg/xattr)
[![Go Report Card](https://goreportcard.com/badge/github.com/pkg/xattr)](https://goreportcard.com/report/github.com/pkg/xattr)
[![Build Status](https://github.com/pkg/xattr/workflows/build/badge.svg)](https://github.com/pkg/xattr/actions?query=workflow%3Abuild)
[![Codecov](https://codecov.io/gh/pkg/xattr/branch/master/graph/badge.svg)](https://codecov.io/gh/pkg/xattr)

xattr
=====
Extended attribute support for Go (linux + darwin + freebsd + netbsd + solaris).

"Extended attributes are name:value pairs associated permanently with files and directories, similar to the environment strings associated with a process. An attribute may be defined or undefined. If it is defined, its value may be empty or non-empty." [See more...](https://en.wikipedia.org/wiki/Extended_file_attributes)

`SetWithFlags` allows to additionally pass system flags to be forwarded to the underlying calls. FreeBSD and NetBSD do not support this and the parameter will be ignored.

The `L` variants of all functions (`LGet/LSet/...`) are identical to `Get/Set/...` except that they
do not reference a symlink that appears at the end of a path. See
[GoDoc](http://godoc.org/github.com/pkg/xattr) for details.

### Example
```go
  const path = "/tmp/myfile"
  const prefix = "user."

  if err := xattr.Set(path, prefix+"test", []byte("test-attr-value")); err != nil {
  	log.Fatal(err)
  }

  var list []string
  if list, err = xattr.List(path); err != nil {
  	log.Fatal(err)
  }

  var data []byte
  if data, err = xattr.Get(path, prefix+"test"); err != nil {
  	log.Fatal(err)
  }

  if err = xattr.Remove(path, prefix+"test"); err != nil {
  	log.Fatal(err)
  }

  // One can also specify the flags parameter to be passed to the OS.
  if err := xattr.SetWithFlags(path, prefix+"test", []byte("test-attr-value"), xattr.XATTR_CREATE); err != nil {
  	log.Fatal(err)
  }
```
