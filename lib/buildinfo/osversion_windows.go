package buildinfo

import (
	"regexp"
	"strings"
	"unsafe"

	"github.com/shirou/gopsutil/v3/host"
	"golang.org/x/sys/windows"
)

// GetOSVersion returns OS version, kernel and bitness
// On Windows it performs additional output enhancements.
func GetOSVersion() (osVersion, osKernel string) {
	if platform, _, version, err := host.PlatformInformation(); err == nil && platform != "" {
		osVersion = platform
		if version != "" {
			osVersion += " " + version
		}
	}

	if version, err := host.KernelVersion(); err == nil && version != "" {
		osKernel = version

		// Prevent duplication of output on Windows
		if strings.Contains(osVersion, osKernel) {
			deduped := strings.TrimSpace(strings.Replace(osVersion, osKernel, "", 1))
			if deduped != "" {
				osVersion = deduped
			}
		}

		// Simplify kernel output: `MAJOR.MINOR.BUILD.REVISION Build BUILD.REVISION` -> `MAJOR.MINOR.BUILD.REVISION`
		match := regexp.MustCompile(`^(\d+\.\d+\.(\d+\.\d+)) Build (\d+\.\d+)$`).FindStringSubmatch(osKernel)
		if len(match) == 4 && match[2] == match[3] {
			osKernel = match[1]
		}
	}

	if osVersion != "" {
		// Include the friendly-name of the version, which is typically what is referred to.
		// Until Windows 10 version 2004 (May 2020) this can be found from registry entry
		// ReleaseID, after that we must use entry DisplayVersion (ReleaseId is stuck at 2009).
		// Source: https://ss64.com/nt/ver.html
		friendlyName := getRegistryVersionString("DisplayVersion")
		if friendlyName == "" {
			friendlyName = getRegistryVersionString("ReleaseId")
		}
		if friendlyName != "" {
			osVersion += " " + friendlyName
		}
	}

	if arch, err := host.KernelArch(); err == nil && arch != "" {
		if strings.HasSuffix(arch, "64") && osVersion != "" {
			osVersion += " (64 bit)"
		}
		if osKernel != "" {
			osKernel += " (" + arch + ")"
		}
	}

	return
}

var regVersionKeyUTF16 = windows.StringToUTF16Ptr(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`)

func getRegistryVersionString(name string) string {
	var (
		err     error
		handle  windows.Handle
		bufLen  uint32
		valType uint32
	)

	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, regVersionKeyUTF16, 0, windows.KEY_READ|windows.KEY_WOW64_64KEY, &handle)
	if err != nil {
		return ""
	}
	defer func() {
		_ = windows.RegCloseKey(handle)
	}()

	nameUTF16 := windows.StringToUTF16Ptr(name)
	err = windows.RegQueryValueEx(handle, nameUTF16, nil, &valType, nil, &bufLen)
	if err != nil {
		return ""
	}

	regBuf := make([]uint16, bufLen/2+1)
	err = windows.RegQueryValueEx(handle, nameUTF16, nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
	if err != nil {
		return ""
	}

	return windows.UTF16ToString(regBuf)
}
