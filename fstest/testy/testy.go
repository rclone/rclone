// Package testy contains test utilities for rclone
package testy

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

// CI returns true if we are running on the CI server
func CI() bool {
	return os.Getenv("CI") != ""
}

// SkipUnreliable skips this test if running on CI
func SkipUnreliable(t *testing.T) {
	if !CI() {
		return
	}
	t.Skip("Skipping Unreliable Test on CI")
}

var dockerOnce struct {
	sync.Once
	ok bool
}

// HaveDocker returns true if the fstest/testserver docker framework can
// be used on this host. The framework brings up containers (e.g. minio)
// by exec'ing the bash init.d scripts which in turn shell out to docker,
// so it needs both a POSIX shell (not available on Windows) and a
// reachable docker daemon. The result is cached after the first call.
func HaveDocker() bool {
	dockerOnce.Do(func() {
		// The init.d scripts are bash and cannot be exec'd on Windows.
		if runtime.GOOS == "windows" {
			return
		}
		// docker version exits non-zero if the client is installed but
		// the daemon is unreachable, which is exactly what we want to
		// detect.
		dockerOnce.ok = exec.Command("docker", "version").Run() == nil
	})
	return dockerOnce.ok
}

// SkipUnlessDocker skips this test unless a working docker daemon is
// available. Use it for tests that rely on the fstest/testserver docker
// framework, which is not available on all platforms (e.g. Windows and
// macOS CI runners).
func SkipUnlessDocker(t *testing.T) {
	if !HaveDocker() {
		t.Skip("Skipping test as docker is not available")
	}
}
