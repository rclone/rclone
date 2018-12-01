package genny

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gobuffalo/packd"
	"github.com/pkg/errors"
)

// Disk is a virtual file system that works
// with both dry and wet runners. Perfect for seeding
// Files or non-destructively deleting files
type Disk struct {
	Runner *Runner
	files  map[string]File
	moot   *sync.RWMutex
}

func (d *Disk) AddBox(box packd.Walker) error {
	return box.Walk(func(path string, file packd.File) error {
		d.Add(NewFile(path, file))
		return nil
	})
}

// Files returns a sorted list of all the files in the disk
func (d *Disk) Files() []File {
	var files []File
	for _, f := range d.files {
		if s, ok := f.(io.Seeker); ok {
			s.Seek(0, 0)
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
	return files
}

func newDisk(r *Runner) *Disk {
	return &Disk{
		Runner: r,
		files:  map[string]File{},
		moot:   &sync.RWMutex{},
	}
}

// Remove a file(s) from the virtual disk.
func (d *Disk) Remove(name string) {
	d.moot.Lock()
	defer d.moot.Unlock()
	for f, _ := range d.files {
		if strings.HasPrefix(f, name) {
			delete(d.files, f)
		}
	}
}

// Delete calls the Runner#Delete function
func (d *Disk) Delete(name string) error {
	return d.Runner.Delete(name)
}

// Add file to the virtual disk
func (d *Disk) Add(f File) {
	d.moot.Lock()
	defer d.moot.Unlock()
	d.files[f.Name()] = f
}

// Find a file from the virtual disk. If the file doesn't
// exist it will try to read the file from the physical disk.
func (d *Disk) Find(name string) (File, error) {

	d.moot.RLock()
	if f, ok := d.files[name]; ok {
		if seek, ok := f.(io.Seeker); ok {
			seek.Seek(0, 0)
		}
		d.moot.RUnlock()
		return f, nil
	}
	d.moot.RUnlock()

	gf := NewFile(name, bytes.NewReader([]byte("")))

	osname := name
	if runtime.GOOS == "windows" {
		osname = strings.Replace(osname, "/", "\\", -1)
	}
	f, err := os.Open(osname)
	if err != nil {
		return gf, errors.WithStack(err)
	}
	defer f.Close()

	bb := &bytes.Buffer{}

	if _, err := io.Copy(bb, f); err != nil {
		return gf, errors.WithStack(err)
	}
	gf = NewFile(name, bb)
	d.Add(gf)
	return gf, nil
}
