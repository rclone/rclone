package bisync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBisyncRejectsNameTransform(t *testing.T) {
	for _, test := range []struct {
		name   string
		resync bool
	}{
		{name: "normal", resync: false},
		{name: "resync", resync: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			ci.NameTransform = []string{"lowercase"}

			root := t.TempDir()
			path1 := filepath.Join(root, "one")
			path2 := filepath.Join(root, "two")
			require.NoError(t, os.MkdirAll(path1, 0o755))
			require.NoError(t, os.MkdirAll(path2, 0o755))
			report := filepath.Join(path1, "Report.TXT")
			note := filepath.Join(path1, "note.txt")
			require.NoError(t, os.WriteFile(report, []byte("report\n"), 0o644))
			require.NoError(t, os.WriteFile(note, []byte("1\n"), 0o644))

			fs1, err := cache.Get(ctx, path1)
			require.NoError(t, err)
			fs2, err := cache.Get(ctx, path2)
			require.NoError(t, err)

			workdir := filepath.Join(root, "workdir")
			err = Bisync(ctx, fs1, fs2, &Options{Resync: test.resync, Workdir: workdir})
			require.EqualError(t, err, "--name-transform is not supported by bisync")

			got, err := os.ReadFile(report)
			require.NoError(t, err)
			assert.Equal(t, "report\n", string(got))
			entries, err := os.ReadDir(path1)
			require.NoError(t, err)
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
			assert.ElementsMatch(t, []string{"Report.TXT", "note.txt"}, names)
			got, err = os.ReadFile(note)
			require.NoError(t, err)
			assert.Equal(t, "1\n", string(got))
			_, err = os.Stat(workdir)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}
