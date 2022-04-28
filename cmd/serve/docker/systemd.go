//go:build linux && !android
// +build linux,!android

package docker

import (
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/go-systemd/util"
)

func systemdActivationFiles() []*os.File {
	if util.IsRunningSystemd() {
		return activation.Files(false)
	}
	return nil
}
