package alias_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/backend/alias"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectFetchFeaturesPreserved(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	const baseName = "fetch-alias-wrapper-base"
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

	wrapped, err := alias.NewFs(ctx, "fetch-alias", "", configmap.Simple{"remote": baseName + ":"})
	require.NoError(t, err)
	assert.Same(t, base, wrapped)
	assert.True(t, wrapped.Features().PublicLinkIsDirect)
	assert.NotNil(t, wrapped.Features().ServerSideFetchURL)
	assert.NotNil(t, wrapped.Features().PublicLink)
}
