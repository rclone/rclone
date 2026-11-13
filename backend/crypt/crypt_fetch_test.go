package crypt_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/backend/crypt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectFetchFeaturesMasked(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	const baseName = "fetch-crypt-wrapper-base"
	base, err := mockfs.NewFs(ctx, baseName, "", nil)
	require.NoError(t, err)
	base.Features().PublicLinkIsDirect = true
	base.Features().PublicLink = func(context.Context, string, fs.Duration, bool) (string, error) {
		return "https://example.invalid/object", nil
	}
	base.Features().ServerSideFetchURL = func(context.Context, string, string, fs.ObjectInfo, ...fs.OpenOption) (fs.Object, error) {
		return nil, fs.ErrorCantCopy
	}
	cache.Put(baseName+":", base)
	t.Cleanup(func() { cache.ClearConfig(baseName) })

	reg, err := fs.Find("crypt")
	require.NoError(t, err)
	m := fs.ConfigMap("crypt", reg.Options, "fetch-crypt", configmap.Simple{
		"remote":              baseName + ":",
		"password":            obscure.MustObscure("test-only"),
		"filename_encryption": "standard",
	})
	wrapped, err := crypt.NewFs(ctx, "fetch-crypt", "", m)
	require.NoError(t, err)
	assert.False(t, wrapped.Features().PublicLinkIsDirect)
	assert.Nil(t, wrapped.Features().ServerSideFetchURL)
	assert.NotNil(t, wrapped.Features().PublicLink)
}
