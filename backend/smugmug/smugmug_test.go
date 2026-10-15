package smugmug_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/smugmug"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testImage = "rclone-test-image1.jpg"

// TestIntegration runs media-aware integration tests against the remote.
func TestIntegration(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()

	if *fstest.RemoteName == "" {
		*fstest.RemoteName = "TestSmugMug:"
	}
	f, err := fs.NewFs(ctx, *fstest.RemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("couldn't create SmugMug backend - skipping tests: %v", err)
	}
	require.NoError(t, err)

	t.Run("RootCommand", func(t *testing.T) {
		do, ok := f.(fs.Commander)
		require.True(t, ok, "SmugMug should implement fs.Commander")

		result, err := do.Command(ctx, "root", nil, nil)
		require.NoError(t, err)

		var root struct {
			RootNodeURI string `json:"root_node_uri"`
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &root))
		assert.NotEmpty(t, root.RootNodeURI)
	})

	localFs, err := fs.NewFs(ctx, filepath.FromSlash("../googlephotos/testfiles"))
	require.NoError(t, err)
	srcObj, err := localFs.NewObject(ctx, testImage)
	require.NoError(t, err)

	remote := "rclone-smugmug-test-" + random.String(12) + ".jpg"
	srcHash, err := srcObj.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	require.NotEmpty(t, srcHash)

	in, err := srcObj.Open(ctx)
	require.NoError(t, err)
	dstObj, putErr := f.Put(ctx, in, fs.NewOverrideRemote(srcObj, remote), fs.MetadataOption(fs.Metadata{
		"title":   "rclone integration test",
		"caption": "temporary SmugMug backend test image",
	}))
	_ = in.Close()
	require.NoError(t, putErr)
	require.Equal(t, remote, dstObj.Remote())
	t.Cleanup(func() {
		if err := dstObj.Remove(ctx); err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
			t.Logf("failed to remove uploaded test image %q: %v", remote, err)
		}
	})

	t.Run("ObjectHash", func(t *testing.T) {
		dstHash, err := dstObj.Hash(ctx, hash.MD5)
		require.NoError(t, err)
		assert.Equal(t, srcHash, dstHash)
	})

	t.Run("ObjectMetadata", func(t *testing.T) {
		metadata, err := fs.GetMetadata(ctx, dstObj)
		require.NoError(t, err)
		assert.Equal(t, "rclone integration test", metadata["title"])
		assert.Equal(t, "temporary SmugMug backend test image", metadata["caption"])
	})

	t.Run("ListAndNewObject", func(t *testing.T) {
		require.NoError(t, waitForSmugMug(t, func() error {
			entries, err := f.List(ctx, "")
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.Remote() == remote {
					return nil
				}
			}
			return fmt.Errorf("%q not found in listing", remote)
		}))

		listedObj, err := f.NewObject(ctx, remote)
		require.NoError(t, err)
		require.Equal(t, remote, listedObj.Remote())

		dstHash, err := listedObj.Hash(ctx, hash.MD5)
		require.NoError(t, err)
		if dstHash != "" {
			assert.Equal(t, srcHash, dstHash)
		}
	})

	t.Run("ObjectOpen", func(t *testing.T) {
		require.NoError(t, waitForSmugMug(t, func() error {
			obj, err := f.NewObject(ctx, remote)
			if err != nil {
				return err
			}
			in, err := obj.Open(ctx)
			if err != nil {
				return err
			}
			defer func() {
				_ = in.Close()
			}()

			buf := make([]byte, 512)
			n, err := io.ReadFull(in, buf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return err
			}
			if got := http.DetectContentType(buf[:n]); got != "image/jpeg" {
				return fmt.Errorf("download content type = %q, want image/jpeg", got)
			}
			return nil
		}))
	})
}

func waitForSmugMug(t *testing.T, fn func() error) error {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}
