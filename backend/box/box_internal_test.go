package box

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
)

// TODO: Test more than just "description" states
func TestRiConfig(t *testing.T) {
	const (
		descriptionState         = "description"
		descriptionCompleteState = "description_complete"
		newDescription           = "New description"
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "description input",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: descriptionState},
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
