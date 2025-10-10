package huaweidrive

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/encoder"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as no remote name")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:               *fstest.RemoteName,
		NilObject:                (*Object)(nil),
		SkipBadWindowsCharacters: true,
	})
}

// TestNewFs tests the NewFs constructor
func TestNewFs(t *testing.T) {
	ctx := context.Background()

	// Test with empty config - should fail due to missing OAuth credentials
	m := configmap.Simple{}
	_, err := NewFs(ctx, "test", "", m)
	if err == nil {
		t.Fatal("expected error with empty config")
	}
}

// TestFsName tests the filesystem name
func TestFsName(t *testing.T) {
	f := &Fs{name: "test"}
	if f.Name() != "test" {
		t.Errorf("expected name 'test', got %q", f.Name())
	}
}

// TestFsRoot tests the filesystem root
func TestFsRoot(t *testing.T) {
	f := &Fs{root: "test/path"}
	if f.Root() != "test/path" {
		t.Errorf("expected root 'test/path', got %q", f.Root())
	}
}

// TestFsString tests the filesystem string representation
func TestFsString(t *testing.T) {
	f := &Fs{root: "test/path"}
	expected := "Huawei Drive root 'test/path'"
	if f.String() != expected {
		t.Errorf("expected string %q, got %q", expected, f.String())
	}
}

// TestFsPrecision tests the filesystem precision
func TestFsPrecision(t *testing.T) {
	f := &Fs{}
	precision := f.Precision()
	expectedPrecision := fs.ModTimeNotSupported
	if precision != expectedPrecision {
		t.Errorf("expected precision %v (ModTimeNotSupported), got %v", expectedPrecision, precision)
	}
}

// TestFsHashes tests the supported hash types
func TestFsHashes(t *testing.T) {
	f := &Fs{}
	hashes := f.Hashes()
	if !hashes.Contains(hash.SHA256) {
		t.Error("expected SHA256 hash support")
	}
}

// TestModTimeSupport tests if the filesystem supports modification time preservation
func TestModTimeSupport(t *testing.T) {
	f := &Fs{}
	// Huawei Drive does NOT preserve original modification times
	// Despite the API accepting editedTime/createdTime parameters,
	// the server always overwrites them with current server timestamp
	expectedPrecision := fs.ModTimeNotSupported
	if f.Precision() != expectedPrecision {
		t.Errorf("expected precision of %v to indicate no time preservation, got %v", expectedPrecision, f.Precision())
	}
}

// TestTimeFormats tests time format handling for RFC 3339
func TestTimeFormats(t *testing.T) {
	// Test that we can format time correctly for Huawei Drive API
	testTime := time.Date(2023, 10, 15, 14, 30, 45, 0, time.UTC)
	formatted := testTime.Format(time.RFC3339)
	expected := "2023-10-15T14:30:45Z"

	if formatted != expected {
		t.Errorf("expected RFC3339 format %q, got %q", expected, formatted)
	}

	// Test parsing back
	parsed, err := time.Parse(time.RFC3339, formatted)
	if err != nil {
		t.Errorf("failed to parse RFC3339 time: %v", err)
	}

	if !parsed.Equal(testTime) {
		t.Errorf("parsed time %v does not match original %v", parsed, testTime)
	}
}

