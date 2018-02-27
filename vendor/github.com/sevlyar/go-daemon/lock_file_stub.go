// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!plan9,!solaris

package daemon

func lockFile(fd uintptr) error {
	return errNotSupported
}

func unlockFile(fd uintptr) error {
	return errNotSupported
}

func getFdName(fd uintptr) (name string, err error) {
	return "", errNotSupported
}
