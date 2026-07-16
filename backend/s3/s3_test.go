// Test S3 filesystem interface
package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func SetupS3Test(t *testing.T) (context.Context, *Options, *http.Client) {
	ctx, opt := context.Background(), new(Options)
	opt.Provider = "AWS"
	client := getClient(ctx, opt)
	return ctx, opt, client
}

// s3SecretTestHeaderNames is a deliberately literal copy of
// s3RedirectSecretHeaders: deriving the test inputs from the production list
// would make the redirect tests unable to detect a header missing from it.
// TestRedirectSecretHeadersMatchTestList keeps the two lists in sync.
var s3SecretTestHeaderNames = []string{
	"X-Amz-Security-Token",
	"X-Amz-S3session-Token",
	"Authorization",
	"ibm-service-instance-id",
	"X-Amz-Server-Side-Encryption-Customer-Algorithm",
	"X-Amz-Server-Side-Encryption-Customer-Key",
	"X-Amz-Server-Side-Encryption-Customer-Key-Md5",
	"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Algorithm",
	"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Key",
	"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Key-Md5",
	"Referer",
}

// TestRedirectSecretHeadersMatchTestList fails when a header is added to
// s3RedirectSecretHeaders without a matching literal entry in
// s3SecretTestHeaderNames (or vice versa), so every stripped header stays
// covered by the redirect tests.
func TestRedirectSecretHeadersMatchTestList(t *testing.T) {
	assert.ElementsMatch(t, s3RedirectSecretHeaders, s3SecretTestHeaderNames)
}

// s3SecretTestHeaders assigns each header a distinct test value
func s3SecretTestHeaders() map[string]string {
	headers := make(map[string]string, len(s3SecretTestHeaderNames))
	for _, header := range s3SecretTestHeaderNames {
		headers[header] = "secret-" + header
	}
	return headers
}

func TestClientRemovesSecretHeadersOnCrossHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)
	secretHeaders := s3SecretTestHeaders()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for header := range secretHeaders {
			assert.Empty(t, r.Header.Get(header), "%s should have been stripped", header)
		}
		assert.Equal(t, "date", r.Header.Get("X-Amz-Date"))
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectServer.Close()

	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectServer.URL, http.StatusTemporaryRedirect)
	}))
	defer initialServer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, initialServer.URL, nil)
	require.NoError(t, err)
	for header, value := range secretHeaders {
		req.Header.Set(header, value)
	}
	req.Header.Set("X-Amz-Date", "date")

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}

func TestClientDoesNotRestoreSecretHeadersAfterCrossHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)
	secretHeaders := s3SecretTestHeaders()

	assertStripped := func(r *http.Request) {
		for header := range secretHeaders {
			assert.Empty(t, r.Header.Get(header), "%s should have been stripped", header)
		}
		assert.Equal(t, "date", r.Header.Get("X-Amz-Date"))
	}

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/middle":
			assertStripped(r)
			http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
		case "/final":
			assertStripped(r)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer redirectServer.Close()

	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectServer.URL+"/middle", http.StatusTemporaryRedirect)
	}))
	defer initialServer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, initialServer.URL, nil)
	require.NoError(t, err)
	for header, value := range secretHeaders {
		req.Header.Set(header, value)
	}
	req.Header.Set("X-Amz-Date", "date")

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}

func TestClientKeepsSecretHeadersOnSameHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)
	secretHeaders := s3SecretTestHeaders()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
		case "/redirected":
			for header, value := range secretHeaders {
				assert.Equal(t, value, r.Header.Get(header), "%s should have been preserved", header)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	for header, value := range secretHeaders {
		req.Header.Set(header, value)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}

// TestClientRemovesGeneratedRefererOnCrossHostRedirect checks that the
// Referer header net/http generates automatically when following a redirect -
// which for a presigned request carries the signed query string - is not
// forwarded to a different host. The same-host hop first proves the client
// really does generate the Referer, so the cross-host assertion can't pass
// vacuously.
func TestClientRemovesGeneratedRefererOnCrossHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)

	crossHostServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Referer"), "Referer should have been stripped on cross-host redirect")
		w.WriteHeader(http.StatusOK)
	}))
	defer crossHostServer.Close()

	var presignedURL string
	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bucket/object":
			http.Redirect(w, r, "/middle", http.StatusTemporaryRedirect)
		case "/middle":
			assert.Equal(t, presignedURL, r.Header.Get("Referer"), "client should generate a Referer holding the presigned URL")
			http.Redirect(w, r, crossHostServer.URL, http.StatusTemporaryRedirect)
		default:
			http.NotFound(w, r)
		}
	}))
	defer initialServer.Close()
	presignedURL = initialServer.URL + "/bucket/object?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=secret-signature"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignedURL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}

func mustNewGet(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	return req
}

func TestS3CheckRedirectRejectsSchemeDowngrade(t *testing.T) {
	for _, test := range []struct {
		name string
		via  string
		req  string
	}{
		{"SameHostDowngrade", "https://bucket.example.com/", "http://bucket.example.com/redirected"},
		{"CrossHostDowngrade", "https://bucket.example.com/", "http://evil.example.com/redirected"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := s3CheckRedirect(mustNewGet(t, test.req), []*http.Request{mustNewGet(t, test.via)})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "HTTPS to HTTP")
		})
	}
}

func TestRedirectCrossesHost(t *testing.T) {
	mustReq := func(method, url string) *http.Request {
		req, err := http.NewRequest(method, url, nil)
		require.NoError(t, err)
		return req
	}
	for _, test := range []struct {
		name string
		via  []string
		req  string
		want bool
	}{
		{
			name: "SameHost",
			via:  []string{"https://bucket.example.com/"},
			req:  "https://bucket.example.com/redirected",
			want: false,
		},
		{
			name: "DifferentHost",
			via:  []string{"https://bucket.example.com/"},
			req:  "https://evil.example.com/redirected",
			want: true,
		},
		{
			name: "SchemeDowngradeSameHost",
			via:  []string{"https://bucket.example.com/"},
			req:  "http://bucket.example.com/redirected",
			want: true,
		},
		{
			name: "SchemeDowngradeMidChain",
			via:  []string{"https://bucket.example.com/", "http://bucket.example.com/middle"},
			req:  "http://bucket.example.com/final",
			want: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			via := make([]*http.Request, len(test.via))
			for i, url := range test.via {
				via[i] = mustReq(http.MethodGet, url)
			}
			got := s3RedirectCrossesHost(mustReq(http.MethodGet, test.req), via)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestClientStopsAfterTenRedirects(t *testing.T) {
	_, _, client := SetupS3Test(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	resp, err := client.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stopped after 10 redirects")
}

func TestObjectNotFoundMapping(t *testing.T) {
	ctx, opt, client := SetupS3Test(t)
	gotHead, gotGet := false, false

	// Return 404 for all requests.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			gotHead = true
		case http.MethodGet:
			gotGet = true
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	opt.Endpoint = server.URL
	opt.ForcePathStyle = true
	opt.Region = "us-east-1"
	opt.AccessKeyID = "id"
	opt.SecretAccessKey = "secret"
	c, _, err := s3Connection(ctx, opt, client)
	require.NoError(t, err)

	f := &Fs{
		name:  "s3test",
		opt:   *opt,
		ctx:   ctx,
		c:     c,
		pacer: fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep))),
		cache: bucket.NewCache(),
	}
	f.setRoot("bucket")

	// HEAD path: NewObject reads metadata via HeadObject.
	_, headErr := f.NewObject(ctx, "missing.txt")
	require.True(t, gotHead, "server should have received a HEAD request")
	assert.ErrorIs(t, headErr, fs.ErrorObjectNotFound)

	// GET path: Object.Open issues a GetObject.
	o := &Object{fs: f, remote: "missing.txt"}
	in, getErr := o.Open(ctx)
	if in != nil {
		_ = in.Close()
	}
	require.True(t, gotGet, "server should have received a GET request")
	assert.ErrorIs(t, getErr, fs.ErrorObjectNotFound)

	assert.Equal(t, headErr, getErr, "HeadObject and GetObject should map a 404 to the same error")
}

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	opt := &fstests.Opt{
		RemoteName:  "TestS3:",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"STANDARD"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
	}
	// Test wider range of tiers on AWS
	if *fstest.RemoteName == "" || *fstest.RemoteName == "TestS3:" {
		opt.TiersToTest = []string{"STANDARD", "STANDARD_IA"}
	}
	fstests.Run(t, opt)

}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	name := "TestS3"
	fstests.Run(t, &fstests.Opt{
		RemoteName:  name + ":",
		NilObject:   (*Object)(nil),
		TiersToTest: []string{"STANDARD", "STANDARD_IA"},
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "true"},
		},
	})
}

