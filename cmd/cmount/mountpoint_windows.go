//go:build cmount && windows
// +build cmount,windows

package cmount

import (
	"fmt"
	"os"
	"errors"
	"path/filepath"
	"regexp"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/file"
)

var isDriveRegex = regexp.MustCompile(`^[a-zA-Z]\:$`)
var isDriveRootPathRegex = regexp.MustCompile(`^[a-zA-Z]\:\\$`)
var isDriveOrRootPathRegex = regexp.MustCompile(`^[a-zA-Z]\:\\?$`)
var isNetworkSharePathRegex = regexp.MustCompile(`^\\\\[^\\]+\\[^\\]`)

// isNetworkSharePath returns true if the given string is a valid network share path,
// in the basic UNC format "\\Server\Share\Path", where the first two path components
// are required ("\\Server\Share", which represents the volume).
// Extended-length UNC format "\\?\UNC\Server\Share\Path" is not considered, as it is
// not supported by cgofuse/winfsp.
// Note: There is a UNCPath function in lib/file, but it refers to any extended-length
// paths using prefix "\\?\", and not necessarily network resource UNC paths.
func isNetworkSharePath(l string) bool {
	return isNetworkSharePathRegex.MatchString(l)
}

// isDrive returns true if given string is a drive letter followed by the volume separator, e.g. "X:".
// This is the format supported by cgofuse/winfsp for mounting as drive.
// Extended-length format "\\?\X:" is not considered, as it is not supported by cgofuse/winfsp.
func isDrive(l string) bool {
	return isDriveRegex.MatchString(l)
}

// isDriveRootPath returns true if given string is a drive letter followed by the volume separator,
// as well as a path separator, e.g. "X:\". This is a format often used instead of the format without the
// trailing path separator to denote a drive or volume, in addition to representing the drive's root directory.
// This format is not accepted by cgofuse/winfsp for mounting as drive, but can easily be by trimming off
// the path separator. Extended-length format "\\?\X:\" is not considered.
func isDriveRootPath(l string) bool {
	return isDriveRootPathRegex.MatchString(l)
}

// isDriveOrRootPath returns true if given string is a drive letter followed by the volume separator,
// and optionally a path separator. See isDrive and isDriveRootPath functions.
func isDriveOrRootPath(l string) bool {
	return isDriveOrRootPathRegex.MatchString(l)
}

// isDefaultPath returns true if given string is a special keyword used to trigger default mount.
func isDefaultPath(l string) bool {
	return l == "" || l == "*"
}

// getUnusedDrive find unused drive letter and returns string with drive letter followed by volume separator.
func getUnusedDrive() (string, error) {
	driveLetter := file.FindUnusedDriveLetter()
	if driveLetter == 0 {
		return "", errors.New("could not find unused drive letter")
	}
	mountpoint := string(driveLetter) + ":" // Drive letter with volume separator only, no trailing backslash, which is what cgofuse/winfsp expects
	fs.Logf(nil, "Assigning drive letter %q", mountpoint)
	return mountpoint, nil
}

// handleDefaultMountpath handles the case where mount path is not set, or set to a special keyword.
// This will automatically pick an unused drive letter to use as mountpoint.
func handleDefaultMountpath() (string, error) {
	return getUnusedDrive()
}

// handleNetworkShareMountpath handles the case where mount path is a network share path.
// Sets volume name option and returns a mountpoint string.
func handleNetworkShareMountpath(mountpath string, opt *mountlib.Options) (string, error) {
	// Assuming mount path is a valid network share path (UNC format, "\\Server\Share").
	// Always mount as network drive, regardless of the NetworkMode option.
	// Find an unused drive letter to use as mountpoint, the the supplied path can
	// be used as volume prefix (network share path) instead of mountpoint.
	if !opt.NetworkMode {
		fs.Debugf(nil, "Forcing --network-mode because mountpoint path is network share UNC format")
		opt.NetworkMode = true
	}
	mountpoint, err := getUnusedDrive()
	if err != nil {
		return "", err
	}
	return mountpoint, nil
}

