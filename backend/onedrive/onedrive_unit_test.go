package onedrive

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/stretchr/testify/assert"
)

func TestShouldRetry(t *testing.T) {
	ctx := context.Background()

	t.Run("NilResponse", func(t *testing.T) {
		retry, err := shouldRetry(ctx, nil, nil)
		assert.False(t, retry)
		assert.NoError(t, err)
	})

	t.Run("200OK", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Header: http.Header{}}
		retry, err := shouldRetry(ctx, resp, nil)
		assert.False(t, retry)
		assert.NoError(t, err)
	})

	t.Run("401ExpiredToken", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 401,
			Header:     http.Header{"Www-Authenticate": []string{"Bearer realm=\"\", error=\"expired_token\""}},
		}
		retry, err := shouldRetry(ctx, resp, errors.New("unauthorized"))
		assert.True(t, retry)
		assert.Error(t, err)
	})

	t.Run("401RPSError", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 401,
			Header:     http.Header{},
		}
		retry, err := shouldRetry(ctx, resp, errors.New("Unable to initialize RPS"))
		assert.True(t, retry)
		assert.Error(t, err)
	})

	t.Run("401NoSpecialCase", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 401,
			Header:     http.Header{},
		}
		retry, err := shouldRetry(ctx, resp, errors.New("unauthorized"))
		assert.False(t, retry)
		assert.Error(t, err)
	})

	t.Run("400PathTooLong", func(t *testing.T) {
		resp := &http.Response{StatusCode: 400, Header: http.Header{}}
		apiErr := &api.Error{}
		apiErr.ErrorInfo.InnerError.Code = "pathIsTooLong"
		retry, err := shouldRetry(ctx, resp, apiErr)
		assert.False(t, retry)
		assert.True(t, fserrors.IsNoRetryError(err))
	})

	t.Run("429RetryAfter", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 429,
			Header:     http.Header{"Retry-After": []string{"5"}},
		}
		retry, err := shouldRetry(ctx, resp, errors.New("too many requests"))
		assert.True(t, retry)
		assert.Error(t, err)
	})

	t.Run("507InsufficientStorage", func(t *testing.T) {
		resp := &http.Response{StatusCode: 507, Header: http.Header{}}
		retry, err := shouldRetry(ctx, resp, errors.New("storage full"))
		assert.False(t, retry)
		assert.True(t, fserrors.IsFatalError(err))
	})

	t.Run("502Retry", func(t *testing.T) {
		resp := &http.Response{StatusCode: 502, Header: http.Header{}}
		retry, err := shouldRetry(ctx, resp, errors.New("bad gateway"))
		assert.True(t, retry)
		assert.Error(t, err)
	})
}
