//go:build go1.20

package quickxorhash

import "crypto/subtle"

func xorBytes(dst, src []byte) int {
	return subtle.XORBytes(dst, src, dst)
}
