package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkRedirectReq makes a minimal *http.Request pointing at rawURL for
// exercising the CheckRedirect functions.
func mkRedirectReq(t *testing.T, rawURL, method string) *http.Request {
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &http.Request{URL: u, Method: method, Header: http.Header{}}
}

func TestPreserveMethodRedirectFn(t *testing.T) {
	t.Run("PreservesMethod", func(t *testing.T) {
		orig := mkRedirectReq(t, "https://example.com/a", "PROPFIND")
		next := mkRedirectReq(t, "https://example.com/b", "GET")
		require.NoError(t, PreserveMethodRedirectFn(next, []*http.Request{orig}))
		assert.Equal(t, "PROPFIND", next.Method)
	})
	t.Run("RefusesDowngrade", func(t *testing.T) {
		orig := mkRedirectReq(t, "https://example.com/a", "PROPFIND")
		next := mkRedirectReq(t, "http://example.com/b", "PROPFIND")
		assert.ErrorIs(t, PreserveMethodRedirectFn(next, []*http.Request{orig}), ErrHTTPSDowngrade)
	})
	t.Run("AllowsCrossHostHTTPS", func(t *testing.T) {
		orig := mkRedirectReq(t, "https://example.com/a", "PROPFIND")
		next := mkRedirectReq(t, "https://other.example.com/b", "PROPFIND")
		assert.NoError(t, PreserveMethodRedirectFn(next, []*http.Request{orig}))
	})
	t.Run("AllowsPlainHTTP", func(t *testing.T) {
		orig := mkRedirectReq(t, "http://example.com/a", "PROPFIND")
		next := mkRedirectReq(t, "http://example.com/b", "PROPFIND")
		assert.NoError(t, PreserveMethodRedirectFn(next, []*http.Request{orig}))
	})
	t.Run("TooManyRedirects", func(t *testing.T) {
		next := mkRedirectReq(t, "https://example.com/b", "GET")
		via := make([]*http.Request, 10)
		assert.Error(t, PreserveMethodRedirectFn(next, via))
	})
}

func TestRefuseHTTPSDowngradeRedirectFn(t *testing.T) {
	t.Run("RefusesDowngrade", func(t *testing.T) {
		orig := mkRedirectReq(t, "https://example.com/a", "GET")
		next := mkRedirectReq(t, "http://example.com/b", "GET")
		assert.ErrorIs(t, RefuseHTTPSDowngradeRedirectFn(next, []*http.Request{orig}), ErrHTTPSDowngrade)
	})
	t.Run("AllowsUpgrade", func(t *testing.T) {
		orig := mkRedirectReq(t, "http://example.com/a", "GET")
		next := mkRedirectReq(t, "https://example.com/b", "GET")
		assert.NoError(t, RefuseHTTPSDowngradeRedirectFn(next, []*http.Request{orig}))
	})
	t.Run("AllowsSameScheme", func(t *testing.T) {
		orig := mkRedirectReq(t, "https://example.com/a", "GET")
		next := mkRedirectReq(t, "https://example.com/b", "GET")
		assert.NoError(t, RefuseHTTPSDowngradeRedirectFn(next, []*http.Request{orig}))
	})
	t.Run("RefusesDowngradeViaOtherHost", func(t *testing.T) {
		orig := mkRedirectReq(t, "https://example.com/a", "GET")
		mid := mkRedirectReq(t, "https://other.example/b", "GET")
		next := mkRedirectReq(t, "http://example.com/c", "GET")
		assert.ErrorIs(t, RefuseHTTPSDowngradeRedirectFn(next, []*http.Request{orig, mid}), ErrHTTPSDowngrade)
	})
	t.Run("AllowsPlaintextOriginViaHTTPS", func(t *testing.T) {
		orig := mkRedirectReq(t, "http://example.com/a", "GET")
		mid := mkRedirectReq(t, "https://other.example/b", "GET")
		next := mkRedirectReq(t, "http://example.com/c", "GET")
		assert.NoError(t, RefuseHTTPSDowngradeRedirectFn(next, []*http.Request{orig, mid}))
	})
	t.Run("TooManyRedirects", func(t *testing.T) {
		next := mkRedirectReq(t, "https://example.com/b", "GET")
		via := make([]*http.Request, 10)
		assert.Error(t, RefuseHTTPSDowngradeRedirectFn(next, via))
	})
}

