// +build linux darwin freebsd

package mountlib

import (
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

// add any extra platform specific flags
func platformFlags(flags *pflag.FlagSet) {
	flags.IntVarP(&Umask, "umask", "", Umask, "Override the permission bits set by the filesystem.")
	Umask = unix.Umask(0) // read the umask
	unix.Umask(Umask)     // set it back to what it was
	UID = uint32(unix.Geteuid())
	GID = uint32(unix.Getegid())
	flags.Uint32VarP(&UID, "uid", "", UID, "Override the uid field set by the filesystem.")
	flags.Uint32VarP(&GID, "gid", "", GID, "Override the gid field set by the filesystem.")
}
