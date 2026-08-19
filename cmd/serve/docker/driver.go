package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/vfs/vfscommon"
)

const (
	// volTimeout is the maximum time to wait for a single volume to
	// be restored (filesystem setup) during plugin startup.
	volTimeout = 30 * time.Second
)

// Driver implements docker driver api
type Driver struct {
	root      string
	volumes   map[string]*Volume
	statePath string
	dummy     bool // disables real mounting
	mntOpt    mountlib.Options
	vfsOpt    vfscommon.Options
	mu        sync.Mutex
	exitOnce  sync.Once
	hupChan   chan os.Signal
	monChan   chan bool // exit if true for exit, refresh if false
}

// NewDriver makes a new docker driver
func NewDriver(ctx context.Context, root string, mntOpt *mountlib.Options, vfsOpt *vfscommon.Options, dummy, forgetState bool) (*Driver, error) {
	// setup directories
	cacheDir := config.GetCacheDir()
	err := file.MkdirAll(cacheDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %s: %w", cacheDir, err)
	}

	// setup driver state
	if mntOpt == nil {
		mntOpt = &mountlib.Opt
	}
	if vfsOpt == nil {
		vfsOpt = &vfscommon.Opt
	}
	drv := &Driver{
		root:      root,
		statePath: filepath.Join(cacheDir, stateFile),
		volumes:   map[string]*Volume{},
		mntOpt:    *mntOpt,
		vfsOpt:    *vfsOpt,
		dummy:     dummy,
	}
	drv.mntOpt.Daemon = false

	// start mount monitoring - must be before restoreState since
	// restoring mounts sends on monChan
	drv.hupChan = make(chan os.Signal, 1)
	drv.monChan = make(chan bool, 1)
	go drv.monitor()

	// restore from saved state
	if !forgetState {
		if err = drv.restoreState(ctx); err != nil {
			return nil, fmt.Errorf("failed to restore state: %w", err)
		}
	}

	// unmount all volumes on exit
	atexit.Register(func() {
		drv.exitOnce.Do(drv.Exit)
	})

	// notify systemd
	if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		return nil, fmt.Errorf("failed to notify systemd: %w", err)
	}

	return drv, nil
}

// Exit will unmount all currently mounted volumes
func (drv *Driver) Exit() {
	fs.Debugf(nil, "Unmount all volumes")
	drv.mu.Lock()
	defer drv.mu.Unlock()

	reportErr(func() error {
		_, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
		return err
	}())
	drv.monChan <- true // ask monitor to exit
	for _, vol := range drv.volumes {
		reportErr(vol.unmountAll())
		vol.Mounts = []string{} // never persist mounts at exit
	}
	reportErr(drv.saveState())
	drv.dummy = true // no more mounts
}

// monitor all mounts
func (drv *Driver) monitor() {
	for {
		// https://stackoverflow.com/questions/19992334/how-to-listen-to-n-channels-dynamic-select-statement
		monChan := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(drv.monChan),
		}
		hupChan := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(drv.monChan),
		}
		sources := []reflect.SelectCase{monChan, hupChan}
		volumes := []*Volume{nil, nil}

		drv.mu.Lock()
		for _, vol := range drv.volumes {
			if vol.mnt.ErrChan != nil {
				errSource := reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(vol.mnt.ErrChan),
				}
				sources = append(sources, errSource)
				volumes = append(volumes, vol)
			}
		}
		drv.mu.Unlock()

		fs.Debugf(nil, "Monitoring %d volumes", len(sources)-2)
		idx, val, _ := reflect.Select(sources)
		switch idx {
		case 0:
			if val.Bool() {
				fs.Debugf(nil, "Monitoring stopped")
				return
			}
		case 1:
			// user sent SIGHUP to clear the cache
			drv.clearCache()
		default:
			vol := volumes[idx]
			if err := val.Interface(); err != nil {
				fs.Logf(nil, "Volume %q unmounted externally: %v", vol.Name, err)
			} else {
				fs.Infof(nil, "Volume %q unmounted externally", vol.Name)
			}
			drv.mu.Lock()
			reportErr(vol.unmountAll())
			drv.mu.Unlock()
		}
	}
}

// clearCache will clear cache of all volumes
func (drv *Driver) clearCache() {
	fs.Debugf(nil, "Clear all caches")
	drv.mu.Lock()
	defer drv.mu.Unlock()

	for _, vol := range drv.volumes {
		reportErr(vol.clearCache())
	}
}

func reportErr(err error) {
	if err != nil {
		fs.Errorf("docker plugin", "%v", err)
	}
}

// Create volume.
//
// If the volume already exists, the request is treated as a no-op
// (idempotent) so that Docker can safely re-send Create requests
// after a plugin restart without getting "volume already exists".
// To use subpath we are limited to defining a new volume definition via alias.
func (drv *Driver) Create(req *CreateRequest) error {
	ctx := context.Background()
	drv.mu.Lock()
	defer drv.mu.Unlock()

	name := req.Name
	fs.Debugf(nil, "Create volume %q", name)

	if vol, _ := drv.getVolume(name); vol != nil {
		fs.Debugf(nil, "Volume %q already exists, treating Create as no-op", name)
		return nil
	}

	vol, err := newVolume(ctx, name, req.Options, drv)
	if err != nil {
		return err
	}
	drv.volumes[name] = vol
	return drv.saveState()
}

// Remove volume
func (drv *Driver) Remove(req *RemoveRequest) error {
	ctx := context.Background()
	drv.mu.Lock()
	defer drv.mu.Unlock()
	vol, err := drv.getVolume(req.Name)
	if err != nil {
		return err
	}
	if err = vol.remove(ctx); err != nil {
		return err
	}
	delete(drv.volumes, vol.Name)
	return drv.saveState()
}

