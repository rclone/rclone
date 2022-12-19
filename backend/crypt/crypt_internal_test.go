package crypt

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create a temporary local fs to upload things from

func makeTempLocalFs(t *testing.T) (localFs fs.Fs) {
	localFs, err := fs.TemporaryLocalFs(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, localFs.Rmdir(context.Background(), ""))
	})
	return localFs
}

// Upload a file to a remote
func uploadFile(t *testing.T, f fs.Fs, remote, contents string) (obj fs.Object) {
	inBuf := bytes.NewBufferString(contents)
	t1 := time.Date(2012, time.December, 17, 18, 32, 31, 0, time.UTC)
	upSrc := object.NewStaticObjectInfo(remote, t1, int64(len(contents)), true, nil, nil)
	obj, err := f.Put(context.Background(), inBuf, upSrc)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, obj.Remove(context.Background()))
	})
	return obj
}

// Test the ObjectInfo
func testObjectInfo(t *testing.T, f *Fs, wrap bool) {
	var (
		contents = random.String(100)
		path     = "hash_test_object"
		ctx      = context.Background()
	)
	if wrap {
		path = "_wrap"
	}

	localFs := makeTempLocalFs(t)

	obj := uploadFile(t, localFs, path, contents)

	// encrypt the data
	inBuf := bytes.NewBufferString(contents)
	var outBuf bytes.Buffer
	enc, err := f.cipher.newEncrypter(inBuf, nil)
	require.NoError(t, err)
	nonce := enc.nonce // read the nonce at the start
	_, err = io.Copy(&outBuf, enc)
	require.NoError(t, err)

	var oi fs.ObjectInfo = obj
	if wrap {
		// wrap the object in an fs.ObjectUnwrapper if required
		oi = fs.NewOverrideRemote(oi, "new_remote")
	}

	// wrap the object in a crypt for upload using the nonce we
	// saved from the encrypter
	src := f.newObjectInfo(oi, nonce)

	// Test ObjectInfo methods
	if !f.opt.NoDataEncryption {
		assert.Equal(t, int64(outBuf.Len()), src.Size())
	}
	assert.Equal(t, f, src.Fs())
	assert.NotEqual(t, path, src.Remote())

	// Test ObjectInfo.Hash
	wantHash := md5.Sum(outBuf.Bytes())
	gotHash, err := src.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%x", wantHash), gotHash)
}

func testComputeHash(t *testing.T, f *Fs) {
	var (
		contents = random.String(100)
		path     = "compute_hash_test"
		ctx      = context.Background()
		hashType = f.Fs.Hashes().GetOne()
	)

	if hashType == hash.None {
		t.Skipf("%v: does not support hashes", f.Fs)
	}

	localFs := makeTempLocalFs(t)

	// Upload a file to localFs as a test object
	localObj := uploadFile(t, localFs, path, contents)

	// Upload the same data to the remote Fs also
	remoteObj := uploadFile(t, f, path, contents)

	// Calculate the expected Hash of the remote object
	computedHash, err := f.ComputeHash(ctx, remoteObj.(*Object), localObj, hashType)
	require.NoError(t, err)

	// Test computed hash matches remote object hash
	remoteObjHash, err := remoteObj.(*Object).Object.Hash(ctx, hashType)
	require.NoError(t, err)
	assert.Equal(t, remoteObjHash, computedHash)
}

// InternalTest is called by fstests.Run to extra tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("ObjectInfo", func(t *testing.T) { testObjectInfo(t, f, false) })
	t.Run("ObjectInfoWrap", func(t *testing.T) { testObjectInfo(t, f, true) })
	t.Run("ComputeHash", func(t *testing.T) { testComputeHash(t, f) })
}
