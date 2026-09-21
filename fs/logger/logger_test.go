//go:build !plan9

package logger_test

import (
	"fmt"
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
	testscript.Main(m, map[string]func(){
		"rclone": logger.Main,
	})
}

func TestLogger(t *testing.T) {
	// Usage: https://bitfieldconsulting.com/golang/cli-testing

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			src := filepath.Join(env.WorkDir, "src")
			dst := filepath.Join(env.WorkDir, "dst")
			// Fill src and dst with two overlapping trees of files so the
			// scripts have a realistic mix of matching, differing and
			// missing files to compare. This used to download two old
			// rclone source archives from GitHub which made the tests fail
			// whenever the network or GitHub was flaky.
			if err := makeTestTrees(src, dst); err != nil {
				return err
			}
			env.Setenv("SRC", src)
			env.Setenv("DST", dst)
			return nil
		},
	})
}

// makeTestTrees populates src and dst with a deterministic set of files
// covering every comparison category the scripts exercise:
//
//   - identical files present in both (match)
//   - same-named files with different content (differ)
//   - files only in src (missing on dst)
//   - files only in dst (missing on src)
//
// The files are spread across the root and a shared subdirectory so the
// listings need sorting and the directory survives a sync (which deletes the
// dst-only files but keeps the matching ones).
func makeTestTrees(src, dst string) error {
	const perCategory = 25
	dirs := []string{".", "sub"}

	write := func(root, dir, name, content string) error {
		path := filepath.Join(root, dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), 0666)
	}

	for _, dir := range dirs {
		for i := range perCategory {
			match := fmt.Sprintf("the same content for match%02d\n", i)
			if err := write(src, dir, fmt.Sprintf("match%02d.txt", i), match); err != nil {
				return err
			}
			if err := write(dst, dir, fmt.Sprintf("match%02d.txt", i), match); err != nil {
				return err
			}

			if err := write(src, dir, fmt.Sprintf("differ%02d.txt", i), fmt.Sprintf("src content %02d\n", i)); err != nil {
				return err
			}
			if err := write(dst, dir, fmt.Sprintf("differ%02d.txt", i), fmt.Sprintf("dst content %02d differs\n", i)); err != nil {
				return err
			}

			if err := write(src, dir, fmt.Sprintf("srconly%02d.txt", i), fmt.Sprintf("only in src %02d\n", i)); err != nil {
				return err
			}
			if err := write(dst, dir, fmt.Sprintf("dstonly%02d.txt", i), fmt.Sprintf("only in dst %02d\n", i)); err != nil {
				return err
			}
		}
	}
	return nil
}
