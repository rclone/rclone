package cryptomator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

var cipherCombos = []string{cipherComboSivCtrMac, cipherComboSivGcm}

func drawCipherCombo(t *rapid.T) string {
	return rapid.SampledFrom(cipherCombos).Draw(t, "cipherCombo")
}

func drawMasterKey(t *rapid.T) masterKey {
	encKey := fixedSizeByteArray(masterEncryptKeySize).Draw(t, "encKey")
	macKey := fixedSizeByteArray(masterMacKeySize).Draw(t, "macKey")
	return masterKey{EncryptKey: encKey, MacKey: macKey}
}

func drawTestCryptor(t *rapid.T) *cryptor {
	cryptor, err := newCryptor(drawMasterKey(t), drawCipherCombo(t))
	assert.NoError(t, err, "creating cryptor")
	return &cryptor
}

func TestEncryptDecryptFilename(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.String().Draw(t, "name")
		dirID := rapid.String().Draw(t, "dirID")
		cryptor := drawTestCryptor(t)

		encName := cryptor.encryptFilename(name, dirID)
		decName, err := cryptor.decryptFilename(encName, dirID)
		assert.NoError(t, err, "decryption error")

		assert.Equal(t, name, decName)
	})
}
