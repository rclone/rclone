// +build openbsd

package buildinfo

// gopsutil v3.21.3 fails to build on openbsd:
// Error: .../go/pkg/mod/github.com/tklauser/go-sysconf@v0.3.4/sysconf_openbsd.go:22:28: undefined: unix.RLIMIT_NPROC
// Error: .../go/pkg/mod/github.com/shirou/gopsutil/v3@v3.21.3/process/process.go:163:15: undefined: pidsWithContext
// and so on...

// GetOSVersion returns OS version, kernel and bitness
func GetOSVersion() (osVersion, osKernel string) {
	return "OpenBSD", ""
}
