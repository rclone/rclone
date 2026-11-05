//go:build (windows && !amd64 && !arm64) || !(windows || linux || freebsd || netbsd || openbsd || darwin)

package local

import "errors"

// moveToTrash is not supported on this platform or architecture.
func moveToTrash(path string) error {
	return errors.New("--local-use-trash is not supported on this platform")
}
