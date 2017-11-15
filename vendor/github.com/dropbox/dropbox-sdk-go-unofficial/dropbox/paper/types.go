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

// Package paper : This namespace contains endpoints and data types for managing
// docs and folders in Dropbox Paper.
package paper

import (
	"encoding/json"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/sharing"
)

// AddMember : has no documentation (yet)
type AddMember struct {
	// PermissionLevel : Permission for the user.
	PermissionLevel *PaperDocPermissionLevel `json:"permission_level"`
	// Member : User which should be added to the Paper doc. Specify only email
	// address or Dropbox account ID.
	Member *sharing.MemberSelector `json:"member"`
}

// NewAddMember returns a new AddMember instance
func NewAddMember(Member *sharing.MemberSelector) *AddMember {
	s := new(AddMember)
	s.Member = Member
	s.PermissionLevel = &PaperDocPermissionLevel{Tagged: dropbox.Tagged{"edit"}}
	return s
}

// RefPaperDoc : has no documentation (yet)
type RefPaperDoc struct {
	// DocId : The Paper doc ID.
	DocId string `json:"doc_id"`
}

// NewRefPaperDoc returns a new RefPaperDoc instance
func NewRefPaperDoc(DocId string) *RefPaperDoc {
	s := new(RefPaperDoc)
	s.DocId = DocId
	return s
}

// AddPaperDocUser : has no documentation (yet)
type AddPaperDocUser struct {
	RefPaperDoc
	// Members : User which should be added to the Paper doc. Specify only email
	// address or Dropbox account ID.
	Members []*AddMember `json:"members"`
	// CustomMessage : A personal message that will be emailed to each
	// successfully added member.
	CustomMessage string `json:"custom_message,omitempty"`
	// Quiet : Clients should set this to true if no email message shall be sent
	// to added users.
	Quiet bool `json:"quiet"`
}

// NewAddPaperDocUser returns a new AddPaperDocUser instance
func NewAddPaperDocUser(DocId string, Members []*AddMember) *AddPaperDocUser {
	s := new(AddPaperDocUser)
	s.DocId = DocId
	s.Members = Members
	s.Quiet = false
	return s
}

// AddPaperDocUserMemberResult : Per-member result for `docsUsersAdd`.
type AddPaperDocUserMemberResult struct {
	// Member : One of specified input members.
	Member *sharing.MemberSelector `json:"member"`
	// Result : The outcome of the action on this member.
	Result *AddPaperDocUserResult `json:"result"`
}

// NewAddPaperDocUserMemberResult returns a new AddPaperDocUserMemberResult instance
func NewAddPaperDocUserMemberResult(Member *sharing.MemberSelector, Result *AddPaperDocUserResult) *AddPaperDocUserMemberResult {
	s := new(AddPaperDocUserMemberResult)
	s.Member = Member
	s.Result = Result
	return s
}

// AddPaperDocUserResult : has no documentation (yet)
type AddPaperDocUserResult struct {
	dropbox.Tagged
}

// Valid tag values for AddPaperDocUserResult
const (
	AddPaperDocUserResultSuccess                    = "success"
	AddPaperDocUserResultUnknownError               = "unknown_error"
	AddPaperDocUserResultSharingOutsideTeamDisabled = "sharing_outside_team_disabled"
	AddPaperDocUserResultDailyLimitReached          = "daily_limit_reached"
	AddPaperDocUserResultUserIsOwner                = "user_is_owner"
	AddPaperDocUserResultFailedUserDataRetrieval    = "failed_user_data_retrieval"
	AddPaperDocUserResultPermissionAlreadyGranted   = "permission_already_granted"
	AddPaperDocUserResultOther                      = "other"
)

