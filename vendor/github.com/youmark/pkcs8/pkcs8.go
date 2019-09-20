// Package pkcs8 implements functions to parse and convert private keys in PKCS#8 format, as defined in RFC5208 and RFC5958
package pkcs8

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

// Copy from crypto/x509
var (
	oidPublicKeyRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidPublicKeyDSA   = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 1}
	oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
)

// Copy from crypto/x509
var (
	oidNamedCurveP224 = asn1.ObjectIdentifier{1, 3, 132, 0, 33}
	oidNamedCurveP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidNamedCurveP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidNamedCurveP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

// Copy from crypto/x509
func oidFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case elliptic.P224():
		return oidNamedCurveP224, true
	case elliptic.P256():
		return oidNamedCurveP256, true
	case elliptic.P384():
		return oidNamedCurveP384, true
	case elliptic.P521():
		return oidNamedCurveP521, true
	}

	return nil, false
}

// Unecrypted PKCS8
var (
	oidPKCS5PBKDF2    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidAES256CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
	oidAES128CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 2}
	oidHMACWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidDESEDE3CBC     = asn1.ObjectIdentifier{1, 2, 840, 113549, 3, 7}
)

type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}

type privateKeyInfo struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

// Encrypted PKCS8
type prfParam struct {
	IdPRF     asn1.ObjectIdentifier
	NullParam asn1.RawValue
}

type pbkdf2Params struct {
	Salt           []byte
	IterationCount int
	PrfParam       prfParam `asn1:"optional"`
}

type pbkdf2Algorithms struct {
	IdPBKDF2     asn1.ObjectIdentifier
	PBKDF2Params pbkdf2Params
}

type pbkdf2Encs struct {
	EncryAlgo asn1.ObjectIdentifier
	IV        []byte
}

type pbes2Params struct {
	KeyDerivationFunc pbkdf2Algorithms
	EncryptionScheme  pbkdf2Encs
}

type pbes2Algorithms struct {
	IdPBES2     asn1.ObjectIdentifier
	PBES2Params pbes2Params
}

type encryptedPrivateKeyInfo struct {
	EncryptionAlgorithm pbes2Algorithms
	EncryptedData       []byte
}

// ParsePKCS8PrivateKeyRSA parses encrypted/unencrypted private keys in PKCS#8 format. To parse encrypted private keys, a password of []byte type should be provided to the function as the second parameter.
//
// The function can decrypt the private key encrypted with AES-256-CBC mode, and stored in PKCS #5 v2.0 format.
func ParsePKCS8PrivateKeyRSA(der []byte, v ...[]byte) (*rsa.PrivateKey, error) {
	key, err := ParsePKCS8PrivateKey(der, v...)
	if err != nil {
		return nil, err
	}
	typedKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key block is not of type RSA")
	}
	return typedKey, nil
}

// ParsePKCS8PrivateKeyECDSA parses encrypted/unencrypted private keys in PKCS#8 format. To parse encrypted private keys, a password of []byte type should be provided to the function as the second parameter.
//
// The function can decrypt the private key encrypted with AES-256-CBC mode, and stored in PKCS #5 v2.0 format.
func ParsePKCS8PrivateKeyECDSA(der []byte, v ...[]byte) (*ecdsa.PrivateKey, error) {
	key, err := ParsePKCS8PrivateKey(der, v...)
	if err != nil {
		return nil, err
	}
	typedKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key block is not of type ECDSA")
	}
	return typedKey, nil
}

// ParsePKCS8PrivateKey parses encrypted/unencrypted private keys in PKCS#8 format. To parse encrypted private keys, a password of []byte type should be provided to the function as the second parameter.
//
// The function can decrypt the private key encrypted with AES-256-CBC mode, and stored in PKCS #5 v2.0 format.
func ParsePKCS8PrivateKey(der []byte, v ...[]byte) (interface{}, error) {
	// No password provided, assume the private key is unencrypted
	if v == nil {
		return x509.ParsePKCS8PrivateKey(der)
	}

	// Use the password provided to decrypt the private key
	password := v[0]
	var privKey encryptedPrivateKeyInfo
	if _, err := asn1.Unmarshal(der, &privKey); err != nil {
		return nil, errors.New("pkcs8: only PKCS #5 v2.0 supported")
	}

	if !privKey.EncryptionAlgorithm.IdPBES2.Equal(oidPBES2) {
		return nil, errors.New("pkcs8: only PBES2 supported")
	}

	if !privKey.EncryptionAlgorithm.PBES2Params.KeyDerivationFunc.IdPBKDF2.Equal(oidPKCS5PBKDF2) {
		return nil, errors.New("pkcs8: only PBKDF2 supported")
	}

	encParam := privKey.EncryptionAlgorithm.PBES2Params.EncryptionScheme
	kdfParam := privKey.EncryptionAlgorithm.PBES2Params.KeyDerivationFunc.PBKDF2Params

	iv := encParam.IV
	salt := kdfParam.Salt
	iter := kdfParam.IterationCount
	keyHash := sha1.New
	if kdfParam.PrfParam.IdPRF.Equal(oidHMACWithSHA256) {
		keyHash = sha256.New
	}

	encryptedKey := privKey.EncryptedData
	var symkey []byte
	var block cipher.Block
	var err error
	switch {
	case encParam.EncryAlgo.Equal(oidAES128CBC):
		symkey = pbkdf2.Key(password, salt, iter, 16, keyHash)
		block, err = aes.NewCipher(symkey)
	case encParam.EncryAlgo.Equal(oidAES256CBC):
		symkey = pbkdf2.Key(password, salt, iter, 32, keyHash)
		block, err = aes.NewCipher(symkey)
	case encParam.EncryAlgo.Equal(oidDESEDE3CBC):
		symkey = pbkdf2.Key(password, salt, iter, 24, keyHash)
		block, err = des.NewTripleDESCipher(symkey)
	default:
		return nil, errors.New("pkcs8: only AES-256-CBC, AES-128-CBC and DES-EDE3-CBC are supported")
	}
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encryptedKey, encryptedKey)

	key, err := x509.ParsePKCS8PrivateKey(encryptedKey)
	if err != nil {
		return nil, errors.New("pkcs8: incorrect password")
	}
	return key, nil
}

