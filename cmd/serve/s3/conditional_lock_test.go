package s3

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/gofakes3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalRejectedKeys(t *testing.T) {
	for _, key := range []string{"./object.txt", "dir/../object.txt", "dir//object.txt", "../root-secret.txt"} {
		t.Run(key, func(t *testing.T) {
			b, _ := newTestBackend(t)
			cb := newConditionalBackend(b)
			etag := objectETag(t, b, "bucket", "object.txt")
			ctx := conditionalContext("", "*")
			_, err := cb.PutObject(ctx, "bucket", key, map[string]string{}, strings.NewReader("replacement"), 11)
			require.Error(t, err)
			assert.Equal(t, etag, objectETag(t, b, "bucket", "object.txt"))
			assert.Empty(t, cb.locks.locks)
		})
	}
}

func TestConditionalReadLockCleanup(t *testing.T) {
	for _, method := range []string{"Get", "Head"} {
		t.Run(method, func(t *testing.T) {
			b, _ := newTestBackend(t)
			cb := newConditionalBackend(b)
			cb.xfs = true
			if method == "Get" {
				_, err := cb.GetObject(context.Background(), "bucket", "missing", nil)
				require.Error(t, err)
			} else {
				_, err := cb.HeadObject(context.Background(), "bucket", "missing")
				require.Error(t, err)
			}
			assert.Empty(t, cb.locks.locks)
		})
	}
}

func TestConditionalCopyLockOrder(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	cb.xfs = true
	ctx := context.Background()
	for _, key := range []string{"first", "second"} {
		_, err := cb.PutObject(ctx, "bucket", key, map[string]string{}, strings.NewReader(key), int64(len(key)))
		require.NoError(t, err)
	}
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, pair := range [][2]string{{"first", "second"}, {"second", "first"}} {
		go func() {
			<-start
			_, err := cb.CopyObject(ctx, "bucket", pair[0], "bucket", pair[1], map[string]string{})
			done <- err
		}()
	}
	close(start)
	for range 2 {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("opposite-direction copies deadlocked")
		}
	}
	assert.Equal(t, objectETag(t, b, "bucket", "first"), objectETag(t, b, "bucket", "second"))
	assert.Empty(t, cb.locks.locks)
}

func TestConditionalIndependentKeys(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	if cb.objectLockKey(context.Background(), "bucket", "other") == cb.objectLockKey(context.Background(), "bucket", "object.txt") {
		t.Skip("backend coordinates the entire namespace")
	}
	cb.xfs = true
	obj, err := cb.GetObject(context.Background(), "bucket", "object.txt", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = obj.Contents.Close() })
	done := make(chan error, 1)
	go func() {
		_, err := cb.PutObject(context.Background(), "bucket", "other", map[string]string{}, strings.NewReader("new"), 3)
		done <- err
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a read blocked a different object")
	}
	got, err := io.ReadAll(obj.Contents)
	require.NoError(t, err)
	assert.Equal(t, "normal object", string(got))
	require.NoError(t, obj.Contents.Close())
	assert.Empty(t, cb.locks.locks)
}

func TestConditionalCanceledWriter(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	cb.xfs = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	obj, err := cb.GetObject(ctx, "bucket", "object.txt", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = obj.Contents.Close() })
	done := make(chan error, 1)
	go func() {
		_, err := cb.PutObject(ctx, "bucket", "object.txt", nil, strings.NewReader("x"), 1)
		done <- err
	}()
	key := cb.objectLockKey(ctx, "bucket", "object.txt")
	require.Eventually(t, func() bool { cb.locks.mu.Lock(); defer cb.locks.mu.Unlock(); return cb.locks.locks[key].refs == 2 }, 5*time.Second, time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled writer still waiting for GET")
	}
	require.NoError(t, obj.Contents.Close())
	require.Empty(t, cb.locks.locks)
}

func TestConditionalCaseInsensitiveAliases(t *testing.T) {
	b, root := newTestBackend(t)
	original, err := os.Stat(filepath.Join(root, "bucket", "object.txt"))
	require.NoError(t, err)
	alias, err := os.Stat(filepath.Join(root, "bucket", "OBJECT.TXT"))
	if os.IsNotExist(err) {
		t.Skip("requires a case-insensitive filesystem")
	}
	require.NoError(t, err)
	require.True(t, os.SameFile(original, alias))
	b.s.provider.VFS().Opt.CaseInsensitive = true
	cb := newConditionalBackend(b)
	ctx := conditionalContext(objectETag(t, b, "bucket", "object.txt"), "")
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	first, second := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, reader, 2)
		first <- err
	}()
	// The first byte is consumed after the first writer has checked its ETag.
	_, err = writer.Write([]byte("a"))
	require.NoError(t, err)
	go func() {
		_, err := cb.PutObject(ctx, "bucket", "OBJECT.TXT", map[string]string{}, strings.NewReader("b"), 1)
		second <- err
	}()
	key := cb.objectLockKey(ctx, "bucket", "object.txt")
	require.Eventually(t, func() bool {
		cb.locks.mu.Lock()
		defer cb.locks.mu.Unlock()
		lock := cb.locks.locks[key]
		return lock != nil && lock.refs == 2
	}, 5*time.Second, time.Millisecond)
	_, err = writer.Write([]byte("a"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, <-first)
	require.ErrorIs(t, <-second, gofakes3.ErrPreconditionFailed)
}

func TestConditionalCompletionWaitKeepsUpload(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id, err := cb.CreateMultipartUpload(ctx, "bucket", "object.txt", nil)
	require.NoError(t, err)
	defer func() { _ = cb.AbortMultipartUpload(context.Background(), "bucket", "object.txt", id) }()
	unlock, err := cb.locks.lock(ctx, cb.objectLockKey(ctx, "bucket", "object.txt"), writeWeight)
	require.NoError(t, err)
	defer unlock()
	done := make(chan error, 1)
	go func() { _, _, err := cb.CompleteMultipartUpload(ctx, "bucket", "object.txt", id, nil); done <- err }()
	up, err := cb.loadUpload(ctx, id)
	require.NoError(t, err)
	require.Eventually(t, func() bool { up.mu.Lock(); defer up.mu.Unlock(); return up.active == 1 }, 5*time.Second, time.Millisecond)
	b.reapExpiredUploads(time.Now().Add(2*time.Hour), time.Hour)
	_, err = cb.loadUpload(ctx, id)
	require.NoError(t, err)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	up.mu.Lock()
	defer up.mu.Unlock()
	require.Zero(t, up.active)
}
