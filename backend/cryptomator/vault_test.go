package cryptomator_test

import (
	"bytes"
	"testing"

	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func fixedSizeByteArray(constant int) *rapid.Generator[[]byte] {
	return rapid.SliceOfN(rapid.Byte(), constant, constant)
}

func TestVaultConfigRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		buf := &bytes.Buffer{}

		encKey := fixedSizeByteArray(cryptomator.MasterEncryptKeySize).Draw(t, "encKey")
		macKey := fixedSizeByteArray(cryptomator.MasterMacKeySize).Draw(t, "macKey")

		c1, err := cryptomator.NewVaultConfig(encKey, macKey)
		assert.NoError(t, err)

		err = c1.Marshal(buf, encKey, macKey)
		assert.NoError(t, err)

		c2, err := cryptomator.UnmarshalUnverifiedVaultConfig(buf)
		assert.NoError(t, err)

		assert.Empty(t, buf.Bytes())
		assert.Equal(t, c1, c2)

		err = c2.Verify(encKey, macKey)
		assert.NoError(t, err)
	})
}
