// Read, write and edit the config file
// Unix specific functions.

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package configfile

import (
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/rclone/rclone/fs"
)

// attemptCopyGroup tries to keep the group the same. User will be the one
// who is currently running this process.
func attemptCopyGroup(fromPath, toPath string) {
	info, err := os.Stat(fromPath)
	if err != nil || info.Sys() == nil {
		return
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid := int(stat.Uid)
		// prefer self over previous owner of file, because it has a higher chance
		// of success
		if user, err := user.Current(); err == nil {
			if tmpUID, err := strconv.Atoi(user.Uid); err == nil {
				uid = tmpUID
			}
		}
		if err = os.Chown(toPath, uid, int(stat.Gid)); err != nil {
			fs.Debugf(nil, "Failed to keep previous owner of config file: %v", err)
		}
	}
}
