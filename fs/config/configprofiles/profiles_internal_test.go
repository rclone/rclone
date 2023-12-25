package profiles

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfileRequested(t *testing.T) {
	// Clear any RCLONE_PROFILE* env vars set in the caller's
	// environment so they don't leak into the test cases. t.Setenv
	// restores them automatically.
	t.Setenv("RCLONE_PROFILE", "")
	t.Setenv("RCLONE_PROFILE_SAVE", "")

	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()

	cases := []struct {
		name   string
		args   []string
		env    map[string]string
		expect bool
	}{
		{"no flag, no env", []string{"rclone", "ls", "remote:"}, nil, false},
		{"--profile NAME", []string{"rclone", "ls", "remote:", "--profile", "x"}, nil, true},
		{"--profile=NAME", []string{"rclone", "ls", "remote:", "--profile=x"}, nil, true},
		{"--profile-save NAME", []string{"rclone", "ls", "remote:", "--profile-save", "x"}, nil, true},
		{"--profile-save=NAME", []string{"rclone", "ls", "remote:", "--profile-save=x"}, nil, true},
		{"--profile-save-args alone does not trigger", []string{"rclone", "ls", "remote:", "--profile-save-args"}, nil, false},
		{"--profile-strict-flags alone does not trigger", []string{"rclone", "ls", "remote:", "--profile-strict-flags"}, nil, false},
		{"RCLONE_PROFILE env var", []string{"rclone", "ls", "remote:"}, map[string]string{"RCLONE_PROFILE": "x"}, true},
		{"RCLONE_PROFILE_SAVE env var", []string{"rclone", "ls", "remote:"}, map[string]string{"RCLONE_PROFILE_SAVE": "x"}, true},
		{"empty RCLONE_PROFILE does not trigger", []string{"rclone", "ls", "remote:"}, map[string]string{"RCLONE_PROFILE": ""}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			os.Args = c.args
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			assert.Equal(t, c.expect, profileRequested())
		})
	}
}
