# xxhash [![GoDoc](https://godoc.org/github.com/OneOfOne/xxhash?status.svg)](https://godoc.org/github.com/OneOfOne/xxhash) [![Build Status](https://travis-ci.org/OneOfOne/xxhash.svg?branch=master)](https://travis-ci.org/OneOfOne/xxhash) [![Coverage](https://gocover.io/_badge/github.com/OneOfOne/xxhash)](https://gocover.io/github.com/OneOfOne/xxhash)

This is a native Go implementation of the excellent [xxhash](https://github.com/Cyan4973/xxHash)* algorithm, an extremely fast non-cryptographic Hash algorithm, working at speeds close to RAM limits.

* The C implementation is ([Copyright](https://github.com/Cyan4973/xxHash/blob/master/LICENSE) (c) 2012-2014, Yann Collet)

## Install

    go get github.com/OneOfOne/xxhash

## Features

* On Go 1.7+ the pure go version is faster than CGO for all inputs.
* Supports ChecksumString{32,64} xxhash{32,64}.WriteString, which uses no copies when it can, falls back to copy on appengine.
* The native version falls back to a less optimized version on appengine due to the lack of unsafe.
* Almost as fast as the mostly pure assembly version written by the brilliant [cespare](https://github.com/cespare/xxhash), while also supporting seeds.
* To manually toggle the appengine version build with `-tags safe`.

## Benchmark

### Core i7-4790 @ 3.60GHz, Linux 4.12.6-1-ARCH (64bit), Go tip (+ff90f4af66 2017-08-19)

```bash
➤ go test -bench '64' -count 5 -tags cespare | benchstat /dev/stdin
name                          time/op

# https://github.com/cespare/xxhash
XXSum64Cespare/Func-8          160ns ± 2%
XXSum64Cespare/Struct-8        173ns ± 1%
XXSum64ShortCespare/Func-8    6.78ns ± 1%
XXSum64ShortCespare/Struct-8  19.6ns ± 2%

# this package (default mode, using unsafe)
XXSum64/Func-8                 170ns ± 1%
XXSum64/Struct-8               182ns ± 1%
XXSum64Short/Func-8           13.5ns ± 3%
XXSum64Short/Struct-8         20.4ns ± 0%

# this package (appengine, *not* using unsafe)
XXSum64/Func-8                 241ns ± 5%
XXSum64/Struct-8               243ns ± 6%
XXSum64Short/Func-8           15.2ns ± 2%
XXSum64Short/Struct-8         23.7ns ± 5%

CRC64ISO-8                    1.23µs ± 1%
CRC64ISOString-8              2.71µs ± 4%
CRC64ISOShort-8               22.2ns ± 3%

Fnv64-8                       2.34µs ± 1%
Fnv64Short-8                  74.7ns ± 8%
#
```

## Usage

```go
	h := xxhash.New64()
	// r, err := os.Open("......")
	// defer f.Close()
	r := strings.NewReader(F)
	io.Copy(h, r)
	fmt.Println("xxhash.Backend:", xxhash.Backend)
	fmt.Println("File checksum:", h.Sum64())
```

[<kbd>playground</kbd>](http://play.golang.org/p/rhRN3RdQyd)

## TODO

* Rewrite the 32bit version to be more optimized.
* General cleanup as the Go inliner gets smarter.

## License

This project is released under the Apache v2. licence. See [LICENCE](LICENCE) for more details.
