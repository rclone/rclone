package fs_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fetchFeatureFs struct {
	fs.Fs
	ft *fs.Features
}

func (f *fetchFeatureFs) Features() *fs.Features { return f.ft }

func (f *fetchFeatureFs) ServerSideFetchURL(ctx context.Context, remote, sourceURL string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorCantCopy
}

type publicLinkFetchFeatureFs struct {
	fetchFeatureFs
}

func (f *publicLinkFetchFeatureFs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	return "https://example.com/fixed", nil
}

func TestFetchFeatures(t *testing.T) {
	ctx := context.Background()
	f := &fetchFeatureFs{ft: &fs.Features{}}
	got := (&fs.Features{PublicLinkIsDirect: true}).Fill(ctx, f)
	require.NotNil(t, got.ServerSideFetchURL)
	require.True(t, got.PublicLinkIsDirect)
	got.Mask(ctx, f)
	assert.Nil(t, got.ServerSideFetchURL)
	assert.False(t, got.PublicLinkIsDirect)

	got = (&fs.Features{PublicLinkIsDirect: true}).Fill(ctx, f)
	got.Disable("ServerSideFetchURL").Disable("PublicLinkIsDirect")
	assert.Nil(t, got.ServerSideFetchURL)
	assert.False(t, got.PublicLinkIsDirect)
	assert.Contains(t, got.List(), "ServerSideFetchURL")
	assert.Contains(t, got.List(), "PublicLinkIsDirect")
}

func TestFetchFeaturesMasking(t *testing.T) {
	ctx := context.Background()
	f := &fetchFeatureFs{ft: &fs.Features{PublicLinkIsDirect: true}}
	got := (&fs.Features{PublicLinkIsDirect: true}).Fill(ctx, f)
	got.Mask(ctx, f)
	assert.Nil(t, got.ServerSideFetchURL)
	assert.True(t, got.PublicLinkIsDirect)

	f.ft.ServerSideFetchURL = f.ServerSideFetchURL
	got = (&fs.Features{PublicLinkIsDirect: true}).Fill(ctx, f)
	got.Mask(ctx, f)
	require.NotNil(t, got.ServerSideFetchURL)
	assert.True(t, got.PublicLinkIsDirect)

	// A destination that lacks the capability must not gain it from the mask.
	got = &fs.Features{}
	got.Mask(ctx, f)
	assert.False(t, got.PublicLinkIsDirect)
}

func TestFetchFeaturesDisableFeatures(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.DisableFeatures = []string{"ServerSideFetchURL", "PublicLinkIsDirect"}
	f := &fetchFeatureFs{ft: &fs.Features{PublicLinkIsDirect: true}}
	got := (&fs.Features{PublicLinkIsDirect: true}).Fill(ctx, f)
	assert.Nil(t, got.ServerSideFetchURL)
	assert.False(t, got.PublicLinkIsDirect)
}

func TestFetchFeaturesPublicLinkDoesNotInferDirect(t *testing.T) {
	ctx := context.Background()
	f := &publicLinkFetchFeatureFs{fetchFeatureFs{ft: &fs.Features{}}}
	got := (&fs.Features{}).Fill(ctx, f)
	require.NotNil(t, got.PublicLink)
	assert.False(t, got.PublicLinkIsDirect)
}

func TestFetchFeaturesEnabled(t *testing.T) {
	ctx := context.Background()
	f := &fetchFeatureFs{ft: &fs.Features{PublicLinkIsDirect: true}}
	got := (&fs.Features{PublicLinkIsDirect: true}).Fill(ctx, f).Enabled()
	assert.True(t, got["PublicLinkIsDirect"])
	assert.True(t, got["ServerSideFetchURL"])
}
