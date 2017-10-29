package vfs

import (
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

func TestCreateInfo(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	remote := "file/to/be/created"
	ci := newCreateInfo(r.Fremote, remote)

	// Test methods
	assert.Equal(t, r.Fremote, ci.Fs())
	assert.Equal(t, remote, ci.String())
	assert.Equal(t, remote, ci.Remote())
	_, err := ci.Hash(fs.HashMD5)
	assert.Equal(t, fs.ErrHashUnsupported, err)
	assert.WithinDuration(t, time.Now(), ci.ModTime(), time.Second)
	assert.Equal(t, int64(0), ci.Size())
	assert.Equal(t, true, ci.Storable())

}
