// OSS V4 signing (OSS4-HMAC-SHA256) for Alibaba Cloud OSS
//
// Implemented from
// https://www.alibabacloud.com/help/en/oss/developer-reference/recommend-to-use-signature-version-4

package s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const (
	ossAlgorithm       = "OSS4-HMAC-SHA256"
	ossUnsignedPayload = "UNSIGNED-PAYLOAD"
	ossAmzPrefix       = "x-amz-"
	ossPrefix          = "x-oss-"
)

// ossV4Signer signs requests with OSS native V4 signatures
// (OSS4-HMAC-SHA256) as required by Alibaba Cloud OSS endpoints which
// don't accept AWS style signatures.
//
// Implements the HTTPSignerV4 interface.
type ossV4Signer struct {
	region       string // signing region for the credential scope
	product      string // signing product - "oss" or "oss-cloudbox"
	endpointHost string // host of the endpoint to detect virtual hosted style requests
}

// newOSSV4Signer creates an OSS V4 signer from the options.
//
// The signing region is derived from the endpoint if possible,
// otherwise the region option is used.
func newOSSV4Signer(opt *Options) *ossV4Signer {
	host := ossEndpointHost(opt.Endpoint)
	region, product := ossRegionProduct(host)
	if region == "" {
		region = opt.Region
	}
	return &ossV4Signer{
		region:       region,
		product:      product,
		endpointHost: host,
	}
}

// ossEndpointHost returns the lower case host part of an endpoint
// which may or may not have a scheme, or "" if it can't be parsed.
func ossEndpointHost(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// ossRegionProduct derives the signing region and product from an OSS
// endpoint host.
//
// Standard endpoints look like oss-<region>[-internal].aliyuncs.com
// and CloudBox endpoints look like
// <cb-id>.<region>.oss-cloudbox[-internal].aliyuncs.com where the
// signing region is the CloudBox ID.
//
// It returns region == "" if the region could not be derived, eg for
// accelerate endpoints or custom domains.
func ossRegionProduct(host string) (region, product string) {
	product = "oss"
	labels := strings.Split(host, ".")
	for _, label := range labels[1:] {
		if label == "oss-cloudbox" || label == "oss-cloudbox-internal" {
			return labels[0], "oss-cloudbox"
		}
	}
	if r, ok := strings.CutPrefix(labels[0], "oss-"); ok {
		r = strings.TrimSuffix(r, "-internal")
		if r != "" && !strings.HasPrefix(r, "accelerate") {
			region = r
		}
	}
	return region, product
}

// ossStorageClassToOSS maps AWS style storage class names (and upper
// cased native names as produced by SetTier) to the case sensitive
// native OSS names. Native names pass through unchanged.
var ossStorageClassToOSS = map[string]string{
	"STANDARD":        "Standard",
	"STANDARD_IA":     "IA",
	"GLACIER":         "Archive",
	"DEEP_ARCHIVE":    "ColdArchive",
	"ARCHIVE":         "Archive",
	"COLDARCHIVE":     "ColdArchive",
	"DEEPCOLDARCHIVE": "DeepColdArchive",
}

// ossStorageClassFromOSS maps native OSS storage class names back to
// the AWS style names the S3 compatible interface uses.
var ossStorageClassFromOSS = map[string]string{
	"Standard":    "STANDARD",
	"IA":          "STANDARD_IA",
	"Archive":     "GLACIER",
	"ColdArchive": "DEEP_ARCHIVE",
}

// ossHeaderRename returns the x-oss-* header equivalent to the
// x-amz-* header with the given suffix (the part after "x-amz-",
// lower case). hasKey should be true if the request refers to an
// object rather than a bucket.
//
// It returns "" for headers which should be dropped because the
// signer sets the x-oss-* equivalent itself.
func ossHeaderRename(amzSuffix string, hasKey bool) string {
	switch amzSuffix {
	case "date", "content-sha256", "security-token":
		return ""
	case "acl":
		// OSS uses different headers for object and bucket ACLs
		if hasKey {
			return "x-oss-object-acl"
		}
		return "x-oss-acl"
	case "server-side-encryption-aws-kms-key-id":
		return "x-oss-server-side-encryption-key-id"
	}
	return ossPrefix + amzSuffix
}

// ossTranslateRequest renames the x-amz-* headers the SDK generates
// into the x-oss-* equivalents that OSS uses so that they take part
// in the signature. OSS requires all x-oss-* headers sent to be
// signed.
func ossTranslateRequest(req *http.Request, hasKey bool) {
	var keys []string
	for k := range req.Header {
		if strings.HasPrefix(strings.ToLower(k), ossAmzPrefix) {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		values := req.Header.Values(k)
		req.Header.Del(k)
		newKey := ossHeaderRename(strings.ToLower(k)[len(ossAmzPrefix):], hasKey)
		if newKey == "" {
			continue
		}
		switch newKey {
		case "x-oss-copy-source":
			// OSS wants the copy source to start with a "/"
			for i, v := range values {
				if !strings.HasPrefix(v, "/") {
					values[i] = "/" + v
				}
			}
		case "x-oss-storage-class":
			// OSS uses different case sensitive storage class names
			for i, v := range values {
				if native, ok := ossStorageClassToOSS[v]; ok {
					values[i] = native
				}
			}
		}
		for _, v := range values {
			req.Header.Add(newKey, v)
		}
	}
}

// ossCanonicalURI returns the canonical URI for the request,
// expanding a virtual hosted style bucket in the host back to
// /bucket/key form.
func (s *ossV4Signer) ossCanonicalURI(u *url.URL) string {
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	host := strings.ToLower(u.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if s.endpointHost != "" && strings.HasSuffix(host, "."+s.endpointHost) {
		bucket := strings.TrimSuffix(host, "."+s.endpointHost)
		path = "/" + bucket + path
	}
	return path
}

// ossCanonicalQuery returns the canonical query string for the
// request. This signs the query parameters exactly as they are sent
// on the wire except that "+" is re-encoded as %20 and the
// parameters are sorted. Parameters with an empty value are signed
// as a bare name without "=".
func ossCanonicalQuery(rawQuery string) string {
	query := strings.ReplaceAll(rawQuery, "+", "%20")
	type param struct{ name, value string }
	var params []param
	for _, part := range strings.Split(query, "&") {
		if part == "" {
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		params = append(params, param{name, value})
	}
	sort.Slice(params, func(i, j int) bool {
		if params[i].name != params[j].name {
			return params[i].name < params[j].name
		}
		return params[i].value < params[j].value
	})
	var b strings.Builder
	for i, p := range params {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.name)
		if p.value != "" {
			b.WriteByte('=')
			b.WriteString(p.value)
		}
	}
	return b.String()
}

// ossCanonicalHeaders returns the canonical headers for the request.
// OSS signs all x-oss-* headers plus content-type and content-md5.
func ossCanonicalHeaders(header http.Header) string {
	var names []string
	values := make(map[string]string)
	for k, vs := range header {
		name := strings.ToLower(k)
		if name != "content-type" && name != "content-md5" && !strings.HasPrefix(name, ossPrefix) {
			continue
		}
		trimmed := make([]string, len(vs))
		for i, v := range vs {
			trimmed[i] = strings.TrimSpace(v)
		}
		names = append(names, name)
		values[name] = strings.Join(trimmed, ",")
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(values[name])
		b.WriteByte('\n')
	}
	return b.String()
}

// ossHMAC returns the HMAC-SHA256 of value with the given key
func ossHMAC(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(value))
	return h.Sum(nil)
}

// SignHTTP signs the request with an OSS V4 signature.
//
// It rewrites the AWS style x-amz-* headers into their x-oss-*
// equivalents before signing.
//
// Implements the HTTPSignerV4 interface.
func (s *ossV4Signer) SignHTTP(ctx context.Context, credentials aws.Credentials, req *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*v4signer.SignerOptions)) error {
	uri := s.ossCanonicalURI(req.URL)
	// an object request has a path after the /bucket/ prefix
	hasKey := strings.Contains(strings.Trim(uri, "/"), "/")
	ossTranslateRequest(req, hasKey)

	utcTime := signingTime.UTC()
	datetime := utcTime.Format("20060102T150405Z")
	date := utcTime.Format("20060102")
	req.Header.Set("Date", utcTime.Format(http.TimeFormat))
	req.Header.Set("x-oss-date", datetime)
	// OSS only supports unsigned payloads with V4 signatures
	req.Header.Set("x-oss-content-sha256", ossUnsignedPayload)
	if credentials.SessionToken != "" {
		req.Header.Set("x-oss-security-token", credentials.SessionToken)
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		uri,
		ossCanonicalQuery(req.URL.RawQuery),
		ossCanonicalHeaders(req.Header),
		"", // no additional signed headers
		ossUnsignedPayload,
	}, "\n")

	hash := sha256.Sum256([]byte(canonicalRequest))
	scope := date + "/" + s.region + "/" + s.product + "/aliyun_v4_request"
	stringToSign := ossAlgorithm + "\n" + datetime + "\n" + scope + "\n" + hex.EncodeToString(hash[:])

	key := ossHMAC([]byte("aliyun_v4"+credentials.SecretAccessKey), date)
	key = ossHMAC(key, s.region)
	key = ossHMAC(key, s.product)
	key = ossHMAC(key, "aliyun_v4_request")
	signature := hex.EncodeToString(ossHMAC(key, stringToSign))

	req.Header.Set("Authorization", ossAlgorithm+" Credential="+credentials.AccessKeyID+"/"+scope+",Signature="+signature)
	return nil
}

