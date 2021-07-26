package mountlib

import (
	"log"
	"os"
	"path/filepath"
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
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"

	sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
	"github.com/pkg/errors"
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
	MaxReadAhead       fs.SizeSuffix
	ExtraOptions       []string
	ExtraFlags         []string
	AttrTimeout        time.Duration // how long the kernel caches attribute for
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

// Global constants
const (
	MaxLeafSize = 1024 // don't pass file names longer than this
)

func init() {
	// DaemonTimeout defaults to non zero for macOS
	if runtime.GOOS == "darwin" {
		DefaultOpt.DaemonTimeout = 10 * time.Minute
	}
}

// Options set by command line flags
var (
	Opt = DefaultOpt
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("mount", &Opt)
	flags.BoolVarP(flagSet, &Opt.DebugFUSE, "debug-fuse", "", Opt.DebugFUSE, "Debug the FUSE internals - needs -v.")
	flags.DurationVarP(flagSet, &Opt.AttrTimeout, "attr-timeout", "", Opt.AttrTimeout, "Time for which file/directory attributes are cached.")
	flags.StringArrayVarP(flagSet, &Opt.ExtraOptions, "option", "o", []string{}, "Option for libfuse/WinFsp. Repeat if required.")
	flags.StringArrayVarP(flagSet, &Opt.ExtraFlags, "fuse-flag", "", []string{}, "Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.")
	// Non-Windows only
	flags.BoolVarP(flagSet, &Opt.Daemon, "daemon", "", Opt.Daemon, "Run mount as a daemon (background mode). Not supported on Windows.")
	flags.DurationVarP(flagSet, &Opt.DaemonTimeout, "daemon-timeout", "", Opt.DaemonTimeout, "Time limit for rclone to respond to kernel. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.DefaultPermissions, "default-permissions", "", Opt.DefaultPermissions, "Makes kernel enforce access control based on the file mode. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AllowNonEmpty, "allow-non-empty", "", Opt.AllowNonEmpty, "Allow mounting over a non-empty directory. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AllowRoot, "allow-root", "", Opt.AllowRoot, "Allow access to root user. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AllowOther, "allow-other", "", Opt.AllowOther, "Allow access to other users. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.AsyncRead, "async-read", "", Opt.AsyncRead, "Use asynchronous reads. Not supported on Windows.")
	flags.FVarP(flagSet, &Opt.MaxReadAhead, "max-read-ahead", "", "The number of bytes that can be prefetched for sequential reads. Not supported on Windows.")
	flags.BoolVarP(flagSet, &Opt.WritebackCache, "write-back-cache", "", Opt.WritebackCache, "Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used. Not supported on Windows.")
	// Windows and OSX
	flags.StringVarP(flagSet, &Opt.VolumeName, "volname", "", Opt.VolumeName, "Set the volume name. Supported on Windows and OSX only.")
	// OSX only
	flags.BoolVarP(flagSet, &Opt.NoAppleDouble, "noappledouble", "", Opt.NoAppleDouble, "Ignore Apple Double (._) and .DS_Store files. Supported on OSX only.")
	flags.BoolVarP(flagSet, &Opt.NoAppleXattr, "noapplexattr", "", Opt.NoAppleXattr, "Ignore all \"com.apple.*\" extended attributes. Supported on OSX only.")
	// Windows only
	flags.BoolVarP(flagSet, &Opt.NetworkMode, "network-mode", "", Opt.NetworkMode, "Mount as remote network drive, instead of fixed disk drive. Supported on Windows only")
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

			if Opt.Daemon {
				config.PassConfigKeyForDaemonization = true
			}

			// Show stats if the user has specifically requested them
			if cmd.ShowStats() {
				defer cmd.StartStats()()
			}

			mnt := &MountPoint{
				MountFn:    mount,
				MountPoint: args[1],
				Fs:         cmd.NewFsDir(args),
				MountOpt:   Opt,
				VFSOpt:     vfsflags.Opt,
			}

			daemonized, err := mnt.Mount()
			if !daemonized && err == nil {
				err = mnt.Wait()
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
func (m *MountPoint) Mount() (daemonized bool, err error) {
	if err = m.CheckOverlap(); err != nil {
		return false, err
	}

	if err = m.CheckAllowings(); err != nil {
		return false, err
	}
	m.SetVolumeName(m.MountOpt.VolumeName)

	// Start background task if --daemon is specified
	if m.MountOpt.Daemon {
		daemonized = startBackgroundMode()
		if daemonized {
			return true, nil
		}
	}

	m.VFS = vfs.New(m.Fs, &m.VFSOpt)

	m.ErrChan, m.UnmountFn, err = m.MountFn(m.VFS, m.MountPoint, &m.MountOpt)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount FUSE fs")
	}
	return false, nil
}

// CheckOverlap checks that root doesn't overlap with mountpoint
func (m *MountPoint) CheckOverlap() error {
	name := m.Fs.Name()
	if name != "" && name != "local" {
		return nil
	}
	rootAbs := absPath(m.Fs.Root())
	mountpointAbs := absPath(m.MountPoint)
	if strings.HasPrefix(rootAbs, mountpointAbs) || strings.HasPrefix(mountpointAbs, rootAbs) {
		const msg = "mount point %q and directory to be mounted %q mustn't overlap"
		return errors.Errorf(msg, m.MountPoint, m.Fs.Root())
	}
	return nil
}

// absPath is a helper function for MountPoint.CheckOverlap
func absPath(path string) string {
	if abs, err := filepath.EvalSymlinks(path); err == nil {
		path = abs
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.ToSlash(path)
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

// CheckAllowings informs about ignored flags on Windows. If not on Windows
// and not --allow-non-empty flag is used, verify that mountpoint is empty.
func (m *MountPoint) CheckAllowings() error {
	opt := &m.MountOpt
	if runtime.GOOS == "windows" {
		if opt.AllowNonEmpty {
			fs.Logf(nil, "--allow-non-empty flag does nothing on Windows")
		}
		if opt.AllowRoot {
			fs.Logf(nil, "--allow-root flag does nothing on Windows")
		}
		if opt.AllowOther {
			fs.Logf(nil, "--allow-other flag does nothing on Windows")
		}
		return nil
	}
	if !opt.AllowNonEmpty {
		return CheckMountEmpty(m.MountPoint)
	}
	return nil
}

// Wait for mount end
func (m *MountPoint) Wait() error {
	// Unmount on exit
	var finaliseOnce sync.Once
	finalise := func() {
		finaliseOnce.Do(func() {
			_ = sysdnotify.Stopping()
			_ = m.UnmountFn()
		})
	}
	fnHandle := atexit.Register(finalise)
	defer atexit.Unregister(fnHandle)

	// Notify systemd
	if err := sysdnotify.Ready(); err != nil {
		return errors.Wrap(err, "failed to notify systemd")
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
		return errors.Wrap(err, "failed to umount FUSE fs")
	}
	return nil
}

// Unmount the specified mountpoint
func (m *MountPoint) Unmount() (err error) {
	return m.UnmountFn()
}

// SetVolumeName with sensible default
func (m *MountPoint) SetVolumeName(vol string) {
	if vol == "" {
		vol = m.Fs.Name() + ":" + m.Fs.Root()
	}
	m.MountOpt.SetVolumeName(vol)
}

// SetVolumeName removes special characters from volume name if necessary
func (opt *Options) SetVolumeName(vol string) {
	vol = strings.ReplaceAll(vol, ":", " ")
	vol = strings.ReplaceAll(vol, "/", " ")
	vol = strings.TrimSpace(vol)
	if runtime.GOOS == "windows" && len(vol) > 32 {
		vol = vol[:32]
	}
	opt.VolumeName = vol
}
