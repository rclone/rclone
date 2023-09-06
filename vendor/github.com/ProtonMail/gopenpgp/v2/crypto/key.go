package crypto

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/armor"
	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/pkg/errors"

	openpgp "github.com/ProtonMail/go-crypto/openpgp"
	packet "github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Key contains a single private or public key.
type Key struct {
	// PGP entities in this keyring.
	entity *openpgp.Entity
}

// --- Create Key object

// NewKeyFromArmoredReader reads an armored data into a key.
func NewKeyFromArmoredReader(r io.Reader) (key *Key, err error) {
	key = &Key{}
	err = key.readFrom(r, true)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// NewKeyFromReader reads binary data into a Key object.
func NewKeyFromReader(r io.Reader) (key *Key, err error) {
	key = &Key{}
	err = key.readFrom(r, false)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// NewKey creates a new key from the first key in the unarmored binary data.
func NewKey(binKeys []byte) (key *Key, err error) {
	return NewKeyFromReader(bytes.NewReader(clone(binKeys)))
}

// NewKeyFromArmored creates a new key from the first key in an armored string.
func NewKeyFromArmored(armored string) (key *Key, err error) {
	return NewKeyFromArmoredReader(strings.NewReader(armored))
}

func NewKeyFromEntity(entity *openpgp.Entity) (*Key, error) {
	if entity == nil {
		return nil, errors.New("gopenpgp: nil entity provided")
	}
	return &Key{entity: entity}, nil
}

// GenerateRSAKeyWithPrimes generates a RSA key using the given primes.
func GenerateRSAKeyWithPrimes(
	name, email string,
	bits int,
	primeone, primetwo, primethree, primefour []byte,
) (*Key, error) {
	return generateKey(name, email, "rsa", bits, primeone, primetwo, primethree, primefour)
}

// GenerateKey generates a key of the given keyType ("rsa" or "x25519").
// If keyType is "rsa", bits is the RSA bitsize of the key.
// If keyType is "x25519" bits is unused.
func GenerateKey(name, email string, keyType string, bits int) (*Key, error) {
	return generateKey(name, email, keyType, bits, nil, nil, nil, nil)
}

// --- Operate on key

// Copy creates a deep copy of the key.
func (key *Key) Copy() (*Key, error) {
	serialized, err := key.Serialize()
	if err != nil {
		return nil, err
	}

	return NewKey(serialized)
}

// Lock locks a copy of the key.
func (key *Key) Lock(passphrase []byte) (*Key, error) {
	unlocked, err := key.IsUnlocked()
	if err != nil {
		return nil, err
	}

	if !unlocked {
		return nil, errors.New("gopenpgp: key is not unlocked")
	}

	lockedKey, err := key.Copy()
	if err != nil {
		return nil, err
	}

	if passphrase == nil {
		return lockedKey, nil
	}

	if lockedKey.entity.PrivateKey != nil && !lockedKey.entity.PrivateKey.Dummy() {
		err = lockedKey.entity.PrivateKey.Encrypt(passphrase)
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: error in locking key")
		}
	}

	for _, sub := range lockedKey.entity.Subkeys {
		if sub.PrivateKey != nil && !sub.PrivateKey.Dummy() {
			if err := sub.PrivateKey.Encrypt(passphrase); err != nil {
				return nil, errors.Wrap(err, "gopenpgp: error in locking sub key")
			}
		}
	}

	locked, err := lockedKey.IsLocked()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.New("gopenpgp: unable to lock key")
	}

	return lockedKey, nil
}

// Unlock unlocks a copy of the key.
func (key *Key) Unlock(passphrase []byte) (*Key, error) {
	isLocked, err := key.IsLocked()
	if err != nil {
		return nil, err
	}

	if !isLocked {
		if passphrase == nil {
			return key.Copy()
		}
		return nil, errors.New("gopenpgp: key is not locked")
	}

	unlockedKey, err := key.Copy()
	if err != nil {
		return nil, err
	}

	if unlockedKey.entity.PrivateKey != nil && !unlockedKey.entity.PrivateKey.Dummy() {
		err = unlockedKey.entity.PrivateKey.Decrypt(passphrase)
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: error in unlocking key")
		}
	}

	for _, sub := range unlockedKey.entity.Subkeys {
		if sub.PrivateKey != nil && !sub.PrivateKey.Dummy() {
			if err := sub.PrivateKey.Decrypt(passphrase); err != nil {
				return nil, errors.Wrap(err, "gopenpgp: error in unlocking sub key")
			}
		}
	}

	isUnlocked, err := unlockedKey.IsUnlocked()
	if err != nil {
		return nil, err
	}
	if !isUnlocked {
		return nil, errors.New("gopenpgp: unable to unlock key")
	}

	return unlockedKey, nil
}

// --- Export key

