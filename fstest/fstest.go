// Utilities for testing the fs

package fstest

// FIXME put name of test FS in Fs structure

import (
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
)

// Seed the random number generator
func init() {
	rand.Seed(time.Now().UnixNano())

}

// Represents an item for checking
type Item struct {
	Path    string
	Md5sum  string
	ModTime time.Time
	Size    int64
}

// check the mod time to the given precision
func (i *Item) CheckModTime(t *testing.T, obj fs.Object, modTime time.Time, precision time.Duration) {
	dt := modTime.Sub(i.ModTime)
	if dt >= precision || dt <= -precision {
		t.Errorf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", obj.Remote(), dt, precision, modTime, i.ModTime, precision)
	}
}

func (i *Item) Check(t *testing.T, obj fs.Object, precision time.Duration) {
	if obj == nil {
		t.Fatalf("Object is nil")
	}
	// Check attributes
	Md5sum, err := obj.Md5sum()
	if err != nil {
		t.Fatalf("Failed to read md5sum for %q: %v", obj.Remote(), err)
	}
	if i.Md5sum != Md5sum {
		t.Errorf("%s: Md5sum incorrect - expecting %q got %q", obj.Remote(), i.Md5sum, Md5sum)
	}
	if i.Size != obj.Size() {
		t.Errorf("%s: Size incorrect - expecting %d got %d", obj.Remote(), i.Size, obj.Size())
	}
	i.CheckModTime(t, obj, obj.ModTime(), precision)
}

// Represents all items for checking
type Items struct {
	byName map[string]*Item
	items  []Item
}

// Make an Items
func NewItems(items []Item) *Items {
	is := &Items{
		byName: make(map[string]*Item),
		items:  items,
	}
	// Fill up byName
	for i := range items {
		is.byName[items[i].Path] = &items[i]
	}
	return is
}

// Check off an item
func (is *Items) Find(t *testing.T, obj fs.Object, precision time.Duration) {
	i, ok := is.byName[obj.Remote()]
	if !ok {
		t.Errorf("Unexpected file %q", obj.Remote())
	}
	delete(is.byName, obj.Remote())
	i.Check(t, obj, precision)
}

// Check all done
func (is *Items) Done(t *testing.T) {
	if len(is.byName) != 0 {
		for name := range is.byName {
			log.Printf("Not found %q", name)
		}
		t.Errorf("%d objects not found", len(is.byName))
	}
}

// Checks the fs to see if it has the expected contents
func CheckListingWithPrecision(t *testing.T, f fs.Fs, items []Item, precision time.Duration) {
	is := NewItems(items)
	for obj := range f.List() {
		if obj == nil {
			t.Errorf("Unexpected nil in List()")
			continue
		}
		is.Find(t, obj, precision)
	}
	is.Done(t)
}

// Checks the fs to see if it has the expected contents
func CheckListing(t *testing.T, f fs.Fs, items []Item) {
	precision := f.Precision()
	CheckListingWithPrecision(t, f, items, precision)
}

// Parse a time string or explode
func Time(timeString string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		log.Fatalf("Failed to parse time %q: %v", timeString, err)
	}
	return t
}

// Create a random string
func RandomString(n int) string {
	source := "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	for i := range out {
		out[i] = source[rand.Intn(len(source))]
	}
	return string(out)
}

// Creates a temporary directory name for local remotes
func LocalRemote() (path string, err error) {
	path, err = ioutil.TempDir("", "rclone")
	if err == nil {
		// Now remove the directory
		err = os.Remove(path)
	}
	return
}

// Make a random bucket or subdirectory name
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

// Make a random bucket or subdirectory on the remote
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

func TestMkdir(t *testing.T, remote fs.Fs) {
	err := fs.Mkdir(remote)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	CheckListing(t, remote, []Item{})
}

func TestPurge(t *testing.T, remote fs.Fs) {
	err := fs.Purge(remote)
	if err != nil {
		t.Fatalf("Purge failed: %v", err)
	}
	CheckListing(t, remote, []Item{})
}

func TestRmdir(t *testing.T, remote fs.Fs) {
	err := fs.Rmdir(remote)
	if err != nil {
		t.Fatalf("Rmdir failed: %v", err)
	}
}
