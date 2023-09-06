// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"crypto"
	"crypto/rsa"
	"encoding/binary"
	"io"
	"math/big"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp/ecdh"
	"github.com/ProtonMail/go-crypto/openpgp/elgamal"
	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/encoding"
)

const encryptedKeyVersion = 3

// EncryptedKey represents a public-key encrypted session key. See RFC 4880,
// section 5.1.
type EncryptedKey struct {
	KeyId      uint64
	Algo       PublicKeyAlgorithm
	CipherFunc CipherFunction // only valid after a successful Decrypt for a v3 packet
	Key        []byte         // only valid after a successful Decrypt

	encryptedMPI1, encryptedMPI2 encoding.Field
}

func (e *EncryptedKey) parse(r io.Reader) (err error) {
	var buf [10]byte
	_, err = readFull(r, buf[:])
	if err != nil {
		return
	}
	if buf[0] != encryptedKeyVersion {
		return errors.UnsupportedError("unknown EncryptedKey version " + strconv.Itoa(int(buf[0])))
	}
	e.KeyId = binary.BigEndian.Uint64(buf[1:9])
	e.Algo = PublicKeyAlgorithm(buf[9])
	switch e.Algo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly:
		e.encryptedMPI1 = new(encoding.MPI)
		if _, err = e.encryptedMPI1.ReadFrom(r); err != nil {
			return
		}
	case PubKeyAlgoElGamal:
		e.encryptedMPI1 = new(encoding.MPI)
		if _, err = e.encryptedMPI1.ReadFrom(r); err != nil {
			return
		}

		e.encryptedMPI2 = new(encoding.MPI)
		if _, err = e.encryptedMPI2.ReadFrom(r); err != nil {
			return
		}
	case PubKeyAlgoECDH:
		e.encryptedMPI1 = new(encoding.MPI)
		if _, err = e.encryptedMPI1.ReadFrom(r); err != nil {
			return
		}

		e.encryptedMPI2 = new(encoding.OID)
		if _, err = e.encryptedMPI2.ReadFrom(r); err != nil {
			return
		}
	}
	_, err = consumeAll(r)
	return
}

func checksumKeyMaterial(key []byte) uint16 {
	var checksum uint16
	for _, v := range key {
		checksum += uint16(v)
	}
	return checksum
}

// Decrypt decrypts an encrypted session key with the given private key. The
// private key must have been decrypted first.
// If config is nil, sensible defaults will be used.
func (e *EncryptedKey) Decrypt(priv *PrivateKey, config *Config) error {
	if e.KeyId != 0 && e.KeyId != priv.KeyId {
		return errors.InvalidArgumentError("cannot decrypt encrypted session key for key id " + strconv.FormatUint(e.KeyId, 16) + " with private key id " + strconv.FormatUint(priv.KeyId, 16))
	}
	if e.Algo != priv.PubKeyAlgo {
		return errors.InvalidArgumentError("cannot decrypt encrypted session key of type " + strconv.Itoa(int(e.Algo)) + " with private key of type " + strconv.Itoa(int(priv.PubKeyAlgo)))
	}
	if priv.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}

	var err error
	var b []byte

	// TODO(agl): use session key decryption routines here to avoid
	// padding oracle attacks.
	switch priv.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly:
		// Supports both *rsa.PrivateKey and crypto.Decrypter
		k := priv.PrivateKey.(crypto.Decrypter)
		b, err = k.Decrypt(config.Random(), padToKeySize(k.Public().(*rsa.PublicKey), e.encryptedMPI1.Bytes()), nil)
	case PubKeyAlgoElGamal:
		c1 := new(big.Int).SetBytes(e.encryptedMPI1.Bytes())
		c2 := new(big.Int).SetBytes(e.encryptedMPI2.Bytes())
		b, err = elgamal.Decrypt(priv.PrivateKey.(*elgamal.PrivateKey), c1, c2)
	case PubKeyAlgoECDH:
		vsG := e.encryptedMPI1.Bytes()
		m := e.encryptedMPI2.Bytes()
		oid := priv.PublicKey.oid.EncodedBytes()
		b, err = ecdh.Decrypt(priv.PrivateKey.(*ecdh.PrivateKey), vsG, m, oid, priv.PublicKey.Fingerprint[:])
	default:
		err = errors.InvalidArgumentError("cannot decrypt encrypted session key with private key of type " + strconv.Itoa(int(priv.PubKeyAlgo)))
	}

	if err != nil {
		return err
	}

	e.CipherFunc = CipherFunction(b[0])
	if !e.CipherFunc.IsSupported() {
		return errors.UnsupportedError("unsupported encryption function")
	}

	e.Key = b[1 : len(b)-2]
	expectedChecksum := uint16(b[len(b)-2])<<8 | uint16(b[len(b)-1])
	checksum := checksumKeyMaterial(e.Key)
	if checksum != expectedChecksum {
		return errors.StructuralError("EncryptedKey checksum incorrect")
	}

	return nil
}

