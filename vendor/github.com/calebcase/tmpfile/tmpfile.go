// +build !windows

package tmpfile

import (
	"io/ioutil"
	"os"
)

// New creates a new temporary file in the directory dir using ioutil.TempFile
// and then unlinks the file with os.Remove to ensure the file is deleted when
// the calling process exists.
func New(dir, pattern string) (f *os.File, err error) {
	f, err = ioutil.TempFile(dir, pattern)
	if err != nil {
		return
	}

	err = os.Remove(f.Name())
	if err != nil {
		return
	}

	return
}
