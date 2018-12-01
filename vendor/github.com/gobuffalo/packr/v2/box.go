package packr

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2/file"
	"github.com/gobuffalo/packr/v2/file/resolver"
	"github.com/gobuffalo/packr/v2/plog"
	"github.com/markbates/oncer"
	"github.com/pkg/errors"
)

var _ packd.Box = &Box{}
var _ packd.HTTPBox = Box{}
var _ packd.Addable = &Box{}
var _ packd.Walkable = &Box{}
var _ packd.Finder = Box{}

// NewBox returns a Box that can be used to
// retrieve files from either disk or the embedded
// binary.
func NewBox(path string) *Box {
	oncer.Deprecate(0, "packr.NewBox", "Use packr.New instead.")
	return New(path, path)
}

func resolutionDir(og string) string {
	ng, _ := filepath.Abs(og)

	exists := func(s string) bool {
		_, err := os.Stat(s)
		if err != nil {
			return false
		}
		plog.Debug("packr", "resolutionDir", "original", og, "resolved", s)
		return true
	}

	if exists(ng) {
		return ng
	}

	_, filename, _, _ := runtime.Caller(2)

	ng = filepath.Join(filepath.Dir(filename), og)

	// // this little hack courtesy of the `-cover` flag!!
	cov := filepath.Join("_test", "_obj_test")
	ng = strings.Replace(ng, string(filepath.Separator)+cov, "", 1)

	if exists(ng) {
		return ng
	}

	ng = filepath.Join(envy.GoPath(), "src", ng)
	if exists(ng) {
		return ng
	}

	return og
}

func construct(name string, path string) *Box {
	return &Box{
		Path:          path,
		Name:          name,
		ResolutionDir: resolutionDir(path),
		resolvers:     map[string]resolver.Resolver{},
		moot:          &sync.RWMutex{},
	}
}

func New(name string, path string) *Box {
	plog.Debug("packr", "New", "name", name, "path", path)
	b, _ := findBox(name)
	if b != nil {
		return b
	}

	b = construct(name, path)
	plog.Debug(b, "New", "Box", b, "ResolutionDir", b.ResolutionDir)
	b, err := placeBox(b)
	if err != nil {
		panic(err)
	}

	return b
}

// Box represent a folder on a disk you want to
// have access to in the built Go binary.
type Box struct {
	Path            string            `json:"path"`
	Name            string            `json:"name"`
	ResolutionDir   string            `json:"resolution_dir"`
	DefaultResolver resolver.Resolver `json:"default_resolver"`
	resolvers       map[string]resolver.Resolver
	moot            *sync.RWMutex
}

func (b *Box) SetResolver(file string, res resolver.Resolver) {
	b.moot.Lock()
	plog.Debug(b, "SetResolver", "file", file, "resolver", fmt.Sprintf("%T", res))
	b.resolvers[resolver.Key(file)] = res
	b.moot.Unlock()
}

// AddString converts t to a byteslice and delegates to AddBytes to add to b.data
func (b *Box) AddString(path string, t string) error {
	return b.AddBytes(path, []byte(t))
}

// AddBytes sets t in b.data by the given path
func (b *Box) AddBytes(path string, t []byte) error {
	m := map[string]file.File{}
	f, err := file.NewFile(path, t)
	if err != nil {
		return errors.WithStack(err)
	}
	m[resolver.Key(path)] = f
	res := resolver.NewInMemory(m)
	b.SetResolver(path, res)
	return nil
}

// FindString returns either the string of the requested
// file or an error if it can not be found.
func (b Box) FindString(name string) (string, error) {
	bb, err := b.Find(name)
	return string(bb), err
}

// Find returns either the byte slice of the requested
// file or an error if it can not be found.
func (b Box) Find(name string) ([]byte, error) {
	f, err := b.Resolve(name)
	if err != nil {
		return []byte(""), err
	}
	bb := &bytes.Buffer{}
	io.Copy(bb, f)
	return bb.Bytes(), nil
}

// Has returns true if the resource exists in the box
func (b Box) Has(name string) bool {
	_, err := b.Find(name)
	if err != nil {
		return false
	}
	return true
}

// Open returns a File using the http.File interface
func (b Box) Open(name string) (http.File, error) {
	plog.Debug(b, "Open", "name", name)
	if len(filepath.Ext(name)) == 0 {
		d, err := file.NewDir(name)
		plog.Debug(b, "Open", "name", name, "dir", d)
		return d, err
	}
	f, err := b.Resolve(name)
	if err != nil {
		return f, err
	}
	f, err = file.NewFileR(name, f)
	plog.Debug(b, "Open", "name", f.Name(), "file", f.Name())
	return f, err
}

// List shows "What's in the box?"
func (b Box) List() []string {
	var keys []string

	b.Walk(func(path string, info File) error {
		if info == nil {
			return nil
		}
		finfo, _ := info.FileInfo()
		if !finfo.IsDir() {
			keys = append(keys, path)
		}
		return nil
	})
	sort.Strings(keys)
	return keys
}

func (b *Box) Resolve(key string) (file.File, error) {
	key = strings.TrimPrefix(key, "/")
	b.moot.RLock()
	r, ok := b.resolvers[resolver.Key(key)]
	b.moot.RUnlock()
	if !ok {
		r = b.DefaultResolver
		if r == nil {
			r = resolver.DefaultResolver
			if r == nil {
				return nil, errors.New("resolver.DefaultResolver is nil")
			}
		}
	}
	plog.Debug(r, "Resolve", "box", b.Name, "key", key)

	f, err := r.Resolve(b.Name, key)
	if err != nil {
		z := filepath.Join(resolver.OsPath(b.ResolutionDir), resolver.OsPath(key))
		f, err = r.Resolve(b.Name, z)
		if err != nil {
			plog.Debug(r, "Resolve", "box", b.Name, "key", z, "err", err)
			return f, err
		}
		b, err := ioutil.ReadAll(f)
		if err != nil {
			return f, errors.WithStack(err)
		}
		f, err = file.NewFile(key, b)
		if err != nil {
			return f, errors.WithStack(err)
		}
	}
	plog.Debug(r, "Resolve", "box", b.Name, "key", key, "file", f.Name())
	return f, nil
}
