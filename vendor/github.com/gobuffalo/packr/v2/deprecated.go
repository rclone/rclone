package packr

import (
	"encoding/json"
	"errors"

	"github.com/gobuffalo/packr/v2/file"
	"github.com/gobuffalo/packr/v2/file/resolver"
	"github.com/markbates/oncer"
)

// File has been deprecated and file.File should be used instead
type File = file.File

var (
	// ErrResOutsideBox gets returned in case of the requested resources being outside the box
	// Deprecated
	ErrResOutsideBox = errors.New("can't find a resource outside the box")
)

// PackBytes packs bytes for a file into a box.
// Deprecated
func PackBytes(box string, name string, bb []byte) {
	b := NewBox(box)
	d := resolver.NewInMemory(map[string]file.File{})
	f, err := file.NewFile(name, bb)
	if err != nil {
		panic(err)
	}
	if err := d.Pack(name, f); err != nil {
		panic(err)
	}
	b.SetResolver(name, d)
}

// PackBytesGzip packets the gzipped compressed bytes into a box.
// Deprecated
func PackBytesGzip(box string, name string, bb []byte) error {
	// TODO: this function never did what it was supposed to do!
	PackBytes(box, name, bb)
	return nil
}

// PackJSONBytes packs JSON encoded bytes for a file into a box.
// Deprecated
func PackJSONBytes(box string, name string, jbb string) error {
	var bb []byte
	err := json.Unmarshal([]byte(jbb), &bb)
	if err != nil {
		return err
	}
	PackBytes(box, name, bb)
	return nil
}

// Bytes is deprecated. Use Find instead
func (b Box) Bytes(name string) []byte {
	bb, _ := b.Find(name)
	oncer.Deprecate(0, "github.com/gobuffalo/packr/v2#Box.Bytes", "Use github.com/gobuffalo/packr/v2#Box.Find instead.")
	return bb
}

// MustBytes is deprecated. Use Find instead.
func (b Box) MustBytes(name string) ([]byte, error) {
	oncer.Deprecate(0, "github.com/gobuffalo/packr/v2#Box.MustBytes", "Use github.com/gobuffalo/packr/v2#Box.Find instead.")
	return b.Find(name)
}

// String is deprecated. Use FindString instead
func (b Box) String(name string) string {
	oncer.Deprecate(0, "github.com/gobuffalo/packr/v2#Box.String", "Use github.com/gobuffalo/packr/v2#Box.FindString instead.")
	return string(b.Bytes(name))
}

// MustString is deprecated. Use FindString instead
func (b Box) MustString(name string) (string, error) {
	oncer.Deprecate(0, "github.com/gobuffalo/packr/v2#Box.MustString", "Use github.com/gobuffalo/packr/v2#Box.FindString instead.")
	return b.FindString(name)
}
