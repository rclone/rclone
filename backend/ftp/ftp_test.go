// Test FTP filesystem interface
package ftp_test

import (
	"testing"

	"github.com/rclone/rclone/backend/ftp"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFTPProftpd:",
		NilObject:  (*ftp.Object)(nil),
	})
}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFTPRclone:",
		NilObject:  (*ftp.Object)(nil),
	})
}

func TestIntegration3(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFTPPureftpd:",
		NilObject:  (*ftp.Object)(nil),
	})
}

// func TestIntegration4(t *testing.T) {
// 	if *fstest.RemoteName != "" {
// 		t.Skip("skipping as -remote is set")
// 	}
// 	fstests.Run(t, &fstests.Opt{
// 		RemoteName: "TestFTPVsftpd:",
// 		NilObject:  (*ftp.Object)(nil),
// 	})
// }
