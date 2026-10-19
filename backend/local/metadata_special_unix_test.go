//go:build !windows && !plan9 && !js

package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The "mode" metadata on unix is the st_mode as this backend reads it,
// e.g. "42775" for a setgid directory and "104755" for a setuid file.
func TestMetadataUnixSpecialBits(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	f := r.Flocal.(*Fs)
	const special = os.ModeSetuid | os.ModeSetgid | os.ModeSticky

	statMode := func(name string) os.FileMode {
		t.Helper()
		fi, err := os.Lstat(filepath.Join(f.root, name))
		require.NoError(t, err)
		return fi.Mode()
	}
	writeFile := func(name, mode string) {
		t.Helper()
		r.WriteFile(name, "payload", time.Now())
		obj, err := f.NewObject(ctx, name)
		require.NoError(t, err)
		require.NoError(t, obj.(*Object).writeMetadataToFile(fs.Metadata{"mode": mode}))
	}

	t.Run("SetgidDirectoryKept", func(t *testing.T) {
		_, err := f.MkdirMetadata(ctx, "shared", fs.Metadata{"mode": "42775"})
		require.NoError(t, err)
		mode := statMode("shared")
		assert.Equal(t, os.FileMode(0o775), mode.Perm())
		assert.Equal(t, os.ModeSetgid, mode&special, mode.String())
	})

	t.Run("StickyDirectoryStripped", func(t *testing.T) {
		_, err := f.MkdirMetadata(ctx, "tmp", fs.Metadata{"mode": "41777"})
		require.NoError(t, err)
		assert.Zero(t, statMode("tmp")&special)
	})

	t.Run("FilesStrippedByDefault", func(t *testing.T) {
		writeFile("setuid.bin", "104755")
		writeFile("setgid.bin", "102755")
		// a source that claims a directory mode for a file gets no setgid
		writeFile("claims-dir.bin", "42775")
		for name, perm := range map[string]os.FileMode{"setuid.bin": 0o755, "setgid.bin": 0o755, "claims-dir.bin": 0o775} {
			mode := statMode(name)
			assert.Equal(t, perm, mode.Perm(), name)
			assert.Zero(t, mode&special, name+" "+mode.String())
		}
	})

	t.Run("RestoredWithFlag", func(t *testing.T) {
		f.opt.MetadataRestoreSpecial = true
		defer func() { f.opt.MetadataRestoreSpecial = false }()
		writeFile("restored.bin", "104755")
		assert.Equal(t, os.ModeSetuid, statMode("restored.bin")&special)
		_, err := f.MkdirMetadata(ctx, "restored-tmp", fs.Metadata{"mode": "41777"})
		require.NoError(t, err)
		assert.Equal(t, os.ModeSticky, statMode("restored-tmp")&special)
	})
}

func TestModeFromMetadata(t *testing.T) {
	// unix st_mode
	assert.Equal(t, os.ModeSetgid|0o775, modeFromMetadata(0o2775))
	assert.Equal(t, os.ModeSetuid|0o755, modeFromMetadata(0o4755))
	assert.Equal(t, os.ModeSticky|0o777, modeFromMetadata(0o1777))
	assert.Equal(t, os.FileMode(0o644), modeFromMetadata(0o100644).Perm())
	// os.FileMode, as written on other OSes, keeps its own bits
	assert.Equal(t, os.ModeSetuid|0o755, modeFromMetadata(uint32(os.ModeSetuid|0o755)))
}
