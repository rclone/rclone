package huaweidrive

import (
	"context"
	"testing"

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
	if precision.Seconds() != 1 {
		t.Errorf("expected precision 1 second, got %v", precision)
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
