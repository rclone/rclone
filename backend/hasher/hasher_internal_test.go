package hasher

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
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

// readAll opens an object via hasher and reads all its content.
func readAll(ctx context.Context, t *testing.T, o fs.Object) []byte {
	t.Helper()
	rc, err := o.Open(ctx)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return data
}

// readAllErr opens an object via hasher and reads all its content,
// returning both the data and any error from the read.
func readAllErr(ctx context.Context, t *testing.T, o fs.Object) ([]byte, error) {
	t.Helper()
	rc, err := o.Open(ctx)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func (f *Fs) setDBMode(mode string) func() {
	origMode := f.opt.DBMode
	f.opt.DBMode = mode
	return func() {
		f.opt.DBMode = origMode
	}
}

func (f *Fs) testVerifyDownload(t *testing.T) {
	ctx := context.Background()
	const fileName = "verify_match/file1"

	_ = putFile(ctx, t, f, fileName, "hello verify")

	defer f.setDBMode("verify")()

	obj, err := f.NewObject(ctx, fileName)
	require.NoError(t, err)
	data := readAll(ctx, t, obj)
	assert.Equal(t, "hello verify", string(data))

	_ = operations.Purge(ctx, f, "verify_match")
}

func (f *Fs) testVerifyMismatch(t *testing.T) {
	ctx := context.Background()
	const fileName = "verify_mismatch/file1"

	if f.opt.MaxAge <= 0 {
		t.Skip("requires caching (max_age > 0)")
	}

	_ = putFile(ctx, t, f, fileName, "good data")

	hashType := f.keepHashes.GetOne()
	key := f.Fs.Root() + "/" + fileName
	err := f.putRawHashes(ctx, key, anyFingerprint, operations.HashSums{hashType.String(): "badhash000000"})
	require.NoError(t, err)

	defer f.setDBMode("verify")()

	obj, err := f.NewObject(ctx, fileName)
	require.NoError(t, err)
	_, readErr := readAllErr(ctx, t, obj)
	require.Error(t, readErr)
	assert.True(t, fserrors.IsRetryError(readErr), "expected retriable error, got: %v", readErr)
	assert.Contains(t, readErr.Error(), "corrupted on transfer")

	_ = operations.Purge(ctx, f, "verify_mismatch")
}

func (f *Fs) testVerifyFirstTime(t *testing.T) {
	ctx := context.Background()
	const fileName = "verify_firsttime/file1"
	const longTime = fs.ModTimeNotSupported

	if f.opt.MaxAge <= 0 {
		t.Skip("requires caching (max_age > 0)")
	}

	_ = putFile(ctx, t, f, fileName, "first seen data")

	_ = f.pruneHash(fileName)
	hashType := f.keepHashes.GetOne()
	hash, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	assert.Error(t, err)
	assert.Empty(t, hash)

	defer f.setDBMode("verify")()

	obj, err := f.NewObject(ctx, fileName)
	require.NoError(t, err)
	data := readAll(ctx, t, obj)
	assert.Equal(t, "first seen data", string(data))

	hash, err = f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	_ = operations.Purge(ctx, f, "verify_firsttime")
}

func (f *Fs) testReadOnlyWrite(t *testing.T) {
	ctx := context.Background()
	const fileName = "readonly_blocks/file1"
	const longTime = fs.ModTimeNotSupported

	if f.opt.MaxAge <= 0 {
		t.Skip("requires caching (max_age > 0)")
	}

	defer f.setDBMode("readonly")()

	_ = putFile(ctx, t, f, fileName, "readonly test")

	hashType := f.keepHashes.GetOne()
	hash, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	assert.Error(t, err)
	assert.Empty(t, hash)

	_ = operations.Purge(ctx, f, "readonly_blocks")
}

func (f *Fs) testReadOnlyVerify(t *testing.T) {
	ctx := context.Background()
	const fileName = "readonly_verify/file1"
	const longTime = fs.ModTimeNotSupported

	if f.opt.MaxAge <= 0 {
		t.Skip("requires caching (max_age > 0)")
	}

	_ = putFile(ctx, t, f, fileName, "verify readonly data")

	hashType := f.keepHashes.GetOne()
	hash, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	defer f.setDBMode("readonly")()

	obj, err := f.NewObject(ctx, fileName)
	require.NoError(t, err)
	data := readAll(ctx, t, obj)
	assert.Equal(t, "verify readonly data", string(data))

	_ = operations.Purge(ctx, f, "readonly_verify")
}

func (f *Fs) testReadOnlySibling(t *testing.T) {
	ctx := context.Background()
	const fileName = "readonly_sibling/file1"
	const longTime = fs.ModTimeNotSupported

	if f.opt.MaxAge <= 0 {
		t.Skip("requires caching (max_age > 0)")
	}

	siblingIface, err := NewFs(ctx, f.name, f.root, configmap.Simple{
		"remote":    f.opt.Remote,
		"hashes":    f.opt.Hashes.String(),
		"auto_size": fmt.Sprint(f.opt.AutoSize),
		"max_age":   fmt.Sprint(f.opt.MaxAge),
		"db_mode":   "off",
	})
	require.NoError(t, err)

	sibling, ok := siblingIface.(*Fs)
	require.True(t, ok)
	defer func() {
		require.NoError(t, sibling.Shutdown(ctx))
	}()

	// Exercise the shared-DB case directly.
	require.Same(t, f.db, sibling.db)

	defer f.setDBMode("readonly")()

	_ = putFile(ctx, t, sibling, fileName, "sibling data")

	hashType := f.keepHashes.GetOne()
	hash, err := f.getRawHash(ctx, hashType, fileName, anyFingerprint, longTime)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash, "sibling write must succeed even though f is readonly")

	_ = operations.Purge(ctx, f, "readonly_sibling")
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	if !kv.Supported() {
		t.Skip("hasher is not supported on this OS")
	}
	t.Run("UploadFromCrypt", f.testUploadFromCrypt)
	t.Run("VerifyDownload", f.testVerifyDownload)
	t.Run("VerifyMismatch", f.testVerifyMismatch)
	t.Run("VerifyFirstTime", f.testVerifyFirstTime)
	t.Run("ReadOnlyWrite", f.testReadOnlyWrite)
	t.Run("ReadOnlyVerify", f.testReadOnlyVerify)
	t.Run("ReadOnlySibling", f.testReadOnlySibling)
}

var _ fstests.InternalTester = (*Fs)(nil)
