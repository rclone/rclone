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

// Package files : This namespace contains endpoints and data types for basic
// file operations.
package files

import (
	"encoding/json"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/file_properties"
)

// AddTagArg : has no documentation (yet)
type AddTagArg struct {
	// Path : Path to the item to be tagged.
	Path string `json:"path"`
	// TagText : The value of the tag to add. Will be automatically converted to
	// lowercase letters.
	TagText string `json:"tag_text"`
}

// NewAddTagArg returns a new AddTagArg instance
func NewAddTagArg(Path string, TagText string) *AddTagArg {
	s := new(AddTagArg)
	s.Path = Path
	s.TagText = TagText
	return s
}

// BaseTagError : has no documentation (yet)
type BaseTagError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for BaseTagError
const (
	BaseTagErrorPath  = "path"
	BaseTagErrorOther = "other"
)

// UnmarshalJSON deserializes into a BaseTagError instance
func (u *BaseTagError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// AddTagError : has no documentation (yet)
type AddTagError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for AddTagError
const (
	AddTagErrorPath        = "path"
	AddTagErrorOther       = "other"
	AddTagErrorTooManyTags = "too_many_tags"
)

// UnmarshalJSON deserializes into a AddTagError instance
func (u *AddTagError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// GetMetadataArg : has no documentation (yet)
type GetMetadataArg struct {
	// Path : The path of a file or folder on Dropbox.
	Path string `json:"path"`
	// IncludeMediaInfo : If true, `FileMetadata.media_info` is set for photo
	// and video.
	IncludeMediaInfo bool `json:"include_media_info"`
	// IncludeDeleted : If true, `DeletedMetadata` will be returned for deleted
	// file or folder, otherwise `LookupError.not_found` will be returned.
	IncludeDeleted bool `json:"include_deleted"`
	// IncludeHasExplicitSharedMembers : If true, the results will include a
	// flag for each file indicating whether or not  that file has any explicit
	// members.
	IncludeHasExplicitSharedMembers bool `json:"include_has_explicit_shared_members"`
	// IncludePropertyGroups : If set to a valid list of template IDs,
	// `FileMetadata.property_groups` is set if there exists property data
	// associated with the file and each of the listed templates.
	IncludePropertyGroups *file_properties.TemplateFilterBase `json:"include_property_groups,omitempty"`
}

// NewGetMetadataArg returns a new GetMetadataArg instance
func NewGetMetadataArg(Path string) *GetMetadataArg {
	s := new(GetMetadataArg)
	s.Path = Path
	s.IncludeMediaInfo = false
	s.IncludeDeleted = false
	s.IncludeHasExplicitSharedMembers = false
	return s
}

// AlphaGetMetadataArg : has no documentation (yet)
type AlphaGetMetadataArg struct {
	GetMetadataArg
	// IncludePropertyTemplates : If set to a valid list of template IDs,
	// `FileMetadata.property_groups` is set for files with custom properties.
	IncludePropertyTemplates []string `json:"include_property_templates,omitempty"`
}

// NewAlphaGetMetadataArg returns a new AlphaGetMetadataArg instance
func NewAlphaGetMetadataArg(Path string) *AlphaGetMetadataArg {
	s := new(AlphaGetMetadataArg)
	s.Path = Path
	s.IncludeMediaInfo = false
	s.IncludeDeleted = false
	s.IncludeHasExplicitSharedMembers = false
	return s
}

// GetMetadataError : has no documentation (yet)
type GetMetadataError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for GetMetadataError
const (
	GetMetadataErrorPath = "path"
)

// UnmarshalJSON deserializes into a GetMetadataError instance
func (u *GetMetadataError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// AlphaGetMetadataError : has no documentation (yet)
type AlphaGetMetadataError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
	// PropertiesError : has no documentation (yet)
	PropertiesError *file_properties.LookUpPropertiesError `json:"properties_error,omitempty"`
}

// Valid tag values for AlphaGetMetadataError
const (
	AlphaGetMetadataErrorPath            = "path"
	AlphaGetMetadataErrorPropertiesError = "properties_error"
)

// UnmarshalJSON deserializes into a AlphaGetMetadataError instance
func (u *AlphaGetMetadataError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
		// PropertiesError : has no documentation (yet)
		PropertiesError *file_properties.LookUpPropertiesError `json:"properties_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	case "properties_error":
		u.PropertiesError = w.PropertiesError

	}
	return nil
}

// CommitInfo : has no documentation (yet)
type CommitInfo struct {
	// Path : Path in the user's Dropbox to save the file.
	Path string `json:"path"`
	// Mode : Selects what to do if the file already exists.
	Mode *WriteMode `json:"mode"`
	// Autorename : If there's a conflict, as determined by `mode`, have the
	// Dropbox server try to autorename the file to avoid conflict.
	Autorename bool `json:"autorename"`
	// ClientModified : The value to store as the `client_modified` timestamp.
	// Dropbox automatically records the time at which the file was written to
	// the Dropbox servers. It can also record an additional timestamp, provided
	// by Dropbox desktop clients, mobile clients, and API apps of when the file
	// was actually created or modified.
	ClientModified *time.Time `json:"client_modified,omitempty"`
	// Mute : Normally, users are made aware of any file modifications in their
	// Dropbox account via notifications in the client software. If true, this
	// tells the clients that this modification shouldn't result in a user
	// notification.
	Mute bool `json:"mute"`
	// PropertyGroups : List of custom properties to add to file.
	PropertyGroups []*file_properties.PropertyGroup `json:"property_groups,omitempty"`
	// StrictConflict : Be more strict about how each `WriteMode` detects
	// conflict. For example, always return a conflict error when `mode` =
	// `WriteMode.update` and the given "rev" doesn't match the existing file's
	// "rev", even if the existing file has been deleted. This also forces a
	// conflict even when the target path refers to a file with identical
	// contents.
	StrictConflict bool `json:"strict_conflict"`
}

// NewCommitInfo returns a new CommitInfo instance
func NewCommitInfo(Path string) *CommitInfo {
	s := new(CommitInfo)
	s.Path = Path
	s.Mode = &WriteMode{Tagged: dropbox.Tagged{Tag: "add"}}
	s.Autorename = false
	s.Mute = false
	s.StrictConflict = false
	return s
}

// ContentSyncSetting : has no documentation (yet)
type ContentSyncSetting struct {
	// Id : Id of the item this setting is applied to.
	Id string `json:"id"`
	// SyncSetting : Setting for this item.
	SyncSetting *SyncSetting `json:"sync_setting"`
}

// NewContentSyncSetting returns a new ContentSyncSetting instance
func NewContentSyncSetting(Id string, SyncSetting *SyncSetting) *ContentSyncSetting {
	s := new(ContentSyncSetting)
	s.Id = Id
	s.SyncSetting = SyncSetting
	return s
}

// ContentSyncSettingArg : has no documentation (yet)
type ContentSyncSettingArg struct {
	// Id : Id of the item this setting is applied to.
	Id string `json:"id"`
	// SyncSetting : Setting for this item.
	SyncSetting *SyncSettingArg `json:"sync_setting"`
}

// NewContentSyncSettingArg returns a new ContentSyncSettingArg instance
func NewContentSyncSettingArg(Id string, SyncSetting *SyncSettingArg) *ContentSyncSettingArg {
	s := new(ContentSyncSettingArg)
	s.Id = Id
	s.SyncSetting = SyncSetting
	return s
}

// CreateFolderArg : has no documentation (yet)
type CreateFolderArg struct {
	// Path : Path in the user's Dropbox to create.
	Path string `json:"path"`
	// Autorename : If there's a conflict, have the Dropbox server try to
	// autorename the folder to avoid the conflict.
	Autorename bool `json:"autorename"`
}

// NewCreateFolderArg returns a new CreateFolderArg instance
func NewCreateFolderArg(Path string) *CreateFolderArg {
	s := new(CreateFolderArg)
	s.Path = Path
	s.Autorename = false
	return s
}

// CreateFolderBatchArg : has no documentation (yet)
type CreateFolderBatchArg struct {
	// Paths : List of paths to be created in the user's Dropbox. Duplicate path
	// arguments in the batch are considered only once.
	Paths []string `json:"paths"`
	// Autorename : If there's a conflict, have the Dropbox server try to
	// autorename the folder to avoid the conflict.
	Autorename bool `json:"autorename"`
	// ForceAsync : Whether to force the create to happen asynchronously.
	ForceAsync bool `json:"force_async"`
}

// NewCreateFolderBatchArg returns a new CreateFolderBatchArg instance
func NewCreateFolderBatchArg(Paths []string) *CreateFolderBatchArg {
	s := new(CreateFolderBatchArg)
	s.Paths = Paths
	s.Autorename = false
	s.ForceAsync = false
	return s
}

// CreateFolderBatchError : has no documentation (yet)
type CreateFolderBatchError struct {
	dropbox.Tagged
}

// Valid tag values for CreateFolderBatchError
const (
	CreateFolderBatchErrorTooManyFiles = "too_many_files"
	CreateFolderBatchErrorOther        = "other"
)

// CreateFolderBatchJobStatus : has no documentation (yet)
type CreateFolderBatchJobStatus struct {
	dropbox.Tagged
	// Complete : The batch create folder has finished.
	Complete *CreateFolderBatchResult `json:"complete,omitempty"`
	// Failed : The batch create folder has failed.
	Failed *CreateFolderBatchError `json:"failed,omitempty"`
}

// Valid tag values for CreateFolderBatchJobStatus
const (
	CreateFolderBatchJobStatusInProgress = "in_progress"
	CreateFolderBatchJobStatusComplete   = "complete"
	CreateFolderBatchJobStatusFailed     = "failed"
	CreateFolderBatchJobStatusOther      = "other"
)

// UnmarshalJSON deserializes into a CreateFolderBatchJobStatus instance
func (u *CreateFolderBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : The batch create folder has failed.
		Failed *CreateFolderBatchError `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	case "failed":
		u.Failed = w.Failed

	}
	return nil
}

// CreateFolderBatchLaunch : Result returned by `createFolderBatch` that may
// either launch an asynchronous job or complete synchronously.
type CreateFolderBatchLaunch struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : has no documentation (yet)
	Complete *CreateFolderBatchResult `json:"complete,omitempty"`
}

// Valid tag values for CreateFolderBatchLaunch
const (
	CreateFolderBatchLaunchAsyncJobId = "async_job_id"
	CreateFolderBatchLaunchComplete   = "complete"
	CreateFolderBatchLaunchOther      = "other"
)

// UnmarshalJSON deserializes into a CreateFolderBatchLaunch instance
func (u *CreateFolderBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AsyncJobId : This response indicates that the processing is
		// asynchronous. The string is an id that can be used to obtain the
		// status of the asynchronous job.
		AsyncJobId string `json:"async_job_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		u.AsyncJobId = w.AsyncJobId

	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// FileOpsResult : has no documentation (yet)
type FileOpsResult struct {
}

// NewFileOpsResult returns a new FileOpsResult instance
func NewFileOpsResult() *FileOpsResult {
	s := new(FileOpsResult)
	return s
}

// CreateFolderBatchResult : has no documentation (yet)
type CreateFolderBatchResult struct {
	FileOpsResult
	// Entries : Each entry in `CreateFolderBatchArg.paths` will appear at the
	// same position inside `CreateFolderBatchResult.entries`.
	Entries []*CreateFolderBatchResultEntry `json:"entries"`
}

// NewCreateFolderBatchResult returns a new CreateFolderBatchResult instance
func NewCreateFolderBatchResult(Entries []*CreateFolderBatchResultEntry) *CreateFolderBatchResult {
	s := new(CreateFolderBatchResult)
	s.Entries = Entries
	return s
}

// CreateFolderBatchResultEntry : has no documentation (yet)
type CreateFolderBatchResultEntry struct {
	dropbox.Tagged
	// Success : has no documentation (yet)
	Success *CreateFolderEntryResult `json:"success,omitempty"`
	// Failure : has no documentation (yet)
	Failure *CreateFolderEntryError `json:"failure,omitempty"`
}

// Valid tag values for CreateFolderBatchResultEntry
const (
	CreateFolderBatchResultEntrySuccess = "success"
	CreateFolderBatchResultEntryFailure = "failure"
)

// UnmarshalJSON deserializes into a CreateFolderBatchResultEntry instance
func (u *CreateFolderBatchResultEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failure : has no documentation (yet)
		Failure *CreateFolderEntryError `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		if err = json.Unmarshal(body, &u.Success); err != nil {
			return err
		}

	case "failure":
		u.Failure = w.Failure

	}
	return nil
}

// CreateFolderEntryError : has no documentation (yet)
type CreateFolderEntryError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *WriteError `json:"path,omitempty"`
}

// Valid tag values for CreateFolderEntryError
const (
	CreateFolderEntryErrorPath  = "path"
	CreateFolderEntryErrorOther = "other"
)

// UnmarshalJSON deserializes into a CreateFolderEntryError instance
func (u *CreateFolderEntryError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *WriteError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// CreateFolderEntryResult : has no documentation (yet)
type CreateFolderEntryResult struct {
	// Metadata : Metadata of the created folder.
	Metadata *FolderMetadata `json:"metadata"`
}

// NewCreateFolderEntryResult returns a new CreateFolderEntryResult instance
func NewCreateFolderEntryResult(Metadata *FolderMetadata) *CreateFolderEntryResult {
	s := new(CreateFolderEntryResult)
	s.Metadata = Metadata
	return s
}

// CreateFolderError : has no documentation (yet)
type CreateFolderError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *WriteError `json:"path,omitempty"`
}

// Valid tag values for CreateFolderError
const (
	CreateFolderErrorPath = "path"
)

// UnmarshalJSON deserializes into a CreateFolderError instance
func (u *CreateFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *WriteError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// CreateFolderResult : has no documentation (yet)
type CreateFolderResult struct {
	FileOpsResult
	// Metadata : Metadata of the created folder.
	Metadata *FolderMetadata `json:"metadata"`
}

// NewCreateFolderResult returns a new CreateFolderResult instance
func NewCreateFolderResult(Metadata *FolderMetadata) *CreateFolderResult {
	s := new(CreateFolderResult)
	s.Metadata = Metadata
	return s
}

// DeleteArg : has no documentation (yet)
type DeleteArg struct {
	// Path : Path in the user's Dropbox to delete.
	Path string `json:"path"`
	// ParentRev : Perform delete if given "rev" matches the existing file's
	// latest "rev". This field does not support deleting a folder.
	ParentRev string `json:"parent_rev,omitempty"`
}

// NewDeleteArg returns a new DeleteArg instance
func NewDeleteArg(Path string) *DeleteArg {
	s := new(DeleteArg)
	s.Path = Path
	return s
}

// DeleteBatchArg : has no documentation (yet)
type DeleteBatchArg struct {
	// Entries : has no documentation (yet)
	Entries []*DeleteArg `json:"entries"`
}

// NewDeleteBatchArg returns a new DeleteBatchArg instance
func NewDeleteBatchArg(Entries []*DeleteArg) *DeleteBatchArg {
	s := new(DeleteBatchArg)
	s.Entries = Entries
	return s
}

// DeleteBatchError : has no documentation (yet)
type DeleteBatchError struct {
	dropbox.Tagged
}

// Valid tag values for DeleteBatchError
const (
	DeleteBatchErrorTooManyWriteOperations = "too_many_write_operations"
	DeleteBatchErrorOther                  = "other"
)

// DeleteBatchJobStatus : has no documentation (yet)
type DeleteBatchJobStatus struct {
	dropbox.Tagged
	// Complete : The batch delete has finished.
	Complete *DeleteBatchResult `json:"complete,omitempty"`
	// Failed : The batch delete has failed.
	Failed *DeleteBatchError `json:"failed,omitempty"`
}

// Valid tag values for DeleteBatchJobStatus
const (
	DeleteBatchJobStatusInProgress = "in_progress"
	DeleteBatchJobStatusComplete   = "complete"
	DeleteBatchJobStatusFailed     = "failed"
	DeleteBatchJobStatusOther      = "other"
)

// UnmarshalJSON deserializes into a DeleteBatchJobStatus instance
func (u *DeleteBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : The batch delete has failed.
		Failed *DeleteBatchError `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	case "failed":
		u.Failed = w.Failed

	}
	return nil
}

// DeleteBatchLaunch : Result returned by `deleteBatch` that may either launch
// an asynchronous job or complete synchronously.
type DeleteBatchLaunch struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : has no documentation (yet)
	Complete *DeleteBatchResult `json:"complete,omitempty"`
}

// Valid tag values for DeleteBatchLaunch
const (
	DeleteBatchLaunchAsyncJobId = "async_job_id"
	DeleteBatchLaunchComplete   = "complete"
	DeleteBatchLaunchOther      = "other"
)

// UnmarshalJSON deserializes into a DeleteBatchLaunch instance
func (u *DeleteBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AsyncJobId : This response indicates that the processing is
		// asynchronous. The string is an id that can be used to obtain the
		// status of the asynchronous job.
		AsyncJobId string `json:"async_job_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		u.AsyncJobId = w.AsyncJobId

	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// DeleteBatchResult : has no documentation (yet)
type DeleteBatchResult struct {
	FileOpsResult
	// Entries : Each entry in `DeleteBatchArg.entries` will appear at the same
	// position inside `DeleteBatchResult.entries`.
	Entries []*DeleteBatchResultEntry `json:"entries"`
}

// NewDeleteBatchResult returns a new DeleteBatchResult instance
func NewDeleteBatchResult(Entries []*DeleteBatchResultEntry) *DeleteBatchResult {
	s := new(DeleteBatchResult)
	s.Entries = Entries
	return s
}

// DeleteBatchResultData : has no documentation (yet)
type DeleteBatchResultData struct {
	// Metadata : Metadata of the deleted object.
	Metadata IsMetadata `json:"metadata"`
}

// NewDeleteBatchResultData returns a new DeleteBatchResultData instance
func NewDeleteBatchResultData(Metadata IsMetadata) *DeleteBatchResultData {
	s := new(DeleteBatchResultData)
	s.Metadata = Metadata
	return s
}

// UnmarshalJSON deserializes into a DeleteBatchResultData instance
func (u *DeleteBatchResultData) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : Metadata of the deleted object.
		Metadata json.RawMessage `json:"metadata"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	return nil
}

// DeleteBatchResultEntry : has no documentation (yet)
type DeleteBatchResultEntry struct {
	dropbox.Tagged
	// Success : has no documentation (yet)
	Success *DeleteBatchResultData `json:"success,omitempty"`
	// Failure : has no documentation (yet)
	Failure *DeleteError `json:"failure,omitempty"`
}

// Valid tag values for DeleteBatchResultEntry
const (
	DeleteBatchResultEntrySuccess = "success"
	DeleteBatchResultEntryFailure = "failure"
)

// UnmarshalJSON deserializes into a DeleteBatchResultEntry instance
func (u *DeleteBatchResultEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failure : has no documentation (yet)
		Failure *DeleteError `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		if err = json.Unmarshal(body, &u.Success); err != nil {
			return err
		}

	case "failure":
		u.Failure = w.Failure

	}
	return nil
}

