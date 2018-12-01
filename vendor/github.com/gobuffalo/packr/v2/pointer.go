package packr

import (
	"github.com/gobuffalo/packr/v2/file"
	"github.com/gobuffalo/packr/v2/file/resolver"
	"github.com/gobuffalo/packr/v2/plog"
	"github.com/pkg/errors"
)

type Pointer struct {
	ForwardBox  string
	ForwardPath string
}

var _ resolver.Resolver = Pointer{}

func (p Pointer) Resolve(box string, path string) (file.File, error) {
	plog.Debug(p, "Resolve", "box", box, "path", path, "forward-box", p.ForwardBox, "forward-path", p.ForwardPath)
	b, err := findBox(p.ForwardBox)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	f, err := b.Resolve(p.ForwardPath)
	if err != nil {
		return f, errors.WithStack(errors.Wrap(err, path))
	}
	plog.Debug(p, "Resolve", "box", box, "path", path, "file", f)
	return f, nil
}
