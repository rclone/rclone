// +build linux darwin freebsd

package vfsflags

import (
	"github.com/ncw/rclone/fs"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

// add any extra platform specific flags
func platformFlags(flags *pflag.FlagSet) {
	fs.IntVarP(flags, &Opt.Umask, "umask", "", Opt.Umask, "Override the permission bits set by the filesystem.")
	Opt.Umask = unix.Umask(0) // read the umask
	unix.Umask(Opt.Umask)     // set it back to what it was
	Opt.UID = uint32(unix.Geteuid())
	Opt.GID = uint32(unix.Getegid())
	fs.Uint32VarP(flags, &Opt.UID, "uid", "", Opt.UID, "Override the uid field set by the filesystem.")
	fs.Uint32VarP(flags, &Opt.GID, "gid", "", Opt.GID, "Override the gid field set by the filesystem.")
}
