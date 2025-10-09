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
