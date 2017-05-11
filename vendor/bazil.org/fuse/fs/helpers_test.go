package fs_test

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var childHelpers = map[string]func(){}

type childProcess struct {
	name string
	fn   func()
}

var _ flag.Value = (*childProcess)(nil)

func (c *childProcess) String() string {
	return c.name
}

func (c *childProcess) Set(s string) error {
	fn, ok := childHelpers[s]
	if !ok {
		return errors.New("helper not found")
	}
	c.name = s
	c.fn = fn
	return nil
}

var childMode childProcess

func init() {
	flag.Var(&childMode, "fuse.internal.child", "internal use only")
}

// childCmd prepares a test function to be run in a subprocess, with
// childMode set to true. Caller must still call Run or Start.
//
// Re-using the test executable as the subprocess is useful because
// now test executables can e.g. be cross-compiled, transferred
// between hosts, and run in settings where the whole Go development
// environment is not installed.
func childCmd(childName string) (*exec.Cmd, error) {
	// caller may set cwd, so we can't rely on relative paths
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(executable, "-fuse.internal.child="+childName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}

func TestMain(m *testing.M) {
	flag.Parse()
	if childMode.fn != nil {
		childMode.fn()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
