package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/vfs"

	_ "github.com/rclone/rclone/backend/local"
)

// Names under which the fake mount methods are registered with
// mountlib, so that volumes can be mounted without FUSE.
const (
	stuckMountType = "test-docker-stuck-mount"
	noopMountType  = "test-docker-noop-mount"
)

// registerMount registers a fake mount method with mountlib.
func registerMount(t *testing.T, name string, mountFn mountlib.MountFn) {
	t.Helper()
	mountlib.AddRc(name, mountFn)
}

// noopMount returns a mount method which mounts successfully without
// touching the kernel.
func noopMount() mountlib.MountFn {
	return func(*vfs.VFS, string, *mountlib.Options) (<-chan error, func() error, string, error) {
		return nil, nil, "", nil
	}
}

// newTestDriver makes a driver which really mounts, using root as the
// volume base directory.
func newTestDriver(t *testing.T, root string) *Driver {
	t.Helper()
	return &Driver{
		root:      root,
		statePath: filepath.Join(root, stateFile),
		volumes:   map[string]*Volume{},
		monChan:   make(chan bool, 1),
	}
}

// saveStateLocked saves the driver state the way the API handlers do.
func saveStateLocked(t *testing.T, drv *Driver) {
	t.Helper()
	drv.mu.Lock()
	defer drv.mu.Unlock()
	require.NoError(t, drv.saveState())
}

// TestRestoreKeepsVolumeOnMountpointError checks that a volume whose
// mountpoint cannot be inspected keeps its definition: it stays in the
// driver's volume list, survives being saved, and its setup is retried when
// Docker mounts it again.
//
// A volume left behind on a dead FUSE mount fails here with ENOTCONN from
// lstat. The test reproduces that failure without touching a real mount by
// making the mountpoint's parent a regular file, so both lstat and MkdirAll
// fail with ENOTDIR - an error which, like ENOTCONN, is not "does not
// exist" and does not clear by itself.
func TestRestoreKeepsVolumeOnMountpointError(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	dataDir := filepath.Join(testDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0700))

	// The volume base directory cannot be created while this regular file
	// is in the way.
	blocker := filepath.Join(testDir, "blocked")
	require.NoError(t, os.WriteFile(blocker, nil, 0600))
	root := filepath.Join(blocker, "volumes")
	mountpoint := filepath.Join(root, "vol1")

	registerMount(t, noopMountType, noopMount())

	statePath := filepath.Join(testDir, stateFile)
	state := fmt.Sprintf(`[{"name":"vol1","mountpoint":%q,"created":%q,"fs":"","type":"local","path":%q,"options":{"mount-type":%q},"mounts":["id1"]}]`,
		mountpoint, time.Now().Format(time.RFC3339), dataDir, noopMountType)
	require.NoError(t, os.WriteFile(statePath, []byte(state), 0600))

	// The state file has to live outside the blocked base directory.
	drv := newTestDriver(t, root)
	drv.statePath = statePath
	require.NoError(t, drv.restoreState(ctx))

	// The definition must still be reported.
	getRes, err := drv.Get(&GetRequest{Name: "vol1"})
	require.NoError(t, err)
	require.NotNil(t, getRes)
	assert.Equal(t, mountpoint, getRes.Volume.Mountpoint)
	listRes, err := drv.List()
	require.NoError(t, err)
	require.Len(t, listRes.Volumes, 1)
	assert.Equal(t, "vol1", listRes.Volumes[0].Name)

	// It must also survive a save, which used to erase it.
	saveStateLocked(t, drv)
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"name":"vol1"`)

	// The failure was transient: once the mountpoint can be created, a
	// mount request must complete the setup which restore deferred rather
	// than fail for ever.
	require.NoError(t, os.Remove(blocker))
	require.NoError(t, os.MkdirAll(root, 0700))
	mountRes, err := drv.Mount(&MountRequest{Name: "vol1", ID: "id1"})
	require.NoError(t, err)
	assert.Equal(t, mountpoint, mountRes.Mountpoint)
	getRes, err = drv.Get(&GetRequest{Name: "vol1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"id1"}, getRes.Volume.Status["Mounts"])
}

// TestRestoreDropsInvalidVolume checks that a volume whose definition is
// unusable is still dropped rather than kept for a retry which can never
// succeed.
func TestRestoreDropsInvalidVolume(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()

	statePath := filepath.Join(testDir, stateFile)
	state := fmt.Sprintf(`[{"name":"vol1","mountpoint":%q,"created":%q,"fs":"","options":{},"mounts":[]}]`,
		filepath.Join(testDir, "vol1"), time.Now().Format(time.RFC3339))
	require.NoError(t, os.WriteFile(statePath, []byte(state), 0600))

	drv := newTestDriver(t, testDir)
	require.NoError(t, drv.restoreState(ctx))
	_, err := drv.Get(&GetRequest{Name: "vol1"})
	assert.ErrorIs(t, err, ErrVolumeNotFound)
}

// TestVolumeAPINotBlockedByStuckMount checks that a mount which never
// returns does not hold up the volume API for other volumes, and that
// mounting and unmounting the same volume stays ordered.
func TestVolumeAPINotBlockedByStuckMount(t *testing.T) {
	testDir := t.TempDir()
	dataDir := filepath.Join(testDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0700))

	mountStarted := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	defer unblock()

	registerMount(t, stuckMountType, func(*vfs.VFS, string, *mountlib.Options) (<-chan error, func() error, string, error) {
		close(mountStarted)
		<-release
		return nil, nil, "", nil
	})

	drv := newTestDriver(t, testDir)
	for _, name := range []string{"stuck", "other"} {
		require.NoError(t, drv.Create(&CreateRequest{
			Name: name,
			Options: VolOpts{
				"type":       "local",
				"path":       dataDir,
				"mount-type": stuckMountType,
			},
		}))
	}

	mountDone := make(chan error, 1)
	go func() {
		_, err := drv.Mount(&MountRequest{Name: "stuck", ID: "id1"})
		mountDone <- err
	}()
	<-mountStarted

	// The volume API must keep answering while that mount is stuck.
	apiDone := make(chan struct{})
	var getErr, pathErr, listErr error
	var listRes *ListResponse
	var getRes *GetResponse
	go func() {
		defer close(apiDone)
		getRes, getErr = drv.Get(&GetRequest{Name: "other"})
		_, pathErr = drv.Path(&PathRequest{Name: "other"})
		listRes, listErr = drv.List()
	}()
	select {
	case <-apiDone:
	case <-time.After(10 * time.Second):
		t.Fatal("volume API did not answer while another volume was mounting")
	}
	require.NoError(t, getErr)
	require.NoError(t, pathErr)
	require.NoError(t, listErr)
	require.NotNil(t, getRes)
	require.Len(t, listRes.Volumes, 2)

	// A second mount of the same volume must wait for the first.
	secondDone := make(chan error, 1)
	go func() {
		_, err := drv.Mount(&MountRequest{Name: "stuck", ID: "id2"})
		secondDone <- err
	}()
	select {
	case err := <-secondDone:
		t.Fatalf("second mount of the same volume returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	unblock()
	require.NoError(t, <-mountDone)
	require.NoError(t, <-secondDone)
}
