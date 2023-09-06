// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"github.com/zeebo/errs"
)

const (
	// BlockLabelEcPrivateKey is the value to define a block label of EC private key
	// (which is used here only for backwards compatibility). Use a general PKCS#8
	// encoding instead.
	BlockLabelEcPrivateKey = "EC PRIVATE KEY"
	// BlockLabelPrivateKey is the value to define a block label of general private key
	// (used for PKCS#8-encoded private keys of type RSA, ECDSA, and others).
	BlockLabelPrivateKey = "PRIVATE KEY"
	// BlockLabelPublicKey is the value to define a block label of general public key
	// (used for PKIX-encoded public keys of type RSA, ECDSA, and others).
	BlockLabelPublicKey = "PUBLIC KEY"
	// BlockLabelCertificate is the value to define a block label of certificates.
	BlockLabelCertificate = "CERTIFICATE"
	// BlockLabelExtension is the value to define a block label of certificate extensions.
	BlockLabelExtension = "EXTENSION"
)

var (
	// ErrUnsupportedKey is used when key type is not supported.
	ErrUnsupportedKey = errs.Class("unsupported key type")
	// ErrParse is used when an error occurs while parsing a certificate or key.
	ErrParse = errs.Class("unable to parse")
	// ErrSign is used when something goes wrong while generating a signature.
	ErrSign = errs.Class("unable to generate signature")
	// ErrVerifySignature is used when a signature verification error occurs.
	ErrVerifySignature = errs.Class("signature verification")
	// ErrChainLength is used when the length of a cert chain isn't what was expected.
	ErrChainLength = errs.Class("cert chain length")
)
