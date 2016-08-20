package crypt_test

import (
	"os"
	"path/filepath"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/fstests"
)

// Create the TestCrypt: remote
func init() {
	tempdir := filepath.Join(os.TempDir(), "rclone-crypt-test")
	name := "TestCrypt"
	fstests.ExtraConfig = []fstests.ExtraConfigItem{
		{Name: name, Key: "type", Value: "crypt"},
		{Name: name, Key: "remote", Value: tempdir},
		{Name: name, Key: "password", Value: fs.MustObscure("potato")},
	}
}
