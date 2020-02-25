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

// Package sharing : This namespace contains endpoints and data types for
// creating and managing shared links and shared folders.
package sharing

import (
	"encoding/json"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/seen_state"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team_common"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/users"
)

// AccessInheritance : Information about the inheritance policy of a shared
// folder.
type AccessInheritance struct {
	dropbox.Tagged
}

// Valid tag values for AccessInheritance
const (
	AccessInheritanceInherit   = "inherit"
	AccessInheritanceNoInherit = "no_inherit"
	AccessInheritanceOther     = "other"
)

// AccessLevel : Defines the access levels for collaborators.
type AccessLevel struct {
	dropbox.Tagged
}

// Valid tag values for AccessLevel
const (
	AccessLevelOwner           = "owner"
	AccessLevelEditor          = "editor"
	AccessLevelViewer          = "viewer"
	AccessLevelViewerNoComment = "viewer_no_comment"
	AccessLevelOther           = "other"
)

// AclUpdatePolicy : Who can change a shared folder's access control list (ACL).
// In other words, who can add, remove, or change the privileges of members.
type AclUpdatePolicy struct {
	dropbox.Tagged
}

// Valid tag values for AclUpdatePolicy
const (
	AclUpdatePolicyOwner   = "owner"
	AclUpdatePolicyEditors = "editors"
	AclUpdatePolicyOther   = "other"
)

// AddFileMemberArgs : Arguments for `addFileMember`.
type AddFileMemberArgs struct {
	// File : File to which to add members.
	File string `json:"file"`
	// Members : Members to add. Note that even an email address is given, this
	// may result in a user being directy added to the membership if that email
	// is the user's main account email.
	Members []*MemberSelector `json:"members"`
	// CustomMessage : Message to send to added members in their invitation.
	CustomMessage string `json:"custom_message,omitempty"`
	// Quiet : Whether added members should be notified via device notifications
	// of their invitation.
	Quiet bool `json:"quiet"`
	// AccessLevel : AccessLevel union object, describing what access level we
	// want to give new members.
	AccessLevel *AccessLevel `json:"access_level"`
	// AddMessageAsComment : If the custom message should be added as a comment
	// on the file.
	AddMessageAsComment bool `json:"add_message_as_comment"`
}

// NewAddFileMemberArgs returns a new AddFileMemberArgs instance
func NewAddFileMemberArgs(File string, Members []*MemberSelector) *AddFileMemberArgs {
	s := new(AddFileMemberArgs)
	s.File = File
	s.Members = Members
	s.Quiet = false
	s.AccessLevel = &AccessLevel{Tagged: dropbox.Tagged{"viewer"}}
	s.AddMessageAsComment = false
	return s
}

// AddFileMemberError : Errors for `addFileMember`.
type AddFileMemberError struct {
	dropbox.Tagged
	// UserError : has no documentation (yet)
	UserError *SharingUserError `json:"user_error,omitempty"`
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for AddFileMemberError
const (
	AddFileMemberErrorUserError      = "user_error"
	AddFileMemberErrorAccessError    = "access_error"
	AddFileMemberErrorRateLimit      = "rate_limit"
	AddFileMemberErrorInvalidComment = "invalid_comment"
	AddFileMemberErrorOther          = "other"
)

// UnmarshalJSON deserializes into a AddFileMemberError instance
func (u *AddFileMemberError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : has no documentation (yet)
		UserError *SharingUserError `json:"user_error,omitempty"`
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// AddFolderMemberArg : has no documentation (yet)
type AddFolderMemberArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// Members : The intended list of members to add.  Added members will
	// receive invites to join the shared folder.
	Members []*AddMember `json:"members"`
	// Quiet : Whether added members should be notified via email and device
	// notifications of their invite.
	Quiet bool `json:"quiet"`
	// CustomMessage : Optional message to display to added members in their
	// invitation.
	CustomMessage string `json:"custom_message,omitempty"`
}

// NewAddFolderMemberArg returns a new AddFolderMemberArg instance
func NewAddFolderMemberArg(SharedFolderId string, Members []*AddMember) *AddFolderMemberArg {
	s := new(AddFolderMemberArg)
	s.SharedFolderId = SharedFolderId
	s.Members = Members
	s.Quiet = false
	return s
}

// AddFolderMemberError : has no documentation (yet)
type AddFolderMemberError struct {
	dropbox.Tagged
	// AccessError : Unable to access shared folder.
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	// BadMember : `AddFolderMemberArg.members` contains a bad invitation
	// recipient.
	BadMember *AddMemberSelectorError `json:"bad_member,omitempty"`
	// TooManyMembers : The value is the member limit that was reached.
	TooManyMembers uint64 `json:"too_many_members,omitempty"`
	// TooManyPendingInvites : The value is the pending invite limit that was
	// reached.
	TooManyPendingInvites uint64 `json:"too_many_pending_invites,omitempty"`
}

// Valid tag values for AddFolderMemberError
const (
	AddFolderMemberErrorAccessError           = "access_error"
	AddFolderMemberErrorEmailUnverified       = "email_unverified"
	AddFolderMemberErrorBannedMember          = "banned_member"
	AddFolderMemberErrorBadMember             = "bad_member"
	AddFolderMemberErrorCantShareOutsideTeam  = "cant_share_outside_team"
	AddFolderMemberErrorTooManyMembers        = "too_many_members"
	AddFolderMemberErrorTooManyPendingInvites = "too_many_pending_invites"
	AddFolderMemberErrorRateLimit             = "rate_limit"
	AddFolderMemberErrorTooManyInvitees       = "too_many_invitees"
	AddFolderMemberErrorInsufficientPlan      = "insufficient_plan"
	AddFolderMemberErrorTeamFolder            = "team_folder"
	AddFolderMemberErrorNoPermission          = "no_permission"
	AddFolderMemberErrorOther                 = "other"
)

// UnmarshalJSON deserializes into a AddFolderMemberError instance
func (u *AddFolderMemberError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : Unable to access shared folder.
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
		// BadMember : `AddFolderMemberArg.members` contains a bad invitation
		// recipient.
		BadMember *AddMemberSelectorError `json:"bad_member,omitempty"`
		// TooManyMembers : The value is the member limit that was reached.
		TooManyMembers uint64 `json:"too_many_members,omitempty"`
		// TooManyPendingInvites : The value is the pending invite limit that
		// was reached.
		TooManyPendingInvites uint64 `json:"too_many_pending_invites,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	case "bad_member":
		u.BadMember = w.BadMember

		if err != nil {
			return err
		}
	case "too_many_members":
		u.TooManyMembers = w.TooManyMembers

		if err != nil {
			return err
		}
	case "too_many_pending_invites":
		u.TooManyPendingInvites = w.TooManyPendingInvites

		if err != nil {
			return err
		}
	}
	return nil
}

// AddMember : The member and type of access the member should have when added
// to a shared folder.
type AddMember struct {
	// Member : The member to add to the shared folder.
	Member *MemberSelector `json:"member"`
	// AccessLevel : The access level to grant `member` to the shared folder.
	// `AccessLevel.owner` is disallowed.
	AccessLevel *AccessLevel `json:"access_level"`
}

// NewAddMember returns a new AddMember instance
func NewAddMember(Member *MemberSelector) *AddMember {
	s := new(AddMember)
	s.Member = Member
	s.AccessLevel = &AccessLevel{Tagged: dropbox.Tagged{"viewer"}}
	return s
}

// AddMemberSelectorError : has no documentation (yet)
type AddMemberSelectorError struct {
	dropbox.Tagged
	// InvalidDropboxId : The value is the ID that could not be identified.
	InvalidDropboxId string `json:"invalid_dropbox_id,omitempty"`
	// InvalidEmail : The value is the e-email address that is malformed.
	InvalidEmail string `json:"invalid_email,omitempty"`
	// UnverifiedDropboxId : The value is the ID of the Dropbox user with an
	// unverified e-mail address.  Invite unverified users by e-mail address
	// instead of by their Dropbox ID.
	UnverifiedDropboxId string `json:"unverified_dropbox_id,omitempty"`
}

// Valid tag values for AddMemberSelectorError
const (
	AddMemberSelectorErrorAutomaticGroup      = "automatic_group"
	AddMemberSelectorErrorInvalidDropboxId    = "invalid_dropbox_id"
	AddMemberSelectorErrorInvalidEmail        = "invalid_email"
	AddMemberSelectorErrorUnverifiedDropboxId = "unverified_dropbox_id"
	AddMemberSelectorErrorGroupDeleted        = "group_deleted"
	AddMemberSelectorErrorGroupNotOnTeam      = "group_not_on_team"
	AddMemberSelectorErrorOther               = "other"
)

// UnmarshalJSON deserializes into a AddMemberSelectorError instance
func (u *AddMemberSelectorError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// InvalidDropboxId : The value is the ID that could not be identified.
		InvalidDropboxId string `json:"invalid_dropbox_id,omitempty"`
		// InvalidEmail : The value is the e-email address that is malformed.
		InvalidEmail string `json:"invalid_email,omitempty"`
		// UnverifiedDropboxId : The value is the ID of the Dropbox user with an
		// unverified e-mail address.  Invite unverified users by e-mail address
		// instead of by their Dropbox ID.
		UnverifiedDropboxId string `json:"unverified_dropbox_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "invalid_dropbox_id":
		u.InvalidDropboxId = w.InvalidDropboxId

		if err != nil {
			return err
		}
	case "invalid_email":
		u.InvalidEmail = w.InvalidEmail

		if err != nil {
			return err
		}
	case "unverified_dropbox_id":
		u.UnverifiedDropboxId = w.UnverifiedDropboxId

		if err != nil {
			return err
		}
	}
	return nil
}

// AudienceExceptionContentInfo : Information about the content that has a link
// audience different than that of this folder.
type AudienceExceptionContentInfo struct {
	// Name : The name of the content, which is either a file or a folder.
	Name string `json:"name"`
}

// NewAudienceExceptionContentInfo returns a new AudienceExceptionContentInfo instance
func NewAudienceExceptionContentInfo(Name string) *AudienceExceptionContentInfo {
	s := new(AudienceExceptionContentInfo)
	s.Name = Name
	return s
}

// AudienceExceptions : The total count and truncated list of information of
// content inside this folder that has a different audience than the link on
// this folder. This is only returned for folders.
type AudienceExceptions struct {
	// Count : has no documentation (yet)
	Count uint32 `json:"count"`
	// Exceptions : A truncated list of some of the content that is an
	// exception. The length of this list could be smaller than the count since
	// it is only a sample but will not be empty as long as count is not 0.
	Exceptions []*AudienceExceptionContentInfo `json:"exceptions"`
}

// NewAudienceExceptions returns a new AudienceExceptions instance
func NewAudienceExceptions(Count uint32, Exceptions []*AudienceExceptionContentInfo) *AudienceExceptions {
	s := new(AudienceExceptions)
	s.Count = Count
	s.Exceptions = Exceptions
	return s
}

// AudienceRestrictingSharedFolder : Information about the shared folder that
// prevents the link audience for this link from being more restrictive.
type AudienceRestrictingSharedFolder struct {
	// SharedFolderId : The ID of the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// Name : The name of the shared folder.
	Name string `json:"name"`
	// Audience : The link audience of the shared folder.
	Audience *LinkAudience `json:"audience"`
}

// NewAudienceRestrictingSharedFolder returns a new AudienceRestrictingSharedFolder instance
func NewAudienceRestrictingSharedFolder(SharedFolderId string, Name string, Audience *LinkAudience) *AudienceRestrictingSharedFolder {
	s := new(AudienceRestrictingSharedFolder)
	s.SharedFolderId = SharedFolderId
	s.Name = Name
	s.Audience = Audience
	return s
}

// ChangeFileMemberAccessArgs : Arguments for `changeFileMemberAccess`.
type ChangeFileMemberAccessArgs struct {
	// File : File for which we are changing a member's access.
	File string `json:"file"`
	// Member : The member whose access we are changing.
	Member *MemberSelector `json:"member"`
	// AccessLevel : The new access level for the member.
	AccessLevel *AccessLevel `json:"access_level"`
}

// NewChangeFileMemberAccessArgs returns a new ChangeFileMemberAccessArgs instance
func NewChangeFileMemberAccessArgs(File string, Member *MemberSelector, AccessLevel *AccessLevel) *ChangeFileMemberAccessArgs {
	s := new(ChangeFileMemberAccessArgs)
	s.File = File
	s.Member = Member
	s.AccessLevel = AccessLevel
	return s
}

// LinkMetadata : Metadata for a shared link. This can be either a
// `PathLinkMetadata` or `CollectionLinkMetadata`.
type LinkMetadata struct {
	// Url : URL of the shared link.
	Url string `json:"url"`
	// Visibility : Who can access the link.
	Visibility *Visibility `json:"visibility"`
	// Expires : Expiration time, if set. By default the link won't expire.
	Expires time.Time `json:"expires,omitempty"`
}

// NewLinkMetadata returns a new LinkMetadata instance
func NewLinkMetadata(Url string, Visibility *Visibility) *LinkMetadata {
	s := new(LinkMetadata)
	s.Url = Url
	s.Visibility = Visibility
	return s
}

// IsLinkMetadata is the interface type for LinkMetadata and its subtypes
type IsLinkMetadata interface {
	IsLinkMetadata()
}

// IsLinkMetadata implements the IsLinkMetadata interface
func (u *LinkMetadata) IsLinkMetadata() {}

type linkMetadataUnion struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *PathLinkMetadata `json:"path,omitempty"`
	// Collection : has no documentation (yet)
	Collection *CollectionLinkMetadata `json:"collection,omitempty"`
}

// Valid tag values for LinkMetadata
const (
	LinkMetadataPath       = "path"
	LinkMetadataCollection = "collection"
)

// UnmarshalJSON deserializes into a linkMetadataUnion instance
func (u *linkMetadataUnion) UnmarshalJSON(body []byte) error {
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
	case "path":
		err = json.Unmarshal(body, &u.Path)

		if err != nil {
			return err
		}
	case "collection":
		err = json.Unmarshal(body, &u.Collection)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsLinkMetadataFromJSON converts JSON to a concrete IsLinkMetadata instance
func IsLinkMetadataFromJSON(data []byte) (IsLinkMetadata, error) {
	var t linkMetadataUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "path":
		return t.Path, nil

	case "collection":
		return t.Collection, nil

	}
	return nil, nil
}

// CollectionLinkMetadata : Metadata for a collection-based shared link.
type CollectionLinkMetadata struct {
	LinkMetadata
}

// NewCollectionLinkMetadata returns a new CollectionLinkMetadata instance
func NewCollectionLinkMetadata(Url string, Visibility *Visibility) *CollectionLinkMetadata {
	s := new(CollectionLinkMetadata)
	s.Url = Url
	s.Visibility = Visibility
	return s
}

// CreateSharedLinkArg : has no documentation (yet)
type CreateSharedLinkArg struct {
	// Path : The path to share.
	Path string `json:"path"`
	// ShortUrl : Whether to return a shortened URL.
	ShortUrl bool `json:"short_url"`
	// PendingUpload : If it's okay to share a path that does not yet exist, set
	// this to either `PendingUploadMode.file` or `PendingUploadMode.folder` to
	// indicate whether to assume it's a file or folder.
	PendingUpload *PendingUploadMode `json:"pending_upload,omitempty"`
}

// NewCreateSharedLinkArg returns a new CreateSharedLinkArg instance
func NewCreateSharedLinkArg(Path string) *CreateSharedLinkArg {
	s := new(CreateSharedLinkArg)
	s.Path = Path
	s.ShortUrl = false
	return s
}

// CreateSharedLinkError : has no documentation (yet)
type CreateSharedLinkError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *files.LookupError `json:"path,omitempty"`
}

// Valid tag values for CreateSharedLinkError
const (
	CreateSharedLinkErrorPath  = "path"
	CreateSharedLinkErrorOther = "other"
)

// UnmarshalJSON deserializes into a CreateSharedLinkError instance
func (u *CreateSharedLinkError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *files.LookupError `json:"path,omitempty"`
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

		if err != nil {
			return err
		}
	}
	return nil
}

// CreateSharedLinkWithSettingsArg : has no documentation (yet)
type CreateSharedLinkWithSettingsArg struct {
	// Path : The path to be shared by the shared link.
	Path string `json:"path"`
	// Settings : The requested settings for the newly created shared link.
	Settings *SharedLinkSettings `json:"settings,omitempty"`
}

