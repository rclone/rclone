// Test Crypt filesystem interface
package crypt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ncw/rclone/backend/crypt"
	_ "github.com/ncw/rclone/backend/local"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestStandard runs integration tests against the remote
func TestStandard(t *testing.T) {
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
	})
}

// TestOff runs integration tests against the remote
func TestOff(t *testing.T) {
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
	})
}

// TestObfuscate runs integration tests against the remote
func TestObfuscate(t *testing.T) {
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
		SkipBadWindowsCharacters: true,
	})
}