// ossTranslateResponse copies x-oss-* response headers to their
// x-amz-* equivalents so the SDK can parse them, eg for user
// metadata, version IDs and storage class.
func ossTranslateResponse(header http.Header) {
	additions := make(map[string][]string)
	for k, vs := range header {
		suffix, ok := strings.CutPrefix(strings.ToLower(k), ossPrefix)
		if !ok {
			continue
		}
		amzKey := ossAmzPrefix + suffix
		switch suffix {
		case "server-side-encryption-key-id":
			amzKey = "x-amz-server-side-encryption-aws-kms-key-id"
		case "storage-class":
			// OSS uses different case sensitive storage class names
			mapped := make([]string, len(vs))
			for i, v := range vs {
				if aws, ok := ossStorageClassFromOSS[v]; ok {
					mapped[i] = aws
				} else {
					mapped[i] = v
				}
			}
			vs = mapped
		}
		canonical := textproto.CanonicalMIMEHeaderKey(amzKey)
		if _, exists := header[canonical]; !exists {
			additions[canonical] = vs
		}
	}
	for k, vs := range additions {
		header[k] = vs
	}
}

// ossTransport is an http.RoundTripper which makes OSS responses
// look like S3 responses to the SDK.
type ossTransport struct {
	base http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (t *ossTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if resp != nil {
		ossTranslateResponse(resp.Header)
	}
	return resp, err
}

// ossFixupResponses returns a copy of client whose responses have
// their x-oss-* headers copied to the x-amz-* equivalents.
func ossFixupResponses(client *http.Client) *http.Client {
	newClient := *client
	if newClient.Transport == nil {
		newClient.Transport = http.DefaultTransport
	}
	newClient.Transport = &ossTransport{base: newClient.Transport}
	return &newClient
}