// NewCreateSharedLinkWithSettingsArg returns a new CreateSharedLinkWithSettingsArg instance
func NewCreateSharedLinkWithSettingsArg(Path string) *CreateSharedLinkWithSettingsArg {
	s := new(CreateSharedLinkWithSettingsArg)
	s.Path = Path
	return s
}

// CreateSharedLinkWithSettingsError : has no documentation (yet)
type CreateSharedLinkWithSettingsError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *files.LookupError `json:"path,omitempty"`
	// SharedLinkAlreadyExists : The shared link already exists. You can call
	// `listSharedLinks` to get the  existing link, or use the provided metadata
	// if it is returned.
	SharedLinkAlreadyExists *SharedLinkAlreadyExistsMetadata `json:"shared_link_already_exists,omitempty"`
	// SettingsError : There is an error with the given settings.
	SettingsError *SharedLinkSettingsError `json:"settings_error,omitempty"`
}

// Valid tag values for CreateSharedLinkWithSettingsError
const (
	CreateSharedLinkWithSettingsErrorPath                    = "path"
	CreateSharedLinkWithSettingsErrorEmailNotVerified        = "email_not_verified"
	CreateSharedLinkWithSettingsErrorSharedLinkAlreadyExists = "shared_link_already_exists"
	CreateSharedLinkWithSettingsErrorSettingsError           = "settings_error"
	CreateSharedLinkWithSettingsErrorAccessDenied            = "access_denied"
)

// UnmarshalJSON deserializes into a CreateSharedLinkWithSettingsError instance
func (u *CreateSharedLinkWithSettingsError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *files.LookupError `json:"path,omitempty"`
		// SharedLinkAlreadyExists : The shared link already exists. You can
		// call `listSharedLinks` to get the  existing link, or use the provided
		// metadata if it is returned.
		SharedLinkAlreadyExists *SharedLinkAlreadyExistsMetadata `json:"shared_link_already_exists,omitempty"`
		// SettingsError : There is an error with the given settings.
		SettingsError *SharedLinkSettingsError `json:"settings_error,omitempty"`
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

		if err != nil {
			return err
		}
	case "shared_link_already_exists":
		u.SharedLinkAlreadyExists = w.SharedLinkAlreadyExists

		if err != nil {
			return err
		}
	case "settings_error":
		u.SettingsError = w.SettingsError

		if err != nil {
			return err
		}
	}
	return nil
}

// SharedContentLinkMetadataBase : has no documentation (yet)
type SharedContentLinkMetadataBase struct {
	// AccessLevel : The access level on the link for this file.
	AccessLevel *AccessLevel `json:"access_level,omitempty"`
	// AudienceOptions : The audience options that are available for the
	// content. Some audience options may be unavailable. For example, team_only
	// may be unavailable if the content is not owned by a user on a team. The
	// 'default' audience option is always available if the user can modify link
	// settings.
	AudienceOptions []*LinkAudience `json:"audience_options"`
	// AudienceRestrictingSharedFolder : The shared folder that prevents the
	// link audience for this link from being more restrictive.
	AudienceRestrictingSharedFolder *AudienceRestrictingSharedFolder `json:"audience_restricting_shared_folder,omitempty"`
	// CurrentAudience : The current audience of the link.
	CurrentAudience *LinkAudience `json:"current_audience"`
	// Expiry : Whether the link has an expiry set on it. A link with an expiry
	// will have its  audience changed to members when the expiry is reached.
	Expiry time.Time `json:"expiry,omitempty"`
	// LinkPermissions : A list of permissions for actions you can perform on
	// the link.
	LinkPermissions []*LinkPermission `json:"link_permissions"`
	// PasswordProtected : Whether the link is protected by a password.
	PasswordProtected bool `json:"password_protected"`
}

// NewSharedContentLinkMetadataBase returns a new SharedContentLinkMetadataBase instance
func NewSharedContentLinkMetadataBase(AudienceOptions []*LinkAudience, CurrentAudience *LinkAudience, LinkPermissions []*LinkPermission, PasswordProtected bool) *SharedContentLinkMetadataBase {
	s := new(SharedContentLinkMetadataBase)
	s.AudienceOptions = AudienceOptions
	s.CurrentAudience = CurrentAudience
	s.LinkPermissions = LinkPermissions
	s.PasswordProtected = PasswordProtected
	return s
}

// ExpectedSharedContentLinkMetadata : The expected metadata of a shared link
// for a file or folder when a link is first created for the content. Absent if
// the link already exists.
type ExpectedSharedContentLinkMetadata struct {
	SharedContentLinkMetadataBase
}

// NewExpectedSharedContentLinkMetadata returns a new ExpectedSharedContentLinkMetadata instance
func NewExpectedSharedContentLinkMetadata(AudienceOptions []*LinkAudience, CurrentAudience *LinkAudience, LinkPermissions []*LinkPermission, PasswordProtected bool) *ExpectedSharedContentLinkMetadata {
	s := new(ExpectedSharedContentLinkMetadata)
	s.AudienceOptions = AudienceOptions
	s.CurrentAudience = CurrentAudience
	s.LinkPermissions = LinkPermissions
	s.PasswordProtected = PasswordProtected
	return s
}

// FileAction : Sharing actions that may be taken on files.
type FileAction struct {
	dropbox.Tagged
}

// Valid tag values for FileAction
const (
	FileActionDisableViewerInfo     = "disable_viewer_info"
	FileActionEditContents          = "edit_contents"
	FileActionEnableViewerInfo      = "enable_viewer_info"
	FileActionInviteViewer          = "invite_viewer"
	FileActionInviteViewerNoComment = "invite_viewer_no_comment"
	FileActionInviteEditor          = "invite_editor"
	FileActionUnshare               = "unshare"
	FileActionRelinquishMembership  = "relinquish_membership"
	FileActionShareLink             = "share_link"
	FileActionCreateLink            = "create_link"
	FileActionCreateViewLink        = "create_view_link"
	FileActionCreateEditLink        = "create_edit_link"
	FileActionOther                 = "other"
)

// FileErrorResult : has no documentation (yet)
type FileErrorResult struct {
	dropbox.Tagged
	// FileNotFoundError : File specified by id was not found.
	FileNotFoundError string `json:"file_not_found_error,omitempty"`
	// InvalidFileActionError : User does not have permission to take the
	// specified action on the file.
	InvalidFileActionError string `json:"invalid_file_action_error,omitempty"`
	// PermissionDeniedError : User does not have permission to access file
	// specified by file.Id.
	PermissionDeniedError string `json:"permission_denied_error,omitempty"`
}

// Valid tag values for FileErrorResult
const (
	FileErrorResultFileNotFoundError      = "file_not_found_error"
	FileErrorResultInvalidFileActionError = "invalid_file_action_error"
	FileErrorResultPermissionDeniedError  = "permission_denied_error"
	FileErrorResultOther                  = "other"
)

// UnmarshalJSON deserializes into a FileErrorResult instance
func (u *FileErrorResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// FileNotFoundError : File specified by id was not found.
		FileNotFoundError string `json:"file_not_found_error,omitempty"`
		// InvalidFileActionError : User does not have permission to take the
		// specified action on the file.
		InvalidFileActionError string `json:"invalid_file_action_error,omitempty"`
		// PermissionDeniedError : User does not have permission to access file
		// specified by file.Id.
		PermissionDeniedError string `json:"permission_denied_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "file_not_found_error":
		u.FileNotFoundError = w.FileNotFoundError

		if err != nil {
			return err
		}
	case "invalid_file_action_error":
		u.InvalidFileActionError = w.InvalidFileActionError

		if err != nil {
			return err
		}
	case "permission_denied_error":
		u.PermissionDeniedError = w.PermissionDeniedError

		if err != nil {
			return err
		}
	}
	return nil
}

// SharedLinkMetadata : The metadata of a shared link.
type SharedLinkMetadata struct {
	// Url : URL of the shared link.
	Url string `json:"url"`
	// Id : A unique identifier for the linked file.
	Id string `json:"id,omitempty"`
	// Name : The linked file name (including extension). This never contains a
	// slash.
	Name string `json:"name"`
	// Expires : Expiration time, if set. By default the link won't expire.
	Expires time.Time `json:"expires,omitempty"`
	// PathLower : The lowercased full path in the user's Dropbox. This always
	// starts with a slash. This field will only be present only if the linked
	// file is in the authenticated user's  dropbox.
	PathLower string `json:"path_lower,omitempty"`
	// LinkPermissions : The link's access permissions.
	LinkPermissions *LinkPermissions `json:"link_permissions"`
	// TeamMemberInfo : The team membership information of the link's owner.
	// This field will only be present  if the link's owner is a team member.
	TeamMemberInfo *TeamMemberInfo `json:"team_member_info,omitempty"`
	// ContentOwnerTeamInfo : The team information of the content's owner. This
	// field will only be present if the content's owner is a team member and
	// the content's owner team is different from the link's owner team.
	ContentOwnerTeamInfo *users.Team `json:"content_owner_team_info,omitempty"`
}

// NewSharedLinkMetadata returns a new SharedLinkMetadata instance
func NewSharedLinkMetadata(Url string, Name string, LinkPermissions *LinkPermissions) *SharedLinkMetadata {
	s := new(SharedLinkMetadata)
	s.Url = Url
	s.Name = Name
	s.LinkPermissions = LinkPermissions
	return s
}

// IsSharedLinkMetadata is the interface type for SharedLinkMetadata and its subtypes
type IsSharedLinkMetadata interface {
	IsSharedLinkMetadata()
}

// IsSharedLinkMetadata implements the IsSharedLinkMetadata interface
func (u *SharedLinkMetadata) IsSharedLinkMetadata() {}

type sharedLinkMetadataUnion struct {
	dropbox.Tagged
	// File : has no documentation (yet)
	File *FileLinkMetadata `json:"file,omitempty"`
	// Folder : has no documentation (yet)
	Folder *FolderLinkMetadata `json:"folder,omitempty"`
}

// Valid tag values for SharedLinkMetadata
const (
	SharedLinkMetadataFile   = "file"
	SharedLinkMetadataFolder = "folder"
)

// UnmarshalJSON deserializes into a sharedLinkMetadataUnion instance
func (u *sharedLinkMetadataUnion) UnmarshalJSON(body []byte) error {
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
		err = json.Unmarshal(body, &u.File)

		if err != nil {
			return err
		}
	case "folder":
		err = json.Unmarshal(body, &u.Folder)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsSharedLinkMetadataFromJSON converts JSON to a concrete IsSharedLinkMetadata instance
func IsSharedLinkMetadataFromJSON(data []byte) (IsSharedLinkMetadata, error) {
	var t sharedLinkMetadataUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "file":
		return t.File, nil

	case "folder":
		return t.Folder, nil

	}
	return nil, nil
}

// FileLinkMetadata : The metadata of a file shared link.
type FileLinkMetadata struct {
	SharedLinkMetadata
	// ClientModified : The modification time set by the desktop client when the
	// file was added to Dropbox. Since this time is not verified (the Dropbox
	// server stores whatever the desktop client sends up), this should only be
	// used for display purposes (such as sorting) and not, for example, to
	// determine if a file has changed or not.
	ClientModified time.Time `json:"client_modified"`
	// ServerModified : The last time the file was modified on Dropbox.
	ServerModified time.Time `json:"server_modified"`
	// Rev : A unique identifier for the current revision of a file. This field
	// is the same rev as elsewhere in the API and can be used to detect changes
	// and avoid conflicts.
	Rev string `json:"rev"`
	// Size : The file size in bytes.
	Size uint64 `json:"size"`
}

// NewFileLinkMetadata returns a new FileLinkMetadata instance
func NewFileLinkMetadata(Url string, Name string, LinkPermissions *LinkPermissions, ClientModified time.Time, ServerModified time.Time, Rev string, Size uint64) *FileLinkMetadata {
	s := new(FileLinkMetadata)
	s.Url = Url
	s.Name = Name
	s.LinkPermissions = LinkPermissions
	s.ClientModified = ClientModified
	s.ServerModified = ServerModified
	s.Rev = Rev
	s.Size = Size
	return s
}

// FileMemberActionError : has no documentation (yet)
type FileMemberActionError struct {
	dropbox.Tagged
	// AccessError : Specified file was invalid or user does not have access.
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	// NoExplicitAccess : The action cannot be completed because the target
	// member does not have explicit access to the file. The return value is the
	// access that the member has to the file from a parent folder.
	NoExplicitAccess *MemberAccessLevelResult `json:"no_explicit_access,omitempty"`
}

// Valid tag values for FileMemberActionError
const (
	FileMemberActionErrorInvalidMember    = "invalid_member"
	FileMemberActionErrorNoPermission     = "no_permission"
	FileMemberActionErrorAccessError      = "access_error"
	FileMemberActionErrorNoExplicitAccess = "no_explicit_access"
	FileMemberActionErrorOther            = "other"
)

// UnmarshalJSON deserializes into a FileMemberActionError instance
func (u *FileMemberActionError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : Specified file was invalid or user does not have
		// access.
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	case "no_explicit_access":
		err = json.Unmarshal(body, &u.NoExplicitAccess)

		if err != nil {
			return err
		}
	}
	return nil
}

// FileMemberActionIndividualResult : has no documentation (yet)
type FileMemberActionIndividualResult struct {
	dropbox.Tagged
	// Success : Member was successfully removed from this file. If AccessLevel
	// is given, the member still has access via a parent shared folder.
	Success *AccessLevel `json:"success,omitempty"`
	// MemberError : User was not able to perform this action.
	MemberError *FileMemberActionError `json:"member_error,omitempty"`
}

// Valid tag values for FileMemberActionIndividualResult
const (
	FileMemberActionIndividualResultSuccess     = "success"
	FileMemberActionIndividualResultMemberError = "member_error"
)

// UnmarshalJSON deserializes into a FileMemberActionIndividualResult instance
func (u *FileMemberActionIndividualResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Success : Member was successfully removed from this file. If
		// AccessLevel is given, the member still has access via a parent shared
		// folder.
		Success *AccessLevel `json:"success,omitempty"`
		// MemberError : User was not able to perform this action.
		MemberError *FileMemberActionError `json:"member_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "success":
		u.Success = w.Success

		if err != nil {
			return err
		}
	case "member_error":
		u.MemberError = w.MemberError

		if err != nil {
			return err
		}
	}
	return nil
}

// FileMemberActionResult : Per-member result for `addFileMember` or
// `changeFileMemberAccess`.
type FileMemberActionResult struct {
	// Member : One of specified input members.
	Member *MemberSelector `json:"member"`
	// Result : The outcome of the action on this member.
	Result *FileMemberActionIndividualResult `json:"result"`
}

// NewFileMemberActionResult returns a new FileMemberActionResult instance
func NewFileMemberActionResult(Member *MemberSelector, Result *FileMemberActionIndividualResult) *FileMemberActionResult {
	s := new(FileMemberActionResult)
	s.Member = Member
	s.Result = Result
	return s
}

// FileMemberRemoveActionResult : has no documentation (yet)
type FileMemberRemoveActionResult struct {
	dropbox.Tagged
	// Success : Member was successfully removed from this file.
	Success *MemberAccessLevelResult `json:"success,omitempty"`
	// MemberError : User was not able to remove this member.
	MemberError *FileMemberActionError `json:"member_error,omitempty"`
}

// Valid tag values for FileMemberRemoveActionResult
const (
	FileMemberRemoveActionResultSuccess     = "success"
	FileMemberRemoveActionResultMemberError = "member_error"
	FileMemberRemoveActionResultOther       = "other"
)

// UnmarshalJSON deserializes into a FileMemberRemoveActionResult instance
func (u *FileMemberRemoveActionResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MemberError : User was not able to remove this member.
		MemberError *FileMemberActionError `json:"member_error,omitempty"`
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
	case "member_error":
		u.MemberError = w.MemberError

		if err != nil {
			return err
		}
	}
	return nil
}

// FilePermission : Whether the user is allowed to take the sharing action on
// the file.
type FilePermission struct {
	// Action : The action that the user may wish to take on the file.
	Action *FileAction `json:"action"`
	// Allow : True if the user is allowed to take the action.
	Allow bool `json:"allow"`
	// Reason : The reason why the user is denied the permission. Not present if
	// the action is allowed.
	Reason *PermissionDeniedReason `json:"reason,omitempty"`
}

// NewFilePermission returns a new FilePermission instance
func NewFilePermission(Action *FileAction, Allow bool) *FilePermission {
	s := new(FilePermission)
	s.Action = Action
	s.Allow = Allow
	return s
}

// FolderAction : Actions that may be taken on shared folders.
type FolderAction struct {
	dropbox.Tagged
}

