# Token Bucket (tb) [![Build Status](https://secure.travis-ci.org/tsenart/tb.png)](http://travis-ci.org/tsenart/tb) [![GoDoc](https://godoc.org/github.com/tsenart/tb?status.png)](https://godoc.org/github.com/tsenart/tb)

This package provides a generic lock-free implementation of the "Token bucket"
algorithm where handling of non-conformity is left to the user.


> The token bucket is an algorithm used in packet switched computer networks and telecommunications networks. It can be used to check that data transmissions, in the form of packets, conform to defined limits on bandwidth and burstiness (a measure of the unevenness or variations in the traffic flow)
-- <cite>[Wikipedia](http://en.wikipedia.org/wiki/Token_bucket)</cite>

This implementation of the token bucket generalises its applications beyond packet rate conformance. Hence, the word *generic*. You can use it to throttle any flow over time as long as it can be expressed as a number (bytes/s, requests/s, messages/s, packets/s, potatoes/s, heartbeats/s, etc...).

The *lock-free* part of the description refers to the lock-free programming techniques (CAS loop) used in the core `Bucket` methods (`Take` and `Put`). [Here is](http://preshing.com/20120612/an-introduction-to-lock-free-programming/) a good overview of lock-free programming you can refer to.

All utility pacakges such as [http](http/) and [io](io/) are just wrappers around the core package.
This ought to be your one stop shop for all things **throttling** in Go so feel free to propose missing common functionality.



## Install
```shell
$ go get github.com/tsenart/tb
```

## Usage
Read up the [docs](https://godoc.org/github.com/tsenart/tb) and have a look at some [examples](examples/).

## Licence
```
The MIT License (MIT)

Copyright (c) 2014-2015 Tomás Senart

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```
