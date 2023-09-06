// ref: NIST SP 800-108 5.1

package smb2

import (
	"crypto/hmac"
	"crypto/sha256"
)

// KDF in Counter Mode with h = 256, r = 32, L = 128
func kdf(ki, label, context []byte) []byte {
	h := hmac.New(sha256.New, ki)

	h.Write([]byte{0x00, 0x00, 0x00, 0x01})
	h.Write(label)
	h.Write([]byte{0x00})
	h.Write(context)
	h.Write([]byte{0x00, 0x00, 0x00, 0x80})

	return h.Sum(nil)[:16]
}
