/*

This provides Run for use in creating test suites

To use this declare a TestMain

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

And then make and destroy a Run in each test

func TestMkdir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	// test stuff
}

This will make r.Fremote and r.Flocal for a remote and a local
remote.  The remote is determined by the -remote flag passed in.

*/

package fstest

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run holds the remotes for a test run
type Run struct {
	LocalName    string
	Flocal       fs.Fs
	Fremote      fs.Fs
	FremoteName  string
	Precision    time.Duration
	cleanRemote  func()
	mkdir        map[string]bool // whether the remote has been made yet for the fs name
	Logf, Fatalf func(text string, args ...interface{})
}

// TestMain drives the tests
func TestMain(m *testing.M) {
	flag.Parse()
	if !*Individual {
		oneRun = newRun()
	}
	rc := m.Run()
	if !*Individual {
		oneRun.Finalise()
	}
	os.Exit(rc)
}

// oneRun holds the master run data if individual is not set
var oneRun *Run

// newRun initialise the remote and local for testing and returns a
// run object.
//
// r.Flocal is an empty local Fs
// r.Fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func newRun() *Run {
	r := &Run{
		Logf:   log.Printf,
		Fatalf: log.Fatalf,
		mkdir:  make(map[string]bool),
	}

	Initialise()

	var err error
	r.Fremote, r.FremoteName, r.cleanRemote, err = RandomRemote()
	if err != nil {
		r.Fatalf("Failed to open remote %q: %v", *RemoteName, err)
	}

	r.LocalName, err = os.MkdirTemp("", "rclone")
	if err != nil {
		r.Fatalf("Failed to create temp dir: %v", err)
	}
	r.LocalName = filepath.ToSlash(r.LocalName)
	r.Flocal, err = fs.NewFs(context.Background(), r.LocalName)
	if err != nil {
		r.Fatalf("Failed to make %q: %v", r.LocalName, err)
	}

	r.Precision = fs.GetModifyWindow(context.Background(), r.Fremote, r.Flocal)

	return r
}

// run f(), retrying it until it returns with no error or the limit
// expires and it calls t.Fatalf
func retry(t *testing.T, what string, f func() error) {
	var err error
	for try := 1; try <= *ListRetries; try++ {
		err = f()
		if err == nil {
			return
		}
		t.Logf("%s failed - try %d/%d: %v", what, try, *ListRetries, err)
		time.Sleep(time.Second)
	}
	t.Logf("%s failed: %v", what, err)
}

// newRunIndividual initialise the remote and local for testing and
// returns a run object.  Pass in true to make individual tests or
// false to use the global one.
//
// r.Flocal is an empty local Fs
// r.Fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func newRunIndividual(t *testing.T, individual bool) *Run {
	ctx := context.Background()
	var r *Run
	if individual {
		r = newRun()
	} else {
		// If not individual, use the global one with the clean method overridden
		r = new(Run)
		*r = *oneRun
		r.cleanRemote = func() {
			var toDelete []string
			err := walk.ListR(ctx, r.Fremote, "", true, -1, walk.ListAll, func(entries fs.DirEntries) error {
				for _, entry := range entries {
					switch x := entry.(type) {
					case fs.Object:
						retry(t, fmt.Sprintf("removing file %q", x.Remote()), func() error { return x.Remove(ctx) })
					case fs.Directory:
						toDelete = append(toDelete, x.Remote())
					}
				}
				return nil
			})
			if err == fs.ErrorDirNotFound {
				return
			}
			require.NoError(t, err)
			sort.Strings(toDelete)
			for i := len(toDelete) - 1; i >= 0; i-- {
				dir := toDelete[i]
				retry(t, fmt.Sprintf("removing dir %q", dir), func() error {
					return r.Fremote.Rmdir(ctx, dir)
				})
			}
			// Check remote is empty
			CheckListingWithPrecision(t, r.Fremote, []Item{}, []string{}, r.Fremote.Precision())
			// Clear the remote cache
			cache.Clear()
		}
	}
	r.Logf = t.Logf
	r.Fatalf = t.Fatalf
	r.Logf("Remote %q, Local %q, Modify Window %q", r.Fremote, r.Flocal, fs.GetModifyWindow(ctx, r.Fremote))
	t.Cleanup(r.Finalise)
	return r
}