// DeleteError : has no documentation (yet)
type DeleteError struct {
	dropbox.Tagged
	// PathLookup : has no documentation (yet)
	PathLookup *LookupError `json:"path_lookup,omitempty"`
	// PathWrite : has no documentation (yet)
	PathWrite *WriteError `json:"path_write,omitempty"`
}

// Valid tag values for DeleteError
const (
	DeleteErrorPathLookup             = "path_lookup"
	DeleteErrorPathWrite              = "path_write"
	DeleteErrorTooManyWriteOperations = "too_many_write_operations"
	DeleteErrorTooManyFiles           = "too_many_files"
	DeleteErrorOther                  = "other"
)

// UnmarshalJSON deserializes into a DeleteError instance
func (u *DeleteError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PathLookup : has no documentation (yet)
		PathLookup *LookupError `json:"path_lookup,omitempty"`
		// PathWrite : has no documentation (yet)
		PathWrite *WriteError `json:"path_write,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path_lookup":
		u.PathLookup = w.PathLookup

	case "path_write":
		u.PathWrite = w.PathWrite

	}
	return nil
}

// DeleteResult : has no documentation (yet)
type DeleteResult struct {
	FileOpsResult
	// Metadata : Metadata of the deleted object.
	Metadata IsMetadata `json:"metadata"`
}

// NewDeleteResult returns a new DeleteResult instance
func NewDeleteResult(Metadata IsMetadata) *DeleteResult {
	s := new(DeleteResult)
	s.Metadata = Metadata
	return s
}

// UnmarshalJSON deserializes into a DeleteResult instance
func (u *DeleteResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : Metadata of the deleted object.
		Metadata json.RawMessage `json:"metadata"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	return nil
}

// Metadata : Metadata for a file or folder.
type Metadata struct {
	// Name : The last component of the path (including extension). This never
	// contains a slash.
	Name string `json:"name"`
	// PathLower : The lowercased full path in the user's Dropbox. This always
	// starts with a slash. This field will be null if the file or folder is not
	// mounted.
	PathLower string `json:"path_lower,omitempty"`
	// PathDisplay : The cased path to be used for display purposes only. In
	// rare instances the casing will not correctly match the user's filesystem,
	// but this behavior will match the path provided in the Core API v1, and at
	// least the last path component will have the correct casing. Changes to
	// only the casing of paths won't be returned by `listFolderContinue`. This
	// field will be null if the file or folder is not mounted.
	PathDisplay string `json:"path_display,omitempty"`
	// ParentSharedFolderId : Please use
	// `FileSharingInfo.parent_shared_folder_id` or
	// `FolderSharingInfo.parent_shared_folder_id` instead.
	ParentSharedFolderId string `json:"parent_shared_folder_id,omitempty"`
	// PreviewUrl : The preview URL of the file.
	PreviewUrl string `json:"preview_url,omitempty"`
}

// NewMetadata returns a new Metadata instance
func NewMetadata(Name string) *Metadata {
	s := new(Metadata)
	s.Name = Name
	return s
}

// IsMetadata is the interface type for Metadata and its subtypes
type IsMetadata interface {
	IsMetadata()
}

// IsMetadata implements the IsMetadata interface
func (u *Metadata) IsMetadata() {}

type metadataUnion struct {
	dropbox.Tagged
	// File : has no documentation (yet)
	File *FileMetadata `json:"file,omitempty"`
	// Folder : has no documentation (yet)
	Folder *FolderMetadata `json:"folder,omitempty"`
	// Deleted : has no documentation (yet)
	Deleted *DeletedMetadata `json:"deleted,omitempty"`
}

// Valid tag values for Metadata
const (
	MetadataFile    = "file"
	MetadataFolder  = "folder"
	MetadataDeleted = "deleted"
)

// UnmarshalJSON deserializes into a metadataUnion instance
func (u *metadataUnion) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "file":
		if err = json.Unmarshal(body, &u.File); err != nil {
			return err
		}

	case "folder":
		if err = json.Unmarshal(body, &u.Folder); err != nil {
			return err
		}

	case "deleted":
		if err = json.Unmarshal(body, &u.Deleted); err != nil {
			return err
		}

	}
	return nil
}

// IsMetadataFromJSON converts JSON to a concrete IsMetadata instance
func IsMetadataFromJSON(data []byte) (IsMetadata, error) {
	var t metadataUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "file":
		return t.File, nil

	case "folder":
		return t.Folder, nil

	case "deleted":
		return t.Deleted, nil

	}
	return nil, nil
}

// DeletedMetadata : Indicates that there used to be a file or folder at this
// path, but it no longer exists.
type DeletedMetadata struct {
	Metadata
}

// NewDeletedMetadata returns a new DeletedMetadata instance
func NewDeletedMetadata(Name string) *DeletedMetadata {
	s := new(DeletedMetadata)
	s.Name = Name
	return s
}

// Dimensions : Dimensions for a photo or video.
type Dimensions struct {
	// Height : Height of the photo/video.
	Height uint64 `json:"height"`
	// Width : Width of the photo/video.
	Width uint64 `json:"width"`
}

// NewDimensions returns a new Dimensions instance
func NewDimensions(Height uint64, Width uint64) *Dimensions {
	s := new(Dimensions)
	s.Height = Height
	s.Width = Width
	return s
}

// DownloadArg : has no documentation (yet)
type DownloadArg struct {
	// Path : The path of the file to download.
	Path string `json:"path"`
	// Rev : Please specify revision in `path` instead.
	Rev string `json:"rev,omitempty"`
	// ExtraHeaders can be used to pass Range, If-None-Match headers
	ExtraHeaders map[string]string `json:"-"`
}

// NewDownloadArg returns a new DownloadArg instance
func NewDownloadArg(Path string) *DownloadArg {
	s := new(DownloadArg)
	s.Path = Path
	return s
}

// DownloadError : has no documentation (yet)
type DownloadError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for DownloadError
const (
	DownloadErrorPath            = "path"
	DownloadErrorUnsupportedFile = "unsupported_file"
	DownloadErrorOther           = "other"
)

