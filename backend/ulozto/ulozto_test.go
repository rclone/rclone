package ulozto

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/ulozto/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestUlozto:",
		NilObject:  (*Object)(nil),
	})
}

// TestListWithoutMetadata verifies that basic operations can be performed even if the remote file wasn't written by
// rclone, or the serialized metadata can't be read.
func TestListWithoutMetadata(t *testing.T) {
	const (
		remoteName = "TestUlozto:"
		payload    = "42foobar42"
		sha256     = "d41f400003e93eb0891977f525e73ecedfa04272d2036f6137106168ecb196ab"
		md5        = "8ad32cfeb3dc0f5092261268f335e0a5"
		filesize   = len(payload)
	)
	ctx := context.Background()
	fstest.Initialise()
	subRemoteName, subRemoteLeaf, err := fstest.RandomRemoteName(remoteName)
	require.NoError(t, err)
	f, err := fs.NewFs(ctx, subRemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Logf("Didn't find %q in config file - skipping tests", remoteName)
		return
	}
	require.NoError(t, err)

	file := fstest.Item{ModTime: time.UnixMilli(123456789), Path: subRemoteLeaf, Size: int64(filesize), Hashes: map[hash.Type]string{
		hash.SHA256: sha256,
		hash.MD5:    md5,
	}}

	// Create a file with the given content and metadata
	obj := fstests.PutTestContents(ctx, t, f, &file, payload, false)

	// Verify the file has been uploaded
	fstest.CheckListing(t, f, []fstest.Item{file})

	// Now delete the description metadata
	uloztoObj := obj.(*Object)
	err = uloztoObj.updateFileProperties(ctx, api.UpdateDescriptionRequest{
		Description: "",
	})

	require.NoError(t, err)

	// Listing the file should still succeed, although with estimated mtime and no hashes
	fileWithoutDetails := fstest.Item{Path: subRemoteLeaf, Size: int64(filesize), ModTime: uloztoObj.remoteFsMtime, Hashes: map[hash.Type]string{
		hash.SHA256: "",
		hash.MD5:    "",
	}}
	fstest.CheckListing(t, f, []fstest.Item{fileWithoutDetails})

	mtime := time.UnixMilli(987654321)

	// When we update the mtime it should be reflected but hashes should stay intact
	require.NoError(t, obj.SetModTime(ctx, mtime))
	updatedMtimeFile := fstest.Item{Path: subRemoteLeaf, Size: int64(filesize), ModTime: mtime, Hashes: map[hash.Type]string{
		hash.SHA256: "",
		hash.MD5:    "",
	}}
	fstest.CheckListing(t, f, []fstest.Item{updatedMtimeFile})

	// Tear down
	require.NoError(t, operations.Purge(ctx, f, ""))
}