// Cursor : has no documentation (yet)
type Cursor struct {
	// Value : The actual cursor value.
	Value string `json:"value"`
	// Expiration : Expiration time of `value`. Some cursors might have
	// expiration time assigned. This is a UTC value after which the cursor is
	// no longer valid and the API starts returning an error. If cursor expires
	// a new one needs to be obtained and pagination needs to be restarted. Some
	// cursors might be short-lived some cursors might be long-lived. This
	// really depends on the sorting type and order, e.g.: 1. on one hand,
	// listing docs created by the user, sorted by the created time ascending
	// will have undefinite expiration because the results cannot change while
	// the iteration is happening. This cursor would be suitable for long term
	// polling. 2. on the other hand, listing docs sorted by the last modified
	// time will have a very short expiration as docs do get modified very often
	// and the modified time can be changed while the iteration is happening
	// thus altering the results.
	Expiration time.Time `json:"expiration,omitempty"`
}

// NewCursor returns a new Cursor instance
func NewCursor(Value string) *Cursor {
	s := new(Cursor)
	s.Value = Value
	return s
}

// PaperApiBaseError : has no documentation (yet)
type PaperApiBaseError struct {
	dropbox.Tagged
}

// Valid tag values for PaperApiBaseError
const (
	PaperApiBaseErrorInsufficientPermissions = "insufficient_permissions"
	PaperApiBaseErrorOther                   = "other"
)

// DocLookupError : has no documentation (yet)
type DocLookupError struct {
	dropbox.Tagged
}

// Valid tag values for DocLookupError
const (
	DocLookupErrorInsufficientPermissions = "insufficient_permissions"
	DocLookupErrorOther                   = "other"
	DocLookupErrorDocNotFound             = "doc_not_found"
)

// DocSubscriptionLevel : The subscription level of a Paper doc.
type DocSubscriptionLevel struct {
	dropbox.Tagged
}

// Valid tag values for DocSubscriptionLevel
const (
	DocSubscriptionLevelDefault = "default"
	DocSubscriptionLevelIgnore  = "ignore"
	DocSubscriptionLevelEvery   = "every"
	DocSubscriptionLevelNoEmail = "no_email"
)

// ExportFormat : The desired export format of the Paper doc.
type ExportFormat struct {
	dropbox.Tagged
}

// Valid tag values for ExportFormat
const (
	ExportFormatHtml     = "html"
	ExportFormatMarkdown = "markdown"
	ExportFormatOther    = "other"
)

// Folder : Data structure representing a Paper folder.
type Folder struct {
	// Id : Paper folder ID. This ID uniquely identifies the folder.
	Id string `json:"id"`
	// Name : Paper folder name.
	Name string `json:"name"`
}

// NewFolder returns a new Folder instance
func NewFolder(Id string, Name string) *Folder {
	s := new(Folder)
	s.Id = Id
	s.Name = Name
	return s
}

// FolderSharingPolicyType : The sharing policy of a Paper folder.  Note: The
// sharing policy of subfolders is inherited from the root folder.
type FolderSharingPolicyType struct {
	dropbox.Tagged
}

// Valid tag values for FolderSharingPolicyType
const (
	FolderSharingPolicyTypeTeam       = "team"
	FolderSharingPolicyTypeInviteOnly = "invite_only"
)

// FolderSubscriptionLevel : The subscription level of a Paper folder.
type FolderSubscriptionLevel struct {
	dropbox.Tagged
}

// Valid tag values for FolderSubscriptionLevel
const (
	FolderSubscriptionLevelNone         = "none"
	FolderSubscriptionLevelActivityOnly = "activity_only"
	FolderSubscriptionLevelDailyEmails  = "daily_emails"
	FolderSubscriptionLevelWeeklyEmails = "weekly_emails"
)

// FoldersContainingPaperDoc : Metadata about Paper folders containing the
// specififed Paper doc.
type FoldersContainingPaperDoc struct {
	// FolderSharingPolicyType : The sharing policy of the folder containing the
	// Paper doc.
	FolderSharingPolicyType *FolderSharingPolicyType `json:"folder_sharing_policy_type,omitempty"`
	// Folders : The folder path. If present the first folder is the root
	// folder.
	Folders []*Folder `json:"folders,omitempty"`
}

