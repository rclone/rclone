// Package adb_test runs the standard rclone integration suite against
// the adb backend. It also includes a unit test for the df output parser
// extracted from Fs.About.
//
// Integration tests require a TestADB remote configured in rclone.conf
// and a connected ADB device. They skip cleanly without configuration.
package adb_test

import (
	"testing"

	"github.com/rclone/rclone/backend/adb"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs the standard rclone fstests integration suite
// against the TestADB: remote. Without configuration, fstests.Run
// skips on its own.
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestADB:",
		NilObject:  (*adb.Object)(nil),
	})
}

// TestParseDfOutput tests the parseDfOutput helper with real df -k output
// captured from three Android devices spanning the scoped-storage boundary:
//
//   - Android 8 (API 26): /data/media backing filesystem, legacy mount
//   - Android 10 (API 29): /data/media backing filesystem
//   - Android 16 (API 36): /dev/fuse backing filesystem, scoped storage
//
// Output captured via:
//
//	adb shell df -k /sdcard
func TestParseDfOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTotal int64
		wantUsed  int64
		wantFree  int64
		wantErr   bool
	}{
		{
			// Android 8 (API 26) - /data/media backing
			// Filesystem     1K-blocks    Used Available Use% Mounted on
			// /data/media     24837032 7480516  17356516  31% /storage/emulated
			name:      "data_media_api26",
			input:     "Filesystem     1K-blocks    Used Available Use% Mounted on\n/data/media     24837032 7480516  17356516  31% /storage/emulated\n",
			wantTotal: 24837032 * 1024,
			wantUsed:  7480516 * 1024,
			wantFree:  17356516 * 1024,
		},
		{
			// Android 16 (API 36) - /dev/fuse backing, scoped storage
			// Filesystem     1K-blocks     Used Available Use% Mounted on
			// /dev/fuse      114582612 44629660  69821880  39% /storage/emulated
			name:      "dev_fuse_api36",
			input:     "Filesystem     1K-blocks     Used Available Use% Mounted on\n/dev/fuse      114582612 44629660  69821880  39% /storage/emulated\n",
			wantTotal: 114582612 * 1024,
			wantUsed:  44629660 * 1024,
			wantFree:  69821880 * 1024,
		},
		{
			// Malformed: only one field on data line - parser must return error
			name:    "malformed_too_few_fields",
			input:   "Filesystem     1K-blocks    Used Available Use% Mounted on\n/data/media\n",
			wantErr: true,
		},
		{
			// Malformed: non-numeric blocks field - ParseInt must return error
			name:    "malformed_non_numeric_blocks",
			input:   "Filesystem     1K-blocks    Used Available Use% Mounted on\n/data/media     BADVAL 7480516  17356516  31% /storage/emulated\n",
			wantErr: true,
		},
		{
			// Android 10 (API 29) - /data/media backing
			// Filesystem     1K-blocks    Used Available Use% Mounted on
			// /data/media     21997548 9001420  12953656  41% /storage/emulated
			name:      "data_media_api29",
			input:     "Filesystem     1K-blocks    Used Available Use% Mounted on\n/data/media     21997548 9001420  12953656  41% /storage/emulated\n",
			wantTotal: 21997548 * 1024,
			wantUsed:  9001420 * 1024,
			wantFree:  12953656 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, used, free, err := adb.ParseDfOutput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDfOutput(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDfOutput(%q) error: %v", tt.input, err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
			if used != tt.wantUsed {
				t.Errorf("used = %d, want %d", used, tt.wantUsed)
			}
			if free != tt.wantFree {
				t.Errorf("free = %d, want %d", free, tt.wantFree)
			}
		})
	}
}

// TestParseExitCodeTrailer covers the helper that parses the trailing
// ":N" emitted by the shell wrapper used in execTwoPathCmd and
// execCommandWithExitCode. The wrapper appends "; echo :$?" so the
// shell exit code is recoverable even on the ADB shell: service which
// does not transmit exit codes natively.
func TestParseExitCodeTrailer(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStdout string
		wantCode   int
		wantErr    bool
		wantErrSub string
	}{
		{
			name:       "success_no_stdout",
			input:      ":0",
			wantStdout: "",
			wantCode:   0,
		},
		{
			name:       "success_with_stdout",
			input:      "hello world\n:0",
			wantStdout: "hello world\n",
			wantCode:   0,
		},
		{
			name:       "real_world_mv_success",
			input:      ":0",
			wantStdout: "",
			wantCode:   0,
		},
		{
			name:       "exit_code_1_no_such_file",
			input:      "rm: 'foo': No such file or directory\n:1",
			wantStdout: "rm: 'foo': No such file or directory\n",
			wantCode:   1,
			wantErr:    true,
			wantErrSub: "exit code 1",
		},
		{
			name:       "exit_code_127_command_not_found",
			input:      "/system/bin/sh: nonexistent: not found\n:127",
			wantStdout: "/system/bin/sh: nonexistent: not found\n",
			wantCode:   127,
			wantErr:    true,
			wantErrSub: "exit code 127",
		},
		{
			name:       "trailer_with_whitespace",
			input:      ":  42 ",
			wantStdout: "",
			wantCode:   42,
			wantErr:    true,
		},
		{
			name:       "missing_trailer",
			input:      "no colon here",
			wantStdout: "no colon here",
			wantCode:   -1,
			wantErr:    true,
			wantErrSub: "cannot parse exit code",
		},
		{
			name:       "non_numeric_trailer",
			input:      "output\n:abc",
			wantStdout: "output\n:abc",
			wantCode:   -1,
			wantErr:    true,
			wantErrSub: "trailer parse",
		},
		{
			name:       "empty_trailer_value",
			input:      "output\n:",
			wantStdout: "output\n:",
			wantCode:   -1,
			wantErr:    true,
		},
		{
			name:       "embedded_colon_in_stdout",
			input:      "key: value\nanother: line\n:0",
			wantStdout: "key: value\nanother: line\n",
			wantCode:   0,
		},
		{
			name:       "stdout_contains_colon_then_negative_code_treated_as_error",
			input:      "::-1",
			wantStdout: ":",
			wantCode:   -1,
			wantErr:    true,
			wantErrSub: "exit code -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, code, err := adb.ParseExitCodeTrailer(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseExitCodeTrailer(%q) = nil error, want error", tt.input)
				}
				if tt.wantErrSub != "" && !contains(err.Error(), tt.wantErrSub) {
					t.Errorf("err = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
			} else if err != nil {
				t.Fatalf("ParseExitCodeTrailer(%q) error: %v", tt.input, err)
			}

			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if code != tt.wantCode {
				t.Errorf("code = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// contains is a tiny helper to keep the test file dependency-free.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
