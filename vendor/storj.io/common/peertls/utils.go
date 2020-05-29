// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

// Many cryptography standards use ASN.1 to define their data structures,
// and Distinguished Encoding Rules (DER) to serialize those structures.
// Because DER produces binary output, it can be challenging to transmit
// the resulting files through systems, like electronic mail, that only
// support ASCII. The PEM format solves this problem by encoding the
// binary data using base64.
// (see https://en.wikipedia.org/wiki/Privacy-enhanced_Electronic_Mail)

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"math/big"

	"github.com/zeebo/errs"

	"storj.io/common/pkcrypto"
)

// NonTemporaryError is an error with a `Temporary` method which always returns false.
// It is intended for use with grpc.
//
// (see https://godoc.org/google.golang.org/grpc#WithDialer
// and https://godoc.org/google.golang.org/grpc#FailOnNonTempDialError).
type NonTemporaryError struct{ error }

// NewNonTemporaryError returns a new temporary error for use with grpc.
func NewNonTemporaryError(err error) NonTemporaryError {
	return NonTemporaryError{
		error: errs.Wrap(err),
	}
}

// DoubleSHA256PublicKey returns the hash of the hash of (double-hash, SHA226)
// the binary format of the given public key.
func DoubleSHA256PublicKey(k crypto.PublicKey) ([sha256.Size]byte, error) {
	kb, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	mid := sha256.Sum256(kb)
	end := sha256.Sum256(mid[:])
	return end, nil
}

// Temporary returns false to indicate that is is a non-temporary error.
func (nte NonTemporaryError) Temporary() bool {
	return false
}

// Err returns the underlying error.
func (nte NonTemporaryError) Err() error {
	return nte.error
}

func verifyChainSignatures(certs []*x509.Certificate) error {
	for i, cert := range certs {
		j := len(certs)
		if i+1 < j {
			err := verifyCertSignature(certs[i+1], cert)
			if err != nil {
				return ErrVerifyCertificateChain.Wrap(err)
			}

			continue
		}

		err := verifyCertSignature(cert, cert)
		if err != nil {
			return ErrVerifyCertificateChain.Wrap(err)
		}

	}

	return nil
}

func verifyCertSignature(parentCert, childCert *x509.Certificate) error {
	return pkcrypto.HashAndVerifySignature(parentCert.PublicKey, childCert.RawTBSCertificate, childCert.Signature)
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
