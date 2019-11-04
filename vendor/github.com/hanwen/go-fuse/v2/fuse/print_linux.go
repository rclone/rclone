// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"fmt"
	"syscall"
)

func init() {
	openFlagNames[syscall.O_DIRECT] = "DIRECT"
	openFlagNames[syscall.O_LARGEFILE] = "LARGEFILE"
	openFlagNames[syscall_O_NOATIME] = "NOATIME"
}

func (a *Attr) string() string {
	return fmt.Sprintf(
		"{M0%o SZ=%d L=%d "+
			"%d:%d "+
			"B%d*%d i%d:%d "+
			"A %f "+
			"M %f "+
			"C %f}",
		a.Mode, a.Size, a.Nlink,
		a.Uid, a.Gid,
		a.Blocks, a.Blksize,
		a.Rdev, a.Ino, ft(a.Atime, a.Atimensec), ft(a.Mtime, a.Mtimensec),
		ft(a.Ctime, a.Ctimensec))
}

func (in *CreateIn) string() string {
	return fmt.Sprintf(
		"{0%o [%s] (0%o)}", in.Mode,
		flagString(openFlagNames, int64(in.Flags), "O_RDONLY"), in.Umask)
}

func (in *GetAttrIn) string() string {
	return fmt.Sprintf("{Fh %d}", in.Fh_)
}

func (in *MknodIn) string() string {
	return fmt.Sprintf("{0%o (0%o), %d}", in.Mode, in.Umask, in.Rdev)
}

func (in *ReadIn) string() string {
	return fmt.Sprintf("{Fh %d [%d +%d) %s L %d %s}",
		in.Fh, in.Offset, in.Size,
		flagString(readFlagNames, int64(in.ReadFlags), ""),
		in.LockOwner,
		flagString(openFlagNames, int64(in.Flags), "RDONLY"))
}

func (in *WriteIn) string() string {
	return fmt.Sprintf("{Fh %d [%d +%d) %s L %d %s}",
		in.Fh, in.Offset, in.Size,
		flagString(writeFlagNames, int64(in.WriteFlags), ""),
		in.LockOwner,
		flagString(openFlagNames, int64(in.Flags), "RDONLY"))
}
