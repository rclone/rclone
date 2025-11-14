package hasher

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/object"
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

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	if !kv.Supported() {
		t.Skip("hasher is not supported on this OS")
	}
	t.Run("UploadFromCrypt", f.testUploadFromCrypt)
	t.Run("ReadOnlyFlag", f.testReadOnlyFlag)
}

type testGetter struct{}

func (s *testGetter) Get(key string) (value string, ok bool) {
	switch key {
	case "remote":
		return "/hasher-test", true
	case "hashes":
		return "md5", true
	case "max_age":
		return "off", true
	case "auto_size":
		return "0", true
	case "read_only":
		return "true", true
	default:
		return key, true
	}
}

func (f *Fs) testReadOnlyFlag(t *testing.T) {
	ctx := context.Background()
	mapper := configmap.New()
	mapper.AddGetter(&testGetter{}, 1)
	hasherFs, err := NewFs(ctx, "hasher-test", "/hasher-test", mapper)
	assert.NoError(t, err)
	assert.NotNil(t, hasherFs)

	fileInfo := object.NewStaticObjectInfo("hasher-test", time.Now(), 128, true, nil, nil)
	_, err = hasherFs.Put(ctx, strings.NewReader("dogs and cats"), fileInfo)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only file system")
}
