//go:build linux || freebsd || netbsd || openbsd

package local

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// trashDir returns the user's freedesktop.org trash directory,
// creating the files/ and info/ subdirectories if necessary.
func trashDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	trash := filepath.Join(dataHome, "Trash")
	for _, sub := range []string{"files", "info"} {
		if err := os.MkdirAll(filepath.Join(trash, sub), 0o700); err != nil {
			return "", err
		}
	}
	return trash, nil
}

// moveToTrash moves the file or directory to the user's trash following
// the freedesktop.org Trash specification.
func moveToTrash(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	trash, err := trashDir()
	if err != nil {
		return fmt.Errorf("failed to use trash: %w", err)
	}
	base := filepath.Base(path)
	// pick a free name in both files/ and info/, then reserve it by
	// creating the .trashinfo file with O_EXCL
	var info *os.File
	var name string
	for i := 1; ; i++ {
		name = base
		if i > 1 {
			ext := filepath.Ext(base)
			name = fmt.Sprintf("%s.%d%s", base[:len(base)-len(ext)], i, ext)
		}
		info, err = os.OpenFile(filepath.Join(trash, "info", name+".trashinfo"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return fmt.Errorf("failed to write trashinfo: %w", err)
		}
	}
	// percent-encode like a URL path, keeping "/" separators
	escaped := (&url.URL{Path: path}).EscapedPath()
	_, err = fmt.Fprintf(info, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", escaped, time.Now().Format("2006-01-02T15:04:05"))
	if closeErr := info.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to write trashinfo: %w", err)
	}
	err = os.Rename(path, filepath.Join(trash, "files", name))
	if err != nil {
		_ = os.Remove(filepath.Join(trash, "info", name+".trashinfo"))
		if errors.Is(err, syscall.EXDEV) {
			return fmt.Errorf("%q is on a different filesystem than the trash directory - can't move it to the trash", path)
		}
		return err
	}
	return nil
}
