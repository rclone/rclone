/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2017 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package minio

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"hash"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	md5simd "github.com/minio/md5-simd"
	"github.com/minio/minio-go/v6/pkg/s3utils"
	"github.com/minio/sha256-simd"
)

func trimEtag(etag string) string {
	etag = strings.TrimPrefix(etag, "\"")
	return strings.TrimSuffix(etag, "\"")
}

// xmlDecoder provide decoded value in xml.
func xmlDecoder(body io.Reader, v interface{}) error {
	d := xml.NewDecoder(body)
	return d.Decode(v)
}

// sum256 calculate sha256sum for an input byte array, returns hex encoded.
func sum256Hex(data []byte) string {
	hash := newSHA256Hasher()
	defer hash.Close()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// sumMD5Base64 calculate md5sum for an input byte array, returns base64 encoded.
func sumMD5Base64(data []byte) string {
	hash := newMd5Hasher()
	defer hash.Close()
	hash.Write(data)
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

// getEndpointURL - construct a new endpoint.
func getEndpointURL(endpoint string, secure bool) (*url.URL, error) {
	if strings.Contains(endpoint, ":") {
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			return nil, err
		}
		if !s3utils.IsValidIP(host) && !s3utils.IsValidDomain(host) {
			msg := "Endpoint: " + endpoint + " does not follow ip address or domain name standards."
			return nil, ErrInvalidArgument(msg)
		}
	} else {
		if !s3utils.IsValidIP(endpoint) && !s3utils.IsValidDomain(endpoint) {
			msg := "Endpoint: " + endpoint + " does not follow ip address or domain name standards."
			return nil, ErrInvalidArgument(msg)
		}
	}
	// If secure is false, use 'http' scheme.
	scheme := "https"
	if !secure {
		scheme = "http"
	}

	// Construct a secured endpoint URL.
	endpointURLStr := scheme + "://" + endpoint
	endpointURL, err := url.Parse(endpointURLStr)
	if err != nil {
		return nil, err
	}

	// Validate incoming endpoint URL.
	if err := isValidEndpointURL(*endpointURL); err != nil {
		return nil, err
	}
	return endpointURL, nil
}

// closeResponse close non nil response with any response Body.
// convenient wrapper to drain any remaining data on response body.
//
// Subsequently this allows golang http RoundTripper
// to re-use the same connection for future requests.
func closeResponse(resp *http.Response) {
	// Callers should close resp.Body when done reading from it.
	// If resp.Body is not closed, the Client's underlying RoundTripper
	// (typically Transport) may not be able to re-use a persistent TCP
	// connection to the server for a subsequent "keep-alive" request.
	if resp != nil && resp.Body != nil {
		// Drain any remaining Body and then close the connection.
		// Without this closing connection would disallow re-using
		// the same connection for future uses.
		//  - http://stackoverflow.com/a/17961593/4465767
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

var (
	// Hex encoded string of nil sha256sum bytes.
	emptySHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Sentinel URL is the default url value which is invalid.
	sentinelURL = url.URL{}
)

// Verify if input endpoint URL is valid.
func isValidEndpointURL(endpointURL url.URL) error {
	if endpointURL == sentinelURL {
		return ErrInvalidArgument("Endpoint url cannot be empty.")
	}
	if endpointURL.Path != "/" && endpointURL.Path != "" {
		return ErrInvalidArgument("Endpoint url cannot have fully qualified paths.")
	}
	if strings.Contains(endpointURL.Host, ".s3.amazonaws.com") {
		if !s3utils.IsAmazonEndpoint(endpointURL) {
			return ErrInvalidArgument("Amazon S3 endpoint should be 's3.amazonaws.com'.")
		}
	}
	if strings.Contains(endpointURL.Host, ".googleapis.com") {
		if !s3utils.IsGoogleEndpoint(endpointURL) {
			return ErrInvalidArgument("Google Cloud Storage endpoint should be 'storage.googleapis.com'.")
		}
	}
	return nil
}

// Verify if input expires value is valid.
func isValidExpiry(expires time.Duration) error {
	expireSeconds := int64(expires / time.Second)
	if expireSeconds < 1 {
		return ErrInvalidArgument("Expires cannot be lesser than 1 second.")
	}
	if expireSeconds > 604800 {
		return ErrInvalidArgument("Expires cannot be greater than 7 days.")
	}
	return nil
}

// Extract only necessary metadata header key/values by
// filtering them out with a list of custom header keys.
func extractObjMetadata(header http.Header) http.Header {
	preserveKeys := []string{
		"Content-Type",
		"Cache-Control",
		"Content-Encoding",
		"Content-Language",
		"Content-Disposition",
		"X-Amz-Storage-Class",
		"X-Amz-Object-Lock-Mode",
		"X-Amz-Object-Lock-Retain-Until-Date",
		"X-Amz-Object-Lock-Legal-Hold",
		"X-Amz-Website-Redirect-Location",
		"X-Amz-Server-Side-Encryption",
		"X-Amz-Tagging-Count",
		"X-Amz-Meta-",
		// Add new headers to be preserved.
		// if you add new headers here, please extend
		// PutObjectOptions{} to preserve them
		// upon upload as well.
	}
	filteredHeader := make(http.Header)
	for k, v := range header {
		var found bool
		for _, prefix := range preserveKeys {
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			found = true
			break
		}
		if found {
			filteredHeader[k] = v
		}
	}
	return filteredHeader
}

// ToObjectInfo converts http header values into ObjectInfo type,
// extracts metadata and fills in all the necessary fields in ObjectInfo.
func ToObjectInfo(bucketName string, objectName string, h http.Header) (ObjectInfo, error) {
	var err error
	// Trim off the odd double quotes from ETag in the beginning and end.
	etag := trimEtag(h.Get("ETag"))

	// Parse content length is exists
	var size int64 = -1
	contentLengthStr := h.Get("Content-Length")
	if contentLengthStr != "" {
		size, err = strconv.ParseInt(contentLengthStr, 10, 64)
		if err != nil {
			// Content-Length is not valid
			return ObjectInfo{}, ErrorResponse{
				Code:       "InternalError",
				Message:    "Content-Length is invalid. " + reportIssue,
				BucketName: bucketName,
				Key:        objectName,
				RequestID:  h.Get("x-amz-request-id"),
				HostID:     h.Get("x-amz-id-2"),
				Region:     h.Get("x-amz-bucket-region"),
			}
		}
	}

	// Parse Last-Modified has http time format.
	date, err := time.Parse(http.TimeFormat, h.Get("Last-Modified"))
	if err != nil {
		return ObjectInfo{}, ErrorResponse{
			Code:       "InternalError",
			Message:    "Last-Modified time format is invalid. " + reportIssue,
			BucketName: bucketName,
			Key:        objectName,
			RequestID:  h.Get("x-amz-request-id"),
			HostID:     h.Get("x-amz-id-2"),
			Region:     h.Get("x-amz-bucket-region"),
		}
	}

	// Fetch content type if any present.
	contentType := strings.TrimSpace(h.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	expiryStr := h.Get("Expires")
	var expTime time.Time
	if t, err := time.Parse(http.TimeFormat, expiryStr); err == nil {
		expTime = t.UTC()
	}

	metadata := extractObjMetadata(h)
	userMetadata := make(map[string]string)
	for k, v := range metadata {
		if strings.HasPrefix(k, "X-Amz-Meta-") {
			userMetadata[strings.TrimPrefix(k, "X-Amz-Meta-")] = v[0]
		}
	}
	userTags := s3utils.TagDecode(h.Get(amzTaggingHeader))

	// Save object metadata info.
	return ObjectInfo{
		ETag:         etag,
		Key:          objectName,
		Size:         size,
		LastModified: date,
		ContentType:  contentType,
		Expires:      expTime,
		// Extract only the relevant header keys describing the object.
		// following function filters out a list of standard set of keys
		// which are not part of object metadata.
		Metadata:     metadata,
		UserMetadata: userMetadata,
		UserTags:     userTags,
	}, nil
}

// regCred matches credential string in HTTP header
var regCred = regexp.MustCompile("Credential=([A-Z0-9]+)/")

// regCred matches signature string in HTTP header
var regSign = regexp.MustCompile("Signature=([[0-9a-f]+)")

// Redact out signature value from authorization string.
func redactSignature(origAuth string) string {
	if !strings.HasPrefix(origAuth, signV4Algorithm) {
		// Set a temporary redacted auth
		return "AWS **REDACTED**:**REDACTED**"
	}

	/// Signature V4 authorization header.

	// Strip out accessKeyID from:
	// Credential=<access-key-id>/<date>/<aws-region>/<aws-service>/aws4_request
	newAuth := regCred.ReplaceAllString(origAuth, "Credential=**REDACTED**/")

	// Strip out 256-bit signature from: Signature=<256-bit signature>
	return regSign.ReplaceAllString(newAuth, "Signature=**REDACTED**")
}

// Get default location returns the location based on the input
// URL `u`, if region override is provided then all location
// defaults to regionOverride.
//
// If no other cases match then the location is set to `us-east-1`
// as a last resort.
func getDefaultLocation(u url.URL, regionOverride string) (location string) {
	if regionOverride != "" {
		return regionOverride
	}
	region := s3utils.GetRegionFromURL(u)
	if region == "" {
		region = "us-east-1"
	}
	return region
}

var supportedHeaders = []string{
	"content-type",
	"cache-control",
	"content-encoding",
	"content-disposition",
	"content-language",
	"x-amz-website-redirect-location",
	"x-amz-object-lock-mode",
	"x-amz-metadata-directive",
	"x-amz-object-lock-retain-until-date",
	"expires",
	// Add more supported headers here.
}

// isStorageClassHeader returns true if the header is a supported storage class header
func isStorageClassHeader(headerKey string) bool {
	return strings.EqualFold(amzStorageClass, headerKey)
}

// isStandardHeader returns true if header is a supported header and not a custom header
func isStandardHeader(headerKey string) bool {
	key := strings.ToLower(headerKey)
	for _, header := range supportedHeaders {
		if strings.ToLower(header) == key {
			return true
		}
	}
	return false
}

// sseHeaders is list of server side encryption headers
var sseHeaders = []string{
	"x-amz-server-side-encryption",
	"x-amz-server-side-encryption-aws-kms-key-id",
	"x-amz-server-side-encryption-context",
	"x-amz-server-side-encryption-customer-algorithm",
	"x-amz-server-side-encryption-customer-key",
	"x-amz-server-side-encryption-customer-key-MD5",
}

// isSSEHeader returns true if header is a server side encryption header.
func isSSEHeader(headerKey string) bool {
	key := strings.ToLower(headerKey)
	for _, h := range sseHeaders {
		if strings.ToLower(h) == key {
			return true
		}
	}
	return false
}

// isAmzHeader returns true if header is a x-amz-meta-* or x-amz-acl header.
func isAmzHeader(headerKey string) bool {
	key := strings.ToLower(headerKey)

	return strings.HasPrefix(key, "x-amz-meta-") || strings.HasPrefix(key, "x-amz-grant-") || key == "x-amz-acl" || isSSEHeader(headerKey)
}

var md5Pool = sync.Pool{New: func() interface{} { return md5.New() }}
var sha256Pool = sync.Pool{New: func() interface{} { return sha256.New() }}

func newMd5Hasher() md5simd.Hasher {
	return hashWrapper{Hash: md5Pool.New().(hash.Hash), isMD5: true}
}

func newSHA256Hasher() md5simd.Hasher {
	return hashWrapper{Hash: sha256Pool.New().(hash.Hash), isSHA256: true}
}

// hashWrapper implements the md5simd.Hasher interface.
type hashWrapper struct {
	hash.Hash
	isMD5    bool
	isSHA256 bool
}

// Close will put the hasher back into the pool.
func (m hashWrapper) Close() {
	if m.isMD5 && m.Hash != nil {
		m.Reset()
		md5Pool.Put(m.Hash)
	}
	if m.isSHA256 && m.Hash != nil {
		m.Reset()
		sha256Pool.Put(m.Hash)
	}
	m.Hash = nil
}
