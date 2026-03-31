//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListingCheckpointSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	checkpointPath := filepath.Join(dir, "checkpoint.json")

	state := listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "bucket-a",
		Prefix:    "prefix/path/",
		Delimiter: "",
		Marker:    "obj-1000",
	}

	if err := saveListingCheckpoint(checkpointPath, state); err != nil {
		t.Fatalf("saveListingCheckpoint() error = %v", err)
	}

	marker, found, err := loadListingCheckpoint(checkpointPath, "bucket-a", "prefix/path/", "")
	if err != nil {
		t.Fatalf("loadListingCheckpoint() error = %v", err)
	}
	if !found {
		t.Fatalf("loadListingCheckpoint() found = false, want true")
	}
	if marker != "obj-1000" {
		t.Fatalf("loadListingCheckpoint() marker = %q, want %q", marker, "obj-1000")
	}
}

func TestListingCheckpointLoadNotFound(t *testing.T) {
	marker, found, err := loadListingCheckpoint(filepath.Join(t.TempDir(), "missing.json"), "bucket-a", "", "")
	if err != nil {
		t.Fatalf("loadListingCheckpoint() error = %v", err)
	}
	if found {
		t.Fatalf("loadListingCheckpoint() found = true, want false")
	}
	if marker != "" {
		t.Fatalf("loadListingCheckpoint() marker = %q, want empty", marker)
	}
}

func TestListingCheckpointLoadReadFailure(t *testing.T) {
	checkpointPath := t.TempDir()

	marker, found, err := loadListingCheckpoint(checkpointPath, "bucket-a", "", "")
	if err == nil {
		t.Fatal("loadListingCheckpoint() error = nil, want structured error")
	}
	if found {
		t.Fatalf("loadListingCheckpoint() found = true, want false")
	}
	if marker != "" {
		t.Fatalf("loadListingCheckpoint() marker = %q, want empty", marker)
	}
	if !strings.Contains(err.Error(), `"code":"checkpoint_read_failed"`) {
		t.Fatalf("loadListingCheckpoint() error = %q, want checkpoint_read_failed", err)
	}
}

func TestListingCheckpointLoadScopeMismatch(t *testing.T) {
	dir := t.TempDir()
	checkpointPath := filepath.Join(dir, "checkpoint.json")

	state := listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "bucket-a",
		Prefix:    "prefix/path/",
		Delimiter: "/",
		Marker:    "obj-1000",
	}
	if err := saveListingCheckpoint(checkpointPath, state); err != nil {
		t.Fatalf("saveListingCheckpoint() error = %v", err)
	}

	marker, found, err := loadListingCheckpoint(checkpointPath, "bucket-a", "different/", "/")
	if err == nil {
		t.Fatal("loadListingCheckpoint() error = nil, want structured error")
	}
	if found {
		t.Fatalf("loadListingCheckpoint() found = true, want false")
	}
	if marker != "" {
		t.Fatalf("loadListingCheckpoint() marker = %q, want empty", marker)
	}
	if !strings.Contains(err.Error(), `"code":"checkpoint_scope_mismatch"`) {
		t.Fatalf("loadListingCheckpoint() error = %q, want checkpoint_scope_mismatch", err)
	}
}

func TestListingCheckpointLoadVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	checkpointPath := filepath.Join(dir, "checkpoint.json")

	raw := listingCheckpoint{
		Version:   listingCheckpointVersion + 1,
		Bucket:    "bucket-a",
		Prefix:    "prefix/path/",
		Delimiter: "",
		Marker:    "obj-1000",
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(checkpointPath, payload, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	marker, found, err := loadListingCheckpoint(checkpointPath, "bucket-a", "prefix/path/", "")
	if err == nil {
		// expected
	} else {
		t.Fatalf("loadListingCheckpoint() error = %v", err)
	}
	if !found {
		t.Fatalf("loadListingCheckpoint() found = false, want true")
	}
	if marker != "obj-1000" {
		t.Fatalf("loadListingCheckpoint() marker = %q, want %q", marker, "obj-1000")
	}
}

func TestListingCheckpointLoadCorruptJSON(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(checkpointPath, []byte(`{"version":`), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	marker, found, err := loadListingCheckpoint(checkpointPath, "bucket-a", "", "")
	if err == nil {
		t.Fatal("loadListingCheckpoint() error = nil, want structured error")
	}
	if found {
		t.Fatalf("loadListingCheckpoint() found = true, want false")
	}
	if marker != "" {
		t.Fatalf("loadListingCheckpoint() marker = %q, want empty", marker)
	}
	if !strings.Contains(err.Error(), `"code":"checkpoint_corrupt"`) {
		t.Fatalf("loadListingCheckpoint() error = %q, want checkpoint_corrupt", err)
	}
}

func TestListingCheckpointSaveFailure(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	err := saveListingCheckpoint(filepath.Join(parentFile, "checkpoint.json"), listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    "bucket-a",
		Prefix:    "",
		Delimiter: "",
		Marker:    "obj-1000",
	})
	if err == nil {
		t.Fatal("saveListingCheckpoint() error = nil, want structured error")
	}
	if !strings.Contains(err.Error(), `"code":"checkpoint_save_failed"`) {
		t.Fatalf("saveListingCheckpoint() error = %q, want checkpoint_save_failed", err)
	}
}
