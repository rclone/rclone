// Package fstest provides utilities for testing the Fs
package fstest

// FIXME put name of test FS in Fs structure

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
)

// Seed the random number generator
func init() {
	rand.Seed(time.Now().UnixNano())

}

// Item represents an item for checking
type Item struct {
	Path    string
	Hashes  map[fs.HashType]string
	ModTime time.Time
	Size    int64
	WinPath string
}

// NewItem creates an item from a string content
func NewItem(Path, Content string, modTime time.Time) Item {
	i := Item{
		Path:    Path,
		ModTime: modTime,
		Size:    int64(len(Content)),
	}
	hash := fs.NewMultiHasher()
	buf := bytes.NewBufferString(Content)
	_, err := io.Copy(hash, buf)
	if err != nil {
		log.Fatalf("Failed to create item: %v", err)
	}
	i.Hashes = hash.Sums()
	return i
}

// CheckTimeEqualWithPrecision checks the times are equal within the
// precision, returns the delta and a flag
func CheckTimeEqualWithPrecision(t0, t1 time.Time, precision time.Duration) (time.Duration, bool) {
	dt := t0.Sub(t1)
	if dt >= precision || dt <= -precision {
		return dt, false
	}
	return dt, true
}

// CheckModTime checks the mod time to the given precision
func (i *Item) CheckModTime(t *testing.T, obj fs.Object, modTime time.Time, precision time.Duration) {
	dt, ok := CheckTimeEqualWithPrecision(modTime, i.ModTime, precision)
	if !ok {
		t.Errorf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", obj.Remote(), dt, precision, modTime, i.ModTime, precision)
	}
}

// CheckHashes checks all the hashes the object supports are correct
func (i *Item) CheckHashes(t *testing.T, obj fs.Object) {
	if obj == nil {
		t.Fatalf("Object is nil")
	}
	types := obj.Fs().Hashes().Array()
	for _, hash := range types {
		// Check attributes
		sum, err := obj.Hash(hash)
		if err != nil {
			t.Fatalf("%s: Failed to read hash %v for %q: %v", obj.Fs().String(), hash, obj.Remote(), err)
		}
		if !fs.HashEquals(i.Hashes[hash], sum) {
			t.Errorf("%s/%s: %v hash incorrect - expecting %q got %q", obj.Fs().String(), obj.Remote(), hash, i.Hashes[hash], sum)
		}
	}
}

// Check checks all the attributes of the object are correct
func (i *Item) Check(t *testing.T, obj fs.Object, precision time.Duration) {
	i.CheckHashes(t, obj)
	if i.Size != obj.Size() {
		t.Errorf("%s/%s: Size incorrect - expecting %d got %d", obj.Fs().String(), obj.Remote(), i.Size, obj.Size())
	}
	i.CheckModTime(t, obj, obj.ModTime(), precision)
}

// Items represents all items for checking
type Items struct {
	byName    map[string]*Item
	byNameAlt map[string]*Item
	items     []Item
}

// NewItems makes an Items
func NewItems(items []Item) *Items {
	is := &Items{
		byName:    make(map[string]*Item),
		byNameAlt: make(map[string]*Item),
		items:     items,
	}
	// Fill up byName
	for i := range items {
		is.byName[items[i].Path] = &items[i]
		is.byNameAlt[items[i].WinPath] = &items[i]
	}
	return is
}

// Find checks off an item
func (is *Items) Find(t *testing.T, obj fs.Object, precision time.Duration) {
	i, ok := is.byName[obj.Remote()]
	if !ok {
		i, ok = is.byNameAlt[obj.Remote()]
		if !ok {
			t.Errorf("Unexpected file %q", obj.Remote())
			return
		}
	}
	delete(is.byName, i.Path)
	delete(is.byName, i.WinPath)
	i.Check(t, obj, precision)
}

// Done checks all finished
func (is *Items) Done(t *testing.T) {
	if len(is.byName) != 0 {
		for name := range is.byName {
			t.Logf("Not found %q", name)
		}
		t.Errorf("%d objects not found", len(is.byName))
	}
}

