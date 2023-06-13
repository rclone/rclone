package fs

// OverrideDirectory is a wrapper to override the Remote for an
// Directory
type OverrideDirectory struct {
	Directory
	remote string
}

// NewOverrideDirectory returns an OverrideDirectoryObject which will
// return the remote specified
func NewOverrideDirectory(oi Directory, remote string) *OverrideDirectory {
	// re-wrap an OverrideDirectory
	if or, ok := oi.(*OverrideDirectory); ok {
		return &OverrideDirectory{
			Directory: or.Directory,
			remote:    remote,
		}
	}
	return &OverrideDirectory{
		Directory: oi,
		remote:    remote,
	}
}

// Remote returns the overridden remote name
func (o *OverrideDirectory) Remote() string {
	return o.remote
}

// String returns the overridden remote name
func (o *OverrideDirectory) String() string {
	return o.remote
}
