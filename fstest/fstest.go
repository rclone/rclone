// Package fstest provides utilities for testing the Fs
package fstest

// FIXME put name of test FS in Fs structure

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Globals
var (
	RemoteName      = flag.String("remote", "", "Remote to test with, defaults to local filesystem")
	SubDir          = flag.Bool("subdir", false, "Set to test with a sub directory")
	Verbose         = flag.Bool("verbose", false, "Set to enable logging")
	DumpHeaders     = flag.Bool("dump-headers", false, "Set to dump headers (needs -verbose)")
	DumpBodies      = flag.Bool("dump-bodies", false, "Set to dump bodies (needs -verbose)")
	Individual      = flag.Bool("individual", false, "Make individual bucket/container/directory for each test - much slower")
	LowLevelRetries = flag.Int("low-level-retries", 10, "Number of low level retries")
	UseListR        = flag.Bool("fast-list", false, "Use recursive list if available. Uses more memory but fewer transactions.")
	// ListRetries is the number of times to retry a listing to overcome eventual consistency
	ListRetries = flag.Int("list-retries", 6, "Number or times to retry listing")
	// MatchTestRemote matches the remote names used for testing
	MatchTestRemote = regexp.MustCompile(`^rclone-test-[abcdefghijklmnopqrstuvwxyz0123456789]{24}$`)
)

// Seed the random number generator
func init() {
	rand.Seed(time.Now().UnixNano())

}

// Initialise rclone for testing
func Initialise() {
	// Never ask for passwords, fail instead.
	// If your local config is encrypted set environment variable
	// "RCLONE_CONFIG_PASS=hunter2" (or your password)
	*fs.AskPassword = false
	fs.LoadConfig()
	if *Verbose {
		fs.Config.LogLevel = fs.LogLevelDebug
	}
	fs.Config.DumpHeaders = *DumpHeaders
	fs.Config.DumpBodies = *DumpBodies
	fs.Config.LowLevelRetries = *LowLevelRetries
	fs.Config.UseListR = *UseListR
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
	assert.True(t, ok, fmt.Sprintf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", obj.Remote(), dt, precision, modTime, i.ModTime, precision))
}

// CheckHashes checks all the hashes the object supports are correct
func (i *Item) CheckHashes(t *testing.T, obj fs.Object) {
	require.NotNil(t, obj)
	types := obj.Fs().Hashes().Array()
	for _, hash := range types {
		// Check attributes
		sum, err := obj.Hash(hash)
		require.NoError(t, err)
		assert.True(t, fs.HashEquals(i.Hashes[hash], sum), fmt.Sprintf("%s/%s: %v hash incorrect - expecting %q got %q", obj.Fs().String(), obj.Remote(), hash, i.Hashes[hash], sum))
	}
}

// Check checks all the attributes of the object are correct
func (i *Item) Check(t *testing.T, obj fs.Object, precision time.Duration) {
	i.CheckHashes(t, obj)
	assert.Equal(t, i.Size, obj.Size(), fmt.Sprintf("%s: size incorrect file=%d vs obj=%d", i.Path, i.Size, obj.Size()))
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
		assert.True(t, ok, fmt.Sprintf("Unexpected file %q", obj.Remote()))
	}
	if i != nil {
		delete(is.byName, i.Path)
		delete(is.byName, i.WinPath)
		i.Check(t, obj, precision)
	}
}

// Done checks all finished
func (is *Items) Done(t *testing.T) {
	if len(is.byName) != 0 {
		for name := range is.byName {
			t.Logf("Not found %q", name)
		}
	}
	assert.Equal(t, 0, len(is.byName), fmt.Sprintf("%d objects not found", len(is.byName)))
}

// makeListingFromItems returns a string representation of the items
//
// it returns two possible strings, one normal and one for windows
func makeListingFromItems(items []Item) (string, string) {
	nameLengths1 := make([]string, len(items))
	nameLengths2 := make([]string, len(items))
	for i, item := range items {
		remote1 := item.Path
		remote2 := item.Path
		if item.WinPath != "" {
			remote2 = item.WinPath
		}
		nameLengths1[i] = fmt.Sprintf("%s (%d)", remote1, item.Size)
		nameLengths2[i] = fmt.Sprintf("%s (%d)", remote2, item.Size)
	}
	sort.Strings(nameLengths1)
	sort.Strings(nameLengths2)
	return strings.Join(nameLengths1, ", "), strings.Join(nameLengths2, ", ")
}

// makeListingFromObjects returns a string representation of the objects
func makeListingFromObjects(objs []fs.Object) string {
	nameLengths := make([]string, len(objs))
	for i, obj := range objs {
		nameLengths[i] = fmt.Sprintf("%s (%d)", obj.Remote(), obj.Size())
	}
	sort.Strings(nameLengths)
	return strings.Join(nameLengths, ", ")
}

// filterEmptyDirs removes any empty (or containing only directories)
// directories from expectedDirs
func filterEmptyDirs(t *testing.T, items []Item, expectedDirs []string) (newExpectedDirs []string) {
	dirs := map[string]struct{}{"": struct{}{}}
	for _, item := range items {
		base := item.Path
		for {
			base = path.Dir(base)
			if base == "." || base == "/" {
				break
			}
			dirs[base] = struct{}{}
		}
	}
	for _, expectedDir := range expectedDirs {
		if _, found := dirs[expectedDir]; found {
			newExpectedDirs = append(newExpectedDirs, expectedDir)
		} else {
			t.Logf("Filtering empty directory %q", expectedDir)
		}
	}
	return newExpectedDirs
}

