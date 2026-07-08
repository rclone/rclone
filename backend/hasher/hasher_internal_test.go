package hasher

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
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

type hashLessObject struct {
	fs.Object
	fs *hashLessFs
}

func (o *hashLessObject) Fs() fs.Info {
	return o.fs
}

func (o *hashLessObject) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

type hashLessFs struct {
	fs.Fs
}

func (f *hashLessFs) Hashes() hash.Set {
	return hash.Set(0)
}

func (f *hashLessFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	return &hashLessObject{Object: o, fs: f}, nil
}

func (f *hashLessFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.Fs.Put(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return &hashLessObject{Object: o, fs: f}, nil
}

func (f *Fs) testUpdateInFlightHashing(t *testing.T) {
	ctx := context.Background()
	hashType := hash.MD5

	// Test Case A: Underlying Fs does NOT support hashes (like Google Photos)
	t.Run("UnderlyingLacksHashes", func(t *testing.T) {
		tempRoot, err := fstest.LocalRemote()
		require.NoError(t, err)
		defer func() {
			_ = os.RemoveAll(tempRoot)
		}()

		localFs, err := fs.NewFs(ctx, tempRoot)
		require.NoError(t, err)

		baseFs := &hashLessFs{Fs: localFs}

		hasherFs := &Fs{
			Fs:   baseFs,
			name: "TestHasher",
			root: "",
			opt: &Options{
				Remote: tempRoot,
				Hashes: fs.CommaSepList{"md5"},
				MaxAge: fs.DurationOff,
			},
		}
		hasherFs.keepHashes.Add(hashType)
		hasherFs.suppHashes.Add(hashType)
		hasherFs.autoHashes.Add(hashType)

		gob.Register(hashRecord{})
		db, err := kv.Start(ctx, "hasher", hasherFs.Fs)
		require.NoError(t, err)
		hasherFs.db = db
		defer func() {
			_ = db.Stop(true)
		}()

		// Put initial file
		src := putFile(ctx, t, baseFs, "test_file.txt", "initial contents")
		in, err := src.Open(ctx)
		require.NoError(t, err)
		dst, err := hasherFs.Put(ctx, in, src)
		require.NoError(t, err)
		require.NotNil(t, dst)

		// Prune the hash from the DB to simulate it being empty before update
		err = hasherFs.pruneHash("test_file.txt")
		require.NoError(t, err)

		// Verify hash is indeed gone
		h, err := hasherFs.getRawHash(ctx, hashType, "test_file.txt", anyFingerprint, time.Duration(fs.DurationOff))
		assert.Error(t, err)
		assert.Empty(t, h)

		// Run Update (overwrite)
		src2 := putFile(ctx, t, baseFs, "test_file.txt", "updated contents")
		in2, err := src2.Open(ctx)
		require.NoError(t, err)

		err = dst.Update(ctx, in2, src2)
		require.NoError(t, err)

		// Verify that the hash was computed in-flight and cached back into BoltDB
		h, err = hasherFs.getRawHash(ctx, hashType, "test_file.txt", anyFingerprint, time.Duration(fs.DurationOff))
		assert.NoError(t, err)
		assert.NotEmpty(t, h)
	})

	// Test Case B: Underlying Fs DOES support hashes natively (like Local, S3, B2)
	t.Run("UnderlyingSupportsHashes", func(t *testing.T) {
		tempRoot, err := fstest.LocalRemote()
		require.NoError(t, err)
		defer func() {
			_ = os.RemoveAll(tempRoot)
		}()

		localFs, err := fs.NewFs(ctx, tempRoot)
		require.NoError(t, err)

		hasherFs := &Fs{
			Fs:   localFs,
			name: "TestHasher",
			root: "",
			opt: &Options{
				Remote: tempRoot,
				Hashes: fs.CommaSepList{"md5"},
				MaxAge: fs.DurationOff,
			},
		}
		hasherFs.keepHashes.Add(hashType)
		hasherFs.suppHashes.Add(hashType)
		hasherFs.autoHashes.Add(hashType)

		db, err := kv.Start(ctx, "hasher", hasherFs.Fs)
		require.NoError(t, err)
		hasherFs.db = db
		defer func() {
			_ = db.Stop(true)
		}()

		// Put initial file
		src := putFile(ctx, t, localFs, "test_file.txt", "initial contents")
		in, err := src.Open(ctx)
		require.NoError(t, err)
		dst, err := hasherFs.Put(ctx, in, src)
		require.NoError(t, err)
		require.NotNil(t, dst)

		// Prune the hash from the DB
		err = hasherFs.pruneHash("test_file.txt")
		require.NoError(t, err)

		// Run Update (overwrite)
		src2 := putFile(ctx, t, localFs, "test_file.txt", "updated contents")
		in2, err := src2.Open(ctx)
		require.NoError(t, err)

		err = dst.Update(ctx, in2, src2)
		require.NoError(t, err)

		// Verify that the hash was pruned and NOT cached (relies on post-transfer check)
		h, err := hasherFs.getRawHash(ctx, hashType, "test_file.txt", anyFingerprint, time.Duration(fs.DurationOff))
		assert.Error(t, err)
		assert.Empty(t, h)
	})
}

func (f *Fs) testFingerprintIgnoreSize(t *testing.T) {
	ctx := context.Background()
	hashType := hash.MD5

	tempRoot, err := fstest.LocalRemote()
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	localFs, err := fs.NewFs(ctx, tempRoot)
	require.NoError(t, err)

	hasherFs := &Fs{
		Fs:   localFs,
		name: "TestHasher",
		root: "",
		opt: &Options{
			Remote: tempRoot,
			Hashes: fs.CommaSepList{"md5"},
			MaxAge: fs.DurationOff,
		},
	}
	hasherFs.keepHashes.Add(hashType)
	hasherFs.suppHashes.Add(hashType)
	hasherFs.autoHashes.Add(hashType)

	db, err := kv.Start(ctx, "hasher", hasherFs.Fs)
	require.NoError(t, err)
	hasherFs.db = db
	defer func() {
		_ = db.Stop(true)
	}()

	// Put a hash under fingerprint: "100,-,-" (size 100, no modtime, no hash)
	fp1 := "100,-,-"
	remote := "test_ignore_size.txt"
	dbKey := path.Join(hasherFs.Fs.Root(), remote)
	err = hasherFs.putRawHashes(ctx, dbKey, fp1, operations.HashSums{"md5": "abc123hash"})
	require.NoError(t, err)

	// Case 1: Standard lookup with mismatching size (120,-,-) -> should fail
	_, err = hasherFs.getRawHash(ctx, hashType, remote, "120,-,-", time.Duration(fs.DurationOff))
	assert.Error(t, err, "standard lookup with different size should fail")

	// Case 2: Enable ignore_size in config map and lookup with mismatching size -> should succeed
	ci := fs.GetConfig(ctx)
	ci.IgnoreSize = true
	h, err := hasherFs.getRawHash(ctx, hashType, remote, "120,-,-", time.Duration(fs.DurationOff))
	assert.NoError(t, err)
	assert.Equal(t, "abc123hash", h, "lookup with different size should succeed when IgnoreSize=true")

	// Clean up config change for other tests
	ci.IgnoreSize = false
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	if !kv.Supported() {
		t.Skip("hasher is not supported on this OS")
	}
	t.Run("UploadFromCrypt", f.testUploadFromCrypt)
	t.Run("UpdateInFlightHashing", func(t *testing.T) {
		f.testUpdateInFlightHashing(t)
	})
	t.Run("FingerprintIgnoreSize", f.testFingerprintIgnoreSize)
}

var _ fstests.InternalTester = (*Fs)(nil)
