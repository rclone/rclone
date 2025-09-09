package filelu

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
)

// errFileNotFound represent file not found error
var errFileNotFound = errors.New("file not found")

// getFileCode retrieves the file code for a given file path
func (f *Fs) getFileCode(ctx context.Context, filePath string) (string, error) {
	// Prepare parent directory
	parentDir := path.Dir(filePath)

	// Call List to get all the files
	result, err := f.getFolderList(ctx, parentDir)
	if err != nil {
		return "", err
	}

	for _, file := range result.Result.Files {
		filePathFromServer := parentDir + "/" + file.Name
		if parentDir == "/" {
			filePathFromServer = "/" + file.Name
		}
		if filePath == filePathFromServer {
			return file.FileCode, nil
		}
	}

	return "", errFileNotFound
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) fromStandardPath(remote string) string {
	return f.opt.Enc.FromStandardPath(remote)
}

func (f *Fs) toStandardPath(remote string) string {
	return f.opt.Enc.ToStandardPath(remote)
}

// Hashes returns an empty hash set, indicating no hash support
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet() // Properly creates an empty hash set
}

// Name returns the remote name
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root path
func (f *Fs) Root() string {
	return f.root
}

// Precision returns the precision of the remote
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

func (f *Fs) String() string {
	return fmt.Sprintf("FileLu root '%s'", f.root)
}

// isFileCode checks if a string looks like a file code
func isFileCode(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func shouldRetry(err error) bool {
	return fserrors.ShouldRetry(err)
}

func shouldRetryHTTP(code int) bool {
	return code == 429 || code >= 500
}

func rootSplit(absPath string) (bucket, bucketPath string) {
	// No bucket
	if absPath == "" {
		return "", ""
	}
	slash := strings.IndexRune(absPath, '/')
	// Bucket but no path
	if slash < 0 {
		return absPath, ""
	}
	return absPath[:slash], absPath[slash+1:]
}
