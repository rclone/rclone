package version

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/config"
	"github.com/stretchr/testify/assert"
)

func TestVersionWorksWithoutAccessibleConfigFile(t *testing.T) {
	// create temp config file
	tempFile, err := ioutil.TempFile("", "unreadable_config.conf")
	assert.NoError(t, err)
	path := tempFile.Name()
	defer func() {
		err := os.Remove(path)
		assert.NoError(t, err)
	}()
	assert.NoError(t, tempFile.Close())
	assert.NoError(t, os.Chmod(path, 0000))
	// re-wire
	oldOsStdout := os.Stdout
	oldConfigPath := config.ConfigPath
	config.ConfigPath = path
	os.Stdout = nil
	defer func() {
		os.Stdout = oldOsStdout
		config.ConfigPath = oldConfigPath
	}()

	cmd.Root.SetArgs([]string{"version"})
	assert.NotPanics(t, func() {
		assert.NoError(t, cmd.Root.Execute())
	})

	// This causes rclone to exit and the tests to stop!
	// cmd.Root.SetArgs([]string{"--version"})
	// assert.NotPanics(t, func() {
	// 	assert.NoError(t, cmd.Root.Execute())
	// })
}

func TestVersionNew(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    version
		wantErr bool
	}{
		{"v1.41", version{1, 41}, false},
		{"rclone v1.41", version{1, 41}, false},
		{"rclone v1.41.23", version{1, 41, 23}, false},
		{"rclone v1.41.23-100", version{1, 41, 23, 100}, false},
		{"rclone v1.41-100", version{1, 41, 0, 100}, false},
		{"rclone v1.41.23-100-g12312a", version{1, 41, 23, 100}, false},
		{"rclone v1.41-100-g12312a", version{1, 41, 0, 100}, false},
		{"rclone v1.42-005-g56e1e820β", version{1, 42, 0, 5}, false},
		{"rclone v1.42-005-g56e1e820-feature-branchβ", version{1, 42, 0, 5}, false},

		{"v1.41s", nil, true},
		{"rclone v1-41", nil, true},
		{"rclone v1.41.2c3", nil, true},
		{"rclone v1.41.23-100 potato", nil, true},
		{"rclone 1.41-100", nil, true},
		{"rclone v1.41.23-100-12312a", nil, true},
	} {
		what := fmt.Sprintf("in=%q", test.in)
		got, err := newVersion(test.in)
		if test.wantErr {
			assert.Error(t, err, what)
		} else {
			assert.NoError(t, err, what)
		}
		assert.Equal(t, test.want, got, what)
	}

}

func TestVersionCmp(t *testing.T) {
	for _, test := range []struct {
		a, b version
		want int
	}{
		{version{1}, version{1}, 0},
		{version{1}, version{2}, -1},
		{version{2}, version{1}, 1},
		{version{2}, version{2, 1}, -1},
		{version{2, 1}, version{2}, 1},
		{version{2, 1}, version{2, 1}, 0},
		{version{2, 1}, version{2, 2}, -1},
		{version{2, 2}, version{2, 1}, 1},
	} {
		got := test.a.cmp(test.b)
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		assert.Equal(t, test.want, got, fmt.Sprintf("%v cmp %v", test.a, test.b))
		// test the reverse
		got = -test.b.cmp(test.a)
		assert.Equal(t, test.want, got, fmt.Sprintf("%v cmp %v", test.b, test.a))
	}
}