// Valid tag values for FolderAction
const (
	FolderActionChangeOptions         = "change_options"
	FolderActionDisableViewerInfo     = "disable_viewer_info"
	FolderActionEditContents          = "edit_contents"
	FolderActionEnableViewerInfo      = "enable_viewer_info"
	FolderActionInviteEditor          = "invite_editor"
	FolderActionInviteViewer          = "invite_viewer"
	FolderActionInviteViewerNoComment = "invite_viewer_no_comment"
	FolderActionRelinquishMembership  = "relinquish_membership"
	FolderActionUnmount               = "unmount"
	FolderActionUnshare               = "unshare"
	FolderActionLeaveACopy            = "leave_a_copy"
	FolderActionShareLink             = "share_link"
	FolderActionCreateLink            = "create_link"
	FolderActionSetAccessInheritance  = "set_access_inheritance"
	FolderActionOther                 = "other"
)

// FolderLinkMetadata : The metadata of a folder shared link.
type FolderLinkMetadata struct {
	SharedLinkMetadata
}

// NewFolderLinkMetadata returns a new FolderLinkMetadata instance
func NewFolderLinkMetadata(Url string, Name string, LinkPermissions *LinkPermissions) *FolderLinkMetadata {
	s := new(FolderLinkMetadata)
	s.Url = Url
	s.Name = Name
	s.LinkPermissions = LinkPermissions
	return s
}

// FolderPermission : Whether the user is allowed to take the action on the
// shared folder.
type FolderPermission struct {
	// Action : The action that the user may wish to take on the folder.
	Action *FolderAction `json:"action"`
	// Allow : True if the user is allowed to take the action.
	Allow bool `json:"allow"`
	// Reason : The reason why the user is denied the permission. Not present if
	// the action is allowed, or if no reason is available.
	Reason *PermissionDeniedReason `json:"reason,omitempty"`
}

// NewFolderPermission returns a new FolderPermission instance
func NewFolderPermission(Action *FolderAction, Allow bool) *FolderPermission {
	s := new(FolderPermission)
	s.Action = Action
	s.Allow = Allow
	return s
}

// FolderPolicy : A set of policies governing membership and privileges for a
// shared folder.
type FolderPolicy struct {
	// MemberPolicy : Who can be a member of this shared folder, as set on the
	// folder itself. The effective policy may differ from this value if the
	// team-wide policy is more restrictive. Present only if the folder is owned
	// by a team.
	MemberPolicy *MemberPolicy `json:"member_policy,omitempty"`
	// ResolvedMemberPolicy : Who can be a member of this shared folder, taking
	// into account both the folder and the team-wide policy. This value may
	// differ from that of member_policy if the team-wide policy is more
	// restrictive than the folder policy. Present only if the folder is owned
	// by a team.
	ResolvedMemberPolicy *MemberPolicy `json:"resolved_member_policy,omitempty"`
	// AclUpdatePolicy : Who can add and remove members from this shared folder.
	AclUpdatePolicy *AclUpdatePolicy `json:"acl_update_policy"`
	// SharedLinkPolicy : Who links can be shared with.
	SharedLinkPolicy *SharedLinkPolicy `json:"shared_link_policy"`
	// ViewerInfoPolicy : Who can enable/disable viewer info for this shared
	// folder.
	ViewerInfoPolicy *ViewerInfoPolicy `json:"viewer_info_policy,omitempty"`
}

// NewFolderPolicy returns a new FolderPolicy instance
func NewFolderPolicy(AclUpdatePolicy *AclUpdatePolicy, SharedLinkPolicy *SharedLinkPolicy) *FolderPolicy {
	s := new(FolderPolicy)
	s.AclUpdatePolicy = AclUpdatePolicy
	s.SharedLinkPolicy = SharedLinkPolicy
	return s
}

// GetFileMetadataArg : Arguments of `getFileMetadata`.
type GetFileMetadataArg struct {
	// File : The file to query.
	File string `json:"file"`
	// Actions : A list of `FileAction`s corresponding to `FilePermission`s that
	// should appear in the  response's `SharedFileMetadata.permissions` field
	// describing the actions the  authenticated user can perform on the file.
	Actions []*FileAction `json:"actions,omitempty"`
}

// NewGetFileMetadataArg returns a new GetFileMetadataArg instance
func NewGetFileMetadataArg(File string) *GetFileMetadataArg {
	s := new(GetFileMetadataArg)
	s.File = File
	return s
}

// GetFileMetadataBatchArg : Arguments of `getFileMetadataBatch`.
type GetFileMetadataBatchArg struct {
	// Files : The files to query.
	Files []string `json:"files"`
	// Actions : A list of `FileAction`s corresponding to `FilePermission`s that
	// should appear in the  response's `SharedFileMetadata.permissions` field
	// describing the actions the  authenticated user can perform on the file.
	Actions []*FileAction `json:"actions,omitempty"`
}

// NewGetFileMetadataBatchArg returns a new GetFileMetadataBatchArg instance
func NewGetFileMetadataBatchArg(Files []string) *GetFileMetadataBatchArg {
	s := new(GetFileMetadataBatchArg)
	s.Files = Files
	return s
}

// GetFileMetadataBatchResult : Per file results of `getFileMetadataBatch`.
type GetFileMetadataBatchResult struct {
	// File : This is the input file identifier corresponding to one of
	// `GetFileMetadataBatchArg.files`.
	File string `json:"file"`
	// Result : The result for this particular file.
	Result *GetFileMetadataIndividualResult `json:"result"`
}

// NewGetFileMetadataBatchResult returns a new GetFileMetadataBatchResult instance
func NewGetFileMetadataBatchResult(File string, Result *GetFileMetadataIndividualResult) *GetFileMetadataBatchResult {
	s := new(GetFileMetadataBatchResult)
	s.File = File
	s.Result = Result
	return s
}

// GetFileMetadataError : Error result for `getFileMetadata`.
type GetFileMetadataError struct {
	dropbox.Tagged
	// UserError : has no documentation (yet)
	UserError *SharingUserError `json:"user_error,omitempty"`
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for GetFileMetadataError
const (
	GetFileMetadataErrorUserError   = "user_error"
	GetFileMetadataErrorAccessError = "access_error"
	GetFileMetadataErrorOther       = "other"
)

// UnmarshalJSON deserializes into a GetFileMetadataError instance
func (u *GetFileMetadataError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : has no documentation (yet)
		UserError *SharingUserError `json:"user_error,omitempty"`
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// GetFileMetadataIndividualResult : has no documentation (yet)
type GetFileMetadataIndividualResult struct {
	dropbox.Tagged
	// Metadata : The result for this file if it was successful.
	Metadata *SharedFileMetadata `json:"metadata,omitempty"`
	// AccessError : The result for this file if it was an error.
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for GetFileMetadataIndividualResult
const (
	GetFileMetadataIndividualResultMetadata    = "metadata"
	GetFileMetadataIndividualResultAccessError = "access_error"
	GetFileMetadataIndividualResultOther       = "other"
)

// UnmarshalJSON deserializes into a GetFileMetadataIndividualResult instance
func (u *GetFileMetadataIndividualResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : The result for this file if it was an error.
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "metadata":
		err = json.Unmarshal(body, &u.Metadata)

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// GetMetadataArgs : has no documentation (yet)
type GetMetadataArgs struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// Actions : A list of `FolderAction`s corresponding to `FolderPermission`s
	// that should appear in the  response's `SharedFolderMetadata.permissions`
	// field describing the actions the  authenticated user can perform on the
	// folder.
	Actions []*FolderAction `json:"actions,omitempty"`
}

// NewGetMetadataArgs returns a new GetMetadataArgs instance
func NewGetMetadataArgs(SharedFolderId string) *GetMetadataArgs {
	s := new(GetMetadataArgs)
	s.SharedFolderId = SharedFolderId
	return s
}

// SharedLinkError : has no documentation (yet)
type SharedLinkError struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkError
const (
	SharedLinkErrorSharedLinkNotFound     = "shared_link_not_found"
	SharedLinkErrorSharedLinkAccessDenied = "shared_link_access_denied"
	SharedLinkErrorUnsupportedLinkType    = "unsupported_link_type"
	SharedLinkErrorOther                  = "other"
)

// GetSharedLinkFileError : has no documentation (yet)
type GetSharedLinkFileError struct {
	dropbox.Tagged
}

// Valid tag values for GetSharedLinkFileError
const (
	GetSharedLinkFileErrorSharedLinkNotFound     = "shared_link_not_found"
	GetSharedLinkFileErrorSharedLinkAccessDenied = "shared_link_access_denied"
	GetSharedLinkFileErrorUnsupportedLinkType    = "unsupported_link_type"
	GetSharedLinkFileErrorOther                  = "other"
	GetSharedLinkFileErrorSharedLinkIsDirectory  = "shared_link_is_directory"
)

// GetSharedLinkMetadataArg : has no documentation (yet)
type GetSharedLinkMetadataArg struct {
	// Url : URL of the shared link.
	Url string `json:"url"`
	// Path : If the shared link is to a folder, this parameter can be used to
	// retrieve the metadata for a specific file or sub-folder in this folder. A
	// relative path should be used.
	Path string `json:"path,omitempty"`
	// LinkPassword : If the shared link has a password, this parameter can be
	// used.
	LinkPassword string `json:"link_password,omitempty"`
}

// NewGetSharedLinkMetadataArg returns a new GetSharedLinkMetadataArg instance
func NewGetSharedLinkMetadataArg(Url string) *GetSharedLinkMetadataArg {
	s := new(GetSharedLinkMetadataArg)
	s.Url = Url
	return s
}

// GetSharedLinksArg : has no documentation (yet)
type GetSharedLinksArg struct {
	// Path : See `getSharedLinks` description.
	Path string `json:"path,omitempty"`
}

// NewGetSharedLinksArg returns a new GetSharedLinksArg instance
func NewGetSharedLinksArg() *GetSharedLinksArg {
	s := new(GetSharedLinksArg)
	return s
}

// GetSharedLinksError : has no documentation (yet)
type GetSharedLinksError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path string `json:"path,omitempty"`
}

// Valid tag values for GetSharedLinksError
const (
	GetSharedLinksErrorPath  = "path"
	GetSharedLinksErrorOther = "other"
)

// UnmarshalJSON deserializes into a GetSharedLinksError instance
func (u *GetSharedLinksError) UnmarshalJSON(body []byte) error {
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

		if err != nil {
			return err
		}
	}
	return nil
}

// GetSharedLinksResult : has no documentation (yet)
type GetSharedLinksResult struct {
	// Links : Shared links applicable to the path argument.
	Links []IsLinkMetadata `json:"links"`
}

// NewGetSharedLinksResult returns a new GetSharedLinksResult instance
func NewGetSharedLinksResult(Links []IsLinkMetadata) *GetSharedLinksResult {
	s := new(GetSharedLinksResult)
	s.Links = Links
	return s
}

// UnmarshalJSON deserializes into a GetSharedLinksResult instance
func (u *GetSharedLinksResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Links : Shared links applicable to the path argument.
		Links []json.RawMessage `json:"links"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	u.Links = make([]IsLinkMetadata, len(w.Links))
	for i, e := range w.Links {
		v, err := IsLinkMetadataFromJSON(e)
		if err != nil {
			return err
		}
		u.Links[i] = v
	}
	return nil
}

// GroupInfo : The information about a group. Groups is a way to manage a list
// of users  who need same access permission to the shared folder.
type GroupInfo struct {
	team_common.GroupSummary
	// GroupType : The type of group.
	GroupType *team_common.GroupType `json:"group_type"`
	// IsMember : If the current user is a member of the group.
	IsMember bool `json:"is_member"`
	// IsOwner : If the current user is an owner of the group.
	IsOwner bool `json:"is_owner"`
	// SameTeam : If the group is owned by the current user's team.
	SameTeam bool `json:"same_team"`
}

// NewGroupInfo returns a new GroupInfo instance
func NewGroupInfo(GroupName string, GroupId string, GroupManagementType *team_common.GroupManagementType, GroupType *team_common.GroupType, IsMember bool, IsOwner bool, SameTeam bool) *GroupInfo {
	s := new(GroupInfo)
	s.GroupName = GroupName
	s.GroupId = GroupId
	s.GroupManagementType = GroupManagementType
	s.GroupType = GroupType
	s.IsMember = IsMember
	s.IsOwner = IsOwner
	s.SameTeam = SameTeam
	return s
}

// MembershipInfo : The information about a member of the shared content.
type MembershipInfo struct {
	// AccessType : The access type for this member. It contains inherited
	// access type from parent folder, and acquired access type from this
	// folder.
	AccessType *AccessLevel `json:"access_type"`
	// Permissions : The permissions that requesting user has on this member.
	// The set of permissions corresponds to the MemberActions in the request.
	Permissions []*MemberPermission `json:"permissions,omitempty"`
	// Initials : Never set.
	Initials string `json:"initials,omitempty"`
	// IsInherited : True if the member has access from a parent folder.
	IsInherited bool `json:"is_inherited"`
}

// NewMembershipInfo returns a new MembershipInfo instance
func NewMembershipInfo(AccessType *AccessLevel) *MembershipInfo {
	s := new(MembershipInfo)
	s.AccessType = AccessType
	s.IsInherited = false
	return s
}

// GroupMembershipInfo : The information about a group member of the shared
// content.
type GroupMembershipInfo struct {
	MembershipInfo
	// Group : The information about the membership group.
	Group *GroupInfo `json:"group"`
}

// NewGroupMembershipInfo returns a new GroupMembershipInfo instance
func NewGroupMembershipInfo(AccessType *AccessLevel, Group *GroupInfo) *GroupMembershipInfo {
	s := new(GroupMembershipInfo)
	s.AccessType = AccessType
	s.Group = Group
	s.IsInherited = false
	return s
}

// InsufficientPlan : has no documentation (yet)
type InsufficientPlan struct {
	// Message : A message to tell the user to upgrade in order to support
	// expected action.
	Message string `json:"message"`
	// UpsellUrl : A URL to send the user to in order to obtain the account type
	// they need, e.g. upgrading. Absent if there is no action the user can take
	// to upgrade.
	UpsellUrl string `json:"upsell_url,omitempty"`
}

// NewInsufficientPlan returns a new InsufficientPlan instance
func NewInsufficientPlan(Message string) *InsufficientPlan {
	s := new(InsufficientPlan)
	s.Message = Message
	return s
}

// InsufficientQuotaAmounts : has no documentation (yet)
type InsufficientQuotaAmounts struct {
	// SpaceNeeded : The amount of space needed to add the item (the size of the
	// item).
	SpaceNeeded uint64 `json:"space_needed"`
	// SpaceShortage : The amount of extra space needed to add the item.
	SpaceShortage uint64 `json:"space_shortage"`
	// SpaceLeft : The amount of space left in the user's Dropbox, less than
	// space_needed.
	SpaceLeft uint64 `json:"space_left"`
}

// NewInsufficientQuotaAmounts returns a new InsufficientQuotaAmounts instance
func NewInsufficientQuotaAmounts(SpaceNeeded uint64, SpaceShortage uint64, SpaceLeft uint64) *InsufficientQuotaAmounts {
	s := new(InsufficientQuotaAmounts)
	s.SpaceNeeded = SpaceNeeded
	s.SpaceShortage = SpaceShortage
	s.SpaceLeft = SpaceLeft
	return s
}

// InviteeInfo : Information about the recipient of a shared content invitation.
type InviteeInfo struct {
	dropbox.Tagged
	// Email : E-mail address of invited user.
	Email string `json:"email,omitempty"`
}

// Valid tag values for InviteeInfo
const (
	InviteeInfoEmail = "email"
	InviteeInfoOther = "other"
)

// UnmarshalJSON deserializes into a InviteeInfo instance
func (u *InviteeInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Email : E-mail address of invited user.
		Email string `json:"email,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "email":
		u.Email = w.Email

		if err != nil {
			return err
		}
	}
	return nil
}

// InviteeMembershipInfo : Information about an invited member of a shared
// content.
type InviteeMembershipInfo struct {
	MembershipInfo
	// Invitee : Recipient of the invitation.
	Invitee *InviteeInfo `json:"invitee"`
	// User : The user this invitation is tied to, if available.
	User *UserInfo `json:"user,omitempty"`
}

// NewInviteeMembershipInfo returns a new InviteeMembershipInfo instance
func NewInviteeMembershipInfo(AccessType *AccessLevel, Invitee *InviteeInfo) *InviteeMembershipInfo {
	s := new(InviteeMembershipInfo)
	s.AccessType = AccessType
	s.Invitee = Invitee
	s.IsInherited = false
	return s
}

