package helper

import (
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/internal"
	"github.com/pkg/errors"
)

// SignCleartextMessageArmored signs text given a private key and its
// passphrase, canonicalizes and trims the newlines, and returns the
// PGP-compliant special armoring.
func SignCleartextMessageArmored(privateKey string, passphrase []byte, text string) (string, error) {
	signingKey, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in creating key object")
	}

	unlockedKey, err := signingKey.Unlock(passphrase)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in unlocking key")
	}
	defer unlockedKey.ClearPrivateParams()

	keyRing, err := crypto.NewKeyRing(unlockedKey)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in creating keyring")
	}

	return SignCleartextMessage(keyRing, text)
}

// VerifyCleartextMessageArmored verifies PGP-compliant armored signed plain
// text given the public key and returns the text or err if the verification
// fails.
func VerifyCleartextMessageArmored(publicKey, armored string, verifyTime int64) (string, error) {
	signingKey, err := crypto.NewKeyFromArmored(publicKey)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in creating key object")
	}

	verifyKeyRing, err := crypto.NewKeyRing(signingKey)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in creating key ring")
	}

	return VerifyCleartextMessage(verifyKeyRing, armored, verifyTime)
}

// SignCleartextMessage signs text given a private keyring, canonicalizes and
// trims the newlines, and returns the PGP-compliant special armoring.
func SignCleartextMessage(keyRing *crypto.KeyRing, text string) (string, error) {
	message := crypto.NewPlainMessageFromString(internal.TrimEachLine(text))

	signature, err := keyRing.SignDetached(message)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in signing cleartext message")
	}

	return crypto.NewClearTextMessage(message.GetBinary(), signature.GetBinary()).GetArmored()
}

// VerifyCleartextMessage verifies PGP-compliant armored signed plain text
// given the public keyring and returns the text or err if the verification
// fails.
func VerifyCleartextMessage(keyRing *crypto.KeyRing, armored string, verifyTime int64) (string, error) {
	clearTextMessage, err := crypto.NewClearTextMessageFromArmored(armored)
	if err != nil {
		return "", errors.Wrap(err, "gopengpp: unable to unarmor cleartext message")
	}

	message := crypto.NewPlainMessageFromString(internal.TrimEachLine(clearTextMessage.GetString()))
	signature := crypto.NewPGPSignature(clearTextMessage.GetBinarySignature())
	err = keyRing.VerifyDetached(message, signature, verifyTime)
	if err != nil {
		return "", errors.Wrap(err, "gopengpp: unable to verify cleartext message")
	}

	return message.GetString(), nil
}
