//go:build linux
// +build linux

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func DoSysctrl(mib string) ([]string, error) {
	cmd := exec.Command("sysctl", "-n", mib)
	cmd.Env = getSysctrlEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return []string{}, err
	}
	v := strings.Replace(string(out), "{ ", "", 1)
	v = strings.Replace(string(v), " }", "", 1)
	values := strings.Fields(string(v))

	return values, nil
}

func NumProcs() (uint64, error) {
	return NumProcsWithContext(context.Background())
}

func NumProcsWithContext(ctx context.Context) (uint64, error) {
	f, err := os.Open(HostProcWithContext(ctx))
	if err != nil {
		return 0, err
	}
	defer f.Close()

	list, err := f.Readdirnames(-1)
	if err != nil {
		return 0, err
	}
	var cnt uint64

	for _, v := range list {
		if _, err = strconv.ParseUint(v, 10, 64); err == nil {
			cnt++
		}
	}

	return cnt, nil
}

func BootTimeWithContext(ctx context.Context) (uint64, error) {
	system, role, err := VirtualizationWithContext(ctx)
	if err != nil {
		return 0, err
	}

	statFile := "stat"
	if system == "lxc" && role == "guest" {
		// if lxc, /proc/uptime is used.
		statFile = "uptime"
	} else if system == "docker" && role == "guest" {
		// also docker, guest
		statFile = "uptime"
	}

	filename := HostProcWithContext(ctx, statFile)
	lines, err := ReadLines(filename)
	if os.IsPermission(err) {
		var info syscall.Sysinfo_t
		err := syscall.Sysinfo(&info)
		if err != nil {
			return 0, err
		}

		currentTime := time.Now().UnixNano() / int64(time.Second)
		t := currentTime - int64(info.Uptime)
		return uint64(t), nil
	}
	if err != nil {
		return 0, err
	}

	if statFile == "stat" {
		for _, line := range lines {
			if strings.HasPrefix(line, "btime") {
				f := strings.Fields(line)
				if len(f) != 2 {
					return 0, fmt.Errorf("wrong btime format")
				}
				b, err := strconv.ParseInt(f[1], 10, 64)
				if err != nil {
					return 0, err
				}
				t := uint64(b)
				return t, nil
			}
		}
	} else if statFile == "uptime" {
		if len(lines) != 1 {
			return 0, fmt.Errorf("wrong uptime format")
		}
		f := strings.Fields(lines[0])
		b, err := strconv.ParseFloat(f[0], 64)
		if err != nil {
			return 0, err
		}
		currentTime := float64(time.Now().UnixNano()) / float64(time.Second)
		t := currentTime - b
		return uint64(t), nil
	}

	return 0, fmt.Errorf("could not find btime")
}

func Virtualization() (string, string, error) {
	return VirtualizationWithContext(context.Background())
}

