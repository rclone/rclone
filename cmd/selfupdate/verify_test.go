//go:build !noselfupdate

package selfupdate

import (
	"context"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify(t *testing.T) {
	ctx := context.Background()
	sumsBuf, err := os.ReadFile("testdata/verify/SHA256SUMS")
	require.NoError(t, err)
	hash, err := hex.DecodeString("b20b47f579a2c790ca752fb5d8e5651fade7d5867cbac0a4f71e805fc5c468d0")
	require.NoError(t, err)

	t.Run("NoError", func(t *testing.T) {
		err = verifyHashsumDownloaded(ctx, sumsBuf, "archive.zip", hash)
		require.NoError(t, err)
	})
	t.Run("BadSig", func(t *testing.T) {
		sumsBuf[0x60] ^= 1 // change the signature by one bit
		err = verifyHashsumDownloaded(ctx, sumsBuf, "archive.zip", hash)
		assert.ErrorContains(t, err, "invalid signature")
		sumsBuf[0x60] ^= 1 // undo the change
	})
	t.Run("BadSum", func(t *testing.T) {
		hash[0] ^= 1 // change the SHA256 by one bit
		err = verifyHashsumDownloaded(ctx, sumsBuf, "archive.zip", hash)
		assert.ErrorContains(t, err, "archive hash mismatch")
		hash[0] ^= 1 // undo the change
	})
	t.Run("BadName", func(t *testing.T) {
		err = verifyHashsumDownloaded(ctx, sumsBuf, "archive.zipX", hash)
		assert.ErrorContains(t, err, "unable to find hash")
	})
}
