package file

import (
	"bytes"
	"io"

	"github.com/gobuffalo/packd"
)

// File represents a virtual, or physical, backing of
// a file object in a Box
type File = packd.File

// FileMappable types are capable of returning a map of
// path => File
type FileMappable interface {
	FileMap() map[string]File
}

// NewFile returns a virtual File implementation
func NewFile(name string, b []byte) (File, error) {
	return packd.NewFile(name, bytes.NewReader(b))
}

// NewDir returns a virtual dir implementation
func NewDir(name string) (File, error) {
	return packd.NewDir(name)
}

func NewFileR(name string, r io.Reader) (File, error) {
	return packd.NewFile(name, r)
}
