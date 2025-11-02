package list

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSorter(t *testing.T) {
	ctx := context.Background()
	da := mockdir.New("a")
	oA := mockobject.Object("A")
	callback := func(entries fs.DirEntries) error {
		require.Equal(t, fs.DirEntries{oA, da}, entries)
		return nil
	}
	ls, err := NewSorter(ctx, nil, callback, nil)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%p", callback), fmt.Sprintf("%p", ls.callback))
	assert.Equal(t, fmt.Sprintf("%p", identityKeyFn), fmt.Sprintf("%p", ls.keyFn))
	assert.Equal(t, fs.DirEntries(nil), ls.entries)

	// Test Add
	err = ls.Add(fs.DirEntries{da})
	require.NoError(t, err)
	assert.Equal(t, fs.DirEntries{da}, ls.entries)
	err = ls.Add(fs.DirEntries{oA})
	require.NoError(t, err)
	assert.Equal(t, fs.DirEntries{da, oA}, ls.entries)

	// Test Send
	err = ls.Send()
	require.NoError(t, err)

	// Test Cleanup
	ls.CleanUp()
	assert.Equal(t, fs.DirEntries(nil), ls.entries)
}

func TestSorterIdentity(t *testing.T) {
	ctx := context.Background()
	cmpFn := func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Remote(), b.Remote())
	}
	callback := func(entries fs.DirEntries) error {
		assert.True(t, slices.IsSortedFunc(entries, cmpFn))
		assert.Equal(t, "a", entries[0].Remote())
		return nil
	}
	ls, err := NewSorter(ctx, nil, callback, nil)
	require.NoError(t, err)
	defer ls.CleanUp()

	// Add things in reverse alphabetical order
	for i := 'z'; i >= 'a'; i-- {
		err = ls.Add(fs.DirEntries{mockobject.Object(string(i))})
		require.NoError(t, err)
	}
	assert.Equal(t, "z", ls.entries[0].Remote())
	assert.False(t, slices.IsSortedFunc(ls.entries, cmpFn))

	// Check they get sorted
	err = ls.Send()
	require.NoError(t, err)
}

func TestSorterKeyFn(t *testing.T) {
	ctx := context.Background()
	keyFn := func(entry fs.DirEntry) string {
		s := entry.Remote()
		return string('z' - s[0])
	}
	cmpFn := func(a, b fs.DirEntry) int {
		return cmp.Compare(keyFn(a), keyFn(b))
	}
	callback := func(entries fs.DirEntries) error {
		assert.True(t, slices.IsSortedFunc(entries, cmpFn))
		assert.Equal(t, "z", entries[0].Remote())
		return nil
	}
	ls, err := NewSorter(ctx, nil, callback, keyFn)
	require.NoError(t, err)
	defer ls.CleanUp()

	// Add things in reverse sorted order
	for i := 'a'; i <= 'z'; i++ {
		err = ls.Add(fs.DirEntries{mockobject.Object(string(i))})
		require.NoError(t, err)
	}
	assert.Equal(t, "a", ls.entries[0].Remote())
	assert.False(t, slices.IsSortedFunc(ls.entries, cmpFn))

	// Check they get sorted
	err = ls.Send()
	require.NoError(t, err)
}

// testFs implements enough of the fs.Fs interface for Sorter
type testFs struct {
	t          *testing.T
	entriesMap map[string]fs.DirEntry
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *testFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	entry, ok := f.entriesMap[remote]
	assert.True(f.t, ok, "entry not found")
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}
	obj, ok := entry.(fs.Object)
	assert.True(f.t, ok, "expected entry to be object: %#v", entry)
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}
	return obj, nil
}

// String outputs info about the Fs
func (f *testFs) String() string {
	return "testFs"
}

// used to sort the entries case insensitively
func keyCaseInsensitive(entry fs.DirEntry) string {
	return strings.ToLower(entry.Remote())
}

