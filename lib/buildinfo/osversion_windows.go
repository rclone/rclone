// +build !openbsd !windows

package buildinfo

import (
	"fmt"
	"regexp"
	"strings"
	"unsafe"

	"github.com/shirou/gopsutil/v3/host"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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

		// Simplify kernel output: `RELEASE.BUILD Build BUILD` -> `RELEASE.BUILD`
		match := regexp.MustCompile(`^([\d\.]+?\.)(\d+) Build (\d+)$`).FindStringSubmatch(osKernel)
		if len(match) == 4 && match[2] == match[3] {
			osKernel = match[1] + match[2]
		}
	}

	friendlyName := getRegistryVersionString("ReleaseId")
	if osVersion != "" && friendlyName != "" {
		osVersion += " " + friendlyName
	}

	updateRevision := getRegistryVersionInt("UBR")
	if osKernel != "" && updateRevision != 0 {
		osKernel += fmt.Sprintf(".%d", updateRevision)
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

	return windows.UTF16ToString(regBuf[:])
}

func getRegistryVersionInt(name string) int {
	var (
		err     error
		handle  windows.Handle
		bufLen  uint32
		valType uint32
	)

	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, regVersionKeyUTF16, 0, windows.KEY_READ|windows.KEY_WOW64_64KEY, &handle)
	if err != nil {
		return 0
	}
	defer func() {
		_ = windows.RegCloseKey(handle)
	}()

	nameUTF16 := windows.StringToUTF16Ptr(name)
	err = windows.RegQueryValueEx(handle, nameUTF16, nil, &valType, nil, &bufLen)
	if err != nil {
		return 0
	}

	if valType != registry.DWORD || bufLen != 4 {
		return 0
	}
	var val32 uint32
	err = windows.RegQueryValueEx(handle, nameUTF16, nil, &valType, (*byte)(unsafe.Pointer(&val32)), &bufLen)
	if err != nil {
		return 0
	}

	return int(val32)
}
