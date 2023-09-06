// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errors contains common error types for the OpenPGP packages.
package errors // import "github.com/ProtonMail/go-crypto/openpgp/errors"

import (
	"strconv"
)

// A StructuralError is returned when OpenPGP data is found to be syntactically
// invalid.
type StructuralError string

func (s StructuralError) Error() string {
	return "openpgp: invalid data: " + string(s)
}

// UnsupportedError indicates that, although the OpenPGP data is valid, it
// makes use of currently unimplemented features.
type UnsupportedError string

func (s UnsupportedError) Error() string {
	return "openpgp: unsupported feature: " + string(s)
}

// InvalidArgumentError indicates that the caller is in error and passed an
// incorrect value.
type InvalidArgumentError string

func (i InvalidArgumentError) Error() string {
	return "openpgp: invalid argument: " + string(i)
}

// SignatureError indicates that a syntactically valid signature failed to
// validate.
type SignatureError string

func (b SignatureError) Error() string {
	return "openpgp: invalid signature: " + string(b)
}

var ErrMDCHashMismatch error = SignatureError("MDC hash mismatch")
var ErrMDCMissing error = SignatureError("MDC packet not found")

type signatureExpiredError int

func (se signatureExpiredError) Error() string {
	return "openpgp: signature expired"
}

var ErrSignatureExpired error = signatureExpiredError(0)

type keyExpiredError int

func (ke keyExpiredError) Error() string {
	return "openpgp: key expired"
}

var ErrKeyExpired error = keyExpiredError(0)

type keyIncorrectError int

func (ki keyIncorrectError) Error() string {
	return "openpgp: incorrect key"
}

var ErrKeyIncorrect error = keyIncorrectError(0)

// KeyInvalidError indicates that the public key parameters are invalid
// as they do not match the private ones
type KeyInvalidError string

func (e KeyInvalidError) Error() string {
	return "openpgp: invalid key: " + string(e)
}

type unknownIssuerError int

func (unknownIssuerError) Error() string {
	return "openpgp: signature made by unknown entity"
}

var ErrUnknownIssuer error = unknownIssuerError(0)

type keyRevokedError int

func (keyRevokedError) Error() string {
	return "openpgp: signature made by revoked key"
}

var ErrKeyRevoked error = keyRevokedError(0)

type UnknownPacketTypeError uint8

func (upte UnknownPacketTypeError) Error() string {
	return "openpgp: unknown packet type: " + strconv.Itoa(int(upte))
}

// AEADError indicates that there is a problem when initializing or using a
// AEAD instance, configuration struct, nonces or index values.
type AEADError string

func (ae AEADError) Error() string {
	return "openpgp: aead error: " + string(ae)
}

// ErrDummyPrivateKey results when operations are attempted on a private key
// that is just a dummy key. See
// https://git.gnupg.org/cgi-bin/gitweb.cgi?p=gnupg.git;a=blob;f=doc/DETAILS;h=fe55ae16ab4e26d8356dc574c9e8bc935e71aef1;hb=23191d7851eae2217ecdac6484349849a24fd94a#l1109
type ErrDummyPrivateKey string

func (dke ErrDummyPrivateKey) Error() string {
	return "openpgp: s2k GNU dummy key: " + string(dke)
}
