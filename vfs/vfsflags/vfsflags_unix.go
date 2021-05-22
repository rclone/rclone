// +build linux darwin freebsd

package vfsflags

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

// add any extra platform specific flags
func platformFlags(flagSet *pflag.FlagSet) {
	Opt.Umask = unix.Umask(0) // read the umask
	unix.Umask(Opt.Umask)     // set it back to what it was
	flags.IntVarP(flagSet, &Opt.Umask, "umask", "", Opt.Umask, "Override the permission bits set by the filesystem. Not supported on Windows.")
	Opt.UID = uint32(unix.Geteuid())
	Opt.GID = uint32(unix.Getegid())
	flags.Uint32VarP(flagSet, &Opt.UID, "uid", "", Opt.UID, "Override the uid field set by the filesystem. Not supported on Windows.")
	flags.Uint32VarP(flagSet, &Opt.GID, "gid", "", Opt.GID, "Override the gid field set by the filesystem. Not supported on Windows.")
}