func TestAWSDualStackOption(t *testing.T) {
	{
		// test enabled
		ctx, opt, client := SetupS3Test(t)
		opt.UseDualStack = true
		s3Conn, _, err := s3Connection(ctx, opt, client)
		require.NoError(t, err)
		assert.Equal(t, aws.DualStackEndpointStateEnabled, s3Conn.Options().EndpointOptions.UseDualStackEndpoint)
	}
	{
		// test default case
		ctx, opt, client := SetupS3Test(t)
		s3Conn, _, err := s3Connection(ctx, opt, client)
		require.NoError(t, err)
		assert.Equal(t, aws.DualStackEndpointStateDisabled, s3Conn.Options().EndpointOptions.UseDualStackEndpoint)
	}
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

func (f *Fs) SetUploadCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadCutoff(cs)
}

func (f *Fs) SetCopyCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setCopyCutoff(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
	_ fstests.SetCopyCutoffer     = (*Fs)(nil)
)

func TestParseRetainUntilDate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		checkFunc func(t *testing.T, result time.Time)
	}{
		{
			name:    "RFC3339 date",
			input:   "2030-01-15T10:30:00Z",
			wantErr: false,
			checkFunc: func(t *testing.T, result time.Time) {
				expected, _ := time.Parse(time.RFC3339, "2030-01-15T10:30:00Z")
				assert.Equal(t, expected, result)
			},
		},
		{
			name:    "RFC3339 date with timezone",
			input:   "2030-06-15T10:30:00+02:00",
			wantErr: false,
			checkFunc: func(t *testing.T, result time.Time) {
				expected, _ := time.Parse(time.RFC3339, "2030-06-15T10:30:00+02:00")
				assert.Equal(t, expected, result)
			},
		},
		{
			name:    "duration days",
			input:   "365d",
			wantErr: false,
			checkFunc: func(t *testing.T, result time.Time) {
				expected := now.Add(365 * 24 * time.Hour)
				diff := result.Sub(expected)
				assert.Less(t, diff.Abs(), 2*time.Second, "result should be ~365 days from now")
			},
		},
		{
			name:    "duration hours",
			input:   "24h",
			wantErr: false,
			checkFunc: func(t *testing.T, result time.Time) {
				expected := now.Add(24 * time.Hour)
				diff := result.Sub(expected)
				assert.Less(t, diff.Abs(), 2*time.Second, "result should be ~24 hours from now")
			},
		},
		{
			name:    "duration minutes",
			input:   "30m",
			wantErr: false,
			checkFunc: func(t *testing.T, result time.Time) {
				expected := now.Add(30 * time.Minute)
				diff := result.Sub(expected)
				assert.Less(t, diff.Abs(), 2*time.Second, "result should be ~30 minutes from now")
			},
		},
		{
			name:    "invalid input",
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseRetainUntilDate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}
