// Copyright 2010 The Go Authors. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE.golang
// file.
//
// This is a copied and modified version of the code provided in:
// https://golang.org/src/io/ioutil/tempfile.go

package tmpfile

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// Random number state.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.
var rand uint32
var randmu sync.Mutex

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextRandom() string {
	randmu.Lock()
	r := rand
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	rand = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

// New creates a new temporary file in the directory dir using the same method
// as ioutil.TempFile and then unlinks the file with os.Remove to ensure the
// file is deleted when the calling process exists.
func New(dir, pattern string) (f *os.File, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	var prefix, suffix string
	if pos := strings.LastIndex(pattern, "*"); pos != -1 {
		prefix, suffix = pattern[:pos], pattern[pos+1:]
	} else {
		prefix = pattern
	}

	nconflict := 0
	for i := 0; i < 10000; i++ {
		name := filepath.Join(dir, prefix+nextRandom()+suffix)

		// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea
		handle, err := windows.CreateFile(
			windows.StringToUTF16Ptr(name),                            // File Name
			windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE, // Desired Access
			windows.FILE_SHARE_DELETE,                                 // Share Mode
			nil,                                                       // Security Attributes
			windows.CREATE_NEW,                                        // Create Disposition
			windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, // Flags & Attributes
			0, // Template File
		)
		if os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				randmu.Lock()
				rand = reseed()
				randmu.Unlock()
			}

			continue
		}

		f = os.NewFile(uintptr(handle), name)

		break
	}

	err = os.Remove(f.Name())
	if err != nil {
		return
	}

	return f, nil
}
