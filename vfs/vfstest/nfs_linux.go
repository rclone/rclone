//go:build linux

package vfstest

import "syscall"

// nfsSuperMagic is NFS_SUPER_MAGIC from linux/magic.h
const nfsSuperMagic = 0x6969

// isNFSMount returns true if the mount under test is an NFS mount.
func (r *Run) isNFSMount() bool {
	if r.useVFS {
		return false
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(r.mountPath, &st); err != nil {
		return false
	}
	return st.Type == nfsSuperMagic
}
