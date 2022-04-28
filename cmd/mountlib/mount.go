package mountlib

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/daemonize"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"

	sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Options for creating the mount
type Options struct {
	DebugFUSE          bool
	AllowNonEmpty      bool
	AllowRoot          bool
	AllowOther         bool
	DefaultPermissions bool
	WritebackCache     bool
	Daemon             bool
	DaemonWait         time.Duration // time to wait for ready mount from daemon, maximum on Linux or constant on macOS/BSD
	MaxReadAhead       fs.SizeSuffix
	ExtraOptions       []string
	ExtraFlags         []string
	AttrTimeout        time.Duration // how long the kernel caches attribute for
	DeviceName         string
	VolumeName         string
	NoAppleDouble      bool
	NoAppleXattr       bool
	DaemonTimeout      time.Duration // OSXFUSE only
	AsyncRead          bool
	NetworkMode        bool // Windows only
}

// DefaultOpt is the default values for creating the mount
var DefaultOpt = Options{
	MaxReadAhead:  128 * 1024,
	AttrTimeout:   1 * time.Second, // how long the kernel caches attribute for
	NoAppleDouble: true,            // use noappledouble by default
	NoAppleXattr:  false,           // do not use noapplexattr by default
	AsyncRead:     true,            // do async reads by default
}

type (
	// UnmountFn is called to unmount the file system
	UnmountFn func() error
	// MountFn is called to mount the file system
	MountFn func(VFS *vfs.VFS, mountpoint string, opt *Options) (<-chan error, func() error, error)
)

// MountPoint represents a mount with options and runtime state
type MountPoint struct {
	MountPoint string
	MountedOn  time.Time
	MountOpt   Options
	VFSOpt     vfscommon.Options
	Fs         fs.Fs
	VFS        *vfs.VFS
	MountFn    MountFn
	UnmountFn  UnmountFn
	ErrChan    <-chan error
}

// NewMountPoint makes a new mounting structure
func NewMountPoint(mount MountFn, mountPoint string, f fs.Fs, mountOpt *Options, vfsOpt *vfscommon.Options) *MountPoint {
	return &MountPoint{
		MountFn:    mount,
		MountPoint: mountPoint,
		Fs:         f,
		MountOpt:   *mountOpt,
		VFSOpt:     *vfsOpt,
	}
}

// Global constants
const (
	MaxLeafSize = 1024 // don't pass file names longer than this
)

func init() {
	switch runtime.GOOS {
	case "darwin":
		// DaemonTimeout defaults to non-zero for macOS
		// (this is a macOS specific kernel option unrelated to DaemonWait)
		DefaultOpt.DaemonTimeout = 10 * time.Minute
	}

	switch runtime.GOOS {
	case "linux":
		// Linux provides /proc/mounts to check mount status
		// so --daemon-wait means *maximum* time to wait
		DefaultOpt.DaemonWait = 60 * time.Second
	case "darwin", "openbsd", "freebsd", "netbsd":
		// On BSD we can't check mount status yet
		// so --daemon-wait is just a *constant* delay
		DefaultOpt.DaemonWait = 5 * time.Second
	}

	// Opt must be assigned in the init block to ensure changes really get in
	Opt = DefaultOpt
}

