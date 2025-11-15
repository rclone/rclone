package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsmeta"
	"github.com/stretchr/testify/require"
)

func TestMetadataSidecar(t *testing.T) {
	ctx := context.Background()
	opt := vfscommon.Opt
	opt.PersistMetadata = "all"
	opt.MetadataStore = "sidecar"
	r, v := newTestVFSOpt(t, &opt)
	defer r.Finalise()
	r.WriteObject(ctx, "file", "", time.Now())
	m := uint32(0o100600)
	require.NoError(t, v.SaveMetadata(ctx, "file", false, vfsmeta.Meta{Mode: &m}))
	got, err := v.LoadMetadata(ctx, "file", false)
	require.NoError(t, err)
	require.NotNil(t, got.Mode)
	require.Equal(t, m, *got.Mode)

	uid := uint32(1010)
	require.NoError(t, v.SaveMetadata(ctx, "file", false, vfsmeta.Meta{UID: &uid}))
	got, err = v.LoadMetadata(ctx, "file", false)
	require.NoError(t, err)
	require.NotNil(t, got.Mode)
	require.Equal(t, m, *got.Mode)
	require.NotNil(t, got.UID)
	require.Equal(t, uid, *got.UID)
}

func TestMetadataSidecarMerge(t *testing.T) {
	ctx := context.Background()
	opt := vfscommon.Opt
	opt.PersistMetadata = "all"
	opt.MetadataStore = "sidecar"
	r, v := newTestVFSOpt(t, &opt)
	defer r.Finalise()

	store := newSidecarStore(v, v.Opt.MetadataExtension)
	mode := uint32(0o640)
	require.NoError(t, store.Save(ctx, "standalone", false, vfsmeta.Meta{Mode: &mode}))
	gid := uint32(2000)
	require.NoError(t, store.Save(ctx, "standalone", false, vfsmeta.Meta{GID: &gid}))

	got, err := store.Load(ctx, "standalone", false)
	require.NoError(t, err)
	require.NotNil(t, got.Mode)
	require.Equal(t, mode, *got.Mode)
	require.NotNil(t, got.GID)
	require.Equal(t, gid, *got.GID)
}

func TestMetadataSidecarSkipsMetaPath(t *testing.T) {
	ctx := context.Background()
	opt := vfscommon.Opt
	opt.PersistMetadata = "all"
	opt.MetadataStore = "sidecar"
	opt.MetadataExtension = ".metadata"
	r, v := newTestVFSOpt(t, &opt)
	defer r.Finalise()

	r.WriteObject(ctx, "file", "", time.Now())

	mode := uint32(0o600)
	require.NoError(t, v.SaveMetadata(ctx, "file.metadata", false, vfsmeta.Meta{Mode: &mode}))

	_, err := v.ReadFile("file.metadata.metadata")
	require.Error(t, err)
	require.ErrorIs(t, err, ENOENT)

	_, err = v.LoadMetadata(ctx, "file.metadata", false)
	require.Error(t, err)
	require.ErrorIs(t, err, fs.ErrorObjectNotFound)
}

func TestMetadataSidecarFieldSelection(t *testing.T) {
	ctx := context.Background()
	opt := vfscommon.Opt
	opt.PersistMetadata = "owner"
	opt.MetadataStore = "sidecar"
	r, v := newTestVFSOpt(t, &opt)
	defer r.Finalise()

	r.WriteObject(ctx, "file", "data", time.Now())
	mode := uint32(0o644)
	uid := uint32(1234)
	meta := vfsmeta.Meta{Mode: &mode, UID: &uid}
	require.NoError(t, v.SaveMetadata(ctx, "file", false, meta))

	got, err := v.LoadMetadata(ctx, "file", false)
	require.NoError(t, err)
	require.Nil(t, got.Mode)
	require.NotNil(t, got.UID)
	require.Equal(t, uid, *got.UID)
}

func TestMetadataSidecarHiddenFromListing(t *testing.T) {
	ctx := context.Background()
	opt := vfscommon.Opt
	opt.PersistMetadata = "all"
	opt.MetadataStore = "sidecar"
	opt.HideMetadata = true
	r, v := newTestVFSOpt(t, &opt)
	defer r.Finalise()

	r.WriteObject(ctx, "file", "payload", time.Now())
	mode := uint32(0o600)
	require.NoError(t, v.SaveMetadata(ctx, "file", false, vfsmeta.Meta{Mode: &mode}))

	entries, err := v.ReadDir("")
	require.NoError(t, err)
	ext := v.Opt.MetadataExtension
	for _, entry := range entries {
		require.NotEqual(t, "file"+ext, entry.Name())
	}
	content, err := v.ReadFile("file" + ext)
	require.NoError(t, err)
	require.Contains(t, string(content), "mode")
}

func TestMetadataSidecarVisibleWhenNotHidden(t *testing.T) {
	ctx := context.Background()
	opt := vfscommon.Opt
	opt.PersistMetadata = "all"
	opt.MetadataStore = "sidecar"
	opt.HideMetadata = false
	r, v := newTestVFSOpt(t, &opt)
	defer r.Finalise()

	r.WriteObject(ctx, "file", "payload", time.Now())
	mode := uint32(0o600)
	require.NoError(t, v.SaveMetadata(ctx, "file", false, vfsmeta.Meta{Mode: &mode}))

	entries, err := v.ReadDir("")
	require.NoError(t, err)
	ext := v.Opt.MetadataExtension
	found := false
	for _, entry := range entries {
		if entry.Name() == "file"+ext {
			found = true
			break
		}
	}
	require.True(t, found, "expected metadata sidecar to be listed when hide flag disabled")
}
