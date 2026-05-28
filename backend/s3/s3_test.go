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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func SetupS3Test(t *testing.T) (context.Context, *Options, *http.Client) {
	ctx, opt := context.Background(), new(Options)
	opt.Provider = "AWS"
	client := getClient(ctx, opt)
	return ctx, opt, client
}

func TestClientRemovesSecurityTokenOnCrossHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("X-Amz-Security-Token"))
		assert.Equal(t, "date", r.Header.Get("X-Amz-Date"))
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectServer.Close()

	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token", r.Header.Get("X-Amz-Security-Token"))
		http.Redirect(w, r, redirectServer.URL, http.StatusTemporaryRedirect)
	}))
	defer initialServer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, initialServer.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Security-Token", "token")
	req.Header.Set("X-Amz-Date", "date")

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}

func TestClientDoesNotRestoreSecurityTokenAfterCrossHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/middle":
			assert.Empty(t, r.Header.Get("X-Amz-Security-Token"))
			assert.Equal(t, "date", r.Header.Get("X-Amz-Date"))
			http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
		case "/final":
			assert.Empty(t, r.Header.Get("X-Amz-Security-Token"))
			assert.Equal(t, "date", r.Header.Get("X-Amz-Date"))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer redirectServer.Close()

	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token", r.Header.Get("X-Amz-Security-Token"))
		http.Redirect(w, r, redirectServer.URL+"/middle", http.StatusTemporaryRedirect)
	}))
	defer initialServer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, initialServer.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Security-Token", "token")
	req.Header.Set("X-Amz-Date", "date")

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}

func TestClientKeepsSecurityTokenOnSameHostRedirect(t *testing.T) {
	ctx, _, client := SetupS3Test(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			assert.Equal(t, "token", r.Header.Get("X-Amz-Security-Token"))
			http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
		case "/redirected":
			assert.Equal(t, "token", r.Header.Get("X-Amz-Security-Token"))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Security-Token", "token")

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
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
