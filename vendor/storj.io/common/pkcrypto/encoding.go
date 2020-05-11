// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"io"
	"math/big"

	"github.com/zeebo/errs"
)

// WritePublicKeyPEM writes the public key, in a PEM-enveloped
// PKIX form.
func WritePublicKeyPEM(w io.Writer, key crypto.PublicKey) error {
	kb, err := PublicKeyToPKIX(key)
	if err != nil {
		return err
	}
	err = pem.Encode(w, &pem.Block{Type: BlockLabelPublicKey, Bytes: kb})
	return errs.Wrap(err)
}

// PublicKeyToPEM encodes a public key to a PEM-enveloped PKIX form.
func PublicKeyToPEM(key crypto.PublicKey) ([]byte, error) {
	kb, err := PublicKeyToPKIX(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: BlockLabelPublicKey, Bytes: kb}), nil
}

// PublicKeyToPKIX serializes a public key to a PKIX-encoded form.
func PublicKeyToPKIX(key crypto.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(key)
}

// PublicKeyFromPKIX parses a public key from its PKIX encoding.
func PublicKeyFromPKIX(pkixData []byte) (crypto.PublicKey, error) {
	return x509.ParsePKIXPublicKey(pkixData)
}

// PublicKeyFromPEM parses a public key from its PEM-enveloped PKIX
// encoding.
func PublicKeyFromPEM(pemData []byte) (crypto.PublicKey, error) {
	pb, _ := pem.Decode(pemData)
	if pb == nil {
		return nil, ErrParse.New("could not parse PEM encoding")
	}
	if pb.Type != BlockLabelPublicKey {
		return nil, ErrParse.New("can not parse public key from PEM block labeled %q", pb.Type)
	}
	return PublicKeyFromPKIX(pb.Bytes)
}

// WritePrivateKeyPEM writes the private key to the writer, in a PEM-enveloped
// PKCS#8 form.
func WritePrivateKeyPEM(w io.Writer, key crypto.PrivateKey) error {
	kb, err := PrivateKeyToPKCS8(key)
	if err != nil {
		return errs.Wrap(err)
	}
	err = pem.Encode(w, &pem.Block{Type: BlockLabelPrivateKey, Bytes: kb})
	return errs.Wrap(err)
}

// PrivateKeyToPEM serializes a private key to a PEM-enveloped PKCS#8 form.
func PrivateKeyToPEM(key crypto.PrivateKey) ([]byte, error) {
	kb, err := PrivateKeyToPKCS8(key)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: BlockLabelPrivateKey, Bytes: kb}), nil
}

// PrivateKeyToPKCS8 serializes a private key to a PKCS#8-encoded form.
func PrivateKeyToPKCS8(key crypto.PrivateKey) ([]byte, error) {
	return x509.MarshalPKCS8PrivateKey(key)
}

// PrivateKeyFromPKCS8 parses a private key from its PKCS#8 encoding.
func PrivateKeyFromPKCS8(keyBytes []byte) (crypto.PrivateKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}
	return crypto.PrivateKey(key), nil
}

// PrivateKeyFromPEM parses a private key from its PEM-enveloped PKCS#8
// encoding.
func PrivateKeyFromPEM(keyBytes []byte) (crypto.PrivateKey, error) {
	pb, _ := pem.Decode(keyBytes)
	if pb == nil {
		return nil, ErrParse.New("could not parse PEM encoding")
	}
	switch pb.Type {
	case BlockLabelEcPrivateKey:
		return ecPrivateKeyFromASN1(pb.Bytes)
	case BlockLabelPrivateKey:
		return PrivateKeyFromPKCS8(pb.Bytes)
	}
	return nil, ErrParse.New("can not parse private key from PEM block labeled %q", pb.Type)
}

