package resolver

import (
	"io/ioutil"

	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2/file"
	"github.com/gobuffalo/packr/v2/plog"
	"github.com/pkg/errors"
)

var _ Resolver = &InMemory{}

type InMemory struct {
	*packd.MemoryBox
}

func (d InMemory) String() string {
	return String(&d)
}

func (d *InMemory) Resolve(box string, name string) (file.File, error) {
	b, err := d.MemoryBox.Find(name)
	if err != nil {
		return nil, err
	}
	return file.NewFile(name, b)
}

func (d *InMemory) Pack(name string, f file.File) error {
	plog.Debug(d, "Pack", "name", name)
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.WithStack(err)
	}
	d.AddBytes(name, b)
	return nil
}

func (d *InMemory) FileMap() map[string]file.File {
	m := map[string]file.File{}
	d.Walk(func(path string, file file.File) error {
		m[path] = file
		return nil
	})
	return m
}

func NewInMemory(files map[string]file.File) *InMemory {
	if files == nil {
		files = map[string]file.File{}
	}
	box := packd.NewMemoryBox()

	for p, f := range files {
		if b, err := ioutil.ReadAll(f); err == nil {
			box.AddBytes(p, b)
		}
	}

	return &InMemory{
		MemoryBox: box,
	}
}
