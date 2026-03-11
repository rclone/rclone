// Package resume manages checkpoint state files for resumable listings.
//
// When --resume-listings is set to a local directory path, rclone
// saves listing progress so that interrupted listings can be resumed
// from where they left off rather than starting from scratch.
package resume

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rclone/rclone/fs"
)

const checkpointVersion = 1

// Checkpoint represents the saved state of a listing in progress.
type Checkpoint struct {
	Version    int       `json:"version"`
	RemoteName string    `json:"remoteName"` // e.g. "s3:mybucket/path"
	Dir        string    `json:"dir"`        // directory being listed
	LastKey    string    `json:"lastKey"`    // last key delivered to callback
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Store manages checkpoint files in a local directory.
type Store struct {
	dir string
}

// NewStore creates a Store that persists checkpoints in dir.
// The directory is created if it does not exist.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return nil, fmt.Errorf("resume: create store directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// stateKey returns a deterministic file path for the given remote+dir.
func (s *Store) stateKey(remoteName, dir string) string {
	h := sha256.Sum256([]byte(remoteName + ":" + dir))
	return filepath.Join(s.dir, fmt.Sprintf("%x.json", h))
}

// Load reads a checkpoint for the given remote and directory.
// Returns nil, nil if no checkpoint exists or the checkpoint doesn't
// match (wrong version, different remote/dir).
func (s *Store) Load(remoteName, dir string) (*Checkpoint, error) {
	path := s.stateKey(remoteName, dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("resume: read checkpoint: %w", err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("resume: unmarshal checkpoint: %w", err)
	}

	if cp.Version != checkpointVersion {
		return nil, nil
	}
	if cp.RemoteName != remoteName || cp.Dir != dir {
		return nil, nil
	}

	return &cp, nil
}

// Save atomically writes a checkpoint to disk.
func (s *Store) Save(cp *Checkpoint) error {
	cp.Version = checkpointVersion
	cp.UpdatedAt = time.Now().UTC()

	payload, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("resume: marshal checkpoint: %w", err)
	}

	path := s.stateKey(cp.RemoteName, cp.Dir)

	// Atomic write: temp file + rename
	tmpFile, err := os.CreateTemp(s.dir, ".rclone-resume-*.tmp")
	if err != nil {
		return fmt.Errorf("resume: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, writeErr := tmpFile.Write(payload)
	closeErr := tmpFile.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("resume: write temp file: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("resume: close temp file: %w", closeErr)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("resume: rename temp file: %w", err)
	}

	return nil
}

// WrapCallback loads any existing checkpoint for this remote+dir and returns:
//   - startAfter: last key from a previous interrupted listing ("" if none)
//   - wrappedCallback: tracks lastKey per page, saves checkpoint after each page
//   - done: defer this — it deletes the checkpoint only if *errp == nil
func (s *Store) WrapCallback(remoteName, dir string, callback fs.ListRCallback) (
	startAfter string, wrappedCallback fs.ListRCallback, done func(errp *error), err error) {
	cp, err := s.Load(remoteName, dir)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resume: load checkpoint: %w", err)
	}
	if cp != nil {
		startAfter = cp.LastKey
		fs.Infof(nil, "Resuming listing of %q from checkpoint %q", dir, startAfter)
	}

	var lastKey string
	wrappedCallback = func(entries fs.DirEntries) error {
		// Track the max key in this page for checkpointing
		for _, entry := range entries {
			key := entry.Remote()
			if key > lastKey {
				lastKey = key
			}
		}

		// Pass entries through to the original callback
		if err := callback(entries); err != nil {
			return err
		}

		// Save checkpoint after each page
		if lastKey != "" {
			saveCP := &Checkpoint{
				RemoteName: remoteName,
				Dir:        dir,
				LastKey:    lastKey,
			}
			if err := s.Save(saveCP); err != nil {
				fs.Errorf(nil, "Failed to save listing checkpoint: %v", err)
			}
		}
		return nil
	}

	done = func(errp *error) {
		if *errp != nil {
			return
		}
		if err := s.Delete(remoteName, dir); err != nil {
			fs.Errorf(nil, "Failed to delete listing checkpoint: %v", err)
		}
	}

	return startAfter, wrappedCallback, done, nil
}

// Setup is the top-level entry point for backends to add resume support.
//
// If --resume-listings is not configured or enabled is false, it returns
// passthrough values: startAfter="", the original callback, and a no-op done.
//
// Otherwise it creates a Store, loads any existing checkpoint, and wraps the
// callback to save progress after each page. done should be deferred — it
// deletes the checkpoint only when *errp == nil.
//
// Typical usage in a backend ListP/ListR:
//
//	func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) (returnErr error) {
//	    bucket, directory := f.split(dir)
//	    startAfter, callback, done, err := resume.Setup(ctx, bucket != "", f, dir, callback)
//	    if err != nil { return err }
//	    defer done(&returnErr)
//	    ...
//	}
func Setup(ctx context.Context, enabled bool, f fs.Fs, dir string, callback fs.ListRCallback) (
	startAfter string, wrappedCallback fs.ListRCallback, done func(errp *error), err error) {
	ci := fs.GetConfig(ctx)
	if !enabled || ci.ResumeListings == "" {
		return "", callback, func(*error) {}, nil
	}
	store, err := NewStore(ci.ResumeListings)
	if err != nil {
		return "", nil, nil, err
	}
	return store.WrapCallback(fs.ConfigString(f), dir, callback)
}

// Delete removes the checkpoint for the given remote and directory.
// It is not an error if the checkpoint does not exist.
func (s *Store) Delete(remoteName, dir string) error {
	path := s.stateKey(remoteName, dir)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("resume: delete checkpoint: %w", err)
	}
	return nil
}
