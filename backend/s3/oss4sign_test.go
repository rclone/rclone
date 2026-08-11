package s3

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ossV4TestRequest makes the test request used by the signing test
// vectors from the official OSS Go SDK:
// https://github.com/aliyun/alibabacloud-oss-go-sdk-v2/blob/master/oss/signer/signer_test.go
func ossV4TestRequest(t *testing.T, host string) *http.Request {
	req, err := http.NewRequest("PUT", "http://"+host+"/1234%2B-/123/1.txt", nil)
	require.NoError(t, err)
	req.Header.Add("x-oss-head1", "value")
	req.Header.Add("abc", "value")
	req.Header.Add("ZAbc", "value")
	req.Header.Add("XYZ", "value")
	req.Header.Add("content-type", "text/plain")
	values := url.Values{}
	values.Add("param1", "value1")
	values.Add("+param1", "value3")
	values.Add("|param1", "value4")
	values.Add("+param2", "")
	values.Add("|param2", "")
	values.Add("param2", "")
	req.URL.RawQuery = values.Encode()
	return req
}

func TestOSSV4SignHTTP(t *testing.T) {
	signer := &ossV4Signer{
		region:       "cn-hangzhou",
		product:      "oss",
		endpointHost: "oss-cn-hangzhou.aliyuncs.com",
	}
	req := ossV4TestRequest(t, "bucket.oss-cn-hangzhou.aliyuncs.com")
	creds := aws.Credentials{AccessKeyID: "ak", SecretAccessKey: "sk"}
	err := signer.SignHTTP(context.Background(), creds, req, "", "s3", "us-east-1", time.Unix(1702743657, 0))
	require.NoError(t, err)
	assert.Equal(t, "OSS4-HMAC-SHA256 Credential=ak/20231216/cn-hangzhou/oss/aliyun_v4_request,Signature=e21d18daa82167720f9b1047ae7e7f1ce7cb77a31e8203a7d5f4624fa0284afe", req.Header.Get("Authorization"))
	assert.Equal(t, time.Unix(1702743657, 0).UTC().Format("20060102T150405Z"), req.Header.Get("x-oss-date"))
	assert.Equal(t, "UNSIGNED-PAYLOAD", req.Header.Get("x-oss-content-sha256"))
}

func TestOSSV4SignHTTPToken(t *testing.T) {
	signer := &ossV4Signer{
		region:       "cn-hangzhou",
		product:      "oss",
		endpointHost: "oss-cn-hangzhou.aliyuncs.com",
	}
	req := ossV4TestRequest(t, "bucket.oss-cn-hangzhou.aliyuncs.com")
	creds := aws.Credentials{AccessKeyID: "ak", SecretAccessKey: "sk", SessionToken: "token"}
	err := signer.SignHTTP(context.Background(), creds, req, "", "s3", "us-east-1", time.Unix(1702784856, 0))
	require.NoError(t, err)
	assert.Equal(t, "OSS4-HMAC-SHA256 Credential=ak/20231217/cn-hangzhou/oss/aliyun_v4_request,Signature=b94a3f999cf85bcdc00d332fbd3734ba03e48382c36fa4d5af5df817395bd9ea", req.Header.Get("Authorization"))
	assert.Equal(t, "token", req.Header.Get("x-oss-security-token"))
}

func TestOSSV4SignHTTPCloudBox(t *testing.T) {
	signer := &ossV4Signer{
		region:       "cb-123",
		product:      "oss-cloudbox",
		endpointHost: "cb-123.cn-hangzhou.oss-cloudbox.aliyuncs.com",
	}
	req := ossV4TestRequest(t, "bucket.cb-123.cn-hangzhou.oss-cloudbox.aliyuncs.com")
	creds := aws.Credentials{AccessKeyID: "ak", SecretAccessKey: "sk"}
	err := signer.SignHTTP(context.Background(), creds, req, "", "s3", "us-east-1", time.Unix(1702743657, 0))
	require.NoError(t, err)
	assert.Equal(t, "OSS4-HMAC-SHA256 Credential=ak/20231216/cb-123/oss-cloudbox/aliyun_v4_request,Signature=94ce1f12c17d148ea681030275a94449d3357f5b5b21133996eec80af3e08a43", req.Header.Get("Authorization"))
}

func TestOSSRegionProduct(t *testing.T) {
	for _, test := range []struct {
		host    string
		region  string
		product string
	}{
		{"oss-cn-hangzhou.aliyuncs.com", "cn-hangzhou", "oss"},
		{"oss-cn-hangzhou-internal.aliyuncs.com", "cn-hangzhou", "oss"},
		{"oss-cn-jinan-acdr-ut-1-internal.aliyuncs.com", "cn-jinan-acdr-ut-1", "oss"},
		{"oss-us-east-1.aliyuncs.com", "us-east-1", "oss"},
		{"oss-accelerate.aliyuncs.com", "", "oss"},
		{"oss-accelerate-overseas.aliyuncs.com", "", "oss"},
		{"cb-123.cn-hangzhou.oss-cloudbox.aliyuncs.com", "cb-123", "oss-cloudbox"},
		{"cb-123.cn-hangzhou.oss-cloudbox-internal.aliyuncs.com", "cb-123", "oss-cloudbox"},
		{"my.custom.domain.example.com", "", "oss"},
		{"", "", "oss"},
	} {
		region, product := ossRegionProduct(test.host)
		assert.Equal(t, test.region, region, test.host)
		assert.Equal(t, test.product, product, test.host)
	}
}

func TestOSSEndpointHost(t *testing.T) {
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", ossEndpointHost("oss-cn-hangzhou.aliyuncs.com"))
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", ossEndpointHost("https://oss-cn-hangzhou.aliyuncs.com"))
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", ossEndpointHost("http://OSS-cn-hangzhou.aliyuncs.com:8080"))
	assert.Equal(t, "", ossEndpointHost(""))
}

