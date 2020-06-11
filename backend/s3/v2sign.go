// v2 signing

package s3

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

// URL parameters that need to be added to the signature
var s3ParamsToSign = map[string]struct{}{
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
	"lifecycle":                    {},
	"website":                      {},
	"delete":                       {},
	"cors":                         {},
	"restore":                      {},
}

// Warn once about empty endpoint
var warnEmptyEndpointOnce sync.Once

// sign signs requests using v2 auth
//
// Cobbled together from goamz and aws-sdk-go
func v2sign(opt *Options, req *http.Request) {
	// Set date
	date := time.Now().UTC().Format(time.RFC1123)
	req.Header.Set("Date", date)

	// Sort out URI
	uri := req.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}
	// If not using path style then need to stick the bucket on
	// the start of the requests if doing a bucket based query
	if !opt.ForcePathStyle {
		if opt.Endpoint == "" {
			warnEmptyEndpointOnce.Do(func() {
				fs.Logf(nil, "If using v2 auth with AWS and force_path_style=false, endpoint must be set in the config")
			})
		} else if req.URL.Host != opt.Endpoint {
			// read the bucket off the start of the hostname
			i := strings.IndexRune(req.URL.Host, '.')
			if i >= 0 {
				uri = "/" + req.URL.Host[:i] + uri
			}
		}
	}

	// Look through headers of interest
	var md5 string
	var contentType string
	var headersToSign [][2]string // slice of key, value pairs
	for k, v := range req.Header {
		k = strings.ToLower(k)
		switch k {
		case "content-md5":
			md5 = v[0]
		case "content-type":
			contentType = v[0]
		default:
			if strings.HasPrefix(k, "x-amz-") {
				vall := strings.Join(v, ",")
				headersToSign = append(headersToSign, [2]string{k, vall})
			}
		}
	}
	// Make headers of interest into canonical string
	var joinedHeadersToSign string
	if len(headersToSign) > 0 {
		// sort by keys
		sort.Slice(headersToSign, func(i, j int) bool {
			return headersToSign[i][0] < headersToSign[j][0]
		})
		// join into key:value\n
		var out strings.Builder
		for _, kv := range headersToSign {
			out.WriteString(kv[0])
			out.WriteRune(':')
			out.WriteString(kv[1])
			out.WriteRune('\n')
		}
		joinedHeadersToSign = out.String()
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
	hash := hmac.New(sha1.New, []byte(opt.SecretAccessKey))
	_, _ = hash.Write([]byte(payload))
	signature := make([]byte, base64.StdEncoding.EncodedLen(hash.Size()))
	base64.StdEncoding.Encode(signature, hash.Sum(nil))

	// Set signature in request
	req.Header.Set("Authorization", "AWS "+opt.AccessKeyID+":"+string(signature))
}