// NewRun initialise the remote and local for testing and returns a
// run object.  Call this from the tests.
//
// r.Flocal is an empty local Fs
// r.Fremote is an empty remote Fs
func NewRun(t *testing.T) *Run {
	return newRunIndividual(t, *Individual)
}

// NewRunIndividual as per NewRun but makes an individual remote for this test
func NewRunIndividual(t *testing.T) *Run {
	return newRunIndividual(t, true)
}

// RenameFile renames a file in local
func (r *Run) RenameFile(item Item, newpath string) Item {
	oldFilepath := path.Join(r.LocalName, item.Path)
	newFilepath := path.Join(r.LocalName, newpath)
	if err := os.Rename(oldFilepath, newFilepath); err != nil {
		r.Fatalf("Failed to rename file from %q to %q: %v", item.Path, newpath, err)
	}

	item.Path = newpath

	return item
}

// WriteFile writes a file to local
func (r *Run) WriteFile(filePath, content string, t time.Time) Item {
	item := NewItem(filePath, content, t)
	// FIXME make directories?
	filePath = path.Join(r.LocalName, filePath)
	dirPath := path.Dir(filePath)
	err := file.MkdirAll(dirPath, 0770)
	if err != nil {
		r.Fatalf("Failed to make directories %q: %v", dirPath, err)
	}
	err = os.WriteFile(filePath, []byte(content), 0600)
	if err != nil {
		r.Fatalf("Failed to write file %q: %v", filePath, err)
	}
	err = os.Chtimes(filePath, t, t)
	if err != nil {
		r.Fatalf("Failed to chtimes file %q: %v", filePath, err)
	}
	return item
}

// ForceMkdir creates the remote
func (r *Run) ForceMkdir(ctx context.Context, f fs.Fs) {
	err := f.Mkdir(ctx, "")
	if err != nil {
		r.Fatalf("Failed to mkdir %q: %v", f, err)
	}
	r.mkdir[f.String()] = true
}

// Mkdir creates the remote if it hasn't been created already
func (r *Run) Mkdir(ctx context.Context, f fs.Fs) {
	if !r.mkdir[f.String()] {
		r.ForceMkdir(ctx, f)
	}
}

// WriteObjectTo writes an object to the fs, remote passed in
func (r *Run) WriteObjectTo(ctx context.Context, f fs.Fs, remote, content string, modTime time.Time, useUnchecked bool) Item {
	put := f.Put
	if useUnchecked {
		put = f.Features().PutUnchecked
		if put == nil {
			r.Fatalf("Fs doesn't support PutUnchecked")
		}
	}
	r.Mkdir(ctx, f)

	// calculate all hashes f supports for content
	hash, err := hash.NewMultiHasherTypes(f.Hashes())
	if err != nil {
		r.Fatalf("Failed to make new multi hasher: %v", err)
	}
	_, err = hash.Write([]byte(content))
	if err != nil {
		r.Fatalf("Failed to make write to hash: %v", err)
	}
	hashSums := hash.Sums()

	const maxTries = 10
	for tries := 1; ; tries++ {
		in := bytes.NewBufferString(content)
		objinfo := object.NewStaticObjectInfo(remote, modTime, int64(len(content)), true, hashSums, nil)
		_, err := put(ctx, in, objinfo)
		if err == nil {
			break
		}
		// Retry if err returned a retry error
		if fserrors.IsRetryError(err) && tries < maxTries {
			r.Logf("Retry Put of %q to %v: %d/%d (%v)", remote, f, tries, maxTries, err)
			time.Sleep(2 * time.Second)
			continue
		}
		r.Fatalf("Failed to put %q to %q: %v", remote, f, err)
	}
	return NewItem(remote, content, modTime)
}

// WriteObject writes an object to the remote
func (r *Run) WriteObject(ctx context.Context, remote, content string, modTime time.Time) Item {
	return r.WriteObjectTo(ctx, r.Fremote, remote, content, modTime, false)
}

