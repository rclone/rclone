package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/version"
)

// Error describes a B2 error response
type Error struct {
	Status  int    `json:"status"`  // The numeric HTTP status code. Always matches the status in the HTTP response.
	Code    string `json:"code"`    // A single-identifier code that identifies the error.
	Message string `json:"message"` // A human-readable message, in English, saying what went wrong.
}

// Error satisfies the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%s (%d %s)", e.Message, e.Status, e.Code)
}

// Fatal satisfies the Fatal interface
//
// It indicates which errors should be treated as fatal
func (e *Error) Fatal() bool {
	return e.Status == 403 // 403 errors shouldn't be retried
}

var _ fserrors.Fataler = (*Error)(nil)

// Bucket describes a B2 bucket
type Bucket struct {
	ID        string `json:"bucketId"`
	AccountID string `json:"accountId"`
	Name      string `json:"bucketName"`
	Type      string `json:"bucketType"`
}

// Timestamp is a UTC time when this file was uploaded. It is a base
// 10 number of milliseconds since midnight, January 1, 1970 UTC. This
// fits in a 64 bit integer such as the type "long" in the programming
// language Java. It is intended to be compatible with Java's time
// long. For example, it can be passed directly into the java call
// Date.setTime(long time).
type Timestamp time.Time

// MarshalJSON turns a Timestamp into JSON (in UTC)
func (t *Timestamp) MarshalJSON() (out []byte, err error) {
	timestamp := (*time.Time)(t).UTC().UnixNano()
	return []byte(strconv.FormatInt(timestamp/1e6, 10)), nil
}

// UnmarshalJSON turns JSON into a Timestamp
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	timestamp, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*t = Timestamp(time.Unix(timestamp/1e3, (timestamp%1e3)*1e6).UTC())
	return nil
}

// HasVersion returns true if it looks like the passed filename has a timestamp on it.
//
// Note that the passed filename's timestamp may still be invalid even if this
// function returns true.
func HasVersion(remote string) bool {
	return version.Match(remote)
}

// AddVersion adds the timestamp as a version string into the filename passed in.
func (t Timestamp) AddVersion(remote string) string {
	return version.Add(remote, time.Time(t))
}

// RemoveVersion removes the timestamp from a filename as a version string.
//
// It returns the new file name and a timestamp, or the old filename
// and a zero timestamp.
func RemoveVersion(remote string) (t Timestamp, newRemote string) {
	time, newRemote := version.Remove(remote)
	t = Timestamp(time)
	return
}

// IsZero returns true if the timestamp is uninitialized
func (t Timestamp) IsZero() bool {
	return time.Time(t).IsZero()
}

// Equal compares two timestamps
//
// If either are !IsZero then it returns false
func (t Timestamp) Equal(s Timestamp) bool {
	if time.Time(t).IsZero() {
		return false
	}
	if time.Time(s).IsZero() {
		return false
	}
	return time.Time(t).Equal(time.Time(s))
}

// File is info about a file
type File struct {
	ID              string            `json:"fileId"`          // The unique identifier for this version of this file. Used with b2_get_file_info, b2_download_file_by_id, and b2_delete_file_version.
	Name            string            `json:"fileName"`        // The name of this file, which can be used with b2_download_file_by_name.
	Action          string            `json:"action"`          // Either "upload" or "hide". "upload" means a file that was uploaded to B2 Cloud Storage. "hide" means a file version marking the file as hidden, so that it will not show up in b2_list_file_names. The result of b2_list_file_names will contain only "upload". The result of b2_list_file_versions may have both.
	Size            int64             `json:"size"`            // The number of bytes in the file.
	UploadTimestamp Timestamp         `json:"uploadTimestamp"` // This is a UTC time when this file was uploaded.
	SHA1            string            `json:"contentSha1"`     // The SHA1 of the bytes stored in the file.
	ContentType     string            `json:"contentType"`     // The MIME type of the file.
	Info            map[string]string `json:"fileInfo"`        // The custom information that was uploaded with the file. This is a JSON object, holding the name/value pairs that were uploaded with the file.
}

