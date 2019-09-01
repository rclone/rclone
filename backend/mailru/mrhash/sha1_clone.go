// +build go1.10

package mrhash

import (
	"crypto/sha1"
	"encoding"
	"hash"
)

// Make a clone of SHA1 hash
func cloneSHA1(orig hash.Hash) (clone hash.Hash, err error) {
	state, err := orig.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	clone = sha1.New()
	err = clone.(encoding.BinaryUnmarshaler).UnmarshalBinary(state)
	return
}
