package filelu

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
)

// ErrFileNotFound represent file not found error
var ErrFileNotFound error = errors.New("file not found")

var commandHelp = []fs.CommandHelp{{
	Name:  "rename",
	Short: "Rename a file in a FileLu directory",
	Long: `
For example:

    rclone backend rename filelu:/file-path/hello.txt "hello_new_name.txt"
`,
}, {
	Name:  "movefile",
	Short: "Move file within the remote FileLu directory",
	Long: `
For example:

    rclone backend movefile filelu:/source-path/hello.txt /destination-path/
`,
}, {
	Name:  "movefolder",
	Short: "Move a folder on remote FileLu",
	Long: `
For example:

    rclone backend movefolder filelu:/sorce-fld-path/hello-folder/ /destication-fld-path/hello-folder/
`,
}, {
	Name:  "renamefolder",
	Short: "Rename a folder on FileLu",
	Long: `
For example:

    rclone backend renamefolder filelu:/folder-path/folder-name "new-folder-name"
`,
}}

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
		if filePath == parentDir+"/"+file.Name {
			return file.FileCode, nil
		}
	}

	return "", ErrFileNotFound
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) fromStandardPath(remote string) string {
	return encodePath(remote)
}

func (f *Fs) toStandardPath(remote string) string {
	return decodePath(remote)
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

func encodePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if segment == "" {
			continue // Skip empty segments to preserve leading/trailing slashes
		}
		// Encode the segment if it contains '/' or '?' or is not ASCII
		if strings.Contains(segment, "/") || strings.Contains(segment, "?") || !isASCII(segment) {
			// Encode segment using Base64 URL encoding
			encoded := base64.URLEncoding.EncodeToString([]byte(segment))
			encoded = strings.ReplaceAll(encoded, "_", "_u_")
			segments[i] = strings.ReplaceAll(encoded, "=", "_e_")
		}
	}
	return strings.Join(segments, "/")
}

// Function to decode the encoded path
func decodePath(encodedPath string) string {
	segments := strings.Split(encodedPath, "/")
	for i, segment := range segments {
		if segment == "" {
			continue // Skip empty segments
		}
		segment = strings.ReplaceAll(segment, "_e_", "=")
		segment = strings.ReplaceAll(segment, "_u_", "_")
		decoded, err := base64.URLEncoding.DecodeString(segment)
		if err == nil && utf8.Valid(decoded) {
			segments[i] = string(decoded)
		}
		// If decoding fails, leave the segment as is
	}
	return strings.Join(segments, "/")
}

// Helper function to check if a string contains only ASCII characters
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
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

// DeleteLocalFile deletes a file from the local file system.
func DeleteLocalFile(localPath string) error {
	err := os.Remove(localPath)
	if err != nil {
		return fmt.Errorf("failed to delete local file %q: %w", localPath, err)
	}
	return nil
}
