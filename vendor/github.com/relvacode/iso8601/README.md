A fast ISO8601 date parser for Go

[![GoDoc](https://godoc.org/github.com/relvacode/iso8601?status.svg)](https://godoc.org/github.com/relvacode/iso8601) ![Build Status](https://github.com/relvacode/iso8601/actions/workflows/verify.yml/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/relvacode/iso8601)](https://goreportcard.com/report/github.com/relvacode/iso8601)


```
go get github.com/relvacode/iso8601
```

The built-in RFC3333 time layout in Go is too restrictive to support any ISO8601 date-time.

This library parses any ISO8601 date into a native Go time object without regular expressions.

## Usage

```go
package main

import "github.com/relvacode/iso8601"

// iso8601.Time can be used as a drop-in replacement for time.Time with JSON responses
type ExternalAPIResponse struct {
	Timestamp *iso8601.Time
}


func main() {
	// iso8601.ParseString can also be called directly
	t, err := iso8601.ParseString("2020-01-02T16:20:00")
}
```

## Benchmark

```
BenchmarkParse-16        	13364954	        77.7 ns/op	       0 B/op	       0 allocs/op
```

## Release History

  - `1.3.0`

  Allow a leading `+` sign in the year component [#11](https://github.com/relvacode/iso8601/issues/11)
  - `1.2.0` 
  
  Time range validity checking equivalent to the standard library.
  Note that previous versions would not validate that a given date string was in the expected range. Additionally, this version no longer accepts `0000-00-00T00:00:00` as a valid input which can be the zero time representation in other languages nor does it support leap seconds (such that the seconds field is `60`) as is the case in the [standard library](https://github.com/golang/go/issues/15247)
  - `1.1.0` 
  
  Check for `-0` time zone
  - `1.0.0` 
  
  Initial release
