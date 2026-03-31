// Test S3 filesystem interface
package s3

import (
	"context"
	"net/http"
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