// required variables for concurrency safe virtualization caching
var (
	cachedVirtMap   map[string]string
	cachedVirtMutex sync.RWMutex
	cachedVirtOnce  sync.Once
)

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	var system, role string

	// if cached already, return from cache
	cachedVirtMutex.RLock() // unlock won't be deferred so concurrent reads don't wait for long
	if cachedVirtMap != nil {
		cachedSystem, cachedRole := cachedVirtMap["system"], cachedVirtMap["role"]
		cachedVirtMutex.RUnlock()
		return cachedSystem, cachedRole, nil
	}
	cachedVirtMutex.RUnlock()

	filename := HostProcWithContext(ctx, "xen")
	if PathExists(filename) {
		system = "xen"
		role = "guest" // assume guest

		if PathExists(filepath.Join(filename, "capabilities")) {
			contents, err := ReadLines(filepath.Join(filename, "capabilities"))
			if err == nil {
				if StringsContains(contents, "control_d") {
					role = "host"
				}
			}
		}
	}

	filename = HostProcWithContext(ctx, "modules")
	if PathExists(filename) {
		contents, err := ReadLines(filename)
		if err == nil {
			if StringsContains(contents, "kvm") {
				system = "kvm"
				role = "host"
			} else if StringsContains(contents, "hv_util") {
				system = "hyperv"
				role = "guest"
			} else if StringsContains(contents, "vboxdrv") {
				system = "vbox"
				role = "host"
			} else if StringsContains(contents, "vboxguest") {
				system = "vbox"
				role = "guest"
			} else if StringsContains(contents, "vmware") {
				system = "vmware"
				role = "guest"
			}
		}
	}

	filename = HostProcWithContext(ctx, "cpuinfo")
	if PathExists(filename) {
		contents, err := ReadLines(filename)
		if err == nil {
			if StringsContains(contents, "QEMU Virtual CPU") ||
				StringsContains(contents, "Common KVM processor") ||
				StringsContains(contents, "Common 32-bit KVM processor") {
				system = "kvm"
				role = "guest"
			}
		}
	}

	filename = HostProcWithContext(ctx, "bus/pci/devices")
	if PathExists(filename) {
		contents, err := ReadLines(filename)
		if err == nil {
			if StringsContains(contents, "virtio-pci") {
				role = "guest"
			}
		}
	}

	filename = HostProcWithContext(ctx)
	if PathExists(filepath.Join(filename, "bc", "0")) {
		system = "openvz"
		role = "host"
	} else if PathExists(filepath.Join(filename, "vz")) {
		system = "openvz"
		role = "guest"
	}

	// not use dmidecode because it requires root
	if PathExists(filepath.Join(filename, "self", "status")) {
		contents, err := ReadLines(filepath.Join(filename, "self", "status"))
		if err == nil {
			if StringsContains(contents, "s_context:") ||
				StringsContains(contents, "VxID:") {
				system = "linux-vserver"
			}
			// TODO: guest or host
		}
	}

	if PathExists(filepath.Join(filename, "1", "environ")) {
		contents, err := ReadFile(filepath.Join(filename, "1", "environ"))

		if err == nil {
			if strings.Contains(contents, "container=lxc") {
				system = "lxc"
				role = "guest"
			}
		}
	}

	if PathExists(filepath.Join(filename, "self", "cgroup")) {
		contents, err := ReadLines(filepath.Join(filename, "self", "cgroup"))
		if err == nil {
			if StringsContains(contents, "lxc") {
				system = "lxc"
				role = "guest"
			} else if StringsContains(contents, "docker") {
				system = "docker"
				role = "guest"
			} else if StringsContains(contents, "machine-rkt") {
				system = "rkt"
				role = "guest"
			} else if PathExists("/usr/bin/lxc-version") {
				system = "lxc"
				role = "host"
			}
		}
	}

	if PathExists(HostEtcWithContext(ctx, "os-release")) {
		p, _, err := GetOSReleaseWithContext(ctx)
		if err == nil && p == "coreos" {
			system = "rkt" // Is it true?
			role = "host"
		}
	}

	if PathExists(HostRootWithContext(ctx, ".dockerenv")) {
		system = "docker"
		role = "guest"
	}

	// before returning for the first time, cache the system and role
	cachedVirtOnce.Do(func() {
		cachedVirtMutex.Lock()
		defer cachedVirtMutex.Unlock()
		cachedVirtMap = map[string]string{
			"system": system,
			"role":   role,
		}
	})

	return system, role, nil
}

func GetOSRelease() (platform string, version string, err error) {
	return GetOSReleaseWithContext(context.Background())
}

func GetOSReleaseWithContext(ctx context.Context) (platform string, version string, err error) {
	contents, err := ReadLines(HostEtcWithContext(ctx, "os-release"))
	if err != nil {
		return "", "", nil // return empty
	}
	for _, line := range contents {
		field := strings.Split(line, "=")
		if len(field) < 2 {
			continue
		}
		switch field[0] {
		case "ID": // use ID for lowercase
			platform = trimQuotes(field[1])
		case "VERSION":
			version = trimQuotes(field[1])
		}
	}

	// cleanup amazon ID
	if platform == "amzn" {
		platform = "amazon"
	}

	return platform, version, nil
}

// Remove quotes of the source string
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}
