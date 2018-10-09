// v2 signing

package s3

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"sort"
	"strings"
	"time"
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
}

// sign signs requests using v2 auth
//
// Cobbled together from goamz and aws-sdk-go
func sign(AccessKey, SecretKey string, req *http.Request) {
	// Set date
	date := time.Now().UTC().Format(time.RFC1123)
	req.Header.Set("Date", date)

	// Sort out URI
	uri := req.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}

	// Look through headers of interest
	var md5 string
	var contentType string
	var headersToSign []string
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
				headersToSign = append(headersToSign, k+":"+vall)
			}
		}
	}
	// Make headers of interest into canonical string
	var joinedHeadersToSign string
	if len(headersToSign) > 0 {
		sort.StringSlice(headersToSign).Sort()
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
	hash := hmac.New(sha1.New, []byte(SecretKey))
	_, _ = hash.Write([]byte(payload))
	signature := make([]byte, base64.StdEncoding.EncodedLen(hash.Size()))
	base64.StdEncoding.Encode(signature, hash.Sum(nil))

	// Set signature in request
	req.Header.Set("Authorization", "AWS "+AccessKey+":"+string(signature))
}
