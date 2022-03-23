package googlephotos

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
)

func TestRiConfig(t *testing.T) {
	const (
		googlephotosCompleteState = "googlephotos_complete"
		newDescription            = "New description."
		descriptionState          = "description"
		descriptionCompleteState  = "description_complete"
		warningState              = "warning"
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "empty state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: ""},
			ExpectState: warningState,
		},
		{
			Name:        "warning state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: warningState},
			ExpectState: descriptionState,
		},
		{
			Name:        "description state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: descriptionState},
			ExpectState: descriptionCompleteState,
		},
		{
			Name:            "description complete state",
			Mapper:          configmap.Simple{},
			Input:           fs.ConfigIn{State: descriptionCompleteState, Result: newDescription},
			ExpectNilOutput: true,
			ExpectMapper:     configmap.Simple{fs.ConfigDescription: newDescription},
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
