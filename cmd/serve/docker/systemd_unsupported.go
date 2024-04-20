//go:build !linux || android

package docker

import (
	"os"
)

//lint:ignore U1000 unused when not building linux
func systemdActivationFiles() []*os.File {
	return nil
}
