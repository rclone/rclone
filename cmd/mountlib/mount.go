// Package mountlib provides the mount command.
package mountlib

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/daemonize"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//go:embed mount.md
var mountHelp string

// help returns the help string cleaned up to simplify appending
func help(commandName string) string {
	return strings.TrimSpace(strings.ReplaceAll(mountHelp, "@", commandName)) + "\n\n"
}

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "debug_fuse",
	Default: false,
	Help:    "Debug the FUSE internals - needs -v",
	Groups:  "Mount",
}, {
	Name:    "attr_timeout",
	Default: fs.Duration(1 * time.Second),
	Help:    "Time for which file/directory attributes are cached",
	Groups:  "Mount",
}, {
	Name:     "option",
	Default:  []string{},
	Help:     "Option for libfuse/WinFsp (repeat if required)",
	Groups:   "Mount",
	ShortOpt: "o",
}, {
	Name:    "fuse_flag",
	Default: []string{},
	Help:    "Flags or arguments to be passed direct to libfuse/WinFsp (repeat if required)",
	Groups:  "Mount",
}, {
	Name:    "daemon",
	Default: false,
	Help:    "Run mount in background and exit parent process (as background output is suppressed, use --log-file with --log-format=pid,... to monitor) (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name: "daemon_timeout",
	Default: func() fs.Duration {
		if runtime.GOOS == "darwin" {
			// DaemonTimeout defaults to non-zero for macOS
			// (this is a macOS specific kernel option unrelated to DaemonWait)
			return fs.Duration(10 * time.Minute)
		}
		return 0
	}(),
	Help:   "Time limit for rclone to respond to kernel (not supported on Windows)",
	Groups: "Mount",
}, {
	Name:    "default_permissions",
	Default: false,
	Help:    "Makes kernel enforce access control based on the file mode (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "allow_non_empty",
	Default: false,
	Help:    "Allow mounting over a non-empty directory (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "allow_root",
	Default: false,
	Help:    "Allow access to root user (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "allow_other",
	Default: false,
	Help:    "Allow access to other users (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "async_read",
	Default: true,
	Help:    "Use asynchronous reads (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "max_read_ahead",
	Default: fs.SizeSuffix(128 * 1024),
	Help:    "The number of bytes that can be prefetched for sequential reads (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "write_back_cache",
	Default: false,
	Help:    "Makes kernel buffer writes before sending them to rclone (without this, writethrough caching is used) (not supported on Windows)",
	Groups:  "Mount",
}, {
	Name:    "devname",
	Default: "",
	Help:    "Set the device name - default is remote:path",
	Groups:  "Mount",
}, {
	Name:    "mount_case_insensitive",
	Default: fs.Tristate{},
	Help:    "Tell the OS the mount is case insensitive (true) or sensitive (false) regardless of the backend (auto)",
	Groups:  "Mount",
}, {
	Name:    "direct_io",
	Default: false,
	Help:    "Use Direct IO, disables caching of data",
	Groups:  "Mount",
}, {
	Name:    "volname",
	Default: "",
	Help:    "Set the volume name (supported on Windows and OSX only)",
	Groups:  "Mount",
}, {
	Name:    "noappledouble",
	Default: true,
	Help:    "Ignore Apple Double (._) and .DS_Store files (supported on OSX only)",
	Groups:  "Mount",
}, {
	Name:    "noapplexattr",
	Default: false,
	Help:    "Ignore all \"com.apple.*\" extended attributes (supported on OSX only)",
	Groups:  "Mount",
}, {
	Name:    "network_mode",
	Default: false,
	Help:    "Mount as remote network drive, instead of fixed disk drive (supported on Windows only)",
	Groups:  "Mount",
}, {
	Name: "daemon_wait",
	Default: func() fs.Duration {
		switch runtime.GOOS {
		case "linux":
			// Linux provides /proc/mounts to check mount status
			// so --daemon-wait means *maximum* time to wait
			return fs.Duration(60 * time.Second)
		case "darwin", "openbsd", "freebsd", "netbsd":
			// On BSD we can't check mount status yet
			// so --daemon-wait is just a *constant* delay
			return fs.Duration(5 * time.Second)
		}
		return 0
	}(),
	Help:   "Time to wait for ready mount from daemon (maximum time on Linux, constant sleep time on OSX/BSD) (not supported on Windows)",
	Groups: "Mount",
}}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "mount", Opt: &Opt, Options: OptionsInfo})
}

