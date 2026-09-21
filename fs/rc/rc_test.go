package rc

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/rclone/rclone/fs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	err := WriteJSON(&buf, Params{
		"String": "hello",
		"Int":    42,
	})
	require.NoError(t, err)
	assert.Equal(t, `{
	"Int": 42,
	"String": "hello"
}
`, buf.String())
}

// Check --rc-log-calls can be turned off and on again with options/set
func TestLogCall(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	oldLevel := fs.GetConfig(ctx).LogLevel
	fs.GetConfig(ctx).LogLevel = fs.LogLevelDebug
	fs.SetLogger(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	defer func() {
		fs.GetConfig(ctx).LogLevel = oldLevel
		fs.SetLogger(slog.NewTextHandler(io.Discard, nil))
	}()

	set := func(logCalls bool) {
		call := Calls.Get("options/set")
		require.NotNil(t, call)
		_, err := call.Fn(ctx, Params{"rc": Params{"LogCalls": logCalls}})
		require.NoError(t, err)
	}
	defer set(true)

	// Logged by default
	LogCall("rc: %s: called", "core/stats")
	assert.Contains(t, buf.String(), "rc: core/stats: called")

	// Not logged when turned off
	set(false)
	buf.Reset()
	LogCall("rc: %s: called", "core/stats")
	assert.Equal(t, "", buf.String())

	// and logged again when turned back on
	set(true)
	LogCall("rc: %s: called", "core/stats")
	assert.Contains(t, buf.String(), "rc: core/stats: called")
}
