// Read, write and edit the config file
// Non-unix specific functions.

//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package configfile

// attemptCopyGroup tries to keep the group the same, which only makes sense
// for system with user-group-world permission model.
func attemptCopyGroup(fromPath, toPath string) {}
