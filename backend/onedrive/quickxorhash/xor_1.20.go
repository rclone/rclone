//go:build 1.20 || 1.21 || 1.22

package quickxorhash

import "crypto/subtle"

func xorBytes(dst, src []byte) int {
	return subtle.XORBytes(dst, src, dst)
}