// JobError : Error occurred while performing an asynchronous job from
// `unshareFolder` or `removeFolderMember`.
type JobError struct {
	dropbox.Tagged
	// UnshareFolderError : Error occurred while performing `unshareFolder`
	// action.
	UnshareFolderError *UnshareFolderError `json:"unshare_folder_error,omitempty"`
	// RemoveFolderMemberError : Error occurred while performing
	// `removeFolderMember` action.
	RemoveFolderMemberError *RemoveFolderMemberError `json:"remove_folder_member_error,omitempty"`
	// RelinquishFolderMembershipError : Error occurred while performing
	// `relinquishFolderMembership` action.
	RelinquishFolderMembershipError *RelinquishFolderMembershipError `json:"relinquish_folder_membership_error,omitempty"`
}

// Valid tag values for JobError
const (
	JobErrorUnshareFolderError              = "unshare_folder_error"
	JobErrorRemoveFolderMemberError         = "remove_folder_member_error"
	JobErrorRelinquishFolderMembershipError = "relinquish_folder_membership_error"
	JobErrorOther                           = "other"
)

// UnmarshalJSON deserializes into a JobError instance
func (u *JobError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UnshareFolderError : Error occurred while performing `unshareFolder`
		// action.
		UnshareFolderError *UnshareFolderError `json:"unshare_folder_error,omitempty"`
		// RemoveFolderMemberError : Error occurred while performing
		// `removeFolderMember` action.
		RemoveFolderMemberError *RemoveFolderMemberError `json:"remove_folder_member_error,omitempty"`
		// RelinquishFolderMembershipError : Error occurred while performing
		// `relinquishFolderMembership` action.
		RelinquishFolderMembershipError *RelinquishFolderMembershipError `json:"relinquish_folder_membership_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "unshare_folder_error":
		u.UnshareFolderError = w.UnshareFolderError

		if err != nil {
			return err
		}
	case "remove_folder_member_error":
		u.RemoveFolderMemberError = w.RemoveFolderMemberError

		if err != nil {
			return err
		}
	case "relinquish_folder_membership_error":
		u.RelinquishFolderMembershipError = w.RelinquishFolderMembershipError

		if err != nil {
			return err
		}
	}
	return nil
}

// JobStatus : has no documentation (yet)
type JobStatus struct {
	dropbox.Tagged
	// Failed : The asynchronous job returned an error.
	Failed *JobError `json:"failed,omitempty"`
}

// Valid tag values for JobStatus
const (
	JobStatusInProgress = "in_progress"
	JobStatusComplete   = "complete"
	JobStatusFailed     = "failed"
)

// UnmarshalJSON deserializes into a JobStatus instance
func (u *JobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : The asynchronous job returned an error.
		Failed *JobError `json:"failed,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "failed":
		u.Failed = w.Failed

		if err != nil {
			return err
		}
	}
	return nil
}

// LinkAccessLevel : has no documentation (yet)
type LinkAccessLevel struct {
	dropbox.Tagged
}

// Valid tag values for LinkAccessLevel
const (
	LinkAccessLevelViewer = "viewer"
	LinkAccessLevelEditor = "editor"
	LinkAccessLevelOther  = "other"
)

// LinkAction : Actions that can be performed on a link.
type LinkAction struct {
	dropbox.Tagged
}

// Valid tag values for LinkAction
const (
	LinkActionChangeAccessLevel = "change_access_level"
	LinkActionChangeAudience    = "change_audience"
	LinkActionRemoveExpiry      = "remove_expiry"
	LinkActionRemovePassword    = "remove_password"
	LinkActionSetExpiry         = "set_expiry"
	LinkActionSetPassword       = "set_password"
	LinkActionOther             = "other"
)

// LinkAudience : has no documentation (yet)
type LinkAudience struct {
	dropbox.Tagged
}

// Valid tag values for LinkAudience
const (
	LinkAudiencePublic   = "public"
	LinkAudienceTeam     = "team"
	LinkAudienceNoOne    = "no_one"
	LinkAudiencePassword = "password"
	LinkAudienceMembers  = "members"
	LinkAudienceOther    = "other"
)

// LinkExpiry : has no documentation (yet)
type LinkExpiry struct {
	dropbox.Tagged
	// SetExpiry : Set a new expiry or change an existing expiry.
	SetExpiry time.Time `json:"set_expiry,omitempty"`
}

// Valid tag values for LinkExpiry
const (
	LinkExpiryRemoveExpiry = "remove_expiry"
	LinkExpirySetExpiry    = "set_expiry"
	LinkExpiryOther        = "other"
)

// UnmarshalJSON deserializes into a LinkExpiry instance
func (u *LinkExpiry) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// SetExpiry : Set a new expiry or change an existing expiry.
		SetExpiry time.Time `json:"set_expiry,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "set_expiry":
		u.SetExpiry = w.SetExpiry

		if err != nil {
			return err
		}
	}
	return nil
}

// LinkPassword : has no documentation (yet)
type LinkPassword struct {
	dropbox.Tagged
	// SetPassword : Set a new password or change an existing password.
	SetPassword string `json:"set_password,omitempty"`
}

// Valid tag values for LinkPassword
const (
	LinkPasswordRemovePassword = "remove_password"
	LinkPasswordSetPassword    = "set_password"
	LinkPasswordOther          = "other"
)

// UnmarshalJSON deserializes into a LinkPassword instance
func (u *LinkPassword) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// SetPassword : Set a new password or change an existing password.
		SetPassword string `json:"set_password,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "set_password":
		u.SetPassword = w.SetPassword

		if err != nil {
			return err
		}
	}
	return nil
}

// LinkPermission : Permissions for actions that can be performed on a link.
type LinkPermission struct {
	// Action : has no documentation (yet)
	Action *LinkAction `json:"action"`
	// Allow : has no documentation (yet)
	Allow bool `json:"allow"`
	// Reason : has no documentation (yet)
	Reason *PermissionDeniedReason `json:"reason,omitempty"`
}

// NewLinkPermission returns a new LinkPermission instance
func NewLinkPermission(Action *LinkAction, Allow bool) *LinkPermission {
	s := new(LinkPermission)
	s.Action = Action
	s.Allow = Allow
	return s
}

// LinkPermissions : has no documentation (yet)
type LinkPermissions struct {
	// ResolvedVisibility : The current visibility of the link after considering
	// the shared links policies of the the team (in case the link's owner is
	// part of a team) and the shared folder (in case the linked file is part of
	// a shared folder). This field is shown only if the caller has access to
	// this info (the link's owner always has access to this data). For some
	// links, an effective_audience value is returned instead.
	ResolvedVisibility *ResolvedVisibility `json:"resolved_visibility,omitempty"`
	// RequestedVisibility : The shared link's requested visibility. This can be
	// overridden by the team and shared folder policies. The final visibility,
	// after considering these policies, can be found in `resolved_visibility`.
	// This is shown only if the caller is the link's owner and
	// resolved_visibility is returned instead of effective_audience.
	RequestedVisibility *RequestedVisibility `json:"requested_visibility,omitempty"`
	// CanRevoke : Whether the caller can revoke the shared link.
	CanRevoke bool `json:"can_revoke"`
	// RevokeFailureReason : The failure reason for revoking the link. This
	// field will only be present if the `can_revoke` is false.
	RevokeFailureReason *SharedLinkAccessFailureReason `json:"revoke_failure_reason,omitempty"`
	// EffectiveAudience : The type of audience who can benefit from the access
	// level specified by the `link_access_level` field.
	EffectiveAudience *LinkAudience `json:"effective_audience,omitempty"`
	// LinkAccessLevel : The access level that the link will grant to its users.
	// A link can grant additional rights to a user beyond their current access
	// level. For example, if a user was invited as a viewer to a file, and then
	// opens a link with `link_access_level` set to `editor`, then they will
	// gain editor privileges. The `link_access_level` is a property of the
	// link, and does not depend on who is calling this API. In particular,
	// `link_access_level` does not take into account the API caller's current
	// permissions to the content.
	LinkAccessLevel *LinkAccessLevel `json:"link_access_level,omitempty"`
}

// NewLinkPermissions returns a new LinkPermissions instance
func NewLinkPermissions(CanRevoke bool) *LinkPermissions {
	s := new(LinkPermissions)
	s.CanRevoke = CanRevoke
	return s
}

// LinkSettings : Settings that apply to a link.
type LinkSettings struct {
	// AccessLevel : The access level on the link for this file. Currently, it
	// only accepts 'viewer' and 'viewer_no_comment'.
	AccessLevel *AccessLevel `json:"access_level,omitempty"`
	// Audience : The type of audience on the link for this file.
	Audience *LinkAudience `json:"audience,omitempty"`
	// Expiry : An expiry timestamp to set on a link.
	Expiry *LinkExpiry `json:"expiry,omitempty"`
	// Password : The password for the link.
	Password *LinkPassword `json:"password,omitempty"`
}

// NewLinkSettings returns a new LinkSettings instance
func NewLinkSettings() *LinkSettings {
	s := new(LinkSettings)
	return s
}

// ListFileMembersArg : Arguments for `listFileMembers`.
type ListFileMembersArg struct {
	// File : The file for which you want to see members.
	File string `json:"file"`
	// Actions : The actions for which to return permissions on a member.
	Actions []*MemberAction `json:"actions,omitempty"`
	// IncludeInherited : Whether to include members who only have access from a
	// parent shared folder.
	IncludeInherited bool `json:"include_inherited"`
	// Limit : Number of members to return max per query. Defaults to 100 if no
	// limit is specified.
	Limit uint32 `json:"limit"`
}

// NewListFileMembersArg returns a new ListFileMembersArg instance
func NewListFileMembersArg(File string) *ListFileMembersArg {
	s := new(ListFileMembersArg)
	s.File = File
	s.IncludeInherited = true
	s.Limit = 100
	return s
}

// ListFileMembersBatchArg : Arguments for `listFileMembersBatch`.
type ListFileMembersBatchArg struct {
	// Files : Files for which to return members.
	Files []string `json:"files"`
	// Limit : Number of members to return max per query. Defaults to 10 if no
	// limit is specified.
	Limit uint32 `json:"limit"`
}

// NewListFileMembersBatchArg returns a new ListFileMembersBatchArg instance
func NewListFileMembersBatchArg(Files []string) *ListFileMembersBatchArg {
	s := new(ListFileMembersBatchArg)
	s.Files = Files
	s.Limit = 10
	return s
}

// ListFileMembersBatchResult : Per-file result for `listFileMembersBatch`.
type ListFileMembersBatchResult struct {
	// File : This is the input file identifier, whether an ID or a path.
	File string `json:"file"`
	// Result : The result for this particular file.
	Result *ListFileMembersIndividualResult `json:"result"`
}

// NewListFileMembersBatchResult returns a new ListFileMembersBatchResult instance
func NewListFileMembersBatchResult(File string, Result *ListFileMembersIndividualResult) *ListFileMembersBatchResult {
	s := new(ListFileMembersBatchResult)
	s.File = File
	s.Result = Result
	return s
}

// ListFileMembersContinueArg : Arguments for `listFileMembersContinue`.
type ListFileMembersContinueArg struct {
	// Cursor : The cursor returned by your last call to `listFileMembers`,
	// `listFileMembersContinue`, or `listFileMembersBatch`.
	Cursor string `json:"cursor"`
}

// NewListFileMembersContinueArg returns a new ListFileMembersContinueArg instance
func NewListFileMembersContinueArg(Cursor string) *ListFileMembersContinueArg {
	s := new(ListFileMembersContinueArg)
	s.Cursor = Cursor
	return s
}

