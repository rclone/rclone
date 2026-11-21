//go:build !linux

package vfstest

// isNFSMount returns true if the mount under test is an NFS mount.
func (r *Run) isNFSMount() bool {
	return false
}
