package walk

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/stretchr/testify/assert"
)

// Test that walk stops listing when the context is cancelled.
func TestWalkContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{}, 1)
	listDir := func(ctx context.Context, f fs.Fs, includeAll bool, dir string) (entries fs.DirEntries, err error) {
		select {
		case started <- struct{}{}:
		default:
		}
		// Every directory contains one subdirectory so the walk never
		// finishes on its own.
		return fs.DirEntries{mockdir.New("sub")}, nil
	}

	done := make(chan error, 1)
	go func() {
		done <- walk(ctx, nil, "", false, -1, func(path string, entries fs.DirEntries, err error) error {
			return nil
		}, listDir)
	}()

	// Wait for the walk to start.
	<-started

	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("walk did not stop after the context was cancelled")
	}
}