// CheckListingWithPrecision checks the fs to see if it has the
// expected contents with the given precision.
//
// If expectedDirs is non nil then we check those too.  Note that no
// directories returned is also OK as some remotes don't return
// directories.
func CheckListingWithPrecision(t *testing.T, f fs.Fs, items []Item, expectedDirs []string, precision time.Duration) {
	if expectedDirs != nil && !f.Features().CanHaveEmptyDirectories {
		expectedDirs = filterEmptyDirs(t, items, expectedDirs)
	}
	is := NewItems(items)
	oldErrors := fs.Stats.GetErrors()
	var objs []fs.Object
	var dirs []fs.Directory
	var err error
	var retries = *ListRetries
	sleep := time.Second / 2
	wantListing1, wantListing2 := makeListingFromItems(items)
	gotListing := "<unset>"
	listingOK := false
	for i := 1; i <= retries; i++ {
		objs, dirs, err = fs.WalkGetAll(f, "", true, -1)
		if err != nil && err != fs.ErrorDirNotFound {
			t.Fatalf("Error listing: %v", err)
		}
		gotListing = makeListingFromObjects(objs)
		listingOK = wantListing1 == gotListing || wantListing2 == gotListing
		if listingOK && (expectedDirs == nil || len(dirs) == len(expectedDirs)) {
			// Put an extra sleep in if we did any retries just to make sure it really
			// is consistent (here is looking at you Amazon Drive!)
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
		if doDirCacheFlush := f.Features().DirCacheFlush; doDirCacheFlush != nil {
			t.Logf("Flushing the directory cache")
			doDirCacheFlush()
		}
	}
	assert.True(t, listingOK, fmt.Sprintf("listing wrong, want\n  %s or\n  %s got\n  %s", wantListing1, wantListing2, gotListing))
	for _, obj := range objs {
		require.NotNil(t, obj)
		is.Find(t, obj, precision)
	}
	is.Done(t)
	// Don't notice an error when listing an empty directory
	if len(items) == 0 && oldErrors == 0 && fs.Stats.GetErrors() == 1 {
		fs.Stats.ResetErrors()
	}
	// Check the directories
	if expectedDirs != nil {
		actualDirs := []string{}
		for _, dir := range dirs {
			actualDirs = append(actualDirs, dir.Remote())
		}
		sort.Strings(actualDirs)
		sort.Strings(expectedDirs)
		assert.Equal(t, expectedDirs, actualDirs, "directories")
	}
}

// CheckListing checks the fs to see if it has the expected contents
func CheckListing(t *testing.T, f fs.Fs, items []Item) {
	precision := f.Precision()
	CheckListingWithPrecision(t, f, items, nil, precision)
}

// CheckItems checks the fs to see if it has only the items passed in
// using a precision of fs.Config.ModifyWindow
func CheckItems(t *testing.T, f fs.Fs, items ...Item) {
	CheckListingWithPrecision(t, f, items, nil, fs.Config.ModifyWindow)
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
	const (
		vowel     = "aeiou"
		consonant = "bcdfghjklmnpqrstvwxyz"
		digit     = "0123456789"
	)
	pattern := []string{consonant, vowel, consonant, vowel, consonant, vowel, consonant, digit}
	out := make([]byte, n)
	p := 0
	for i := range out {
		source := pattern[p]
		p = (p + 1) % len(pattern)
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
		leafName = "rclone-test-" + RandomString(24)
		if !MatchTestRemote.MatchString(leafName) {
			log.Fatalf("%q didn't match the test remote name regexp", leafName)
		}
		remoteName += leafName
	}
	return remoteName, leafName, nil
}

// RandomRemote makes a random bucket or subdirectory on the remote
//
// Call the finalise function returned to Purge the fs at the end (and
// the parent if necessary)
//
// Returns the remote, its url, a finaliser and an error
func RandomRemote(remoteName string, subdir bool) (fs.Fs, string, func(), error) {
	var err error
	var parentRemote fs.Fs

	remoteName, _, err = RandomRemoteName(remoteName)
	if err != nil {
		return nil, "", nil, err
	}

	if subdir {
		parentRemote, err = fs.NewFs(remoteName)
		if err != nil {
			return nil, "", nil, err
		}
		remoteName += "/rclone-test-subdir-" + RandomString(8)
	}

	remote, err := fs.NewFs(remoteName)
	if err != nil {
		return nil, "", nil, err
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

	return remote, remoteName, finalise, nil
}

// TestMkdir tests Mkdir works
func TestMkdir(t *testing.T, remote fs.Fs) {
	err := fs.Mkdir(remote, "")
	require.NoError(t, err)
	CheckListing(t, remote, []Item{})
}

// TestPurge tests Purge works
func TestPurge(t *testing.T, remote fs.Fs) {
	err := fs.Purge(remote)
	require.NoError(t, err)
	CheckListing(t, remote, []Item{})
}

// TestRmdir tests Rmdir works
func TestRmdir(t *testing.T, remote fs.Fs) {
	err := fs.Rmdir(remote, "")
	require.NoError(t, err)
}