// CheckListingWithPrecision checks the fs to see if it has the
// expected contents with the given precision.
func CheckListingWithPrecision(t *testing.T, f fs.Fs, items []Item, precision time.Duration) {
	is := NewItems(items)
	oldErrors := fs.Stats.GetErrors()
	var objs []fs.Object
	const retries = 6
	sleep := time.Second / 2
	for i := 1; i <= retries; i++ {
		objs = nil
		for obj := range f.List() {
			objs = append(objs, obj)
		}
		if len(objs) == len(items) {
			// Put an extra sleep in if we did any retries just to make sure it really
			// is consistent (here is looking at you Amazon Cloud Drive!)
			if i != 1 {
				extraSleep := 5*time.Second + sleep
				t.Logf("Sleeping for %v just to make sure", extraSleep)
				time.Sleep(extraSleep)
			}
			break
		}
		sleep *= 2
		t.Logf("Sleeping for %v for list eventual consistency: %d/%d", sleep, i, retries)
		time.Sleep(sleep)
	}
	for _, obj := range objs {
		if obj == nil {
			t.Errorf("Unexpected nil in List()")
			continue
		}
		is.Find(t, obj, precision)
	}
	is.Done(t)
	// Don't notice an error when listing an empty directory
	if len(items) == 0 && oldErrors == 0 && fs.Stats.GetErrors() == 1 {
		fs.Stats.ResetErrors()
	}
}

// CheckListing checks the fs to see if it has the expected contents
func CheckListing(t *testing.T, f fs.Fs, items []Item) {
	precision := f.Precision()
	CheckListingWithPrecision(t, f, items, precision)
}

// CheckItems checks the fs to see if it has only the items passed in
// using a precision of fs.Config.ModifyWindow
func CheckItems(t *testing.T, f fs.Fs, items ...Item) {
	CheckListingWithPrecision(t, f, items, fs.Config.ModifyWindow)
}

// Time parses a time string or logs a fatal error
func Time(timeString string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		log.Fatalf("Failed to parse time %q: %v", timeString, err)
	}
	return t
}

// RandomString create a random string for test purposes
func RandomString(n int) string {
	source := "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	for i := range out {
		out[i] = source[rand.Intn(len(source))]
	}
	return string(out)
}

// LocalRemote creates a temporary directory name for local remotes
func LocalRemote() (path string, err error) {
	path, err = ioutil.TempDir("", "rclone")
	if err == nil {
		// Now remove the directory
		err = os.Remove(path)
	}
	path = filepath.ToSlash(path)
	return
}

// RandomRemoteName makes a random bucket or subdirectory name
//
// Returns a random remote name plus the leaf name
func RandomRemoteName(remoteName string) (string, string, error) {
	var err error
	var leafName string

	// Make a directory if remote name is null
	if remoteName == "" {
		remoteName, err = LocalRemote()
		if err != nil {
			return "", "", err
		}
	} else {
		if !strings.HasSuffix(remoteName, ":") {
			remoteName += "/"
		}
		leafName = RandomString(32)
		remoteName += leafName
	}
	return remoteName, leafName, nil
}

// RandomRemote makes a random bucket or subdirectory on the remote
//
// Call the finalise function returned to Purge the fs at the end (and
// the parent if necessary)
func RandomRemote(remoteName string, subdir bool) (fs.Fs, func(), error) {
	var err error
	var parentRemote fs.Fs

	remoteName, _, err = RandomRemoteName(remoteName)
	if err != nil {
		return nil, nil, err
	}

	if subdir {
		parentRemote, err = fs.NewFs(remoteName)
		if err != nil {
			return nil, nil, err
		}
		remoteName += "/" + RandomString(8)
	}

	remote, err := fs.NewFs(remoteName)
	if err != nil {
		return nil, nil, err
	}

	finalise := func() {
		_ = fs.Purge(remote) // ignore error
		if parentRemote != nil {
			err = fs.Purge(parentRemote) // ignore error
			if err != nil {
				log.Printf("Failed to purge %v: %v", parentRemote, err)
			}
		}
	}

	return remote, finalise, nil
}

// TestMkdir tests Mkdir works
func TestMkdir(t *testing.T, remote fs.Fs) {
	err := fs.Mkdir(remote)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	CheckListing(t, remote, []Item{})
}

// TestPurge tests Purge works
func TestPurge(t *testing.T, remote fs.Fs) {
	err := fs.Purge(remote)
	if err != nil {
		t.Fatalf("Purge failed: %v", err)
	}
	CheckListing(t, remote, []Item{})
}

// TestRmdir tests Rmdir works
func TestRmdir(t *testing.T, remote fs.Fs) {
	err := fs.Rmdir(remote)
	if err != nil {
		t.Fatalf("Rmdir failed: %v", err)
	}
}
