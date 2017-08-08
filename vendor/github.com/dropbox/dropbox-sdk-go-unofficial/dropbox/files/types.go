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

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/properties"
)

// PropertiesError : has no documentation (yet)
type PropertiesError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for PropertiesError
const (
	PropertiesErrorPath = "path"
)

// UnmarshalJSON deserializes into a PropertiesError instance
func (u *PropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// InvalidPropertyGroupError : has no documentation (yet)
type InvalidPropertyGroupError struct {
	dropbox.Tagged
}

// Valid tag values for InvalidPropertyGroupError
const (
	InvalidPropertyGroupErrorPropertyFieldTooLarge = "property_field_too_large"
	InvalidPropertyGroupErrorDoesNotFitTemplate    = "does_not_fit_template"
)

// AddPropertiesError : has no documentation (yet)
type AddPropertiesError struct {
	dropbox.Tagged
}

// Valid tag values for AddPropertiesError
const (
	AddPropertiesErrorPropertyGroupAlreadyExists = "property_group_already_exists"
)

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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// AlphaGetMetadataError : has no documentation (yet)
type AlphaGetMetadataError struct {
	dropbox.Tagged
	// PropertiesError : has no documentation (yet)
	PropertiesError *LookUpPropertiesError `json:"properties_error,omitempty"`
}

// Valid tag values for AlphaGetMetadataError
const (
	AlphaGetMetadataErrorPropertiesError = "properties_error"
)

// UnmarshalJSON deserializes into a AlphaGetMetadataError instance
func (u *AlphaGetMetadataError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PropertiesError : has no documentation (yet)
		PropertiesError json.RawMessage `json:"properties_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "properties_error":
		err = json.Unmarshal(w.PropertiesError, &u.PropertiesError)

		if err != nil {
			return err
		}
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
	ClientModified time.Time `json:"client_modified,omitempty"`
	// Mute : Normally, users are made aware of any file modifications in their
	// Dropbox account via notifications in the client software. If true, this
	// tells the clients that this modification shouldn't result in a user
	// notification.
	Mute bool `json:"mute"`
}

// NewCommitInfo returns a new CommitInfo instance
func NewCommitInfo(Path string) *CommitInfo {
	s := new(CommitInfo)
	s.Path = Path
	s.Mode = &WriteMode{Tagged: dropbox.Tagged{"add"}}
	s.Autorename = false
	s.Mute = false
	return s
}

// CommitInfoWithProperties : has no documentation (yet)
type CommitInfoWithProperties struct {
	CommitInfo
	// PropertyGroups : List of custom properties to add to file.
	PropertyGroups []*properties.PropertyGroup `json:"property_groups,omitempty"`
}

// NewCommitInfoWithProperties returns a new CommitInfoWithProperties instance
func NewCommitInfoWithProperties(Path string) *CommitInfoWithProperties {
	s := new(CommitInfoWithProperties)
	s.Path = Path
	s.Mode = &WriteMode{Tagged: dropbox.Tagged{"add"}}
	s.Autorename = false
	s.Mute = false
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
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
	DeleteBatchJobStatusComplete = "complete"
	DeleteBatchJobStatusFailed   = "failed"
	DeleteBatchJobStatusOther    = "other"
)

// UnmarshalJSON deserializes into a DeleteBatchJobStatus instance
func (u *DeleteBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : The batch delete has finished.
		Complete json.RawMessage `json:"complete,omitempty"`
		// Failed : The batch delete has failed.
		Failed json.RawMessage `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
			return err
		}
	case "failed":
		err = json.Unmarshal(w.Failed, &u.Failed)

		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatchLaunch : Result returned by `deleteBatch` that may either launch
// an asynchronous job or complete synchronously.
type DeleteBatchLaunch struct {
	dropbox.Tagged
	// Complete : has no documentation (yet)
	Complete *DeleteBatchResult `json:"complete,omitempty"`
}

// Valid tag values for DeleteBatchLaunch
const (
	DeleteBatchLaunchComplete = "complete"
	DeleteBatchLaunchOther    = "other"
)

// UnmarshalJSON deserializes into a DeleteBatchLaunch instance
func (u *DeleteBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : has no documentation (yet)
		Complete json.RawMessage `json:"complete,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatchResult : has no documentation (yet)
type DeleteBatchResult struct {
	FileOpsResult
	// Entries : has no documentation (yet)
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
		// Success : has no documentation (yet)
		Success json.RawMessage `json:"success,omitempty"`
		// Failure : has no documentation (yet)
		Failure json.RawMessage `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		err = json.Unmarshal(body, &u.Success)

		if err != nil {
			return err
		}
	case "failure":
		err = json.Unmarshal(w.Failure, &u.Failure)

		if err != nil {
			return err
		}
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
		PathLookup json.RawMessage `json:"path_lookup,omitempty"`
		// PathWrite : has no documentation (yet)
		PathWrite json.RawMessage `json:"path_write,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path_lookup":
		err = json.Unmarshal(w.PathLookup, &u.PathLookup)

		if err != nil {
			return err
		}
	case "path_write":
		err = json.Unmarshal(w.PathWrite, &u.PathWrite)

		if err != nil {
			return err
		}
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
	// ParentSharedFolderId : Deprecated. Please use
	// `FileSharingInfo.parent_shared_folder_id` or
	// `FolderSharingInfo.parent_shared_folder_id` instead.
	ParentSharedFolderId string `json:"parent_shared_folder_id,omitempty"`
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
		// File : has no documentation (yet)
		File json.RawMessage `json:"file,omitempty"`
		// Folder : has no documentation (yet)
		Folder json.RawMessage `json:"folder,omitempty"`
		// Deleted : has no documentation (yet)
		Deleted json.RawMessage `json:"deleted,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "file":
		err = json.Unmarshal(body, &u.File)

		if err != nil {
			return err
		}
	case "folder":
		err = json.Unmarshal(body, &u.Folder)

		if err != nil {
			return err
		}
	case "deleted":
		err = json.Unmarshal(body, &u.Deleted)

		if err != nil {
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
	// Rev : Deprecated. Please specify revision in `path` instead.
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
	DownloadErrorPath  = "path"
	DownloadErrorOther = "other"
)

// UnmarshalJSON deserializes into a DownloadError instance
func (u *DownloadError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
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
	// MediaInfo : Additional information if the file is a photo or video.
	MediaInfo *MediaInfo `json:"media_info,omitempty"`
	// SharingInfo : Set if this file is contained in a shared folder.
	SharingInfo *FileSharingInfo `json:"sharing_info,omitempty"`
	// PropertyGroups : Additional information if the file has custom properties
	// with the property template specified.
	PropertyGroups []*properties.PropertyGroup `json:"property_groups,omitempty"`
	// HasExplicitSharedMembers : This flag will only be present if
	// include_has_explicit_shared_members  is true in `listFolder` or
	// `getMetadata`. If this  flag is present, it will be true if this file has
	// any explicit shared  members. This is different from sharing_info in that
	// this could be true  in the case where a file has explicit members but is
	// not contained within  a shared folder.
	HasExplicitSharedMembers bool `json:"has_explicit_shared_members,omitempty"`
	// ContentHash : A hash of the file content. This field can be used to
	// verify data integrity. For more information see our `Content hash`
	// </developers/reference/content-hash> page.
	ContentHash string `json:"content_hash,omitempty"`
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

// FolderMetadata : has no documentation (yet)
type FolderMetadata struct {
	Metadata
	// Id : A unique identifier for the folder.
	Id string `json:"id"`
	// SharedFolderId : Deprecated. Please use `sharing_info` instead.
	SharedFolderId string `json:"shared_folder_id,omitempty"`
	// SharingInfo : Set if the folder is contained in a shared folder or is a
	// shared folder mount point.
	SharingInfo *FolderSharingInfo `json:"sharing_info,omitempty"`
	// PropertyGroups : Additional information if the file has custom properties
	// with the property template specified.
	PropertyGroups []*properties.PropertyGroup `json:"property_groups,omitempty"`
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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
	GetTemporaryLinkErrorPath  = "path"
	GetTemporaryLinkErrorOther = "other"
)

// UnmarshalJSON deserializes into a GetTemporaryLinkError instance
func (u *GetTemporaryLinkError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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

// ListFolderArg : has no documentation (yet)
type ListFolderArg struct {
	// Path : A unique identifier for the file.
	Path string `json:"path"`
	// Recursive : If true, the list folder operation will be applied
	// recursively to all subfolders and the response will contain contents of
	// all subfolders.
	Recursive bool `json:"recursive"`
	// IncludeMediaInfo : If true, `FileMetadata.media_info` is set for photo
	// and video.
	IncludeMediaInfo bool `json:"include_media_info"`
	// IncludeDeleted : If true, the results will include entries for files and
	// folders that used to exist but were deleted.
	IncludeDeleted bool `json:"include_deleted"`
	// IncludeHasExplicitSharedMembers : If true, the results will include a
	// flag for each file indicating whether or not  that file has any explicit
	// members.
	IncludeHasExplicitSharedMembers bool `json:"include_has_explicit_shared_members"`
}

// NewListFolderArg returns a new ListFolderArg instance
func NewListFolderArg(Path string) *ListFolderArg {
	s := new(ListFolderArg)
	s.Path = Path
	s.Recursive = false
	s.IncludeMediaInfo = false
	s.IncludeDeleted = false
	s.IncludeHasExplicitSharedMembers = false
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// ListFolderError : has no documentation (yet)
type ListFolderError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for ListFolderError
const (
	ListFolderErrorPath  = "path"
	ListFolderErrorOther = "other"
)

// UnmarshalJSON deserializes into a ListFolderError instance
func (u *ListFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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

// ListRevisionsArg : has no documentation (yet)
type ListRevisionsArg struct {
	// Path : The path to the file you want to see the revisions of.
	Path string `json:"path"`
	// Limit : The maximum number of revision entries returned.
	Limit uint64 `json:"limit"`
}

// NewListRevisionsArg returns a new ListRevisionsArg instance
func NewListRevisionsArg(Path string) *ListRevisionsArg {
	s := new(ListRevisionsArg)
	s.Path = Path
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// ListRevisionsResult : has no documentation (yet)
type ListRevisionsResult struct {
	// IsDeleted : If the file is deleted.
	IsDeleted bool `json:"is_deleted"`
	// ServerDeleted : The time of deletion if the file was deleted.
	ServerDeleted time.Time `json:"server_deleted,omitempty"`
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

// LookUpPropertiesError : has no documentation (yet)
type LookUpPropertiesError struct {
	dropbox.Tagged
}

// Valid tag values for LookUpPropertiesError
const (
	LookUpPropertiesErrorPropertyGroupNotFound = "property_group_not_found"
)

// LookupError : has no documentation (yet)
type LookupError struct {
	dropbox.Tagged
	// MalformedPath : has no documentation (yet)
	MalformedPath string `json:"malformed_path,omitempty"`
}

// Valid tag values for LookupError
const (
	LookupErrorMalformedPath     = "malformed_path"
	LookupErrorNotFound          = "not_found"
	LookupErrorNotFile           = "not_file"
	LookupErrorNotFolder         = "not_folder"
	LookupErrorRestrictedContent = "restricted_content"
	LookupErrorOther             = "other"
)

// UnmarshalJSON deserializes into a LookupError instance
func (u *LookupError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MalformedPath : has no documentation (yet)
		MalformedPath json.RawMessage `json:"malformed_path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "malformed_path":
		err = json.Unmarshal(body, &u.MalformedPath)

		if err != nil {
			return err
		}
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
		u.Metadata, err = IsMediaMetadataFromJSON(body)

		if err != nil {
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
	TimeTaken time.Time `json:"time_taken,omitempty"`
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
		// Photo : has no documentation (yet)
		Photo json.RawMessage `json:"photo,omitempty"`
		// Video : has no documentation (yet)
		Video json.RawMessage `json:"video,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "photo":
		err = json.Unmarshal(body, &u.Photo)

		if err != nil {
			return err
		}
	case "video":
		err = json.Unmarshal(body, &u.Video)

		if err != nil {
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
	// Rev : Deprecated. Please specify revision in `path` instead.
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// PropertyGroupUpdate : has no documentation (yet)
type PropertyGroupUpdate struct {
	// TemplateId : A unique identifier for a property template.
	TemplateId string `json:"template_id"`
	// AddOrUpdateFields : List of property fields to update if the field
	// already exists. If the field doesn't exist, add the field to the property
	// group.
	AddOrUpdateFields []*properties.PropertyField `json:"add_or_update_fields,omitempty"`
	// RemoveFields : List of property field names to remove from property group
	// if the field exists.
	RemoveFields []string `json:"remove_fields,omitempty"`
}

// NewPropertyGroupUpdate returns a new PropertyGroupUpdate instance
func NewPropertyGroupUpdate(TemplateId string) *PropertyGroupUpdate {
	s := new(PropertyGroupUpdate)
	s.TemplateId = TemplateId
	return s
}

// PropertyGroupWithPath : has no documentation (yet)
type PropertyGroupWithPath struct {
	// Path : A unique identifier for the file.
	Path string `json:"path"`
	// PropertyGroups : Filled custom property templates associated with a file.
	PropertyGroups []*properties.PropertyGroup `json:"property_groups"`
}

// NewPropertyGroupWithPath returns a new PropertyGroupWithPath instance
func NewPropertyGroupWithPath(Path string, PropertyGroups []*properties.PropertyGroup) *PropertyGroupWithPath {
	s := new(PropertyGroupWithPath)
	s.Path = Path
	s.PropertyGroups = PropertyGroups
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
	// AllowSharedFolder : If true, `copy` will copy contents in shared folder,
	// otherwise `RelocationError.cant_copy_shared_folder` will be returned if
	// `from_path` contains shared folder. This field is always true for `move`.
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
	// Entries : List of entries to be moved or copied. Each entry is
	// `RelocationPath`.
	Entries []*RelocationPath `json:"entries"`
	// AllowSharedFolder : If true, `copyBatch` will copy contents in shared
	// folder, otherwise `RelocationError.cant_copy_shared_folder` will be
	// returned if `RelocationPath.from_path` contains shared folder.  This
	// field is always true for `moveBatch`.
	AllowSharedFolder bool `json:"allow_shared_folder"`
	// Autorename : If there's a conflict with any file, have the Dropbox server
	// try to autorename that file to avoid the conflict.
	Autorename bool `json:"autorename"`
	// AllowOwnershipTransfer : Allow moves by owner even if it would result in
	// an ownership transfer for the content being moved. This does not apply to
	// copies.
	AllowOwnershipTransfer bool `json:"allow_ownership_transfer"`
}

// NewRelocationBatchArg returns a new RelocationBatchArg instance
func NewRelocationBatchArg(Entries []*RelocationPath) *RelocationBatchArg {
	s := new(RelocationBatchArg)
	s.Entries = Entries
	s.AllowSharedFolder = false
	s.Autorename = false
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
	RelocationErrorOther                    = "other"
)

// UnmarshalJSON deserializes into a RelocationError instance
func (u *RelocationError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// FromLookup : has no documentation (yet)
		FromLookup json.RawMessage `json:"from_lookup,omitempty"`
		// FromWrite : has no documentation (yet)
		FromWrite json.RawMessage `json:"from_write,omitempty"`
		// To : has no documentation (yet)
		To json.RawMessage `json:"to,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "from_lookup":
		err = json.Unmarshal(w.FromLookup, &u.FromLookup)

		if err != nil {
			return err
		}
	case "from_write":
		err = json.Unmarshal(w.FromWrite, &u.FromWrite)

		if err != nil {
			return err
		}
	case "to":
		err = json.Unmarshal(w.To, &u.To)

		if err != nil {
			return err
		}
	}
	return nil
}

// RelocationBatchError : has no documentation (yet)
type RelocationBatchError struct {
	dropbox.Tagged
}

// Valid tag values for RelocationBatchError
const (
	RelocationBatchErrorTooManyWriteOperations = "too_many_write_operations"
)

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
	RelocationBatchJobStatusComplete = "complete"
	RelocationBatchJobStatusFailed   = "failed"
)

// UnmarshalJSON deserializes into a RelocationBatchJobStatus instance
func (u *RelocationBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : The copy or move batch job has finished.
		Complete json.RawMessage `json:"complete,omitempty"`
		// Failed : The copy or move batch job has failed with exception.
		Failed json.RawMessage `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
			return err
		}
	case "failed":
		err = json.Unmarshal(w.Failed, &u.Failed)

		if err != nil {
			return err
		}
	}
	return nil
}

// RelocationBatchLaunch : Result returned by `copyBatch` or `moveBatch` that
// may either launch an asynchronous job or complete synchronously.
type RelocationBatchLaunch struct {
	dropbox.Tagged
	// Complete : has no documentation (yet)
	Complete *RelocationBatchResult `json:"complete,omitempty"`
}

// Valid tag values for RelocationBatchLaunch
const (
	RelocationBatchLaunchComplete = "complete"
	RelocationBatchLaunchOther    = "other"
)

// UnmarshalJSON deserializes into a RelocationBatchLaunch instance
func (u *RelocationBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : has no documentation (yet)
		Complete json.RawMessage `json:"complete,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
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

// RemovePropertiesArg : has no documentation (yet)
type RemovePropertiesArg struct {
	// Path : A unique identifier for the file.
	Path string `json:"path"`
	// PropertyTemplateIds : A list of identifiers for a property template
	// created by route properties/template/add.
	PropertyTemplateIds []string `json:"property_template_ids"`
}

// NewRemovePropertiesArg returns a new RemovePropertiesArg instance
func NewRemovePropertiesArg(Path string, PropertyTemplateIds []string) *RemovePropertiesArg {
	s := new(RemovePropertiesArg)
	s.Path = Path
	s.PropertyTemplateIds = PropertyTemplateIds
	return s
}

// RemovePropertiesError : has no documentation (yet)
type RemovePropertiesError struct {
	dropbox.Tagged
	// PropertyGroupLookup : has no documentation (yet)
	PropertyGroupLookup *LookUpPropertiesError `json:"property_group_lookup,omitempty"`
}

// Valid tag values for RemovePropertiesError
const (
	RemovePropertiesErrorPropertyGroupLookup = "property_group_lookup"
)

// UnmarshalJSON deserializes into a RemovePropertiesError instance
func (u *RemovePropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PropertyGroupLookup : has no documentation (yet)
		PropertyGroupLookup json.RawMessage `json:"property_group_lookup,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "property_group_lookup":
		err = json.Unmarshal(w.PropertyGroupLookup, &u.PropertyGroupLookup)

		if err != nil {
			return err
		}
	}
	return nil
}

// RestoreArg : has no documentation (yet)
type RestoreArg struct {
	// Path : The path to the file you want to restore.
	Path string `json:"path"`
	// Rev : The revision to restore for the file.
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
	RestoreErrorOther           = "other"
)

// UnmarshalJSON deserializes into a RestoreError instance
func (u *RestoreError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PathLookup : An error occurs when downloading metadata for the file.
		PathLookup json.RawMessage `json:"path_lookup,omitempty"`
		// PathWrite : An error occurs when trying to restore the file to that
		// path.
		PathWrite json.RawMessage `json:"path_write,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path_lookup":
		err = json.Unmarshal(w.PathLookup, &u.PathLookup)

		if err != nil {
			return err
		}
	case "path_write":
		err = json.Unmarshal(w.PathWrite, &u.PathWrite)

		if err != nil {
			return err
		}
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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
	SaveUrlJobStatusComplete = "complete"
	SaveUrlJobStatusFailed   = "failed"
)

// UnmarshalJSON deserializes into a SaveUrlJobStatus instance
func (u *SaveUrlJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : Metadata of the file where the URL is saved to.
		Complete json.RawMessage `json:"complete,omitempty"`
		// Failed : has no documentation (yet)
		Failed json.RawMessage `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
			return err
		}
	case "failed":
		err = json.Unmarshal(w.Failed, &u.Failed)

		if err != nil {
			return err
		}
	}
	return nil
}

// SaveUrlResult : has no documentation (yet)
type SaveUrlResult struct {
	dropbox.Tagged
	// Complete : Metadata of the file where the URL is saved to.
	Complete *FileMetadata `json:"complete,omitempty"`
}

// Valid tag values for SaveUrlResult
const (
	SaveUrlResultComplete = "complete"
)

// UnmarshalJSON deserializes into a SaveUrlResult instance
func (u *SaveUrlResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : Metadata of the file where the URL is saved to.
		Complete json.RawMessage `json:"complete,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
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
	// Query : The string to search for. The search string is split on spaces
	// into multiple tokens. For file name searching, the last token is used for
	// prefix matching (i.e. "bat c" matches "bat cave" but not "batman car").
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
	s.Mode = &SearchMode{Tagged: dropbox.Tagged{"filename"}}
	return s
}

// SearchError : has no documentation (yet)
type SearchError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *LookupError `json:"path,omitempty"`
}

// Valid tag values for SearchError
const (
	SearchErrorPath  = "path"
	SearchErrorOther = "other"
)

// UnmarshalJSON deserializes into a SearchError instance
func (u *SearchError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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
}

// NewThumbnailArg returns a new ThumbnailArg instance
func NewThumbnailArg(Path string) *ThumbnailArg {
	s := new(ThumbnailArg)
	s.Path = Path
	s.Format = &ThumbnailFormat{Tagged: dropbox.Tagged{"jpeg"}}
	s.Size = &ThumbnailSize{Tagged: dropbox.Tagged{"w64h64"}}
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
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
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

// ThumbnailSize : has no documentation (yet)
type ThumbnailSize struct {
	dropbox.Tagged
}

// Valid tag values for ThumbnailSize
const (
	ThumbnailSizeW32h32    = "w32h32"
	ThumbnailSizeW64h64    = "w64h64"
	ThumbnailSizeW128h128  = "w128h128"
	ThumbnailSizeW640h480  = "w640h480"
	ThumbnailSizeW1024h768 = "w1024h768"
)

// UpdatePropertiesError : has no documentation (yet)
type UpdatePropertiesError struct {
	dropbox.Tagged
	// PropertyGroupLookup : has no documentation (yet)
	PropertyGroupLookup *LookUpPropertiesError `json:"property_group_lookup,omitempty"`
}

// Valid tag values for UpdatePropertiesError
const (
	UpdatePropertiesErrorPropertyGroupLookup = "property_group_lookup"
)

// UnmarshalJSON deserializes into a UpdatePropertiesError instance
func (u *UpdatePropertiesError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PropertyGroupLookup : has no documentation (yet)
		PropertyGroupLookup json.RawMessage `json:"property_group_lookup,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "property_group_lookup":
		err = json.Unmarshal(w.PropertyGroupLookup, &u.PropertyGroupLookup)

		if err != nil {
			return err
		}
	}
	return nil
}

// UpdatePropertyGroupArg : has no documentation (yet)
type UpdatePropertyGroupArg struct {
	// Path : A unique identifier for the file.
	Path string `json:"path"`
	// UpdatePropertyGroups : Filled custom property templates associated with a
	// file.
	UpdatePropertyGroups []*PropertyGroupUpdate `json:"update_property_groups"`
}

// NewUpdatePropertyGroupArg returns a new UpdatePropertyGroupArg instance
func NewUpdatePropertyGroupArg(Path string, UpdatePropertyGroups []*PropertyGroupUpdate) *UpdatePropertyGroupArg {
	s := new(UpdatePropertyGroupArg)
	s.Path = Path
	s.UpdatePropertyGroups = UpdatePropertyGroups
	return s
}

// UploadError : has no documentation (yet)
type UploadError struct {
	dropbox.Tagged
	// Path : Unable to save the uploaded contents to a file.
	Path *UploadWriteFailed `json:"path,omitempty"`
}

// Valid tag values for UploadError
const (
	UploadErrorPath  = "path"
	UploadErrorOther = "other"
)

// UnmarshalJSON deserializes into a UploadError instance
func (u *UploadError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : Unable to save the uploaded contents to a file.
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "path":
		err = json.Unmarshal(body, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// UploadErrorWithProperties : has no documentation (yet)
type UploadErrorWithProperties struct {
	dropbox.Tagged
	// PropertiesError : has no documentation (yet)
	PropertiesError *InvalidPropertyGroupError `json:"properties_error,omitempty"`
}

// Valid tag values for UploadErrorWithProperties
const (
	UploadErrorWithPropertiesPropertiesError = "properties_error"
)

// UnmarshalJSON deserializes into a UploadErrorWithProperties instance
func (u *UploadErrorWithProperties) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// PropertiesError : has no documentation (yet)
		PropertiesError json.RawMessage `json:"properties_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "properties_error":
		err = json.Unmarshal(w.PropertiesError, &u.PropertiesError)

		if err != nil {
			return err
		}
	}
	return nil
}

// UploadSessionAppendArg : has no documentation (yet)
type UploadSessionAppendArg struct {
	// Cursor : Contains the upload session ID and the offset.
	Cursor *UploadSessionCursor `json:"cursor"`
	// Close : If true, the current session will be closed, at which point you
	// won't be able to call `uploadSessionAppendV2` anymore with the current
	// session.
	Close bool `json:"close"`
}

// NewUploadSessionAppendArg returns a new UploadSessionAppendArg instance
func NewUploadSessionAppendArg(Cursor *UploadSessionCursor) *UploadSessionAppendArg {
	s := new(UploadSessionAppendArg)
	s.Cursor = Cursor
	s.Close = false
	return s
}

// UploadSessionCursor : has no documentation (yet)
type UploadSessionCursor struct {
	// SessionId : The upload session ID (returned by `uploadSessionStart`).
	SessionId string `json:"session_id"`
	// Offset : The amount of data that has been uploaded so far. We use this to
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
	UploadSessionFinishBatchJobStatusComplete = "complete"
)

// UnmarshalJSON deserializes into a UploadSessionFinishBatchJobStatus instance
func (u *UploadSessionFinishBatchJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : The `uploadSessionFinishBatch` has finished.
		Complete json.RawMessage `json:"complete,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
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
	// Complete : has no documentation (yet)
	Complete *UploadSessionFinishBatchResult `json:"complete,omitempty"`
}

// Valid tag values for UploadSessionFinishBatchLaunch
const (
	UploadSessionFinishBatchLaunchComplete = "complete"
	UploadSessionFinishBatchLaunchOther    = "other"
)

// UnmarshalJSON deserializes into a UploadSessionFinishBatchLaunch instance
func (u *UploadSessionFinishBatchLaunch) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Complete : has no documentation (yet)
		Complete json.RawMessage `json:"complete,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
			return err
		}
	}
	return nil
}

// UploadSessionFinishBatchResult : has no documentation (yet)
type UploadSessionFinishBatchResult struct {
	// Entries : Commit result for each file in the batch.
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
		// Success : has no documentation (yet)
		Success json.RawMessage `json:"success,omitempty"`
		// Failure : has no documentation (yet)
		Failure json.RawMessage `json:"failure,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		err = json.Unmarshal(body, &u.Success)

		if err != nil {
			return err
		}
	case "failure":
		err = json.Unmarshal(w.Failure, &u.Failure)

		if err != nil {
			return err
		}
	}
	return nil
}

// UploadSessionFinishError : has no documentation (yet)
type UploadSessionFinishError struct {
	dropbox.Tagged
	// LookupFailed : The session arguments are incorrect; the value explains
	// the reason.
	LookupFailed *UploadSessionLookupError `json:"lookup_failed,omitempty"`
	// Path : Unable to save the uploaded contents to a file.
	Path *WriteError `json:"path,omitempty"`
}

// Valid tag values for UploadSessionFinishError
const (
	UploadSessionFinishErrorLookupFailed               = "lookup_failed"
	UploadSessionFinishErrorPath                       = "path"
	UploadSessionFinishErrorTooManySharedFolderTargets = "too_many_shared_folder_targets"
	UploadSessionFinishErrorTooManyWriteOperations     = "too_many_write_operations"
	UploadSessionFinishErrorOther                      = "other"
)

// UnmarshalJSON deserializes into a UploadSessionFinishError instance
func (u *UploadSessionFinishError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// LookupFailed : The session arguments are incorrect; the value
		// explains the reason.
		LookupFailed json.RawMessage `json:"lookup_failed,omitempty"`
		// Path : Unable to save the uploaded contents to a file.
		Path json.RawMessage `json:"path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "lookup_failed":
		err = json.Unmarshal(w.LookupFailed, &u.LookupFailed)

		if err != nil {
			return err
		}
	case "path":
		err = json.Unmarshal(w.Path, &u.Path)

		if err != nil {
			return err
		}
	}
	return nil
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
	UploadSessionLookupErrorNotFound        = "not_found"
	UploadSessionLookupErrorIncorrectOffset = "incorrect_offset"
	UploadSessionLookupErrorClosed          = "closed"
	UploadSessionLookupErrorNotClosed       = "not_closed"
	UploadSessionLookupErrorOther           = "other"
)

// UnmarshalJSON deserializes into a UploadSessionLookupError instance
func (u *UploadSessionLookupError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// IncorrectOffset : The specified offset was incorrect. See the value
		// for the correct offset. This error may occur when a previous request
		// was received and processed successfully but the client did not
		// receive the response, e.g. due to a network error.
		IncorrectOffset json.RawMessage `json:"incorrect_offset,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "incorrect_offset":
		err = json.Unmarshal(body, &u.IncorrectOffset)

		if err != nil {
			return err
		}
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
	// won't be able to call `uploadSessionAppendV2` anymore with the current
	// session.
	Close bool `json:"close"`
}

// NewUploadSessionStartArg returns a new UploadSessionStartArg instance
func NewUploadSessionStartArg() *UploadSessionStartArg {
	s := new(UploadSessionStartArg)
	s.Close = false
	return s
}

// UploadSessionStartResult : has no documentation (yet)
type UploadSessionStartResult struct {
	// SessionId : A unique identifier for the upload session. Pass this to
	// `uploadSessionAppendV2` and `uploadSessionFinish`.
	SessionId string `json:"session_id"`
}

// NewUploadSessionStartResult returns a new UploadSessionStartResult instance
func NewUploadSessionStartResult(SessionId string) *UploadSessionStartResult {
	s := new(UploadSessionStartResult)
	s.SessionId = SessionId
	return s
}

// UploadWriteFailed : has no documentation (yet)
type UploadWriteFailed struct {
	// Reason : The reason why the file couldn't be saved.
	Reason *WriteError `json:"reason"`
	// UploadSessionId : The upload session ID; this may be used to retry the
	// commit.
	UploadSessionId string `json:"upload_session_id"`
}

// NewUploadWriteFailed returns a new UploadWriteFailed instance
func NewUploadWriteFailed(Reason *WriteError, UploadSessionId string) *UploadWriteFailed {
	s := new(UploadWriteFailed)
	s.Reason = Reason
	s.UploadSessionId = UploadSessionId
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
	// MalformedPath : has no documentation (yet)
	MalformedPath string `json:"malformed_path,omitempty"`
	// Conflict : Couldn't write to the target path because there was something
	// in the way.
	Conflict *WriteConflictError `json:"conflict,omitempty"`
}

// Valid tag values for WriteError
const (
	WriteErrorMalformedPath     = "malformed_path"
	WriteErrorConflict          = "conflict"
	WriteErrorNoWritePermission = "no_write_permission"
	WriteErrorInsufficientSpace = "insufficient_space"
	WriteErrorDisallowedName    = "disallowed_name"
	WriteErrorTeamFolder        = "team_folder"
	WriteErrorOther             = "other"
)

// UnmarshalJSON deserializes into a WriteError instance
func (u *WriteError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MalformedPath : has no documentation (yet)
		MalformedPath json.RawMessage `json:"malformed_path,omitempty"`
		// Conflict : Couldn't write to the target path because there was
		// something in the way.
		Conflict json.RawMessage `json:"conflict,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "malformed_path":
		err = json.Unmarshal(body, &u.MalformedPath)

		if err != nil {
			return err
		}
	case "conflict":
		err = json.Unmarshal(w.Conflict, &u.Conflict)

		if err != nil {
			return err
		}
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
	// The autorename strategy is to append the string "conflicted copy" to the
	// file name. For example, "document.txt" might become "document (conflicted
	// copy).txt" or "document (Panda's conflicted copy).txt".
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
