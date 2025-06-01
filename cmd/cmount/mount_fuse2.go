//go:build !fuse3

// Package cmount implements a FUSE mounting system for rclone remotes.
//
// FUSE2 mount options.
package cmount

import (
	"fmt"
	"runtime"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/vfs"
)

// mountOptions configures the options from the command line flags
//
// nolint:unused This function is used by mount.go.
func mountOptions(VFS *vfs.VFS, device string, mountpoint string, opt *mountlib.Options) (options []string) {
	// Options
	options = []string{
		"-o", fmt.Sprintf("attr_timeout=%g", time.Duration(opt.AttrTimeout).Seconds()),
	}
	if opt.DebugFUSE {
		options = append(options, "-o", "debug")
	}

	if runtime.GOOS == "windows" {
		options = append(options, "-o", "uid=-1")
		options = append(options, "-o", "gid=-1")
		options = append(options, "--FileSystemName=rclone")
		if opt.VolumeName != "" {
			if opt.NetworkMode {
				options = append(options, "--VolumePrefix="+opt.VolumeName)
			} else {
				options = append(options, "-o", "volname="+opt.VolumeName)
			}
		}
	} else {
		options = append(options, "-o", "fsname="+device)
		options = append(options, "-o", "subtype=rclone")
		options = append(options, "-o", fmt.Sprintf("max_readahead=%d", opt.MaxReadAhead))
		// This causes FUSE to supply O_TRUNC with the Open
		// call which is more efficient for cmount.  However
		// it does not work with cgofuse on Windows with
		// WinFSP so cmount must work with or without it.
		options = append(options, "-o", "atomic_o_trunc")
		if opt.DaemonTimeout != 0 {
			options = append(options, "-o", fmt.Sprintf("daemon_timeout=%d", int(time.Duration(opt.DaemonTimeout).Seconds())))
		}
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
		if runtime.GOOS == "darwin" {
			if opt.VolumeName != "" {
				options = append(options, "-o", "volname="+opt.VolumeName)
			}
			if opt.NoAppleDouble {
				options = append(options, "-o", "noappledouble")
			}
			if opt.NoAppleXattr {
				options = append(options, "-o", "noapplexattr")
			}
		}
	}
	for _, option := range opt.ExtraOptions {
		options = append(options, "-o", option)
	}
	options = append(options, opt.ExtraFlags...)
	return options
}
