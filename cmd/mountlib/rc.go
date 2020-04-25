package mountlib

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

var (
	// Mount functions available
	mountFns map[string]MountFn
	// Map of mounted path => unmount function
	unmountFns map[string]UnmountFn
)

func init() {
	rc.Add(rc.Call{
		Path:         "mount/mount",
		AuthRequired: true,
		Fn:           mountRc,
		Title:        "Create a new mount point",
		Help: `rclone allows Linux, FreeBSD, macOS and Windows to mount any of
Rclone's cloud storage systems as a file system with FUSE.

If no mountOption is provided, the priority is given as follows: 1. mount 2.cmount 3.mount2

This takes the following parameters

- fs - a remote path to be mounted (required)
- mountPoint: valid path on the local machine (required)
- mountOption: One of the values (mount, cmount, mount2) specifies the mount implementation to use

Eg

    rclone rc mount/mount fs=mydrive: mountPoint=/home/<user>/mountPoint
    rclone rc mount/mount fs=mydrive: mountPoint=/home/<user>/mountPoint mountOption=mount
`,
	})

	rc.Add(rc.Call{
		Path:         "mount/unmount",
		AuthRequired: true,
		Fn:           unMountRc,
		Title:        "Unmount all active mounts",
		Help: `
rclone allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

This takes the following parameters

- mountPoint: valid path on the local machine where the mount was created (required)

Eg

    rclone rc mount/unmount mountPoint=/home/<user>/mountPoint
`,
	})

}

// AddRc adds mount and unmount functionality to rc
func AddRc(mountUtilName string, mountFunction MountFn) {
	if mountFns == nil {
		mountFns = make(map[string]MountFn)
	}
	if unmountFns == nil {
		unmountFns = make(map[string]UnmountFn)
	}
	// rcMount allows the mount command to be run from rc
	mountFns[mountUtilName] = mountFunction
}

// rcMount allows the umount command to be run from rc
func unMountRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	mountPoint, err := in.GetString("mountPoint")
	if err != nil {
		return nil, err
	}

	if unmountFns != nil && unmountFns[mountPoint] != nil {
		err := unmountFns[mountPoint]()
		if err != nil {
			return nil, err
		}
		unmountFns[mountPoint] = nil
	} else {
		return nil, errors.New("mount not found")
	}

	return nil, nil
}

// rcMount allows the mount command to be run from rc
func mountRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	mountPoint, err := in.GetString("mountPoint")
	if err != nil {
		return nil, err
	}

	mountOption, err := in.GetString("mountOption")

	if err != nil || mountOption == "" {
		if mountFns["mount"] != nil {
			mountOption = "mount"
		} else if mountFns["cmount"] != nil {
			mountOption = "cmount"
		} else if mountFns["mount2"] != nil {
			mountOption = "mount2"
		}
	}

	// Get Fs.fs to be mounted from fs parameter in the params
	fdst, err := rc.GetFs(in)
	if err != nil {
		return nil, err
	}

	if mountFns[mountOption] != nil {
		_, _, unmountFns[mountPoint], err = mountFns[mountOption](fdst, mountPoint)
		if err != nil {
			log.Printf("mount FAILED: %v", err)
			return nil, err
		}
		fs.Debugf(nil, "Mount for %s created at %s using %s", fdst.String(), mountPoint, mountOption)
		return nil, nil
	}
	return nil, errors.New("Mount Option specified is not registered, or is invalid")
}
