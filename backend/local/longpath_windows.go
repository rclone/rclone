//go:build windows

package local

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unicode/utf16"
)

const windowsMaxPath = 260

const (
	windowsErrorInvalidName        syscall.Errno = 123 // ERROR_INVALID_NAME
	windowsErrorFilenameExcedRange syscall.Errno = 206 // ERROR_FILENAME_EXCED_RANGE
)

var errWindowsLongPath = errors.New("windows path length limit exceeded")

func windowsPathLength(path string) int {
	return len(utf16.Encode([]rune(path)))
}

func hasWindowsLongPathPrefix(path string) bool {
	return strings.HasPrefix(path, `\\?\`)
}

func isWindowsPathLengthError(err error) bool {
	return errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) ||
		errors.Is(err, windowsErrorInvalidName) ||
		errors.Is(err, windowsErrorFilenameExcedRange)
}

func wrapWindowsPathLengthError(path string, noUNC bool, err error) error {
	if err == nil {
		return nil
	}
	if !noUNC || hasWindowsLongPathPrefix(path) || !isWindowsPathLengthError(err) {
		return err
	}
	pathLen := windowsPathLength(path)
	if pathLen < windowsMaxPath {
		return err
	}
	return fmt.Errorf("%w: %w", newWindowsLongPathError(path, pathLen), err)
}

func newWindowsLongPathError(path string, pathLen int) error {
	msg := fmt.Sprintf("local path %q is %d UTF-16 code units and may exceed a Windows path length limit while --local-nounc disables UNC long path conversion; disable --local-nounc or shorten the path", path, pathLen)
	return fmt.Errorf("%s: %w", msg, errWindowsLongPath)
}
