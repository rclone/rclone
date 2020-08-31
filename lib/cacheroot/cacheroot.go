package cacheroot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/file"
)

// CreateCacheRoot will derive and make a subsystem cache path.
//
// Returned root OS path is an absolute path with UNC prefix,
// OS-specific path separators, and encoded with OS-specific encoder.
//
// Additionally it is returned as a standard path without UNC prefix,
// with slash path separators, and standard (internal) encoding.
//
// Care is taken when creating OS paths so that the ':' separator
// following a drive letter is not encoded, e.g. into unicode fullwidth colon.
//
// parentOSPath should contain an absolute local path in OS encoding.
//
// Note: instead of fs.Fs it takes name and root as plain strings
// to prevent import loops due to dependency on the fs package.
func CreateCacheRoot(parentOSPath, fsName, fsRoot, cacheName string) (rootOSPath, standardPath string, err error) {
	// Get a relative cache path representing the remote.
	relativeDir := fsRoot
	if runtime.GOOS == "windows" && strings.HasPrefix(relativeDir, `//?/`) {
		// Trim off the leading "//" to make the result
		// valid for appending to another path.
		relativeDir = relativeDir[2:]
	}
	relativeDir = fsName + "/" + relativeDir

	// Derive and make the cache root directory
	relativeOSPath := filepath.FromSlash(encoder.OS.FromStandardPath(relativeDir))
	rootOSPath = file.UNCPath(filepath.Join(parentOSPath, cacheName, relativeOSPath))
	if err = os.MkdirAll(rootOSPath, 0700); err != nil {
		return "", "", errors.Wrapf(err, "failed to create %s cache directory", cacheName)
	}

	parentStdPath := encoder.OS.ToStandardPath(filepath.ToSlash(parentOSPath))
	standardPath = fmt.Sprintf("%s/%s/%s", parentStdPath, cacheName, relativeDir)
	return rootOSPath, standardPath, nil
}
