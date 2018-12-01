package genny

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gobuffalo/packd"
	"github.com/pkg/errors"
)

// ForceBox will mount each file in the box and wrap it with ForceFile
func ForceBox(g *Generator, box packd.Walker, force bool) error {
	return box.Walk(func(path string, bf packd.File) error {
		f := NewFile(path, bf)
		ff := ForceFile(f, force)
		f, err := ff(f)
		if err != nil {
			return errors.WithStack(err)
		}
		g.File(f)
		return nil
	})
}

// ForceFile is a TransformerFn that will return an error if the path exists if `force` is false. If `force` is true it will delete the path.
func ForceFile(f File, force bool) TransformerFn {
	return func(f File) (File, error) {
		path := f.Name()
		path, err := filepath.Abs(path)
		if err != nil {
			return f, errors.WithStack(err)
		}
		_, err = os.Stat(path)
		if err != nil {
			// path doesn't exist. move on.
			return f, nil
		}
		if !force {
			return f, errors.Errorf("path %s already exists", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return f, errors.WithStack(err)
		}
		return f, nil
	}
}

// Force is a RunFn that will return an error if the path exists if `force` is false. If `force` is true it will delete the path.
// Is is recommended to use ForceFile when you can.
func Force(path string, force bool) RunFn {
	if path == "." || path == "" {
		pwd, _ := os.Getwd()
		path = pwd
	}
	return func(r *Runner) error {
		path, err := filepath.Abs(path)
		if err != nil {
			return errors.WithStack(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			// path doesn't exist. move on.
			return nil
		}
		if !force {
			if !fi.IsDir() {
				return errors.Errorf("path %s already exists", path)
			}
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return errors.WithStack(err)
			}
			if len(files) > 0 {
				return errors.Errorf("path %s already exists", path)
			}
			return nil
		}
		if err := os.RemoveAll(path); err != nil {
			return errors.WithStack(err)
		}
		return nil
	}
}
