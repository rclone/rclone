package pkcs8

import (
	"crypto/aes"
	"encoding/asn1"
)

var (
	oidAES128CBC = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 2}
	oidAES256CBC = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
)

func init() {
	RegisterCipher(oidAES128CBC, func() Cipher {
		return AES128CBC
	})
	RegisterCipher(oidAES256CBC, func() Cipher {
		return AES256CBC
	})
}

// AES128CBC is the 128-bit key AES cipher in CBC mode.
var AES128CBC = cipherWithBlock{
	ivSize:   aes.BlockSize,
	keySize:  16,
	newBlock: aes.NewCipher,
	oid:      oidAES128CBC,
}

// AES256CBC is the 256-bit key AES cipher in CBC mode.
var AES256CBC = cipherWithBlock{
	ivSize:   aes.BlockSize,
	keySize:  32,
	newBlock: aes.NewCipher,
	oid:      oidAES256CBC,
}
