//go:build fuse3 && (linux || freebsd)

// Package cmount implements a FUSE mounting system for rclone remotes.
package cmount

const isFuse3 = true
