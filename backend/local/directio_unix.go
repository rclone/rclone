//go:build linux

package local

import (
	"os"
	"syscall"
)

const directIOSupported = true

func directIOOpenFile(name string, flag int, perm os.FileMode) (file *os.File, err error) {
	return os.OpenFile(name, flag|syscall.O_DIRECT, perm)
}