// ListFileMembersContinueError : Error for `listFileMembersContinue`.
type ListFileMembersContinueError struct {
	dropbox.Tagged
	// UserError : has no documentation (yet)
	UserError *SharingUserError `json:"user_error,omitempty"`
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for ListFileMembersContinueError
const (
	ListFileMembersContinueErrorUserError     = "user_error"
	ListFileMembersContinueErrorAccessError   = "access_error"
	ListFileMembersContinueErrorInvalidCursor = "invalid_cursor"
	ListFileMembersContinueErrorOther         = "other"
)

// UnmarshalJSON deserializes into a ListFileMembersContinueError instance
func (u *ListFileMembersContinueError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : has no documentation (yet)
		UserError *SharingUserError `json:"user_error,omitempty"`
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// ListFileMembersCountResult : has no documentation (yet)
type ListFileMembersCountResult struct {
	// Members : A list of members on this file.
	Members *SharedFileMembers `json:"members"`
	// MemberCount : The number of members on this file. This does not include
	// inherited members.
	MemberCount uint32 `json:"member_count"`
}

// NewListFileMembersCountResult returns a new ListFileMembersCountResult instance
func NewListFileMembersCountResult(Members *SharedFileMembers, MemberCount uint32) *ListFileMembersCountResult {
	s := new(ListFileMembersCountResult)
	s.Members = Members
	s.MemberCount = MemberCount
	return s
}

// ListFileMembersError : Error for `listFileMembers`.
type ListFileMembersError struct {
	dropbox.Tagged
	// UserError : has no documentation (yet)
	UserError *SharingUserError `json:"user_error,omitempty"`
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for ListFileMembersError
const (
	ListFileMembersErrorUserError   = "user_error"
	ListFileMembersErrorAccessError = "access_error"
	ListFileMembersErrorOther       = "other"
)

// UnmarshalJSON deserializes into a ListFileMembersError instance
func (u *ListFileMembersError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : has no documentation (yet)
		UserError *SharingUserError `json:"user_error,omitempty"`
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// ListFileMembersIndividualResult : has no documentation (yet)
type ListFileMembersIndividualResult struct {
	dropbox.Tagged
	// Result : The results of the query for this file if it was successful.
	Result *ListFileMembersCountResult `json:"result,omitempty"`
	// AccessError : The result of the query for this file if it was an error.
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for ListFileMembersIndividualResult
const (
	ListFileMembersIndividualResultResult      = "result"
	ListFileMembersIndividualResultAccessError = "access_error"
	ListFileMembersIndividualResultOther       = "other"
)

// UnmarshalJSON deserializes into a ListFileMembersIndividualResult instance
func (u *ListFileMembersIndividualResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : The result of the query for this file if it was an
		// error.
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "result":
		err = json.Unmarshal(body, &u.Result)

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// ListFilesArg : Arguments for `listReceivedFiles`.
type ListFilesArg struct {
	// Limit : Number of files to return max per query. Defaults to 100 if no
	// limit is specified.
	Limit uint32 `json:"limit"`
	// Actions : A list of `FileAction`s corresponding to `FilePermission`s that
	// should appear in the  response's `SharedFileMetadata.permissions` field
	// describing the actions the  authenticated user can perform on the file.
	Actions []*FileAction `json:"actions,omitempty"`
}

// NewListFilesArg returns a new ListFilesArg instance
func NewListFilesArg() *ListFilesArg {
	s := new(ListFilesArg)
	s.Limit = 100
	return s
}

// ListFilesContinueArg : Arguments for `listReceivedFilesContinue`.
type ListFilesContinueArg struct {
	// Cursor : Cursor in `ListFilesResult.cursor`.
	Cursor string `json:"cursor"`
}

// NewListFilesContinueArg returns a new ListFilesContinueArg instance
func NewListFilesContinueArg(Cursor string) *ListFilesContinueArg {
	s := new(ListFilesContinueArg)
	s.Cursor = Cursor
	return s
}

// ListFilesContinueError : Error results for `listReceivedFilesContinue`.
type ListFilesContinueError struct {
	dropbox.Tagged
	// UserError : User account had a problem.
	UserError *SharingUserError `json:"user_error,omitempty"`
}

// Valid tag values for ListFilesContinueError
const (
	ListFilesContinueErrorUserError     = "user_error"
	ListFilesContinueErrorInvalidCursor = "invalid_cursor"
	ListFilesContinueErrorOther         = "other"
)

// UnmarshalJSON deserializes into a ListFilesContinueError instance
func (u *ListFilesContinueError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : User account had a problem.
		UserError *SharingUserError `json:"user_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	}
	return nil
}

// ListFilesResult : Success results for `listReceivedFiles`.
type ListFilesResult struct {
	// Entries : Information about the files shared with current user.
	Entries []*SharedFileMetadata `json:"entries"`
	// Cursor : Cursor used to obtain additional shared files.
	Cursor string `json:"cursor,omitempty"`
}

// NewListFilesResult returns a new ListFilesResult instance
func NewListFilesResult(Entries []*SharedFileMetadata) *ListFilesResult {
	s := new(ListFilesResult)
	s.Entries = Entries
	return s
}

// ListFolderMembersCursorArg : has no documentation (yet)
type ListFolderMembersCursorArg struct {
	// Actions : This is a list indicating whether each returned member will
	// include a boolean value `MemberPermission.allow` that describes whether
	// the current user can perform the MemberAction on the member.
	Actions []*MemberAction `json:"actions,omitempty"`
	// Limit : The maximum number of results that include members, groups and
	// invitees to return per request.
	Limit uint32 `json:"limit"`
}

// NewListFolderMembersCursorArg returns a new ListFolderMembersCursorArg instance
func NewListFolderMembersCursorArg() *ListFolderMembersCursorArg {
	s := new(ListFolderMembersCursorArg)
	s.Limit = 1000
	return s
}

// ListFolderMembersArgs : has no documentation (yet)
type ListFolderMembersArgs struct {
	ListFolderMembersCursorArg
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
}

// NewListFolderMembersArgs returns a new ListFolderMembersArgs instance
func NewListFolderMembersArgs(SharedFolderId string) *ListFolderMembersArgs {
	s := new(ListFolderMembersArgs)
	s.SharedFolderId = SharedFolderId
	s.Limit = 1000
	return s
}

// ListFolderMembersContinueArg : has no documentation (yet)
type ListFolderMembersContinueArg struct {
	// Cursor : The cursor returned by your last call to `listFolderMembers` or
	// `listFolderMembersContinue`.
	Cursor string `json:"cursor"`
}

// NewListFolderMembersContinueArg returns a new ListFolderMembersContinueArg instance
func NewListFolderMembersContinueArg(Cursor string) *ListFolderMembersContinueArg {
	s := new(ListFolderMembersContinueArg)
	s.Cursor = Cursor
	return s
}

// ListFolderMembersContinueError : has no documentation (yet)
type ListFolderMembersContinueError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for ListFolderMembersContinueError
const (
	ListFolderMembersContinueErrorAccessError   = "access_error"
	ListFolderMembersContinueErrorInvalidCursor = "invalid_cursor"
	ListFolderMembersContinueErrorOther         = "other"
)

// UnmarshalJSON deserializes into a ListFolderMembersContinueError instance
func (u *ListFolderMembersContinueError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// ListFoldersArgs : has no documentation (yet)
type ListFoldersArgs struct {
	// Limit : The maximum number of results to return per request.
	Limit uint32 `json:"limit"`
	// Actions : A list of `FolderAction`s corresponding to `FolderPermission`s
	// that should appear in the  response's `SharedFolderMetadata.permissions`
	// field describing the actions the  authenticated user can perform on the
	// folder.
	Actions []*FolderAction `json:"actions,omitempty"`
}

// NewListFoldersArgs returns a new ListFoldersArgs instance
func NewListFoldersArgs() *ListFoldersArgs {
	s := new(ListFoldersArgs)
	s.Limit = 1000
	return s
}

// ListFoldersContinueArg : has no documentation (yet)
type ListFoldersContinueArg struct {
	// Cursor : The cursor returned by the previous API call specified in the
	// endpoint description.
	Cursor string `json:"cursor"`
}

// NewListFoldersContinueArg returns a new ListFoldersContinueArg instance
func NewListFoldersContinueArg(Cursor string) *ListFoldersContinueArg {
	s := new(ListFoldersContinueArg)
	s.Cursor = Cursor
	return s
}

// ListFoldersContinueError : has no documentation (yet)
type ListFoldersContinueError struct {
	dropbox.Tagged
}

// Valid tag values for ListFoldersContinueError
const (
	ListFoldersContinueErrorInvalidCursor = "invalid_cursor"
	ListFoldersContinueErrorOther         = "other"
)

// ListFoldersResult : Result for `listFolders` or `listMountableFolders`,
// depending on which endpoint was requested. Unmounted shared folders can be
// identified by the absence of `SharedFolderMetadata.path_lower`.
type ListFoldersResult struct {
	// Entries : List of all shared folders the authenticated user has access
	// to.
	Entries []*SharedFolderMetadata `json:"entries"`
	// Cursor : Present if there are additional shared folders that have not
	// been returned yet. Pass the cursor into the corresponding continue
	// endpoint (either `listFoldersContinue` or `listMountableFoldersContinue`)
	// to list additional folders.
	Cursor string `json:"cursor,omitempty"`
}

// NewListFoldersResult returns a new ListFoldersResult instance
func NewListFoldersResult(Entries []*SharedFolderMetadata) *ListFoldersResult {
	s := new(ListFoldersResult)
	s.Entries = Entries
	return s
}

// ListSharedLinksArg : has no documentation (yet)
type ListSharedLinksArg struct {
	// Path : See `listSharedLinks` description.
	Path string `json:"path,omitempty"`
	// Cursor : The cursor returned by your last call to `listSharedLinks`.
	Cursor string `json:"cursor,omitempty"`
	// DirectOnly : See `listSharedLinks` description.
	DirectOnly bool `json:"direct_only,omitempty"`
}

// NewListSharedLinksArg returns a new ListSharedLinksArg instance
func NewListSharedLinksArg() *ListSharedLinksArg {
	s := new(ListSharedLinksArg)
	return s
}

// ListSharedLinksError : has no documentation (yet)
type ListSharedLinksError struct {
	dropbox.Tagged
	// Path : has no documentation (yet)
	Path *files.LookupError `json:"path,omitempty"`
}

// Valid tag values for ListSharedLinksError
const (
	ListSharedLinksErrorPath  = "path"
	ListSharedLinksErrorReset = "reset"
	ListSharedLinksErrorOther = "other"
)

// UnmarshalJSON deserializes into a ListSharedLinksError instance
func (u *ListSharedLinksError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Path : has no documentation (yet)
		Path *files.LookupError `json:"path,omitempty"`
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

		if err != nil {
			return err
		}
	}
	return nil
}

// ListSharedLinksResult : has no documentation (yet)
type ListSharedLinksResult struct {
	// Links : Shared links applicable to the path argument.
	Links []IsSharedLinkMetadata `json:"links"`
	// HasMore : Is true if there are additional shared links that have not been
	// returned yet. Pass the cursor into `listSharedLinks` to retrieve them.
	HasMore bool `json:"has_more"`
	// Cursor : Pass the cursor into `listSharedLinks` to obtain the additional
	// links. Cursor is returned only if no path is given.
	Cursor string `json:"cursor,omitempty"`
}

// NewListSharedLinksResult returns a new ListSharedLinksResult instance
func NewListSharedLinksResult(Links []IsSharedLinkMetadata, HasMore bool) *ListSharedLinksResult {
	s := new(ListSharedLinksResult)
	s.Links = Links
	s.HasMore = HasMore
	return s
}

// UnmarshalJSON deserializes into a ListSharedLinksResult instance
func (u *ListSharedLinksResult) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// Links : Shared links applicable to the path argument.
		Links []json.RawMessage `json:"links"`
		// HasMore : Is true if there are additional shared links that have not
		// been returned yet. Pass the cursor into `listSharedLinks` to retrieve
		// them.
		HasMore bool `json:"has_more"`
		// Cursor : Pass the cursor into `listSharedLinks` to obtain the
		// additional links. Cursor is returned only if no path is given.
		Cursor string `json:"cursor,omitempty"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	u.Links = make([]IsSharedLinkMetadata, len(w.Links))
	for i, e := range w.Links {
		v, err := IsSharedLinkMetadataFromJSON(e)
		if err != nil {
			return err
		}
		u.Links[i] = v
	}
	u.HasMore = w.HasMore
	u.Cursor = w.Cursor
	return nil
}

// MemberAccessLevelResult : Contains information about a member's access level
// to content after an operation.
type MemberAccessLevelResult struct {
	// AccessLevel : The member still has this level of access to the content
	// through a parent folder.
	AccessLevel *AccessLevel `json:"access_level,omitempty"`
	// Warning : A localized string with additional information about why the
	// user has this access level to the content.
	Warning string `json:"warning,omitempty"`
	// AccessDetails : The parent folders that a member has access to. The field
	// is present if the user has access to the first parent folder where the
	// member gains access.
	AccessDetails []*ParentFolderAccessInfo `json:"access_details,omitempty"`
}

// NewMemberAccessLevelResult returns a new MemberAccessLevelResult instance
func NewMemberAccessLevelResult() *MemberAccessLevelResult {
	s := new(MemberAccessLevelResult)
	return s
}

// MemberAction : Actions that may be taken on members of a shared folder.
type MemberAction struct {
	dropbox.Tagged
}

// Valid tag values for MemberAction
const (
	MemberActionLeaveACopy          = "leave_a_copy"
	MemberActionMakeEditor          = "make_editor"
	MemberActionMakeOwner           = "make_owner"
	MemberActionMakeViewer          = "make_viewer"
	MemberActionMakeViewerNoComment = "make_viewer_no_comment"
	MemberActionRemove              = "remove"
	MemberActionOther               = "other"
)

// MemberPermission : Whether the user is allowed to take the action on the
// associated member.
type MemberPermission struct {
	// Action : The action that the user may wish to take on the member.
	Action *MemberAction `json:"action"`
	// Allow : True if the user is allowed to take the action.
	Allow bool `json:"allow"`
	// Reason : The reason why the user is denied the permission. Not present if
	// the action is allowed.
	Reason *PermissionDeniedReason `json:"reason,omitempty"`
}

// NewMemberPermission returns a new MemberPermission instance
func NewMemberPermission(Action *MemberAction, Allow bool) *MemberPermission {
	s := new(MemberPermission)
	s.Action = Action
	s.Allow = Allow
	return s
}

// MemberPolicy : Policy governing who can be a member of a shared folder. Only
// applicable to folders owned by a user on a team.
type MemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for MemberPolicy
const (
	MemberPolicyTeam   = "team"
	MemberPolicyAnyone = "anyone"
	MemberPolicyOther  = "other"
)

// MemberSelector : Includes different ways to identify a member of a shared
// folder.
type MemberSelector struct {
	dropbox.Tagged
	// DropboxId : Dropbox account, team member, or group ID of member.
	DropboxId string `json:"dropbox_id,omitempty"`
	// Email : E-mail address of member.
	Email string `json:"email,omitempty"`
}

// Valid tag values for MemberSelector
const (
	MemberSelectorDropboxId = "dropbox_id"
	MemberSelectorEmail     = "email"
	MemberSelectorOther     = "other"
)

// UnmarshalJSON deserializes into a MemberSelector instance
func (u *MemberSelector) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// DropboxId : Dropbox account, team member, or group ID of member.
		DropboxId string `json:"dropbox_id,omitempty"`
		// Email : E-mail address of member.
		Email string `json:"email,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "dropbox_id":
		u.DropboxId = w.DropboxId

		if err != nil {
			return err
		}
	case "email":
		u.Email = w.Email

		if err != nil {
			return err
		}
	}
	return nil
}

// ModifySharedLinkSettingsArgs : has no documentation (yet)
type ModifySharedLinkSettingsArgs struct {
	// Url : URL of the shared link to change its settings.
	Url string `json:"url"`
	// Settings : Set of settings for the shared link.
	Settings *SharedLinkSettings `json:"settings"`
	// RemoveExpiration : If set to true, removes the expiration of the shared
	// link.
	RemoveExpiration bool `json:"remove_expiration"`
}

// NewModifySharedLinkSettingsArgs returns a new ModifySharedLinkSettingsArgs instance
func NewModifySharedLinkSettingsArgs(Url string, Settings *SharedLinkSettings) *ModifySharedLinkSettingsArgs {
	s := new(ModifySharedLinkSettingsArgs)
	s.Url = Url
	s.Settings = Settings
	s.RemoveExpiration = false
	return s
}

// ModifySharedLinkSettingsError : has no documentation (yet)
type ModifySharedLinkSettingsError struct {
	dropbox.Tagged
	// SettingsError : There is an error with the given settings.
	SettingsError *SharedLinkSettingsError `json:"settings_error,omitempty"`
}

// Valid tag values for ModifySharedLinkSettingsError
const (
	ModifySharedLinkSettingsErrorSharedLinkNotFound     = "shared_link_not_found"
	ModifySharedLinkSettingsErrorSharedLinkAccessDenied = "shared_link_access_denied"
	ModifySharedLinkSettingsErrorUnsupportedLinkType    = "unsupported_link_type"
	ModifySharedLinkSettingsErrorOther                  = "other"
	ModifySharedLinkSettingsErrorSettingsError          = "settings_error"
	ModifySharedLinkSettingsErrorEmailNotVerified       = "email_not_verified"
)

// UnmarshalJSON deserializes into a ModifySharedLinkSettingsError instance
func (u *ModifySharedLinkSettingsError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// SettingsError : There is an error with the given settings.
		SettingsError *SharedLinkSettingsError `json:"settings_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "settings_error":
		u.SettingsError = w.SettingsError

		if err != nil {
			return err
		}
	}
	return nil
}

// MountFolderArg : has no documentation (yet)
type MountFolderArg struct {
	// SharedFolderId : The ID of the shared folder to mount.
	SharedFolderId string `json:"shared_folder_id"`
}

// NewMountFolderArg returns a new MountFolderArg instance
func NewMountFolderArg(SharedFolderId string) *MountFolderArg {
	s := new(MountFolderArg)
	s.SharedFolderId = SharedFolderId
	return s
}

// MountFolderError : has no documentation (yet)
type MountFolderError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	// InsufficientQuota : The current user does not have enough space to mount
	// the shared folder.
	InsufficientQuota *InsufficientQuotaAmounts `json:"insufficient_quota,omitempty"`
}

// Valid tag values for MountFolderError
const (
	MountFolderErrorAccessError        = "access_error"
	MountFolderErrorInsideSharedFolder = "inside_shared_folder"
	MountFolderErrorInsufficientQuota  = "insufficient_quota"
	MountFolderErrorAlreadyMounted     = "already_mounted"
	MountFolderErrorNoPermission       = "no_permission"
	MountFolderErrorNotMountable       = "not_mountable"
	MountFolderErrorOther              = "other"
)

// UnmarshalJSON deserializes into a MountFolderError instance
func (u *MountFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	case "insufficient_quota":
		err = json.Unmarshal(body, &u.InsufficientQuota)

		if err != nil {
			return err
		}
	}
	return nil
}

// ParentFolderAccessInfo : Contains information about a parent folder that a
// member has access to.
type ParentFolderAccessInfo struct {
	// FolderName : Display name for the folder.
	FolderName string `json:"folder_name"`
	// SharedFolderId : The identifier of the parent shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// Permissions : The user's permissions for the parent shared folder.
	Permissions []*MemberPermission `json:"permissions"`
	// Path : The full path to the parent shared folder relative to the acting
	// user's root.
	Path string `json:"path"`
}

// NewParentFolderAccessInfo returns a new ParentFolderAccessInfo instance
func NewParentFolderAccessInfo(FolderName string, SharedFolderId string, Permissions []*MemberPermission, Path string) *ParentFolderAccessInfo {
	s := new(ParentFolderAccessInfo)
	s.FolderName = FolderName
	s.SharedFolderId = SharedFolderId
	s.Permissions = Permissions
	s.Path = Path
	return s
}

// PathLinkMetadata : Metadata for a path-based shared link.
type PathLinkMetadata struct {
	LinkMetadata
	// Path : Path in user's Dropbox.
	Path string `json:"path"`
}

// NewPathLinkMetadata returns a new PathLinkMetadata instance
func NewPathLinkMetadata(Url string, Visibility *Visibility, Path string) *PathLinkMetadata {
	s := new(PathLinkMetadata)
	s.Url = Url
	s.Visibility = Visibility
	s.Path = Path
	return s
}

// PendingUploadMode : Flag to indicate pending upload default (for linking to
// not-yet-existing paths).
type PendingUploadMode struct {
	dropbox.Tagged
}

// Valid tag values for PendingUploadMode
const (
	PendingUploadModeFile   = "file"
	PendingUploadModeFolder = "folder"
)

// PermissionDeniedReason : Possible reasons the user is denied a permission.
type PermissionDeniedReason struct {
	dropbox.Tagged
	// InsufficientPlan : has no documentation (yet)
	InsufficientPlan *InsufficientPlan `json:"insufficient_plan,omitempty"`
}

// Valid tag values for PermissionDeniedReason
const (
	PermissionDeniedReasonUserNotSameTeamAsOwner     = "user_not_same_team_as_owner"
	PermissionDeniedReasonUserNotAllowedByOwner      = "user_not_allowed_by_owner"
	PermissionDeniedReasonTargetIsIndirectMember     = "target_is_indirect_member"
	PermissionDeniedReasonTargetIsOwner              = "target_is_owner"
	PermissionDeniedReasonTargetIsSelf               = "target_is_self"
	PermissionDeniedReasonTargetNotActive            = "target_not_active"
	PermissionDeniedReasonFolderIsLimitedTeamFolder  = "folder_is_limited_team_folder"
	PermissionDeniedReasonOwnerNotOnTeam             = "owner_not_on_team"
	PermissionDeniedReasonPermissionDenied           = "permission_denied"
	PermissionDeniedReasonRestrictedByTeam           = "restricted_by_team"
	PermissionDeniedReasonUserAccountType            = "user_account_type"
	PermissionDeniedReasonUserNotOnTeam              = "user_not_on_team"
	PermissionDeniedReasonFolderIsInsideSharedFolder = "folder_is_inside_shared_folder"
	PermissionDeniedReasonRestrictedByParentFolder   = "restricted_by_parent_folder"
	PermissionDeniedReasonInsufficientPlan           = "insufficient_plan"
	PermissionDeniedReasonOther                      = "other"
)

// UnmarshalJSON deserializes into a PermissionDeniedReason instance
func (u *PermissionDeniedReason) UnmarshalJSON(body []byte) error {
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
	case "insufficient_plan":
		err = json.Unmarshal(body, &u.InsufficientPlan)

		if err != nil {
			return err
		}
	}
	return nil
}

// RelinquishFileMembershipArg : has no documentation (yet)
type RelinquishFileMembershipArg struct {
	// File : The path or id for the file.
	File string `json:"file"`
}

// NewRelinquishFileMembershipArg returns a new RelinquishFileMembershipArg instance
func NewRelinquishFileMembershipArg(File string) *RelinquishFileMembershipArg {
	s := new(RelinquishFileMembershipArg)
	s.File = File
	return s
}

// RelinquishFileMembershipError : has no documentation (yet)
type RelinquishFileMembershipError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for RelinquishFileMembershipError
const (
	RelinquishFileMembershipErrorAccessError  = "access_error"
	RelinquishFileMembershipErrorGroupAccess  = "group_access"
	RelinquishFileMembershipErrorNoPermission = "no_permission"
	RelinquishFileMembershipErrorOther        = "other"
)

// UnmarshalJSON deserializes into a RelinquishFileMembershipError instance
func (u *RelinquishFileMembershipError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// RelinquishFolderMembershipArg : has no documentation (yet)
type RelinquishFolderMembershipArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// LeaveACopy : Keep a copy of the folder's contents upon relinquishing
	// membership.
	LeaveACopy bool `json:"leave_a_copy"`
}

// NewRelinquishFolderMembershipArg returns a new RelinquishFolderMembershipArg instance
func NewRelinquishFolderMembershipArg(SharedFolderId string) *RelinquishFolderMembershipArg {
	s := new(RelinquishFolderMembershipArg)
	s.SharedFolderId = SharedFolderId
	s.LeaveACopy = false
	return s
}

// RelinquishFolderMembershipError : has no documentation (yet)
type RelinquishFolderMembershipError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for RelinquishFolderMembershipError
const (
	RelinquishFolderMembershipErrorAccessError      = "access_error"
	RelinquishFolderMembershipErrorFolderOwner      = "folder_owner"
	RelinquishFolderMembershipErrorMounted          = "mounted"
	RelinquishFolderMembershipErrorGroupAccess      = "group_access"
	RelinquishFolderMembershipErrorTeamFolder       = "team_folder"
	RelinquishFolderMembershipErrorNoPermission     = "no_permission"
	RelinquishFolderMembershipErrorNoExplicitAccess = "no_explicit_access"
	RelinquishFolderMembershipErrorOther            = "other"
)

// UnmarshalJSON deserializes into a RelinquishFolderMembershipError instance
func (u *RelinquishFolderMembershipError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveFileMemberArg : Arguments for `removeFileMember2`.
type RemoveFileMemberArg struct {
	// File : File from which to remove members.
	File string `json:"file"`
	// Member : Member to remove from this file. Note that even if an email is
	// specified, it may result in the removal of a user (not an invitee) if the
	// user's main account corresponds to that email address.
	Member *MemberSelector `json:"member"`
}

// NewRemoveFileMemberArg returns a new RemoveFileMemberArg instance
func NewRemoveFileMemberArg(File string, Member *MemberSelector) *RemoveFileMemberArg {
	s := new(RemoveFileMemberArg)
	s.File = File
	s.Member = Member
	return s
}

// RemoveFileMemberError : Errors for `removeFileMember2`.
type RemoveFileMemberError struct {
	dropbox.Tagged
	// UserError : has no documentation (yet)
	UserError *SharingUserError `json:"user_error,omitempty"`
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	// NoExplicitAccess : This member does not have explicit access to the file
	// and therefore cannot be removed. The return value is the access that a
	// user might have to the file from a parent folder.
	NoExplicitAccess *MemberAccessLevelResult `json:"no_explicit_access,omitempty"`
}

// Valid tag values for RemoveFileMemberError
const (
	RemoveFileMemberErrorUserError        = "user_error"
	RemoveFileMemberErrorAccessError      = "access_error"
	RemoveFileMemberErrorNoExplicitAccess = "no_explicit_access"
	RemoveFileMemberErrorOther            = "other"
)

// UnmarshalJSON deserializes into a RemoveFileMemberError instance
func (u *RemoveFileMemberError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : has no documentation (yet)
		UserError *SharingUserError `json:"user_error,omitempty"`
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	case "no_explicit_access":
		err = json.Unmarshal(body, &u.NoExplicitAccess)

		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveFolderMemberArg : has no documentation (yet)
type RemoveFolderMemberArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// Member : The member to remove from the folder.
	Member *MemberSelector `json:"member"`
	// LeaveACopy : If true, the removed user will keep their copy of the folder
	// after it's unshared, assuming it was mounted. Otherwise, it will be
	// removed from their Dropbox. Also, this must be set to false when kicking
	// a group.
	LeaveACopy bool `json:"leave_a_copy"`
}

// NewRemoveFolderMemberArg returns a new RemoveFolderMemberArg instance
func NewRemoveFolderMemberArg(SharedFolderId string, Member *MemberSelector, LeaveACopy bool) *RemoveFolderMemberArg {
	s := new(RemoveFolderMemberArg)
	s.SharedFolderId = SharedFolderId
	s.Member = Member
	s.LeaveACopy = LeaveACopy
	return s
}

// RemoveFolderMemberError : has no documentation (yet)
type RemoveFolderMemberError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	// MemberError : has no documentation (yet)
	MemberError *SharedFolderMemberError `json:"member_error,omitempty"`
}

// Valid tag values for RemoveFolderMemberError
const (
	RemoveFolderMemberErrorAccessError  = "access_error"
	RemoveFolderMemberErrorMemberError  = "member_error"
	RemoveFolderMemberErrorFolderOwner  = "folder_owner"
	RemoveFolderMemberErrorGroupAccess  = "group_access"
	RemoveFolderMemberErrorTeamFolder   = "team_folder"
	RemoveFolderMemberErrorNoPermission = "no_permission"
	RemoveFolderMemberErrorTooManyFiles = "too_many_files"
	RemoveFolderMemberErrorOther        = "other"
)

// UnmarshalJSON deserializes into a RemoveFolderMemberError instance
func (u *RemoveFolderMemberError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
		// MemberError : has no documentation (yet)
		MemberError *SharedFolderMemberError `json:"member_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	case "member_error":
		u.MemberError = w.MemberError

		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveMemberJobStatus : has no documentation (yet)
type RemoveMemberJobStatus struct {
	dropbox.Tagged
	// Complete : Removing the folder member has finished. The value is
	// information about whether the member has another form of access.
	Complete *MemberAccessLevelResult `json:"complete,omitempty"`
	// Failed : has no documentation (yet)
	Failed *RemoveFolderMemberError `json:"failed,omitempty"`
}

// Valid tag values for RemoveMemberJobStatus
const (
	RemoveMemberJobStatusInProgress = "in_progress"
	RemoveMemberJobStatusComplete   = "complete"
	RemoveMemberJobStatusFailed     = "failed"
)

// UnmarshalJSON deserializes into a RemoveMemberJobStatus instance
func (u *RemoveMemberJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : has no documentation (yet)
		Failed *RemoveFolderMemberError `json:"failed,omitempty"`
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
		u.Failed = w.Failed

		if err != nil {
			return err
		}
	}
	return nil
}

// RequestedLinkAccessLevel : has no documentation (yet)
type RequestedLinkAccessLevel struct {
	dropbox.Tagged
}

// Valid tag values for RequestedLinkAccessLevel
const (
	RequestedLinkAccessLevelViewer = "viewer"
	RequestedLinkAccessLevelEditor = "editor"
	RequestedLinkAccessLevelMax    = "max"
	RequestedLinkAccessLevelOther  = "other"
)

// RequestedVisibility : The access permission that can be requested by the
// caller for the shared link. Note that the final resolved visibility of the
// shared link takes into account other aspects, such as team and shared folder
// settings. Check the `ResolvedVisibility` for more info on the possible
// resolved visibility values of shared links.
type RequestedVisibility struct {
	dropbox.Tagged
}

// Valid tag values for RequestedVisibility
const (
	RequestedVisibilityPublic   = "public"
	RequestedVisibilityTeamOnly = "team_only"
	RequestedVisibilityPassword = "password"
)

// ResolvedVisibility : The actual access permissions values of shared links
// after taking into account user preferences and the team and shared folder
// settings. Check the `RequestedVisibility` for more info on the possible
// visibility values that can be set by the shared link's owner.
type ResolvedVisibility struct {
	dropbox.Tagged
}

// Valid tag values for ResolvedVisibility
const (
	ResolvedVisibilityPublic           = "public"
	ResolvedVisibilityTeamOnly         = "team_only"
	ResolvedVisibilityPassword         = "password"
	ResolvedVisibilityTeamAndPassword  = "team_and_password"
	ResolvedVisibilitySharedFolderOnly = "shared_folder_only"
	ResolvedVisibilityOther            = "other"
)

// RevokeSharedLinkArg : has no documentation (yet)
type RevokeSharedLinkArg struct {
	// Url : URL of the shared link.
	Url string `json:"url"`
}

// NewRevokeSharedLinkArg returns a new RevokeSharedLinkArg instance
func NewRevokeSharedLinkArg(Url string) *RevokeSharedLinkArg {
	s := new(RevokeSharedLinkArg)
	s.Url = Url
	return s
}

// RevokeSharedLinkError : has no documentation (yet)
type RevokeSharedLinkError struct {
	dropbox.Tagged
}

// Valid tag values for RevokeSharedLinkError
const (
	RevokeSharedLinkErrorSharedLinkNotFound     = "shared_link_not_found"
	RevokeSharedLinkErrorSharedLinkAccessDenied = "shared_link_access_denied"
	RevokeSharedLinkErrorUnsupportedLinkType    = "unsupported_link_type"
	RevokeSharedLinkErrorOther                  = "other"
	RevokeSharedLinkErrorSharedLinkMalformed    = "shared_link_malformed"
)

// SetAccessInheritanceArg : has no documentation (yet)
type SetAccessInheritanceArg struct {
	// AccessInheritance : The access inheritance settings for the folder.
	AccessInheritance *AccessInheritance `json:"access_inheritance"`
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
}

// NewSetAccessInheritanceArg returns a new SetAccessInheritanceArg instance
func NewSetAccessInheritanceArg(SharedFolderId string) *SetAccessInheritanceArg {
	s := new(SetAccessInheritanceArg)
	s.SharedFolderId = SharedFolderId
	s.AccessInheritance = &AccessInheritance{Tagged: dropbox.Tagged{"inherit"}}
	return s
}

// SetAccessInheritanceError : has no documentation (yet)
type SetAccessInheritanceError struct {
	dropbox.Tagged
	// AccessError : Unable to access shared folder.
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for SetAccessInheritanceError
const (
	SetAccessInheritanceErrorAccessError  = "access_error"
	SetAccessInheritanceErrorNoPermission = "no_permission"
	SetAccessInheritanceErrorOther        = "other"
)

// UnmarshalJSON deserializes into a SetAccessInheritanceError instance
func (u *SetAccessInheritanceError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : Unable to access shared folder.
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// ShareFolderArgBase : has no documentation (yet)
type ShareFolderArgBase struct {
	// AclUpdatePolicy : Who can add and remove members of this shared folder.
	AclUpdatePolicy *AclUpdatePolicy `json:"acl_update_policy,omitempty"`
	// ForceAsync : Whether to force the share to happen asynchronously.
	ForceAsync bool `json:"force_async"`
	// MemberPolicy : Who can be a member of this shared folder. Only applicable
	// if the current user is on a team.
	MemberPolicy *MemberPolicy `json:"member_policy,omitempty"`
	// Path : The path to the folder to share. If it does not exist, then a new
	// one is created.
	Path string `json:"path"`
	// SharedLinkPolicy : The policy to apply to shared links created for
	// content inside this shared folder.  The current user must be on a team to
	// set this policy to `SharedLinkPolicy.members`.
	SharedLinkPolicy *SharedLinkPolicy `json:"shared_link_policy,omitempty"`
	// ViewerInfoPolicy : Who can enable/disable viewer info for this shared
	// folder.
	ViewerInfoPolicy *ViewerInfoPolicy `json:"viewer_info_policy,omitempty"`
	// AccessInheritance : The access inheritance settings for the folder.
	AccessInheritance *AccessInheritance `json:"access_inheritance"`
}

// NewShareFolderArgBase returns a new ShareFolderArgBase instance
func NewShareFolderArgBase(Path string) *ShareFolderArgBase {
	s := new(ShareFolderArgBase)
	s.Path = Path
	s.ForceAsync = false
	s.AccessInheritance = &AccessInheritance{Tagged: dropbox.Tagged{"inherit"}}
	return s
}

// ShareFolderArg : has no documentation (yet)
type ShareFolderArg struct {
	ShareFolderArgBase
	// Actions : A list of `FolderAction`s corresponding to `FolderPermission`s
	// that should appear in the  response's `SharedFolderMetadata.permissions`
	// field describing the actions the  authenticated user can perform on the
	// folder.
	Actions []*FolderAction `json:"actions,omitempty"`
	// LinkSettings : Settings on the link for this folder.
	LinkSettings *LinkSettings `json:"link_settings,omitempty"`
}

// NewShareFolderArg returns a new ShareFolderArg instance
func NewShareFolderArg(Path string) *ShareFolderArg {
	s := new(ShareFolderArg)
	s.Path = Path
	s.ForceAsync = false
	s.AccessInheritance = &AccessInheritance{Tagged: dropbox.Tagged{"inherit"}}
	return s
}

// ShareFolderErrorBase : has no documentation (yet)
type ShareFolderErrorBase struct {
	dropbox.Tagged
	// BadPath : `ShareFolderArg.path` is invalid.
	BadPath *SharePathError `json:"bad_path,omitempty"`
}

// Valid tag values for ShareFolderErrorBase
const (
	ShareFolderErrorBaseEmailUnverified                 = "email_unverified"
	ShareFolderErrorBaseBadPath                         = "bad_path"
	ShareFolderErrorBaseTeamPolicyDisallowsMemberPolicy = "team_policy_disallows_member_policy"
	ShareFolderErrorBaseDisallowedSharedLinkPolicy      = "disallowed_shared_link_policy"
	ShareFolderErrorBaseOther                           = "other"
)

// UnmarshalJSON deserializes into a ShareFolderErrorBase instance
func (u *ShareFolderErrorBase) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// BadPath : `ShareFolderArg.path` is invalid.
		BadPath *SharePathError `json:"bad_path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "bad_path":
		u.BadPath = w.BadPath

		if err != nil {
			return err
		}
	}
	return nil
}

