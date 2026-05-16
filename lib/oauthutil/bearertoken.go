// Bearer token transport for external token providers.

package oauthutil

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/singleflight"
)

// bearerTokenTransport is an http.RoundTripper that adds a Bearer token
// obtained from an external command. On 401 responses it re-fetches the
// token and retries the request once.
type bearerTokenTransport struct {
	cmd   fs.SpaceSepList
	mu    sync.Mutex
	token string
	wrap  http.RoundTripper
	sf    singleflight.Group
}

// newBearerTokenTransport creates a transport that fetches a bearer token
// from cmd and injects it into every request. It fetches an initial token
// before returning.
func newBearerTokenTransport(cmd fs.SpaceSepList, wrap http.RoundTripper) (*bearerTokenTransport, error) {
	t := &bearerTokenTransport{
		cmd:  cmd,
		wrap: wrap,
	}
	token, err := t.fetchToken()
	if err != nil {
		return nil, err
	}
	t.token = token
	return t, nil
}

// fetchToken runs the bearer_token_command and returns the token string.
func (t *bearerTokenTransport) fetchToken() (string, error) {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
		c      = exec.Command(t.cmd[0], t.cmd[1:]...)
	)
	c.Stdout = &stdout
	c.Stderr = &stderr
	var (
		err          = c.Run()
		stdoutString = strings.TrimSpace(stdout.String())
		stderrString = strings.TrimSpace(stderr.String())
	)
	if err != nil {
		if stderrString == "" {
			stderrString = stdoutString
		}
		return "", fmt.Errorf("failed to get bearer token using %q: %s: %w", t.cmd, stderrString, err)
	}
	return stdoutString, nil
}

// refreshToken re-fetches the token using singleflight to prevent
// concurrent refreshes.
func (t *bearerTokenTransport) refreshToken() (string, error) {
	v, err, _ := t.sf.Do("refresh", func() (any, error) {
		token, err := t.fetchToken()
		if err != nil {
			return nil, err
		}
		t.mu.Lock()
		t.token = token
		t.mu.Unlock()
		return token, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// RoundTrip implements http.RoundTripper. It sets the Authorization header,
// and on 401 re-fetches the token and retries once.
func (t *bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	token := t.token
	t.mu.Unlock()

	// Clone the request so we can safely modify headers
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.wrap.RoundTrip(req2)
	if err != nil || resp.StatusCode != 401 {
		return resp, err
	}

	// Got 401 — try to refresh and retry once
	fs.Debugf(nil, "Bearer token expired, refreshing")
	_ = resp.Body.Close()

	newToken, refreshErr := t.refreshToken()
	if refreshErr != nil {
		return resp, refreshErr
	}

	req3 := req.Clone(req.Context())
	req3.Header.Set("Authorization", "Bearer "+newToken)
	return t.wrap.RoundTrip(req3)
}
