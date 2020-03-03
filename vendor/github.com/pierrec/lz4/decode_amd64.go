// +build !appengine
// +build gc
// +build !noasm

package lz4

//go:noescape
func decodeBlock(dst, src []byte) int
