package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/file"
)

// Errors
var (
	ErrVolumeNotFound   = errors.New("volume not found")
	ErrVolumeExists     = errors.New("volume already exists")
	ErrMountpointExists = errors.New("non-empty mountpoint already exists")
)

// Volume keeps volume runtime state
// Public members get persisted in saved state
type Volume struct {
	Name       string    `json:"name"`
	MountPoint string    `json:"mountpoint"`
	CreatedAt  time.Time `json:"created"`
	Fs         string    `json:"fs"`             // remote[,connectString]:path
	Type       string    `json:"type,omitempty"` // same as ":backend:"
	Path       string    `json:"path,omitempty"` // for "remote:path" or ":backend:path"
	Options    VolOpts   `json:"options"`        // all options together
	Mounts     []string  `json:"mounts"`         // mountReqs as a string list
	mountReqs  map[string]interface{}
	fsString   string // result of merging Fs, Type and Options
	persist    bool
	mountType  string
	drv        *Driver
	mnt        *mountlib.MountPoint
}

// VolOpts keeps volume options
type VolOpts map[string]string

// VolInfo represents a volume for Get and List requests
type VolInfo struct {
	Name       string
	Mountpoint string                 `json:",omitempty"`
	CreatedAt  string                 `json:",omitempty"`
	Status     map[string]interface{} `json:",omitempty"`
}

func newVolume(ctx context.Context, name string, volOpt VolOpts, drv *Driver) (*Volume, error) {
	path := filepath.Join(drv.root, name)
	mnt := &mountlib.MountPoint{
		MountPoint: path,
	}
	vol := &Volume{
		Name:       name,
		MountPoint: path,
		CreatedAt:  time.Now(),
		drv:        drv,
		mnt:        mnt,
		mountReqs:  make(map[string]interface{}),
	}
	err := vol.applyOptions(volOpt)
	if err == nil {
		err = vol.setup(ctx)
	}
	if err != nil {
		return nil, err
	}
	return vol, nil
}

// getInfo returns short digest about volume
func (vol *Volume) getInfo() *VolInfo {
	vol.prepareState()
	return &VolInfo{
		Name:       vol.Name,
		CreatedAt:  vol.CreatedAt.Format(time.RFC3339),
		Mountpoint: vol.MountPoint,
		Status:     rc.Params{"Mounts": vol.Mounts},
	}
}

// prepareState prepares volume for saving state
func (vol *Volume) prepareState() {
	vol.Mounts = []string{}
	for id := range vol.mountReqs {
		vol.Mounts = append(vol.Mounts, id)
	}
	sort.Strings(vol.Mounts)
}

// restoreState updates volume from saved state
func (vol *Volume) restoreState(ctx context.Context, drv *Driver) error {
	vol.drv = drv
	vol.mnt = &mountlib.MountPoint{
		MountPoint: vol.MountPoint,
	}
	volOpt := vol.Options
	volOpt["fs"] = vol.Fs
	volOpt["type"] = vol.Type
	if err := vol.applyOptions(volOpt); err != nil {
		return err
	}
	if err := vol.validate(); err != nil {
		return err
	}
	if err := vol.setup(ctx); err != nil {
		return err
	}
	for _, id := range vol.Mounts {
		if err := vol.mount(id); err != nil {
			return err
		}
	}
	return nil
}

// validate volume
func (vol *Volume) validate() error {
	if vol.Name == "" {
		return errors.New("volume name is required")
	}
	if (vol.Type != "" && vol.Fs != "") || (vol.Type == "" && vol.Fs == "") {
		return errors.New("volume must have either remote or backend type")
	}
	if vol.persist && vol.Type == "" {
		return errors.New("backend type is required to persist remotes")
	}
	if vol.persist && !canPersist {
		return errors.New("using backend type to persist remotes is prohibited")
	}
	if vol.MountPoint == "" {
		return errors.New("mount point is required")
	}
	if vol.mountReqs == nil {
		vol.mountReqs = make(map[string]interface{})
	}
	return nil
}

// checkMountpoint verifies that mount point is an existing empty directory
func (vol *Volume) checkMountpoint() error {
	path := vol.mnt.MountPoint
	if runtime.GOOS == "windows" {
		path = filepath.Dir(path)
	}
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err = file.MkdirAll(path, 0700); err != nil {
			return fmt.Errorf("failed to create mountpoint: %s: %w", path, err)
		}
	} else if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := mountlib.CheckMountEmpty(path); err != nil {
			return ErrMountpointExists
		}
	}
	return nil
}

