package zoho

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
)

// TODO: Test more than just "description" states
func TestRiConfig(t *testing.T) {
	const (
		descriptionCompleteState = "description_complete"
		descriptionState         = "description"
		newDescription           = "New description."
		region                   = "testRegion"
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "description",
			Mapper:      configmap.Simple{"region": region},
			Input:       fs.ConfigIn{State: descriptionState},
			ExpectState: descriptionCompleteState,
		},
		{
			Name:   "description complete",
			Mapper: configmap.Simple{"region": region},
			Input:  fs.ConfigIn{State: descriptionCompleteState, Result: newDescription},
			ExpectMapper: configmap.Simple{
				fs.ConfigDescription: newDescription,
				"region":             region,
			},
			ExpectNilOutput: true,
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