// NewFoldersContainingPaperDoc returns a new FoldersContainingPaperDoc instance
func NewFoldersContainingPaperDoc() *FoldersContainingPaperDoc {
	s := new(FoldersContainingPaperDoc)
	return s
}

// ImportFormat : The import format of the incoming data.
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

// InviteeInfoWithPermissionLevel : has no documentation (yet)
type InviteeInfoWithPermissionLevel struct {
	// Invitee : Email address invited to the Paper doc.
	Invitee *sharing.InviteeInfo `json:"invitee"`
	// PermissionLevel : Permission level for the invitee.
	PermissionLevel *PaperDocPermissionLevel `json:"permission_level"`
}

// NewInviteeInfoWithPermissionLevel returns a new InviteeInfoWithPermissionLevel instance
func NewInviteeInfoWithPermissionLevel(Invitee *sharing.InviteeInfo, PermissionLevel *PaperDocPermissionLevel) *InviteeInfoWithPermissionLevel {
	s := new(InviteeInfoWithPermissionLevel)
	s.Invitee = Invitee
	s.PermissionLevel = PermissionLevel
	return s
}

// ListDocsCursorError : has no documentation (yet)
type ListDocsCursorError struct {
	dropbox.Tagged
	// CursorError : has no documentation (yet)
	CursorError *PaperApiCursorError `json:"cursor_error,omitempty"`
}

// Valid tag values for ListDocsCursorError
const (
	ListDocsCursorErrorCursorError = "cursor_error"
	ListDocsCursorErrorOther       = "other"
)

