package cryptcheck

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func put(ctx context.Context, t *testing.T, f fs.Fs, remote, contents string) {
	t.Helper()
	_, err := operations.Rcat(ctx, f, remote, io.NopCloser(strings.NewReader(contents)), time.Now(), nil)
	require.NoError(t, err)
}

func TestCryptCheck(t *testing.T) {
	ctx := context.Background()
	fcrypt, err := fs.NewFs(ctx, ":crypt,remote='"+t.TempDir()+"',password="+obscure.MustObscure("potato")+":")
	require.NoError(t, err)
	fplain, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)
	fother, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)

	put(ctx, t, fcrypt, "file.txt", "hello world")
	put(ctx, t, fplain, "file.txt", "hello world")

	t.Run("CryptIsDst", func(t *testing.T) {
		assert.NoError(t, cryptCheck(ctx, fcrypt, fplain))
	})
	t.Run("CryptIsSrc", func(t *testing.T) {
		assert.NoError(t, cryptCheck(ctx, fplain, fcrypt))
	})
	t.Run("NoCrypt", func(t *testing.T) {
		err := cryptCheck(ctx, fother, fplain)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a crypt remote")
	})

	// Same size, different contents, so only the hash check catches it
	put(ctx, t, fplain, "file.txt", "HELLO WORLD")

	t.Run("CryptIsDstDiffer", func(t *testing.T) {
		assert.Error(t, cryptCheck(ctx, fcrypt, fplain))
	})
	t.Run("CryptIsSrcDiffer", func(t *testing.T) {
		assert.Error(t, cryptCheck(ctx, fplain, fcrypt))
	})
}
