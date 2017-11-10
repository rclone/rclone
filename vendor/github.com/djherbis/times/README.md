times 
==========

[![GoDoc](https://godoc.org/github.com/djherbis/times?status.svg)](https://godoc.org/github.com/djherbis/times)
[![Release](https://img.shields.io/github/release/djherbis/times.svg)](https://github.com/djherbis/times/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE.txt)
[![Build Status](https://travis-ci.org/djherbis/times.svg?branch=master)](https://travis-ci.org/djherbis/times)
[![Coverage Status](https://coveralls.io/repos/djherbis/times/badge.svg?branch=master)](https://coveralls.io/r/djherbis/times?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/djherbis/times)](https://goreportcard.com/report/github.com/djherbis/times)

Usage
------------
File Times for #golang

Go has a hidden time functions for most platforms, this repo makes them accessible.

```go
package main

import (
  "log"

  "github.com/djherbis/times"
)

func main() {
  t, err := times.Stat("myfile")
  if err != nil {
    log.Fatal(err.Error())
  }

  log.Println(t.AccessTime())
  log.Println(t.ModTime())

  if t.HasChangeTime() {
    log.Println(t.ChangeTime())
  }

  if t.HasBirthTime() {
    log.Println(t.BirthTime())
  }
}
```

Supported Times
------------
|  | windows | linux | solaris | dragonfly | nacl | freebsd | darwin | netbsd | openbsd | plan9 |
|:-----:|:-------:|:-----:|:-------:|:---------:|:------:|:-------:|:----:|:------:|:-------:|:-----:|
| atime | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| mtime | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| ctime | ✓* | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |
| btime | ✓ |  |  |  |  | ✓ |  ✓| ✓ |  

* Windows XP does not have ChangeTime so HasChangeTime = false, 
however Vista onward does have ChangeTime so Timespec.HasChangeTime() will 
only return false on those platforms when the syscall used to obtain them fails.
* Also note, Get(FileInfo) will now only return values available in FileInfo.Sys(), this means Stat() is required to get ChangeTime on Windows

Installation
------------
```sh
go get github.com/djherbis/times
```
