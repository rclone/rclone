// Remove the unused import if the package is not needed
import (
	"testing"
	"github.com/rclone/rclone/fstest/fstests"
)

func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFileLu:",
		NilObject:  nil,
	})
}