// UnmarshalJSON deserializes into a ListDocsCursorError instance
func (u *ListDocsCursorError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// CursorError : has no documentation (yet)
		CursorError json.RawMessage `json:"cursor_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "cursor_error":
		err = json.Unmarshal(w.CursorError, &u.CursorError)

		if err != nil {
			return err
		}
	}
	return nil
}

// ListPaperDocsArgs : has no documentation (yet)
type ListPaperDocsArgs struct {
	// FilterBy : Allows user to specify how the Paper docs should be filtered.
	FilterBy *ListPaperDocsFilterBy `json:"filter_by"`
	// SortBy : Allows user to specify how the Paper docs should be sorted.
	SortBy *ListPaperDocsSortBy `json:"sort_by"`
	// SortOrder : Allows user to specify the sort order of the result.
	SortOrder *ListPaperDocsSortOrder `json:"sort_order"`
	// Limit : Size limit per batch. The maximum number of docs that can be
	// retrieved per batch is 1000. Higher value results in invalid arguments
	// error.
	Limit int32 `json:"limit"`
}

// NewListPaperDocsArgs returns a new ListPaperDocsArgs instance
func NewListPaperDocsArgs() *ListPaperDocsArgs {
	s := new(ListPaperDocsArgs)
	s.FilterBy = &ListPaperDocsFilterBy{Tagged: dropbox.Tagged{"docs_accessed"}}
	s.SortBy = &ListPaperDocsSortBy{Tagged: dropbox.Tagged{"accessed"}}
	s.SortOrder = &ListPaperDocsSortOrder{Tagged: dropbox.Tagged{"ascending"}}
	s.Limit = 1000
	return s
}

// ListPaperDocsContinueArgs : has no documentation (yet)
type ListPaperDocsContinueArgs struct {
	// Cursor : The cursor obtained from `docsList` or `docsListContinue`.
	// Allows for pagination.
	Cursor string `json:"cursor"`
}

// NewListPaperDocsContinueArgs returns a new ListPaperDocsContinueArgs instance
func NewListPaperDocsContinueArgs(Cursor string) *ListPaperDocsContinueArgs {
	s := new(ListPaperDocsContinueArgs)
	s.Cursor = Cursor
	return s
}

// ListPaperDocsFilterBy : has no documentation (yet)
type ListPaperDocsFilterBy struct {
	dropbox.Tagged
}

// Valid tag values for ListPaperDocsFilterBy
const (
	ListPaperDocsFilterByDocsAccessed = "docs_accessed"
	ListPaperDocsFilterByDocsCreated  = "docs_created"
	ListPaperDocsFilterByOther        = "other"
)

// ListPaperDocsResponse : has no documentation (yet)
type ListPaperDocsResponse struct {
	// DocIds : The list of Paper doc IDs that can be used to access the given
	// Paper docs or supplied to other API methods. The list is sorted in the
	// order specified by the initial call to `docsList`.
	DocIds []string `json:"doc_ids"`
	// Cursor : Pass the cursor into `docsListContinue` to paginate through all
	// files. The cursor preserves all properties as specified in the original
	// call to `docsList`.
	Cursor *Cursor `json:"cursor"`
	// HasMore : Will be set to True if a subsequent call with the provided
	// cursor to `docsListContinue` returns immediately with some results. If
	// set to False please allow some delay before making another call to
	// `docsListContinue`.
	HasMore bool `json:"has_more"`
}

// NewListPaperDocsResponse returns a new ListPaperDocsResponse instance
func NewListPaperDocsResponse(DocIds []string, Cursor *Cursor, HasMore bool) *ListPaperDocsResponse {
	s := new(ListPaperDocsResponse)
	s.DocIds = DocIds
	s.Cursor = Cursor
	s.HasMore = HasMore
	return s
}

// ListPaperDocsSortBy : has no documentation (yet)
type ListPaperDocsSortBy struct {
	dropbox.Tagged
}

// Valid tag values for ListPaperDocsSortBy
const (
	ListPaperDocsSortByAccessed = "accessed"
	ListPaperDocsSortByModified = "modified"
	ListPaperDocsSortByCreated  = "created"
	ListPaperDocsSortByOther    = "other"
)

// ListPaperDocsSortOrder : has no documentation (yet)
type ListPaperDocsSortOrder struct {
	dropbox.Tagged
}

// Valid tag values for ListPaperDocsSortOrder
const (
	ListPaperDocsSortOrderAscending  = "ascending"
	ListPaperDocsSortOrderDescending = "descending"
	ListPaperDocsSortOrderOther      = "other"
)

// ListUsersCursorError : has no documentation (yet)
type ListUsersCursorError struct {
	dropbox.Tagged
	// CursorError : has no documentation (yet)
	CursorError *PaperApiCursorError `json:"cursor_error,omitempty"`
}

// Valid tag values for ListUsersCursorError
const (
	ListUsersCursorErrorInsufficientPermissions = "insufficient_permissions"
	ListUsersCursorErrorOther                   = "other"
	ListUsersCursorErrorDocNotFound             = "doc_not_found"
	ListUsersCursorErrorCursorError             = "cursor_error"
)

// UnmarshalJSON deserializes into a ListUsersCursorError instance
func (u *ListUsersCursorError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// CursorError : has no documentation (yet)
		CursorError json.RawMessage `json:"cursor_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "cursor_error":
		err = json.Unmarshal(w.CursorError, &u.CursorError)

		if err != nil {
			return err
		}
	}
	return nil
}

// ListUsersOnFolderArgs : has no documentation (yet)
type ListUsersOnFolderArgs struct {
	RefPaperDoc
	// Limit : Size limit per batch. The maximum number of users that can be
	// retrieved per batch is 1000. Higher value results in invalid arguments
	// error.
	Limit int32 `json:"limit"`
}

// NewListUsersOnFolderArgs returns a new ListUsersOnFolderArgs instance
func NewListUsersOnFolderArgs(DocId string) *ListUsersOnFolderArgs {
	s := new(ListUsersOnFolderArgs)
	s.DocId = DocId
	s.Limit = 1000
	return s
}

// ListUsersOnFolderContinueArgs : has no documentation (yet)
type ListUsersOnFolderContinueArgs struct {
	RefPaperDoc
	// Cursor : The cursor obtained from `docsFolderUsersList` or
	// `docsFolderUsersListContinue`. Allows for pagination.
	Cursor string `json:"cursor"`
}

