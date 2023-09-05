//go:build illumos || js || plan9 || solaris

package diskusage

// New returns the disk status for dir.
//
// May return Unsupported error if it doesn't work on this platform.
func New(dir string) (info Info, err error) {
	return info, ErrUnsupported
}
