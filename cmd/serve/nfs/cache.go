//go:build unix

package nfs

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	billy "github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/file"
	"github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// Errors on cache initialisation
var (
	ErrorSymlinkCacheNotSupported = errors.New("symlink cache not supported on " + runtime.GOOS)
	ErrorSymlinkCacheNoPermission = errors.New("symlink cache must be run as root or with CAP_DAC_READ_SEARCH")
)

// Cache controls the file handle cache implementation
type Cache interface {
	// ToHandle takes a file and represents it with an opaque handle to reference it.
	// In stateless nfs (when it's serving a unix fs) this can be the device + inode
	// but we can generalize with a stateful local cache of handed out IDs.
	ToHandle(f billy.Filesystem, path []string) []byte

	// FromHandle converts from an opaque handle to the file it represents
	FromHandle(fh []byte) (billy.Filesystem, []string, error)

	// Invalidate the handle passed - used on rename and delete
	InvalidateHandle(fs billy.Filesystem, handle []byte) error

	// HandleLimit exports how many file handles can be safely stored by this cache.
	HandleLimit() int
}

// Set the cache of the handler to the type required by the user
func (h *Handler) getCache() (c Cache, err error) {
	fs.Debugf("nfs", "Starting %v handle cache", h.opt.HandleCache)
	switch h.opt.HandleCache {
	case cacheMemory:
		return nfshelper.NewCachingHandler(h, h.opt.HandleLimit), nil
	case cacheDisk:
		return newDiskHandler(h)
	case cacheSymlink:
		dh, err := newDiskHandler(h)
		if err != nil {
			return nil, err
		}
		err = dh.makeSymlinkCache()
		if err != nil {
			return nil, err
		}
		return dh, nil
	}
	return nil, errors.New("unknown handle cache type")
}

// diskHandler implements an on disk NFS file handle cache
type diskHandler struct {
	mu         sync.RWMutex
	cacheDir   string
	billyFS    billy.Filesystem
	write      func(fh []byte, cachePath string, fullPath string) ([]byte, error)
	read       func(fh []byte, cachePath string) ([]byte, error)
	remove     func(fh []byte, cachePath string) error
	handleType int32 //nolint:unused // used by the symlink cache
}

// Create a new disk handler
func newDiskHandler(h *Handler) (dh *diskHandler, err error) {
	cacheDir := h.opt.HandleCacheDir
	// If cacheDir isn't set then make one from the config
	if cacheDir == "" {
		// How the VFS was configured
		configString := fs.ConfigString(h.vfs.Fs())
		// Turn it into a valid OS directory name
		dirName := encoder.OS.ToStandardName(configString)
		cacheDir = filepath.Join(config.GetCacheDir(), "serve-nfs-handle-cache-"+h.opt.HandleCache.String(), dirName)
	}
	// Create the cache dir
	err = file.MkdirAll(cacheDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("disk handler mkdir failed: %v", err)
	}
	dh = &diskHandler{
		cacheDir: cacheDir,
		billyFS:  h.billyFS,
		write:    dh.diskCacheWrite,
		read:     dh.diskCacheRead,
		remove:   dh.diskCacheRemove,
	}
	fs.Infof("nfs", "Storing handle cache in %q", dh.cacheDir)
	return dh, nil
}

// Convert a path to a hash
func hashPath(fullPath string) []byte {
	hash := md5.Sum([]byte(fullPath))
	return hash[:]
}

// Convert a handle to a path on disk for the handle
func (dh *diskHandler) handleToPath(fh []byte) (cachePath string) {
	fhString := hex.EncodeToString(fh)
	if len(fhString) <= 4 {
		cachePath = filepath.Join(dh.cacheDir, fhString)
	} else {
		cachePath = filepath.Join(dh.cacheDir, fhString[0:2], fhString[2:4], fhString)
	}
	return cachePath
}

// ToHandle takes a file and represents it with an opaque handle to reference it.
// In stateless nfs (when it's serving a unix fs) this can be the device + inode
// but we can generalize with a stateful local cache of handed out IDs.
func (dh *diskHandler) ToHandle(f billy.Filesystem, splitPath []string) (fh []byte) {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	fullPath := path.Join(splitPath...)
	fh = hashPath(fullPath)
	cachePath := dh.handleToPath(fh)
	cacheDir := filepath.Dir(cachePath)
	err := os.MkdirAll(cacheDir, 0700)
	if err != nil {
		fs.Errorf("nfs", "Couldn't create cache file handle directory: %v", err)
		return fh
	}
	fh, err = dh.write(fh, cachePath, fullPath)
	if err != nil {
		fs.Errorf("nfs", "Couldn't create cache file handle: %v", err)
		return fh
	}
	return fh
}

// Write the fullPath into cachePath returning the possibly updated fh
func (dh *diskHandler) diskCacheWrite(fh []byte, cachePath string, fullPath string) ([]byte, error) {
	return fh, os.WriteFile(cachePath, []byte(fullPath), 0600)
}

var errStaleHandle = &nfs.NFSStatusError{NFSStatus: nfs.NFSStatusStale}

// FromHandle converts from an opaque handle to the file it represents
func (dh *diskHandler) FromHandle(fh []byte) (f billy.Filesystem, splitPath []string, err error) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()
	cachePath := dh.handleToPath(fh)
	fullPathBytes, err := dh.read(fh, cachePath)
	if err != nil {
		fs.Errorf("nfs", "Stale handle %q: %v", cachePath, err)
		return nil, nil, errStaleHandle
	}
	splitPath = strings.Split(string(fullPathBytes), "/")
	return dh.billyFS, splitPath, nil
}

// Read the contents of (fh, cachePath)
func (dh *diskHandler) diskCacheRead(fh []byte, cachePath string) ([]byte, error) {
	return os.ReadFile(cachePath)
}

// Invalidate the handle passed - used on rename and delete
func (dh *diskHandler) InvalidateHandle(f billy.Filesystem, fh []byte) error {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	cachePath := dh.handleToPath(fh)
	err := dh.remove(fh, cachePath)
	if err != nil {
		fs.Errorf("nfs", "Failed to remove handle %q: %v", cachePath, err)
	}
	return nil
}

// Remove the (fh, cachePath) file
func (dh *diskHandler) diskCacheRemove(fh []byte, cachePath string) error {
	return os.Remove(cachePath)
}

// HandleLimit exports how many file handles can be safely stored by this cache.
func (dh *diskHandler) HandleLimit() int {
	return math.MaxInt
}
