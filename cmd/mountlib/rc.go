package mountlib

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
)

// MountInfo defines the configuration for a mount
type MountInfo struct {
	unmountFn  UnmountFn
	MountPoint string    `json:"MountPoint"`
	MountedOn  time.Time `json:"MountedOn"`
	Fs         string    `json:"Fs"`
	MountOpt   *Options
	VFSOpt     *vfscommon.Options
}

var (
	// mutex to protect all the variables in this block
	mountMu sync.Mutex
	// Mount functions available
	mountFns = map[string]MountFn{}
	// Map of mounted path => MountInfo
	liveMounts = map[string]MountInfo{}
)

// AddRc adds mount and unmount functionality to rc
func AddRc(mountUtilName string, mountFunction MountFn) {
	mountMu.Lock()
	defer mountMu.Unlock()
	// rcMount allows the mount command to be run from rc
	mountFns[mountUtilName] = mountFunction
}

func init() {
	rc.Add(rc.Call{
		Path:         "mount/mount",
		AuthRequired: true,
		Fn:           mountRc,
		Title:        "Create a new mount point",
		Help: `rclone allows Linux, FreeBSD, macOS and Windows to mount any of
Rclone's cloud storage systems as a file system with FUSE.

If no mountType is provided, the priority is given as follows: 1. mount 2.cmount 3.mount2

This takes the following parameters

- fs - a remote path to be mounted (required)
- mountPoint: valid path on the local machine (required)
- mountType: One of the values (mount, cmount, mount2) specifies the mount implementation to use
- mountOpt: a JSON object with Mount options in.
- vfsOpt: a JSON object with VFS options in.

Eg

    rclone rc mount/mount fs=mydrive: mountPoint=/home/<user>/mountPoint
    rclone rc mount/mount fs=mydrive: mountPoint=/home/<user>/mountPoint mountType=mount
    rclone rc mount/mount fs=TestDrive: mountPoint=/mnt/tmp vfsOpt='{"CacheMode": 2}' mountOpt='{"AllowOther": true}'

The vfsOpt are as described in options/get and can be seen in the the
"vfs" section when running and the mountOpt can be seen in the "mount" section.

    rclone rc options/get
`,
	})
}

// mountRc allows the mount command to be run from rc
func mountRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	mountPoint, err := in.GetString("mountPoint")
	if err != nil {
		return nil, err
	}

	vfsOpt := vfsflags.Opt
	err = in.GetStructMissingOK("vfsOpt", &vfsOpt)
	if err != nil {
		return nil, err
	}

	mountOpt := Opt
	err = in.GetStructMissingOK("mountOpt", &mountOpt)
	if err != nil {
		return nil, err
	}

	mountType, err := in.GetString("mountType")

	mountMu.Lock()
	defer mountMu.Unlock()

	if err != nil || mountType == "" {
		if mountFns["mount"] != nil {
			mountType = "mount"
		} else if mountFns["cmount"] != nil {
			mountType = "cmount"
		} else if mountFns["mount2"] != nil {
			mountType = "mount2"
		}
	}

	// Get Fs.fs to be mounted from fs parameter in the params
	fdst, err := rc.GetFs(in)
	if err != nil {
		return nil, err
	}

	if mountFns[mountType] != nil {
		VFS := vfs.New(fdst, &vfsOpt)
		_, unmountFn, err := mountFns[mountType](VFS, mountPoint, &mountOpt)

		if err != nil {
			log.Printf("mount FAILED: %v", err)
			return nil, err
		}
		// Add mount to list if mount point was successfully created
		liveMounts[mountPoint] = MountInfo{
			unmountFn:  unmountFn,
			MountedOn:  time.Now(),
			Fs:         fdst.Name(),
			MountPoint: mountPoint,
			VFSOpt:     &vfsOpt,
			MountOpt:   &mountOpt,
		}

		fs.Debugf(nil, "Mount for %s created at %s using %s", fdst.String(), mountPoint, mountType)
		return nil, nil
	}
	return nil, errors.New("Mount Option specified is not registered, or is invalid")
}

func init() {
	rc.Add(rc.Call{
		Path:         "mount/unmount",
		AuthRequired: true,
		Fn:           unMountRc,
		Title:        "Unmount selected active mount",
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

// unMountRc allows the umount command to be run from rc
func unMountRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	mountPoint, err := in.GetString("mountPoint")
	if err != nil {
		return nil, err
	}
	mountMu.Lock()
	defer mountMu.Unlock()
	err = performUnMount(mountPoint)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "mount/types",
		AuthRequired: true,
		Fn:           mountTypesRc,
		Title:        "Show all possible mount types",
		Help: `This shows all possible mount types and returns them as a list.

This takes no parameters and returns

- mountTypes: list of mount types

The mount types are strings like "mount", "mount2", "cmount" and can
be passed to mount/mount as the mountType parameter.

Eg

    rclone rc mount/types
`,
	})
}

// mountTypesRc returns a list of available mount types.
func mountTypesRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	var mountTypes = []string{}
	mountMu.Lock()
	defer mountMu.Unlock()
	for mountType := range mountFns {
		mountTypes = append(mountTypes, mountType)
	}
	sort.Strings(mountTypes)
	return rc.Params{
		"mountTypes": mountTypes,
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "mount/listmounts",
		AuthRequired: true,
		Fn:           listMountsRc,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc mount/listmounts
`,
	})
}

// listMountsRc returns a list of current mounts
func listMountsRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	var mountTypes = []MountInfo{}
	mountMu.Lock()
	defer mountMu.Unlock()
	for _, a := range liveMounts {
		mountTypes = append(mountTypes, a)
	}
	return rc.Params{
		"mountPoints": mountTypes,
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "mount/unmountall",
		AuthRequired: true,
		Fn:           unmountAll,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns error if unmount does not succeed.

Eg

    rclone rc mount/unmountall
`,
	})
}

// unmountAll unmounts all the created mounts
func unmountAll(_ context.Context, in rc.Params) (out rc.Params, err error) {
	mountMu.Lock()
	defer mountMu.Unlock()
	for key, mountInfo := range liveMounts {
		err = performUnMount(mountInfo.MountPoint)
		if err != nil {
			fs.Debugf(nil, "Couldn't unmount : %s", key)
			return nil, err
		}
	}
	return nil, nil
}

// performUnMount unmounts the specified mountPoint
func performUnMount(mountPoint string) (err error) {
	mountInfo, ok := liveMounts[mountPoint]
	if ok {
		err := mountInfo.unmountFn()
		if err != nil {
			return err
		}
		delete(liveMounts, mountPoint)
	} else {
		return errors.New("mount not found")
	}
	return nil
}