// AuthorizeAccountResponse is as returned from the b2_authorize_account call
type AuthorizeAccountResponse struct {
	AbsoluteMinimumPartSize int      `json:"absoluteMinimumPartSize"` // The smallest possible size of a part of a large file.
	AccountID               string   `json:"accountId"`               // The identifier for the account.
	Allowed                 struct { // An object (see below) containing the capabilities of this auth token, and any restrictions on using it.
		BucketID     string      `json:"bucketId"`     // When present, access is restricted to one bucket.
		BucketName   string      `json:"bucketName"`   // When present, name of bucket - may be empty
		Capabilities []string    `json:"capabilities"` // A list of strings, each one naming a capability the key has.
		NamePrefix   interface{} `json:"namePrefix"`   // When present, access is restricted to files whose names start with the prefix
	} `json:"allowed"`
	APIURL              string `json:"apiUrl"`              // The base URL to use for all API calls except for uploading and downloading files.
	AuthorizationToken  string `json:"authorizationToken"`  // An authorization token to use with all calls, other than b2_authorize_account, that need an Authorization header.
	DownloadURL         string `json:"downloadUrl"`         // The base URL to use for downloading files.
	MinimumPartSize     int    `json:"minimumPartSize"`     // DEPRECATED: This field will always have the same value as recommendedPartSize. Use recommendedPartSize instead.
	RecommendedPartSize int    `json:"recommendedPartSize"` // The recommended size for each part of a large file. We recommend using this part size for optimal upload performance.
}

// ListBucketsRequest is parameters for b2_list_buckets call
type ListBucketsRequest struct {
	AccountID   string   `json:"accountId"`             // The identifier for the account.
	BucketID    string   `json:"bucketId,omitempty"`    // When specified, the result will be a list containing just this bucket.
	BucketName  string   `json:"bucketName,omitempty"`  // When specified, the result will be a list containing just this bucket.
	BucketTypes []string `json:"bucketTypes,omitempty"` // If present, B2 will use it as a filter for bucket types returned in the list buckets response.
}

// ListBucketsResponse is as returned from the b2_list_buckets call
type ListBucketsResponse struct {
	Buckets []Bucket `json:"buckets"`
}

// ListFileNamesRequest is as passed to b2_list_file_names or b2_list_file_versions
type ListFileNamesRequest struct {
	BucketID      string `json:"bucketId"`                // required - The bucket to look for file names in.
	StartFileName string `json:"startFileName,omitempty"` // optional - The first file name to return. If there is a file with this name, it will be returned in the list. If not, the first file name after this the first one after this name.
	MaxFileCount  int    `json:"maxFileCount,omitempty"`  // optional - The maximum number of files to return from this call. The default value is 100, and the maximum allowed is 1000.
	StartFileID   string `json:"startFileId,omitempty"`   // optional - What to pass in to startFileId for the next search to continue where this one left off.
	Prefix        string `json:"prefix,omitempty"`        // optional - Files returned will be limited to those with the given prefix. Defaults to the empty string, which matches all files.
	Delimiter     string `json:"delimiter,omitempty"`     // Files returned will be limited to those within the top folder, or any one subfolder. Defaults to NULL. Folder names will also be returned. The delimiter character will be used to "break" file names into folders.
}

// ListFileNamesResponse is as received from b2_list_file_names or b2_list_file_versions
type ListFileNamesResponse struct {
	Files        []File  `json:"files"`        // An array of objects, each one describing one file.
	NextFileName *string `json:"nextFileName"` // What to pass in to startFileName for the next search to continue where this one left off, or null if there are no more files.
	NextFileID   *string `json:"nextFileId"`   // What to pass in to startFileId for the next search to continue where this one left off, or null if there are no more files.
}

// GetUploadURLRequest is passed to b2_get_upload_url
type GetUploadURLRequest struct {
	BucketID string `json:"bucketId"` // The ID of the bucket that you want to upload to.
}

// GetUploadURLResponse is received from b2_get_upload_url
type GetUploadURLResponse struct {
	BucketID           string `json:"bucketId"`           // The unique ID of the bucket.
	UploadURL          string `json:"uploadUrl"`          // The URL that can be used to upload files to this bucket, see b2_upload_file.
	AuthorizationToken string `json:"authorizationToken"` // The authorizationToken that must be used when uploading files to this bucket, see b2_upload_file.
}

// GetDownloadAuthorizationRequest is passed to b2_get_download_authorization
type GetDownloadAuthorizationRequest struct {
	BucketID               string `json:"bucketId"`                       // The ID of the bucket that you want to upload to.
	FileNamePrefix         string `json:"fileNamePrefix"`                 // The file name prefix of files the download authorization token will allow access to.
	ValidDurationInSeconds int64  `json:"validDurationInSeconds"`         // The number of seconds before the authorization token will expire. The minimum value is 1 second. The maximum value is 604800 which is one week in seconds.
	B2ContentDisposition   string `json:"b2ContentDisposition,omitempty"` // optional - If this is present, download requests using the returned authorization must include the same value for b2ContentDisposition.
}

