// Test discord filesystem interface
package discord_test

import (
	"testing"

	"github.com/rclone/rclone/backend/discord"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDiscord:",
		NilObject:  (*discord.Object)(nil),
	})
}
