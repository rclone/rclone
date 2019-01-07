package file

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create a test directory then tidy up
func testDir(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "rclone-test")
	require.NoError(t, err)
	return dir, func() {
		assert.NoError(t, os.RemoveAll(dir))
	}
}

// This lists dir and checks the listing is as expected without checking the size
func checkListingNoSize(t *testing.T, dir string, want []string) {
	var got []string
	nodes, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	for _, node := range nodes {
		got = append(got, fmt.Sprintf("%s,%v", node.Name(), node.IsDir()))
	}
	assert.Equal(t, want, got)
}

// This lists dir and checks the listing is as expected
func checkListing(t *testing.T, dir string, want []string) {
	var got []string
	nodes, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	for _, node := range nodes {
		got = append(got, fmt.Sprintf("%s,%d,%v", node.Name(), node.Size(), node.IsDir()))
	}
	assert.Equal(t, want, got)
}

// Test we can rename an open file
func TestOpenFileRename(t *testing.T) {
	dir, tidy := testDir(t)
	defer tidy()

	filepath := path.Join(dir, "file1")
	f, err := Create(filepath)
	require.NoError(t, err)

	_, err = f.Write([]byte("hello"))
	assert.NoError(t, err)

	checkListingNoSize(t, dir, []string{
		"file1,false",
	})

	// Delete the file first
	assert.NoError(t, os.Remove(filepath))

	// .. then close it
	assert.NoError(t, f.Close())

	checkListing(t, dir, nil)
}

// Test we can delete an open file
func TestOpenFileDelete(t *testing.T) {
	dir, tidy := testDir(t)
	defer tidy()

	filepath := path.Join(dir, "file1")
	f, err := Create(filepath)
	require.NoError(t, err)

	_, err = f.Write([]byte("hello"))
	assert.NoError(t, err)

	checkListingNoSize(t, dir, []string{
		"file1,false",
	})

	// Rename the file while open
	filepath2 := path.Join(dir, "file2")
	assert.NoError(t, os.Rename(filepath, filepath2))

	checkListingNoSize(t, dir, []string{
		"file2,false",
	})

	// .. then close it
	assert.NoError(t, f.Close())

	checkListing(t, dir, []string{
		"file2,5,false",
	})
}

// Smoke test the Open, OpenFile and Create functions
func TestOpenFileOperations(t *testing.T) {
	dir, tidy := testDir(t)
	defer tidy()

	filepath := path.Join(dir, "file1")

	// Create the file

	f, err := Create(filepath)
	require.NoError(t, err)

	_, err = f.Write([]byte("hello"))
	assert.NoError(t, err)

	assert.NoError(t, f.Close())

	checkListing(t, dir, []string{
		"file1,5,false",
	})

	// Append onto the file

	f, err = OpenFile(filepath, os.O_RDWR|os.O_APPEND, 0666)
	require.NoError(t, err)

	_, err = f.Write([]byte("HI"))
	assert.NoError(t, err)

	assert.NoError(t, f.Close())

	checkListing(t, dir, []string{
		"file1,7,false",
	})

	// Read it back in

	f, err = Open(filepath)
	require.NoError(t, err)
	var b = make([]byte, 10)
	n, err := f.Read(b)
	assert.True(t, err == io.EOF || err == nil)
	assert.Equal(t, 7, n)
	assert.Equal(t, "helloHI", string(b[:n]))

	assert.NoError(t, f.Close())

	checkListing(t, dir, []string{
		"file1,7,false",
	})

}
