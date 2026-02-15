// Package raid3_test: tests for server-side operations (Copy, Move, DirMove).
//
// These tests verify that server-side operations succeed within the same backend
// (same config name) and are rejected with fs.ErrorCant* when called across
// different backends (different config names).
package raid3_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/raid3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerSideSameBackendSuccess verifies that server-side Move, Copy (if supported),
// and DirMove succeed when source and destination are the same raid3 backend.
//
// Runs with local temp dirs when -remote is not set (tests local-backed raid3).
// Runs with the configured remote when -remote is set (e.g. -remote minioraid3: for MinIO).
// With local backends, Copy is not supported (local does not implement Copier); Move and DirMove are.
func TestServerSideSameBackendSuccess(t *testing.T) {
	ctx := context.Background()

	var f fs.Fs
	var err error

	if *fstest.RemoteName != "" {
		// Use configured remote (e.g. minioraid3:) for MinIO-backed raid3
		f, err = fs.NewFs(ctx, *fstest.RemoteName)
		require.NoError(t, err)
	} else {
		// Local-backed raid3
		evenDir := t.TempDir()
		oddDir := t.TempDir()
		parityDir := t.TempDir()
		m := configmap.Simple{"even": evenDir, "odd": oddDir, "parity": parityDir}
		f, err = raid3.NewFs(ctx, "TestServerSideLocal", "", m)
		require.NoError(t, err)
	}

	// Require at least Move (local and S3 support it)
	require.NotNil(t, f.Features().Move, "backend must support server-side Move for this test")

	// --- Move within same backend ---
	putRemote := "server-side-test-move-src.txt"
	data := []byte("server-side move test")
	info := object.NewStaticObjectInfo(putRemote, time.Now(), int64(len(data)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	obj, err := f.NewObject(ctx, putRemote)
	require.NoError(t, err)

	moveDest := "server-side-test-move-dst.txt"
	moved, err := operations.Move(ctx, f, nil, moveDest, obj)
	require.NoError(t, err)
	require.NotNil(t, moved)
	assert.Equal(t, moveDest, moved.Remote())

	_, err = f.NewObject(ctx, moveDest)
	require.NoError(t, err)
	_, err = f.NewObject(ctx, putRemote)
	assert.True(t, err != nil, "source should be gone after move")

	// --- Copy within same backend (skip if backend does not support Copy, e.g. local) ---
	if f.Features().Copy != nil {
		copySrc := "server-side-test-copy-src.txt"
		copyData := []byte("server-side copy test")
		copyInfo := object.NewStaticObjectInfo(copySrc, time.Now(), int64(len(copyData)), true, nil, nil)
		_, err = f.Put(ctx, bytes.NewReader(copyData), copyInfo)
		require.NoError(t, err)

		copyObj, err := f.NewObject(ctx, copySrc)
		require.NoError(t, err)

		copyDest := "server-side-test-copy-dst.txt"
		copied, err := operations.Copy(ctx, f, nil, copyDest, copyObj)
		require.NoError(t, err)
		require.NotNil(t, copied)
		assert.Equal(t, copyDest, copied.Remote())

		// Both source and dest should exist
		_, err = f.NewObject(ctx, copySrc)
		require.NoError(t, err)
		_, err = f.NewObject(ctx, copyDest)
		require.NoError(t, err)
	}

	// --- DirMove within same backend ---
	if f.Features().DirMove != nil {
		dirName := "server-side-dirmove-dir"
		err = f.Mkdir(ctx, dirName)
		require.NoError(t, err)

		fileInDir := dirName + "/file.txt"
		fileData := []byte("inside dir")
		fileInfo := object.NewStaticObjectInfo(fileInDir, time.Now(), int64(len(fileData)), true, nil, nil)
		_, err = f.Put(ctx, bytes.NewReader(fileData), fileInfo)
		require.NoError(t, err)

		dirMoved := "server-side-dirmove-dir-renamed"
		err = operations.DirMove(ctx, f, dirName, dirMoved)
		require.NoError(t, err)

		// Old dir should be gone, new dir should have the file
		_, err = f.NewObject(ctx, fileInDir)
		assert.True(t, err != nil)
		renamedPath := dirMoved + "/file.txt"
		got, err := f.NewObject(ctx, renamedPath)
		require.NoError(t, err)
		rc, err := got.Open(ctx)
		require.NoError(t, err)
		gotData, err := io.ReadAll(rc)
		_ = rc.Close()
		require.NoError(t, err)
		assert.Equal(t, fileData, gotData)
	}
}

// TestServerSideCrossBackendRejected verifies that server-side Move, Copy, and DirMove
// return fs.ErrorCantMove, fs.ErrorCantCopy, fs.ErrorCantDirMove when source and
// destination are different raid3 backends (different config names).
//
// This test uses local-backed raid3 only (two Fs with different names).
func TestServerSideCrossBackendRejected(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set (cross-backend test uses local-only setup)")
	}

	ctx := context.Background()

	// Two separate raid3 backends (different names, different temp dirs)
	evenA, oddA, parityA := t.TempDir(), t.TempDir(), t.TempDir()
	evenB, oddB, parityB := t.TempDir(), t.TempDir(), t.TempDir()

	mA := configmap.Simple{"even": evenA, "odd": oddA, "parity": parityA}
	mB := configmap.Simple{"even": evenB, "odd": oddB, "parity": parityB}

	fA, err := raid3.NewFs(ctx, "raid3a", "", mA)
	require.NoError(t, err)
	fB, err := raid3.NewFs(ctx, "raid3b", "", mB)
	require.NoError(t, err)

	// Create file on backend A
	remoteA := "cross-backend-file.txt"
	data := []byte("data on A")
	info := object.NewStaticObjectInfo(remoteA, time.Now(), int64(len(data)), true, nil, nil)
	_, err = fA.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	objA, err := fA.NewObject(ctx, remoteA)
	require.NoError(t, err)

	// --- Move: fB's Move feature with objA must return ErrorCantMove ---
	doMoveB := fB.Features().Move
	require.NotNil(t, doMoveB, "backend B must support Move for this test")
	_, err = doMoveB(ctx, objA, "moved-on-b.txt")
	assert.True(t, errors.Is(err, fs.ErrorCantMove), "expected ErrorCantMove for cross-backend move, got: %v", err)

	// Re-get objA (Move may have been attempted and we need a valid object for Copy)
	objA, err = fA.NewObject(ctx, remoteA)
	require.NoError(t, err)

	// --- Copy: fB's Copy feature with objA must return ErrorCantCopy (if B supports Copy) ---
	if doCopyB := fB.Features().Copy; doCopyB != nil {
		_, err = doCopyB(ctx, objA, "copied-on-b.txt")
		assert.True(t, errors.Is(err, fs.ErrorCantCopy), "expected ErrorCantCopy for cross-backend copy, got: %v", err)
	}

	// --- DirMove: fB's DirMove with fA must return ErrorCantDirMove ---
	err = fA.Mkdir(ctx, "dir-on-a")
	require.NoError(t, err)
	doDirMoveB := fB.Features().DirMove
	require.NotNil(t, doDirMoveB, "backend B must support DirMove for this test")
	err = doDirMoveB(ctx, fA, "dir-on-a", "dir-on-b")
	assert.True(t, errors.Is(err, fs.ErrorCantDirMove), "expected ErrorCantDirMove for cross-backend dirmove, got: %v", err)
}
