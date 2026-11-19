package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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
	ErrMountpointExists = errors.New("non-empty mountpoint already exists")
)

// retryableError marks a volume setup failure which may clear by itself,
// e.g. ENOTCONN from the mountpoint of a dead FUSE mount, so the volume
// definition is kept and the setup is retried on the next Mount request.
type retryableError struct{ err error }

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() error { return e.err }

// isRetryable reports whether err is a setup failure which a later mount
// attempt may get past.
func isRetryable(err error) bool {
	var retryable retryableError
	return errors.As(err, &retryable)
}

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

	// mountMu serialises mounting and unmounting this volume, and guards
	// the fields of vol.mnt. It is held for the whole of a mount, which
	// may block indefinitely in the kernel, but never while mu is held.
	mountMu sync.Mutex

	// mu guards the fields below. It is never held across any I/O, so
	// reporting or saving a volume never waits for its mount.
	mu        sync.Mutex
	setupDone bool // filesystem setup has succeeded

	mountReqs     map[string]any
	pendingMounts []string     // mount IDs to restore after server starts
	errChan       <-chan error // mirror of vol.mnt.ErrChan for the monitor

	fsString  string // result of merging Fs, Type and Options
	persist   bool
	mountType string
	drv       *Driver
	mnt       *mountlib.MountPoint
}

// VolOpts keeps volume options
type VolOpts map[string]string

// VolInfo represents a volume for Get and List requests
type VolInfo struct {
	Name       string
	Mountpoint string         `json:",omitempty"`
	CreatedAt  string         `json:",omitempty"`
	Status     map[string]any `json:",omitempty"`
}

// volumeMountPath returns the mountpoint for a volume called name below
// root, together with whether that path is a strict descendant of root.
//
// filepath.Join cleans its result, collapsing any ".." components in the
// volume name, so a crafted name could otherwise resolve to an arbitrary
// host path outside root, or to root itself (for an empty or "." name)
// where a mount would shadow every other volume. Callers must reject names
// for which confined is false rather than creating a directory and mounting
// there.
func volumeMountPath(root, name string) (path string, confined bool) {
	path = filepath.Join(root, name)
	root = filepath.Clean(root)
	return path, strings.HasPrefix(path, root+string(filepath.Separator))
}

func newVolume(ctx context.Context, name string, volOpt VolOpts, drv *Driver) (*Volume, error) {
	path, confined := volumeMountPath(drv.root, name)
	if !confined {
		return nil, fmt.Errorf("invalid volume name %q: resolves outside the base directory", name)
	}
	mnt := &mountlib.MountPoint{
		MountPoint: path,
	}
	vol := &Volume{
		Name:       name,
		MountPoint: path,
		CreatedAt:  time.Now(),
		drv:        drv,
		mnt:        mnt,
		mountReqs:  make(map[string]any),
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
	vol.mu.Lock()
	defer vol.mu.Unlock()
	vol.prepareStateLocked()
	return &VolInfo{
		Name:       vol.Name,
		CreatedAt:  vol.CreatedAt.Format(time.RFC3339),
		Mountpoint: vol.MountPoint,
		Status:     rc.Params{"Mounts": vol.Mounts},
	}
}

// prepareState prepares volume for saving state
func (vol *Volume) prepareState() {
	vol.mu.Lock()
	defer vol.mu.Unlock()
	vol.prepareStateLocked()
}

// prepareStateLocked prepares volume for saving state
//
// vol.mu must be held.
func (vol *Volume) prepareStateLocked() {
	vol.Mounts = []string{}
	for id := range vol.mountReqs {
		vol.Mounts = append(vol.Mounts, id)
	}
	sort.Strings(vol.Mounts)
}

// restoreState updates volume from saved state.
//
// It restores the volume configuration and filesystem but does not
// perform FUSE mounts. The pending mount IDs are saved and can be
// retrieved with getPendingMounts for deferred mounting.
func (vol *Volume) restoreState(ctx context.Context, drv *Driver) error {
	vol.drv = drv
	// Re-derive the mountpoint from the base directory and name rather
	// than trusting the persisted path, which an older rclone or a
	// tampered state file could have left pointing outside the base
	// directory.
	path, confined := volumeMountPath(drv.root, vol.Name)
	if !confined {
		return fmt.Errorf("invalid volume name %q: resolves outside the base directory", vol.Name)
	}
	vol.MountPoint = path
	vol.mnt = &mountlib.MountPoint{
		MountPoint: path,
	}
	// Save pending mounts before applyOptions clears them
	vol.pendingMounts = vol.Mounts
	volOpt := vol.Options
	volOpt["fs"] = vol.Fs
	volOpt["type"] = vol.Type
	// applyOptions consumes "path" into vol.Path rather than leaving it in
	// vol.Options, so it must be fed back explicitly like fs and type.
	volOpt["path"] = vol.Path
	if err := vol.applyOptions(volOpt); err != nil {
		return err
	}
	if err := vol.validate(); err != nil {
		return err
	}
	if err := vol.setup(ctx); err != nil {
		return err
	}
	return nil
}

// getPendingMounts returns and clears the list of mount IDs that
// were saved from state and need to be re-mounted.
func (vol *Volume) getPendingMounts() []string {
	vol.mu.Lock()
	defer vol.mu.Unlock()
	mounts := vol.pendingMounts
	vol.pendingMounts = nil
	return mounts
}

// markSetupDone records that the volume's filesystem is set up
func (vol *Volume) markSetupDone() {
	vol.mu.Lock()
	vol.setupDone = true
	vol.mu.Unlock()
}

// getErrChan returns the channel reporting the end of this volume's mount,
// or nil if it is not mounted.
func (vol *Volume) getErrChan() <-chan error {
	vol.mu.Lock()
	defer vol.mu.Unlock()
	return vol.errChan
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
		vol.mountReqs = make(map[string]any)
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
			return retryableError{fmt.Errorf("failed to create mountpoint: %s: %w", path, err)}
		}
	} else if err != nil {
		// A mountpoint left behind by a dead FUSE mount reports
		// ENOTCONN here, which may clear without the volume changing.
		return retryableError{err}
	}
	if runtime.GOOS != "windows" {
		if err := mountlib.CheckMountEmpty(path); err != nil {
			return ErrMountpointExists
		}
	}
	return nil
}

