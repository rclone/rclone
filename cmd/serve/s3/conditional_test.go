package s3

import (
	"context"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/gofakes3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func conditionalContext(match, none string) context.Context {
	h := http.Header{}
	if match != "" {
		h.Set("If-Match", match)
	}
	if none != "" {
		h.Set("If-None-Match", none)
	}
	c := parseWriteConditions(h)
	if c == nil {
		return context.Background()
	}
	return context.WithValue(context.Background(), writeConditionsKey{}, c)
}

func objectETag(t *testing.T, b *s3Backend, bucket, object string) string {
	t.Helper()
	obj, err := b.HeadObject(context.Background(), bucket, object)
	require.NoError(t, err)
	defer func() { _ = obj.Contents.Close() }()
	return `"` + hex.EncodeToString(obj.Hash) + `"`
}

func TestConditionalPutObject(t *testing.T) {
	b, root := newTestBackend(t)
	cb := newConditionalBackend(b)
	oldETag := objectETag(t, b, "bucket", "object.txt")
	objectPath := filepath.Join(root, "bucket", "object.txt")

	t.Run("IfMatchSuccess", func(t *testing.T) {
		ctx := conditionalContext(oldETag, "")
		const body = "conditional replacement"
		_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader(body), int64(len(body)))
		require.NoError(t, err)

		got, err := os.ReadFile(objectPath)
		require.NoError(t, err)
		assert.Equal(t, body, string(got))
	})

	t.Run("IfMatchStale", func(t *testing.T) {
		ctx := conditionalContext(oldETag, "")
		_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader("must not win"), 12)
		assert.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)

		got, readErr := os.ReadFile(objectPath)
		require.NoError(t, readErr)
		assert.Equal(t, "conditional replacement", string(got))
	})

	t.Run("IfNoneMatchCreate", func(t *testing.T) {
		ctx := conditionalContext("", "*")
		const body = "created once"
		_, err := cb.PutObject(ctx, "bucket", "new.txt", map[string]string{}, strings.NewReader(body), int64(len(body)))
		require.NoError(t, err)

		ctx = conditionalContext("", "*")
		_, err = cb.PutObject(ctx, "bucket", "new.txt", map[string]string{}, strings.NewReader("second"), 6)
		assert.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)

	})
}

func TestConditionalBackendPutRace(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	etag := objectETag(t, b, "bucket", "object.txt")

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, body := range []string{"writer-one", "writer-two"} {
		go func() {
			ready.Done()
			<-start
			ctx := conditionalContext(etag, "")
			_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader(body), int64(len(body)))
			results <- err
		}()
	}
	ready.Wait()
	close(start)

	var successes, failures int
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else {
			assert.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)
			failures++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, failures)
}
