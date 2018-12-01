package resolver

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gobuffalo/packr/v2/file"
	"github.com/gobuffalo/packr/v2/plog"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
)

var _ Resolver = &Disk{}

type Disk struct {
	Root string
}

func (d Disk) String() string {
	return String(&d)
}

func (d *Disk) Resolve(box string, name string) (file.File, error) {
	path := OsPath(name)
	if !filepath.IsAbs(path) {
		path = filepath.Join(OsPath(d.Root), path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return file.NewDir(OsPath(name))
	}
	if bb, err := ioutil.ReadFile(path); err == nil {
		return file.NewFile(OsPath(name), bb)
	}
	return nil, os.ErrNotExist
}

var _ file.FileMappable = &Disk{}

func (d *Disk) FileMap() map[string]file.File {
	moot := &sync.Mutex{}
	m := map[string]file.File{}
	root := OsPath(d.Root)
	if _, err := os.Stat(root); err != nil {
		return m
	}
	callback := func(path string, de *godirwalk.Dirent) error {
		if _, err := os.Stat(root); err != nil {
			return nil
		}
		if !de.IsRegular() {
			return nil
		}
		moot.Lock()
		name := strings.TrimPrefix(path, root+string(filepath.Separator))
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.WithStack(err)
		}
		m[name], err = file.NewFile(name, b)
		if err != nil {
			return errors.WithStack(err)
		}
		moot.Unlock()
		return nil
	}
	err := godirwalk.Walk(root, &godirwalk.Options{
		FollowSymbolicLinks: true,
		Callback:            callback,
	})
	if err != nil {
		plog.Logger.Errorf("[%s] error walking %v", root, err)
	}
	return m
}
