# Flect

<p align="center">
<a href="https://godoc.org/github.com/gobuffalo/flect"><img src="https://godoc.org/github.com/gobuffalo/flect?status.svg" alt="GoDoc" /></a>
<a href="https://travis-ci.org/gobuffalo/flect"><img src="https://travis-ci.org/gobuffalo/flect.svg?branch=master" alt="Build Status" /></a>
<a href="https://goreportcard.com/report/github.com/gobuffalo/flect"><img src="https://goreportcard.com/badge/github.com/gobuffalo/flect" alt="Go Report Card" /></a>
</p>

This is a new inflection engine to replace [https://github.com/markbates/inflect](https://github.com/markbates/inflect) designed to be more modular, more readable, and easier to fix issues on than the original.

## Installation

```bash
$ go get -u -v github.com/gobuffalo/flect
```

## `github.com/gobuffalo/flect`
<a href="https://godoc.org/github.com/gobuffalo/flect"><img src="https://godoc.org/github.com/gobuffalo/flect?status.svg" alt="GoDoc" /></a>

The `github.com/gobuffalo/flect` package contains "basic" inflection tools, like pluralization, singularization, etc...

### The `Ident` Type

In addition to helpful methods that take in a `string` and return a `string`, there is an `Ident` type that can be used to create new, custom, inflection rules.

The `Ident` type contains two fields.

* `Original` - This is the original `string` that was used to create the `Ident`
* `Parts` - This is a `[]string` that represents all of the "parts" of the string, that have been split apart, making the segments easier to work with

Examples of creating new inflection rules using `Ident` can be found in the `github.com/gobuffalo/flect/name` package.

## `github.com/gobuffalo/flect/name`
<a href="https://godoc.org/github.com/gobuffalo/flect/name"><img src="https://godoc.org/github.com/gobuffalo/flect/name?status.svg" alt="GoDoc" /></a>

The `github.com/gobuffalo/flect/name` package contains more "business" inflection rules like creating proper names, table names, etc...
