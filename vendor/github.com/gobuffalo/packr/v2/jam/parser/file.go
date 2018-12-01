package parser

import (
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"
)

// File that is to be parsed
type File struct {
	io.Reader
	Path    string
	AbsPath string
}

// Name of the file "app.go"
func (f File) Name() string {
	return f.Path
}

// String returns the contents of the reader
func (f *File) String() string {
	src, _ := ioutil.ReadAll(f)
	f.Reader = bytes.NewReader(src)
	return string(src)
}

func (s *File) Write(p []byte) (int, error) {
	bb := &bytes.Buffer{}
	i, err := bb.Write(p)
	s.Reader = bb
	return i, err
}

// NewFile takes the name of the file you want to
// write to and a reader to reader from
func NewFile(path string, r io.Reader) *File {
	if r == nil {
		r = &bytes.Buffer{}
	}
	if seek, ok := r.(io.Seeker); ok {
		seek.Seek(0, 0)
	}
	abs := path
	if !filepath.IsAbs(path) {
		abs, _ = filepath.Abs(path)
	}
	return &File{
		Reader:  r,
		Path:    path,
		AbsPath: abs,
	}
}
