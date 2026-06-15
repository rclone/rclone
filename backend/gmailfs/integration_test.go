// Integration tests for the gmailfs backend plus the registration-presence
// guard that checks the backend is wired into all the rclone site/nav files.
package gmailfs

import (
	"context"
	"errors"
	"os"
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
		*fstest.RemoteName = "TestGmailFs:"
	}
	f, err := fs.NewFs(ctx, *fstest.RemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("Couldn't create Gmail backend - skipping integration tests: %v", err)
	}
	require.NoError(t, err)

	t.Run("RootList", func(t *testing.T) {
		entries, err := f.List(ctx, "")
		require.NoError(t, err)
		assert.NotEmpty(t, entries, "root list must return at least one year")
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

// sharedRegistrationFiles are the rclone site/nav registration points that must
// mention both backends.
var sharedRegistrationFiles = []string{
	"backend/all/all.go",
	"fstest/test_all/config.yaml",
	"docs/layouts/chrome/navbar.html",
	"bin/make_manual.py",
	"README.md",
	"docs/content/docs.md",
	"docs/content/_index.md",
}

// perBackendFiles are the per-backend doc and data files that must each exist and
// reference their own backend.
var perBackendFiles = []string{
	"docs/content/gmailfs.md",
	"docs/content/gcalfs.md",
	"docs/data/backends/gmailfs.yaml",
	"docs/data/backends/gcalfs.yaml",
}

// repoRoot walks up from the package directory (backend/gmailfs) to the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	// backend/gmailfs -> repo root is two levels up.
	return wd + "/../.."
}

// hasGmail reports whether content references the Gmail backend.
func hasGmail(content string) bool {
	return strings.Contains(content, "gmailfs") || strings.Contains(content, "Gmail")
}

// hasGcal reports whether content references the Google Calendar backend.
func hasGcal(content string) bool {
	return strings.Contains(content, "gcalfs") || strings.Contains(content, "Google Calendar")
}

// TestRegistrationFilePresence verifies that the gmailfs and gcalfs backends are
// registered in every required rclone site/navigation file. Shared files must
// reference both backends; per-backend doc and data files must reference their own.
func TestRegistrationFilePresence(t *testing.T) {
	root := repoRoot(t)

	for _, rel := range sharedRegistrationFiles {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(root + "/" + rel)
			require.NoError(t, err, "registration file must exist: %s", rel)
			content := string(data)
			assert.True(t, hasGmail(content), "%s must reference the Gmail backend", rel)
			assert.True(t, hasGcal(content), "%s must reference the Google Calendar backend", rel)
		})
	}

	for _, rel := range perBackendFiles {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(root + "/" + rel)
			require.NoError(t, err, "per-backend file must exist: %s", rel)
			content := string(data)
			if strings.Contains(rel, "gmailfs") {
				assert.True(t, hasGmail(content), "%s must reference the Gmail backend", rel)
			} else {
				assert.True(t, hasGcal(content), "%s must reference the Google Calendar backend", rel)
			}
		})
	}
}
