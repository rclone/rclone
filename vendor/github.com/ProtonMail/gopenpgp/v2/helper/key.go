package helper

import (
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"
)

// UpdatePrivateKeyPassphrase decrypts the given armored privateKey with oldPassphrase,
// re-encrypts it with newPassphrase, and returns the new armored key.
func UpdatePrivateKeyPassphrase(
	privateKey string,
	oldPassphrase, newPassphrase []byte,
) (string, error) {
	key, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to parse key")
	}

	unlocked, err := key.Unlock(oldPassphrase)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to unlock old key")
	}
	defer unlocked.ClearPrivateParams()

	locked, err := unlocked.Lock(newPassphrase)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to lock new key")
	}

	armored, err := locked.Armor()
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to armor new key")
	}

	return armored, nil
}

// GenerateKey generates a key of the given keyType ("rsa" or "x25519"), encrypts it, and returns an armored string.
// If keyType is "rsa", bits is the RSA bitsize of the key.
// If keyType is "x25519" bits is unused.
func GenerateKey(name, email string, passphrase []byte, keyType string, bits int) (string, error) {
	key, err := crypto.GenerateKey(name, email, keyType, bits)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to generate new key")
	}
	defer key.ClearPrivateParams()

	locked, err := key.Lock(passphrase)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to lock new key")
	}

	return locked.Armor()
}

func GetSHA256Fingerprints(publicKey string) ([]string, error) {
	key, err := crypto.NewKeyFromArmored(publicKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse key")
	}

	return key.GetSHA256Fingerprints(), nil
}