func TestSameHost(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want bool
	}{
		{"https://example.com/a", "https://example.com/b", true},
		{"https://example.com/", "https://EXAMPLE.com/", true},
		{"https://example.com/", "https://example.com:443/", true},
		{"http://example.com/", "http://example.com:80/", true},
		{"https://example.com/", "http://example.com/", false},
		{"https://example.com:8443/", "https://example.com:8444/", false},
		{"https://example.com/", "https://www.example.com/", false},
		{"https://example.com/", "https://example.com.evil/", false},
		{"http://[::1]:8080/", "http://[::1]:8080/", true},
		{"http://[::1]:8080/", "http://[::1]:8081/", false},
	} {
		a, err := url.Parse(test.a)
		require.NoError(t, err)
		b, err := url.Parse(test.b)
		require.NoError(t, err)
		assert.Equal(t, test.want, SameHost(a, b), "%s vs %s", test.a, test.b)
	}
}

// newDowngradeServers returns an HTTPS server that redirects every
// request to a plaintext HTTP server on the same host, together with a
// flag that records whether the plaintext server ever received an
// Authorization header. Use tlsSrv.Client() for a client that trusts the
// test certificate.
func newDowngradeServers(t *testing.T) (tlsSrv *httptest.Server, sawAuth *atomic.Bool) {
	t.Helper()
	sawAuth = new(atomic.Bool)

	// Plaintext HTTP target that records whether it received credentials.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			sawAuth.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)

	// HTTPS server that redirects to the plaintext target on the same host.
	tlsSrv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(tlsSrv.Close)

	return tlsSrv, sawAuth
}

// TestRefuseHTTPSDowngradeRedirectEndToEnd drives a real HTTPS-to-HTTP
// redirect through rest.Client with credentials set via SetUserPass, the
// same way the webdav backend does. It shows that the default client
// leaks credentials to the plaintext hop, whereas the redirect handlers
// refuse to follow the downgrade and the plaintext hop never sees them.
func TestRefuseHTTPSDowngradeRedirectEndToEnd(t *testing.T) {
	ctx := context.Background()

	// Baseline: without any check the credentials set by SetUserPass are
	// forwarded over the plaintext hop, which is the vulnerability.
	t.Run("DefaultLeaks", func(t *testing.T) {
		tlsSrv, sawAuth := newDowngradeServers(t)
		api := NewClient(tlsSrv.Client()).SetRoot(tlsSrv.URL)
		api.SetUserPass("user", "pass")
		_, err := api.Call(ctx, &Opts{Method: "GET", Path: "/file", NoResponse: true})
		require.NoError(t, err)
		assert.True(t, sawAuth.Load(), "expected credentials to be sent over plaintext")
	})

	// Default path (all normal webdav calls): the client refuses the
	// downgrade so nothing is sent.
	t.Run("DefaultPathRefused", func(t *testing.T) {
		tlsSrv, sawAuth := newDowngradeServers(t)
		client := tlsSrv.Client()
		client.CheckRedirect = RefuseHTTPSDowngradeRedirectFn
		api := NewClient(client).SetRoot(tlsSrv.URL)
		api.SetUserPass("user", "pass")
		_, err := api.Call(ctx, &Opts{Method: "GET", Path: "/file", NoResponse: true})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrHTTPSDowngrade)
		assert.False(t, sawAuth.Load(), "plaintext hop must not receive credentials")
	})

	// PROPFIND path (readMetaDataForPath): the per-call CheckRedirect
	// refuses the downgrade too.
	t.Run("PropfindPathRefused", func(t *testing.T) {
		tlsSrv, sawAuth := newDowngradeServers(t)
		api := NewClient(tlsSrv.Client()).SetRoot(tlsSrv.URL)
		api.SetUserPass("user", "pass")
		_, err := api.Call(ctx, &Opts{Method: "PROPFIND", Path: "/dir", NoResponse: true, CheckRedirect: PreserveMethodRedirectFn})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrHTTPSDowngrade)
		assert.False(t, sawAuth.Load(), "plaintext hop must not receive credentials")
	})
}
