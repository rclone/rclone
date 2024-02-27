// Package bilib provides common stuff for bisync and bisync_test
// Here it's got local file/directory helpers (nice to have in lib/file)
package bilib

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// PermSecure is a Unix permission for a file accessible only by its owner
const PermSecure = 0600

var (
	regexLocalPath   = regexp.MustCompile(`^[./\\]`)
	regexWindowsPath = regexp.MustCompile(`^[a-zA-Z]:`)
	regexRemotePath  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*:`)
)

// IsLocalPath returns true if its argument is a non-remote path.
// Empty string or a relative path will be considered local.
// Note: `c:dir` will be considered local on Windows but remote on Linux.
func IsLocalPath(path string) bool {
	if path == "" || regexLocalPath.MatchString(path) {
		return true
	}
	if runtime.GOOS == "windows" && regexWindowsPath.MatchString(path) {
		return true
	}
	return !regexRemotePath.MatchString(path)
}

// FileExists returns true if the local file exists
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

// CopyFileIfExists is like CopyFile but does not fail if source does not exist
func CopyFileIfExists(srcFile, dstFile string) error {
	if !FileExists(srcFile) {
		return nil
	}
	return CopyFile(srcFile, dstFile)
}

// CopyFile copies a local file
func CopyFile(src, dst string) (err error) {
	var (
		rd   io.ReadCloser
		wr   io.WriteCloser
		info os.FileInfo
	)
	if info, err = os.Stat(src); err != nil {
		return
	}
	if rd, err = os.Open(src); err != nil {
		return
	}
	defer func() {
		_ = rd.Close()
	}()
	if wr, err = os.Create(dst); err != nil {
		return
	}
	_, err = io.Copy(wr, rd)
	if e := wr.Close(); err == nil {
		err = e
	}
	if e := os.Chmod(dst, info.Mode()); err == nil {
		err = e
	}
	if e := os.Chtimes(dst, info.ModTime(), info.ModTime()); err == nil {
		err = e
	}
	return
}

// CopyDir copies a local directory
func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		return fmt.Errorf("destination already exists")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}

	return
}
