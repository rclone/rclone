//go:build !plan9 && !solaris && !js
// +build !plan9,!solaris,!js

package azureblob

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

func (f *Fs) InternalTest(t *testing.T) {
	// Check first feature flags are set on this
	// remote
	enabled := f.Features().SetTier
	assert.True(t, enabled)
	enabled = f.Features().GetTier
	assert.True(t, enabled)
}

func TestIncrement(t *testing.T) {
	for _, test := range []struct {
		in   []byte
		want []byte
	}{
		{[]byte{0, 0, 0, 0}, []byte{1, 0, 0, 0}},
		{[]byte{0xFE, 0, 0, 0}, []byte{0xFF, 0, 0, 0}},
		{[]byte{0xFF, 0, 0, 0}, []byte{0, 1, 0, 0}},
		{[]byte{0, 1, 0, 0}, []byte{1, 1, 0, 0}},
		{[]byte{0xFF, 0xFF, 0xFF, 0xFE}, []byte{0, 0, 0, 0xFF}},
		{[]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte{0, 0, 0, 0}},
	} {
		increment(test.in)
		assert.Equal(t, test.want, test.in)
	}
}

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
