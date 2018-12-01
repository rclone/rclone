package genny

import (
	"bytes"
	"io"
	"runtime"
	"strings"

	"github.com/gobuffalo/packd"
)

// File interface for working with files
type File = packd.SimpleFile

// NewFile takes the name of the file you want to
// write to and a reader to reader from
func NewFile(name string, r io.Reader) File {
	osname := name
	if runtime.GOOS == "windows" {
		osname = strings.Replace(osname, "\\", "/", -1)
	}
	f, _ := packd.NewFile(osname, r)
	return f
}

func NewFileS(name string, s string) File {
	return NewFile(name, strings.NewReader(s))
}

func NewFileB(name string, s []byte) File {
	return NewFile(name, bytes.NewReader(s))
}