// Test the external sorting
func testSorterExt(t *testing.T, cutoff, N int, wantExtSort bool, keyFn KeyFn) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.ListCutoff = cutoff

	// Make the directory entries
	entriesMap := make(map[string]fs.DirEntry, N)
	for i := range N {
		remote := fmt.Sprintf("%010d", i)
		prefix := "a"
		if i%3 == 0 {
			prefix = "A"
		}
		remote = prefix + remote
		if i%2 == 0 {
			entriesMap[remote] = mockobject.New(remote)
		} else {
			entriesMap[remote] = mockdir.New(remote)
		}
	}
	assert.Equal(t, N, len(entriesMap))
	f := &testFs{t: t, entriesMap: entriesMap}

	// In the callback delete entries from the map when they are
	// found
	prevKey := ""
	callback := func(entries fs.DirEntries) error {
		for _, gotEntry := range entries {
			remote := gotEntry.Remote()
			key := remote
			if keyFn != nil {
				key = keyFn(gotEntry)
			}
			require.Less(t, prevKey, key, "Not sorted")
			prevKey = key
			wantEntry, ok := entriesMap[remote]
			assert.True(t, ok, "Entry not found %q", remote)
			_, wantDir := wantEntry.(fs.Directory)
			_, gotDir := wantEntry.(fs.Directory)
			_, wantObj := wantEntry.(fs.Object)
			_, gotObj := wantEntry.(fs.Object)
			require.True(t, (wantDir && gotDir) || (wantObj && gotObj), "Wrong types %#v, %#v", wantEntry, gotEntry)
			delete(entriesMap, remote)
		}
		return nil
	}

	ls, err := NewSorter(ctx, f, callback, keyFn)
	require.NoError(t, err)

	// Send the entries in random (map) order
	for _, entry := range entriesMap {
		err = ls.Add(fs.DirEntries{entry})
		require.NoError(t, err)
	}

	// Check we are extsorting if required
	assert.Equal(t, wantExtSort, ls.extSort)

	// Test Send
	err = ls.Send()
	require.NoError(t, err)

	// All the entries should have been seen
	assert.Equal(t, 0, len(entriesMap))

	// Test Cleanup
	ls.CleanUp()
	assert.Equal(t, fs.DirEntries(nil), ls.entries)
}

// Test the external sorting
func TestSorterExt(t *testing.T) {
	for _, test := range []struct {
		cutoff      int
		N           int
		wantExtSort bool
		keyFn       KeyFn
	}{
		{cutoff: 1000, N: 100, wantExtSort: false},
		{cutoff: 100, N: 1000, wantExtSort: true},
		{cutoff: 1000, N: 100, wantExtSort: false, keyFn: keyCaseInsensitive},
		{cutoff: 100, N: 1000, wantExtSort: true, keyFn: keyCaseInsensitive},
		{cutoff: 100001, N: 100000, wantExtSort: false},
		{cutoff: 100000, N: 100001, wantExtSort: true},
		// {cutoff: 100_000, N: 1_000_000, wantExtSort: true},
		// {cutoff: 100_000, N: 10_000_000, wantExtSort: true},
	} {
		t.Run(fmt.Sprintf("cutoff=%d,N=%d,wantExtSort=%v,keyFn=%v", test.cutoff, test.N, test.wantExtSort, test.keyFn != nil), func(t *testing.T) {
			testSorterExt(t, test.cutoff, test.N, test.wantExtSort, test.keyFn)
		})
	}
}

// benchFs implements enough of the fs.Fs interface for Sorter
type benchFs struct{}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (benchFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Recreate the mock objects
	return mockobject.New(remote), nil
}

// String outputs info about the Fs
func (benchFs) String() string {
	return "benchFs"
}

func BenchmarkSorterExt(t *testing.B) {
	const cutoff = 1000
	const N = 10_000_000

	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.ListCutoff = cutoff
	keyFn := keyCaseInsensitive

	// In the callback check entries are in order
	prevKey := ""
	entriesReceived := 0
	callback := func(entries fs.DirEntries) error {
		for _, gotEntry := range entries {
			remote := gotEntry.Remote()
			key := remote
			if keyFn != nil {
				key = keyFn(gotEntry)
			}
			require.Less(t, prevKey, key, "Not sorted")
			prevKey = key
			entriesReceived++
		}
		return nil
	}

	f := benchFs{}
	ls, err := NewSorter(ctx, f, callback, keyFn)
	require.NoError(t, err)

	// Send the entries in reverse order in batches of 1000 like the backends do
	var entries = make(fs.DirEntries, 0, 1000)
	for i := N - 1; i >= 0; i-- {
		remote := fmt.Sprintf("%050d", i) // UUID length plus a bit
		prefix := "a"
		if i%3 == 0 {
			prefix = "A"
		}
		remote = prefix + remote
		if i%2 == 0 {
			entries = append(entries, mockobject.New(remote))
		} else {
			entries = append(entries, mockdir.New(remote))
		}
		if len(entries) > 1000 {
			err = ls.Add(entries)
			require.NoError(t, err)
			entries = entries[:0]
		}
	}
	err = ls.Add(entries)
	require.NoError(t, err)

	// Check we are extsorting
	assert.True(t, ls.extSort)

	// Test Send
	err = ls.Send()
	require.NoError(t, err)

	// All the entries should have been seen
	assert.Equal(t, N, entriesReceived)

	// Cleanup
	ls.CleanUp()
}