// ShareFolderError : has no documentation (yet)
type ShareFolderError struct {
	dropbox.Tagged
	// BadPath : `ShareFolderArg.path` is invalid.
	BadPath *SharePathError `json:"bad_path,omitempty"`
}

// Valid tag values for ShareFolderError
const (
	ShareFolderErrorEmailUnverified                 = "email_unverified"
	ShareFolderErrorBadPath                         = "bad_path"
	ShareFolderErrorTeamPolicyDisallowsMemberPolicy = "team_policy_disallows_member_policy"
	ShareFolderErrorDisallowedSharedLinkPolicy      = "disallowed_shared_link_policy"
	ShareFolderErrorOther                           = "other"
	ShareFolderErrorNoPermission                    = "no_permission"
)

// UnmarshalJSON deserializes into a ShareFolderError instance
func (u *ShareFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// BadPath : `ShareFolderArg.path` is invalid.
		BadPath *SharePathError `json:"bad_path,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "bad_path":
		u.BadPath = w.BadPath

		if err != nil {
			return err
		}
	}
	return nil
}

// ShareFolderJobStatus : has no documentation (yet)
type ShareFolderJobStatus struct {
	dropbox.Tagged
	// Complete : The share job has finished. The value is the metadata for the
	// folder.
	Complete *SharedFolderMetadata `json:"complete,omitempty"`
	// Failed : has no documentation (yet)
	Failed *ShareFolderError `json:"failed,omitempty"`
}

// Valid tag values for ShareFolderJobStatus
const (
	ShareFolderJobStatusInProgress = "in_progress"
	ShareFolderJobStatusComplete   = "complete"
	ShareFolderJobStatusFailed     = "failed"
)

// UnmarshalJSON deserializes into a ShareFolderJobStatus instance
func (u *ShareFolderJobStatus) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Failed : has no documentation (yet)
		Failed *ShareFolderError `json:"failed,omitempty"`
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
		u.Failed = w.Failed

		if err != nil {
			return err
		}
	}
	return nil
}

// ShareFolderLaunch : has no documentation (yet)
type ShareFolderLaunch struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
	// Complete : has no documentation (yet)
	Complete *SharedFolderMetadata `json:"complete,omitempty"`
}

// Valid tag values for ShareFolderLaunch
const (
	ShareFolderLaunchAsyncJobId = "async_job_id"
	ShareFolderLaunchComplete   = "complete"
)

// UnmarshalJSON deserializes into a ShareFolderLaunch instance
func (u *ShareFolderLaunch) UnmarshalJSON(body []byte) error {
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

		if err != nil {
			return err
		}
	case "complete":
		err = json.Unmarshal(body, &u.Complete)

		if err != nil {
			return err
		}
	}
	return nil
}

// SharePathError : has no documentation (yet)
type SharePathError struct {
	dropbox.Tagged
	// AlreadyShared : Folder is already shared. Contains metadata about the
	// existing shared folder.
	AlreadyShared *SharedFolderMetadata `json:"already_shared,omitempty"`
}

// Valid tag values for SharePathError
const (
	SharePathErrorIsFile               = "is_file"
	SharePathErrorInsideSharedFolder   = "inside_shared_folder"
	SharePathErrorContainsSharedFolder = "contains_shared_folder"
	SharePathErrorContainsAppFolder    = "contains_app_folder"
	SharePathErrorContainsTeamFolder   = "contains_team_folder"
	SharePathErrorIsAppFolder          = "is_app_folder"
	SharePathErrorInsideAppFolder      = "inside_app_folder"
	SharePathErrorIsPublicFolder       = "is_public_folder"
	SharePathErrorInsidePublicFolder   = "inside_public_folder"
	SharePathErrorAlreadyShared        = "already_shared"
	SharePathErrorInvalidPath          = "invalid_path"
	SharePathErrorIsOsxPackage         = "is_osx_package"
	SharePathErrorInsideOsxPackage     = "inside_osx_package"
	SharePathErrorOther                = "other"
)

// UnmarshalJSON deserializes into a SharePathError instance
func (u *SharePathError) UnmarshalJSON(body []byte) error {
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
	case "already_shared":
		err = json.Unmarshal(body, &u.AlreadyShared)

		if err != nil {
			return err
		}
	}
	return nil
}

// SharedContentLinkMetadata : Metadata of a shared link for a file or folder.
type SharedContentLinkMetadata struct {
	SharedContentLinkMetadataBase
	// AudienceExceptions : The content inside this folder with link audience
	// different than this folder's. This is only returned when an endpoint that
	// returns metadata for a single shared folder is called, e.g.
	// /get_folder_metadata.
	AudienceExceptions *AudienceExceptions `json:"audience_exceptions,omitempty"`
	// Url : The URL of the link.
	Url string `json:"url"`
}

// NewSharedContentLinkMetadata returns a new SharedContentLinkMetadata instance
func NewSharedContentLinkMetadata(AudienceOptions []*LinkAudience, CurrentAudience *LinkAudience, LinkPermissions []*LinkPermission, PasswordProtected bool, Url string) *SharedContentLinkMetadata {
	s := new(SharedContentLinkMetadata)
	s.AudienceOptions = AudienceOptions
	s.CurrentAudience = CurrentAudience
	s.LinkPermissions = LinkPermissions
	s.PasswordProtected = PasswordProtected
	s.Url = Url
	return s
}

// SharedFileMembers : Shared file user, group, and invitee membership. Used for
// the results of `listFileMembers` and `listFileMembersContinue`, and used as
// part of the results for `listFileMembersBatch`.
type SharedFileMembers struct {
	// Users : The list of user members of the shared file.
	Users []*UserFileMembershipInfo `json:"users"`
	// Groups : The list of group members of the shared file.
	Groups []*GroupMembershipInfo `json:"groups"`
	// Invitees : The list of invited members of a file, but have not logged in
	// and claimed this.
	Invitees []*InviteeMembershipInfo `json:"invitees"`
	// Cursor : Present if there are additional shared file members that have
	// not been returned yet. Pass the cursor into `listFileMembersContinue` to
	// list additional members.
	Cursor string `json:"cursor,omitempty"`
}

// NewSharedFileMembers returns a new SharedFileMembers instance
func NewSharedFileMembers(Users []*UserFileMembershipInfo, Groups []*GroupMembershipInfo, Invitees []*InviteeMembershipInfo) *SharedFileMembers {
	s := new(SharedFileMembers)
	s.Users = Users
	s.Groups = Groups
	s.Invitees = Invitees
	return s
}

// SharedFileMetadata : Properties of the shared file.
type SharedFileMetadata struct {
	// AccessType : The current user's access level for this shared file.
	AccessType *AccessLevel `json:"access_type,omitempty"`
	// Id : The ID of the file.
	Id string `json:"id"`
	// ExpectedLinkMetadata : The expected metadata of the link associated for
	// the file when it is first shared. Absent if the link already exists. This
	// is for an unreleased feature so it may not be returned yet.
	ExpectedLinkMetadata *ExpectedSharedContentLinkMetadata `json:"expected_link_metadata,omitempty"`
	// LinkMetadata : The metadata of the link associated for the file. This is
	// for an unreleased feature so it may not be returned yet.
	LinkMetadata *SharedContentLinkMetadata `json:"link_metadata,omitempty"`
	// Name : The name of this file.
	Name string `json:"name"`
	// OwnerDisplayNames : The display names of the users that own the file. If
	// the file is part of a team folder, the display names of the team admins
	// are also included. Absent if the owner display names cannot be fetched.
	OwnerDisplayNames []string `json:"owner_display_names,omitempty"`
	// OwnerTeam : The team that owns the file. This field is not present if the
	// file is not owned by a team.
	OwnerTeam *users.Team `json:"owner_team,omitempty"`
	// ParentSharedFolderId : The ID of the parent shared folder. This field is
	// present only if the file is contained within a shared folder.
	ParentSharedFolderId string `json:"parent_shared_folder_id,omitempty"`
	// PathDisplay : The cased path to be used for display purposes only. In
	// rare instances the casing will not correctly match the user's filesystem,
	// but this behavior will match the path provided in the Core API v1. Absent
	// for unmounted files.
	PathDisplay string `json:"path_display,omitempty"`
	// PathLower : The lower-case full path of this file. Absent for unmounted
	// files.
	PathLower string `json:"path_lower,omitempty"`
	// Permissions : The sharing permissions that requesting user has on this
	// file. This corresponds to the entries given in
	// `GetFileMetadataBatchArg.actions` or `GetFileMetadataArg.actions`.
	Permissions []*FilePermission `json:"permissions,omitempty"`
	// Policy : Policies governing this shared file.
	Policy *FolderPolicy `json:"policy"`
	// PreviewUrl : URL for displaying a web preview of the shared file.
	PreviewUrl string `json:"preview_url"`
	// TimeInvited : Timestamp indicating when the current user was invited to
	// this shared file. If the user was not invited to the shared file, the
	// timestamp will indicate when the user was invited to the parent shared
	// folder. This value may be absent.
	TimeInvited time.Time `json:"time_invited,omitempty"`
}

// NewSharedFileMetadata returns a new SharedFileMetadata instance
func NewSharedFileMetadata(Id string, Name string, Policy *FolderPolicy, PreviewUrl string) *SharedFileMetadata {
	s := new(SharedFileMetadata)
	s.Id = Id
	s.Name = Name
	s.Policy = Policy
	s.PreviewUrl = PreviewUrl
	return s
}

// SharedFolderAccessError : There is an error accessing the shared folder.
type SharedFolderAccessError struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderAccessError
const (
	SharedFolderAccessErrorInvalidId       = "invalid_id"
	SharedFolderAccessErrorNotAMember      = "not_a_member"
	SharedFolderAccessErrorEmailUnverified = "email_unverified"
	SharedFolderAccessErrorUnmounted       = "unmounted"
	SharedFolderAccessErrorOther           = "other"
)

// SharedFolderMemberError : has no documentation (yet)
type SharedFolderMemberError struct {
	dropbox.Tagged
	// NoExplicitAccess : The target member only has inherited access to the
	// shared folder.
	NoExplicitAccess *MemberAccessLevelResult `json:"no_explicit_access,omitempty"`
}

// Valid tag values for SharedFolderMemberError
const (
	SharedFolderMemberErrorInvalidDropboxId = "invalid_dropbox_id"
	SharedFolderMemberErrorNotAMember       = "not_a_member"
	SharedFolderMemberErrorNoExplicitAccess = "no_explicit_access"
	SharedFolderMemberErrorOther            = "other"
)

// UnmarshalJSON deserializes into a SharedFolderMemberError instance
func (u *SharedFolderMemberError) UnmarshalJSON(body []byte) error {
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
	case "no_explicit_access":
		err = json.Unmarshal(body, &u.NoExplicitAccess)

		if err != nil {
			return err
		}
	}
	return nil
}

// SharedFolderMembers : Shared folder user and group membership.
type SharedFolderMembers struct {
	// Users : The list of user members of the shared folder.
	Users []*UserMembershipInfo `json:"users"`
	// Groups : The list of group members of the shared folder.
	Groups []*GroupMembershipInfo `json:"groups"`
	// Invitees : The list of invitees to the shared folder.
	Invitees []*InviteeMembershipInfo `json:"invitees"`
	// Cursor : Present if there are additional shared folder members that have
	// not been returned yet. Pass the cursor into `listFolderMembersContinue`
	// to list additional members.
	Cursor string `json:"cursor,omitempty"`
}

// NewSharedFolderMembers returns a new SharedFolderMembers instance
func NewSharedFolderMembers(Users []*UserMembershipInfo, Groups []*GroupMembershipInfo, Invitees []*InviteeMembershipInfo) *SharedFolderMembers {
	s := new(SharedFolderMembers)
	s.Users = Users
	s.Groups = Groups
	s.Invitees = Invitees
	return s
}

// SharedFolderMetadataBase : Properties of the shared folder.
type SharedFolderMetadataBase struct {
	// AccessType : The current user's access level for this shared folder.
	AccessType *AccessLevel `json:"access_type"`
	// IsInsideTeamFolder : Whether this folder is inside of a team folder.
	IsInsideTeamFolder bool `json:"is_inside_team_folder"`
	// IsTeamFolder : Whether this folder is a `team folder`
	// <https://www.dropbox.com/en/help/986>.
	IsTeamFolder bool `json:"is_team_folder"`
	// OwnerDisplayNames : The display names of the users that own the folder.
	// If the folder is part of a team folder, the display names of the team
	// admins are also included. Absent if the owner display names cannot be
	// fetched.
	OwnerDisplayNames []string `json:"owner_display_names,omitempty"`
	// OwnerTeam : The team that owns the folder. This field is not present if
	// the folder is not owned by a team.
	OwnerTeam *users.Team `json:"owner_team,omitempty"`
	// ParentSharedFolderId : The ID of the parent shared folder. This field is
	// present only if the folder is contained within another shared folder.
	ParentSharedFolderId string `json:"parent_shared_folder_id,omitempty"`
	// PathLower : The lower-cased full path of this shared folder. Absent for
	// unmounted folders.
	PathLower string `json:"path_lower,omitempty"`
}

// NewSharedFolderMetadataBase returns a new SharedFolderMetadataBase instance
func NewSharedFolderMetadataBase(AccessType *AccessLevel, IsInsideTeamFolder bool, IsTeamFolder bool) *SharedFolderMetadataBase {
	s := new(SharedFolderMetadataBase)
	s.AccessType = AccessType
	s.IsInsideTeamFolder = IsInsideTeamFolder
	s.IsTeamFolder = IsTeamFolder
	return s
}

// SharedFolderMetadata : The metadata which includes basic information about
// the shared folder.
type SharedFolderMetadata struct {
	SharedFolderMetadataBase
	// LinkMetadata : The metadata of the shared content link to this shared
	// folder. Absent if there is no link on the folder. This is for an
	// unreleased feature so it may not be returned yet.
	LinkMetadata *SharedContentLinkMetadata `json:"link_metadata,omitempty"`
	// Name : The name of the this shared folder.
	Name string `json:"name"`
	// Permissions : Actions the current user may perform on the folder and its
	// contents. The set of permissions corresponds to the FolderActions in the
	// request.
	Permissions []*FolderPermission `json:"permissions,omitempty"`
	// Policy : Policies governing this shared folder.
	Policy *FolderPolicy `json:"policy"`
	// PreviewUrl : URL for displaying a web preview of the shared folder.
	PreviewUrl string `json:"preview_url"`
	// SharedFolderId : The ID of the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// TimeInvited : Timestamp indicating when the current user was invited to
	// this shared folder.
	TimeInvited time.Time `json:"time_invited"`
	// AccessInheritance : Whether the folder inherits its members from its
	// parent.
	AccessInheritance *AccessInheritance `json:"access_inheritance"`
}

// NewSharedFolderMetadata returns a new SharedFolderMetadata instance
func NewSharedFolderMetadata(AccessType *AccessLevel, IsInsideTeamFolder bool, IsTeamFolder bool, Name string, Policy *FolderPolicy, PreviewUrl string, SharedFolderId string, TimeInvited time.Time) *SharedFolderMetadata {
	s := new(SharedFolderMetadata)
	s.AccessType = AccessType
	s.IsInsideTeamFolder = IsInsideTeamFolder
	s.IsTeamFolder = IsTeamFolder
	s.Name = Name
	s.Policy = Policy
	s.PreviewUrl = PreviewUrl
	s.SharedFolderId = SharedFolderId
	s.TimeInvited = TimeInvited
	s.AccessInheritance = &AccessInheritance{Tagged: dropbox.Tagged{"inherit"}}
	return s
}

// SharedLinkAccessFailureReason : has no documentation (yet)
type SharedLinkAccessFailureReason struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkAccessFailureReason
const (
	SharedLinkAccessFailureReasonLoginRequired       = "login_required"
	SharedLinkAccessFailureReasonEmailVerifyRequired = "email_verify_required"
	SharedLinkAccessFailureReasonPasswordRequired    = "password_required"
	SharedLinkAccessFailureReasonTeamOnly            = "team_only"
	SharedLinkAccessFailureReasonOwnerOnly           = "owner_only"
	SharedLinkAccessFailureReasonOther               = "other"
)

// SharedLinkAlreadyExistsMetadata : has no documentation (yet)
type SharedLinkAlreadyExistsMetadata struct {
	dropbox.Tagged
	// Metadata : Metadata of the shared link that already exists.
	Metadata IsSharedLinkMetadata `json:"metadata,omitempty"`
}

// Valid tag values for SharedLinkAlreadyExistsMetadata
const (
	SharedLinkAlreadyExistsMetadataMetadata = "metadata"
	SharedLinkAlreadyExistsMetadataOther    = "other"
)

// UnmarshalJSON deserializes into a SharedLinkAlreadyExistsMetadata instance
func (u *SharedLinkAlreadyExistsMetadata) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Metadata : Metadata of the shared link that already exists.
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
		u.Metadata, err = IsSharedLinkMetadataFromJSON(w.Metadata)

		if err != nil {
			return err
		}
	}
	return nil
}

// SharedLinkPolicy : Who can view shared links in this folder.
type SharedLinkPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkPolicy
const (
	SharedLinkPolicyAnyone  = "anyone"
	SharedLinkPolicyTeam    = "team"
	SharedLinkPolicyMembers = "members"
	SharedLinkPolicyOther   = "other"
)

// SharedLinkSettings : has no documentation (yet)
type SharedLinkSettings struct {
	// RequestedVisibility : The requested access for this shared link.
	RequestedVisibility *RequestedVisibility `json:"requested_visibility,omitempty"`
	// LinkPassword : If `requested_visibility` is
	// `RequestedVisibility.password` this is needed to specify the password to
	// access the link.
	LinkPassword string `json:"link_password,omitempty"`
	// Expires : Expiration time of the shared link. By default the link won't
	// expire.
	Expires time.Time `json:"expires,omitempty"`
	// Audience : The new audience who can benefit from the access level
	// specified by the link's access level specified in the `link_access_level`
	// field of `LinkPermissions`. This is used in conjunction with team
	// policies and shared folder policies to determine the final effective
	// audience type in the `effective_audience` field of `LinkPermissions.
	Audience *LinkAudience `json:"audience,omitempty"`
	// Access : Requested access level you want the audience to gain from this
	// link.
	Access *RequestedLinkAccessLevel `json:"access,omitempty"`
}

