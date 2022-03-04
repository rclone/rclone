//go:build !race
// +build !race

package docker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/cmd/serve/docker"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/testy"
	"github.com/rclone/rclone/lib/file"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/memory"
	_ "github.com/rclone/rclone/cmd/cmount"
	_ "github.com/rclone/rclone/cmd/mount"
)

func initialise(ctx context.Context, t *testing.T) (string, fs.Fs) {
	fstest.Initialise()

	// Make test cache directory
	testDir, err := fstest.LocalRemote()
	require.NoError(t, err)
	err = file.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	// Make test file system
	testFs, err := fs.NewFs(ctx, testDir)
	require.NoError(t, err)
	return testDir, testFs
}

func assertErrorContains(t *testing.T, err error, errString string, msgAndArgs ...interface{}) {
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), errString, msgAndArgs...)
	}
}

func assertVolumeInfo(t *testing.T, v *docker.VolInfo, name, path string) {
	assert.Equal(t, name, v.Name)
	assert.Equal(t, path, v.Mountpoint)
	assert.NotEmpty(t, v.CreatedAt)
	_, err := time.Parse(time.RFC3339, v.CreatedAt)
	assert.NoError(t, err)
}

func TestDockerPluginLogic(t *testing.T) {
	ctx := context.Background()
	oldCacheDir := config.GetCacheDir()
	testDir, testFs := initialise(ctx, t)
	err := config.SetCacheDir(testDir)
	require.NoError(t, err)
	defer func() {
		_ = config.SetCacheDir(oldCacheDir)
		if !t.Failed() {
			fstest.Purge(testFs)
			_ = os.RemoveAll(testDir)
		}
	}()

	// Create dummy volume driver
	drv, err := docker.NewDriver(ctx, testDir, nil, nil, true, true)
	require.NoError(t, err)
	require.NotNil(t, drv)

	// 1st volume request
	volReq := &docker.CreateRequest{
		Name:    "vol1",
		Options: docker.VolOpts{},
	}
	assertErrorContains(t, drv.Create(volReq), "volume must have either remote or backend")

	volReq.Options["remote"] = testDir
	assert.NoError(t, drv.Create(volReq))
	path1 := filepath.Join(testDir, "vol1")

	assert.ErrorIs(t, drv.Create(volReq), docker.ErrVolumeExists)

	getReq := &docker.GetRequest{Name: "vol1"}
	getRes, err := drv.Get(getReq)
	assert.NoError(t, err)
	require.NotNil(t, getRes)
	assertVolumeInfo(t, getRes.Volume, "vol1", path1)

	// 2nd volume request
	volReq.Name = "vol2"
	assert.NoError(t, drv.Create(volReq))
	path2 := filepath.Join(testDir, "vol2")

	listRes, err := drv.List()
	require.NoError(t, err)
	require.Equal(t, 2, len(listRes.Volumes))
	assertVolumeInfo(t, listRes.Volumes[0], "vol1", path1)
	assertVolumeInfo(t, listRes.Volumes[1], "vol2", path2)

	// Try prohibited volume options
	volReq.Name = "vol99"
	volReq.Options["remote"] = testDir
	volReq.Options["type"] = "memory"
	err = drv.Create(volReq)
	assertErrorContains(t, err, "volume must have either remote or backend")

	volReq.Options["persist"] = "WrongBoolean"
	err = drv.Create(volReq)
	assertErrorContains(t, err, "cannot parse option")

	volReq.Options["persist"] = "true"
	delete(volReq.Options, "remote")
	err = drv.Create(volReq)
	assertErrorContains(t, err, "persist remotes is prohibited")

	volReq.Options["persist"] = "false"
	volReq.Options["memory-option-broken"] = "some-value"
	err = drv.Create(volReq)
	assertErrorContains(t, err, "unsupported backend option")

	getReq.Name = "vol99"
	getRes, err = drv.Get(getReq)
	assert.Error(t, err)
	assert.Nil(t, getRes)

	// Test mount requests
	mountReq := &docker.MountRequest{
		Name: "vol2",
		ID:   "id1",
	}
	mountRes, err := drv.Mount(mountReq)
	assert.NoError(t, err)
	require.NotNil(t, mountRes)
	assert.Equal(t, path2, mountRes.Mountpoint)

	mountRes, err = drv.Mount(mountReq)
	assert.Error(t, err)
	assert.Nil(t, mountRes)
	assertErrorContains(t, err, "already mounted by this id")

	mountReq.ID = "id2"
	mountRes, err = drv.Mount(mountReq)
	assert.NoError(t, err)
	require.NotNil(t, mountRes)
	assert.Equal(t, path2, mountRes.Mountpoint)

	unmountReq := &docker.UnmountRequest{
		Name: "vol2",
		ID:   "id1",
	}
	err = drv.Unmount(unmountReq)
	assert.NoError(t, err)

	err = drv.Unmount(unmountReq)
	assert.Error(t, err)
	assertErrorContains(t, err, "not mounted by this id")

	// Simulate plugin restart
	drv2, err := docker.NewDriver(ctx, testDir, nil, nil, true, false)
	assert.NoError(t, err)
	require.NotNil(t, drv2)

	// New plugin instance should pick up the saved state
	listRes, err = drv2.List()
	require.NoError(t, err)
	require.Equal(t, 2, len(listRes.Volumes))
	assertVolumeInfo(t, listRes.Volumes[0], "vol1", path1)
	assertVolumeInfo(t, listRes.Volumes[1], "vol2", path2)

	rmReq := &docker.RemoveRequest{Name: "vol2"}
	err = drv.Remove(rmReq)
	assertErrorContains(t, err, "volume is in use")

	unmountReq.ID = "id1"
	err = drv.Unmount(unmountReq)
	assert.Error(t, err)
	assertErrorContains(t, err, "not mounted by this id")

	unmountReq.ID = "id2"
	err = drv.Unmount(unmountReq)
	assert.NoError(t, err)

	err = drv.Unmount(unmountReq)
	assert.EqualError(t, err, "volume is not mounted")

	err = drv.Remove(rmReq)
	assert.NoError(t, err)
}