// List volumes handled by the driver
func (drv *Driver) List() (*ListResponse, error) {
	drv.mu.Lock()
	defer drv.mu.Unlock()

	volumeList := drv.listVolumes()
	fs.Debugf(nil, "List: %v", volumeList)

	res := &ListResponse{
		Volumes: []*VolInfo{},
	}
	for _, name := range volumeList {
		vol := drv.volumes[name]
		res.Volumes = append(res.Volumes, vol.getInfo())
	}
	return res, nil
}

// Get volume info
func (drv *Driver) Get(req *GetRequest) (*GetResponse, error) {
	drv.mu.Lock()
	defer drv.mu.Unlock()
	vol, err := drv.getVolume(req.Name)
	if err != nil {
		return nil, err
	}
	return &GetResponse{Volume: vol.getInfo()}, nil
}

// Path returns path of the requested volume
func (drv *Driver) Path(req *PathRequest) (*PathResponse, error) {
	drv.mu.Lock()
	defer drv.mu.Unlock()
	vol, err := drv.getVolume(req.Name)
	if err != nil {
		return nil, err
	}
	return &PathResponse{Mountpoint: vol.MountPoint}, nil
}

// Mount volume
func (drv *Driver) Mount(req *MountRequest) (*MountResponse, error) {
	drv.mu.Lock()
	defer drv.mu.Unlock()
	vol, err := drv.getVolume(req.Name)
	if err == nil {
		err = vol.mount(req.ID)
	}
	if err == nil {
		err = drv.saveState()
	}
	if err != nil {
		return nil, err
	}
	return &MountResponse{Mountpoint: vol.MountPoint}, nil
}

// Unmount volume
func (drv *Driver) Unmount(req *UnmountRequest) error {
	drv.mu.Lock()
	defer drv.mu.Unlock()
	vol, err := drv.getVolume(req.Name)
	if err == nil {
		err = vol.unmount(req.ID)
	}
	if err == nil {
		err = drv.saveState()
	}
	return err
}

// getVolume returns volume by name
func (drv *Driver) getVolume(name string) (*Volume, error) {
	vol := drv.volumes[name]
	if vol == nil {
		return nil, ErrVolumeNotFound
	}
	return vol, nil
}

// listVolumes returns list volume listVolumes
func (drv *Driver) listVolumes() []string {
	names := []string{}
	for key := range drv.volumes {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}

// saveState saves volumes handled by driver to persistent store
func (drv *Driver) saveState() error {
	volumeList := drv.listVolumes()
	fs.Debugf(nil, "Save state %v to %s", volumeList, drv.statePath)

	state := []*Volume{}
	for _, key := range volumeList {
		vol := drv.volumes[key]
		vol.prepareState()
		state = append(state, vol)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	ctx := context.Background()
	retries := fs.GetConfig(ctx).LowLevelRetries
	for i := 0; i <= retries; i++ {
		err = os.WriteFile(drv.statePath, data, 0600)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}
	return fmt.Errorf("failed to save state: %w", err)
}

// restoreState recreates volumes from saved driver state.
//
// It restores volume metadata and filesystems but defers the actual
// FUSE mounts. Call restoreMounts afterwards (typically after the
// server socket is listening) to perform the mounts.
func (drv *Driver) restoreState(ctx context.Context) error {
	fs.Debugf(nil, "Restore state from %s", drv.statePath)

	data, err := os.ReadFile(drv.statePath)
	if os.IsNotExist(err) {
		return nil
	}

	var state []*Volume
	if err == nil {
		err = json.Unmarshal(data, &state)
	}
	if err != nil {
		fs.Logf(nil, "Failed to restore plugin state: %v", err)
		return nil
	}

	// Restore volumes concurrently so one slow remote doesn't
	// block restoring the others.
	type result struct {
		vol *Volume
		err error
	}
	results := make([]result, len(state))
	var wg sync.WaitGroup
	for i, vol := range state {
		wg.Add(1)
		go func(i int, vol *Volume) {
			defer wg.Done()
			// Use a timeout so that a slow or unreachable remote does
			// not block the plugin from starting up.
			volCtx, cancel := context.WithTimeout(ctx, volTimeout)
			defer cancel()
			results[i] = result{vol: vol, err: vol.restoreState(volCtx, drv)}
		}(i, vol)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			fs.Logf(nil, "Failed to restore volume %q: %v", r.vol.Name, r.err)
			continue
		}
		drv.volumes[r.vol.Name] = r.vol
	}
	return nil
}

// RestoreMounts mounts all volumes that were previously mounted.
//
// This should be called after the server socket is listening so that
// Docker can communicate with the plugin even if individual mounts
// are slow or fail. Mounts are performed concurrently.
func (drv *Driver) RestoreMounts() {
	// Collect pending mounts under the lock
	type pendingMount struct {
		vol    *Volume
		mounts []string
	}
	drv.mu.Lock()
	var pending []pendingMount
	for _, vol := range drv.volumes {
		mounts := vol.getPendingMounts()
		if len(mounts) > 0 {
			pending = append(pending, pendingMount{vol: vol, mounts: mounts})
		}
	}
	drv.mu.Unlock()

	// Mount concurrently without holding the driver lock
	var wg sync.WaitGroup
	for _, p := range pending {
		wg.Add(1)
		go func(vol *Volume, mounts []string) {
			defer wg.Done()
			for _, id := range mounts {
				drv.mu.Lock()
				err := vol.mount(id)
				drv.mu.Unlock()
				if err != nil {
					fs.Logf(nil, "Failed to restore mount %q for volume %q: %v", id, vol.Name, err)
				}
			}
		}(p.vol, p.mounts)
	}
	wg.Wait()
}
