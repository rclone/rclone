package box

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test box_config_file has ~ and environment variables expanded
func TestGetBoxConfigShellExpand(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"enterpriseID":"enterprise-id"}`), 0600))
	t.Setenv("RCLONE_TEST_BOX_CONFIG_DIR", dir)
	m := configmap.Simple{
		"box_config_file": "$RCLONE_TEST_BOX_CONFIG_DIR/config.json",
	}
	boxConfig, err := getBoxConfig(m)
	require.NoError(t, err)
	assert.Equal(t, "enterprise-id", boxConfig.EnterpriseID)
}
