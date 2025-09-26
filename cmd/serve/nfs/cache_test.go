//go:build unix

package nfs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NB to test the symlink cache, running with elevated permissions is needed
const testSymlinkCache = "go test -c && sudo setcap cap_dac_read_search+ep ./nfs.test && ./nfs.test -test.v -test.run TestCache/symlink"

// Check basic CRUD operations
func testCacheCRUD(t *testing.T, h *Handler, c Cache, fileName string) {
	isMetadata := strings.HasSuffix(fileName, ".metadata")

	// Check reading a non existent handle returns an error
	_, _, err := c.FromHandle([]byte{10})
	assert.Error(t, err)

	// Write a handle
	splitPath := []string{"dir", fileName}
	fh := c.ToHandle(h.billyFS, splitPath)
	assert.True(t, len(fh) > 0)
	if isMetadata {
		assert.Equal(t, metadataSuffix, fh[len(fh)-len(metadataSuffix):])
	} else {
		assert.NotEqual(t, metadataSuffix, fh[len(fh)-len(metadataSuffix):])
	}

	// Read the handle back
	newFs, newSplitPath, err := c.FromHandle(fh)
	require.NoError(t, err)
	assert.Equal(t, h.billyFS, newFs)
	assert.Equal(t, splitPath, newSplitPath)

	// Invalidate the handle
	err = c.InvalidateHandle(h.billyFS, fh)
	require.NoError(t, err)

	// Invalidate the handle twice
	err = c.InvalidateHandle(h.billyFS, fh)
	require.NoError(t, err)

	// Check the handle is gone and returning stale handle error
	_, _, err = c.FromHandle(fh)
	if !isMetadata {
		require.Error(t, err)
		assert.Equal(t, errStaleHandle, err)
	} else {
		// Can't invalidate metadata handles
		require.NoError(t, err)
	}
}

// Thrash the cache operations in parallel on different files
func testCacheThrashDifferent(t *testing.T, h *Handler, c Cache) {
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testCacheCRUD(t, h, c, fmt.Sprintf("file-%d", i))
		}()
	}
	wg.Wait()
}

// Thrash the cache operations in parallel on the same file
func testCacheThrashSame(t *testing.T, h *Handler, c Cache) {
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Write a handle
			splitPath := []string{"file"}
			fh := c.ToHandle(h.billyFS, splitPath)
			assert.True(t, len(fh) > 0)

			// Read the handle back
			newFs, newSplitPath, err := c.FromHandle(fh)
			if err != nil {
				assert.Equal(t, errStaleHandle, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, h.billyFS, newFs)
				assert.Equal(t, splitPath, newSplitPath)
			}

			// Invalidate the handle
			err = c.InvalidateHandle(h.billyFS, fh)
			require.NoError(t, err)

			// Check the handle is gone and returning stale handle error
			_, _, err = c.FromHandle(fh)
			if err != nil {
				require.Error(t, err)
				assert.Equal(t, errStaleHandle, err)
			}
		}()
	}
	wg.Wait()
}

func TestCache(t *testing.T) {
	// Quieten the flood of ERROR messages!
	ci := fs.GetConfig(context.Background())
	oldLogLevel := ci.LogLevel
	ci.LogLevel = fs.LogLevelEmergency
	//ci.LogLevel = fs.LogLevelDebug
	defer func() {
		ci.LogLevel = oldLogLevel
	}()
	billyFS := &FS{nil} // place holder billyFS
	for _, cacheType := range []handleCache{cacheMemory, cacheDisk, cacheSymlink} {
		t.Run(cacheType.String(), func(t *testing.T) {
			h := &Handler{
				vfs:     vfs.New(object.MemoryFs, nil),
				billyFS: billyFS,
			}
			h.vfs.Opt.MetadataExtension = ".metadata"
			h.opt.HandleLimit = 1000
			h.opt.HandleCache = cacheType
			h.opt.HandleCacheDir = t.TempDir()
			c, err := h.getCache()
			if err == ErrorSymlinkCacheNotSupported {
				t.Skip(err.Error())
			}
			if err == ErrorSymlinkCacheNoPermission {
				t.Skip("Need more permissions to run symlink cache tests: " + testSymlinkCache)
			}
			require.NoError(t, err)

			t.Run("Empty", func(t *testing.T) {
				// Write a handle
				splitPath := []string{""}
				fh := c.ToHandle(h.billyFS, splitPath)
				assert.True(t, len(fh) > 0)

				// Read the handle back
				newFs, newSplitPath, err := c.FromHandle(fh)
				require.NoError(t, err)
				assert.Equal(t, h.billyFS, newFs)
				assert.Equal(t, splitPath, newSplitPath)
				testCacheCRUD(t, h, c, "file")
			})
			t.Run("CRUD", func(t *testing.T) {
				testCacheCRUD(t, h, c, "file")
			})
			// NB the default caching handler is not thread safe!
			if cacheType != cacheMemory {
				t.Run("ThrashDifferent", func(t *testing.T) {
					testCacheThrashDifferent(t, h, c)
				})
				t.Run("ThrashSame", func(t *testing.T) {
					testCacheThrashSame(t, h, c)
				})
				// Metadata file handles only supported on non memory
				t.Run("CRUDMetadata", func(t *testing.T) {
					testCacheCRUD(t, h, c, "file.metadata")
				})
			}
		})
	}
}
