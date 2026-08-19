package quark

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetUploadChunkSize changes the multipart size for the standard backend tests.
func (f *Fs) SetUploadChunkSize(size fs.SizeSuffix) (fs.SizeSuffix, error) {
	if size < 0 {
		return f.chunkSize, errors.New("chunk size must not be negative")
	}
	old := f.chunkSize
	f.chunkSize = size
	return old, nil
}

var _ fstests.SetUploadChunkSizer = (*Fs)(nil)

// TestIntegration runs the standard rclone backend tests.
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestQuark:",
		NilObject:  (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize:       100 * fs.Kibi,
			MaxChunkSize:       256 * fs.Kibi,
			NeedMultipleChunks: true,
		},
	})
}

// TestLiveMultipart verifies a real multipart round trip against the configured account.
func TestLiveMultipart(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()
	remoteName := "TestQuark:rclone-live-multipart-" + random.String(8)
	f, err := fs.NewFs(ctx, remoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skip(`"TestQuark:" not configured`)
	}
	require.NoError(t, err)
	require.NoError(t, f.Mkdir(ctx, ""))
	t.Cleanup(func() {
		assert.NoError(t, operations.Purge(ctx, f, ""))
	})

	const size int64 = int64(20*fs.Mebi) + 123
	modTime := time.Unix(1710000000, 0)
	src := object.NewStaticObjectInfo("multipart.bin", modTime, size, true, nil, f)
	uploadHash := hash.NewMultiHasher()
	obj, err := f.Put(ctx, io.TeeReader(readers.NewPatternReader(size), uploadHash), src)
	require.NoError(t, err)
	require.EqualValues(t, size, obj.Size())

	downloadHash := hash.NewMultiHasher()
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	written, err := io.Copy(downloadHash, rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.EqualValues(t, size, written)
	assert.Equal(t, uploadHash.Sums(), downloadHash.Sums())
}

// TestLivePublicLinkUnlink verifies creation and removal of a real public link.
func TestLivePublicLinkUnlink(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()
	remoteName := "TestQuark:rclone-live-share-" + random.String(8)
	f, err := fs.NewFs(ctx, remoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skip(`"TestQuark:" not configured`)
	}
	require.NoError(t, err)
	require.NoError(t, f.Mkdir(ctx, ""))
	t.Cleanup(func() {
		assert.NoError(t, operations.Purge(ctx, f, ""))
	})

	content := []byte("quark public link test")
	src := object.NewStaticObjectInfo("share.txt", time.Now(), int64(len(content)), true, nil, f)
	_, err = f.Put(ctx, bytes.NewReader(content), src)
	require.NoError(t, err)
	_ = fstest.NewObject(ctx, t, f, "share.txt")
	publicLink := f.Features().PublicLink
	require.NotNil(t, publicLink)
	link, err := publicLink(ctx, "share.txt", fs.Duration(24*time.Hour), false)
	require.NoError(t, err)
	assert.NotEmpty(t, link)
	link, err = publicLink(ctx, "share.txt", fs.DurationOff, true)
	require.NoError(t, err)
	assert.Empty(t, link)
}
