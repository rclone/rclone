//go:build linux || freebsd

package docker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/lib/file"
)

func newUnixListener(path string, gid int) (net.Listener, string, error) {
	// try systemd socket activation
	fds := systemdActivationFiles()
	switch len(fds) {
	case 0:
		// fall thru
	case 1:
		listener, err := net.FileListener(fds[0])
		return listener, "", err
	default:
		return nil, "", fmt.Errorf("expected only one socket from systemd, got %d", len(fds))
	}

	// create socket ourselves
	if filepath.Ext(path) == "" {
		path += ".sock"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(sockDir, path)
	}

	if err := file.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, "", err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, "", err
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", err
	}

	if err = os.Chmod(path, 0660); err != nil {
		return nil, "", err
	}
	if os.Geteuid() == 0 {
		if err = os.Chown(path, 0, gid); err != nil {
			return nil, "", err
		}
	}

	// we don't use spec file with unix sockets
	return listener, path, nil
}