// setup volume filesystem
func (vol *Volume) setup(ctx context.Context) error {
	fs.Debugf(nil, "Setup volume %q as %q at path %s", vol.Name, vol.fsString, vol.MountPoint)

	if err := vol.checkMountpoint(); err != nil {
		return err
	}
	if vol.drv.dummy {
		return nil
	}

	_, mountFn := mountlib.ResolveMountMethod(vol.mountType)
	if mountFn == nil {
		if vol.mountType != "" {
			return fmt.Errorf("unsupported mount type %q", vol.mountType)
		}
		return errors.New("mount command unsupported by this build")
	}
	vol.mnt.MountFn = mountFn

	if vol.persist {
		// Add remote to config file
		params := rc.Params{}
		for key, val := range vol.Options {
			params[key] = val
		}
		updateMode := config.UpdateRemoteOpt{}
		_, err := config.CreateRemote(ctx, vol.Name, vol.Type, params, updateMode)
		if err != nil {
			return err
		}
	}

	// Use existing remote
	f, err := fs.NewFs(ctx, vol.fsString)
	if err == nil {
		vol.mnt.Fs = f
	}
	return err
}

// remove volume filesystem and mounts
func (vol *Volume) remove(ctx context.Context) error {
	count := len(vol.mountReqs)
	fs.Debugf(nil, "Remove volume %q (count %d)", vol.Name, count)

	if count > 0 {
		return errors.New("volume is in use")
	}

	if !vol.drv.dummy {
		shutdownFn := vol.mnt.Fs.Features().Shutdown
		if shutdownFn != nil {
			if err := shutdownFn(ctx); err != nil {
				return err
			}
		}
	}

	if vol.persist {
		// Remote remote from config file
		config.DeleteRemote(vol.Name)
	}
	return nil
}

// clearCache will clear VFS cache for the volume
func (vol *Volume) clearCache() error {
	VFS := vol.mnt.VFS
	if VFS == nil {
		return nil
	}
	root, err := VFS.Root()
	if err != nil {
		return fmt.Errorf("error reading root: %v: %w", VFS.Fs(), err)
	}
	root.ForgetAll()
	return nil
}

// mount volume filesystem
func (vol *Volume) mount(id string) error {
	drv := vol.drv
	count := len(vol.mountReqs)
	fs.Debugf(nil, "Mount volume %q for id %q at path %s (count %d)",
		vol.Name, id, vol.MountPoint, count)

	if _, found := vol.mountReqs[id]; found {
		return errors.New("volume is already mounted by this id")
	}

	if count > 0 { // already mounted
		vol.mountReqs[id] = nil
		return nil
	}
	if drv.dummy {
		vol.mountReqs[id] = nil
		return nil
	}
	if vol.mnt.Fs == nil {
		return errors.New("volume filesystem is not ready")
	}

	if _, err := vol.mnt.Mount(); err != nil {
		return err
	}
	vol.mountReqs[id] = nil
	vol.drv.monChan <- false // ask monitor to refresh channels
	return nil
}

// unmount volume
func (vol *Volume) unmount(id string) error {
	count := len(vol.mountReqs)
	fs.Debugf(nil, "Unmount volume %q from id %q at path %s (count %d)",
		vol.Name, id, vol.MountPoint, count)

	if count == 0 {
		return errors.New("volume is not mounted")
	}
	if _, found := vol.mountReqs[id]; !found {
		return errors.New("volume is not mounted by this id")
	}

	delete(vol.mountReqs, id)
	if len(vol.mountReqs) > 0 {
		return nil // more mounts left
	}

	if vol.drv.dummy {
		return nil
	}

	mnt := vol.mnt
	if mnt.UnmountFn != nil {
		if err := mnt.UnmountFn(); err != nil {
			return err
		}
	}
	mnt.ErrChan = nil
	mnt.UnmountFn = nil
	mnt.VFS = nil
	vol.drv.monChan <- false // ask monitor to refresh channels
	return nil
}

func (vol *Volume) unmountAll() error {
	var firstErr error
	for id := range vol.mountReqs {
		err := vol.unmount(id)
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
