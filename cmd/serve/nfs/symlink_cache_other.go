//go:build unix && !linux

package nfs

// Turn the diskHandler into a symlink cache
func (dh *diskHandler) makeSymlinkCache() error {
	return ErrorSymlinkCacheNotSupported
}
