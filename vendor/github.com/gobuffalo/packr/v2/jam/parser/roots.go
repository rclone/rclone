package parser

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gobuffalo/packr/v2/plog"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
)

type RootsOptions struct {
	IgnoreImports bool
	Ignores       []string
}

func (r RootsOptions) String() string {
	x, _ := json.Marshal(r)
	return string(x)
}

// NewFromRoots scans the file roots provided and returns a
// new Parser containing the prospects
func NewFromRoots(roots []string, opts *RootsOptions) (*Parser, error) {
	if opts == nil {
		opts = &RootsOptions{}
	}

	if len(roots) == 0 {
		pwd, _ := os.Getwd()
		roots = append(roots, pwd)
	}
	p := New()
	plog.Debug(p, "NewFromRoots", "roots", roots, "options", opts)
	callback := func(path string, de *godirwalk.Dirent) error {
		if IsProspect(path, opts.Ignores...) {
			if de.IsDir() {
				return nil
			}
			roots = append(roots, path)
			return nil
		}
		if de.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	wopts := &godirwalk.Options{
		FollowSymbolicLinks: true,
		Callback:            callback,
	}
	for _, root := range roots {
		plog.Debug(p, "NewFromRoots", "walking", root)
		err := godirwalk.Walk(root, wopts)
		if err != nil {
			return p, errors.WithStack(err)
		}
	}

	dd := map[string]string{}
	fd := &finder{id: time.Now()}
	for _, r := range roots {
		var names []string
		if opts.IgnoreImports {
			names, _ = fd.findAllGoFiles(r)
		} else {
			names, _ = fd.findAllGoFilesImports(r)
		}
		for _, n := range names {
			if IsProspect(n) {
				plog.Debug(p, "NewFromRoots", "mapping", n)
				dd[n] = n
			}
		}
	}
	for path := range dd {
		plog.Debug(p, "NewFromRoots", "reading file", path)
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		p.Prospects = append(p.Prospects, NewFile(path, bytes.NewReader(b)))
	}
	plog.Debug(p, "NewFromRoots", "found prospects", len(p.Prospects))
	return p, nil
}
