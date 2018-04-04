package local

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check we can remove an open file
func TestRemove(t *testing.T) {
	fd, err := ioutil.TempFile("", "rclone-remove-test")
	require.NoError(t, err)
	name := fd.Name()
	defer func() {
		_ = os.Remove(name)
	}()

	exists := func() bool {
		_, err := os.Stat(name)
		if err == nil {
			return true
		} else if os.IsNotExist(err) {
			return false
		}
		require.NoError(t, err)
		return false
	}

	assert.True(t, exists())
	// close the file in the background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(250 * time.Millisecond)
		require.NoError(t, fd.Close())
	}()
	// delete the open file
	err = remove(name)
	require.NoError(t, err)
	// check it no longer exists
	assert.False(t, exists())
	// wait for background close
	wg.Wait()
}
