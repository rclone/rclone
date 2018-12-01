// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goimports

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/imports"
)

type File struct {
	Name string
	In   io.Reader
	Out  io.Writer
}

type Runner struct {
	files []File
}

func New(path ...string) (Runner, error) {
	r := Runner{}
	files, err := buildFiles(path...)
	if err != nil {
		return r, errors.WithStack(err)
	}
	r.files = files
	return r, nil
}

func NewFromFiles(files ...File) Runner {
	return Runner{
		files: files,
	}
}

func (r Runner) Run() error {
	for _, file := range r.files {
		if err := r.processFile(file); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (r Runner) processFile(file File) error {
	var src []byte
	var err error
	if file.In == nil {
		src, err = ioutil.ReadFile(file.Name)
	} else {
		src, err = ioutil.ReadAll(file.In)
	}
	if err != nil {
		return errors.WithStack(err)
	}
	res, err := imports.Process(file.Name, src, nil)
	if err != nil && err != io.EOF {
		return errors.WithStack(err)
	}
	if bytes.Equal(src, res) {
		if s, ok := file.In.(io.Seeker); ok {
			s.Seek(0, 0)
		}
		return nil
	}
	if file.Out == nil {
		if err = ioutil.WriteFile(file.Name, res, 0); err != nil {
			return errors.WithStack(err)
		}
		return nil
	}
	_, err = file.Out.Write(res)
	if c, ok := file.Out.(io.Closer); ok {
		c.Close()
	}
	return err
}

func isGoFile(f os.FileInfo) bool {
	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

func buildFiles(paths ...string) ([]File, error) {
	var files []File
	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, _ error) error {
			if info == nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if !isGoFile(info) {
				return nil
			}
			b, err := ioutil.ReadFile(path)
			if err != nil {
				return errors.WithStack(err)
			}
			files = append(files, File{
				Name: path,
				In:   bytes.NewReader(b),
			})
			return nil
		})
		if err != nil {
			return files, errors.WithStack(err)
		}
	}
	return files, nil
}
