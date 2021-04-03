// +build !openbsd

package buildinfo

import (
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/v3/host"
)

// GetOSVersion returns OS version, kernel and bitness
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
