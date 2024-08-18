//go:build brew && darwin

// Package cmount implements a FUSE mounting system for rclone remotes.
//
// Build for macos with the brew tag to handle the absence
// of fuse and print an appropriate error message
package cmount

import (
	"errors"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/vfs"
)

func init() {
	name := "mount"
	cmd := mountlib.NewMountCommand(name, false, mount)
	cmd.Aliases = append(cmd.Aliases, "cmount")
	mountlib.AddRc("cmount", mount)
}

// mount the file system
//
// The mount point will be ready when this returns.
//
// returns an error, and an error channel for the serve process to
// report an error when fusermount is called.
func mount(_ *vfs.VFS, _ string, _ *mountlib.Options) (<-chan error, func() error, error) {
	return nil, nil, errors.New("rclone mount is not supported on MacOS when rclone is installed via Homebrew. " +
		"Please install the rclone binaries available at https://rclone.org/downloads/ " +
		"instead if you want to use the rclone mount command")
}
