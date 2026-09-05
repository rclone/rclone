package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/local"
)

// persistedCopy returns vol as it comes back from the plugin state file:
// only the exported fields survive being marshalled and reloaded.
func persistedCopy(t *testing.T, vol *Volume) *Volume {
	t.Helper()
	vol.prepareState()
	data, err := json.Marshal(vol)
	require.NoError(t, err)
	restored := &Volume{}
	require.NoError(t, json.Unmarshal(data, restored))
	return restored
}

// TestRestoreStatePath checks that the on-remote path of a volume survives
// being reloaded from the plugin state file, for each of the option
// combinations that can carry one.
func TestRestoreStatePath(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name     string
		volOpt   VolOpts
		fsString string
	}{{
		name:     "type and path",
		volOpt:   VolOpts{"type": "local", "path": "/tmp/path"},
		fsString: ":local:/tmp/path",
	}, {
		name:     "remote only",
		volOpt:   VolOpts{"remote": "/tmp/remote"},
		fsString: ":local:/tmp/remote",
	}, {
		// an explicit path overrides the path of the connection string,
		// and must keep doing so after a restart
		name:     "remote and path",
		volOpt:   VolOpts{"remote": "/tmp/remote", "path": "/tmp/path"},
		fsString: ":local:/tmp/path",
	}, {
		name:     "type only",
		volOpt:   VolOpts{"type": "local"},
		fsString: ":local:",
	}} {
		t.Run(test.name, func(t *testing.T) {
			drv := &Driver{
				root:    t.TempDir(),
				volumes: map[string]*Volume{},
				dummy:   true,
			}
			vol, err := newVolume(ctx, "vol1", test.volOpt, drv)
			require.NoError(t, err)
			require.Equal(t, test.fsString, vol.fsString)

			restored := persistedCopy(t, vol)
			require.NoError(t, restored.restoreState(ctx, drv))
			assert.Equal(t, test.fsString, restored.fsString)
			assert.Equal(t, vol.Path, restored.Path)
		})
	}
}

// TestDriverRestoreStatePath checks the same through the real state file: a
// volume persisted by a previous plugin instance is reloaded with its path.
func TestDriverRestoreStatePath(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()

	state := fmt.Sprintf(`[{"name":"vol1","mountpoint":%q,"created":%q,"fs":"","type":"local","path":"/tmp/path","options":{},"mounts":[]}]`,
		filepath.Join(testDir, "vol1"), time.Now().Format(time.RFC3339))
	statePath := filepath.Join(testDir, stateFile)
	require.NoError(t, os.WriteFile(statePath, []byte(state), 0600))

	drv := &Driver{
		root:      testDir,
		statePath: statePath,
		volumes:   map[string]*Volume{},
		dummy:     true,
	}
	require.NoError(t, drv.restoreState(ctx))

	vol, err := drv.getVolume("vol1")
	require.NoError(t, err)
	assert.Equal(t, ":local:/tmp/path", vol.fsString)
	assert.Equal(t, "/tmp/path", vol.Path)
}
