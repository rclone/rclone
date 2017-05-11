package fstestutil

// MountInfo describes a mounted file system.
type MountInfo struct {
	FSName string
	Type   string
}

// GetMountInfo finds information about the mount at mnt. It is
// intended for use by tests only, and only fetches information
// relevant to the current tests.
func GetMountInfo(mnt string) (*MountInfo, error) {
	return getMountInfo(mnt)
}

// cstr converts a nil-terminated C string into a Go string
func cstr(ca []int8) string {
	s := make([]byte, 0, len(ca))
	for _, c := range ca {
		if c == 0x00 {
			break
		}
		s = append(s, byte(c))
	}
	return string(s)
}
