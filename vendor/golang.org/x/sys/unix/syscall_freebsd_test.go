// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build freebsd

package unix_test

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestSysctlUint64(t *testing.T) {
	_, err := unix.SysctlUint64("security.mac.labeled")
	if err != nil {
		t.Fatal(err)
	}
}
