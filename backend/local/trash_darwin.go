//go:build darwin

package local

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// moveToTrash moves the file or directory into the user's ~/.Trash.
func moveToTrash(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	trash := filepath.Join(home, ".Trash")
	if fi, err := os.Stat(trash); err != nil || !fi.IsDir() {
		return fmt.Errorf("trash directory %q not usable: %v", trash, err)
	}
	base := filepath.Base(path)
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			ext := filepath.Ext(base)
			name = fmt.Sprintf("%s %d%s", base[:len(base)-len(ext)], i, ext)
		}
		target := filepath.Join(trash, name)
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		err = os.Rename(path, target)
		if err != nil && errors.Is(err, syscall.EXDEV) {
			return fmt.Errorf("%q is on a different filesystem than the trash directory - can't move it to the trash", path)
		}
		return err
	}
}
