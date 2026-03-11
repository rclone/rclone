package resume

import (
	"context"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	cp := &Checkpoint{
		RemoteName: "s3:mybucket/path",
		Dir:        "subdir/",
		LastKey:    "subdir/file-1000",
	}
	require.NoError(t, store.Save(cp))

	loaded, err := store.Load("s3:mybucket/path", "subdir/")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, checkpointVersion, loaded.Version)
	assert.Equal(t, "s3:mybucket/path", loaded.RemoteName)
	assert.Equal(t, "subdir/", loaded.Dir)
	assert.Equal(t, "subdir/file-1000", loaded.LastKey)
	assert.False(t, loaded.UpdatedAt.IsZero())
}

func TestLoadNotFound(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	cp, err := store.Load("s3:mybucket", "")
	require.NoError(t, err)
	assert.Nil(t, cp)
}

func TestLoadRemoteMismatch(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	cp := &Checkpoint{
		RemoteName: "s3:bucket-a",
		Dir:        "dir/",
		LastKey:    "dir/obj-500",
	}
	require.NoError(t, store.Save(cp))

	// Different remote name - hash differs, so file not found
	loaded, err := store.Load("s3:bucket-b", "dir/")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestLoadDirMismatch(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	cp := &Checkpoint{
		RemoteName: "s3:bucket",
		Dir:        "dir-a/",
		LastKey:    "dir-a/obj-500",
	}
	require.NoError(t, store.Save(cp))

	// Different dir - hash differs, so file not found
	loaded, err := store.Load("s3:bucket", "dir-b/")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestDelete(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	cp := &Checkpoint{
		RemoteName: "s3:bucket",
		Dir:        "",
		LastKey:    "obj-100",
	}
	require.NoError(t, store.Save(cp))

	// Verify it exists
	loaded, err := store.Load("s3:bucket", "")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Delete
	require.NoError(t, store.Delete("s3:bucket", ""))

	// Verify it's gone
	loaded, err = store.Load("s3:bucket", "")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestDeleteNonExistent(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	// Should not error
	require.NoError(t, store.Delete("s3:bucket", "missing/"))
}

func TestWrapCallbackNoCheckpoint(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	var received fs.DirEntries
	callback := func(entries fs.DirEntries) error {
		received = append(received, entries...)
		return nil
	}

	startAfter, wrapped, done, err := store.WrapCallback("s3:bucket", "dir", callback)
	require.NoError(t, err)
	assert.Equal(t, "", startAfter, "no checkpoint should give empty startAfter")

	// Send a page of entries
	entries := fs.DirEntries{
		fs.NewDir("dir/aaa", time.Time{}),
		fs.NewDir("dir/zzz", time.Time{}),
	}
	require.NoError(t, wrapped(entries))
	assert.Len(t, received, 2, "entries should pass through")

	// Checkpoint should exist now
	cp, err := store.Load("s3:bucket", "dir")
	require.NoError(t, err)
	require.NotNil(t, cp)
	assert.Equal(t, "dir/zzz", cp.LastKey)

	// Call done with nil error - checkpoint should be deleted
	var noErr error
	done(&noErr)
	cp, err = store.Load("s3:bucket", "dir")
	require.NoError(t, err)
	assert.Nil(t, cp)
}

func TestWrapCallbackWithCheckpoint(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	// Save an existing checkpoint
	require.NoError(t, store.Save(&Checkpoint{
		RemoteName: "s3:bucket",
		Dir:        "",
		LastKey:    "file-500",
	}))

	callback := func(entries fs.DirEntries) error { return nil }

	startAfter, _, _, err := store.WrapCallback("s3:bucket", "", callback)
	require.NoError(t, err)
	assert.Equal(t, "file-500", startAfter, "should resume from checkpoint")
}

func TestWrapCallbackTracksMaxKey(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	callback := func(entries fs.DirEntries) error { return nil }
	_, wrapped, _, err := store.WrapCallback("s3:bucket", "", callback)
	require.NoError(t, err)

	// First page
	require.NoError(t, wrapped(fs.DirEntries{
		fs.NewDir("bbb", time.Time{}),
		fs.NewDir("aaa", time.Time{}),
	}))
	cp, err := store.Load("s3:bucket", "")
	require.NoError(t, err)
	require.NotNil(t, cp)
	assert.Equal(t, "bbb", cp.LastKey, "should track max key across entries")

	// Second page with higher keys
	require.NoError(t, wrapped(fs.DirEntries{
		fs.NewDir("ddd", time.Time{}),
		fs.NewDir("ccc", time.Time{}),
	}))
	cp, err = store.Load("s3:bucket", "")
	require.NoError(t, err)
	require.NotNil(t, cp)
	assert.Equal(t, "ddd", cp.LastKey, "should track max key across pages")
}

func TestSetupDisabled(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.ResumeListings = "" // not configured

	called := false
	callback := func(entries fs.DirEntries) error {
		called = true
		return nil
	}

	startAfter, wrapped, done, err := Setup(ctx, true, nil, "", callback)
	require.NoError(t, err)
	assert.Equal(t, "", startAfter)

	// Should be the original callback (passthrough)
	require.NoError(t, wrapped(nil))
	assert.True(t, called)

	// done should be safe to call
	var noErr error
	done(&noErr)
}

func TestSetupEnabledFalse(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.ResumeListings = t.TempDir()

	callback := func(entries fs.DirEntries) error { return nil }

	// enabled=false should return passthrough even with ResumeListings set
	startAfter, _, done, err := Setup(ctx, false, nil, "", callback)
	require.NoError(t, err)
	assert.Equal(t, "", startAfter)
	var noErr error
	done(&noErr)
}

func TestSetupEnabled(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.ResumeListings = t.TempDir()

	memFs, err := fs.NewFs(ctx, ":memory:")
	require.NoError(t, err)
	remoteName := fs.ConfigString(memFs)

	var received fs.DirEntries
	callback := func(entries fs.DirEntries) error {
		received = append(received, entries...)
		return nil
	}

	startAfter, wrapped, done, err := Setup(ctx, true, memFs, "dir", callback)
	require.NoError(t, err)
	assert.Equal(t, "", startAfter, "no prior checkpoint")

	// Send entries through wrapped callback
	require.NoError(t, wrapped(fs.DirEntries{
		fs.NewDir("dir/file1", time.Time{}),
	}))
	assert.Len(t, received, 1)

	// Checkpoint should exist
	store, err := NewStore(ci.ResumeListings)
	require.NoError(t, err)
	cp, err := store.Load(remoteName, "dir")
	require.NoError(t, err)
	require.NotNil(t, cp)
	assert.Equal(t, "dir/file1", cp.LastKey)

	// done() cleans up on success
	var noErr error
	done(&noErr)
	cp, err = store.Load(remoteName, "dir")
	require.NoError(t, err)
	assert.Nil(t, cp)
}

func TestSaveOverwrites(t *testing.T) {
	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	cp := &Checkpoint{
		RemoteName: "s3:bucket",
		Dir:        "",
		LastKey:    "obj-100",
	}
	require.NoError(t, store.Save(cp))

	cp.LastKey = "obj-200"
	require.NoError(t, store.Save(cp))

	loaded, err := store.Load("s3:bucket", "")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "obj-200", loaded.LastKey)
}
