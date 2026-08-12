package overview

import (
	"strings"
	"testing"

	"github.com/rclone/rclone/docs/data/backends"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBackendConfig(t *testing.T) {
	// s3 tier test
	conf, err := GetBackendConfig("S3")
	require.NoError(t, err, "failed to load s3.yaml")
	require.NotNil(t, conf, "config should not be nil")

	assert.Equal(t, "Tier 1", conf.Tier, "s3 should be tier 1")

	// memory backend (unlikely to change)
	conf, err = GetBackendConfig("memory")
	require.NoError(t, err, "failed to load s3.yaml")
	require.NotNil(t, conf, "config should not be nil")

	expectedMemoryConfig := &BackendConfig{
		Backend:          "memory",
		Name:             "Memory",
		Tier:             "Tier 1",
		Maintainers:      "Core",
		FeaturesScore:    4,
		IntegrationTests: "Passing",
		DataIntegrity:    "Hash",
		Performance:      "High",
		Adoption:         "Widely used",
		Docs:             "Full",
		Security:         "High",
		Virtual:          false,
		Remote:           ":memory:",
		Features: []string{
			"BucketBased",
			"BucketBasedRootOK",
			"Copy",
			"ListP",
			"ListR",
			"PutStream",
			"ReadMimeType",
			"WriteMimeType",
		},
		Hashes: []string{
			"md5",
		},
		Precision: 1,
	}

	assert.Equal(t, expectedMemoryConfig, conf, "parsed memory.yaml should match")

}

// Check every embedded backend YAML file parses on all architectures
func TestGetBackendConfigAll(t *testing.T) {
	entries, err := backends.BackendFS.ReadDir(".")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		conf, err := GetBackendConfig(name)
		assert.NoError(t, err, "failed to load %s", entry.Name())
		assert.NotNil(t, conf)
	}
}
