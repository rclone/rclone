//go:build unix

package nfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckUnmountedOnlyUnregistersFinishedServers(t *testing.T) {
	gone, stillMounted := &Server{}, &Server{}
	var goneCalls, stillCalls int
	gone.OnUnmount(func() bool { goneCalls++; return true })
	stillMounted.OnUnmount(func() bool { stillCalls++; return false })
	t.Cleanup(func() {
		unmountMu.Lock()
		delete(unmountFuncs, stillMounted)
		unmountMu.Unlock()
	})

	assert.True(t, checkUnmounted())
	assert.False(t, checkUnmounted())

	assert.Equal(t, 1, goneCalls)
	assert.Equal(t, 2, stillCalls)
}