// NewSharedLinkSettings returns a new SharedLinkSettings instance
func NewSharedLinkSettings() *SharedLinkSettings {
	s := new(SharedLinkSettings)
	return s
}

// SharedLinkSettingsError : has no documentation (yet)
type SharedLinkSettingsError struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkSettingsError
const (
	SharedLinkSettingsErrorInvalidSettings = "invalid_settings"
	SharedLinkSettingsErrorNotAuthorized   = "not_authorized"
)

// SharingFileAccessError : User could not access this file.
type SharingFileAccessError struct {
	dropbox.Tagged
}

// Valid tag values for SharingFileAccessError
const (
	SharingFileAccessErrorNoPermission       = "no_permission"
	SharingFileAccessErrorInvalidFile        = "invalid_file"
	SharingFileAccessErrorIsFolder           = "is_folder"
	SharingFileAccessErrorInsidePublicFolder = "inside_public_folder"
	SharingFileAccessErrorInsideOsxPackage   = "inside_osx_package"
	SharingFileAccessErrorOther              = "other"
)

// SharingUserError : User account had a problem preventing this action.
type SharingUserError struct {
	dropbox.Tagged
}

// Valid tag values for SharingUserError
const (
	SharingUserErrorEmailUnverified = "email_unverified"
	SharingUserErrorOther           = "other"
)

