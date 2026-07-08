//go:build linux

package local

import (
	"io"
	"os"
	"syscall"

	"github.com/rclone/rclone/fs"
)

const directIOAlignSize = 512

// directIOCopy copies from src to dst using O_DIRECT-compatible aligned writes.
// The buf must be aligned. For the final partial block (not a multiple of
// directIOAlignSize), O_DIRECT is dropped from the fd via fcntl and the
// remainder is written normally.
func directIOCopy(dst *os.File, src io.Reader, buf []byte) (written int64, err error) {
	droppedDirect := false
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			toWrite := buf[0:nr]
			if !droppedDirect && nr%directIOAlignSize != 0 {
				// Final partial block: drop O_DIRECT for this write
				if err := dropDirectIO(dst); err != nil {
					fs.Debugf(nil, "Failed to drop O_DIRECT for final write: %v", err)
				}
				droppedDirect = true
			}
			nw, ew := dst.Write(toWrite)
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

// dropDirectIO removes the O_DIRECT flag from an open file descriptor.
func dropDirectIO(f *os.File) error {
	fd := f.Fd()
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 {
		return errno
	}
	_, _, errno = syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, flags&^syscall.O_DIRECT)
	if errno != 0 {
		return errno
	}
	return nil
}
