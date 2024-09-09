package docker

import (
	"fmt"
	"math"
	"strings"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"

	"github.com/spf13/pflag"
)

// applyOptions configures volume from request options.
//
// There are 5 special options:
//   - "remote" aka "fs" determines existing remote from config file
//     with a path or on-the-fly remote using the ":backend:" syntax.
//     It is usually named "remote" in documentation but can be aliased as
//     "fs" to avoid confusion with the "remote" option of some backends.
//   - "type" is equivalent to the ":backend:" syntax (optional).
//   - "path" provides explicit on-remote path for "type" (optional).
//   - "mount-type" can be "mount", "cmount" or "mount2", defaults to
//     first found (optional).
//   - "persist" is reserved for future to create remotes persisted
//     in rclone.conf similar to rcd (optional).
//
// Unlike rcd we use the flat naming scheme for mount, vfs and backend
// options without substructures. Dashes, underscores and mixed case
// in option names can be used interchangeably. Option name conflicts
// can be resolved in a manner similar to rclone CLI by adding prefixes:
// "vfs-", primary mount backend type like "sftp-", and so on.
//
// After triaging the options are put in MountOpt, VFSOpt or connect
// string for actual filesystem setup and in volume.Options for saving
// the state.
func (vol *Volume) applyOptions(volOpt VolOpts) error {
	// copy options to override later
	mntOpt := &vol.mnt.MountOpt
	vfsOpt := &vol.mnt.VFSOpt
	*mntOpt = vol.drv.mntOpt
	*vfsOpt = vol.drv.vfsOpt

	// vol.Options has all options except "remote" and "type"
	vol.Options = VolOpts{}
	vol.fsString = ""

	var fsName, fsPath, fsType string
	var explicitPath string
	var fsOpt configmap.Simple

	// parse "remote" or "type"
	for key, str := range volOpt {
		switch key {
		case "":
			continue
		case "remote", "fs":
			if str != "" {
				p, err := fspath.Parse(str)
				if err != nil || p.Name == ":" {
					return fmt.Errorf("cannot parse path %q: %w", str, err)
				}
				fsName, fsPath, fsOpt = p.Name, p.Path, p.Config
				vol.Fs = str
			}
		case "type":
			fsType = str
			vol.Type = str
		case "path":
			explicitPath = str
			vol.Path = str
		default:
			vol.Options[key] = str
		}
	}

	// find options supported by backend
	if strings.HasPrefix(fsName, ":") {
		fsType = fsName[1:]
		fsName = ""
	}
	if fsType == "" {
		fsType = "local"
		if fsName != "" {
			var ok bool
			fsType, ok = fs.ConfigMap("", nil, fsName, nil).Get("type")
			if !ok {
				return fs.ErrorNotFoundInConfigFile
			}
		}
	}
	if explicitPath != "" {
		if fsPath != "" {
			fs.Logf(nil, "Explicit path will override connection string")
		}
		fsPath = explicitPath
	}
	fsInfo, err := fs.Find(fsType)
	if err != nil {
		return fmt.Errorf("unknown filesystem type %q", fsType)
	}

	// handle remaining options, override fsOpt
	if fsOpt == nil {
		fsOpt = configmap.Simple{}
	}
	opt := rc.Params{}
	for key, val := range vol.Options {
		opt[key] = val
	}
	for key := range opt {
		var ok bool
		var err error

		switch normalOptName(key) {
		case "persist":
			vol.persist, err = opt.GetBool(key)
			ok = true
		case "mount-type":
			vol.mountType, err = opt.GetString(key)
			ok = true
		}
		if err != nil {
			return fmt.Errorf("cannot parse option %q: %w", key, err)
		}

		if !ok {
			// try to use as a mount option in mntOpt
			ok, err = getMountOption(mntOpt, opt, key)
			if ok && err != nil {
				return fmt.Errorf("cannot parse mount option %q: %w", key, err)
			}
		}
		if !ok {
			// try as a vfs option in vfsOpt
			ok, err = getVFSOption(vfsOpt, opt, key)
			if ok && err != nil {
				return fmt.Errorf("cannot parse vfs option %q: %w", key, err)
			}
		}

		if !ok {
			// try as a backend option in fsOpt (backends use "_" instead of "-")
			optWithPrefix := strings.ReplaceAll(normalOptName(key), "-", "_")
			fsOptName := strings.TrimPrefix(optWithPrefix, fsType+"_")
			hasFsPrefix := optWithPrefix != fsOptName
			if !hasFsPrefix || fsInfo.Options.Get(fsOptName) == nil {
				fs.Logf(nil, "Option %q is not supported by backend %q", key, fsType)
				return fmt.Errorf("unsupported backend option %q", key)
			}
			fsOpt[fsOptName], err = opt.GetString(key)
			if err != nil {
				return fmt.Errorf("cannot parse backend option %q: %w", key, err)
			}
		}
	}

	// build remote string from fsName, fsType, fsOpt, fsPath
	colon := ":"
	comma := ","
	if fsName == "" {
		fsName = ":" + fsType
	}
	connString := fsOpt.String()
	if fsName == "" && fsType == "" {
		colon = ""
		connString = ""
	}
	if connString == "" {
		comma = ""
	}
	vol.fsString = fsName + comma + connString + colon + fsPath

	return vol.validate()
}

