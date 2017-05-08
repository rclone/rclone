// +build cgo
// +build linux darwin freebsd

package cmount

import "golang.org/x/sys/unix"

const commandName = "cmount"

func init() {
	umask = unix.Umask(0) // read the umask
	unix.Umask(umask)     // set it back to what it was
	uid = uint32(unix.Geteuid())
	gid = uint32(unix.Getegid())
	commandDefintion.Flags().Uint32VarP(&uid, "uid", "", uid, "Override the uid field set by the filesystem.")
	commandDefintion.Flags().Uint32VarP(&gid, "gid", "", gid, "Override the gid field set by the filesystem.")
}
