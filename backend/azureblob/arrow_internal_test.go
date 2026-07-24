//go:build !plan9 && !solaris && !js

package azureblob

import (
	"context"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testArrowList exercises the experimental Apache Arrow listing.
//
// It is skipped automatically if Arrow listing is not enabled on the test
// account (the server returns XML, or 409 on HNS accounts).
func (f *Fs) testArrowList(t *testing.T) {
	ctx := context.Background()
	containerName, _ := f.split("probe")
	if containerName == "" {
		t.Skip("Arrow list test needs a container in the remote root")
	}

	// withArrow runs fn with UseArrowList temporarily set.
	withArrow := func(on bool, fn func()) {
		old := f.opt.UseArrowList
		f.opt.UseArrowList = on
		defer func() { f.opt.UseArrowList = old }()
		fn()
	}

	// Create a set of known objects under a unique directory: flat files plus
	// one in a subdirectory (so we exercise both the flat and delimited paths).
	dir := "arrow-list-" + random.String(8)
	// Includes upper-case and digit first-characters: these byte-sort between
	// the digit and lowercase ladder boundaries, so they verify the server's
	// endBefore filtering is byte-ordered (not case-insensitive, which would
	// drop them from a shard and make parallel != sequential).
	objects := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "sub/x.txt", "Foo.txt", "Zoo.txt", "0num.txt", "9end.txt"}
	wantSizes := map[string]int64{}
	created := make([]string, 0, len(objects))
	for i, n := range objects {
		contents := random.String(10 + i)
		remote := dir + "/" + n
		item := fstest.NewItem(remote, contents, fstest.Time("2001-05-06T04:05:06.499Z"))
		_ = fstests.PutTestContents(ctx, t, f, &item, contents, true)
		created = append(created, remote)
		wantSizes[remote] = int64(len(contents))
	}
	defer func() {
		for _, remote := range created {
			if o, err := f.NewObject(ctx, remote); err == nil {
				_ = o.Remove(ctx)
			}
		}
	}()

	// listing returns remote->size for dir (dirs reported as size -1).
	listing := func(recurse bool) (map[string]int64, error) {
		got := map[string]int64{}
		add := func(entries fs.DirEntries) error {
			for _, e := range entries {
				if o, ok := e.(fs.Object); ok {
					got[e.Remote()] = o.Size()
				} else {
					got[e.Remote()] = -1
				}
			}
			return nil
		}
		if recurse {
			return got, f.ListR(ctx, dir, add)
		}
		entries, err := f.List(ctx, dir)
		if err != nil {
			return nil, err
		}
		_ = add(entries)
		return got, nil
	}

	// Probe: is Arrow available? If not, skip cleanly.
	var probeErr error
	withArrow(true, func() { _, probeErr = listing(true) })
	if probeErr != nil {
		if strings.Contains(probeErr.Error(), "OperationNotSupportedWithFeatureMissing") || strings.Contains(probeErr.Error(), "409") {
			t.Skipf("Arrow listing not available on this account: %v", probeErr)
		}
		require.NoError(t, probeErr)
	}

	// Arrow listing must equal XML listing, recursively and per-directory
	// (including the "sub" subdirectory).
	t.Run("EqualsXML", func(t *testing.T) {
		for _, recurse := range []bool{true, false} {
			var xml, arrow map[string]int64
			var err error
			withArrow(false, func() { xml, err = listing(recurse) })
			require.NoError(t, err)
			withArrow(true, func() { arrow, err = listing(recurse) })
			require.NoError(t, err)
			assert.Equal(t, xml, arrow, "recurse=%v: Arrow listing differs from XML", recurse)
		}

		// Recursive Arrow listing contains every object with the right size.
		var arrow map[string]int64
		withArrow(true, func() { arrow, _ = listing(true) })
		for remote, size := range wantSizes {
			assert.Equal(t, size, arrow[remote], remote)
		}

		// Single-directory Arrow listing reports "sub" as a directory.
		var entries fs.DirEntries
		withArrow(true, func() {
			var err error
			entries, err = f.List(ctx, dir)
			require.NoError(t, err)
		})
		foundSubDir := false
		for _, e := range entries {
			if e.Remote() == dir+"/sub" {
				_, isObj := e.(fs.Object)
				foundSubDir = !isObj
			}
		}
		assert.True(t, foundSubDir, "expected %q to be listed as a directory", dir+"/sub")
	})
}