func (key *Key) Serialize() ([]byte, error) {
	var buffer bytes.Buffer
	var err error

	if key.entity.PrivateKey == nil {
		err = key.entity.Serialize(&buffer)
	} else {
		err = key.entity.SerializePrivateWithoutSigning(&buffer, nil)
	}

	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in serializing key")
	}

	return buffer.Bytes(), nil
}

// Armor returns the armored key as a string with default gopenpgp headers.
func (key *Key) Armor() (string, error) {
	serialized, err := key.Serialize()
	if err != nil {
		return "", err
	}

	if key.IsPrivate() {
		return armor.ArmorWithType(serialized, constants.PrivateKeyHeader)
	}

	return armor.ArmorWithType(serialized, constants.PublicKeyHeader)
}

// ArmorWithCustomHeaders returns the armored key as a string, with
// the given headers. Empty parameters are omitted from the headers.
func (key *Key) ArmorWithCustomHeaders(comment, version string) (string, error) {
	serialized, err := key.Serialize()
	if err != nil {
		return "", err
	}

	return armor.ArmorWithTypeAndCustomHeaders(serialized, constants.PrivateKeyHeader, version, comment)
}

// GetArmoredPublicKey returns the armored public keys from this keyring.
func (key *Key) GetArmoredPublicKey() (s string, err error) {
	serialized, err := key.GetPublicKey()
	if err != nil {
		return "", err
	}

	return armor.ArmorWithType(serialized, constants.PublicKeyHeader)
}

// GetArmoredPublicKeyWithCustomHeaders returns the armored public key as a string, with
// the given headers. Empty parameters are omitted from the headers.
func (key *Key) GetArmoredPublicKeyWithCustomHeaders(comment, version string) (string, error) {
	serialized, err := key.GetPublicKey()
	if err != nil {
		return "", err
	}

	return armor.ArmorWithTypeAndCustomHeaders(serialized, constants.PublicKeyHeader, version, comment)
}

// GetPublicKey returns the unarmored public keys from this keyring.
func (key *Key) GetPublicKey() (b []byte, err error) {
	var outBuf bytes.Buffer
	if err = key.entity.Serialize(&outBuf); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in serializing public key")
	}

	return outBuf.Bytes(), nil
}

// --- Key object properties

// CanVerify returns true if any of the subkeys can be used for verification.
func (key *Key) CanVerify() bool {
	_, canVerify := key.entity.SigningKey(getNow())
	return canVerify
}

// CanEncrypt returns true if any of the subkeys can be used for encryption.
func (key *Key) CanEncrypt() bool {
	_, canEncrypt := key.entity.EncryptionKey(getNow())
	return canEncrypt
}

// IsExpired checks whether the key is expired.
func (key *Key) IsExpired() bool {
	i := key.entity.PrimaryIdentity()
	return key.entity.PrimaryKey.KeyExpired(i.SelfSignature, getNow()) || // primary key has expired
		i.SelfSignature.SigExpired(getNow()) // user ID self-signature has expired
}

// IsRevoked checks whether the key or the primary identity has a valid revocation signature.
func (key *Key) IsRevoked() bool {
	return key.entity.Revoked(getNow()) || key.entity.PrimaryIdentity().Revoked(getNow())
}

// IsPrivate returns true if the key is private.
func (key *Key) IsPrivate() bool {
	return key.entity.PrivateKey != nil
}

// IsLocked checks if a private key is locked.
func (key *Key) IsLocked() (bool, error) {
	if key.entity.PrivateKey == nil {
		return true, errors.New("gopenpgp: a public key cannot be locked")
	}

	encryptedKeys := 0

	for _, sub := range key.entity.Subkeys {
		if sub.PrivateKey != nil && !sub.PrivateKey.Dummy() && sub.PrivateKey.Encrypted {
			encryptedKeys++
		}
	}

	if key.entity.PrivateKey.Encrypted {
		encryptedKeys++
	}

	return encryptedKeys > 0, nil
}

// IsUnlocked checks if a private key is unlocked.
func (key *Key) IsUnlocked() (bool, error) {
	if key.entity.PrivateKey == nil {
		return true, errors.New("gopenpgp: a public key cannot be unlocked")
	}

	encryptedKeys := 0

	for _, sub := range key.entity.Subkeys {
		if sub.PrivateKey != nil && !sub.PrivateKey.Dummy() && sub.PrivateKey.Encrypted {
			encryptedKeys++
		}
	}

	if key.entity.PrivateKey.Encrypted {
		encryptedKeys++
	}

	return encryptedKeys == 0, nil
}

// Check verifies if the public keys match the private key parameters by
// signing and verifying.
// Deprecated: all keys are now checked on parsing.
func (key *Key) Check() (bool, error) {
	return true, nil
}