// NewListUsersOnFolderContinueArgs returns a new ListUsersOnFolderContinueArgs instance
func NewListUsersOnFolderContinueArgs(DocId string, Cursor string) *ListUsersOnFolderContinueArgs {
	s := new(ListUsersOnFolderContinueArgs)
	s.DocId = DocId
	s.Cursor = Cursor
	return s
}

// ListUsersOnFolderResponse : has no documentation (yet)
type ListUsersOnFolderResponse struct {
	// Invitees : List of email addresses that are invited on the Paper folder.
	Invitees []*sharing.InviteeInfo `json:"invitees"`
	// Users : List of users that are invited on the Paper folder.
	Users []*sharing.UserInfo `json:"users"`
	// Cursor : Pass the cursor into `docsFolderUsersListContinue` to paginate
	// through all users. The cursor preserves all properties as specified in
	// the original call to `docsFolderUsersList`.
	Cursor *Cursor `json:"cursor"`
	// HasMore : Will be set to True if a subsequent call with the provided
	// cursor to `docsFolderUsersListContinue` returns immediately with some
	// results. If set to False please allow some delay before making another
	// call to `docsFolderUsersListContinue`.
	HasMore bool `json:"has_more"`
}

// NewListUsersOnFolderResponse returns a new ListUsersOnFolderResponse instance
func NewListUsersOnFolderResponse(Invitees []*sharing.InviteeInfo, Users []*sharing.UserInfo, Cursor *Cursor, HasMore bool) *ListUsersOnFolderResponse {
	s := new(ListUsersOnFolderResponse)
	s.Invitees = Invitees
	s.Users = Users
	s.Cursor = Cursor
	s.HasMore = HasMore
	return s
}

// ListUsersOnPaperDocArgs : has no documentation (yet)
type ListUsersOnPaperDocArgs struct {
	RefPaperDoc
	// Limit : Size limit per batch. The maximum number of users that can be
	// retrieved per batch is 1000. Higher value results in invalid arguments
	// error.
	Limit int32 `json:"limit"`
	// FilterBy : Specify this attribute if you want to obtain users that have
	// already accessed the Paper doc.
	FilterBy *UserOnPaperDocFilter `json:"filter_by"`
}

// NewListUsersOnPaperDocArgs returns a new ListUsersOnPaperDocArgs instance
func NewListUsersOnPaperDocArgs(DocId string) *ListUsersOnPaperDocArgs {
	s := new(ListUsersOnPaperDocArgs)
	s.DocId = DocId
	s.Limit = 1000
	s.FilterBy = &UserOnPaperDocFilter{Tagged: dropbox.Tagged{"shared"}}
	return s
}

// ListUsersOnPaperDocContinueArgs : has no documentation (yet)
type ListUsersOnPaperDocContinueArgs struct {
	RefPaperDoc
	// Cursor : The cursor obtained from `docsUsersList` or
	// `docsUsersListContinue`. Allows for pagination.
	Cursor string `json:"cursor"`
}

// NewListUsersOnPaperDocContinueArgs returns a new ListUsersOnPaperDocContinueArgs instance
func NewListUsersOnPaperDocContinueArgs(DocId string, Cursor string) *ListUsersOnPaperDocContinueArgs {
	s := new(ListUsersOnPaperDocContinueArgs)
	s.DocId = DocId
	s.Cursor = Cursor
	return s
}

