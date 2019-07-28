// Test Crypt filesystem interface
package crypt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/backend/crypt"
	_ "github.com/rclone/rclone/backend/drive" // for integration tests
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/swift" // for integration tests
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:                   *fstest.RemoteName,
		NilObject:                    (*crypt.Object)(nil),
		UnimplementableFsMethods:     []string{"OpenWriterAt"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

// TestStandard runs integration tests against the remote
func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-crypt-test-standard")
	name := "TestCrypt"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*crypt.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "crypt"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "password", Value: obscure.MustObscure("potato")},
			{Name: name, Key: "filename_encryption", Value: "standard"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

// TestOff runs integration tests against the remote
func TestOff(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-crypt-test-off")
	name := "TestCrypt2"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*crypt.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "crypt"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "password", Value: obscure.MustObscure("potato2")},
			{Name: name, Key: "filename_encryption", Value: "off"},
		},
		UnimplementableFsMethods:     []string{"OpenWriterAt"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}

// TestObfuscate runs integration tests against the remote
func TestObfuscate(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-crypt-test-obfuscate")
	name := "TestCrypt3"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*crypt.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "crypt"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "password", Value: obscure.MustObscure("potato2")},
			{Name: name, Key: "filename_encryption", Value: "obfuscate"},
		},
		SkipBadWindowsCharacters:     true,
		UnimplementableFsMethods:     []string{"OpenWriterAt"},
		UnimplementableObjectMethods: []string{"MimeType"},
	})
}
