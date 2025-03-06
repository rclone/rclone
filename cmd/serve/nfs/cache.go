//go:build unix

package nfs

import (
	"bytes"
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

// Metadata files have the file handle of their source file with this
// suffixed so we can look them up directly from the file handle.
//
// Note that this is 4 bytes - using a non multiple of 4 will cause
// the Linux NFS client not to be able to read any files.
//
// The value is big endian 0x00000001
var metadataSuffix = []byte{0x00, 0x00, 0x00, 0x01}

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
	suffix     func(fh []byte) []byte // returns nil for no suffix or the suffix
	handleType int32                  //nolint:unused // used by the symlink cache
	metadata   string                 // extension for metadata
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
		suffix:   dh.diskCacheSuffix,
		metadata: h.vfs.Opt.MetadataExtension,
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

// Return true if name represents a metadata file
//
// It returns the underlying path
func (dh *diskHandler) isMetadataFile(name string) (rawName string, found bool) {
	if dh.metadata == "" {
		return name, false
	}
	rawName, found = strings.CutSuffix(name, dh.metadata)
	return rawName, found
}

// ToHandle takes a file and represents it with an opaque handle to reference it.
// In stateless nfs (when it's serving a unix fs) this can be the device + inode
// but we can generalize with a stateful local cache of handed out IDs.
func (dh *diskHandler) ToHandle(f billy.Filesystem, splitPath []string) (fh []byte) {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	fullPath := path.Join(splitPath...)
	// metadata file has file handle of original file
	fullPath, isMetadataFile := dh.isMetadataFile(fullPath)
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
	// metadata file handle is suffixed with metadataSuffix
	if isMetadataFile {
		fh = append(fh, metadataSuffix...)
	}
	return fh
}

// Write the fullPath into cachePath returning the possibly updated fh
func (dh *diskHandler) diskCacheWrite(fh []byte, cachePath string, fullPath string) ([]byte, error) {
	return fh, os.WriteFile(cachePath, []byte(fullPath), 0600)
}

var (
	errStaleHandle = &nfs.NFSStatusError{NFSStatus: nfs.NFSStatusStale}
)

// Test to see if a fh is a metadata handle and if so return the underlying handle
func (dh *diskHandler) isMetadataHandle(fh []byte) (isMetadata bool, newFh []byte, err error) {
	if dh.metadata == "" {
		return false, fh, nil
	}
	suffix := dh.suffix(fh)
	if len(suffix) == 0 {
		// OK
		return false, fh, nil
	} else if bytes.Equal(suffix, metadataSuffix) {
		return true, fh[:len(fh)-len(suffix)], nil
	}
	fs.Errorf("nfs", "Bad file handle suffix %X", suffix)
	return false, nil, errStaleHandle
}

// FromHandle converts from an opaque handle to the file it represents
func (dh *diskHandler) FromHandle(fh []byte) (f billy.Filesystem, splitPath []string, err error) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()
	isMetadata, fh, err := dh.isMetadataHandle(fh)
	if err != nil {
		return nil, nil, err
	}
	cachePath := dh.handleToPath(fh)
	fullPathBytes, err := dh.read(fh, cachePath)
	if err != nil {
		fs.Errorf("nfs", "Stale handle %q: %v", cachePath, err)
		return nil, nil, errStaleHandle
	}
	if isMetadata {
		fullPathBytes = append(fullPathBytes, []byte(dh.metadata)...)
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
	isMetadata, fh, err := dh.isMetadataHandle(fh)
	if err != nil {
		return err
	}
	if isMetadata {
		// Can't invalidate a metadata handle as it is synthetic
		return nil
	}
	cachePath := dh.handleToPath(fh)
	err = dh.remove(fh, cachePath)
	if err != nil {
		fs.Errorf("nfs", "Failed to remove handle %q: %v", cachePath, err)
	}
	return nil
}

// Remove the (fh, cachePath) file
func (dh *diskHandler) diskCacheRemove(fh []byte, cachePath string) error {
	return os.Remove(cachePath)
}

// Return a suffix for the file handle or nil
func (dh *diskHandler) diskCacheSuffix(fh []byte) []byte {
	if len(fh) <= md5.Size {
		return nil
	}
	return fh[md5.Size:]
}

// HandleLimit exports how many file handles can be safely stored by this cache.
func (dh *diskHandler) HandleLimit() int {
	return math.MaxInt
}
