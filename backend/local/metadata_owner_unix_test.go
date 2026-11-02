//go:build !windows && !plan9 && !js

package local

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataOwnershipAsNonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("needs a user other than root")
	}
	uid, gid := os.Geteuid(), os.Getegid()
	other := uid + 1
	notMember := gid + 1
	for slices.Contains(processGroups(), notMember) {
		notMember++
	}

	// only what a non-root user may set: its own uid, its groups
	setUID, setGID := ownerToSet(other, notMember)
	assert.Equal(t, [2]int{-1, -1}, [2]int{setUID, setGID})
	setUID, setGID = ownerToSet(other, gid)
	assert.Equal(t, [2]int{-1, gid}, [2]int{setUID, setGID})
	setUID, setGID = ownerToSet(uid, notMember)
	assert.Equal(t, [2]int{uid, -1}, [2]int{setUID, setGID})

	// a file of another user (e.g. root:root) is copied with our ownership
	// instead of failing with "operation not permitted"
	r := fstest.NewRun(t)
	r.WriteFile("owned.txt", "payload", time.Now())
	f := r.Flocal.(*Fs)
	obj, err := f.NewObject(context.Background(), "owned.txt")
	require.NoError(t, err)
	require.NoError(t, obj.(*Object).writeMetadataToFile(fs.Metadata{"uid": "0", "gid": "0"}))
	fi, err := os.Stat(filepath.Join(f.root, "owned.txt"))
	require.NoError(t, err)
	assert.Equal(t, uint32(uid), fi.Sys().(*syscall.Stat_t).Uid)
}
