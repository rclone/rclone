//go:build unix

// Package daemonize provides daemonization interface for Unix platforms.
package daemonize

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

// StartDaemon runs background twin of current process.
// It executes separate parts of code in child and parent processes.
// Returns child process pid in the parent or nil in the child.
// The method looks like a fork but safe for goroutines.
func StartDaemon(args []string) (*os.Process, error) {
	if fs.IsDaemon() {
		// This process is already daemonized
		return nil, nil
	}

	env := append(os.Environ(), fs.DaemonMarkVar+"="+fs.DaemonMarkChild)

	me, err := os.Executable()
	if err != nil {
		me = os.Args[0]
	}

	// os.Executable might have resolved symbolic link to the executable
	// so we run the background process with pre-converted CLI arguments.
	// Double conversion is still probable but isn't a problem as it should
	// preserve the converted command line.
	if len(args) != 0 {
		args[0] = me
	}

	if fs.PassDaemonArgsAsEnviron {
		args, env = argsToEnv(args, env)
	}

	null, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	files := []*os.File{
		null, // (0) stdin
		null, // (1) stdout
		null, // (2) stderr
	}
	sysAttr := &syscall.SysProcAttr{
		// setsid (https://linux.die.net/man/2/setsid) in the child process will reset
		// its process group id (PGID) to its PID thus detaching it from parent.
		// This would make autofs fail because it detects mounting process by its PGID.
		Setsid: false,
	}
	attr := &os.ProcAttr{
		Env:   env,
		Files: files,
		Sys:   sysAttr,
	}

	daemon, err := os.StartProcess(me, args, attr)
	if err != nil {
		return nil, err
	}

	return daemon, nil
}

// Processed command line flags of mount helper have simple structure:
// `--flag` or `--flag=value` but never `--flag value` or `-x`
// so we can easily pass them as environment variables.
func argsToEnv(origArgs, origEnv []string) (args, env []string) {
	env = origEnv
	if len(origArgs) == 0 {
		return
	}
	args = []string{origArgs[0]}
	for _, arg := range origArgs[1:] {
		if !strings.HasPrefix(arg, "--") {
			args = append(args, arg)
			continue
		}

		arg = arg[2:]
		key, val := arg, "true"
		if idx := strings.Index(arg, "="); idx != -1 {
			key, val = arg[:idx], arg[idx+1:]
		}

		name := "RCLONE_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))

		pref := name + "="
		line := name + "=" + val
		found := false
		for i, s := range env {
			if strings.HasPrefix(s, pref) {
				env[i] = line
				found = true
			}
		}
		if !found {
			env = append(env, line)
		}
	}
	return
}

// Check returns non nil if the daemon process has died
func Check(daemon *os.Process) error {
	var status unix.WaitStatus
	wpid, err := unix.Wait4(daemon.Pid, &status, unix.WNOHANG, nil)
	// fs.Debugf(nil, "wait4 returned wpid=%d, err=%v, status=%d", wpid, err, status)
	if err != nil {
		return err
	}
	if wpid == 0 {
		return nil
	}
	if status.Exited() {
		return fmt.Errorf("daemon exited with error code %d", status.ExitStatus())
	}
	return nil
}