// UnmarshalJSON deserializes into a DownloadError instance
func (u *DownloadError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// DownloadZipArg : has no documentation (yet)
type DownloadZipArg struct {
	// Path : The path of the folder to download.
	Path string `json:"path"`
}

// NewDownloadZipArg returns a new DownloadZipArg instance
func NewDownloadZipArg(Path string) *DownloadZipArg {
	s := new(DownloadZipArg)
	s.Path = Path
	return s
}

// DownloadZipError : has no documentation (yet)
type DownloadZipError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for DownloadZipError
const (
	DownloadZipErrorPath         = "path"
	DownloadZipErrorTooLarge     = "too_large"
	DownloadZipErrorTooManyFiles = "too_many_files"
	DownloadZipErrorOther        = "other"
)

// UnmarshalJSON deserializes into a DownloadZipError instance
func (u *DownloadZipError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// DownloadZipResult : has no documentation (yet)
type DownloadZipResult struct {
	// Metadata : has no documentation (yet)
	Metadata *FolderMetadata `json:"metadata"`
}

// NewDownloadZipResult returns a new DownloadZipResult instance
func NewDownloadZipResult(Metadata *FolderMetadata) *DownloadZipResult {
	s := new(DownloadZipResult)
	s.Metadata = Metadata
	return s
}

// ExportArg : has no documentation (yet)
type ExportArg struct {
	// Path : The path of the file to be exported.
	Path string `json:"path"`
	// ExportFormat : The file format to which the file should be exported. This
	// must be one of the formats listed in the file's export_options returned
	// by `getMetadata`. If none is specified, the default format (specified in
	// export_as in file metadata) will be used.
	ExportFormat string `json:"export_format,omitempty"`
}

// NewExportArg returns a new ExportArg instance
func NewExportArg(Path string) *ExportArg {
	s := new(ExportArg)
	s.Path = Path
	return s
}

// ExportError : has no documentation (yet)
type ExportError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for ExportError
const (
	ExportErrorPath                = "path"
	ExportErrorNonExportable       = "non_exportable"
	ExportErrorInvalidExportFormat = "invalid_export_format"
	ExportErrorRetryError          = "retry_error"
	ExportErrorOther               = "other"
)

// UnmarshalJSON deserializes into a ExportError instance
func (u *ExportError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// ExportInfo : Export information for a file.
type ExportInfo struct {
	// ExportAs : Format to which the file can be exported to.
	ExportAs string `json:"export_as,omitempty"`
	// ExportOptions : Additional formats to which the file can be exported.
	// These values can be specified as the export_format in /files/export.
	ExportOptions []string `json:"export_options,omitempty"`
}

// NewExportInfo returns a new ExportInfo instance
func NewExportInfo() *ExportInfo {
	s := new(ExportInfo)
	return s
}

// ExportMetadata : has no documentation (yet)
type ExportMetadata struct {
	// Name : The last component of the path (including extension). This never
	// contains a slash.
	Name string `json:"name"`
	// Size : The file size in bytes.
	Size uint64 `json:"size"`
	// ExportHash : A hash based on the exported file content. This field can be
	// used to verify data integrity. Similar to content hash. For more
	// information see our `Content hash`
	// <https://www.dropbox.com/developers/reference/content-hash> page.
	ExportHash string `json:"export_hash,omitempty"`
	// PaperRevision : If the file is a Paper doc, this gives the latest doc
	// revision which can be used in `paperUpdate`.
	PaperRevision int64 `json:"paper_revision,omitempty"`
}

// NewExportMetadata returns a new ExportMetadata instance
func NewExportMetadata(Name string, Size uint64) *ExportMetadata {
	s := new(ExportMetadata)
	s.Name = Name
	s.Size = Size
	return s
}

// ExportResult : has no documentation (yet)
type ExportResult struct {
	// ExportMetadata : Metadata for the exported version of the file.
	ExportMetadata *ExportMetadata `json:"export_metadata"`
	// FileMetadata : Metadata for the original file.
	FileMetadata *FileMetadata `json:"file_metadata"`
}

// NewExportResult returns a new ExportResult instance
func NewExportResult(ExportMetadata *ExportMetadata, FileMetadata *FileMetadata) *ExportResult {
	s := new(ExportResult)
	s.ExportMetadata = ExportMetadata
	s.FileMetadata = FileMetadata
	return s
}

// FileCategory : has no documentation (yet)
type FileCategory struct {
	dropbox.Tagged
}

// Valid tag values for FileCategory
const (
	FileCategoryImage        = "image"
	FileCategoryDocument     = "document"
	FileCategoryPdf          = "pdf"
	FileCategorySpreadsheet  = "spreadsheet"
	FileCategoryPresentation = "presentation"
	FileCategoryAudio        = "audio"
	FileCategoryVideo        = "video"
	FileCategoryFolder       = "folder"
	FileCategoryPaper        = "paper"
	FileCategoryOthers       = "others"
	FileCategoryOther        = "other"
)

// FileLock : has no documentation (yet)
type FileLock struct {
	// Content : The lock description.
	Content *FileLockContent `json:"content"`
}

// NewFileLock returns a new FileLock instance
func NewFileLock(Content *FileLockContent) *FileLock {
	s := new(FileLock)
	s.Content = Content
	return s
}

// FileLockContent : has no documentation (yet)
type FileLockContent struct {
	dropbox.Tagged
	// SingleUser : A lock held by a single user.
	SingleUser *SingleUserLock `json:"single_user,omitempty"`
}

// Valid tag values for FileLockContent
const (
	FileLockContentUnlocked   = "unlocked"
	FileLockContentSingleUser = "single_user"
	FileLockContentOther      = "other"
)

// UnmarshalJSON deserializes into a FileLockContent instance
func (u *FileLockContent) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "single_user":
		if err = json.Unmarshal(body, &u.SingleUser); err != nil {
			return err
		}

	}
	return nil
}

// FileLockMetadata : has no documentation (yet)
type FileLockMetadata struct {
	// IsLockholder : True if caller holds the file lock.
	IsLockholder bool `json:"is_lockholder,omitempty"`
	// LockholderName : The display name of the lock holder.
	LockholderName string `json:"lockholder_name,omitempty"`
	// LockholderAccountId : The account ID of the lock holder if known.
	LockholderAccountId string `json:"lockholder_account_id,omitempty"`
	// Created : The timestamp of the lock was created.
	Created *time.Time `json:"created,omitempty"`
}

// NewFileLockMetadata returns a new FileLockMetadata instance
func NewFileLockMetadata() *FileLockMetadata {
	s := new(FileLockMetadata)
	return s
}

// FileMetadata : has no documentation (yet)
type FileMetadata struct {
	Metadata
	// Id : A unique identifier for the file.
	Id string `json:"id"`
	// ClientModified : For files, this is the modification time set by the
	// desktop client when the file was added to Dropbox. Since this time is not
	// verified (the Dropbox server stores whatever the desktop client sends
	// up), this should only be used for display purposes (such as sorting) and
	// not, for example, to determine if a file has changed or not.
	ClientModified time.Time `json:"client_modified"`
	// ServerModified : The last time the file was modified on Dropbox.
	ServerModified time.Time `json:"server_modified"`
	// Rev : A unique identifier for the current revision of a file. This field
	// is the same rev as elsewhere in the API and can be used to detect changes
	// and avoid conflicts.
	Rev string `json:"rev"`
	// Size : The file size in bytes.
	Size uint64 `json:"size"`
	// MediaInfo : Additional information if the file is a photo or video. This
	// field will not be set on entries returned by `listFolder`,
	// `listFolderContinue`, or `getThumbnailBatch`, starting December 2, 2019.
	MediaInfo *MediaInfo `json:"media_info,omitempty"`
	// SymlinkInfo : Set if this file is a symlink.
	SymlinkInfo *SymlinkInfo `json:"symlink_info,omitempty"`
	// SharingInfo : Set if this file is contained in a shared folder.
	SharingInfo *FileSharingInfo `json:"sharing_info,omitempty"`
	// IsDownloadable : If true, file can be downloaded directly; else the file
	// must be exported.
	IsDownloadable bool `json:"is_downloadable"`
	// ExportInfo : Information about format this file can be exported to. This
	// filed must be set if `is_downloadable` is set to false.
	ExportInfo *ExportInfo `json:"export_info,omitempty"`
	// PropertyGroups : Additional information if the file has custom properties
	// with the property template specified.
	PropertyGroups []*file_properties.PropertyGroup `json:"property_groups,omitempty"`
	// HasExplicitSharedMembers : This flag will only be present if
	// include_has_explicit_shared_members  is true in `listFolder` or
	// `getMetadata`. If this  flag is present, it will be true if this file has
	// any explicit shared  members. This is different from sharing_info in that
	// this could be true  in the case where a file has explicit members but is
	// not contained within  a shared folder.
	HasExplicitSharedMembers bool `json:"has_explicit_shared_members,omitempty"`
	// ContentHash : A hash of the file content. This field can be used to
	// verify data integrity. For more information see our `Content hash`
	// <https://www.dropbox.com/developers/reference/content-hash> page.
	ContentHash string `json:"content_hash,omitempty"`
	// FileLockInfo : If present, the metadata associated with the file's
	// current lock.
	FileLockInfo *FileLockMetadata `json:"file_lock_info,omitempty"`
}

// NewFileMetadata returns a new FileMetadata instance
func NewFileMetadata(Name string, Id string, ClientModified time.Time, ServerModified time.Time, Rev string, Size uint64) *FileMetadata {
	s := new(FileMetadata)
	s.Name = Name
	s.Id = Id
	s.ClientModified = ClientModified
	s.ServerModified = ServerModified
	s.Rev = Rev
	s.Size = Size
	s.IsDownloadable = true
	return s
}

// SharingInfo : Sharing info for a file or folder.
type SharingInfo struct {
	// ReadOnly : True if the file or folder is inside a read-only shared
	// folder.
	ReadOnly bool `json:"read_only"`
}

// NewSharingInfo returns a new SharingInfo instance
func NewSharingInfo(ReadOnly bool) *SharingInfo {
	s := new(SharingInfo)
	s.ReadOnly = ReadOnly
	return s
}

// FileSharingInfo : Sharing info for a file which is contained by a shared
// folder.
type FileSharingInfo struct {
	SharingInfo
	// ParentSharedFolderId : ID of shared folder that holds this file.
	ParentSharedFolderId string `json:"parent_shared_folder_id"`
	// ModifiedBy : The last user who modified the file. This field will be null
	// if the user's account has been deleted.
	ModifiedBy string `json:"modified_by,omitempty"`
}

// NewFileSharingInfo returns a new FileSharingInfo instance
func NewFileSharingInfo(ReadOnly bool, ParentSharedFolderId string) *FileSharingInfo {
	s := new(FileSharingInfo)
	s.ReadOnly = ReadOnly
	s.ParentSharedFolderId = ParentSharedFolderId
	return s
}

// FileStatus : has no documentation (yet)
type FileStatus struct {
	dropbox.Tagged
}

// Valid tag values for FileStatus
const (
	FileStatusActive  = "active"
	FileStatusDeleted = "deleted"
	FileStatusOther   = "other"
)

// FolderMetadata : has no documentation (yet)
type FolderMetadata struct {
	Metadata
	// Id : A unique identifier for the folder.
	Id string `json:"id"`
	// SharedFolderId : Please use `sharing_info` instead.
	SharedFolderId string `json:"shared_folder_id,omitempty"`
	// SharingInfo : Set if the folder is contained in a shared folder or is a
	// shared folder mount point.
	SharingInfo *FolderSharingInfo `json:"sharing_info,omitempty"`
	// PropertyGroups : Additional information if the file has custom properties
	// with the property template specified. Note that only properties
	// associated with user-owned templates, not team-owned templates, can be
	// attached to folders.
	PropertyGroups []*file_properties.PropertyGroup `json:"property_groups,omitempty"`
}

// NewFolderMetadata returns a new FolderMetadata instance
func NewFolderMetadata(Name string, Id string) *FolderMetadata {
	s := new(FolderMetadata)
	s.Name = Name
	s.Id = Id
	return s
}

// FolderSharingInfo : Sharing info for a folder which is contained in a shared
// folder or is a shared folder mount point.
type FolderSharingInfo struct {
	SharingInfo
	// ParentSharedFolderId : Set if the folder is contained by a shared folder.
	ParentSharedFolderId string `json:"parent_shared_folder_id,omitempty"`
	// SharedFolderId : If this folder is a shared folder mount point, the ID of
	// the shared folder mounted at this location.
	SharedFolderId string `json:"shared_folder_id,omitempty"`
	// TraverseOnly : Specifies that the folder can only be traversed and the
	// user can only see a limited subset of the contents of this folder because
	// they don't have read access to this folder. They do, however, have access
	// to some sub folder.
	TraverseOnly bool `json:"traverse_only"`
	// NoAccess : Specifies that the folder cannot be accessed by the user.
	NoAccess bool `json:"no_access"`
}

// NewFolderSharingInfo returns a new FolderSharingInfo instance
func NewFolderSharingInfo(ReadOnly bool) *FolderSharingInfo {
	s := new(FolderSharingInfo)
	s.ReadOnly = ReadOnly
	s.TraverseOnly = false
	s.NoAccess = false
	return s
}

// GetCopyReferenceArg : has no documentation (yet)
type GetCopyReferenceArg struct {
	// Path : The path to the file or folder you want to get a copy reference
	// to.
	Path string `json:"path"`
}

// NewGetCopyReferenceArg returns a new GetCopyReferenceArg instance
func NewGetCopyReferenceArg(Path string) *GetCopyReferenceArg {
	s := new(GetCopyReferenceArg)
	s.Path = Path
	return s
}

// GetCopyReferenceError : has no documentation (yet)
type GetCopyReferenceError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for GetCopyReferenceError
const (
	GetCopyReferenceErrorPath  = "path"
	GetCopyReferenceErrorOther = "other"
)

// UnmarshalJSON deserializes into a GetCopyReferenceError instance
func (u *GetCopyReferenceError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// GetCopyReferenceResult : has no documentation (yet)
type GetCopyReferenceResult struct {
	// Metadata : Metadata of the file or folder.
	Metadata IsMetadata `json:"metadata"`
	// CopyReference : A copy reference to the file or folder.
	CopyReference string `json:"copy_reference"`
	// Expires : The expiration date of the copy reference. This value is
	// currently set to be far enough in the future so that expiration is
	// effectively not an issue.
	Expires time.Time `json:"expires"`
}

// NewGetCopyReferenceResult returns a new GetCopyReferenceResult instance
func NewGetCopyReferenceResult(Metadata IsMetadata, CopyReference string, Expires time.Time) *GetCopyReferenceResult {
	s := new(GetCopyReferenceResult)
	s.Metadata = Metadata
	s.CopyReference = CopyReference
	s.Expires = Expires
	return s
}

// UnmarshalJSON deserializes into a GetCopyReferenceResult instance
func (u *GetCopyReferenceResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : Metadata of the file or folder.
		Metadata json.RawMessage `json:"metadata"`
		// CopyReference : A copy reference to the file or folder.
		CopyReference string `json:"copy_reference"`
		// Expires : The expiration date of the copy reference. This value is
		// currently set to be far enough in the future so that expiration is
		// effectively not an issue.
		Expires time.Time `json:"expires"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	u.CopyReference = w.CopyReference
	u.Expires = w.Expires
	return nil
}

// GetTagsArg : has no documentation (yet)
type GetTagsArg struct {
	// Paths : Path to the items.
	Paths []string `json:"paths"`
}

// NewGetTagsArg returns a new GetTagsArg instance
func NewGetTagsArg(Paths []string) *GetTagsArg {
	s := new(GetTagsArg)
	s.Paths = Paths
	return s
}

// GetTagsResult : has no documentation (yet)
type GetTagsResult struct {
	// PathsToTags : List of paths and their corresponding tags.
	PathsToTags []*PathToTags `json:"paths_to_tags"`
}

// NewGetTagsResult returns a new GetTagsResult instance
func NewGetTagsResult(PathsToTags []*PathToTags) *GetTagsResult {
	s := new(GetTagsResult)
	s.PathsToTags = PathsToTags
	return s
}

// GetTemporaryLinkArg : has no documentation (yet)
type GetTemporaryLinkArg struct {
	// Path : The path to the file you want a temporary link to.
	Path string `json:"path"`
}

// NewGetTemporaryLinkArg returns a new GetTemporaryLinkArg instance
func NewGetTemporaryLinkArg(Path string) *GetTemporaryLinkArg {
	s := new(GetTemporaryLinkArg)
	s.Path = Path
	return s
}

// GetTemporaryLinkError : has no documentation (yet)
type GetTemporaryLinkError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for GetTemporaryLinkError
const (
	GetTemporaryLinkErrorPath             = "path"
	GetTemporaryLinkErrorEmailNotVerified = "email_not_verified"
	GetTemporaryLinkErrorUnsupportedFile  = "unsupported_file"
	GetTemporaryLinkErrorNotAllowed       = "not_allowed"
	GetTemporaryLinkErrorOther            = "other"
)

// UnmarshalJSON deserializes into a GetTemporaryLinkError instance
func (u *GetTemporaryLinkError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// GetTemporaryLinkResult : has no documentation (yet)
type GetTemporaryLinkResult struct {
	// Metadata : Metadata of the file.
	Metadata *FileMetadata `json:"metadata"`
	// Link : The temporary link which can be used to stream content the file.
	Link string `json:"link"`
}

// NewGetTemporaryLinkResult returns a new GetTemporaryLinkResult instance
func NewGetTemporaryLinkResult(Metadata *FileMetadata, Link string) *GetTemporaryLinkResult {
	s := new(GetTemporaryLinkResult)
	s.Metadata = Metadata
	s.Link = Link
	return s
}

// GetTemporaryUploadLinkArg : has no documentation (yet)
type GetTemporaryUploadLinkArg struct {
	// CommitInfo : Contains the path and other optional modifiers for the
	// future upload commit. Equivalent to the parameters provided to `upload`.
	CommitInfo *CommitInfo `json:"commit_info"`
	// Duration : How long before this link expires, in seconds.  Attempting to
	// start an upload with this link longer than this period  of time after
	// link creation will result in an error.
	Duration float64 `json:"duration"`
}

// NewGetTemporaryUploadLinkArg returns a new GetTemporaryUploadLinkArg instance
func NewGetTemporaryUploadLinkArg(CommitInfo *CommitInfo) *GetTemporaryUploadLinkArg {
	s := new(GetTemporaryUploadLinkArg)
	s.CommitInfo = CommitInfo
	s.Duration = 14400.0
	return s
}

// GetTemporaryUploadLinkResult : has no documentation (yet)
type GetTemporaryUploadLinkResult struct {
	// Link : The temporary link which can be used to stream a file to a Dropbox
	// location.
	Link string `json:"link"`
}

// NewGetTemporaryUploadLinkResult returns a new GetTemporaryUploadLinkResult instance
func NewGetTemporaryUploadLinkResult(Link string) *GetTemporaryUploadLinkResult {
	s := new(GetTemporaryUploadLinkResult)
	s.Link = Link
	return s
}

// GetThumbnailBatchArg : Arguments for `getThumbnailBatch`.
type GetThumbnailBatchArg struct {
	// Entries : List of files to get thumbnails.
	Entries []*ThumbnailArg `json:"entries"`
}

// NewGetThumbnailBatchArg returns a new GetThumbnailBatchArg instance
func NewGetThumbnailBatchArg(Entries []*ThumbnailArg) *GetThumbnailBatchArg {
	s := new(GetThumbnailBatchArg)
	s.Entries = Entries
	return s
}

// GetThumbnailBatchError : has no documentation (yet)
type GetThumbnailBatchError struct {
	dropbox.Tagged
}

// Valid tag values for GetThumbnailBatchError
const (
	GetThumbnailBatchErrorTooManyFiles = "too_many_files"
	GetThumbnailBatchErrorOther        = "other"
)

// GetThumbnailBatchResult : has no documentation (yet)
type GetThumbnailBatchResult struct {
	// Entries : List of files and their thumbnails.
	Entries []*GetThumbnailBatchResultEntry `json:"entries"`
}

// NewGetThumbnailBatchResult returns a new GetThumbnailBatchResult instance
func NewGetThumbnailBatchResult(Entries []*GetThumbnailBatchResultEntry) *GetThumbnailBatchResult {
	s := new(GetThumbnailBatchResult)
	s.Entries = Entries
	return s
}

// GetThumbnailBatchResultData : has no documentation (yet)
type GetThumbnailBatchResultData struct {
	// Metadata : has no documentation (yet)
	Metadata *FileMetadata `json:"metadata"`
	// Thumbnail : A string containing the base64-encoded thumbnail data for
	// this file.
	Thumbnail string `json:"thumbnail"`
}

// NewGetThumbnailBatchResultData returns a new GetThumbnailBatchResultData instance
func NewGetThumbnailBatchResultData(Metadata *FileMetadata, Thumbnail string) *GetThumbnailBatchResultData {
	s := new(GetThumbnailBatchResultData)
	s.Metadata = Metadata
	s.Thumbnail = Thumbnail
	return s
}

// GetThumbnailBatchResultEntry : has no documentation (yet)
type GetThumbnailBatchResultEntry struct {
	dropbox.Tagged
	// Success : has no documentation (yet)
	Success *GetThumbnailBatchResultData `json:"success,omitempty"`
	// Failure : The result for this file if it was an error.
	Failure *ThumbnailError `json:"failure,omitempty"`
}

// Valid tag values for GetThumbnailBatchResultEntry
const (
	GetThumbnailBatchResultEntrySuccess = "success"
	GetThumbnailBatchResultEntryFailure = "failure"
	GetThumbnailBatchResultEntryOther   = "other"
)

// UnmarshalJSON deserializes into a GetThumbnailBatchResultEntry instance
func (u *GetThumbnailBatchResultEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failure : The result for this file if it was an error.
		Failure *ThumbnailError `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		if err = json.Unmarshal(body, &u.Success); err != nil {
			return err
		}

	case "failure":
		u.Failure = w.Failure

	}
	return nil
}

// GpsCoordinates : GPS coordinates for a photo or video.
type GpsCoordinates struct {
	// Latitude : Latitude of the GPS coordinates.
	Latitude float64 `json:"latitude"`
	// Longitude : Longitude of the GPS coordinates.
	Longitude float64 `json:"longitude"`
}

// NewGpsCoordinates returns a new GpsCoordinates instance
func NewGpsCoordinates(Latitude float64, Longitude float64) *GpsCoordinates {
	s := new(GpsCoordinates)
	s.Latitude = Latitude
	s.Longitude = Longitude
	return s
}

// HighlightSpan : has no documentation (yet)
type HighlightSpan struct {
	// HighlightStr : String to be determined whether it should be highlighted
	// or not.
	HighlightStr string `json:"highlight_str"`
	// IsHighlighted : The string should be highlighted or not.
	IsHighlighted bool `json:"is_highlighted"`
}

// NewHighlightSpan returns a new HighlightSpan instance
func NewHighlightSpan(HighlightStr string, IsHighlighted bool) *HighlightSpan {
	s := new(HighlightSpan)
	s.HighlightStr = HighlightStr
	s.IsHighlighted = IsHighlighted
	return s
}

// ImportFormat : The import format of the incoming Paper doc content.
type ImportFormat struct {
	dropbox.Tagged
}

// Valid tag values for ImportFormat
const (
	ImportFormatHtml      = "html"
	ImportFormatMarkdown  = "markdown"
	ImportFormatPlainText = "plain_text"
	ImportFormatOther     = "other"
)

// ListFolderArg : has no documentation (yet)
type ListFolderArg struct {
	// Path : A unique identifier for the file.
	Path string `json:"path"`
	// Recursive : If true, the list folder operation will be applied
	// recursively to all subfolders and the response will contain contents of
	// all subfolders.
	Recursive bool `json:"recursive"`
	// IncludeMediaInfo : If true, `FileMetadata.media_info` is set for photo
	// and video. This parameter will no longer have an effect starting December
	// 2, 2019.
	IncludeMediaInfo bool `json:"include_media_info"`
	// IncludeDeleted : If true, the results will include entries for files and
	// folders that used to exist but were deleted.
	IncludeDeleted bool `json:"include_deleted"`
	// IncludeHasExplicitSharedMembers : If true, the results will include a
	// flag for each file indicating whether or not  that file has any explicit
	// members.
	IncludeHasExplicitSharedMembers bool `json:"include_has_explicit_shared_members"`
	// IncludeMountedFolders : If true, the results will include entries under
	// mounted folders which includes app folder, shared folder and team folder.
	IncludeMountedFolders bool `json:"include_mounted_folders"`
	// Limit : The maximum number of results to return per request. Note: This
	// is an approximate number and there can be slightly more entries returned
	// in some cases.
	Limit uint32 `json:"limit,omitempty"`
	// SharedLink : A shared link to list the contents of. If the link is
	// password-protected, the password must be provided. If this field is
	// present, `ListFolderArg.path` will be relative to root of the shared
	// link. Only non-recursive mode is supported for shared link.
	SharedLink *SharedLink `json:"shared_link,omitempty"`
	// IncludePropertyGroups : If set to a valid list of template IDs,
	// `FileMetadata.property_groups` is set if there exists property data
	// associated with the file and each of the listed templates.
	IncludePropertyGroups *file_properties.TemplateFilterBase `json:"include_property_groups,omitempty"`
	// IncludeNonDownloadableFiles : If true, include files that are not
	// downloadable, i.e. Google Docs.
	IncludeNonDownloadableFiles bool `json:"include_non_downloadable_files"`
}

// NewListFolderArg returns a new ListFolderArg instance
func NewListFolderArg(Path string) *ListFolderArg {
	s := new(ListFolderArg)
	s.Path = Path
	s.Recursive = false
	s.IncludeMediaInfo = false
	s.IncludeDeleted = false
	s.IncludeHasExplicitSharedMembers = false
	s.IncludeMountedFolders = true
	s.IncludeNonDownloadableFiles = true
	return s
}

// ListFolderContinueArg : has no documentation (yet)
type ListFolderContinueArg struct {
	// Cursor : The cursor returned by your last call to `listFolder` or
	// `listFolderContinue`.
	Cursor string `json:"cursor"`
}

// NewListFolderContinueArg returns a new ListFolderContinueArg instance
func NewListFolderContinueArg(Cursor string) *ListFolderContinueArg {
	s := new(ListFolderContinueArg)
	s.Cursor = Cursor
	return s
}

// ListFolderContinueError : has no documentation (yet)
type ListFolderContinueError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for ListFolderContinueError
const (
	ListFolderContinueErrorPath  = "path"
	ListFolderContinueErrorReset = "reset"
	ListFolderContinueErrorOther = "other"
)

// UnmarshalJSON deserializes into a ListFolderContinueError instance
func (u *ListFolderContinueError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// ListFolderError : has no documentation (yet)
type ListFolderError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
	// TemplateError : has no documentation (yet)
	TemplateError *file_properties.TemplateError `json:"template_error,omitempty"`
}

// Valid tag values for ListFolderError
const (
	ListFolderErrorPath          = "path"
	ListFolderErrorTemplateError = "template_error"
	ListFolderErrorOther         = "other"
)

// UnmarshalJSON deserializes into a ListFolderError instance
func (u *ListFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
		// TemplateError : has no documentation (yet)
		TemplateError *file_properties.TemplateError `json:"template_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	case "template_error":
		u.TemplateError = w.TemplateError

	}
	return nil
}

// ListFolderGetLatestCursorResult : has no documentation (yet)
type ListFolderGetLatestCursorResult struct {
	// Cursor : Pass the cursor into `listFolderContinue` to see what's changed
	// in the folder since your previous query.
	Cursor string `json:"cursor"`
}

// NewListFolderGetLatestCursorResult returns a new ListFolderGetLatestCursorResult instance
func NewListFolderGetLatestCursorResult(Cursor string) *ListFolderGetLatestCursorResult {
	s := new(ListFolderGetLatestCursorResult)
	s.Cursor = Cursor
	return s
}

// ListFolderLongpollArg : has no documentation (yet)
type ListFolderLongpollArg struct {
	// Cursor : A cursor as returned by `listFolder` or `listFolderContinue`.
	// Cursors retrieved by setting `ListFolderArg.include_media_info` to true
	// are not supported.
	Cursor string `json:"cursor"`
	// Timeout : A timeout in seconds. The request will block for at most this
	// length of time, plus up to 90 seconds of random jitter added to avoid the
	// thundering herd problem. Care should be taken when using this parameter,
	// as some network infrastructure does not support long timeouts.
	Timeout uint64 `json:"timeout"`
}

// NewListFolderLongpollArg returns a new ListFolderLongpollArg instance
func NewListFolderLongpollArg(Cursor string) *ListFolderLongpollArg {
	s := new(ListFolderLongpollArg)
	s.Cursor = Cursor
	s.Timeout = 30
	return s
}

// ListFolderLongpollError : has no documentation (yet)
type ListFolderLongpollError struct {
	dropbox.Tagged
}

// Valid tag values for ListFolderLongpollError
const (
	ListFolderLongpollErrorReset = "reset"
	ListFolderLongpollErrorOther = "other"
)

// ListFolderLongpollResult : has no documentation (yet)
type ListFolderLongpollResult struct {
	// Changes : Indicates whether new changes are available. If true, call
	// `listFolderContinue` to retrieve the changes.
	Changes bool `json:"changes"`
	// Backoff : If present, backoff for at least this many seconds before
	// calling `listFolderLongpoll` again.
	Backoff uint64 `json:"backoff,omitempty"`
}

// NewListFolderLongpollResult returns a new ListFolderLongpollResult instance
func NewListFolderLongpollResult(Changes bool) *ListFolderLongpollResult {
	s := new(ListFolderLongpollResult)
	s.Changes = Changes
	return s
}

// ListFolderResult : has no documentation (yet)
type ListFolderResult struct {
	// Entries : The files and (direct) subfolders in the folder.
	Entries []IsMetadata `json:"entries"`
	// Cursor : Pass the cursor into `listFolderContinue` to see what's changed
	// in the folder since your previous query.
	Cursor string `json:"cursor"`
	// HasMore : If true, then there are more entries available. Pass the cursor
	// to `listFolderContinue` to retrieve the rest.
	HasMore bool `json:"has_more"`
}

// NewListFolderResult returns a new ListFolderResult instance
func NewListFolderResult(Entries []IsMetadata, Cursor string, HasMore bool) *ListFolderResult {
	s := new(ListFolderResult)
	s.Entries = Entries
	s.Cursor = Cursor
	s.HasMore = HasMore
	return s
}

// UnmarshalJSON deserializes into a ListFolderResult instance
func (u *ListFolderResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Entries : The files and (direct) subfolders in the folder.
		Entries []json.RawMessage `json:"entries"`
		// Cursor : Pass the cursor into `listFolderContinue` to see what's
		// changed in the folder since your previous query.
		Cursor string `json:"cursor"`
		// HasMore : If true, then there are more entries available. Pass the
		// cursor to `listFolderContinue` to retrieve the rest.
		HasMore bool `json:"has_more"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	u.Entries = make([]IsMetadata, len(w.Entries))
	for i, e := range w.Entries {
		v, err := IsMetadataFromJSON(e)
		if err != nil {
			return err
		}
		u.Entries[i] = v
	}
	u.Cursor = w.Cursor
	u.HasMore = w.HasMore
	return nil
}

// ListRevisionsArg : has no documentation (yet)
type ListRevisionsArg struct {
	// Path : The path to the file you want to see the revisions of.
	Path string `json:"path"`
	// Mode : Determines the behavior of the API in listing the revisions for a
	// given file path or id.
	Mode *ListRevisionsMode `json:"mode"`
	// Limit : The maximum number of revision entries returned.
	Limit uint64 `json:"limit"`
}

// NewListRevisionsArg returns a new ListRevisionsArg instance
func NewListRevisionsArg(Path string) *ListRevisionsArg {
	s := new(ListRevisionsArg)
	s.Path = Path
	s.Mode = &ListRevisionsMode{Tagged: dropbox.Tagged{Tag: "path"}}
	s.Limit = 10
	return s
}

// ListRevisionsError : has no documentation (yet)
type ListRevisionsError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for ListRevisionsError
const (
	ListRevisionsErrorPath  = "path"
	ListRevisionsErrorOther = "other"
)

// UnmarshalJSON deserializes into a ListRevisionsError instance
func (u *ListRevisionsError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// ListRevisionsMode : has no documentation (yet)
type ListRevisionsMode struct {
	dropbox.Tagged
}

// Valid tag values for ListRevisionsMode
const (
	ListRevisionsModePath  = "path"
	ListRevisionsModeId    = "id"
	ListRevisionsModeOther = "other"
)

// ListRevisionsResult : has no documentation (yet)
type ListRevisionsResult struct {
	// IsDeleted : If the file identified by the latest revision in the response
	// is either deleted or moved.
	IsDeleted bool `json:"is_deleted"`
	// ServerDeleted : The time of deletion if the file was deleted.
	ServerDeleted *time.Time `json:"server_deleted,omitempty"`
	// Entries : The revisions for the file. Only revisions that are not deleted
	// will show up here.
	Entries []*FileMetadata `json:"entries"`
}

// NewListRevisionsResult returns a new ListRevisionsResult instance
func NewListRevisionsResult(IsDeleted bool, Entries []*FileMetadata) *ListRevisionsResult {
	s := new(ListRevisionsResult)
	s.IsDeleted = IsDeleted
	s.Entries = Entries
	return s
}

// LockConflictError : has no documentation (yet)
type LockConflictError struct {
	// Lock : The lock that caused the conflict.
	Lock *FileLock `json:"lock"`
}

// NewLockConflictError returns a new LockConflictError instance
func NewLockConflictError(Lock *FileLock) *LockConflictError {
	s := new(LockConflictError)
	s.Lock = Lock
	return s
}

// LockFileArg : has no documentation (yet)
type LockFileArg struct {
	// Path : Path in the user's Dropbox to a file.
	Path string `json:"path"`
}

// NewLockFileArg returns a new LockFileArg instance
func NewLockFileArg(Path string) *LockFileArg {
	s := new(LockFileArg)
	s.Path = Path
	return s
}

// LockFileBatchArg : has no documentation (yet)
type LockFileBatchArg struct {
	// Entries : List of 'entries'. Each 'entry' contains a path of the file
	// which will be locked or queried. Duplicate path arguments in the batch
	// are considered only once.
	Entries []*LockFileArg `json:"entries"`
}

// NewLockFileBatchArg returns a new LockFileBatchArg instance
func NewLockFileBatchArg(Entries []*LockFileArg) *LockFileBatchArg {
	s := new(LockFileBatchArg)
	s.Entries = Entries
	return s
}

// LockFileBatchResult : has no documentation (yet)
type LockFileBatchResult struct {
	FileOpsResult
	// Entries : Each Entry in the 'entries' will have '.tag' with the operation
	// status (e.g. success), the metadata for the file and the lock state after
	// the operation.
	Entries []*LockFileResultEntry `json:"entries"`
}

// NewLockFileBatchResult returns a new LockFileBatchResult instance
func NewLockFileBatchResult(Entries []*LockFileResultEntry) *LockFileBatchResult {
	s := new(LockFileBatchResult)
	s.Entries = Entries
	return s
}

// LockFileError : has no documentation (yet)
type LockFileError struct {
	dropbox.Tagged
	// PathLookup : Could not find the specified resource.
	PathLookup *LookupError `json:"path_lookup,omitempty"`
	// LockConflict : The user action conflicts with an existing lock on the
	// file.
	LockConflict *LockConflictError `json:"lock_conflict,omitempty"`
}

// Valid tag values for LockFileError
const (
	LockFileErrorPathLookup             = "path_lookup"
	LockFileErrorTooManyWriteOperations = "too_many_write_operations"
	LockFileErrorTooManyFiles           = "too_many_files"
	LockFileErrorNoWritePermission      = "no_write_permission"
	LockFileErrorCannotBeLocked         = "cannot_be_locked"
	LockFileErrorFileNotShared          = "file_not_shared"
	LockFileErrorLockConflict           = "lock_conflict"
	LockFileErrorInternalError          = "internal_error"
	LockFileErrorOther                  = "other"
)

// UnmarshalJSON deserializes into a LockFileError instance
func (u *LockFileError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PathLookup : Could not find the specified resource.
		PathLookup *LookupError `json:"path_lookup,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path_lookup":
		u.PathLookup = w.PathLookup

	case "lock_conflict":
		if err = json.Unmarshal(body, &u.LockConflict); err != nil {
			return err
		}

	}
	return nil
}

// LockFileResult : has no documentation (yet)
type LockFileResult struct {
	// Metadata : Metadata of the file.
	Metadata IsMetadata `json:"metadata"`
	// Lock : The file lock state after the operation.
	Lock *FileLock `json:"lock"`
}

// NewLockFileResult returns a new LockFileResult instance
func NewLockFileResult(Metadata IsMetadata, Lock *FileLock) *LockFileResult {
	s := new(LockFileResult)
	s.Metadata = Metadata
	s.Lock = Lock
	return s
}

// UnmarshalJSON deserializes into a LockFileResult instance
func (u *LockFileResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : Metadata of the file.
		Metadata json.RawMessage `json:"metadata"`
		// Lock : The file lock state after the operation.
		Lock *FileLock `json:"lock"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	u.Lock = w.Lock
	return nil
}

// LockFileResultEntry : has no documentation (yet)
type LockFileResultEntry struct {
	dropbox.Tagged
	// Success : has no documentation (yet)
	Success *LockFileResult `json:"success,omitempty"`
	// Failure : has no documentation (yet)
	Failure *LockFileError `json:"failure,omitempty"`
}

// Valid tag values for LockFileResultEntry
const (
	LockFileResultEntrySuccess = "success"
	LockFileResultEntryFailure = "failure"
)

// UnmarshalJSON deserializes into a LockFileResultEntry instance
func (u *LockFileResultEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failure : has no documentation (yet)
		Failure *LockFileError `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		if err = json.Unmarshal(body, &u.Success); err != nil {
			return err
		}

	case "failure":
		u.Failure = w.Failure

	}
	return nil
}

// LookupError : has no documentation (yet)
type LookupError struct {
	dropbox.Tagged
	// MalformedPath : The given path does not satisfy the required path format.
	// Please refer to the `Path formats documentation`
	// <https://www.dropbox.com/developers/documentation/http/documentation#path-formats>
	// for more information.
	MalformedPath string `json:"malformed_path,omitempty"`
}

// Valid tag values for LookupError
const (
	LookupErrorMalformedPath          = "malformed_path"
	LookupErrorNotFound               = "not_found"
	LookupErrorNotFile                = "not_file"
	LookupErrorNotFolder              = "not_folder"
	LookupErrorRestrictedContent      = "restricted_content"
	LookupErrorUnsupportedContentType = "unsupported_content_type"
	LookupErrorLocked                 = "locked"
	LookupErrorOther                  = "other"
)

// UnmarshalJSON deserializes into a LookupError instance
func (u *LookupError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MalformedPath : The given path does not satisfy the required path
		// format. Please refer to the `Path formats documentation`
		// <https://www.dropbox.com/developers/documentation/http/documentation#path-formats>
		// for more information.
		MalformedPath string `json:"malformed_path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "malformed_path":
		u.MalformedPath = w.MalformedPath

	}
	return nil
}

// MediaInfo : has no documentation (yet)
type MediaInfo struct {
	dropbox.Tagged
	// Metadata : The metadata for the photo/video.
	Metadata IsMediaMetadata `json:"metadata,omitempty"`
}

// Valid tag values for MediaInfo
const (
	MediaInfoPending  = "pending"
	MediaInfoMetadata = "metadata"
)

// UnmarshalJSON deserializes into a MediaInfo instance
func (u *MediaInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Metadata : The metadata for the photo/video.
		Metadata json.RawMessage `json:"metadata,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "metadata":
		if u.Metadata, err = IsMediaMetadataFromJSON(w.Metadata); err != nil {
			return err
		}

	}
	return nil
}

// MediaMetadata : Metadata for a photo or video.
type MediaMetadata struct {
	// Dimensions : Dimension of the photo/video.
	Dimensions *Dimensions `json:"dimensions,omitempty"`
	// Location : The GPS coordinate of the photo/video.
	Location *GpsCoordinates `json:"location,omitempty"`
	// TimeTaken : The timestamp when the photo/video is taken.
	TimeTaken *time.Time `json:"time_taken,omitempty"`
}

// NewMediaMetadata returns a new MediaMetadata instance
func NewMediaMetadata() *MediaMetadata {
	s := new(MediaMetadata)
	return s
}

// IsMediaMetadata is the interface type for MediaMetadata and its subtypes
type IsMediaMetadata interface {
	IsMediaMetadata()
}

// IsMediaMetadata implements the IsMediaMetadata interface
func (u *MediaMetadata) IsMediaMetadata() {}

type mediaMetadataUnion struct {
	dropbox.Tagged
	// Photo : has no documentation (yet)
	Photo *PhotoMetadata `json:"photo,omitempty"`
	// Video : has no documentation (yet)
	Video *VideoMetadata `json:"video,omitempty"`
}

// Valid tag values for MediaMetadata
const (
	MediaMetadataPhoto = "photo"
	MediaMetadataVideo = "video"
)

// UnmarshalJSON deserializes into a mediaMetadataUnion instance
func (u *mediaMetadataUnion) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "photo":
		if err = json.Unmarshal(body, &u.Photo); err != nil {
			return err
		}

	case "video":
		if err = json.Unmarshal(body, &u.Video); err != nil {
			return err
		}

	}
	return nil
}

// IsMediaMetadataFromJSON converts JSON to a concrete IsMediaMetadata instance
func IsMediaMetadataFromJSON(data []byte) (IsMediaMetadata, error) {
	var t mediaMetadataUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "photo":
		return t.Photo, nil

	case "video":
		return t.Video, nil

	}
	return nil, nil
}

// MetadataV2 : Metadata for a file, folder or other resource types.
type MetadataV2 struct {
	dropbox.Tagged
	// Metadata : has no documentation (yet)
	Metadata IsMetadata `json:"metadata,omitempty"`
}

// Valid tag values for MetadataV2
const (
	MetadataV2Metadata = "metadata"
	MetadataV2Other    = "other"
)

// UnmarshalJSON deserializes into a MetadataV2 instance
func (u *MetadataV2) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Metadata : has no documentation (yet)
		Metadata json.RawMessage `json:"metadata,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "metadata":
		if u.Metadata, err = IsMetadataFromJSON(w.Metadata); err != nil {
			return err
		}

	}
	return nil
}

