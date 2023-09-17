// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"errors"
	"io"
	"os"
	"strings"
)

// FileInfo represents an file interface
type FileInfo interface {
	os.FileInfo

	Owner() string
	Group() string
}

// Driver is an interface that you will implement to create a driver for your
// chosen persistence layer. The server will create a new instance of your
// driver for each client that connects and delegate to it as required.
//
// Note that if the driver also implements the Auth interface then
// this will be called instead of calling Options.Auth. This allows
// the Auth mechanism to change the driver configuration.
type Driver interface {
	// params  - a file path
	// returns - a time indicating when the requested path was last modified
	//         - an error if the file doesn't exist or the user lacks
	//           permissions
	Stat(*Context, string) (os.FileInfo, error)

	// params  - path, function on file or subdir found
	// returns - error
	//           path
	ListDir(*Context, string, func(os.FileInfo) error) error

	// params  - path
	// returns - nil if the directory was deleted or any error encountered
	DeleteDir(*Context, string) error

	// params  - path
	// returns - nil if the file was deleted or any error encountered
	DeleteFile(*Context, string) error

	// params  - from_path, to_path
	// returns - nil if the file was renamed or any error encountered
	Rename(*Context, string, string) error

	// params  - path
	// returns - nil if the new directory was created or any error encountered
	MakeDir(*Context, string) error

	// params  - path, filepos
	// returns - a string containing the file data to send to the client
	GetFile(*Context, string, int64) (int64, io.ReadCloser, error)

	// params  - destination path, an io.Reader containing the file data
	// returns - the number of bytes written and the first error encountered while writing, if any.
	PutFile(*Context, string, io.Reader, int64) (int64, error)
}

var _ Driver = &MultiDriver{}

// MultiDriver represents a composite driver
type MultiDriver struct {
	drivers map[string]Driver
}

// NewMultiDriver creates a multi driver to combind multiple driver
func NewMultiDriver(drivers map[string]Driver) Driver {
	return &MultiDriver{
		drivers: drivers,
	}
}

// Stat implements Driver
func (driver *MultiDriver) Stat(ctx *Context, path string) (os.FileInfo, error) {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(path, prefix) {
			return driver.Stat(ctx, strings.TrimPrefix(path, prefix))
		}
	}
	return nil, errors.New("Not a file")
}

// ListDir implements Driver
func (driver *MultiDriver) ListDir(ctx *Context, path string, callback func(os.FileInfo) error) error {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(path, prefix) {
			return driver.ListDir(ctx, strings.TrimPrefix(path, prefix), callback)
		}
	}
	return errors.New("Not a directory")
}

// DeleteDir implements Driver
func (driver *MultiDriver) DeleteDir(ctx *Context, path string) error {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(path, prefix) {
			return driver.DeleteDir(ctx, strings.TrimPrefix(path, prefix))
		}
	}
	return errors.New("Not a directory")
}

// DeleteFile implements Driver
func (driver *MultiDriver) DeleteFile(ctx *Context, path string) error {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(path, prefix) {
			return driver.DeleteFile(ctx, strings.TrimPrefix(path, prefix))
		}
	}

	return errors.New("Not a file")
}

// Rename implements Driver
func (driver *MultiDriver) Rename(ctx *Context, fromPath string, toPath string) error {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(fromPath, prefix) {
			return driver.Rename(ctx, strings.TrimPrefix(fromPath, prefix), strings.TrimPrefix(toPath, prefix))
		}
	}

	return errors.New("Not a file")
}

// MakeDir implements Driver
func (driver *MultiDriver) MakeDir(ctx *Context, path string) error {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(path, prefix) {
			return driver.MakeDir(ctx, strings.TrimPrefix(path, prefix))
		}
	}
	return errors.New("Not a directory")
}

// GetFile implements Driver
func (driver *MultiDriver) GetFile(ctx *Context, path string, offset int64) (int64, io.ReadCloser, error) {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(path, prefix) {
			return driver.GetFile(ctx, strings.TrimPrefix(path, prefix), offset)
		}
	}

	return 0, nil, errors.New("Not a file")
}

// PutFile implements Driver
func (driver *MultiDriver) PutFile(ctx *Context, destPath string, data io.Reader, offset int64) (int64, error) {
	for prefix, driver := range driver.drivers {
		if strings.HasPrefix(destPath, prefix) {
			return driver.PutFile(ctx, strings.TrimPrefix(destPath, prefix), data, offset)
		}
	}

	return 0, errors.New("Not a file")
}
