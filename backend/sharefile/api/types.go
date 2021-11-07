// Package api contains definitions for using the premiumize.me API
package api

import (
	"errors"
	"fmt"
	"time"
)

// ListRequestSelect should be used in $select for Items/Children
const ListRequestSelect = "odata.count,FileCount,Name,FileName,CreationDate,IsHidden,FileSizeBytes,odata.type,Id,Hash,ClientModifiedDate"

// ListResponse is returned from the Items/Children call
type ListResponse struct {
	OdataCount int    `json:"odata.count"`
	Value      []Item `json:"value"`
}

// Item Types
const (
	ItemTypeFolder = "ShareFile.Api.Models.Folder"
	ItemTypeFile   = "ShareFile.Api.Models.File"
)

// Item refers to a file or folder
type Item struct {
	FileCount  int32     `json:"FileCount,omitempty"`
	Name       string    `json:"Name,omitempty"`
	FileName   string    `json:"FileName,omitempty"`
	CreatedAt  time.Time `json:"CreationDate,omitempty"`
	ModifiedAt time.Time `json:"ClientModifiedDate,omitempty"`
	IsHidden   bool      `json:"IsHidden,omitempty"`
	Size       int64     `json:"FileSizeBytes,omitempty"`
	Type       string    `json:"odata.type,omitempty"`
	ID         string    `json:"Id,omitempty"`
	Hash       string    `json:"Hash,omitempty"`
}

// Error is an odata error return
type Error struct {
	Code    string `json:"code"`
	Message struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	} `json:"message"`
	Reason string `json:"reason"`
}

// Satisfy error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Message.Value, e.Code, e.Reason)
}

// Check Error satisfies error interface
var _ error = &Error{}

// DownloadSpecification is the response to /Items/Download
type DownloadSpecification struct {
	Token    string `json:"DownloadToken"`
	URL      string `json:"DownloadUrl"`
	Metadata string `json:"odata.metadata"`
	Type     string `json:"odata.type"`
}

// UploadRequest is set to /Items/Upload2 to receive an UploadSpecification
type UploadRequest struct {
	Method         string    `json:"method"`                   // Upload method: one of: standard, streamed or threaded
	Raw            bool      `json:"raw"`                      // Raw post if true or MIME upload if false
	Filename       string    `json:"fileName"`                 // Uploaded item file name.
	Filesize       *int64    `json:"fileSize,omitempty"`       // Uploaded item file size.
	Overwrite      bool      `json:"overwrite"`                // Indicates whether items with the same name will be overwritten or not.
	CreatedDate    time.Time `json:"ClientCreatedDate"`        // Created Date of this Item.
	ModifiedDate   time.Time `json:"ClientModifiedDate"`       // Modified Date of this Item.
	BatchID        string    `json:"batchId,omitempty"`        // Indicates part of a batch. Batched uploads do not send notification until the whole batch is completed.
	BatchLast      *bool     `json:"batchLast,omitempty"`      // Indicates is the last in a batch. Upload notifications for the whole batch are sent after this upload.
	CanResume      *bool     `json:"canResume,omitempty"`      // Indicates uploader supports resume.
	StartOver      *bool     `json:"startOver,omitempty"`      // Indicates uploader wants to restart the file - i.e., ignore previous failed upload attempts.
	Tool           string    `json:"tool,omitempty"`           // Identifies the uploader tool.
	Title          string    `json:"title,omitempty"`          // Item Title
	Details        string    `json:"details,omitempty"`        // Item description
	IsSend         *bool     `json:"isSend,omitempty"`         // Indicates that this upload is part of a Send operation
	SendGUID       string    `json:"sendGuid,omitempty"`       // Used if IsSend is true. Specifies which Send operation this upload is part of.
	OpID           string    `json:"opid,omitempty"`           // Used for Asynchronous copy/move operations - called by Zones to push files to other Zones
	ThreadCount    *int      `json:"threadCount,omitempty"`    // Specifies the number of threads the threaded uploader will use. Only used is method is threaded, ignored otherwise
	Notify         *bool     `json:"notify,omitempty"`         // Indicates whether users will be notified of this upload - based on folder preferences
	ExpirationDays *int      `json:"expirationDays,omitempty"` // File expiration days
	BaseFileID     string    `json:"baseFileId,omitempty"`     // Used to check conflict in file during File Upload.
}

// UploadSpecification is returned from /Items/Upload
type UploadSpecification struct {
	Method             string `json:"Method"`             // The Upload method that must be used for this upload
	PrepareURI         string `json:"PrepareUri"`         // If provided, clients must issue a request to this Uri before uploading any data.
	ChunkURI           string `json:"ChunkUri"`           // Specifies the URI the client must send the file data to
	FinishURI          string `json:"FinishUri"`          // If provided, specifies the final call the client must perform to finish the upload process
	ProgressData       string `json:"ProgressData"`       // Allows the client to check progress of standard uploads
	IsResume           bool   `json:"IsResume"`           // Specifies a Resumable upload is supported.
	ResumeIndex        int64  `json:"ResumeIndex"`        // Specifies the initial index for resuming, if IsResume is true.
	ResumeOffset       int64  `json:"ResumeOffset"`       // Specifies the initial file offset by bytes, if IsResume is true
	ResumeFileHash     string `json:"ResumeFileHash"`     // Specifies the MD5 hash of the first ResumeOffset bytes of the partial file found at the server
	MaxNumberOfThreads int    `json:"MaxNumberOfThreads"` // Specifies the max number of chunks that can be sent simultaneously for threaded uploads
}

// UploadFinishResponse is returns from calling UploadSpecification.FinishURI
type UploadFinishResponse struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"errorMessage"`
	ErrorCode    int    `json:"errorCode"`
	Value        []struct {
		UploadID    string `json:"uploadid"`
		ParentID    string `json:"parentid"`
		ID          string `json:"id"`
		StreamID    string `json:"streamid"`
		FileName    string `json:"filename"`
		DisplayName string `json:"displayname"`
		Size        int    `json:"size"`
		Md5         string `json:"md5"`
	} `json:"value"`
}

// ID returns the ID of the first response if available
func (finish *UploadFinishResponse) ID() (string, error) {
	if finish.Error {
		return "", fmt.Errorf("upload failed: %s (%d)", finish.ErrorMessage, finish.ErrorCode)
	}
	if len(finish.Value) == 0 {
		return "", errors.New("upload failed: no results returned")
	}
	return finish.Value[0].ID, nil
}

// Parent is the ID of the parent folder
type Parent struct {
	ID string `json:"Id,omitempty"`
}

// Zone is where the data is stored
type Zone struct {
	ID string `json:"Id,omitempty"`
}

// UpdateItemRequest is sent to PATCH /v3/Items(id)
type UpdateItemRequest struct {
	Name           string     `json:"Name,omitempty"`
	FileName       string     `json:"FileName,omitempty"`
	Description    string     `json:"Description,omitempty"`
	ExpirationDate *time.Time `json:"ExpirationDate,omitempty"`
	Parent         *Parent    `json:"Parent,omitempty"`
	Zone           *Zone      `json:"Zone,omitempty"`
	ModifiedAt     *time.Time `json:"ClientModifiedDate,omitempty"`
}
