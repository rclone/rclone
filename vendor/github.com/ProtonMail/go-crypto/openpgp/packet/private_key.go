// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"bytes"
	"crypto"
	"crypto/cipher"
	"crypto/dsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"io"
	"io/ioutil"
	"math/big"
	"strconv"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/ecdh"
	"github.com/ProtonMail/go-crypto/openpgp/ecdsa"
	"github.com/ProtonMail/go-crypto/openpgp/eddsa"
	"github.com/ProtonMail/go-crypto/openpgp/elgamal"
	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/encoding"
	"github.com/ProtonMail/go-crypto/openpgp/s2k"
)

// PrivateKey represents a possibly encrypted private key. See RFC 4880,
// section 5.5.3.
type PrivateKey struct {
	PublicKey
	Encrypted     bool // if true then the private key is unavailable until Decrypt has been called.
	encryptedData []byte
	cipher        CipherFunction
	s2k           func(out, in []byte)
	// An *{rsa|dsa|elgamal|ecdh|ecdsa|ed25519}.PrivateKey or
	// crypto.Signer/crypto.Decrypter (Decryptor RSA only).
	PrivateKey   interface{}
	sha1Checksum bool
	iv           []byte

	// Type of encryption of the S2K packet
	// Allowed values are 0 (Not encrypted), 254 (SHA1), or
	// 255 (2-byte checksum)
	s2kType S2KType
	// Full parameters of the S2K packet
	s2kParams *s2k.Params
}

// S2KType s2k packet type
type S2KType uint8

const (
	// S2KNON unencrypt
	S2KNON S2KType = 0
	// S2KSHA1 sha1 sum check
	S2KSHA1 S2KType = 254
	// S2KCHECKSUM sum check
	S2KCHECKSUM S2KType = 255
)

