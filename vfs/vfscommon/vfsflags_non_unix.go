//go:build !linux && !darwin && !freebsd

package vfscommon

// get the current umask
func getUmask() int {
	return 0000
}

// get the current uid
func getUID() uint32 {
	return ^uint32(0) // these values instruct WinFSP-FUSE to use the current user
}

// get the current gid
func getGID() uint32 {
	return ^uint32(0) // these values instruct WinFSP-FUSE to use the current user
}
