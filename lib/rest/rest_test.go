package rest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

// TestReadBodyLimit checks that ReadBody refuses to buffer more than
// drainLimit bytes, so a server streaming an endless response can't
// make rclone allocate memory without bound.
func TestReadBodyLimit(t *testing.T) {
	newResp := func(body io.Reader) *http.Response {
		return &http.Response{Body: io.NopCloser(body)}
	}
	t.Run("AtLimit", func(t *testing.T) {
		want := bytes.Repeat([]byte("x"), drainLimit)
		got, err := ReadBody(newResp(bytes.NewReader(want)))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("OverLimit", func(t *testing.T) {
		got, err := ReadBody(newResp(bytes.NewReader(make([]byte, drainLimit+1))))
		assert.ErrorIs(t, err, ErrBodyTooLarge)
		assert.Nil(t, got)
	})

	// A server which answers with an error status and then streams a
	// body far bigger than drainLimit. The default error handler must
	// give up at the limit rather than read it all.
	t.Run("EndlessErrorBody", func(t *testing.T) {
		const chunk = 1 << 20
		const maxChunks = 64 // bounds the test if the client keeps reading
		var written atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			buf := bytes.Repeat([]byte("x"), chunk)
			for range maxChunks {
				n, err := w.Write(buf)
				written.Add(int64(n))
				if err != nil {
					return
				}
				w.(http.Flusher).Flush()
			}
		}))
		t.Cleanup(srv.Close)

		api := NewClient(srv.Client()).SetRoot(srv.URL)
		_, err := api.Call(context.Background(), &Opts{Method: "GET", Path: "/"})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrBodyTooLarge)
		assert.Less(t, written.Load(), int64(maxChunks*chunk), "client should stop reading before the server stops sending")
	})
}

func TestRedirectLeavesHost(t *testing.T) {
	for _, test := range []struct {
		name string
		via  []string
		req  string
		want bool
	}{
		{"NoVia", nil, "https://example.com/a", false},
		{"SameHost", []string{"https://example.com/"}, "https://example.com/b", false},
		{"SameHostDifferentCase", []string{"https://example.com/"}, "https://EXAMPLE.com/b", false},
		{"SameHostDefaultPort", []string{"https://example.com/"}, "https://example.com:443/b", false},
		// An upgrade changes the default port so counts as a new host
		{"SameHostUpgrade", []string{"http://example.com/"}, "https://example.com/b", true},
		{"DifferentHost", []string{"https://example.com/"}, "https://other.example.com/b", true},
		{"DifferentPort", []string{"https://example.com/"}, "https://example.com:8443/b", true},
		{"BackToOrigin", []string{"https://example.com/", "https://other.example.com/b"}, "https://example.com/c", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			via := make([]*http.Request, len(test.via))
			for i, rawURL := range test.via {
				via[i] = mkRedirectReq(t, rawURL, "GET")
			}
			assert.Equal(t, test.want, redirectLeavesHost(mkRedirectReq(t, test.req, "GET"), via))
		})
	}
}

// stripTestHeaders are the headers StripHeadersOnCrossHostRedirectFn
// is asked to strip in the tests, with the value each is sent with.
//
// The one starting with "*" is sent without canonicalising.
var stripTestHeaders = map[string]string{
	"Authorization":   "secret-token",
	"X-Custom-Secret": "secret-custom",
	"*x-raw-secret":   "secret-raw",
}

// stripTestRequest makes a GET request to rawURL carrying the headers
// to be stripped plus a harmless marker header.
func stripTestRequest(t *testing.T, rawURL string) *http.Request {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	require.NoError(t, err)
	for header, value := range stripTestHeaders {
		if raw, ok := strings.CutPrefix(header, "*"); ok {
			req.Header[raw] = []string{value}
		} else {
			req.Header.Set(header, value)
		}
	}
	req.Header.Set("X-Marker", "marker")
	return req
}