func NewRSAPrivateKey(creationTime time.Time, priv *rsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewRSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewDSAPrivateKey(creationTime time.Time, priv *dsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewDSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewElGamalPrivateKey(creationTime time.Time, priv *elgamal.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewElGamalPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewECDSAPrivateKey(creationTime time.Time, priv *ecdsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewECDSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewEdDSAPrivateKey(creationTime time.Time, priv *eddsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewEdDSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewECDHPrivateKey(creationTime time.Time, priv *ecdh.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewECDHPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

// NewSignerPrivateKey creates a PrivateKey from a crypto.Signer that
// implements RSA, ECDSA or EdDSA.
func NewSignerPrivateKey(creationTime time.Time, signer interface{}) *PrivateKey {
	pk := new(PrivateKey)
	// In general, the public Keys should be used as pointers. We still
	// type-switch on the values, for backwards-compatibility.
	switch pubkey := signer.(type) {
	case *rsa.PrivateKey:
		pk.PublicKey = *NewRSAPublicKey(creationTime, &pubkey.PublicKey)
	case rsa.PrivateKey:
		pk.PublicKey = *NewRSAPublicKey(creationTime, &pubkey.PublicKey)
	case *ecdsa.PrivateKey:
		pk.PublicKey = *NewECDSAPublicKey(creationTime, &pubkey.PublicKey)
	case ecdsa.PrivateKey:
		pk.PublicKey = *NewECDSAPublicKey(creationTime, &pubkey.PublicKey)
	case *eddsa.PrivateKey:
		pk.PublicKey = *NewEdDSAPublicKey(creationTime, &pubkey.PublicKey)
	case eddsa.PrivateKey:
		pk.PublicKey = *NewEdDSAPublicKey(creationTime, &pubkey.PublicKey)
	default:
		panic("openpgp: unknown signer type in NewSignerPrivateKey")
	}
	pk.PrivateKey = signer
	return pk
}

// NewDecrypterPrivateKey creates a PrivateKey from a *{rsa|elgamal|ecdh}.PrivateKey.
func NewDecrypterPrivateKey(creationTime time.Time, decrypter interface{}) *PrivateKey {
	pk := new(PrivateKey)
	switch priv := decrypter.(type) {
	case *rsa.PrivateKey:
		pk.PublicKey = *NewRSAPublicKey(creationTime, &priv.PublicKey)
	case *elgamal.PrivateKey:
		pk.PublicKey = *NewElGamalPublicKey(creationTime, &priv.PublicKey)
	case *ecdh.PrivateKey:
		pk.PublicKey = *NewECDHPublicKey(creationTime, &priv.PublicKey)
	default:
		panic("openpgp: unknown decrypter type in NewDecrypterPrivateKey")
	}
	pk.PrivateKey = decrypter
	return pk
}

func (pk *PrivateKey) parse(r io.Reader) (err error) {
	err = (&pk.PublicKey).parse(r)
	if err != nil {
		return
	}
	v5 := pk.PublicKey.Version == 5

	var buf [1]byte
	_, err = readFull(r, buf[:])
	if err != nil {
		return
	}
	pk.s2kType = S2KType(buf[0])
	var optCount [1]byte
	if v5 {
		if _, err = readFull(r, optCount[:]); err != nil {
			return
		}
	}

	switch pk.s2kType {
	case S2KNON:
		pk.s2k = nil
		pk.Encrypted = false
	case S2KSHA1, S2KCHECKSUM:
		if v5 && pk.s2kType == S2KCHECKSUM {
			return errors.StructuralError("wrong s2k identifier for version 5")
		}
		_, err = readFull(r, buf[:])
		if err != nil {
			return
		}
		pk.cipher = CipherFunction(buf[0])
		if pk.cipher != 0 && !pk.cipher.IsSupported() {
			return errors.UnsupportedError("unsupported cipher function in private key")
		}
		pk.s2kParams, err = s2k.ParseIntoParams(r)
		if err != nil {
			return
		}
		if pk.s2kParams.Dummy() {
			return
		}
		pk.s2k, err = pk.s2kParams.Function()
		if err != nil {
			return
		}
		pk.Encrypted = true
		if pk.s2kType == S2KSHA1 {
			pk.sha1Checksum = true
		}
	default:
		return errors.UnsupportedError("deprecated s2k function in private key")
	}

	if pk.Encrypted {
		blockSize := pk.cipher.blockSize()
		if blockSize == 0 {
			return errors.UnsupportedError("unsupported cipher in private key: " + strconv.Itoa(int(pk.cipher)))
		}
		pk.iv = make([]byte, blockSize)
		_, err = readFull(r, pk.iv)
		if err != nil {
			return
		}
	}

	var privateKeyData []byte
	if v5 {
		var n [4]byte /* secret material four octet count */
		_, err = readFull(r, n[:])
		if err != nil {
			return
		}
		count := uint32(uint32(n[0])<<24 | uint32(n[1])<<16 | uint32(n[2])<<8 | uint32(n[3]))
		if !pk.Encrypted {
			count = count + 2 /* two octet checksum */
		}
		privateKeyData = make([]byte, count)
		_, err = readFull(r, privateKeyData)
		if err != nil {
			return
		}
	} else {
		privateKeyData, err = ioutil.ReadAll(r)
		if err != nil {
			return
		}
	}
	if !pk.Encrypted {
		if len(privateKeyData) < 2 {
			return errors.StructuralError("truncated private key data")
		}
		var sum uint16
		for i := 0; i < len(privateKeyData)-2; i++ {
			sum += uint16(privateKeyData[i])
		}
		if privateKeyData[len(privateKeyData)-2] != uint8(sum>>8) ||
			privateKeyData[len(privateKeyData)-1] != uint8(sum) {
			return errors.StructuralError("private key checksum failure")
		}
		privateKeyData = privateKeyData[:len(privateKeyData)-2]
		return pk.parsePrivateKey(privateKeyData)
	}

	pk.encryptedData = privateKeyData
	return
}

// Dummy returns true if the private key is a dummy key. This is a GNU extension.
func (pk *PrivateKey) Dummy() bool {
	return pk.s2kParams.Dummy()
}

func mod64kHash(d []byte) uint16 {
	var h uint16
	for _, b := range d {
		h += uint16(b)
	}
	return h
}

func (pk *PrivateKey) Serialize(w io.Writer) (err error) {
	contents := bytes.NewBuffer(nil)
	err = pk.PublicKey.serializeWithoutHeaders(contents)
	if err != nil {
		return
	}
	if _, err = contents.Write([]byte{uint8(pk.s2kType)}); err != nil {
		return
	}

	optional := bytes.NewBuffer(nil)
	if pk.Encrypted || pk.Dummy() {
		optional.Write([]byte{uint8(pk.cipher)})
		if err := pk.s2kParams.Serialize(optional); err != nil {
			return err
		}
		if pk.Encrypted {
			optional.Write(pk.iv)
		}
	}
	if pk.Version == 5 {
		contents.Write([]byte{uint8(optional.Len())})
	}
	io.Copy(contents, optional)

	if !pk.Dummy() {
		l := 0
		var priv []byte
		if !pk.Encrypted {
			buf := bytes.NewBuffer(nil)
			err = pk.serializePrivateKey(buf)
			if err != nil {
				return err
			}
			l = buf.Len()
			checksum := mod64kHash(buf.Bytes())
			buf.Write([]byte{byte(checksum >> 8), byte(checksum)})
			priv = buf.Bytes()
		} else {
			priv, l = pk.encryptedData, len(pk.encryptedData)
		}

		if pk.Version == 5 {
			contents.Write([]byte{byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)})
		}
		contents.Write(priv)
	}

	ptype := packetTypePrivateKey
	if pk.IsSubkey {
		ptype = packetTypePrivateSubkey
	}
	err = serializeHeader(w, ptype, contents.Len())
	if err != nil {
		return
	}
	_, err = io.Copy(w, contents)
	if err != nil {
		return
	}
	return
}

func serializeRSAPrivateKey(w io.Writer, priv *rsa.PrivateKey) error {
	if _, err := w.Write(new(encoding.MPI).SetBig(priv.D).EncodedBytes()); err != nil {
		return err
	}
	if _, err := w.Write(new(encoding.MPI).SetBig(priv.Primes[1]).EncodedBytes()); err != nil {
		return err
	}
	if _, err := w.Write(new(encoding.MPI).SetBig(priv.Primes[0]).EncodedBytes()); err != nil {
		return err
	}
	_, err := w.Write(new(encoding.MPI).SetBig(priv.Precomputed.Qinv).EncodedBytes())
	return err
}

func serializeDSAPrivateKey(w io.Writer, priv *dsa.PrivateKey) error {
	_, err := w.Write(new(encoding.MPI).SetBig(priv.X).EncodedBytes())
	return err
}

func serializeElGamalPrivateKey(w io.Writer, priv *elgamal.PrivateKey) error {
	_, err := w.Write(new(encoding.MPI).SetBig(priv.X).EncodedBytes())
	return err
}

func serializeECDSAPrivateKey(w io.Writer, priv *ecdsa.PrivateKey) error {
	_, err := w.Write(encoding.NewMPI(priv.MarshalIntegerSecret()).EncodedBytes())
	return err
}

func serializeEdDSAPrivateKey(w io.Writer, priv *eddsa.PrivateKey) error {
	_, err := w.Write(encoding.NewMPI(priv.MarshalByteSecret()).EncodedBytes())
	return err
}

func serializeECDHPrivateKey(w io.Writer, priv *ecdh.PrivateKey) error {
	_, err := w.Write(encoding.NewMPI(priv.MarshalByteSecret()).EncodedBytes())
	return err
}

// decrypt decrypts an encrypted private key using a decryption key.
func (pk *PrivateKey) decrypt(decryptionKey []byte) error {
	if pk.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	if !pk.Encrypted {
		return nil
	}

	block := pk.cipher.new(decryptionKey)
	cfb := cipher.NewCFBDecrypter(block, pk.iv)

	data := make([]byte, len(pk.encryptedData))
	cfb.XORKeyStream(data, pk.encryptedData)

	if pk.sha1Checksum {
		if len(data) < sha1.Size {
			return errors.StructuralError("truncated private key data")
		}
		h := sha1.New()
		h.Write(data[:len(data)-sha1.Size])
		sum := h.Sum(nil)
		if !bytes.Equal(sum, data[len(data)-sha1.Size:]) {
			return errors.StructuralError("private key checksum failure")
		}
		data = data[:len(data)-sha1.Size]
	} else {
		if len(data) < 2 {
			return errors.StructuralError("truncated private key data")
		}
		var sum uint16
		for i := 0; i < len(data)-2; i++ {
			sum += uint16(data[i])
		}
		if data[len(data)-2] != uint8(sum>>8) ||
			data[len(data)-1] != uint8(sum) {
			return errors.StructuralError("private key checksum failure")
		}
		data = data[:len(data)-2]
	}

	err := pk.parsePrivateKey(data)
	if _, ok := err.(errors.KeyInvalidError); ok {
		return errors.KeyInvalidError("invalid key parameters")
	}
	if err != nil {
		return err
	}

	// Mark key as unencrypted
	pk.s2kType = S2KNON
	pk.s2k = nil
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}

func (pk *PrivateKey) decryptWithCache(passphrase []byte, keyCache *s2k.Cache) error {
	if pk.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	if !pk.Encrypted {
		return nil
	}

	key, err := keyCache.GetOrComputeDerivedKey(passphrase, pk.s2kParams, pk.cipher.KeySize())
	if err != nil {
		return err
	}
	return pk.decrypt(key)
}

// Decrypt decrypts an encrypted private key using a passphrase.
func (pk *PrivateKey) Decrypt(passphrase []byte) error {
	if pk.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	if !pk.Encrypted {
		return nil
	}

	key := make([]byte, pk.cipher.KeySize())
	pk.s2k(key, passphrase)
	return pk.decrypt(key)
}

// DecryptPrivateKeys decrypts all encrypted keys with the given config and passphrase.
// Avoids recomputation of similar s2k key derivations. 
func DecryptPrivateKeys(keys []*PrivateKey, passphrase []byte) error {
	// Create a cache to avoid recomputation of key derviations for the same passphrase.
	s2kCache := &s2k.Cache{}
	for _, key := range keys {
		if key != nil && !key.Dummy() && key.Encrypted {
			err := key.decryptWithCache(passphrase, s2kCache)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// encrypt encrypts an unencrypted private key.
func (pk *PrivateKey) encrypt(key []byte, params *s2k.Params, cipherFunction CipherFunction) error {
	if pk.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	if pk.Encrypted {
		return nil
	}
	// check if encryptionKey has the correct size
	if len(key) != cipherFunction.KeySize() {
		return errors.InvalidArgumentError("supplied encryption key has the wrong size")
	}
	
	priv := bytes.NewBuffer(nil)
	err := pk.serializePrivateKey(priv)
	if err != nil {
		return err
	}

	pk.cipher = cipherFunction
	pk.s2kParams = params
	pk.s2k, err = pk.s2kParams.Function()
	if err != nil {
		return err
	} 

	privateKeyBytes := priv.Bytes()
	pk.sha1Checksum = true
	block := pk.cipher.new(key)
	pk.iv = make([]byte, pk.cipher.blockSize())
	_, err = rand.Read(pk.iv)
	if err != nil {
		return err
	}
	cfb := cipher.NewCFBEncrypter(block, pk.iv)

	if pk.sha1Checksum {
		pk.s2kType = S2KSHA1
		h := sha1.New()
		h.Write(privateKeyBytes)
		sum := h.Sum(nil)
		privateKeyBytes = append(privateKeyBytes, sum...)
	} else {
		pk.s2kType = S2KCHECKSUM
		var sum uint16
		for _, b := range privateKeyBytes {
			sum += uint16(b)
		}
		priv.Write([]byte{uint8(sum >> 8), uint8(sum)})
	}

	pk.encryptedData = make([]byte, len(privateKeyBytes))
	cfb.XORKeyStream(pk.encryptedData, privateKeyBytes)
	pk.Encrypted = true
	pk.PrivateKey = nil
	return err
}

// EncryptWithConfig encrypts an unencrypted private key using the passphrase and the config.
func (pk *PrivateKey) EncryptWithConfig(passphrase []byte, config *Config) error {
	params, err := s2k.Generate(config.Random(), config.S2K())
	if err != nil {
		return err
	}
	// Derive an encryption key with the configured s2k function.
	key := make([]byte, config.Cipher().KeySize())
	s2k, err := params.Function()
	if err != nil {
		return err
	}
	s2k(key, passphrase)
	// Encrypt the private key with the derived encryption key.
	return pk.encrypt(key, params, config.Cipher())
}

// EncryptPrivateKeys encrypts all unencrypted keys with the given config and passphrase.
// Only derives one key from the passphrase, which is then used to encrypt each key.
func EncryptPrivateKeys(keys []*PrivateKey, passphrase []byte, config *Config) error {
	params, err := s2k.Generate(config.Random(), config.S2K())
	if err != nil {
		return err
	}
	// Derive an encryption key with the configured s2k function.
	encryptionKey := make([]byte, config.Cipher().KeySize())
	s2k, err := params.Function()
	if err != nil {
		return err
	}
	s2k(encryptionKey, passphrase)
	for _, key := range keys {
		if key != nil && !key.Dummy() && !key.Encrypted {
			err = key.encrypt(encryptionKey, params, config.Cipher())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Encrypt encrypts an unencrypted private key using a passphrase.
func (pk *PrivateKey) Encrypt(passphrase []byte) error {
	// Default config of private key encryption
	config := &Config{
		S2KConfig: &s2k.Config{
			S2KMode:  s2k.IteratedSaltedS2K,
			S2KCount: 65536,
			Hash:     crypto.SHA256,
		} ,
		DefaultCipher: CipherAES256,
	}
	return pk.EncryptWithConfig(passphrase, config)
}

func (pk *PrivateKey) serializePrivateKey(w io.Writer) (err error) {
	switch priv := pk.PrivateKey.(type) {
	case *rsa.PrivateKey:
		err = serializeRSAPrivateKey(w, priv)
	case *dsa.PrivateKey:
		err = serializeDSAPrivateKey(w, priv)
	case *elgamal.PrivateKey:
		err = serializeElGamalPrivateKey(w, priv)
	case *ecdsa.PrivateKey:
		err = serializeECDSAPrivateKey(w, priv)
	case *eddsa.PrivateKey:
		err = serializeEdDSAPrivateKey(w, priv)
	case *ecdh.PrivateKey:
		err = serializeECDHPrivateKey(w, priv)
	default:
		err = errors.InvalidArgumentError("unknown private key type")
	}
	return
}

func (pk *PrivateKey) parsePrivateKey(data []byte) (err error) {
	switch pk.PublicKey.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly, PubKeyAlgoRSAEncryptOnly:
		return pk.parseRSAPrivateKey(data)
	case PubKeyAlgoDSA:
		return pk.parseDSAPrivateKey(data)
	case PubKeyAlgoElGamal:
		return pk.parseElGamalPrivateKey(data)
	case PubKeyAlgoECDSA:
		return pk.parseECDSAPrivateKey(data)
	case PubKeyAlgoECDH:
		return pk.parseECDHPrivateKey(data)
	case PubKeyAlgoEdDSA:
		return pk.parseEdDSAPrivateKey(data)
	}
	panic("impossible")
}

func (pk *PrivateKey) parseRSAPrivateKey(data []byte) (err error) {
	rsaPub := pk.PublicKey.PublicKey.(*rsa.PublicKey)
	rsaPriv := new(rsa.PrivateKey)
	rsaPriv.PublicKey = *rsaPub

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	p := new(encoding.MPI)
	if _, err := p.ReadFrom(buf); err != nil {
		return err
	}

	q := new(encoding.MPI)
	if _, err := q.ReadFrom(buf); err != nil {
		return err
	}

	rsaPriv.D = new(big.Int).SetBytes(d.Bytes())
	rsaPriv.Primes = make([]*big.Int, 2)
	rsaPriv.Primes[0] = new(big.Int).SetBytes(p.Bytes())
	rsaPriv.Primes[1] = new(big.Int).SetBytes(q.Bytes())
	if err := rsaPriv.Validate(); err != nil {
		return errors.KeyInvalidError(err.Error())
	}
	rsaPriv.Precompute()
	pk.PrivateKey = rsaPriv

	return nil
}

func (pk *PrivateKey) parseDSAPrivateKey(data []byte) (err error) {
	dsaPub := pk.PublicKey.PublicKey.(*dsa.PublicKey)
	dsaPriv := new(dsa.PrivateKey)
	dsaPriv.PublicKey = *dsaPub

	buf := bytes.NewBuffer(data)
	x := new(encoding.MPI)
	if _, err := x.ReadFrom(buf); err != nil {
		return err
	}

	dsaPriv.X = new(big.Int).SetBytes(x.Bytes())
	if err := validateDSAParameters(dsaPriv); err != nil {
		return err
	}
	pk.PrivateKey = dsaPriv

	return nil
}

func (pk *PrivateKey) parseElGamalPrivateKey(data []byte) (err error) {
	pub := pk.PublicKey.PublicKey.(*elgamal.PublicKey)
	priv := new(elgamal.PrivateKey)
	priv.PublicKey = *pub

	buf := bytes.NewBuffer(data)
	x := new(encoding.MPI)
	if _, err := x.ReadFrom(buf); err != nil {
		return err
	}

	priv.X = new(big.Int).SetBytes(x.Bytes())
	if err := validateElGamalParameters(priv); err != nil {
		return err
	}
	pk.PrivateKey = priv

	return nil
}

func (pk *PrivateKey) parseECDSAPrivateKey(data []byte) (err error) {
	ecdsaPub := pk.PublicKey.PublicKey.(*ecdsa.PublicKey)
	ecdsaPriv := ecdsa.NewPrivateKey(*ecdsaPub)

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	if err := ecdsaPriv.UnmarshalIntegerSecret(d.Bytes()); err != nil {
		return err
	}
	if err := ecdsa.Validate(ecdsaPriv); err != nil {
		return err
	}
	pk.PrivateKey = ecdsaPriv

	return nil
}

func (pk *PrivateKey) parseECDHPrivateKey(data []byte) (err error) {
	ecdhPub := pk.PublicKey.PublicKey.(*ecdh.PublicKey)
	ecdhPriv := ecdh.NewPrivateKey(*ecdhPub)

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	if err := ecdhPriv.UnmarshalByteSecret(d.Bytes()); err != nil {
		return err
	}

	if err := ecdh.Validate(ecdhPriv); err != nil {
		return err
	}

	pk.PrivateKey = ecdhPriv

	return nil
}

func (pk *PrivateKey) parseEdDSAPrivateKey(data []byte) (err error) {
	eddsaPub := pk.PublicKey.PublicKey.(*eddsa.PublicKey)
	eddsaPriv := eddsa.NewPrivateKey(*eddsaPub)
	eddsaPriv.PublicKey = *eddsaPub

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	if err = eddsaPriv.UnmarshalByteSecret(d.Bytes()); err != nil {
		return err
	}

	if err := eddsa.Validate(eddsaPriv); err != nil {
		return err
	}

	pk.PrivateKey = eddsaPriv

	return nil
}

func validateDSAParameters(priv *dsa.PrivateKey) error {
	p := priv.P // group prime
	q := priv.Q // subgroup order
	g := priv.G // g has order q mod p
	x := priv.X // secret
	y := priv.Y // y == g**x mod p
	one := big.NewInt(1)
	// expect g, y >= 2 and g < p
	if g.Cmp(one) <= 0 || y.Cmp(one) <= 0 || g.Cmp(p) > 0 {
		return errors.KeyInvalidError("dsa: invalid group")
	}
	// expect p > q
	if p.Cmp(q) <= 0 {
		return errors.KeyInvalidError("dsa: invalid group prime")
	}
	// q should be large enough and divide p-1
	pSub1 := new(big.Int).Sub(p, one)
	if q.BitLen() < 150 || new(big.Int).Mod(pSub1, q).Cmp(big.NewInt(0)) != 0 {
		return errors.KeyInvalidError("dsa: invalid order")
	}
	// confirm that g has order q mod p
	if !q.ProbablyPrime(32) || new(big.Int).Exp(g, q, p).Cmp(one) != 0 {
		return errors.KeyInvalidError("dsa: invalid order")
	}
	// check y
	if new(big.Int).Exp(g, x, p).Cmp(y) != 0 {
		return errors.KeyInvalidError("dsa: mismatching values")
	}

	return nil
}

func validateElGamalParameters(priv *elgamal.PrivateKey) error {
	p := priv.P // group prime
	g := priv.G // g has order p-1 mod p
	x := priv.X // secret
	y := priv.Y // y == g**x mod p
	one := big.NewInt(1)
	// Expect g, y >= 2 and g < p
	if g.Cmp(one) <= 0 || y.Cmp(one) <= 0 || g.Cmp(p) > 0 {
		return errors.KeyInvalidError("elgamal: invalid group")
	}
	if p.BitLen() < 1024 {
		return errors.KeyInvalidError("elgamal: group order too small")
	}
	pSub1 := new(big.Int).Sub(p, one)
	if new(big.Int).Exp(g, pSub1, p).Cmp(one) != 0 {
		return errors.KeyInvalidError("elgamal: invalid group")
	}
	// Since p-1 is not prime, g might have a smaller order that divides p-1.
	// We cannot confirm the exact order of g, but we make sure it is not too small.
	gExpI := new(big.Int).Set(g)
	i := 1
	threshold := 2 << 17 // we want order > threshold
	for i < threshold {
		i++ // we check every order to make sure key validation is not easily bypassed by guessing y'
		gExpI.Mod(new(big.Int).Mul(gExpI, g), p)
		if gExpI.Cmp(one) == 0 {
			return errors.KeyInvalidError("elgamal: order too small")
		}
	}
	// Check y
	if new(big.Int).Exp(g, x, p).Cmp(y) != 0 {
		return errors.KeyInvalidError("elgamal: mismatching values")
	}

	return nil
}
