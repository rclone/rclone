// v2 signing

package s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// URL parameters that need to be added to the signature
var s3ParamsToSign = map[string]struct{}{
	"delete":                       {},
	"acl":                          {},
	"location":                     {},
	"logging":                      {},
	"notification":                 {},
	"partNumber":                   {},
	"policy":                       {},
	"requestPayment":               {},
	"torrent":                      {},
	"uploadId":                     {},
	"uploads":                      {},
	"versionId":                    {},
	"versioning":                   {},
	"versions":                     {},
	"response-content-type":        {},
	"response-content-language":    {},
	"response-expires":             {},
	"response-cache-control":       {},
	"response-content-disposition": {},
	"response-content-encoding":    {},
}

// Implement HTTPSignerV4 interface
type v2Signer struct {
	opt *Options
}

// canonicalResourceBucket returns the bucket named by host, or "" if host
// addresses the endpoint directly and the bucket is already in the path.
//
// v2 signs "/bucket/key" regardless of addressing style, so with virtual host
// style the bucket has to come from the Host header. endpoint may be empty,
// in which case host is assumed to be AWS.
//
// See https://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html
func canonicalResourceBucket(host, endpoint string) string {
	if host == "" {
		return ""
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	base := endpoint
	if base != "" {
		// Strip the scheme and any port to get the endpoint host
		if i := strings.Index(base, "://"); i >= 0 {
			base = base[i+3:]
		}
		base = strings.TrimSuffix(base, "/")
		if i := strings.IndexByte(base, '/'); i >= 0 {
			base = base[:i]
		}
		if i := strings.IndexByte(base, ':'); i >= 0 {
			base = base[:i]
		}
	} else {
		// AWS hosts are bucket.s3.amazonaws.com or
		// bucket.s3.region.amazonaws.com, so the s3 label starts the endpoint
		labels := strings.Split(host, ".")
		for i, label := range labels {
			if label == "s3" || strings.HasPrefix(label, "s3-") {
				base = strings.Join(labels[i:], ".")
				break
			}
		}
		if base == "" {
			return ""
		}
	}
	if !strings.HasSuffix(host, "."+base) {
		return ""
	}
	return strings.TrimSuffix(host, "."+base)
}

// SignHTTP signs requests using v2 auth.
//
// Cobbled together from goamz and aws-sdk-go.
//
// Bodged up to compile with AWS SDK v2
func (v2 *v2Signer) SignHTTP(ctx context.Context, credentials aws.Credentials, req *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*v4signer.SignerOptions)) error {
	// Set date
	date := time.Now().UTC().Format(time.RFC1123)
	req.Header.Set("Date", date)

	// Sort out URI
	uri := req.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}
	// v2 signs the bucket as part of the resource, but with virtual host
	// style addressing it is in the Host header rather than the path
	if bucket := canonicalResourceBucket(req.URL.Host, v2.opt.Endpoint); bucket != "" {
		uri = "/" + bucket + uri
	}

	// Look through headers of interest
	var md5 string
	var contentType string
	var headersToSign []string
	tmpHeadersToSign := make(map[string][]string)
	for k, v := range req.Header {
		k = strings.ToLower(k)
		switch k {
		case "content-md5":
			md5 = v[0]
		case "content-type":
			contentType = v[0]
		default:
			if strings.HasPrefix(k, "x-amz-") {
				tmpHeadersToSign[k] = v
			}
		}
	}
	var keys []string
	for k := range tmpHeadersToSign {
		keys = append(keys, k)
	}
	// https://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html
	sort.Strings(keys)

	for _, key := range keys {
		vall := strings.Join(tmpHeadersToSign[key], ",")
		headersToSign = append(headersToSign, key+":"+vall)
	}
	// Make headers of interest into canonical string
	var joinedHeadersToSign string
	if len(headersToSign) > 0 {
		joinedHeadersToSign = strings.Join(headersToSign, "\n") + "\n"
	}

	// Look for query parameters which need to be added to the signature
	params := req.URL.Query()
	var queriesToSign []string
	for k, vs := range params {
		if _, ok := s3ParamsToSign[k]; ok {
			for _, v := range vs {
				if v == "" {
					queriesToSign = append(queriesToSign, k)
				} else {
					queriesToSign = append(queriesToSign, k+"="+v)
				}
			}
		}
	}
	// Add query parameters to URI
	if len(queriesToSign) > 0 {
		sort.StringSlice(queriesToSign).Sort()
		uri += "?" + strings.Join(queriesToSign, "&")
	}

	// Make signature
	payload := req.Method + "\n" + md5 + "\n" + contentType + "\n" + date + "\n" + joinedHeadersToSign + uri
	hash := hmac.New(sha1.New, []byte(v2.opt.SecretAccessKey))
	_, _ = hash.Write([]byte(payload))
	signature := make([]byte, base64.StdEncoding.EncodedLen(hash.Size()))
	base64.StdEncoding.Encode(signature, hash.Sum(nil))

	// Set signature in request
	req.Header.Set("Authorization", "AWS "+v2.opt.AccessKeyID+":"+string(signature))
	return nil
}
