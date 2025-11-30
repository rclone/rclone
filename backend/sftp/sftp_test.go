// Test Sftp filesystem interface

//go:build !plan9

package sftp_test

import (
	"testing"

	"github.com/rclone/rclone/backend/sftp"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/encoder"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSFTPOpenssh:",
		NilObject:  (*sftp.Object)(nil),
	})
}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSFTPRclone:",
		NilObject:  (*sftp.Object)(nil),
	})
}

func TestIntegration3(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSFTPRcloneSSH:",
		NilObject:  (*sftp.Object)(nil),
	})
}

func TestEncoding(t *testing.T) {
	// Test that encoding is properly applied to paths
	testCases := []struct {
		name        string
		encoding    string
		input       string
		wantEncoded string
	}{
		{
			name:        "Win encoding with colon",
			encoding:    "Win",
			input:       "test:file.txt",
			wantEncoded: "test：file.txt",
		},
		{
			name:        "Win encoding with multiple special chars",
			encoding:    "Win",
			input:       "file:name?.txt",
			wantEncoded: "file：name？.txt",
		},
		{
			name:        "Win encoding with trailing space",
			encoding:    "Win",
			input:       "trailing space ",
			wantEncoded: "trailing space␠",
		},
		{
			name:        "Win encoding with trailing period",
			encoding:    "Win",
			input:       "trailing.",
			wantEncoded: "trailing．",
		},
		{
			name:        "No encoding",
			encoding:    "None",
			input:       "test:file.txt",
			wantEncoded: "test:file.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create encoder
			var enc encoder.MultiEncoder
			if tc.encoding != "" && tc.encoding != "None" {
				err := enc.Set(tc.encoding)
				if err != nil {
					t.Fatalf("Failed to set encoding: %v", err)
				}
			}

			// Test encoding
			encoded := enc.FromStandardName(tc.input)
			if encoded != tc.wantEncoded {
				t.Errorf("Encode(%q) = %q, want %q", tc.input, encoded, tc.wantEncoded)
			}

			// Test decoding
			decoded := enc.ToStandardName(encoded)
			if decoded != tc.input {
				t.Errorf("Decode(%q) = %q, want %q", encoded, decoded, tc.input)
			}
		})
	}
}
