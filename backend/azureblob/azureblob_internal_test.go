//go:build !plan9 && !solaris && !js

package azureblob

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockIDCreator(t *testing.T) {
	// Check creation and random number
	bic, err := newBlockIDCreator()
	require.NoError(t, err)
	bic2, err := newBlockIDCreator()
	require.NoError(t, err)
	assert.NotEqual(t, bic.random, bic2.random)
	assert.NotEqual(t, bic.random, [8]byte{})

	// Set random to known value for tests
	bic.random = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	chunkNumber := uint64(0xFEDCBA9876543210)

	// Check creation of ID
	want := base64.StdEncoding.EncodeToString([]byte{0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10, 1, 2, 3, 4, 5, 6, 7, 8})
	assert.Equal(t, "/ty6mHZUMhABAgMEBQYHCA==", want)
	got := bic.newBlockID(chunkNumber)
	assert.Equal(t, want, got)
	assert.Equal(t, "/ty6mHZUMhABAgMEBQYHCA==", got)

	// Test checkID is working
	assert.NoError(t, bic.checkID(chunkNumber, got))
	assert.ErrorContains(t, bic.checkID(chunkNumber, "$"+got), "illegal base64")
	assert.ErrorContains(t, bic.checkID(chunkNumber, "AAAA"+got), "bad block ID length")
	assert.ErrorContains(t, bic.checkID(chunkNumber+1, got), "expecting decoded")
	assert.ErrorContains(t, bic2.checkID(chunkNumber, got), "random bytes")
}

func (f *Fs) testFeatures(t *testing.T) {
	// Check first feature flags are set on this remote
	enabled := f.Features().SetTier
	assert.True(t, enabled)
	enabled = f.Features().GetTier
	assert.True(t, enabled)
}

type ReadSeekCloser struct {
	*strings.Reader
}

func (r *ReadSeekCloser) Close() error {
	return nil
}

// Stage a block at remote but don't commit it
func (f *Fs) stageBlockWithoutCommit(ctx context.Context, t *testing.T, remote string) {
	var (
		containerName, blobPath = f.split(remote)
		containerClient         = f.cntSVC(containerName)
		blobClient              = containerClient.NewBlockBlobClient(blobPath)
		data                    = "uncommitted data"
		blockID                 = "1"
		blockIDBase64           = base64.StdEncoding.EncodeToString([]byte(blockID))
	)
	r := &ReadSeekCloser{strings.NewReader(data)}
	_, err := blobClient.StageBlock(ctx, blockIDBase64, r, nil)
	require.NoError(t, err)

	// Verify the block is staged but not committed
	blockList, err := blobClient.GetBlockList(ctx, blockblob.BlockListTypeAll, nil)
	require.NoError(t, err)
	found := false
	for _, block := range blockList.UncommittedBlocks {
		if *block.Name == blockIDBase64 {
			found = true
			break
		}
	}
	require.True(t, found, "Block ID not found in uncommitted blocks")
}

// This tests uploading a blob where it has uncommitted blocks with a different ID size.
//
// https://gauravmantri.com/2013/05/18/windows-azure-blob-storage-dealing-with-the-specified-blob-or-block-content-is-invalid-error/
//
// TestIntegration/FsMkdir/FsPutFiles/Internal/WriteUncommittedBlocks
func (f *Fs) testWriteUncommittedBlocks(t *testing.T) {
	var (
		ctx    = context.Background()
		remote = "testBlob"
	)

	// Multipart copy the blob please
	oldUseCopyBlob, oldCopyCutoff := f.opt.UseCopyBlob, f.opt.CopyCutoff
	f.opt.UseCopyBlob = false
	f.opt.CopyCutoff = f.opt.ChunkSize
	defer func() {
		f.opt.UseCopyBlob, f.opt.CopyCutoff = oldUseCopyBlob, oldCopyCutoff
	}()

	// Create a blob with uncommitted blocks
	f.stageBlockWithoutCommit(ctx, t, remote)

	// Now attempt to overwrite the block with a different sized block ID to provoke this error

	// Check the object does not exist
	_, err := f.NewObject(ctx, remote)
	require.Equal(t, fs.ErrorObjectNotFound, err)

	// Upload a multipart file over the block with uncommitted chunks of a different ID size
	size := 4*int(f.opt.ChunkSize) - 1
	contents := random.String(size)
	item := fstest.NewItem(remote, contents, fstest.Time("2001-05-06T04:05:06.499Z"))
	o := fstests.PutTestContents(ctx, t, f, &item, contents, true)

	// Check size
	assert.Equal(t, int64(size), o.Size())

	// Create a new blob with uncommitted blocks
	newRemote := "testBlob2"
	f.stageBlockWithoutCommit(ctx, t, newRemote)

	// Copy over that block
	dst, err := f.Copy(ctx, o, newRemote)
	require.NoError(t, err)

	// Check basics
	assert.Equal(t, int64(size), dst.Size())
	assert.Equal(t, newRemote, dst.Remote())

	// Check contents
	gotContents := fstests.ReadObject(ctx, t, dst, -1)
	assert.Equal(t, contents, gotContents)

	// Remove the object
	require.NoError(t, dst.Remove(ctx))
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Features", f.testFeatures)
	t.Run("WriteUncommittedBlocks", f.testWriteUncommittedBlocks)
}
