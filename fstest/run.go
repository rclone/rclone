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

This will make r.Fremote and r.Flocal for a remote remote and a local
remote.  The remote is determined by the -remote flag passed in.

*/

package fstest

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run holds the remotes for a test run
type Run struct {
	LocalName    string
	Flocal       fs.Fs
	Fremote      fs.Fs
	FremoteName  string
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
	r.Fremote, r.FremoteName, r.cleanRemote, err = RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		r.Fatalf("Failed to open remote %q: %v", *RemoteName, err)
	}

	r.LocalName, err = ioutil.TempDir("", "rclone")
	if err != nil {
		r.Fatalf("Failed to create temp dir: %v", err)
	}
	r.LocalName = filepath.ToSlash(r.LocalName)
	r.Flocal, err = fs.NewFs(r.LocalName)
	if err != nil {
		r.Fatalf("Failed to make %q: %v", r.LocalName, err)
	}
	fs.CalculateModifyWindow(r.Fremote, r.Flocal)
	return r
}

// dirsToRemove sorts by string length
type dirsToRemove []string

func (d dirsToRemove) Len() int           { return len(d) }
func (d dirsToRemove) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d dirsToRemove) Less(i, j int) bool { return len(d[i]) > len(d[j]) }

// NewRun initialise the remote and local for testing and returns a
// run object.  Call this from the tests.
//
// r.Flocal is an empty local Fs
// r.Fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func NewRun(t *testing.T) *Run {
	var r *Run
	if *Individual {
		r = newRun()
	} else {
		// If not individual, use the global one with the clean method overridden
		r = new(Run)
		*r = *oneRun
		r.cleanRemote = func() {
			var toDelete dirsToRemove
			err := fs.Walk(r.Fremote, "", true, -1, func(dirPath string, entries fs.DirEntries, err error) error {
				if err != nil {
					if err == fs.ErrorDirNotFound {
						return nil
					}
					t.Fatalf("Error listing: %v", err)
				}
				for _, entry := range entries {
					switch x := entry.(type) {
					case fs.Object:
						err = x.Remove()
						if err != nil {
							t.Errorf("Error removing file %q: %v", x.Remote(), err)
						}
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
			sort.Sort(toDelete)
			for _, dir := range toDelete {
				err := r.Fremote.Rmdir(dir)
				if err != nil {
					t.Errorf("Error removing dir %q: %v", dir, err)
				}
			}
			// Check remote is empty
			CheckListingWithPrecision(t, r.Fremote, []Item{}, []string{}, r.Fremote.Precision())
		}
	}
	r.Logf = t.Logf
	r.Fatalf = t.Fatalf
	r.Logf("Remote %q, Local %q, Modify Window %q", r.Fremote, r.Flocal, fs.Config.ModifyWindow)
	return r
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
	err := os.MkdirAll(dirPath, 0770)
	if err != nil {
		r.Fatalf("Failed to make directories %q: %v", dirPath, err)
	}
	err = ioutil.WriteFile(filePath, []byte(content), 0600)
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
func (r *Run) ForceMkdir(f fs.Fs) {
	err := f.Mkdir("")
	if err != nil {
		r.Fatalf("Failed to mkdir %q: %v", f, err)
	}
	r.mkdir[f.String()] = true
}

// Mkdir creates the remote if it hasn't been created already
func (r *Run) Mkdir(f fs.Fs) {
	if !r.mkdir[f.String()] {
		r.ForceMkdir(f)
	}
}

// WriteObjectTo writes an object to the fs, remote passed in
func (r *Run) WriteObjectTo(f fs.Fs, remote, content string, modTime time.Time, useUnchecked bool) Item {
	put := f.Put
	if useUnchecked {
		put = f.Features().PutUnchecked
		if put == nil {
			r.Fatalf("Fs doesn't support PutUnchecked")
		}
	}
	r.Mkdir(f)
	const maxTries = 10
	for tries := 1; ; tries++ {
		in := bytes.NewBufferString(content)
		objinfo := fs.NewStaticObjectInfo(remote, modTime, int64(len(content)), true, nil, nil)
		_, err := put(in, objinfo)
		if err == nil {
			break
		}
		// Retry if err returned a retry error
		if fs.IsRetryError(err) && tries < maxTries {
			r.Logf("Retry Put of %q to %v: %d/%d (%v)", remote, f, tries, maxTries, err)
			time.Sleep(2 * time.Second)
			continue
		}
		r.Fatalf("Failed to put %q to %q: %v", remote, f, err)
	}
	return NewItem(remote, content, modTime)
}

// WriteObject writes an object to the remote
func (r *Run) WriteObject(remote, content string, modTime time.Time) Item {
	return r.WriteObjectTo(r.Fremote, remote, content, modTime, false)
}

// WriteUncheckedObject writes an object to the remote not checking for duplicates
func (r *Run) WriteUncheckedObject(remote, content string, modTime time.Time) Item {
	return r.WriteObjectTo(r.Fremote, remote, content, modTime, true)
}

// WriteBoth calls WriteObject and WriteFile with the same arguments
func (r *Run) WriteBoth(remote, content string, modTime time.Time) Item {
	r.WriteFile(remote, content, modTime)
	return r.WriteObject(remote, content, modTime)
}

// CheckWithDuplicates does a test but allows duplicates
func (r *Run) CheckWithDuplicates(t *testing.T, items ...Item) {
	objects, size, err := fs.Count(r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(len(items)), objects)
	wantSize := int64(0)
	for _, item := range items {
		wantSize += item.Size
	}
	assert.Equal(t, wantSize, size)
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
}
