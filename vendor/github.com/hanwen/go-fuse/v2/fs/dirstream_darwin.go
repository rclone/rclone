// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"io"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
)

func NewLoopbackDirStream(nm string) (DirStream, syscall.Errno) {
	f, err := os.Open(nm)
	if err != nil {
		return nil, ToErrno(err)
	}
	defer f.Close()

	var entries []fuse.DirEntry
	for {
		want := 100
		infos, err := f.Readdir(want)
		for _, info := range infos {
			s := fuse.ToStatT(info)
			if s == nil {
				continue
			}

			entries = append(entries, fuse.DirEntry{
				Name: info.Name(),
				Mode: uint32(s.Mode),
				Ino:  s.Ino,
			})
		}
		if len(infos) < want || err == io.EOF {
			break
		}

		if err != nil {
			return nil, ToErrno(err)
		}
	}

	return &dirArray{entries}, OK
}
