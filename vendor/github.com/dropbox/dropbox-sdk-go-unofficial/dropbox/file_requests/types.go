// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package file_requests : This namespace contains endpoints and data types for
// file request operations.
package file_requests

import (
	"encoding/json"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// CreateFileRequestArgs : Arguments for `create`.
type CreateFileRequestArgs struct {
	// Title : The title of the file request. Must not be empty.
	Title string `json:"title"`
	// Destination : The path of the folder in the Dropbox where uploaded files
	// will be sent. For apps with the app folder permission, this will be
	// relative to the app folder.
	Destination string `json:"destination"`
	// Deadline : The deadline for the file request. Deadlines can only be set
	// by Pro and Business accounts.
	Deadline *FileRequestDeadline `json:"deadline,omitempty"`
	// Open : Whether or not the file request should be open. If the file
	// request is closed, it will not accept any file submissions, but it can be
	// opened later.
	Open bool `json:"open"`
}

// NewCreateFileRequestArgs returns a new CreateFileRequestArgs instance
func NewCreateFileRequestArgs(Title string, Destination string) *CreateFileRequestArgs {
	s := new(CreateFileRequestArgs)
	s.Title = Title
	s.Destination = Destination
	s.Open = true
	return s
}

// GeneralFileRequestsError : There is an error accessing the file requests
// functionality.
type GeneralFileRequestsError struct {
	dropbox.Tagged
}

// Valid tag values for GeneralFileRequestsError
const (
	GeneralFileRequestsErrorDisabledForTeam = "disabled_for_team"
	GeneralFileRequestsErrorOther           = "other"
)

// FileRequestError : There is an error with the file request.
type FileRequestError struct {
	dropbox.Tagged
}

// Valid tag values for FileRequestError
const (
	FileRequestErrorDisabledForTeam = "disabled_for_team"
	FileRequestErrorOther           = "other"
	FileRequestErrorNotFound        = "not_found"
	FileRequestErrorNotAFolder      = "not_a_folder"
	FileRequestErrorAppLacksAccess  = "app_lacks_access"
	FileRequestErrorNoPermission    = "no_permission"
	FileRequestErrorEmailUnverified = "email_unverified"
	FileRequestErrorValidationError = "validation_error"
)

// CreateFileRequestError : There was an error creating the file request.
type CreateFileRequestError struct {
	dropbox.Tagged
}

// Valid tag values for CreateFileRequestError
const (
	CreateFileRequestErrorDisabledForTeam = "disabled_for_team"
	CreateFileRequestErrorOther           = "other"
	CreateFileRequestErrorNotFound        = "not_found"
	CreateFileRequestErrorNotAFolder      = "not_a_folder"
	CreateFileRequestErrorAppLacksAccess  = "app_lacks_access"
	CreateFileRequestErrorNoPermission    = "no_permission"
	CreateFileRequestErrorEmailUnverified = "email_unverified"
	CreateFileRequestErrorValidationError = "validation_error"
	CreateFileRequestErrorInvalidLocation = "invalid_location"
	CreateFileRequestErrorRateLimit       = "rate_limit"
)

// FileRequest : A `file request` <https://www.dropbox.com/help/9090> for
// receiving files into the user's Dropbox account.
type FileRequest struct {
	// Id : The ID of the file request.
	Id string `json:"id"`
	// Url : The URL of the file request.
	Url string `json:"url"`
	// Title : The title of the file request.
	Title string `json:"title"`
	// Destination : The path of the folder in the Dropbox where uploaded files
	// will be sent. This can be nil if the destination was removed. For apps
	// with the app folder permission, this will be relative to the app folder.
	Destination string `json:"destination,omitempty"`
	// Created : When this file request was created.
	Created time.Time `json:"created"`
	// Deadline : The deadline for this file request. Only set if the request
	// has a deadline.
	Deadline *FileRequestDeadline `json:"deadline,omitempty"`
	// IsOpen : Whether or not the file request is open. If the file request is
	// closed, it will not accept any more file submissions.
	IsOpen bool `json:"is_open"`
	// FileCount : The number of files this file request has received.
	FileCount int64 `json:"file_count"`
}

// NewFileRequest returns a new FileRequest instance
func NewFileRequest(Id string, Url string, Title string, Created time.Time, IsOpen bool, FileCount int64) *FileRequest {
	s := new(FileRequest)
	s.Id = Id
	s.Url = Url
	s.Title = Title
	s.Created = Created
	s.IsOpen = IsOpen
	s.FileCount = FileCount
	return s
}

// FileRequestDeadline : has no documentation (yet)
type FileRequestDeadline struct {
	// Deadline : The deadline for this file request.
	Deadline time.Time `json:"deadline"`
	// AllowLateUploads : If set, allow uploads after the deadline has passed.
	// These     uploads will be marked overdue.
	AllowLateUploads *GracePeriod `json:"allow_late_uploads,omitempty"`
}

// NewFileRequestDeadline returns a new FileRequestDeadline instance
func NewFileRequestDeadline(Deadline time.Time) *FileRequestDeadline {
	s := new(FileRequestDeadline)
	s.Deadline = Deadline
	return s
}

// GetFileRequestArgs : Arguments for `get`.
type GetFileRequestArgs struct {
	// Id : The ID of the file request to retrieve.
	Id string `json:"id"`
}

// NewGetFileRequestArgs returns a new GetFileRequestArgs instance
func NewGetFileRequestArgs(Id string) *GetFileRequestArgs {
	s := new(GetFileRequestArgs)
	s.Id = Id
	return s
}

// GetFileRequestError : There was an error retrieving the specified file
// request.
type GetFileRequestError struct {
	dropbox.Tagged
}

// Valid tag values for GetFileRequestError
const (
	GetFileRequestErrorDisabledForTeam = "disabled_for_team"
	GetFileRequestErrorOther           = "other"
	GetFileRequestErrorNotFound        = "not_found"
	GetFileRequestErrorNotAFolder      = "not_a_folder"
	GetFileRequestErrorAppLacksAccess  = "app_lacks_access"
	GetFileRequestErrorNoPermission    = "no_permission"
	GetFileRequestErrorEmailUnverified = "email_unverified"
	GetFileRequestErrorValidationError = "validation_error"
)

// GracePeriod : has no documentation (yet)
type GracePeriod struct {
	dropbox.Tagged
}

// Valid tag values for GracePeriod
const (
	GracePeriodOneDay     = "one_day"
	GracePeriodTwoDays    = "two_days"
	GracePeriodSevenDays  = "seven_days"
	GracePeriodThirtyDays = "thirty_days"
	GracePeriodAlways     = "always"
	GracePeriodOther      = "other"
)

// ListFileRequestsError : There was an error retrieving the file requests.
type ListFileRequestsError struct {
	dropbox.Tagged
}

// Valid tag values for ListFileRequestsError
const (
	ListFileRequestsErrorDisabledForTeam = "disabled_for_team"
	ListFileRequestsErrorOther           = "other"
)

// ListFileRequestsResult : Result for `list`.
type ListFileRequestsResult struct {
	// FileRequests : The file requests owned by this user. Apps with the app
	// folder permission will only see file requests in their app folder.
	FileRequests []*FileRequest `json:"file_requests"`
}

// NewListFileRequestsResult returns a new ListFileRequestsResult instance
func NewListFileRequestsResult(FileRequests []*FileRequest) *ListFileRequestsResult {
	s := new(ListFileRequestsResult)
	s.FileRequests = FileRequests
	return s
}

// UpdateFileRequestArgs : Arguments for `update`.
type UpdateFileRequestArgs struct {
	// Id : The ID of the file request to update.
	Id string `json:"id"`
	// Title : The new title of the file request. Must not be empty.
	Title string `json:"title,omitempty"`
	// Destination : The new path of the folder in the Dropbox where uploaded
	// files will be sent. For apps with the app folder permission, this will be
	// relative to the app folder.
	Destination string `json:"destination,omitempty"`
	// Deadline : The new deadline for the file request.
	Deadline *UpdateFileRequestDeadline `json:"deadline"`
	// Open : Whether to set this file request as open or closed.
	Open bool `json:"open,omitempty"`
}

// NewUpdateFileRequestArgs returns a new UpdateFileRequestArgs instance
func NewUpdateFileRequestArgs(Id string) *UpdateFileRequestArgs {
	s := new(UpdateFileRequestArgs)
	s.Id = Id
	s.Deadline = &UpdateFileRequestDeadline{Tagged: dropbox.Tagged{"no_update"}}
	return s
}

// UpdateFileRequestDeadline : has no documentation (yet)
type UpdateFileRequestDeadline struct {
	dropbox.Tagged
	// Update : If nil, the file request's deadline is cleared.
	Update *FileRequestDeadline `json:"update,omitempty"`
}

// Valid tag values for UpdateFileRequestDeadline
const (
	UpdateFileRequestDeadlineNoUpdate = "no_update"
	UpdateFileRequestDeadlineUpdate   = "update"
	UpdateFileRequestDeadlineOther    = "other"
)

// UnmarshalJSON deserializes into a UpdateFileRequestDeadline instance
func (u *UpdateFileRequestDeadline) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Update : If nil, the file request's deadline is cleared.
		Update json.RawMessage `json:"update,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "update":
		err = json.Unmarshal(body, &u.Update)

		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateFileRequestError : There is an error updating the file request.
type UpdateFileRequestError struct {
	dropbox.Tagged
}

// Valid tag values for UpdateFileRequestError
const (
	UpdateFileRequestErrorDisabledForTeam = "disabled_for_team"
	UpdateFileRequestErrorOther           = "other"
	UpdateFileRequestErrorNotFound        = "not_found"
	UpdateFileRequestErrorNotAFolder      = "not_a_folder"
	UpdateFileRequestErrorAppLacksAccess  = "app_lacks_access"
	UpdateFileRequestErrorNoPermission    = "no_permission"
	UpdateFileRequestErrorEmailUnverified = "email_unverified"
	UpdateFileRequestErrorValidationError = "validation_error"
)
