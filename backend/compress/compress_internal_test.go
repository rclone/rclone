package compress

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
)

func TestRiConfig(t *testing.T) {
	const (
		descriptionComplete = "description_complete"
		newDescription      = "New description"
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "empty state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: ""},
			ExpectState: descriptionComplete,
		},
		{
			Name:            "description complete",
			Mapper:          configmap.Simple{},
			Input:           fs.ConfigIn{State: descriptionComplete, Result: newDescription},
			ExpectMapper:    configmap.Simple{fs.ConfigDescription: newDescription},
			ExpectNilOutput: true,
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
