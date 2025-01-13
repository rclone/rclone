package cryptomator_test

import (
	"testing"

	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

var cipherCombos = []string{cryptomator.CipherComboSivCtrMac, cryptomator.CipherComboSivGcm}

func drawCipherCombo(t *rapid.T) string {
	return rapid.SampledFrom(cipherCombos).Draw(t, "cipherCombo")
}

func drawMasterKey(t *rapid.T) cryptomator.MasterKey {
	encKey := fixedSizeByteArray(cryptomator.MasterEncryptKeySize).Draw(t, "encKey")
	macKey := fixedSizeByteArray(cryptomator.MasterMacKeySize).Draw(t, "macKey")
	return cryptomator.MasterKey{EncryptKey: encKey, MacKey: macKey}
}

func drawTestCryptor(t *rapid.T) *cryptomator.Cryptor {
	cryptor, err := cryptomator.NewCryptor(drawMasterKey(t), drawCipherCombo(t))
	assert.NoError(t, err, "creating cryptor")
	return &cryptor
}

func TestEncryptDecryptFilename(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.String().Draw(t, "name")
		dirID := rapid.String().Draw(t, "dirID")
		cryptor := drawTestCryptor(t)

		encName, err := cryptor.EncryptFilename(name, dirID)
		assert.NoError(t, err, "encryption error")

		decName, err := cryptor.DecryptFilename(encName, dirID)
		assert.NoError(t, err, "decryption error")

		assert.Equal(t, name, decName)
	})
}
