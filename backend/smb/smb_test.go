// Test smb filesystem interface
package smb_test

import (
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/backend/smb"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSMB:rclone",
		NilObject:  (*smb.Object)(nil),
	})
}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	krb5Dir := t.TempDir()
	t.Setenv("KRB5_CONFIG", filepath.Join(krb5Dir, "krb5.conf"))
	t.Setenv("KRB5CCNAME", filepath.Join(krb5Dir, "ccache"))
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSMBKerberos:rclone",
		NilObject:  (*smb.Object)(nil),
	})
}

func TestIntegration3(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}

	krb5Dir := t.TempDir()
	t.Setenv("KRB5_CONFIG", filepath.Join(krb5Dir, "krb5.conf"))
	ccache := filepath.Join(krb5Dir, "ccache")
	t.Setenv("RCLONE_TEST_CUSTOM_CCACHE_LOCATION", ccache)

	name := "TestSMBKerberosCcache"

	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":rclone",
		NilObject:  (*smb.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "kerberos_ccache", Value: ccache},
		},
	})
}
