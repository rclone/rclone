//go:build ios
// +build ios

package buildinfo

// GetOSVersion returns OS version, kernel and bitness
func GetOSVersion() (osVersion, osKernel string) {
	return "ios", "unknown"
}