const (
	httpTimeout = 2 * time.Second
	tempDelay   = 10 * time.Millisecond
)

type APIClient struct {
	t    *testing.T
	cli  *http.Client
	host string
}

func newAPIClient(t *testing.T, host, unixPath string) *APIClient {
	tr := &http.Transport{
		MaxIdleConns:       1,
		IdleConnTimeout:    httpTimeout,
		DisableCompression: true,
	}

	if unixPath != "" {
		tr.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", unixPath)
		}
	} else {
		dialer := &net.Dialer{
			Timeout:   httpTimeout,
			KeepAlive: httpTimeout,
		}
		tr.DialContext = dialer.DialContext
	}

	cli := &http.Client{
		Transport: tr,
		Timeout:   httpTimeout,
	}
	return &APIClient{
		t:    t,
		cli:  cli,
		host: host,
	}
}

func (a *APIClient) request(path string, in, out interface{}, wantErr bool) {
	t := a.t
	var (
		dataIn  []byte
		dataOut []byte
		err     error
	)

	realm := "VolumeDriver"
	if path == "Activate" {
		realm = "Plugin"
	}
	url := fmt.Sprintf("http://%s/%s.%s", a.host, realm, path)

	if str, isString := in.(string); isString {
		dataIn = []byte(str)
	} else {
		dataIn, err = json.Marshal(in)
		require.NoError(t, err)
	}
	fs.Logf(path, "<-- %s", dataIn)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(dataIn))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	res, err := a.cli.Do(req)
	require.NoError(t, err)

	wantStatus := http.StatusOK
	if wantErr {
		wantStatus = http.StatusInternalServerError
	}
	assert.Equal(t, wantStatus, res.StatusCode)

	dataOut, err = ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	err = res.Body.Close()
	require.NoError(t, err)

	if strPtr, isString := out.(*string); isString || wantErr {
		require.True(t, isString, "must use string for error response")
		if wantErr {
			var errRes docker.ErrorResponse
			err = json.Unmarshal(dataOut, &errRes)
			require.NoError(t, err)
			*strPtr = errRes.Err
		} else {
			*strPtr = strings.TrimSpace(string(dataOut))
		}
	} else {
		err = json.Unmarshal(dataOut, out)
		require.NoError(t, err)
	}
	fs.Logf(path, "--> %s", dataOut)
	time.Sleep(tempDelay)
}

