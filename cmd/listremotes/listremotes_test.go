package ls

import (
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetFilterFlags() {
	filterName = ""
	filterType = ""
	filterSource = ""
	filterDescription = ""
}

func TestTypeFilterDefaultIsFuzzy(t *testing.T) {
	resetFilterFlags()
	filterType = "box"
	t.Cleanup(resetFilterFlags)

	filters, err := compileFilters("", false)
	require.NoError(t, err)

	assert.True(t, includeRemote(config.Remote{Type: "box"}, filters))
	assert.True(t, includeRemote(config.Remote{Type: "dropbox"}, filters))
}

func TestTypeFilterExactMatchesWholeValue(t *testing.T) {
	resetFilterFlags()
	filterType = "box"
	t.Cleanup(resetFilterFlags)

	filters, err := compileFilters("", true)
	require.NoError(t, err)

	assert.True(t, includeRemote(config.Remote{Type: "box"}, filters))
	assert.True(t, includeRemote(config.Remote{Type: "BoX"}, filters))
	assert.False(t, includeRemote(config.Remote{Type: "dropbox"}, filters))
}

func TestPositionalFilterExactAlsoMatchesWholeValue(t *testing.T) {
	resetFilterFlags()
	t.Cleanup(resetFilterFlags)

	filters, err := compileFilters("box", true)
	require.NoError(t, err)

	assert.True(t, includeRemote(config.Remote{Type: "box"}, filters))
	assert.False(t, includeRemote(config.Remote{Name: "mybox"}, filters))
	assert.False(t, includeRemote(config.Remote{Description: "my dropbox remote"}, filters))
}
