package buildinfo

// Skip building GetOSVersion on openbsd/386 because dependency
// https://github.com/shirou/gopsutil v3.21.6 fails to build on openbsd/386:
//
// Error: ../../../../go/pkg/mod/github.com/tklauser/go-sysconf@v0.3.6/sysconf_openbsd.go:22:28: undefined: unix.RLIMIT_NPROC
//
// Resolution pending on issues which can be resolved after go 1.17:
// - https://github.com/tklauser/go-sysconf/issues/21
// - https://golang.org/cl/341069
//
// Support for openbsd/386 can be restored after we deprecate go 1.17

// GetOSVersion on openbsd/386 returns stub OS version, kernel, bitness
func GetOSVersion() (osVersion, osKernel string) {
	return "OpenBSD/i386", ""
}
