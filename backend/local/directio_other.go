//go:build !linux

package local

import (
	"os"
)

const directIOSupported = false

func directIOOpenFile(name string, flag int, perm os.FileMode) (file *os.File, err error) {
	panic("no implementation")
}
