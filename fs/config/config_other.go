// Read, write and edit the config file
// Non-unix specific functions.

// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package config

// attemptCopyGroups tries to keep the group the same, which only makes sense
// for system with user-group-world permission model.
func attemptCopyGroup(fromPath, toPath string) {}
