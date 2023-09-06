// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import "os"

// FileInfo represents an file interface
type FileInfo interface {
	os.FileInfo

	Owner() string
	Group() string
}
