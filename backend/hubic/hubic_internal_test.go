package hubic

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
)

func TestRiConfig(t *testing.T) {
	const (
		descriptionState         = "description"
		descriptionCompleteState = "description_complete"
		newDescription           = "New description"
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "empty state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: ""},
			ExpectState: descriptionState,
		},
		{
			Name:        "description state",
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