// TestEncoding tests filename encoding for Huawei Drive restrictions
func TestEncoding(t *testing.T) {
	// Create encoder with Huawei Drive restrictions
	enc := encoder.MultiEncoder( //nolint:unconvert
		encoder.Display |
			encoder.EncodeBackSlash |
			encoder.EncodeInvalidUtf8 |
			encoder.EncodeRightSpace |
			encoder.EncodeLeftSpace |
			encoder.EncodeLeftTilde |
			encoder.EncodeRightPeriod |
			encoder.EncodeLeftPeriod |
			encoder.EncodeColon |
			encoder.EncodePipe |
			encoder.EncodeDoubleQuote |
			encoder.EncodeLtGt |
			encoder.EncodeQuestion |
			encoder.EncodeAsterisk |
			encoder.EncodeCtl |
			encoder.EncodeDot)

	// Test cases for problematic characters that should be encoded
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"angle_brackets", "file<>name", "file＜＞name"},
		{"quotes", "file\"name", "file＂name"},
		{"pipe", "file|name", "file｜name"},
		{"colon", "file:name", "file：name"},
		{"asterisk", "file*name", "file＊name"},
		{"question", "file?name", "file？name"},
		{"backslash", "file\\name", "file＼name"},
		{"forward_slash", "file/name", "file／name"},
		{"leading_space", " filename", "␠filename"},
		{"trailing_space", "filename ", "filename␠"},
		{"leading_dot", ".filename", "．filename"},
		{"control_chars", "file\tname\ntest", "file␉name␊test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := enc.FromStandardName(tc.input)
			if encoded != tc.expected {
				t.Errorf("encoding %q: expected %q, got %q", tc.input, tc.expected, encoded)
			}

			// Test decoding back - only for reversible encodings
			// Some characters like forward slash and control chars are one-way encoded
			decoded := enc.ToStandardName(encoded)
			if tc.name != "forward_slash" && tc.name != "control_chars" {
				if decoded != tc.input {
					t.Errorf("decoding %q: expected %q, got %q", encoded, tc.input, decoded)
				}
			}
		})
	}
}

// TestFileNameEncoding tests that problematic characters are properly encoded
func TestFileNameEncoding(t *testing.T) {
	// Create a mock Fs with default encoding options
	opts := Options{}
	// Set the default encoding from our config
	opts.Enc = (encoder.Display |
		encoder.EncodeBackSlash |
		encoder.EncodeInvalidUtf8 |
		encoder.EncodeRightSpace |
		encoder.EncodeLeftSpace |
		encoder.EncodeLeftTilde |
		encoder.EncodeRightPeriod |
		encoder.EncodeLeftPeriod |
		encoder.EncodeColon |
		encoder.EncodePipe |
		encoder.EncodeDoubleQuote |
		encoder.EncodeLtGt |
		encoder.EncodeQuestion |
		encoder.EncodeAsterisk |
		encoder.EncodeCtl |
		encoder.EncodeDot)

	f := &Fs{opt: opts}

	// Test problematic characters that Huawei Drive rejects
	testCases := []struct {
		input string
		desc  string
	}{
		{`file<name>.txt`, "angle brackets"},
		{`file|name.txt`, "pipe character"},
		{`file:name.txt`, "colon"},
		{`file"name.txt`, "double quote"},
		{`file*name.txt`, "asterisk"},
		{`file?name.txt`, "question mark"},
		{`file\name.txt`, "backslash"},
		{` leading_space.txt`, "leading space"},
		{`trailing_space.txt `, "trailing space"},
		{`.leading_dot.txt`, "leading dot"},
		{`trailing_dot.txt.`, "trailing dot"},
		{`~leading_tilde.txt`, "leading tilde"},
		{"file\x00name.txt", "control character"},
	}

	for _, tc := range testCases {
		encoded := f.opt.Enc.FromStandardName(tc.input)
		// The encoded name should be different from input (meaning it was encoded)
		if encoded == tc.input {
			t.Errorf("Expected %s (%q) to be encoded, but got same string", tc.desc, tc.input)
		}

		// Test that we can decode it back (skip control characters as they are not reversible)
		decoded := f.opt.Enc.ToStandardName(encoded)
		if tc.desc != "control character" && decoded != tc.input {
			t.Errorf("Round-trip failed for %s: input=%q, encoded=%q, decoded=%q", tc.desc, tc.input, encoded, decoded)
		}

		t.Logf("✓ %s: %q → %q → %q", tc.desc, tc.input, encoded, decoded)
	}
}
