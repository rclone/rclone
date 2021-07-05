package atexit

import (
	"os"
	"runtime"
	"testing"

	"github.com/rclone/rclone/lib/exitcode"
	"github.com/stretchr/testify/assert"
)

type fakeSignal struct{}

func (*fakeSignal) String() string {
	return "fake"
}

func (*fakeSignal) Signal() {
}

var _ os.Signal = (*fakeSignal)(nil)

func TestExitCode(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "plan9":
		for _, i := range []os.Signal{
			os.Interrupt,
			os.Kill,
		} {
			assert.Equal(t, exitCode(i), exitcode.UncategorizedError)
		}

	default:
		// SIGINT (2) and SIGKILL (9) are portable numbers specified by POSIX.
		assert.Equal(t, exitCode(os.Interrupt), 128+2)
		assert.Equal(t, exitCode(os.Kill), 128+9)
	}

	// Never a real signal
	assert.Equal(t, exitCode(&fakeSignal{}), exitcode.UncategorizedError)
}
