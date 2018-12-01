package packd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
)

var _ File = &virtualFile{}
var _ io.Reader = &virtualFile{}
var _ io.Writer = &virtualFile{}
var _ fmt.Stringer = &virtualFile{}

type virtualFile struct {
	buf      *bytes.Buffer
	name     string
	info     fileInfo
	original []byte
}

func (f virtualFile) Name() string {
	return f.name
}

func (f *virtualFile) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		f.buf = bytes.NewBuffer(f.original)
		return 0, nil
	}
	return -1, errors.New("Unsuported Seek operation")
}

func (f virtualFile) FileInfo() (os.FileInfo, error) {
	return f.info, nil
}

func (f *virtualFile) Close() error {
	return nil
}

func (f virtualFile) Readdir(count int) ([]os.FileInfo, error) {
	return []os.FileInfo{f.info}, nil
}

func (f virtualFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

func (f virtualFile) String() string {
	return string(f.original)
}

func (s *virtualFile) Read(p []byte) (int, error) {
	i, err := s.buf.Read(p)

	if i == 0 || err == io.EOF {
		s.buf = bytes.NewBuffer(s.original)
	}
	return i, err
}

func (s *virtualFile) Write(p []byte) (int, error) {
	bb := &bytes.Buffer{}
	i, err := bb.Write(p)
	if err != nil {
		return i, errors.WithStack(err)
	}
	s.buf = bb
	s.original = bb.Bytes()
	s.info = fileInfo{
		Path:     s.name,
		Contents: bb.Bytes(),
		size:     int64(bb.Len()),
		modTime:  time.Now(),
	}
	return i, nil
}

// NewDir returns a new "virtual" file
func NewFile(name string, r io.Reader) (File, error) {
	bb := &bytes.Buffer{}
	if r != nil {
		io.Copy(bb, r)
	}
	return &virtualFile{
		buf:      bb,
		name:     name,
		original: bb.Bytes(),
		info: fileInfo{
			Path:     name,
			Contents: bb.Bytes(),
			size:     int64(bb.Len()),
			modTime:  time.Now(),
		},
	}, nil
}

// NewDir returns a new "virtual" directory
func NewDir(name string) (File, error) {
	bb := &bytes.Buffer{}
	return &virtualFile{
		buf:  bb,
		name: name,
		info: fileInfo{
			Path:     name,
			Contents: bb.Bytes(),
			size:     int64(bb.Len()),
			modTime:  time.Now(),
			isDir:    true,
		},
	}, nil
}