// ListUsersOnPaperDocResponse : has no documentation (yet)
type ListUsersOnPaperDocResponse struct {
	// Invitees : List of email addresses with their respective permission
	// levels that are invited on the Paper doc.
	Invitees []*InviteeInfoWithPermissionLevel `json:"invitees"`
	// Users : List of users with their respective permission levels that are
	// invited on the Paper folder.
	Users []*UserInfoWithPermissionLevel `json:"users"`
	// DocOwner : The Paper doc owner. This field is populated on every single
	// response.
	DocOwner *sharing.UserInfo `json:"doc_owner"`
	// Cursor : Pass the cursor into `docsUsersListContinue` to paginate through
	// all users. The cursor preserves all properties as specified in the
	// original call to `docsUsersList`.
	Cursor *Cursor `json:"cursor"`
	// HasMore : Will be set to True if a subsequent call with the provided
	// cursor to `docsUsersListContinue` returns immediately with some results.
	// If set to False please allow some delay before making another call to
	// `docsUsersListContinue`.
	HasMore bool `json:"has_more"`
}

// NewListUsersOnPaperDocResponse returns a new ListUsersOnPaperDocResponse instance
func NewListUsersOnPaperDocResponse(Invitees []*InviteeInfoWithPermissionLevel, Users []*UserInfoWithPermissionLevel, DocOwner *sharing.UserInfo, Cursor *Cursor, HasMore bool) *ListUsersOnPaperDocResponse {
	s := new(ListUsersOnPaperDocResponse)
	s.Invitees = Invitees
	s.Users = Users
	s.DocOwner = DocOwner
	s.Cursor = Cursor
	s.HasMore = HasMore
	return s
}

// PaperApiCursorError : has no documentation (yet)
type PaperApiCursorError struct {
	dropbox.Tagged
}

// Valid tag values for PaperApiCursorError
const (
	PaperApiCursorErrorExpiredCursor     = "expired_cursor"
	PaperApiCursorErrorInvalidCursor     = "invalid_cursor"
	PaperApiCursorErrorWrongUserInCursor = "wrong_user_in_cursor"
	PaperApiCursorErrorReset             = "reset"
	PaperApiCursorErrorOther             = "other"
)

// PaperDocCreateArgs : has no documentation (yet)
type PaperDocCreateArgs struct {
	// ParentFolderId : The Paper folder ID where the Paper document should be
	// created. The API user has to have write access to this folder or error is
	// thrown.
	ParentFolderId string `json:"parent_folder_id,omitempty"`
	// ImportFormat : The format of provided data.
	ImportFormat *ImportFormat `json:"import_format"`
}

// NewPaperDocCreateArgs returns a new PaperDocCreateArgs instance
func NewPaperDocCreateArgs(ImportFormat *ImportFormat) *PaperDocCreateArgs {
	s := new(PaperDocCreateArgs)
	s.ImportFormat = ImportFormat
	return s
}

// PaperDocCreateError : has no documentation (yet)
type PaperDocCreateError struct {
	dropbox.Tagged
}

// Valid tag values for PaperDocCreateError
const (
	PaperDocCreateErrorInsufficientPermissions = "insufficient_permissions"
	PaperDocCreateErrorOther                   = "other"
	PaperDocCreateErrorContentMalformed        = "content_malformed"
	PaperDocCreateErrorFolderNotFound          = "folder_not_found"
	PaperDocCreateErrorDocLengthExceeded       = "doc_length_exceeded"
	PaperDocCreateErrorImageSizeExceeded       = "image_size_exceeded"
)

// PaperDocCreateUpdateResult : has no documentation (yet)
type PaperDocCreateUpdateResult struct {
	// DocId : Doc ID of the newly created doc.
	DocId string `json:"doc_id"`
	// Revision : The Paper doc revision. Simply an ever increasing number.
	Revision int64 `json:"revision"`
	// Title : The Paper doc title.
	Title string `json:"title"`
}

// NewPaperDocCreateUpdateResult returns a new PaperDocCreateUpdateResult instance
func NewPaperDocCreateUpdateResult(DocId string, Revision int64, Title string) *PaperDocCreateUpdateResult {
	s := new(PaperDocCreateUpdateResult)
	s.DocId = DocId
	s.Revision = Revision
	s.Title = Title
	return s
}

// PaperDocExport : has no documentation (yet)
type PaperDocExport struct {
	RefPaperDoc
	// ExportFormat : has no documentation (yet)
	ExportFormat *ExportFormat `json:"export_format"`
}