// Opt contains options set by command line flags
var Opt Options

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("mount", &Opt)
	flags.BoolVarP(flagSet, &Opt.DebugFUSE, "debug-fuse", "", Opt.DebugFUSE, "Debug the FUSE internals - needs -v")
	flags.DurationVarP(flagSet, &Opt.AttrTimeout, "attr-timeout", "", Opt.AttrTimeout, "Time for which file/directory attributes are cached")
	flags.StringArrayVarP(flagSet, &Opt.ExtraOptions, "option", "o", []string{}, "Option for libfuse/WinFsp (repeat if required)")
	flags.StringArrayVarP(flagSet, &Opt.ExtraFlags, "fuse-flag", "", []string{}, "Flags or arguments to be passed direct to libfuse/WinFsp (repeat if required)")
	// Non-Windows only
	flags.BoolVarP(flagSet, &Opt.Daemon, "daemon", "", Opt.Daemon, "Run mount in background and exit parent process (as background output is suppressed, use --log-file with --log-format=pid,... to monitor) (not supported on Windows)")
	flags.DurationVarP(flagSet, &Opt.DaemonTimeout, "daemon-timeout", "", Opt.DaemonTimeout, "Time limit for rclone to respond to kernel (not supported on Windows)")
	flags.BoolVarP(flagSet, &Opt.DefaultPermissions, "default-permissions", "", Opt.DefaultPermissions, "Makes kernel enforce access control based on the file mode (not supported on Windows)")
	flags.BoolVarP(flagSet, &Opt.AllowNonEmpty, "allow-non-empty", "", Opt.AllowNonEmpty, "Allow mounting over a non-empty directory (not supported on Windows)")
	flags.BoolVarP(flagSet, &Opt.AllowRoot, "allow-root", "", Opt.AllowRoot, "Allow access to root user (not supported on Windows)")
	flags.BoolVarP(flagSet, &Opt.AllowOther, "allow-other", "", Opt.AllowOther, "Allow access to other users (not supported on Windows)")
	flags.BoolVarP(flagSet, &Opt.AsyncRead, "async-read", "", Opt.AsyncRead, "Use asynchronous reads (not supported on Windows)")
	flags.FVarP(flagSet, &Opt.MaxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads (not supported on Windows)")
	flags.BoolVarP(flagSet, &Opt.WritebackCache, "write-back-cache", "", Opt.WritebackCache, "Makes kernel buffer writes before sending them to rclone (without this, writethrough caching is used) (not supported on Windows)")
	flags.StringVarP(flagSet, &Opt.DeviceName, "devname", "", Opt.DeviceName, "Set the device name - default is remote:path")
	// Windows and OSX
	flags.StringVarP(flagSet, &Opt.VolumeName, "volname", "", Opt.VolumeName, "Set the volume name (supported on Windows and OSX only)")
	// OSX only
	flags.BoolVarP(flagSet, &Opt.NoAppleDouble, "noappledouble", "", Opt.NoAppleDouble, "Ignore Apple Double (._) and .DS_Store files (supported on OSX only)")
	flags.BoolVarP(flagSet, &Opt.NoAppleXattr, "noapplexattr", "", Opt.NoAppleXattr, "Ignore all \"com.apple.*\" extended attributes (supported on OSX only)")
	// Windows only
	flags.BoolVarP(flagSet, &Opt.NetworkMode, "network-mode", "", Opt.NetworkMode, "Mount as remote network drive, instead of fixed disk drive (supported on Windows only)")
	// Unix only
	flags.DurationVarP(flagSet, &Opt.DaemonWait, "daemon-wait", "", Opt.DaemonWait, "Time to wait for ready mount from daemon (maximum time on Linux, constant sleep time on OSX/BSD) (not supported on Windows)")
}

