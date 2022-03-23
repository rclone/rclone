package googlecloudstorage

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

// TestConfigRi test that configRi handles ConfigIn.State correctly
func TestConfigRi(t *testing.T) {
	const (
		descriptionState         = "description"
		descriptionCompleteState = "description_complete"
		newDescription           = "New description."
	)

	ctx := context.Background()
	cm := configmap.Simple{}
	ci := fs.ConfigIn{State: ""}

	configOut, _ := riConfig(ctx, "gcs", cm, ci)

	assert.Equal(t, descriptionState, configOut.State)

	ci = fs.ConfigIn{State: descriptionState}
	configOut, _ = riConfig(ctx, "gcs", cm, ci)

	assert.Equal(t, descriptionCompleteState, configOut.State)

	ci = fs.ConfigIn{State: descriptionCompleteState, Result: newDescription}
	configOut, _ = riConfig(ctx, "gcs", cm, ci)

	assert.Equal(t, configmap.Simple{
		fs.ConfigDescription: newDescription,
	}, cm)
	assert.Nil(t, configOut)
}

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
			Name:            "description complete",
			Mapper:          configmap.Simple{},
			Input:           fs.ConfigIn{State: descriptionState},
			ExpectState:     descriptionCompleteState,
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
