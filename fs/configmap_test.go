package fs

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureLog redirects the fs logger into a buffer and enables debug
// logging for the duration of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	oldLogger := logger
	t.Cleanup(func() { logger = oldLogger })
	SetLogger(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ci := GetConfig(context.Background())
	oldLevel := ci.LogLevel
	t.Cleanup(func() { ci.LogLevel = oldLevel })
	ci.LogLevel = LogLevelDebug
	return &buf
}

// TestConfigEnvVarsRedactsPassword checks that the value of a
// password config option set via the environment
// (RCLONE_CONFIG_backend_option) is not logged (see #5794).
func TestConfigEnvVarsRedactsPassword(t *testing.T) {
	t.Setenv("RCLONE_CONFIG_SRC_PASS", "obscured_pass")
	t.Setenv("RCLONE_CONFIG_SRC_HOST", "example.com")
	buf := captureLog(t)

	config := configEnvVars{
		configName: "src",
		options: Options{
			{Name: "pass", IsPassword: true},
			{Name: "host"},
		},
	}

	// The value is still returned to the caller...
	value, ok := config.Get("pass")
	assert.True(t, ok)
	assert.Equal(t, "obscured_pass", value)
	// ...but not logged
	assert.NotContains(t, buf.String(), "obscured_pass")

	// Non sensitive values are still logged
	buf.Reset()
	value, ok = config.Get("host")
	assert.True(t, ok)
	assert.Equal(t, "example.com", value)
	assert.Contains(t, buf.String(), "example.com")
}

// TestConfigEnvVarsDumpAuthShowsPassword checks that with --dump auth
// set the password value is still shown in the log for debugging.
func TestConfigEnvVarsDumpAuthShowsPassword(t *testing.T) {
	t.Setenv("RCLONE_CONFIG_SRC_PASS", "obscured_pass")
	buf := captureLog(t)
	ci := GetConfig(context.Background())
	oldDump := ci.Dump
	t.Cleanup(func() { ci.Dump = oldDump })
	ci.Dump = DumpAuth

	config := configEnvVars{
		configName: "src",
		options: Options{
			{Name: "pass", IsPassword: true},
		},
	}
	_, ok := config.Get("pass")
	assert.True(t, ok)
	assert.Contains(t, buf.String(), "obscured_pass")
}

// TestOptionEnvVarsRedactsPassword checks that the value of a
// password option set via the environment (RCLONE_backend_option) is
// not logged (see #5794).
func TestOptionEnvVarsRedactsPassword(t *testing.T) {
	t.Setenv("RCLONE_SFTP_PASS", "obscured_pass")
	buf := captureLog(t)

	oev := optionEnvVars{
		prefix:  "sftp",
		options: Options{{Name: "pass", IsPassword: true}},
	}
	value, ok := oev.Get("pass")
	assert.True(t, ok)
	assert.Equal(t, "obscured_pass", value)
	assert.NotContains(t, buf.String(), "obscured_pass")
}