// setup volume filesystem
//
// vol.mountMu must be held, except while the volume is still being
// created and not yet reachable by any other goroutine. Failures which may
// clear on their own are returned as retryableError.
func (vol *Volume) setup(ctx context.Context) error {
	fs.Debugf(nil, "Setup volume %q as %q at path %s", vol.Name, vol.fsString, vol.MountPoint)

	if err := vol.checkMountpoint(); err != nil {
		return err
	}
	if vol.drv.dummy {
		vol.markSetupDone()
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
			return retryableError{err}
		}
	}

	// Use existing remote
	f, err := fs.NewFs(ctx, vol.fsString)
	if err != nil {
		return retryableError{err}
	}
	vol.mnt.Fs = f
	vol.markSetupDone()
	return nil
}

// remove volume filesystem and mounts
func (vol *Volume) remove(ctx context.Context) error {
	vol.mountMu.Lock()
	defer vol.mountMu.Unlock()

	vol.mu.Lock()
	count := len(vol.mountReqs)
	vol.mu.Unlock()
	fs.Debugf(nil, "Remove volume %q (count %d)", vol.Name, count)

	if count > 0 {
		return errors.New("volume is in use")
	}

	// vol.mnt.Fs is nil when the setup of a restored volume is still
	// deferred, so there is nothing to shut down.
	if !vol.drv.dummy && vol.mnt.Fs != nil {
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
	vol.mountMu.Lock()
	defer vol.mountMu.Unlock()
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
//
// vol.mountMu is held throughout, including the blocking filesystem
// mount, so that mounts and unmounts of this volume stay ordered while
// other volumes and volume metadata reads are unaffected. Filesystem
// setup deferred by a failed restore is retried here.
func (vol *Volume) mount(id string) error {
	vol.mountMu.Lock()
	defer vol.mountMu.Unlock()

	drv := vol.drv
	vol.mu.Lock()
	count := len(vol.mountReqs)
	fs.Debugf(nil, "Mount volume %q for id %q at path %s (count %d)",
		vol.Name, id, vol.MountPoint, count)

	if _, found := vol.mountReqs[id]; found {
		vol.mu.Unlock()
		return errors.New("volume is already mounted by this id")
	}

	if count > 0 { // already mounted
		vol.mountReqs[id] = nil
		vol.mu.Unlock()
		return nil
	}
	if drv.dummy {
		vol.mountReqs[id] = nil
		vol.mu.Unlock()
		return nil
	}
	setupDone := vol.setupDone
	vol.mu.Unlock()

	if !setupDone {
		if err := vol.setup(context.Background()); err != nil {
			return err
		}
	}
	if vol.mnt.Fs == nil {
		return errors.New("volume filesystem is not ready")
	}

	if _, err := vol.mnt.Mount(); err != nil {
		return err
	}

	vol.mu.Lock()
	vol.mountReqs[id] = nil
	vol.errChan = vol.mnt.ErrChan
	vol.mu.Unlock()
	drv.notifyMonitor()
	return nil
}

// unmount volume
//
// vol.mountMu is held throughout so that this cannot interleave with a
// mount or unmount of the same volume.
func (vol *Volume) unmount(id string) error {
	vol.mountMu.Lock()
	defer vol.mountMu.Unlock()

	drv := vol.drv
	vol.mu.Lock()
	count := len(vol.mountReqs)
	fs.Debugf(nil, "Unmount volume %q from id %q at path %s (count %d)",
		vol.Name, id, vol.MountPoint, count)

	if count == 0 {
		vol.mu.Unlock()
		return errors.New("volume is not mounted")
	}
	if _, found := vol.mountReqs[id]; !found {
		vol.mu.Unlock()
		return errors.New("volume is not mounted by this id")
	}

	delete(vol.mountReqs, id)
	if len(vol.mountReqs) > 0 {
		vol.mu.Unlock()
		return nil // more mounts left
	}
	vol.mu.Unlock()

	if drv.dummy {
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
	vol.mu.Lock()
	vol.errChan = nil
	vol.mu.Unlock()
	drv.notifyMonitor()
	return nil
}

func (vol *Volume) unmountAll() error {
	vol.mu.Lock()
	ids := make([]string, 0, len(vol.mountReqs))
	for id := range vol.mountReqs {
		ids = append(ids, id)
	}
	vol.mu.Unlock()

	var firstErr error
	for _, id := range ids {
		err := vol.unmount(id)
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
