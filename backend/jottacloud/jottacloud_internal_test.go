package jottacloud

import (
	"crypto/md5"
	"fmt"
	"io"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadMD5(t *testing.T) {
	// Check readMD5 for different size and threshold
	for _, size := range []int64{0, 1024, 10 * 1024, 100 * 1024} {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			hasher := md5.New()
			n, err := io.Copy(hasher, readers.NewPatternReader(size))
			require.NoError(t, err)
			assert.Equal(t, n, size)
			wantMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
			for _, threshold := range []int64{512, 1024, 10 * 1024, 20 * 1024} {
				t.Run(fmt.Sprintf("%d", threshold), func(t *testing.T) {
					in := readers.NewPatternReader(size)
					gotMD5, out, cleanup, err := readMD5(in, size, threshold)
					defer cleanup()
					require.NoError(t, err)
					assert.Equal(t, wantMD5, gotMD5)

					// check md5hash of out
					hasher := md5.New()
					n, err := io.Copy(hasher, out)
					require.NoError(t, err)
					assert.Equal(t, n, size)
					outMD5 := fmt.Sprintf("%x", hasher.Sum(nil))
					assert.Equal(t, wantMD5, outMD5)
				})
			}
		})
	}
}

// TODO: test all ConfigIn states, not just "description" states
func TestRiConfig(t *testing.T) {
	const (
		endState                 = "end"
		descriptionCompleteState = "description_complete"
		newDescription           = "New description."
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "end state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: endState},
			ExpectState: descriptionCompleteState,
		},
		{
			Name:            "description complete",
			Mapper:          configmap.Simple{},
			Input:           fs.ConfigIn{State: descriptionCompleteState, Result: newDescription},
			ExpectMapper:    configmap.Simple{fs.ConfigDescription: newDescription},
			ExpectNilOutput: true,
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
