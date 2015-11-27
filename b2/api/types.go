package api

import (
	"fmt"
	"strconv"
	"time"
)

// Error describes a B2 error response
type Error struct {
	Status  int    `json:"status"`  // The numeric HTTP status code. Always matches the status in the HTTP response.
	Code    string `json:"code"`    // A single-identifier code that identifies the error.
	Message string `json:"message"` // A human-readable message, in English, saying what went wrong.
}

// Error statisfies the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%s (%d %s)", e.Message, e.Status, e.Code)
}

// Account describes a B2 account
type Account struct {
	ID string `json:"accountId"` // The identifier for the account.
}

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
	return []byte(strconv.FormatInt(timestamp/1E6, 10)), nil
}

// UnmarshalJSON turns JSON into a Timestamp
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	timestamp, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*t = Timestamp(time.Unix(timestamp/1E3, (timestamp%1E3)*1E6))
	return nil
}

// File is info about a file
type File struct {
	ID              string    `json:"fileId"`          // The unique identifier for this version of this file. Used with b2_get_file_info, b2_download_file_by_id, and b2_delete_file_version.
	Name            string    `json:"fileName"`        // The name of this file, which can be used with b2_download_file_by_name.
	Action          string    `json:"action"`          // Either "upload" or "hide". "upload" means a file that was uploaded to B2 Cloud Storage. "hide" means a file version marking the file as hidden, so that it will not show up in b2_list_file_names. The result of b2_list_file_names will contain only "upload". The result of b2_list_file_versions may have both.
	Size            int64     `json:"size"`            // The number of bytes in the file.
	UploadTimestamp Timestamp `json:"uploadTimestamp"` // This is a UTC time when this file was uploaded.
}

// AuthorizeAccountResponse is as returned from the b2_authorize_account call
type AuthorizeAccountResponse struct {
	AccountID          string `json:"accountId"`          // The identifier for the account.
	AuthorizationToken string `json:"authorizationToken"` // An authorization token to use with all calls, other than b2_authorize_account, that need an Authorization header.
	APIURL             string `json:"apiUrl"`             // The base URL to use for all API calls except for uploading and downloading files.
	DownloadURL        string `json:"downloadUrl"`        // The base URL to use for downloading files.
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

// FileInfo is received from b2_upload_file and b2_get_file_info
type FileInfo struct {
	ID          string            `json:"fileId"`        // The unique identifier for this version of this file. Used with b2_get_file_info, b2_download_file_by_id, and b2_delete_file_version.
	Name        string            `json:"fileName"`      // The name of this file, which can be used with b2_download_file_by_name.
	AccountID   string            `json:"accountId"`     // Your account ID.
	BucketID    string            `json:"bucketId"`      // The bucket that the file is in.
	Size        int64             `json:"contentLength"` // The number of bytes stored in the file.
	SHA1        string            `json:"contentSha1"`   // The SHA1 of the bytes stored in the file.
	ContentType string            `json:"contentType"`   // The MIME type of the file.
	Info        map[string]string `json:"fileInfo"`      // The custom information that was uploaded with the file. This is a JSON object, holding the name/value pairs that were uploaded with the file.
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
