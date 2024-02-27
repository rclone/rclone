package fs_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFs(t *testing.T) {
	ctx := context.Background()

	// Register mockfs temporarily
	oldRegistry := fs.Registry
	mockfs.Register()
	defer func() {
		fs.Registry = oldRegistry
	}()

	f1, err := fs.NewFs(ctx, ":mockfs:/tmp")
	require.NoError(t, err)
	assert.Equal(t, ":mockfs", f1.Name())
	assert.Equal(t, "/tmp", f1.Root())

	assert.Equal(t, ":mockfs:/tmp", fs.ConfigString(f1))

	f2, err := fs.NewFs(ctx, ":mockfs,potato:/tmp")
	require.NoError(t, err)
	assert.Equal(t, ":mockfs{S_NHG}", f2.Name())
	assert.Equal(t, "/tmp", f2.Root())

	assert.Equal(t, ":mockfs{S_NHG}:/tmp", fs.ConfigString(f2))
	assert.Equal(t, ":mockfs,potato='true':/tmp", fs.ConfigStringFull(f2))

	f3, err := fs.NewFs(ctx, ":mockfs,potato='true':/tmp")
	require.NoError(t, err)
	assert.Equal(t, ":mockfs{S_NHG}", f3.Name())
	assert.Equal(t, "/tmp", f3.Root())

	assert.Equal(t, ":mockfs{S_NHG}:/tmp", fs.ConfigString(f3))
	assert.Equal(t, ":mockfs,potato='true':/tmp", fs.ConfigStringFull(f3))
}
