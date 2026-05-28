package oauthutil

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRcOAuthStatus(t *testing.T) {
	call := rc.Calls.Get("config/oauthstatus")
	require.NotNil(t, call)
	ctx := context.Background()

	// Status should be "stopped" when no OAuth is running
	out, err := call.Fn(ctx, rc.Params{})
	require.NoError(t, err)
	assert.Equal(t, "stopped", out["status"])

	// Simulate an active OAuth flow
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	oauthCancelMu.Lock()
	oauthCancelFn = cancel
	oauthURL = "http://127.0.0.1:53682/auth?state=xyz"
	oauthCancelMu.Unlock()
	defer func() {
		oauthCancelMu.Lock()
		oauthCancelFn = nil
		oauthURL = ""
		oauthCancelMu.Unlock()
	}()

	// Status should be "running"
	out, err = call.Fn(ctx2, rc.Params{})
	require.NoError(t, err)
	assert.Equal(t, "running", out["status"])
	assert.Equal(t, "http://127.0.0.1:53682/auth?state=xyz", out["url"])
}

func TestRcOAuthStop(t *testing.T) {
	call := rc.Calls.Get("config/oauthstop")
	require.NotNil(t, call)
	ctx := context.Background()

	// Stop should return error when no OAuth is running
	_, err := call.Fn(ctx, rc.Params{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no oauth authentication is in progress")

	// Simulate an active OAuth flow
	_, cancel := context.WithCancel(ctx)
	oauthCancelMu.Lock()
	oauthCancelFn = cancel
	oauthCancelMu.Unlock()

	// Stop should succeed
	out, err := call.Fn(ctx, rc.Params{})
	require.NoError(t, err)
	assert.Nil(t, out)

	// Subsequent stop should return error
	_, err = call.Fn(ctx, rc.Params{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no oauth authentication is in progress")
}