// MinimalFileLinkMetadata : has no documentation (yet)
type MinimalFileLinkMetadata struct {
	// Url : URL of the shared link.
	Url string `json:"url"`
	// Id : Unique identifier for the linked file.
	Id string `json:"id,omitempty"`
	// Path : Full path in the user's Dropbox. This always starts with a slash.
	// This field will only be present only if the linked file is in the
	// authenticated user's Dropbox.
	Path string `json:"path,omitempty"`
	// Rev : A unique identifier for the current revision of a file. This field
	// is the same rev as elsewhere in the API and can be used to detect changes
	// and avoid conflicts.
	Rev string `json:"rev"`
}

// NewMinimalFileLinkMetadata returns a new MinimalFileLinkMetadata instance
func NewMinimalFileLinkMetadata(Url string, Rev string) *MinimalFileLinkMetadata {
	s := new(MinimalFileLinkMetadata)
	s.Url = Url
	s.Rev = Rev
	return s
}

// RelocationBatchArgBase : has no documentation (yet)
type RelocationBatchArgBase struct {
	// Entries : List of entries to be moved or copied. Each entry is
	// `RelocationPath`.
	Entries []*RelocationPath `json:"entries"`
	// Autorename : If there's a conflict with any file, have the Dropbox server
	// try to autorename that file to avoid the conflict.
	Autorename bool `json:"autorename"`
}

// NewRelocationBatchArgBase returns a new RelocationBatchArgBase instance
func NewRelocationBatchArgBase(Entries []*RelocationPath) *RelocationBatchArgBase {
	s := new(RelocationBatchArgBase)
	s.Entries = Entries
	s.Autorename = false
	return s
}

// MoveBatchArg : has no documentation (yet)
type MoveBatchArg struct {
	RelocationBatchArgBase
	// AllowOwnershipTransfer : Allow moves by owner even if it would result in
	// an ownership transfer for the content being moved. This does not apply to
	// copies.
	AllowOwnershipTransfer bool `json:"allow_ownership_transfer"`
}

// NewMoveBatchArg returns a new MoveBatchArg instance
func NewMoveBatchArg(Entries []*RelocationPath) *MoveBatchArg {
	s := new(MoveBatchArg)
	s.Entries = Entries
	s.Autorename = false
	s.AllowOwnershipTransfer = false
	return s
}

// MoveIntoFamilyError : has no documentation (yet)
type MoveIntoFamilyError struct {
	dropbox.Tagged
}

// Valid tag values for MoveIntoFamilyError
const (
	MoveIntoFamilyErrorIsSharedFolder = "is_shared_folder"
	MoveIntoFamilyErrorOther          = "other"
)

// MoveIntoVaultError : has no documentation (yet)
type MoveIntoVaultError struct {
	dropbox.Tagged
}

// Valid tag values for MoveIntoVaultError
const (
	MoveIntoVaultErrorIsSharedFolder = "is_shared_folder"
	MoveIntoVaultErrorOther          = "other"
)

// PaperContentError : has no documentation (yet)
type PaperContentError struct {
	dropbox.Tagged
}

// Valid tag values for PaperContentError
const (
	PaperContentErrorInsufficientPermissions = "insufficient_permissions"
	PaperContentErrorContentMalformed        = "content_malformed"
	PaperContentErrorDocLengthExceeded       = "doc_length_exceeded"
	PaperContentErrorImageSizeExceeded       = "image_size_exceeded"
	PaperContentErrorOther                   = "other"
)

// PaperCreateArg : has no documentation (yet)
type PaperCreateArg struct {
	// Path : The fully qualified path to the location in the user's Dropbox
	// where the Paper Doc should be created. This should include the document's
	// title and end with .paper.
	Path string `json:"path"`
	// ImportFormat : The format of the provided data.
	ImportFormat *ImportFormat `json:"import_format"`
}

// NewPaperCreateArg returns a new PaperCreateArg instance
func NewPaperCreateArg(Path string, ImportFormat *ImportFormat) *PaperCreateArg {
	s := new(PaperCreateArg)
	s.Path = Path
	s.ImportFormat = ImportFormat
	return s
}

// PaperCreateError : has no documentation (yet)
type PaperCreateError struct {
	dropbox.Tagged
}

// Valid tag values for PaperCreateError
const (
	PaperCreateErrorInsufficientPermissions = "insufficient_permissions"
	PaperCreateErrorContentMalformed        = "content_malformed"
	PaperCreateErrorDocLengthExceeded       = "doc_length_exceeded"
	PaperCreateErrorImageSizeExceeded       = "image_size_exceeded"
	PaperCreateErrorOther                   = "other"
	PaperCreateErrorInvalidPath             = "invalid_path"
	PaperCreateErrorEmailUnverified         = "email_unverified"
	PaperCreateErrorInvalidFileExtension    = "invalid_file_extension"
	PaperCreateErrorPaperDisabled           = "paper_disabled"
)