func getMountOption(mntOpt *mountlib.Options, opt rc.Params, key string) (ok bool, err error) {
	ok = true
	switch normalOptName(key) {
	case "debug-fuse":
		mntOpt.DebugFUSE, err = opt.GetBool(key)
	case "attr-timeout":
		mntOpt.AttrTimeout, err = opt.GetFsDuration(key)
	case "option":
		mntOpt.ExtraOptions, err = getStringArray(opt, key)
	case "fuse-flag":
		mntOpt.ExtraFlags, err = getStringArray(opt, key)
	case "daemon":
		mntOpt.Daemon, err = opt.GetBool(key)
	case "daemon-timeout":
		mntOpt.DaemonTimeout, err = opt.GetFsDuration(key)
	case "default-permissions":
		mntOpt.DefaultPermissions, err = opt.GetBool(key)
	case "allow-non-empty":
		mntOpt.AllowNonEmpty, err = opt.GetBool(key)
	case "allow-root":
		mntOpt.AllowRoot, err = opt.GetBool(key)
	case "allow-other":
		mntOpt.AllowOther, err = opt.GetBool(key)
	case "async-read":
		mntOpt.AsyncRead, err = opt.GetBool(key)
	case "max-read-ahead":
		err = getFVarP(&mntOpt.MaxReadAhead, opt, key)
	case "write-back-cache":
		mntOpt.WritebackCache, err = opt.GetBool(key)
	case "volname":
		mntOpt.VolumeName, err = opt.GetString(key)
	case "noappledouble":
		mntOpt.NoAppleDouble, err = opt.GetBool(key)
	case "noapplexattr":
		mntOpt.NoAppleXattr, err = opt.GetBool(key)
	case "network-mode":
		mntOpt.NetworkMode, err = opt.GetBool(key)
	default:
		ok = false
	}
	return
}

func getVFSOption(vfsOpt *vfscommon.Options, opt rc.Params, key string) (ok bool, err error) {
	var intVal int64
	ok = true
	switch normalOptName(key) {

	// options prefixed with "vfs-"
	case "vfs-cache-mode":
		err = getFVarP(&vfsOpt.CacheMode, opt, key)
	case "vfs-cache-poll-interval":
		vfsOpt.CachePollInterval, err = opt.GetFsDuration(key)
	case "vfs-cache-max-age":
		vfsOpt.CacheMaxAge, err = opt.GetFsDuration(key)
	case "vfs-cache-max-size":
		err = getFVarP(&vfsOpt.CacheMaxSize, opt, key)
	case "vfs-read-chunk-size":
		err = getFVarP(&vfsOpt.ChunkSize, opt, key)
	case "vfs-read-chunk-size-limit":
		err = getFVarP(&vfsOpt.ChunkSizeLimit, opt, key)
	case "vfs-case-insensitive":
		vfsOpt.CaseInsensitive, err = opt.GetBool(key)
	case "vfs-write-wait":
		vfsOpt.WriteWait, err = opt.GetFsDuration(key)
	case "vfs-read-wait":
		vfsOpt.ReadWait, err = opt.GetFsDuration(key)
	case "vfs-write-back":
		vfsOpt.WriteBack, err = opt.GetFsDuration(key)
	case "vfs-read-ahead":
		err = getFVarP(&vfsOpt.ReadAhead, opt, key)
	case "vfs-used-is-size":
		vfsOpt.UsedIsSize, err = opt.GetBool(key)
	case "vfs-read-chunk-streams":
		intVal, err = opt.GetInt64(key)
		if err == nil {
			if intVal >= 0 && intVal <= math.MaxInt {
				vfsOpt.ChunkStreams = int(intVal)
			} else {
				err = fmt.Errorf("key %q (%v) overflows int", key, intVal)
			}
		}

	// unprefixed vfs options
	case "no-modtime":
		vfsOpt.NoModTime, err = opt.GetBool(key)
	case "no-checksum":
		vfsOpt.NoChecksum, err = opt.GetBool(key)
	case "dir-cache-time":
		vfsOpt.DirCacheTime, err = opt.GetFsDuration(key)
	case "poll-interval":
		vfsOpt.PollInterval, err = opt.GetFsDuration(key)
	case "read-only":
		vfsOpt.ReadOnly, err = opt.GetBool(key)
	case "dir-perms":
		err = getFVarP(&vfsOpt.DirPerms, opt, key)
	case "file-perms":
		err = getFVarP(&vfsOpt.FilePerms, opt, key)

	// unprefixed unix-only vfs options
	case "umask":
		err = getFVarP(&vfsOpt.Umask, opt, key)
	case "uid":
		intVal, err = opt.GetInt64(key)
		if err == nil {
			if intVal >= 0 && intVal <= math.MaxUint32 {
				vfsOpt.UID = uint32(intVal)
			} else {
				err = fmt.Errorf("key %q (%v) overflows uint32", key, intVal)
			}
		}
	case "gid":
		intVal, err = opt.GetInt64(key)
		if err == nil {
			if intVal >= 0 && intVal <= math.MaxUint32 {
				vfsOpt.UID = uint32(intVal)
			} else {
				err = fmt.Errorf("key %q (%v) overflows uint32", key, intVal)
			}
		}

	// non-vfs options
	default:
		ok = false
	}
	return
}

func getFVarP(pvalue pflag.Value, opt rc.Params, key string) error {
	str, err := opt.GetString(key)
	if err != nil {
		return err
	}
	return pvalue.Set(str)
}

func getStringArray(opt rc.Params, key string) ([]string, error) {
	str, err := opt.GetString(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(str, ","), nil
}

func normalOptName(key string) string {
	return strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(key), "--"), "_", "-")
}
