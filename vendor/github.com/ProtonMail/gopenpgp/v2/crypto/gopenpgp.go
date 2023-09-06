// Package crypto provides a high-level API for common OpenPGP functionality.
package crypto

import "sync"

// GopenPGP is used as a "namespace" for many of the functions in this package.
// It is a struct that keeps track of time skew between server and client.
type GopenPGP struct {
	latestServerTime int64
	generationOffset int64
	lock             *sync.RWMutex
}

var pgp = GopenPGP{
	latestServerTime: 0,
	generationOffset: 0,
	lock:             &sync.RWMutex{},
}

// clone returns a clone of the byte slice. Internal function used to make sure
// we don't retain a reference to external data.
func clone(input []byte) []byte {
	data := make([]byte, len(input))
	copy(data, input)
	return data
}
