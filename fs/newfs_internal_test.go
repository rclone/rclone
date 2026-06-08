package fs

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When no override/global keys exist, ctx must be returned unchanged.
func TestAddConfigToContext_NoChanges(t *testing.T) {
	ctx := context.Background()
	newCtx, err := addConfigToContext(ctx, "unit-test", configmap.Simple{})
	require.NoError(t, err)
	assert.Equal(t, newCtx, ctx)
}

// A single override.key must create a new ctx, but leave the
// background ctx untouched.
func TestAddConfigToContext_OverrideOnly(t *testing.T) {
	override := configmap.Simple{
		"override.user_agent": "potato",
	}
	ctx := context.Background()
	globalCI := GetConfig(ctx)
	original := globalCI.UserAgent
	newCtx, err := addConfigToContext(ctx, "unit-test", override)
	require.NoError(t, err)
	assert.NotEqual(t, newCtx, ctx)
	assert.Equal(t, original, globalCI.UserAgent)
	ci := GetConfig(newCtx)
	assert.Equal(t, "potato", ci.UserAgent)
}

// A single global.key must create a new ctx and update the
// background/global config.
func TestAddConfigToContext_GlobalOnly(t *testing.T) {
	global := configmap.Simple{
		"global.user_agent": "potato2",
	}
	ctx := context.Background()
	globalCI := GetConfig(ctx)
	original := globalCI.UserAgent
	defer func() {
		globalCI.UserAgent = original
	}()
	newCtx, err := addConfigToContext(ctx, "unit-test", global)
	require.NoError(t, err)
	assert.NotEqual(t, newCtx, ctx)
	assert.Equal(t, "potato2", globalCI.UserAgent)
	ci := GetConfig(newCtx)
	assert.Equal(t, "potato2", ci.UserAgent)
}

// When the ctx is marked as a remote control (rc) request, a global.key must
// apply to the backend's own ctx but must NOT change the process-wide config.
func TestAddConfigToContext_GlobalFromRC(t *testing.T) {
	global := configmap.Simple{
		"global.user_agent": "potato3",
	}
	ctx := WithRCRequest(context.Background())
	globalCI := GetConfig(ctx)
	original := globalCI.UserAgent
	defer func() {
		globalCI.UserAgent = original
	}()
	newCtx, err := addConfigToContext(ctx, "unit-test", global)
	require.NoError(t, err)
	assert.NotEqual(t, newCtx, ctx)
	// The process-wide config must be untouched
	assert.Equal(t, original, globalCI.UserAgent)
	// but the backend's own ctx still gets the value
	ci := GetConfig(newCtx)
	assert.Equal(t, "potato3", ci.UserAgent)
}
