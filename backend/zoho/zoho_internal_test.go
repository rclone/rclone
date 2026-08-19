package zoho

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigRootFolderID drives the interactive Config state machine directly
// and checks that an existing root_folder_id is preserved on update/reconnect:
// workspace selection only runs when the id is empty or the user opts in. The
// seeded token is already the Zoho type and no region service is contacted, so
// these cases make no network calls.
func TestConfigRootFolderID(t *testing.T) {
	regInfo := fs.MustFind("zoho")
	const rootID = "abc123rootfolderid"
	// Already the Zoho custom type, so the "type" state's token rewrite is a
	// no-op and nothing is sent to the network.
	const token = `{"access_token":"x","token_type":"Zoho-oauthtoken"}`

	newMapper := func(withRoot bool) configmap.Simple {
		m := configmap.Simple{"region": "eu", "token": token}
		if withRoot {
			m[configRootID] = rootID
		}
		return m
	}

	ctx := context.Background()

	t.Run("SetRootAsksBeforeChanging", func(t *testing.T) {
		m := newMapper(true)
		out, err := regInfo.Config(ctx, "zoho", m, fs.ConfigIn{State: "type"})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "root_change", out.State)
		require.NotNil(t, out.Option)
		got, _ := m.Get(configRootID)
		assert.Equal(t, rootID, got, "id must not change before the user answers")
	})

	t.Run("EmptyRootGoesToEdition", func(t *testing.T) {
		m := newMapper(false)
		out, err := regInfo.Config(ctx, "zoho", m, fs.ConfigIn{State: "type"})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "select_edition", out.State)
		assert.Nil(t, out.Option, "goto, not a question")
	})

	t.Run("KeepRootOnNo", func(t *testing.T) {
		m := newMapper(true)
		out, err := regInfo.Config(ctx, "zoho", m, fs.ConfigIn{State: "root_change", Result: "false"})
		require.NoError(t, err)
		assert.Nil(t, out, "answering No ends config and keeps the id")
		got, _ := m.Get(configRootID)
		assert.Equal(t, rootID, got)
	})

	t.Run("ChangeRootOnYes", func(t *testing.T) {
		m := newMapper(true)
		out, err := regInfo.Config(ctx, "zoho", m, fs.ConfigIn{State: "root_change", Result: "true"})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "select_edition", out.State)
	})
}
