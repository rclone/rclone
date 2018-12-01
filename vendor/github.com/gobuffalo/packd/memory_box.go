package packd

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobuffalo/syncx"
	"github.com/pkg/errors"
)

var _ Addable = NewMemoryBox()
var _ Finder = NewMemoryBox()
var _ Lister = NewMemoryBox()
var _ HTTPBox = NewMemoryBox()
var _ Haser = NewMemoryBox()
var _ Walkable = NewMemoryBox()
var _ Box = NewMemoryBox()

// MemoryBox is a thread-safe, in-memory, implementation of the Box interface.
type MemoryBox struct {
	files *syncx.ByteMap
}

func (m *MemoryBox) Has(path string) bool {
	_, ok := m.files.Load(path)
	return ok
}

func (m *MemoryBox) List() []string {
	var names []string
	m.files.Range(func(key string, value []byte) bool {
		names = append(names, key)
		return true
	})

	sort.Strings(names)
	return names
}

func (m *MemoryBox) Open(path string) (http.File, error) {
	cpath := strings.TrimPrefix(path, "/")

	if filepath.Ext(cpath) == "" {
		// it's a directory
		return NewDir(path)
	}

	if len(cpath) == 0 {
		cpath = "index.html"
	}

	b, err := m.Find(cpath)
	if err != nil {
		return nil, err
	}

	cpath = filepath.FromSlash(cpath)

	f, err := NewFile(cpath, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (m *MemoryBox) FindString(path string) (string, error) {
	bb, err := m.Find(path)
	return string(bb), err
}

func (m *MemoryBox) Find(path string) (ret []byte, e error) {
	res, ok := m.files.Load(path)
	if !ok {

		var b []byte
		lpath := strings.ToLower(path)
		err := m.Walk(func(p string, file File) error {
			lp := strings.ToLower(p)
			if lp != lpath {
				return nil
			}

			res := file.String()
			b = []byte(res)
			m.AddString(lp, res)
			return nil
		})
		if err != nil {
			return b, os.ErrNotExist
		}
		if len(b) == 0 {
			return b, os.ErrNotExist
		}
		return b, nil
	}
	return res, nil
}

func (m *MemoryBox) AddString(path string, t string) error {
	return m.AddBytes(path, []byte(t))
}

func (m *MemoryBox) AddBytes(path string, t []byte) error {
	m.files.Store(path, t)
	return nil
}

func (m *MemoryBox) Walk(wf WalkFunc) error {
	var err error
	m.files.Range(func(path string, b []byte) bool {
		var f File
		f, err = NewFile(path, bytes.NewReader(b))
		if err != nil {
			return false
		}

		err = wf(path, f)
		if err != nil {
			if errors.Cause(err) == filepath.SkipDir {
				err = nil
				return true
			}
			return false
		}

		return true
	})

	if errors.Cause(err) == filepath.SkipDir {
		return nil
	}
	return err
}

func (m *MemoryBox) WalkPrefix(pre string, wf WalkFunc) error {
	return m.Walk(func(path string, file File) error {
		if strings.HasPrefix(path, pre) {
			return wf(path, file)
		}
		return nil
	})
}

func (m *MemoryBox) Remove(path string) {
	m.files.Delete(path)
	m.files.Delete(strings.ToLower(path))
}

// NewMemoryBox returns a configured *MemoryBox
func NewMemoryBox() *MemoryBox {
	return &MemoryBox{
		files: &syncx.ByteMap{},
	}
}
