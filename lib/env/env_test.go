package env

import (
	"os"
	"testing"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellExpand(t *testing.T) {
	home, err := homedir.Dir()
	require.NoError(t, err)
	require.NoError(t, os.Setenv("EXPAND_TEST", "potato"))
	defer func() {
		require.NoError(t, os.Unsetenv("EXPAND_TEST"))
	}()
	for _, test := range []struct {
		in, want string
	}{
		{"", ""},
		{"~", home},
		{"~/dir/file.txt", home + "/dir/file.txt"},
		{"/dir/~/file.txt", "/dir/~/file.txt"},
		{"~/${EXPAND_TEST}", home + "/potato"},
	} {
		got := ShellExpand(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}