// TeamMemberInfo : Information about a team member.
type TeamMemberInfo struct {
	// TeamInfo : Information about the member's team.
	TeamInfo *users.Team `json:"team_info"`
	// DisplayName : The display name of the user.
	DisplayName string `json:"display_name"`
	// MemberId : ID of user as a member of a team. This field will only be
	// present if the member is in the same team as current user.
	MemberId string `json:"member_id,omitempty"`
}

// NewTeamMemberInfo returns a new TeamMemberInfo instance
func NewTeamMemberInfo(TeamInfo *users.Team, DisplayName string) *TeamMemberInfo {
	s := new(TeamMemberInfo)
	s.TeamInfo = TeamInfo
	s.DisplayName = DisplayName
	return s
}

// TransferFolderArg : has no documentation (yet)
type TransferFolderArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// ToDropboxId : A account or team member ID to transfer ownership to.
	ToDropboxId string `json:"to_dropbox_id"`
}

// NewTransferFolderArg returns a new TransferFolderArg instance
func NewTransferFolderArg(SharedFolderId string, ToDropboxId string) *TransferFolderArg {
	s := new(TransferFolderArg)
	s.SharedFolderId = SharedFolderId
	s.ToDropboxId = ToDropboxId
	return s
}

// TransferFolderError : has no documentation (yet)
type TransferFolderError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for TransferFolderError
const (
	TransferFolderErrorAccessError             = "access_error"
	TransferFolderErrorInvalidDropboxId        = "invalid_dropbox_id"
	TransferFolderErrorNewOwnerNotAMember      = "new_owner_not_a_member"
	TransferFolderErrorNewOwnerUnmounted       = "new_owner_unmounted"
	TransferFolderErrorNewOwnerEmailUnverified = "new_owner_email_unverified"
	TransferFolderErrorTeamFolder              = "team_folder"
	TransferFolderErrorNoPermission            = "no_permission"
	TransferFolderErrorOther                   = "other"
)

// UnmarshalJSON deserializes into a TransferFolderError instance
func (u *TransferFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// UnmountFolderArg : has no documentation (yet)
type UnmountFolderArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
}

// NewUnmountFolderArg returns a new UnmountFolderArg instance
func NewUnmountFolderArg(SharedFolderId string) *UnmountFolderArg {
	s := new(UnmountFolderArg)
	s.SharedFolderId = SharedFolderId
	return s
}

// UnmountFolderError : has no documentation (yet)
type UnmountFolderError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for UnmountFolderError
const (
	UnmountFolderErrorAccessError    = "access_error"
	UnmountFolderErrorNoPermission   = "no_permission"
	UnmountFolderErrorNotUnmountable = "not_unmountable"
	UnmountFolderErrorOther          = "other"
)

// UnmarshalJSON deserializes into a UnmountFolderError instance
func (u *UnmountFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// UnshareFileArg : Arguments for `unshareFile`.
type UnshareFileArg struct {
	// File : The file to unshare.
	File string `json:"file"`
}

// NewUnshareFileArg returns a new UnshareFileArg instance
func NewUnshareFileArg(File string) *UnshareFileArg {
	s := new(UnshareFileArg)
	s.File = File
	return s
}

// UnshareFileError : Error result for `unshareFile`.
type UnshareFileError struct {
	dropbox.Tagged
	// UserError : has no documentation (yet)
	UserError *SharingUserError `json:"user_error,omitempty"`
	// AccessError : has no documentation (yet)
	AccessError *SharingFileAccessError `json:"access_error,omitempty"`
}

// Valid tag values for UnshareFileError
const (
	UnshareFileErrorUserError   = "user_error"
	UnshareFileErrorAccessError = "access_error"
	UnshareFileErrorOther       = "other"
)

// UnmarshalJSON deserializes into a UnshareFileError instance
func (u *UnshareFileError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserError : has no documentation (yet)
		UserError *SharingUserError `json:"user_error,omitempty"`
		// AccessError : has no documentation (yet)
		AccessError *SharingFileAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_error":
		u.UserError = w.UserError

		if err != nil {
			return err
		}
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// UnshareFolderArg : has no documentation (yet)
type UnshareFolderArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// LeaveACopy : If true, members of this shared folder will get a copy of
	// this folder after it's unshared. Otherwise, it will be removed from their
	// Dropbox. The current user, who is an owner, will always retain their
	// copy.
	LeaveACopy bool `json:"leave_a_copy"`
}

// NewUnshareFolderArg returns a new UnshareFolderArg instance
func NewUnshareFolderArg(SharedFolderId string) *UnshareFolderArg {
	s := new(UnshareFolderArg)
	s.SharedFolderId = SharedFolderId
	s.LeaveACopy = false
	return s
}

// UnshareFolderError : has no documentation (yet)
type UnshareFolderError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for UnshareFolderError
const (
	UnshareFolderErrorAccessError  = "access_error"
	UnshareFolderErrorTeamFolder   = "team_folder"
	UnshareFolderErrorNoPermission = "no_permission"
	UnshareFolderErrorTooManyFiles = "too_many_files"
	UnshareFolderErrorOther        = "other"
)

// UnmarshalJSON deserializes into a UnshareFolderError instance
func (u *UnshareFolderError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateFileMemberArgs : Arguments for `updateFileMember`.
type UpdateFileMemberArgs struct {
	ChangeFileMemberAccessArgs
}

// NewUpdateFileMemberArgs returns a new UpdateFileMemberArgs instance
func NewUpdateFileMemberArgs(File string, Member *MemberSelector, AccessLevel *AccessLevel) *UpdateFileMemberArgs {
	s := new(UpdateFileMemberArgs)
	s.File = File
	s.Member = Member
	s.AccessLevel = AccessLevel
	return s
}

// UpdateFolderMemberArg : has no documentation (yet)
type UpdateFolderMemberArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// Member : The member of the shared folder to update.  Only the
	// `MemberSelector.dropbox_id` may be set at this time.
	Member *MemberSelector `json:"member"`
	// AccessLevel : The new access level for `member`. `AccessLevel.owner` is
	// disallowed.
	AccessLevel *AccessLevel `json:"access_level"`
}

// NewUpdateFolderMemberArg returns a new UpdateFolderMemberArg instance
func NewUpdateFolderMemberArg(SharedFolderId string, Member *MemberSelector, AccessLevel *AccessLevel) *UpdateFolderMemberArg {
	s := new(UpdateFolderMemberArg)
	s.SharedFolderId = SharedFolderId
	s.Member = Member
	s.AccessLevel = AccessLevel
	return s
}

// UpdateFolderMemberError : has no documentation (yet)
type UpdateFolderMemberError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	// MemberError : has no documentation (yet)
	MemberError *SharedFolderMemberError `json:"member_error,omitempty"`
	// NoExplicitAccess : If updating the access type required the member to be
	// added to the shared folder and there was an error when adding the member.
	NoExplicitAccess *AddFolderMemberError `json:"no_explicit_access,omitempty"`
}

// Valid tag values for UpdateFolderMemberError
const (
	UpdateFolderMemberErrorAccessError      = "access_error"
	UpdateFolderMemberErrorMemberError      = "member_error"
	UpdateFolderMemberErrorNoExplicitAccess = "no_explicit_access"
	UpdateFolderMemberErrorInsufficientPlan = "insufficient_plan"
	UpdateFolderMemberErrorNoPermission     = "no_permission"
	UpdateFolderMemberErrorOther            = "other"
)

// UnmarshalJSON deserializes into a UpdateFolderMemberError instance
func (u *UpdateFolderMemberError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
		// MemberError : has no documentation (yet)
		MemberError *SharedFolderMemberError `json:"member_error,omitempty"`
		// NoExplicitAccess : If updating the access type required the member to
		// be added to the shared folder and there was an error when adding the
		// member.
		NoExplicitAccess *AddFolderMemberError `json:"no_explicit_access,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	case "member_error":
		u.MemberError = w.MemberError

		if err != nil {
			return err
		}
	case "no_explicit_access":
		u.NoExplicitAccess = w.NoExplicitAccess

		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateFolderPolicyArg : If any of the policies are unset, then they retain
// their current setting.
type UpdateFolderPolicyArg struct {
	// SharedFolderId : The ID for the shared folder.
	SharedFolderId string `json:"shared_folder_id"`
	// MemberPolicy : Who can be a member of this shared folder. Only applicable
	// if the current user is on a team.
	MemberPolicy *MemberPolicy `json:"member_policy,omitempty"`
	// AclUpdatePolicy : Who can add and remove members of this shared folder.
	AclUpdatePolicy *AclUpdatePolicy `json:"acl_update_policy,omitempty"`
	// ViewerInfoPolicy : Who can enable/disable viewer info for this shared
	// folder.
	ViewerInfoPolicy *ViewerInfoPolicy `json:"viewer_info_policy,omitempty"`
	// SharedLinkPolicy : The policy to apply to shared links created for
	// content inside this shared folder. The current user must be on a team to
	// set this policy to `SharedLinkPolicy.members`.
	SharedLinkPolicy *SharedLinkPolicy `json:"shared_link_policy,omitempty"`
	// LinkSettings : Settings on the link for this folder.
	LinkSettings *LinkSettings `json:"link_settings,omitempty"`
	// Actions : A list of `FolderAction`s corresponding to `FolderPermission`s
	// that should appear in the  response's `SharedFolderMetadata.permissions`
	// field describing the actions the  authenticated user can perform on the
	// folder.
	Actions []*FolderAction `json:"actions,omitempty"`
}

// NewUpdateFolderPolicyArg returns a new UpdateFolderPolicyArg instance
func NewUpdateFolderPolicyArg(SharedFolderId string) *UpdateFolderPolicyArg {
	s := new(UpdateFolderPolicyArg)
	s.SharedFolderId = SharedFolderId
	return s
}

// UpdateFolderPolicyError : has no documentation (yet)
type UpdateFolderPolicyError struct {
	dropbox.Tagged
	// AccessError : has no documentation (yet)
	AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
}

// Valid tag values for UpdateFolderPolicyError
const (
	UpdateFolderPolicyErrorAccessError                     = "access_error"
	UpdateFolderPolicyErrorNotOnTeam                       = "not_on_team"
	UpdateFolderPolicyErrorTeamPolicyDisallowsMemberPolicy = "team_policy_disallows_member_policy"
	UpdateFolderPolicyErrorDisallowedSharedLinkPolicy      = "disallowed_shared_link_policy"
	UpdateFolderPolicyErrorNoPermission                    = "no_permission"
	UpdateFolderPolicyErrorTeamFolder                      = "team_folder"
	UpdateFolderPolicyErrorOther                           = "other"
)

// UnmarshalJSON deserializes into a UpdateFolderPolicyError instance
func (u *UpdateFolderPolicyError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AccessError : has no documentation (yet)
		AccessError *SharedFolderAccessError `json:"access_error,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "access_error":
		u.AccessError = w.AccessError

		if err != nil {
			return err
		}
	}
	return nil
}

// UserMembershipInfo : The information about a user member of the shared
// content.
type UserMembershipInfo struct {
	MembershipInfo
	// User : The account information for the membership user.
	User *UserInfo `json:"user"`
}

// NewUserMembershipInfo returns a new UserMembershipInfo instance
func NewUserMembershipInfo(AccessType *AccessLevel, User *UserInfo) *UserMembershipInfo {
	s := new(UserMembershipInfo)
	s.AccessType = AccessType
	s.User = User
	s.IsInherited = false
	return s
}

// UserFileMembershipInfo : The information about a user member of the shared
// content with an appended last seen timestamp.
type UserFileMembershipInfo struct {
	UserMembershipInfo
	// TimeLastSeen : The UTC timestamp of when the user has last seen the
	// content, if they have.
	TimeLastSeen time.Time `json:"time_last_seen,omitempty"`
	// PlatformType : The platform on which the user has last seen the content,
	// or unknown.
	PlatformType *seen_state.PlatformType `json:"platform_type,omitempty"`
}

// NewUserFileMembershipInfo returns a new UserFileMembershipInfo instance
func NewUserFileMembershipInfo(AccessType *AccessLevel, User *UserInfo) *UserFileMembershipInfo {
	s := new(UserFileMembershipInfo)
	s.AccessType = AccessType
	s.User = User
	s.IsInherited = false
	return s
}

// UserInfo : Basic information about a user. Use `usersAccount` and
// `usersAccountBatch` to obtain more detailed information.
type UserInfo struct {
	// AccountId : The account ID of the user.
	AccountId string `json:"account_id"`
	// Email : Email address of user.
	Email string `json:"email"`
	// DisplayName : The display name of the user.
	DisplayName string `json:"display_name"`
	// SameTeam : If the user is in the same team as current user.
	SameTeam bool `json:"same_team"`
	// TeamMemberId : The team member ID of the shared folder member. Only
	// present if `same_team` is true.
	TeamMemberId string `json:"team_member_id,omitempty"`
}

// NewUserInfo returns a new UserInfo instance
func NewUserInfo(AccountId string, Email string, DisplayName string, SameTeam bool) *UserInfo {
	s := new(UserInfo)
	s.AccountId = AccountId
	s.Email = Email
	s.DisplayName = DisplayName
	s.SameTeam = SameTeam
	return s
}

// ViewerInfoPolicy : has no documentation (yet)
type ViewerInfoPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ViewerInfoPolicy
const (
	ViewerInfoPolicyEnabled  = "enabled"
	ViewerInfoPolicyDisabled = "disabled"
	ViewerInfoPolicyOther    = "other"
)

// Visibility : Who can access a shared link. The most open visibility is
// `public`. The default depends on many aspects, such as team and user
// preferences and shared folder settings.
type Visibility struct {
	dropbox.Tagged
}

// Valid tag values for Visibility
const (
	VisibilityPublic           = "public"
	VisibilityTeamOnly         = "team_only"
	VisibilityPassword         = "password"
	VisibilityTeamAndPassword  = "team_and_password"
	VisibilitySharedFolderOnly = "shared_folder_only"
	VisibilityOther            = "other"
)
