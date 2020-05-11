// Copyright (C) 2014 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package errors is a flexible error support library for Go

Motivation

Go's standard library is intentionally sparse on providing error utilities, and
developers coming from other programming languages may miss some features they
took for granted [1]. This package is an attempt at providing those features in
an idiomatic Go way.

The main features this package provides (in addition to miscellaneous
utilities) are:

 * Error hierarchies
 * Stack traces
 * Arbitrary error values

Error hierarchies

While Go has very deliberately not implemented class hierarchies, a quick
perusal of Go's net and os packages should indicate that sometimes error
hierarchies are useful. Go programmers should be familiar with the net.Error
interface (and the types that fulfill it) as well as the os helper functions
such as os.IsNotExist, os.IsPermission, etc.

Unfortunately, to implement something similar, a developer will have to
implement a struct that matches the error interface as well as any testing
methods or any more detailed interfaces they may choose to export. It's not
hard, but it is friction, and developers tend to use fmt.Errorf instead due
to ease of use, thus missing out on useful features that functions like
os.IsNotExist and friends provide.

The errors package provides reusable components for building similar
features while reducing friction as much as possible. With the errors package,
the os error handling routines can be mimicked as follows:

  package osmimic

  import (
    "github.com/spacemonkeygo/errors"
  )

  var (
    OSError = errors.NewClass("OS Error")
    NotExist = OSError.NewClass("Not Exist")
  )

  func Open(path string) (*File, error) {
    // actually do something here
    return nil, NotExist.New("path %#v doesn't exist", path)
  }

  func MyMethod() error {
    fh, err := Open(mypath)
    if err != nil {
      if NotExist.Contains(err) {
        // file doesn't exist, do stuff
      }
      return err
    }
    // do stuff
  }

Stack traces

It doesn't take long during Go development before you may find yourself
wondering where an error came from. In other languages, as soon as an error is
raised, a stack trace is captured and is displayed as part of the language's
error handling. Go error types are simply basic values and no such magic
happens to tell you what line or what stack an error came from.

The errors package fixes this by optionally (but by default) capturing the
stack trace as part of your error. This behavior can be turned off and on for
specific error classes and comes in two flavors. You can have the stack trace
be appended to the error's Error() message, or you can have the stack trace
be logged immediately, every time an error of that type is instantiated.

Every error and error class supports hierarchical settings, in the sense that
if a setting was not explicitly set on that error or error class, setting
resolution traverses the error class hierarchy until it finds a valid setting,
or returns the default.

See CaptureStack()/NoCaptureStack() and LogOnCreation()/NoLogOnCreation() for
how to control this feature.

Arbitrary error values

These hierarchical settings (for whether or not errors captured or logged stack
traces) were so useful, we generalized the system to allow users to extend
the errors system with their own values. A user can tag a specific error with
some value given a statically defined key, or tag a whole error class subtree.

Arbitrary error values can easily handle situtations like net.Error's
Temporary() field, where some errors are temporary and others aren't. This can
be mimicked as follows:

  package netmimic

  import (
    "github.com/spacemonkeygo/errors"
  )

  var (
    NetError = errors.NewClass("Net Error")
    OpError = NetError.NewClass("Op Error")

    tempErrorKey = errors.GenSym()
  )

  func SetIsTemporary() errors.ErrorOption {
    return errors.SetData(tempErrorKey, true)
  }

  func IsTemporary(err error) bool {
    v, ok := errors.GetData(err, tempErrorKey).(bool)
    if !ok {
      return false
    }
    return v
  }

  func NetworkOp() error {
    // actually do something here
    return OpError.NewWith("failed operation", SetIsTemporary())
  }

  func Example() error {
    for {
      err := NetworkOp()
      if err != nil {
        if IsTemporary(err) {
          // probably should do exponential backoff
          continue
        }
        return err
      }
    }
  }

HTTP handling

Another great example of arbitrary error value functionality is the errhttp
subpackage. See the errhttp source for more examples of how to use
SetData/GetData.

The errhttp package really helped clean up our error code. Take a look to
see if it can help your error handling with HTTP stacks too.

http://godoc.org/github.com/spacemonkeygo/errors/errhttp

Exit recording

So you have stack traces, which tells you how the error was generated, but
perhaps you're interested in keeping track of how the error was handled?

Every time you call errors.Record(err), it adds the current line information
to the error's output. As an example:

  func MyFunction() error {
    err := Something()
    if err != nil {
      if IsTemporary(err) {
        // manage the temporary error
        return errors.Record(err)
      } else {
        // manage the permanent error
        return errors.Record(err)
      }
    }
  }

errors.Record will help you keep track of which error handling branch your
code took.

ErrorGroup

There's a few different types of ErrorGroup utilities in this package, but they
all work the same way. Make sure to check out the ErrorGroup example.

CatchPanic

CatchPanic helps you easily manage functions that you think might panic, and
instead return errors. CatchPanic works by taking a pointer to your named error
return value. Check out the CatchPanic example for more.

Footnotes

[1] This errors package started while porting a large Python codebase to Go.
https://www.spacemonkey.com/blog/posts/go-space-monkey
*/
package errors