// WriteCertPEM writes the certificate to the writer, in a PEM-enveloped DER
// encoding.
func WriteCertPEM(w io.Writer, certs ...*x509.Certificate) error {
	if len(certs) == 0 {
		return errs.New("no certs to encode")
	}
	encodeErrs := new(errs.Group)
	for _, cert := range certs {
		encodeErrs.Add(pem.Encode(w, &pem.Block{Type: BlockLabelCertificate, Bytes: cert.Raw}))
	}
	return encodeErrs.Err()
}

// CertToPEM returns the bytes of the certificate, in a PEM-enveloped DER
// encoding.
func CertToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: BlockLabelCertificate, Bytes: cert.Raw})
}

// CertToDER returns the bytes of the certificate, in a DER encoding.
//
// Note that this is fairly useless, as x509.Certificate objects are always
// supposed to have a member containing the raw DER encoding. But this is
// included for completeness with the rest of this module's API.
func CertToDER(cert *x509.Certificate) ([]byte, error) {
	return cert.Raw, nil
}

// CertFromDER parses an X.509 certificate from its DER encoding.
func CertFromDER(certDER []byte) (*x509.Certificate, error) {
	return x509.ParseCertificate(certDER)
}

// CertFromPEM parses an X.509 certificate from its PEM-enveloped DER encoding.
func CertFromPEM(certPEM []byte) (*x509.Certificate, error) {
	kb, _ := pem.Decode(certPEM)
	if kb == nil {
		return nil, ErrParse.New("could not decode certificate as PEM")
	}
	if kb.Type != BlockLabelCertificate {
		return nil, ErrParse.New("can not parse certificate from PEM block labeled %q", kb.Type)
	}
	return CertFromDER(kb.Bytes)
}

// CertsFromDER parses an x509 certificate from each of the given byte
// slices, which should be encoded in DER.
func CertsFromDER(rawCerts [][]byte) ([]*x509.Certificate, error) {
	certs := make([]*x509.Certificate, len(rawCerts))
	for i, c := range rawCerts {
		var err error
		certs[i], err = CertFromDER(c)
		if err != nil {
			return nil, ErrParse.New("unable to parse certificate at index %d", i)
		}
	}
	return certs, nil
}

// CertsFromPEM parses a PEM chain from a single byte string (the PEM-enveloped
// certificates should be concatenated). The PEM blocks may include PKIX
// extensions.
func CertsFromPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	var (
		encChain  encodedChain
		blockErrs errs.Group
	)
	for {
		var pemBlock *pem.Block
		pemBlock, pemBytes = pem.Decode(pemBytes)
		if pemBlock == nil {
			break
		}
		if pemBlock.Type == BlockLabelCertificate {
			encChain.AddCert(pemBlock.Bytes)
		}
	}
	if err := blockErrs.Err(); err != nil {
		return nil, err
	}

	return encChain.Parse()
}

type encodedChain struct {
	chain [][]byte
}

func (e *encodedChain) AddCert(b []byte) {
	e.chain = append(e.chain, b)
}

func (e *encodedChain) Parse() ([]*x509.Certificate, error) {
	chain, err := CertsFromDER(e.chain)
	if err != nil {
		return nil, err
	}

	return chain, nil
}

type ecdsaSignature struct {
	R, S *big.Int
}

func marshalECDSASignature(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(ecdsaSignature{R: r, S: s})
}

func unmarshalECDSASignature(signatureBytes []byte) (r, s *big.Int, err error) {
	var signature ecdsaSignature
	if _, err = asn1.Unmarshal(signatureBytes, &signature); err != nil {
		return nil, nil, err
	}
	return signature.R, signature.S, nil
}

// ecPrivateKeyFromASN1 parses a private key from the special Elliptic Curve
// Private Key ASN.1 structure. This is here only for backward compatibility.
// Use PKCS#8 instead.
func ecPrivateKeyFromASN1(privKeyData []byte) (crypto.PrivateKey, error) {
	key, err := x509.ParseECPrivateKey(privKeyData)
	if err != nil {
		return nil, err
	}
	return crypto.PrivateKey(key), nil
}
