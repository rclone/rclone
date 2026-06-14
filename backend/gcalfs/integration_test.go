// Integration tests for the gcalfs (Google Calendar) backend.
package gcalfs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()

	if *fstest.RemoteName == "" {
		*fstest.RemoteName = "TestGcalFs:"
	}
	f, err := fs.NewFs(ctx, *fstest.RemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("Couldn't create Google Calendar backend - skipping integration tests: %v", err)
	}
	require.NoError(t, err)

	t.Run("RootList", func(t *testing.T) {
		entries, err := f.List(ctx, "")
		require.NoError(t, err)
		assert.NotEmpty(t, entries, "root list must return at least one entry")
	})

	t.Run("WriteOpsReturnError", func(t *testing.T) {
		src := object.NewStaticObjectInfo("test.txt", time.Now(), 4, true, nil, nil)
		_, err := f.Put(ctx, strings.NewReader("data"), src)
		assert.Error(t, err, "Put must return error on read-only backend")

		err = f.Mkdir(ctx, "testdir")
		assert.Error(t, err, "Mkdir must return error on read-only backend")

		err = f.Rmdir(ctx, "testdir")
		assert.Error(t, err, "Rmdir must return error on read-only backend")
	})
}
