package vfstest

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Functions to run and control the mount subprocess

var (
	runMount = flag.String("run-mount", "", "If set, run the mount subprocess with the options (internal use only)")
)

// Options for the mount sub processes passed with the -run-mount flag
type runMountOpt struct {
	MountPoint string
	MountOpt   mountlib.Options
	VFSOpt     vfscommon.Options
	Remote     string
}

// Start the mount subprocess and wait for it to start
func (r *Run) startMountSubProcess() {
	// If testing the VFS we don't start a subprocess, we just use
	// the VFS directly
	if r.useVFS {
		vfs := vfs.New(r.fremote, r.vfsOpt)
		r.os = vfsOs{vfs}
		return
	}
	r.os = realOs{}
	r.mountPath = findMountPath()
	fs.Logf(nil, "startMountSubProcess %q (%q) %q", r.fremote, r.fremoteName, r.mountPath)

	opt := runMountOpt{
		MountPoint: r.mountPath,
		MountOpt:   mountlib.Opt,
		VFSOpt:     *r.vfsOpt,
		Remote:     r.fremoteName,
	}

	opts, err := json.Marshal(&opt)
	if err != nil {
		fs.Fatal(nil, fmt.Sprint(err))
	}

	// Re-run this executable with a new option -run-mount
	args := append(os.Args, "-run-mount", string(opts))
	r.cmd = exec.Command(args[0], args[1:]...)
	r.cmd.Stderr = os.Stderr
	r.out, err = r.cmd.StdinPipe()
	if err != nil {
		fs.Fatal(nil, fmt.Sprint(err))
	}
	r.in, err = r.cmd.StdoutPipe()
	if err != nil {
		fs.Fatal(nil, fmt.Sprint(err))
	}
	err = r.cmd.Start()
	if err != nil {
		fs.Fatal(nil, fmt.Sprint("startMountSubProcess failed", err))
	}
	r.scanner = bufio.NewScanner(r.in)

	// Wait it for startup
	fs.Log(nil, "Waiting for mount to start")
	for r.scanner.Scan() {
		rx := strings.TrimSpace(r.scanner.Text())
		if rx == "STARTED" {
			break
		}
		fs.Logf(nil, "..Mount said: %s", rx)
	}
	if r.scanner.Err() != nil {
		fs.Logf(nil, "scanner err %v", r.scanner.Err())
	}

	fs.Logf(nil, "startMountSubProcess: end")
}

// Find a free path to run the mount on
func findMountPath() string {
	if runtime.GOOS != "windows" {
		mountPath, err := os.MkdirTemp("", "rclonefs-mount")
		if err != nil {
			fs.Fatalf(nil, "Failed to create mount dir: %v", err)
		}
		return mountPath
	}

	// Find a free drive letter
	letter := file.FindUnusedDriveLetter()
	drive := ""
	if letter == 0 {
		fs.Fatalf(nil, "Couldn't find free drive letter for test")
	} else {
		drive = string(letter) + ":"
	}
	return drive
}

// Return true if we are running as a subprocess to run the mount
func isSubProcess() bool {
	return *runMount != ""
}

// Run the mount - this is running in a subprocesses and the config
// is passed JSON encoded as the -run-mount parameter
//
// It reads commands from standard input and writes results to
// standard output.
func startMount(mountFn mountlib.MountFn, useVFS bool, opts string) {
	fs.Log(nil, "startMount")
	ctx := context.Background()

	var opt runMountOpt
	err := json.Unmarshal([]byte(opts), &opt)
	if err != nil {
		fs.Fatalf(nil, "Unmarshal failed: %v", err)
	}

	fstest.Initialise()

	f, err := cache.Get(ctx, opt.Remote)
	if err != nil {
		fs.Fatalf(nil, "Failed to open remote %q: %v", opt.Remote, err)
	}

	err = f.Mkdir(ctx, "")
	if err != nil {
		fs.Fatalf(nil, "Failed to mkdir %q: %v", opt.Remote, err)
	}

	fs.Logf(nil, "startMount: Mounting %q on %q with %q", opt.Remote, opt.MountPoint, opt.VFSOpt.CacheMode)
	mnt := mountlib.NewMountPoint(mountFn, opt.MountPoint, f, &opt.MountOpt, &opt.VFSOpt)

	_, err = mnt.Mount()
	if err != nil {
		fs.Fatalf(nil, "mount FAILED %q: %v", opt.Remote, err)
	}
	defer umount(mnt)
	fs.Logf(nil, "startMount: mount OK")
	fmt.Println("STARTED") // signal to parent all is good

	// Read commands from stdin
	scanner := bufio.NewScanner(os.Stdin)
	exit := false
	for !exit && scanner.Scan() {
		rx := strings.Trim(scanner.Text(), "\r\n")
		var tx string
		tx, exit = doMountCommand(mnt.VFS, rx)
		fmt.Println(tx)
	}

	err = scanner.Err()
	if err != nil {
		fs.Fatalf(nil, "scanner failed %q: %v", opt.Remote, err)
	}
}

