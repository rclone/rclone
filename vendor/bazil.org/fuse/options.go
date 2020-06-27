package fuse

import (
	"strings"
)

func dummyOption(conf *mountConfig) error {
	return nil
}

// mountConfig holds the configuration for a mount operation.
// Use it by passing MountOption values to Mount.
type mountConfig struct {
	options             map[string]string
	maxReadahead        uint32
	initFlags           InitFlags
	maxBackground       uint16
	congestionThreshold uint16
}

func escapeComma(s string) string {
	s = strings.Replace(s, `\`, `\\`, -1)
	s = strings.Replace(s, `,`, `\,`, -1)
	return s
}

// getOptions makes a string of options suitable for passing to FUSE
// mount flag `-o`. Returns an empty string if no options were set.
// Any platform specific adjustments should happen before the call.
func (m *mountConfig) getOptions() string {
	var opts []string
	for k, v := range m.options {
		k = escapeComma(k)
		if v != "" {
			k += "=" + escapeComma(v)
		}
		opts = append(opts, k)
	}
	return strings.Join(opts, ",")
}

type mountOption func(*mountConfig) error

// MountOption is passed to Mount to change the behavior of the mount.
type MountOption mountOption

// FSName sets the file system name (also called source) that is
// visible in the list of mounted file systems.
//
// FreeBSD ignores this option.
func FSName(name string) MountOption {
	return func(conf *mountConfig) error {
		conf.options["fsname"] = name
		return nil
	}
}

// Subtype sets the subtype of the mount. The main type is always
// `fuse`. The type in a list of mounted file systems will look like
// `fuse.foo`.
//
// FreeBSD ignores this option.
func Subtype(fstype string) MountOption {
	return func(conf *mountConfig) error {
		conf.options["subtype"] = fstype
		return nil
	}
}

// Deprecated: Ignored, OS X remnant.
func LocalVolume() MountOption {
	return dummyOption
}

// Deprecated: Ignored, OS X remnant.
func VolumeName(name string) MountOption {
	return dummyOption
}

// Deprecated: Ignored, OS X remnant.
func NoAppleDouble() MountOption {
	return dummyOption
}

// Deprecated: Ignored, OS X remnant.
func NoAppleXattr() MountOption {
	return dummyOption
}

// Deprecated: Ignored, OS X remnant.
func NoBrowse() MountOption {
	return dummyOption
}

// Deprecated: Ignored, OS X remnant.
func ExclCreate() MountOption {
	return dummyOption
}

// DaemonTimeout sets the time in seconds between a request and a reply before
// the FUSE mount is declared dead.
//
// FreeBSD only. Others ignore this option.
func DaemonTimeout(name string) MountOption {
	return daemonTimeout(name)
}

// AllowOther allows other users to access the file system.
func AllowOther() MountOption {
	return func(conf *mountConfig) error {
		conf.options["allow_other"] = ""
		return nil
	}
}

// AllowDev enables interpreting character or block special devices on the
// filesystem.
func AllowDev() MountOption {
	return func(conf *mountConfig) error {
		conf.options["dev"] = ""
		return nil
	}
}

// AllowSUID allows set-user-identifier or set-group-identifier bits to take
// effect.
func AllowSUID() MountOption {
	return func(conf *mountConfig) error {
		conf.options["suid"] = ""
		return nil
	}
}

// DefaultPermissions makes the kernel enforce access control based on
// the file mode (as in chmod).
//
// Without this option, the Node itself decides what is and is not
// allowed. This is normally ok because FUSE file systems cannot be
// accessed by other users without AllowOther.
//
// FreeBSD ignores this option.
func DefaultPermissions() MountOption {
	return func(conf *mountConfig) error {
		conf.options["default_permissions"] = ""
		return nil
	}
}

// ReadOnly makes the mount read-only.
func ReadOnly() MountOption {
	return func(conf *mountConfig) error {
		conf.options["ro"] = ""
		return nil
	}
}

// MaxReadahead sets the number of bytes that can be prefetched for
// sequential reads. The kernel can enforce a maximum value lower than
// this.
//
// This setting makes the kernel perform speculative reads that do not
// originate from any client process. This usually tremendously
// improves read performance.
func MaxReadahead(n uint32) MountOption {
	return func(conf *mountConfig) error {
		conf.maxReadahead = n
		return nil
	}
}

// AsyncRead enables multiple outstanding read requests for the same
// handle. Without this, there is at most one request in flight at a
// time.
func AsyncRead() MountOption {
	return func(conf *mountConfig) error {
		conf.initFlags |= InitAsyncRead
		return nil
	}
}

// WritebackCache enables the kernel to buffer writes before sending
// them to the FUSE server. Without this, writethrough caching is
// used.
func WritebackCache() MountOption {
	return func(conf *mountConfig) error {
		conf.initFlags |= InitWritebackCache
		return nil
	}
}

// Deprecated: Not used, OS X remnant.
type OSXFUSEPaths struct {
	// Prefix for the device file. At mount time, an incrementing
	// number is suffixed until a free FUSE device is found.
	DevicePrefix string
	// Path of the load helper, used to load the kernel extension if
	// no device files are found.
	Load string
	// Path of the mount helper, used for the actual mount operation.
	Mount string
	// Environment variable used to pass the path to the executable
	// calling the mount helper.
	DaemonVar string
}

// Deprecated: Not used, OS X remnant.
var (
	OSXFUSELocationV3 = OSXFUSEPaths{
		DevicePrefix: "/dev/osxfuse",
		Load:         "/Library/Filesystems/osxfuse.fs/Contents/Resources/load_osxfuse",
		Mount:        "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse",
		DaemonVar:    "MOUNT_OSXFUSE_DAEMON_PATH",
	}
	OSXFUSELocationV2 = OSXFUSEPaths{
		DevicePrefix: "/dev/osxfuse",
		Load:         "/Library/Filesystems/osxfusefs.fs/Support/load_osxfusefs",
		Mount:        "/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs",
		DaemonVar:    "MOUNT_FUSEFS_DAEMON_PATH",
	}
)

// Deprecated: Ignored, OS X remnant.
func OSXFUSELocations(paths ...OSXFUSEPaths) MountOption {
	return dummyOption
}

// AllowNonEmptyMount allows the mounting over a non-empty directory.
//
// The files in it will be shadowed by the freshly created mount. By
// default these mounts are rejected to prevent accidental covering up
// of data, which could for example prevent automatic backup.
func AllowNonEmptyMount() MountOption {
	return func(conf *mountConfig) error {
		conf.options["nonempty"] = ""
		return nil
	}
}

// MaxBackground sets the maximum number of FUSE requests the kernel
// will submit in the background. Background requests are used when an
// immediate answer is not needed. This may help with request latency.
//
// On Linux, this can be adjusted on the fly with
// /sys/fs/fuse/connections/CONN/max_background
func MaxBackground(n uint16) MountOption {
	return func(conf *mountConfig) error {
		conf.maxBackground = n
		return nil
	}
}

// CongestionThreshold sets the number of outstanding background FUSE
// requests beyond which the kernel considers the filesystem
// congested. This may help with request latency.
//
// On Linux, this can be adjusted on the fly with
// /sys/fs/fuse/connections/CONN/congestion_threshold
func CongestionThreshold(n uint16) MountOption {
	// TODO to test this, we'd have to figure out our connection id
	// and read /sys
	return func(conf *mountConfig) error {
		conf.congestionThreshold = n
		return nil
	}
}

// LockingFlock enables flock-based (BSD) locking. This is mostly
// useful for distributed filesystems with global locking. Without
// this, kernel manages local locking automatically.
func LockingFlock() MountOption {
	return func(conf *mountConfig) error {
		conf.initFlags |= InitFlockLocks
		return nil
	}
}

// LockingPOSIX enables flock-based (BSD) locking. This is mostly
// useful for distributed filesystems with global locking. Without
// this, kernel manages local locking automatically.
//
// Beware POSIX locks are a broken API with unintuitive behavior for
// callers.
func LockingPOSIX() MountOption {
	return func(conf *mountConfig) error {
		conf.initFlags |= InitPOSIXLocks
		return nil
	}
}
