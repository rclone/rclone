// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// http://golang.org/src/os/stat_freebsd.go

package times

import (
	"os"
	"syscall"
	"time"
)

// HasChangeTime and HasBirthTime are true if and only if
// the target OS supports them.
const (
	HasChangeTime = true
	HasBirthTime  = true
)

type timespec struct {
	atime
	mtime
	ctime
	btime
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}

func getTimespec(fi os.FileInfo) (t timespec) {
	stat := fi.Sys().(*syscall.Stat_t)
	t.atime.v = timespecToTime(stat.Atimespec)
	t.mtime.v = timespecToTime(stat.Mtimespec)
	t.ctime.v = timespecToTime(stat.Ctimespec)
	t.btime.v = timespecToTime(stat.Birthtimespec)
	return t
}
