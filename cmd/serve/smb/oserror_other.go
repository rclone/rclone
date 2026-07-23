//go:build !windows

package smb

// osErrorStatus has no extra OS-specific mappings on non-Windows platforms.
func osErrorStatus(err error) (uint32, bool) { return 0, false }
