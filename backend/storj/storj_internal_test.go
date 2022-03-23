package storj

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
)

// TODO: Test states other than just "description" states
func TestRiConfig(t *testing.T) {
	const (
		descriptionCompleteState = "description_complete"
		descriptionState         = "description"
		newDescription           = "New description."
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "description state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: descriptionState},
			ExpectState: descriptionCompleteState,
		},
		{
			Name:            "description",
			Mapper:          configmap.Simple{},
			Input:           fs.ConfigIn{State: descriptionCompleteState, Result: newDescription},
			ExpectMapper:    configmap.Simple{fs.ConfigDescription: newDescription},
			ExpectNilOutput: true,
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