// NewPaperDocExport returns a new PaperDocExport instance
func NewPaperDocExport(DocId string, ExportFormat *ExportFormat) *PaperDocExport {
	s := new(PaperDocExport)
	s.DocId = DocId
	s.ExportFormat = ExportFormat
	return s
}

// PaperDocExportResult : has no documentation (yet)
type PaperDocExportResult struct {
	// Owner : The Paper doc owner's email address.
	Owner string `json:"owner"`
	// Title : The Paper doc title.
	Title string `json:"title"`
	// Revision : The Paper doc revision. Simply an ever increasing number.
	Revision int64 `json:"revision"`
	// MimeType : MIME type of the export. This corresponds to `ExportFormat`
	// specified in the request.
	MimeType string `json:"mime_type"`
}

// NewPaperDocExportResult returns a new PaperDocExportResult instance
func NewPaperDocExportResult(Owner string, Title string, Revision int64, MimeType string) *PaperDocExportResult {
	s := new(PaperDocExportResult)
	s.Owner = Owner
	s.Title = Title
	s.Revision = Revision
	s.MimeType = MimeType
	return s
}

// PaperDocPermissionLevel : has no documentation (yet)
type PaperDocPermissionLevel struct {
	dropbox.Tagged
}

// Valid tag values for PaperDocPermissionLevel
const (
	PaperDocPermissionLevelEdit           = "edit"
	PaperDocPermissionLevelViewAndComment = "view_and_comment"
	PaperDocPermissionLevelOther          = "other"
)

// PaperDocSharingPolicy : has no documentation (yet)
type PaperDocSharingPolicy struct {
	RefPaperDoc
	// SharingPolicy : The default sharing policy to be set for the Paper doc.
	SharingPolicy *SharingPolicy `json:"sharing_policy"`
}

// NewPaperDocSharingPolicy returns a new PaperDocSharingPolicy instance
func NewPaperDocSharingPolicy(DocId string, SharingPolicy *SharingPolicy) *PaperDocSharingPolicy {
	s := new(PaperDocSharingPolicy)
	s.DocId = DocId
	s.SharingPolicy = SharingPolicy
	return s
}

// PaperDocUpdateArgs : has no documentation (yet)
type PaperDocUpdateArgs struct {
	RefPaperDoc
	// DocUpdatePolicy : The policy used for the current update call.
	DocUpdatePolicy *PaperDocUpdatePolicy `json:"doc_update_policy"`
	// Revision : The latest doc revision. This value must match the head
	// revision or an error code will be returned. This is to prevent colliding
	// writes.
	Revision int64 `json:"revision"`
	// ImportFormat : The format of provided data.
	ImportFormat *ImportFormat `json:"import_format"`
}

// NewPaperDocUpdateArgs returns a new PaperDocUpdateArgs instance
func NewPaperDocUpdateArgs(DocId string, DocUpdatePolicy *PaperDocUpdatePolicy, Revision int64, ImportFormat *ImportFormat) *PaperDocUpdateArgs {
	s := new(PaperDocUpdateArgs)
	s.DocId = DocId
	s.DocUpdatePolicy = DocUpdatePolicy
	s.Revision = Revision
	s.ImportFormat = ImportFormat
	return s
}

// PaperDocUpdateError : has no documentation (yet)
type PaperDocUpdateError struct {
	dropbox.Tagged
}

// Valid tag values for PaperDocUpdateError
const (
	PaperDocUpdateErrorInsufficientPermissions = "insufficient_permissions"
	PaperDocUpdateErrorOther                   = "other"
	PaperDocUpdateErrorDocNotFound             = "doc_not_found"
	PaperDocUpdateErrorContentMalformed        = "content_malformed"
	PaperDocUpdateErrorRevisionMismatch        = "revision_mismatch"
	PaperDocUpdateErrorDocLengthExceeded       = "doc_length_exceeded"
	PaperDocUpdateErrorImageSizeExceeded       = "image_size_exceeded"
	PaperDocUpdateErrorDocArchived             = "doc_archived"
	PaperDocUpdateErrorDocDeleted              = "doc_deleted"
)

