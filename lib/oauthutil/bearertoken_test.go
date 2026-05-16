package oauthutil

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientWithBaseClient_BearerTokenCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	t.Run("UsesTokenCommandWhenSet", func(t *testing.T) {
		m := configmap.Simple{
			"bearer_token_command": "echo test-token",
		}
		oauthConfig := &Config{
			ClientID: "unused",
			TokenURL: "https://unused.example.com/token",
			AuthURL:  "https://unused.example.com/auth",
		}
		client, ts, err := NewClientWithBaseClient(context.Background(), "test", m, oauthConfig, &http.Client{})
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.NotNil(t, ts, "TokenSource should be non-nil (inert) when using bearer_token_command")

		// Verify the transport is a bearerTokenTransport
		bt, ok := client.Transport.(*bearerTokenTransport)
		require.True(t, ok, "client transport should be bearerTokenTransport")
		assert.Equal(t, "test-token", bt.token)
	})

	t.Run("SkippedWhenNotSet", func(t *testing.T) {
		m := configmap.Simple{}
		oauthConfig := &Config{
			ClientID: "test-client",
			TokenURL: "https://unused.example.com/token",
			AuthURL:  "https://unused.example.com/auth",
		}
		// This will fail because there's no token in the config, but it
		// proves the bearer_token_command path was NOT taken
		_, _, err := NewClientWithBaseClient(context.Background(), "test", m, oauthConfig, &http.Client{})
		assert.Error(t, err, "should fail because no token exists (normal OAuth path)")
	})

	t.Run("FailsOnBadCommand", func(t *testing.T) {
		m := configmap.Simple{
			"bearer_token_command": "nonexistent-command-xyz",
		}
		oauthConfig := &Config{}
		_, _, err := NewClientWithBaseClient(context.Background(), "test", m, oauthConfig, &http.Client{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get bearer token")
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
