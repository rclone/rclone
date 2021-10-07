//go:build !linux || android
// +build !linux android

package docker

import (
	"os"
)

func systemdActivationFiles() []*os.File {
	return nil
}