func convertPrivateKeyToPKCS8(priv interface{}) ([]byte, error) {
	var pkey privateKeyInfo

	switch priv := priv.(type) {
	case *ecdsa.PrivateKey:
		eckey, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			return nil, err
		}

		oidNamedCurve, ok := oidFromNamedCurve(priv.Curve)
		if !ok {
			return nil, errors.New("pkcs8: unknown elliptic curve")
		}

		// Per RFC5958, if publicKey is present, then version is set to v2(1) else version is set to v1(0).
		// But openssl set to v1 even publicKey is present
		pkey.Version = 1
		pkey.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 2)
		pkey.PrivateKeyAlgorithm[0] = oidPublicKeyECDSA
		pkey.PrivateKeyAlgorithm[1] = oidNamedCurve
		pkey.PrivateKey = eckey
	case *rsa.PrivateKey:

		// Per RFC5958, if publicKey is present, then version is set to v2(1) else version is set to v1(0).
		// But openssl set to v1 even publicKey is present
		pkey.Version = 0
		pkey.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 1)
		pkey.PrivateKeyAlgorithm[0] = oidPublicKeyRSA
		pkey.PrivateKey = x509.MarshalPKCS1PrivateKey(priv)
	default:
		return nil, fmt.Errorf("unsupported key type: %T", priv)
	}

	return asn1.Marshal(pkey)
}

func convertPrivateKeyToPKCS8Encrypted(priv interface{}, password []byte) ([]byte, error) {
	// Convert private key into PKCS8 format
	pkey, err := convertPrivateKeyToPKCS8(priv)
	if err != nil {
		return nil, err
	}

	// Calculate key from password based on PKCS5 algorithm
	// Use 8 byte salt, 16 byte IV, and 2048 iteration
	iter := 2048
	salt := make([]byte, 8)
	iv := make([]byte, 16)
	_, err = rand.Read(salt)
	if err != nil {
		return nil, err
	}
	_, err = rand.Read(iv)
	if err != nil {
		return nil, err
	}

	key := pbkdf2.Key(password, salt, iter, 32, sha256.New)

	// Use AES256-CBC mode, pad plaintext with PKCS5 padding scheme
	padding := aes.BlockSize - len(pkey)%aes.BlockSize
	if padding > 0 {
		n := len(pkey)
		pkey = append(pkey, make([]byte, padding)...)
		for i := 0; i < padding; i++ {
			pkey[n+i] = byte(padding)
		}
	}

	encryptedKey := make([]byte, len(pkey))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encryptedKey, pkey)

	//	pbkdf2algo := pbkdf2Algorithms{oidPKCS5PBKDF2, pbkdf2Params{salt, iter, prfParam{oidHMACWithSHA256}}}

	pbkdf2algo := pbkdf2Algorithms{oidPKCS5PBKDF2, pbkdf2Params{salt, iter, prfParam{oidHMACWithSHA256, asn1.RawValue{Tag: asn1.TagNull}}}}
	pbkdf2encs := pbkdf2Encs{oidAES256CBC, iv}
	pbes2algo := pbes2Algorithms{oidPBES2, pbes2Params{pbkdf2algo, pbkdf2encs}}

	encryptedPkey := encryptedPrivateKeyInfo{pbes2algo, encryptedKey}

	return asn1.Marshal(encryptedPkey)
}

// ConvertPrivateKeyToPKCS8 converts the private key into PKCS#8 format.
// To encrypt the private key, the password of []byte type should be provided as the second parameter.
//
// The only supported key types are RSA and ECDSA (*rsa.PrivateKey or *ecdsa.PrivateKey for priv)
func ConvertPrivateKeyToPKCS8(priv interface{}, v ...[]byte) ([]byte, error) {
	if v == nil {
		return convertPrivateKeyToPKCS8(priv)
	}

	password := v[0]
	return convertPrivateKeyToPKCS8Encrypted(priv, password)
}