func testMountAPI(t *testing.T, sockAddr string) {
	// Disable tests under macOS and linux in the CI since they are locking up
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		testy.SkipUnreliable(t)
	}
	if _, mountFn := mountlib.ResolveMountMethod(""); mountFn == nil {
		t.Skip("Test requires working mount command")
	}

	ctx := context.Background()
	oldCacheDir := config.GetCacheDir()
	testDir, testFs := initialise(ctx, t)
	err := config.SetCacheDir(testDir)
	require.NoError(t, err)
	defer func() {
		_ = config.SetCacheDir(oldCacheDir)
		if !t.Failed() {
			fstest.Purge(testFs)
			_ = os.RemoveAll(testDir)
		}
	}()

	// Prepare API client
	var cli *APIClient
	var unixPath string
	if sockAddr != "" {
		cli = newAPIClient(t, sockAddr, "")
	} else {
		unixPath = filepath.Join(testDir, "rclone.sock")
		cli = newAPIClient(t, "localhost", unixPath)
	}

	// Create mounting volume driver and listen for requests
	drv, err := docker.NewDriver(ctx, testDir, nil, nil, false, true)
	require.NoError(t, err)
	require.NotNil(t, drv)
	defer drv.Exit()

	srv := docker.NewServer(drv)
	go func() {
		var errServe error
		if unixPath != "" {
			errServe = srv.ServeUnix(unixPath, os.Getgid())
		} else {
			errServe = srv.ServeTCP(sockAddr, testDir, nil, false)
		}
		assert.ErrorIs(t, errServe, http.ErrServerClosed)
	}()
	defer func() {
		err := srv.Shutdown(ctx)
		assert.NoError(t, err)
		fs.Logf(nil, "Server stopped")
		time.Sleep(tempDelay)
	}()
	time.Sleep(tempDelay) // Let server start

	// Run test sequence
	path1 := filepath.Join(testDir, "path1")
	require.NoError(t, file.MkdirAll(path1, 0755))
	mount1 := filepath.Join(testDir, "vol1")
	res := ""

	cli.request("Activate", "{}", &res, false)
	assert.Contains(t, res, `"VolumeDriver"`)

	createReq := docker.CreateRequest{
		Name:    "vol1",
		Options: docker.VolOpts{"remote": path1},
	}
	cli.request("Create", createReq, &res, false)
	assert.Equal(t, "{}", res)
	cli.request("Create", createReq, &res, true)
	assert.Contains(t, res, "volume already exists")

	mountReq := docker.MountRequest{Name: "vol1", ID: "id1"}
	var mountRes docker.MountResponse
	cli.request("Mount", mountReq, &mountRes, false)
	assert.Equal(t, mount1, mountRes.Mountpoint)
	cli.request("Mount", mountReq, &res, true)
	assert.Contains(t, res, "already mounted by this id")

	removeReq := docker.RemoveRequest{Name: "vol1"}
	cli.request("Remove", removeReq, &res, true)
	assert.Contains(t, res, "volume is in use")

	text := []byte("banana")
	err = ioutil.WriteFile(filepath.Join(mount1, "txt"), text, 0644)
	assert.NoError(t, err)
	time.Sleep(tempDelay)

	text2, err := ioutil.ReadFile(filepath.Join(path1, "txt"))
	assert.NoError(t, err)
	if runtime.GOOS != "windows" {
		// this check sometimes fails on windows - ignore
		assert.Equal(t, text, text2)
	}

	unmountReq := docker.UnmountRequest{Name: "vol1", ID: "id1"}
	cli.request("Unmount", unmountReq, &res, false)
	assert.Equal(t, "{}", res)
	cli.request("Unmount", unmountReq, &res, true)
	assert.Equal(t, "volume is not mounted", res)

	cli.request("Remove", removeReq, &res, false)
	assert.Equal(t, "{}", res)
	cli.request("Remove", removeReq, &res, true)
	assert.Equal(t, "volume not found", res)

	var listRes docker.ListResponse
	cli.request("List", "{}", &listRes, false)
	assert.Empty(t, listRes.Volumes)
}

func TestDockerPluginMountTCP(t *testing.T) {
	testMountAPI(t, "localhost:53789")
}

func TestDockerPluginMountUnix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is Linux-only")
	}
	testMountAPI(t, "")
}
