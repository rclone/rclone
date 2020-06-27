// Copyright 2020 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileDriver implements Driver directly read local file system
type FileDriver struct {
	RootPath string
	Perm
}

func (driver *FileDriver) realPath(path string) string {
	paths := strings.Split(path, "/")
	return filepath.Join(append([]string{driver.RootPath}, paths...)...)
}

// Stat implements Driver
func (driver *FileDriver) Stat(path string) (FileInfo, error) {
	basepath := driver.realPath(path)
	rPath, err := filepath.Abs(basepath)
	if err != nil {
		return nil, err
	}
	f, err := os.Lstat(rPath)
	if err != nil {
		return nil, err
	}
	mode, err := driver.Perm.GetMode(path)
	if err != nil {
		return nil, err
	}
	if f.IsDir() {
		mode |= os.ModeDir
	}
	owner, err := driver.Perm.GetOwner(path)
	if err != nil {
		return nil, err
	}
	group, err := driver.Perm.GetGroup(path)
	if err != nil {
		return nil, err
	}
	return &fileInfo{f, mode, owner, group}, nil
}

// ListDir implements Driver
func (driver *FileDriver) ListDir(path string, callback func(FileInfo) error) error {
	basepath := driver.realPath(path)
	return filepath.Walk(basepath, func(f string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rPath, _ := filepath.Rel(basepath, f)
		if rPath == info.Name() {
			mode, err := driver.Perm.GetMode(rPath)
			if err != nil {
				return err
			}
			if info.IsDir() {
				mode |= os.ModeDir
			}
			owner, err := driver.Perm.GetOwner(rPath)
			if err != nil {
				return err
			}
			group, err := driver.Perm.GetGroup(rPath)
			if err != nil {
				return err
			}
			err = callback(&fileInfo{info, mode, owner, group})
			if err != nil {
				return err
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
}

// DeleteDir implements Driver
func (driver *FileDriver) DeleteDir(path string) error {
	rPath := driver.realPath(path)
	f, err := os.Lstat(rPath)
	if err != nil {
		return err
	}
	if f.IsDir() {
		return os.RemoveAll(rPath)
	}
	return errors.New("Not a directory")
}

// DeleteFile implements Driver
func (driver *FileDriver) DeleteFile(path string) error {
	rPath := driver.realPath(path)
	f, err := os.Lstat(rPath)
	if err != nil {
		return err
	}
	if !f.IsDir() {
		return os.Remove(rPath)
	}
	return errors.New("Not a file")
}

// Rename implements Driver
func (driver *FileDriver) Rename(fromPath string, toPath string) error {
	oldPath := driver.realPath(fromPath)
	newPath := driver.realPath(toPath)
	return os.Rename(oldPath, newPath)
}

// MakeDir implements Driver
func (driver *FileDriver) MakeDir(path string) error {
	rPath := driver.realPath(path)
	return os.MkdirAll(rPath, os.ModePerm)
}

// GetFile implements Driver
func (driver *FileDriver) GetFile(path string, offset int64) (int64, io.ReadCloser, error) {
	rPath := driver.realPath(path)
	f, err := os.Open(rPath)
	if err != nil {
		return 0, nil, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, nil, err
	}

	f.Seek(offset, io.SeekStart)

	return info.Size() - offset, f, nil
}

// PutFile implements Driver
func (driver *FileDriver) PutFile(destPath string, data io.Reader, appendData bool) (int64, error) {
	rPath := driver.realPath(destPath)
	var isExist bool
	f, err := os.Lstat(rPath)
	if err == nil {
		isExist = true
		if f.IsDir() {
			return 0, errors.New("A dir has the same name")
		}
	} else {
		if os.IsNotExist(err) {
			isExist = false
		} else {
			return 0, errors.New(fmt.Sprintln("Put File error:", err))
		}
	}

	if appendData && !isExist {
		appendData = false
	}

	if !appendData {
		if isExist {
			err = os.Remove(rPath)
			if err != nil {
				return 0, err
			}
		}
		f, err := os.Create(rPath)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		bytes, err := io.Copy(f, data)
		if err != nil {
			return 0, err
		}
		return bytes, nil
	}

	of, err := os.OpenFile(rPath, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		return 0, err
	}
	defer of.Close()

	_, err = of.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, err
	}

	bytes, err := io.Copy(of, data)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

// FileDriverFactory implements DriverFactory
type FileDriverFactory struct {
	RootPath string
	Perm
}

// NewDriver implements DriverFactory
func (factory *FileDriverFactory) NewDriver() (Driver, error) {
	var err error
	factory.RootPath, err = filepath.Abs(factory.RootPath)
	if err != nil {
		return nil, err
	}
	return &FileDriver{factory.RootPath, factory.Perm}, nil
}