func TestOSSCanonicalURI(t *testing.T) {
	signer := &ossV4Signer{endpointHost: "oss-cn-hangzhou.aliyuncs.com"}
	for _, test := range []struct {
		url    string
		want   string
		hasKey bool
	}{
		// virtual hosted style
		{"https://bucket.oss-cn-hangzhou.aliyuncs.com/", "/bucket/", false},
		{"https://bucket.oss-cn-hangzhou.aliyuncs.com/key", "/bucket/key", true},
		{"https://bucket.oss-cn-hangzhou.aliyuncs.com/dir/key", "/bucket/dir/key", true},
		{"https://bucket.oss-cn-hangzhou.aliyuncs.com", "/bucket/", false},
		// path style
		{"https://oss-cn-hangzhou.aliyuncs.com/", "/", false},
		{"https://oss-cn-hangzhou.aliyuncs.com/bucket", "/bucket", false},
		{"https://oss-cn-hangzhou.aliyuncs.com/bucket/key", "/bucket/key", true},
	} {
		u, err := url.Parse(test.url)
		require.NoError(t, err)
		uri := signer.ossCanonicalURI(u)
		assert.Equal(t, test.want, uri, test.url)
		hasKey := strings.Contains(strings.Trim(uri, "/"), "/")
		assert.Equal(t, test.hasKey, hasKey, test.url)
	}
}

func TestOSSTranslateRequest(t *testing.T) {
	req, err := http.NewRequest("PUT", "https://bucket.oss-cn-hangzhou.aliyuncs.com/key", nil)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Meta-Mtime", "1400000000.000000")
	req.Header.Set("X-Amz-Copy-Source", "bucket/path/file")
	req.Header.Set("X-Amz-Acl", "private")
	req.Header.Set("X-Amz-Storage-Class", "GLACIER")
	req.Header.Set("X-Amz-Date", "junk")
	req.Header.Set("X-Amz-Content-Sha256", "junk")
	req.Header.Set("X-Amz-Security-Token", "junk")
	req.Header.Set("X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id", "keyid")
	req.Header.Set("Content-Type", "text/plain")
	ossTranslateRequest(req, true)
	assert.Equal(t, "1400000000.000000", req.Header.Get("x-oss-meta-mtime"))
	assert.Equal(t, "/bucket/path/file", req.Header.Get("x-oss-copy-source"))
	assert.Equal(t, "private", req.Header.Get("x-oss-object-acl"))
	assert.Equal(t, "Archive", req.Header.Get("x-oss-storage-class"))
	assert.Equal(t, "keyid", req.Header.Get("x-oss-server-side-encryption-key-id"))
	assert.Equal(t, "text/plain", req.Header.Get("Content-Type"))
	for k := range req.Header {
		assert.False(t, strings.HasPrefix(strings.ToLower(k), "x-amz-"), k)
	}

	// a copy source which already has a leading / is left alone
	req.Header.Set("X-Amz-Copy-Source", "/bucket/path/file")
	ossTranslateRequest(req, true)
	assert.Equal(t, "/bucket/path/file", req.Header.Get("x-oss-copy-source"))

	// storage class names are mapped to the native OSS names
	for _, test := range []struct{ in, want string }{
		{"STANDARD", "Standard"},
		{"STANDARD_IA", "IA"},
		{"GLACIER", "Archive"},
		{"DEEP_ARCHIVE", "ColdArchive"},
		{"DEEPCOLDARCHIVE", "DeepColdArchive"},
		{"Standard", "Standard"},
		{"IA", "IA"},
		{"unknown", "unknown"},
	} {
		req.Header.Del("x-oss-storage-class")
		req.Header.Set("X-Amz-Storage-Class", test.in)
		ossTranslateRequest(req, true)
		assert.Equal(t, test.want, req.Header.Get("x-oss-storage-class"), test.in)
	}

	// bucket level requests use x-oss-acl
	req, err = http.NewRequest("PUT", "https://bucket.oss-cn-hangzhou.aliyuncs.com/", nil)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Acl", "private")
	ossTranslateRequest(req, false)
	assert.Equal(t, "private", req.Header.Get("x-oss-acl"))
}

func TestOSSTranslateResponse(t *testing.T) {
	header := http.Header{}
	header.Set("X-Oss-Meta-Mtime", "1400000000.000000")
	header.Set("X-Oss-Request-Id", "1234")
	header.Set("X-Oss-Version-Id", "abcd")
	header.Set("X-Oss-Storage-Class", "Standard")
	header.Set("X-Oss-Server-Side-Encryption-Key-Id", "keyid")
	header.Set("Etag", `"abc"`)
	ossTranslateResponse(header)
	assert.Equal(t, "1400000000.000000", header.Get("x-amz-meta-mtime"))
	assert.Equal(t, "1234", header.Get("x-amz-request-id"))
	assert.Equal(t, "abcd", header.Get("x-amz-version-id"))
	assert.Equal(t, "STANDARD", header.Get("x-amz-storage-class"))
	assert.Equal(t, "keyid", header.Get("x-amz-server-side-encryption-aws-kms-key-id"))
	// the x-oss- headers are left in place
	assert.Equal(t, "1234", header.Get("x-oss-request-id"))
	// an existing x-amz- header is not overwritten
	header = http.Header{}
	header.Set("X-Oss-Meta-Mtime", "1")
	header.Set("X-Amz-Meta-Mtime", "2")
	ossTranslateResponse(header)
	assert.Equal(t, "2", header.Get("x-amz-meta-mtime"))
}
