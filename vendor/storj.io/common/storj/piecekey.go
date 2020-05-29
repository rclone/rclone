// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"database/sql/driver"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/ed25519"
)

// ErrPieceKey is used when something goes wrong with a piece key.
var ErrPieceKey = errs.Class("piece key error")

// PiecePublicKey is the unique identifier for pieces.
type PiecePublicKey struct {
	pub ed25519.PublicKey
}

// PiecePrivateKey is the unique identifier for pieces.
type PiecePrivateKey struct {
	priv ed25519.PrivateKey
}

// NewPieceKey creates a piece key pair.
func NewPieceKey() (PiecePublicKey, PiecePrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(nil)

	return PiecePublicKey{pub}, PiecePrivateKey{priv}, ErrPieceKey.Wrap(err)
}

// PiecePublicKeyFromBytes converts bytes to a piece public key.
func PiecePublicKeyFromBytes(data []byte) (PiecePublicKey, error) {
	if len(data) != ed25519.PublicKeySize {
		return PiecePublicKey{}, ErrPieceKey.New("invalid public key length %v", len(data))
	}
	return PiecePublicKey{ed25519.PublicKey(data)}, nil
}

// PiecePrivateKeyFromBytes converts bytes to a piece private key.
func PiecePrivateKeyFromBytes(data []byte) (PiecePrivateKey, error) {
	if len(data) != ed25519.PrivateKeySize {
		return PiecePrivateKey{}, ErrPieceKey.New("invalid private key length %v", len(data))
	}
	return PiecePrivateKey{ed25519.PrivateKey(data)}, nil
}

// Sign signs the message with privateKey and returns a signature.
func (key PiecePrivateKey) Sign(data []byte) ([]byte, error) {
	if len(key.priv) != ed25519.PrivateKeySize {
		return nil, ErrPieceKey.New("invalid private key length %v", len(key.priv))
	}
	return ed25519.Sign(key.priv, data), nil
}

// Verify reports whether signature is a valid signature of message by publicKey.
func (key PiecePublicKey) Verify(data, signature []byte) error {
	if len(key.pub) != ed25519.PublicKeySize {
		return ErrPieceKey.New("invalid public key length %v", len(key.pub))
	}
	if !ed25519.Verify(key.pub, data, signature) {
		return ErrPieceKey.New("invalid signature")
	}
	return nil
}

// Bytes returns bytes of the piece public key.
func (key PiecePublicKey) Bytes() []byte { return key.pub[:] }

// Bytes returns bytes of the piece private key.
func (key PiecePrivateKey) Bytes() []byte { return key.priv[:] }

// IsZero returns whether the key is empty.
func (key PiecePublicKey) IsZero() bool { return len(key.pub) == 0 }

// IsZero returns whether the key is empty.
func (key PiecePrivateKey) IsZero() bool { return len(key.priv) == 0 }

// Marshal serializes a piece public key.
func (key PiecePublicKey) Marshal() ([]byte, error) { return key.Bytes(), nil }

// Marshal serializes a piece private key.
func (key PiecePrivateKey) Marshal() ([]byte, error) { return key.Bytes(), nil }

// MarshalTo serializes a piece public key into the passed byte slice.
func (key *PiecePublicKey) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, key.Bytes())
	return n, nil
}

// MarshalTo serializes a piece private key into the passed byte slice.
func (key *PiecePrivateKey) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, key.Bytes())
	return n, nil
}

// Unmarshal deserializes a piece public key.
func (key *PiecePublicKey) Unmarshal(data []byte) error {
	// allow empty keys
	if len(data) == 0 {
		key.pub = nil
		return nil
	}
	var err error
	*key, err = PiecePublicKeyFromBytes(data)
	return err
}

// Unmarshal deserializes a piece private key.
func (key *PiecePrivateKey) Unmarshal(data []byte) error {
	// allow empty keys
	if len(data) == 0 {
		key.priv = nil
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	var err error
	*key, err = PiecePrivateKeyFromBytes(data)
	return err
}

// Size returns the length of a piece public key (implements gogo's custom type interface).
func (key *PiecePublicKey) Size() int { return len(key.pub) }

// Size returns the length of a piece private key (implements gogo's custom type interface).
func (key *PiecePrivateKey) Size() int { return len(key.priv) }

// Value set a PiecePublicKey to a database field.
func (key PiecePublicKey) Value() (driver.Value, error) {
	return key.Bytes(), nil
}

// Value set a PiecePrivateKey to a database field.
func (key PiecePrivateKey) Value() (driver.Value, error) { return key.Bytes(), nil }

// Scan extracts a PiecePublicKey from a database field.
func (key *PiecePublicKey) Scan(src interface{}) (err error) {
	b, ok := src.([]byte)
	if !ok {
		return ErrPieceKey.New("PiecePublicKey Scan expects []byte")
	}
	n, err := PiecePublicKeyFromBytes(b)
	*key = n
	return err
}

// Scan extracts a PiecePrivateKey from a database field.
func (key *PiecePrivateKey) Scan(src interface{}) (err error) {
	b, ok := src.([]byte)
	if !ok {
		return ErrPieceKey.New("PiecePrivateKey Scan expects []byte")
	}
	n, err := PiecePrivateKeyFromBytes(b)
	*key = n
	return err
}