// PaperDocUpdatePolicy : has no documentation (yet)
type PaperDocUpdatePolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperDocUpdatePolicy
const (
	PaperDocUpdatePolicyAppend       = "append"
	PaperDocUpdatePolicyPrepend      = "prepend"
	PaperDocUpdatePolicyOverwriteAll = "overwrite_all"
	PaperDocUpdatePolicyOther        = "other"
)

// RemovePaperDocUser : has no documentation (yet)
type RemovePaperDocUser struct {
	RefPaperDoc
	// Member : User which should be removed from the Paper doc. Specify only
	// email address or Dropbox account ID.
	Member *sharing.MemberSelector `json:"member"`
}

// NewRemovePaperDocUser returns a new RemovePaperDocUser instance
func NewRemovePaperDocUser(DocId string, Member *sharing.MemberSelector) *RemovePaperDocUser {
	s := new(RemovePaperDocUser)
	s.DocId = DocId
	s.Member = Member
	return s
}

// SharingPolicy : Sharing policy of Paper doc.
type SharingPolicy struct {
	// PublicSharingPolicy : This value applies to the non-team members.
	PublicSharingPolicy *SharingPublicPolicyType `json:"public_sharing_policy,omitempty"`
	// TeamSharingPolicy : This value applies to the team members only. The
	// value is null for all personal accounts.
	TeamSharingPolicy *SharingTeamPolicyType `json:"team_sharing_policy,omitempty"`
}

// NewSharingPolicy returns a new SharingPolicy instance
func NewSharingPolicy() *SharingPolicy {
	s := new(SharingPolicy)
	return s
}

// SharingTeamPolicyType : The sharing policy type of the Paper doc.
type SharingTeamPolicyType struct {
	dropbox.Tagged
}

// Valid tag values for SharingTeamPolicyType
const (
	SharingTeamPolicyTypePeopleWithLinkCanEdit           = "people_with_link_can_edit"
	SharingTeamPolicyTypePeopleWithLinkCanViewAndComment = "people_with_link_can_view_and_comment"
	SharingTeamPolicyTypeInviteOnly                      = "invite_only"
)

// SharingPublicPolicyType : has no documentation (yet)
type SharingPublicPolicyType struct {
	dropbox.Tagged
}

// Valid tag values for SharingPublicPolicyType
const (
	SharingPublicPolicyTypePeopleWithLinkCanEdit           = "people_with_link_can_edit"
	SharingPublicPolicyTypePeopleWithLinkCanViewAndComment = "people_with_link_can_view_and_comment"
	SharingPublicPolicyTypeInviteOnly                      = "invite_only"
	SharingPublicPolicyTypeDisabled                        = "disabled"
)

// UserInfoWithPermissionLevel : has no documentation (yet)
type UserInfoWithPermissionLevel struct {
	// User : User shared on the Paper doc.
	User *sharing.UserInfo `json:"user"`
	// PermissionLevel : Permission level for the user.
	PermissionLevel *PaperDocPermissionLevel `json:"permission_level"`
}

// NewUserInfoWithPermissionLevel returns a new UserInfoWithPermissionLevel instance
func NewUserInfoWithPermissionLevel(User *sharing.UserInfo, PermissionLevel *PaperDocPermissionLevel) *UserInfoWithPermissionLevel {
	s := new(UserInfoWithPermissionLevel)
	s.User = User
	s.PermissionLevel = PermissionLevel
	return s
}

// UserOnPaperDocFilter : has no documentation (yet)
type UserOnPaperDocFilter struct {
	dropbox.Tagged
}

// Valid tag values for UserOnPaperDocFilter
const (
	UserOnPaperDocFilterVisited = "visited"
	UserOnPaperDocFilterShared  = "shared"
	UserOnPaperDocFilterOther   = "other"
)
