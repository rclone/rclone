package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	require.NoError(t, os.Setenv("EXPAND_TEST", "potato"))
	defer func() {
		require.NoError(t, os.Unsetenv("EXPAND_TEST"))
	}()
	for _, test := range []struct {
		in, want string
	}{
		{"", ""},
		{"~", filepath.FromSlash(home)},
		{filepath.FromSlash("~/dir/file.txt"), filepath.FromSlash(home + "/dir/file.txt")},
		{filepath.FromSlash("/dir/~/file.txt"), filepath.FromSlash("/dir/~/file.txt")},
		{filepath.FromSlash("~/${EXPAND_TEST}"), filepath.FromSlash(home + "/potato")},
	} {
		got := ShellExpand(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}
