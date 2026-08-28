package imagekit

import (
	"testing"

	"github.com/PhateValleyman/rclone/fstest"
	"github.com/PhateValleyman/rclone/fstest/fstests"
)

func TestIntegration(t *testing.T) {
	debug := true
	fstest.Verbose = &debug
	fstests.Run(t, &fstests.Opt{
		RemoteName:      "TestImageKit:",
		NilObject:       (*Object)(nil),
		SkipFsCheckWrap: true,
	})
}
