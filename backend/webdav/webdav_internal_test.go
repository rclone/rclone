package webdav_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/webdav"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

func TestChunkedUploadRetriesOnlyFailedChunk(t *testing.T) {
	tests := []struct {
		name       string
		failChunk1 bool
		wantError  bool
		wantChunk2 int32
	}{
		{name: "retry failed chunk", wantChunk2: 1},
		{name: "stop after chunk retry budget", failChunk1: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var chunk0, chunk1, chunk2 atomic.Int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/dav/uploads/alice/"):
					switch {
					case strings.HasSuffix(r.URL.Path, "000000000000000-000000000000003"):
						chunk0.Add(1)
					case strings.HasSuffix(r.URL.Path, "000000000000004-000000000000007"):
						if test.failChunk1 || chunk1.Add(1) == 1 {
							if test.failChunk1 {
								chunk1.Add(1)
							}
							w.WriteHeader(http.StatusServiceUnavailable)
							return
						}
					case strings.HasSuffix(r.URL.Path, "000000000000008-000000000000009"):
						chunk2.Add(1)
					default:
						t.Errorf("unexpected chunk path %q", r.URL.Path)
					}
					w.WriteHeader(http.StatusCreated)
				case r.Method == "MKCOL":
					w.WriteHeader(http.StatusCreated)
				case r.Method == "MOVE":
					w.WriteHeader(http.StatusCreated)
				case r.Method == "DELETE":
					w.WriteHeader(http.StatusNotFound)
				case r.Method == "PROPFIND":
					w.WriteHeader(http.StatusMultiStatus)
					_, _ = io.WriteString(w, `<d:multistatus xmlns:d="DAV:"><d:response><d:href>/file</d:href><d:propstat><d:prop><d:getcontentlength>10</d:getcontentlength><d:resourcetype/></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
				case r.Method == "PATCH":
					w.Header().Set("OC-Checksum", "SHA1:0000000000000000000000000000000000000000")
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			ts := httptest.NewServer(handler)
			defer ts.Close()
			configfile.Install()
			ctx, ci := fs.AddConfig(context.Background())
			ci.LowLevelRetries = 2
			m := configmap.Simple{
				"type":                 "webdav",
				"url":                  ts.URL + "/remote.php/dav/files/alice",
				"vendor":               "nextcloud",
				"nextcloud_chunk_size": "4B",
			}
			f, err := webdav.NewFs(ctx, remoteName, "", m)
			require.NoError(t, err)
			src := object.NewStaticObjectInfo("file", time.Now(), 10, true, nil, f)
			body := []byte("abcdefghij")
			err = operations.Retry(ctx, src, ci.LowLevelRetries, func() error {
				_, err := f.Put(ctx, bytes.NewReader(body), src)
				return err
			})
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.EqualValues(t, 1, chunk0.Load(), "successful first chunk should not be uploaded again")
			assert.EqualValues(t, 2, chunk1.Load(), "only the failed chunk should be retried")
			assert.EqualValues(t, test.wantChunk2, chunk2.Load(), "later chunks should not upload after failure")
		})
	}
}

var (
	remoteName = "TestWebDAV"
	headers    = []string{"X-Potato", "sausage", "X-Rhubarb", "cucumber"}
)

// prepareServer the test server and return a function to tidy it up afterwards
// with each request the headers option tests are executed
func prepareServer(t *testing.T) (configmap.Simple, func()) {
	// test the headers are there send send a dummy response to About
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		what := fmt.Sprintf("%s %s: Header ", r.Method, r.URL.Path)
		assert.Equal(t, headers[1], r.Header.Get(headers[0]), what+headers[0])
		assert.Equal(t, headers[3], r.Header.Get(headers[2]), what+headers[2])
		_, err := fmt.Fprintf(w, `<d:multistatus xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns" xmlns:oc="http://owncloud.org/ns" xmlns:nc="http://nextcloud.org/ns">
<d:response>
 <d:href>/remote.php/webdav/</d:href>
 <d:propstat>
  <d:prop>
   <d:quota-available-bytes>-3</d:quota-available-bytes>
   <d:quota-used-bytes>376461895</d:quota-used-bytes>
  </d:prop>
  <d:status>HTTP/1.1 200 OK</d:status>
 </d:propstat>
</d:response>
</d:multistatus>`)
		require.NoError(t, err)
	})
	// Make the test server
	ts := httptest.NewServer(handler)

	// Configure the remote
	configfile.Install()

	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		// add headers to test the headers option
		"headers": strings.Join(headers, ","),
	}

	// return a function to tidy up
	return m, ts.Close
}

// prepare the test server and return a function to tidy it up afterwards
func prepare(t *testing.T) (fs.Fs, func()) {
	m, tidy := prepareServer(t)

	// Instantiate the WebDAV server
	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	return f, tidy
}

// TestHeaders any request will test the headers option
func TestHeaders(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	// send an About response since that is all the dummy server can return
	_, err := f.Features().About(context.Background())
	require.NoError(t, err)
}

func TestDcacheEncoding(t *testing.T) {
	for _, name := range []string{"valid.txt", "␁", "．", "．．", "a／b"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/"+name, r.URL.Path)
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = io.WriteString(w, fileInfoResponse)
			}))
			defer server.Close()
			ctx := context.Background()
			f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{"url": server.URL, "vendor": "dcache"})
			require.NoError(t, err)
			_, err = f.NewObject(ctx, name)
			require.NoError(t, err)
		})
	}
}

func TestUploadRedirect(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		readBody   bool
		chain      bool
		empty      bool
		auth       bool
		omitType   bool
		dcache     bool
		defaults   bool
		disabled   bool
		wantError  string
		delay      time.Duration
		expect     time.Duration
		decomposed bool
	}{
		{name: "307", status: http.StatusTemporaryRedirect},
		{name: "slow door short timeout", status: http.StatusTemporaryRedirect, delay: 100 * time.Millisecond, expect: 10 * time.Millisecond, wantError: "after reading"},
		{name: "slow door long timeout", status: http.StatusTemporaryRedirect, delay: 100 * time.Millisecond, expect: time.Second},
		{name: "308", status: http.StatusPermanentRedirect},
		{name: "redirect chain", status: http.StatusTemporaryRedirect, chain: true},
		{name: "empty file", status: http.StatusTemporaryRedirect, empty: true},
		{name: "preserve auth", status: http.StatusTemporaryRedirect, auth: true},
		{name: "omit content type", status: http.StatusTemporaryRedirect, omitType: true},
		{name: "dcache defaults", status: http.StatusTemporaryRedirect, dcache: true, defaults: true, omitType: true},
		{name: "other defaults", status: http.StatusTemporaryRedirect, defaults: true, disabled: true, wantError: "307"},
		{name: "dcache keep content type", status: http.StatusTemporaryRedirect, dcache: true},
		{name: "dcache preserves Unicode path", status: http.StatusTemporaryRedirect, dcache: true, decomposed: true},
		{name: "dcache preserves authenticated Unicode path", status: http.StatusTemporaryRedirect, dcache: true, decomposed: true, auth: true},
		{name: "other uses redirect Unicode path", status: http.StatusTemporaryRedirect, decomposed: true},
		{name: "dcache disable redirects", status: http.StatusTemporaryRedirect, dcache: true, disabled: true, wantError: "307"},
		{name: "no redirect", status: http.StatusCreated, readBody: true},
		{name: "body already sent", status: http.StatusTemporaryRedirect, readBody: true, wantError: "after reading"},
		{name: "method changing redirect", status: http.StatusFound, wantError: "307 or 308"},
		{name: "disabled", status: http.StatusTemporaryRedirect, disabled: true, wantError: "307"},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := "abcdefghij"
			if test.empty {
				content = ""
			}
			var uploads atomic.Int32
			pool := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					assert.Equal(t, "100-continue", r.Header.Get("Expect"))
					assert.EqualValues(t, len(content), r.ContentLength)
					if test.omitType {
						assert.Empty(t, r.Header.Get("Content-Type"))
					} else {
						assert.Contains(t, r.Header.Get("Content-Type"), "text/plain")
					}
				}
				assert.Equal(t, "token=secret", r.URL.RawQuery)
				if test.decomposed {
					want := "/caf\u00e9.txt"
					if test.dcache {
						want = norm.NFD.String(want)
					}
					assert.Equal(t, want, r.URL.Path)
				}
				if test.chain && r.URL.Path == "/pool" {
					w.Header().Set("Connection", "close")
					w.Header().Set("Location", "/final?token=secret")
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				}
				if test.auth {
					assert.Equal(t, "Bearer macaroon", r.Header.Get("Authorization"))
					assert.Equal(t, "private", r.Header.Get("X-Secret"))
				} else {
					assert.Empty(t, r.Header.Get("Authorization"))
					assert.Empty(t, r.Header.Get("X-Secret"))
				}
				if r.Method == http.MethodGet {
					_, _ = io.WriteString(w, content)
					return
				}
				assert.Equal(t, http.MethodPut, r.Method)
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Equal(t, content, string(body))
				uploads.Add(1)
				w.WriteHeader(http.StatusCreated)
			}))
			defer pool.Close()
			door := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					redirectPath := (&url.URL{Path: norm.NFC.String(r.URL.Path)}).EscapedPath()
					w.Header().Set("Location", strings.Replace(pool.URL, "127.0.0.1", "localhost", 1)+redirectPath+"?token=secret")
					w.WriteHeader(http.StatusTemporaryRedirect)
				case http.MethodPut:
					if !test.disabled {
						assert.Equal(t, "100-continue", r.Header.Get("Expect"))
					} else {
						assert.Empty(t, r.Header.Get("Expect"))
					}
					assert.Equal(t, "Bearer macaroon", r.Header.Get("Authorization"))
					time.Sleep(test.delay)
					if test.readBody {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)
						assert.Equal(t, content, string(body))
					}
					redirectPath := "/pool"
					if test.decomposed {
						redirectPath = (&url.URL{Path: norm.NFC.String(r.URL.Path)}).EscapedPath()
					}
					w.Header().Set("Location", strings.Replace(pool.URL, "127.0.0.1", "localhost", 1)+redirectPath+"?token=secret")
					w.WriteHeader(test.status)
				case "PROPFIND":
					w.WriteHeader(http.StatusMultiStatus)
					_, _ = io.WriteString(w, fileInfoResponse)
				default:
					w.WriteHeader(http.StatusCreated)
				}
			}))
			defer door.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctx, ci := fs.AddConfig(ctx)
			if test.expect != 0 {
				ci.ExpectContinueTimeout = fs.Duration(test.expect)
			}
			configfile.Install()
			m := configmap.Simple{
				"url": door.URL, "vendor": "other", "upload_redirect": fmt.Sprint(!test.disabled),
				"bearer_token": "macaroon", "headers": "X-Secret,private",
				"auth_redirect":                   fmt.Sprint(test.auth),
				"upload_redirect_no_content_type": fmt.Sprint(test.omitType),
			}
			if test.dcache {
				m["vendor"] = "dcache"
			}
			if test.defaults {
				delete(m, "upload_redirect")
				delete(m, "upload_redirect_no_content_type")
				delete(m, "auth_redirect")
			}
			ri, err := fs.Find("webdav")
			require.NoError(t, err)
			f, err := webdav.NewFs(ctx, remoteName, "", fs.ConfigMap("webdav", ri.Options, remoteName, m))
			require.NoError(t, err)
			name := "file.txt"
			if test.decomposed {
				name = norm.NFD.String("caf\u00e9.txt")
			}
			src := object.NewStaticObjectInfo(name, time.Now(), int64(len(content)), true, nil, f)
			// Hide Seek and concrete reader types to exercise a streaming upload.
			o, err := f.Put(ctx, struct{ io.Reader }{strings.NewReader(content)}, src)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				assert.Zero(t, uploads.Load())
			} else {
				require.NoError(t, err)
				if test.status == http.StatusCreated {
					assert.Zero(t, uploads.Load())
				} else {
					assert.EqualValues(t, 1, uploads.Load())
				}
				if test.decomposed {
					in, err := o.Open(ctx)
					require.NoError(t, err)
					body, err := io.ReadAll(in)
					require.NoError(t, err)
					require.NoError(t, in.Close())
					assert.Equal(t, content, string(body))
				}
			}
		})
	}
}

func TestUploadRedirectRetriesLocalCopy(t *testing.T) {
	for _, test := range []struct {
		name       string
		alwaysLate bool
		timeout    bool
	}{
		{name: "after continue"},
		{name: "after expect timeout", timeout: true},
		{name: "retry limit", alwaysLate: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctx, ci := fs.AddConfig(ctx)
			ci.LowLevelRetries = 2
			ci.BufferSize = 128 * 1024
			ci.ExpectContinueTimeout = fs.Duration(10 * time.Millisecond)
			content := strings.Repeat("upload redirect replay\n", 65536)
			var puts, uploads atomic.Int32
			pool := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Equal(t, content, string(body))
				uploads.Add(1)
				w.WriteHeader(http.StatusCreated)
			}))
			defer pool.Close()
			door := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPut:
					attempt := puts.Add(1)
					if test.timeout && attempt == 1 {
						// Read without sending 100 Continue, so the client must hit its expect timeout.
						conn, rw, err := w.(http.Hijacker).Hijack()
						require.NoError(t, err)
						defer func() { assert.NoError(t, conn.Close()) }()
						_, err = io.CopyN(io.Discard, rw, r.ContentLength)
						require.NoError(t, err)
						_, err = fmt.Fprintf(rw, "HTTP/1.1 307 Temporary Redirect\r\nLocation: %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", pool.URL)
						require.NoError(t, err)
						assert.NoError(t, rw.Flush())
						return
					}
					if test.alwaysLate || attempt == 1 {
						// Reading the body sends 100 Continue before the redirect.
						_, err := io.Copy(io.Discard, r.Body)
						assert.NoError(t, err)
					}
					w.Header().Set("Location", pool.URL)
					w.WriteHeader(http.StatusTemporaryRedirect)
				case "PROPFIND":
					w.WriteHeader(http.StatusMultiStatus)
					_, _ = io.WriteString(w, strings.Replace(fileInfoResponse, ">10<", fmt.Sprintf(">%d<", len(content)), 1))
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer door.Close()
			configfile.Install()
			dst, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{"url": door.URL, "vendor": "dcache"})
			require.NoError(t, err)
			srcFs, err := local.NewFs(ctx, "local", t.TempDir(), configmap.Simple{})
			require.NoError(t, err)
			info := object.NewStaticObjectInfo("file.txt", time.Now(), int64(len(content)), true, nil, srcFs)
			src, err := srcFs.Put(ctx, strings.NewReader(content), info)
			require.NoError(t, err)
			_, err = operations.Copy(ctx, dst, nil, "file.txt", src)
			if test.alwaysLate {
				require.ErrorContains(t, err, "after reading")
				assert.Zero(t, uploads.Load())
			} else {
				require.NoError(t, err)
				assert.EqualValues(t, 1, uploads.Load())
			}
			assert.EqualValues(t, 2, puts.Load(), "retry budget must be respected")
		})
	}
}

type blockedUploadReader struct {
	started chan struct{}
	release chan struct{}
	first   bool
}

func (r *blockedUploadReader) Read(p []byte) (int, error) {
	if !r.first {
		r.first = true
		p[0] = 'a'
		return 1, nil
	}
	close(r.started)
	<-r.release
	return 0, io.EOF
}

func TestUploadRedirectWaitsForReader(t *testing.T) {
	in := &blockedUploadReader{started: make(chan struct{}), release: make(chan struct{})}
	responded := make(chan struct{})
	pool := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b [1]byte
		_, err := io.ReadFull(r.Body, b[:])
		assert.NoError(t, err)
		<-in.started
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusForbidden)
		w.(http.Flusher).Flush()
		close(responded)
	}))
	defer pool.Close()
	door := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.Header().Set("Location", pool.URL)
			w.WriteHeader(http.StatusTemporaryRedirect)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer door.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{"url": door.URL, "upload_redirect": "true"})
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		src := object.NewStaticObjectInfo("file.txt", time.Now(), 2, true, nil, f)
		_, err := f.Put(ctx, in, src)
		done <- err
	}()
	select {
	case <-responded:
	case <-ctx.Done():
		close(in.release)
		t.Fatal("pool did not respond")
	}
	select {
	case err := <-done:
		close(in.release)
		t.Fatalf("upload returned while source read was active: %v", err)
	case <-time.After(1500 * time.Millisecond):
	}
	close(in.release)
	select {
	case err := <-done:
		require.Error(t, err)
	case <-ctx.Done():
		t.Fatal("upload did not finish after releasing source")
	}
}

// TestListAllAuthRedirect checks auth_redirect is honoured on listAll PROPFIND.
func TestListAllAuthRedirect(t *testing.T) {
	var targetAuth string
	var targetHits int

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits++
		targetAuth = r.Header.Get("Authorization")
		_, err := fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
		require.NoError(t, err)
	}))
	defer target.Close()

	// Redirect via a different hostname so net/http strips Authorization on cross-host redirect.
	targetURL := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetURL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	configfile.Install()
	m := configmap.Simple{
		"type":          "webdav",
		"url":           source.URL,
		"user":          "alice",
		"pass":          obscure.MustObscure("secret"),
		"auth_redirect": "true",
	}

	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	_, _ = f.List(context.Background(), "")

	assert.GreaterOrEqual(t, targetHits, 1, "redirect target should receive the request")
	assert.NotEmpty(t, targetAuth, "Authorization header should be preserved across redirect")
}

// redirectTestHeaders are the headers option for the redirect tests.
// The one starting with "*" is sent without canonicalising.
var redirectTestHeaders = []string{"X-Potato", "sausage", "*x-marrow", "pea"}

// redirectHeaders makes a webdav remote with redirectTestHeaders
// pointing at a server which redirects every request to a second
// server on hostname, or back to itself if hostname is empty, and
// exercises each of the backend's redirect policies against it: the
// PROPFIND used to look up an object, the GET used to read it and the
// PROPFIND used to list a directory. It returns the headers the
// redirect target saw on each of those requests.
func redirectHeaders(t *testing.T, hostname string) []http.Header {
	var target string
	var mu sync.Mutex
	var seen []http.Header
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/moved/") {
			http.Redirect(w, r, target+"/moved"+r.URL.Path, http.StatusTemporaryRedirect)
			return
		}
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		switch r.Method {
		case "PROPFIND":
			w.WriteHeader(http.StatusMultiStatus)
			_, err := fmt.Fprint(w, fileInfoResponse)
			assert.NoError(t, err)
		case http.MethodGet:
			_, err := fmt.Fprint(w, "hello")
			assert.NoError(t, err)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	if hostname == "" {
		target = ts.URL
	} else {
		// Serve the final response from a second server on a different host
		other := httptest.NewServer(handler)
		defer other.Close()
		target = strings.Replace(other.URL, "127.0.0.1", hostname, 1)
	}

	configfile.Install()
	m := configmap.Simple{
		"type":    "webdav",
		"url":     ts.URL,
		"headers": strings.Join(redirectTestHeaders, ","),
	}
	ctx := context.Background()
	f, err := webdav.NewFs(ctx, remoteName, "", m)
	require.NoError(t, err)

	// PROPFIND with Depth 0 uses rest.PreserveMethodRedirectFn
	o, err := f.NewObject(ctx, "file.txt")
	require.NoError(t, err)
	// GET uses the client's default redirect policy
	fd, err := o.Open(ctx)
	require.NoError(t, err)
	data, err := io.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())
	assert.Equal(t, "hello", string(data))
	// PROPFIND with Depth 1 uses the client's default redirect policy
	_, err = f.List(ctx, "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 3, "expected the redirect target to see all three requests")
	return seen
}

func TestRedirectKeepsHeadersOnSameHost(t *testing.T) {
	for _, got := range redirectHeaders(t, "") {
		for i := 0; i < len(redirectTestHeaders); i += 2 {
			assert.Equal(t, redirectTestHeaders[i+1], got.Get(strings.TrimPrefix(redirectTestHeaders[i], "*")))
		}
	}
}

func TestRedirectStripsHeadersOnHostChange(t *testing.T) {
	// Redirect to a different host name and port. net/http strips
	// Authorization on its own here so this checks the custom headers.
	for _, got := range redirectHeaders(t, "localhost") {
		for i := 0; i < len(redirectTestHeaders); i += 2 {
			assert.Empty(t, got.Values(strings.TrimPrefix(redirectTestHeaders[i], "*")), "header %q leaked to redirect target", redirectTestHeaders[i])
		}
	}
}

// TestReservedCharactersInPathAreEscaped verifies that reserved characters
// like semicolons and equals signs in file paths are percent-encoded in
// HTTP requests to the WebDAV server (RFC 3986 compliance).
func TestReservedCharactersInPathAreEscaped(t *testing.T) {
	var capturedPath string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.RequestURI
		// Return a 404 so the NewObject call fails cleanly
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	}

	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	// Try to access a file with a semicolon in the name.
	// We expect the request to fail (404), but the path should be escaped.
	_, _ = f.NewObject(context.Background(), "my;test")

	// The semicolon must be percent-encoded as %3B
	assert.Contains(t, capturedPath, "my%3Btest", "semicolons in path should be percent-encoded")
	assert.NotContains(t, capturedPath, "my;test", "raw semicolons should not appear in path")
}

const fileInfoResponse = `<d:multistatus xmlns:d="DAV:">
<d:response>
 <d:href>/file.txt</d:href>
 <d:propstat>
  <d:prop>
   <d:getcontentlength>10</d:getcontentlength>
   <d:resourcetype/>
  </d:prop>
  <d:status>HTTP/1.1 200 OK</d:status>
 </d:propstat>
</d:response>
</d:multistatus>`

func prepareFileObject(ctx context.Context, t *testing.T, getHandler http.HandlerFunc) (fs.Object, func()) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(http.StatusMultiStatus)
			_, err := fmt.Fprint(w, fileInfoResponse)
			require.NoError(t, err)
			return
		}
		if r.Method == http.MethodGet {
			getHandler(w, r)
			return
		}
		http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
	})
	ts := httptest.NewServer(handler)

	configfile.Install()
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	})
	require.NoError(t, err)
	o, err := f.NewObject(ctx, "file.txt")
	require.NoError(t, err)
	return o, ts.Close
}

func TestOpenDoesNotRetryIgnoredRange(t *testing.T) {
	var getRequests atomic.Int32
	o, tidy := prepareFileObject(context.Background(), t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=2-4", r.Header.Get("Range"))
		getRequests.Add(1)
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, "abcdefghij")
		require.NoError(t, err)
	})
	defer tidy()

	in, err := o.Open(context.Background(), &fs.RangeOption{Start: 2, End: 4})
	assert.Nil(t, in)
	assert.ErrorIs(t, err, fs.ErrorRangeIgnored)
	assert.Equal(t, int32(1), getRequests.Load())
}

func TestOpenRetriesMismatchedContentRange(t *testing.T) {
	var getRequests atomic.Int32
	o, tidy := prepareFileObject(context.Background(), t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=2-4", r.Header.Get("Range"))
		if getRequests.Add(1) == 1 {
			w.Header().Set("Content-Length", "3")
			w.Header().Set("Content-Range", "bytes 0-2/10")
			w.WriteHeader(http.StatusPartialContent)
			_, err := io.WriteString(w, "abc")
			require.NoError(t, err)
			return
		}
		w.Header().Set("Content-Length", "3")
		w.Header().Set("Content-Range", "bytes 2-4/10")
		w.WriteHeader(http.StatusPartialContent)
		_, err := io.WriteString(w, "cde")
		require.NoError(t, err)
	})
	defer tidy()

	in, err := o.Open(context.Background(), &fs.RangeOption{Start: 2, End: 4})
	require.NoError(t, err)
	defer func() { require.NoError(t, in.Close()) }()
	contents, err := io.ReadAll(in)
	require.NoError(t, err)
	assert.Equal(t, "cde", string(contents))
	assert.Equal(t, int32(2), getRequests.Load())
}

func TestCopyFallsBackWhenRangeIgnored(t *testing.T) {
	var rangeRequests atomic.Int32
	var fullRequests atomic.Int32
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 2
	ci.MultiThreadStreams = 2
	ci.MultiThreadSet = true
	ci.MultiThreadCutoff = 1
	ci.MultiThreadChunkSize = 4

	src, tidy := prepareFileObject(ctx, t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "" {
			fullRequests.Add(1)
		} else {
			rangeRequests.Add(1)
		}
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, "abcdefghij")
		require.NoError(t, err)
	})
	defer tidy()

	dstFs, err := local.NewFs(ctx, "local", t.TempDir(), configmap.Simple{
		"no_preallocate": "true",
		"no_sparse":      "true",
	})
	require.NoError(t, err)
	dst, err := operations.Copy(ctx, dstFs, nil, "file.txt", src)
	require.NoError(t, err)

	in, err := dst.Open(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, in.Close()) }()
	contents, err := io.ReadAll(in)
	require.NoError(t, err)
	assert.Equal(t, "abcdefghij", string(contents))
	assert.Positive(t, rangeRequests.Load())
	assert.LessOrEqual(t, rangeRequests.Load(), int32(3))
	assert.Equal(t, int32(1), fullRequests.Load())
}

// TestListAllRetryDoesNotConcatenate verifies that when a listing PROPFIND
// fails after having partially decoded and is retried, the final listing is
// exactly the retried response, with no entries carried over from the
// failed attempt.
func TestListAllRetryDoesNotConcatenate(t *testing.T) {
	entryXML := func(i int) string {
		name := fmt.Sprintf("file-%03d.bin", i)
		return "<d:response><d:href>/" + name + "</d:href><d:propstat><d:prop>" +
			"<d:displayname>" + name + "</d:displayname><d:getcontentlength>1024</d:getcontentlength>" +
			"</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>\n"
	}
	head := `<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:">`
	self := head + `<d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection xmlns:d="DAV:"/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`
	writeEntries := func(w http.ResponseWriter, from, to int) {
		_, err := fmt.Fprint(w, head)
		require.NoError(t, err)
		for i := from; i <= to; i++ {
			_, err := fmt.Fprint(w, entryXML(i))
			require.NoError(t, err)
		}
	}

	var listCalls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PROPFIND", r.Method)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		if r.Header.Get("Depth") == "0" {
			w.WriteHeader(207)
			_, err := fmt.Fprint(w, self)
			require.NoError(t, err)
			return
		}
		if listCalls.Add(1) == 1 {
			// return file-000..file-010 but kill the response before it
			// completes, so the client has parsed those entries when it
			// hits an unexpected EOF
			w.Header().Set("Content-Length", "1000000")
			w.WriteHeader(207)
			writeEntries(w, 0, 10)
			return
		}
		// return the disjoint file-011..file-030 as a complete listing
		w.WriteHeader(207)
		writeEntries(w, 11, 30)
		_, err := fmt.Fprint(w, "</d:multistatus>")
		require.NoError(t, err)
	}))
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	}
	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	var remotes []string
	for _, e := range entries {
		remotes = append(remotes, e.Remote())
	}
	var want []string
	for i := 11; i <= 30; i++ {
		want = append(want, fmt.Sprintf("file-%03d.bin", i))
	}
	assert.ElementsMatch(t, want, remotes)
}

