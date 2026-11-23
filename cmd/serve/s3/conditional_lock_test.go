package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestConditionalCopyLockOrder(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)

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
	ctx, cancel := context.WithTimeout(conditionalContext(objectETag(t, b, "bucket", "object.txt"), ""), 5*time.Second)
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	first, second := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, reader, 2)
		first <- err
	}()
	// The first upload is staged while the second publishes.
	_, err = writer.Write([]byte("a"))
	require.NoError(t, err)
	go func() {
		_, err := cb.PutObject(ctx, "bucket", "OBJECT.TXT", map[string]string{}, strings.NewReader("b"), 1)
		second <- err
	}()
	require.NoError(t, <-second)
	_, err = writer.Write([]byte("a"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.ErrorIs(t, <-first, gofakes3.ErrPreconditionFailed)
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

func TestStagedPutAllowsOtherWrites(t *testing.T) {
	b, _ := newTestBackend(t)
	features := b.s.provider.VFS().Fs().Features()
	previous := features.CaseInsensitive
	features.CaseInsensitive = true
	t.Cleanup(func() { features.CaseInsensitive = previous })
	cb := newConditionalBackend(b)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, cb.CreateBucket(ctx, "another"))
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	done := make(chan error, 1)
	go func() { _, err := cb.PutObject(ctx, "bucket", "slow", nil, reader, 2); done <- err }()
	_, err := writer.Write([]byte("a"))
	require.NoError(t, err)
	for _, bucket := range []string{"bucket", "another"} {
		_, err := cb.PutObject(ctx, bucket, "fast", nil, strings.NewReader("b"), 1)
		require.NoError(t, err)
	}
	_, err = writer.Write([]byte("b"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, <-done)
}

// pauseCopyBackend lets a writer queue after gofakes3 reads the source metadata.
type pauseCopyBackend struct {
	*conditionalBackend
	head   chan struct{}
	resume chan struct{}
}

func (b *pauseCopyBackend) HeadObject(ctx context.Context, bucket, key string) (*gofakes3.Object, error) {
	obj, err := b.conditionalBackend.HeadObject(ctx, bucket, key)
	close(b.head)
	<-b.resume
	return obj, err
}

func TestCopyUsesValidatedSource(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		t.Run(map[bool]string{false: "Path", true: "VirtualHost"}[virtual], func(t *testing.T) {
			b, root := newTestBackend(t)
			cb := newConditionalBackend(b)
			paused := &pauseCopyBackend{cb, make(chan struct{}), make(chan struct{})}
			handler := withConditions(gofakes3.New(paused, gofakes3.WithHostBucket(virtual)).Server())
			url := "http://localhost/bucket/copied"
			if virtual {
				url = "http://bucket.localhost/copied"
			}
			req := httptest.NewRequest(http.MethodPut, url, nil)
			req.Header.Set("X-Amz-Copy-Source", "/bucket/object.txt")
			req.Header.Set("X-Amz-Copy-Source-If-Match", objectETag(t, b, "bucket", "object.txt"))
			response := httptest.NewRecorder()
			complete := make(chan struct{})
			go func() { defer close(complete); handler.ServeHTTP(response, req) }()
			var resumed bool
			defer func() {
				if !resumed {
					close(paused.resume)
				}
			}()
			select {
			case <-paused.head:
			case <-time.After(5 * time.Second):
				t.Fatal("copy did not read source")
			}
			writer := make(chan error, 1)
			go func() {
				_, err := cb.PutObject(context.Background(), "bucket", "object.txt", nil, strings.NewReader("new"), 3)
				writer <- err
			}()
			key := cb.objectLockKey(context.Background(), "bucket", "object.txt")
			require.Eventually(t, func() bool {
				cb.locks.mu.Lock()
				defer cb.locks.mu.Unlock()
				l := cb.locks.locks[key]
				return l != nil && l.refs >= 2
			}, 5*time.Second, time.Millisecond)
			close(paused.resume)
			resumed = true
			select {
			case <-complete:
			case <-time.After(5 * time.Second):
				t.Fatal("copy did not complete")
			}
			require.Equal(t, 200, response.Code, response.Body.String())
			require.NoError(t, <-writer)
			contents, err := os.ReadFile(filepath.Join(root, "bucket", "copied"))
			require.NoError(t, err)
			require.Equal(t, "normal object", string(contents))
		})
	}
}
