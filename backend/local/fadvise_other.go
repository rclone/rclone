//+build !linux

package local

import (
	"io"
	"os"
)

func newFadviseReadCloser(o *Object, f *os.File, offset, limit int64) io.ReadCloser {
	return f
}
