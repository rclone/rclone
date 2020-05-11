// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/peertls"
	"storj.io/common/pkcrypto"
)

var (
	// RevocationCheckHandler ensures that a remote peer's certificate chain
	// doesn't contain any revoked certificates.
	RevocationCheckHandler = NewHandlerFactory(&RevocationExtID, revocationChecker)
	// RevocationUpdateHandler looks for certificate revocation extensions on a
	// remote peer's certificate chain, adding them to the revocation DB if valid.
	RevocationUpdateHandler = NewHandlerFactory(&RevocationExtID, revocationUpdater)
)

// ErrRevocation is used when an error occurs involving a certificate revocation
var ErrRevocation = errs.Class("revocation processing error")

// ErrRevocationDB is used when an error occurs involving the revocations database
var ErrRevocationDB = errs.Class("revocation database error")

// ErrRevokedCert is used when a certificate in the chain is revoked and not expected to be
var ErrRevokedCert = ErrRevocation.New("a certificate in the chain is revoked")

// ErrRevocationTimestamp is used when a revocation's timestamp is older than the last recorded revocation
var ErrRevocationTimestamp = Error.New("revocation timestamp is older than last known revocation")

// Revocation represents a certificate revocation for storage in the revocation
// database and for use in a TLS extension.
type Revocation struct {
	Timestamp int64
	KeyHash   []byte
	Signature []byte
}

// RevocationDB stores certificate revocation data.
type RevocationDB interface {
	Get(ctx context.Context, chain []*x509.Certificate) (*Revocation, error)
	Put(ctx context.Context, chain []*x509.Certificate, ext pkix.Extension) error
	List(ctx context.Context) ([]*Revocation, error)
}

// NewRevocationExt generates a revocation extension for a certificate.
func NewRevocationExt(key crypto.PrivateKey, revokedCert *x509.Certificate) (pkix.Extension, error) {
	nowUnix := time.Now().Unix()

	keyHash, err := peertls.DoubleSHA256PublicKey(revokedCert.PublicKey)
	if err != nil {
		return pkix.Extension{}, err
	}
	rev := Revocation{
		Timestamp: nowUnix,
		KeyHash:   keyHash[:],
	}

	if err := rev.Sign(key); err != nil {
		return pkix.Extension{}, err
	}

	revBytes, err := rev.Marshal()
	if err != nil {
		return pkix.Extension{}, err
	}

	ext := pkix.Extension{
		Id:    RevocationExtID,
		Value: revBytes,
	}

	return ext, nil
}

func revocationChecker(opts *Options) HandlerFunc {
	return func(_ pkix.Extension, chains [][]*x509.Certificate) error {
		ca, leaf := chains[0][peertls.CAIndex], chains[0][peertls.LeafIndex]
		lastRev, lastRevErr := opts.RevocationDB.Get(context.TODO(), chains[0])
		if lastRevErr != nil {
			return Error.Wrap(lastRevErr)
		}
		if lastRev == nil {
			return nil
		}

		nodeID, err := peertls.DoubleSHA256PublicKey(ca.PublicKey)
		if err != nil {
			return err
		}
		leafKeyHash, err := peertls.DoubleSHA256PublicKey(leaf.PublicKey)
		if err != nil {
			return err
		}

		// NB: we trust that anything that made it into the revocation DB is valid
		//		(i.e. no need for further verification)
		switch {
		case bytes.Equal(lastRev.KeyHash, nodeID[:]):
			fallthrough
		case bytes.Equal(lastRev.KeyHash, leafKeyHash[:]):
			return ErrRevokedCert
		default:
			return nil
		}
	}
}

func revocationUpdater(opts *Options) HandlerFunc {
	return func(ext pkix.Extension, chains [][]*x509.Certificate) error {
		if err := opts.RevocationDB.Put(context.TODO(), chains[0], ext); err != nil {
			return err
		}
		return nil
	}
}

// Verify checks if the signature of the revocation was produced by the passed cert's public key.
func (r Revocation) Verify(signingCert *x509.Certificate) error {
	pubKey, ok := signingCert.PublicKey.(crypto.PublicKey)
	if !ok {
		return pkcrypto.ErrUnsupportedKey.New("%T", signingCert.PublicKey)
	}

	data := r.TBSBytes()
	if err := pkcrypto.HashAndVerifySignature(pubKey, data, r.Signature); err != nil {
		return err
	}
	return nil
}

// TBSBytes (ToBeSigned) returns the hash of the revoked certificate key hash
// and the timestamp (i.e. hash(hash(cert bytes) + timestamp)).
func (r *Revocation) TBSBytes() []byte {
	var tsBytes [binary.MaxVarintLen64]byte
	binary.PutVarint(tsBytes[:], r.Timestamp)
	toHash := append(append([]byte{}, r.KeyHash...), tsBytes[:]...)

	return pkcrypto.SHA256Hash(toHash)
}

// Sign generates a signature using the passed key and attaches it to the revocation.
func (r *Revocation) Sign(key crypto.PrivateKey) error {
	data := r.TBSBytes()
	sig, err := pkcrypto.HashAndSign(key, data)
	if err != nil {
		return err
	}
	r.Signature = sig
	return nil
}

// Marshal serializes a revocation to bytes
func (r Revocation) Marshal() ([]byte, error) {
	return (&revocationEncoder{}).encode(r)
}

// Unmarshal deserializes a revocation from bytes
func (r *Revocation) Unmarshal(data []byte) error {
	revocation, err := (&revocationDecoder{}).decode(data)
	if err != nil {
		return err
	}
	*r = revocation
	return nil
}