// GetDownloadAuthorizationResponse is received from b2_get_download_authorization
type GetDownloadAuthorizationResponse struct {
	BucketID           string `json:"bucketId"`           // The unique ID of the bucket.
	FileNamePrefix     string `json:"fileNamePrefix"`     // The file name prefix of files the download authorization token will allow access to.
	AuthorizationToken string `json:"authorizationToken"` // The authorizationToken that must be used when downloading files, see b2_download_file_by_name.
}

// FileInfo is received from b2_upload_file, b2_get_file_info and b2_finish_large_file
type FileInfo struct {
	ID              string            `json:"fileId"`          // The unique identifier for this version of this file. Used with b2_get_file_info, b2_download_file_by_id, and b2_delete_file_version.
	Name            string            `json:"fileName"`        // The name of this file, which can be used with b2_download_file_by_name.
	Action          string            `json:"action"`          // Either "upload" or "hide". "upload" means a file that was uploaded to B2 Cloud Storage. "hide" means a file version marking the file as hidden, so that it will not show up in b2_list_file_names. The result of b2_list_file_names will contain only "upload". The result of b2_list_file_versions may have both.
	AccountID       string            `json:"accountId"`       // Your account ID.
	BucketID        string            `json:"bucketId"`        // The bucket that the file is in.
	Size            int64             `json:"contentLength"`   // The number of bytes stored in the file.
	UploadTimestamp Timestamp         `json:"uploadTimestamp"` // This is a UTC time when this file was uploaded.
	SHA1            string            `json:"contentSha1"`     // The SHA1 of the bytes stored in the file.
	ContentType     string            `json:"contentType"`     // The MIME type of the file.
	Info            map[string]string `json:"fileInfo"`        // The custom information that was uploaded with the file. This is a JSON object, holding the name/value pairs that were uploaded with the file.
}

// CreateBucketRequest is used to create a bucket
type CreateBucketRequest struct {
	AccountID string `json:"accountId"`
	Name      string `json:"bucketName"`
	Type      string `json:"bucketType"`
}

// DeleteBucketRequest is used to create a bucket
type DeleteBucketRequest struct {
	ID        string `json:"bucketId"`
	AccountID string `json:"accountId"`
}

// DeleteFileRequest is used to delete a file version
type DeleteFileRequest struct {
	ID   string `json:"fileId"`   // The ID of the file, as returned by b2_upload_file, b2_list_file_names, or b2_list_file_versions.
	Name string `json:"fileName"` // The name of this file.
}

// HideFileRequest is used to delete a file
type HideFileRequest struct {
	BucketID string `json:"bucketId"` // The bucket containing the file to hide.
	Name     string `json:"fileName"` // The name of the file to hide.
}

// GetFileInfoRequest is used to return a FileInfo struct with b2_get_file_info
type GetFileInfoRequest struct {
	ID string `json:"fileId"` // The ID of the file, as returned by b2_upload_file, b2_list_file_names, or b2_list_file_versions.
}

// StartLargeFileRequest (b2_start_large_file) Prepares for uploading the parts of a large file.
//
// If the original source of the file being uploaded has a last
// modified time concept, Backblaze recommends using
// src_last_modified_millis as the name, and a string holding the base
// 10 number number of milliseconds since midnight, January 1, 1970
// UTC. This fits in a 64 bit integer such as the type "long" in the
// programming language Java. It is intended to be compatible with
// Java's time long. For example, it can be passed directly into the
// Java call Date.setTime(long time).
//
// If the caller knows the SHA1 of the entire large file being
// uploaded, Backblaze recommends using large_file_sha1 as the name,
// and a 40 byte hex string representing the SHA1.
//
// Example: { "src_last_modified_millis" : "1452802803026", "large_file_sha1" : "a3195dc1e7b46a2ff5da4b3c179175b75671e80d", "color": "blue" }
type StartLargeFileRequest struct {
	BucketID    string            `json:"bucketId"`    //The ID of the bucket that the file will go in.
	Name        string            `json:"fileName"`    // The name of the file. See Files for requirements on file names.
	ContentType string            `json:"contentType"` // The MIME type of the content of the file, which will be returned in the Content-Type header when downloading the file. Use the Content-Type b2/x-auto to automatically set the stored Content-Type post upload. In the case where a file extension is absent or the lookup fails, the Content-Type is set to application/octet-stream.
	Info        map[string]string `json:"fileInfo"`    // A JSON object holding the name/value pairs for the custom file info.
}

