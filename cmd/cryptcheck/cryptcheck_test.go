package cryptcheck

import (
	"context"
	"os"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCryptCheckValidatesBeforeOpeningReports(t *testing.T) {
	ctx := context.Background()
	fsrc, err := fs.TemporaryLocalFs(ctx)
	require.NoError(t, err)
	fdst, err := fs.TemporaryLocalFs(ctx)
	require.NoError(t, err)
	require.NoError(t, fsrc.Mkdir(ctx, ""))
	require.NoError(t, fdst.Mkdir(ctx, ""))
	t.Cleanup(func() {
		require.NoError(t, operations.Purge(ctx, fsrc, ""))
		require.NoError(t, operations.Purge(ctx, fdst, ""))
	})

	report := t.TempDir() + "/combined.txt"
	require.NoError(t, os.WriteFile(report, []byte("keep"), 0o600))
	combinedFlag := commandDefinition.Flags().Lookup("combined")
	require.NotNil(t, combinedFlag)
	oldCombined := combinedFlag.Value.String()
	require.NoError(t, combinedFlag.Value.Set(report))
	t.Cleanup(func() { require.NoError(t, combinedFlag.Value.Set(oldCombined)) })

	err = cryptCheck(ctx, fdst, fsrc)
	require.Error(t, err)
	contents, readErr := os.ReadFile(report)
	require.NoError(t, readErr)
	assert.Equal(t, "keep", string(contents))
}