// PaperCreateResult : has no documentation (yet)
type PaperCreateResult struct {
	// Url : URL to open the Paper Doc.
	Url string `json:"url"`
	// ResultPath : The fully qualified path the Paper Doc was actually created
	// at.
	ResultPath string `json:"result_path"`
	// FileId : The id to use in Dropbox APIs when referencing the Paper Doc.
	FileId string `json:"file_id"`
	// PaperRevision : The current doc revision.
	PaperRevision int64 `json:"paper_revision"`
}

// NewPaperCreateResult returns a new PaperCreateResult instance
func NewPaperCreateResult(Url string, ResultPath string, FileId string, PaperRevision int64) *PaperCreateResult {
	s := new(PaperCreateResult)
	s.Url = Url
	s.ResultPath = ResultPath
	s.FileId = FileId
	s.PaperRevision = PaperRevision
	return s
}

// PaperDocUpdatePolicy : has no documentation (yet)
type PaperDocUpdatePolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperDocUpdatePolicy
const (
	PaperDocUpdatePolicyUpdate    = "update"
	PaperDocUpdatePolicyOverwrite = "overwrite"
	PaperDocUpdatePolicyPrepend   = "prepend"
	PaperDocUpdatePolicyAppend    = "append"
	PaperDocUpdatePolicyOther     = "other"
)

// PaperUpdateArg : has no documentation (yet)
type PaperUpdateArg struct {
	// Path : Path in the user's Dropbox to update. The path must correspond to
	// a Paper doc or an error will be returned.
	Path string `json:"path"`
	// ImportFormat : The format of the provided data.
	ImportFormat *ImportFormat `json:"import_format"`
	// DocUpdatePolicy : How the provided content should be applied to the doc.
	DocUpdatePolicy *PaperDocUpdatePolicy `json:"doc_update_policy"`
	// PaperRevision : The latest doc revision. Required when doc_update_policy
	// is update. This value must match the current revision of the doc or error
	// revision_mismatch will be returned.
	PaperRevision int64 `json:"paper_revision,omitempty"`
}

// NewPaperUpdateArg returns a new PaperUpdateArg instance
func NewPaperUpdateArg(Path string, ImportFormat *ImportFormat, DocUpdatePolicy *PaperDocUpdatePolicy) *PaperUpdateArg {
	s := new(PaperUpdateArg)
	s.Path = Path
	s.ImportFormat = ImportFormat
	s.DocUpdatePolicy = DocUpdatePolicy
	return s
}

// PaperUpdateError : has no documentation (yet)
type PaperUpdateError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for PaperUpdateError
const (
	PaperUpdateErrorInsufficientPermissions = "insufficient_permissions"
	PaperUpdateErrorContentMalformed        = "content_malformed"
	PaperUpdateErrorDocLengthExceeded       = "doc_length_exceeded"
	PaperUpdateErrorImageSizeExceeded       = "image_size_exceeded"
	PaperUpdateErrorOther                   = "other"
	PaperUpdateErrorPath                    = "path"
	PaperUpdateErrorRevisionMismatch        = "revision_mismatch"
	PaperUpdateErrorDocArchived             = "doc_archived"
	PaperUpdateErrorDocDeleted              = "doc_deleted"
)

// UnmarshalJSON deserializes into a PaperUpdateError instance
func (u *PaperUpdateError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// PaperUpdateResult : has no documentation (yet)
type PaperUpdateResult struct {
	// PaperRevision : The current doc revision.
	PaperRevision int64 `json:"paper_revision"`
}

// NewPaperUpdateResult returns a new PaperUpdateResult instance
func NewPaperUpdateResult(PaperRevision int64) *PaperUpdateResult {
	s := new(PaperUpdateResult)
	s.PaperRevision = PaperRevision
	return s
}

// PathOrLink : has no documentation (yet)
type PathOrLink struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path string `json:"path,omitempty"`
	// Link : has no documentation (yet)
	Link *SharedLinkFileInfo `json:"link,omitempty"`
}

// Valid tag values for PathOrLink
const (
	PathOrLinkPath  = "path"
	PathOrLinkLink  = "link"
	PathOrLinkOther = "other"
)

// UnmarshalJSON deserializes into a PathOrLink instance
func (u *PathOrLink) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path string `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	case "link":
		if err = json.Unmarshal(body, &u.Link); err != nil {
			return err
		}

	}
	return nil
}

// PathToTags : has no documentation (yet)
type PathToTags struct {
	// Path : Path of the item.
	Path string `json:"path"`
	// Tags : Tags assigned to this item.
	Tags []*Tag `json:"tags"`
}

// NewPathToTags returns a new PathToTags instance
func NewPathToTags(Path string, Tags []*Tag) *PathToTags {
	s := new(PathToTags)
	s.Path = Path
	s.Tags = Tags
	return s
}

// PhotoMetadata : Metadata for a photo.
type PhotoMetadata struct {
	MediaMetadata
}

// NewPhotoMetadata returns a new PhotoMetadata instance
func NewPhotoMetadata() *PhotoMetadata {
	s := new(PhotoMetadata)
	return s
}

// PreviewArg : has no documentation (yet)
type PreviewArg struct {
	// Path : The path of the file to preview.
	Path string `json:"path"`
	// Rev : Please specify revision in `path` instead.
	Rev string `json:"rev,omitempty"`
}

// NewPreviewArg returns a new PreviewArg instance
func NewPreviewArg(Path string) *PreviewArg {
	s := new(PreviewArg)
	s.Path = Path
	return s
}

// PreviewError : has no documentation (yet)
type PreviewError struct {
	dropbox.Tagged
	// Path : An error occurs when downloading metadata for the file.
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for PreviewError
const (
	PreviewErrorPath                 = "path"
	PreviewErrorInProgress           = "in_progress"
	PreviewErrorUnsupportedExtension = "unsupported_extension"
	PreviewErrorUnsupportedContent   = "unsupported_content"
)

// UnmarshalJSON deserializes into a PreviewError instance
func (u *PreviewError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : An error occurs when downloading metadata for the file.
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// PreviewResult : has no documentation (yet)
type PreviewResult struct {
	// FileMetadata : Metadata corresponding to the file received as an
	// argument. Will be populated if the endpoint is called with a path
	// (ReadPath).
	FileMetadata *FileMetadata `json:"file_metadata,omitempty"`
	// LinkMetadata : Minimal metadata corresponding to the file received as an
	// argument. Will be populated if the endpoint is called using a shared link
	// (SharedLinkFileInfo).
	LinkMetadata *MinimalFileLinkMetadata `json:"link_metadata,omitempty"`
}

// NewPreviewResult returns a new PreviewResult instance
func NewPreviewResult() *PreviewResult {
	s := new(PreviewResult)
	return s
}

// RelocationPath : has no documentation (yet)
type RelocationPath struct {
	// FromPath : Path in the user's Dropbox to be copied or moved.
	FromPath string `json:"from_path"`
	// ToPath : Path in the user's Dropbox that is the destination.
	ToPath string `json:"to_path"`
}

// NewRelocationPath returns a new RelocationPath instance
func NewRelocationPath(FromPath string, ToPath string) *RelocationPath {
	s := new(RelocationPath)
	s.FromPath = FromPath
	s.ToPath = ToPath
	return s
}

// RelocationArg : has no documentation (yet)
type RelocationArg struct {
	RelocationPath
	// AllowSharedFolder : This flag has no effect.
	AllowSharedFolder bool `json:"allow_shared_folder"`
	// Autorename : If there's a conflict, have the Dropbox server try to
	// autorename the file to avoid the conflict.
	Autorename bool `json:"autorename"`
	// AllowOwnershipTransfer : Allow moves by owner even if it would result in
	// an ownership transfer for the content being moved. This does not apply to
	// copies.
	AllowOwnershipTransfer bool `json:"allow_ownership_transfer"`
}

// NewRelocationArg returns a new RelocationArg instance
func NewRelocationArg(FromPath string, ToPath string) *RelocationArg {
	s := new(RelocationArg)
	s.FromPath = FromPath
	s.ToPath = ToPath
	s.AllowSharedFolder = false
	s.Autorename = false
	s.AllowOwnershipTransfer = false
	return s
}

// RelocationBatchArg : has no documentation (yet)
type RelocationBatchArg struct {
	RelocationBatchArgBase
	// AllowSharedFolder : This flag has no effect.
	AllowSharedFolder bool `json:"allow_shared_folder"`
	// AllowOwnershipTransfer : Allow moves by owner even if it would result in
	// an ownership transfer for the content being moved. This does not apply to
	// copies.
	AllowOwnershipTransfer bool `json:"allow_ownership_transfer"`
}

// NewRelocationBatchArg returns a new RelocationBatchArg instance
func NewRelocationBatchArg(Entries []*RelocationPath) *RelocationBatchArg {
	s := new(RelocationBatchArg)
	s.Entries = Entries
	s.Autorename = false
	s.AllowSharedFolder = false
	s.AllowOwnershipTransfer = false
	return s
}

// RelocationError : has no documentation (yet)
type RelocationError struct {
	dropbox.Tagged
	// FromLookup : has no documentation (yet)
	FromLookup *LookupError `json:"from_lookup,omitempty"`
	// FromWrite : has no documentation (yet)
	FromWrite *WriteError `json:"from_write,omitempty"`
	// To : has no documentation (yet)
	To *WriteError `json:"to,omitempty"`
	// CantMoveIntoVault : Some content cannot be moved into Vault under certain
	// circumstances, see detailed error.
	CantMoveIntoVault *MoveIntoVaultError `json:"cant_move_into_vault,omitempty"`
	// CantMoveIntoFamily : Some content cannot be moved into the Family Room
	// folder under certain circumstances, see detailed error.
	CantMoveIntoFamily *MoveIntoFamilyError `json:"cant_move_into_family,omitempty"`
}

// Valid tag values for RelocationError
const (
	RelocationErrorFromLookup               = "from_lookup"
	RelocationErrorFromWrite                = "from_write"
	RelocationErrorTo                       = "to"
	RelocationErrorCantCopySharedFolder     = "cant_copy_shared_folder"
	RelocationErrorCantNestSharedFolder     = "cant_nest_shared_folder"
	RelocationErrorCantMoveFolderIntoItself = "cant_move_folder_into_itself"
	RelocationErrorTooManyFiles             = "too_many_files"
	RelocationErrorDuplicatedOrNestedPaths  = "duplicated_or_nested_paths"
	RelocationErrorCantTransferOwnership    = "cant_transfer_ownership"
	RelocationErrorInsufficientQuota        = "insufficient_quota"
	RelocationErrorInternalError            = "internal_error"
	RelocationErrorCantMoveSharedFolder     = "cant_move_shared_folder"
	RelocationErrorCantMoveIntoVault        = "cant_move_into_vault"
	RelocationErrorCantMoveIntoFamily       = "cant_move_into_family"
	RelocationErrorOther                    = "other"
)

// UnmarshalJSON deserializes into a RelocationError instance
func (u *RelocationError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// FromLookup : has no documentation (yet)
		FromLookup *LookupError `json:"from_lookup,omitempty"`
		// FromWrite : has no documentation (yet)
		FromWrite *WriteError `json:"from_write,omitempty"`
		// To : has no documentation (yet)
		To *WriteError `json:"to,omitempty"`
		// CantMoveIntoVault : Some content cannot be moved into Vault under
		// certain circumstances, see detailed error.
		CantMoveIntoVault *MoveIntoVaultError `json:"cant_move_into_vault,omitempty"`
		// CantMoveIntoFamily : Some content cannot be moved into the Family
		// Room folder under certain circumstances, see detailed error.
		CantMoveIntoFamily *MoveIntoFamilyError `json:"cant_move_into_family,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "from_lookup":
		u.FromLookup = w.FromLookup

	case "from_write":
		u.FromWrite = w.FromWrite

	case "to":
		u.To = w.To

	case "cant_move_into_vault":
		u.CantMoveIntoVault = w.CantMoveIntoVault

	case "cant_move_into_family":
		u.CantMoveIntoFamily = w.CantMoveIntoFamily

	}
	return nil
}

// RelocationBatchError : has no documentation (yet)
type RelocationBatchError struct {
	dropbox.Tagged
	// FromLookup : has no documentation (yet)
	FromLookup *LookupError `json:"from_lookup,omitempty"`
	// FromWrite : has no documentation (yet)
	FromWrite *WriteError `json:"from_write,omitempty"`
	// To : has no documentation (yet)
	To *WriteError `json:"to,omitempty"`
	// CantMoveIntoVault : Some content cannot be moved into Vault under certain
	// circumstances, see detailed error.
	CantMoveIntoVault *MoveIntoVaultError `json:"cant_move_into_vault,omitempty"`
	// CantMoveIntoFamily : Some content cannot be moved into the Family Room
	// folder under certain circumstances, see detailed error.
	CantMoveIntoFamily *MoveIntoFamilyError `json:"cant_move_into_family,omitempty"`
}

// Valid tag values for RelocationBatchError
const (
	RelocationBatchErrorFromLookup               = "from_lookup"
	RelocationBatchErrorFromWrite                = "from_write"
	RelocationBatchErrorTo                       = "to"
	RelocationBatchErrorCantCopySharedFolder     = "cant_copy_shared_folder"
	RelocationBatchErrorCantNestSharedFolder     = "cant_nest_shared_folder"
	RelocationBatchErrorCantMoveFolderIntoItself = "cant_move_folder_into_itself"
	RelocationBatchErrorTooManyFiles             = "too_many_files"
	RelocationBatchErrorDuplicatedOrNestedPaths  = "duplicated_or_nested_paths"
	RelocationBatchErrorCantTransferOwnership    = "cant_transfer_ownership"
	RelocationBatchErrorInsufficientQuota        = "insufficient_quota"
	RelocationBatchErrorInternalError            = "internal_error"
	RelocationBatchErrorCantMoveSharedFolder     = "cant_move_shared_folder"
	RelocationBatchErrorCantMoveIntoVault        = "cant_move_into_vault"
	RelocationBatchErrorCantMoveIntoFamily       = "cant_move_into_family"
	RelocationBatchErrorOther                    = "other"
	RelocationBatchErrorTooManyWriteOperations   = "too_many_write_operations"
)

// UnmarshalJSON deserializes into a RelocationBatchError instance
func (u *RelocationBatchError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// FromLookup : has no documentation (yet)
		FromLookup *LookupError `json:"from_lookup,omitempty"`
		// FromWrite : has no documentation (yet)
		FromWrite *WriteError `json:"from_write,omitempty"`
		// To : has no documentation (yet)
		To *WriteError `json:"to,omitempty"`
		// CantMoveIntoVault : Some content cannot be moved into Vault under
		// certain circumstances, see detailed error.
		CantMoveIntoVault *MoveIntoVaultError `json:"cant_move_into_vault,omitempty"`
		// CantMoveIntoFamily : Some content cannot be moved into the Family
		// Room folder under certain circumstances, see detailed error.
		CantMoveIntoFamily *MoveIntoFamilyError `json:"cant_move_into_family,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "from_lookup":
		u.FromLookup = w.FromLookup

	case "from_write":
		u.FromWrite = w.FromWrite

	case "to":
		u.To = w.To

	case "cant_move_into_vault":
		u.CantMoveIntoVault = w.CantMoveIntoVault

	case "cant_move_into_family":
		u.CantMoveIntoFamily = w.CantMoveIntoFamily

	}
	return nil
}

// RelocationBatchErrorEntry : has no documentation (yet)
type RelocationBatchErrorEntry struct {
	dropbox.Tagged
	// RelocationError : User errors that retry won't help.
	RelocationError *RelocationError `json:"relocation_error,omitempty"`
}

// Valid tag values for RelocationBatchErrorEntry
const (
	RelocationBatchErrorEntryRelocationError        = "relocation_error"
	RelocationBatchErrorEntryInternalError          = "internal_error"
	RelocationBatchErrorEntryTooManyWriteOperations = "too_many_write_operations"
	RelocationBatchErrorEntryOther                  = "other"
)

// UnmarshalJSON deserializes into a RelocationBatchErrorEntry instance
func (u *RelocationBatchErrorEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// RelocationError : User errors that retry won't help.
		RelocationError *RelocationError `json:"relocation_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "relocation_error":
		u.RelocationError = w.RelocationError

	}
	return nil
}

// RelocationBatchJobStatus : has no documentation (yet)
type RelocationBatchJobStatus struct {
	dropbox.Tagged
	// Complete : The copy or move batch job has finished.
	Complete *RelocationBatchResult `json:"complete,omitempty"`
	// Failed : The copy or move batch job has failed with exception.
	Failed *RelocationBatchError `json:"failed,omitempty"`
}

// Valid tag values for RelocationBatchJobStatus
const (
	RelocationBatchJobStatusInProgress = "in_progress"
	RelocationBatchJobStatusComplete   = "complete"
	RelocationBatchJobStatusFailed     = "failed"
)