// Options for creating the mount
type Options struct {
	DebugFUSE          bool          `config:"debug_fuse"`
	AllowNonEmpty      bool          `config:"allow_non_empty"`
	AllowRoot          bool          `config:"allow_root"`
	AllowOther         bool          `config:"allow_other"`
	DefaultPermissions bool          `config:"default_permissions"`
	WritebackCache     bool          `config:"write_back_cache"`
	Daemon             bool          `config:"daemon"`
	DaemonWait         fs.Duration   `config:"daemon_wait"` // time to wait for ready mount from daemon, maximum on Linux or constant on macOS/BSD
	MaxReadAhead       fs.SizeSuffix `config:"max_read_ahead"`
	ExtraOptions       []string      `config:"option"`
	ExtraFlags         []string      `config:"fuse_flag"`
	AttrTimeout        fs.Duration   `config:"attr_timeout"` // how long the kernel caches attribute for
	DeviceName         string        `config:"devname"`
	VolumeName         string        `config:"volname"`
	NoAppleDouble      bool          `config:"noappledouble"`
	NoAppleXattr       bool          `config:"noapplexattr"`
	DaemonTimeout      fs.Duration   `config:"daemon_timeout"` // OSXFUSE only
	AsyncRead          bool          `config:"async_read"`
	NetworkMode        bool          `config:"network_mode"` // Windows only
	DirectIO           bool          `config:"direct_io"`    // use Direct IO for file access
	CaseInsensitive    fs.Tristate   `config:"mount_case_insensitive"`
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

// Opt contains options set by command line flags
var Opt Options

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}

const (
	pollInterval = 100 * time.Millisecond
)

// WaitMountReady waits until mountpoint is mounted by rclone.
//
// If the mount daemon dies prematurely it will notice too.
func WaitMountReady(mountpoint string, timeout time.Duration, daemon *os.Process) (err error) {
	endTime := time.Now().Add(timeout)
	for {
		if CanCheckMountReady {
			err = CheckMountReady(mountpoint)
			if err == nil {
				break
			}
		}
		err = daemonize.Check(daemon)
		if err != nil {
			return err
		}
		delay := time.Until(endTime)
		if delay <= 0 {
			break
		}
		if delay > pollInterval {
			delay = pollInterval
		}
		time.Sleep(delay)
	}
	return
}

// NewMountCommand makes a mount command with the given name and Mount function
func NewMountCommand(commandName string, hidden bool, mount MountFn) *cobra.Command {
	var commandDefinition = &cobra.Command{
		Use:    commandName + " remote:path /path/to/mountpoint",
		Hidden: hidden,
		Short:  `Mount the remote as file system on a mountpoint.`,
		Long:   help(commandName) + vfs.Help(),
		Annotations: map[string]string{
			"versionIntroduced": "v1.33",
			"groups":            "Filter",
		},
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

			mnt := NewMountPoint(mount, args[1], cmd.NewFsDir(args), &Opt, &vfscommon.Opt)
			mountDaemon, err := mnt.Mount()

			// Wait for foreground mount, if any...
			if mountDaemon == nil {
				if err == nil {
					defer systemd.Notify()()
					err = mnt.Wait()
				}
				if err != nil {
					fs.Fatalf(nil, "Fatal error: %v", err)
				}
				return
			}

			// Wait for mountDaemon, if any...
			killOnce := sync.Once{}
			killDaemon := func(reason string) {
				killOnce.Do(func() {
					if err := mountDaemon.Signal(os.Interrupt); err != nil {
						fs.Errorf(nil, "%s. Failed to terminate daemon pid %d: %v", reason, mountDaemon.Pid, err)
						return
					}
					fs.Debugf(nil, "%s. Terminating daemon pid %d", reason, mountDaemon.Pid)
				})
			}

			if err == nil && Opt.DaemonWait > 0 {
				handle := atexit.Register(func() {
					killDaemon("Got interrupt")
				})
				err = WaitMountReady(mnt.MountPoint, time.Duration(Opt.DaemonWait), mountDaemon)
				if err != nil {
					killDaemon("Daemon timed out")
				}
				atexit.Unregister(handle)
			}
			if err != nil {
				fs.Fatalf(nil, "Fatal error: %v", err)
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
func (m *MountPoint) Mount() (mountDaemon *os.Process, err error) {

	// Ensure sensible defaults
	m.SetVolumeName(m.MountOpt.VolumeName)
	m.SetDeviceName(m.MountOpt.DeviceName)

	// Start background task if --daemon is specified
	if m.MountOpt.Daemon {
		mountDaemon, err = daemonize.StartDaemon(os.Args)
		if mountDaemon != nil || err != nil {
			return mountDaemon, err
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
