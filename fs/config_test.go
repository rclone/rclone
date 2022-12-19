package fs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	ctx := context.Background()

	// Check nil
	config := GetConfig(nil) //lint:ignore SA1012 we want to test passing a nil Context and therefore ignore lint suggestion of using context.TODO
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