// UnmarshalJSON deserializes into a RelocationBatchJobStatus instance
func (u *RelocationBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : The copy or move batch job has failed with exception.
		Failed *RelocationBatchError `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	case "failed":
		u.Failed = w.Failed

	}
	return nil
}

// RelocationBatchLaunch : Result returned by `copyBatch` or `moveBatch` that
// may either launch an asynchronous job or complete synchronously.
type RelocationBatchLaunch struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : has no documentation (yet)
	Complete *RelocationBatchResult `json:"complete,omitempty"`
}

// Valid tag values for RelocationBatchLaunch
const (
	RelocationBatchLaunchAsyncJobId = "async_job_id"
	RelocationBatchLaunchComplete   = "complete"
	RelocationBatchLaunchOther      = "other"
)

// UnmarshalJSON deserializes into a RelocationBatchLaunch instance
func (u *RelocationBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AsyncJobId : This response indicates that the processing is
		// asynchronous. The string is an id that can be used to obtain the
		// status of the asynchronous job.
		AsyncJobId string `json:"async_job_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		u.AsyncJobId = w.AsyncJobId

	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// RelocationBatchResult : has no documentation (yet)
type RelocationBatchResult struct {
	FileOpsResult
	// Entries : has no documentation (yet)
	Entries []*RelocationBatchResultData `json:"entries"`
}

// NewRelocationBatchResult returns a new RelocationBatchResult instance
func NewRelocationBatchResult(Entries []*RelocationBatchResultData) *RelocationBatchResult {
	s := new(RelocationBatchResult)
	s.Entries = Entries
	return s
}

// RelocationBatchResultData : has no documentation (yet)
type RelocationBatchResultData struct {
	// Metadata : Metadata of the relocated object.
	Metadata IsMetadata `json:"metadata"`
}

// NewRelocationBatchResultData returns a new RelocationBatchResultData instance
func NewRelocationBatchResultData(Metadata IsMetadata) *RelocationBatchResultData {
	s := new(RelocationBatchResultData)
	s.Metadata = Metadata
	return s
}

// UnmarshalJSON deserializes into a RelocationBatchResultData instance
func (u *RelocationBatchResultData) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : Metadata of the relocated object.
		Metadata json.RawMessage `json:"metadata"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	return nil
}

// RelocationBatchResultEntry : has no documentation (yet)
type RelocationBatchResultEntry struct {
	dropbox.Tagged
	// Success : has no documentation (yet)
	Success IsMetadata `json:"success,omitempty"`
	// Failure : has no documentation (yet)
	Failure *RelocationBatchErrorEntry `json:"failure,omitempty"`
}

// Valid tag values for RelocationBatchResultEntry
const (
	RelocationBatchResultEntrySuccess = "success"
	RelocationBatchResultEntryFailure = "failure"
	RelocationBatchResultEntryOther   = "other"
)

// UnmarshalJSON deserializes into a RelocationBatchResultEntry instance
func (u *RelocationBatchResultEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Success : has no documentation (yet)
		Success json.RawMessage `json:"success,omitempty"`
		// Failure : has no documentation (yet)
		Failure *RelocationBatchErrorEntry `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		if u.Success, err = IsMetadataFromJSON(w.Success); err != nil {
			return err
		}

	case "failure":
		u.Failure = w.Failure

	}
	return nil
}

// RelocationBatchV2JobStatus : Result returned by `copyBatchCheck` or
// `moveBatchCheck` that may either be in progress or completed with result for
// each entry.
type RelocationBatchV2JobStatus struct {
	dropbox.Tagged
	// Complete : The copy or move batch job has finished.
	Complete *RelocationBatchV2Result `json:"complete,omitempty"`
}

// Valid tag values for RelocationBatchV2JobStatus
const (
	RelocationBatchV2JobStatusInProgress = "in_progress"
	RelocationBatchV2JobStatusComplete   = "complete"
)

// UnmarshalJSON deserializes into a RelocationBatchV2JobStatus instance
func (u *RelocationBatchV2JobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// RelocationBatchV2Launch : Result returned by `copyBatch` or `moveBatch` that
// may either launch an asynchronous job or complete synchronously.
type RelocationBatchV2Launch struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : has no documentation (yet)
	Complete *RelocationBatchV2Result `json:"complete,omitempty"`
}

// Valid tag values for RelocationBatchV2Launch
const (
	RelocationBatchV2LaunchAsyncJobId = "async_job_id"
	RelocationBatchV2LaunchComplete   = "complete"
)

// UnmarshalJSON deserializes into a RelocationBatchV2Launch instance
func (u *RelocationBatchV2Launch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AsyncJobId : This response indicates that the processing is
		// asynchronous. The string is an id that can be used to obtain the
		// status of the asynchronous job.
		AsyncJobId string `json:"async_job_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		u.AsyncJobId = w.AsyncJobId

	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// RelocationBatchV2Result : has no documentation (yet)
type RelocationBatchV2Result struct {
	FileOpsResult
	// Entries : Each entry in CopyBatchArg.entries or `MoveBatchArg.entries`
	// will appear at the same position inside
	// `RelocationBatchV2Result.entries`.
	Entries []*RelocationBatchResultEntry `json:"entries"`
}

// NewRelocationBatchV2Result returns a new RelocationBatchV2Result instance
func NewRelocationBatchV2Result(Entries []*RelocationBatchResultEntry) *RelocationBatchV2Result {
	s := new(RelocationBatchV2Result)
	s.Entries = Entries
	return s
}

// RelocationResult : has no documentation (yet)
type RelocationResult struct {
	FileOpsResult
	// Metadata : Metadata of the relocated object.
	Metadata IsMetadata `json:"metadata"`
}

// NewRelocationResult returns a new RelocationResult instance
func NewRelocationResult(Metadata IsMetadata) *RelocationResult {
	s := new(RelocationResult)
	s.Metadata = Metadata
	return s
}

// UnmarshalJSON deserializes into a RelocationResult instance
func (u *RelocationResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : Metadata of the relocated object.
		Metadata json.RawMessage `json:"metadata"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	return nil
}

// RemoveTagArg : has no documentation (yet)
type RemoveTagArg struct {
	// Path : Path to the item to tag.
	Path string `json:"path"`
	// TagText : The tag to remove. Will be automatically converted to lowercase
	// letters.
	TagText string `json:"tag_text"`
}

// NewRemoveTagArg returns a new RemoveTagArg instance
func NewRemoveTagArg(Path string, TagText string) *RemoveTagArg {
	s := new(RemoveTagArg)
	s.Path = Path
	s.TagText = TagText
	return s
}

// RemoveTagError : has no documentation (yet)
type RemoveTagError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for RemoveTagError
const (
	RemoveTagErrorPath          = "path"
	RemoveTagErrorOther         = "other"
	RemoveTagErrorTagNotPresent = "tag_not_present"
)

// UnmarshalJSON deserializes into a RemoveTagError instance
func (u *RemoveTagError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// RestoreArg : has no documentation (yet)
type RestoreArg struct {
	// Path : The path to save the restored file.
	Path string `json:"path"`
	// Rev : The revision to restore.
	Rev string `json:"rev"`
}

// NewRestoreArg returns a new RestoreArg instance
func NewRestoreArg(Path string, Rev string) *RestoreArg {
	s := new(RestoreArg)
	s.Path = Path
	s.Rev = Rev
	return s
}

// RestoreError : has no documentation (yet)
type RestoreError struct {
	dropbox.Tagged
	// PathLookup : An error occurs when downloading metadata for the file.
	PathLookup *LookupError `json:"path_lookup,omitempty"`
	// PathWrite : An error occurs when trying to restore the file to that path.
	PathWrite *WriteError `json:"path_write,omitempty"`
}

// Valid tag values for RestoreError
const (
	RestoreErrorPathLookup      = "path_lookup"
	RestoreErrorPathWrite       = "path_write"
	RestoreErrorInvalidRevision = "invalid_revision"
	RestoreErrorInProgress      = "in_progress"
	RestoreErrorOther           = "other"
)

// UnmarshalJSON deserializes into a RestoreError instance
func (u *RestoreError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PathLookup : An error occurs when downloading metadata for the file.
		PathLookup *LookupError `json:"path_lookup,omitempty"`
		// PathWrite : An error occurs when trying to restore the file to that
		// path.
		PathWrite *WriteError `json:"path_write,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path_lookup":
		u.PathLookup = w.PathLookup

	case "path_write":
		u.PathWrite = w.PathWrite

	}
	return nil
}

// SaveCopyReferenceArg : has no documentation (yet)
type SaveCopyReferenceArg struct {
	// CopyReference : A copy reference returned by `copyReferenceGet`.
	CopyReference string `json:"copy_reference"`
	// Path : Path in the user's Dropbox that is the destination.
	Path string `json:"path"`
}

// NewSaveCopyReferenceArg returns a new SaveCopyReferenceArg instance
func NewSaveCopyReferenceArg(CopyReference string, Path string) *SaveCopyReferenceArg {
	s := new(SaveCopyReferenceArg)
	s.CopyReference = CopyReference
	s.Path = Path
	return s
}

// SaveCopyReferenceError : has no documentation (yet)
type SaveCopyReferenceError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *WriteError `json:"path,omitempty"`
}

// Valid tag values for SaveCopyReferenceError
const (
	SaveCopyReferenceErrorPath                 = "path"
	SaveCopyReferenceErrorInvalidCopyReference = "invalid_copy_reference"
	SaveCopyReferenceErrorNoPermission         = "no_permission"
	SaveCopyReferenceErrorNotFound             = "not_found"
	SaveCopyReferenceErrorTooManyFiles         = "too_many_files"
	SaveCopyReferenceErrorOther                = "other"
)

// UnmarshalJSON deserializes into a SaveCopyReferenceError instance
func (u *SaveCopyReferenceError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *WriteError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// SaveCopyReferenceResult : has no documentation (yet)
type SaveCopyReferenceResult struct {
	// Metadata : The metadata of the saved file or folder in the user's
	// Dropbox.
	Metadata IsMetadata `json:"metadata"`
}

// NewSaveCopyReferenceResult returns a new SaveCopyReferenceResult instance
func NewSaveCopyReferenceResult(Metadata IsMetadata) *SaveCopyReferenceResult {
	s := new(SaveCopyReferenceResult)
	s.Metadata = Metadata
	return s
}

// UnmarshalJSON deserializes into a SaveCopyReferenceResult instance
func (u *SaveCopyReferenceResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Metadata : The metadata of the saved file or folder in the user's
		// Dropbox.
		Metadata json.RawMessage `json:"metadata"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	return nil
}

// SaveUrlArg : has no documentation (yet)
type SaveUrlArg struct {
	// Path : The path in Dropbox where the URL will be saved to.
	Path string `json:"path"`
	// Url : The URL to be saved.
	Url string `json:"url"`
}

// NewSaveUrlArg returns a new SaveUrlArg instance
func NewSaveUrlArg(Path string, Url string) *SaveUrlArg {
	s := new(SaveUrlArg)
	s.Path = Path
	s.Url = Url
	return s
}

// SaveUrlError : has no documentation (yet)
type SaveUrlError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *WriteError `json:"path,omitempty"`
}

// Valid tag values for SaveUrlError
const (
	SaveUrlErrorPath           = "path"
	SaveUrlErrorDownloadFailed = "download_failed"
	SaveUrlErrorInvalidUrl     = "invalid_url"
	SaveUrlErrorNotFound       = "not_found"
	SaveUrlErrorOther          = "other"
)

// UnmarshalJSON deserializes into a SaveUrlError instance
func (u *SaveUrlError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *WriteError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// SaveUrlJobStatus : has no documentation (yet)
type SaveUrlJobStatus struct {
	dropbox.Tagged
	// Complete : Metadata of the file where the URL is saved to.
	Complete *FileMetadata `json:"complete,omitempty"`
	// Failed : has no documentation (yet)
	Failed *SaveUrlError `json:"failed,omitempty"`
}

// Valid tag values for SaveUrlJobStatus
const (
	SaveUrlJobStatusInProgress = "in_progress"
	SaveUrlJobStatusComplete   = "complete"
	SaveUrlJobStatusFailed     = "failed"
)

// UnmarshalJSON deserializes into a SaveUrlJobStatus instance
func (u *SaveUrlJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : has no documentation (yet)
		Failed *SaveUrlError `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	case "failed":
		u.Failed = w.Failed

	}
	return nil
}

// SaveUrlResult : has no documentation (yet)
type SaveUrlResult struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : Metadata of the file where the URL is saved to.
	Complete *FileMetadata `json:"complete,omitempty"`
}

// Valid tag values for SaveUrlResult
const (
	SaveUrlResultAsyncJobId = "async_job_id"
	SaveUrlResultComplete   = "complete"
)

// UnmarshalJSON deserializes into a SaveUrlResult instance
func (u *SaveUrlResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AsyncJobId : This response indicates that the processing is
		// asynchronous. The string is an id that can be used to obtain the
		// status of the asynchronous job.
		AsyncJobId string `json:"async_job_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		u.AsyncJobId = w.AsyncJobId

	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// SearchArg : has no documentation (yet)
type SearchArg struct {
	// Path : The path in the user's Dropbox to search. Should probably be a
	// folder.
	Path string `json:"path"`
	// Query : The string to search for. Query string may be rewritten to
	// improve relevance of results. The string is split on spaces into multiple
	// tokens. For file name searching, the last token is used for prefix
	// matching (i.e. "bat c" matches "bat cave" but not "batman car").
	Query string `json:"query"`
	// Start : The starting index within the search results (used for paging).
	Start uint64 `json:"start"`
	// MaxResults : The maximum number of search results to return.
	MaxResults uint64 `json:"max_results"`
	// Mode : The search mode (filename, filename_and_content, or
	// deleted_filename). Note that searching file content is only available for
	// Dropbox Business accounts.
	Mode *SearchMode `json:"mode"`
}

// NewSearchArg returns a new SearchArg instance
func NewSearchArg(Path string, Query string) *SearchArg {
	s := new(SearchArg)
	s.Path = Path
	s.Query = Query
	s.Start = 0
	s.MaxResults = 100
	s.Mode = &SearchMode{Tagged: dropbox.Tagged{Tag: "filename"}}
	return s
}

// SearchError : has no documentation (yet)
type SearchError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
	// InvalidArgument : has no documentation (yet)
	InvalidArgument string `json:"invalid_argument,omitempty"`
}

// Valid tag values for SearchError
const (
	SearchErrorPath            = "path"
	SearchErrorInvalidArgument = "invalid_argument"
	SearchErrorInternalError   = "internal_error"
	SearchErrorOther           = "other"
)

// UnmarshalJSON deserializes into a SearchError instance
func (u *SearchError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
		// InvalidArgument : has no documentation (yet)
		InvalidArgument string `json:"invalid_argument,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	case "invalid_argument":
		u.InvalidArgument = w.InvalidArgument

	}
	return nil
}

// SearchMatch : has no documentation (yet)
type SearchMatch struct {
	// MatchType : The type of the match.
	MatchType *SearchMatchType `json:"match_type"`
	// Metadata : The metadata for the matched file or folder.
	Metadata IsMetadata `json:"metadata"`
}

// NewSearchMatch returns a new SearchMatch instance
func NewSearchMatch(MatchType *SearchMatchType, Metadata IsMetadata) *SearchMatch {
	s := new(SearchMatch)
	s.MatchType = MatchType
	s.Metadata = Metadata
	return s
}

// UnmarshalJSON deserializes into a SearchMatch instance
func (u *SearchMatch) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// MatchType : The type of the match.
		MatchType *SearchMatchType `json:"match_type"`
		// Metadata : The metadata for the matched file or folder.
		Metadata json.RawMessage `json:"metadata"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	u.MatchType = w.MatchType
	Metadata, err := IsMetadataFromJSON(w.Metadata)
	if err != nil {
		return err
	}
	u.Metadata = Metadata
	return nil
}

// SearchMatchFieldOptions : has no documentation (yet)
type SearchMatchFieldOptions struct {
	// IncludeHighlights : Whether to include highlight span from file title.
	IncludeHighlights bool `json:"include_highlights"`
}

// NewSearchMatchFieldOptions returns a new SearchMatchFieldOptions instance
func NewSearchMatchFieldOptions() *SearchMatchFieldOptions {
	s := new(SearchMatchFieldOptions)
	s.IncludeHighlights = false
	return s
}

// SearchMatchType : Indicates what type of match was found for a given item.
type SearchMatchType struct {
	dropbox.Tagged
}

// Valid tag values for SearchMatchType
const (
	SearchMatchTypeFilename = "filename"
	SearchMatchTypeContent  = "content"
	SearchMatchTypeBoth     = "both"
)

// SearchMatchTypeV2 : Indicates what type of match was found for a given item.
type SearchMatchTypeV2 struct {
	dropbox.Tagged
}

// Valid tag values for SearchMatchTypeV2
const (
	SearchMatchTypeV2Filename           = "filename"
	SearchMatchTypeV2FileContent        = "file_content"
	SearchMatchTypeV2FilenameAndContent = "filename_and_content"
	SearchMatchTypeV2ImageContent       = "image_content"
	SearchMatchTypeV2Other              = "other"
)

// SearchMatchV2 : has no documentation (yet)
type SearchMatchV2 struct {
	// Metadata : The metadata for the matched file or folder.
	Metadata *MetadataV2 `json:"metadata"`
	// MatchType : The type of the match.
	MatchType *SearchMatchTypeV2 `json:"match_type,omitempty"`
	// HighlightSpans : The list of HighlightSpan determines which parts of the
	// file title should be highlighted.
	HighlightSpans []*HighlightSpan `json:"highlight_spans,omitempty"`
}

// NewSearchMatchV2 returns a new SearchMatchV2 instance
func NewSearchMatchV2(Metadata *MetadataV2) *SearchMatchV2 {
	s := new(SearchMatchV2)
	s.Metadata = Metadata
	return s
}

// SearchMode : has no documentation (yet)
type SearchMode struct {
	dropbox.Tagged
}

// Valid tag values for SearchMode
const (
	SearchModeFilename           = "filename"
	SearchModeFilenameAndContent = "filename_and_content"
	SearchModeDeletedFilename    = "deleted_filename"
)

// SearchOptions : has no documentation (yet)
type SearchOptions struct {
	// Path : Scopes the search to a path in the user's Dropbox. Searches the
	// entire Dropbox if not specified.
	Path string `json:"path,omitempty"`
	// MaxResults : The maximum number of search results to return.
	MaxResults uint64 `json:"max_results"`
	// OrderBy : Specified property of the order of search results. By default,
	// results are sorted by relevance.
	OrderBy *SearchOrderBy `json:"order_by,omitempty"`
	// FileStatus : Restricts search to the given file status.
	FileStatus *FileStatus `json:"file_status"`
	// FilenameOnly : Restricts search to only match on filenames.
	FilenameOnly bool `json:"filename_only"`
	// FileExtensions : Restricts search to only the extensions specified. Only
	// supported for active file search.
	FileExtensions []string `json:"file_extensions,omitempty"`
	// FileCategories : Restricts search to only the file categories specified.
	// Only supported for active file search.
	FileCategories []*FileCategory `json:"file_categories,omitempty"`
	// AccountId : Restricts results to the given account id.
	AccountId string `json:"account_id,omitempty"`
}

// NewSearchOptions returns a new SearchOptions instance
func NewSearchOptions() *SearchOptions {
	s := new(SearchOptions)
	s.MaxResults = 100
	s.FileStatus = &FileStatus{Tagged: dropbox.Tagged{Tag: "active"}}
	s.FilenameOnly = false
	return s
}

// SearchOrderBy : has no documentation (yet)
type SearchOrderBy struct {
	dropbox.Tagged
}

// Valid tag values for SearchOrderBy
const (
	SearchOrderByRelevance        = "relevance"
	SearchOrderByLastModifiedTime = "last_modified_time"
	SearchOrderByOther            = "other"
)

// SearchResult : has no documentation (yet)
type SearchResult struct {
	// Matches : A list (possibly empty) of matches for the query.
	Matches []*SearchMatch `json:"matches"`
	// More : Used for paging. If true, indicates there is another page of
	// results available that can be fetched by calling `search` again.
	More bool `json:"more"`
	// Start : Used for paging. Value to set the start argument to when calling
	// `search` to fetch the next page of results.
	Start uint64 `json:"start"`
}

// NewSearchResult returns a new SearchResult instance
func NewSearchResult(Matches []*SearchMatch, More bool, Start uint64) *SearchResult {
	s := new(SearchResult)
	s.Matches = Matches
	s.More = More
	s.Start = Start
	return s
}

// SearchV2Arg : has no documentation (yet)
type SearchV2Arg struct {
	// Query : The string to search for. May match across multiple fields based
	// on the request arguments.
	Query string `json:"query"`
	// Options : Options for more targeted search results.
	Options *SearchOptions `json:"options,omitempty"`
	// MatchFieldOptions : Options for search results match fields.
	MatchFieldOptions *SearchMatchFieldOptions `json:"match_field_options,omitempty"`
	// IncludeHighlights : Deprecated and moved this option to
	// SearchMatchFieldOptions.
	IncludeHighlights bool `json:"include_highlights,omitempty"`
}

// NewSearchV2Arg returns a new SearchV2Arg instance
func NewSearchV2Arg(Query string) *SearchV2Arg {
	s := new(SearchV2Arg)
	s.Query = Query
	return s
}

// SearchV2ContinueArg : has no documentation (yet)
type SearchV2ContinueArg struct {
	// Cursor : The cursor returned by your last call to `search`. Used to fetch
	// the next page of results.
	Cursor string `json:"cursor"`
}

// NewSearchV2ContinueArg returns a new SearchV2ContinueArg instance
func NewSearchV2ContinueArg(Cursor string) *SearchV2ContinueArg {
	s := new(SearchV2ContinueArg)
	s.Cursor = Cursor
	return s
}

// SearchV2Result : has no documentation (yet)
type SearchV2Result struct {
	// Matches : A list (possibly empty) of matches for the query.
	Matches []*SearchMatchV2 `json:"matches"`
	// HasMore : Used for paging. If true, indicates there is another page of
	// results available that can be fetched by calling `searchContinue` with
	// the cursor.
	HasMore bool `json:"has_more"`
	// Cursor : Pass the cursor into `searchContinue` to fetch the next page of
	// results.
	Cursor string `json:"cursor,omitempty"`
}

// NewSearchV2Result returns a new SearchV2Result instance
func NewSearchV2Result(Matches []*SearchMatchV2, HasMore bool) *SearchV2Result {
	s := new(SearchV2Result)
	s.Matches = Matches
	s.HasMore = HasMore
	return s
}

// SharedLink : has no documentation (yet)
type SharedLink struct {
	// Url : Shared link url.
	Url string `json:"url"`
	// Password : Password for the shared link.
	Password string `json:"password,omitempty"`
}

// NewSharedLink returns a new SharedLink instance
func NewSharedLink(Url string) *SharedLink {
	s := new(SharedLink)
	s.Url = Url
	return s
}

// SharedLinkFileInfo : has no documentation (yet)
type SharedLinkFileInfo struct {
	// Url : The shared link corresponding to either a file or shared link to a
	// folder. If it is for a folder shared link, we use the path param to
	// determine for which file in the folder the view is for.
	Url string `json:"url"`
	// Path : The path corresponding to a file in a shared link to a folder.
	// Required for shared links to folders.
	Path string `json:"path,omitempty"`
	// Password : Password for the shared link. Required for password-protected
	// shared links to files  unless it can be read from a cookie.
	Password string `json:"password,omitempty"`
}

// NewSharedLinkFileInfo returns a new SharedLinkFileInfo instance
func NewSharedLinkFileInfo(Url string) *SharedLinkFileInfo {
	s := new(SharedLinkFileInfo)
	s.Url = Url
	return s
}

// SingleUserLock : has no documentation (yet)
type SingleUserLock struct {
	// Created : The time the lock was created.
	Created time.Time `json:"created"`
	// LockHolderAccountId : The account ID of the lock holder if known.
	LockHolderAccountId string `json:"lock_holder_account_id"`
	// LockHolderTeamId : The id of the team of the account holder if it exists.
	LockHolderTeamId string `json:"lock_holder_team_id,omitempty"`
}

// NewSingleUserLock returns a new SingleUserLock instance
func NewSingleUserLock(Created time.Time, LockHolderAccountId string) *SingleUserLock {
	s := new(SingleUserLock)
	s.Created = Created
	s.LockHolderAccountId = LockHolderAccountId
	return s
}

// SymlinkInfo : has no documentation (yet)
type SymlinkInfo struct {
	// Target : The target this symlink points to.
	Target string `json:"target"`
}

// NewSymlinkInfo returns a new SymlinkInfo instance
func NewSymlinkInfo(Target string) *SymlinkInfo {
	s := new(SymlinkInfo)
	s.Target = Target
	return s
}

// SyncSetting : has no documentation (yet)
type SyncSetting struct {
	dropbox.Tagged
}

// Valid tag values for SyncSetting
const (
	SyncSettingDefault           = "default"
	SyncSettingNotSynced         = "not_synced"
	SyncSettingNotSyncedInactive = "not_synced_inactive"
	SyncSettingOther             = "other"
)

// SyncSettingArg : has no documentation (yet)
type SyncSettingArg struct {
	dropbox.Tagged
}

// Valid tag values for SyncSettingArg
const (
	SyncSettingArgDefault   = "default"
	SyncSettingArgNotSynced = "not_synced"
	SyncSettingArgOther     = "other"
)

// SyncSettingsError : has no documentation (yet)
type SyncSettingsError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for SyncSettingsError
const (
	SyncSettingsErrorPath                     = "path"
	SyncSettingsErrorUnsupportedCombination   = "unsupported_combination"
	SyncSettingsErrorUnsupportedConfiguration = "unsupported_configuration"
	SyncSettingsErrorOther                    = "other"
)

// UnmarshalJSON deserializes into a SyncSettingsError instance
func (u *SyncSettingsError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// Tag : Tag that can be added in multiple ways.
type Tag struct {
	dropbox.Tagged
	// UserGeneratedTag : Tag generated by the user.
	UserGeneratedTag *UserGeneratedTag `json:"user_generated_tag,omitempty"`
}

// Valid tag values for Tag
const (
	TagUserGeneratedTag = "user_generated_tag"
	TagOther            = "other"
)

// UnmarshalJSON deserializes into a Tag instance
func (u *Tag) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_generated_tag":
		if err = json.Unmarshal(body, &u.UserGeneratedTag); err != nil {
			return err
		}

	}
	return nil
}

