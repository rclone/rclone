package docker

import (
	"fmt"
	"strings"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
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
	mntMap := configmap.Simple{}
	vfsMap := configmap.Simple{}
	for key := range opt {
		var ok bool
		var err error
		normalKey := normalOptName(key)
		underscoreKey := strings.ReplaceAll(normalKey, "-", "_")

		switch normalKey {
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
			// try to use as a mount option in mntMap
			if mountlib.OptionsInfo.Get(underscoreKey) != nil {
				mntMap[underscoreKey] = vol.Options[key]
				ok = true
			}
		}
		if !ok {
			// try as a vfs option in vfsMap
			if vfscommon.OptionsInfo.Get(underscoreKey) != nil {
				vfsMap[underscoreKey] = vol.Options[key]
				ok = true
			}
		}

		if !ok {
			// try as a backend option in fsOpt (backends use "_" instead of "-")
			fsOptName := strings.TrimPrefix(underscoreKey, fsType+"_")
			hasFsPrefix := underscoreKey != fsOptName
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

	// Parse VFS options
	err = configstruct.Set(vfsMap, vfsOpt)
	if err != nil {
		return fmt.Errorf("cannot parse vfs options: %w", err)
	}

	// Parse Mount options
	err = configstruct.Set(mntMap, mntOpt)
	if err != nil {
		return fmt.Errorf("cannot parse mount options: %w", err)
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

func normalOptName(key string) string {
	return strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(key), "--"), "_", "-")
}
