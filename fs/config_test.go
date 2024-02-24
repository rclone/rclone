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