// handleLocalMountpath handles the case where mount path is a local file system path.
func handleLocalMountpath(mountpath string, opt *mountlib.Options) (string, error) {
	// Assuming path is drive letter or directory path, not network share (UNC) path.
	// If drive letter: Must be given as a single character followed by ":" and nothing else.
	// Else, assume directory path: Directory must not exist, but its parent must.
	if _, err := os.Stat(mountpath); err == nil {
		return "", errors.New("mountpoint path already exists: " + mountpath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to retrieve mountpoint path information: %w", err)
	}
	if isDriveRootPath(mountpath) { // Assume intention with "X:\" was "X:"
		mountpath = mountpath[:len(mountpath)-1] // WinFsp needs drive mountpoints without trailing path separator
	}
	if !isDrive(mountpath) {
		// Assuming directory path, since it is not a pure drive letter string such as "X:".
		// Drive letter string can be used as is, since we have already checked it does not exist,
		// but directory path needs more checks.
		if opt.NetworkMode {
			fs.Errorf(nil, "Ignoring --network-mode as it is not supported with directory mountpoint")
			opt.NetworkMode = false
		}
		var err error
		if mountpath, err = filepath.Abs(mountpath); err != nil { // Ensures parent is found but also more informative log messages
			return "", fmt.Errorf("mountpoint path is not valid: %s: %w", mountpath, err)
		}
		parent := filepath.Join(mountpath, "..")
		if _, err = os.Stat(parent); err != nil {
			if os.IsNotExist(err) {
				return "", errors.New("parent of mountpoint directory does not exist: " + parent)
			}
			return "", fmt.Errorf("failed to retrieve mountpoint directory parent information: %w", err)
		}
	}
	return mountpath, nil
}

// handleVolumeName handles the volume name option.
func handleVolumeName(opt *mountlib.Options, volumeName string) {
	// If volumeName parameter is set, then just set that into options replacing any existing value.
	// Else, ensure the volume name option is a valid network share UNC path if network mode,
	// and ensure network mode if configured volume name is already UNC path.
	if volumeName != "" {
		opt.VolumeName = volumeName
	} else if opt.VolumeName != "" { // Should always be true due to code in mountlib caller
		// Use value of given volume name option, but check if it is disk volume name or network volume prefix
		if isNetworkSharePath(opt.VolumeName) {
			// Specified volume name is network share UNC path, assume network mode and use it as volume prefix
			opt.VolumeName = opt.VolumeName[1:] // WinFsp requires volume prefix as UNC-like path but with only a single backslash
			if !opt.NetworkMode {
				// Specified volume name is network share UNC path, force network mode and use it as volume prefix
				fs.Debugf(nil, "Forcing network mode due to network share (UNC) volume name")
				opt.NetworkMode = true
			}
		} else if opt.NetworkMode {
			// Plain volume name treated as share name in network mode, append to hard coded "\\server" prefix to get full volume prefix.
			opt.VolumeName = "\\server\\" + opt.VolumeName
		}
	} else if opt.NetworkMode {
		// Hard coded default
		opt.VolumeName = "\\server\\share"
	}
}

// getMountpoint handles mounting details on Windows,
// where disk and network based file systems are treated different.
func getMountpoint(mountpath string, opt *mountlib.Options) (mountpoint string, err error) {

	// First handle mountpath
	var volumeName string
	if isDefaultPath(mountpath) {
		// Mount path indicates defaults, which will automatically pick an unused drive letter.
		mountpoint, err = handleDefaultMountpath()
	} else if isNetworkSharePath(mountpath) {
		// Mount path is a valid network share path (UNC format, "\\Server\Share" prefix).
		mountpoint, err = handleNetworkShareMountpath(mountpath, opt)
		// In this case the volume name is taken from the mount path, will replace any existing volume name option.
		volumeName = mountpath[1:] // WinFsp requires volume prefix as UNC-like path but with only a single backslash
	} else {
		// Mount path is drive letter or directory path.
		mountpoint, err = handleLocalMountpath(mountpath, opt)
	}

	// Second handle volume name
	handleVolumeName(opt, volumeName)

	// Done, return mountpoint to be used, together with updated mount options.
	if opt.NetworkMode {
		fs.Debugf(nil, "Network mode mounting is enabled")
	} else {
		fs.Debugf(nil, "Network mode mounting is disabled")
	}
	return
}
