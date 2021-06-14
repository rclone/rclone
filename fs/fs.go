// Package fs is a generic file system interface for rclone object storage systems
package fs

import (
	"context"
	"io"
	"math"
	"time"

	"github.com/pkg/errors"
)

// Constants
const (
	// ModTimeNotSupported is a very large precision value to show
	// mod time isn't supported on this Fs
	ModTimeNotSupported = 100 * 365 * 24 * time.Hour
	// MaxLevel is a sentinel representing an infinite depth for listings
	MaxLevel = math.MaxInt32
)

// Globals
var (
	// ErrorNotFoundInConfigFile is returned by NewFs if not found in config file
	ErrorNotFoundInConfigFile        = errors.New("didn't find section in config file")
	ErrorCantPurge                   = errors.New("can't purge directory")
	ErrorCantCopy                    = errors.New("can't copy object - incompatible remotes")
	ErrorCantMove                    = errors.New("can't move object - incompatible remotes")
	ErrorCantDirMove                 = errors.New("can't move directory - incompatible remotes")
	ErrorCantUploadEmptyFiles        = errors.New("can't upload empty files to this remote")
	ErrorDirExists                   = errors.New("can't copy directory - destination already exists")
	ErrorCantSetModTime              = errors.New("can't set modified time")
	ErrorCantSetModTimeWithoutDelete = errors.New("can't set modified time without deleting existing object")
	ErrorDirNotFound                 = errors.New("directory not found")
	ErrorObjectNotFound              = errors.New("object not found")
	ErrorLevelNotSupported           = errors.New("level value not supported")
	ErrorListAborted                 = errors.New("list aborted")
	ErrorListBucketRequired          = errors.New("bucket or container name is needed in remote")
	ErrorIsFile                      = errors.New("is a file not a directory")
	ErrorNotAFile                    = errors.New("is not a regular file")
	ErrorNotDeleting                 = errors.New("not deleting files as there were IO errors")
	ErrorNotDeletingDirs             = errors.New("not deleting directories as there were IO errors")
	ErrorOverlapping                 = errors.New("can't sync or move files on overlapping remotes")
	ErrorDirectoryNotEmpty           = errors.New("directory not empty")
	ErrorImmutableModified           = errors.New("immutable file modified")
	ErrorPermissionDenied            = errors.New("permission denied")
	ErrorCantShareDirectories        = errors.New("this backend can't share directories with link")
	ErrorNotImplemented              = errors.New("optional feature not implemented")
	ErrorCommandNotFound             = errors.New("command not found")
	ErrorFileNameTooLong             = errors.New("file name too long")
)

// CheckClose is a utility function used to check the return from
// Close in a defer statement.
func CheckClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// FileExists returns true if a file remote exists.
// If remote is a directory, FileExists returns false.
func FileExists(ctx context.Context, fs Fs, remote string) (bool, error) {
	_, err := fs.NewObject(ctx, remote)
	if err != nil {
		if err == ErrorObjectNotFound || err == ErrorNotAFile || err == ErrorPermissionDenied {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetModifyWindow calculates the maximum modify window between the given Fses
// and the Config.ModifyWindow parameter.
func GetModifyWindow(ctx context.Context, fss ...Info) time.Duration {
	window := GetConfig(ctx).ModifyWindow
	for _, f := range fss {
		if f != nil {
			precision := f.Precision()
			if precision == ModTimeNotSupported {
				return ModTimeNotSupported
			}
			if precision > window {
				window = precision
			}
		}
	}
	return window
}