// TestNotFoundInMultistatus checks that a missing path is detected when the
// server reports it by appending a Sabre NotFound error to a 207 response it
// has already started, as ownCloud 10.16 does, rather than by returning 404.
func TestNotFoundInMultistatus(t *testing.T) {
	head := `<?xml version="1.0"?>` + "\n" + `<d:multistatus xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns" xmlns:oc="http://owncloud.org/ns"`
	root := head + `><d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`
	notFound := head + `<?xml version="1.0" encoding="utf-8"?>
<d:error xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns">
  <s:exception>Sabre\DAV\Exception\NotFound</s:exception>
  <s:message>File with name missing could not be located</s:message>
</d:error>
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PROPFIND", r.Method)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		body := notFound
		if r.URL.Path == "/" {
			body = root
		}
		_, err := fmt.Fprint(w, body)
		require.NoError(t, err)
	}))
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	}
	ctx := context.Background()

	// A root which doesn't exist yet
	_, err := webdav.NewFs(ctx, remoteName, "missing", m)
	require.NoError(t, err)

	f, err := webdav.NewFs(ctx, remoteName, "", m)
	require.NoError(t, err)

	_, err = f.NewObject(ctx, "missing")
	assert.Equal(t, fs.ErrorObjectNotFound, err)

	_, err = f.List(ctx, "missing")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

// TestNextcloudPublicShareHeader checks X-Requested-With is sent for
// Nextcloud public shares unless the user overrides it.
func TestNextcloudPublicShareHeader(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		headers string
		want    string
	}{
		{"public share", "/public.php/dav/files/token", "", "XMLHttpRequest"},
		{"user override", "/public.php/dav/files/token", "X-Requested-With,custom", "custom"},
		{"regular nextcloud", "/remote.php/dav/files/user", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got atomic.Value
			got.Store("")
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got.Store(r.Header.Get("X-Requested-With"))
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = io.WriteString(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
			}))
			defer ts.Close()
			configfile.Install()
			m := configmap.Simple{
				"type":    "webdav",
				"url":     ts.URL + test.path,
				"vendor":  "nextcloud",
				"headers": test.headers,
			}
			f, err := webdav.NewFs(context.Background(), remoteName, "", m)
			require.NoError(t, err)
			_, _ = f.List(context.Background(), "")
			assert.Equal(t, test.want, got.Load().(string))
		})
	}
}
