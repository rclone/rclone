package lsjson

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The listing must be written to the given writer, not to standard output.
func TestLsJSONWritesToWriter(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "../lsf/testfiles")
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	require.NoError(t, lsJSON(context.Background(), f, "", buf))

	var items []struct{ Path string }
	require.NoError(t, json.Unmarshal(buf.Bytes(), &items), buf.String())
	var names []string
	for _, item := range items {
		names = append(names, item.Path)
	}
	assert.ElementsMatch(t, []string{"file1", "file2", "file3", "subdir"}, names)
}

func TestLsJSONStatWritesToWriter(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "../lsf/testfiles")
	require.NoError(t, err)

	statOnly = true
	defer func() { statOnly = false }()
	buf := new(bytes.Buffer)
	require.NoError(t, lsJSON(context.Background(), f, "file1", buf))

	var item struct{ Path string }
	require.NoError(t, json.Unmarshal(buf.Bytes(), &item), buf.String())
	assert.Equal(t, "file1", item.Path)
}

func TestLsJSONEmptyDir(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), t.TempDir())
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	require.NoError(t, lsJSON(context.Background(), f, "", buf))
	assert.Equal(t, "[\n]\n", buf.String())
}
