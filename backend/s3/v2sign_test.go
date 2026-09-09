package s3

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalResourceBucket(t *testing.T) {
	for _, test := range []struct {
		name     string
		host     string
		endpoint string
		want     string
	}{
		// AWS, no endpoint configured
		{"aws virtual host", "bucket.s3.amazonaws.com", "", "bucket"},
		{"aws virtual host region", "bucket.s3.eu-west-1.amazonaws.com", "", "bucket"},
		{"aws virtual host dash region", "bucket.s3-eu-west-1.amazonaws.com", "", "bucket"},
		{"aws path style", "s3.amazonaws.com", "", ""},
		{"aws path style region", "s3.eu-west-1.amazonaws.com", "", ""},

		// Configured endpoint
		{"endpoint virtual host", "bucket.example.com", "https://example.com", "bucket"},
		{"endpoint path style", "example.com", "https://example.com", ""},
		{"endpoint with port", "bucket.example.com:9000", "http://example.com:9000", "bucket"},
		{"endpoint no scheme", "bucket.example.com", "example.com", "bucket"},
		{"endpoint with path", "bucket.example.com", "https://example.com/", "bucket"},

		// Bucket names may contain dots on a custom endpoint
		{"dotted bucket", "my.bucket.example.com", "https://example.com", "my.bucket"},

		// Nothing to do
		{"empty host", "", "", ""},
		{"unrecognised host", "localhost:8080", "", ""},
		{"host not under endpoint", "elsewhere.com", "https://example.com", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, canonicalResourceBucket(test.host, test.endpoint))
		})
	}
}

// The same bucket and key addressed either way signs identically, since v2
// signs "/bucket/key" in both cases.
func TestV2SignVirtualHostMatchesPathStyle(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "accesskey", SecretAccessKey: "secretkey"}
	signer := &v2Signer{opt: &Options{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
	}}

	sign := func(url string) (auth, date string) {
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		require.NoError(t, signer.SignHTTP(context.Background(), creds, req, "", "s3", "us-east-1", time.Now()))
		return req.Header.Get("Authorization"), req.Header.Get("Date")
	}

	// SignHTTP stamps Date from time.Now with second resolution, so retry if
	// the two land either side of a tick
	var pathAuth, hostAuth string
	for range 5 {
		var pathDate, hostDate string
		pathAuth, pathDate = sign("https://s3.amazonaws.com/bucket/some/key")
		hostAuth, hostDate = sign("https://bucket.s3.amazonaws.com/some/key")
		if pathDate == hostDate {
			break
		}
	}
	assert.Equal(t, pathAuth, hostAuth)
}

// A path style request already carries the bucket in its path, so nothing is
// prefixed.
func TestV2SignPathStyleUnchanged(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "accesskey", SecretAccessKey: "secretkey"}
	signer := &v2Signer{opt: &Options{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		Endpoint:        "https://example.com",
	}}
	req, err := http.NewRequest("GET", "https://example.com/bucket/some/key", nil)
	require.NoError(t, err)
	require.NoError(t, signer.SignHTTP(context.Background(), creds, req, "", "s3", "us-east-1", time.Now()))
	assert.Contains(t, req.Header.Get("Authorization"), "AWS accesskey:")
}