// Serialize writes the encrypted key packet, e, to w.
func (e *EncryptedKey) Serialize(w io.Writer) error {
	var mpiLen int
	switch e.Algo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly:
		mpiLen = int(e.encryptedMPI1.EncodedLength())
	case PubKeyAlgoElGamal:
		mpiLen = int(e.encryptedMPI1.EncodedLength()) + int(e.encryptedMPI2.EncodedLength())
	case PubKeyAlgoECDH:
		mpiLen = int(e.encryptedMPI1.EncodedLength()) + int(e.encryptedMPI2.EncodedLength())
	default:
		return errors.InvalidArgumentError("don't know how to serialize encrypted key type " + strconv.Itoa(int(e.Algo)))
	}

	err := serializeHeader(w, packetTypeEncryptedKey, 1 /* version */ +8 /* key id */ +1 /* algo */ +mpiLen)
	if err != nil {
		return err
	}

	w.Write([]byte{encryptedKeyVersion})
	binary.Write(w, binary.BigEndian, e.KeyId)
	w.Write([]byte{byte(e.Algo)})

	switch e.Algo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly:
		_, err := w.Write(e.encryptedMPI1.EncodedBytes())
		return err
	case PubKeyAlgoElGamal:
		if _, err := w.Write(e.encryptedMPI1.EncodedBytes()); err != nil {
			return err
		}
		_, err := w.Write(e.encryptedMPI2.EncodedBytes())
		return err
	case PubKeyAlgoECDH:
		if _, err := w.Write(e.encryptedMPI1.EncodedBytes()); err != nil {
			return err
		}
		_, err := w.Write(e.encryptedMPI2.EncodedBytes())
		return err
	default:
		panic("internal error")
	}
}

// SerializeEncryptedKey serializes an encrypted key packet to w that contains
// key, encrypted to pub.
// If config is nil, sensible defaults will be used.
func SerializeEncryptedKey(w io.Writer, pub *PublicKey, cipherFunc CipherFunction, key []byte, config *Config) error {
	var buf [10]byte
	buf[0] = encryptedKeyVersion
	binary.BigEndian.PutUint64(buf[1:9], pub.KeyId)
	buf[9] = byte(pub.PubKeyAlgo)

	keyBlock := make([]byte, 1 /* cipher type */ +len(key)+2 /* checksum */)
	keyBlock[0] = byte(cipherFunc)
	copy(keyBlock[1:], key)
	checksum := checksumKeyMaterial(key)
	keyBlock[1+len(key)] = byte(checksum >> 8)
	keyBlock[1+len(key)+1] = byte(checksum)

	switch pub.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly:
		return serializeEncryptedKeyRSA(w, config.Random(), buf, pub.PublicKey.(*rsa.PublicKey), keyBlock)
	case PubKeyAlgoElGamal:
		return serializeEncryptedKeyElGamal(w, config.Random(), buf, pub.PublicKey.(*elgamal.PublicKey), keyBlock)
	case PubKeyAlgoECDH:
		return serializeEncryptedKeyECDH(w, config.Random(), buf, pub.PublicKey.(*ecdh.PublicKey), keyBlock, pub.oid, pub.Fingerprint)
	case PubKeyAlgoDSA, PubKeyAlgoRSASignOnly:
		return errors.InvalidArgumentError("cannot encrypt to public key of type " + strconv.Itoa(int(pub.PubKeyAlgo)))
	}

	return errors.UnsupportedError("encrypting a key to public key of type " + strconv.Itoa(int(pub.PubKeyAlgo)))
}

func serializeEncryptedKeyRSA(w io.Writer, rand io.Reader, header [10]byte, pub *rsa.PublicKey, keyBlock []byte) error {
	cipherText, err := rsa.EncryptPKCS1v15(rand, pub, keyBlock)
	if err != nil {
		return errors.InvalidArgumentError("RSA encryption failed: " + err.Error())
	}

	cipherMPI := encoding.NewMPI(cipherText)
	packetLen := 10 /* header length */ + int(cipherMPI.EncodedLength())

	err = serializeHeader(w, packetTypeEncryptedKey, packetLen)
	if err != nil {
		return err
	}
	_, err = w.Write(header[:])
	if err != nil {
		return err
	}
	_, err = w.Write(cipherMPI.EncodedBytes())
	return err
}

func serializeEncryptedKeyElGamal(w io.Writer, rand io.Reader, header [10]byte, pub *elgamal.PublicKey, keyBlock []byte) error {
	c1, c2, err := elgamal.Encrypt(rand, pub, keyBlock)
	if err != nil {
		return errors.InvalidArgumentError("ElGamal encryption failed: " + err.Error())
	}

	packetLen := 10 /* header length */
	packetLen += 2 /* mpi size */ + (c1.BitLen()+7)/8
	packetLen += 2 /* mpi size */ + (c2.BitLen()+7)/8

	err = serializeHeader(w, packetTypeEncryptedKey, packetLen)
	if err != nil {
		return err
	}
	_, err = w.Write(header[:])
	if err != nil {
		return err
	}
	if _, err = w.Write(new(encoding.MPI).SetBig(c1).EncodedBytes()); err != nil {
		return err
	}
	_, err = w.Write(new(encoding.MPI).SetBig(c2).EncodedBytes())
	return err
}

func serializeEncryptedKeyECDH(w io.Writer, rand io.Reader, header [10]byte, pub *ecdh.PublicKey, keyBlock []byte, oid encoding.Field, fingerprint []byte) error {
	vsG, c, err := ecdh.Encrypt(rand, pub, keyBlock, oid.EncodedBytes(), fingerprint)
	if err != nil {
		return errors.InvalidArgumentError("ECDH encryption failed: " + err.Error())
	}

	g := encoding.NewMPI(vsG)
	m := encoding.NewOID(c)

	packetLen := 10 /* header length */
	packetLen += int(g.EncodedLength()) + int(m.EncodedLength())

	err = serializeHeader(w, packetTypeEncryptedKey, packetLen)
	if err != nil {
		return err
	}

	_, err = w.Write(header[:])
	if err != nil {
		return err
	}
	if _, err = w.Write(g.EncodedBytes()); err != nil {
		return err
	}
	_, err = w.Write(m.EncodedBytes())
	return err
}