// ThumbnailArg : has no documentation (yet)
type ThumbnailArg struct {
	// Path : The path to the image file you want to thumbnail.
	Path string `json:"path"`
	// Format : The format for the thumbnail image, jpeg (default) or png. For
	// images that are photos, jpeg should be preferred, while png is  better
	// for screenshots and digital arts.
	Format *ThumbnailFormat `json:"format"`
	// Size : The size for the thumbnail image.
	Size *ThumbnailSize `json:"size"`
	// Mode : How to resize and crop the image to achieve the desired size.
	Mode *ThumbnailMode `json:"mode"`
}

// NewThumbnailArg returns a new ThumbnailArg instance
func NewThumbnailArg(Path string) *ThumbnailArg {
	s := new(ThumbnailArg)
	s.Path = Path
	s.Format = &ThumbnailFormat{Tagged: dropbox.Tagged{Tag: "jpeg"}}
	s.Size = &ThumbnailSize{Tagged: dropbox.Tagged{Tag: "w64h64"}}
	s.Mode = &ThumbnailMode{Tagged: dropbox.Tagged{Tag: "strict"}}
	return s
}

// ThumbnailError : has no documentation (yet)
type ThumbnailError struct {
	dropbox.Tagged
	// Path : An error occurs when downloading metadata for the image.
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for ThumbnailError
const (
	ThumbnailErrorPath                 = "path"
	ThumbnailErrorUnsupportedExtension = "unsupported_extension"
	ThumbnailErrorUnsupportedImage     = "unsupported_image"
	ThumbnailErrorConversionError      = "conversion_error"
)

// UnmarshalJSON deserializes into a ThumbnailError instance
func (u *ThumbnailError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : An error occurs when downloading metadata for the image.
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// ThumbnailFormat : has no documentation (yet)
type ThumbnailFormat struct {
	dropbox.Tagged
}

// Valid tag values for ThumbnailFormat
const (
	ThumbnailFormatJpeg = "jpeg"
	ThumbnailFormatPng  = "png"
)

// ThumbnailMode : has no documentation (yet)
type ThumbnailMode struct {
	dropbox.Tagged
}

// Valid tag values for ThumbnailMode
const (
	ThumbnailModeStrict        = "strict"
	ThumbnailModeBestfit       = "bestfit"
	ThumbnailModeFitoneBestfit = "fitone_bestfit"
)

// ThumbnailSize : has no documentation (yet)
type ThumbnailSize struct {
	dropbox.Tagged
}

// Valid tag values for ThumbnailSize
const (
	ThumbnailSizeW32h32     = "w32h32"
	ThumbnailSizeW64h64     = "w64h64"
	ThumbnailSizeW128h128   = "w128h128"
	ThumbnailSizeW256h256   = "w256h256"
	ThumbnailSizeW480h320   = "w480h320"
	ThumbnailSizeW640h480   = "w640h480"
	ThumbnailSizeW960h640   = "w960h640"
	ThumbnailSizeW1024h768  = "w1024h768"
	ThumbnailSizeW2048h1536 = "w2048h1536"
)

// ThumbnailV2Arg : has no documentation (yet)
type ThumbnailV2Arg struct {
	// Resource : Information specifying which file to preview. This could be a
	// path to a file, a shared link pointing to a file, or a shared link
	// pointing to a folder, with a relative path.
	Resource *PathOrLink `json:"resource"`
	// Format : The format for the thumbnail image, jpeg (default) or png. For
	// images that are photos, jpeg should be preferred, while png is  better
	// for screenshots and digital arts.
	Format *ThumbnailFormat `json:"format"`
	// Size : The size for the thumbnail image.
	Size *ThumbnailSize `json:"size"`
	// Mode : How to resize and crop the image to achieve the desired size.
	Mode *ThumbnailMode `json:"mode"`
}

// NewThumbnailV2Arg returns a new ThumbnailV2Arg instance
func NewThumbnailV2Arg(Resource *PathOrLink) *ThumbnailV2Arg {
	s := new(ThumbnailV2Arg)
	s.Resource = Resource
	s.Format = &ThumbnailFormat{Tagged: dropbox.Tagged{Tag: "jpeg"}}
	s.Size = &ThumbnailSize{Tagged: dropbox.Tagged{Tag: "w64h64"}}
	s.Mode = &ThumbnailMode{Tagged: dropbox.Tagged{Tag: "strict"}}
	return s
}

// ThumbnailV2Error : has no documentation (yet)
type ThumbnailV2Error struct {
	dropbox.Tagged
	// Path : An error occurred when downloading metadata for the image.
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for ThumbnailV2Error
const (
	ThumbnailV2ErrorPath                 = "path"
	ThumbnailV2ErrorUnsupportedExtension = "unsupported_extension"
	ThumbnailV2ErrorUnsupportedImage     = "unsupported_image"
	ThumbnailV2ErrorConversionError      = "conversion_error"
	ThumbnailV2ErrorAccessDenied         = "access_denied"
	ThumbnailV2ErrorNotFound             = "not_found"
	ThumbnailV2ErrorOther                = "other"
)

// UnmarshalJSON deserializes into a ThumbnailV2Error instance
func (u *ThumbnailV2Error) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : An error occurred when downloading metadata for the image.
		Path *LookupError `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		u.Path = w.Path

	}
	return nil
}

// UnlockFileArg : has no documentation (yet)
type UnlockFileArg struct {
	// Path : Path in the user's Dropbox to a file.
	Path string `json:"path"`
}

// NewUnlockFileArg returns a new UnlockFileArg instance
func NewUnlockFileArg(Path string) *UnlockFileArg {
	s := new(UnlockFileArg)
	s.Path = Path
	return s
}

// UnlockFileBatchArg : has no documentation (yet)
type UnlockFileBatchArg struct {
	// Entries : List of 'entries'. Each 'entry' contains a path of the file
	// which will be unlocked. Duplicate path arguments in the batch are
	// considered only once.
	Entries []*UnlockFileArg `json:"entries"`
}

// NewUnlockFileBatchArg returns a new UnlockFileBatchArg instance
func NewUnlockFileBatchArg(Entries []*UnlockFileArg) *UnlockFileBatchArg {
	s := new(UnlockFileBatchArg)
	s.Entries = Entries
	return s
}

// UploadArg : has no documentation (yet)
type UploadArg struct {
	CommitInfo
	// ContentHash : A hash of the file content uploaded in this call. If
	// provided and the uploaded content does not match this hash, an error will
	// be returned. For more information see our `Content hash`
	// <https://www.dropbox.com/developers/reference/content-hash> page.
	ContentHash string `json:"content_hash,omitempty"`
}

// NewUploadArg returns a new UploadArg instance
func NewUploadArg(Path string) *UploadArg {
	s := new(UploadArg)
	s.Path = Path
	s.Mode = &WriteMode{Tagged: dropbox.Tagged{Tag: "add"}}
	s.Autorename = false
	s.Mute = false
	s.StrictConflict = false
	return s
}

// UploadError : has no documentation (yet)
type UploadError struct {
	dropbox.Tagged
	// Path : Unable to save the uploaded contents to a file.
	Path *UploadWriteFailed `json:"path,omitempty"`
	// PropertiesError : The supplied property group is invalid. The file has
	// uploaded without property groups.
	PropertiesError *file_properties.InvalidPropertyGroupError `json:"properties_error,omitempty"`
}

// Valid tag values for UploadError
const (
	UploadErrorPath                = "path"
	UploadErrorPropertiesError     = "properties_error"
	UploadErrorPayloadTooLarge     = "payload_too_large"
	UploadErrorContentHashMismatch = "content_hash_mismatch"
	UploadErrorOther               = "other"
)

// UnmarshalJSON deserializes into a UploadError instance
func (u *UploadError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PropertiesError : The supplied property group is invalid. The file
		// has uploaded without property groups.
		PropertiesError *file_properties.InvalidPropertyGroupError `json:"properties_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		if err = json.Unmarshal(body, &u.Path); err != nil {
			return err
		}

	case "properties_error":
		u.PropertiesError = w.PropertiesError

	}
	return nil
}

// UploadSessionAppendArg : has no documentation (yet)
type UploadSessionAppendArg struct {
	// Cursor : Contains the upload session ID and the offset.
	Cursor *UploadSessionCursor `json:"cursor"`
	// Close : If true, the current session will be closed, at which point you
	// won't be able to call `uploadSessionAppend` anymore with the current
	// session.
	Close bool `json:"close"`
	// ContentHash : A hash of the file content uploaded in this call. If
	// provided and the uploaded content does not match this hash, an error will
	// be returned. For more information see our `Content hash`
	// <https://www.dropbox.com/developers/reference/content-hash> page.
	ContentHash string `json:"content_hash,omitempty"`
}

// NewUploadSessionAppendArg returns a new UploadSessionAppendArg instance
func NewUploadSessionAppendArg(Cursor *UploadSessionCursor) *UploadSessionAppendArg {
	s := new(UploadSessionAppendArg)
	s.Cursor = Cursor
	s.Close = false
	return s
}

// UploadSessionLookupError : has no documentation (yet)
type UploadSessionLookupError struct {
	dropbox.Tagged
	// IncorrectOffset : The specified offset was incorrect. See the value for
	// the correct offset. This error may occur when a previous request was
	// received and processed successfully but the client did not receive the
	// response, e.g. due to a network error.
	IncorrectOffset *UploadSessionOffsetError `json:"incorrect_offset,omitempty"`
}

// Valid tag values for UploadSessionLookupError
const (
	UploadSessionLookupErrorNotFound                         = "not_found"
	UploadSessionLookupErrorIncorrectOffset                  = "incorrect_offset"
	UploadSessionLookupErrorClosed                           = "closed"
	UploadSessionLookupErrorNotClosed                        = "not_closed"
	UploadSessionLookupErrorTooLarge                         = "too_large"
	UploadSessionLookupErrorConcurrentSessionInvalidOffset   = "concurrent_session_invalid_offset"
	UploadSessionLookupErrorConcurrentSessionInvalidDataSize = "concurrent_session_invalid_data_size"
	UploadSessionLookupErrorPayloadTooLarge                  = "payload_too_large"
	UploadSessionLookupErrorOther                            = "other"
)

// UnmarshalJSON deserializes into a UploadSessionLookupError instance
func (u *UploadSessionLookupError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "incorrect_offset":
		if err = json.Unmarshal(body, &u.IncorrectOffset); err != nil {
			return err
		}

	}
	return nil
}

// UploadSessionAppendError : has no documentation (yet)
type UploadSessionAppendError struct {
	dropbox.Tagged
	// IncorrectOffset : The specified offset was incorrect. See the value for
	// the correct offset. This error may occur when a previous request was
	// received and processed successfully but the client did not receive the
	// response, e.g. due to a network error.
	IncorrectOffset *UploadSessionOffsetError `json:"incorrect_offset,omitempty"`
}

