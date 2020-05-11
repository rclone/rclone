// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"math/big"
	"reflect"
)

const (
	// StorjPSSSaltLength holds the correct value for the PSS salt length
	// when signing with RSA in Storj code and verifying RSA signatures
	// from Storj.
	StorjPSSSaltLength = rsa.PSSSaltLengthAuto

	// StorjRSAKeyBits holds the number of bits to use for new RSA keys
	// by default.
	StorjRSAKeyBits = 2048
)

var (
	authECCurve = elliptic.P256()

	pssParams = rsa.PSSOptions{
		SaltLength: StorjPSSSaltLength,
		Hash:       crypto.SHA256,
	}
)

// GeneratePrivateKey returns a new PrivateKey for signing messages
func GeneratePrivateKey() (crypto.PrivateKey, error) {
	return GeneratePrivateECDSAKey(authECCurve)
	// return GeneratePrivateRSAKey(StorjRSAKeyBits)
}

// GeneratePrivateECDSAKey returns a new private ECDSA key for signing messages
func GeneratePrivateECDSAKey(curve elliptic.Curve) (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(curve, rand.Reader)
}

// GeneratePrivateRSAKey returns a new private RSA key for signing messages
func GeneratePrivateRSAKey(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// HashAndVerifySignature checks that signature was made by the private key
// corresponding to the given public key, over a SHA-256 digest of the given
// data. It returns an error if verification fails, or nil otherwise.
func HashAndVerifySignature(key crypto.PublicKey, data, signature []byte) error {
	digest := SHA256Hash(data)
	return VerifySignatureWithoutHashing(key, digest, signature)
}

// VerifySignatureWithoutHashing checks the signature against the passed data
// (which is normally a digest) and public key. It returns an error if
// verification fails, or nil otherwise.
func VerifySignatureWithoutHashing(pubKey crypto.PublicKey, digest, signature []byte) error {
	switch key := pubKey.(type) {
	case *ecdsa.PublicKey:
		return verifyECDSASignatureWithoutHashing(key, digest, signature)
	case *rsa.PublicKey:
		return verifyRSASignatureWithoutHashing(key, digest, signature)
	}
	return ErrUnsupportedKey.New("%T", pubKey)
}

func verifyECDSASignatureWithoutHashing(pubKey *ecdsa.PublicKey, digest, signatureBytes []byte) error {
	r, s, err := unmarshalECDSASignature(signatureBytes)
	if err != nil {
		return ErrVerifySignature.New("unable to unmarshal ecdsa signature: %v", err)
	}
	if !ecdsa.Verify(pubKey, digest, r, s) {
		return ErrVerifySignature.New("signature is not valid")
	}
	return nil
}

func verifyRSASignatureWithoutHashing(pubKey *rsa.PublicKey, digest, signatureBytes []byte) error {
	err := rsa.VerifyPSS(pubKey, pssParams.Hash, digest, signatureBytes, &pssParams)
	if err != nil {
		return ErrVerifySignature.New("signature is not valid")
	}
	return nil
}

// PublicKeyFromPrivate returns the public key corresponding to a given private
// key.
// It returns an error if the key isn't of an accepted implementation.
func PublicKeyFromPrivate(privKey crypto.PrivateKey) (crypto.PublicKey, error) {
	switch key := privKey.(type) {
	case *ecdsa.PrivateKey:
		return key.Public(), nil
	case *rsa.PrivateKey:
		return key.Public(), nil
	}
	return nil, ErrUnsupportedKey.New("%T", privKey)
}

// SignWithoutHashing signs the given digest with the private key and returns
// the new signature.
func SignWithoutHashing(privKey crypto.PrivateKey, digest []byte) ([]byte, error) {
	switch key := privKey.(type) {
	case *ecdsa.PrivateKey:
		return signECDSAWithoutHashing(key, digest)
	case *rsa.PrivateKey:
		return signRSAWithoutHashing(key, digest)
	}
	return nil, ErrUnsupportedKey.New("%T", privKey)
}

func signECDSAWithoutHashing(privKey *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, privKey, digest)
	if err != nil {
		return nil, ErrSign.Wrap(err)
	}
	return marshalECDSASignature(r, s)
}

func signRSAWithoutHashing(privKey *rsa.PrivateKey, digest []byte) ([]byte, error) {
	return privKey.Sign(rand.Reader, digest, &pssParams)
}

// HashAndSign signs a SHA-256 digest of the given data and returns the new
// signature.
func HashAndSign(key crypto.PrivateKey, data []byte) ([]byte, error) {
	digest := SHA256Hash(data)
	signature, err := SignWithoutHashing(key, digest)
	if err != nil {
		return nil, ErrSign.Wrap(err)
	}
	return signature, nil
}

// PublicKeyEqual returns true if two public keys are the same.
func PublicKeyEqual(a, b crypto.PublicKey) bool {
	switch aConcrete := a.(type) {
	case *ecdsa.PublicKey:
		bConcrete, ok := b.(*ecdsa.PublicKey)
		if !ok {
			return false
		}
		return publicECDSAKeyEqual(aConcrete, bConcrete)
	case *rsa.PublicKey:
		bConcrete, ok := b.(*rsa.PublicKey)
		if !ok {
			return false
		}
		return publicRSAKeyEqual(aConcrete, bConcrete)
	}
	// a best-effort here is probably better than adding an err return
	return reflect.DeepEqual(a, b)
}

// publicECDSAKeyEqual returns true if two ECDSA public keys are the same.
func publicECDSAKeyEqual(a, b *ecdsa.PublicKey) bool {
	return a.Curve == b.Curve && bigIntEq(a.X, b.X) && bigIntEq(a.Y, b.Y)
}

// publicRSAKeyEqual returns true if two RSA public keys are the same.
func publicRSAKeyEqual(a, b *rsa.PublicKey) bool {
	return bigIntEq(a.N, b.N) && a.E == b.E
}

func bigIntEq(a, b *big.Int) bool {
	return a.Cmp(b) == 0
}
