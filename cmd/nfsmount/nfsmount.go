//go:build darwin && !cmount
// +build darwin,!cmount

// Package nfsmount implements mounting functionality using serve nfs command
//
// NFS mount is only needed for macOS since it has no
// support for FUSE-based file systems
package nfsmount

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/cmd/serve/nfs"
	"github.com/rclone/rclone/vfs"
)

func init() {
	cmd := mountlib.NewMountCommand("mount", false, mount)
	cmd.Aliases = append(cmd.Aliases, "nfsmount")
	mountlib.AddRc("nfsmount", mount)
}

func mount(VFS *vfs.VFS, mountpoint string, opt *mountlib.Options) (asyncerrors <-chan error, unmount func() error, err error) {
	s, err := nfs.NewServer(context.Background(), VFS, &nfs.Options{})
	if err != nil {
		return
	}
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Serve()
	}()
	// The port is always picked at random after the NFS server has started
	// we need to query the server for the port number so we can mount it
	_, port, err := net.SplitHostPort(s.Addr().String())
	if err != nil {
		err = fmt.Errorf("cannot find port number in %s", s.Addr().String())
		return
	}
	optionsString := strings.Join(opt.ExtraOptions, ",")
	err = exec.Command("mount", fmt.Sprintf("-oport=%s,mountport=%s,%s", port, port, optionsString), "localhost:", mountpoint).Run()
	if err != nil {
		err = fmt.Errorf("failed to mount NFS volume %e", err)
		return
	}
	asyncerrors = errChan
	unmount = func() error {
		var umountErr error
		if runtime.GOOS == "darwin" {
			umountErr = exec.Command("diskutil", "umount", "force", mountpoint).Run()
		} else {
			umountErr = exec.Command("umount", "-f", mountpoint).Run()
		}
		shutdownErr := s.Shutdown()
		VFS.Shutdown()
		if umountErr != nil {
			return fmt.Errorf("failed to umount the NFS volume %e", umountErr)
		} else if shutdownErr != nil {
			return fmt.Errorf("failed to shutdown NFS server: %e", shutdownErr)
		}
		return nil
	}
	return
}