// WriteUncheckedObject writes an object to the remote not checking for duplicates
func (r *Run) WriteUncheckedObject(ctx context.Context, remote, content string, modTime time.Time) Item {
	return r.WriteObjectTo(ctx, r.Fremote, remote, content, modTime, true)
}

// WriteBoth calls WriteObject and WriteFile with the same arguments
func (r *Run) WriteBoth(ctx context.Context, remote, content string, modTime time.Time) Item {
	r.WriteFile(remote, content, modTime)
	return r.WriteObject(ctx, remote, content, modTime)
}

// CheckWithDuplicates does a test but allows duplicates
func (r *Run) CheckWithDuplicates(t *testing.T, items ...Item) {
	var want, got []string

	// construct a []string of desired items
	for _, item := range items {
		want = append(want, fmt.Sprintf("%q %d", item.Path, item.Size))
	}
	sort.Strings(want)

	// do the listing
	objs, _, err := walk.GetAll(context.Background(), r.Fremote, "", true, -1)
	if err != nil && err != fs.ErrorDirNotFound {
		t.Fatalf("Error listing: %v", err)
	}

	// construct a []string of actual items
	for _, o := range objs {
		got = append(got, fmt.Sprintf("%q %d", o.Remote(), o.Size()))
	}
	sort.Strings(got)

	assert.Equal(t, want, got)
}

// CheckLocalItems checks the local fs with proper precision
// to see if it has the expected items.
func (r *Run) CheckLocalItems(t *testing.T, items ...Item) {
	CheckItemsWithPrecision(t, r.Flocal, r.Precision, items...)
}

// CheckRemoteItems checks the remote fs with proper precision
// to see if it has the expected items.
func (r *Run) CheckRemoteItems(t *testing.T, items ...Item) {
	CheckItemsWithPrecision(t, r.Fremote, r.Precision, items...)
}

// CheckLocalListing checks the local fs with proper precision
// to see if it has the expected contents.
//
// If expectedDirs is non nil then we check those too.  Note that no
// directories returned is also OK as some remotes don't return
// directories.
func (r *Run) CheckLocalListing(t *testing.T, items []Item, expectedDirs []string) {
	CheckListingWithPrecision(t, r.Flocal, items, expectedDirs, r.Precision)
}

// CheckRemoteListing checks the remote fs with proper precision
// to see if it has the expected contents.
//
// If expectedDirs is non nil then we check those too.  Note that no
// directories returned is also OK as some remotes don't return
// directories.
func (r *Run) CheckRemoteListing(t *testing.T, items []Item, expectedDirs []string) {
	CheckListingWithPrecision(t, r.Fremote, items, expectedDirs, r.Precision)
}

// CheckDirectoryModTimes checks that the directory names in r.Flocal has the correct modtime compared to r.Fremote
func (r *Run) CheckDirectoryModTimes(t *testing.T, names ...string) {
	if r.Fremote.Features().DirSetModTime == nil && r.Fremote.Features().MkdirMetadata == nil {
		fs.Debugf(r.Fremote, "Skipping modtime test as remote does not support DirSetModTime or MkdirMetadata")
		return
	}
	ctx := context.Background()
	for _, name := range names {
		wantT := NewDirectory(ctx, t, r.Flocal, name).ModTime(ctx)
		got := NewDirectory(ctx, t, r.Fremote, name)
		CheckDirModTime(ctx, t, r.Fremote, got, wantT)
	}
}

// Clean the temporary directory
func (r *Run) cleanTempDir() {
	err := os.RemoveAll(r.LocalName)
	if err != nil {
		r.Logf("Failed to clean temporary directory %q: %v", r.LocalName, err)
	}
}

// Finalise cleans the remote and local
func (r *Run) Finalise() {
	// r.Logf("Cleaning remote %q", r.Fremote)
	r.cleanRemote()
	// r.Logf("Cleaning local %q", r.LocalName)
	r.cleanTempDir()
	// Clear the remote cache
	cache.Clear()
}
