//go:build linux || freebsd || openbsd || darwin || solaris
// +build linux freebsd openbsd darwin solaris

package host

import "golang.org/x/sys/unix"

func KernelArch() (string, error) {
	var utsname unix.Utsname
	err := unix.Uname(&utsname)
	if err != nil {
		return "", err
	}
	return unix.ByteSliceToString(utsname.Machine[:]), nil
}
