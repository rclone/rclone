package packd

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

type WalkFunc func(string, File) error

// Box represents the entirety of the necessary
// interfaces to form a "full" box.
// github.com/gobuffalo/packr#Box is an example of this interface.
type Box interface {
	HTTPBox
	Lister
	Addable
	Finder
	Walkable
	Haser
}

type Haser interface {
	Has(string) bool
}

type Walker interface {
	Walk(wf WalkFunc) error
}

type Walkable interface {
	Walker
	WalkPrefix(prefix string, wf WalkFunc) error
}

type Finder interface {
	Find(string) ([]byte, error)
	FindString(name string) (string, error)
}

type HTTPBox interface {
	Open(name string) (http.File, error)
}

type Lister interface {
	List() []string
}

type Addable interface {
	AddString(path string, t string) error
	AddBytes(path string, t []byte) error
}

type SimpleFile interface {
	fmt.Stringer
	io.Reader
	io.Writer
	Name() string
}

type HTTPFile interface {
	SimpleFile
	io.Closer
	io.Seeker
	Readdir(count int) ([]os.FileInfo, error)
	Stat() (os.FileInfo, error)
}

type File interface {
	HTTPFile
	FileInfo() (os.FileInfo, error)
}

// LegacyBox represents deprecated methods
// that older Box implementations might have had.
// github.com/gobuffalo/packr v1 is an example of a LegacyBox.
type LegacyBox interface {
	String(name string) string
	MustString(name string) (string, error)
	Bytes(name string) []byte
	MustBytes(name string) ([]byte, error)
}
