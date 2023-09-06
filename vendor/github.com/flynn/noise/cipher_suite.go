package noise

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"hash"
	"io"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// A DHKey is a keypair used for Diffie-Hellman key agreement.
type DHKey struct {
	Private []byte
	Public  []byte
}

// A DHFunc implements Diffie-Hellman key agreement.
type DHFunc interface {
	// GenerateKeypair generates a new keypair using random as a source of
	// entropy.
	GenerateKeypair(random io.Reader) (DHKey, error)

	// DH performs a Diffie-Hellman calculation between the provided private and
	// public keys and returns the result.
	DH(privkey, pubkey []byte) ([]byte, error)

	// DHLen is the number of bytes returned by DH.
	DHLen() int

	// DHName is the name of the DH function.
	DHName() string
}

// A HashFunc implements a cryptographic hash function.
type HashFunc interface {
	// Hash returns a hash state.
	Hash() hash.Hash

	// HashName is the name of the hash function.
	HashName() string
}

// A CipherFunc implements an AEAD symmetric cipher.
type CipherFunc interface {
	// Cipher initializes the algorithm with the provided key and returns a Cipher.
	Cipher(k [32]byte) Cipher

	// CipherName is the name of the cipher.
	CipherName() string
}

// A Cipher is a AEAD cipher that has been initialized with a key.
type Cipher interface {
	// Encrypt encrypts the provided plaintext with a nonce and then appends the
	// ciphertext to out along with an authentication tag over the ciphertext
	// and optional authenticated data.
	Encrypt(out []byte, n uint64, ad, plaintext []byte) []byte

	// Decrypt authenticates the ciphertext and optional authenticated data and
	// then decrypts the provided ciphertext using the provided nonce and
	// appends it to out.
	Decrypt(out []byte, n uint64, ad, ciphertext []byte) ([]byte, error)
}

// A CipherSuite is a set of cryptographic primitives used in a Noise protocol.
// It should be constructed with NewCipherSuite.
type CipherSuite interface {
	DHFunc
	CipherFunc
	HashFunc
	Name() []byte
}

// NewCipherSuite returns a CipherSuite constructed from the specified
// primitives.
func NewCipherSuite(dh DHFunc, c CipherFunc, h HashFunc) CipherSuite {
	return ciphersuite{
		DHFunc:     dh,
		CipherFunc: c,
		HashFunc:   h,
		name:       []byte(dh.DHName() + "_" + c.CipherName() + "_" + h.HashName()),
	}
}

type ciphersuite struct {
	DHFunc
	CipherFunc
	HashFunc
	name []byte
}

func (s ciphersuite) Name() []byte { return s.name }

// DH25519 is the Curve25519 ECDH function.
var DH25519 DHFunc = dh25519{}

type dh25519 struct{}

func (dh25519) GenerateKeypair(rng io.Reader) (DHKey, error) {
	privkey := make([]byte, 32)
	if rng == nil {
		rng = rand.Reader
	}
	if _, err := io.ReadFull(rng, privkey); err != nil {
		return DHKey{}, err
	}
	pubkey, err := curve25519.X25519(privkey, curve25519.Basepoint)
	if err != nil {
		return DHKey{}, err
	}
	return DHKey{Private: privkey, Public: pubkey}, nil
}

func (dh25519) DH(privkey, pubkey []byte) ([]byte, error) {
	return curve25519.X25519(privkey, pubkey)
}

func (dh25519) DHLen() int     { return 32 }
func (dh25519) DHName() string { return "25519" }

type cipherFn struct {
	fn   func([32]byte) Cipher
	name string
}

func (c cipherFn) Cipher(k [32]byte) Cipher { return c.fn(k) }
func (c cipherFn) CipherName() string       { return c.name }

// CipherAESGCM is the AES256-GCM AEAD cipher.
var CipherAESGCM CipherFunc = cipherFn{cipherAESGCM, "AESGCM"}

func cipherAESGCM(k [32]byte) Cipher {
	c, err := aes.NewCipher(k[:])
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		panic(err)
	}
	return aeadCipher{
		gcm,
		func(n uint64) []byte {
			var nonce [12]byte
			binary.BigEndian.PutUint64(nonce[4:], n)
			return nonce[:]
		},
	}
}

// CipherChaChaPoly is the ChaCha20-Poly1305 AEAD cipher construction.
var CipherChaChaPoly CipherFunc = cipherFn{cipherChaChaPoly, "ChaChaPoly"}

func cipherChaChaPoly(k [32]byte) Cipher {
	c, err := chacha20poly1305.New(k[:])
	if err != nil {
		panic(err)
	}
	return aeadCipher{
		c,
		func(n uint64) []byte {
			var nonce [12]byte
			binary.LittleEndian.PutUint64(nonce[4:], n)
			return nonce[:]
		},
	}
}

type aeadCipher struct {
	cipher.AEAD
	nonce func(uint64) []byte
}

func (c aeadCipher) Encrypt(out []byte, n uint64, ad, plaintext []byte) []byte {
	return c.Seal(out, c.nonce(n), plaintext, ad)
}

func (c aeadCipher) Decrypt(out []byte, n uint64, ad, ciphertext []byte) ([]byte, error) {
	return c.Open(out, c.nonce(n), ciphertext, ad)
}

type hashFn struct {
	fn   func() hash.Hash
	name string
}

func (h hashFn) Hash() hash.Hash  { return h.fn() }
func (h hashFn) HashName() string { return h.name }

// HashSHA256 is the SHA-256 hash function.
var HashSHA256 HashFunc = hashFn{sha256.New, "SHA256"}

// HashSHA512 is the SHA-512 hash function.
var HashSHA512 HashFunc = hashFn{sha512.New, "SHA512"}

func blake2bNew() hash.Hash {
	h, err := blake2b.New512(nil)
	if err != nil {
		panic(err)
	}
	return h
}

// HashBLAKE2b is the BLAKE2b hash function.
var HashBLAKE2b HashFunc = hashFn{blake2bNew, "BLAKE2b"}

func blake2sNew() hash.Hash {
	h, err := blake2s.New256(nil)
	if err != nil {
		panic(err)
	}
	return h
}

// HashBLAKE2s is the BLAKE2s hash function.
var HashBLAKE2s HashFunc = hashFn{blake2sNew, "BLAKE2s"}
