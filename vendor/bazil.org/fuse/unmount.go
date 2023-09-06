package fuse

// Unmount tries to unmount the filesystem mounted at dir.
func Unmount(dir string) error {
	return unmount(dir)
}