// Do a mount command which is a line read from stdin and return a
// line to send to stdout with an exit flag.
//
// The format of the lines is
//
//	command \t parameter (optional)
//
// The response should be
//
//	OK|ERR \t result (optional)
func doMountCommand(vfs *vfs.VFS, rx string) (tx string, exit bool) {
	command := strings.Split(rx, "\t")
	// log.Printf("doMountCommand: %q received", command)
	var out = []string{"OK", ""}
	switch command[0] {
	case "waitForWriters":
		vfs.WaitForWriters(waitForWritersDelay)
	case "forget":
		root, err := vfs.Root()
		if err != nil {
			out = []string{"ERR", err.Error()}
		} else {
			root.ForgetPath(command[1], fs.EntryDirectory)
		}
	case "exit":
		exit = true
	default:
		out = []string{"ERR", "command not found"}
	}
	return strings.Join(out, "\t"), exit
}

// Send a command to the mount subprocess and await a response
func (r *Run) sendMountCommand(args ...string) {
	r.cmdMu.Lock()
	defer r.cmdMu.Unlock()
	tx := strings.Join(args, "\t")
	// log.Printf("Send mount command: %q", tx)
	var rx string
	if r.useVFS {
		// if using VFS do the VFS command directly
		rx, _ = doMountCommand(r.os.(vfsOs).VFS, tx)
	} else {
		_, err := io.WriteString(r.out, tx+"\n")
		if err != nil {
			fs.Fatalf(nil, "WriteString err %v", err)
		}
		if !r.scanner.Scan() {
			fs.Fatalf(nil, "Mount has gone away")
		}
		rx = strings.Trim(r.scanner.Text(), "\r\n")
	}
	in := strings.Split(rx, "\t")
	// log.Printf("Answer is %q", in)
	if in[0] != "OK" {
		fs.Fatalf(nil, "Error from mount: %q", in[1:])
	}
}

// wait for any files being written to be released by fuse
func (r *Run) waitForWriters() {
	r.sendMountCommand("waitForWriters")
}

// forget the directory passed in
func (r *Run) forget(dir string) {
	r.sendMountCommand("forget", dir)
}

// Unmount the mount
func umount(mnt *mountlib.MountPoint) {
	/*
		log.Printf("Calling fusermount -u %q", mountPath)
		err := exec.Command("fusermount", "-u", mountPath).Run()
		if err != nil {
			log.Printf("fusermount failed: %v", err)
		}
	*/
	fs.Logf(nil, "Unmounting %q", mnt.MountPoint)
	err := mnt.Unmount()
	if err != nil {
		fs.Logf(nil, "signal to umount failed - retrying: %v", err)
		time.Sleep(3 * time.Second)
		err = mnt.Unmount()
	}
	if err != nil {
		fs.Fatalf(nil, "signal to umount failed: %v", err)
	}
	fs.Logf(nil, "Waiting for umount")
	err = <-mnt.ErrChan
	if err != nil {
		fs.Fatalf(nil, "umount failed: %v", err)
	}

	// Cleanup the VFS cache - umount has called Shutdown
	err = mnt.VFS.CleanUp()
	if err != nil {
		fs.Logf(nil, "Failed to cleanup the VFS cache: %v", err)
	}
}
