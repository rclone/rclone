// +build !go1.10

package mrhash

import (
	"hash"
	"reflect"
)

// Make a clone of SHA1 hash
func cloneSHA1(orig hash.Hash) (clone hash.Hash, err error) {
	digestValue := reflect.ValueOf(orig).Elem()
	clonePtr := reflect.New(digestValue.Type())
	clonePtr.Elem().Set(digestValue)
	clone = clonePtr.Interface().(hash.Hash)
	err = nil
	return
}
