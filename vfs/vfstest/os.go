package vfstest

import (
	"os"
	"time"

	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/vfs"
)

// Oser defines the things that the "os" package can do
//
// This covers what the VFS can do also
type Oser interface {
	Chtimes(name string, atime time.Time, mtime time.Time) error
	Create(name string) (vfs.Handle, error)
	Mkdir(name string, perm os.FileMode) error
	Open(name string) (vfs.Handle, error)
	OpenFile(name string, flags int, perm os.FileMode) (fd vfs.Handle, err error)
	ReadDir(dirname string) ([]os.FileInfo, error)
	ReadFile(filename string) (b []byte, err error)
	Remove(name string) error
	Rename(oldName, newName string) error
	Stat(path string) (os.FileInfo, error)
}

// realOs is an implementation of Oser backed by the "os" package
type realOs struct {
}

// realOsFile is an implementation of vfs.Handle
type realOsFile struct {
	*os.File
}

// Flush
func (f realOsFile) Flush() error {
	return nil
}

// Release
func (f realOsFile) Release() error {
	return f.File.Close()
}

// Node
func (f realOsFile) Node() vfs.Node {
	return nil
}

func (f realOsFile) Lock() error {
	return os.ErrInvalid
}

func (f realOsFile) Unlock() error {
	return os.ErrInvalid
}

// Chtimes
func (r realOs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

// Create
func (r realOs) Create(name string) (vfs.Handle, error) {
	fd, err := file.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	return realOsFile{File: fd}, err
}

// Mkdir
func (r realOs) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

// Open
func (r realOs) Open(name string) (vfs.Handle, error) {
	fd, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return realOsFile{File: fd}, err
}

// OpenFile
func (r realOs) OpenFile(name string, flags int, perm os.FileMode) (vfs.Handle, error) {
	fd, err := file.OpenFile(name, flags, perm)
	if err != nil {
		return nil, err
	}
	return realOsFile{File: fd}, err
}

// ReadDir
func (r realOs) ReadDir(dirname string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// ReadFile
func (r realOs) ReadFile(filename string) (b []byte, err error) {
	return os.ReadFile(filename)
}

// Remove
func (r realOs) Remove(name string) error {
	return os.Remove(name)
}

// Rename
func (r realOs) Rename(oldName, newName string) error {
	return os.Rename(oldName, newName)
}

// Stat
func (r realOs) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Check interfaces
var _ Oser = &realOs{}
var _ vfs.Handle = &realOsFile{}
