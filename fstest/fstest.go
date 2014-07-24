// Utilities for testing the fs

package fstest

// FIXME put name of test FS in Fs structure

import (
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
)

var Fatalf = log.Fatalf

// Represents an item for checking
type Item struct {
	Path    string
	Md5sum  string
	ModTime time.Time
	Size    int64
}

func (i *Item) Check(obj fs.Object) {
	if obj == nil {
		Fatalf("Object is nil")
	}
	// Check attributes
	Md5sum, err := obj.Md5sum()
	if err != nil {
		Fatalf("Failed to read md5sum for %q: %v", obj.Remote(), err)
	}
	if i.Md5sum != Md5sum {
		Fatalf("%s: Md5sum incorrect - expecting %q got %q", obj.Remote(), i.Md5sum, Md5sum)
	}
	if i.Size != obj.Size() {
		Fatalf("%s: Size incorrect - expecting %d got %d", obj.Remote(), i.Size, obj.Size())
	}
	// check the mod time to the given precision
	modTime := obj.ModTime()
	dt := modTime.Sub(i.ModTime)
	if dt >= fs.Config.ModifyWindow || dt <= -fs.Config.ModifyWindow {
		Fatalf("%s: Modification time difference too big |%s| > %s (%s vs %s)", obj.Remote(), dt, fs.Config.ModifyWindow, modTime, i.ModTime)
	}

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
func (is *Items) Find(obj fs.Object) {
	i, ok := is.byName[obj.Remote()]
	if !ok {
		Fatalf("Unexpected file %q", obj.Remote())
	}
	delete(is.byName, obj.Remote())
	i.Check(obj)
}

// Check all done
func (is *Items) Done() {
	if len(is.byName) != 0 {
		for name := range is.byName {
			log.Printf("Not found %q", name)
		}
		Fatalf("%d objects not found", len(is.byName))
	}
}

// Checks the fs to see if it has the expected contents
func CheckListing(f fs.Fs, items []Item) {
	is := NewItems(items)
	for obj := range f.List() {
		is.Find(obj)
	}
	is.Done()
}

// Parse a time string or explode
func Time(timeString string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		Fatalf("Failed to parse time %q: %v", timeString, err)
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

// Make a random bucket or subdirectory on the remote
//
// Call the finalise function returned to Purge the fs at the end (and
// the parent if necessary)
func RandomRemote(remoteName string, subdir bool) (fs.Fs, func()) {
	// Make a directory if remote name is null
	rmdir := ""
	var err error
	if remoteName == "" {
		remoteName, err = ioutil.TempDir("", "rclone")
		if err != nil {
			Fatalf("Failed to create temp dir: %v", err)
		}
		rmdir = remoteName
	}

	if !strings.HasSuffix(remoteName, ":") {
		remoteName += "/"
	}
	remoteName += RandomString(32)
	var parentRemote fs.Fs
	if subdir {
		var err error
		parentRemote, err = fs.NewFs(remoteName)
		if err != nil {
			log.Fatalf("Failed to make parent %q: %v", remoteName, err)
		}
		remoteName += "/" + RandomString(8)
	}

	remote, err := fs.NewFs(remoteName)
	if err != nil {
		log.Fatalf("Failed to make %q: %v", remoteName, err)
	}

	finalise := func() {
		TestPurge(remote)
		if parentRemote != nil {
			TestPurge(parentRemote)
		}
		// Delete directory if we made one above
		if rmdir != "" {
			err := os.RemoveAll(rmdir)
			if err != nil {
				Fatalf("Failed to remove %q: %v", rmdir, err)
			}
		}
	}

	return remote, finalise
}

func TestMkdir(remote fs.Fs) {
	err := fs.Mkdir(remote)
	if err != nil {
		Fatalf("Mkdir failed: %v", err)
	}
	CheckListing(remote, []Item{})
}

func TestPurge(remote fs.Fs) {
	err := fs.Purge(remote)
	if err != nil {
		log.Fatalf("Purge failed: %v", err)
	}
	CheckListing(remote, []Item{})
}

func TestRmdir(remote fs.Fs) {
	err := fs.Rmdir(remote)
	if err != nil {
		Fatalf("Rmdir failed: %v", err)
	}
}
