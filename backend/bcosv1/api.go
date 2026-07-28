// Package bcosv1 provides a read-only backend for BCOS v1
// (Bloomberg Cloud Object Store, legacy), used as a migration source.
//
// Protocol reference:
//   - API docs: https://tutti.prod.bloomberg.com/bcos/docs/v1/api
//   - v1/v2 differences (ETag, size limits, multipart, inline objects):
//     https://tutti.prod.bloomberg.com/bcos/docs/v2/info_v1
//   - Server implementation (ground truth for anything the docs are vague
//     about — request/response headers, listing XML, error codes):
//   - auth/validation, validate_account_credentials:
//     https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/__init__.py#L87-L107
//   - put_object:
//     https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L136
//   - delete_object:
//     https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L296
//   - get_object:
//     https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L450
//   - bucket listing, get_bucket:
//     https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/bucket.py#L39
//   - error codes and their HTTP status:
//     https://bbgithub.dev.bloomberg.com/bcos/bcos-v1-common/blob/0.1.6/src/bloomberg/bcos/v1/common/storage_service/exceptions.py#L114-L161
package bcosv1

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// listBucketResult is the XML returned by GET /v1/{bucket} (list objects).
//
// NextMarker/NextKeyMarker+NextVersionIdMarker point at the row *after* the
// page just returned, not the last row *of* it (the reverse of S3's own
// "last returned key" semantics) — confirmed in the server's get_bucket:
// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/bucket.py#L117-L145
// It queries max_keys+1 rows, and when that extra row exists (is_truncated),
// pops it off and echoes its own key/version back as Next*Marker, never
// touching the last row actually included in the page. Passing that value
// straight back as the next request's marker is therefore exactly correct —
// no re-derivation from the last returned entry needed.
type listBucketResult struct {
	XMLName        xml.Name    `xml:"ListBucketResult"`
	Name           string      `xml:"Name"`
	Prefix         string      `xml:"Prefix"`
	Marker         string      `xml:"Marker"`
	NextMarker     string      `xml:"NextMarker"`
	MaxKeys        int         `xml:"MaxKeys"`
	Delimiter      string      `xml:"Delimiter"`
	IsTruncated    bool        `xml:"IsTruncated"`
	Contents       []xmlObject `xml:"Contents"`
	CommonPrefixes []xmlPrefix `xml:"CommonPrefixes"`
}

// listVersionsResult is the XML returned by GET /v1/{bucket}?versions
type listVersionsResult struct {
	XMLName             xml.Name    `xml:"ListVersionsResult"`
	Name                string      `xml:"Name"`
	Prefix              string      `xml:"Prefix"`
	KeyMarker           string      `xml:"KeyMarker"`
	VersionIdMarker     string      `xml:"VersionIdMarker"`
	NextKeyMarker       string      `xml:"NextKeyMarker"`
	NextVersionIdMarker string      `xml:"NextVersionIdMarker"`
	MaxKeys             int         `xml:"MaxKeys"`
	Delimiter           string      `xml:"Delimiter"`
	IsTruncated         bool        `xml:"IsTruncated"`
	Versions            []xmlObject `xml:"Version"`
	DeleteMarkers       []xmlObject `xml:"DeleteMarker"`
	CommonPrefixes      []xmlPrefix `xml:"CommonPrefixes"`
}

type xmlPrefix struct {
	Prefix string `xml:"Prefix"`
}

// xmlObject models both <Contents> (list) and <Version>/<DeleteMarker> nodes.
// A DeleteMarker row carries no ETag/Size — the server's get_bucket picks the
// element name by checking exactly that:
// `if row.etag is None and row.size is None: nodetype = "DeleteMarker"`.
// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/bucket.py#L159-L162
type xmlObject struct {
	Key          string `xml:"Key"`
	VersionId    string `xml:"VersionId"`
	IsLatest     bool   `xml:"IsLatest"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// listItem is the normalized form used by the lister callback
type listItem struct {
	key            string
	versionID      string // "" when listing latest-only (objects mode)
	isLatest       bool
	isDeleteMarker bool
	size           int64
	md5            string // hex, unquoted
	modTime        time.Time
}

// bcosError models BCOS's XML <Error> body, returned on GET/list failures
// (never on HEAD, which carries no body regardless of Content-Length). Codes
// and their HTTP status are enumerated as BcosError subclasses here:
// https://bbgithub.dev.bloomberg.com/bcos/bcos-v1-common/blob/0.1.6/src/bloomberg/bcos/v1/common/storage_service/exceptions.py#L114-L161
// NoSuchBucket (L114) / NoSuchKey (L119) / NoSuchVersion (L124) -> 404,
// InvalidSecurity (L154) / BadApiKey (L134) -> 403. There is no dedicated
// rate-limit exception there (no 429/503) — the 429/5xx retry classification
// in shouldRetry is a general defensive assumption about the network path in
// front of BCOS, not a documented BCOS behavior.
type bcosError struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

func (e *bcosError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("bcosv1: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("bcosv1: %s", e.Code)
}

// unquoteETag strips surrounding quotes and lowercases the hex MD5. BCOS's
// ETag is always a plain hex MD5 — put_object hashes the upload with
// HashingStream and writes `'"%s"' % etag.hexdigest()` — no multipart, no
// SSE, so this holds for every object regardless of size.
// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L273-L292
func unquoteETag(etag string) string {
	return strings.ToLower(strings.Trim(etag, `"`))
}

// parseTime parses BCOS timestamps. Listings use ISO-8601 with ms + Z (e.g.
// 2026-07-16T14:04:01.678Z), produced by format_datetime_like_s3:
// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/bucket.py#L19-L27
// GET/HEAD's Last-Modified header is a standard HTTP-date instead (Flask's
// Response.last_modified, set in get_object):
// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L586
// Both layouts are tried; be tolerant of other forms too.
func parseTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123,
		time.RFC1123Z,
		"Mon, 02 Jan 2006 15:04:05 GMT",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// escapePath URL-escapes each path segment while preserving the "/" separators
func escapePath(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
