//go:build fuse3 && (linux || freebsd)

// // Package cmount implements a FUSE mounting system for rclone remotes.
//
// This uses the cgo based cgofuse library
package cmount

import (
	"fmt"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/vfs"
)

// mountOptions configures the options from the command line flags
func mountOptions(VFS *vfs.VFS, device string, mountpoint string, opt *mountlib.Options) (options []string) {
	// Options
	options = []string{
		"-o", fmt.Sprintf("attr_timeout=%g", time.Duration(opt.AttrTimeout).Seconds()),
	}
	if opt.DebugFUSE {
		options = append(options, "-o", "debug")
	}

	options = append(options, "-o", "fsname="+device)
	options = append(options, "-o", "subtype=rclone")
	if opt.AllowOther {
		options = append(options, "-o", "allow_other")
	}
	if opt.AllowRoot {
		options = append(options, "-o", "allow_root")
	}
	if opt.DefaultPermissions {
		options = append(options, "-o", "default_permissions")
	}
	if VFS.Opt.ReadOnly {
		options = append(options, "-o", "ro")
	}
	//if opt.WritebackCache {
	// FIXME? options = append(options, "-o", WritebackCache())
	//}

	for _, option := range opt.ExtraOptions {
		options = append(options, "-o", option)
	}
	options = append(options, opt.ExtraFlags...)
	return options
}