// NewMountCommand makes a mount command with the given name and Mount function
func NewMountCommand(commandName string, hidden bool, mount MountFn) *cobra.Command {
	var commandDefinition = &cobra.Command{
		Use:    commandName + " remote:path /path/to/mountpoint",
		Hidden: hidden,
		Short:  `Mount the remote as file system on a mountpoint.`,
		Long:   strings.ReplaceAll(strings.ReplaceAll(mountHelp, "|", "`"), "@", commandName) + vfs.Help,
		Run: func(command *cobra.Command, args []string) {
			cmd.CheckArgs(2, 2, command, args)

			if fs.GetConfig(context.Background()).UseListR {
				fs.Logf(nil, "--fast-list does nothing on a mount")
			}

			if Opt.Daemon {
				config.PassConfigKeyForDaemonization = true
			}

			if os.Getenv("PATH") == "" && runtime.GOOS != "windows" {
				// PATH can be unset when running under Autofs or Systemd mount
				fs.Debugf(nil, "Using fallback PATH to run fusermount")
				_ = os.Setenv("PATH", "/bin:/usr/bin")
			}

			// Show stats if the user has specifically requested them
			if cmd.ShowStats() {
				defer cmd.StartStats()()
			}

			mnt := NewMountPoint(mount, args[1], cmd.NewFsDir(args), &Opt, &vfsflags.Opt)
			daemon, err := mnt.Mount()

			// Wait for foreground mount, if any...
			if daemon == nil {
				if err == nil {
					err = mnt.Wait()
				}
				if err != nil {
					log.Fatalf("Fatal error: %v", err)
				}
				return
			}

			// Wait for daemon, if any...
			killOnce := sync.Once{}
			killDaemon := func(reason string) {
				killOnce.Do(func() {
					if err := daemon.Signal(os.Interrupt); err != nil {
						fs.Errorf(nil, "%s. Failed to terminate daemon pid %d: %v", reason, daemon.Pid, err)
						return
					}
					fs.Debugf(nil, "%s. Terminating daemon pid %d", reason, daemon.Pid)
				})
			}

			if err == nil && Opt.DaemonWait > 0 {
				handle := atexit.Register(func() {
					killDaemon("Got interrupt")
				})
				err = WaitMountReady(mnt.MountPoint, Opt.DaemonWait)
				if err != nil {
					killDaemon("Daemon timed out")
				}
				atexit.Unregister(handle)
			}
			if err != nil {
				log.Fatalf("Fatal error: %v", err)
			}
		},
	}

	// Register the command
	cmd.Root.AddCommand(commandDefinition)

	// Add flags
	cmdFlags := commandDefinition.Flags()
	AddFlags(cmdFlags)
	vfsflags.AddFlags(cmdFlags)

	return commandDefinition
}

// Mount the remote at mountpoint
func (m *MountPoint) Mount() (daemon *os.Process, err error) {
	if err = m.CheckOverlap(); err != nil {
		return nil, err
	}

	if err = m.CheckAllowings(); err != nil {
		return nil, err
	}
	m.SetVolumeName(m.MountOpt.VolumeName)
	m.SetDeviceName(m.MountOpt.DeviceName)

	// Start background task if --daemon is specified
	if m.MountOpt.Daemon {
		daemon, err = daemonize.StartDaemon(os.Args)
		if daemon != nil || err != nil {
			return daemon, err
		}
	}

	m.VFS = vfs.New(m.Fs, &m.VFSOpt)

	m.ErrChan, m.UnmountFn, err = m.MountFn(m.VFS, m.MountPoint, &m.MountOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to mount FUSE fs: %w", err)
	}
	m.MountedOn = time.Now()
	return nil, nil
}

// Wait for mount end
func (m *MountPoint) Wait() error {
	// Unmount on exit
	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			_ = sysdnotify.Stopping()
			// Unmount only if directory was mounted by rclone, e.g. don't unmount autofs hooks.
			if err := CheckMountReady(m.MountPoint); err != nil {
				fs.Debugf(m.MountPoint, "Unmounted externally. Just exit now.")
				return
			}
			if err := m.Unmount(); err != nil {
				fs.Errorf(m.MountPoint, "Failed to unmount: %v", err)
			} else {
				fs.Errorf(m.MountPoint, "Unmounted rclone mount")
			}
		})
	}
	fnHandle := atexit.Register(finalise)
	defer atexit.Unregister(fnHandle)

	// Notify systemd
	if err := sysdnotify.Ready(); err != nil {
		return fmt.Errorf("failed to notify systemd: %w", err)
	}

	// Reload VFS cache on SIGHUP
	sigHup := make(chan os.Signal, 1)
	NotifyOnSigHup(sigHup)
	var err error

	waiting := true
	for waiting {
		select {
		// umount triggered outside the app
		case err = <-m.ErrChan:
			waiting = false
		// user sent SIGHUP to clear the cache
		case <-sigHup:
			root, err := m.VFS.Root()
			if err != nil {
				fs.Errorf(m.VFS.Fs(), "Error reading root: %v", err)
			} else {
				root.ForgetAll()
			}
		}
	}

	finalise()

	if err != nil {
		return fmt.Errorf("failed to umount FUSE fs: %w", err)
	}
	return nil
}

// Unmount the specified mountpoint
func (m *MountPoint) Unmount() (err error) {
	return m.UnmountFn()
}
