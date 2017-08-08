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

// Package team_log : has no documentation (yet)
package team_log

import (
	"encoding/json"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team_common"
)

// AccessMethodLogInfo : Indicates the method in which the action was performed.
type AccessMethodLogInfo struct {
	dropbox.Tagged
	// EndUser : End user session details.
	EndUser IsSessionLogInfo `json:"end_user,omitempty"`
	// SignInAs : Sign in as session details.
	SignInAs *WebSessionLogInfo `json:"sign_in_as,omitempty"`
	// ContentManager : Content manager session details.
	ContentManager *WebSessionLogInfo `json:"content_manager,omitempty"`
	// AdminConsole : Admin console session details.
	AdminConsole *WebSessionLogInfo `json:"admin_console,omitempty"`
	// Api : Api session details.
	Api *ApiSessionLogInfo `json:"api,omitempty"`
}

// Valid tag values for AccessMethodLogInfo
const (
	AccessMethodLogInfoEndUser        = "end_user"
	AccessMethodLogInfoSignInAs       = "sign_in_as"
	AccessMethodLogInfoContentManager = "content_manager"
	AccessMethodLogInfoAdminConsole   = "admin_console"
	AccessMethodLogInfoApi            = "api"
	AccessMethodLogInfoOther          = "other"
)

// UnmarshalJSON deserializes into a AccessMethodLogInfo instance
func (u *AccessMethodLogInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// EndUser : End user session details.
		EndUser json.RawMessage `json:"end_user,omitempty"`
		// SignInAs : Sign in as session details.
		SignInAs json.RawMessage `json:"sign_in_as,omitempty"`
		// ContentManager : Content manager session details.
		ContentManager json.RawMessage `json:"content_manager,omitempty"`
		// AdminConsole : Admin console session details.
		AdminConsole json.RawMessage `json:"admin_console,omitempty"`
		// Api : Api session details.
		Api json.RawMessage `json:"api,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "end_user":
		u.EndUser, err = IsSessionLogInfoFromJSON(body)

		if err != nil {
			return err
		}
	case "sign_in_as":
		err = json.Unmarshal(body, &u.SignInAs)

		if err != nil {
			return err
		}
	case "content_manager":
		err = json.Unmarshal(body, &u.ContentManager)

		if err != nil {
			return err
		}
	case "admin_console":
		err = json.Unmarshal(body, &u.AdminConsole)

		if err != nil {
			return err
		}
	case "api":
		err = json.Unmarshal(body, &u.Api)

		if err != nil {
			return err
		}
	}
	return nil
}

// AccountCaptureAvailability : has no documentation (yet)
type AccountCaptureAvailability struct {
	dropbox.Tagged
}

// Valid tag values for AccountCaptureAvailability
const (
	AccountCaptureAvailabilityUnavailable = "unavailable"
	AccountCaptureAvailabilityAvailable   = "available"
	AccountCaptureAvailabilityOther       = "other"
)

// AccountCaptureChangeAvailabilityDetails : Granted or revoked the option to
// enable account capture on domains belonging to the team.
type AccountCaptureChangeAvailabilityDetails struct {
	// NewValue : New account capture availabilty value.
	NewValue *AccountCaptureAvailability `json:"new_value"`
	// PreviousValue : Previous account capture availabilty value. Might be
	// missing due to historical data gap.
	PreviousValue *AccountCaptureAvailability `json:"previous_value,omitempty"`
}

// NewAccountCaptureChangeAvailabilityDetails returns a new AccountCaptureChangeAvailabilityDetails instance
func NewAccountCaptureChangeAvailabilityDetails(NewValue *AccountCaptureAvailability) *AccountCaptureChangeAvailabilityDetails {
	s := new(AccountCaptureChangeAvailabilityDetails)
	s.NewValue = NewValue
	return s
}

// AccountCaptureChangePolicyDetails : Changed the account capture policy on a
// domain belonging to the team.
type AccountCaptureChangePolicyDetails struct {
	// NewValue : New account capture policy.
	NewValue *AccountCapturePolicy `json:"new_value"`
	// PreviousValue : Previous account capture policy. Might be missing due to
	// historical data gap.
	PreviousValue *AccountCapturePolicy `json:"previous_value,omitempty"`
}

// NewAccountCaptureChangePolicyDetails returns a new AccountCaptureChangePolicyDetails instance
func NewAccountCaptureChangePolicyDetails(NewValue *AccountCapturePolicy) *AccountCaptureChangePolicyDetails {
	s := new(AccountCaptureChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// AccountCaptureMigrateAccountDetails : Account captured user migrated their
// account to the team.
type AccountCaptureMigrateAccountDetails struct {
	// DomainName : Domain name.
	DomainName string `json:"domain_name"`
}

// NewAccountCaptureMigrateAccountDetails returns a new AccountCaptureMigrateAccountDetails instance
func NewAccountCaptureMigrateAccountDetails(DomainName string) *AccountCaptureMigrateAccountDetails {
	s := new(AccountCaptureMigrateAccountDetails)
	s.DomainName = DomainName
	return s
}

// AccountCapturePolicy : has no documentation (yet)
type AccountCapturePolicy struct {
	dropbox.Tagged
}

// Valid tag values for AccountCapturePolicy
const (
	AccountCapturePolicyDisabled     = "disabled"
	AccountCapturePolicyInvitedUsers = "invited_users"
	AccountCapturePolicyAllUsers     = "all_users"
	AccountCapturePolicyOther        = "other"
)

// AccountCaptureRelinquishAccountDetails : Account captured user relinquished
// their account by changing the email address associated with it.
type AccountCaptureRelinquishAccountDetails struct {
	// DomainName : Domain name.
	DomainName string `json:"domain_name"`
}

// NewAccountCaptureRelinquishAccountDetails returns a new AccountCaptureRelinquishAccountDetails instance
func NewAccountCaptureRelinquishAccountDetails(DomainName string) *AccountCaptureRelinquishAccountDetails {
	s := new(AccountCaptureRelinquishAccountDetails)
	s.DomainName = DomainName
	return s
}

// ActorLogInfo : The entity who performed the action.
type ActorLogInfo struct {
	dropbox.Tagged
	// User : The user who did the action.
	User IsUserLogInfo `json:"user,omitempty"`
	// Admin : The admin who did the action.
	Admin IsUserLogInfo `json:"admin,omitempty"`
	// App : The application who did the action.
	App IsAppLogInfo `json:"app,omitempty"`
	// Reseller : Action done by reseller.
	Reseller *ResellerLogInfo `json:"reseller,omitempty"`
}

// Valid tag values for ActorLogInfo
const (
	ActorLogInfoUser     = "user"
	ActorLogInfoAdmin    = "admin"
	ActorLogInfoApp      = "app"
	ActorLogInfoReseller = "reseller"
	ActorLogInfoDropbox  = "dropbox"
	ActorLogInfoOther    = "other"
)

// UnmarshalJSON deserializes into a ActorLogInfo instance
func (u *ActorLogInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// User : The user who did the action.
		User json.RawMessage `json:"user,omitempty"`
		// Admin : The admin who did the action.
		Admin json.RawMessage `json:"admin,omitempty"`
		// App : The application who did the action.
		App json.RawMessage `json:"app,omitempty"`
		// Reseller : Action done by reseller.
		Reseller json.RawMessage `json:"reseller,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user":
		u.User, err = IsUserLogInfoFromJSON(body)

		if err != nil {
			return err
		}
	case "admin":
		u.Admin, err = IsUserLogInfoFromJSON(body)

		if err != nil {
			return err
		}
	case "app":
		u.App, err = IsAppLogInfoFromJSON(body)

		if err != nil {
			return err
		}
	case "reseller":
		err = json.Unmarshal(body, &u.Reseller)

		if err != nil {
			return err
		}
	}
	return nil
}

// AdminRole : has no documentation (yet)
type AdminRole struct {
	dropbox.Tagged
}

// Valid tag values for AdminRole
const (
	AdminRoleUser                = "user"
	AdminRoleLimitedAdmin        = "limited_admin"
	AdminRoleSupportAdmin        = "support_admin"
	AdminRoleUserManagementAdmin = "user_management_admin"
	AdminRoleTeamAdmin           = "team_admin"
	AdminRoleOther               = "other"
)

// AllowDownloadDisabledDetails : Disabled allow downloads.
type AllowDownloadDisabledDetails struct {
}

// NewAllowDownloadDisabledDetails returns a new AllowDownloadDisabledDetails instance
func NewAllowDownloadDisabledDetails() *AllowDownloadDisabledDetails {
	s := new(AllowDownloadDisabledDetails)
	return s
}

// AllowDownloadEnabledDetails : Enabled allow downloads.
type AllowDownloadEnabledDetails struct {
}

// NewAllowDownloadEnabledDetails returns a new AllowDownloadEnabledDetails instance
func NewAllowDownloadEnabledDetails() *AllowDownloadEnabledDetails {
	s := new(AllowDownloadEnabledDetails)
	return s
}

// ApiSessionLogInfo : Api session.
type ApiSessionLogInfo struct {
	// RequestId : Api request ID.
	RequestId string `json:"request_id"`
}

// NewApiSessionLogInfo returns a new ApiSessionLogInfo instance
func NewApiSessionLogInfo(RequestId string) *ApiSessionLogInfo {
	s := new(ApiSessionLogInfo)
	s.RequestId = RequestId
	return s
}

// AppLinkTeamDetails : Linked an app for team.
type AppLinkTeamDetails struct {
	// AppInfo : Relevant application details.
	AppInfo IsAppLogInfo `json:"app_info"`
}

// NewAppLinkTeamDetails returns a new AppLinkTeamDetails instance
func NewAppLinkTeamDetails(AppInfo IsAppLogInfo) *AppLinkTeamDetails {
	s := new(AppLinkTeamDetails)
	s.AppInfo = AppInfo
	return s
}

// AppLinkUserDetails : Linked an app for team member.
type AppLinkUserDetails struct {
	// AppInfo : Relevant application details.
	AppInfo IsAppLogInfo `json:"app_info"`
}

// NewAppLinkUserDetails returns a new AppLinkUserDetails instance
func NewAppLinkUserDetails(AppInfo IsAppLogInfo) *AppLinkUserDetails {
	s := new(AppLinkUserDetails)
	s.AppInfo = AppInfo
	return s
}

// AppLogInfo : App's logged information.
type AppLogInfo struct {
	// AppId : App unique ID. Might be missing due to historical data gap.
	AppId string `json:"app_id,omitempty"`
	// DisplayName : App display name. Might be missing due to historical data
	// gap.
	DisplayName string `json:"display_name,omitempty"`
}

// NewAppLogInfo returns a new AppLogInfo instance
func NewAppLogInfo() *AppLogInfo {
	s := new(AppLogInfo)
	return s
}

// IsAppLogInfo is the interface type for AppLogInfo and its subtypes
type IsAppLogInfo interface {
	IsAppLogInfo()
}

// IsAppLogInfo implements the IsAppLogInfo interface
func (u *AppLogInfo) IsAppLogInfo() {}

type appLogInfoUnion struct {
	dropbox.Tagged
	// UserOrTeamLinkedApp : has no documentation (yet)
	UserOrTeamLinkedApp *UserOrTeamLinkedAppLogInfo `json:"user_or_team_linked_app,omitempty"`
	// UserLinkedApp : has no documentation (yet)
	UserLinkedApp *UserLinkedAppLogInfo `json:"user_linked_app,omitempty"`
	// TeamLinkedApp : has no documentation (yet)
	TeamLinkedApp *TeamLinkedAppLogInfo `json:"team_linked_app,omitempty"`
}

// Valid tag values for AppLogInfo
const (
	AppLogInfoUserOrTeamLinkedApp = "user_or_team_linked_app"
	AppLogInfoUserLinkedApp       = "user_linked_app"
	AppLogInfoTeamLinkedApp       = "team_linked_app"
)

// UnmarshalJSON deserializes into a appLogInfoUnion instance
func (u *appLogInfoUnion) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// UserOrTeamLinkedApp : has no documentation (yet)
		UserOrTeamLinkedApp json.RawMessage `json:"user_or_team_linked_app,omitempty"`
		// UserLinkedApp : has no documentation (yet)
		UserLinkedApp json.RawMessage `json:"user_linked_app,omitempty"`
		// TeamLinkedApp : has no documentation (yet)
		TeamLinkedApp json.RawMessage `json:"team_linked_app,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user_or_team_linked_app":
		err = json.Unmarshal(body, &u.UserOrTeamLinkedApp)

		if err != nil {
			return err
		}
	case "user_linked_app":
		err = json.Unmarshal(body, &u.UserLinkedApp)

		if err != nil {
			return err
		}
	case "team_linked_app":
		err = json.Unmarshal(body, &u.TeamLinkedApp)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsAppLogInfoFromJSON converts JSON to a concrete IsAppLogInfo instance
func IsAppLogInfoFromJSON(data []byte) (IsAppLogInfo, error) {
	var t appLogInfoUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "user_or_team_linked_app":
		return t.UserOrTeamLinkedApp, nil

	case "user_linked_app":
		return t.UserLinkedApp, nil

	case "team_linked_app":
		return t.TeamLinkedApp, nil

	}
	return nil, nil
}

// AppUnlinkTeamDetails : Unlinked an app for team.
type AppUnlinkTeamDetails struct {
	// AppInfo : Relevant application details.
	AppInfo IsAppLogInfo `json:"app_info"`
}

// NewAppUnlinkTeamDetails returns a new AppUnlinkTeamDetails instance
func NewAppUnlinkTeamDetails(AppInfo IsAppLogInfo) *AppUnlinkTeamDetails {
	s := new(AppUnlinkTeamDetails)
	s.AppInfo = AppInfo
	return s
}

// AppUnlinkUserDetails : Unlinked an app for team member.
type AppUnlinkUserDetails struct {
	// AppInfo : Relevant application details.
	AppInfo IsAppLogInfo `json:"app_info"`
}

// NewAppUnlinkUserDetails returns a new AppUnlinkUserDetails instance
func NewAppUnlinkUserDetails(AppInfo IsAppLogInfo) *AppUnlinkUserDetails {
	s := new(AppUnlinkUserDetails)
	s.AppInfo = AppInfo
	return s
}

// AssetLogInfo : Asset details.
type AssetLogInfo struct {
	dropbox.Tagged
	// File : File's details.
	File *FileLogInfo `json:"file,omitempty"`
	// Folder : Folder's details.
	Folder *FolderLogInfo `json:"folder,omitempty"`
	// PaperDocument : Paper docuement's details.
	PaperDocument *PaperDocumentLogInfo `json:"paper_document,omitempty"`
	// PaperFolder : Paper folder's details.
	PaperFolder *PaperFolderLogInfo `json:"paper_folder,omitempty"`
}

// Valid tag values for AssetLogInfo
const (
	AssetLogInfoFile          = "file"
	AssetLogInfoFolder        = "folder"
	AssetLogInfoPaperDocument = "paper_document"
	AssetLogInfoPaperFolder   = "paper_folder"
	AssetLogInfoOther         = "other"
)

// UnmarshalJSON deserializes into a AssetLogInfo instance
func (u *AssetLogInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// File : File's details.
		File json.RawMessage `json:"file,omitempty"`
		// Folder : Folder's details.
		Folder json.RawMessage `json:"folder,omitempty"`
		// PaperDocument : Paper docuement's details.
		PaperDocument json.RawMessage `json:"paper_document,omitempty"`
		// PaperFolder : Paper folder's details.
		PaperFolder json.RawMessage `json:"paper_folder,omitempty"`
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
	case "paper_document":
		err = json.Unmarshal(body, &u.PaperDocument)

		if err != nil {
			return err
		}
	case "paper_folder":
		err = json.Unmarshal(body, &u.PaperFolder)

		if err != nil {
			return err
		}
	}
	return nil
}

// Certificate : Certificate details.
type Certificate struct {
	// Subject : Certificate subject.
	Subject string `json:"subject"`
	// Issuer : Certificate issuer.
	Issuer string `json:"issuer"`
	// IssueDate : Certificate issue date.
	IssueDate string `json:"issue_date"`
	// ExpirationDate : Certificate expiration date.
	ExpirationDate string `json:"expiration_date"`
	// SerialNumber : Certificate serial number.
	SerialNumber string `json:"serial_number"`
	// Sha1Fingerprint : Certificate sha1 fingerprint.
	Sha1Fingerprint string `json:"sha1_fingerprint"`
	// CommonName : Certificate common name.
	CommonName string `json:"common_name"`
}

// NewCertificate returns a new Certificate instance
func NewCertificate(Subject string, Issuer string, IssueDate string, ExpirationDate string, SerialNumber string, Sha1Fingerprint string, CommonName string) *Certificate {
	s := new(Certificate)
	s.Subject = Subject
	s.Issuer = Issuer
	s.IssueDate = IssueDate
	s.ExpirationDate = ExpirationDate
	s.SerialNumber = SerialNumber
	s.Sha1Fingerprint = Sha1Fingerprint
	s.CommonName = CommonName
	return s
}

// CollectionShareDetails : Shared an album.
type CollectionShareDetails struct {
	// AlbumName : Album name.
	AlbumName string `json:"album_name"`
}

// NewCollectionShareDetails returns a new CollectionShareDetails instance
func NewCollectionShareDetails(AlbumName string) *CollectionShareDetails {
	s := new(CollectionShareDetails)
	s.AlbumName = AlbumName
	return s
}

// Confidentiality : has no documentation (yet)
type Confidentiality struct {
	dropbox.Tagged
}

// Valid tag values for Confidentiality
const (
	ConfidentialityConfidential    = "confidential"
	ConfidentialityNonConfidential = "non_confidential"
	ConfidentialityOther           = "other"
)

// ContentPermanentDeletePolicy : Policy for pemanent content deletion
type ContentPermanentDeletePolicy struct {
	dropbox.Tagged
}

// Valid tag values for ContentPermanentDeletePolicy
const (
	ContentPermanentDeletePolicyDisabled = "disabled"
	ContentPermanentDeletePolicyEnabled  = "enabled"
	ContentPermanentDeletePolicyOther    = "other"
)

// ContextLogInfo : The primary entity on which the action was done.
type ContextLogInfo struct {
	dropbox.Tagged
	// TeamMember : Action was done on behalf of a team member.
	TeamMember *TeamMemberLogInfo `json:"team_member,omitempty"`
	// NonTeamMember : Action was done on behalf of a non team member.
	NonTeamMember *NonTeamMemberLogInfo `json:"non_team_member,omitempty"`
}

// Valid tag values for ContextLogInfo
const (
	ContextLogInfoTeamMember    = "team_member"
	ContextLogInfoNonTeamMember = "non_team_member"
	ContextLogInfoTeam          = "team"
	ContextLogInfoOther         = "other"
)

// UnmarshalJSON deserializes into a ContextLogInfo instance
func (u *ContextLogInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// TeamMember : Action was done on behalf of a team member.
		TeamMember json.RawMessage `json:"team_member,omitempty"`
		// NonTeamMember : Action was done on behalf of a non team member.
		NonTeamMember json.RawMessage `json:"non_team_member,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "team_member":
		err = json.Unmarshal(body, &u.TeamMember)

		if err != nil {
			return err
		}
	case "non_team_member":
		err = json.Unmarshal(body, &u.NonTeamMember)

		if err != nil {
			return err
		}
	}
	return nil
}

// CreateFolderDetails : Created folders.
type CreateFolderDetails struct {
}

// NewCreateFolderDetails returns a new CreateFolderDetails instance
func NewCreateFolderDetails() *CreateFolderDetails {
	s := new(CreateFolderDetails)
	return s
}

// DataPlacementRestrictionChangePolicyDetails : Set a restriction policy
// regarding the location of data centers where team data resides.
type DataPlacementRestrictionChangePolicyDetails struct {
	// PreviousValue : Previous placement restriction.
	PreviousValue *PlacementRestriction `json:"previous_value"`
	// NewValue : New placement restriction.
	NewValue *PlacementRestriction `json:"new_value"`
}

// NewDataPlacementRestrictionChangePolicyDetails returns a new DataPlacementRestrictionChangePolicyDetails instance
func NewDataPlacementRestrictionChangePolicyDetails(PreviousValue *PlacementRestriction, NewValue *PlacementRestriction) *DataPlacementRestrictionChangePolicyDetails {
	s := new(DataPlacementRestrictionChangePolicyDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// DataPlacementRestrictionSatisfyPolicyDetails : Satisfied a previously set
// restriction policy regarding the location of data centers where team data
// resides (i.e. all data have been migrated according to the restriction
// placed).
type DataPlacementRestrictionSatisfyPolicyDetails struct {
	// PlacementRestriction : Placement restriction.
	PlacementRestriction *PlacementRestriction `json:"placement_restriction"`
}

// NewDataPlacementRestrictionSatisfyPolicyDetails returns a new DataPlacementRestrictionSatisfyPolicyDetails instance
func NewDataPlacementRestrictionSatisfyPolicyDetails(PlacementRestriction *PlacementRestriction) *DataPlacementRestrictionSatisfyPolicyDetails {
	s := new(DataPlacementRestrictionSatisfyPolicyDetails)
	s.PlacementRestriction = PlacementRestriction
	return s
}

// SessionLogInfo : Session's logged information.
type SessionLogInfo struct {
	// SessionId : Session ID. Might be missing due to historical data gap.
	SessionId string `json:"session_id,omitempty"`
}

// NewSessionLogInfo returns a new SessionLogInfo instance
func NewSessionLogInfo() *SessionLogInfo {
	s := new(SessionLogInfo)
	return s
}

// IsSessionLogInfo is the interface type for SessionLogInfo and its subtypes
type IsSessionLogInfo interface {
	IsSessionLogInfo()
}

// IsSessionLogInfo implements the IsSessionLogInfo interface
func (u *SessionLogInfo) IsSessionLogInfo() {}

type sessionLogInfoUnion struct {
	dropbox.Tagged
	// Web : has no documentation (yet)
	Web *WebSessionLogInfo `json:"web,omitempty"`
	// Desktop : has no documentation (yet)
	Desktop *DesktopSessionLogInfo `json:"desktop,omitempty"`
	// Mobile : has no documentation (yet)
	Mobile *MobileSessionLogInfo `json:"mobile,omitempty"`
}

// Valid tag values for SessionLogInfo
const (
	SessionLogInfoWeb     = "web"
	SessionLogInfoDesktop = "desktop"
	SessionLogInfoMobile  = "mobile"
)

// UnmarshalJSON deserializes into a sessionLogInfoUnion instance
func (u *sessionLogInfoUnion) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Web : has no documentation (yet)
		Web json.RawMessage `json:"web,omitempty"`
		// Desktop : has no documentation (yet)
		Desktop json.RawMessage `json:"desktop,omitempty"`
		// Mobile : has no documentation (yet)
		Mobile json.RawMessage `json:"mobile,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "web":
		err = json.Unmarshal(body, &u.Web)

		if err != nil {
			return err
		}
	case "desktop":
		err = json.Unmarshal(body, &u.Desktop)

		if err != nil {
			return err
		}
	case "mobile":
		err = json.Unmarshal(body, &u.Mobile)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsSessionLogInfoFromJSON converts JSON to a concrete IsSessionLogInfo instance
func IsSessionLogInfoFromJSON(data []byte) (IsSessionLogInfo, error) {
	var t sessionLogInfoUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "web":
		return t.Web, nil

	case "desktop":
		return t.Desktop, nil

	case "mobile":
		return t.Mobile, nil

	}
	return nil, nil
}

// DesktopSessionLogInfo : Desktop session.
type DesktopSessionLogInfo struct {
	SessionLogInfo
}

// NewDesktopSessionLogInfo returns a new DesktopSessionLogInfo instance
func NewDesktopSessionLogInfo() *DesktopSessionLogInfo {
	s := new(DesktopSessionLogInfo)
	return s
}

// DeviceApprovalsChangeDesktopPolicyDetails : Set or removed a limit on the
// number of computers each team member can link to their work Dropbox account.
type DeviceApprovalsChangeDesktopPolicyDetails struct {
	// NewValue : New desktop device approvals policy. Might be missing due to
	// historical data gap.
	NewValue *DeviceApprovalsPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous desktop device approvals policy. Might be
	// missing due to historical data gap.
	PreviousValue *DeviceApprovalsPolicy `json:"previous_value,omitempty"`
}

// NewDeviceApprovalsChangeDesktopPolicyDetails returns a new DeviceApprovalsChangeDesktopPolicyDetails instance
func NewDeviceApprovalsChangeDesktopPolicyDetails() *DeviceApprovalsChangeDesktopPolicyDetails {
	s := new(DeviceApprovalsChangeDesktopPolicyDetails)
	return s
}

// DeviceApprovalsChangeMobilePolicyDetails : Set or removed a limit on the
// number of mobiles devices each team member can link to their work Dropbox
// account.
type DeviceApprovalsChangeMobilePolicyDetails struct {
	// NewValue : New mobile device approvals policy. Might be missing due to
	// historical data gap.
	NewValue *DeviceApprovalsPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous mobile device approvals policy. Might be missing
	// due to historical data gap.
	PreviousValue *DeviceApprovalsPolicy `json:"previous_value,omitempty"`
}

// NewDeviceApprovalsChangeMobilePolicyDetails returns a new DeviceApprovalsChangeMobilePolicyDetails instance
func NewDeviceApprovalsChangeMobilePolicyDetails() *DeviceApprovalsChangeMobilePolicyDetails {
	s := new(DeviceApprovalsChangeMobilePolicyDetails)
	return s
}

// DeviceApprovalsChangeOverageActionDetails : Changed the action taken when a
// team member is already over the limits (e.g when they join the team, an admin
// lowers limits, etc.).
type DeviceApprovalsChangeOverageActionDetails struct {
	// NewValue : New over the limits policy. Might be missing due to historical
	// data gap.
	NewValue *DeviceApprovalsRolloutPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous over the limit policy. Might be missing due to
	// historical data gap.
	PreviousValue *DeviceApprovalsRolloutPolicy `json:"previous_value,omitempty"`
}

// NewDeviceApprovalsChangeOverageActionDetails returns a new DeviceApprovalsChangeOverageActionDetails instance
func NewDeviceApprovalsChangeOverageActionDetails() *DeviceApprovalsChangeOverageActionDetails {
	s := new(DeviceApprovalsChangeOverageActionDetails)
	return s
}

// DeviceApprovalsChangeUnlinkActionDetails : Changed the action taken with
// respect to approval limits when a team member unlinks an approved device.
type DeviceApprovalsChangeUnlinkActionDetails struct {
	// NewValue : New device unlink policy. Might be missing due to historical
	// data gap.
	NewValue *DeviceUnlinkPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous device unlink policy. Might be missing due to
	// historical data gap.
	PreviousValue *DeviceUnlinkPolicy `json:"previous_value,omitempty"`
}

// NewDeviceApprovalsChangeUnlinkActionDetails returns a new DeviceApprovalsChangeUnlinkActionDetails instance
func NewDeviceApprovalsChangeUnlinkActionDetails() *DeviceApprovalsChangeUnlinkActionDetails {
	s := new(DeviceApprovalsChangeUnlinkActionDetails)
	return s
}

// DeviceApprovalsPolicy : has no documentation (yet)
type DeviceApprovalsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for DeviceApprovalsPolicy
const (
	DeviceApprovalsPolicyUnlimited = "unlimited"
	DeviceApprovalsPolicyLimited   = "limited"
	DeviceApprovalsPolicyOther     = "other"
)

// DeviceApprovalsRolloutPolicy : has no documentation (yet)
type DeviceApprovalsRolloutPolicy struct {
	dropbox.Tagged
}

// Valid tag values for DeviceApprovalsRolloutPolicy
const (
	DeviceApprovalsRolloutPolicyRemoveOldest = "remove_oldest"
	DeviceApprovalsRolloutPolicyRemoveAll    = "remove_all"
	DeviceApprovalsRolloutPolicyAddException = "add_exception"
	DeviceApprovalsRolloutPolicyOther        = "other"
)

// DeviceChangeIpDesktopDetails : IP address associated with active desktop
// session changed.
type DeviceChangeIpDesktopDetails struct {
	// DeviceInfo : Device information.
	DeviceInfo *DeviceLogInfo `json:"device_info"`
}

// NewDeviceChangeIpDesktopDetails returns a new DeviceChangeIpDesktopDetails instance
func NewDeviceChangeIpDesktopDetails(DeviceInfo *DeviceLogInfo) *DeviceChangeIpDesktopDetails {
	s := new(DeviceChangeIpDesktopDetails)
	s.DeviceInfo = DeviceInfo
	return s
}

// DeviceChangeIpMobileDetails : IP address associated with active mobile
// session changed.
type DeviceChangeIpMobileDetails struct {
	// DeviceInfo : Device information.
	DeviceInfo *DeviceLogInfo `json:"device_info"`
}

// NewDeviceChangeIpMobileDetails returns a new DeviceChangeIpMobileDetails instance
func NewDeviceChangeIpMobileDetails(DeviceInfo *DeviceLogInfo) *DeviceChangeIpMobileDetails {
	s := new(DeviceChangeIpMobileDetails)
	s.DeviceInfo = DeviceInfo
	return s
}

// DeviceChangeIpWebDetails : IP address associated with active Web session
// changed.
type DeviceChangeIpWebDetails struct {
	// DeviceInfo : Device information. Might be missing due to historical data
	// gap.
	DeviceInfo *DeviceLogInfo `json:"device_info,omitempty"`
	// UserAgent : Web browser name.
	UserAgent string `json:"user_agent"`
}

// NewDeviceChangeIpWebDetails returns a new DeviceChangeIpWebDetails instance
func NewDeviceChangeIpWebDetails(UserAgent string) *DeviceChangeIpWebDetails {
	s := new(DeviceChangeIpWebDetails)
	s.UserAgent = UserAgent
	return s
}

// DeviceDeleteOnUnlinkFailDetails : Failed to delete all files from an unlinked
// device.
type DeviceDeleteOnUnlinkFailDetails struct {
	// DeviceInfo : Device information.
	DeviceInfo *DeviceLogInfo `json:"device_info"`
	// NumFailures : The number of times that remote file deletion failed.
	NumFailures int64 `json:"num_failures"`
}

// NewDeviceDeleteOnUnlinkFailDetails returns a new DeviceDeleteOnUnlinkFailDetails instance
func NewDeviceDeleteOnUnlinkFailDetails(DeviceInfo *DeviceLogInfo, NumFailures int64) *DeviceDeleteOnUnlinkFailDetails {
	s := new(DeviceDeleteOnUnlinkFailDetails)
	s.DeviceInfo = DeviceInfo
	s.NumFailures = NumFailures
	return s
}

// DeviceDeleteOnUnlinkSuccessDetails : Deleted all files from an unlinked
// device.
type DeviceDeleteOnUnlinkSuccessDetails struct {
	// DeviceInfo : Device information.
	DeviceInfo *DeviceLogInfo `json:"device_info"`
}

// NewDeviceDeleteOnUnlinkSuccessDetails returns a new DeviceDeleteOnUnlinkSuccessDetails instance
func NewDeviceDeleteOnUnlinkSuccessDetails(DeviceInfo *DeviceLogInfo) *DeviceDeleteOnUnlinkSuccessDetails {
	s := new(DeviceDeleteOnUnlinkSuccessDetails)
	s.DeviceInfo = DeviceInfo
	return s
}

// DeviceLinkFailDetails : Failed to link a device.
type DeviceLinkFailDetails struct {
	// DeviceInfo : Device information. Might be missing due to historical data
	// gap.
	DeviceInfo *DeviceLogInfo `json:"device_info,omitempty"`
	// DeviceType : A description of the device used while user approval
	// blocked.
	DeviceType *DeviceType `json:"device_type"`
}

// NewDeviceLinkFailDetails returns a new DeviceLinkFailDetails instance
func NewDeviceLinkFailDetails(DeviceType *DeviceType) *DeviceLinkFailDetails {
	s := new(DeviceLinkFailDetails)
	s.DeviceType = DeviceType
	return s
}

// DeviceLinkSuccessDetails : Linked a device.
type DeviceLinkSuccessDetails struct {
	// DeviceInfo : Device information.
	DeviceInfo *DeviceLogInfo `json:"device_info"`
}

// NewDeviceLinkSuccessDetails returns a new DeviceLinkSuccessDetails instance
func NewDeviceLinkSuccessDetails(DeviceInfo *DeviceLogInfo) *DeviceLinkSuccessDetails {
	s := new(DeviceLinkSuccessDetails)
	s.DeviceInfo = DeviceInfo
	return s
}

// DeviceLogInfo : Device's logged information.
type DeviceLogInfo struct {
	// DeviceId : Device unique id. Might be missing due to historical data gap.
	DeviceId string `json:"device_id,omitempty"`
	// DisplayName : Device display name. Might be missing due to historical
	// data gap.
	DisplayName string `json:"display_name,omitempty"`
	// IsEmmManaged : True if this device is emm managed, false otherwise. Might
	// be missing due to historical data gap.
	IsEmmManaged bool `json:"is_emm_managed,omitempty"`
	// Platform : Device platform name. Might be missing due to historical data
	// gap.
	Platform string `json:"platform,omitempty"`
	// MacAddress : Device mac address. Might be missing due to historical data
	// gap.
	MacAddress string `json:"mac_address,omitempty"`
	// OsVersion : Device OS version. Might be missing due to historical data
	// gap.
	OsVersion string `json:"os_version,omitempty"`
	// DeviceType : Device type. Might be missing due to historical data gap.
	DeviceType string `json:"device_type,omitempty"`
	// IpAddress : IP address. Might be missing due to historical data gap.
	IpAddress string `json:"ip_address,omitempty"`
	// LastActivity : Last activity. Might be missing due to historical data
	// gap.
	LastActivity string `json:"last_activity,omitempty"`
	// AppVersion : Linking app version. Might be missing due to historical data
	// gap.
	AppVersion string `json:"app_version,omitempty"`
}

// NewDeviceLogInfo returns a new DeviceLogInfo instance
func NewDeviceLogInfo() *DeviceLogInfo {
	s := new(DeviceLogInfo)
	return s
}

// DeviceManagementDisabledDetails : Disable Device Management.
type DeviceManagementDisabledDetails struct {
}

// NewDeviceManagementDisabledDetails returns a new DeviceManagementDisabledDetails instance
func NewDeviceManagementDisabledDetails() *DeviceManagementDisabledDetails {
	s := new(DeviceManagementDisabledDetails)
	return s
}

// DeviceManagementEnabledDetails : Enable Device Management.
type DeviceManagementEnabledDetails struct {
}

// NewDeviceManagementEnabledDetails returns a new DeviceManagementEnabledDetails instance
func NewDeviceManagementEnabledDetails() *DeviceManagementEnabledDetails {
	s := new(DeviceManagementEnabledDetails)
	return s
}

// DeviceType : has no documentation (yet)
type DeviceType struct {
	dropbox.Tagged
}

// Valid tag values for DeviceType
const (
	DeviceTypeDesktop = "desktop"
	DeviceTypeMobile  = "mobile"
	DeviceTypeOther   = "other"
)

// DeviceUnlinkDetails : Disconnected a device.
type DeviceUnlinkDetails struct {
	// DeviceInfo : Device information.
	DeviceInfo *DeviceLogInfo `json:"device_info"`
	// DeleteData : True if the user requested to delete data after device
	// unlink, false otherwise.
	DeleteData bool `json:"delete_data"`
}

// NewDeviceUnlinkDetails returns a new DeviceUnlinkDetails instance
func NewDeviceUnlinkDetails(DeviceInfo *DeviceLogInfo, DeleteData bool) *DeviceUnlinkDetails {
	s := new(DeviceUnlinkDetails)
	s.DeviceInfo = DeviceInfo
	s.DeleteData = DeleteData
	return s
}

// DeviceUnlinkPolicy : has no documentation (yet)
type DeviceUnlinkPolicy struct {
	dropbox.Tagged
}

// Valid tag values for DeviceUnlinkPolicy
const (
	DeviceUnlinkPolicyRemove = "remove"
	DeviceUnlinkPolicyKeep   = "keep"
	DeviceUnlinkPolicyOther  = "other"
)

// DisabledDomainInvitesDetails : Disabled domain invites.
type DisabledDomainInvitesDetails struct {
}

// NewDisabledDomainInvitesDetails returns a new DisabledDomainInvitesDetails instance
func NewDisabledDomainInvitesDetails() *DisabledDomainInvitesDetails {
	s := new(DisabledDomainInvitesDetails)
	return s
}

// DomainInvitesApproveRequestToJoinTeamDetails : Approved a member's request to
// join the team.
type DomainInvitesApproveRequestToJoinTeamDetails struct {
}

// NewDomainInvitesApproveRequestToJoinTeamDetails returns a new DomainInvitesApproveRequestToJoinTeamDetails instance
func NewDomainInvitesApproveRequestToJoinTeamDetails() *DomainInvitesApproveRequestToJoinTeamDetails {
	s := new(DomainInvitesApproveRequestToJoinTeamDetails)
	return s
}

// DomainInvitesDeclineRequestToJoinTeamDetails : Declined a user's request to
// join the team.
type DomainInvitesDeclineRequestToJoinTeamDetails struct {
}

// NewDomainInvitesDeclineRequestToJoinTeamDetails returns a new DomainInvitesDeclineRequestToJoinTeamDetails instance
func NewDomainInvitesDeclineRequestToJoinTeamDetails() *DomainInvitesDeclineRequestToJoinTeamDetails {
	s := new(DomainInvitesDeclineRequestToJoinTeamDetails)
	return s
}

// DomainInvitesEmailExistingUsersDetails : Sent domain invites to existing
// domain accounts.
type DomainInvitesEmailExistingUsersDetails struct {
	// DomainName : Domain names.
	DomainName []string `json:"domain_name"`
	// NumRecipients : Number of recipients.
	NumRecipients uint64 `json:"num_recipients"`
}

// NewDomainInvitesEmailExistingUsersDetails returns a new DomainInvitesEmailExistingUsersDetails instance
func NewDomainInvitesEmailExistingUsersDetails(DomainName []string, NumRecipients uint64) *DomainInvitesEmailExistingUsersDetails {
	s := new(DomainInvitesEmailExistingUsersDetails)
	s.DomainName = DomainName
	s.NumRecipients = NumRecipients
	return s
}

// DomainInvitesRequestToJoinTeamDetails : Asked to join the team.
type DomainInvitesRequestToJoinTeamDetails struct {
}

// NewDomainInvitesRequestToJoinTeamDetails returns a new DomainInvitesRequestToJoinTeamDetails instance
func NewDomainInvitesRequestToJoinTeamDetails() *DomainInvitesRequestToJoinTeamDetails {
	s := new(DomainInvitesRequestToJoinTeamDetails)
	return s
}

// DomainInvitesSetInviteNewUserPrefToNoDetails : Turned off u201cAutomatically
// invite new usersu201d.
type DomainInvitesSetInviteNewUserPrefToNoDetails struct {
}

// NewDomainInvitesSetInviteNewUserPrefToNoDetails returns a new DomainInvitesSetInviteNewUserPrefToNoDetails instance
func NewDomainInvitesSetInviteNewUserPrefToNoDetails() *DomainInvitesSetInviteNewUserPrefToNoDetails {
	s := new(DomainInvitesSetInviteNewUserPrefToNoDetails)
	return s
}

// DomainInvitesSetInviteNewUserPrefToYesDetails : Turned on u201cAutomatically
// invite new usersu201d.
type DomainInvitesSetInviteNewUserPrefToYesDetails struct {
}

// NewDomainInvitesSetInviteNewUserPrefToYesDetails returns a new DomainInvitesSetInviteNewUserPrefToYesDetails instance
func NewDomainInvitesSetInviteNewUserPrefToYesDetails() *DomainInvitesSetInviteNewUserPrefToYesDetails {
	s := new(DomainInvitesSetInviteNewUserPrefToYesDetails)
	return s
}

// DomainVerificationAddDomainFailDetails : Failed to verify a domain belonging
// to the team.
type DomainVerificationAddDomainFailDetails struct {
	// DomainName : Domain name.
	DomainName string `json:"domain_name"`
	// VerificationMethod : Domain name verification method. Might be missing
	// due to historical data gap.
	VerificationMethod string `json:"verification_method,omitempty"`
}

// NewDomainVerificationAddDomainFailDetails returns a new DomainVerificationAddDomainFailDetails instance
func NewDomainVerificationAddDomainFailDetails(DomainName string) *DomainVerificationAddDomainFailDetails {
	s := new(DomainVerificationAddDomainFailDetails)
	s.DomainName = DomainName
	return s
}

// DomainVerificationAddDomainSuccessDetails : Verified a domain belonging to
// the team.
type DomainVerificationAddDomainSuccessDetails struct {
	// DomainNames : Domain names.
	DomainNames []string `json:"domain_names"`
	// VerificationMethod : Domain name verification method. Might be missing
	// due to historical data gap.
	VerificationMethod string `json:"verification_method,omitempty"`
}

// NewDomainVerificationAddDomainSuccessDetails returns a new DomainVerificationAddDomainSuccessDetails instance
func NewDomainVerificationAddDomainSuccessDetails(DomainNames []string) *DomainVerificationAddDomainSuccessDetails {
	s := new(DomainVerificationAddDomainSuccessDetails)
	s.DomainNames = DomainNames
	return s
}

// DomainVerificationRemoveDomainDetails : Removed a domain from the list of
// verified domains belonging to the team.
type DomainVerificationRemoveDomainDetails struct {
	// DomainNames : Domain names.
	DomainNames []string `json:"domain_names"`
	// VerificationMethod : Domain name verification method. Might be missing
	// due to historical data gap.
	VerificationMethod string `json:"verification_method,omitempty"`
}

// NewDomainVerificationRemoveDomainDetails returns a new DomainVerificationRemoveDomainDetails instance
func NewDomainVerificationRemoveDomainDetails(DomainNames []string) *DomainVerificationRemoveDomainDetails {
	s := new(DomainVerificationRemoveDomainDetails)
	s.DomainNames = DomainNames
	return s
}

// DurationLogInfo : Represents a time duration: unit and amount
type DurationLogInfo struct {
	// Unit : Time unit.
	Unit *TimeUnit `json:"unit"`
	// Amount : Amount of time.
	Amount uint64 `json:"amount"`
}

// NewDurationLogInfo returns a new DurationLogInfo instance
func NewDurationLogInfo(Unit *TimeUnit, Amount uint64) *DurationLogInfo {
	s := new(DurationLogInfo)
	s.Unit = Unit
	s.Amount = Amount
	return s
}

// EmmAddExceptionDetails : Added an exception for one or more team members to
// optionally use the regular Dropbox app when EMM is enabled.
type EmmAddExceptionDetails struct {
}

// NewEmmAddExceptionDetails returns a new EmmAddExceptionDetails instance
func NewEmmAddExceptionDetails() *EmmAddExceptionDetails {
	s := new(EmmAddExceptionDetails)
	return s
}

// EmmChangePolicyDetails : Enabled or disabled enterprise mobility management
// for team members.
type EmmChangePolicyDetails struct {
	// NewValue : New enterprise mobility management policy.
	NewValue *EmmPolicy `json:"new_value"`
	// PreviousValue : Previous enterprise mobility management policy. Might be
	// missing due to historical data gap.
	PreviousValue *EmmPolicy `json:"previous_value,omitempty"`
}

// NewEmmChangePolicyDetails returns a new EmmChangePolicyDetails instance
func NewEmmChangePolicyDetails(NewValue *EmmPolicy) *EmmChangePolicyDetails {
	s := new(EmmChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// EmmCreateExceptionsReportDetails : EMM excluded users report created.
type EmmCreateExceptionsReportDetails struct {
}

// NewEmmCreateExceptionsReportDetails returns a new EmmCreateExceptionsReportDetails instance
func NewEmmCreateExceptionsReportDetails() *EmmCreateExceptionsReportDetails {
	s := new(EmmCreateExceptionsReportDetails)
	return s
}

// EmmCreateUsageReportDetails : EMM mobile app usage report created.
type EmmCreateUsageReportDetails struct {
}

// NewEmmCreateUsageReportDetails returns a new EmmCreateUsageReportDetails instance
func NewEmmCreateUsageReportDetails() *EmmCreateUsageReportDetails {
	s := new(EmmCreateUsageReportDetails)
	return s
}

// EmmLoginSuccessDetails : Signed in using the Dropbox EMM app.
type EmmLoginSuccessDetails struct {
}

// NewEmmLoginSuccessDetails returns a new EmmLoginSuccessDetails instance
func NewEmmLoginSuccessDetails() *EmmLoginSuccessDetails {
	s := new(EmmLoginSuccessDetails)
	return s
}

// EmmPolicy : Enterprise mobility management policy
type EmmPolicy struct {
	dropbox.Tagged
}

// Valid tag values for EmmPolicy
const (
	EmmPolicyDisabled = "disabled"
	EmmPolicyOptional = "optional"
	EmmPolicyRequired = "required"
	EmmPolicyOther    = "other"
)

// EmmRefreshAuthTokenDetails : Refreshed the auth token used for setting up
// enterprise mobility management.
type EmmRefreshAuthTokenDetails struct {
}

// NewEmmRefreshAuthTokenDetails returns a new EmmRefreshAuthTokenDetails instance
func NewEmmRefreshAuthTokenDetails() *EmmRefreshAuthTokenDetails {
	s := new(EmmRefreshAuthTokenDetails)
	return s
}

// EmmRemoveExceptionDetails : Removed an exception for one or more team members
// to optionally use the regular Dropbox app when EMM is enabled.
type EmmRemoveExceptionDetails struct {
}

// NewEmmRemoveExceptionDetails returns a new EmmRemoveExceptionDetails instance
func NewEmmRemoveExceptionDetails() *EmmRemoveExceptionDetails {
	s := new(EmmRemoveExceptionDetails)
	return s
}

// EnabledDomainInvitesDetails : Enabled domain invites.
type EnabledDomainInvitesDetails struct {
}

// NewEnabledDomainInvitesDetails returns a new EnabledDomainInvitesDetails instance
func NewEnabledDomainInvitesDetails() *EnabledDomainInvitesDetails {
	s := new(EnabledDomainInvitesDetails)
	return s
}

// EventCategory : Category of events in event audit log.
type EventCategory struct {
	dropbox.Tagged
}

// Valid tag values for EventCategory
const (
	EventCategoryAccountCapture  = "account_capture"
	EventCategoryAdministration  = "administration"
	EventCategoryApps            = "apps"
	EventCategoryAuthentication  = "authentication"
	EventCategoryComments        = "comments"
	EventCategoryContentAccess   = "content_access"
	EventCategoryDevices         = "devices"
	EventCategoryDeviceApprovals = "device_approvals"
	EventCategoryDomains         = "domains"
	EventCategoryEmm             = "emm"
	EventCategoryErrors          = "errors"
	EventCategoryFiles           = "files"
	EventCategoryFileOperations  = "file_operations"
	EventCategoryFileRequests    = "file_requests"
	EventCategoryGroups          = "groups"
	EventCategoryLogins          = "logins"
	EventCategoryMembers         = "members"
	EventCategoryPaper           = "paper"
	EventCategoryPasswords       = "passwords"
	EventCategoryReports         = "reports"
	EventCategorySessions        = "sessions"
	EventCategorySharedFiles     = "shared_files"
	EventCategorySharedFolders   = "shared_folders"
	EventCategorySharedLinks     = "shared_links"
	EventCategorySharing         = "sharing"
	EventCategorySharingPolicies = "sharing_policies"
	EventCategorySso             = "sso"
	EventCategoryTeamFolders     = "team_folders"
	EventCategoryTeamPolicies    = "team_policies"
	EventCategoryTeamProfile     = "team_profile"
	EventCategoryTfa             = "tfa"
	EventCategoryOther           = "other"
)

// EventDetails : Additional fields depending on the event type.
type EventDetails struct {
	dropbox.Tagged
	// MemberChangeMembershipTypeDetails : Changed the membership type (limited
	// vs full) for team member.
	MemberChangeMembershipTypeDetails *MemberChangeMembershipTypeDetails `json:"member_change_membership_type_details,omitempty"`
	// MemberPermanentlyDeleteAccountContentsDetails : Permanently deleted
	// contents of a removed team member account.
	MemberPermanentlyDeleteAccountContentsDetails *MemberPermanentlyDeleteAccountContentsDetails `json:"member_permanently_delete_account_contents_details,omitempty"`
	// MemberSpaceLimitsChangeStatusDetails : Changed the status with respect to
	// whether the team member is under or over storage quota specified by
	// policy.
	MemberSpaceLimitsChangeStatusDetails *MemberSpaceLimitsChangeStatusDetails `json:"member_space_limits_change_status_details,omitempty"`
	// MemberTransferAccountContentsDetails : Transferred contents of a removed
	// team member account to another member.
	MemberTransferAccountContentsDetails *MemberTransferAccountContentsDetails `json:"member_transfer_account_contents_details,omitempty"`
	// PaperEnabledUsersGroupAdditionDetails : Users added to Paper enabled
	// users list.
	PaperEnabledUsersGroupAdditionDetails *PaperEnabledUsersGroupAdditionDetails `json:"paper_enabled_users_group_addition_details,omitempty"`
	// PaperEnabledUsersGroupRemovalDetails : Users removed from Paper enabled
	// users list.
	PaperEnabledUsersGroupRemovalDetails *PaperEnabledUsersGroupRemovalDetails `json:"paper_enabled_users_group_removal_details,omitempty"`
	// PaperExternalViewAllowDetails : Paper external sharing policy changed:
	// anyone.
	PaperExternalViewAllowDetails *PaperExternalViewAllowDetails `json:"paper_external_view_allow_details,omitempty"`
	// PaperExternalViewDefaultTeamDetails : Paper external sharing policy
	// changed: default team.
	PaperExternalViewDefaultTeamDetails *PaperExternalViewDefaultTeamDetails `json:"paper_external_view_default_team_details,omitempty"`
	// PaperExternalViewForbidDetails : Paper external sharing policy changed:
	// team-only.
	PaperExternalViewForbidDetails *PaperExternalViewForbidDetails `json:"paper_external_view_forbid_details,omitempty"`
	// SfExternalInviteWarnDetails : Admin settings: team members see a warning
	// before sharing folders outside the team (DEPRECATED FEATURE).
	SfExternalInviteWarnDetails *SfExternalInviteWarnDetails `json:"sf_external_invite_warn_details,omitempty"`
	// TeamMergeFromDetails : Merged another team into this team.
	TeamMergeFromDetails *TeamMergeFromDetails `json:"team_merge_from_details,omitempty"`
	// TeamMergeToDetails : Merged this team into another team.
	TeamMergeToDetails *TeamMergeToDetails `json:"team_merge_to_details,omitempty"`
	// AppLinkTeamDetails : Linked an app for team.
	AppLinkTeamDetails *AppLinkTeamDetails `json:"app_link_team_details,omitempty"`
	// AppLinkUserDetails : Linked an app for team member.
	AppLinkUserDetails *AppLinkUserDetails `json:"app_link_user_details,omitempty"`
	// AppUnlinkTeamDetails : Unlinked an app for team.
	AppUnlinkTeamDetails *AppUnlinkTeamDetails `json:"app_unlink_team_details,omitempty"`
	// AppUnlinkUserDetails : Unlinked an app for team member.
	AppUnlinkUserDetails *AppUnlinkUserDetails `json:"app_unlink_user_details,omitempty"`
	// DeviceChangeIpDesktopDetails : IP address associated with active desktop
	// session changed.
	DeviceChangeIpDesktopDetails *DeviceChangeIpDesktopDetails `json:"device_change_ip_desktop_details,omitempty"`
	// DeviceChangeIpMobileDetails : IP address associated with active mobile
	// session changed.
	DeviceChangeIpMobileDetails *DeviceChangeIpMobileDetails `json:"device_change_ip_mobile_details,omitempty"`
	// DeviceChangeIpWebDetails : IP address associated with active Web session
	// changed.
	DeviceChangeIpWebDetails *DeviceChangeIpWebDetails `json:"device_change_ip_web_details,omitempty"`
	// DeviceDeleteOnUnlinkFailDetails : Failed to delete all files from an
	// unlinked device.
	DeviceDeleteOnUnlinkFailDetails *DeviceDeleteOnUnlinkFailDetails `json:"device_delete_on_unlink_fail_details,omitempty"`
	// DeviceDeleteOnUnlinkSuccessDetails : Deleted all files from an unlinked
	// device.
	DeviceDeleteOnUnlinkSuccessDetails *DeviceDeleteOnUnlinkSuccessDetails `json:"device_delete_on_unlink_success_details,omitempty"`
	// DeviceLinkFailDetails : Failed to link a device.
	DeviceLinkFailDetails *DeviceLinkFailDetails `json:"device_link_fail_details,omitempty"`
	// DeviceLinkSuccessDetails : Linked a device.
	DeviceLinkSuccessDetails *DeviceLinkSuccessDetails `json:"device_link_success_details,omitempty"`
	// DeviceManagementDisabledDetails : Disable Device Management.
	DeviceManagementDisabledDetails *DeviceManagementDisabledDetails `json:"device_management_disabled_details,omitempty"`
	// DeviceManagementEnabledDetails : Enable Device Management.
	DeviceManagementEnabledDetails *DeviceManagementEnabledDetails `json:"device_management_enabled_details,omitempty"`
	// DeviceUnlinkDetails : Disconnected a device.
	DeviceUnlinkDetails *DeviceUnlinkDetails `json:"device_unlink_details,omitempty"`
	// EmmRefreshAuthTokenDetails : Refreshed the auth token used for setting up
	// enterprise mobility management.
	EmmRefreshAuthTokenDetails *EmmRefreshAuthTokenDetails `json:"emm_refresh_auth_token_details,omitempty"`
	// AccountCaptureChangeAvailabilityDetails : Granted or revoked the option
	// to enable account capture on domains belonging to the team.
	AccountCaptureChangeAvailabilityDetails *AccountCaptureChangeAvailabilityDetails `json:"account_capture_change_availability_details,omitempty"`
	// AccountCaptureMigrateAccountDetails : Account captured user migrated
	// their account to the team.
	AccountCaptureMigrateAccountDetails *AccountCaptureMigrateAccountDetails `json:"account_capture_migrate_account_details,omitempty"`
	// AccountCaptureRelinquishAccountDetails : Account captured user
	// relinquished their account by changing the email address associated with
	// it.
	AccountCaptureRelinquishAccountDetails *AccountCaptureRelinquishAccountDetails `json:"account_capture_relinquish_account_details,omitempty"`
	// DisabledDomainInvitesDetails : Disabled domain invites.
	DisabledDomainInvitesDetails *DisabledDomainInvitesDetails `json:"disabled_domain_invites_details,omitempty"`
	// DomainInvitesApproveRequestToJoinTeamDetails : Approved a member's
	// request to join the team.
	DomainInvitesApproveRequestToJoinTeamDetails *DomainInvitesApproveRequestToJoinTeamDetails `json:"domain_invites_approve_request_to_join_team_details,omitempty"`
	// DomainInvitesDeclineRequestToJoinTeamDetails : Declined a user's request
	// to join the team.
	DomainInvitesDeclineRequestToJoinTeamDetails *DomainInvitesDeclineRequestToJoinTeamDetails `json:"domain_invites_decline_request_to_join_team_details,omitempty"`
	// DomainInvitesEmailExistingUsersDetails : Sent domain invites to existing
	// domain accounts.
	DomainInvitesEmailExistingUsersDetails *DomainInvitesEmailExistingUsersDetails `json:"domain_invites_email_existing_users_details,omitempty"`
	// DomainInvitesRequestToJoinTeamDetails : Asked to join the team.
	DomainInvitesRequestToJoinTeamDetails *DomainInvitesRequestToJoinTeamDetails `json:"domain_invites_request_to_join_team_details,omitempty"`
	// DomainInvitesSetInviteNewUserPrefToNoDetails : Turned off
	// u201cAutomatically invite new usersu201d.
	DomainInvitesSetInviteNewUserPrefToNoDetails *DomainInvitesSetInviteNewUserPrefToNoDetails `json:"domain_invites_set_invite_new_user_pref_to_no_details,omitempty"`
	// DomainInvitesSetInviteNewUserPrefToYesDetails : Turned on
	// u201cAutomatically invite new usersu201d.
	DomainInvitesSetInviteNewUserPrefToYesDetails *DomainInvitesSetInviteNewUserPrefToYesDetails `json:"domain_invites_set_invite_new_user_pref_to_yes_details,omitempty"`
	// DomainVerificationAddDomainFailDetails : Failed to verify a domain
	// belonging to the team.
	DomainVerificationAddDomainFailDetails *DomainVerificationAddDomainFailDetails `json:"domain_verification_add_domain_fail_details,omitempty"`
	// DomainVerificationAddDomainSuccessDetails : Verified a domain belonging
	// to the team.
	DomainVerificationAddDomainSuccessDetails *DomainVerificationAddDomainSuccessDetails `json:"domain_verification_add_domain_success_details,omitempty"`
	// DomainVerificationRemoveDomainDetails : Removed a domain from the list of
	// verified domains belonging to the team.
	DomainVerificationRemoveDomainDetails *DomainVerificationRemoveDomainDetails `json:"domain_verification_remove_domain_details,omitempty"`
	// EnabledDomainInvitesDetails : Enabled domain invites.
	EnabledDomainInvitesDetails *EnabledDomainInvitesDetails `json:"enabled_domain_invites_details,omitempty"`
	// CreateFolderDetails : Created folders.
	CreateFolderDetails *CreateFolderDetails `json:"create_folder_details,omitempty"`
	// FileAddDetails : Added files and/or folders.
	FileAddDetails *FileAddDetails `json:"file_add_details,omitempty"`
	// FileCopyDetails : Copied files and/or folders.
	FileCopyDetails *FileCopyDetails `json:"file_copy_details,omitempty"`
	// FileDeleteDetails : Deleted files and/or folders.
	FileDeleteDetails *FileDeleteDetails `json:"file_delete_details,omitempty"`
	// FileDownloadDetails : Downloaded files and/or folders.
	FileDownloadDetails *FileDownloadDetails `json:"file_download_details,omitempty"`
	// FileEditDetails : Edited files.
	FileEditDetails *FileEditDetails `json:"file_edit_details,omitempty"`
	// FileGetCopyReferenceDetails : Create a copy reference to a file or
	// folder.
	FileGetCopyReferenceDetails *FileGetCopyReferenceDetails `json:"file_get_copy_reference_details,omitempty"`
	// FileMoveDetails : Moved files and/or folders.
	FileMoveDetails *FileMoveDetails `json:"file_move_details,omitempty"`
	// FilePermanentlyDeleteDetails : Permanently deleted files and/or folders.
	FilePermanentlyDeleteDetails *FilePermanentlyDeleteDetails `json:"file_permanently_delete_details,omitempty"`
	// FilePreviewDetails : Previewed files and/or folders.
	FilePreviewDetails *FilePreviewDetails `json:"file_preview_details,omitempty"`
	// FileRenameDetails : Renamed files and/or folders.
	FileRenameDetails *FileRenameDetails `json:"file_rename_details,omitempty"`
	// FileRestoreDetails : Restored deleted files and/or folders.
	FileRestoreDetails *FileRestoreDetails `json:"file_restore_details,omitempty"`
	// FileRevertDetails : Reverted files to a previous version.
	FileRevertDetails *FileRevertDetails `json:"file_revert_details,omitempty"`
	// FileRollbackChangesDetails : Rolled back file change location changes.
	FileRollbackChangesDetails *FileRollbackChangesDetails `json:"file_rollback_changes_details,omitempty"`
	// FileSaveCopyReferenceDetails : Save a file or folder using a copy
	// reference.
	FileSaveCopyReferenceDetails *FileSaveCopyReferenceDetails `json:"file_save_copy_reference_details,omitempty"`
	// FileRequestAddDeadlineDetails : Added a deadline to a file request.
	FileRequestAddDeadlineDetails *FileRequestAddDeadlineDetails `json:"file_request_add_deadline_details,omitempty"`
	// FileRequestChangeFolderDetails : Changed the file request folder.
	FileRequestChangeFolderDetails *FileRequestChangeFolderDetails `json:"file_request_change_folder_details,omitempty"`
	// FileRequestChangeTitleDetails : Change the file request title.
	FileRequestChangeTitleDetails *FileRequestChangeTitleDetails `json:"file_request_change_title_details,omitempty"`
	// FileRequestCloseDetails : Closed a file request.
	FileRequestCloseDetails *FileRequestCloseDetails `json:"file_request_close_details,omitempty"`
	// FileRequestCreateDetails : Created a file request.
	FileRequestCreateDetails *FileRequestCreateDetails `json:"file_request_create_details,omitempty"`
	// FileRequestReceiveFileDetails : Received files for a file request.
	FileRequestReceiveFileDetails *FileRequestReceiveFileDetails `json:"file_request_receive_file_details,omitempty"`
	// FileRequestRemoveDeadlineDetails : Removed the file request deadline.
	FileRequestRemoveDeadlineDetails *FileRequestRemoveDeadlineDetails `json:"file_request_remove_deadline_details,omitempty"`
	// FileRequestSendDetails : Sent file request to users via email.
	FileRequestSendDetails *FileRequestSendDetails `json:"file_request_send_details,omitempty"`
	// GroupAddExternalIdDetails : Added an external ID for group.
	GroupAddExternalIdDetails *GroupAddExternalIdDetails `json:"group_add_external_id_details,omitempty"`
	// GroupAddMemberDetails : Added team members to a group.
	GroupAddMemberDetails *GroupAddMemberDetails `json:"group_add_member_details,omitempty"`
	// GroupChangeExternalIdDetails : Changed the external ID for group.
	GroupChangeExternalIdDetails *GroupChangeExternalIdDetails `json:"group_change_external_id_details,omitempty"`
	// GroupChangeManagementTypeDetails : Changed group management type.
	GroupChangeManagementTypeDetails *GroupChangeManagementTypeDetails `json:"group_change_management_type_details,omitempty"`
	// GroupChangeMemberRoleDetails : Changed the manager permissions belonging
	// to a group member.
	GroupChangeMemberRoleDetails *GroupChangeMemberRoleDetails `json:"group_change_member_role_details,omitempty"`
	// GroupCreateDetails : Created a group.
	GroupCreateDetails *GroupCreateDetails `json:"group_create_details,omitempty"`
	// GroupDeleteDetails : Deleted a group.
	GroupDeleteDetails *GroupDeleteDetails `json:"group_delete_details,omitempty"`
	// GroupDescriptionUpdatedDetails : Updated a group.
	GroupDescriptionUpdatedDetails *GroupDescriptionUpdatedDetails `json:"group_description_updated_details,omitempty"`
	// GroupJoinPolicyUpdatedDetails : Updated a group join policy.
	GroupJoinPolicyUpdatedDetails *GroupJoinPolicyUpdatedDetails `json:"group_join_policy_updated_details,omitempty"`
	// GroupMovedDetails : Moved a group.
	GroupMovedDetails *GroupMovedDetails `json:"group_moved_details,omitempty"`
	// GroupRemoveExternalIdDetails : Removed the external ID for group.
	GroupRemoveExternalIdDetails *GroupRemoveExternalIdDetails `json:"group_remove_external_id_details,omitempty"`
	// GroupRemoveMemberDetails : Removed team members from a group.
	GroupRemoveMemberDetails *GroupRemoveMemberDetails `json:"group_remove_member_details,omitempty"`
	// GroupRenameDetails : Renamed a group.
	GroupRenameDetails *GroupRenameDetails `json:"group_rename_details,omitempty"`
	// EmmLoginSuccessDetails : Signed in using the Dropbox EMM app.
	EmmLoginSuccessDetails *EmmLoginSuccessDetails `json:"emm_login_success_details,omitempty"`
	// LogoutDetails : Signed out.
	LogoutDetails *LogoutDetails `json:"logout_details,omitempty"`
	// PasswordLoginFailDetails : Failed to sign in using a password.
	PasswordLoginFailDetails *PasswordLoginFailDetails `json:"password_login_fail_details,omitempty"`
	// PasswordLoginSuccessDetails : Signed in using a password.
	PasswordLoginSuccessDetails *PasswordLoginSuccessDetails `json:"password_login_success_details,omitempty"`
	// ResellerSupportSessionEndDetails : Ended reseller support session.
	ResellerSupportSessionEndDetails *ResellerSupportSessionEndDetails `json:"reseller_support_session_end_details,omitempty"`
	// ResellerSupportSessionStartDetails : Started reseller support session.
	ResellerSupportSessionStartDetails *ResellerSupportSessionStartDetails `json:"reseller_support_session_start_details,omitempty"`
	// SignInAsSessionEndDetails : Ended admin sign-in-as session.
	SignInAsSessionEndDetails *SignInAsSessionEndDetails `json:"sign_in_as_session_end_details,omitempty"`
	// SignInAsSessionStartDetails : Started admin sign-in-as session.
	SignInAsSessionStartDetails *SignInAsSessionStartDetails `json:"sign_in_as_session_start_details,omitempty"`
	// SsoLoginFailDetails : Failed to sign in using SSO.
	SsoLoginFailDetails *SsoLoginFailDetails `json:"sso_login_fail_details,omitempty"`
	// MemberAddNameDetails : Set team member name when joining team.
	MemberAddNameDetails *MemberAddNameDetails `json:"member_add_name_details,omitempty"`
	// MemberChangeAdminRoleDetails : Change the admin role belonging to team
	// member.
	MemberChangeAdminRoleDetails *MemberChangeAdminRoleDetails `json:"member_change_admin_role_details,omitempty"`
	// MemberChangeEmailDetails : Changed team member email address.
	MemberChangeEmailDetails *MemberChangeEmailDetails `json:"member_change_email_details,omitempty"`
	// MemberChangeNameDetails : Changed team member name.
	MemberChangeNameDetails *MemberChangeNameDetails `json:"member_change_name_details,omitempty"`
	// MemberChangeStatusDetails : Changed the membership status of a team
	// member.
	MemberChangeStatusDetails *MemberChangeStatusDetails `json:"member_change_status_details,omitempty"`
	// MemberSuggestDetails : Suggested a new team member to be added to the
	// team.
	MemberSuggestDetails *MemberSuggestDetails `json:"member_suggest_details,omitempty"`
	// PaperContentAddMemberDetails : Added users to the membership of a Paper
	// doc or folder.
	PaperContentAddMemberDetails *PaperContentAddMemberDetails `json:"paper_content_add_member_details,omitempty"`
	// PaperContentAddToFolderDetails : Added Paper doc or folder to a folder.
	PaperContentAddToFolderDetails *PaperContentAddToFolderDetails `json:"paper_content_add_to_folder_details,omitempty"`
	// PaperContentArchiveDetails : Archived Paper doc or folder.
	PaperContentArchiveDetails *PaperContentArchiveDetails `json:"paper_content_archive_details,omitempty"`
	// PaperContentChangeSubscriptionDetails : Followed or unfollowed a Paper
	// doc or folder.
	PaperContentChangeSubscriptionDetails *PaperContentChangeSubscriptionDetails `json:"paper_content_change_subscription_details,omitempty"`
	// PaperContentCreateDetails : Created a Paper doc or folder.
	PaperContentCreateDetails *PaperContentCreateDetails `json:"paper_content_create_details,omitempty"`
	// PaperContentPermanentlyDeleteDetails : Permanently deleted a Paper doc or
	// folder.
	PaperContentPermanentlyDeleteDetails *PaperContentPermanentlyDeleteDetails `json:"paper_content_permanently_delete_details,omitempty"`
	// PaperContentRemoveFromFolderDetails : Removed Paper doc or folder from a
	// folder.
	PaperContentRemoveFromFolderDetails *PaperContentRemoveFromFolderDetails `json:"paper_content_remove_from_folder_details,omitempty"`
	// PaperContentRemoveMemberDetails : Removed a user from the membership of a
	// Paper doc or folder.
	PaperContentRemoveMemberDetails *PaperContentRemoveMemberDetails `json:"paper_content_remove_member_details,omitempty"`
	// PaperContentRenameDetails : Renamed Paper doc or folder.
	PaperContentRenameDetails *PaperContentRenameDetails `json:"paper_content_rename_details,omitempty"`
	// PaperContentRestoreDetails : Restored an archived Paper doc or folder.
	PaperContentRestoreDetails *PaperContentRestoreDetails `json:"paper_content_restore_details,omitempty"`
	// PaperDocAddCommentDetails : Added a Paper doc comment.
	PaperDocAddCommentDetails *PaperDocAddCommentDetails `json:"paper_doc_add_comment_details,omitempty"`
	// PaperDocChangeMemberRoleDetails : Changed the access type of a Paper doc
	// member.
	PaperDocChangeMemberRoleDetails *PaperDocChangeMemberRoleDetails `json:"paper_doc_change_member_role_details,omitempty"`
	// PaperDocChangeSharingPolicyDetails : Changed the sharing policy for Paper
	// doc.
	PaperDocChangeSharingPolicyDetails *PaperDocChangeSharingPolicyDetails `json:"paper_doc_change_sharing_policy_details,omitempty"`
	// PaperDocDeletedDetails : Paper doc archived.
	PaperDocDeletedDetails *PaperDocDeletedDetails `json:"paper_doc_deleted_details,omitempty"`
	// PaperDocDeleteCommentDetails : Deleted a Paper doc comment.
	PaperDocDeleteCommentDetails *PaperDocDeleteCommentDetails `json:"paper_doc_delete_comment_details,omitempty"`
	// PaperDocDownloadDetails : Downloaded a Paper doc in a particular output
	// format.
	PaperDocDownloadDetails *PaperDocDownloadDetails `json:"paper_doc_download_details,omitempty"`
	// PaperDocEditDetails : Edited a Paper doc.
	PaperDocEditDetails *PaperDocEditDetails `json:"paper_doc_edit_details,omitempty"`
	// PaperDocEditCommentDetails : Edited a Paper doc comment.
	PaperDocEditCommentDetails *PaperDocEditCommentDetails `json:"paper_doc_edit_comment_details,omitempty"`
	// PaperDocFollowedDetails : Followed a Paper doc.
	PaperDocFollowedDetails *PaperDocFollowedDetails `json:"paper_doc_followed_details,omitempty"`
	// PaperDocMentionDetails : Mentioned a member in a Paper doc.
	PaperDocMentionDetails *PaperDocMentionDetails `json:"paper_doc_mention_details,omitempty"`
	// PaperDocRequestAccessDetails : Requested to be a member on a Paper doc.
	PaperDocRequestAccessDetails *PaperDocRequestAccessDetails `json:"paper_doc_request_access_details,omitempty"`
	// PaperDocResolveCommentDetails : Paper doc comment resolved.
	PaperDocResolveCommentDetails *PaperDocResolveCommentDetails `json:"paper_doc_resolve_comment_details,omitempty"`
	// PaperDocRevertDetails : Restored a Paper doc to previous revision.
	PaperDocRevertDetails *PaperDocRevertDetails `json:"paper_doc_revert_details,omitempty"`
	// PaperDocSlackShareDetails : Paper doc link shared via slack.
	PaperDocSlackShareDetails *PaperDocSlackShareDetails `json:"paper_doc_slack_share_details,omitempty"`
	// PaperDocTeamInviteDetails : Paper doc shared with team member.
	PaperDocTeamInviteDetails *PaperDocTeamInviteDetails `json:"paper_doc_team_invite_details,omitempty"`
	// PaperDocUnresolveCommentDetails : Unresolved a Paper doc comment.
	PaperDocUnresolveCommentDetails *PaperDocUnresolveCommentDetails `json:"paper_doc_unresolve_comment_details,omitempty"`
	// PaperDocViewDetails : Viewed Paper doc.
	PaperDocViewDetails *PaperDocViewDetails `json:"paper_doc_view_details,omitempty"`
	// PaperFolderDeletedDetails : Paper folder archived.
	PaperFolderDeletedDetails *PaperFolderDeletedDetails `json:"paper_folder_deleted_details,omitempty"`
	// PaperFolderFollowedDetails : Followed a Paper folder.
	PaperFolderFollowedDetails *PaperFolderFollowedDetails `json:"paper_folder_followed_details,omitempty"`
	// PaperFolderTeamInviteDetails : Paper folder shared with team member.
	PaperFolderTeamInviteDetails *PaperFolderTeamInviteDetails `json:"paper_folder_team_invite_details,omitempty"`
	// PasswordChangeDetails : Changed password.
	PasswordChangeDetails *PasswordChangeDetails `json:"password_change_details,omitempty"`
	// PasswordResetDetails : Reset password.
	PasswordResetDetails *PasswordResetDetails `json:"password_reset_details,omitempty"`
	// PasswordResetAllDetails : Reset all team member passwords.
	PasswordResetAllDetails *PasswordResetAllDetails `json:"password_reset_all_details,omitempty"`
	// EmmCreateExceptionsReportDetails : EMM excluded users report created.
	EmmCreateExceptionsReportDetails *EmmCreateExceptionsReportDetails `json:"emm_create_exceptions_report_details,omitempty"`
	// EmmCreateUsageReportDetails : EMM mobile app usage report created.
	EmmCreateUsageReportDetails *EmmCreateUsageReportDetails `json:"emm_create_usage_report_details,omitempty"`
	// SmartSyncCreateAdminPrivilegeReportDetails : Smart Sync non-admin devices
	// report created.
	SmartSyncCreateAdminPrivilegeReportDetails *SmartSyncCreateAdminPrivilegeReportDetails `json:"smart_sync_create_admin_privilege_report_details,omitempty"`
	// TeamActivityCreateReportDetails : Created a team activity report.
	TeamActivityCreateReportDetails *TeamActivityCreateReportDetails `json:"team_activity_create_report_details,omitempty"`
	// CollectionShareDetails : Shared an album.
	CollectionShareDetails *CollectionShareDetails `json:"collection_share_details,omitempty"`
	// FileAddCommentDetails : Added a file comment.
	FileAddCommentDetails *FileAddCommentDetails `json:"file_add_comment_details,omitempty"`
	// FileLikeCommentDetails : Liked a file comment.
	FileLikeCommentDetails *FileLikeCommentDetails `json:"file_like_comment_details,omitempty"`
	// FileUnlikeCommentDetails : Unliked a file comment.
	FileUnlikeCommentDetails *FileUnlikeCommentDetails `json:"file_unlike_comment_details,omitempty"`
	// NoteAclInviteOnlyDetails : Changed a Paper document to be invite-only.
	NoteAclInviteOnlyDetails *NoteAclInviteOnlyDetails `json:"note_acl_invite_only_details,omitempty"`
	// NoteAclLinkDetails : Changed a Paper document to be link accessible.
	NoteAclLinkDetails *NoteAclLinkDetails `json:"note_acl_link_details,omitempty"`
	// NoteAclTeamLinkDetails : Changed a Paper document to be link accessible
	// for the team.
	NoteAclTeamLinkDetails *NoteAclTeamLinkDetails `json:"note_acl_team_link_details,omitempty"`
	// NoteSharedDetails : Shared a Paper doc.
	NoteSharedDetails *NoteSharedDetails `json:"note_shared_details,omitempty"`
	// NoteShareReceiveDetails : Shared Paper document received.
	NoteShareReceiveDetails *NoteShareReceiveDetails `json:"note_share_receive_details,omitempty"`
	// OpenNoteSharedDetails : Opened a shared Paper doc.
	OpenNoteSharedDetails *OpenNoteSharedDetails `json:"open_note_shared_details,omitempty"`
	// SfAddGroupDetails : Added the team to a shared folder.
	SfAddGroupDetails *SfAddGroupDetails `json:"sf_add_group_details,omitempty"`
	// SfAllowNonMembersToViewSharedLinksDetails : Allowed non collaborators to
	// view links to files in a shared folder.
	SfAllowNonMembersToViewSharedLinksDetails *SfAllowNonMembersToViewSharedLinksDetails `json:"sf_allow_non_members_to_view_shared_links_details,omitempty"`
	// SfInviteGroupDetails : Invited a group to a shared folder.
	SfInviteGroupDetails *SfInviteGroupDetails `json:"sf_invite_group_details,omitempty"`
	// SfNestDetails : Changed parent of shared folder.
	SfNestDetails *SfNestDetails `json:"sf_nest_details,omitempty"`
	// SfTeamDeclineDetails : Declined a team member's invitation to a shared
	// folder.
	SfTeamDeclineDetails *SfTeamDeclineDetails `json:"sf_team_decline_details,omitempty"`
	// SfTeamGrantAccessDetails : Granted access to a shared folder.
	SfTeamGrantAccessDetails *SfTeamGrantAccessDetails `json:"sf_team_grant_access_details,omitempty"`
	// SfTeamInviteDetails : Invited team members to a shared folder.
	SfTeamInviteDetails *SfTeamInviteDetails `json:"sf_team_invite_details,omitempty"`
	// SfTeamInviteChangeRoleDetails : Changed a team member's role in a shared
	// folder.
	SfTeamInviteChangeRoleDetails *SfTeamInviteChangeRoleDetails `json:"sf_team_invite_change_role_details,omitempty"`
	// SfTeamJoinDetails : Joined a team member's shared folder.
	SfTeamJoinDetails *SfTeamJoinDetails `json:"sf_team_join_details,omitempty"`
	// SfTeamJoinFromOobLinkDetails : Joined a team member's shared folder from
	// a link.
	SfTeamJoinFromOobLinkDetails *SfTeamJoinFromOobLinkDetails `json:"sf_team_join_from_oob_link_details,omitempty"`
	// SfTeamUninviteDetails : Unshared a folder with a team member.
	SfTeamUninviteDetails *SfTeamUninviteDetails `json:"sf_team_uninvite_details,omitempty"`
	// SharedContentAddInviteesDetails : Sent an email invitation to the
	// membership of a shared file or folder.
	SharedContentAddInviteesDetails *SharedContentAddInviteesDetails `json:"shared_content_add_invitees_details,omitempty"`
	// SharedContentAddLinkExpiryDetails : Added an expiry to the link for the
	// shared file or folder.
	SharedContentAddLinkExpiryDetails *SharedContentAddLinkExpiryDetails `json:"shared_content_add_link_expiry_details,omitempty"`
	// SharedContentAddLinkPasswordDetails : Added a password to the link for
	// the shared file or folder.
	SharedContentAddLinkPasswordDetails *SharedContentAddLinkPasswordDetails `json:"shared_content_add_link_password_details,omitempty"`
	// SharedContentAddMemberDetails : Added users and/or groups to the
	// membership of a shared file or folder.
	SharedContentAddMemberDetails *SharedContentAddMemberDetails `json:"shared_content_add_member_details,omitempty"`
	// SharedContentChangeDownloadsPolicyDetails : Changed whether members can
	// download the shared file or folder.
	SharedContentChangeDownloadsPolicyDetails *SharedContentChangeDownloadsPolicyDetails `json:"shared_content_change_downloads_policy_details,omitempty"`
	// SharedContentChangeInviteeRoleDetails : Changed the access type of an
	// invitee to a shared file or folder before the invitation was claimed.
	SharedContentChangeInviteeRoleDetails *SharedContentChangeInviteeRoleDetails `json:"shared_content_change_invitee_role_details,omitempty"`
	// SharedContentChangeLinkAudienceDetails : Changed the audience of the link
	// for a shared file or folder.
	SharedContentChangeLinkAudienceDetails *SharedContentChangeLinkAudienceDetails `json:"shared_content_change_link_audience_details,omitempty"`
	// SharedContentChangeLinkExpiryDetails : Changed the expiry of the link for
	// the shared file or folder.
	SharedContentChangeLinkExpiryDetails *SharedContentChangeLinkExpiryDetails `json:"shared_content_change_link_expiry_details,omitempty"`
	// SharedContentChangeLinkPasswordDetails : Changed the password on the link
	// for the shared file or folder.
	SharedContentChangeLinkPasswordDetails *SharedContentChangeLinkPasswordDetails `json:"shared_content_change_link_password_details,omitempty"`
	// SharedContentChangeMemberRoleDetails : Changed the access type of a
	// shared file or folder member.
	SharedContentChangeMemberRoleDetails *SharedContentChangeMemberRoleDetails `json:"shared_content_change_member_role_details,omitempty"`
	// SharedContentChangeViewerInfoPolicyDetails : Changed whether members can
	// see who viewed the shared file or folder.
	SharedContentChangeViewerInfoPolicyDetails *SharedContentChangeViewerInfoPolicyDetails `json:"shared_content_change_viewer_info_policy_details,omitempty"`
	// SharedContentClaimInvitationDetails : Claimed membership to a team
	// member's shared folder.
	SharedContentClaimInvitationDetails *SharedContentClaimInvitationDetails `json:"shared_content_claim_invitation_details,omitempty"`
	// SharedContentCopyDetails : Copied the shared file or folder to own
	// Dropbox.
	SharedContentCopyDetails *SharedContentCopyDetails `json:"shared_content_copy_details,omitempty"`
	// SharedContentDownloadDetails : Downloaded the shared file or folder.
	SharedContentDownloadDetails *SharedContentDownloadDetails `json:"shared_content_download_details,omitempty"`
	// SharedContentRelinquishMembershipDetails : Left the membership of a
	// shared file or folder.
	SharedContentRelinquishMembershipDetails *SharedContentRelinquishMembershipDetails `json:"shared_content_relinquish_membership_details,omitempty"`
	// SharedContentRemoveInviteeDetails : Removed an invitee from the
	// membership of a shared file or folder before it was claimed.
	SharedContentRemoveInviteeDetails *SharedContentRemoveInviteeDetails `json:"shared_content_remove_invitee_details,omitempty"`
	// SharedContentRemoveLinkExpiryDetails : Removed the expiry of the link for
	// the shared file or folder.
	SharedContentRemoveLinkExpiryDetails *SharedContentRemoveLinkExpiryDetails `json:"shared_content_remove_link_expiry_details,omitempty"`
	// SharedContentRemoveLinkPasswordDetails : Removed the password on the link
	// for the shared file or folder.
	SharedContentRemoveLinkPasswordDetails *SharedContentRemoveLinkPasswordDetails `json:"shared_content_remove_link_password_details,omitempty"`
	// SharedContentRemoveMemberDetails : Removed a user or a group from the
	// membership of a shared file or folder.
	SharedContentRemoveMemberDetails *SharedContentRemoveMemberDetails `json:"shared_content_remove_member_details,omitempty"`
	// SharedContentRequestAccessDetails : Requested to be on the membership of
	// a shared file or folder.
	SharedContentRequestAccessDetails *SharedContentRequestAccessDetails `json:"shared_content_request_access_details,omitempty"`
	// SharedContentUnshareDetails : Unshared a shared file or folder by
	// clearing its membership and turning off its link.
	SharedContentUnshareDetails *SharedContentUnshareDetails `json:"shared_content_unshare_details,omitempty"`
	// SharedContentViewDetails : Previewed the shared file or folder.
	SharedContentViewDetails *SharedContentViewDetails `json:"shared_content_view_details,omitempty"`
	// SharedFolderChangeConfidentialityDetails : Set or unset the confidential
	// flag on a shared folder.
	SharedFolderChangeConfidentialityDetails *SharedFolderChangeConfidentialityDetails `json:"shared_folder_change_confidentiality_details,omitempty"`
	// SharedFolderChangeLinkPolicyDetails : Changed who can access the shared
	// folder via a link.
	SharedFolderChangeLinkPolicyDetails *SharedFolderChangeLinkPolicyDetails `json:"shared_folder_change_link_policy_details,omitempty"`
	// SharedFolderChangeMemberManagementPolicyDetails : Changed who can manage
	// the membership of a shared folder.
	SharedFolderChangeMemberManagementPolicyDetails *SharedFolderChangeMemberManagementPolicyDetails `json:"shared_folder_change_member_management_policy_details,omitempty"`
	// SharedFolderChangeMemberPolicyDetails : Changed who can become a member
	// of the shared folder.
	SharedFolderChangeMemberPolicyDetails *SharedFolderChangeMemberPolicyDetails `json:"shared_folder_change_member_policy_details,omitempty"`
	// SharedFolderCreateDetails : Created a shared folder.
	SharedFolderCreateDetails *SharedFolderCreateDetails `json:"shared_folder_create_details,omitempty"`
	// SharedFolderMountDetails : Added a shared folder to own Dropbox.
	SharedFolderMountDetails *SharedFolderMountDetails `json:"shared_folder_mount_details,omitempty"`
	// SharedFolderTransferOwnershipDetails : Transferred the ownership of a
	// shared folder to another member.
	SharedFolderTransferOwnershipDetails *SharedFolderTransferOwnershipDetails `json:"shared_folder_transfer_ownership_details,omitempty"`
	// SharedFolderUnmountDetails : Deleted a shared folder from Dropbox.
	SharedFolderUnmountDetails *SharedFolderUnmountDetails `json:"shared_folder_unmount_details,omitempty"`
	// SharedNoteOpenedDetails : Shared Paper document was opened.
	SharedNoteOpenedDetails *SharedNoteOpenedDetails `json:"shared_note_opened_details,omitempty"`
	// ShmodelAppCreateDetails : Created a link to a file using an app.
	ShmodelAppCreateDetails *ShmodelAppCreateDetails `json:"shmodel_app_create_details,omitempty"`
	// ShmodelCreateDetails : Created a new link.
	ShmodelCreateDetails *ShmodelCreateDetails `json:"shmodel_create_details,omitempty"`
	// ShmodelDisableDetails : Removed a link.
	ShmodelDisableDetails *ShmodelDisableDetails `json:"shmodel_disable_details,omitempty"`
	// ShmodelFbShareDetails : Shared a link with Facebook users.
	ShmodelFbShareDetails *ShmodelFbShareDetails `json:"shmodel_fb_share_details,omitempty"`
	// ShmodelGroupShareDetails : Shared a link with a group.
	ShmodelGroupShareDetails *ShmodelGroupShareDetails `json:"shmodel_group_share_details,omitempty"`
	// ShmodelRemoveExpirationDetails : Removed the expiration date from a link.
	ShmodelRemoveExpirationDetails *ShmodelRemoveExpirationDetails `json:"shmodel_remove_expiration_details,omitempty"`
	// ShmodelSetExpirationDetails : Added an expiration date to a link.
	ShmodelSetExpirationDetails *ShmodelSetExpirationDetails `json:"shmodel_set_expiration_details,omitempty"`
	// ShmodelTeamCopyDetails : Added a team member's file/folder to their
	// Dropbox from a link.
	ShmodelTeamCopyDetails *ShmodelTeamCopyDetails `json:"shmodel_team_copy_details,omitempty"`
	// ShmodelTeamDownloadDetails : Downloaded a team member's file/folder from
	// a link.
	ShmodelTeamDownloadDetails *ShmodelTeamDownloadDetails `json:"shmodel_team_download_details,omitempty"`
	// ShmodelTeamShareDetails : Shared a link with team members.
	ShmodelTeamShareDetails *ShmodelTeamShareDetails `json:"shmodel_team_share_details,omitempty"`
	// ShmodelTeamViewDetails : Opened a team member's link.
	ShmodelTeamViewDetails *ShmodelTeamViewDetails `json:"shmodel_team_view_details,omitempty"`
	// ShmodelVisibilityPasswordDetails : Password-protected a link.
	ShmodelVisibilityPasswordDetails *ShmodelVisibilityPasswordDetails `json:"shmodel_visibility_password_details,omitempty"`
	// ShmodelVisibilityPublicDetails : Made a file/folder visible to anyone
	// with the link.
	ShmodelVisibilityPublicDetails *ShmodelVisibilityPublicDetails `json:"shmodel_visibility_public_details,omitempty"`
	// ShmodelVisibilityTeamOnlyDetails : Made a file/folder visible only to
	// team members with the link.
	ShmodelVisibilityTeamOnlyDetails *ShmodelVisibilityTeamOnlyDetails `json:"shmodel_visibility_team_only_details,omitempty"`
	// RemoveLogoutUrlDetails : Removed single sign-on logout URL.
	RemoveLogoutUrlDetails *RemoveLogoutUrlDetails `json:"remove_logout_url_details,omitempty"`
	// RemoveSsoUrlDetails : Changed the sign-out URL for SSO.
	RemoveSsoUrlDetails *RemoveSsoUrlDetails `json:"remove_sso_url_details,omitempty"`
	// SsoChangeCertDetails : Changed the X.509 certificate for SSO.
	SsoChangeCertDetails *SsoChangeCertDetails `json:"sso_change_cert_details,omitempty"`
	// SsoChangeLoginUrlDetails : Changed the sign-in URL for SSO.
	SsoChangeLoginUrlDetails *SsoChangeLoginUrlDetails `json:"sso_change_login_url_details,omitempty"`
	// SsoChangeLogoutUrlDetails : Changed the sign-out URL for SSO.
	SsoChangeLogoutUrlDetails *SsoChangeLogoutUrlDetails `json:"sso_change_logout_url_details,omitempty"`
	// SsoChangeSamlIdentityModeDetails : Changed the SAML identity mode for
	// SSO.
	SsoChangeSamlIdentityModeDetails *SsoChangeSamlIdentityModeDetails `json:"sso_change_saml_identity_mode_details,omitempty"`
	// TeamFolderChangeStatusDetails : Changed the archival status of a team
	// folder.
	TeamFolderChangeStatusDetails *TeamFolderChangeStatusDetails `json:"team_folder_change_status_details,omitempty"`
	// TeamFolderCreateDetails : Created a new team folder in active status.
	TeamFolderCreateDetails *TeamFolderCreateDetails `json:"team_folder_create_details,omitempty"`
	// TeamFolderDowngradeDetails : Downgraded a team folder to a regular shared
	// folder.
	TeamFolderDowngradeDetails *TeamFolderDowngradeDetails `json:"team_folder_downgrade_details,omitempty"`
	// TeamFolderPermanentlyDeleteDetails : Permanently deleted an archived team
	// folder.
	TeamFolderPermanentlyDeleteDetails *TeamFolderPermanentlyDeleteDetails `json:"team_folder_permanently_delete_details,omitempty"`
	// TeamFolderRenameDetails : Renamed an active or archived team folder.
	TeamFolderRenameDetails *TeamFolderRenameDetails `json:"team_folder_rename_details,omitempty"`
	// AccountCaptureChangePolicyDetails : Changed the account capture policy on
	// a domain belonging to the team.
	AccountCaptureChangePolicyDetails *AccountCaptureChangePolicyDetails `json:"account_capture_change_policy_details,omitempty"`
	// AllowDownloadDisabledDetails : Disabled allow downloads.
	AllowDownloadDisabledDetails *AllowDownloadDisabledDetails `json:"allow_download_disabled_details,omitempty"`
	// AllowDownloadEnabledDetails : Enabled allow downloads.
	AllowDownloadEnabledDetails *AllowDownloadEnabledDetails `json:"allow_download_enabled_details,omitempty"`
	// DataPlacementRestrictionChangePolicyDetails : Set a restriction policy
	// regarding the location of data centers where team data resides.
	DataPlacementRestrictionChangePolicyDetails *DataPlacementRestrictionChangePolicyDetails `json:"data_placement_restriction_change_policy_details,omitempty"`
	// DataPlacementRestrictionSatisfyPolicyDetails : Satisfied a previously set
	// restriction policy regarding the location of data centers where team data
	// resides (i.e. all data have been migrated according to the restriction
	// placed).
	DataPlacementRestrictionSatisfyPolicyDetails *DataPlacementRestrictionSatisfyPolicyDetails `json:"data_placement_restriction_satisfy_policy_details,omitempty"`
	// DeviceApprovalsChangeDesktopPolicyDetails : Set or removed a limit on the
	// number of computers each team member can link to their work Dropbox
	// account.
	DeviceApprovalsChangeDesktopPolicyDetails *DeviceApprovalsChangeDesktopPolicyDetails `json:"device_approvals_change_desktop_policy_details,omitempty"`
	// DeviceApprovalsChangeMobilePolicyDetails : Set or removed a limit on the
	// number of mobiles devices each team member can link to their work Dropbox
	// account.
	DeviceApprovalsChangeMobilePolicyDetails *DeviceApprovalsChangeMobilePolicyDetails `json:"device_approvals_change_mobile_policy_details,omitempty"`
	// DeviceApprovalsChangeOverageActionDetails : Changed the action taken when
	// a team member is already over the limits (e.g when they join the team, an
	// admin lowers limits, etc.).
	DeviceApprovalsChangeOverageActionDetails *DeviceApprovalsChangeOverageActionDetails `json:"device_approvals_change_overage_action_details,omitempty"`
	// DeviceApprovalsChangeUnlinkActionDetails : Changed the action taken with
	// respect to approval limits when a team member unlinks an approved device.
	DeviceApprovalsChangeUnlinkActionDetails *DeviceApprovalsChangeUnlinkActionDetails `json:"device_approvals_change_unlink_action_details,omitempty"`
	// EmmAddExceptionDetails : Added an exception for one or more team members
	// to optionally use the regular Dropbox app when EMM is enabled.
	EmmAddExceptionDetails *EmmAddExceptionDetails `json:"emm_add_exception_details,omitempty"`
	// EmmChangePolicyDetails : Enabled or disabled enterprise mobility
	// management for team members.
	EmmChangePolicyDetails *EmmChangePolicyDetails `json:"emm_change_policy_details,omitempty"`
	// EmmRemoveExceptionDetails : Removed an exception for one or more team
	// members to optionally use the regular Dropbox app when EMM is enabled.
	EmmRemoveExceptionDetails *EmmRemoveExceptionDetails `json:"emm_remove_exception_details,omitempty"`
	// ExtendedVersionHistoryChangePolicyDetails : Accepted or opted out of
	// extended version history.
	ExtendedVersionHistoryChangePolicyDetails *ExtendedVersionHistoryChangePolicyDetails `json:"extended_version_history_change_policy_details,omitempty"`
	// FileCommentsChangePolicyDetails : Enabled or disabled commenting on team
	// files.
	FileCommentsChangePolicyDetails *FileCommentsChangePolicyDetails `json:"file_comments_change_policy_details,omitempty"`
	// FileRequestsChangePolicyDetails : Enabled or disabled file requests.
	FileRequestsChangePolicyDetails *FileRequestsChangePolicyDetails `json:"file_requests_change_policy_details,omitempty"`
	// FileRequestsEmailsEnabledDetails : Enabled file request emails for
	// everyone.
	FileRequestsEmailsEnabledDetails *FileRequestsEmailsEnabledDetails `json:"file_requests_emails_enabled_details,omitempty"`
	// FileRequestsEmailsRestrictedToTeamOnlyDetails : Allowed file request
	// emails for the team.
	FileRequestsEmailsRestrictedToTeamOnlyDetails *FileRequestsEmailsRestrictedToTeamOnlyDetails `json:"file_requests_emails_restricted_to_team_only_details,omitempty"`
	// GoogleSsoChangePolicyDetails : Enabled or disabled Google single sign-on
	// for the team.
	GoogleSsoChangePolicyDetails *GoogleSsoChangePolicyDetails `json:"google_sso_change_policy_details,omitempty"`
	// GroupUserManagementChangePolicyDetails : Changed who can create groups.
	GroupUserManagementChangePolicyDetails *GroupUserManagementChangePolicyDetails `json:"group_user_management_change_policy_details,omitempty"`
	// MemberRequestsChangePolicyDetails : Changed whether users can find the
	// team when not invited.
	MemberRequestsChangePolicyDetails *MemberRequestsChangePolicyDetails `json:"member_requests_change_policy_details,omitempty"`
	// MemberSpaceLimitsAddExceptionDetails : Added an exception for one or more
	// team members to bypass space limits imposed by policy.
	MemberSpaceLimitsAddExceptionDetails *MemberSpaceLimitsAddExceptionDetails `json:"member_space_limits_add_exception_details,omitempty"`
	// MemberSpaceLimitsChangePolicyDetails : Changed the storage limits applied
	// to team members by policy.
	MemberSpaceLimitsChangePolicyDetails *MemberSpaceLimitsChangePolicyDetails `json:"member_space_limits_change_policy_details,omitempty"`
	// MemberSpaceLimitsRemoveExceptionDetails : Removed an exception for one or
	// more team members to bypass space limits imposed by policy.
	MemberSpaceLimitsRemoveExceptionDetails *MemberSpaceLimitsRemoveExceptionDetails `json:"member_space_limits_remove_exception_details,omitempty"`
	// MemberSuggestionsChangePolicyDetails : Enabled or disabled the option for
	// team members to suggest new members to add to the team.
	MemberSuggestionsChangePolicyDetails *MemberSuggestionsChangePolicyDetails `json:"member_suggestions_change_policy_details,omitempty"`
	// MicrosoftOfficeAddinChangePolicyDetails : Enabled or disabled the
	// Microsoft Office add-in, which lets team members save files to Dropbox
	// directly from Microsoft Office.
	MicrosoftOfficeAddinChangePolicyDetails *MicrosoftOfficeAddinChangePolicyDetails `json:"microsoft_office_addin_change_policy_details,omitempty"`
	// NetworkControlChangePolicyDetails : Enabled or disabled network control.
	NetworkControlChangePolicyDetails *NetworkControlChangePolicyDetails `json:"network_control_change_policy_details,omitempty"`
	// PaperChangeDeploymentPolicyDetails : Changed whether Dropbox Paper, when
	// enabled, is deployed to all teams or to specific members of the team.
	PaperChangeDeploymentPolicyDetails *PaperChangeDeploymentPolicyDetails `json:"paper_change_deployment_policy_details,omitempty"`
	// PaperChangeMemberPolicyDetails : Changed whether team members can share
	// Paper documents externally (i.e. outside the team), and if so, whether
	// they should be accessible only by team members or anyone by default.
	PaperChangeMemberPolicyDetails *PaperChangeMemberPolicyDetails `json:"paper_change_member_policy_details,omitempty"`
	// PaperChangePolicyDetails : Enabled or disabled Dropbox Paper for the
	// team.
	PaperChangePolicyDetails *PaperChangePolicyDetails `json:"paper_change_policy_details,omitempty"`
	// PermanentDeleteChangePolicyDetails : Enabled or disabled the ability of
	// team members to permanently delete content.
	PermanentDeleteChangePolicyDetails *PermanentDeleteChangePolicyDetails `json:"permanent_delete_change_policy_details,omitempty"`
	// SharingChangeFolderJoinPolicyDetails : Changed whether team members can
	// join shared folders owned externally (i.e. outside the team).
	SharingChangeFolderJoinPolicyDetails *SharingChangeFolderJoinPolicyDetails `json:"sharing_change_folder_join_policy_details,omitempty"`
	// SharingChangeLinkPolicyDetails : Changed whether team members can share
	// links externally (i.e. outside the team), and if so, whether links should
	// be accessible only by team members or anyone by default.
	SharingChangeLinkPolicyDetails *SharingChangeLinkPolicyDetails `json:"sharing_change_link_policy_details,omitempty"`
	// SharingChangeMemberPolicyDetails : Changed whether team members can share
	// files and folders externally (i.e. outside the team).
	SharingChangeMemberPolicyDetails *SharingChangeMemberPolicyDetails `json:"sharing_change_member_policy_details,omitempty"`
	// SmartSyncChangePolicyDetails : Changed the default Smart Sync policy for
	// team members.
	SmartSyncChangePolicyDetails *SmartSyncChangePolicyDetails `json:"smart_sync_change_policy_details,omitempty"`
	// SmartSyncNotOptOutDetails : Opted team into Smart Sync.
	SmartSyncNotOptOutDetails *SmartSyncNotOptOutDetails `json:"smart_sync_not_opt_out_details,omitempty"`
	// SmartSyncOptOutDetails : Opted team out of Smart Sync.
	SmartSyncOptOutDetails *SmartSyncOptOutDetails `json:"smart_sync_opt_out_details,omitempty"`
	// SsoChangePolicyDetails : Change the single sign-on policy for the team.
	SsoChangePolicyDetails *SsoChangePolicyDetails `json:"sso_change_policy_details,omitempty"`
	// TfaChangePolicyDetails : Change two-step verification policy for the
	// team.
	TfaChangePolicyDetails *TfaChangePolicyDetails `json:"tfa_change_policy_details,omitempty"`
	// TwoAccountChangePolicyDetails : Enabled or disabled the option for team
	// members to link a personal Dropbox account in addition to their work
	// account to the same computer.
	TwoAccountChangePolicyDetails *TwoAccountChangePolicyDetails `json:"two_account_change_policy_details,omitempty"`
	// WebSessionsChangeFixedLengthPolicyDetails : Changed how long team members
	// can stay signed in to Dropbox on the web.
	WebSessionsChangeFixedLengthPolicyDetails *WebSessionsChangeFixedLengthPolicyDetails `json:"web_sessions_change_fixed_length_policy_details,omitempty"`
	// WebSessionsChangeIdleLengthPolicyDetails : Changed how long team members
	// can be idle while signed in to Dropbox on the web.
	WebSessionsChangeIdleLengthPolicyDetails *WebSessionsChangeIdleLengthPolicyDetails `json:"web_sessions_change_idle_length_policy_details,omitempty"`
	// TeamProfileAddLogoDetails : Added a team logo to be displayed on shared
	// link headers.
	TeamProfileAddLogoDetails *TeamProfileAddLogoDetails `json:"team_profile_add_logo_details,omitempty"`
	// TeamProfileChangeLogoDetails : Changed the team logo to be displayed on
	// shared link headers.
	TeamProfileChangeLogoDetails *TeamProfileChangeLogoDetails `json:"team_profile_change_logo_details,omitempty"`
	// TeamProfileChangeNameDetails : Changed the team name.
	TeamProfileChangeNameDetails *TeamProfileChangeNameDetails `json:"team_profile_change_name_details,omitempty"`
	// TeamProfileRemoveLogoDetails : Removed the team logo to be displayed on
	// shared link headers.
	TeamProfileRemoveLogoDetails *TeamProfileRemoveLogoDetails `json:"team_profile_remove_logo_details,omitempty"`
	// TfaAddBackupPhoneDetails : Added a backup phone for two-step
	// verification.
	TfaAddBackupPhoneDetails *TfaAddBackupPhoneDetails `json:"tfa_add_backup_phone_details,omitempty"`
	// TfaAddSecurityKeyDetails : Added a security key for two-step
	// verification.
	TfaAddSecurityKeyDetails *TfaAddSecurityKeyDetails `json:"tfa_add_security_key_details,omitempty"`
	// TfaChangeBackupPhoneDetails : Changed the backup phone for two-step
	// verification.
	TfaChangeBackupPhoneDetails *TfaChangeBackupPhoneDetails `json:"tfa_change_backup_phone_details,omitempty"`
	// TfaChangeStatusDetails : Enabled, disabled or changed the configuration
	// for two-step verification.
	TfaChangeStatusDetails *TfaChangeStatusDetails `json:"tfa_change_status_details,omitempty"`
	// TfaRemoveBackupPhoneDetails : Removed the backup phone for two-step
	// verification.
	TfaRemoveBackupPhoneDetails *TfaRemoveBackupPhoneDetails `json:"tfa_remove_backup_phone_details,omitempty"`
	// TfaRemoveSecurityKeyDetails : Removed a security key for two-step
	// verification.
	TfaRemoveSecurityKeyDetails *TfaRemoveSecurityKeyDetails `json:"tfa_remove_security_key_details,omitempty"`
	// TfaResetDetails : Reset two-step verification for team member.
	TfaResetDetails *TfaResetDetails `json:"tfa_reset_details,omitempty"`
	// MissingDetails : Hints that this event was returned with missing details
	// due to an internal error.
	MissingDetails *MissingDetails `json:"missing_details,omitempty"`
}

// Valid tag values for EventDetails
const (
	EventDetailsMemberChangeMembershipTypeDetails               = "member_change_membership_type_details"
	EventDetailsMemberPermanentlyDeleteAccountContentsDetails   = "member_permanently_delete_account_contents_details"
	EventDetailsMemberSpaceLimitsChangeStatusDetails            = "member_space_limits_change_status_details"
	EventDetailsMemberTransferAccountContentsDetails            = "member_transfer_account_contents_details"
	EventDetailsPaperEnabledUsersGroupAdditionDetails           = "paper_enabled_users_group_addition_details"
	EventDetailsPaperEnabledUsersGroupRemovalDetails            = "paper_enabled_users_group_removal_details"
	EventDetailsPaperExternalViewAllowDetails                   = "paper_external_view_allow_details"
	EventDetailsPaperExternalViewDefaultTeamDetails             = "paper_external_view_default_team_details"
	EventDetailsPaperExternalViewForbidDetails                  = "paper_external_view_forbid_details"
	EventDetailsSfExternalInviteWarnDetails                     = "sf_external_invite_warn_details"
	EventDetailsTeamMergeFromDetails                            = "team_merge_from_details"
	EventDetailsTeamMergeToDetails                              = "team_merge_to_details"
	EventDetailsAppLinkTeamDetails                              = "app_link_team_details"
	EventDetailsAppLinkUserDetails                              = "app_link_user_details"
	EventDetailsAppUnlinkTeamDetails                            = "app_unlink_team_details"
	EventDetailsAppUnlinkUserDetails                            = "app_unlink_user_details"
	EventDetailsDeviceChangeIpDesktopDetails                    = "device_change_ip_desktop_details"
	EventDetailsDeviceChangeIpMobileDetails                     = "device_change_ip_mobile_details"
	EventDetailsDeviceChangeIpWebDetails                        = "device_change_ip_web_details"
	EventDetailsDeviceDeleteOnUnlinkFailDetails                 = "device_delete_on_unlink_fail_details"
	EventDetailsDeviceDeleteOnUnlinkSuccessDetails              = "device_delete_on_unlink_success_details"
	EventDetailsDeviceLinkFailDetails                           = "device_link_fail_details"
	EventDetailsDeviceLinkSuccessDetails                        = "device_link_success_details"
	EventDetailsDeviceManagementDisabledDetails                 = "device_management_disabled_details"
	EventDetailsDeviceManagementEnabledDetails                  = "device_management_enabled_details"
	EventDetailsDeviceUnlinkDetails                             = "device_unlink_details"
	EventDetailsEmmRefreshAuthTokenDetails                      = "emm_refresh_auth_token_details"
	EventDetailsAccountCaptureChangeAvailabilityDetails         = "account_capture_change_availability_details"
	EventDetailsAccountCaptureMigrateAccountDetails             = "account_capture_migrate_account_details"
	EventDetailsAccountCaptureRelinquishAccountDetails          = "account_capture_relinquish_account_details"
	EventDetailsDisabledDomainInvitesDetails                    = "disabled_domain_invites_details"
	EventDetailsDomainInvitesApproveRequestToJoinTeamDetails    = "domain_invites_approve_request_to_join_team_details"
	EventDetailsDomainInvitesDeclineRequestToJoinTeamDetails    = "domain_invites_decline_request_to_join_team_details"
	EventDetailsDomainInvitesEmailExistingUsersDetails          = "domain_invites_email_existing_users_details"
	EventDetailsDomainInvitesRequestToJoinTeamDetails           = "domain_invites_request_to_join_team_details"
	EventDetailsDomainInvitesSetInviteNewUserPrefToNoDetails    = "domain_invites_set_invite_new_user_pref_to_no_details"
	EventDetailsDomainInvitesSetInviteNewUserPrefToYesDetails   = "domain_invites_set_invite_new_user_pref_to_yes_details"
	EventDetailsDomainVerificationAddDomainFailDetails          = "domain_verification_add_domain_fail_details"
	EventDetailsDomainVerificationAddDomainSuccessDetails       = "domain_verification_add_domain_success_details"
	EventDetailsDomainVerificationRemoveDomainDetails           = "domain_verification_remove_domain_details"
	EventDetailsEnabledDomainInvitesDetails                     = "enabled_domain_invites_details"
	EventDetailsCreateFolderDetails                             = "create_folder_details"
	EventDetailsFileAddDetails                                  = "file_add_details"
	EventDetailsFileCopyDetails                                 = "file_copy_details"
	EventDetailsFileDeleteDetails                               = "file_delete_details"
	EventDetailsFileDownloadDetails                             = "file_download_details"
	EventDetailsFileEditDetails                                 = "file_edit_details"
	EventDetailsFileGetCopyReferenceDetails                     = "file_get_copy_reference_details"
	EventDetailsFileMoveDetails                                 = "file_move_details"
	EventDetailsFilePermanentlyDeleteDetails                    = "file_permanently_delete_details"
	EventDetailsFilePreviewDetails                              = "file_preview_details"
	EventDetailsFileRenameDetails                               = "file_rename_details"
	EventDetailsFileRestoreDetails                              = "file_restore_details"
	EventDetailsFileRevertDetails                               = "file_revert_details"
	EventDetailsFileRollbackChangesDetails                      = "file_rollback_changes_details"
	EventDetailsFileSaveCopyReferenceDetails                    = "file_save_copy_reference_details"
	EventDetailsFileRequestAddDeadlineDetails                   = "file_request_add_deadline_details"
	EventDetailsFileRequestChangeFolderDetails                  = "file_request_change_folder_details"
	EventDetailsFileRequestChangeTitleDetails                   = "file_request_change_title_details"
	EventDetailsFileRequestCloseDetails                         = "file_request_close_details"
	EventDetailsFileRequestCreateDetails                        = "file_request_create_details"
	EventDetailsFileRequestReceiveFileDetails                   = "file_request_receive_file_details"
	EventDetailsFileRequestRemoveDeadlineDetails                = "file_request_remove_deadline_details"
	EventDetailsFileRequestSendDetails                          = "file_request_send_details"
	EventDetailsGroupAddExternalIdDetails                       = "group_add_external_id_details"
	EventDetailsGroupAddMemberDetails                           = "group_add_member_details"
	EventDetailsGroupChangeExternalIdDetails                    = "group_change_external_id_details"
	EventDetailsGroupChangeManagementTypeDetails                = "group_change_management_type_details"
	EventDetailsGroupChangeMemberRoleDetails                    = "group_change_member_role_details"
	EventDetailsGroupCreateDetails                              = "group_create_details"
	EventDetailsGroupDeleteDetails                              = "group_delete_details"
	EventDetailsGroupDescriptionUpdatedDetails                  = "group_description_updated_details"
	EventDetailsGroupJoinPolicyUpdatedDetails                   = "group_join_policy_updated_details"
	EventDetailsGroupMovedDetails                               = "group_moved_details"
	EventDetailsGroupRemoveExternalIdDetails                    = "group_remove_external_id_details"
	EventDetailsGroupRemoveMemberDetails                        = "group_remove_member_details"
	EventDetailsGroupRenameDetails                              = "group_rename_details"
	EventDetailsEmmLoginSuccessDetails                          = "emm_login_success_details"
	EventDetailsLogoutDetails                                   = "logout_details"
	EventDetailsPasswordLoginFailDetails                        = "password_login_fail_details"
	EventDetailsPasswordLoginSuccessDetails                     = "password_login_success_details"
	EventDetailsResellerSupportSessionEndDetails                = "reseller_support_session_end_details"
	EventDetailsResellerSupportSessionStartDetails              = "reseller_support_session_start_details"
	EventDetailsSignInAsSessionEndDetails                       = "sign_in_as_session_end_details"
	EventDetailsSignInAsSessionStartDetails                     = "sign_in_as_session_start_details"
	EventDetailsSsoLoginFailDetails                             = "sso_login_fail_details"
	EventDetailsMemberAddNameDetails                            = "member_add_name_details"
	EventDetailsMemberChangeAdminRoleDetails                    = "member_change_admin_role_details"
	EventDetailsMemberChangeEmailDetails                        = "member_change_email_details"
	EventDetailsMemberChangeNameDetails                         = "member_change_name_details"
	EventDetailsMemberChangeStatusDetails                       = "member_change_status_details"
	EventDetailsMemberSuggestDetails                            = "member_suggest_details"
	EventDetailsPaperContentAddMemberDetails                    = "paper_content_add_member_details"
	EventDetailsPaperContentAddToFolderDetails                  = "paper_content_add_to_folder_details"
	EventDetailsPaperContentArchiveDetails                      = "paper_content_archive_details"
	EventDetailsPaperContentChangeSubscriptionDetails           = "paper_content_change_subscription_details"
	EventDetailsPaperContentCreateDetails                       = "paper_content_create_details"
	EventDetailsPaperContentPermanentlyDeleteDetails            = "paper_content_permanently_delete_details"
	EventDetailsPaperContentRemoveFromFolderDetails             = "paper_content_remove_from_folder_details"
	EventDetailsPaperContentRemoveMemberDetails                 = "paper_content_remove_member_details"
	EventDetailsPaperContentRenameDetails                       = "paper_content_rename_details"
	EventDetailsPaperContentRestoreDetails                      = "paper_content_restore_details"
	EventDetailsPaperDocAddCommentDetails                       = "paper_doc_add_comment_details"
	EventDetailsPaperDocChangeMemberRoleDetails                 = "paper_doc_change_member_role_details"
	EventDetailsPaperDocChangeSharingPolicyDetails              = "paper_doc_change_sharing_policy_details"
	EventDetailsPaperDocDeletedDetails                          = "paper_doc_deleted_details"
	EventDetailsPaperDocDeleteCommentDetails                    = "paper_doc_delete_comment_details"
	EventDetailsPaperDocDownloadDetails                         = "paper_doc_download_details"
	EventDetailsPaperDocEditDetails                             = "paper_doc_edit_details"
	EventDetailsPaperDocEditCommentDetails                      = "paper_doc_edit_comment_details"
	EventDetailsPaperDocFollowedDetails                         = "paper_doc_followed_details"
	EventDetailsPaperDocMentionDetails                          = "paper_doc_mention_details"
	EventDetailsPaperDocRequestAccessDetails                    = "paper_doc_request_access_details"
	EventDetailsPaperDocResolveCommentDetails                   = "paper_doc_resolve_comment_details"
	EventDetailsPaperDocRevertDetails                           = "paper_doc_revert_details"
	EventDetailsPaperDocSlackShareDetails                       = "paper_doc_slack_share_details"
	EventDetailsPaperDocTeamInviteDetails                       = "paper_doc_team_invite_details"
	EventDetailsPaperDocUnresolveCommentDetails                 = "paper_doc_unresolve_comment_details"
	EventDetailsPaperDocViewDetails                             = "paper_doc_view_details"
	EventDetailsPaperFolderDeletedDetails                       = "paper_folder_deleted_details"
	EventDetailsPaperFolderFollowedDetails                      = "paper_folder_followed_details"
	EventDetailsPaperFolderTeamInviteDetails                    = "paper_folder_team_invite_details"
	EventDetailsPasswordChangeDetails                           = "password_change_details"
	EventDetailsPasswordResetDetails                            = "password_reset_details"
	EventDetailsPasswordResetAllDetails                         = "password_reset_all_details"
	EventDetailsEmmCreateExceptionsReportDetails                = "emm_create_exceptions_report_details"
	EventDetailsEmmCreateUsageReportDetails                     = "emm_create_usage_report_details"
	EventDetailsSmartSyncCreateAdminPrivilegeReportDetails      = "smart_sync_create_admin_privilege_report_details"
	EventDetailsTeamActivityCreateReportDetails                 = "team_activity_create_report_details"
	EventDetailsCollectionShareDetails                          = "collection_share_details"
	EventDetailsFileAddCommentDetails                           = "file_add_comment_details"
	EventDetailsFileLikeCommentDetails                          = "file_like_comment_details"
	EventDetailsFileUnlikeCommentDetails                        = "file_unlike_comment_details"
	EventDetailsNoteAclInviteOnlyDetails                        = "note_acl_invite_only_details"
	EventDetailsNoteAclLinkDetails                              = "note_acl_link_details"
	EventDetailsNoteAclTeamLinkDetails                          = "note_acl_team_link_details"
	EventDetailsNoteSharedDetails                               = "note_shared_details"
	EventDetailsNoteShareReceiveDetails                         = "note_share_receive_details"
	EventDetailsOpenNoteSharedDetails                           = "open_note_shared_details"
	EventDetailsSfAddGroupDetails                               = "sf_add_group_details"
	EventDetailsSfAllowNonMembersToViewSharedLinksDetails       = "sf_allow_non_members_to_view_shared_links_details"
	EventDetailsSfInviteGroupDetails                            = "sf_invite_group_details"
	EventDetailsSfNestDetails                                   = "sf_nest_details"
	EventDetailsSfTeamDeclineDetails                            = "sf_team_decline_details"
	EventDetailsSfTeamGrantAccessDetails                        = "sf_team_grant_access_details"
	EventDetailsSfTeamInviteDetails                             = "sf_team_invite_details"
	EventDetailsSfTeamInviteChangeRoleDetails                   = "sf_team_invite_change_role_details"
	EventDetailsSfTeamJoinDetails                               = "sf_team_join_details"
	EventDetailsSfTeamJoinFromOobLinkDetails                    = "sf_team_join_from_oob_link_details"
	EventDetailsSfTeamUninviteDetails                           = "sf_team_uninvite_details"
	EventDetailsSharedContentAddInviteesDetails                 = "shared_content_add_invitees_details"
	EventDetailsSharedContentAddLinkExpiryDetails               = "shared_content_add_link_expiry_details"
	EventDetailsSharedContentAddLinkPasswordDetails             = "shared_content_add_link_password_details"
	EventDetailsSharedContentAddMemberDetails                   = "shared_content_add_member_details"
	EventDetailsSharedContentChangeDownloadsPolicyDetails       = "shared_content_change_downloads_policy_details"
	EventDetailsSharedContentChangeInviteeRoleDetails           = "shared_content_change_invitee_role_details"
	EventDetailsSharedContentChangeLinkAudienceDetails          = "shared_content_change_link_audience_details"
	EventDetailsSharedContentChangeLinkExpiryDetails            = "shared_content_change_link_expiry_details"
	EventDetailsSharedContentChangeLinkPasswordDetails          = "shared_content_change_link_password_details"
	EventDetailsSharedContentChangeMemberRoleDetails            = "shared_content_change_member_role_details"
	EventDetailsSharedContentChangeViewerInfoPolicyDetails      = "shared_content_change_viewer_info_policy_details"
	EventDetailsSharedContentClaimInvitationDetails             = "shared_content_claim_invitation_details"
	EventDetailsSharedContentCopyDetails                        = "shared_content_copy_details"
	EventDetailsSharedContentDownloadDetails                    = "shared_content_download_details"
	EventDetailsSharedContentRelinquishMembershipDetails        = "shared_content_relinquish_membership_details"
	EventDetailsSharedContentRemoveInviteeDetails               = "shared_content_remove_invitee_details"
	EventDetailsSharedContentRemoveLinkExpiryDetails            = "shared_content_remove_link_expiry_details"
	EventDetailsSharedContentRemoveLinkPasswordDetails          = "shared_content_remove_link_password_details"
	EventDetailsSharedContentRemoveMemberDetails                = "shared_content_remove_member_details"
	EventDetailsSharedContentRequestAccessDetails               = "shared_content_request_access_details"
	EventDetailsSharedContentUnshareDetails                     = "shared_content_unshare_details"
	EventDetailsSharedContentViewDetails                        = "shared_content_view_details"
	EventDetailsSharedFolderChangeConfidentialityDetails        = "shared_folder_change_confidentiality_details"
	EventDetailsSharedFolderChangeLinkPolicyDetails             = "shared_folder_change_link_policy_details"
	EventDetailsSharedFolderChangeMemberManagementPolicyDetails = "shared_folder_change_member_management_policy_details"
	EventDetailsSharedFolderChangeMemberPolicyDetails           = "shared_folder_change_member_policy_details"
	EventDetailsSharedFolderCreateDetails                       = "shared_folder_create_details"
	EventDetailsSharedFolderMountDetails                        = "shared_folder_mount_details"
	EventDetailsSharedFolderTransferOwnershipDetails            = "shared_folder_transfer_ownership_details"
	EventDetailsSharedFolderUnmountDetails                      = "shared_folder_unmount_details"
	EventDetailsSharedNoteOpenedDetails                         = "shared_note_opened_details"
	EventDetailsShmodelAppCreateDetails                         = "shmodel_app_create_details"
	EventDetailsShmodelCreateDetails                            = "shmodel_create_details"
	EventDetailsShmodelDisableDetails                           = "shmodel_disable_details"
	EventDetailsShmodelFbShareDetails                           = "shmodel_fb_share_details"
	EventDetailsShmodelGroupShareDetails                        = "shmodel_group_share_details"
	EventDetailsShmodelRemoveExpirationDetails                  = "shmodel_remove_expiration_details"
	EventDetailsShmodelSetExpirationDetails                     = "shmodel_set_expiration_details"
	EventDetailsShmodelTeamCopyDetails                          = "shmodel_team_copy_details"
	EventDetailsShmodelTeamDownloadDetails                      = "shmodel_team_download_details"
	EventDetailsShmodelTeamShareDetails                         = "shmodel_team_share_details"
	EventDetailsShmodelTeamViewDetails                          = "shmodel_team_view_details"
	EventDetailsShmodelVisibilityPasswordDetails                = "shmodel_visibility_password_details"
	EventDetailsShmodelVisibilityPublicDetails                  = "shmodel_visibility_public_details"
	EventDetailsShmodelVisibilityTeamOnlyDetails                = "shmodel_visibility_team_only_details"
	EventDetailsRemoveLogoutUrlDetails                          = "remove_logout_url_details"
	EventDetailsRemoveSsoUrlDetails                             = "remove_sso_url_details"
	EventDetailsSsoChangeCertDetails                            = "sso_change_cert_details"
	EventDetailsSsoChangeLoginUrlDetails                        = "sso_change_login_url_details"
	EventDetailsSsoChangeLogoutUrlDetails                       = "sso_change_logout_url_details"
	EventDetailsSsoChangeSamlIdentityModeDetails                = "sso_change_saml_identity_mode_details"
	EventDetailsTeamFolderChangeStatusDetails                   = "team_folder_change_status_details"
	EventDetailsTeamFolderCreateDetails                         = "team_folder_create_details"
	EventDetailsTeamFolderDowngradeDetails                      = "team_folder_downgrade_details"
	EventDetailsTeamFolderPermanentlyDeleteDetails              = "team_folder_permanently_delete_details"
	EventDetailsTeamFolderRenameDetails                         = "team_folder_rename_details"
	EventDetailsAccountCaptureChangePolicyDetails               = "account_capture_change_policy_details"
	EventDetailsAllowDownloadDisabledDetails                    = "allow_download_disabled_details"
	EventDetailsAllowDownloadEnabledDetails                     = "allow_download_enabled_details"
	EventDetailsDataPlacementRestrictionChangePolicyDetails     = "data_placement_restriction_change_policy_details"
	EventDetailsDataPlacementRestrictionSatisfyPolicyDetails    = "data_placement_restriction_satisfy_policy_details"
	EventDetailsDeviceApprovalsChangeDesktopPolicyDetails       = "device_approvals_change_desktop_policy_details"
	EventDetailsDeviceApprovalsChangeMobilePolicyDetails        = "device_approvals_change_mobile_policy_details"
	EventDetailsDeviceApprovalsChangeOverageActionDetails       = "device_approvals_change_overage_action_details"
	EventDetailsDeviceApprovalsChangeUnlinkActionDetails        = "device_approvals_change_unlink_action_details"
	EventDetailsEmmAddExceptionDetails                          = "emm_add_exception_details"
	EventDetailsEmmChangePolicyDetails                          = "emm_change_policy_details"
	EventDetailsEmmRemoveExceptionDetails                       = "emm_remove_exception_details"
	EventDetailsExtendedVersionHistoryChangePolicyDetails       = "extended_version_history_change_policy_details"
	EventDetailsFileCommentsChangePolicyDetails                 = "file_comments_change_policy_details"
	EventDetailsFileRequestsChangePolicyDetails                 = "file_requests_change_policy_details"
	EventDetailsFileRequestsEmailsEnabledDetails                = "file_requests_emails_enabled_details"
	EventDetailsFileRequestsEmailsRestrictedToTeamOnlyDetails   = "file_requests_emails_restricted_to_team_only_details"
	EventDetailsGoogleSsoChangePolicyDetails                    = "google_sso_change_policy_details"
	EventDetailsGroupUserManagementChangePolicyDetails          = "group_user_management_change_policy_details"
	EventDetailsMemberRequestsChangePolicyDetails               = "member_requests_change_policy_details"
	EventDetailsMemberSpaceLimitsAddExceptionDetails            = "member_space_limits_add_exception_details"
	EventDetailsMemberSpaceLimitsChangePolicyDetails            = "member_space_limits_change_policy_details"
	EventDetailsMemberSpaceLimitsRemoveExceptionDetails         = "member_space_limits_remove_exception_details"
	EventDetailsMemberSuggestionsChangePolicyDetails            = "member_suggestions_change_policy_details"
	EventDetailsMicrosoftOfficeAddinChangePolicyDetails         = "microsoft_office_addin_change_policy_details"
	EventDetailsNetworkControlChangePolicyDetails               = "network_control_change_policy_details"
	EventDetailsPaperChangeDeploymentPolicyDetails              = "paper_change_deployment_policy_details"
	EventDetailsPaperChangeMemberPolicyDetails                  = "paper_change_member_policy_details"
	EventDetailsPaperChangePolicyDetails                        = "paper_change_policy_details"
	EventDetailsPermanentDeleteChangePolicyDetails              = "permanent_delete_change_policy_details"
	EventDetailsSharingChangeFolderJoinPolicyDetails            = "sharing_change_folder_join_policy_details"
	EventDetailsSharingChangeLinkPolicyDetails                  = "sharing_change_link_policy_details"
	EventDetailsSharingChangeMemberPolicyDetails                = "sharing_change_member_policy_details"
	EventDetailsSmartSyncChangePolicyDetails                    = "smart_sync_change_policy_details"
	EventDetailsSmartSyncNotOptOutDetails                       = "smart_sync_not_opt_out_details"
	EventDetailsSmartSyncOptOutDetails                          = "smart_sync_opt_out_details"
	EventDetailsSsoChangePolicyDetails                          = "sso_change_policy_details"
	EventDetailsTfaChangePolicyDetails                          = "tfa_change_policy_details"
	EventDetailsTwoAccountChangePolicyDetails                   = "two_account_change_policy_details"
	EventDetailsWebSessionsChangeFixedLengthPolicyDetails       = "web_sessions_change_fixed_length_policy_details"
	EventDetailsWebSessionsChangeIdleLengthPolicyDetails        = "web_sessions_change_idle_length_policy_details"
	EventDetailsTeamProfileAddLogoDetails                       = "team_profile_add_logo_details"
	EventDetailsTeamProfileChangeLogoDetails                    = "team_profile_change_logo_details"
	EventDetailsTeamProfileChangeNameDetails                    = "team_profile_change_name_details"
	EventDetailsTeamProfileRemoveLogoDetails                    = "team_profile_remove_logo_details"
	EventDetailsTfaAddBackupPhoneDetails                        = "tfa_add_backup_phone_details"
	EventDetailsTfaAddSecurityKeyDetails                        = "tfa_add_security_key_details"
	EventDetailsTfaChangeBackupPhoneDetails                     = "tfa_change_backup_phone_details"
	EventDetailsTfaChangeStatusDetails                          = "tfa_change_status_details"
	EventDetailsTfaRemoveBackupPhoneDetails                     = "tfa_remove_backup_phone_details"
	EventDetailsTfaRemoveSecurityKeyDetails                     = "tfa_remove_security_key_details"
	EventDetailsTfaResetDetails                                 = "tfa_reset_details"
	EventDetailsMissingDetails                                  = "missing_details"
	EventDetailsOther                                           = "other"
)

// UnmarshalJSON deserializes into a EventDetails instance
func (u *EventDetails) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MemberChangeMembershipTypeDetails : Changed the membership type
		// (limited vs full) for team member.
		MemberChangeMembershipTypeDetails json.RawMessage `json:"member_change_membership_type_details,omitempty"`
		// MemberPermanentlyDeleteAccountContentsDetails : Permanently deleted
		// contents of a removed team member account.
		MemberPermanentlyDeleteAccountContentsDetails json.RawMessage `json:"member_permanently_delete_account_contents_details,omitempty"`
		// MemberSpaceLimitsChangeStatusDetails : Changed the status with
		// respect to whether the team member is under or over storage quota
		// specified by policy.
		MemberSpaceLimitsChangeStatusDetails json.RawMessage `json:"member_space_limits_change_status_details,omitempty"`
		// MemberTransferAccountContentsDetails : Transferred contents of a
		// removed team member account to another member.
		MemberTransferAccountContentsDetails json.RawMessage `json:"member_transfer_account_contents_details,omitempty"`
		// PaperEnabledUsersGroupAdditionDetails : Users added to Paper enabled
		// users list.
		PaperEnabledUsersGroupAdditionDetails json.RawMessage `json:"paper_enabled_users_group_addition_details,omitempty"`
		// PaperEnabledUsersGroupRemovalDetails : Users removed from Paper
		// enabled users list.
		PaperEnabledUsersGroupRemovalDetails json.RawMessage `json:"paper_enabled_users_group_removal_details,omitempty"`
		// PaperExternalViewAllowDetails : Paper external sharing policy
		// changed: anyone.
		PaperExternalViewAllowDetails json.RawMessage `json:"paper_external_view_allow_details,omitempty"`
		// PaperExternalViewDefaultTeamDetails : Paper external sharing policy
		// changed: default team.
		PaperExternalViewDefaultTeamDetails json.RawMessage `json:"paper_external_view_default_team_details,omitempty"`
		// PaperExternalViewForbidDetails : Paper external sharing policy
		// changed: team-only.
		PaperExternalViewForbidDetails json.RawMessage `json:"paper_external_view_forbid_details,omitempty"`
		// SfExternalInviteWarnDetails : Admin settings: team members see a
		// warning before sharing folders outside the team (DEPRECATED FEATURE).
		SfExternalInviteWarnDetails json.RawMessage `json:"sf_external_invite_warn_details,omitempty"`
		// TeamMergeFromDetails : Merged another team into this team.
		TeamMergeFromDetails json.RawMessage `json:"team_merge_from_details,omitempty"`
		// TeamMergeToDetails : Merged this team into another team.
		TeamMergeToDetails json.RawMessage `json:"team_merge_to_details,omitempty"`
		// AppLinkTeamDetails : Linked an app for team.
		AppLinkTeamDetails json.RawMessage `json:"app_link_team_details,omitempty"`
		// AppLinkUserDetails : Linked an app for team member.
		AppLinkUserDetails json.RawMessage `json:"app_link_user_details,omitempty"`
		// AppUnlinkTeamDetails : Unlinked an app for team.
		AppUnlinkTeamDetails json.RawMessage `json:"app_unlink_team_details,omitempty"`
		// AppUnlinkUserDetails : Unlinked an app for team member.
		AppUnlinkUserDetails json.RawMessage `json:"app_unlink_user_details,omitempty"`
		// DeviceChangeIpDesktopDetails : IP address associated with active
		// desktop session changed.
		DeviceChangeIpDesktopDetails json.RawMessage `json:"device_change_ip_desktop_details,omitempty"`
		// DeviceChangeIpMobileDetails : IP address associated with active
		// mobile session changed.
		DeviceChangeIpMobileDetails json.RawMessage `json:"device_change_ip_mobile_details,omitempty"`
		// DeviceChangeIpWebDetails : IP address associated with active Web
		// session changed.
		DeviceChangeIpWebDetails json.RawMessage `json:"device_change_ip_web_details,omitempty"`
		// DeviceDeleteOnUnlinkFailDetails : Failed to delete all files from an
		// unlinked device.
		DeviceDeleteOnUnlinkFailDetails json.RawMessage `json:"device_delete_on_unlink_fail_details,omitempty"`
		// DeviceDeleteOnUnlinkSuccessDetails : Deleted all files from an
		// unlinked device.
		DeviceDeleteOnUnlinkSuccessDetails json.RawMessage `json:"device_delete_on_unlink_success_details,omitempty"`
		// DeviceLinkFailDetails : Failed to link a device.
		DeviceLinkFailDetails json.RawMessage `json:"device_link_fail_details,omitempty"`
		// DeviceLinkSuccessDetails : Linked a device.
		DeviceLinkSuccessDetails json.RawMessage `json:"device_link_success_details,omitempty"`
		// DeviceManagementDisabledDetails : Disable Device Management.
		DeviceManagementDisabledDetails json.RawMessage `json:"device_management_disabled_details,omitempty"`
		// DeviceManagementEnabledDetails : Enable Device Management.
		DeviceManagementEnabledDetails json.RawMessage `json:"device_management_enabled_details,omitempty"`
		// DeviceUnlinkDetails : Disconnected a device.
		DeviceUnlinkDetails json.RawMessage `json:"device_unlink_details,omitempty"`
		// EmmRefreshAuthTokenDetails : Refreshed the auth token used for
		// setting up enterprise mobility management.
		EmmRefreshAuthTokenDetails json.RawMessage `json:"emm_refresh_auth_token_details,omitempty"`
		// AccountCaptureChangeAvailabilityDetails : Granted or revoked the
		// option to enable account capture on domains belonging to the team.
		AccountCaptureChangeAvailabilityDetails json.RawMessage `json:"account_capture_change_availability_details,omitempty"`
		// AccountCaptureMigrateAccountDetails : Account captured user migrated
		// their account to the team.
		AccountCaptureMigrateAccountDetails json.RawMessage `json:"account_capture_migrate_account_details,omitempty"`
		// AccountCaptureRelinquishAccountDetails : Account captured user
		// relinquished their account by changing the email address associated
		// with it.
		AccountCaptureRelinquishAccountDetails json.RawMessage `json:"account_capture_relinquish_account_details,omitempty"`
		// DisabledDomainInvitesDetails : Disabled domain invites.
		DisabledDomainInvitesDetails json.RawMessage `json:"disabled_domain_invites_details,omitempty"`
		// DomainInvitesApproveRequestToJoinTeamDetails : Approved a member's
		// request to join the team.
		DomainInvitesApproveRequestToJoinTeamDetails json.RawMessage `json:"domain_invites_approve_request_to_join_team_details,omitempty"`
		// DomainInvitesDeclineRequestToJoinTeamDetails : Declined a user's
		// request to join the team.
		DomainInvitesDeclineRequestToJoinTeamDetails json.RawMessage `json:"domain_invites_decline_request_to_join_team_details,omitempty"`
		// DomainInvitesEmailExistingUsersDetails : Sent domain invites to
		// existing domain accounts.
		DomainInvitesEmailExistingUsersDetails json.RawMessage `json:"domain_invites_email_existing_users_details,omitempty"`
		// DomainInvitesRequestToJoinTeamDetails : Asked to join the team.
		DomainInvitesRequestToJoinTeamDetails json.RawMessage `json:"domain_invites_request_to_join_team_details,omitempty"`
		// DomainInvitesSetInviteNewUserPrefToNoDetails : Turned off
		// u201cAutomatically invite new usersu201d.
		DomainInvitesSetInviteNewUserPrefToNoDetails json.RawMessage `json:"domain_invites_set_invite_new_user_pref_to_no_details,omitempty"`
		// DomainInvitesSetInviteNewUserPrefToYesDetails : Turned on
		// u201cAutomatically invite new usersu201d.
		DomainInvitesSetInviteNewUserPrefToYesDetails json.RawMessage `json:"domain_invites_set_invite_new_user_pref_to_yes_details,omitempty"`
		// DomainVerificationAddDomainFailDetails : Failed to verify a domain
		// belonging to the team.
		DomainVerificationAddDomainFailDetails json.RawMessage `json:"domain_verification_add_domain_fail_details,omitempty"`
		// DomainVerificationAddDomainSuccessDetails : Verified a domain
		// belonging to the team.
		DomainVerificationAddDomainSuccessDetails json.RawMessage `json:"domain_verification_add_domain_success_details,omitempty"`
		// DomainVerificationRemoveDomainDetails : Removed a domain from the
		// list of verified domains belonging to the team.
		DomainVerificationRemoveDomainDetails json.RawMessage `json:"domain_verification_remove_domain_details,omitempty"`
		// EnabledDomainInvitesDetails : Enabled domain invites.
		EnabledDomainInvitesDetails json.RawMessage `json:"enabled_domain_invites_details,omitempty"`
		// CreateFolderDetails : Created folders.
		CreateFolderDetails json.RawMessage `json:"create_folder_details,omitempty"`
		// FileAddDetails : Added files and/or folders.
		FileAddDetails json.RawMessage `json:"file_add_details,omitempty"`
		// FileCopyDetails : Copied files and/or folders.
		FileCopyDetails json.RawMessage `json:"file_copy_details,omitempty"`
		// FileDeleteDetails : Deleted files and/or folders.
		FileDeleteDetails json.RawMessage `json:"file_delete_details,omitempty"`
		// FileDownloadDetails : Downloaded files and/or folders.
		FileDownloadDetails json.RawMessage `json:"file_download_details,omitempty"`
		// FileEditDetails : Edited files.
		FileEditDetails json.RawMessage `json:"file_edit_details,omitempty"`
		// FileGetCopyReferenceDetails : Create a copy reference to a file or
		// folder.
		FileGetCopyReferenceDetails json.RawMessage `json:"file_get_copy_reference_details,omitempty"`
		// FileMoveDetails : Moved files and/or folders.
		FileMoveDetails json.RawMessage `json:"file_move_details,omitempty"`
		// FilePermanentlyDeleteDetails : Permanently deleted files and/or
		// folders.
		FilePermanentlyDeleteDetails json.RawMessage `json:"file_permanently_delete_details,omitempty"`
		// FilePreviewDetails : Previewed files and/or folders.
		FilePreviewDetails json.RawMessage `json:"file_preview_details,omitempty"`
		// FileRenameDetails : Renamed files and/or folders.
		FileRenameDetails json.RawMessage `json:"file_rename_details,omitempty"`
		// FileRestoreDetails : Restored deleted files and/or folders.
		FileRestoreDetails json.RawMessage `json:"file_restore_details,omitempty"`
		// FileRevertDetails : Reverted files to a previous version.
		FileRevertDetails json.RawMessage `json:"file_revert_details,omitempty"`
		// FileRollbackChangesDetails : Rolled back file change location
		// changes.
		FileRollbackChangesDetails json.RawMessage `json:"file_rollback_changes_details,omitempty"`
		// FileSaveCopyReferenceDetails : Save a file or folder using a copy
		// reference.
		FileSaveCopyReferenceDetails json.RawMessage `json:"file_save_copy_reference_details,omitempty"`
		// FileRequestAddDeadlineDetails : Added a deadline to a file request.
		FileRequestAddDeadlineDetails json.RawMessage `json:"file_request_add_deadline_details,omitempty"`
		// FileRequestChangeFolderDetails : Changed the file request folder.
		FileRequestChangeFolderDetails json.RawMessage `json:"file_request_change_folder_details,omitempty"`
		// FileRequestChangeTitleDetails : Change the file request title.
		FileRequestChangeTitleDetails json.RawMessage `json:"file_request_change_title_details,omitempty"`
		// FileRequestCloseDetails : Closed a file request.
		FileRequestCloseDetails json.RawMessage `json:"file_request_close_details,omitempty"`
		// FileRequestCreateDetails : Created a file request.
		FileRequestCreateDetails json.RawMessage `json:"file_request_create_details,omitempty"`
		// FileRequestReceiveFileDetails : Received files for a file request.
		FileRequestReceiveFileDetails json.RawMessage `json:"file_request_receive_file_details,omitempty"`
		// FileRequestRemoveDeadlineDetails : Removed the file request deadline.
		FileRequestRemoveDeadlineDetails json.RawMessage `json:"file_request_remove_deadline_details,omitempty"`
		// FileRequestSendDetails : Sent file request to users via email.
		FileRequestSendDetails json.RawMessage `json:"file_request_send_details,omitempty"`
		// GroupAddExternalIdDetails : Added an external ID for group.
		GroupAddExternalIdDetails json.RawMessage `json:"group_add_external_id_details,omitempty"`
		// GroupAddMemberDetails : Added team members to a group.
		GroupAddMemberDetails json.RawMessage `json:"group_add_member_details,omitempty"`
		// GroupChangeExternalIdDetails : Changed the external ID for group.
		GroupChangeExternalIdDetails json.RawMessage `json:"group_change_external_id_details,omitempty"`
		// GroupChangeManagementTypeDetails : Changed group management type.
		GroupChangeManagementTypeDetails json.RawMessage `json:"group_change_management_type_details,omitempty"`
		// GroupChangeMemberRoleDetails : Changed the manager permissions
		// belonging to a group member.
		GroupChangeMemberRoleDetails json.RawMessage `json:"group_change_member_role_details,omitempty"`
		// GroupCreateDetails : Created a group.
		GroupCreateDetails json.RawMessage `json:"group_create_details,omitempty"`
		// GroupDeleteDetails : Deleted a group.
		GroupDeleteDetails json.RawMessage `json:"group_delete_details,omitempty"`
		// GroupDescriptionUpdatedDetails : Updated a group.
		GroupDescriptionUpdatedDetails json.RawMessage `json:"group_description_updated_details,omitempty"`
		// GroupJoinPolicyUpdatedDetails : Updated a group join policy.
		GroupJoinPolicyUpdatedDetails json.RawMessage `json:"group_join_policy_updated_details,omitempty"`
		// GroupMovedDetails : Moved a group.
		GroupMovedDetails json.RawMessage `json:"group_moved_details,omitempty"`
		// GroupRemoveExternalIdDetails : Removed the external ID for group.
		GroupRemoveExternalIdDetails json.RawMessage `json:"group_remove_external_id_details,omitempty"`
		// GroupRemoveMemberDetails : Removed team members from a group.
		GroupRemoveMemberDetails json.RawMessage `json:"group_remove_member_details,omitempty"`
		// GroupRenameDetails : Renamed a group.
		GroupRenameDetails json.RawMessage `json:"group_rename_details,omitempty"`
		// EmmLoginSuccessDetails : Signed in using the Dropbox EMM app.
		EmmLoginSuccessDetails json.RawMessage `json:"emm_login_success_details,omitempty"`
		// LogoutDetails : Signed out.
		LogoutDetails json.RawMessage `json:"logout_details,omitempty"`
		// PasswordLoginFailDetails : Failed to sign in using a password.
		PasswordLoginFailDetails json.RawMessage `json:"password_login_fail_details,omitempty"`
		// PasswordLoginSuccessDetails : Signed in using a password.
		PasswordLoginSuccessDetails json.RawMessage `json:"password_login_success_details,omitempty"`
		// ResellerSupportSessionEndDetails : Ended reseller support session.
		ResellerSupportSessionEndDetails json.RawMessage `json:"reseller_support_session_end_details,omitempty"`
		// ResellerSupportSessionStartDetails : Started reseller support
		// session.
		ResellerSupportSessionStartDetails json.RawMessage `json:"reseller_support_session_start_details,omitempty"`
		// SignInAsSessionEndDetails : Ended admin sign-in-as session.
		SignInAsSessionEndDetails json.RawMessage `json:"sign_in_as_session_end_details,omitempty"`
		// SignInAsSessionStartDetails : Started admin sign-in-as session.
		SignInAsSessionStartDetails json.RawMessage `json:"sign_in_as_session_start_details,omitempty"`
		// SsoLoginFailDetails : Failed to sign in using SSO.
		SsoLoginFailDetails json.RawMessage `json:"sso_login_fail_details,omitempty"`
		// MemberAddNameDetails : Set team member name when joining team.
		MemberAddNameDetails json.RawMessage `json:"member_add_name_details,omitempty"`
		// MemberChangeAdminRoleDetails : Change the admin role belonging to
		// team member.
		MemberChangeAdminRoleDetails json.RawMessage `json:"member_change_admin_role_details,omitempty"`
		// MemberChangeEmailDetails : Changed team member email address.
		MemberChangeEmailDetails json.RawMessage `json:"member_change_email_details,omitempty"`
		// MemberChangeNameDetails : Changed team member name.
		MemberChangeNameDetails json.RawMessage `json:"member_change_name_details,omitempty"`
		// MemberChangeStatusDetails : Changed the membership status of a team
		// member.
		MemberChangeStatusDetails json.RawMessage `json:"member_change_status_details,omitempty"`
		// MemberSuggestDetails : Suggested a new team member to be added to the
		// team.
		MemberSuggestDetails json.RawMessage `json:"member_suggest_details,omitempty"`
		// PaperContentAddMemberDetails : Added users to the membership of a
		// Paper doc or folder.
		PaperContentAddMemberDetails json.RawMessage `json:"paper_content_add_member_details,omitempty"`
		// PaperContentAddToFolderDetails : Added Paper doc or folder to a
		// folder.
		PaperContentAddToFolderDetails json.RawMessage `json:"paper_content_add_to_folder_details,omitempty"`
		// PaperContentArchiveDetails : Archived Paper doc or folder.
		PaperContentArchiveDetails json.RawMessage `json:"paper_content_archive_details,omitempty"`
		// PaperContentChangeSubscriptionDetails : Followed or unfollowed a
		// Paper doc or folder.
		PaperContentChangeSubscriptionDetails json.RawMessage `json:"paper_content_change_subscription_details,omitempty"`
		// PaperContentCreateDetails : Created a Paper doc or folder.
		PaperContentCreateDetails json.RawMessage `json:"paper_content_create_details,omitempty"`
		// PaperContentPermanentlyDeleteDetails : Permanently deleted a Paper
		// doc or folder.
		PaperContentPermanentlyDeleteDetails json.RawMessage `json:"paper_content_permanently_delete_details,omitempty"`
		// PaperContentRemoveFromFolderDetails : Removed Paper doc or folder
		// from a folder.
		PaperContentRemoveFromFolderDetails json.RawMessage `json:"paper_content_remove_from_folder_details,omitempty"`
		// PaperContentRemoveMemberDetails : Removed a user from the membership
		// of a Paper doc or folder.
		PaperContentRemoveMemberDetails json.RawMessage `json:"paper_content_remove_member_details,omitempty"`
		// PaperContentRenameDetails : Renamed Paper doc or folder.
		PaperContentRenameDetails json.RawMessage `json:"paper_content_rename_details,omitempty"`
		// PaperContentRestoreDetails : Restored an archived Paper doc or
		// folder.
		PaperContentRestoreDetails json.RawMessage `json:"paper_content_restore_details,omitempty"`
		// PaperDocAddCommentDetails : Added a Paper doc comment.
		PaperDocAddCommentDetails json.RawMessage `json:"paper_doc_add_comment_details,omitempty"`
		// PaperDocChangeMemberRoleDetails : Changed the access type of a Paper
		// doc member.
		PaperDocChangeMemberRoleDetails json.RawMessage `json:"paper_doc_change_member_role_details,omitempty"`
		// PaperDocChangeSharingPolicyDetails : Changed the sharing policy for
		// Paper doc.
		PaperDocChangeSharingPolicyDetails json.RawMessage `json:"paper_doc_change_sharing_policy_details,omitempty"`
		// PaperDocDeletedDetails : Paper doc archived.
		PaperDocDeletedDetails json.RawMessage `json:"paper_doc_deleted_details,omitempty"`
		// PaperDocDeleteCommentDetails : Deleted a Paper doc comment.
		PaperDocDeleteCommentDetails json.RawMessage `json:"paper_doc_delete_comment_details,omitempty"`
		// PaperDocDownloadDetails : Downloaded a Paper doc in a particular
		// output format.
		PaperDocDownloadDetails json.RawMessage `json:"paper_doc_download_details,omitempty"`
		// PaperDocEditDetails : Edited a Paper doc.
		PaperDocEditDetails json.RawMessage `json:"paper_doc_edit_details,omitempty"`
		// PaperDocEditCommentDetails : Edited a Paper doc comment.
		PaperDocEditCommentDetails json.RawMessage `json:"paper_doc_edit_comment_details,omitempty"`
		// PaperDocFollowedDetails : Followed a Paper doc.
		PaperDocFollowedDetails json.RawMessage `json:"paper_doc_followed_details,omitempty"`
		// PaperDocMentionDetails : Mentioned a member in a Paper doc.
		PaperDocMentionDetails json.RawMessage `json:"paper_doc_mention_details,omitempty"`
		// PaperDocRequestAccessDetails : Requested to be a member on a Paper
		// doc.
		PaperDocRequestAccessDetails json.RawMessage `json:"paper_doc_request_access_details,omitempty"`
		// PaperDocResolveCommentDetails : Paper doc comment resolved.
		PaperDocResolveCommentDetails json.RawMessage `json:"paper_doc_resolve_comment_details,omitempty"`
		// PaperDocRevertDetails : Restored a Paper doc to previous revision.
		PaperDocRevertDetails json.RawMessage `json:"paper_doc_revert_details,omitempty"`
		// PaperDocSlackShareDetails : Paper doc link shared via slack.
		PaperDocSlackShareDetails json.RawMessage `json:"paper_doc_slack_share_details,omitempty"`
		// PaperDocTeamInviteDetails : Paper doc shared with team member.
		PaperDocTeamInviteDetails json.RawMessage `json:"paper_doc_team_invite_details,omitempty"`
		// PaperDocUnresolveCommentDetails : Unresolved a Paper doc comment.
		PaperDocUnresolveCommentDetails json.RawMessage `json:"paper_doc_unresolve_comment_details,omitempty"`
		// PaperDocViewDetails : Viewed Paper doc.
		PaperDocViewDetails json.RawMessage `json:"paper_doc_view_details,omitempty"`
		// PaperFolderDeletedDetails : Paper folder archived.
		PaperFolderDeletedDetails json.RawMessage `json:"paper_folder_deleted_details,omitempty"`
		// PaperFolderFollowedDetails : Followed a Paper folder.
		PaperFolderFollowedDetails json.RawMessage `json:"paper_folder_followed_details,omitempty"`
		// PaperFolderTeamInviteDetails : Paper folder shared with team member.
		PaperFolderTeamInviteDetails json.RawMessage `json:"paper_folder_team_invite_details,omitempty"`
		// PasswordChangeDetails : Changed password.
		PasswordChangeDetails json.RawMessage `json:"password_change_details,omitempty"`
		// PasswordResetDetails : Reset password.
		PasswordResetDetails json.RawMessage `json:"password_reset_details,omitempty"`
		// PasswordResetAllDetails : Reset all team member passwords.
		PasswordResetAllDetails json.RawMessage `json:"password_reset_all_details,omitempty"`
		// EmmCreateExceptionsReportDetails : EMM excluded users report created.
		EmmCreateExceptionsReportDetails json.RawMessage `json:"emm_create_exceptions_report_details,omitempty"`
		// EmmCreateUsageReportDetails : EMM mobile app usage report created.
		EmmCreateUsageReportDetails json.RawMessage `json:"emm_create_usage_report_details,omitempty"`
		// SmartSyncCreateAdminPrivilegeReportDetails : Smart Sync non-admin
		// devices report created.
		SmartSyncCreateAdminPrivilegeReportDetails json.RawMessage `json:"smart_sync_create_admin_privilege_report_details,omitempty"`
		// TeamActivityCreateReportDetails : Created a team activity report.
		TeamActivityCreateReportDetails json.RawMessage `json:"team_activity_create_report_details,omitempty"`
		// CollectionShareDetails : Shared an album.
		CollectionShareDetails json.RawMessage `json:"collection_share_details,omitempty"`
		// FileAddCommentDetails : Added a file comment.
		FileAddCommentDetails json.RawMessage `json:"file_add_comment_details,omitempty"`
		// FileLikeCommentDetails : Liked a file comment.
		FileLikeCommentDetails json.RawMessage `json:"file_like_comment_details,omitempty"`
		// FileUnlikeCommentDetails : Unliked a file comment.
		FileUnlikeCommentDetails json.RawMessage `json:"file_unlike_comment_details,omitempty"`
		// NoteAclInviteOnlyDetails : Changed a Paper document to be
		// invite-only.
		NoteAclInviteOnlyDetails json.RawMessage `json:"note_acl_invite_only_details,omitempty"`
		// NoteAclLinkDetails : Changed a Paper document to be link accessible.
		NoteAclLinkDetails json.RawMessage `json:"note_acl_link_details,omitempty"`
		// NoteAclTeamLinkDetails : Changed a Paper document to be link
		// accessible for the team.
		NoteAclTeamLinkDetails json.RawMessage `json:"note_acl_team_link_details,omitempty"`
		// NoteSharedDetails : Shared a Paper doc.
		NoteSharedDetails json.RawMessage `json:"note_shared_details,omitempty"`
		// NoteShareReceiveDetails : Shared Paper document received.
		NoteShareReceiveDetails json.RawMessage `json:"note_share_receive_details,omitempty"`
		// OpenNoteSharedDetails : Opened a shared Paper doc.
		OpenNoteSharedDetails json.RawMessage `json:"open_note_shared_details,omitempty"`
		// SfAddGroupDetails : Added the team to a shared folder.
		SfAddGroupDetails json.RawMessage `json:"sf_add_group_details,omitempty"`
		// SfAllowNonMembersToViewSharedLinksDetails : Allowed non collaborators
		// to view links to files in a shared folder.
		SfAllowNonMembersToViewSharedLinksDetails json.RawMessage `json:"sf_allow_non_members_to_view_shared_links_details,omitempty"`
		// SfInviteGroupDetails : Invited a group to a shared folder.
		SfInviteGroupDetails json.RawMessage `json:"sf_invite_group_details,omitempty"`
		// SfNestDetails : Changed parent of shared folder.
		SfNestDetails json.RawMessage `json:"sf_nest_details,omitempty"`
		// SfTeamDeclineDetails : Declined a team member's invitation to a
		// shared folder.
		SfTeamDeclineDetails json.RawMessage `json:"sf_team_decline_details,omitempty"`
		// SfTeamGrantAccessDetails : Granted access to a shared folder.
		SfTeamGrantAccessDetails json.RawMessage `json:"sf_team_grant_access_details,omitempty"`
		// SfTeamInviteDetails : Invited team members to a shared folder.
		SfTeamInviteDetails json.RawMessage `json:"sf_team_invite_details,omitempty"`
		// SfTeamInviteChangeRoleDetails : Changed a team member's role in a
		// shared folder.
		SfTeamInviteChangeRoleDetails json.RawMessage `json:"sf_team_invite_change_role_details,omitempty"`
		// SfTeamJoinDetails : Joined a team member's shared folder.
		SfTeamJoinDetails json.RawMessage `json:"sf_team_join_details,omitempty"`
		// SfTeamJoinFromOobLinkDetails : Joined a team member's shared folder
		// from a link.
		SfTeamJoinFromOobLinkDetails json.RawMessage `json:"sf_team_join_from_oob_link_details,omitempty"`
		// SfTeamUninviteDetails : Unshared a folder with a team member.
		SfTeamUninviteDetails json.RawMessage `json:"sf_team_uninvite_details,omitempty"`
		// SharedContentAddInviteesDetails : Sent an email invitation to the
		// membership of a shared file or folder.
		SharedContentAddInviteesDetails json.RawMessage `json:"shared_content_add_invitees_details,omitempty"`
		// SharedContentAddLinkExpiryDetails : Added an expiry to the link for
		// the shared file or folder.
		SharedContentAddLinkExpiryDetails json.RawMessage `json:"shared_content_add_link_expiry_details,omitempty"`
		// SharedContentAddLinkPasswordDetails : Added a password to the link
		// for the shared file or folder.
		SharedContentAddLinkPasswordDetails json.RawMessage `json:"shared_content_add_link_password_details,omitempty"`
		// SharedContentAddMemberDetails : Added users and/or groups to the
		// membership of a shared file or folder.
		SharedContentAddMemberDetails json.RawMessage `json:"shared_content_add_member_details,omitempty"`
		// SharedContentChangeDownloadsPolicyDetails : Changed whether members
		// can download the shared file or folder.
		SharedContentChangeDownloadsPolicyDetails json.RawMessage `json:"shared_content_change_downloads_policy_details,omitempty"`
		// SharedContentChangeInviteeRoleDetails : Changed the access type of an
		// invitee to a shared file or folder before the invitation was claimed.
		SharedContentChangeInviteeRoleDetails json.RawMessage `json:"shared_content_change_invitee_role_details,omitempty"`
		// SharedContentChangeLinkAudienceDetails : Changed the audience of the
		// link for a shared file or folder.
		SharedContentChangeLinkAudienceDetails json.RawMessage `json:"shared_content_change_link_audience_details,omitempty"`
		// SharedContentChangeLinkExpiryDetails : Changed the expiry of the link
		// for the shared file or folder.
		SharedContentChangeLinkExpiryDetails json.RawMessage `json:"shared_content_change_link_expiry_details,omitempty"`
		// SharedContentChangeLinkPasswordDetails : Changed the password on the
		// link for the shared file or folder.
		SharedContentChangeLinkPasswordDetails json.RawMessage `json:"shared_content_change_link_password_details,omitempty"`
		// SharedContentChangeMemberRoleDetails : Changed the access type of a
		// shared file or folder member.
		SharedContentChangeMemberRoleDetails json.RawMessage `json:"shared_content_change_member_role_details,omitempty"`
		// SharedContentChangeViewerInfoPolicyDetails : Changed whether members
		// can see who viewed the shared file or folder.
		SharedContentChangeViewerInfoPolicyDetails json.RawMessage `json:"shared_content_change_viewer_info_policy_details,omitempty"`
		// SharedContentClaimInvitationDetails : Claimed membership to a team
		// member's shared folder.
		SharedContentClaimInvitationDetails json.RawMessage `json:"shared_content_claim_invitation_details,omitempty"`
		// SharedContentCopyDetails : Copied the shared file or folder to own
		// Dropbox.
		SharedContentCopyDetails json.RawMessage `json:"shared_content_copy_details,omitempty"`
		// SharedContentDownloadDetails : Downloaded the shared file or folder.
		SharedContentDownloadDetails json.RawMessage `json:"shared_content_download_details,omitempty"`
		// SharedContentRelinquishMembershipDetails : Left the membership of a
		// shared file or folder.
		SharedContentRelinquishMembershipDetails json.RawMessage `json:"shared_content_relinquish_membership_details,omitempty"`
		// SharedContentRemoveInviteeDetails : Removed an invitee from the
		// membership of a shared file or folder before it was claimed.
		SharedContentRemoveInviteeDetails json.RawMessage `json:"shared_content_remove_invitee_details,omitempty"`
		// SharedContentRemoveLinkExpiryDetails : Removed the expiry of the link
		// for the shared file or folder.
		SharedContentRemoveLinkExpiryDetails json.RawMessage `json:"shared_content_remove_link_expiry_details,omitempty"`
		// SharedContentRemoveLinkPasswordDetails : Removed the password on the
		// link for the shared file or folder.
		SharedContentRemoveLinkPasswordDetails json.RawMessage `json:"shared_content_remove_link_password_details,omitempty"`
		// SharedContentRemoveMemberDetails : Removed a user or a group from the
		// membership of a shared file or folder.
		SharedContentRemoveMemberDetails json.RawMessage `json:"shared_content_remove_member_details,omitempty"`
		// SharedContentRequestAccessDetails : Requested to be on the membership
		// of a shared file or folder.
		SharedContentRequestAccessDetails json.RawMessage `json:"shared_content_request_access_details,omitempty"`
		// SharedContentUnshareDetails : Unshared a shared file or folder by
		// clearing its membership and turning off its link.
		SharedContentUnshareDetails json.RawMessage `json:"shared_content_unshare_details,omitempty"`
		// SharedContentViewDetails : Previewed the shared file or folder.
		SharedContentViewDetails json.RawMessage `json:"shared_content_view_details,omitempty"`
		// SharedFolderChangeConfidentialityDetails : Set or unset the
		// confidential flag on a shared folder.
		SharedFolderChangeConfidentialityDetails json.RawMessage `json:"shared_folder_change_confidentiality_details,omitempty"`
		// SharedFolderChangeLinkPolicyDetails : Changed who can access the
		// shared folder via a link.
		SharedFolderChangeLinkPolicyDetails json.RawMessage `json:"shared_folder_change_link_policy_details,omitempty"`
		// SharedFolderChangeMemberManagementPolicyDetails : Changed who can
		// manage the membership of a shared folder.
		SharedFolderChangeMemberManagementPolicyDetails json.RawMessage `json:"shared_folder_change_member_management_policy_details,omitempty"`
		// SharedFolderChangeMemberPolicyDetails : Changed who can become a
		// member of the shared folder.
		SharedFolderChangeMemberPolicyDetails json.RawMessage `json:"shared_folder_change_member_policy_details,omitempty"`
		// SharedFolderCreateDetails : Created a shared folder.
		SharedFolderCreateDetails json.RawMessage `json:"shared_folder_create_details,omitempty"`
		// SharedFolderMountDetails : Added a shared folder to own Dropbox.
		SharedFolderMountDetails json.RawMessage `json:"shared_folder_mount_details,omitempty"`
		// SharedFolderTransferOwnershipDetails : Transferred the ownership of a
		// shared folder to another member.
		SharedFolderTransferOwnershipDetails json.RawMessage `json:"shared_folder_transfer_ownership_details,omitempty"`
		// SharedFolderUnmountDetails : Deleted a shared folder from Dropbox.
		SharedFolderUnmountDetails json.RawMessage `json:"shared_folder_unmount_details,omitempty"`
		// SharedNoteOpenedDetails : Shared Paper document was opened.
		SharedNoteOpenedDetails json.RawMessage `json:"shared_note_opened_details,omitempty"`
		// ShmodelAppCreateDetails : Created a link to a file using an app.
		ShmodelAppCreateDetails json.RawMessage `json:"shmodel_app_create_details,omitempty"`
		// ShmodelCreateDetails : Created a new link.
		ShmodelCreateDetails json.RawMessage `json:"shmodel_create_details,omitempty"`
		// ShmodelDisableDetails : Removed a link.
		ShmodelDisableDetails json.RawMessage `json:"shmodel_disable_details,omitempty"`
		// ShmodelFbShareDetails : Shared a link with Facebook users.
		ShmodelFbShareDetails json.RawMessage `json:"shmodel_fb_share_details,omitempty"`
		// ShmodelGroupShareDetails : Shared a link with a group.
		ShmodelGroupShareDetails json.RawMessage `json:"shmodel_group_share_details,omitempty"`
		// ShmodelRemoveExpirationDetails : Removed the expiration date from a
		// link.
		ShmodelRemoveExpirationDetails json.RawMessage `json:"shmodel_remove_expiration_details,omitempty"`
		// ShmodelSetExpirationDetails : Added an expiration date to a link.
		ShmodelSetExpirationDetails json.RawMessage `json:"shmodel_set_expiration_details,omitempty"`
		// ShmodelTeamCopyDetails : Added a team member's file/folder to their
		// Dropbox from a link.
		ShmodelTeamCopyDetails json.RawMessage `json:"shmodel_team_copy_details,omitempty"`
		// ShmodelTeamDownloadDetails : Downloaded a team member's file/folder
		// from a link.
		ShmodelTeamDownloadDetails json.RawMessage `json:"shmodel_team_download_details,omitempty"`
		// ShmodelTeamShareDetails : Shared a link with team members.
		ShmodelTeamShareDetails json.RawMessage `json:"shmodel_team_share_details,omitempty"`
		// ShmodelTeamViewDetails : Opened a team member's link.
		ShmodelTeamViewDetails json.RawMessage `json:"shmodel_team_view_details,omitempty"`
		// ShmodelVisibilityPasswordDetails : Password-protected a link.
		ShmodelVisibilityPasswordDetails json.RawMessage `json:"shmodel_visibility_password_details,omitempty"`
		// ShmodelVisibilityPublicDetails : Made a file/folder visible to anyone
		// with the link.
		ShmodelVisibilityPublicDetails json.RawMessage `json:"shmodel_visibility_public_details,omitempty"`
		// ShmodelVisibilityTeamOnlyDetails : Made a file/folder visible only to
		// team members with the link.
		ShmodelVisibilityTeamOnlyDetails json.RawMessage `json:"shmodel_visibility_team_only_details,omitempty"`
		// RemoveLogoutUrlDetails : Removed single sign-on logout URL.
		RemoveLogoutUrlDetails json.RawMessage `json:"remove_logout_url_details,omitempty"`
		// RemoveSsoUrlDetails : Changed the sign-out URL for SSO.
		RemoveSsoUrlDetails json.RawMessage `json:"remove_sso_url_details,omitempty"`
		// SsoChangeCertDetails : Changed the X.509 certificate for SSO.
		SsoChangeCertDetails json.RawMessage `json:"sso_change_cert_details,omitempty"`
		// SsoChangeLoginUrlDetails : Changed the sign-in URL for SSO.
		SsoChangeLoginUrlDetails json.RawMessage `json:"sso_change_login_url_details,omitempty"`
		// SsoChangeLogoutUrlDetails : Changed the sign-out URL for SSO.
		SsoChangeLogoutUrlDetails json.RawMessage `json:"sso_change_logout_url_details,omitempty"`
		// SsoChangeSamlIdentityModeDetails : Changed the SAML identity mode for
		// SSO.
		SsoChangeSamlIdentityModeDetails json.RawMessage `json:"sso_change_saml_identity_mode_details,omitempty"`
		// TeamFolderChangeStatusDetails : Changed the archival status of a team
		// folder.
		TeamFolderChangeStatusDetails json.RawMessage `json:"team_folder_change_status_details,omitempty"`
		// TeamFolderCreateDetails : Created a new team folder in active status.
		TeamFolderCreateDetails json.RawMessage `json:"team_folder_create_details,omitempty"`
		// TeamFolderDowngradeDetails : Downgraded a team folder to a regular
		// shared folder.
		TeamFolderDowngradeDetails json.RawMessage `json:"team_folder_downgrade_details,omitempty"`
		// TeamFolderPermanentlyDeleteDetails : Permanently deleted an archived
		// team folder.
		TeamFolderPermanentlyDeleteDetails json.RawMessage `json:"team_folder_permanently_delete_details,omitempty"`
		// TeamFolderRenameDetails : Renamed an active or archived team folder.
		TeamFolderRenameDetails json.RawMessage `json:"team_folder_rename_details,omitempty"`
		// AccountCaptureChangePolicyDetails : Changed the account capture
		// policy on a domain belonging to the team.
		AccountCaptureChangePolicyDetails json.RawMessage `json:"account_capture_change_policy_details,omitempty"`
		// AllowDownloadDisabledDetails : Disabled allow downloads.
		AllowDownloadDisabledDetails json.RawMessage `json:"allow_download_disabled_details,omitempty"`
		// AllowDownloadEnabledDetails : Enabled allow downloads.
		AllowDownloadEnabledDetails json.RawMessage `json:"allow_download_enabled_details,omitempty"`
		// DataPlacementRestrictionChangePolicyDetails : Set a restriction
		// policy regarding the location of data centers where team data
		// resides.
		DataPlacementRestrictionChangePolicyDetails json.RawMessage `json:"data_placement_restriction_change_policy_details,omitempty"`
		// DataPlacementRestrictionSatisfyPolicyDetails : Satisfied a previously
		// set restriction policy regarding the location of data centers where
		// team data resides (i.e. all data have been migrated according to the
		// restriction placed).
		DataPlacementRestrictionSatisfyPolicyDetails json.RawMessage `json:"data_placement_restriction_satisfy_policy_details,omitempty"`
		// DeviceApprovalsChangeDesktopPolicyDetails : Set or removed a limit on
		// the number of computers each team member can link to their work
		// Dropbox account.
		DeviceApprovalsChangeDesktopPolicyDetails json.RawMessage `json:"device_approvals_change_desktop_policy_details,omitempty"`
		// DeviceApprovalsChangeMobilePolicyDetails : Set or removed a limit on
		// the number of mobiles devices each team member can link to their work
		// Dropbox account.
		DeviceApprovalsChangeMobilePolicyDetails json.RawMessage `json:"device_approvals_change_mobile_policy_details,omitempty"`
		// DeviceApprovalsChangeOverageActionDetails : Changed the action taken
		// when a team member is already over the limits (e.g when they join the
		// team, an admin lowers limits, etc.).
		DeviceApprovalsChangeOverageActionDetails json.RawMessage `json:"device_approvals_change_overage_action_details,omitempty"`
		// DeviceApprovalsChangeUnlinkActionDetails : Changed the action taken
		// with respect to approval limits when a team member unlinks an
		// approved device.
		DeviceApprovalsChangeUnlinkActionDetails json.RawMessage `json:"device_approvals_change_unlink_action_details,omitempty"`
		// EmmAddExceptionDetails : Added an exception for one or more team
		// members to optionally use the regular Dropbox app when EMM is
		// enabled.
		EmmAddExceptionDetails json.RawMessage `json:"emm_add_exception_details,omitempty"`
		// EmmChangePolicyDetails : Enabled or disabled enterprise mobility
		// management for team members.
		EmmChangePolicyDetails json.RawMessage `json:"emm_change_policy_details,omitempty"`
		// EmmRemoveExceptionDetails : Removed an exception for one or more team
		// members to optionally use the regular Dropbox app when EMM is
		// enabled.
		EmmRemoveExceptionDetails json.RawMessage `json:"emm_remove_exception_details,omitempty"`
		// ExtendedVersionHistoryChangePolicyDetails : Accepted or opted out of
		// extended version history.
		ExtendedVersionHistoryChangePolicyDetails json.RawMessage `json:"extended_version_history_change_policy_details,omitempty"`
		// FileCommentsChangePolicyDetails : Enabled or disabled commenting on
		// team files.
		FileCommentsChangePolicyDetails json.RawMessage `json:"file_comments_change_policy_details,omitempty"`
		// FileRequestsChangePolicyDetails : Enabled or disabled file requests.
		FileRequestsChangePolicyDetails json.RawMessage `json:"file_requests_change_policy_details,omitempty"`
		// FileRequestsEmailsEnabledDetails : Enabled file request emails for
		// everyone.
		FileRequestsEmailsEnabledDetails json.RawMessage `json:"file_requests_emails_enabled_details,omitempty"`
		// FileRequestsEmailsRestrictedToTeamOnlyDetails : Allowed file request
		// emails for the team.
		FileRequestsEmailsRestrictedToTeamOnlyDetails json.RawMessage `json:"file_requests_emails_restricted_to_team_only_details,omitempty"`
		// GoogleSsoChangePolicyDetails : Enabled or disabled Google single
		// sign-on for the team.
		GoogleSsoChangePolicyDetails json.RawMessage `json:"google_sso_change_policy_details,omitempty"`
		// GroupUserManagementChangePolicyDetails : Changed who can create
		// groups.
		GroupUserManagementChangePolicyDetails json.RawMessage `json:"group_user_management_change_policy_details,omitempty"`
		// MemberRequestsChangePolicyDetails : Changed whether users can find
		// the team when not invited.
		MemberRequestsChangePolicyDetails json.RawMessage `json:"member_requests_change_policy_details,omitempty"`
		// MemberSpaceLimitsAddExceptionDetails : Added an exception for one or
		// more team members to bypass space limits imposed by policy.
		MemberSpaceLimitsAddExceptionDetails json.RawMessage `json:"member_space_limits_add_exception_details,omitempty"`
		// MemberSpaceLimitsChangePolicyDetails : Changed the storage limits
		// applied to team members by policy.
		MemberSpaceLimitsChangePolicyDetails json.RawMessage `json:"member_space_limits_change_policy_details,omitempty"`
		// MemberSpaceLimitsRemoveExceptionDetails : Removed an exception for
		// one or more team members to bypass space limits imposed by policy.
		MemberSpaceLimitsRemoveExceptionDetails json.RawMessage `json:"member_space_limits_remove_exception_details,omitempty"`
		// MemberSuggestionsChangePolicyDetails : Enabled or disabled the option
		// for team members to suggest new members to add to the team.
		MemberSuggestionsChangePolicyDetails json.RawMessage `json:"member_suggestions_change_policy_details,omitempty"`
		// MicrosoftOfficeAddinChangePolicyDetails : Enabled or disabled the
		// Microsoft Office add-in, which lets team members save files to
		// Dropbox directly from Microsoft Office.
		MicrosoftOfficeAddinChangePolicyDetails json.RawMessage `json:"microsoft_office_addin_change_policy_details,omitempty"`
		// NetworkControlChangePolicyDetails : Enabled or disabled network
		// control.
		NetworkControlChangePolicyDetails json.RawMessage `json:"network_control_change_policy_details,omitempty"`
		// PaperChangeDeploymentPolicyDetails : Changed whether Dropbox Paper,
		// when enabled, is deployed to all teams or to specific members of the
		// team.
		PaperChangeDeploymentPolicyDetails json.RawMessage `json:"paper_change_deployment_policy_details,omitempty"`
		// PaperChangeMemberPolicyDetails : Changed whether team members can
		// share Paper documents externally (i.e. outside the team), and if so,
		// whether they should be accessible only by team members or anyone by
		// default.
		PaperChangeMemberPolicyDetails json.RawMessage `json:"paper_change_member_policy_details,omitempty"`
		// PaperChangePolicyDetails : Enabled or disabled Dropbox Paper for the
		// team.
		PaperChangePolicyDetails json.RawMessage `json:"paper_change_policy_details,omitempty"`
		// PermanentDeleteChangePolicyDetails : Enabled or disabled the ability
		// of team members to permanently delete content.
		PermanentDeleteChangePolicyDetails json.RawMessage `json:"permanent_delete_change_policy_details,omitempty"`
		// SharingChangeFolderJoinPolicyDetails : Changed whether team members
		// can join shared folders owned externally (i.e. outside the team).
		SharingChangeFolderJoinPolicyDetails json.RawMessage `json:"sharing_change_folder_join_policy_details,omitempty"`
		// SharingChangeLinkPolicyDetails : Changed whether team members can
		// share links externally (i.e. outside the team), and if so, whether
		// links should be accessible only by team members or anyone by default.
		SharingChangeLinkPolicyDetails json.RawMessage `json:"sharing_change_link_policy_details,omitempty"`
		// SharingChangeMemberPolicyDetails : Changed whether team members can
		// share files and folders externally (i.e. outside the team).
		SharingChangeMemberPolicyDetails json.RawMessage `json:"sharing_change_member_policy_details,omitempty"`
		// SmartSyncChangePolicyDetails : Changed the default Smart Sync policy
		// for team members.
		SmartSyncChangePolicyDetails json.RawMessage `json:"smart_sync_change_policy_details,omitempty"`
		// SmartSyncNotOptOutDetails : Opted team into Smart Sync.
		SmartSyncNotOptOutDetails json.RawMessage `json:"smart_sync_not_opt_out_details,omitempty"`
		// SmartSyncOptOutDetails : Opted team out of Smart Sync.
		SmartSyncOptOutDetails json.RawMessage `json:"smart_sync_opt_out_details,omitempty"`
		// SsoChangePolicyDetails : Change the single sign-on policy for the
		// team.
		SsoChangePolicyDetails json.RawMessage `json:"sso_change_policy_details,omitempty"`
		// TfaChangePolicyDetails : Change two-step verification policy for the
		// team.
		TfaChangePolicyDetails json.RawMessage `json:"tfa_change_policy_details,omitempty"`
		// TwoAccountChangePolicyDetails : Enabled or disabled the option for
		// team members to link a personal Dropbox account in addition to their
		// work account to the same computer.
		TwoAccountChangePolicyDetails json.RawMessage `json:"two_account_change_policy_details,omitempty"`
		// WebSessionsChangeFixedLengthPolicyDetails : Changed how long team
		// members can stay signed in to Dropbox on the web.
		WebSessionsChangeFixedLengthPolicyDetails json.RawMessage `json:"web_sessions_change_fixed_length_policy_details,omitempty"`
		// WebSessionsChangeIdleLengthPolicyDetails : Changed how long team
		// members can be idle while signed in to Dropbox on the web.
		WebSessionsChangeIdleLengthPolicyDetails json.RawMessage `json:"web_sessions_change_idle_length_policy_details,omitempty"`
		// TeamProfileAddLogoDetails : Added a team logo to be displayed on
		// shared link headers.
		TeamProfileAddLogoDetails json.RawMessage `json:"team_profile_add_logo_details,omitempty"`
		// TeamProfileChangeLogoDetails : Changed the team logo to be displayed
		// on shared link headers.
		TeamProfileChangeLogoDetails json.RawMessage `json:"team_profile_change_logo_details,omitempty"`
		// TeamProfileChangeNameDetails : Changed the team name.
		TeamProfileChangeNameDetails json.RawMessage `json:"team_profile_change_name_details,omitempty"`
		// TeamProfileRemoveLogoDetails : Removed the team logo to be displayed
		// on shared link headers.
		TeamProfileRemoveLogoDetails json.RawMessage `json:"team_profile_remove_logo_details,omitempty"`
		// TfaAddBackupPhoneDetails : Added a backup phone for two-step
		// verification.
		TfaAddBackupPhoneDetails json.RawMessage `json:"tfa_add_backup_phone_details,omitempty"`
		// TfaAddSecurityKeyDetails : Added a security key for two-step
		// verification.
		TfaAddSecurityKeyDetails json.RawMessage `json:"tfa_add_security_key_details,omitempty"`
		// TfaChangeBackupPhoneDetails : Changed the backup phone for two-step
		// verification.
		TfaChangeBackupPhoneDetails json.RawMessage `json:"tfa_change_backup_phone_details,omitempty"`
		// TfaChangeStatusDetails : Enabled, disabled or changed the
		// configuration for two-step verification.
		TfaChangeStatusDetails json.RawMessage `json:"tfa_change_status_details,omitempty"`
		// TfaRemoveBackupPhoneDetails : Removed the backup phone for two-step
		// verification.
		TfaRemoveBackupPhoneDetails json.RawMessage `json:"tfa_remove_backup_phone_details,omitempty"`
		// TfaRemoveSecurityKeyDetails : Removed a security key for two-step
		// verification.
		TfaRemoveSecurityKeyDetails json.RawMessage `json:"tfa_remove_security_key_details,omitempty"`
		// TfaResetDetails : Reset two-step verification for team member.
		TfaResetDetails json.RawMessage `json:"tfa_reset_details,omitempty"`
		// MissingDetails : Hints that this event was returned with missing
		// details due to an internal error.
		MissingDetails json.RawMessage `json:"missing_details,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "member_change_membership_type_details":
		err = json.Unmarshal(body, &u.MemberChangeMembershipTypeDetails)

		if err != nil {
			return err
		}
	case "member_permanently_delete_account_contents_details":
		err = json.Unmarshal(body, &u.MemberPermanentlyDeleteAccountContentsDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_change_status_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeStatusDetails)

		if err != nil {
			return err
		}
	case "member_transfer_account_contents_details":
		err = json.Unmarshal(body, &u.MemberTransferAccountContentsDetails)

		if err != nil {
			return err
		}
	case "paper_enabled_users_group_addition_details":
		err = json.Unmarshal(body, &u.PaperEnabledUsersGroupAdditionDetails)

		if err != nil {
			return err
		}
	case "paper_enabled_users_group_removal_details":
		err = json.Unmarshal(body, &u.PaperEnabledUsersGroupRemovalDetails)

		if err != nil {
			return err
		}
	case "paper_external_view_allow_details":
		err = json.Unmarshal(body, &u.PaperExternalViewAllowDetails)

		if err != nil {
			return err
		}
	case "paper_external_view_default_team_details":
		err = json.Unmarshal(body, &u.PaperExternalViewDefaultTeamDetails)

		if err != nil {
			return err
		}
	case "paper_external_view_forbid_details":
		err = json.Unmarshal(body, &u.PaperExternalViewForbidDetails)

		if err != nil {
			return err
		}
	case "sf_external_invite_warn_details":
		err = json.Unmarshal(body, &u.SfExternalInviteWarnDetails)

		if err != nil {
			return err
		}
	case "team_merge_from_details":
		err = json.Unmarshal(body, &u.TeamMergeFromDetails)

		if err != nil {
			return err
		}
	case "team_merge_to_details":
		err = json.Unmarshal(body, &u.TeamMergeToDetails)

		if err != nil {
			return err
		}
	case "app_link_team_details":
		err = json.Unmarshal(body, &u.AppLinkTeamDetails)

		if err != nil {
			return err
		}
	case "app_link_user_details":
		err = json.Unmarshal(body, &u.AppLinkUserDetails)

		if err != nil {
			return err
		}
	case "app_unlink_team_details":
		err = json.Unmarshal(body, &u.AppUnlinkTeamDetails)

		if err != nil {
			return err
		}
	case "app_unlink_user_details":
		err = json.Unmarshal(body, &u.AppUnlinkUserDetails)

		if err != nil {
			return err
		}
	case "device_change_ip_desktop_details":
		err = json.Unmarshal(body, &u.DeviceChangeIpDesktopDetails)

		if err != nil {
			return err
		}
	case "device_change_ip_mobile_details":
		err = json.Unmarshal(body, &u.DeviceChangeIpMobileDetails)

		if err != nil {
			return err
		}
	case "device_change_ip_web_details":
		err = json.Unmarshal(body, &u.DeviceChangeIpWebDetails)

		if err != nil {
			return err
		}
	case "device_delete_on_unlink_fail_details":
		err = json.Unmarshal(body, &u.DeviceDeleteOnUnlinkFailDetails)

		if err != nil {
			return err
		}
	case "device_delete_on_unlink_success_details":
		err = json.Unmarshal(body, &u.DeviceDeleteOnUnlinkSuccessDetails)

		if err != nil {
			return err
		}
	case "device_link_fail_details":
		err = json.Unmarshal(body, &u.DeviceLinkFailDetails)

		if err != nil {
			return err
		}
	case "device_link_success_details":
		err = json.Unmarshal(body, &u.DeviceLinkSuccessDetails)

		if err != nil {
			return err
		}
	case "device_management_disabled_details":
		err = json.Unmarshal(body, &u.DeviceManagementDisabledDetails)

		if err != nil {
			return err
		}
	case "device_management_enabled_details":
		err = json.Unmarshal(body, &u.DeviceManagementEnabledDetails)

		if err != nil {
			return err
		}
	case "device_unlink_details":
		err = json.Unmarshal(body, &u.DeviceUnlinkDetails)

		if err != nil {
			return err
		}
	case "emm_refresh_auth_token_details":
		err = json.Unmarshal(body, &u.EmmRefreshAuthTokenDetails)

		if err != nil {
			return err
		}
	case "account_capture_change_availability_details":
		err = json.Unmarshal(body, &u.AccountCaptureChangeAvailabilityDetails)

		if err != nil {
			return err
		}
	case "account_capture_migrate_account_details":
		err = json.Unmarshal(body, &u.AccountCaptureMigrateAccountDetails)

		if err != nil {
			return err
		}
	case "account_capture_relinquish_account_details":
		err = json.Unmarshal(body, &u.AccountCaptureRelinquishAccountDetails)

		if err != nil {
			return err
		}
	case "disabled_domain_invites_details":
		err = json.Unmarshal(body, &u.DisabledDomainInvitesDetails)

		if err != nil {
			return err
		}
	case "domain_invites_approve_request_to_join_team_details":
		err = json.Unmarshal(body, &u.DomainInvitesApproveRequestToJoinTeamDetails)

		if err != nil {
			return err
		}
	case "domain_invites_decline_request_to_join_team_details":
		err = json.Unmarshal(body, &u.DomainInvitesDeclineRequestToJoinTeamDetails)

		if err != nil {
			return err
		}
	case "domain_invites_email_existing_users_details":
		err = json.Unmarshal(body, &u.DomainInvitesEmailExistingUsersDetails)

		if err != nil {
			return err
		}
	case "domain_invites_request_to_join_team_details":
		err = json.Unmarshal(body, &u.DomainInvitesRequestToJoinTeamDetails)

		if err != nil {
			return err
		}
	case "domain_invites_set_invite_new_user_pref_to_no_details":
		err = json.Unmarshal(body, &u.DomainInvitesSetInviteNewUserPrefToNoDetails)

		if err != nil {
			return err
		}
	case "domain_invites_set_invite_new_user_pref_to_yes_details":
		err = json.Unmarshal(body, &u.DomainInvitesSetInviteNewUserPrefToYesDetails)

		if err != nil {
			return err
		}
	case "domain_verification_add_domain_fail_details":
		err = json.Unmarshal(body, &u.DomainVerificationAddDomainFailDetails)

		if err != nil {
			return err
		}
	case "domain_verification_add_domain_success_details":
		err = json.Unmarshal(body, &u.DomainVerificationAddDomainSuccessDetails)

		if err != nil {
			return err
		}
	case "domain_verification_remove_domain_details":
		err = json.Unmarshal(body, &u.DomainVerificationRemoveDomainDetails)

		if err != nil {
			return err
		}
	case "enabled_domain_invites_details":
		err = json.Unmarshal(body, &u.EnabledDomainInvitesDetails)

		if err != nil {
			return err
		}
	case "create_folder_details":
		err = json.Unmarshal(body, &u.CreateFolderDetails)

		if err != nil {
			return err
		}
	case "file_add_details":
		err = json.Unmarshal(body, &u.FileAddDetails)

		if err != nil {
			return err
		}
	case "file_copy_details":
		err = json.Unmarshal(body, &u.FileCopyDetails)

		if err != nil {
			return err
		}
	case "file_delete_details":
		err = json.Unmarshal(body, &u.FileDeleteDetails)

		if err != nil {
			return err
		}
	case "file_download_details":
		err = json.Unmarshal(body, &u.FileDownloadDetails)

		if err != nil {
			return err
		}
	case "file_edit_details":
		err = json.Unmarshal(body, &u.FileEditDetails)

		if err != nil {
			return err
		}
	case "file_get_copy_reference_details":
		err = json.Unmarshal(body, &u.FileGetCopyReferenceDetails)

		if err != nil {
			return err
		}
	case "file_move_details":
		err = json.Unmarshal(body, &u.FileMoveDetails)

		if err != nil {
			return err
		}
	case "file_permanently_delete_details":
		err = json.Unmarshal(body, &u.FilePermanentlyDeleteDetails)

		if err != nil {
			return err
		}
	case "file_preview_details":
		err = json.Unmarshal(body, &u.FilePreviewDetails)

		if err != nil {
			return err
		}
	case "file_rename_details":
		err = json.Unmarshal(body, &u.FileRenameDetails)

		if err != nil {
			return err
		}
	case "file_restore_details":
		err = json.Unmarshal(body, &u.FileRestoreDetails)

		if err != nil {
			return err
		}
	case "file_revert_details":
		err = json.Unmarshal(body, &u.FileRevertDetails)

		if err != nil {
			return err
		}
	case "file_rollback_changes_details":
		err = json.Unmarshal(body, &u.FileRollbackChangesDetails)

		if err != nil {
			return err
		}
	case "file_save_copy_reference_details":
		err = json.Unmarshal(body, &u.FileSaveCopyReferenceDetails)

		if err != nil {
			return err
		}
	case "file_request_add_deadline_details":
		err = json.Unmarshal(body, &u.FileRequestAddDeadlineDetails)

		if err != nil {
			return err
		}
	case "file_request_change_folder_details":
		err = json.Unmarshal(body, &u.FileRequestChangeFolderDetails)

		if err != nil {
			return err
		}
	case "file_request_change_title_details":
		err = json.Unmarshal(body, &u.FileRequestChangeTitleDetails)

		if err != nil {
			return err
		}
	case "file_request_close_details":
		err = json.Unmarshal(body, &u.FileRequestCloseDetails)

		if err != nil {
			return err
		}
	case "file_request_create_details":
		err = json.Unmarshal(body, &u.FileRequestCreateDetails)

		if err != nil {
			return err
		}
	case "file_request_receive_file_details":
		err = json.Unmarshal(body, &u.FileRequestReceiveFileDetails)

		if err != nil {
			return err
		}
	case "file_request_remove_deadline_details":
		err = json.Unmarshal(body, &u.FileRequestRemoveDeadlineDetails)

		if err != nil {
			return err
		}
	case "file_request_send_details":
		err = json.Unmarshal(body, &u.FileRequestSendDetails)

		if err != nil {
			return err
		}
	case "group_add_external_id_details":
		err = json.Unmarshal(body, &u.GroupAddExternalIdDetails)

		if err != nil {
			return err
		}
	case "group_add_member_details":
		err = json.Unmarshal(body, &u.GroupAddMemberDetails)

		if err != nil {
			return err
		}
	case "group_change_external_id_details":
		err = json.Unmarshal(body, &u.GroupChangeExternalIdDetails)

		if err != nil {
			return err
		}
	case "group_change_management_type_details":
		err = json.Unmarshal(body, &u.GroupChangeManagementTypeDetails)

		if err != nil {
			return err
		}
	case "group_change_member_role_details":
		err = json.Unmarshal(body, &u.GroupChangeMemberRoleDetails)

		if err != nil {
			return err
		}
	case "group_create_details":
		err = json.Unmarshal(body, &u.GroupCreateDetails)

		if err != nil {
			return err
		}
	case "group_delete_details":
		err = json.Unmarshal(body, &u.GroupDeleteDetails)

		if err != nil {
			return err
		}
	case "group_description_updated_details":
		err = json.Unmarshal(body, &u.GroupDescriptionUpdatedDetails)

		if err != nil {
			return err
		}
	case "group_join_policy_updated_details":
		err = json.Unmarshal(body, &u.GroupJoinPolicyUpdatedDetails)

		if err != nil {
			return err
		}
	case "group_moved_details":
		err = json.Unmarshal(body, &u.GroupMovedDetails)

		if err != nil {
			return err
		}
	case "group_remove_external_id_details":
		err = json.Unmarshal(body, &u.GroupRemoveExternalIdDetails)

		if err != nil {
			return err
		}
	case "group_remove_member_details":
		err = json.Unmarshal(body, &u.GroupRemoveMemberDetails)

		if err != nil {
			return err
		}
	case "group_rename_details":
		err = json.Unmarshal(body, &u.GroupRenameDetails)

		if err != nil {
			return err
		}
	case "emm_login_success_details":
		err = json.Unmarshal(body, &u.EmmLoginSuccessDetails)

		if err != nil {
			return err
		}
	case "logout_details":
		err = json.Unmarshal(body, &u.LogoutDetails)

		if err != nil {
			return err
		}
	case "password_login_fail_details":
		err = json.Unmarshal(body, &u.PasswordLoginFailDetails)

		if err != nil {
			return err
		}
	case "password_login_success_details":
		err = json.Unmarshal(body, &u.PasswordLoginSuccessDetails)

		if err != nil {
			return err
		}
	case "reseller_support_session_end_details":
		err = json.Unmarshal(body, &u.ResellerSupportSessionEndDetails)

		if err != nil {
			return err
		}
	case "reseller_support_session_start_details":
		err = json.Unmarshal(body, &u.ResellerSupportSessionStartDetails)

		if err != nil {
			return err
		}
	case "sign_in_as_session_end_details":
		err = json.Unmarshal(body, &u.SignInAsSessionEndDetails)

		if err != nil {
			return err
		}
	case "sign_in_as_session_start_details":
		err = json.Unmarshal(body, &u.SignInAsSessionStartDetails)

		if err != nil {
			return err
		}
	case "sso_login_fail_details":
		err = json.Unmarshal(body, &u.SsoLoginFailDetails)

		if err != nil {
			return err
		}
	case "member_add_name_details":
		err = json.Unmarshal(body, &u.MemberAddNameDetails)

		if err != nil {
			return err
		}
	case "member_change_admin_role_details":
		err = json.Unmarshal(body, &u.MemberChangeAdminRoleDetails)

		if err != nil {
			return err
		}
	case "member_change_email_details":
		err = json.Unmarshal(body, &u.MemberChangeEmailDetails)

		if err != nil {
			return err
		}
	case "member_change_name_details":
		err = json.Unmarshal(body, &u.MemberChangeNameDetails)

		if err != nil {
			return err
		}
	case "member_change_status_details":
		err = json.Unmarshal(body, &u.MemberChangeStatusDetails)

		if err != nil {
			return err
		}
	case "member_suggest_details":
		err = json.Unmarshal(body, &u.MemberSuggestDetails)

		if err != nil {
			return err
		}
	case "paper_content_add_member_details":
		err = json.Unmarshal(body, &u.PaperContentAddMemberDetails)

		if err != nil {
			return err
		}
	case "paper_content_add_to_folder_details":
		err = json.Unmarshal(body, &u.PaperContentAddToFolderDetails)

		if err != nil {
			return err
		}
	case "paper_content_archive_details":
		err = json.Unmarshal(body, &u.PaperContentArchiveDetails)

		if err != nil {
			return err
		}
	case "paper_content_change_subscription_details":
		err = json.Unmarshal(body, &u.PaperContentChangeSubscriptionDetails)

		if err != nil {
			return err
		}
	case "paper_content_create_details":
		err = json.Unmarshal(body, &u.PaperContentCreateDetails)

		if err != nil {
			return err
		}
	case "paper_content_permanently_delete_details":
		err = json.Unmarshal(body, &u.PaperContentPermanentlyDeleteDetails)

		if err != nil {
			return err
		}
	case "paper_content_remove_from_folder_details":
		err = json.Unmarshal(body, &u.PaperContentRemoveFromFolderDetails)

		if err != nil {
			return err
		}
	case "paper_content_remove_member_details":
		err = json.Unmarshal(body, &u.PaperContentRemoveMemberDetails)

		if err != nil {
			return err
		}
	case "paper_content_rename_details":
		err = json.Unmarshal(body, &u.PaperContentRenameDetails)

		if err != nil {
			return err
		}
	case "paper_content_restore_details":
		err = json.Unmarshal(body, &u.PaperContentRestoreDetails)

		if err != nil {
			return err
		}
	case "paper_doc_add_comment_details":
		err = json.Unmarshal(body, &u.PaperDocAddCommentDetails)

		if err != nil {
			return err
		}
	case "paper_doc_change_member_role_details":
		err = json.Unmarshal(body, &u.PaperDocChangeMemberRoleDetails)

		if err != nil {
			return err
		}
	case "paper_doc_change_sharing_policy_details":
		err = json.Unmarshal(body, &u.PaperDocChangeSharingPolicyDetails)

		if err != nil {
			return err
		}
	case "paper_doc_deleted_details":
		err = json.Unmarshal(body, &u.PaperDocDeletedDetails)

		if err != nil {
			return err
		}
	case "paper_doc_delete_comment_details":
		err = json.Unmarshal(body, &u.PaperDocDeleteCommentDetails)

		if err != nil {
			return err
		}
	case "paper_doc_download_details":
		err = json.Unmarshal(body, &u.PaperDocDownloadDetails)

		if err != nil {
			return err
		}
	case "paper_doc_edit_details":
		err = json.Unmarshal(body, &u.PaperDocEditDetails)

		if err != nil {
			return err
		}
	case "paper_doc_edit_comment_details":
		err = json.Unmarshal(body, &u.PaperDocEditCommentDetails)

		if err != nil {
			return err
		}
	case "paper_doc_followed_details":
		err = json.Unmarshal(body, &u.PaperDocFollowedDetails)

		if err != nil {
			return err
		}
	case "paper_doc_mention_details":
		err = json.Unmarshal(body, &u.PaperDocMentionDetails)

		if err != nil {
			return err
		}
	case "paper_doc_request_access_details":
		err = json.Unmarshal(body, &u.PaperDocRequestAccessDetails)

		if err != nil {
			return err
		}
	case "paper_doc_resolve_comment_details":
		err = json.Unmarshal(body, &u.PaperDocResolveCommentDetails)

		if err != nil {
			return err
		}
	case "paper_doc_revert_details":
		err = json.Unmarshal(body, &u.PaperDocRevertDetails)

		if err != nil {
			return err
		}
	case "paper_doc_slack_share_details":
		err = json.Unmarshal(body, &u.PaperDocSlackShareDetails)

		if err != nil {
			return err
		}
	case "paper_doc_team_invite_details":
		err = json.Unmarshal(body, &u.PaperDocTeamInviteDetails)

		if err != nil {
			return err
		}
	case "paper_doc_unresolve_comment_details":
		err = json.Unmarshal(body, &u.PaperDocUnresolveCommentDetails)

		if err != nil {
			return err
		}
	case "paper_doc_view_details":
		err = json.Unmarshal(body, &u.PaperDocViewDetails)

		if err != nil {
			return err
		}
	case "paper_folder_deleted_details":
		err = json.Unmarshal(body, &u.PaperFolderDeletedDetails)

		if err != nil {
			return err
		}
	case "paper_folder_followed_details":
		err = json.Unmarshal(body, &u.PaperFolderFollowedDetails)

		if err != nil {
			return err
		}
	case "paper_folder_team_invite_details":
		err = json.Unmarshal(body, &u.PaperFolderTeamInviteDetails)

		if err != nil {
			return err
		}
	case "password_change_details":
		err = json.Unmarshal(body, &u.PasswordChangeDetails)

		if err != nil {
			return err
		}
	case "password_reset_details":
		err = json.Unmarshal(body, &u.PasswordResetDetails)

		if err != nil {
			return err
		}
	case "password_reset_all_details":
		err = json.Unmarshal(body, &u.PasswordResetAllDetails)

		if err != nil {
			return err
		}
	case "emm_create_exceptions_report_details":
		err = json.Unmarshal(body, &u.EmmCreateExceptionsReportDetails)

		if err != nil {
			return err
		}
	case "emm_create_usage_report_details":
		err = json.Unmarshal(body, &u.EmmCreateUsageReportDetails)

		if err != nil {
			return err
		}
	case "smart_sync_create_admin_privilege_report_details":
		err = json.Unmarshal(body, &u.SmartSyncCreateAdminPrivilegeReportDetails)

		if err != nil {
			return err
		}
	case "team_activity_create_report_details":
		err = json.Unmarshal(body, &u.TeamActivityCreateReportDetails)

		if err != nil {
			return err
		}
	case "collection_share_details":
		err = json.Unmarshal(body, &u.CollectionShareDetails)

		if err != nil {
			return err
		}
	case "file_add_comment_details":
		err = json.Unmarshal(body, &u.FileAddCommentDetails)

		if err != nil {
			return err
		}
	case "file_like_comment_details":
		err = json.Unmarshal(body, &u.FileLikeCommentDetails)

		if err != nil {
			return err
		}
	case "file_unlike_comment_details":
		err = json.Unmarshal(body, &u.FileUnlikeCommentDetails)

		if err != nil {
			return err
		}
	case "note_acl_invite_only_details":
		err = json.Unmarshal(body, &u.NoteAclInviteOnlyDetails)

		if err != nil {
			return err
		}
	case "note_acl_link_details":
		err = json.Unmarshal(body, &u.NoteAclLinkDetails)

		if err != nil {
			return err
		}
	case "note_acl_team_link_details":
		err = json.Unmarshal(body, &u.NoteAclTeamLinkDetails)

		if err != nil {
			return err
		}
	case "note_shared_details":
		err = json.Unmarshal(body, &u.NoteSharedDetails)

		if err != nil {
			return err
		}
	case "note_share_receive_details":
		err = json.Unmarshal(body, &u.NoteShareReceiveDetails)

		if err != nil {
			return err
		}
	case "open_note_shared_details":
		err = json.Unmarshal(body, &u.OpenNoteSharedDetails)

		if err != nil {
			return err
		}
	case "sf_add_group_details":
		err = json.Unmarshal(body, &u.SfAddGroupDetails)

		if err != nil {
			return err
		}
	case "sf_allow_non_members_to_view_shared_links_details":
		err = json.Unmarshal(body, &u.SfAllowNonMembersToViewSharedLinksDetails)

		if err != nil {
			return err
		}
	case "sf_invite_group_details":
		err = json.Unmarshal(body, &u.SfInviteGroupDetails)

		if err != nil {
			return err
		}
	case "sf_nest_details":
		err = json.Unmarshal(body, &u.SfNestDetails)

		if err != nil {
			return err
		}
	case "sf_team_decline_details":
		err = json.Unmarshal(body, &u.SfTeamDeclineDetails)

		if err != nil {
			return err
		}
	case "sf_team_grant_access_details":
		err = json.Unmarshal(body, &u.SfTeamGrantAccessDetails)

		if err != nil {
			return err
		}
	case "sf_team_invite_details":
		err = json.Unmarshal(body, &u.SfTeamInviteDetails)

		if err != nil {
			return err
		}
	case "sf_team_invite_change_role_details":
		err = json.Unmarshal(body, &u.SfTeamInviteChangeRoleDetails)

		if err != nil {
			return err
		}
	case "sf_team_join_details":
		err = json.Unmarshal(body, &u.SfTeamJoinDetails)

		if err != nil {
			return err
		}
	case "sf_team_join_from_oob_link_details":
		err = json.Unmarshal(body, &u.SfTeamJoinFromOobLinkDetails)

		if err != nil {
			return err
		}
	case "sf_team_uninvite_details":
		err = json.Unmarshal(body, &u.SfTeamUninviteDetails)

		if err != nil {
			return err
		}
	case "shared_content_add_invitees_details":
		err = json.Unmarshal(body, &u.SharedContentAddInviteesDetails)

		if err != nil {
			return err
		}
	case "shared_content_add_link_expiry_details":
		err = json.Unmarshal(body, &u.SharedContentAddLinkExpiryDetails)

		if err != nil {
			return err
		}
	case "shared_content_add_link_password_details":
		err = json.Unmarshal(body, &u.SharedContentAddLinkPasswordDetails)

		if err != nil {
			return err
		}
	case "shared_content_add_member_details":
		err = json.Unmarshal(body, &u.SharedContentAddMemberDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_downloads_policy_details":
		err = json.Unmarshal(body, &u.SharedContentChangeDownloadsPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_invitee_role_details":
		err = json.Unmarshal(body, &u.SharedContentChangeInviteeRoleDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_link_audience_details":
		err = json.Unmarshal(body, &u.SharedContentChangeLinkAudienceDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_link_expiry_details":
		err = json.Unmarshal(body, &u.SharedContentChangeLinkExpiryDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_link_password_details":
		err = json.Unmarshal(body, &u.SharedContentChangeLinkPasswordDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_member_role_details":
		err = json.Unmarshal(body, &u.SharedContentChangeMemberRoleDetails)

		if err != nil {
			return err
		}
	case "shared_content_change_viewer_info_policy_details":
		err = json.Unmarshal(body, &u.SharedContentChangeViewerInfoPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_content_claim_invitation_details":
		err = json.Unmarshal(body, &u.SharedContentClaimInvitationDetails)

		if err != nil {
			return err
		}
	case "shared_content_copy_details":
		err = json.Unmarshal(body, &u.SharedContentCopyDetails)

		if err != nil {
			return err
		}
	case "shared_content_download_details":
		err = json.Unmarshal(body, &u.SharedContentDownloadDetails)

		if err != nil {
			return err
		}
	case "shared_content_relinquish_membership_details":
		err = json.Unmarshal(body, &u.SharedContentRelinquishMembershipDetails)

		if err != nil {
			return err
		}
	case "shared_content_remove_invitee_details":
		err = json.Unmarshal(body, &u.SharedContentRemoveInviteeDetails)

		if err != nil {
			return err
		}
	case "shared_content_remove_link_expiry_details":
		err = json.Unmarshal(body, &u.SharedContentRemoveLinkExpiryDetails)

		if err != nil {
			return err
		}
	case "shared_content_remove_link_password_details":
		err = json.Unmarshal(body, &u.SharedContentRemoveLinkPasswordDetails)

		if err != nil {
			return err
		}
	case "shared_content_remove_member_details":
		err = json.Unmarshal(body, &u.SharedContentRemoveMemberDetails)

		if err != nil {
			return err
		}
	case "shared_content_request_access_details":
		err = json.Unmarshal(body, &u.SharedContentRequestAccessDetails)

		if err != nil {
			return err
		}
	case "shared_content_unshare_details":
		err = json.Unmarshal(body, &u.SharedContentUnshareDetails)

		if err != nil {
			return err
		}
	case "shared_content_view_details":
		err = json.Unmarshal(body, &u.SharedContentViewDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_confidentiality_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeConfidentialityDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_link_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeLinkPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_member_management_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeMemberManagementPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_member_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeMemberPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_create_details":
		err = json.Unmarshal(body, &u.SharedFolderCreateDetails)

		if err != nil {
			return err
		}
	case "shared_folder_mount_details":
		err = json.Unmarshal(body, &u.SharedFolderMountDetails)

		if err != nil {
			return err
		}
	case "shared_folder_transfer_ownership_details":
		err = json.Unmarshal(body, &u.SharedFolderTransferOwnershipDetails)

		if err != nil {
			return err
		}
	case "shared_folder_unmount_details":
		err = json.Unmarshal(body, &u.SharedFolderUnmountDetails)

		if err != nil {
			return err
		}
	case "shared_note_opened_details":
		err = json.Unmarshal(body, &u.SharedNoteOpenedDetails)

		if err != nil {
			return err
		}
	case "shmodel_app_create_details":
		err = json.Unmarshal(body, &u.ShmodelAppCreateDetails)

		if err != nil {
			return err
		}
	case "shmodel_create_details":
		err = json.Unmarshal(body, &u.ShmodelCreateDetails)

		if err != nil {
			return err
		}
	case "shmodel_disable_details":
		err = json.Unmarshal(body, &u.ShmodelDisableDetails)

		if err != nil {
			return err
		}
	case "shmodel_fb_share_details":
		err = json.Unmarshal(body, &u.ShmodelFbShareDetails)

		if err != nil {
			return err
		}
	case "shmodel_group_share_details":
		err = json.Unmarshal(body, &u.ShmodelGroupShareDetails)

		if err != nil {
			return err
		}
	case "shmodel_remove_expiration_details":
		err = json.Unmarshal(body, &u.ShmodelRemoveExpirationDetails)

		if err != nil {
			return err
		}
	case "shmodel_set_expiration_details":
		err = json.Unmarshal(body, &u.ShmodelSetExpirationDetails)

		if err != nil {
			return err
		}
	case "shmodel_team_copy_details":
		err = json.Unmarshal(body, &u.ShmodelTeamCopyDetails)

		if err != nil {
			return err
		}
	case "shmodel_team_download_details":
		err = json.Unmarshal(body, &u.ShmodelTeamDownloadDetails)

		if err != nil {
			return err
		}
	case "shmodel_team_share_details":
		err = json.Unmarshal(body, &u.ShmodelTeamShareDetails)

		if err != nil {
			return err
		}
	case "shmodel_team_view_details":
		err = json.Unmarshal(body, &u.ShmodelTeamViewDetails)

		if err != nil {
			return err
		}
	case "shmodel_visibility_password_details":
		err = json.Unmarshal(body, &u.ShmodelVisibilityPasswordDetails)

		if err != nil {
			return err
		}
	case "shmodel_visibility_public_details":
		err = json.Unmarshal(body, &u.ShmodelVisibilityPublicDetails)

		if err != nil {
			return err
		}
	case "shmodel_visibility_team_only_details":
		err = json.Unmarshal(body, &u.ShmodelVisibilityTeamOnlyDetails)

		if err != nil {
			return err
		}
	case "remove_logout_url_details":
		err = json.Unmarshal(body, &u.RemoveLogoutUrlDetails)

		if err != nil {
			return err
		}
	case "remove_sso_url_details":
		err = json.Unmarshal(body, &u.RemoveSsoUrlDetails)

		if err != nil {
			return err
		}
	case "sso_change_cert_details":
		err = json.Unmarshal(body, &u.SsoChangeCertDetails)

		if err != nil {
			return err
		}
	case "sso_change_login_url_details":
		err = json.Unmarshal(body, &u.SsoChangeLoginUrlDetails)

		if err != nil {
			return err
		}
	case "sso_change_logout_url_details":
		err = json.Unmarshal(body, &u.SsoChangeLogoutUrlDetails)

		if err != nil {
			return err
		}
	case "sso_change_saml_identity_mode_details":
		err = json.Unmarshal(body, &u.SsoChangeSamlIdentityModeDetails)

		if err != nil {
			return err
		}
	case "team_folder_change_status_details":
		err = json.Unmarshal(body, &u.TeamFolderChangeStatusDetails)

		if err != nil {
			return err
		}
	case "team_folder_create_details":
		err = json.Unmarshal(body, &u.TeamFolderCreateDetails)

		if err != nil {
			return err
		}
	case "team_folder_downgrade_details":
		err = json.Unmarshal(body, &u.TeamFolderDowngradeDetails)

		if err != nil {
			return err
		}
	case "team_folder_permanently_delete_details":
		err = json.Unmarshal(body, &u.TeamFolderPermanentlyDeleteDetails)

		if err != nil {
			return err
		}
	case "team_folder_rename_details":
		err = json.Unmarshal(body, &u.TeamFolderRenameDetails)

		if err != nil {
			return err
		}
	case "account_capture_change_policy_details":
		err = json.Unmarshal(body, &u.AccountCaptureChangePolicyDetails)

		if err != nil {
			return err
		}
	case "allow_download_disabled_details":
		err = json.Unmarshal(body, &u.AllowDownloadDisabledDetails)

		if err != nil {
			return err
		}
	case "allow_download_enabled_details":
		err = json.Unmarshal(body, &u.AllowDownloadEnabledDetails)

		if err != nil {
			return err
		}
	case "data_placement_restriction_change_policy_details":
		err = json.Unmarshal(body, &u.DataPlacementRestrictionChangePolicyDetails)

		if err != nil {
			return err
		}
	case "data_placement_restriction_satisfy_policy_details":
		err = json.Unmarshal(body, &u.DataPlacementRestrictionSatisfyPolicyDetails)

		if err != nil {
			return err
		}
	case "device_approvals_change_desktop_policy_details":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeDesktopPolicyDetails)

		if err != nil {
			return err
		}
	case "device_approvals_change_mobile_policy_details":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeMobilePolicyDetails)

		if err != nil {
			return err
		}
	case "device_approvals_change_overage_action_details":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeOverageActionDetails)

		if err != nil {
			return err
		}
	case "device_approvals_change_unlink_action_details":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeUnlinkActionDetails)

		if err != nil {
			return err
		}
	case "emm_add_exception_details":
		err = json.Unmarshal(body, &u.EmmAddExceptionDetails)

		if err != nil {
			return err
		}
	case "emm_change_policy_details":
		err = json.Unmarshal(body, &u.EmmChangePolicyDetails)

		if err != nil {
			return err
		}
	case "emm_remove_exception_details":
		err = json.Unmarshal(body, &u.EmmRemoveExceptionDetails)

		if err != nil {
			return err
		}
	case "extended_version_history_change_policy_details":
		err = json.Unmarshal(body, &u.ExtendedVersionHistoryChangePolicyDetails)

		if err != nil {
			return err
		}
	case "file_comments_change_policy_details":
		err = json.Unmarshal(body, &u.FileCommentsChangePolicyDetails)

		if err != nil {
			return err
		}
	case "file_requests_change_policy_details":
		err = json.Unmarshal(body, &u.FileRequestsChangePolicyDetails)

		if err != nil {
			return err
		}
	case "file_requests_emails_enabled_details":
		err = json.Unmarshal(body, &u.FileRequestsEmailsEnabledDetails)

		if err != nil {
			return err
		}
	case "file_requests_emails_restricted_to_team_only_details":
		err = json.Unmarshal(body, &u.FileRequestsEmailsRestrictedToTeamOnlyDetails)

		if err != nil {
			return err
		}
	case "google_sso_change_policy_details":
		err = json.Unmarshal(body, &u.GoogleSsoChangePolicyDetails)

		if err != nil {
			return err
		}
	case "group_user_management_change_policy_details":
		err = json.Unmarshal(body, &u.GroupUserManagementChangePolicyDetails)

		if err != nil {
			return err
		}
	case "member_requests_change_policy_details":
		err = json.Unmarshal(body, &u.MemberRequestsChangePolicyDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_add_exception_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsAddExceptionDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_change_policy_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangePolicyDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_remove_exception_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsRemoveExceptionDetails)

		if err != nil {
			return err
		}
	case "member_suggestions_change_policy_details":
		err = json.Unmarshal(body, &u.MemberSuggestionsChangePolicyDetails)

		if err != nil {
			return err
		}
	case "microsoft_office_addin_change_policy_details":
		err = json.Unmarshal(body, &u.MicrosoftOfficeAddinChangePolicyDetails)

		if err != nil {
			return err
		}
	case "network_control_change_policy_details":
		err = json.Unmarshal(body, &u.NetworkControlChangePolicyDetails)

		if err != nil {
			return err
		}
	case "paper_change_deployment_policy_details":
		err = json.Unmarshal(body, &u.PaperChangeDeploymentPolicyDetails)

		if err != nil {
			return err
		}
	case "paper_change_member_policy_details":
		err = json.Unmarshal(body, &u.PaperChangeMemberPolicyDetails)

		if err != nil {
			return err
		}
	case "paper_change_policy_details":
		err = json.Unmarshal(body, &u.PaperChangePolicyDetails)

		if err != nil {
			return err
		}
	case "permanent_delete_change_policy_details":
		err = json.Unmarshal(body, &u.PermanentDeleteChangePolicyDetails)

		if err != nil {
			return err
		}
	case "sharing_change_folder_join_policy_details":
		err = json.Unmarshal(body, &u.SharingChangeFolderJoinPolicyDetails)

		if err != nil {
			return err
		}
	case "sharing_change_link_policy_details":
		err = json.Unmarshal(body, &u.SharingChangeLinkPolicyDetails)

		if err != nil {
			return err
		}
	case "sharing_change_member_policy_details":
		err = json.Unmarshal(body, &u.SharingChangeMemberPolicyDetails)

		if err != nil {
			return err
		}
	case "smart_sync_change_policy_details":
		err = json.Unmarshal(body, &u.SmartSyncChangePolicyDetails)

		if err != nil {
			return err
		}
	case "smart_sync_not_opt_out_details":
		err = json.Unmarshal(body, &u.SmartSyncNotOptOutDetails)

		if err != nil {
			return err
		}
	case "smart_sync_opt_out_details":
		err = json.Unmarshal(body, &u.SmartSyncOptOutDetails)

		if err != nil {
			return err
		}
	case "sso_change_policy_details":
		err = json.Unmarshal(body, &u.SsoChangePolicyDetails)

		if err != nil {
			return err
		}
	case "tfa_change_policy_details":
		err = json.Unmarshal(body, &u.TfaChangePolicyDetails)

		if err != nil {
			return err
		}
	case "two_account_change_policy_details":
		err = json.Unmarshal(body, &u.TwoAccountChangePolicyDetails)

		if err != nil {
			return err
		}
	case "web_sessions_change_fixed_length_policy_details":
		err = json.Unmarshal(body, &u.WebSessionsChangeFixedLengthPolicyDetails)

		if err != nil {
			return err
		}
	case "web_sessions_change_idle_length_policy_details":
		err = json.Unmarshal(body, &u.WebSessionsChangeIdleLengthPolicyDetails)

		if err != nil {
			return err
		}
	case "team_profile_add_logo_details":
		err = json.Unmarshal(body, &u.TeamProfileAddLogoDetails)

		if err != nil {
			return err
		}
	case "team_profile_change_logo_details":
		err = json.Unmarshal(body, &u.TeamProfileChangeLogoDetails)

		if err != nil {
			return err
		}
	case "team_profile_change_name_details":
		err = json.Unmarshal(body, &u.TeamProfileChangeNameDetails)

		if err != nil {
			return err
		}
	case "team_profile_remove_logo_details":
		err = json.Unmarshal(body, &u.TeamProfileRemoveLogoDetails)

		if err != nil {
			return err
		}
	case "tfa_add_backup_phone_details":
		err = json.Unmarshal(body, &u.TfaAddBackupPhoneDetails)

		if err != nil {
			return err
		}
	case "tfa_add_security_key_details":
		err = json.Unmarshal(body, &u.TfaAddSecurityKeyDetails)

		if err != nil {
			return err
		}
	case "tfa_change_backup_phone_details":
		err = json.Unmarshal(body, &u.TfaChangeBackupPhoneDetails)

		if err != nil {
			return err
		}
	case "tfa_change_status_details":
		err = json.Unmarshal(body, &u.TfaChangeStatusDetails)

		if err != nil {
			return err
		}
	case "tfa_remove_backup_phone_details":
		err = json.Unmarshal(body, &u.TfaRemoveBackupPhoneDetails)

		if err != nil {
			return err
		}
	case "tfa_remove_security_key_details":
		err = json.Unmarshal(body, &u.TfaRemoveSecurityKeyDetails)

		if err != nil {
			return err
		}
	case "tfa_reset_details":
		err = json.Unmarshal(body, &u.TfaResetDetails)

		if err != nil {
			return err
		}
	case "missing_details":
		err = json.Unmarshal(body, &u.MissingDetails)

		if err != nil {
			return err
		}
	}
	return nil
}

// EventType : The type of the event.
type EventType struct {
	dropbox.Tagged
}

// Valid tag values for EventType
const (
	EventTypeMemberChangeMembershipType               = "member_change_membership_type"
	EventTypeMemberPermanentlyDeleteAccountContents   = "member_permanently_delete_account_contents"
	EventTypeMemberSpaceLimitsChangeStatus            = "member_space_limits_change_status"
	EventTypeMemberTransferAccountContents            = "member_transfer_account_contents"
	EventTypePaperEnabledUsersGroupAddition           = "paper_enabled_users_group_addition"
	EventTypePaperEnabledUsersGroupRemoval            = "paper_enabled_users_group_removal"
	EventTypePaperExternalViewAllow                   = "paper_external_view_allow"
	EventTypePaperExternalViewDefaultTeam             = "paper_external_view_default_team"
	EventTypePaperExternalViewForbid                  = "paper_external_view_forbid"
	EventTypeSfExternalInviteWarn                     = "sf_external_invite_warn"
	EventTypeTeamMergeFrom                            = "team_merge_from"
	EventTypeTeamMergeTo                              = "team_merge_to"
	EventTypeAppLinkTeam                              = "app_link_team"
	EventTypeAppLinkUser                              = "app_link_user"
	EventTypeAppUnlinkTeam                            = "app_unlink_team"
	EventTypeAppUnlinkUser                            = "app_unlink_user"
	EventTypeDeviceChangeIpDesktop                    = "device_change_ip_desktop"
	EventTypeDeviceChangeIpMobile                     = "device_change_ip_mobile"
	EventTypeDeviceChangeIpWeb                        = "device_change_ip_web"
	EventTypeDeviceDeleteOnUnlinkFail                 = "device_delete_on_unlink_fail"
	EventTypeDeviceDeleteOnUnlinkSuccess              = "device_delete_on_unlink_success"
	EventTypeDeviceLinkFail                           = "device_link_fail"
	EventTypeDeviceLinkSuccess                        = "device_link_success"
	EventTypeDeviceManagementDisabled                 = "device_management_disabled"
	EventTypeDeviceManagementEnabled                  = "device_management_enabled"
	EventTypeDeviceUnlink                             = "device_unlink"
	EventTypeEmmRefreshAuthToken                      = "emm_refresh_auth_token"
	EventTypeAccountCaptureChangeAvailability         = "account_capture_change_availability"
	EventTypeAccountCaptureMigrateAccount             = "account_capture_migrate_account"
	EventTypeAccountCaptureRelinquishAccount          = "account_capture_relinquish_account"
	EventTypeDisabledDomainInvites                    = "disabled_domain_invites"
	EventTypeDomainInvitesApproveRequestToJoinTeam    = "domain_invites_approve_request_to_join_team"
	EventTypeDomainInvitesDeclineRequestToJoinTeam    = "domain_invites_decline_request_to_join_team"
	EventTypeDomainInvitesEmailExistingUsers          = "domain_invites_email_existing_users"
	EventTypeDomainInvitesRequestToJoinTeam           = "domain_invites_request_to_join_team"
	EventTypeDomainInvitesSetInviteNewUserPrefToNo    = "domain_invites_set_invite_new_user_pref_to_no"
	EventTypeDomainInvitesSetInviteNewUserPrefToYes   = "domain_invites_set_invite_new_user_pref_to_yes"
	EventTypeDomainVerificationAddDomainFail          = "domain_verification_add_domain_fail"
	EventTypeDomainVerificationAddDomainSuccess       = "domain_verification_add_domain_success"
	EventTypeDomainVerificationRemoveDomain           = "domain_verification_remove_domain"
	EventTypeEnabledDomainInvites                     = "enabled_domain_invites"
	EventTypeCreateFolder                             = "create_folder"
	EventTypeFileAdd                                  = "file_add"
	EventTypeFileCopy                                 = "file_copy"
	EventTypeFileDelete                               = "file_delete"
	EventTypeFileDownload                             = "file_download"
	EventTypeFileEdit                                 = "file_edit"
	EventTypeFileGetCopyReference                     = "file_get_copy_reference"
	EventTypeFileMove                                 = "file_move"
	EventTypeFilePermanentlyDelete                    = "file_permanently_delete"
	EventTypeFilePreview                              = "file_preview"
	EventTypeFileRename                               = "file_rename"
	EventTypeFileRestore                              = "file_restore"
	EventTypeFileRevert                               = "file_revert"
	EventTypeFileRollbackChanges                      = "file_rollback_changes"
	EventTypeFileSaveCopyReference                    = "file_save_copy_reference"
	EventTypeFileRequestAddDeadline                   = "file_request_add_deadline"
	EventTypeFileRequestChangeFolder                  = "file_request_change_folder"
	EventTypeFileRequestChangeTitle                   = "file_request_change_title"
	EventTypeFileRequestClose                         = "file_request_close"
	EventTypeFileRequestCreate                        = "file_request_create"
	EventTypeFileRequestReceiveFile                   = "file_request_receive_file"
	EventTypeFileRequestRemoveDeadline                = "file_request_remove_deadline"
	EventTypeFileRequestSend                          = "file_request_send"
	EventTypeGroupAddExternalId                       = "group_add_external_id"
	EventTypeGroupAddMember                           = "group_add_member"
	EventTypeGroupChangeExternalId                    = "group_change_external_id"
	EventTypeGroupChangeManagementType                = "group_change_management_type"
	EventTypeGroupChangeMemberRole                    = "group_change_member_role"
	EventTypeGroupCreate                              = "group_create"
	EventTypeGroupDelete                              = "group_delete"
	EventTypeGroupDescriptionUpdated                  = "group_description_updated"
	EventTypeGroupJoinPolicyUpdated                   = "group_join_policy_updated"
	EventTypeGroupMoved                               = "group_moved"
	EventTypeGroupRemoveExternalId                    = "group_remove_external_id"
	EventTypeGroupRemoveMember                        = "group_remove_member"
	EventTypeGroupRename                              = "group_rename"
	EventTypeEmmLoginSuccess                          = "emm_login_success"
	EventTypeLogout                                   = "logout"
	EventTypePasswordLoginFail                        = "password_login_fail"
	EventTypePasswordLoginSuccess                     = "password_login_success"
	EventTypeResellerSupportSessionEnd                = "reseller_support_session_end"
	EventTypeResellerSupportSessionStart              = "reseller_support_session_start"
	EventTypeSignInAsSessionEnd                       = "sign_in_as_session_end"
	EventTypeSignInAsSessionStart                     = "sign_in_as_session_start"
	EventTypeSsoLoginFail                             = "sso_login_fail"
	EventTypeMemberAddName                            = "member_add_name"
	EventTypeMemberChangeAdminRole                    = "member_change_admin_role"
	EventTypeMemberChangeEmail                        = "member_change_email"
	EventTypeMemberChangeName                         = "member_change_name"
	EventTypeMemberChangeStatus                       = "member_change_status"
	EventTypeMemberSuggest                            = "member_suggest"
	EventTypePaperContentAddMember                    = "paper_content_add_member"
	EventTypePaperContentAddToFolder                  = "paper_content_add_to_folder"
	EventTypePaperContentArchive                      = "paper_content_archive"
	EventTypePaperContentChangeSubscription           = "paper_content_change_subscription"
	EventTypePaperContentCreate                       = "paper_content_create"
	EventTypePaperContentPermanentlyDelete            = "paper_content_permanently_delete"
	EventTypePaperContentRemoveFromFolder             = "paper_content_remove_from_folder"
	EventTypePaperContentRemoveMember                 = "paper_content_remove_member"
	EventTypePaperContentRename                       = "paper_content_rename"
	EventTypePaperContentRestore                      = "paper_content_restore"
	EventTypePaperDocAddComment                       = "paper_doc_add_comment"
	EventTypePaperDocChangeMemberRole                 = "paper_doc_change_member_role"
	EventTypePaperDocChangeSharingPolicy              = "paper_doc_change_sharing_policy"
	EventTypePaperDocDeleted                          = "paper_doc_deleted"
	EventTypePaperDocDeleteComment                    = "paper_doc_delete_comment"
	EventTypePaperDocDownload                         = "paper_doc_download"
	EventTypePaperDocEdit                             = "paper_doc_edit"
	EventTypePaperDocEditComment                      = "paper_doc_edit_comment"
	EventTypePaperDocFollowed                         = "paper_doc_followed"
	EventTypePaperDocMention                          = "paper_doc_mention"
	EventTypePaperDocRequestAccess                    = "paper_doc_request_access"
	EventTypePaperDocResolveComment                   = "paper_doc_resolve_comment"
	EventTypePaperDocRevert                           = "paper_doc_revert"
	EventTypePaperDocSlackShare                       = "paper_doc_slack_share"
	EventTypePaperDocTeamInvite                       = "paper_doc_team_invite"
	EventTypePaperDocUnresolveComment                 = "paper_doc_unresolve_comment"
	EventTypePaperDocView                             = "paper_doc_view"
	EventTypePaperFolderDeleted                       = "paper_folder_deleted"
	EventTypePaperFolderFollowed                      = "paper_folder_followed"
	EventTypePaperFolderTeamInvite                    = "paper_folder_team_invite"
	EventTypePasswordChange                           = "password_change"
	EventTypePasswordReset                            = "password_reset"
	EventTypePasswordResetAll                         = "password_reset_all"
	EventTypeEmmCreateExceptionsReport                = "emm_create_exceptions_report"
	EventTypeEmmCreateUsageReport                     = "emm_create_usage_report"
	EventTypeSmartSyncCreateAdminPrivilegeReport      = "smart_sync_create_admin_privilege_report"
	EventTypeTeamActivityCreateReport                 = "team_activity_create_report"
	EventTypeCollectionShare                          = "collection_share"
	EventTypeFileAddComment                           = "file_add_comment"
	EventTypeFileLikeComment                          = "file_like_comment"
	EventTypeFileUnlikeComment                        = "file_unlike_comment"
	EventTypeNoteAclInviteOnly                        = "note_acl_invite_only"
	EventTypeNoteAclLink                              = "note_acl_link"
	EventTypeNoteAclTeamLink                          = "note_acl_team_link"
	EventTypeNoteShared                               = "note_shared"
	EventTypeNoteShareReceive                         = "note_share_receive"
	EventTypeOpenNoteShared                           = "open_note_shared"
	EventTypeSfAddGroup                               = "sf_add_group"
	EventTypeSfAllowNonMembersToViewSharedLinks       = "sf_allow_non_members_to_view_shared_links"
	EventTypeSfInviteGroup                            = "sf_invite_group"
	EventTypeSfNest                                   = "sf_nest"
	EventTypeSfTeamDecline                            = "sf_team_decline"
	EventTypeSfTeamGrantAccess                        = "sf_team_grant_access"
	EventTypeSfTeamInvite                             = "sf_team_invite"
	EventTypeSfTeamInviteChangeRole                   = "sf_team_invite_change_role"
	EventTypeSfTeamJoin                               = "sf_team_join"
	EventTypeSfTeamJoinFromOobLink                    = "sf_team_join_from_oob_link"
	EventTypeSfTeamUninvite                           = "sf_team_uninvite"
	EventTypeSharedContentAddInvitees                 = "shared_content_add_invitees"
	EventTypeSharedContentAddLinkExpiry               = "shared_content_add_link_expiry"
	EventTypeSharedContentAddLinkPassword             = "shared_content_add_link_password"
	EventTypeSharedContentAddMember                   = "shared_content_add_member"
	EventTypeSharedContentChangeDownloadsPolicy       = "shared_content_change_downloads_policy"
	EventTypeSharedContentChangeInviteeRole           = "shared_content_change_invitee_role"
	EventTypeSharedContentChangeLinkAudience          = "shared_content_change_link_audience"
	EventTypeSharedContentChangeLinkExpiry            = "shared_content_change_link_expiry"
	EventTypeSharedContentChangeLinkPassword          = "shared_content_change_link_password"
	EventTypeSharedContentChangeMemberRole            = "shared_content_change_member_role"
	EventTypeSharedContentChangeViewerInfoPolicy      = "shared_content_change_viewer_info_policy"
	EventTypeSharedContentClaimInvitation             = "shared_content_claim_invitation"
	EventTypeSharedContentCopy                        = "shared_content_copy"
	EventTypeSharedContentDownload                    = "shared_content_download"
	EventTypeSharedContentRelinquishMembership        = "shared_content_relinquish_membership"
	EventTypeSharedContentRemoveInvitee               = "shared_content_remove_invitee"
	EventTypeSharedContentRemoveLinkExpiry            = "shared_content_remove_link_expiry"
	EventTypeSharedContentRemoveLinkPassword          = "shared_content_remove_link_password"
	EventTypeSharedContentRemoveMember                = "shared_content_remove_member"
	EventTypeSharedContentRequestAccess               = "shared_content_request_access"
	EventTypeSharedContentUnshare                     = "shared_content_unshare"
	EventTypeSharedContentView                        = "shared_content_view"
	EventTypeSharedFolderChangeConfidentiality        = "shared_folder_change_confidentiality"
	EventTypeSharedFolderChangeLinkPolicy             = "shared_folder_change_link_policy"
	EventTypeSharedFolderChangeMemberManagementPolicy = "shared_folder_change_member_management_policy"
	EventTypeSharedFolderChangeMemberPolicy           = "shared_folder_change_member_policy"
	EventTypeSharedFolderCreate                       = "shared_folder_create"
	EventTypeSharedFolderMount                        = "shared_folder_mount"
	EventTypeSharedFolderTransferOwnership            = "shared_folder_transfer_ownership"
	EventTypeSharedFolderUnmount                      = "shared_folder_unmount"
	EventTypeSharedNoteOpened                         = "shared_note_opened"
	EventTypeShmodelAppCreate                         = "shmodel_app_create"
	EventTypeShmodelCreate                            = "shmodel_create"
	EventTypeShmodelDisable                           = "shmodel_disable"
	EventTypeShmodelFbShare                           = "shmodel_fb_share"
	EventTypeShmodelGroupShare                        = "shmodel_group_share"
	EventTypeShmodelRemoveExpiration                  = "shmodel_remove_expiration"
	EventTypeShmodelSetExpiration                     = "shmodel_set_expiration"
	EventTypeShmodelTeamCopy                          = "shmodel_team_copy"
	EventTypeShmodelTeamDownload                      = "shmodel_team_download"
	EventTypeShmodelTeamShare                         = "shmodel_team_share"
	EventTypeShmodelTeamView                          = "shmodel_team_view"
	EventTypeShmodelVisibilityPassword                = "shmodel_visibility_password"
	EventTypeShmodelVisibilityPublic                  = "shmodel_visibility_public"
	EventTypeShmodelVisibilityTeamOnly                = "shmodel_visibility_team_only"
	EventTypeRemoveLogoutUrl                          = "remove_logout_url"
	EventTypeRemoveSsoUrl                             = "remove_sso_url"
	EventTypeSsoChangeCert                            = "sso_change_cert"
	EventTypeSsoChangeLoginUrl                        = "sso_change_login_url"
	EventTypeSsoChangeLogoutUrl                       = "sso_change_logout_url"
	EventTypeSsoChangeSamlIdentityMode                = "sso_change_saml_identity_mode"
	EventTypeTeamFolderChangeStatus                   = "team_folder_change_status"
	EventTypeTeamFolderCreate                         = "team_folder_create"
	EventTypeTeamFolderDowngrade                      = "team_folder_downgrade"
	EventTypeTeamFolderPermanentlyDelete              = "team_folder_permanently_delete"
	EventTypeTeamFolderRename                         = "team_folder_rename"
	EventTypeAccountCaptureChangePolicy               = "account_capture_change_policy"
	EventTypeAllowDownloadDisabled                    = "allow_download_disabled"
	EventTypeAllowDownloadEnabled                     = "allow_download_enabled"
	EventTypeDataPlacementRestrictionChangePolicy     = "data_placement_restriction_change_policy"
	EventTypeDataPlacementRestrictionSatisfyPolicy    = "data_placement_restriction_satisfy_policy"
	EventTypeDeviceApprovalsChangeDesktopPolicy       = "device_approvals_change_desktop_policy"
	EventTypeDeviceApprovalsChangeMobilePolicy        = "device_approvals_change_mobile_policy"
	EventTypeDeviceApprovalsChangeOverageAction       = "device_approvals_change_overage_action"
	EventTypeDeviceApprovalsChangeUnlinkAction        = "device_approvals_change_unlink_action"
	EventTypeEmmAddException                          = "emm_add_exception"
	EventTypeEmmChangePolicy                          = "emm_change_policy"
	EventTypeEmmRemoveException                       = "emm_remove_exception"
	EventTypeExtendedVersionHistoryChangePolicy       = "extended_version_history_change_policy"
	EventTypeFileCommentsChangePolicy                 = "file_comments_change_policy"
	EventTypeFileRequestsChangePolicy                 = "file_requests_change_policy"
	EventTypeFileRequestsEmailsEnabled                = "file_requests_emails_enabled"
	EventTypeFileRequestsEmailsRestrictedToTeamOnly   = "file_requests_emails_restricted_to_team_only"
	EventTypeGoogleSsoChangePolicy                    = "google_sso_change_policy"
	EventTypeGroupUserManagementChangePolicy          = "group_user_management_change_policy"
	EventTypeMemberRequestsChangePolicy               = "member_requests_change_policy"
	EventTypeMemberSpaceLimitsAddException            = "member_space_limits_add_exception"
	EventTypeMemberSpaceLimitsChangePolicy            = "member_space_limits_change_policy"
	EventTypeMemberSpaceLimitsRemoveException         = "member_space_limits_remove_exception"
	EventTypeMemberSuggestionsChangePolicy            = "member_suggestions_change_policy"
	EventTypeMicrosoftOfficeAddinChangePolicy         = "microsoft_office_addin_change_policy"
	EventTypeNetworkControlChangePolicy               = "network_control_change_policy"
	EventTypePaperChangeDeploymentPolicy              = "paper_change_deployment_policy"
	EventTypePaperChangeMemberPolicy                  = "paper_change_member_policy"
	EventTypePaperChangePolicy                        = "paper_change_policy"
	EventTypePermanentDeleteChangePolicy              = "permanent_delete_change_policy"
	EventTypeSharingChangeFolderJoinPolicy            = "sharing_change_folder_join_policy"
	EventTypeSharingChangeLinkPolicy                  = "sharing_change_link_policy"
	EventTypeSharingChangeMemberPolicy                = "sharing_change_member_policy"
	EventTypeSmartSyncChangePolicy                    = "smart_sync_change_policy"
	EventTypeSmartSyncNotOptOut                       = "smart_sync_not_opt_out"
	EventTypeSmartSyncOptOut                          = "smart_sync_opt_out"
	EventTypeSsoChangePolicy                          = "sso_change_policy"
	EventTypeTfaChangePolicy                          = "tfa_change_policy"
	EventTypeTwoAccountChangePolicy                   = "two_account_change_policy"
	EventTypeWebSessionsChangeFixedLengthPolicy       = "web_sessions_change_fixed_length_policy"
	EventTypeWebSessionsChangeIdleLengthPolicy        = "web_sessions_change_idle_length_policy"
	EventTypeTeamProfileAddLogo                       = "team_profile_add_logo"
	EventTypeTeamProfileChangeLogo                    = "team_profile_change_logo"
	EventTypeTeamProfileChangeName                    = "team_profile_change_name"
	EventTypeTeamProfileRemoveLogo                    = "team_profile_remove_logo"
	EventTypeTfaAddBackupPhone                        = "tfa_add_backup_phone"
	EventTypeTfaAddSecurityKey                        = "tfa_add_security_key"
	EventTypeTfaChangeBackupPhone                     = "tfa_change_backup_phone"
	EventTypeTfaChangeStatus                          = "tfa_change_status"
	EventTypeTfaRemoveBackupPhone                     = "tfa_remove_backup_phone"
	EventTypeTfaRemoveSecurityKey                     = "tfa_remove_security_key"
	EventTypeTfaReset                                 = "tfa_reset"
	EventTypeOther                                    = "other"
)

// ExtendedVersionHistoryChangePolicyDetails : Accepted or opted out of extended
// version history.
type ExtendedVersionHistoryChangePolicyDetails struct {
	// NewValue : New extended version history policy.
	NewValue *ExtendedVersionHistoryPolicy `json:"new_value"`
	// PreviousValue : Previous extended version history policy. Might be
	// missing due to historical data gap.
	PreviousValue *ExtendedVersionHistoryPolicy `json:"previous_value,omitempty"`
}

// NewExtendedVersionHistoryChangePolicyDetails returns a new ExtendedVersionHistoryChangePolicyDetails instance
func NewExtendedVersionHistoryChangePolicyDetails(NewValue *ExtendedVersionHistoryPolicy) *ExtendedVersionHistoryChangePolicyDetails {
	s := new(ExtendedVersionHistoryChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// ExtendedVersionHistoryPolicy : has no documentation (yet)
type ExtendedVersionHistoryPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ExtendedVersionHistoryPolicy
const (
	ExtendedVersionHistoryPolicyLimited   = "limited"
	ExtendedVersionHistoryPolicyUnlimited = "unlimited"
	ExtendedVersionHistoryPolicyOther     = "other"
)

// FailureDetailsLogInfo : Provides details about a failure
type FailureDetailsLogInfo struct {
	// UserFriendlyMessage : A user friendly explanation of the error. Might be
	// missing due to historical data gap.
	UserFriendlyMessage string `json:"user_friendly_message,omitempty"`
	// TechnicalErrorMessage : A technical explanation of the error. This is
	// relevant for some errors.
	TechnicalErrorMessage string `json:"technical_error_message,omitempty"`
}

// NewFailureDetailsLogInfo returns a new FailureDetailsLogInfo instance
func NewFailureDetailsLogInfo() *FailureDetailsLogInfo {
	s := new(FailureDetailsLogInfo)
	return s
}

// FileAddCommentDetails : Added a file comment.
type FileAddCommentDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileAddCommentDetails returns a new FileAddCommentDetails instance
func NewFileAddCommentDetails(TargetIndex int64) *FileAddCommentDetails {
	s := new(FileAddCommentDetails)
	s.TargetIndex = TargetIndex
	return s
}

// FileAddDetails : Added files and/or folders.
type FileAddDetails struct {
}

// NewFileAddDetails returns a new FileAddDetails instance
func NewFileAddDetails() *FileAddDetails {
	s := new(FileAddDetails)
	return s
}

// FileCommentsChangePolicyDetails : Enabled or disabled commenting on team
// files.
type FileCommentsChangePolicyDetails struct {
	// NewValue : New commenting on team files policy.
	NewValue *FileCommentsPolicy `json:"new_value"`
	// PreviousValue : Previous commenting on team files policy. Might be
	// missing due to historical data gap.
	PreviousValue *FileCommentsPolicy `json:"previous_value,omitempty"`
}

// NewFileCommentsChangePolicyDetails returns a new FileCommentsChangePolicyDetails instance
func NewFileCommentsChangePolicyDetails(NewValue *FileCommentsPolicy) *FileCommentsChangePolicyDetails {
	s := new(FileCommentsChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// FileCommentsPolicy : File comments policy
type FileCommentsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for FileCommentsPolicy
const (
	FileCommentsPolicyDisabled = "disabled"
	FileCommentsPolicyEnabled  = "enabled"
	FileCommentsPolicyOther    = "other"
)

// FileCopyDetails : Copied files and/or folders.
type FileCopyDetails struct {
	// RelocateActionDetails : Relocate action details.
	RelocateActionDetails []*RelocateAssetReferencesLogInfo `json:"relocate_action_details"`
}

// NewFileCopyDetails returns a new FileCopyDetails instance
func NewFileCopyDetails(RelocateActionDetails []*RelocateAssetReferencesLogInfo) *FileCopyDetails {
	s := new(FileCopyDetails)
	s.RelocateActionDetails = RelocateActionDetails
	return s
}

// FileDeleteDetails : Deleted files and/or folders.
type FileDeleteDetails struct {
}

// NewFileDeleteDetails returns a new FileDeleteDetails instance
func NewFileDeleteDetails() *FileDeleteDetails {
	s := new(FileDeleteDetails)
	return s
}

// FileDownloadDetails : Downloaded files and/or folders.
type FileDownloadDetails struct {
}

// NewFileDownloadDetails returns a new FileDownloadDetails instance
func NewFileDownloadDetails() *FileDownloadDetails {
	s := new(FileDownloadDetails)
	return s
}

// FileEditDetails : Edited files.
type FileEditDetails struct {
}

// NewFileEditDetails returns a new FileEditDetails instance
func NewFileEditDetails() *FileEditDetails {
	s := new(FileEditDetails)
	return s
}

// FileGetCopyReferenceDetails : Create a copy reference to a file or folder.
type FileGetCopyReferenceDetails struct {
}

// NewFileGetCopyReferenceDetails returns a new FileGetCopyReferenceDetails instance
func NewFileGetCopyReferenceDetails() *FileGetCopyReferenceDetails {
	s := new(FileGetCopyReferenceDetails)
	return s
}

// FileLikeCommentDetails : Liked a file comment.
type FileLikeCommentDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileLikeCommentDetails returns a new FileLikeCommentDetails instance
func NewFileLikeCommentDetails(TargetIndex int64) *FileLikeCommentDetails {
	s := new(FileLikeCommentDetails)
	s.TargetIndex = TargetIndex
	return s
}

// FileOrFolderLogInfo : Generic information relevant both for files and folders
type FileOrFolderLogInfo struct {
	// Path : Path relative to event context.
	Path *PathLogInfo `json:"path"`
	// DisplayName : Display name. Might be missing due to historical data gap.
	DisplayName string `json:"display_name,omitempty"`
	// FileId : Unique ID. Might be missing due to historical data gap.
	FileId string `json:"file_id,omitempty"`
}

// NewFileOrFolderLogInfo returns a new FileOrFolderLogInfo instance
func NewFileOrFolderLogInfo(Path *PathLogInfo) *FileOrFolderLogInfo {
	s := new(FileOrFolderLogInfo)
	s.Path = Path
	return s
}

// FileLogInfo : File's logged information.
type FileLogInfo struct {
	FileOrFolderLogInfo
}

// NewFileLogInfo returns a new FileLogInfo instance
func NewFileLogInfo(Path *PathLogInfo) *FileLogInfo {
	s := new(FileLogInfo)
	s.Path = Path
	return s
}

// FileMoveDetails : Moved files and/or folders.
type FileMoveDetails struct {
	// RelocateActionDetails : Relocate action details.
	RelocateActionDetails []*RelocateAssetReferencesLogInfo `json:"relocate_action_details"`
}

// NewFileMoveDetails returns a new FileMoveDetails instance
func NewFileMoveDetails(RelocateActionDetails []*RelocateAssetReferencesLogInfo) *FileMoveDetails {
	s := new(FileMoveDetails)
	s.RelocateActionDetails = RelocateActionDetails
	return s
}

// FilePermanentlyDeleteDetails : Permanently deleted files and/or folders.
type FilePermanentlyDeleteDetails struct {
}

// NewFilePermanentlyDeleteDetails returns a new FilePermanentlyDeleteDetails instance
func NewFilePermanentlyDeleteDetails() *FilePermanentlyDeleteDetails {
	s := new(FilePermanentlyDeleteDetails)
	return s
}

// FilePreviewDetails : Previewed files and/or folders.
type FilePreviewDetails struct {
}

// NewFilePreviewDetails returns a new FilePreviewDetails instance
func NewFilePreviewDetails() *FilePreviewDetails {
	s := new(FilePreviewDetails)
	return s
}

// FileRenameDetails : Renamed files and/or folders.
type FileRenameDetails struct {
	// RelocateActionDetails : Relocate action details.
	RelocateActionDetails []*RelocateAssetReferencesLogInfo `json:"relocate_action_details"`
}

// NewFileRenameDetails returns a new FileRenameDetails instance
func NewFileRenameDetails(RelocateActionDetails []*RelocateAssetReferencesLogInfo) *FileRenameDetails {
	s := new(FileRenameDetails)
	s.RelocateActionDetails = RelocateActionDetails
	return s
}

// FileRequestAddDeadlineDetails : Added a deadline to a file request.
type FileRequestAddDeadlineDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestAddDeadlineDetails returns a new FileRequestAddDeadlineDetails instance
func NewFileRequestAddDeadlineDetails(RequestTitle string) *FileRequestAddDeadlineDetails {
	s := new(FileRequestAddDeadlineDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestChangeFolderDetails : Changed the file request folder.
type FileRequestChangeFolderDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestChangeFolderDetails returns a new FileRequestChangeFolderDetails instance
func NewFileRequestChangeFolderDetails(RequestTitle string) *FileRequestChangeFolderDetails {
	s := new(FileRequestChangeFolderDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestChangeTitleDetails : Change the file request title.
type FileRequestChangeTitleDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestChangeTitleDetails returns a new FileRequestChangeTitleDetails instance
func NewFileRequestChangeTitleDetails(RequestTitle string) *FileRequestChangeTitleDetails {
	s := new(FileRequestChangeTitleDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestCloseDetails : Closed a file request.
type FileRequestCloseDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestCloseDetails returns a new FileRequestCloseDetails instance
func NewFileRequestCloseDetails(RequestTitle string) *FileRequestCloseDetails {
	s := new(FileRequestCloseDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestCreateDetails : Created a file request.
type FileRequestCreateDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestCreateDetails returns a new FileRequestCreateDetails instance
func NewFileRequestCreateDetails(RequestTitle string) *FileRequestCreateDetails {
	s := new(FileRequestCreateDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestReceiveFileDetails : Received files for a file request.
type FileRequestReceiveFileDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
	// SubmittedFileNames : Submitted file names.
	SubmittedFileNames []string `json:"submitted_file_names"`
}

// NewFileRequestReceiveFileDetails returns a new FileRequestReceiveFileDetails instance
func NewFileRequestReceiveFileDetails(RequestTitle string, SubmittedFileNames []string) *FileRequestReceiveFileDetails {
	s := new(FileRequestReceiveFileDetails)
	s.RequestTitle = RequestTitle
	s.SubmittedFileNames = SubmittedFileNames
	return s
}

// FileRequestRemoveDeadlineDetails : Removed the file request deadline.
type FileRequestRemoveDeadlineDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestRemoveDeadlineDetails returns a new FileRequestRemoveDeadlineDetails instance
func NewFileRequestRemoveDeadlineDetails(RequestTitle string) *FileRequestRemoveDeadlineDetails {
	s := new(FileRequestRemoveDeadlineDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestSendDetails : Sent file request to users via email.
type FileRequestSendDetails struct {
	// RequestTitle : File request title.
	RequestTitle string `json:"request_title"`
}

// NewFileRequestSendDetails returns a new FileRequestSendDetails instance
func NewFileRequestSendDetails(RequestTitle string) *FileRequestSendDetails {
	s := new(FileRequestSendDetails)
	s.RequestTitle = RequestTitle
	return s
}

// FileRequestsChangePolicyDetails : Enabled or disabled file requests.
type FileRequestsChangePolicyDetails struct {
	// NewValue : New file requests policy.
	NewValue *FileRequestsPolicy `json:"new_value"`
	// PreviousValue : Previous file requests policy. Might be missing due to
	// historical data gap.
	PreviousValue *FileRequestsPolicy `json:"previous_value,omitempty"`
}

// NewFileRequestsChangePolicyDetails returns a new FileRequestsChangePolicyDetails instance
func NewFileRequestsChangePolicyDetails(NewValue *FileRequestsPolicy) *FileRequestsChangePolicyDetails {
	s := new(FileRequestsChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// FileRequestsEmailsEnabledDetails : Enabled file request emails for everyone.
type FileRequestsEmailsEnabledDetails struct {
}

// NewFileRequestsEmailsEnabledDetails returns a new FileRequestsEmailsEnabledDetails instance
func NewFileRequestsEmailsEnabledDetails() *FileRequestsEmailsEnabledDetails {
	s := new(FileRequestsEmailsEnabledDetails)
	return s
}

// FileRequestsEmailsRestrictedToTeamOnlyDetails : Allowed file request emails
// for the team.
type FileRequestsEmailsRestrictedToTeamOnlyDetails struct {
}

// NewFileRequestsEmailsRestrictedToTeamOnlyDetails returns a new FileRequestsEmailsRestrictedToTeamOnlyDetails instance
func NewFileRequestsEmailsRestrictedToTeamOnlyDetails() *FileRequestsEmailsRestrictedToTeamOnlyDetails {
	s := new(FileRequestsEmailsRestrictedToTeamOnlyDetails)
	return s
}

// FileRequestsPolicy : File requests policy
type FileRequestsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for FileRequestsPolicy
const (
	FileRequestsPolicyDisabled = "disabled"
	FileRequestsPolicyEnabled  = "enabled"
	FileRequestsPolicyOther    = "other"
)

// FileRestoreDetails : Restored deleted files and/or folders.
type FileRestoreDetails struct {
}

// NewFileRestoreDetails returns a new FileRestoreDetails instance
func NewFileRestoreDetails() *FileRestoreDetails {
	s := new(FileRestoreDetails)
	return s
}

// FileRevertDetails : Reverted files to a previous version.
type FileRevertDetails struct {
}

// NewFileRevertDetails returns a new FileRevertDetails instance
func NewFileRevertDetails() *FileRevertDetails {
	s := new(FileRevertDetails)
	return s
}

// FileRollbackChangesDetails : Rolled back file change location changes.
type FileRollbackChangesDetails struct {
}

// NewFileRollbackChangesDetails returns a new FileRollbackChangesDetails instance
func NewFileRollbackChangesDetails() *FileRollbackChangesDetails {
	s := new(FileRollbackChangesDetails)
	return s
}

// FileSaveCopyReferenceDetails : Save a file or folder using a copy reference.
type FileSaveCopyReferenceDetails struct {
	// RelocateActionDetails : Relocate action details.
	RelocateActionDetails []*RelocateAssetReferencesLogInfo `json:"relocate_action_details"`
}

// NewFileSaveCopyReferenceDetails returns a new FileSaveCopyReferenceDetails instance
func NewFileSaveCopyReferenceDetails(RelocateActionDetails []*RelocateAssetReferencesLogInfo) *FileSaveCopyReferenceDetails {
	s := new(FileSaveCopyReferenceDetails)
	s.RelocateActionDetails = RelocateActionDetails
	return s
}

// FileUnlikeCommentDetails : Unliked a file comment.
type FileUnlikeCommentDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileUnlikeCommentDetails returns a new FileUnlikeCommentDetails instance
func NewFileUnlikeCommentDetails(TargetIndex int64) *FileUnlikeCommentDetails {
	s := new(FileUnlikeCommentDetails)
	s.TargetIndex = TargetIndex
	return s
}

// FolderLogInfo : Folder's logged information.
type FolderLogInfo struct {
	FileOrFolderLogInfo
}

// NewFolderLogInfo returns a new FolderLogInfo instance
func NewFolderLogInfo(Path *PathLogInfo) *FolderLogInfo {
	s := new(FolderLogInfo)
	s.Path = Path
	return s
}

// GeoLocationLogInfo : Geographic location details.
type GeoLocationLogInfo struct {
	// City : City name.
	City string `json:"city,omitempty"`
	// Region : Region name.
	Region string `json:"region,omitempty"`
	// Country : Country code.
	Country string `json:"country,omitempty"`
	// IpAddress : IP address.
	IpAddress string `json:"ip_address"`
}

// NewGeoLocationLogInfo returns a new GeoLocationLogInfo instance
func NewGeoLocationLogInfo(IpAddress string) *GeoLocationLogInfo {
	s := new(GeoLocationLogInfo)
	s.IpAddress = IpAddress
	return s
}

// GetTeamEventsArg : has no documentation (yet)
type GetTeamEventsArg struct {
	// Limit : Number of results to return per call.
	Limit uint32 `json:"limit"`
	// AccountId : Filter the events by account ID. Return ony events with this
	// account_id as either Actor, Context, or Participants.
	AccountId string `json:"account_id,omitempty"`
	// Time : Filter by time range.
	Time *team_common.TimeRange `json:"time,omitempty"`
	// Category : Filter the returned events to a single category.
	Category *EventCategory `json:"category,omitempty"`
}

// NewGetTeamEventsArg returns a new GetTeamEventsArg instance
func NewGetTeamEventsArg() *GetTeamEventsArg {
	s := new(GetTeamEventsArg)
	s.Limit = 1000
	return s
}

// GetTeamEventsContinueArg : has no documentation (yet)
type GetTeamEventsContinueArg struct {
	// Cursor : Indicates from what point to get the next set of events.
	Cursor string `json:"cursor"`
}

// NewGetTeamEventsContinueArg returns a new GetTeamEventsContinueArg instance
func NewGetTeamEventsContinueArg(Cursor string) *GetTeamEventsContinueArg {
	s := new(GetTeamEventsContinueArg)
	s.Cursor = Cursor
	return s
}

// GetTeamEventsContinueError : Errors that can be raised when calling
// `getEventsContinue`.
type GetTeamEventsContinueError struct {
	dropbox.Tagged
}

// Valid tag values for GetTeamEventsContinueError
const (
	GetTeamEventsContinueErrorBadCursor = "bad_cursor"
	GetTeamEventsContinueErrorOther     = "other"
)

// GetTeamEventsError : Errors that can be raised when calling `getEvents`.
type GetTeamEventsError struct {
	dropbox.Tagged
}

// Valid tag values for GetTeamEventsError
const (
	GetTeamEventsErrorAccountIdNotFound = "account_id_not_found"
	GetTeamEventsErrorInvalidTimeRange  = "invalid_time_range"
	GetTeamEventsErrorOther             = "other"
)

// GetTeamEventsResult : has no documentation (yet)
type GetTeamEventsResult struct {
	// Events : List of events.
	Events []*TeamEvent `json:"events"`
	// Cursor : Pass the cursor into `getEventsContinue` to obtain additional
	// events.
	Cursor string `json:"cursor"`
	// HasMore : Is true if there are additional events that have not been
	// returned yet. An additional call to `getEventsContinue` can retrieve
	// them.
	HasMore bool `json:"has_more"`
}

// NewGetTeamEventsResult returns a new GetTeamEventsResult instance
func NewGetTeamEventsResult(Events []*TeamEvent, Cursor string, HasMore bool) *GetTeamEventsResult {
	s := new(GetTeamEventsResult)
	s.Events = Events
	s.Cursor = Cursor
	s.HasMore = HasMore
	return s
}

// GoogleSsoChangePolicyDetails : Enabled or disabled Google single sign-on for
// the team.
type GoogleSsoChangePolicyDetails struct {
	// NewValue : New Google single sign-on policy.
	NewValue *GoogleSsoPolicy `json:"new_value"`
	// PreviousValue : Previous Google single sign-on policy. Might be missing
	// due to historical data gap.
	PreviousValue *GoogleSsoPolicy `json:"previous_value,omitempty"`
}

// NewGoogleSsoChangePolicyDetails returns a new GoogleSsoChangePolicyDetails instance
func NewGoogleSsoChangePolicyDetails(NewValue *GoogleSsoPolicy) *GoogleSsoChangePolicyDetails {
	s := new(GoogleSsoChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// GoogleSsoPolicy : Google SSO policy
type GoogleSsoPolicy struct {
	dropbox.Tagged
}

// Valid tag values for GoogleSsoPolicy
const (
	GoogleSsoPolicyDisabled = "disabled"
	GoogleSsoPolicyEnabled  = "enabled"
	GoogleSsoPolicyOther    = "other"
)

// GroupAddExternalIdDetails : Added an external ID for group.
type GroupAddExternalIdDetails struct {
	// NewValue : Current external id.
	NewValue string `json:"new_value"`
}

// NewGroupAddExternalIdDetails returns a new GroupAddExternalIdDetails instance
func NewGroupAddExternalIdDetails(NewValue string) *GroupAddExternalIdDetails {
	s := new(GroupAddExternalIdDetails)
	s.NewValue = NewValue
	return s
}

// GroupAddMemberDetails : Added team members to a group.
type GroupAddMemberDetails struct {
	// IsGroupOwner : Is group owner.
	IsGroupOwner bool `json:"is_group_owner"`
}

// NewGroupAddMemberDetails returns a new GroupAddMemberDetails instance
func NewGroupAddMemberDetails(IsGroupOwner bool) *GroupAddMemberDetails {
	s := new(GroupAddMemberDetails)
	s.IsGroupOwner = IsGroupOwner
	return s
}

// GroupChangeExternalIdDetails : Changed the external ID for group.
type GroupChangeExternalIdDetails struct {
	// NewValue : Current external id.
	NewValue string `json:"new_value"`
	// PreviousValue : Old external id.
	PreviousValue string `json:"previous_value"`
}

// NewGroupChangeExternalIdDetails returns a new GroupChangeExternalIdDetails instance
func NewGroupChangeExternalIdDetails(NewValue string, PreviousValue string) *GroupChangeExternalIdDetails {
	s := new(GroupChangeExternalIdDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// GroupChangeManagementTypeDetails : Changed group management type.
type GroupChangeManagementTypeDetails struct {
	// NewValue : New group management type.
	NewValue *GroupManagementType `json:"new_value"`
	// PreviousValue : Previous group management type. Might be missing due to
	// historical data gap.
	PreviousValue *GroupManagementType `json:"previous_value,omitempty"`
}

// NewGroupChangeManagementTypeDetails returns a new GroupChangeManagementTypeDetails instance
func NewGroupChangeManagementTypeDetails(NewValue *GroupManagementType) *GroupChangeManagementTypeDetails {
	s := new(GroupChangeManagementTypeDetails)
	s.NewValue = NewValue
	return s
}

// GroupChangeMemberRoleDetails : Changed the manager permissions belonging to a
// group member.
type GroupChangeMemberRoleDetails struct {
	// IsGroupOwner : Is group owner.
	IsGroupOwner bool `json:"is_group_owner"`
}

// NewGroupChangeMemberRoleDetails returns a new GroupChangeMemberRoleDetails instance
func NewGroupChangeMemberRoleDetails(IsGroupOwner bool) *GroupChangeMemberRoleDetails {
	s := new(GroupChangeMemberRoleDetails)
	s.IsGroupOwner = IsGroupOwner
	return s
}

// GroupCreateDetails : Created a group.
type GroupCreateDetails struct {
	// IsAdminManaged : Is admin managed group. Might be missing due to
	// historical data gap.
	IsAdminManaged bool `json:"is_admin_managed,omitempty"`
	// JoinPolicy : Group join policy.
	JoinPolicy *GroupJoinPolicy `json:"join_policy"`
}

// NewGroupCreateDetails returns a new GroupCreateDetails instance
func NewGroupCreateDetails(JoinPolicy *GroupJoinPolicy) *GroupCreateDetails {
	s := new(GroupCreateDetails)
	s.JoinPolicy = JoinPolicy
	return s
}

// GroupDeleteDetails : Deleted a group.
type GroupDeleteDetails struct {
	// IsAdminManaged : Is admin managed group. Might be missing due to
	// historical data gap.
	IsAdminManaged bool `json:"is_admin_managed,omitempty"`
}

// NewGroupDeleteDetails returns a new GroupDeleteDetails instance
func NewGroupDeleteDetails() *GroupDeleteDetails {
	s := new(GroupDeleteDetails)
	return s
}

// GroupDescriptionUpdatedDetails : Updated a group.
type GroupDescriptionUpdatedDetails struct {
}

// NewGroupDescriptionUpdatedDetails returns a new GroupDescriptionUpdatedDetails instance
func NewGroupDescriptionUpdatedDetails() *GroupDescriptionUpdatedDetails {
	s := new(GroupDescriptionUpdatedDetails)
	return s
}

// GroupJoinPolicy : has no documentation (yet)
type GroupJoinPolicy struct {
	dropbox.Tagged
}

// Valid tag values for GroupJoinPolicy
const (
	GroupJoinPolicyOpen          = "open"
	GroupJoinPolicyRequestToJoin = "request_to_join"
	GroupJoinPolicyOther         = "other"
)

// GroupJoinPolicyUpdatedDetails : Updated a group join policy.
type GroupJoinPolicyUpdatedDetails struct {
	// IsAdminManaged : Is admin managed group. Might be missing due to
	// historical data gap.
	IsAdminManaged bool `json:"is_admin_managed,omitempty"`
	// JoinPolicy : Group join policy.
	JoinPolicy *GroupJoinPolicy `json:"join_policy"`
}

// NewGroupJoinPolicyUpdatedDetails returns a new GroupJoinPolicyUpdatedDetails instance
func NewGroupJoinPolicyUpdatedDetails(JoinPolicy *GroupJoinPolicy) *GroupJoinPolicyUpdatedDetails {
	s := new(GroupJoinPolicyUpdatedDetails)
	s.JoinPolicy = JoinPolicy
	return s
}

// GroupLogInfo : Group's logged information.
type GroupLogInfo struct {
	// GroupId : The unique id of this group. Might be missing due to historical
	// data gap.
	GroupId string `json:"group_id,omitempty"`
	// DisplayName : The name of this group.
	DisplayName string `json:"display_name"`
	// ExternalId : External group ID. Might be missing due to historical data
	// gap.
	ExternalId string `json:"external_id,omitempty"`
}

// NewGroupLogInfo returns a new GroupLogInfo instance
func NewGroupLogInfo(DisplayName string) *GroupLogInfo {
	s := new(GroupLogInfo)
	s.DisplayName = DisplayName
	return s
}

// GroupManagementType : has no documentation (yet)
type GroupManagementType struct {
	dropbox.Tagged
}

// Valid tag values for GroupManagementType
const (
	GroupManagementTypeAdminManagementGroup  = "admin_management_group"
	GroupManagementTypeMemberManagementGroup = "member_management_group"
	GroupManagementTypeOther                 = "other"
)

// GroupMovedDetails : Moved a group.
type GroupMovedDetails struct {
}

// NewGroupMovedDetails returns a new GroupMovedDetails instance
func NewGroupMovedDetails() *GroupMovedDetails {
	s := new(GroupMovedDetails)
	return s
}

// GroupRemoveExternalIdDetails : Removed the external ID for group.
type GroupRemoveExternalIdDetails struct {
	// PreviousValue : Old external id.
	PreviousValue string `json:"previous_value"`
}

// NewGroupRemoveExternalIdDetails returns a new GroupRemoveExternalIdDetails instance
func NewGroupRemoveExternalIdDetails(PreviousValue string) *GroupRemoveExternalIdDetails {
	s := new(GroupRemoveExternalIdDetails)
	s.PreviousValue = PreviousValue
	return s
}

// GroupRemoveMemberDetails : Removed team members from a group.
type GroupRemoveMemberDetails struct {
}

// NewGroupRemoveMemberDetails returns a new GroupRemoveMemberDetails instance
func NewGroupRemoveMemberDetails() *GroupRemoveMemberDetails {
	s := new(GroupRemoveMemberDetails)
	return s
}

// GroupRenameDetails : Renamed a group.
type GroupRenameDetails struct {
	// PreviousValue : Previous display name.
	PreviousValue string `json:"previous_value"`
}

// NewGroupRenameDetails returns a new GroupRenameDetails instance
func NewGroupRenameDetails(PreviousValue string) *GroupRenameDetails {
	s := new(GroupRenameDetails)
	s.PreviousValue = PreviousValue
	return s
}

// GroupUserManagementChangePolicyDetails : Changed who can create groups.
type GroupUserManagementChangePolicyDetails struct {
	// NewValue : New group users management policy.
	NewValue *GroupUserManagementPolicy `json:"new_value"`
	// PreviousValue : Previous group users management policy. Might be missing
	// due to historical data gap.
	PreviousValue *GroupUserManagementPolicy `json:"previous_value,omitempty"`
}

// NewGroupUserManagementChangePolicyDetails returns a new GroupUserManagementChangePolicyDetails instance
func NewGroupUserManagementChangePolicyDetails(NewValue *GroupUserManagementPolicy) *GroupUserManagementChangePolicyDetails {
	s := new(GroupUserManagementChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// GroupUserManagementPolicy : has no documentation (yet)
type GroupUserManagementPolicy struct {
	dropbox.Tagged
}

// Valid tag values for GroupUserManagementPolicy
const (
	GroupUserManagementPolicyAllUsers   = "all_users"
	GroupUserManagementPolicyOnlyAdmins = "only_admins"
	GroupUserManagementPolicyOther      = "other"
)

// HostLogInfo : Host details.
type HostLogInfo struct {
	// HostId : Host ID. Might be missing due to historical data gap.
	HostId uint64 `json:"host_id,omitempty"`
	// HostName : Host name. Might be missing due to historical data gap.
	HostName string `json:"host_name,omitempty"`
}

// NewHostLogInfo returns a new HostLogInfo instance
func NewHostLogInfo() *HostLogInfo {
	s := new(HostLogInfo)
	return s
}

// JoinTeamDetails : Additional information relevant when a new member joins the
// team.
type JoinTeamDetails struct {
	// LinkedApps : Linked applications.
	LinkedApps []IsAppLogInfo `json:"linked_apps"`
	// LinkedDevices : Linked devices.
	LinkedDevices []*DeviceLogInfo `json:"linked_devices"`
	// LinkedSharedFolders : Linked shared folders.
	LinkedSharedFolders []*FolderLogInfo `json:"linked_shared_folders"`
}

// NewJoinTeamDetails returns a new JoinTeamDetails instance
func NewJoinTeamDetails(LinkedApps []IsAppLogInfo, LinkedDevices []*DeviceLogInfo, LinkedSharedFolders []*FolderLogInfo) *JoinTeamDetails {
	s := new(JoinTeamDetails)
	s.LinkedApps = LinkedApps
	s.LinkedDevices = LinkedDevices
	s.LinkedSharedFolders = LinkedSharedFolders
	return s
}

// LinkAudience : has no documentation (yet)
type LinkAudience struct {
	dropbox.Tagged
}

// Valid tag values for LinkAudience
const (
	LinkAudiencePublic  = "public"
	LinkAudienceTeam    = "team"
	LinkAudienceMembers = "members"
	LinkAudienceOther   = "other"
)

// LogoutDetails : Signed out.
type LogoutDetails struct {
}

// NewLogoutDetails returns a new LogoutDetails instance
func NewLogoutDetails() *LogoutDetails {
	s := new(LogoutDetails)
	return s
}

// MemberAddNameDetails : Set team member name when joining team.
type MemberAddNameDetails struct {
	// Value : User's name.
	Value *UserNameLogInfo `json:"value"`
}

// NewMemberAddNameDetails returns a new MemberAddNameDetails instance
func NewMemberAddNameDetails(Value *UserNameLogInfo) *MemberAddNameDetails {
	s := new(MemberAddNameDetails)
	s.Value = Value
	return s
}

// MemberChangeAdminRoleDetails : Change the admin role belonging to team
// member.
type MemberChangeAdminRoleDetails struct {
	// NewValue : New admin role. This field is relevant when the admin role is
	// changed or whenthe user role changes from no admin rights to with admin
	// rights.
	NewValue *AdminRole `json:"new_value,omitempty"`
	// PreviousValue : Previous admin role. This field is relevant when the
	// admin role is changed or when the admin role is removed.
	PreviousValue *AdminRole `json:"previous_value,omitempty"`
}

// NewMemberChangeAdminRoleDetails returns a new MemberChangeAdminRoleDetails instance
func NewMemberChangeAdminRoleDetails() *MemberChangeAdminRoleDetails {
	s := new(MemberChangeAdminRoleDetails)
	return s
}

// MemberChangeEmailDetails : Changed team member email address.
type MemberChangeEmailDetails struct {
	// NewValue : New email.
	NewValue string `json:"new_value"`
	// PreviousValue : Previous email. Might be missing due to historical data
	// gap.
	PreviousValue string `json:"previous_value,omitempty"`
}

// NewMemberChangeEmailDetails returns a new MemberChangeEmailDetails instance
func NewMemberChangeEmailDetails(NewValue string) *MemberChangeEmailDetails {
	s := new(MemberChangeEmailDetails)
	s.NewValue = NewValue
	return s
}

// MemberChangeMembershipTypeDetails : Changed the membership type (limited vs
// full) for team member.
type MemberChangeMembershipTypeDetails struct {
	// PrevValue : Previous membership type.
	PrevValue *TeamMembershipType `json:"prev_value"`
	// NewValue : New membership type.
	NewValue *TeamMembershipType `json:"new_value"`
}

// NewMemberChangeMembershipTypeDetails returns a new MemberChangeMembershipTypeDetails instance
func NewMemberChangeMembershipTypeDetails(PrevValue *TeamMembershipType, NewValue *TeamMembershipType) *MemberChangeMembershipTypeDetails {
	s := new(MemberChangeMembershipTypeDetails)
	s.PrevValue = PrevValue
	s.NewValue = NewValue
	return s
}

// MemberChangeNameDetails : Changed team member name.
type MemberChangeNameDetails struct {
	// NewValue : New user's name.
	NewValue *UserNameLogInfo `json:"new_value"`
	// PreviousValue : Previous user's name.
	PreviousValue *UserNameLogInfo `json:"previous_value"`
}

// NewMemberChangeNameDetails returns a new MemberChangeNameDetails instance
func NewMemberChangeNameDetails(NewValue *UserNameLogInfo, PreviousValue *UserNameLogInfo) *MemberChangeNameDetails {
	s := new(MemberChangeNameDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// MemberChangeStatusDetails : Changed the membership status of a team member.
type MemberChangeStatusDetails struct {
	// PreviousValue : Previous member status. Might be missing due to
	// historical data gap.
	PreviousValue *MemberStatus `json:"previous_value,omitempty"`
	// NewValue : New member status.
	NewValue *MemberStatus `json:"new_value"`
	// TeamJoinDetails : Additional information relevant when a new member joins
	// the team.
	TeamJoinDetails *JoinTeamDetails `json:"team_join_details,omitempty"`
}

// NewMemberChangeStatusDetails returns a new MemberChangeStatusDetails instance
func NewMemberChangeStatusDetails(NewValue *MemberStatus) *MemberChangeStatusDetails {
	s := new(MemberChangeStatusDetails)
	s.NewValue = NewValue
	return s
}

// MemberPermanentlyDeleteAccountContentsDetails : Permanently deleted contents
// of a removed team member account.
type MemberPermanentlyDeleteAccountContentsDetails struct {
}

// NewMemberPermanentlyDeleteAccountContentsDetails returns a new MemberPermanentlyDeleteAccountContentsDetails instance
func NewMemberPermanentlyDeleteAccountContentsDetails() *MemberPermanentlyDeleteAccountContentsDetails {
	s := new(MemberPermanentlyDeleteAccountContentsDetails)
	return s
}

// MemberRequestsChangePolicyDetails : Changed whether users can find the team
// when not invited.
type MemberRequestsChangePolicyDetails struct {
	// NewValue : New member change requests policy.
	NewValue *MemberRequestsPolicy `json:"new_value"`
	// PreviousValue : Previous member change requests policy. Might be missing
	// due to historical data gap.
	PreviousValue *MemberRequestsPolicy `json:"previous_value,omitempty"`
}

// NewMemberRequestsChangePolicyDetails returns a new MemberRequestsChangePolicyDetails instance
func NewMemberRequestsChangePolicyDetails(NewValue *MemberRequestsPolicy) *MemberRequestsChangePolicyDetails {
	s := new(MemberRequestsChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// MemberRequestsPolicy : has no documentation (yet)
type MemberRequestsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for MemberRequestsPolicy
const (
	MemberRequestsPolicyDisabled        = "disabled"
	MemberRequestsPolicyRequireApproval = "require_approval"
	MemberRequestsPolicyAutoApproval    = "auto_approval"
	MemberRequestsPolicyOther           = "other"
)

// MemberSpaceLimitsAddExceptionDetails : Added an exception for one or more
// team members to bypass space limits imposed by policy.
type MemberSpaceLimitsAddExceptionDetails struct {
}

// NewMemberSpaceLimitsAddExceptionDetails returns a new MemberSpaceLimitsAddExceptionDetails instance
func NewMemberSpaceLimitsAddExceptionDetails() *MemberSpaceLimitsAddExceptionDetails {
	s := new(MemberSpaceLimitsAddExceptionDetails)
	return s
}

// MemberSpaceLimitsChangePolicyDetails : Changed the storage limits applied to
// team members by policy.
type MemberSpaceLimitsChangePolicyDetails struct {
	// PreviousValue : Previous storage limits policy.
	PreviousValue *SpaceLimitsLevel `json:"previous_value"`
	// NewValue : New storage limits policy.
	NewValue *SpaceLimitsLevel `json:"new_value"`
}

// NewMemberSpaceLimitsChangePolicyDetails returns a new MemberSpaceLimitsChangePolicyDetails instance
func NewMemberSpaceLimitsChangePolicyDetails(PreviousValue *SpaceLimitsLevel, NewValue *SpaceLimitsLevel) *MemberSpaceLimitsChangePolicyDetails {
	s := new(MemberSpaceLimitsChangePolicyDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// MemberSpaceLimitsChangeStatusDetails : Changed the status with respect to
// whether the team member is under or over storage quota specified by policy.
type MemberSpaceLimitsChangeStatusDetails struct {
	// PreviousValue : Previous storage quota status.
	PreviousValue *SpaceLimitsStatus `json:"previous_value"`
	// NewValue : New storage quota status.
	NewValue *SpaceLimitsStatus `json:"new_value"`
}

// NewMemberSpaceLimitsChangeStatusDetails returns a new MemberSpaceLimitsChangeStatusDetails instance
func NewMemberSpaceLimitsChangeStatusDetails(PreviousValue *SpaceLimitsStatus, NewValue *SpaceLimitsStatus) *MemberSpaceLimitsChangeStatusDetails {
	s := new(MemberSpaceLimitsChangeStatusDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// MemberSpaceLimitsRemoveExceptionDetails : Removed an exception for one or
// more team members to bypass space limits imposed by policy.
type MemberSpaceLimitsRemoveExceptionDetails struct {
}

// NewMemberSpaceLimitsRemoveExceptionDetails returns a new MemberSpaceLimitsRemoveExceptionDetails instance
func NewMemberSpaceLimitsRemoveExceptionDetails() *MemberSpaceLimitsRemoveExceptionDetails {
	s := new(MemberSpaceLimitsRemoveExceptionDetails)
	return s
}

// MemberStatus : has no documentation (yet)
type MemberStatus struct {
	dropbox.Tagged
}

// Valid tag values for MemberStatus
const (
	MemberStatusNotJoined = "not_joined"
	MemberStatusInvited   = "invited"
	MemberStatusActive    = "active"
	MemberStatusSuspended = "suspended"
	MemberStatusRemoved   = "removed"
	MemberStatusOther     = "other"
)

// MemberSuggestDetails : Suggested a new team member to be added to the team.
type MemberSuggestDetails struct {
}

// NewMemberSuggestDetails returns a new MemberSuggestDetails instance
func NewMemberSuggestDetails() *MemberSuggestDetails {
	s := new(MemberSuggestDetails)
	return s
}

// MemberSuggestionsChangePolicyDetails : Enabled or disabled the option for
// team members to suggest new members to add to the team.
type MemberSuggestionsChangePolicyDetails struct {
	// NewValue : New team member suggestions policy.
	NewValue *MemberSuggestionsPolicy `json:"new_value"`
	// PreviousValue : Previous team member suggestions policy. Might be missing
	// due to historical data gap.
	PreviousValue *MemberSuggestionsPolicy `json:"previous_value,omitempty"`
}

// NewMemberSuggestionsChangePolicyDetails returns a new MemberSuggestionsChangePolicyDetails instance
func NewMemberSuggestionsChangePolicyDetails(NewValue *MemberSuggestionsPolicy) *MemberSuggestionsChangePolicyDetails {
	s := new(MemberSuggestionsChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// MemberSuggestionsPolicy : Member suggestions policy
type MemberSuggestionsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for MemberSuggestionsPolicy
const (
	MemberSuggestionsPolicyDisabled = "disabled"
	MemberSuggestionsPolicyEnabled  = "enabled"
	MemberSuggestionsPolicyOther    = "other"
)

// MemberTransferAccountContentsDetails : Transferred contents of a removed team
// member account to another member.
type MemberTransferAccountContentsDetails struct {
	// SrcIndex : Source asset index.
	SrcIndex int64 `json:"src_index"`
	// DestIndex : Destination asset index.
	DestIndex int64 `json:"dest_index"`
}

// NewMemberTransferAccountContentsDetails returns a new MemberTransferAccountContentsDetails instance
func NewMemberTransferAccountContentsDetails(SrcIndex int64, DestIndex int64) *MemberTransferAccountContentsDetails {
	s := new(MemberTransferAccountContentsDetails)
	s.SrcIndex = SrcIndex
	s.DestIndex = DestIndex
	return s
}

// MicrosoftOfficeAddinChangePolicyDetails : Enabled or disabled the Microsoft
// Office add-in, which lets team members save files to Dropbox directly from
// Microsoft Office.
type MicrosoftOfficeAddinChangePolicyDetails struct {
	// NewValue : New Microsoft Office addin policy.
	NewValue *MicrosoftOfficeAddinPolicy `json:"new_value"`
	// PreviousValue : Previous Microsoft Office addin policy. Might be missing
	// due to historical data gap.
	PreviousValue *MicrosoftOfficeAddinPolicy `json:"previous_value,omitempty"`
}

// NewMicrosoftOfficeAddinChangePolicyDetails returns a new MicrosoftOfficeAddinChangePolicyDetails instance
func NewMicrosoftOfficeAddinChangePolicyDetails(NewValue *MicrosoftOfficeAddinPolicy) *MicrosoftOfficeAddinChangePolicyDetails {
	s := new(MicrosoftOfficeAddinChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// MicrosoftOfficeAddinPolicy : Microsoft Office addin policy
type MicrosoftOfficeAddinPolicy struct {
	dropbox.Tagged
}

// Valid tag values for MicrosoftOfficeAddinPolicy
const (
	MicrosoftOfficeAddinPolicyDisabled = "disabled"
	MicrosoftOfficeAddinPolicyEnabled  = "enabled"
	MicrosoftOfficeAddinPolicyOther    = "other"
)

// MissingDetails : An indication that an event was returned with missing
// details
type MissingDetails struct {
}

// NewMissingDetails returns a new MissingDetails instance
func NewMissingDetails() *MissingDetails {
	s := new(MissingDetails)
	return s
}

// MobileSessionLogInfo : Mobile session.
type MobileSessionLogInfo struct {
	SessionLogInfo
}

// NewMobileSessionLogInfo returns a new MobileSessionLogInfo instance
func NewMobileSessionLogInfo() *MobileSessionLogInfo {
	s := new(MobileSessionLogInfo)
	return s
}

// NamespaceRelativePathLogInfo : Namespace relative path details.
type NamespaceRelativePathLogInfo struct {
	// NsId : Namespace ID. Might be missing due to historical data gap.
	NsId string `json:"ns_id,omitempty"`
	// RelativePath : A path relative to the specified namespace ID. Might be
	// missing due to historical data gap.
	RelativePath string `json:"relative_path,omitempty"`
}

// NewNamespaceRelativePathLogInfo returns a new NamespaceRelativePathLogInfo instance
func NewNamespaceRelativePathLogInfo() *NamespaceRelativePathLogInfo {
	s := new(NamespaceRelativePathLogInfo)
	return s
}

// NetworkControlChangePolicyDetails : Enabled or disabled network control.
type NetworkControlChangePolicyDetails struct {
	// NewValue : New network control policy.
	NewValue *NetworkControlPolicy `json:"new_value"`
	// PreviousValue : Previous network control policy. Might be missing due to
	// historical data gap.
	PreviousValue *NetworkControlPolicy `json:"previous_value,omitempty"`
}

// NewNetworkControlChangePolicyDetails returns a new NetworkControlChangePolicyDetails instance
func NewNetworkControlChangePolicyDetails(NewValue *NetworkControlPolicy) *NetworkControlChangePolicyDetails {
	s := new(NetworkControlChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// NetworkControlPolicy : Network control policy
type NetworkControlPolicy struct {
	dropbox.Tagged
}

// Valid tag values for NetworkControlPolicy
const (
	NetworkControlPolicyDisabled = "disabled"
	NetworkControlPolicyEnabled  = "enabled"
	NetworkControlPolicyOther    = "other"
)

// UserLogInfo : User's logged information.
type UserLogInfo struct {
	// AccountId : User unique ID. Might be missing due to historical data gap.
	AccountId string `json:"account_id,omitempty"`
	// DisplayName : User display name. Might be missing due to historical data
	// gap.
	DisplayName string `json:"display_name,omitempty"`
	// Email : User email address. Might be missing due to historical data gap.
	Email string `json:"email,omitempty"`
}

// NewUserLogInfo returns a new UserLogInfo instance
func NewUserLogInfo() *UserLogInfo {
	s := new(UserLogInfo)
	return s
}

// IsUserLogInfo is the interface type for UserLogInfo and its subtypes
type IsUserLogInfo interface {
	IsUserLogInfo()
}

// IsUserLogInfo implements the IsUserLogInfo interface
func (u *UserLogInfo) IsUserLogInfo() {}

type userLogInfoUnion struct {
	dropbox.Tagged
	// TeamMember : has no documentation (yet)
	TeamMember *TeamMemberLogInfo `json:"team_member,omitempty"`
	// NonTeamMember : has no documentation (yet)
	NonTeamMember *NonTeamMemberLogInfo `json:"non_team_member,omitempty"`
}

// Valid tag values for UserLogInfo
const (
	UserLogInfoTeamMember    = "team_member"
	UserLogInfoNonTeamMember = "non_team_member"
)

// UnmarshalJSON deserializes into a userLogInfoUnion instance
func (u *userLogInfoUnion) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// TeamMember : has no documentation (yet)
		TeamMember json.RawMessage `json:"team_member,omitempty"`
		// NonTeamMember : has no documentation (yet)
		NonTeamMember json.RawMessage `json:"non_team_member,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "team_member":
		err = json.Unmarshal(body, &u.TeamMember)

		if err != nil {
			return err
		}
	case "non_team_member":
		err = json.Unmarshal(body, &u.NonTeamMember)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsUserLogInfoFromJSON converts JSON to a concrete IsUserLogInfo instance
func IsUserLogInfoFromJSON(data []byte) (IsUserLogInfo, error) {
	var t userLogInfoUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "team_member":
		return t.TeamMember, nil

	case "non_team_member":
		return t.NonTeamMember, nil

	}
	return nil, nil
}

// NonTeamMemberLogInfo : Non team member's logged information.
type NonTeamMemberLogInfo struct {
	UserLogInfo
}

// NewNonTeamMemberLogInfo returns a new NonTeamMemberLogInfo instance
func NewNonTeamMemberLogInfo() *NonTeamMemberLogInfo {
	s := new(NonTeamMemberLogInfo)
	return s
}

// NoteAclInviteOnlyDetails : Changed a Paper document to be invite-only.
type NoteAclInviteOnlyDetails struct {
}

// NewNoteAclInviteOnlyDetails returns a new NoteAclInviteOnlyDetails instance
func NewNoteAclInviteOnlyDetails() *NoteAclInviteOnlyDetails {
	s := new(NoteAclInviteOnlyDetails)
	return s
}

// NoteAclLinkDetails : Changed a Paper document to be link accessible.
type NoteAclLinkDetails struct {
}

// NewNoteAclLinkDetails returns a new NoteAclLinkDetails instance
func NewNoteAclLinkDetails() *NoteAclLinkDetails {
	s := new(NoteAclLinkDetails)
	return s
}

// NoteAclTeamLinkDetails : Changed a Paper document to be link accessible for
// the team.
type NoteAclTeamLinkDetails struct {
}

// NewNoteAclTeamLinkDetails returns a new NoteAclTeamLinkDetails instance
func NewNoteAclTeamLinkDetails() *NoteAclTeamLinkDetails {
	s := new(NoteAclTeamLinkDetails)
	return s
}

// NoteShareReceiveDetails : Shared Paper document received.
type NoteShareReceiveDetails struct {
}

// NewNoteShareReceiveDetails returns a new NoteShareReceiveDetails instance
func NewNoteShareReceiveDetails() *NoteShareReceiveDetails {
	s := new(NoteShareReceiveDetails)
	return s
}

// NoteSharedDetails : Shared a Paper doc.
type NoteSharedDetails struct {
}

// NewNoteSharedDetails returns a new NoteSharedDetails instance
func NewNoteSharedDetails() *NoteSharedDetails {
	s := new(NoteSharedDetails)
	return s
}

// OpenNoteSharedDetails : Opened a shared Paper doc.
type OpenNoteSharedDetails struct {
}

// NewOpenNoteSharedDetails returns a new OpenNoteSharedDetails instance
func NewOpenNoteSharedDetails() *OpenNoteSharedDetails {
	s := new(OpenNoteSharedDetails)
	return s
}

// OriginLogInfo : The origin from which the actor performed the action.
type OriginLogInfo struct {
	// GeoLocation : Geographic location details.
	GeoLocation *GeoLocationLogInfo `json:"geo_location,omitempty"`
	// Host : Host details.
	Host *HostLogInfo `json:"host,omitempty"`
	// AccessMethod : The method that was used to perform the action.
	AccessMethod *AccessMethodLogInfo `json:"access_method"`
}

// NewOriginLogInfo returns a new OriginLogInfo instance
func NewOriginLogInfo(AccessMethod *AccessMethodLogInfo) *OriginLogInfo {
	s := new(OriginLogInfo)
	s.AccessMethod = AccessMethod
	return s
}

// PaperAccessType : has no documentation (yet)
type PaperAccessType struct {
	dropbox.Tagged
}

// Valid tag values for PaperAccessType
const (
	PaperAccessTypeViewer    = "viewer"
	PaperAccessTypeCommenter = "commenter"
	PaperAccessTypeEditor    = "editor"
	PaperAccessTypeOther     = "other"
)

// PaperChangeDeploymentPolicyDetails : Changed whether Dropbox Paper, when
// enabled, is deployed to all teams or to specific members of the team.
type PaperChangeDeploymentPolicyDetails struct {
	// NewValue : New Dropbox Paper deployment policy.
	NewValue *PaperDeploymentPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Paper deployment policy. Might be
	// missing due to historical data gap.
	PreviousValue *PaperDeploymentPolicy `json:"previous_value,omitempty"`
}

// NewPaperChangeDeploymentPolicyDetails returns a new PaperChangeDeploymentPolicyDetails instance
func NewPaperChangeDeploymentPolicyDetails(NewValue *PaperDeploymentPolicy) *PaperChangeDeploymentPolicyDetails {
	s := new(PaperChangeDeploymentPolicyDetails)
	s.NewValue = NewValue
	return s
}

// PaperChangeMemberPolicyDetails : Changed whether team members can share Paper
// documents externally (i.e. outside the team), and if so, whether they should
// be accessible only by team members or anyone by default.
type PaperChangeMemberPolicyDetails struct {
	// NewValue : New paper external accessibility policy.
	NewValue *PaperMemberPolicy `json:"new_value"`
	// PreviousValue : Previous paper external accessibility policy. Might be
	// missing due to historical data gap.
	PreviousValue *PaperMemberPolicy `json:"previous_value,omitempty"`
}

// NewPaperChangeMemberPolicyDetails returns a new PaperChangeMemberPolicyDetails instance
func NewPaperChangeMemberPolicyDetails(NewValue *PaperMemberPolicy) *PaperChangeMemberPolicyDetails {
	s := new(PaperChangeMemberPolicyDetails)
	s.NewValue = NewValue
	return s
}

// PaperChangePolicyDetails : Enabled or disabled Dropbox Paper for the team.
type PaperChangePolicyDetails struct {
	// NewValue : New Dropbox Paper policy.
	NewValue *PaperPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Paper policy. Might be missing due to
	// historical data gap.
	PreviousValue *PaperPolicy `json:"previous_value,omitempty"`
}

// NewPaperChangePolicyDetails returns a new PaperChangePolicyDetails instance
func NewPaperChangePolicyDetails(NewValue *PaperPolicy) *PaperChangePolicyDetails {
	s := new(PaperChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// PaperContentAddMemberDetails : Added users to the membership of a Paper doc
// or folder.
type PaperContentAddMemberDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentAddMemberDetails returns a new PaperContentAddMemberDetails instance
func NewPaperContentAddMemberDetails(EventUuid string) *PaperContentAddMemberDetails {
	s := new(PaperContentAddMemberDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentAddToFolderDetails : Added Paper doc or folder to a folder.
type PaperContentAddToFolderDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// ParentIndex : Parent asset index.
	ParentIndex int64 `json:"parent_index"`
}

// NewPaperContentAddToFolderDetails returns a new PaperContentAddToFolderDetails instance
func NewPaperContentAddToFolderDetails(EventUuid string, TargetIndex int64, ParentIndex int64) *PaperContentAddToFolderDetails {
	s := new(PaperContentAddToFolderDetails)
	s.EventUuid = EventUuid
	s.TargetIndex = TargetIndex
	s.ParentIndex = ParentIndex
	return s
}

// PaperContentArchiveDetails : Archived Paper doc or folder.
type PaperContentArchiveDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentArchiveDetails returns a new PaperContentArchiveDetails instance
func NewPaperContentArchiveDetails(EventUuid string) *PaperContentArchiveDetails {
	s := new(PaperContentArchiveDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentChangeSubscriptionDetails : Followed or unfollowed a Paper doc or
// folder.
type PaperContentChangeSubscriptionDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// NewSubscriptionLevel : New subscription level.
	NewSubscriptionLevel *PaperTaggedValue `json:"new_subscription_level"`
	// PreviousSubscriptionLevel : Previous subscription level. Might be missing
	// due to historical data gap.
	PreviousSubscriptionLevel *PaperTaggedValue `json:"previous_subscription_level,omitempty"`
}

// NewPaperContentChangeSubscriptionDetails returns a new PaperContentChangeSubscriptionDetails instance
func NewPaperContentChangeSubscriptionDetails(EventUuid string, NewSubscriptionLevel *PaperTaggedValue) *PaperContentChangeSubscriptionDetails {
	s := new(PaperContentChangeSubscriptionDetails)
	s.EventUuid = EventUuid
	s.NewSubscriptionLevel = NewSubscriptionLevel
	return s
}

// PaperContentCreateDetails : Created a Paper doc or folder.
type PaperContentCreateDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentCreateDetails returns a new PaperContentCreateDetails instance
func NewPaperContentCreateDetails(EventUuid string) *PaperContentCreateDetails {
	s := new(PaperContentCreateDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentPermanentlyDeleteDetails : Permanently deleted a Paper doc or
// folder.
type PaperContentPermanentlyDeleteDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentPermanentlyDeleteDetails returns a new PaperContentPermanentlyDeleteDetails instance
func NewPaperContentPermanentlyDeleteDetails(EventUuid string) *PaperContentPermanentlyDeleteDetails {
	s := new(PaperContentPermanentlyDeleteDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentRemoveFromFolderDetails : Removed Paper doc or folder from a
// folder.
type PaperContentRemoveFromFolderDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentRemoveFromFolderDetails returns a new PaperContentRemoveFromFolderDetails instance
func NewPaperContentRemoveFromFolderDetails(EventUuid string) *PaperContentRemoveFromFolderDetails {
	s := new(PaperContentRemoveFromFolderDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentRemoveMemberDetails : Removed a user from the membership of a
// Paper doc or folder.
type PaperContentRemoveMemberDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentRemoveMemberDetails returns a new PaperContentRemoveMemberDetails instance
func NewPaperContentRemoveMemberDetails(EventUuid string) *PaperContentRemoveMemberDetails {
	s := new(PaperContentRemoveMemberDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentRenameDetails : Renamed Paper doc or folder.
type PaperContentRenameDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentRenameDetails returns a new PaperContentRenameDetails instance
func NewPaperContentRenameDetails(EventUuid string) *PaperContentRenameDetails {
	s := new(PaperContentRenameDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperContentRestoreDetails : Restored an archived Paper doc or folder.
type PaperContentRestoreDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperContentRestoreDetails returns a new PaperContentRestoreDetails instance
func NewPaperContentRestoreDetails(EventUuid string) *PaperContentRestoreDetails {
	s := new(PaperContentRestoreDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDeploymentPolicy : has no documentation (yet)
type PaperDeploymentPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperDeploymentPolicy
const (
	PaperDeploymentPolicyPartial = "partial"
	PaperDeploymentPolicyFull    = "full"
	PaperDeploymentPolicyOther   = "other"
)

// PaperDocAddCommentDetails : Added a Paper doc comment.
type PaperDocAddCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewPaperDocAddCommentDetails returns a new PaperDocAddCommentDetails instance
func NewPaperDocAddCommentDetails(EventUuid string) *PaperDocAddCommentDetails {
	s := new(PaperDocAddCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocChangeMemberRoleDetails : Changed the access type of a Paper doc
// member.
type PaperDocChangeMemberRoleDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// AccessType : Paper doc access type.
	AccessType *PaperAccessType `json:"access_type"`
}

// NewPaperDocChangeMemberRoleDetails returns a new PaperDocChangeMemberRoleDetails instance
func NewPaperDocChangeMemberRoleDetails(EventUuid string, AccessType *PaperAccessType) *PaperDocChangeMemberRoleDetails {
	s := new(PaperDocChangeMemberRoleDetails)
	s.EventUuid = EventUuid
	s.AccessType = AccessType
	return s
}

// PaperDocChangeSharingPolicyDetails : Changed the sharing policy for Paper
// doc.
type PaperDocChangeSharingPolicyDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// PublicSharingPolicy : Sharing policy with external users. Might be
	// missing due to historical data gap.
	PublicSharingPolicy string `json:"public_sharing_policy,omitempty"`
	// TeamSharingPolicy : Sharing policy with team. Might be missing due to
	// historical data gap.
	TeamSharingPolicy string `json:"team_sharing_policy,omitempty"`
}

// NewPaperDocChangeSharingPolicyDetails returns a new PaperDocChangeSharingPolicyDetails instance
func NewPaperDocChangeSharingPolicyDetails(EventUuid string) *PaperDocChangeSharingPolicyDetails {
	s := new(PaperDocChangeSharingPolicyDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocDeleteCommentDetails : Deleted a Paper doc comment.
type PaperDocDeleteCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewPaperDocDeleteCommentDetails returns a new PaperDocDeleteCommentDetails instance
func NewPaperDocDeleteCommentDetails(EventUuid string) *PaperDocDeleteCommentDetails {
	s := new(PaperDocDeleteCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocDeletedDetails : Paper doc archived.
type PaperDocDeletedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocDeletedDetails returns a new PaperDocDeletedDetails instance
func NewPaperDocDeletedDetails(EventUuid string) *PaperDocDeletedDetails {
	s := new(PaperDocDeletedDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocDownloadDetails : Downloaded a Paper doc in a particular output
// format.
type PaperDocDownloadDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// ExportFileFormat : Export file format.
	ExportFileFormat *PaperDownloadFormat `json:"export_file_format"`
}

// NewPaperDocDownloadDetails returns a new PaperDocDownloadDetails instance
func NewPaperDocDownloadDetails(EventUuid string, ExportFileFormat *PaperDownloadFormat) *PaperDocDownloadDetails {
	s := new(PaperDocDownloadDetails)
	s.EventUuid = EventUuid
	s.ExportFileFormat = ExportFileFormat
	return s
}

// PaperDocEditCommentDetails : Edited a Paper doc comment.
type PaperDocEditCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewPaperDocEditCommentDetails returns a new PaperDocEditCommentDetails instance
func NewPaperDocEditCommentDetails(EventUuid string) *PaperDocEditCommentDetails {
	s := new(PaperDocEditCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocEditDetails : Edited a Paper doc.
type PaperDocEditDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocEditDetails returns a new PaperDocEditDetails instance
func NewPaperDocEditDetails(EventUuid string) *PaperDocEditDetails {
	s := new(PaperDocEditDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocFollowedDetails : Followed a Paper doc.
type PaperDocFollowedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocFollowedDetails returns a new PaperDocFollowedDetails instance
func NewPaperDocFollowedDetails(EventUuid string) *PaperDocFollowedDetails {
	s := new(PaperDocFollowedDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocMentionDetails : Mentioned a member in a Paper doc.
type PaperDocMentionDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocMentionDetails returns a new PaperDocMentionDetails instance
func NewPaperDocMentionDetails(EventUuid string) *PaperDocMentionDetails {
	s := new(PaperDocMentionDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocRequestAccessDetails : Requested to be a member on a Paper doc.
type PaperDocRequestAccessDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocRequestAccessDetails returns a new PaperDocRequestAccessDetails instance
func NewPaperDocRequestAccessDetails(EventUuid string) *PaperDocRequestAccessDetails {
	s := new(PaperDocRequestAccessDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocResolveCommentDetails : Paper doc comment resolved.
type PaperDocResolveCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewPaperDocResolveCommentDetails returns a new PaperDocResolveCommentDetails instance
func NewPaperDocResolveCommentDetails(EventUuid string) *PaperDocResolveCommentDetails {
	s := new(PaperDocResolveCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocRevertDetails : Restored a Paper doc to previous revision.
type PaperDocRevertDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocRevertDetails returns a new PaperDocRevertDetails instance
func NewPaperDocRevertDetails(EventUuid string) *PaperDocRevertDetails {
	s := new(PaperDocRevertDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocSlackShareDetails : Paper doc link shared via slack.
type PaperDocSlackShareDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocSlackShareDetails returns a new PaperDocSlackShareDetails instance
func NewPaperDocSlackShareDetails(EventUuid string) *PaperDocSlackShareDetails {
	s := new(PaperDocSlackShareDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocTeamInviteDetails : Paper doc shared with team member.
type PaperDocTeamInviteDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocTeamInviteDetails returns a new PaperDocTeamInviteDetails instance
func NewPaperDocTeamInviteDetails(EventUuid string) *PaperDocTeamInviteDetails {
	s := new(PaperDocTeamInviteDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocUnresolveCommentDetails : Unresolved a Paper doc comment.
type PaperDocUnresolveCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewPaperDocUnresolveCommentDetails returns a new PaperDocUnresolveCommentDetails instance
func NewPaperDocUnresolveCommentDetails(EventUuid string) *PaperDocUnresolveCommentDetails {
	s := new(PaperDocUnresolveCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocViewDetails : Viewed Paper doc.
type PaperDocViewDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocViewDetails returns a new PaperDocViewDetails instance
func NewPaperDocViewDetails(EventUuid string) *PaperDocViewDetails {
	s := new(PaperDocViewDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocumentLogInfo : Paper document's logged information.
type PaperDocumentLogInfo struct {
	// DocId : Papers document Id.
	DocId string `json:"doc_id"`
	// DocTitle : Paper document title.
	DocTitle string `json:"doc_title"`
}

// NewPaperDocumentLogInfo returns a new PaperDocumentLogInfo instance
func NewPaperDocumentLogInfo(DocId string, DocTitle string) *PaperDocumentLogInfo {
	s := new(PaperDocumentLogInfo)
	s.DocId = DocId
	s.DocTitle = DocTitle
	return s
}

// PaperDownloadFormat : has no documentation (yet)
type PaperDownloadFormat struct {
	dropbox.Tagged
}

// Valid tag values for PaperDownloadFormat
const (
	PaperDownloadFormatDocx     = "docx"
	PaperDownloadFormatHtml     = "html"
	PaperDownloadFormatMarkdown = "markdown"
	PaperDownloadFormatOther    = "other"
)

// PaperEnabledUsersGroupAdditionDetails : Users added to Paper enabled users
// list.
type PaperEnabledUsersGroupAdditionDetails struct {
}

// NewPaperEnabledUsersGroupAdditionDetails returns a new PaperEnabledUsersGroupAdditionDetails instance
func NewPaperEnabledUsersGroupAdditionDetails() *PaperEnabledUsersGroupAdditionDetails {
	s := new(PaperEnabledUsersGroupAdditionDetails)
	return s
}

// PaperEnabledUsersGroupRemovalDetails : Users removed from Paper enabled users
// list.
type PaperEnabledUsersGroupRemovalDetails struct {
}

// NewPaperEnabledUsersGroupRemovalDetails returns a new PaperEnabledUsersGroupRemovalDetails instance
func NewPaperEnabledUsersGroupRemovalDetails() *PaperEnabledUsersGroupRemovalDetails {
	s := new(PaperEnabledUsersGroupRemovalDetails)
	return s
}

// PaperExternalViewAllowDetails : Paper external sharing policy changed:
// anyone.
type PaperExternalViewAllowDetails struct {
}

// NewPaperExternalViewAllowDetails returns a new PaperExternalViewAllowDetails instance
func NewPaperExternalViewAllowDetails() *PaperExternalViewAllowDetails {
	s := new(PaperExternalViewAllowDetails)
	return s
}

// PaperExternalViewDefaultTeamDetails : Paper external sharing policy changed:
// default team.
type PaperExternalViewDefaultTeamDetails struct {
}

// NewPaperExternalViewDefaultTeamDetails returns a new PaperExternalViewDefaultTeamDetails instance
func NewPaperExternalViewDefaultTeamDetails() *PaperExternalViewDefaultTeamDetails {
	s := new(PaperExternalViewDefaultTeamDetails)
	return s
}

// PaperExternalViewForbidDetails : Paper external sharing policy changed:
// team-only.
type PaperExternalViewForbidDetails struct {
}

// NewPaperExternalViewForbidDetails returns a new PaperExternalViewForbidDetails instance
func NewPaperExternalViewForbidDetails() *PaperExternalViewForbidDetails {
	s := new(PaperExternalViewForbidDetails)
	return s
}

// PaperFolderDeletedDetails : Paper folder archived.
type PaperFolderDeletedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperFolderDeletedDetails returns a new PaperFolderDeletedDetails instance
func NewPaperFolderDeletedDetails(EventUuid string) *PaperFolderDeletedDetails {
	s := new(PaperFolderDeletedDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperFolderFollowedDetails : Followed a Paper folder.
type PaperFolderFollowedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperFolderFollowedDetails returns a new PaperFolderFollowedDetails instance
func NewPaperFolderFollowedDetails(EventUuid string) *PaperFolderFollowedDetails {
	s := new(PaperFolderFollowedDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperFolderLogInfo : Paper folder's logged information.
type PaperFolderLogInfo struct {
	// FolderId : Papers folder Id.
	FolderId string `json:"folder_id"`
	// FolderName : Paper folder name.
	FolderName string `json:"folder_name"`
}

// NewPaperFolderLogInfo returns a new PaperFolderLogInfo instance
func NewPaperFolderLogInfo(FolderId string, FolderName string) *PaperFolderLogInfo {
	s := new(PaperFolderLogInfo)
	s.FolderId = FolderId
	s.FolderName = FolderName
	return s
}

// PaperFolderTeamInviteDetails : Paper folder shared with team member.
type PaperFolderTeamInviteDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperFolderTeamInviteDetails returns a new PaperFolderTeamInviteDetails instance
func NewPaperFolderTeamInviteDetails(EventUuid string) *PaperFolderTeamInviteDetails {
	s := new(PaperFolderTeamInviteDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperMemberPolicy : Policy for controlling if team members can share Paper
// documents externally.
type PaperMemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperMemberPolicy
const (
	PaperMemberPolicyTeamOnly        = "team_only"
	PaperMemberPolicyDefaultTeamOnly = "default_team_only"
	PaperMemberPolicyDefaultAnyone   = "default_anyone"
	PaperMemberPolicyOther           = "other"
)

// PaperPolicy : Policy for enabling or disabling Dropbox Paper for the team.
type PaperPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperPolicy
const (
	PaperPolicyDisabled = "disabled"
	PaperPolicyEnabled  = "enabled"
	PaperPolicyOther    = "other"
)

// PaperTaggedValue : Paper tagged value.
type PaperTaggedValue struct {
	// Tag : Tag.
	Tag string `json:"tag"`
}

// NewPaperTaggedValue returns a new PaperTaggedValue instance
func NewPaperTaggedValue(Tag string) *PaperTaggedValue {
	s := new(PaperTaggedValue)
	s.Tag = Tag
	return s
}

// ParticipantLogInfo : A user or group
type ParticipantLogInfo struct {
	dropbox.Tagged
	// User : User details.
	User IsUserLogInfo `json:"user,omitempty"`
	// Group : Group details.
	Group *GroupLogInfo `json:"group,omitempty"`
}

// Valid tag values for ParticipantLogInfo
const (
	ParticipantLogInfoUser  = "user"
	ParticipantLogInfoGroup = "group"
	ParticipantLogInfoOther = "other"
)

// UnmarshalJSON deserializes into a ParticipantLogInfo instance
func (u *ParticipantLogInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// User : User details.
		User json.RawMessage `json:"user,omitempty"`
		// Group : Group details.
		Group json.RawMessage `json:"group,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "user":
		u.User, err = IsUserLogInfoFromJSON(body)

		if err != nil {
			return err
		}
	case "group":
		err = json.Unmarshal(body, &u.Group)

		if err != nil {
			return err
		}
	}
	return nil
}

// PasswordChangeDetails : Changed password.
type PasswordChangeDetails struct {
}

// NewPasswordChangeDetails returns a new PasswordChangeDetails instance
func NewPasswordChangeDetails() *PasswordChangeDetails {
	s := new(PasswordChangeDetails)
	return s
}

// PasswordLoginFailDetails : Failed to sign in using a password.
type PasswordLoginFailDetails struct {
	// ErrorDetails : Login failure details.
	ErrorDetails *FailureDetailsLogInfo `json:"error_details"`
}

// NewPasswordLoginFailDetails returns a new PasswordLoginFailDetails instance
func NewPasswordLoginFailDetails(ErrorDetails *FailureDetailsLogInfo) *PasswordLoginFailDetails {
	s := new(PasswordLoginFailDetails)
	s.ErrorDetails = ErrorDetails
	return s
}

// PasswordLoginSuccessDetails : Signed in using a password.
type PasswordLoginSuccessDetails struct {
}

// NewPasswordLoginSuccessDetails returns a new PasswordLoginSuccessDetails instance
func NewPasswordLoginSuccessDetails() *PasswordLoginSuccessDetails {
	s := new(PasswordLoginSuccessDetails)
	return s
}

// PasswordResetAllDetails : Reset all team member passwords.
type PasswordResetAllDetails struct {
}

// NewPasswordResetAllDetails returns a new PasswordResetAllDetails instance
func NewPasswordResetAllDetails() *PasswordResetAllDetails {
	s := new(PasswordResetAllDetails)
	return s
}

// PasswordResetDetails : Reset password.
type PasswordResetDetails struct {
}

// NewPasswordResetDetails returns a new PasswordResetDetails instance
func NewPasswordResetDetails() *PasswordResetDetails {
	s := new(PasswordResetDetails)
	return s
}

// PathLogInfo : Path's details.
type PathLogInfo struct {
	// Contextual : Fully qualified path relative to event's context. Might be
	// missing due to historical data gap.
	Contextual string `json:"contextual,omitempty"`
	// NamespaceRelative : Path relative to the namespace containing the
	// content.
	NamespaceRelative *NamespaceRelativePathLogInfo `json:"namespace_relative"`
}

// NewPathLogInfo returns a new PathLogInfo instance
func NewPathLogInfo(NamespaceRelative *NamespaceRelativePathLogInfo) *PathLogInfo {
	s := new(PathLogInfo)
	s.NamespaceRelative = NamespaceRelative
	return s
}

// PermanentDeleteChangePolicyDetails : Enabled or disabled the ability of team
// members to permanently delete content.
type PermanentDeleteChangePolicyDetails struct {
	// NewValue : New permanent delete content policy.
	NewValue *ContentPermanentDeletePolicy `json:"new_value"`
	// PreviousValue : Previous permanent delete content policy. Might be
	// missing due to historical data gap.
	PreviousValue *ContentPermanentDeletePolicy `json:"previous_value,omitempty"`
}

// NewPermanentDeleteChangePolicyDetails returns a new PermanentDeleteChangePolicyDetails instance
func NewPermanentDeleteChangePolicyDetails(NewValue *ContentPermanentDeletePolicy) *PermanentDeleteChangePolicyDetails {
	s := new(PermanentDeleteChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// PlacementRestriction : has no documentation (yet)
type PlacementRestriction struct {
	dropbox.Tagged
}

// Valid tag values for PlacementRestriction
const (
	PlacementRestrictionEuropeOnly = "europe_only"
	PlacementRestrictionNone       = "none"
	PlacementRestrictionOther      = "other"
)

// RelocateAssetReferencesLogInfo : Provides the indices of the source asset and
// the destination asset for a relocate action.
type RelocateAssetReferencesLogInfo struct {
	// SrcIndex : Source asset index.
	SrcIndex int64 `json:"src_index"`
	// DestIndex : Destination asset index.
	DestIndex int64 `json:"dest_index"`
}

// NewRelocateAssetReferencesLogInfo returns a new RelocateAssetReferencesLogInfo instance
func NewRelocateAssetReferencesLogInfo(SrcIndex int64, DestIndex int64) *RelocateAssetReferencesLogInfo {
	s := new(RelocateAssetReferencesLogInfo)
	s.SrcIndex = SrcIndex
	s.DestIndex = DestIndex
	return s
}

// RemoveLogoutUrlDetails : Removed single sign-on logout URL.
type RemoveLogoutUrlDetails struct {
	// PreviousValue : Previous single sign-on logout URL.
	PreviousValue string `json:"previous_value"`
	// NewValue : New single sign-on logout URL. Might be missing due to
	// historical data gap.
	NewValue string `json:"new_value,omitempty"`
}

// NewRemoveLogoutUrlDetails returns a new RemoveLogoutUrlDetails instance
func NewRemoveLogoutUrlDetails(PreviousValue string) *RemoveLogoutUrlDetails {
	s := new(RemoveLogoutUrlDetails)
	s.PreviousValue = PreviousValue
	return s
}

// RemoveSsoUrlDetails : Changed the sign-out URL for SSO.
type RemoveSsoUrlDetails struct {
	// PreviousValue : Previous single sign-on logout URL.
	PreviousValue string `json:"previous_value"`
}

// NewRemoveSsoUrlDetails returns a new RemoveSsoUrlDetails instance
func NewRemoveSsoUrlDetails(PreviousValue string) *RemoveSsoUrlDetails {
	s := new(RemoveSsoUrlDetails)
	s.PreviousValue = PreviousValue
	return s
}

// ResellerLogInfo : Reseller information.
type ResellerLogInfo struct {
	// ResellerName : Reseller name.
	ResellerName string `json:"reseller_name"`
	// ResellerId : Reseller ID.
	ResellerId string `json:"reseller_id"`
}

// NewResellerLogInfo returns a new ResellerLogInfo instance
func NewResellerLogInfo(ResellerName string, ResellerId string) *ResellerLogInfo {
	s := new(ResellerLogInfo)
	s.ResellerName = ResellerName
	s.ResellerId = ResellerId
	return s
}

// ResellerSupportSessionEndDetails : Ended reseller support session.
type ResellerSupportSessionEndDetails struct {
}

// NewResellerSupportSessionEndDetails returns a new ResellerSupportSessionEndDetails instance
func NewResellerSupportSessionEndDetails() *ResellerSupportSessionEndDetails {
	s := new(ResellerSupportSessionEndDetails)
	return s
}

// ResellerSupportSessionStartDetails : Started reseller support session.
type ResellerSupportSessionStartDetails struct {
}

// NewResellerSupportSessionStartDetails returns a new ResellerSupportSessionStartDetails instance
func NewResellerSupportSessionStartDetails() *ResellerSupportSessionStartDetails {
	s := new(ResellerSupportSessionStartDetails)
	return s
}

// SfAddGroupDetails : Added the team to a shared folder.
type SfAddGroupDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TeamName : Team name.
	TeamName string `json:"team_name"`
}

// NewSfAddGroupDetails returns a new SfAddGroupDetails instance
func NewSfAddGroupDetails(TargetIndex int64, OriginalFolderName string, TeamName string) *SfAddGroupDetails {
	s := new(SfAddGroupDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	s.TeamName = TeamName
	return s
}

// SfAllowNonMembersToViewSharedLinksDetails : Allowed non collaborators to view
// links to files in a shared folder.
type SfAllowNonMembersToViewSharedLinksDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSfAllowNonMembersToViewSharedLinksDetails returns a new SfAllowNonMembersToViewSharedLinksDetails instance
func NewSfAllowNonMembersToViewSharedLinksDetails(TargetIndex int64, OriginalFolderName string) *SfAllowNonMembersToViewSharedLinksDetails {
	s := new(SfAllowNonMembersToViewSharedLinksDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfExternalInviteWarnDetails : Admin settings: team members see a warning
// before sharing folders outside the team (DEPRECATED FEATURE).
type SfExternalInviteWarnDetails struct {
}

// NewSfExternalInviteWarnDetails returns a new SfExternalInviteWarnDetails instance
func NewSfExternalInviteWarnDetails() *SfExternalInviteWarnDetails {
	s := new(SfExternalInviteWarnDetails)
	return s
}

// SfInviteGroupDetails : Invited a group to a shared folder.
type SfInviteGroupDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
}

// NewSfInviteGroupDetails returns a new SfInviteGroupDetails instance
func NewSfInviteGroupDetails(TargetIndex int64) *SfInviteGroupDetails {
	s := new(SfInviteGroupDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SfNestDetails : Changed parent of shared folder.
type SfNestDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// PrevParentNsId : Previous parent namespace ID. Might be missing due to
	// historical data gap.
	PrevParentNsId string `json:"prev_parent_ns_id,omitempty"`
	// NewParentNsId : New parent namespace ID. Might be missing due to
	// historical data gap.
	NewParentNsId string `json:"new_parent_ns_id,omitempty"`
}

// NewSfNestDetails returns a new SfNestDetails instance
func NewSfNestDetails(TargetIndex int64, OriginalFolderName string) *SfNestDetails {
	s := new(SfNestDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamDeclineDetails : Declined a team member's invitation to a shared
// folder.
type SfTeamDeclineDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamDeclineDetails returns a new SfTeamDeclineDetails instance
func NewSfTeamDeclineDetails(TargetIndex int64, OriginalFolderName string) *SfTeamDeclineDetails {
	s := new(SfTeamDeclineDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamGrantAccessDetails : Granted access to a shared folder.
type SfTeamGrantAccessDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamGrantAccessDetails returns a new SfTeamGrantAccessDetails instance
func NewSfTeamGrantAccessDetails(TargetIndex int64, OriginalFolderName string) *SfTeamGrantAccessDetails {
	s := new(SfTeamGrantAccessDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamInviteChangeRoleDetails : Changed a team member's role in a shared
// folder.
type SfTeamInviteChangeRoleDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// NewSharingPermission : New sharing permission. Might be missing due to
	// historical data gap.
	NewSharingPermission string `json:"new_sharing_permission,omitempty"`
	// PreviousSharingPermission : Previous sharing permission. Might be missing
	// due to historical data gap.
	PreviousSharingPermission string `json:"previous_sharing_permission,omitempty"`
}

// NewSfTeamInviteChangeRoleDetails returns a new SfTeamInviteChangeRoleDetails instance
func NewSfTeamInviteChangeRoleDetails(TargetIndex int64, OriginalFolderName string) *SfTeamInviteChangeRoleDetails {
	s := new(SfTeamInviteChangeRoleDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamInviteDetails : Invited team members to a shared folder.
type SfTeamInviteDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
}

// NewSfTeamInviteDetails returns a new SfTeamInviteDetails instance
func NewSfTeamInviteDetails(TargetIndex int64, OriginalFolderName string) *SfTeamInviteDetails {
	s := new(SfTeamInviteDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamJoinDetails : Joined a team member's shared folder.
type SfTeamJoinDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamJoinDetails returns a new SfTeamJoinDetails instance
func NewSfTeamJoinDetails(TargetIndex int64, OriginalFolderName string) *SfTeamJoinDetails {
	s := new(SfTeamJoinDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamJoinFromOobLinkDetails : Joined a team member's shared folder from a
// link.
type SfTeamJoinFromOobLinkDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// TokenKey : Shared link token key.
	TokenKey string `json:"token_key,omitempty"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
}

// NewSfTeamJoinFromOobLinkDetails returns a new SfTeamJoinFromOobLinkDetails instance
func NewSfTeamJoinFromOobLinkDetails(TargetIndex int64, OriginalFolderName string) *SfTeamJoinFromOobLinkDetails {
	s := new(SfTeamJoinFromOobLinkDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamUninviteDetails : Unshared a folder with a team member.
type SfTeamUninviteDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamUninviteDetails returns a new SfTeamUninviteDetails instance
func NewSfTeamUninviteDetails(TargetIndex int64, OriginalFolderName string) *SfTeamUninviteDetails {
	s := new(SfTeamUninviteDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedContentAddInviteesDetails : Sent an email invitation to the membership
// of a shared file or folder.
type SharedContentAddInviteesDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
}

// NewSharedContentAddInviteesDetails returns a new SharedContentAddInviteesDetails instance
func NewSharedContentAddInviteesDetails(TargetIndex int64) *SharedContentAddInviteesDetails {
	s := new(SharedContentAddInviteesDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentAddLinkExpiryDetails : Added an expiry to the link for the
// shared file or folder.
type SharedContentAddLinkExpiryDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// ExpirationStartDate : Expiration starting date.
	ExpirationStartDate string `json:"expiration_start_date"`
	// ExpirationDays : The number of days from the starting expiration date
	// after which the link will expire.
	ExpirationDays int64 `json:"expiration_days"`
}

// NewSharedContentAddLinkExpiryDetails returns a new SharedContentAddLinkExpiryDetails instance
func NewSharedContentAddLinkExpiryDetails(TargetIndex int64, ExpirationStartDate string, ExpirationDays int64) *SharedContentAddLinkExpiryDetails {
	s := new(SharedContentAddLinkExpiryDetails)
	s.TargetIndex = TargetIndex
	s.ExpirationStartDate = ExpirationStartDate
	s.ExpirationDays = ExpirationDays
	return s
}

// SharedContentAddLinkPasswordDetails : Added a password to the link for the
// shared file or folder.
type SharedContentAddLinkPasswordDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentAddLinkPasswordDetails returns a new SharedContentAddLinkPasswordDetails instance
func NewSharedContentAddLinkPasswordDetails(TargetIndex int64) *SharedContentAddLinkPasswordDetails {
	s := new(SharedContentAddLinkPasswordDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentAddMemberDetails : Added users and/or groups to the membership
// of a shared file or folder.
type SharedContentAddMemberDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentAddMemberDetails returns a new SharedContentAddMemberDetails instance
func NewSharedContentAddMemberDetails(TargetIndex int64) *SharedContentAddMemberDetails {
	s := new(SharedContentAddMemberDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentChangeDownloadsPolicyDetails : Changed whether members can
// download the shared file or folder.
type SharedContentChangeDownloadsPolicyDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// NewValue : New downlaod policy.
	NewValue *SharedContentDownloadsPolicy `json:"new_value"`
	// PreviousValue : Previous downlaod policy. Might be missing due to
	// historical data gap.
	PreviousValue *SharedContentDownloadsPolicy `json:"previous_value,omitempty"`
}

// NewSharedContentChangeDownloadsPolicyDetails returns a new SharedContentChangeDownloadsPolicyDetails instance
func NewSharedContentChangeDownloadsPolicyDetails(TargetIndex int64, NewValue *SharedContentDownloadsPolicy) *SharedContentChangeDownloadsPolicyDetails {
	s := new(SharedContentChangeDownloadsPolicyDetails)
	s.TargetIndex = TargetIndex
	s.NewValue = NewValue
	return s
}

// SharedContentChangeInviteeRoleDetails : Changed the access type of an invitee
// to a shared file or folder before the invitation was claimed.
type SharedContentChangeInviteeRoleDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// NewSharingPermission : New sharing permission. Might be missing due to
	// historical data gap.
	NewSharingPermission string `json:"new_sharing_permission,omitempty"`
	// PreviousSharingPermission : Previous sharing permission. Might be missing
	// due to historical data gap.
	PreviousSharingPermission string `json:"previous_sharing_permission,omitempty"`
}

// NewSharedContentChangeInviteeRoleDetails returns a new SharedContentChangeInviteeRoleDetails instance
func NewSharedContentChangeInviteeRoleDetails(TargetIndex int64, OriginalFolderName string) *SharedContentChangeInviteeRoleDetails {
	s := new(SharedContentChangeInviteeRoleDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedContentChangeLinkAudienceDetails : Changed the audience of the link for
// a shared file or folder.
type SharedContentChangeLinkAudienceDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// NewValue : New link audience value.
	NewValue *LinkAudience `json:"new_value"`
	// PreviousValue : Previous link audience value. Might be missing due to
	// historical data gap.
	PreviousValue *LinkAudience `json:"previous_value,omitempty"`
}

// NewSharedContentChangeLinkAudienceDetails returns a new SharedContentChangeLinkAudienceDetails instance
func NewSharedContentChangeLinkAudienceDetails(TargetIndex int64, NewValue *LinkAudience) *SharedContentChangeLinkAudienceDetails {
	s := new(SharedContentChangeLinkAudienceDetails)
	s.TargetIndex = TargetIndex
	s.NewValue = NewValue
	return s
}

// SharedContentChangeLinkExpiryDetails : Changed the expiry of the link for the
// shared file or folder.
type SharedContentChangeLinkExpiryDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// ExpirationStartDate : Expiration starting date.
	ExpirationStartDate string `json:"expiration_start_date"`
	// ExpirationDays : The number of days from the starting expiration date
	// after which the link will expire.
	ExpirationDays int64 `json:"expiration_days"`
}

// NewSharedContentChangeLinkExpiryDetails returns a new SharedContentChangeLinkExpiryDetails instance
func NewSharedContentChangeLinkExpiryDetails(TargetIndex int64, ExpirationStartDate string, ExpirationDays int64) *SharedContentChangeLinkExpiryDetails {
	s := new(SharedContentChangeLinkExpiryDetails)
	s.TargetIndex = TargetIndex
	s.ExpirationStartDate = ExpirationStartDate
	s.ExpirationDays = ExpirationDays
	return s
}

// SharedContentChangeLinkPasswordDetails : Changed the password on the link for
// the shared file or folder.
type SharedContentChangeLinkPasswordDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentChangeLinkPasswordDetails returns a new SharedContentChangeLinkPasswordDetails instance
func NewSharedContentChangeLinkPasswordDetails(TargetIndex int64) *SharedContentChangeLinkPasswordDetails {
	s := new(SharedContentChangeLinkPasswordDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentChangeMemberRoleDetails : Changed the access type of a shared
// file or folder member.
type SharedContentChangeMemberRoleDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// NewSharingPermission : New sharing permission. Might be missing due to
	// historical data gap.
	NewSharingPermission string `json:"new_sharing_permission,omitempty"`
	// PreviousSharingPermission : Previous sharing permission. Might be missing
	// due to historical data gap.
	PreviousSharingPermission string `json:"previous_sharing_permission,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentChangeMemberRoleDetails returns a new SharedContentChangeMemberRoleDetails instance
func NewSharedContentChangeMemberRoleDetails(TargetIndex int64) *SharedContentChangeMemberRoleDetails {
	s := new(SharedContentChangeMemberRoleDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentChangeViewerInfoPolicyDetails : Changed whether members can see
// who viewed the shared file or folder.
type SharedContentChangeViewerInfoPolicyDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// NewValue : New viewer info policy.
	NewValue *SharedContentViewerInfoPolicy `json:"new_value"`
	// PreviousValue : Previous view info policy. Might be missing due to
	// historical data gap.
	PreviousValue *SharedContentViewerInfoPolicy `json:"previous_value,omitempty"`
}

// NewSharedContentChangeViewerInfoPolicyDetails returns a new SharedContentChangeViewerInfoPolicyDetails instance
func NewSharedContentChangeViewerInfoPolicyDetails(TargetIndex int64, NewValue *SharedContentViewerInfoPolicy) *SharedContentChangeViewerInfoPolicyDetails {
	s := new(SharedContentChangeViewerInfoPolicyDetails)
	s.TargetIndex = TargetIndex
	s.NewValue = NewValue
	return s
}

// SharedContentClaimInvitationDetails : Claimed membership to a team member's
// shared folder.
type SharedContentClaimInvitationDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link,omitempty"`
}

// NewSharedContentClaimInvitationDetails returns a new SharedContentClaimInvitationDetails instance
func NewSharedContentClaimInvitationDetails(TargetIndex int64) *SharedContentClaimInvitationDetails {
	s := new(SharedContentClaimInvitationDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentCopyDetails : Copied the shared file or folder to own Dropbox.
type SharedContentCopyDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// RelocateActionDetails : Specifies the source and destination indices in
	// the assets list.
	RelocateActionDetails *RelocateAssetReferencesLogInfo `json:"relocate_action_details"`
}

// NewSharedContentCopyDetails returns a new SharedContentCopyDetails instance
func NewSharedContentCopyDetails(SharedContentLink string, TargetIndex int64, RelocateActionDetails *RelocateAssetReferencesLogInfo) *SharedContentCopyDetails {
	s := new(SharedContentCopyDetails)
	s.SharedContentLink = SharedContentLink
	s.TargetIndex = TargetIndex
	s.RelocateActionDetails = RelocateActionDetails
	return s
}

// SharedContentDownloadDetails : Downloaded the shared file or folder.
type SharedContentDownloadDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
}

// NewSharedContentDownloadDetails returns a new SharedContentDownloadDetails instance
func NewSharedContentDownloadDetails(SharedContentLink string, TargetIndex int64) *SharedContentDownloadDetails {
	s := new(SharedContentDownloadDetails)
	s.SharedContentLink = SharedContentLink
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentDownloadsPolicy : Shared content downloads policy
type SharedContentDownloadsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedContentDownloadsPolicy
const (
	SharedContentDownloadsPolicyDisabled = "disabled"
	SharedContentDownloadsPolicyEnabled  = "enabled"
	SharedContentDownloadsPolicyOther    = "other"
)

// SharedContentRelinquishMembershipDetails : Left the membership of a shared
// file or folder.
type SharedContentRelinquishMembershipDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSharedContentRelinquishMembershipDetails returns a new SharedContentRelinquishMembershipDetails instance
func NewSharedContentRelinquishMembershipDetails(TargetIndex int64, OriginalFolderName string) *SharedContentRelinquishMembershipDetails {
	s := new(SharedContentRelinquishMembershipDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedContentRemoveInviteeDetails : Removed an invitee from the membership of
// a shared file or folder before it was claimed.
type SharedContentRemoveInviteeDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSharedContentRemoveInviteeDetails returns a new SharedContentRemoveInviteeDetails instance
func NewSharedContentRemoveInviteeDetails(TargetIndex int64, OriginalFolderName string) *SharedContentRemoveInviteeDetails {
	s := new(SharedContentRemoveInviteeDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedContentRemoveLinkExpiryDetails : Removed the expiry of the link for the
// shared file or folder.
type SharedContentRemoveLinkExpiryDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentRemoveLinkExpiryDetails returns a new SharedContentRemoveLinkExpiryDetails instance
func NewSharedContentRemoveLinkExpiryDetails(TargetIndex int64) *SharedContentRemoveLinkExpiryDetails {
	s := new(SharedContentRemoveLinkExpiryDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentRemoveLinkPasswordDetails : Removed the password on the link for
// the shared file or folder.
type SharedContentRemoveLinkPasswordDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentRemoveLinkPasswordDetails returns a new SharedContentRemoveLinkPasswordDetails instance
func NewSharedContentRemoveLinkPasswordDetails(TargetIndex int64) *SharedContentRemoveLinkPasswordDetails {
	s := new(SharedContentRemoveLinkPasswordDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentRemoveMemberDetails : Removed a user or a group from the
// membership of a shared file or folder.
type SharedContentRemoveMemberDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSharedContentRemoveMemberDetails returns a new SharedContentRemoveMemberDetails instance
func NewSharedContentRemoveMemberDetails(TargetIndex int64) *SharedContentRemoveMemberDetails {
	s := new(SharedContentRemoveMemberDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentRequestAccessDetails : Requested to be on the membership of a
// shared file or folder.
type SharedContentRequestAccessDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link,omitempty"`
}

// NewSharedContentRequestAccessDetails returns a new SharedContentRequestAccessDetails instance
func NewSharedContentRequestAccessDetails(TargetIndex int64) *SharedContentRequestAccessDetails {
	s := new(SharedContentRequestAccessDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentUnshareDetails : Unshared a shared file or folder by clearing
// its membership and turning off its link.
type SharedContentUnshareDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name,omitempty"`
}

// NewSharedContentUnshareDetails returns a new SharedContentUnshareDetails instance
func NewSharedContentUnshareDetails(TargetIndex int64) *SharedContentUnshareDetails {
	s := new(SharedContentUnshareDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentViewDetails : Previewed the shared file or folder.
type SharedContentViewDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
}

// NewSharedContentViewDetails returns a new SharedContentViewDetails instance
func NewSharedContentViewDetails(SharedContentLink string, TargetIndex int64) *SharedContentViewDetails {
	s := new(SharedContentViewDetails)
	s.SharedContentLink = SharedContentLink
	s.TargetIndex = TargetIndex
	return s
}

// SharedContentViewerInfoPolicy : Shared content viewer info policy
type SharedContentViewerInfoPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedContentViewerInfoPolicy
const (
	SharedContentViewerInfoPolicyDisabled = "disabled"
	SharedContentViewerInfoPolicyEnabled  = "enabled"
	SharedContentViewerInfoPolicyOther    = "other"
)

// SharedFolderChangeConfidentialityDetails : Set or unset the confidential flag
// on a shared folder.
type SharedFolderChangeConfidentialityDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// NewValue : New confidentiality value.
	NewValue *Confidentiality `json:"new_value"`
	// PreviousValue : Previous confidentiality value. Might be missing due to
	// historical data gap.
	PreviousValue *Confidentiality `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeConfidentialityDetails returns a new SharedFolderChangeConfidentialityDetails instance
func NewSharedFolderChangeConfidentialityDetails(TargetIndex int64, OriginalFolderName string, NewValue *Confidentiality) *SharedFolderChangeConfidentialityDetails {
	s := new(SharedFolderChangeConfidentialityDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeLinkPolicyDetails : Changed who can access the shared
// folder via a link.
type SharedFolderChangeLinkPolicyDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// NewValue : New shared folder link policy.
	NewValue *SharedFolderLinkPolicy `json:"new_value"`
	// PreviousValue : Previous shared folder link policy. Might be missing due
	// to historical data gap.
	PreviousValue *SharedFolderLinkPolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeLinkPolicyDetails returns a new SharedFolderChangeLinkPolicyDetails instance
func NewSharedFolderChangeLinkPolicyDetails(TargetIndex int64, OriginalFolderName string, NewValue *SharedFolderLinkPolicy) *SharedFolderChangeLinkPolicyDetails {
	s := new(SharedFolderChangeLinkPolicyDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeMemberManagementPolicyDetails : Changed who can manage the
// membership of a shared folder.
type SharedFolderChangeMemberManagementPolicyDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// NewValue : New membership management policy.
	NewValue *SharedFolderMembershipManagementPolicy `json:"new_value"`
	// PreviousValue : Previous membership management policy. Might be missing
	// due to historical data gap.
	PreviousValue *SharedFolderMembershipManagementPolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeMemberManagementPolicyDetails returns a new SharedFolderChangeMemberManagementPolicyDetails instance
func NewSharedFolderChangeMemberManagementPolicyDetails(TargetIndex int64, OriginalFolderName string, NewValue *SharedFolderMembershipManagementPolicy) *SharedFolderChangeMemberManagementPolicyDetails {
	s := new(SharedFolderChangeMemberManagementPolicyDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeMemberPolicyDetails : Changed who can become a member of
// the shared folder.
type SharedFolderChangeMemberPolicyDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
	// NewValue : New external invite policy.
	NewValue *SharedFolderMemberPolicy `json:"new_value"`
	// PreviousValue : Previous external invite policy. Might be missing due to
	// historical data gap.
	PreviousValue *SharedFolderMemberPolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeMemberPolicyDetails returns a new SharedFolderChangeMemberPolicyDetails instance
func NewSharedFolderChangeMemberPolicyDetails(TargetIndex int64, OriginalFolderName string, NewValue *SharedFolderMemberPolicy) *SharedFolderChangeMemberPolicyDetails {
	s := new(SharedFolderChangeMemberPolicyDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	s.NewValue = NewValue
	return s
}

// SharedFolderCreateDetails : Created a shared folder.
type SharedFolderCreateDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// ParentNsId : Parent namespace ID. Might be missing due to historical data
	// gap.
	ParentNsId string `json:"parent_ns_id,omitempty"`
}

// NewSharedFolderCreateDetails returns a new SharedFolderCreateDetails instance
func NewSharedFolderCreateDetails(TargetIndex int64) *SharedFolderCreateDetails {
	s := new(SharedFolderCreateDetails)
	s.TargetIndex = TargetIndex
	return s
}

// SharedFolderLinkPolicy : has no documentation (yet)
type SharedFolderLinkPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderLinkPolicy
const (
	SharedFolderLinkPolicyMembersOnly    = "members_only"
	SharedFolderLinkPolicyMembersAndTeam = "members_and_team"
	SharedFolderLinkPolicyAnyone         = "anyone"
	SharedFolderLinkPolicyOther          = "other"
)

// SharedFolderMemberPolicy : Policy for controlling who can become a member of
// a shared folder
type SharedFolderMemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderMemberPolicy
const (
	SharedFolderMemberPolicyTeamOnly = "team_only"
	SharedFolderMemberPolicyAnyone   = "anyone"
	SharedFolderMemberPolicyOther    = "other"
)

// SharedFolderMembershipManagementPolicy : has no documentation (yet)
type SharedFolderMembershipManagementPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderMembershipManagementPolicy
const (
	SharedFolderMembershipManagementPolicyOwner   = "owner"
	SharedFolderMembershipManagementPolicyEditors = "editors"
	SharedFolderMembershipManagementPolicyOther   = "other"
)

// SharedFolderMountDetails : Added a shared folder to own Dropbox.
type SharedFolderMountDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSharedFolderMountDetails returns a new SharedFolderMountDetails instance
func NewSharedFolderMountDetails(TargetIndex int64, OriginalFolderName string) *SharedFolderMountDetails {
	s := new(SharedFolderMountDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedFolderTransferOwnershipDetails : Transferred the ownership of a shared
// folder to another member.
type SharedFolderTransferOwnershipDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSharedFolderTransferOwnershipDetails returns a new SharedFolderTransferOwnershipDetails instance
func NewSharedFolderTransferOwnershipDetails(TargetIndex int64, OriginalFolderName string) *SharedFolderTransferOwnershipDetails {
	s := new(SharedFolderTransferOwnershipDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedFolderUnmountDetails : Deleted a shared folder from Dropbox.
type SharedFolderUnmountDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSharedFolderUnmountDetails returns a new SharedFolderUnmountDetails instance
func NewSharedFolderUnmountDetails(TargetIndex int64, OriginalFolderName string) *SharedFolderUnmountDetails {
	s := new(SharedFolderUnmountDetails)
	s.TargetIndex = TargetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SharedNoteOpenedDetails : Shared Paper document was opened.
type SharedNoteOpenedDetails struct {
}

// NewSharedNoteOpenedDetails returns a new SharedNoteOpenedDetails instance
func NewSharedNoteOpenedDetails() *SharedNoteOpenedDetails {
	s := new(SharedNoteOpenedDetails)
	return s
}

// SharingChangeFolderJoinPolicyDetails : Changed whether team members can join
// shared folders owned externally (i.e. outside the team).
type SharingChangeFolderJoinPolicyDetails struct {
	// NewValue : New external join policy.
	NewValue *SharingFolderJoinPolicy `json:"new_value"`
	// PreviousValue : Previous external join policy. Might be missing due to
	// historical data gap.
	PreviousValue *SharingFolderJoinPolicy `json:"previous_value,omitempty"`
}

// NewSharingChangeFolderJoinPolicyDetails returns a new SharingChangeFolderJoinPolicyDetails instance
func NewSharingChangeFolderJoinPolicyDetails(NewValue *SharingFolderJoinPolicy) *SharingChangeFolderJoinPolicyDetails {
	s := new(SharingChangeFolderJoinPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharingChangeLinkPolicyDetails : Changed whether team members can share links
// externally (i.e. outside the team), and if so, whether links should be
// accessible only by team members or anyone by default.
type SharingChangeLinkPolicyDetails struct {
	// NewValue : New external link accessibility policy.
	NewValue *SharingLinkPolicy `json:"new_value"`
	// PreviousValue : Previous external link accessibility policy. Might be
	// missing due to historical data gap.
	PreviousValue *SharingLinkPolicy `json:"previous_value,omitempty"`
}

// NewSharingChangeLinkPolicyDetails returns a new SharingChangeLinkPolicyDetails instance
func NewSharingChangeLinkPolicyDetails(NewValue *SharingLinkPolicy) *SharingChangeLinkPolicyDetails {
	s := new(SharingChangeLinkPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharingChangeMemberPolicyDetails : Changed whether team members can share
// files and folders externally (i.e. outside the team).
type SharingChangeMemberPolicyDetails struct {
	// NewValue : New external invite policy.
	NewValue *SharingMemberPolicy `json:"new_value"`
	// PreviousValue : Previous external invite policy. Might be missing due to
	// historical data gap.
	PreviousValue *SharingMemberPolicy `json:"previous_value,omitempty"`
}

// NewSharingChangeMemberPolicyDetails returns a new SharingChangeMemberPolicyDetails instance
func NewSharingChangeMemberPolicyDetails(NewValue *SharingMemberPolicy) *SharingChangeMemberPolicyDetails {
	s := new(SharingChangeMemberPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharingFolderJoinPolicy : Policy for controlling if team members can join
// shared folders owned by non team members.
type SharingFolderJoinPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharingFolderJoinPolicy
const (
	SharingFolderJoinPolicyTeamOnly = "team_only"
	SharingFolderJoinPolicyAnyone   = "anyone"
	SharingFolderJoinPolicyOther    = "other"
)

// SharingLinkPolicy : Policy for controlling if team members can share links
// externally
type SharingLinkPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharingLinkPolicy
const (
	SharingLinkPolicyTeamOnly        = "team_only"
	SharingLinkPolicyDefaultTeamOnly = "default_team_only"
	SharingLinkPolicyDefaultAnyone   = "default_anyone"
	SharingLinkPolicyOther           = "other"
)

// SharingMemberPolicy : External sharing policy
type SharingMemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharingMemberPolicy
const (
	SharingMemberPolicyTeamOnly = "team_only"
	SharingMemberPolicyAnyone   = "anyone"
	SharingMemberPolicyOther    = "other"
)

// ShmodelAppCreateDetails : Created a link to a file using an app.
type ShmodelAppCreateDetails struct {
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TokenKey : Shared link token key.
	TokenKey string `json:"token_key,omitempty"`
}

// NewShmodelAppCreateDetails returns a new ShmodelAppCreateDetails instance
func NewShmodelAppCreateDetails() *ShmodelAppCreateDetails {
	s := new(ShmodelAppCreateDetails)
	return s
}

// ShmodelCreateDetails : Created a new link.
type ShmodelCreateDetails struct {
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TokenKey : Shared link token key.
	TokenKey string `json:"token_key,omitempty"`
}

// NewShmodelCreateDetails returns a new ShmodelCreateDetails instance
func NewShmodelCreateDetails() *ShmodelCreateDetails {
	s := new(ShmodelCreateDetails)
	return s
}

// ShmodelDisableDetails : Removed a link.
type ShmodelDisableDetails struct {
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TokenKey : Shared link token key.
	TokenKey string `json:"token_key,omitempty"`
}

// NewShmodelDisableDetails returns a new ShmodelDisableDetails instance
func NewShmodelDisableDetails() *ShmodelDisableDetails {
	s := new(ShmodelDisableDetails)
	return s
}

// ShmodelFbShareDetails : Shared a link with Facebook users.
type ShmodelFbShareDetails struct {
	// SharingNonMemberRecipients : Sharing non member recipients.
	SharingNonMemberRecipients []*NonTeamMemberLogInfo `json:"sharing_non_member_recipients"`
}

// NewShmodelFbShareDetails returns a new ShmodelFbShareDetails instance
func NewShmodelFbShareDetails(SharingNonMemberRecipients []*NonTeamMemberLogInfo) *ShmodelFbShareDetails {
	s := new(ShmodelFbShareDetails)
	s.SharingNonMemberRecipients = SharingNonMemberRecipients
	return s
}

// ShmodelGroupShareDetails : Shared a link with a group.
type ShmodelGroupShareDetails struct {
}

// NewShmodelGroupShareDetails returns a new ShmodelGroupShareDetails instance
func NewShmodelGroupShareDetails() *ShmodelGroupShareDetails {
	s := new(ShmodelGroupShareDetails)
	return s
}

// ShmodelRemoveExpirationDetails : Removed the expiration date from a link.
type ShmodelRemoveExpirationDetails struct {
}

// NewShmodelRemoveExpirationDetails returns a new ShmodelRemoveExpirationDetails instance
func NewShmodelRemoveExpirationDetails() *ShmodelRemoveExpirationDetails {
	s := new(ShmodelRemoveExpirationDetails)
	return s
}

// ShmodelSetExpirationDetails : Added an expiration date to a link.
type ShmodelSetExpirationDetails struct {
	// ExpirationStartDate : Expiration starting date.
	ExpirationStartDate string `json:"expiration_start_date"`
	// ExpirationDays : The number of days from the starting expiration date
	// after which the link will expire.
	ExpirationDays int64 `json:"expiration_days"`
}

// NewShmodelSetExpirationDetails returns a new ShmodelSetExpirationDetails instance
func NewShmodelSetExpirationDetails(ExpirationStartDate string, ExpirationDays int64) *ShmodelSetExpirationDetails {
	s := new(ShmodelSetExpirationDetails)
	s.ExpirationStartDate = ExpirationStartDate
	s.ExpirationDays = ExpirationDays
	return s
}

// ShmodelTeamCopyDetails : Added a team member's file/folder to their Dropbox
// from a link.
type ShmodelTeamCopyDetails struct {
}

// NewShmodelTeamCopyDetails returns a new ShmodelTeamCopyDetails instance
func NewShmodelTeamCopyDetails() *ShmodelTeamCopyDetails {
	s := new(ShmodelTeamCopyDetails)
	return s
}

// ShmodelTeamDownloadDetails : Downloaded a team member's file/folder from a
// link.
type ShmodelTeamDownloadDetails struct {
}

// NewShmodelTeamDownloadDetails returns a new ShmodelTeamDownloadDetails instance
func NewShmodelTeamDownloadDetails() *ShmodelTeamDownloadDetails {
	s := new(ShmodelTeamDownloadDetails)
	return s
}

// ShmodelTeamShareDetails : Shared a link with team members.
type ShmodelTeamShareDetails struct {
}

// NewShmodelTeamShareDetails returns a new ShmodelTeamShareDetails instance
func NewShmodelTeamShareDetails() *ShmodelTeamShareDetails {
	s := new(ShmodelTeamShareDetails)
	return s
}

// ShmodelTeamViewDetails : Opened a team member's link.
type ShmodelTeamViewDetails struct {
}

// NewShmodelTeamViewDetails returns a new ShmodelTeamViewDetails instance
func NewShmodelTeamViewDetails() *ShmodelTeamViewDetails {
	s := new(ShmodelTeamViewDetails)
	return s
}

// ShmodelVisibilityPasswordDetails : Password-protected a link.
type ShmodelVisibilityPasswordDetails struct {
}

// NewShmodelVisibilityPasswordDetails returns a new ShmodelVisibilityPasswordDetails instance
func NewShmodelVisibilityPasswordDetails() *ShmodelVisibilityPasswordDetails {
	s := new(ShmodelVisibilityPasswordDetails)
	return s
}

// ShmodelVisibilityPublicDetails : Made a file/folder visible to anyone with
// the link.
type ShmodelVisibilityPublicDetails struct {
}

// NewShmodelVisibilityPublicDetails returns a new ShmodelVisibilityPublicDetails instance
func NewShmodelVisibilityPublicDetails() *ShmodelVisibilityPublicDetails {
	s := new(ShmodelVisibilityPublicDetails)
	return s
}

// ShmodelVisibilityTeamOnlyDetails : Made a file/folder visible only to team
// members with the link.
type ShmodelVisibilityTeamOnlyDetails struct {
}

// NewShmodelVisibilityTeamOnlyDetails returns a new ShmodelVisibilityTeamOnlyDetails instance
func NewShmodelVisibilityTeamOnlyDetails() *ShmodelVisibilityTeamOnlyDetails {
	s := new(ShmodelVisibilityTeamOnlyDetails)
	return s
}

// SignInAsSessionEndDetails : Ended admin sign-in-as session.
type SignInAsSessionEndDetails struct {
}

// NewSignInAsSessionEndDetails returns a new SignInAsSessionEndDetails instance
func NewSignInAsSessionEndDetails() *SignInAsSessionEndDetails {
	s := new(SignInAsSessionEndDetails)
	return s
}

// SignInAsSessionStartDetails : Started admin sign-in-as session.
type SignInAsSessionStartDetails struct {
}

// NewSignInAsSessionStartDetails returns a new SignInAsSessionStartDetails instance
func NewSignInAsSessionStartDetails() *SignInAsSessionStartDetails {
	s := new(SignInAsSessionStartDetails)
	return s
}

// SmartSyncChangePolicyDetails : Changed the default Smart Sync policy for team
// members.
type SmartSyncChangePolicyDetails struct {
	// NewValue : New smart sync policy.
	NewValue *SmartSyncPolicy `json:"new_value"`
	// PreviousValue : Previous smart sync policy. Might be missing due to
	// historical data gap.
	PreviousValue *SmartSyncPolicy `json:"previous_value,omitempty"`
}

// NewSmartSyncChangePolicyDetails returns a new SmartSyncChangePolicyDetails instance
func NewSmartSyncChangePolicyDetails(NewValue *SmartSyncPolicy) *SmartSyncChangePolicyDetails {
	s := new(SmartSyncChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// SmartSyncCreateAdminPrivilegeReportDetails : Smart Sync non-admin devices
// report created.
type SmartSyncCreateAdminPrivilegeReportDetails struct {
}

// NewSmartSyncCreateAdminPrivilegeReportDetails returns a new SmartSyncCreateAdminPrivilegeReportDetails instance
func NewSmartSyncCreateAdminPrivilegeReportDetails() *SmartSyncCreateAdminPrivilegeReportDetails {
	s := new(SmartSyncCreateAdminPrivilegeReportDetails)
	return s
}

// SmartSyncNotOptOutDetails : Opted team into Smart Sync.
type SmartSyncNotOptOutDetails struct {
	// PreviousValue : Previous Smart Sync opt out policy.
	PreviousValue *SmartSyncOptOutPolicy `json:"previous_value"`
	// NewValue : New Smart Sync opt out policy.
	NewValue *SmartSyncOptOutPolicy `json:"new_value"`
}

// NewSmartSyncNotOptOutDetails returns a new SmartSyncNotOptOutDetails instance
func NewSmartSyncNotOptOutDetails(PreviousValue *SmartSyncOptOutPolicy, NewValue *SmartSyncOptOutPolicy) *SmartSyncNotOptOutDetails {
	s := new(SmartSyncNotOptOutDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// SmartSyncOptOutDetails : Opted team out of Smart Sync.
type SmartSyncOptOutDetails struct {
	// PreviousValue : Previous Smart Sync opt out policy.
	PreviousValue *SmartSyncOptOutPolicy `json:"previous_value"`
	// NewValue : New Smart Sync opt out policy.
	NewValue *SmartSyncOptOutPolicy `json:"new_value"`
}

// NewSmartSyncOptOutDetails returns a new SmartSyncOptOutDetails instance
func NewSmartSyncOptOutDetails(PreviousValue *SmartSyncOptOutPolicy, NewValue *SmartSyncOptOutPolicy) *SmartSyncOptOutDetails {
	s := new(SmartSyncOptOutDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// SmartSyncOptOutPolicy : has no documentation (yet)
type SmartSyncOptOutPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SmartSyncOptOutPolicy
const (
	SmartSyncOptOutPolicyOptedOut = "opted_out"
	SmartSyncOptOutPolicyDefault  = "default"
	SmartSyncOptOutPolicyOther    = "other"
)

// SmartSyncPolicy : has no documentation (yet)
type SmartSyncPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SmartSyncPolicy
const (
	SmartSyncPolicyLocalOnly = "local_only"
	SmartSyncPolicySynced    = "synced"
	SmartSyncPolicyOther     = "other"
)

// SpaceLimitsLevel : has no documentation (yet)
type SpaceLimitsLevel struct {
	dropbox.Tagged
}

// Valid tag values for SpaceLimitsLevel
const (
	SpaceLimitsLevelGenerous = "generous"
	SpaceLimitsLevelModerate = "moderate"
	SpaceLimitsLevelNoLimit  = "no_limit"
	SpaceLimitsLevelStrict   = "strict"
	SpaceLimitsLevelOther    = "other"
)

// SpaceLimitsStatus : has no documentation (yet)
type SpaceLimitsStatus struct {
	dropbox.Tagged
}

// Valid tag values for SpaceLimitsStatus
const (
	SpaceLimitsStatusWithinQuota = "within_quota"
	SpaceLimitsStatusNearQuota   = "near_quota"
	SpaceLimitsStatusOverQuota   = "over_quota"
	SpaceLimitsStatusOther       = "other"
)

// SsoChangeCertDetails : Changed the X.509 certificate for SSO.
type SsoChangeCertDetails struct {
	// CertificateDetails : SSO certificate details.
	CertificateDetails *Certificate `json:"certificate_details"`
}

// NewSsoChangeCertDetails returns a new SsoChangeCertDetails instance
func NewSsoChangeCertDetails(CertificateDetails *Certificate) *SsoChangeCertDetails {
	s := new(SsoChangeCertDetails)
	s.CertificateDetails = CertificateDetails
	return s
}

// SsoChangeLoginUrlDetails : Changed the sign-in URL for SSO.
type SsoChangeLoginUrlDetails struct {
	// PreviousValue : Previous single sign-on login URL.
	PreviousValue string `json:"previous_value"`
	// NewValue : New single sign-on login URL.
	NewValue string `json:"new_value"`
}

// NewSsoChangeLoginUrlDetails returns a new SsoChangeLoginUrlDetails instance
func NewSsoChangeLoginUrlDetails(PreviousValue string, NewValue string) *SsoChangeLoginUrlDetails {
	s := new(SsoChangeLoginUrlDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// SsoChangeLogoutUrlDetails : Changed the sign-out URL for SSO.
type SsoChangeLogoutUrlDetails struct {
	// PreviousValue : Previous single sign-on logout URL.
	PreviousValue string `json:"previous_value"`
	// NewValue : New single sign-on logout URL. Might be missing due to
	// historical data gap.
	NewValue string `json:"new_value,omitempty"`
}

// NewSsoChangeLogoutUrlDetails returns a new SsoChangeLogoutUrlDetails instance
func NewSsoChangeLogoutUrlDetails(PreviousValue string) *SsoChangeLogoutUrlDetails {
	s := new(SsoChangeLogoutUrlDetails)
	s.PreviousValue = PreviousValue
	return s
}

// SsoChangePolicyDetails : Change the single sign-on policy for the team.
type SsoChangePolicyDetails struct {
	// NewValue : New single sign-on policy.
	NewValue *SsoPolicy `json:"new_value"`
	// PreviousValue : Previous single sign-on policy. Might be missing due to
	// historical data gap.
	PreviousValue *SsoPolicy `json:"previous_value,omitempty"`
}

// NewSsoChangePolicyDetails returns a new SsoChangePolicyDetails instance
func NewSsoChangePolicyDetails(NewValue *SsoPolicy) *SsoChangePolicyDetails {
	s := new(SsoChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// SsoChangeSamlIdentityModeDetails : Changed the SAML identity mode for SSO.
type SsoChangeSamlIdentityModeDetails struct {
	// PreviousValue : Previous single sign-on identity mode.
	PreviousValue int64 `json:"previous_value"`
	// NewValue : New single sign-on identity mode.
	NewValue int64 `json:"new_value"`
}

// NewSsoChangeSamlIdentityModeDetails returns a new SsoChangeSamlIdentityModeDetails instance
func NewSsoChangeSamlIdentityModeDetails(PreviousValue int64, NewValue int64) *SsoChangeSamlIdentityModeDetails {
	s := new(SsoChangeSamlIdentityModeDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// SsoLoginFailDetails : Failed to sign in using SSO.
type SsoLoginFailDetails struct {
	// ErrorDetails : Login failure details.
	ErrorDetails *FailureDetailsLogInfo `json:"error_details"`
}

// NewSsoLoginFailDetails returns a new SsoLoginFailDetails instance
func NewSsoLoginFailDetails(ErrorDetails *FailureDetailsLogInfo) *SsoLoginFailDetails {
	s := new(SsoLoginFailDetails)
	s.ErrorDetails = ErrorDetails
	return s
}

// SsoPolicy : SSO policy
type SsoPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SsoPolicy
const (
	SsoPolicyDisabled = "disabled"
	SsoPolicyOptional = "optional"
	SsoPolicyRequired = "required"
	SsoPolicyOther    = "other"
)

// TeamActivityCreateReportDetails : Created a team activity report.
type TeamActivityCreateReportDetails struct {
	// StartDate : Report start date.
	StartDate time.Time `json:"start_date"`
	// EndDate : Report end date.
	EndDate time.Time `json:"end_date"`
}

// NewTeamActivityCreateReportDetails returns a new TeamActivityCreateReportDetails instance
func NewTeamActivityCreateReportDetails(StartDate time.Time, EndDate time.Time) *TeamActivityCreateReportDetails {
	s := new(TeamActivityCreateReportDetails)
	s.StartDate = StartDate
	s.EndDate = EndDate
	return s
}

// TeamEvent : An audit log event.
type TeamEvent struct {
	// Timestamp : The Dropbox timestamp representing when the action was taken.
	Timestamp time.Time `json:"timestamp"`
	// EventCategory : The category that this type of action belongs to.
	EventCategory *EventCategory `json:"event_category"`
	// Actor : The entity who actually performed the action.
	Actor *ActorLogInfo `json:"actor"`
	// Origin : The origin from which the actor performed the action including
	// information about host, ip address, location, session, etc. If the action
	// was performed programmatically via the API the origin represents the API
	// client.
	Origin *OriginLogInfo `json:"origin,omitempty"`
	// Participants : Zero or more users and/or groups that are affected by the
	// action. Note that this list doesn't include any actors or users in
	// context.
	Participants []*ParticipantLogInfo `json:"participants,omitempty"`
	// Assets : Zero or more content assets involved in the action. Currently
	// these include Dropbox files and folders but in the future we might add
	// other asset types such as Paper documents, folders, projects, etc.
	Assets []*AssetLogInfo `json:"assets,omitempty"`
	// InvolveNonTeamMember : True if the action involved a non team member
	// either as the actor or as one of the affected users.
	InvolveNonTeamMember bool `json:"involve_non_team_member"`
	// Context : The user or team on whose behalf the actor performed the
	// action.
	Context *ContextLogInfo `json:"context"`
	// EventType : The particular type of action taken.
	EventType *EventType `json:"event_type"`
	// Details : The variable event schema applicable to this type of action,
	// instantiated with respect to this particular action.
	Details *EventDetails `json:"details"`
}

// NewTeamEvent returns a new TeamEvent instance
func NewTeamEvent(Timestamp time.Time, EventCategory *EventCategory, Actor *ActorLogInfo, InvolveNonTeamMember bool, Context *ContextLogInfo, EventType *EventType, Details *EventDetails) *TeamEvent {
	s := new(TeamEvent)
	s.Timestamp = Timestamp
	s.EventCategory = EventCategory
	s.Actor = Actor
	s.InvolveNonTeamMember = InvolveNonTeamMember
	s.Context = Context
	s.EventType = EventType
	s.Details = Details
	return s
}

// TeamFolderChangeStatusDetails : Changed the archival status of a team folder.
type TeamFolderChangeStatusDetails struct {
	// NewValue : New team folder status.
	NewValue *TeamFolderStatus `json:"new_value"`
	// PreviousValue : Previous team folder status. Might be missing due to
	// historical data gap.
	PreviousValue *TeamFolderStatus `json:"previous_value,omitempty"`
}

// NewTeamFolderChangeStatusDetails returns a new TeamFolderChangeStatusDetails instance
func NewTeamFolderChangeStatusDetails(NewValue *TeamFolderStatus) *TeamFolderChangeStatusDetails {
	s := new(TeamFolderChangeStatusDetails)
	s.NewValue = NewValue
	return s
}

// TeamFolderCreateDetails : Created a new team folder in active status.
type TeamFolderCreateDetails struct {
}

// NewTeamFolderCreateDetails returns a new TeamFolderCreateDetails instance
func NewTeamFolderCreateDetails() *TeamFolderCreateDetails {
	s := new(TeamFolderCreateDetails)
	return s
}

// TeamFolderDowngradeDetails : Downgraded a team folder to a regular shared
// folder.
type TeamFolderDowngradeDetails struct {
	// TargetIndex : Target asset index.
	TargetIndex int64 `json:"target_index"`
}

// NewTeamFolderDowngradeDetails returns a new TeamFolderDowngradeDetails instance
func NewTeamFolderDowngradeDetails(TargetIndex int64) *TeamFolderDowngradeDetails {
	s := new(TeamFolderDowngradeDetails)
	s.TargetIndex = TargetIndex
	return s
}

// TeamFolderPermanentlyDeleteDetails : Permanently deleted an archived team
// folder.
type TeamFolderPermanentlyDeleteDetails struct {
}

// NewTeamFolderPermanentlyDeleteDetails returns a new TeamFolderPermanentlyDeleteDetails instance
func NewTeamFolderPermanentlyDeleteDetails() *TeamFolderPermanentlyDeleteDetails {
	s := new(TeamFolderPermanentlyDeleteDetails)
	return s
}

// TeamFolderRenameDetails : Renamed an active or archived team folder.
type TeamFolderRenameDetails struct {
	// RelocateActionDetails : Specifies the source and destination indices in
	// the assets list.
	RelocateActionDetails *RelocateAssetReferencesLogInfo `json:"relocate_action_details"`
}

// NewTeamFolderRenameDetails returns a new TeamFolderRenameDetails instance
func NewTeamFolderRenameDetails(RelocateActionDetails *RelocateAssetReferencesLogInfo) *TeamFolderRenameDetails {
	s := new(TeamFolderRenameDetails)
	s.RelocateActionDetails = RelocateActionDetails
	return s
}

// TeamFolderStatus : has no documentation (yet)
type TeamFolderStatus struct {
	dropbox.Tagged
}

// Valid tag values for TeamFolderStatus
const (
	TeamFolderStatusArchive   = "archive"
	TeamFolderStatusUnarchive = "unarchive"
	TeamFolderStatusOther     = "other"
)

// TeamLinkedAppLogInfo : Team linked app
type TeamLinkedAppLogInfo struct {
	AppLogInfo
}

// NewTeamLinkedAppLogInfo returns a new TeamLinkedAppLogInfo instance
func NewTeamLinkedAppLogInfo() *TeamLinkedAppLogInfo {
	s := new(TeamLinkedAppLogInfo)
	return s
}

// TeamMemberLogInfo : Team member's logged information.
type TeamMemberLogInfo struct {
	UserLogInfo
	// TeamMemberId : Team member ID. Might be missing due to historical data
	// gap.
	TeamMemberId string `json:"team_member_id,omitempty"`
	// MemberExternalId : Team member external ID.
	MemberExternalId string `json:"member_external_id,omitempty"`
}

// NewTeamMemberLogInfo returns a new TeamMemberLogInfo instance
func NewTeamMemberLogInfo() *TeamMemberLogInfo {
	s := new(TeamMemberLogInfo)
	return s
}

// TeamMembershipType : has no documentation (yet)
type TeamMembershipType struct {
	dropbox.Tagged
}

// Valid tag values for TeamMembershipType
const (
	TeamMembershipTypeFree  = "free"
	TeamMembershipTypeFull  = "full"
	TeamMembershipTypeOther = "other"
)

// TeamMergeFromDetails : Merged another team into this team.
type TeamMergeFromDetails struct {
	// TeamName : The name of the team that was merged into this team.
	TeamName string `json:"team_name"`
}

// NewTeamMergeFromDetails returns a new TeamMergeFromDetails instance
func NewTeamMergeFromDetails(TeamName string) *TeamMergeFromDetails {
	s := new(TeamMergeFromDetails)
	s.TeamName = TeamName
	return s
}

// TeamMergeToDetails : Merged this team into another team.
type TeamMergeToDetails struct {
	// TeamName : The name of the team that this team was merged into.
	TeamName string `json:"team_name"`
}

// NewTeamMergeToDetails returns a new TeamMergeToDetails instance
func NewTeamMergeToDetails(TeamName string) *TeamMergeToDetails {
	s := new(TeamMergeToDetails)
	s.TeamName = TeamName
	return s
}

// TeamName : Team name details
type TeamName struct {
	// TeamDisplayName : Team's display name.
	TeamDisplayName string `json:"team_display_name"`
	// TeamLegalName : Team's legal name.
	TeamLegalName string `json:"team_legal_name"`
}

// NewTeamName returns a new TeamName instance
func NewTeamName(TeamDisplayName string, TeamLegalName string) *TeamName {
	s := new(TeamName)
	s.TeamDisplayName = TeamDisplayName
	s.TeamLegalName = TeamLegalName
	return s
}

// TeamProfileAddLogoDetails : Added a team logo to be displayed on shared link
// headers.
type TeamProfileAddLogoDetails struct {
}

// NewTeamProfileAddLogoDetails returns a new TeamProfileAddLogoDetails instance
func NewTeamProfileAddLogoDetails() *TeamProfileAddLogoDetails {
	s := new(TeamProfileAddLogoDetails)
	return s
}

// TeamProfileChangeLogoDetails : Changed the team logo to be displayed on
// shared link headers.
type TeamProfileChangeLogoDetails struct {
}

// NewTeamProfileChangeLogoDetails returns a new TeamProfileChangeLogoDetails instance
func NewTeamProfileChangeLogoDetails() *TeamProfileChangeLogoDetails {
	s := new(TeamProfileChangeLogoDetails)
	return s
}

// TeamProfileChangeNameDetails : Changed the team name.
type TeamProfileChangeNameDetails struct {
	// PreviousValue : Previous teams name. Might be missing due to historical
	// data gap.
	PreviousValue *TeamName `json:"previous_value,omitempty"`
	// NewValue : New team name.
	NewValue *TeamName `json:"new_value"`
}

// NewTeamProfileChangeNameDetails returns a new TeamProfileChangeNameDetails instance
func NewTeamProfileChangeNameDetails(NewValue *TeamName) *TeamProfileChangeNameDetails {
	s := new(TeamProfileChangeNameDetails)
	s.NewValue = NewValue
	return s
}

// TeamProfileRemoveLogoDetails : Removed the team logo to be displayed on
// shared link headers.
type TeamProfileRemoveLogoDetails struct {
}

// NewTeamProfileRemoveLogoDetails returns a new TeamProfileRemoveLogoDetails instance
func NewTeamProfileRemoveLogoDetails() *TeamProfileRemoveLogoDetails {
	s := new(TeamProfileRemoveLogoDetails)
	return s
}

// TfaAddBackupPhoneDetails : Added a backup phone for two-step verification.
type TfaAddBackupPhoneDetails struct {
}

// NewTfaAddBackupPhoneDetails returns a new TfaAddBackupPhoneDetails instance
func NewTfaAddBackupPhoneDetails() *TfaAddBackupPhoneDetails {
	s := new(TfaAddBackupPhoneDetails)
	return s
}

// TfaAddSecurityKeyDetails : Added a security key for two-step verification.
type TfaAddSecurityKeyDetails struct {
}

// NewTfaAddSecurityKeyDetails returns a new TfaAddSecurityKeyDetails instance
func NewTfaAddSecurityKeyDetails() *TfaAddSecurityKeyDetails {
	s := new(TfaAddSecurityKeyDetails)
	return s
}

// TfaChangeBackupPhoneDetails : Changed the backup phone for two-step
// verification.
type TfaChangeBackupPhoneDetails struct {
}

// NewTfaChangeBackupPhoneDetails returns a new TfaChangeBackupPhoneDetails instance
func NewTfaChangeBackupPhoneDetails() *TfaChangeBackupPhoneDetails {
	s := new(TfaChangeBackupPhoneDetails)
	return s
}

// TfaChangePolicyDetails : Change two-step verification policy for the team.
type TfaChangePolicyDetails struct {
	// NewValue : New change policy.
	NewValue *TfaPolicy `json:"new_value"`
	// PreviousValue : Previous change policy. Might be missing due to
	// historical data gap.
	PreviousValue *TfaPolicy `json:"previous_value,omitempty"`
}

// NewTfaChangePolicyDetails returns a new TfaChangePolicyDetails instance
func NewTfaChangePolicyDetails(NewValue *TfaPolicy) *TfaChangePolicyDetails {
	s := new(TfaChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// TfaChangeStatusDetails : Enabled, disabled or changed the configuration for
// two-step verification.
type TfaChangeStatusDetails struct {
	// NewValue : The new two factor authentication configuration.
	NewValue *TfaConfiguration `json:"new_value"`
	// PreviousValue : The previous two factor authentication configuration.
	// Might be missing due to historical data gap.
	PreviousValue *TfaConfiguration `json:"previous_value,omitempty"`
	// UsedRescueCode : Used two factor authentication rescue code. This flag is
	// relevant when the two factor authentication configuration is disabled.
	UsedRescueCode bool `json:"used_rescue_code,omitempty"`
}

// NewTfaChangeStatusDetails returns a new TfaChangeStatusDetails instance
func NewTfaChangeStatusDetails(NewValue *TfaConfiguration) *TfaChangeStatusDetails {
	s := new(TfaChangeStatusDetails)
	s.NewValue = NewValue
	return s
}

// TfaConfiguration : Two factor authentication configuration. Note: the enabled
// option is deprecated.
type TfaConfiguration struct {
	dropbox.Tagged
}

// Valid tag values for TfaConfiguration
const (
	TfaConfigurationDisabled      = "disabled"
	TfaConfigurationEnabled       = "enabled"
	TfaConfigurationSms           = "sms"
	TfaConfigurationAuthenticator = "authenticator"
	TfaConfigurationOther         = "other"
)

// TfaPolicy : Two factor authentication policy
type TfaPolicy struct {
	dropbox.Tagged
}

// Valid tag values for TfaPolicy
const (
	TfaPolicyDisabled = "disabled"
	TfaPolicyOptional = "optional"
	TfaPolicyRequired = "required"
	TfaPolicyOther    = "other"
)

// TfaRemoveBackupPhoneDetails : Removed the backup phone for two-step
// verification.
type TfaRemoveBackupPhoneDetails struct {
}

// NewTfaRemoveBackupPhoneDetails returns a new TfaRemoveBackupPhoneDetails instance
func NewTfaRemoveBackupPhoneDetails() *TfaRemoveBackupPhoneDetails {
	s := new(TfaRemoveBackupPhoneDetails)
	return s
}

// TfaRemoveSecurityKeyDetails : Removed a security key for two-step
// verification.
type TfaRemoveSecurityKeyDetails struct {
}

// NewTfaRemoveSecurityKeyDetails returns a new TfaRemoveSecurityKeyDetails instance
func NewTfaRemoveSecurityKeyDetails() *TfaRemoveSecurityKeyDetails {
	s := new(TfaRemoveSecurityKeyDetails)
	return s
}

// TfaResetDetails : Reset two-step verification for team member.
type TfaResetDetails struct {
}

// NewTfaResetDetails returns a new TfaResetDetails instance
func NewTfaResetDetails() *TfaResetDetails {
	s := new(TfaResetDetails)
	return s
}

// TimeUnit : has no documentation (yet)
type TimeUnit struct {
	dropbox.Tagged
}

// Valid tag values for TimeUnit
const (
	TimeUnitMilliseconds = "milliseconds"
	TimeUnitSeconds      = "seconds"
	TimeUnitMinutes      = "minutes"
	TimeUnitHours        = "hours"
	TimeUnitDays         = "days"
	TimeUnitWeeks        = "weeks"
	TimeUnitMonths       = "months"
	TimeUnitYears        = "years"
	TimeUnitOther        = "other"
)

// TwoAccountChangePolicyDetails : Enabled or disabled the option for team
// members to link a personal Dropbox account in addition to their work account
// to the same computer.
type TwoAccountChangePolicyDetails struct {
	// NewValue : New two account policy.
	NewValue *TwoAccountPolicy `json:"new_value"`
	// PreviousValue : Previous two account policy. Might be missing due to
	// historical data gap.
	PreviousValue *TwoAccountPolicy `json:"previous_value,omitempty"`
}

// NewTwoAccountChangePolicyDetails returns a new TwoAccountChangePolicyDetails instance
func NewTwoAccountChangePolicyDetails(NewValue *TwoAccountPolicy) *TwoAccountChangePolicyDetails {
	s := new(TwoAccountChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// TwoAccountPolicy : Policy for pairing personal account to work account
type TwoAccountPolicy struct {
	dropbox.Tagged
}

// Valid tag values for TwoAccountPolicy
const (
	TwoAccountPolicyDisabled = "disabled"
	TwoAccountPolicyEnabled  = "enabled"
	TwoAccountPolicyOther    = "other"
)

// UserLinkedAppLogInfo : User linked app
type UserLinkedAppLogInfo struct {
	AppLogInfo
}

// NewUserLinkedAppLogInfo returns a new UserLinkedAppLogInfo instance
func NewUserLinkedAppLogInfo() *UserLinkedAppLogInfo {
	s := new(UserLinkedAppLogInfo)
	return s
}

// UserNameLogInfo : User's name logged information
type UserNameLogInfo struct {
	// GivenName : Given name.
	GivenName string `json:"given_name"`
	// Surname : Surname.
	Surname string `json:"surname"`
	// Locale : Locale. Might be missing due to historical data gap.
	Locale string `json:"locale,omitempty"`
}

// NewUserNameLogInfo returns a new UserNameLogInfo instance
func NewUserNameLogInfo(GivenName string, Surname string) *UserNameLogInfo {
	s := new(UserNameLogInfo)
	s.GivenName = GivenName
	s.Surname = Surname
	return s
}

// UserOrTeamLinkedAppLogInfo : User or team linked app. Used when linked type
// is missing due to historical data gap.
type UserOrTeamLinkedAppLogInfo struct {
	AppLogInfo
}

// NewUserOrTeamLinkedAppLogInfo returns a new UserOrTeamLinkedAppLogInfo instance
func NewUserOrTeamLinkedAppLogInfo() *UserOrTeamLinkedAppLogInfo {
	s := new(UserOrTeamLinkedAppLogInfo)
	return s
}

// WebSessionLogInfo : Web session.
type WebSessionLogInfo struct {
	SessionLogInfo
}

// NewWebSessionLogInfo returns a new WebSessionLogInfo instance
func NewWebSessionLogInfo() *WebSessionLogInfo {
	s := new(WebSessionLogInfo)
	return s
}

// WebSessionsChangeFixedLengthPolicyDetails : Changed how long team members can
// stay signed in to Dropbox on the web.
type WebSessionsChangeFixedLengthPolicyDetails struct {
	// NewValue : New session length policy.
	NewValue *WebSessionsFixedLengthPolicy `json:"new_value"`
	// PreviousValue : Previous session length policy.
	PreviousValue *WebSessionsFixedLengthPolicy `json:"previous_value"`
}

// NewWebSessionsChangeFixedLengthPolicyDetails returns a new WebSessionsChangeFixedLengthPolicyDetails instance
func NewWebSessionsChangeFixedLengthPolicyDetails(NewValue *WebSessionsFixedLengthPolicy, PreviousValue *WebSessionsFixedLengthPolicy) *WebSessionsChangeFixedLengthPolicyDetails {
	s := new(WebSessionsChangeFixedLengthPolicyDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// WebSessionsChangeIdleLengthPolicyDetails : Changed how long team members can
// be idle while signed in to Dropbox on the web.
type WebSessionsChangeIdleLengthPolicyDetails struct {
	// NewValue : New idle length policy.
	NewValue *WebSessionsIdleLengthPolicy `json:"new_value"`
	// PreviousValue : Previous idle length policy.
	PreviousValue *WebSessionsIdleLengthPolicy `json:"previous_value"`
}

// NewWebSessionsChangeIdleLengthPolicyDetails returns a new WebSessionsChangeIdleLengthPolicyDetails instance
func NewWebSessionsChangeIdleLengthPolicyDetails(NewValue *WebSessionsIdleLengthPolicy, PreviousValue *WebSessionsIdleLengthPolicy) *WebSessionsChangeIdleLengthPolicyDetails {
	s := new(WebSessionsChangeIdleLengthPolicyDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// WebSessionsFixedLengthPolicy : Web sessions fixed length policy.
type WebSessionsFixedLengthPolicy struct {
	dropbox.Tagged
	// Defined : Defined fixed session length.
	Defined *DurationLogInfo `json:"defined,omitempty"`
}

// Valid tag values for WebSessionsFixedLengthPolicy
const (
	WebSessionsFixedLengthPolicyDefined   = "defined"
	WebSessionsFixedLengthPolicyUndefined = "undefined"
	WebSessionsFixedLengthPolicyOther     = "other"
)

// UnmarshalJSON deserializes into a WebSessionsFixedLengthPolicy instance
func (u *WebSessionsFixedLengthPolicy) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Defined : Defined fixed session length.
		Defined json.RawMessage `json:"defined,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "defined":
		err = json.Unmarshal(body, &u.Defined)

		if err != nil {
			return err
		}
	}
	return nil
}

// WebSessionsIdleLengthPolicy : Web sessions idle length policy.
type WebSessionsIdleLengthPolicy struct {
	dropbox.Tagged
	// Defined : Defined idle session length.
	Defined *DurationLogInfo `json:"defined,omitempty"`
}

// Valid tag values for WebSessionsIdleLengthPolicy
const (
	WebSessionsIdleLengthPolicyDefined   = "defined"
	WebSessionsIdleLengthPolicyUndefined = "undefined"
	WebSessionsIdleLengthPolicyOther     = "other"
)

// UnmarshalJSON deserializes into a WebSessionsIdleLengthPolicy instance
func (u *WebSessionsIdleLengthPolicy) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Defined : Defined idle session length.
		Defined json.RawMessage `json:"defined,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "defined":
		err = json.Unmarshal(body, &u.Defined)

		if err != nil {
			return err
		}
	}
	return nil
}