// StartLargeFileResponse is the response to StartLargeFileRequest
type StartLargeFileResponse struct {
	ID              string            `json:"fileId"`          // The unique identifier for this version of this file. Used with b2_get_file_info, b2_download_file_by_id, and b2_delete_file_version.
	Name            string            `json:"fileName"`        // The name of this file, which can be used with b2_download_file_by_name.
	AccountID       string            `json:"accountId"`       // The identifier for the account.
	BucketID        string            `json:"bucketId"`        // The unique ID of the bucket.
	ContentType     string            `json:"contentType"`     // The MIME type of the file.
	Info            map[string]string `json:"fileInfo"`        // The custom information that was uploaded with the file. This is a JSON object, holding the name/value pairs that were uploaded with the file.
	UploadTimestamp Timestamp         `json:"uploadTimestamp"` // This is a UTC time when this file was uploaded.
}

// GetUploadPartURLRequest is passed to b2_get_upload_part_url
type GetUploadPartURLRequest struct {
	ID string `json:"fileId"` // The unique identifier of the file being uploaded.
}

// GetUploadPartURLResponse is received from b2_get_upload_url
type GetUploadPartURLResponse struct {
	ID                 string `json:"fileId"`             // The unique identifier of the file being uploaded.
	UploadURL          string `json:"uploadUrl"`          // The URL that can be used to upload files to this bucket, see b2_upload_part.
	AuthorizationToken string `json:"authorizationToken"` // The authorizationToken that must be used when uploading files to this bucket, see b2_upload_part.
}

// UploadPartResponse is the response to b2_upload_part
type UploadPartResponse struct {
	ID         string `json:"fileId"`        // The unique identifier of the file being uploaded.
	PartNumber int64  `json:"partNumber"`    // Which part this is (starting from 1)
	Size       int64  `json:"contentLength"` // The number of bytes stored in the file.
	SHA1       string `json:"contentSha1"`   // The SHA1 of the bytes stored in the file.
}

// FinishLargeFileRequest is passed to b2_finish_large_file
//
// The response is a FileInfo object (with extra AccountID and BucketID fields which we ignore).
//
// Large files do not have a SHA1 checksum. The value will always be "none".
type FinishLargeFileRequest struct {
	ID    string   `json:"fileId"`        // The unique identifier of the file being uploaded.
	SHA1s []string `json:"partSha1Array"` // A JSON array of hex SHA1 checksums of the parts of the large file. This is a double-check that the right parts were uploaded in the right order, and that none were missed. Note that the part numbers start at 1, and the SHA1 of the part 1 is the first string in the array, at index 0.
}

// CancelLargeFileRequest is passed to b2_finish_large_file
//
// The response is a CancelLargeFileResponse
type CancelLargeFileRequest struct {
	ID string `json:"fileId"` // The unique identifier of the file being uploaded.
}

// CancelLargeFileResponse is the response to CancelLargeFileRequest
type CancelLargeFileResponse struct {
	ID        string `json:"fileId"`    // The unique identifier of the file being uploaded.
	Name      string `json:"fileName"`  // The name of this file.
	AccountID string `json:"accountId"` // The identifier for the account.
	BucketID  string `json:"bucketId"`  // The unique ID of the bucket.
}

// CopyFileRequest is as passed to b2_copy_file
type CopyFileRequest struct {
	SourceID          string            `json:"sourceFileId"`                  // The ID of the source file being copied.
	Name              string            `json:"fileName"`                      // The name of the new file being created.
	Range             string            `json:"range,omitempty"`               // The range of bytes to copy. If not provided, the whole source file will be copied.
	MetadataDirective string            `json:"metadataDirective,omitempty"`   // The strategy for how to populate metadata for the new file: COPY or REPLACE
	ContentType       string            `json:"contentType,omitempty"`         // The MIME type of the content of the file (REPLACE only)
	Info              map[string]string `json:"fileInfo,omitempty"`            // This field stores the metadata that will be stored with the file. (REPLACE only)
	DestBucketID      string            `json:"destinationBucketId,omitempty"` // The destination ID of the bucket if set, if not the source bucket will be used
}

// CopyPartRequest is the request for b2_copy_part - the response is UploadPartResponse
type CopyPartRequest struct {
	SourceID    string `json:"sourceFileId"`    // The ID of the source file being copied.
	LargeFileID string `json:"largeFileId"`     // The ID of the large file the part will belong to, as returned by b2_start_large_file.
	PartNumber  int64  `json:"partNumber"`      // Which part this is (starting from 1)
	Range       string `json:"range,omitempty"` // The range of bytes to copy. If not provided, the whole source file will be copied.
}