// PrintFingerprints is a debug helper function that prints the key and subkey fingerprints.
func (key *Key) PrintFingerprints() {
	for _, subKey := range key.entity.Subkeys {
		if !subKey.Sig.FlagsValid || subKey.Sig.FlagEncryptStorage || subKey.Sig.FlagEncryptCommunications {
			fmt.Println("SubKey:" + hex.EncodeToString(subKey.PublicKey.Fingerprint))
		}
	}
	fmt.Println("PrimaryKey:" + hex.EncodeToString(key.entity.PrimaryKey.Fingerprint))
}

// GetHexKeyID returns the key ID, hex encoded as a string.
func (key *Key) GetHexKeyID() string {
	return keyIDToHex(key.GetKeyID())
}

// GetKeyID returns the key ID, encoded as 8-byte int.
func (key *Key) GetKeyID() uint64 {
	return key.entity.PrimaryKey.KeyId
}

// GetFingerprint gets the fingerprint from the key.
func (key *Key) GetFingerprint() string {
	return hex.EncodeToString(key.entity.PrimaryKey.Fingerprint)
}

// GetSHA256Fingerprints computes the SHA256 fingerprints of the key and subkeys.
func (key *Key) GetSHA256Fingerprints() (fingerprints []string) {
	fingerprints = append(fingerprints, hex.EncodeToString(getSHA256FingerprintBytes(key.entity.PrimaryKey)))
	for _, sub := range key.entity.Subkeys {
		fingerprints = append(fingerprints, hex.EncodeToString(getSHA256FingerprintBytes(sub.PublicKey)))
	}
	return
}

// GetEntity gets x/crypto Entity object.
func (key *Key) GetEntity() *openpgp.Entity {
	return key.entity
}

// ToPublic returns the corresponding public key of the given private key.
func (key *Key) ToPublic() (publicKey *Key, err error) {
	if !key.IsPrivate() {
		return nil, errors.New("gopenpgp: key is already public")
	}

	publicKey, err = key.Copy()
	if err != nil {
		return nil, err
	}

	publicKey.ClearPrivateParams()
	return
}

// --- Internal methods

// getSHA256FingerprintBytes computes the SHA256 fingerprint of a public key
// object.
func getSHA256FingerprintBytes(pk *packet.PublicKey) []byte {
	fingerPrint := sha256.New()

	// Hashing can't return an error, and has already been done when parsing the key,
	// hence the error is nil
	_ = pk.SerializeForHash(fingerPrint)
	return fingerPrint.Sum(nil)
}

// readFrom reads unarmored and armored keys from r and adds them to the keyring.
func (key *Key) readFrom(r io.Reader, armored bool) error {
	var err error
	var entities openpgp.EntityList
	if armored {
		entities, err = openpgp.ReadArmoredKeyRing(r)
	} else {
		entities, err = openpgp.ReadKeyRing(r)
	}
	if err != nil {
		return errors.Wrap(err, "gopenpgp: error in reading key ring")
	}

	if len(entities) > 1 {
		return errors.New("gopenpgp: the key contains too many entities")
	}

	if len(entities) == 0 {
		return errors.New("gopenpgp: the key does not contain any entity")
	}

	key.entity = entities[0]
	return nil
}

func generateKey(
	name, email string,
	keyType string,
	bits int,
	prime1, prime2, prime3, prime4 []byte,
) (*Key, error) {
	if len(email) == 0 && len(name) == 0 {
		return nil, errors.New("gopenpgp: neither name nor email set.")
	}

	comments := ""

	cfg := &packet.Config{
		Algorithm:              packet.PubKeyAlgoRSA,
		RSABits:                bits,
		Time:                   getKeyGenerationTimeGenerator(),
		DefaultHash:            crypto.SHA256,
		DefaultCipher:          packet.CipherAES256,
		DefaultCompressionAlgo: packet.CompressionZLIB,
	}

	if keyType == "x25519" {
		cfg.Algorithm = packet.PubKeyAlgoEdDSA
	}

	if prime1 != nil && prime2 != nil && prime3 != nil && prime4 != nil {
		var bigPrimes [4]*big.Int
		bigPrimes[0] = new(big.Int)
		bigPrimes[0].SetBytes(prime1)
		bigPrimes[1] = new(big.Int)
		bigPrimes[1].SetBytes(prime2)
		bigPrimes[2] = new(big.Int)
		bigPrimes[2].SetBytes(prime3)
		bigPrimes[3] = new(big.Int)
		bigPrimes[3].SetBytes(prime4)

		cfg.RSAPrimes = bigPrimes[:]
	}

	newEntity, err := openpgp.NewEntity(name, comments, email, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "gopengpp: error in encoding new entity")
	}

	if newEntity.PrivateKey == nil {
		return nil, errors.New("gopenpgp: error in generating private key")
	}

	return NewKeyFromEntity(newEntity)
}

// keyIDToHex casts a keyID to hex with the correct padding.
func keyIDToHex(keyID uint64) string {
	return fmt.Sprintf("%016v", strconv.FormatUint(keyID, 16))
}
