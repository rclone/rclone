package cryptomator

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	aeswrap "github.com/NickBall/go-aes-key-wrap"
	"golang.org/x/crypto/scrypt"
)

const (
	masterEncryptKeySize         = 32
	masterMacKeySize             = masterEncryptKeySize
	masterDefaultVersion         = 999
	masterDefaultScryptCostParam = 32 * 1024
	masterDefaultScryptBlockSize = 8
	masterDefaultScryptSaltSize  = 32
)

// masterKey is the master key for a Cryptomator vault, typically saved in masterkey.cryptomator at the root of the vault.
type masterKey struct {
	EncryptKey []byte
	MacKey     []byte
	Version    uint32
}

type encryptedMasterKey struct {
	ScryptSalt       []byte `json:"scryptSalt"`
	ScryptCostParam  int    `json:"scryptCostParam"`
	ScryptBlockSize  int    `json:"scryptBlockSize"`
	PrimaryMasterKey []byte `json:"primaryMasterKey"`
	HmacMasterKey    []byte `json:"hmacMasterKey"`

	// Deprecated: Vault format 8 no longer uses these fields.
	Version    uint32 `json:"version"`
	VersionMac []byte `json:"versionMac"`
}

// newMasterKey creates a new randomly initialized MasterKey.
func newMasterKey() (m masterKey, err error) {
	m.EncryptKey = make([]byte, masterEncryptKeySize)
	m.MacKey = make([]byte, masterMacKeySize)
	m.Version = masterDefaultVersion

	if _, err = rand.Read(m.EncryptKey); err != nil {
		return
	}

	_, err = rand.Read(m.MacKey)

	return
}

func calcVersionMac(m *masterKey, em *encryptedMasterKey) []byte {
	hash := hmac.New(sha256.New, m.MacKey)
	var verBytes [4]byte
	binary.BigEndian.PutUint32(verBytes[:], em.Version)
	_, err := hash.Write(verBytes[:])
	if err != nil {
		panic(err)
	}
	return hash.Sum(nil)
}

// marshal encrypts the MasterKey with a passphrase and writes it.
func (m masterKey) marshal(w io.Writer, passphrase string) (err error) {
	encKey := encryptedMasterKey{
		Version:         m.Version,
		ScryptCostParam: masterDefaultScryptCostParam,
		ScryptBlockSize: masterDefaultScryptBlockSize,
	}

	encKey.ScryptSalt = make([]byte, masterDefaultScryptSaltSize)
	if _, err = rand.Read(encKey.ScryptSalt); err != nil {
		return
	}

	kek, err := scrypt.Key([]byte(passphrase), encKey.ScryptSalt, encKey.ScryptCostParam, encKey.ScryptBlockSize, 1, masterEncryptKeySize)
	if err != nil {
		return
	}

	cipher, err := aes.NewCipher(kek)
	if err != nil {
		return
	}

	if encKey.PrimaryMasterKey, err = aeswrap.Wrap(cipher, m.EncryptKey); err != nil {
		return
	}
	if encKey.HmacMasterKey, err = aeswrap.Wrap(cipher, m.MacKey); err != nil {
		return
	}

	encKey.VersionMac = calcVersionMac(&m, &encKey)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	err = enc.Encode(encKey)

	return
}

// unmarshalMasterKey reads the master key and decrypts it with a passphrase.
func unmarshalMasterKey(r io.Reader, passphrase string) (m masterKey, err error) {
	encKey := &encryptedMasterKey{}

	if err = json.NewDecoder(r).Decode(encKey); err != nil {
		err = fmt.Errorf("failed to parse master key json: %w", err)
		return
	}

	kek, err := scrypt.Key([]byte(passphrase), encKey.ScryptSalt, encKey.ScryptCostParam, encKey.ScryptBlockSize, 1, masterEncryptKeySize)
	if err != nil {
		return
	}

	cipher, err := aes.NewCipher(kek)
	if err != nil {
		return
	}

	if m.EncryptKey, err = aeswrap.Unwrap(cipher, encKey.PrimaryMasterKey); err != nil {
		err = fmt.Errorf("failed to unwrap primary key: %w", err)
		return
	}
	if m.MacKey, err = aeswrap.Unwrap(cipher, encKey.HmacMasterKey); err != nil {
		err = fmt.Errorf("failed to unwrap hmac key: %w", err)
		return
	}

	if !hmac.Equal(encKey.VersionMac, calcVersionMac(&m, encKey)) {
		err = fmt.Errorf("failed to validate version hmac")
		return
	}
	m.Version = encKey.Version

	return
}
