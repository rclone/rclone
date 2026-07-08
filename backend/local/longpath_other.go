//go:build !windows

package local

import "errors"

var errWindowsLongPath = errors.New("windows path length limit exceeded")

func wrapWindowsPathLengthError(path string, noUNC bool, err error) error {
	return err
}
