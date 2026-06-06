package fs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	ctx := context.Background()

	// Check nil
	//lint:ignore SA1012 false positive when running staticcheck, we want to test passing a nil Context and therefore ignore lint suggestion to use context.TODO
	//nolint:staticcheck // Don't include staticcheck when running golangci-lint to avoid SA1012
	config := GetConfig(nil)
	assert.Equal(t, globalConfig, config)

	// Check empty config
	config = GetConfig(ctx)
	assert.Equal(t, globalConfig, config)

	// Check adding a config
	ctx2, config2 := AddConfig(ctx)
	config2.Transfers++
	assert.NotEqual(t, config2, config)

	// Check can get config back
	config2ctx := GetConfig(ctx2)
	assert.Equal(t, config2, config2ctx)
}

// The rc request marker must survive CopyConfig, which is how rclone
// does detach context but keep config.
func TestRCRequestContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsRCRequest(ctx))

	rcCtx := WithRCRequest(ctx)
	assert.True(t, IsRCRequest(rcCtx))

	// CopyConfig carries the marker even when there is no config to copy
	assert.True(t, IsRCRequest(CopyConfig(context.Background(), rcCtx)))
	// and when there is
	rcCtx, _ = AddConfig(rcCtx)
	assert.True(t, IsRCRequest(CopyConfig(context.Background(), rcCtx)))
	// An unmarked context stays unmarked
	assert.False(t, IsRCRequest(CopyConfig(context.Background(), ctx)))
}
