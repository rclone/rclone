// These are in an external package because we need to import configfile

package config_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	configfile.Install()
}

func TestConfigLoad(t *testing.T) {
	oldConfigPath := config.GetConfigPath()
	assert.NoError(t, config.SetConfigPath("./testdata/plain.conf"))
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
	}()
	config.ClearConfigPassword()
	sections := config.Data().GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data().GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

// TestCreateRemoteEphemeralConfigKeysReachBackend checks that config_* parameters
// (#9572) are readable by the backend via the mapper but not saved to the config file.
func TestCreateRemoteEphemeralConfigKeysReachBackend(t *testing.T) {
	defer testConfigFile(t, simpleOptions, "ephemeral.conf")()
	ctx := context.Background()

	var seenTemplateFile, seenTemplate string
	backendName := "config_template_test_remote"
	if regInfo, _ := fs.Find(backendName); regInfo == nil {
		fs.Register(&fs.RegInfo{
			Name: backendName,
			Config: func(_ context.Context, _ string, m configmap.Mapper, _ fs.ConfigIn) (*fs.ConfigOut, error) {
				seenTemplateFile, _ = m.Get("config_template_file")
				seenTemplate, _ = m.Get("config_template")
				return nil, nil
			},
		})
	}

	_, err := config.CreateRemote(ctx, "eph", backendName, rc.Params{
		"config_template_file": "/path/to/template.html",
		"config_template":      "<html>ok</html>",
	}, config.UpdateRemoteOpt{NonInteractive: true})
	require.NoError(t, err)

	// The backend must be able to read the ephemeral config_* parameters
	// through the mapper (before the fix these came back empty).
	assert.Equal(t, "/path/to/template.html", seenTemplateFile)
	assert.Equal(t, "<html>ok</html>", seenTemplate)

	// ...but they must not be persisted to the config file.
	assert.Equal(t, "", config.GetValue("eph", "config_template_file"))
	assert.Equal(t, "", config.GetValue("eph", "config_template"))
}

// TestUpdateRemoteTokenKeepsToken checks that passing a token answers
// config_refresh_token with false unless the caller answered it (#10208).
func TestUpdateRemoteTokenKeepsToken(t *testing.T) {
	defer testConfigFile(t, simpleOptions, "token.conf")()
	ctx := context.Background()

	var answer string
	backendName := "config_token_test_remote"
	if regInfo, _ := fs.Find(backendName); regInfo == nil {
		fs.Register(&fs.RegInfo{
			Name: backendName,
			Config: func(_ context.Context, _ string, _ configmap.Mapper, in fs.ConfigIn) (*fs.ConfigOut, error) {
				if in.State == "" {
					return fs.ConfigConfirm("done", true, "config_refresh_token", "Token already configured - replace it?")
				}
				answer = in.Result
				return nil, nil
			},
		})
	}
	_, err := config.CreateRemote(ctx, "tok", backendName, rc.Params{}, config.UpdateRemoteOpt{NonInteractive: true})
	require.NoError(t, err)

	for _, test := range []struct {
		params rc.Params
		want   string
	}{
		{rc.Params{"token": `{"access_token":"AAA"}`}, "false"},
		{rc.Params{"token": `{"access_token":"AAA"}`, "config_refresh_token": "true"}, "true"},
		{rc.Params{}, ""},
	} {
		answer = ""
		out, err := config.UpdateRemote(ctx, "tok", test.params, config.UpdateRemoteOpt{NonInteractive: true})
		require.NoError(t, err)
		assert.Equal(t, test.want, answer, test.params)
		if test.want == "" {
			require.NotNil(t, out)
			assert.Equal(t, "config_refresh_token", out.Option.Name)
		}
	}
}
