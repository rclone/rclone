//go:build unix && linux

/*
This implements an efficient disk cache for the NFS file handles for
Linux only.

1. The destination paths are stored as symlink destinations. These
can be stored in the directory for maximum efficiency.

2. The on disk handle of the cache file is returned to NFS with
name_to_handle_at(). This means that if the cache is deleted and
restored, the file handle mapping will be lost.

3. These handles are looked up with open_by_handle_at() so no
searching through directory trees is needed.

Note that open_by_handle_at requires CAP_DAC_READ_SEARCH so rclone
will need to be run as root or with elevated permissions.

Test with

go test -c && sudo setcap cap_dac_read_search+ep ./nfs.test && ./nfs.test -test.v -test.run TestCache/symlink

*/

package nfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

// emptyPath is written instead of "" as symlinks can't be empty
var (
	emptyPath      = "\x01"
	emptyPathBytes = []byte(emptyPath)
)

// Turn the diskHandler into a symlink cache
//
// This also tests the cache works as it may not have enough
// permissions or have be the correct Linux version.
func (dh *diskHandler) makeSymlinkCache() error {
	path := filepath.Join(dh.cacheDir, "test")
	fullPath := "testpath"
	fh := []byte{1, 2, 3, 4, 5}

	// Create a symlink
	newFh, err := dh.symlinkCacheWrite(fh, path, fullPath)
	fs.Debugf(nil, "newFh = %q", newFh)
	if err != nil {
		return fmt.Errorf("symlink cache write test failed: %w", err)
	}
	defer func() {
		_ = os.Remove(path)
	}()

	// Read it back
	newFullPath, err := dh.symlinkCacheRead(newFh, path)
	fs.Debugf(nil, "newFullPath = %q", newFullPath)
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			return ErrorSymlinkCacheNoPermission
		}
		return fmt.Errorf("symlink cache read test failed: %w", err)
	}

	// Check result all OK
	if string(newFullPath) != fullPath {
		return fmt.Errorf("symlink cache read test failed: expecting %q read %q", string(newFullPath), fullPath)
	}

	// If OK install symlink cache
	dh.read = dh.symlinkCacheRead
	dh.write = dh.symlinkCacheWrite
	dh.remove = dh.symlinkCacheRemove
	dh.suffix = dh.symlinkCacheSuffix

	return nil
}

// Prefixes a []byte with its length as a 4-byte big-endian integer.
func addLengthPrefix(data []byte) []byte {
	length := uint32(len(data))
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, length)
	if err != nil {
		// This should never fail
		panic(err)
	}
	buf.Write(data)
	return buf.Bytes()
}

// Removes the 4-byte big-endian length prefix from a []byte.
func removeLengthPrefix(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, errors.New("file handle too short")
	}
	length := binary.BigEndian.Uint32(data[:4])
	if int(length) != len(data)-4 {
		return nil, errors.New("file handle invalid length")
	}
	return data[4 : 4+length], nil
}

// Write the fullPath into cachePath returning the possibly updated fh
//
// This writes the fullPath into the file with the cachePath given and
// returns the handle for that file so we can look it up later.
func (dh *diskHandler) symlinkCacheWrite(fh []byte, cachePath string, fullPath string) (newFh []byte, err error) {
	//defer log.Trace(nil, "fh=%x, cachePath=%q, fullPath=%q", fh, cachePath)("newFh=%x, err=%v", &newFh, &err)

	// Can't write an empty symlink so write a substitution
	if fullPath == "" {
		fullPath = emptyPath
	}

	// Write the symlink
	err = os.Symlink(fullPath, cachePath)
	if err != nil && !errors.Is(err, syscall.EEXIST) {
		return nil, fmt.Errorf("symlink cache create symlink: %w", err)
	}

	// Read the newly created symlinks handle
	handle, _, err := unix.NameToHandleAt(unix.AT_FDCWD, cachePath, 0)
	if err != nil {
		return nil, fmt.Errorf("symlink cache name to handle at: %w", err)
	}

	// Store the handle type if it hasn't changed
	// This should run once only when called by makeSymlinkCache
	if dh.handleType != handle.Type() {
		dh.handleType = handle.Type()
	}

	// Adjust the raw handle so it has a length prefix
	return addLengthPrefix(handle.Bytes()), nil
}

// Read the contents of (fh, cachePath)
//
// This reads the symlink with the corresponding file handle and
// returns the contents. It ignores the cachePath which will be
// pointing in the wrong place.
//
// Note that the caller needs CAP_DAC_READ_SEARCH to use this.
func (dh *diskHandler) symlinkCacheRead(fh []byte, cachePath string) (fullPath []byte, err error) {
	//defer log.Trace(nil, "fh=%x, cachePath=%q", fh, cachePath)("fullPath=%q, err=%v", &fullPath, &err)

	// First check and remove the file handle prefix length
	fh, err = removeLengthPrefix(fh)
	if err != nil {
		return nil, fmt.Errorf("symlink cache open by handle at: %w", err)
	}

	// Find the file with the handle passed in
	handle := unix.NewFileHandle(dh.handleType, fh)
	fd, err := unix.OpenByHandleAt(unix.AT_FDCWD, handle, unix.O_RDONLY|unix.O_PATH|unix.O_NOFOLLOW) // needs O_PATH for symlinks
	if err != nil {
		return nil, fmt.Errorf("symlink cache open by handle at: %w", err)
	}

	// Close it on exit
	defer func() {
		newErr := unix.Close(fd)
		if err != nil {
			err = newErr
		}
	}()

	// Read the symlink which is the path required
	buf := make([]byte, 1024)              // Max path length
	n, err := unix.Readlinkat(fd, "", buf) // It will (silently) truncate the contents, in case the buffer is too small to hold all of the contents.
	if err != nil {
		return nil, fmt.Errorf("symlink cache read: %w", err)
	}
	fullPath = buf[:n:n]

	// Undo empty symlink substitution
	if bytes.Equal(fullPath, emptyPathBytes) {
		fullPath = buf[:0:0]
	}

	return fullPath, nil
}

// Remove the (fh, cachePath) file
func (dh *diskHandler) symlinkCacheRemove(fh []byte, cachePath string) error {
	// First read the path
	fullPath, err := dh.symlinkCacheRead(fh, cachePath)
	if err != nil {
		return err
	}

	// fh for the actual cache file
	fh = hashPath(string(fullPath))

	// cachePath for the actual cache file
	cachePath = dh.handleToPath(fh)

	return os.Remove(cachePath)
}

// Return a suffix for the file handle or nil
func (dh *diskHandler) symlinkCacheSuffix(fh []byte) []byte {
	if len(fh) < 4 {
		return nil
	}
	length := int(binary.BigEndian.Uint32(fh[:4])) + 4
	if len(fh) <= length {
		return nil
	}
	return fh[length:]
}
