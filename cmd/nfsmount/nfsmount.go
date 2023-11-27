//go:build unix
// +build unix

// Package nfsmount implements mounting functionality using serve nfs command
//
// This can potentially work on all unix like systems which can mount NFS.
package nfsmount

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/cmd/serve/nfs"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/vfs"
)

var (
	sudo = false
)

func init() {
	name := "nfsmount"
	cmd := mountlib.NewMountCommand(name, false, mount)
	cmd.Annotations["versionIntroduced"] = "v1.65"
	cmd.Annotations["status"] = "Experimental"
	mountlib.AddRc(name, mount)
	cmdFlags := cmd.Flags()
	flags.BoolVarP(cmdFlags, &sudo, "sudo", "", sudo, "Use sudo to run the mount command as root.", "")
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

	// Options
	options := []string{
		"-o", fmt.Sprintf("port=%s", port),
		"-o", fmt.Sprintf("mountport=%s", port),
	}
	for _, option := range opt.ExtraOptions {
		options = append(options, "-o", option)
	}
	options = append(options, opt.ExtraFlags...)

	cmd := []string{}
	if sudo {
		cmd = append(cmd, "sudo")
	}
	cmd = append(cmd, "mount")
	cmd = append(cmd, options...)
	cmd = append(cmd, "localhost:", mountpoint)
	fs.Debugf(nil, "Running mount command: %q", cmd)

	out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		out = bytes.TrimSpace(out)
		err = fmt.Errorf("%s: failed to mount NFS volume: %v", out, err)
		return
	}
	asyncerrors = errChan
	unmount = func() error {
		var umountErr error
		var out []byte
		if runtime.GOOS == "darwin" {
			out, umountErr = exec.Command("diskutil", "umount", "force", mountpoint).CombinedOutput()
		} else {
			out, umountErr = exec.Command("umount", "-f", mountpoint).CombinedOutput()
		}
		shutdownErr := s.Shutdown()
		VFS.Shutdown()
		if umountErr != nil {
			out = bytes.TrimSpace(out)
			return fmt.Errorf("%s: failed to umount the NFS volume %e", out, umountErr)
		} else if shutdownErr != nil {
			return fmt.Errorf("failed to shutdown NFS server: %e", shutdownErr)
		}
		return nil
	}
	return
}
