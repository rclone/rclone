//go:build !plan9
// +build !plan9

package logger_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/logger"
	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	// This enables the testscript package. See:
	// https://bitfieldconsulting.com/golang/cli-testing
	// https://pkg.go.dev/github.com/rogpeppe/go-internal@v1.11.0/testscript
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"rclone": logger.Main,
	}))
}

func TestLogger(t *testing.T) {
	// Usage: https://bitfieldconsulting.com/golang/cli-testing

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("SRC", filepath.Join("$WORK", "src"))
			env.Setenv("DST", filepath.Join("$WORK", "dst"))
			return nil
		},
	})
}