// stripTestClient makes an http.Client using StripHeadersOnCrossHostRedirectFn
func stripTestClient() *http.Client {
	headers := make([]string, 0, len(stripTestHeaders))
	for header := range stripTestHeaders {
		headers = append(headers, header)
	}
	return &http.Client{CheckRedirect: StripHeadersOnCrossHostRedirectFn(headers...)}
}

func TestStripHeadersOnCrossHostRedirectFn(t *testing.T) {
	assertStripped := func(r *http.Request) {
		for header := range stripTestHeaders {
			assert.Empty(t, r.Header.Get(strings.TrimPrefix(header, "*")), "%s should have been stripped", header)
		}
		assert.Equal(t, "marker", r.Header.Get("X-Marker"))
	}
	assertKept := func(r *http.Request) {
		for header, value := range stripTestHeaders {
			assert.Equal(t, value, r.Header.Get(strings.TrimPrefix(header, "*")), "%s should have been kept", header)
		}
		assert.Equal(t, "marker", r.Header.Get("X-Marker"))
	}

	t.Run("CrossHost", func(t *testing.T) {
		// httptest servers listen on 127.0.0.1:port so two servers
		// count as different hosts.
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertStripped(r)
			w.WriteHeader(http.StatusOK)
		}))
		defer target.Close()
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
		}))
		defer redirector.Close()

		resp, err := stripTestClient().Do(stripTestRequest(t, redirector.URL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NoError(t, resp.Body.Close())
	})

	t.Run("BackToOrigin", func(t *testing.T) {
		// The origin redirects to another host which redirects back
		// to the origin. The headers must stay stripped on the way
		// back as the other host chose the URL.
		var origin *httptest.Server
		bouncer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertStripped(r)
			http.Redirect(w, r, origin.URL+"/final", http.StatusTemporaryRedirect)
		}))
		defer bouncer.Close()
		origin = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				assertKept(r)
				http.Redirect(w, r, bouncer.URL, http.StatusTemporaryRedirect)
			case "/final":
				assertStripped(r)
				w.WriteHeader(http.StatusOK)
			default:
				http.NotFound(w, r)
			}
		}))
		defer origin.Close()

		resp, err := stripTestClient().Do(stripTestRequest(t, origin.URL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NoError(t, resp.Body.Close())
	})

	t.Run("SameHost", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
			case "/redirected":
				assertKept(r)
				w.WriteHeader(http.StatusOK)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		resp, err := stripTestClient().Do(stripTestRequest(t, server.URL))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NoError(t, resp.Body.Close())
	})

	t.Run("SameHostDifferentCase", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				u, err := url.Parse(server.URL)
				require.NoError(t, err)
				http.Redirect(w, r, "http://LOCALHOST:"+u.Port()+"/redirected", http.StatusTemporaryRedirect)
			case "/redirected":
				// Before go1.27 net/http compares hosts case
				// sensitively and drops Authorization itself here.
				r.Header.Set("Authorization", stripTestHeaders["Authorization"])
				assertKept(r)
				w.WriteHeader(http.StatusOK)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		u, err := url.Parse(server.URL)
		require.NoError(t, err)
		resp, err := stripTestClient().Do(stripTestRequest(t, "http://localhost:"+u.Port()+"/"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NoError(t, resp.Body.Close())
	})

	t.Run("RefusesDowngrade", func(t *testing.T) {
		plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Fail(t, "plaintext server should not have been contacted")
		}))
		defer plain.Close()
		tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, plain.URL, http.StatusTemporaryRedirect)
		}))
		defer tlsServer.Close()

		client := stripTestClient()
		client.Transport = tlsServer.Client().Transport
		// On a CheckRedirect error the client returns the redirect
		// response, with its body already closed, alongside the error.
		resp, err := client.Do(stripTestRequest(t, tlsServer.URL)) //nolint:bodyclose // closed by the client
		require.ErrorIs(t, err, ErrHTTPSDowngrade)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	})

	t.Run("StopsAfterTenRedirects", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
		}))
		defer server.Close()

		resp, err := stripTestClient().Do(stripTestRequest(t, server.URL)) //nolint:bodyclose // closed by the client
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stopped after 10 redirects")
		require.NotNil(t, resp)
	})
}