// Valid tag values for UploadSessionAppendError
const (
	UploadSessionAppendErrorNotFound                         = "not_found"
	UploadSessionAppendErrorIncorrectOffset                  = "incorrect_offset"
	UploadSessionAppendErrorClosed                           = "closed"
	UploadSessionAppendErrorNotClosed                        = "not_closed"
	UploadSessionAppendErrorTooLarge                         = "too_large"
	UploadSessionAppendErrorConcurrentSessionInvalidOffset   = "concurrent_session_invalid_offset"
	UploadSessionAppendErrorConcurrentSessionInvalidDataSize = "concurrent_session_invalid_data_size"
	UploadSessionAppendErrorPayloadTooLarge                  = "payload_too_large"
	UploadSessionAppendErrorOther                            = "other"
	UploadSessionAppendErrorContentHashMismatch              = "content_hash_mismatch"
)

// UnmarshalJSON deserializes into a UploadSessionAppendError instance
func (u *UploadSessionAppendError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "incorrect_offset":
		if err = json.Unmarshal(body, &u.IncorrectOffset); err != nil {
			return err
		}

	}
	return nil
}

// UploadSessionCursor : has no documentation (yet)
type UploadSessionCursor struct {
	// SessionId : The upload session ID (returned by `uploadSessionStart`).
	SessionId string `json:"session_id"`
	// Offset : Offset in bytes at which data should be appended. We use this to
	// make sure upload data isn't lost or duplicated in the event of a network
	// error.
	Offset uint64 `json:"offset"`
}

// NewUploadSessionCursor returns a new UploadSessionCursor instance
func NewUploadSessionCursor(SessionId string, Offset uint64) *UploadSessionCursor {
	s := new(UploadSessionCursor)
	s.SessionId = SessionId
	s.Offset = Offset
	return s
}

// UploadSessionFinishArg : has no documentation (yet)
type UploadSessionFinishArg struct {
	// Cursor : Contains the upload session ID and the offset.
	Cursor *UploadSessionCursor `json:"cursor"`
	// Commit : Contains the path and other optional modifiers for the commit.
	Commit *CommitInfo `json:"commit"`
	// ContentHash : A hash of the file content uploaded in this call. If
	// provided and the uploaded content does not match this hash, an error will
	// be returned. For more information see our `Content hash`
	// <https://www.dropbox.com/developers/reference/content-hash> page.
	ContentHash string `json:"content_hash,omitempty"`
}

// NewUploadSessionFinishArg returns a new UploadSessionFinishArg instance
func NewUploadSessionFinishArg(Cursor *UploadSessionCursor, Commit *CommitInfo) *UploadSessionFinishArg {
	s := new(UploadSessionFinishArg)
	s.Cursor = Cursor
	s.Commit = Commit
	return s
}

// UploadSessionFinishBatchArg : has no documentation (yet)
type UploadSessionFinishBatchArg struct {
	// Entries : Commit information for each file in the batch.
	Entries []*UploadSessionFinishArg `json:"entries"`
}

// NewUploadSessionFinishBatchArg returns a new UploadSessionFinishBatchArg instance
func NewUploadSessionFinishBatchArg(Entries []*UploadSessionFinishArg) *UploadSessionFinishBatchArg {
	s := new(UploadSessionFinishBatchArg)
	s.Entries = Entries
	return s
}

// UploadSessionFinishBatchJobStatus : has no documentation (yet)
type UploadSessionFinishBatchJobStatus struct {
	dropbox.Tagged
	// Complete : The `uploadSessionFinishBatch` has finished.
	Complete *UploadSessionFinishBatchResult `json:"complete,omitempty"`
}

// Valid tag values for UploadSessionFinishBatchJobStatus
const (
	UploadSessionFinishBatchJobStatusInProgress = "in_progress"
	UploadSessionFinishBatchJobStatusComplete   = "complete"
)

// UnmarshalJSON deserializes into a UploadSessionFinishBatchJobStatus instance
func (u *UploadSessionFinishBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// UploadSessionFinishBatchLaunch : Result returned by
// `uploadSessionFinishBatch` that may either launch an asynchronous job or
// complete synchronously.
type UploadSessionFinishBatchLaunch struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : has no documentation (yet)
	Complete *UploadSessionFinishBatchResult `json:"complete,omitempty"`
}

// Valid tag values for UploadSessionFinishBatchLaunch
const (
	UploadSessionFinishBatchLaunchAsyncJobId = "async_job_id"
	UploadSessionFinishBatchLaunchComplete   = "complete"
	UploadSessionFinishBatchLaunchOther      = "other"
)

// UnmarshalJSON deserializes into a UploadSessionFinishBatchLaunch instance
func (u *UploadSessionFinishBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AsyncJobId : This response indicates that the processing is
		// asynchronous. The string is an id that can be used to obtain the
		// status of the asynchronous job.
		AsyncJobId string `json:"async_job_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		u.AsyncJobId = w.AsyncJobId

	case "complete":
		if err = json.Unmarshal(body, &u.Complete); err != nil {
			return err
		}

	}
	return nil
}

// UploadSessionFinishBatchResult : has no documentation (yet)
type UploadSessionFinishBatchResult struct {
	// Entries : Each entry in `UploadSessionFinishBatchArg.entries` will appear
	// at the same position inside `UploadSessionFinishBatchResult.entries`.
	Entries []*UploadSessionFinishBatchResultEntry `json:"entries"`
}

// NewUploadSessionFinishBatchResult returns a new UploadSessionFinishBatchResult instance
func NewUploadSessionFinishBatchResult(Entries []*UploadSessionFinishBatchResultEntry) *UploadSessionFinishBatchResult {
	s := new(UploadSessionFinishBatchResult)
	s.Entries = Entries
	return s
}

// UploadSessionFinishBatchResultEntry : has no documentation (yet)
type UploadSessionFinishBatchResultEntry struct {
	dropbox.Tagged
	// Success : has no documentation (yet)
	Success *FileMetadata `json:"success,omitempty"`
	// Failure : has no documentation (yet)
	Failure *UploadSessionFinishError `json:"failure,omitempty"`
}

// Valid tag values for UploadSessionFinishBatchResultEntry
const (
	UploadSessionFinishBatchResultEntrySuccess = "success"
	UploadSessionFinishBatchResultEntryFailure = "failure"
)

// UnmarshalJSON deserializes into a UploadSessionFinishBatchResultEntry instance
func (u *UploadSessionFinishBatchResultEntry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failure : has no documentation (yet)
		Failure *UploadSessionFinishError `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		if err = json.Unmarshal(body, &u.Success); err != nil {
			return err
		}

	case "failure":
		u.Failure = w.Failure

	}
	return nil
}

// UploadSessionFinishError : has no documentation (yet)
type UploadSessionFinishError struct {
	dropbox.Tagged
	// LookupFailed : The session arguments are incorrect; the value explains
	// the reason.
	LookupFailed *UploadSessionLookupError `json:"lookup_failed,omitempty"`
	// Path : Unable to save the uploaded contents to a file. Data has already
	// been appended to the upload session. Please retry with empty data body
	// and updated offset.
	Path *WriteError `json:"path,omitempty"`
	// PropertiesError : The supplied property group is invalid. The file has
	// uploaded without property groups.
	PropertiesError *file_properties.InvalidPropertyGroupError `json:"properties_error,omitempty"`
}

// Valid tag values for UploadSessionFinishError
const (
	UploadSessionFinishErrorLookupFailed                    = "lookup_failed"
	UploadSessionFinishErrorPath                            = "path"
	UploadSessionFinishErrorPropertiesError                 = "properties_error"
	UploadSessionFinishErrorTooManySharedFolderTargets      = "too_many_shared_folder_targets"
	UploadSessionFinishErrorTooManyWriteOperations          = "too_many_write_operations"
	UploadSessionFinishErrorConcurrentSessionDataNotAllowed = "concurrent_session_data_not_allowed"
	UploadSessionFinishErrorConcurrentSessionNotClosed      = "concurrent_session_not_closed"
	UploadSessionFinishErrorConcurrentSessionMissingData    = "concurrent_session_missing_data"
	UploadSessionFinishErrorPayloadTooLarge                 = "payload_too_large"
	UploadSessionFinishErrorContentHashMismatch             = "content_hash_mismatch"
	UploadSessionFinishErrorOther                           = "other"
)

// UnmarshalJSON deserializes into a UploadSessionFinishError instance
func (u *UploadSessionFinishError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// LookupFailed : The session arguments are incorrect; the value
		// explains the reason.
		LookupFailed *UploadSessionLookupError `json:"lookup_failed,omitempty"`
		// Path : Unable to save the uploaded contents to a file. Data has
		// already been appended to the upload session. Please retry with empty
		// data body and updated offset.
		Path *WriteError `json:"path,omitempty"`
		// PropertiesError : The supplied property group is invalid. The file
		// has uploaded without property groups.
		PropertiesError *file_properties.InvalidPropertyGroupError `json:"properties_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "lookup_failed":
		u.LookupFailed = w.LookupFailed

	case "path":
		u.Path = w.Path

	case "properties_error":
		u.PropertiesError = w.PropertiesError

	}
	return nil
}

// UploadSessionOffsetError : has no documentation (yet)
type UploadSessionOffsetError struct {
	// CorrectOffset : The offset up to which data has been collected.
	CorrectOffset uint64 `json:"correct_offset"`
}

// NewUploadSessionOffsetError returns a new UploadSessionOffsetError instance
func NewUploadSessionOffsetError(CorrectOffset uint64) *UploadSessionOffsetError {
	s := new(UploadSessionOffsetError)
	s.CorrectOffset = CorrectOffset
	return s
}

// UploadSessionStartArg : has no documentation (yet)
type UploadSessionStartArg struct {
	// Close : If true, the current session will be closed, at which point you
	// won't be able to call `uploadSessionAppend` anymore with the current
	// session.
	Close bool `json:"close"`
	// SessionType : Type of upload session you want to start. If not specified,
	// default is `UploadSessionType.sequential`.
	SessionType *UploadSessionType `json:"session_type,omitempty"`
	// ContentHash : A hash of the file content uploaded in this call. If
	// provided and the uploaded content does not match this hash, an error will
	// be returned. For more information see our `Content hash`
	// <https://www.dropbox.com/developers/reference/content-hash> page.
	ContentHash string `json:"content_hash,omitempty"`
}

// NewUploadSessionStartArg returns a new UploadSessionStartArg instance
func NewUploadSessionStartArg() *UploadSessionStartArg {
	s := new(UploadSessionStartArg)
	s.Close = false
	return s
}

// UploadSessionStartBatchArg : has no documentation (yet)
type UploadSessionStartBatchArg struct {
	// SessionType : Type of upload session you want to start. If not specified,
	// default is `UploadSessionType.sequential`.
	SessionType *UploadSessionType `json:"session_type,omitempty"`
	// NumSessions : The number of upload sessions to start.
	NumSessions uint64 `json:"num_sessions"`
}

// NewUploadSessionStartBatchArg returns a new UploadSessionStartBatchArg instance
func NewUploadSessionStartBatchArg(NumSessions uint64) *UploadSessionStartBatchArg {
	s := new(UploadSessionStartBatchArg)
	s.NumSessions = NumSessions
	return s
}

// UploadSessionStartBatchResult : has no documentation (yet)
type UploadSessionStartBatchResult struct {
	// SessionIds : A List of unique identifiers for the upload session. Pass
	// each session_id to `uploadSessionAppend` and `uploadSessionFinish`.
	SessionIds []string `json:"session_ids"`
}

// NewUploadSessionStartBatchResult returns a new UploadSessionStartBatchResult instance
func NewUploadSessionStartBatchResult(SessionIds []string) *UploadSessionStartBatchResult {
	s := new(UploadSessionStartBatchResult)
	s.SessionIds = SessionIds
	return s
}

// UploadSessionStartError : has no documentation (yet)
type UploadSessionStartError struct {
	dropbox.Tagged
}

// Valid tag values for UploadSessionStartError
const (
	UploadSessionStartErrorConcurrentSessionDataNotAllowed  = "concurrent_session_data_not_allowed"
	UploadSessionStartErrorConcurrentSessionCloseNotAllowed = "concurrent_session_close_not_allowed"
	UploadSessionStartErrorPayloadTooLarge                  = "payload_too_large"
	UploadSessionStartErrorContentHashMismatch              = "content_hash_mismatch"
	UploadSessionStartErrorOther                            = "other"
)

// UploadSessionStartResult : has no documentation (yet)
type UploadSessionStartResult struct {
	// SessionId : A unique identifier for the upload session. Pass this to
	// `uploadSessionAppend` and `uploadSessionFinish`.
	SessionId string `json:"session_id"`
}

// NewUploadSessionStartResult returns a new UploadSessionStartResult instance
func NewUploadSessionStartResult(SessionId string) *UploadSessionStartResult {
	s := new(UploadSessionStartResult)
	s.SessionId = SessionId
	return s
}

// UploadSessionType : has no documentation (yet)
type UploadSessionType struct {
	dropbox.Tagged
}

// Valid tag values for UploadSessionType
const (
	UploadSessionTypeSequential = "sequential"
	UploadSessionTypeConcurrent = "concurrent"
	UploadSessionTypeOther      = "other"
)

// UploadWriteFailed : has no documentation (yet)
type UploadWriteFailed struct {
	// Reason : The reason why the file couldn't be saved.
	Reason *WriteError `json:"reason"`
	// UploadSessionId : The upload session ID; data has already been uploaded
	// to the corresponding upload session and this ID may be used to retry the
	// commit with `uploadSessionFinish`.
	UploadSessionId string `json:"upload_session_id"`
}

// NewUploadWriteFailed returns a new UploadWriteFailed instance
func NewUploadWriteFailed(Reason *WriteError, UploadSessionId string) *UploadWriteFailed {
	s := new(UploadWriteFailed)
	s.Reason = Reason
	s.UploadSessionId = UploadSessionId
	return s
}

// UserGeneratedTag : has no documentation (yet)
type UserGeneratedTag struct {
	// TagText : has no documentation (yet)
	TagText string `json:"tag_text"`
}

// NewUserGeneratedTag returns a new UserGeneratedTag instance
func NewUserGeneratedTag(TagText string) *UserGeneratedTag {
	s := new(UserGeneratedTag)
	s.TagText = TagText
	return s
}

// VideoMetadata : Metadata for a video.
type VideoMetadata struct {
	MediaMetadata
	// Duration : The duration of the video in milliseconds.
	Duration uint64 `json:"duration,omitempty"`
}

// NewVideoMetadata returns a new VideoMetadata instance
func NewVideoMetadata() *VideoMetadata {
	s := new(VideoMetadata)
	return s
}

// WriteConflictError : has no documentation (yet)
type WriteConflictError struct {
	dropbox.Tagged
}

// Valid tag values for WriteConflictError
const (
	WriteConflictErrorFile         = "file"
	WriteConflictErrorFolder       = "folder"
	WriteConflictErrorFileAncestor = "file_ancestor"
	WriteConflictErrorOther        = "other"
)

// WriteError : has no documentation (yet)
type WriteError struct {
	dropbox.Tagged
	// MalformedPath : The given path does not satisfy the required path format.
	// Please refer to the `Path formats documentation`
	// <https://www.dropbox.com/developers/documentation/http/documentation#path-formats>
	// for more information.
	MalformedPath string `json:"malformed_path,omitempty"`
	// Conflict : Couldn't write to the target path because there was something
	// in the way.
	Conflict *WriteConflictError `json:"conflict,omitempty"`
}

// Valid tag values for WriteError
const (
	WriteErrorMalformedPath          = "malformed_path"
	WriteErrorConflict               = "conflict"
	WriteErrorNoWritePermission      = "no_write_permission"
	WriteErrorInsufficientSpace      = "insufficient_space"
	WriteErrorDisallowedName         = "disallowed_name"
	WriteErrorTeamFolder             = "team_folder"
	WriteErrorOperationSuppressed    = "operation_suppressed"
	WriteErrorTooManyWriteOperations = "too_many_write_operations"
	WriteErrorOther                  = "other"
)

// UnmarshalJSON deserializes into a WriteError instance
func (u *WriteError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MalformedPath : The given path does not satisfy the required path
		// format. Please refer to the `Path formats documentation`
		// <https://www.dropbox.com/developers/documentation/http/documentation#path-formats>
		// for more information.
		MalformedPath string `json:"malformed_path,omitempty"`
		// Conflict : Couldn't write to the target path because there was
		// something in the way.
		Conflict *WriteConflictError `json:"conflict,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "malformed_path":
		u.MalformedPath = w.MalformedPath

	case "conflict":
		u.Conflict = w.Conflict

	}
	return nil
}

// WriteMode : Your intent when writing a file to some path. This is used to
// determine what constitutes a conflict and what the autorename strategy is. In
// some situations, the conflict behavior is identical: (a) If the target path
// doesn't refer to anything, the file is always written; no conflict. (b) If
// the target path refers to a folder, it's always a conflict. (c) If the target
// path refers to a file with identical contents, nothing gets written; no
// conflict. The conflict checking differs in the case where there's a file at
// the target path with contents different from the contents you're trying to
// write.
type WriteMode struct {
	dropbox.Tagged
	// Update : Overwrite if the given "rev" matches the existing file's "rev".
	// The supplied value should be the latest known "rev" of the file, for
	// example, from `FileMetadata`, from when the file was last downloaded by
	// the app. This will cause the file on the Dropbox servers to be
	// overwritten if the given "rev" matches the existing file's current "rev"
	// on the Dropbox servers. The autorename strategy is to append the string
	// "conflicted copy" to the file name. For example, "document.txt" might
	// become "document (conflicted copy).txt" or "document (Panda's conflicted
	// copy).txt".
	Update string `json:"update,omitempty"`
}

// Valid tag values for WriteMode
const (
	WriteModeAdd       = "add"
	WriteModeOverwrite = "overwrite"
	WriteModeUpdate    = "update"
)

// UnmarshalJSON deserializes into a WriteMode instance
func (u *WriteMode) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Update : Overwrite if the given "rev" matches the existing file's
		// "rev". The supplied value should be the latest known "rev" of the
		// file, for example, from `FileMetadata`, from when the file was last
		// downloaded by the app. This will cause the file on the Dropbox
		// servers to be overwritten if the given "rev" matches the existing
		// file's current "rev" on the Dropbox servers. The autorename strategy
		// is to append the string "conflicted copy" to the file name. For
		// example, "document.txt" might become "document (conflicted copy).txt"
		// or "document (Panda's conflicted copy).txt".
		Update string `json:"update,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "update":
		u.Update = w.Update

	}
	return nil
}
