// Test for the --fix-case "destination removed for re-upload" branch
// introduced in commit f60213545. The local fs supports in-place
// modtime updates, so a wrapper backend is needed to mimic the
// Dropbox-class behaviour where SetModTime returns
// ErrorCantSetModTimeWithoutDelete and equal() removes the
// destination before Sync's fix-case logic runs.

package sync

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/require"
)

// cantSetMtimeFs wraps an fs.Fs to make it appear CaseInsensitive and
// to force Object.SetModTime to return
// fs.ErrorCantSetModTimeWithoutDelete -- the way Dropbox does.
//
// Only the Fs/Object methods that the fix-case path touches are
// overridden; the rest delegate to the embedded interface.
type cantSetMtimeFs struct {
	fs.Fs
}

func (f *cantSetMtimeFs) Features() *fs.Features {
	feat := *f.Fs.Features()
	feat.CaseInsensitive = true
	return &feat
}

func (f *cantSetMtimeFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	return &cantSetMtimeObj{Object: o, parent: f}, nil
}

func (f *cantSetMtimeFs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	entries, err := f.Fs.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		if o, ok := e.(fs.Object); ok {
			entries[i] = &cantSetMtimeObj{Object: o, parent: f}
		}
	}
	return entries, nil
}

type cantSetMtimeObj struct {
	fs.Object
	parent *cantSetMtimeFs
}

func (o *cantSetMtimeObj) Fs() fs.Info {
	return o.parent
}

func (o *cantSetMtimeObj) SetModTime(ctx context.Context, _ time.Time) error {
	return fs.ErrorCantSetModTimeWithoutDelete
}

// TestFixCaseDestRemovedForReupload exercises the branch added in
// commit f60213545: when --fix-case is enabled and NeedTransfer
// removed pair.Dst (because SetModTime returned
// ErrorCantSetModTimeWithoutDelete), Sync must detect the now-gone
// destination via NewObject rather than attempting Move on it.
//
// TestFixCase doesn't cover this branch because the local FS supports
// in-place modtime updates -- the SetModTime delete path is
// unreachable from there. Without this regression test the bug
// (originally surfaced as "from_lookup/not_found" on Dropbox) is only
// visible in the Dropbox integration suite, the slowest feedback loop
// in the project, and the commit dance
// (de67f29b3 -> 92058f15c revert -> f60213545 reland) shows the class
// has already been re-introduced once.
func TestFixCaseDestRemovedForReupload(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	ci.FixCase = true

	fakeRemote := &cantSetMtimeFs{Fs: r.Fremote}
	require.True(t, fakeRemote.Features().CaseInsensitive,
		"wrapper must self-report CaseInsensitive=true so the fix-case branch is taken")

	// Same content (same hash, same size) but different modtime and
	// different case. equal() will see modtime mismatch, hash match,
	// try SetModTime -> our ErrorCantSetModTimeWithoutDelete ->
	// dst.Remove() removes the underlying file. Sync's fix-case
	// branch must then detect the missing destination via NewObject
	// and let the upload recreate the file at "hello".
	file1 := r.WriteFile("hello", "potato", t1)
	r.WriteObject(ctx, "HELLO", "potato", t2)

	accounting.GlobalStats().ResetCounters()
	err := Sync(ctx, fakeRemote, r.Flocal, false)
	require.NoError(t, err,
		"Sync must not error when fix-case destination was removed by equal() during NeedTransfer")

	// The underlying remote should now contain a file at the
	// source's case ("hello"). r.CheckRemoteItems lists r.Fremote
	// directly, which is the wrapped backend.
	r.CheckRemoteItems(t, file1)
}
