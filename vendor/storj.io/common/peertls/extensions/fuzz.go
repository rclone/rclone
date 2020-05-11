// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build gofuzz

package extensions

// To run fuzzing tests:
//
// clone github.com/storj/fuzz-corpus
//
// Install fuzzing tools:
//   GO111MODULE=off go get github.com/dvyukov/go-fuzz/...
//
// Build binaries:
//   go-fuzz-build .
//
// Run with test corpus:
//   go-fuzz -bin extensions-fuzz.zip -workdir $FUZZCORPUS/peertls/extensions

// Fuzz implements a simple fuzz test for revocationDecoder.
func Fuzz(data []byte) int {
	var dec revocationDecoder
	_, err := dec.decode(data)
	if err != nil {
		return 0
	}
	return 1
}
