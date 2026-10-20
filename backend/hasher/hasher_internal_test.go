package hasher

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/kv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func putFile(ctx context.Context, t *testing.T, f fs.Fs, name, data string) fs.Object {
	mtime1 := fstest.Time("2001-02-03T04:05:06.499999999Z")
	item := fstest.Item{Path: name, ModTime: mtime1}
	o := fstests.PutTestContents(ctx, t, f, &item, data, true)
	require.NotNil(t, o)
	return o
}

func (f *Fs) testUploadFromCrypt(t *testing.T) {
	// make a temporary local remote
	tempRoot, err := fstest.LocalRemote()
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	// make a temporary crypt remote
	ctx := context.Background()
	pass := obscure.MustObscure("crypt")
	remote := fmt.Sprintf(`:crypt,remote="%s",password="%s":`, tempRoot, pass)
	cryptFs, err := fs.NewFs(ctx, remote)
	require.NoError(t, err)

	// make a test file on the crypt remote
	const dirName = "from_crypt_1"
	const fileName = dirName + "/file_from_crypt_1"
	const longTime = fs.ModTimeNotSupported
	src := putFile(ctx, t, cryptFs, fileName, "doggy froggy")

	// ensure that hash does not exist yet
	_ = f.pruneHash(fileName)
	hashType := f.keepHashes.GetOne()
	hash, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	assert.Error(t, err)
	assert.Empty(t, hash)

	// upload file to hasher
	in, err := src.Open(ctx)
	require.NoError(t, err)
	dst, err := f.Put(ctx, in, src)
	require.NoError(t, err)
	assert.NotNil(t, dst)

	// check that hash was created
	if f.opt.MaxAge > 0 {
		hash, err = f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)
	}
	//t.Logf("hash is %q", hash)
	_ = operations.Purge(ctx, f, dirName)
}

func (f *Fs) testUpdateStoresHash(t *testing.T) {
	// make a temporary local remote
	tempRoot, err := fstest.LocalRemote()
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	// make a temporary crypt remote as source
	ctx := context.Background()
	pass := obscure.MustObscure("crypt")
	remote := fmt.Sprintf(`:crypt,remote="%s",password="%s":`, tempRoot, pass)
	cryptFs, err := fs.NewFs(ctx, remote)
	require.NoError(t, err)

	const dirName = "update_hash_1"
	const fileName = dirName + "/file_update_1"
	const longTime = fs.ModTimeNotSupported
	hashType := f.keepHashes.GetOne()

	// upload initial file to hasher via Put
	src1 := putFile(ctx, t, cryptFs, fileName, "initial content")
	in1, err := src1.Open(ctx)
	require.NoError(t, err)
	dst, err := f.Put(ctx, in1, src1)
	require.NoError(t, err)
	require.NotNil(t, dst)

	// verify hash was stored after Put
	var hash1 string
	if f.opt.MaxAge > 0 {
		hash1, err = f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash1)
	}

	// update the file with new content via Update (this is what sync does for replacements)
	src2 := putFile(ctx, t, cryptFs, fileName, "updated content")
	in2, err := src2.Open(ctx)
	require.NoError(t, err)
	err = dst.Update(ctx, in2, src2)
	require.NoError(t, err)

	// verify hash was stored after Update and is different from the original
	if f.opt.MaxAge > 0 {
		hash2, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash2, "hash should be stored after Update")
		assert.NotEqual(t, hash1, hash2, "hash should change after Update with different content")
	}
	_ = operations.Purge(ctx, f, dirName)
}

func (f *Fs) testSetModTimeKeepsHash(t *testing.T) {
	ctx := context.Background()

	const dirName = "setmodtime_hash_1"
	const fileName = dirName + "/file_setmodtime_1"
	const longTime = fs.ModTimeNotSupported
	hashType := f.keepHashes.GetOne()

	// upload a file to hasher via Put
	src := putFile(ctx, t, f, fileName, "setmodtime content")
	dst, err := f.NewObject(ctx, fileName)
	require.NoError(t, err)
	require.NotNil(t, dst)
	_ = src

	// verify hash was stored after Put
	if f.opt.MaxAge <= 0 {
		t.Skip("hash cache is disabled")
	}
	hash1, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	require.NoError(t, err)
	require.NotEmpty(t, hash1)

	// change only the modification time, as operations does when it
	// syncs the modtime of an otherwise unchanged destination file
	newTime := fstest.Time("2002-03-04T05:06:07.499999999Z")
	require.NoError(t, dst.SetModTime(ctx, newTime))

	// the cached hash must survive a pure modtime change
	hash2, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	assert.NoError(t, err)
	assert.Equal(t, hash1, hash2, "hash should be kept after SetModTime")

	// and it must be found via the fingerprint-checked lookup too
	dst2, err := f.NewObject(ctx, fileName)
	require.NoError(t, err)
	hashVal, err := dst2.Hash(ctx, hashType)
	assert.NoError(t, err)
	assert.Equal(t, hash1, hashVal, "hash should be returned after SetModTime")

	_ = operations.Purge(ctx, f, dirName)
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	if !kv.Supported() {
		t.Skip("hasher is not supported on this OS")
	}
	t.Run("UploadFromCrypt", f.testUploadFromCrypt)
	t.Run("UpdateStoresHash", f.testUpdateStoresHash)
	t.Run("SetModTimeKeepsHash", f.testSetModTimeKeepsHash)
}

var _ fstests.InternalTester = (*Fs)(nil)
