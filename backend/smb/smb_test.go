// Test smb filesystem interface
package smb

import (
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSMB:rclone",
		NilObject:  (*Object)(nil),
	})
}

func TestIntegration2(t *testing.T) {
	krb5Dir := t.TempDir()
	t.Setenv("KRB5_CONFIG", filepath.Join(krb5Dir, "krb5.conf"))
	t.Setenv("KRB5CCNAME", filepath.Join(krb5Dir, "ccache"))
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSMBKerberos:rclone",
		NilObject:  (*Object)(nil),
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

func (f *Fs) SetUploadCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadCutoff(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)
