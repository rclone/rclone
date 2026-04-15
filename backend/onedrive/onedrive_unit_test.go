package onedrive

import (
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBearerTokenTransport_FetchToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	t.Run("Success", func(t *testing.T) {
		bt := &bearerTokenTransport{
			cmd: fs.SpaceSepList{"echo", "my-token"},
		}
		token, err := bt.fetchToken()
		assert.NoError(t, err)
		assert.Equal(t, "my-token", token)
	})

	t.Run("TrimsWhitespace", func(t *testing.T) {
		bt := &bearerTokenTransport{
			cmd: fs.SpaceSepList{"printf", "  token-with-spaces  \n"},
		}
		token, err := bt.fetchToken()
		assert.NoError(t, err)
		assert.Equal(t, "token-with-spaces", token)
	})

	t.Run("CommandFailure", func(t *testing.T) {
		bt := &bearerTokenTransport{
			cmd: fs.SpaceSepList{"false"},
		}
		_, err := bt.fetchToken()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get bearer token")
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		bt := &bearerTokenTransport{
			cmd: fs.SpaceSepList{"nonexistent-command-xyz"},
		}
		_, err := bt.fetchToken()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get bearer token")
	})
}

func TestNewBearerTokenTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	t.Run("Success", func(t *testing.T) {
		bt, err := newBearerTokenTransport(
			fs.SpaceSepList{"echo", "initial-token"},
			http.DefaultTransport,
		)
		require.NoError(t, err)
		assert.Equal(t, "initial-token", bt.token)
	})

	t.Run("CommandFailure", func(t *testing.T) {
		_, err := newBearerTokenTransport(
			fs.SpaceSepList{"false"},
			http.DefaultTransport,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get bearer token")
	})
}

// mockRoundTripper records requests and returns a configurable response.
type mockRoundTripper struct {
	statusCode int
	requests   []*http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	return &http.Response{
		StatusCode: m.statusCode,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestBearerTokenTransport_RoundTrip(t *testing.T) {
	t.Run("SetsAuthorizationHeader", func(t *testing.T) {
		mock := &mockRoundTripper{statusCode: 200}
		bt := &bearerTokenTransport{
			cmd:   fs.SpaceSepList{"echo", "test-token"},
			token: "test-token",
			wrap:  mock,
		}
		req, _ := http.NewRequest("GET", "https://example.com", nil)
		resp, err := bt.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Len(t, mock.requests, 1)
		assert.Equal(t, "Bearer test-token", mock.requests[0].Header.Get("Authorization"))
	})

	t.Run("DoesNotMutateOriginalRequest", func(t *testing.T) {
		mock := &mockRoundTripper{statusCode: 200}
		bt := &bearerTokenTransport{
			cmd:   fs.SpaceSepList{"echo", "test-token"},
			token: "test-token",
			wrap:  mock,
		}
		req, _ := http.NewRequest("GET", "https://example.com", nil)
		_, err := bt.RoundTrip(req)
		require.NoError(t, err)
		assert.Empty(t, req.Header.Get("Authorization"))
	})
}

// mockRoundTripperSequence returns different status codes on successive calls.
type mockRoundTripperSequence struct {
	codes    []int
	call     int
	requests []*http.Request
}

func (m *mockRoundTripperSequence) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	code := m.codes[m.call]
	if m.call < len(m.codes)-1 {
		m.call++
	}
	return &http.Response{
		StatusCode: code,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestBearerTokenTransport_RefreshOn401(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	t.Run("RefreshesAndRetries", func(t *testing.T) {
		mock := &mockRoundTripperSequence{codes: []int{401, 200}}
		bt := &bearerTokenTransport{
			cmd:   fs.SpaceSepList{"echo", "refreshed-token"},
			token: "stale-token",
			wrap:  mock,
		}

		req, _ := http.NewRequest("GET", "https://example.com", nil)
		resp, err := bt.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		// Should have made 2 requests
		assert.Len(t, mock.requests, 2)
		// First request had the stale token
		assert.Equal(t, "Bearer stale-token", mock.requests[0].Header.Get("Authorization"))
		// Second request has the refreshed token
		assert.Equal(t, "Bearer refreshed-token", mock.requests[1].Header.Get("Authorization"))
		// Token on transport is updated
		assert.Equal(t, "refreshed-token", bt.token)
	})
}
