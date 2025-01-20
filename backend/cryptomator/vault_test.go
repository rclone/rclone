package cryptomator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func fixedSizeByteArray(constant int) *rapid.Generator[[]byte] {
	return rapid.SliceOfN(rapid.Byte(), constant, constant)
}

func TestVaultConfigRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := drawMasterKey(t)

		c1 := newVaultConfig()

		token, err := c1.Marshal(key)
		assert.NoError(t, err)

		c2, err := unmarshalVaultConfig(token, func(string) (*masterKey, error) {
			return &key, nil
		})
		assert.NoError(t, err)

		assert.Equal(t, c1, c2)
	})
}
