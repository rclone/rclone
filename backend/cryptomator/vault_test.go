package cryptomator_test

import (
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func fixedSizeByteArray(constant int) *rapid.Generator[[]byte] {
	return rapid.SliceOfN(rapid.Byte(), constant, constant)
}

func TestVaultConfigRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		masterKey := drawMasterKey(t)

		c1 := cryptomator.NewVaultConfig()

		token, err := c1.Marshal(masterKey)
		assert.NoError(t, err)

		c2, err := cryptomator.UnmarshalVaultConfig(token, func(*jwt.Token) (*cryptomator.MasterKey, error) {
			return &masterKey, nil
		})
		assert.NoError(t, err)

		assert.Equal(t, c1, c2)
	})
}
