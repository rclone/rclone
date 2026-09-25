//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"encoding/base64"
	"testing"

	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration2 runs integration tests with directory markers enabled.
func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	name := "TestOracleObjectStorage"
	fstests.Run(t, &fstests.Opt{
		RemoteName:  name + ":",
		TiersToTest: []string{"standard", "archive"},
		NilObject:   (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "true"},
		},
	})
}

// testSSECustomerKey encrypts only transient integration-test resources.
const testSSECustomerKey = "0123456789abcdef0123456789abcdef"

// TestIntegration3 runs integration tests with SSE-C enabled.
func TestIntegration3(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	name := "TestOracleObjectStorage"
	fstests.Run(t, &fstests.Opt{
		RemoteName:  name + ":",
		TiersToTest: []string{"standard", "archive"},
		NilObject:   (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "false"},
			{Name: name, Key: "sse_kms_key_id", Value: ""},
			{Name: name, Key: "sse_customer_key_file", Value: ""},
			{Name: name, Key: "sse_customer_key", Value: base64.StdEncoding.EncodeToString([]byte(testSSECustomerKey))},
			{Name: name, Key: "sse_customer_key_sha256", Value: ""},
			{Name: name, Key: "sse_customer_algorithm", Value: "AES256"},
		},
	})
}
