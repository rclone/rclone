package signature

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ref: https://github.com/minio/minio/cmd/auth-handler.go

const (
	signV4Algorithm = "AWS4-HMAC-SHA256"
	iso8601Format   = "20060102T150405Z"
	yyyymmdd        = "20060102"
	serviceS3       = "s3"
	slashSeparator  = "/"
	stype           = serviceS3

	headerAuth       = "Authorization"
	headerDate       = "Date"
	amzContentSha256 = "X-Amz-Content-Sha256"
	amzDate          = "X-Amz-Date"
)

// getCanonicalHeaders generate a list of request headers with their values
func getCanonicalHeaders(signedHeaders http.Header) string {
	var headers []string
	vals := make(http.Header)
	for k, vv := range signedHeaders {
		headers = append(headers, strings.ToLower(k))
		vals[strings.ToLower(k)] = vv
	}
	sort.Strings(headers)

	var buf bytes.Buffer
	for _, k := range headers {
		buf.WriteString(k)
		buf.WriteByte(':')
		for idx, v := range vals[k] {
			if idx > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(signV4TrimAll(v))
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

// getSignedHeaders generate a string i.e alphabetically sorted, semicolon-separated list of lowercase request header names
func getSignedHeaders(signedHeaders http.Header) string {
	var headers []string
	for k := range signedHeaders {
		headers = append(headers, strings.ToLower(k))
	}
	sort.Strings(headers)
	return strings.Join(headers, ";")
}

// compareSignatureV4 returns true if and only if both signatures
// are equal. The signatures are expected to be HEX encoded strings
// according to the AWS S3 signature V4 spec.
func compareSignatureV4(sig1, sig2 string) bool {
	// The CTC using []byte(str) works because the hex encoding
	// is unique for a sequence of bytes. See also compareSignatureV2.
	return subtle.ConstantTimeCompare([]byte(sig1), []byte(sig2)) == 1
}

// getSignature final signature in hexadecimal form.
func getSignature(signingKey []byte, stringToSign string) string {
	return hex.EncodeToString(sumHMAC(signingKey, []byte(stringToSign)))
}

// Trim leading and trailing spaces and replace sequential spaces with one space, following Trimall()
// in http://docs.aws.amazon.com/general/latest/gr/sigv4-create-canonical-request.html
func signV4TrimAll(input string) string {
	// Compress adjacent spaces (a space is determined by
	// unicode.IsSpace() internally here) to one space and return
	return strings.Join(strings.Fields(input), " ")
}

// getCanonicalRequest generate a canonical request of style
//
// canonicalRequest =
//
//	<HTTPMethod>\n
//	<CanonicalURI>\n
//	<CanonicalQueryString>\n
//	<CanonicalHeaders>\n
//	<SignedHeaders>\n
//	<HashedPayload>
func getCanonicalRequest(extractedSignedHeaders http.Header, payload, queryStr, urlPath, method string) string {
	rawQuery := strings.ReplaceAll(queryStr, "+", "%20")
	encodedPath := encodePath(urlPath)
	canonicalRequest := strings.Join([]string{
		method,
		encodedPath,
		rawQuery,
		getCanonicalHeaders(extractedSignedHeaders),
		getSignedHeaders(extractedSignedHeaders),
		payload,
	}, "\n")
	return canonicalRequest
}

// getStringToSign a string based on selected query values.
func getStringToSign(canonicalRequest string, t time.Time, scope string) string {
	stringToSign := signV4Algorithm + "\n" + t.Format(iso8601Format) + "\n"
	stringToSign += scope + "\n"
	canonicalRequestBytes := sha256.Sum256([]byte(canonicalRequest))
	stringToSign += hex.EncodeToString(canonicalRequestBytes[:])
	return stringToSign
}

// getSigningKey hmac seed to calculate final signature.
func getSigningKey(secretKey string, t time.Time, region string) []byte {
	date := sumHMAC([]byte("AWS4"+secretKey), []byte(t.Format(yyyymmdd)))
	regionBytes := sumHMAC(date, []byte(region))
	service := sumHMAC(regionBytes, []byte(stype))
	signingKey := sumHMAC(service, []byte("aws4_request"))
	return signingKey
}

// Verify - Verify authorization header with calculated header in accordance with
//   - http://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-authenticating-requests.html
//
// returns nil if signature matches.
func Verify(r *http.Request) ErrorCode {
	// Copy request.
	req := *r
	hashedPayload := getContentSha256Cksum(r)

	// Save authorization header.
	v4Auth := req.Header.Get(headerAuth)

	// Parse signature version '4' header.
	signV4Values, Err := parseSignV4(v4Auth)
	if Err != ErrNone {
		return Err
	}

	cred, _, Err := checkKeyValid(r, signV4Values.Credential.accessKey)
	if Err != ErrNone {
		return Err
	}

	// Extract all the signed headers along with its values.
	extractedSignedHeaders, ErrCode := extractSignedHeaders(signV4Values.SignedHeaders, r)
	if ErrCode != ErrNone {
		return ErrCode
	}

	// Extract date, if not present throw Error.
	var date string
	if date = req.Header.Get(amzDate); date == "" {
		if date = r.Header.Get(headerDate); date == "" {
			return errMissingDateHeader
		}
	}

	// Parse date header.
	t, e := time.Parse(iso8601Format, date)
	if e != nil {
		return errMalformedDate
	}

	// Query string.
	queryStr := req.Form.Encode()

	// Get canonical request.
	canonicalRequest := getCanonicalRequest(extractedSignedHeaders, hashedPayload, queryStr, req.URL.Path, req.Method)

	// Get string to sign from canonical request.
	stringToSign := getStringToSign(canonicalRequest, t, signV4Values.Credential.getScope())

	// Get hmac signing key.
	signingKey := getSigningKey(cred.SecretKey, signV4Values.Credential.scope.date, signV4Values.Credential.scope.region)

	// Calculate signature.
	newSignature := getSignature(signingKey, stringToSign)

	// Verify if signature match.
	if !compareSignatureV4(newSignature, signV4Values.Signature) {
		return errSignatureDoesNotMatch
	}

	// Return Error none.
	return ErrNone
}
