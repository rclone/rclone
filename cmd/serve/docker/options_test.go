package docker

import (
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/local"
)

func TestApplyOptions(t *testing.T) {
	vol := &Volume{
		Name:       "testName",
		MountPoint: "testPath",
		drv: &Driver{
			root: "testRoot",
		},
		mnt: &mountlib.MountPoint{
			MountPoint: "testPath",
		},
		mountReqs: make(map[string]any),
	}

	// Happy path
	volOpt := VolOpts{
		"remote":     "/tmp/docker",
		"persist":    "FALSE",
		"mount_type": "potato",
		// backend options
		"--local-case-sensitive": "true",
		"local_no_check_updated": "1",
		// mount options
		"debug-fuse":   "true",
		"attr_timeout": "100s",
		"--async-read": "TRUE",
		// vfs options
		"no-modtime":  "1",
		"no_checksum": "true",
		"--no-seek":   "true",
	}
	err := vol.applyOptions(volOpt)
	require.NoError(t, err)
	// normal options
	assert.Equal(t, ":local,case_sensitive='true',no_check_updated='1':/tmp/docker", vol.fsString)
	assert.Equal(t, false, vol.persist)
	assert.Equal(t, "potato", vol.mountType)
	// mount options
	assert.Equal(t, true, vol.mnt.MountOpt.DebugFUSE)
	assert.Equal(t, fs.Duration(100*time.Second), vol.mnt.MountOpt.AttrTimeout)
	assert.Equal(t, true, vol.mnt.MountOpt.AsyncRead)
	// vfs options
	assert.Equal(t, true, vol.mnt.VFSOpt.NoModTime)
	assert.Equal(t, true, vol.mnt.VFSOpt.NoChecksum)
	assert.Equal(t, true, vol.mnt.VFSOpt.NoSeek)

	// Check errors
	err = vol.applyOptions(VolOpts{
		"debug-fuse": "POTATO",
	})
	require.ErrorContains(t, err, "cannot parse mount options")
	err = vol.applyOptions(VolOpts{
		"no-modtime": "POTATO",
	})
	require.ErrorContains(t, err, "cannot parse vfs options")
	err = vol.applyOptions(VolOpts{
		"remote":          "/tmp/docker",
		"local_not_found": "POTATO",
	})
	require.ErrorContains(t, err, "unsupported backend option")

}
