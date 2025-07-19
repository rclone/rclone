package internxt

import (
	"context"
	"errors"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestInternxt:",
		NilObject:  (*Object)(nil),
	})
}

// TestMakeDir verifies that basic operations (such as mkdir) can be performed
func TestMakeDir(t *testing.T) {
	const (
		remoteName = "TestInternxt:"
	)
	ctx := context.Background()
	fstest.Initialise()
	subRemoteName, _, err := fstest.RandomRemoteName(remoteName)
	require.NoError(t, err)
	f, err := fs.NewFs(ctx, subRemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Logf("Didn't find %q in config file - skipping tests", remoteName)
		return
	}
	require.NoError(t, err)

	entr, err := f.List(ctx, "")
	t.Log(entr)
	require.NoError(t, err)

	err = f.Mkdir(ctx, "hello-integration-test")
	require.NoError(t, err)

	// Tear down
	require.NoError(t, operations.Purge(ctx, f, ""))
}
