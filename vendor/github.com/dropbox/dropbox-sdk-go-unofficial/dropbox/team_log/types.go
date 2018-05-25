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
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/sharing"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team_common"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team_policies"
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

// AccountCaptureChangeAvailabilityDetails : Granted/revoked option to enable
// account capture on team domains.
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

// AccountCaptureChangeAvailabilityType : has no documentation (yet)
type AccountCaptureChangeAvailabilityType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAccountCaptureChangeAvailabilityType returns a new AccountCaptureChangeAvailabilityType instance
func NewAccountCaptureChangeAvailabilityType(Description string) *AccountCaptureChangeAvailabilityType {
	s := new(AccountCaptureChangeAvailabilityType)
	s.Description = Description
	return s
}

// AccountCaptureChangePolicyDetails : Changed account capture setting on team
// domain.
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

// AccountCaptureChangePolicyType : has no documentation (yet)
type AccountCaptureChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAccountCaptureChangePolicyType returns a new AccountCaptureChangePolicyType instance
func NewAccountCaptureChangePolicyType(Description string) *AccountCaptureChangePolicyType {
	s := new(AccountCaptureChangePolicyType)
	s.Description = Description
	return s
}

// AccountCaptureMigrateAccountDetails : Account-captured user migrated account
// to team.
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

// AccountCaptureMigrateAccountType : has no documentation (yet)
type AccountCaptureMigrateAccountType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAccountCaptureMigrateAccountType returns a new AccountCaptureMigrateAccountType instance
func NewAccountCaptureMigrateAccountType(Description string) *AccountCaptureMigrateAccountType {
	s := new(AccountCaptureMigrateAccountType)
	s.Description = Description
	return s
}

// AccountCaptureNotificationEmailsSentDetails : Sent proactive account capture
// email to all unmanaged members.
type AccountCaptureNotificationEmailsSentDetails struct {
	// DomainName : Domain name.
	DomainName string `json:"domain_name"`
}

// NewAccountCaptureNotificationEmailsSentDetails returns a new AccountCaptureNotificationEmailsSentDetails instance
func NewAccountCaptureNotificationEmailsSentDetails(DomainName string) *AccountCaptureNotificationEmailsSentDetails {
	s := new(AccountCaptureNotificationEmailsSentDetails)
	s.DomainName = DomainName
	return s
}

// AccountCaptureNotificationEmailsSentType : has no documentation (yet)
type AccountCaptureNotificationEmailsSentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAccountCaptureNotificationEmailsSentType returns a new AccountCaptureNotificationEmailsSentType instance
func NewAccountCaptureNotificationEmailsSentType(Description string) *AccountCaptureNotificationEmailsSentType {
	s := new(AccountCaptureNotificationEmailsSentType)
	s.Description = Description
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

// AccountCaptureRelinquishAccountDetails : Account-captured user changed
// account email to personal email.
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

// AccountCaptureRelinquishAccountType : has no documentation (yet)
type AccountCaptureRelinquishAccountType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAccountCaptureRelinquishAccountType returns a new AccountCaptureRelinquishAccountType instance
func NewAccountCaptureRelinquishAccountType(Description string) *AccountCaptureRelinquishAccountType {
	s := new(AccountCaptureRelinquishAccountType)
	s.Description = Description
	return s
}

// ActionDetails : Additional information indicating the action taken that
// caused status change.
type ActionDetails struct {
	dropbox.Tagged
	// TeamJoinDetails : Additional information relevant when a new member joins
	// the team.
	TeamJoinDetails *JoinTeamDetails `json:"team_join_details,omitempty"`
	// RemoveAction : Define how the user was removed from the team.
	RemoveAction *MemberRemoveActionType `json:"remove_action,omitempty"`
}

// Valid tag values for ActionDetails
const (
	ActionDetailsTeamJoinDetails = "team_join_details"
	ActionDetailsRemoveAction    = "remove_action"
	ActionDetailsOther           = "other"
)

// UnmarshalJSON deserializes into a ActionDetails instance
func (u *ActionDetails) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// TeamJoinDetails : Additional information relevant when a new member
		// joins the team.
		TeamJoinDetails json.RawMessage `json:"team_join_details,omitempty"`
		// RemoveAction : Define how the user was removed from the team.
		RemoveAction json.RawMessage `json:"remove_action,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "team_join_details":
		err = json.Unmarshal(body, &u.TeamJoinDetails)

		if err != nil {
			return err
		}
	case "remove_action":
		err = json.Unmarshal(w.RemoveAction, &u.RemoveAction)

		if err != nil {
			return err
		}
	}
	return nil
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
	ActorLogInfoUser      = "user"
	ActorLogInfoAdmin     = "admin"
	ActorLogInfoApp       = "app"
	ActorLogInfoReseller  = "reseller"
	ActorLogInfoDropbox   = "dropbox"
	ActorLogInfoAnonymous = "anonymous"
	ActorLogInfoOther     = "other"
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
	AdminRoleTeamAdmin           = "team_admin"
	AdminRoleUserManagementAdmin = "user_management_admin"
	AdminRoleSupportAdmin        = "support_admin"
	AdminRoleLimitedAdmin        = "limited_admin"
	AdminRoleMemberOnly          = "member_only"
	AdminRoleOther               = "other"
)

// AllowDownloadDisabledDetails : Disabled downloads.
type AllowDownloadDisabledDetails struct {
}

// NewAllowDownloadDisabledDetails returns a new AllowDownloadDisabledDetails instance
func NewAllowDownloadDisabledDetails() *AllowDownloadDisabledDetails {
	s := new(AllowDownloadDisabledDetails)
	return s
}

// AllowDownloadDisabledType : has no documentation (yet)
type AllowDownloadDisabledType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAllowDownloadDisabledType returns a new AllowDownloadDisabledType instance
func NewAllowDownloadDisabledType(Description string) *AllowDownloadDisabledType {
	s := new(AllowDownloadDisabledType)
	s.Description = Description
	return s
}

// AllowDownloadEnabledDetails : Enabled downloads.
type AllowDownloadEnabledDetails struct {
}

// NewAllowDownloadEnabledDetails returns a new AllowDownloadEnabledDetails instance
func NewAllowDownloadEnabledDetails() *AllowDownloadEnabledDetails {
	s := new(AllowDownloadEnabledDetails)
	return s
}

// AllowDownloadEnabledType : has no documentation (yet)
type AllowDownloadEnabledType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAllowDownloadEnabledType returns a new AllowDownloadEnabledType instance
func NewAllowDownloadEnabledType(Description string) *AllowDownloadEnabledType {
	s := new(AllowDownloadEnabledType)
	s.Description = Description
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

// AppLinkTeamDetails : Linked app for team.
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

// UnmarshalJSON deserializes into a AppLinkTeamDetails instance
func (u *AppLinkTeamDetails) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// AppInfo : Relevant application details.
		AppInfo json.RawMessage `json:"app_info"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	AppInfo, err := IsAppLogInfoFromJSON(w.AppInfo)
	if err != nil {
		return err
	}
	u.AppInfo = AppInfo
	return nil
}

// AppLinkTeamType : has no documentation (yet)
type AppLinkTeamType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAppLinkTeamType returns a new AppLinkTeamType instance
func NewAppLinkTeamType(Description string) *AppLinkTeamType {
	s := new(AppLinkTeamType)
	s.Description = Description
	return s
}

// AppLinkUserDetails : Linked app for member.
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

// UnmarshalJSON deserializes into a AppLinkUserDetails instance
func (u *AppLinkUserDetails) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// AppInfo : Relevant application details.
		AppInfo json.RawMessage `json:"app_info"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	AppInfo, err := IsAppLogInfoFromJSON(w.AppInfo)
	if err != nil {
		return err
	}
	u.AppInfo = AppInfo
	return nil
}

// AppLinkUserType : has no documentation (yet)
type AppLinkUserType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAppLinkUserType returns a new AppLinkUserType instance
func NewAppLinkUserType(Description string) *AppLinkUserType {
	s := new(AppLinkUserType)
	s.Description = Description
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

// AppUnlinkTeamDetails : Unlinked app for team.
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

// UnmarshalJSON deserializes into a AppUnlinkTeamDetails instance
func (u *AppUnlinkTeamDetails) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// AppInfo : Relevant application details.
		AppInfo json.RawMessage `json:"app_info"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	AppInfo, err := IsAppLogInfoFromJSON(w.AppInfo)
	if err != nil {
		return err
	}
	u.AppInfo = AppInfo
	return nil
}

// AppUnlinkTeamType : has no documentation (yet)
type AppUnlinkTeamType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAppUnlinkTeamType returns a new AppUnlinkTeamType instance
func NewAppUnlinkTeamType(Description string) *AppUnlinkTeamType {
	s := new(AppUnlinkTeamType)
	s.Description = Description
	return s
}

// AppUnlinkUserDetails : Unlinked app for member.
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

// UnmarshalJSON deserializes into a AppUnlinkUserDetails instance
func (u *AppUnlinkUserDetails) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// AppInfo : Relevant application details.
		AppInfo json.RawMessage `json:"app_info"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	AppInfo, err := IsAppLogInfoFromJSON(w.AppInfo)
	if err != nil {
		return err
	}
	u.AppInfo = AppInfo
	return nil
}

// AppUnlinkUserType : has no documentation (yet)
type AppUnlinkUserType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewAppUnlinkUserType returns a new AppUnlinkUserType instance
func NewAppUnlinkUserType(Description string) *AppUnlinkUserType {
	s := new(AppUnlinkUserType)
	s.Description = Description
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
	// ShowcaseDocument : Showcase document's details.
	ShowcaseDocument *ShowcaseDocumentLogInfo `json:"showcase_document,omitempty"`
}

// Valid tag values for AssetLogInfo
const (
	AssetLogInfoFile             = "file"
	AssetLogInfoFolder           = "folder"
	AssetLogInfoPaperDocument    = "paper_document"
	AssetLogInfoPaperFolder      = "paper_folder"
	AssetLogInfoShowcaseDocument = "showcase_document"
	AssetLogInfoOther            = "other"
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
		// ShowcaseDocument : Showcase document's details.
		ShowcaseDocument json.RawMessage `json:"showcase_document,omitempty"`
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
	case "showcase_document":
		err = json.Unmarshal(body, &u.ShowcaseDocument)

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
	CommonName string `json:"common_name,omitempty"`
}

// NewCertificate returns a new Certificate instance
func NewCertificate(Subject string, Issuer string, IssueDate string, ExpirationDate string, SerialNumber string, Sha1Fingerprint string) *Certificate {
	s := new(Certificate)
	s.Subject = Subject
	s.Issuer = Issuer
	s.IssueDate = IssueDate
	s.ExpirationDate = ExpirationDate
	s.SerialNumber = SerialNumber
	s.Sha1Fingerprint = Sha1Fingerprint
	return s
}

// CollectionShareDetails : Shared album.
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

// CollectionShareType : has no documentation (yet)
type CollectionShareType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewCollectionShareType returns a new CollectionShareType instance
func NewCollectionShareType(Description string) *CollectionShareType {
	s := new(CollectionShareType)
	s.Description = Description
	return s
}

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
	ContextLogInfoAnonymous     = "anonymous"
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

// CreateFolderType : has no documentation (yet)
type CreateFolderType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewCreateFolderType returns a new CreateFolderType instance
func NewCreateFolderType(Description string) *CreateFolderType {
	s := new(CreateFolderType)
	s.Description = Description
	return s
}

// DataPlacementRestrictionChangePolicyDetails : Set restrictions on data center
// locations where team data resides.
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

// DataPlacementRestrictionChangePolicyType : has no documentation (yet)
type DataPlacementRestrictionChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDataPlacementRestrictionChangePolicyType returns a new DataPlacementRestrictionChangePolicyType instance
func NewDataPlacementRestrictionChangePolicyType(Description string) *DataPlacementRestrictionChangePolicyType {
	s := new(DataPlacementRestrictionChangePolicyType)
	s.Description = Description
	return s
}

// DataPlacementRestrictionSatisfyPolicyDetails : Completed restrictions on data
// center locations where team data resides.
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

// DataPlacementRestrictionSatisfyPolicyType : has no documentation (yet)
type DataPlacementRestrictionSatisfyPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDataPlacementRestrictionSatisfyPolicyType returns a new DataPlacementRestrictionSatisfyPolicyType instance
func NewDataPlacementRestrictionSatisfyPolicyType(Description string) *DataPlacementRestrictionSatisfyPolicyType {
	s := new(DataPlacementRestrictionSatisfyPolicyType)
	s.Description = Description
	return s
}

// DeviceSessionLogInfo : Device's session logged information.
type DeviceSessionLogInfo struct {
	// IpAddress : The IP address of the last activity from this session. Might
	// be missing due to historical data gap.
	IpAddress string `json:"ip_address,omitempty"`
	// Created : The time this session was created. Might be missing due to
	// historical data gap.
	Created time.Time `json:"created,omitempty"`
	// Updated : The time of the last activity from this session. Might be
	// missing due to historical data gap.
	Updated time.Time `json:"updated,omitempty"`
}

// NewDeviceSessionLogInfo returns a new DeviceSessionLogInfo instance
func NewDeviceSessionLogInfo() *DeviceSessionLogInfo {
	s := new(DeviceSessionLogInfo)
	return s
}

// IsDeviceSessionLogInfo is the interface type for DeviceSessionLogInfo and its subtypes
type IsDeviceSessionLogInfo interface {
	IsDeviceSessionLogInfo()
}

// IsDeviceSessionLogInfo implements the IsDeviceSessionLogInfo interface
func (u *DeviceSessionLogInfo) IsDeviceSessionLogInfo() {}

type deviceSessionLogInfoUnion struct {
	dropbox.Tagged
	// DesktopDeviceSession : has no documentation (yet)
	DesktopDeviceSession *DesktopDeviceSessionLogInfo `json:"desktop_device_session,omitempty"`
	// MobileDeviceSession : has no documentation (yet)
	MobileDeviceSession *MobileDeviceSessionLogInfo `json:"mobile_device_session,omitempty"`
	// WebDeviceSession : has no documentation (yet)
	WebDeviceSession *WebDeviceSessionLogInfo `json:"web_device_session,omitempty"`
	// LegacyDeviceSession : has no documentation (yet)
	LegacyDeviceSession *LegacyDeviceSessionLogInfo `json:"legacy_device_session,omitempty"`
}

// Valid tag values for DeviceSessionLogInfo
const (
	DeviceSessionLogInfoDesktopDeviceSession = "desktop_device_session"
	DeviceSessionLogInfoMobileDeviceSession  = "mobile_device_session"
	DeviceSessionLogInfoWebDeviceSession     = "web_device_session"
	DeviceSessionLogInfoLegacyDeviceSession  = "legacy_device_session"
)

// UnmarshalJSON deserializes into a deviceSessionLogInfoUnion instance
func (u *deviceSessionLogInfoUnion) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// DesktopDeviceSession : has no documentation (yet)
		DesktopDeviceSession json.RawMessage `json:"desktop_device_session,omitempty"`
		// MobileDeviceSession : has no documentation (yet)
		MobileDeviceSession json.RawMessage `json:"mobile_device_session,omitempty"`
		// WebDeviceSession : has no documentation (yet)
		WebDeviceSession json.RawMessage `json:"web_device_session,omitempty"`
		// LegacyDeviceSession : has no documentation (yet)
		LegacyDeviceSession json.RawMessage `json:"legacy_device_session,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "desktop_device_session":
		err = json.Unmarshal(body, &u.DesktopDeviceSession)

		if err != nil {
			return err
		}
	case "mobile_device_session":
		err = json.Unmarshal(body, &u.MobileDeviceSession)

		if err != nil {
			return err
		}
	case "web_device_session":
		err = json.Unmarshal(body, &u.WebDeviceSession)

		if err != nil {
			return err
		}
	case "legacy_device_session":
		err = json.Unmarshal(body, &u.LegacyDeviceSession)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsDeviceSessionLogInfoFromJSON converts JSON to a concrete IsDeviceSessionLogInfo instance
func IsDeviceSessionLogInfoFromJSON(data []byte) (IsDeviceSessionLogInfo, error) {
	var t deviceSessionLogInfoUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "desktop_device_session":
		return t.DesktopDeviceSession, nil

	case "mobile_device_session":
		return t.MobileDeviceSession, nil

	case "web_device_session":
		return t.WebDeviceSession, nil

	case "legacy_device_session":
		return t.LegacyDeviceSession, nil

	}
	return nil, nil
}

// DesktopDeviceSessionLogInfo : Information about linked Dropbox desktop client
// sessions
type DesktopDeviceSessionLogInfo struct {
	DeviceSessionLogInfo
	// SessionInfo : Desktop session unique id. Might be missing due to
	// historical data gap.
	SessionInfo *DesktopSessionLogInfo `json:"session_info,omitempty"`
	// HostName : Name of the hosting desktop.
	HostName string `json:"host_name"`
	// ClientType : The Dropbox desktop client type.
	ClientType *team.DesktopPlatform `json:"client_type"`
	// ClientVersion : The Dropbox client version.
	ClientVersion string `json:"client_version,omitempty"`
	// Platform : Information on the hosting platform.
	Platform string `json:"platform"`
	// IsDeleteOnUnlinkSupported : Whether itu2019s possible to delete all of
	// the account files upon unlinking.
	IsDeleteOnUnlinkSupported bool `json:"is_delete_on_unlink_supported"`
}

// NewDesktopDeviceSessionLogInfo returns a new DesktopDeviceSessionLogInfo instance
func NewDesktopDeviceSessionLogInfo(HostName string, ClientType *team.DesktopPlatform, Platform string, IsDeleteOnUnlinkSupported bool) *DesktopDeviceSessionLogInfo {
	s := new(DesktopDeviceSessionLogInfo)
	s.HostName = HostName
	s.ClientType = ClientType
	s.Platform = Platform
	s.IsDeleteOnUnlinkSupported = IsDeleteOnUnlinkSupported
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

// DeviceApprovalsChangeDesktopPolicyDetails : Set/removed limit on number of
// computers member can link to team Dropbox account.
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

// DeviceApprovalsChangeDesktopPolicyType : has no documentation (yet)
type DeviceApprovalsChangeDesktopPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceApprovalsChangeDesktopPolicyType returns a new DeviceApprovalsChangeDesktopPolicyType instance
func NewDeviceApprovalsChangeDesktopPolicyType(Description string) *DeviceApprovalsChangeDesktopPolicyType {
	s := new(DeviceApprovalsChangeDesktopPolicyType)
	s.Description = Description
	return s
}

// DeviceApprovalsChangeMobilePolicyDetails : Set/removed limit on number of
// mobile devices member can link to team Dropbox account.
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

// DeviceApprovalsChangeMobilePolicyType : has no documentation (yet)
type DeviceApprovalsChangeMobilePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceApprovalsChangeMobilePolicyType returns a new DeviceApprovalsChangeMobilePolicyType instance
func NewDeviceApprovalsChangeMobilePolicyType(Description string) *DeviceApprovalsChangeMobilePolicyType {
	s := new(DeviceApprovalsChangeMobilePolicyType)
	s.Description = Description
	return s
}

// DeviceApprovalsChangeOverageActionDetails : Changed device approvals setting
// when member is over limit.
type DeviceApprovalsChangeOverageActionDetails struct {
	// NewValue : New over the limits policy. Might be missing due to historical
	// data gap.
	NewValue *team_policies.RolloutMethod `json:"new_value,omitempty"`
	// PreviousValue : Previous over the limit policy. Might be missing due to
	// historical data gap.
	PreviousValue *team_policies.RolloutMethod `json:"previous_value,omitempty"`
}

// NewDeviceApprovalsChangeOverageActionDetails returns a new DeviceApprovalsChangeOverageActionDetails instance
func NewDeviceApprovalsChangeOverageActionDetails() *DeviceApprovalsChangeOverageActionDetails {
	s := new(DeviceApprovalsChangeOverageActionDetails)
	return s
}

// DeviceApprovalsChangeOverageActionType : has no documentation (yet)
type DeviceApprovalsChangeOverageActionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceApprovalsChangeOverageActionType returns a new DeviceApprovalsChangeOverageActionType instance
func NewDeviceApprovalsChangeOverageActionType(Description string) *DeviceApprovalsChangeOverageActionType {
	s := new(DeviceApprovalsChangeOverageActionType)
	s.Description = Description
	return s
}

// DeviceApprovalsChangeUnlinkActionDetails : Changed device approvals setting
// when member unlinks approved device.
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

// DeviceApprovalsChangeUnlinkActionType : has no documentation (yet)
type DeviceApprovalsChangeUnlinkActionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceApprovalsChangeUnlinkActionType returns a new DeviceApprovalsChangeUnlinkActionType instance
func NewDeviceApprovalsChangeUnlinkActionType(Description string) *DeviceApprovalsChangeUnlinkActionType {
	s := new(DeviceApprovalsChangeUnlinkActionType)
	s.Description = Description
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

// DeviceChangeIpDesktopDetails : Changed IP address associated with active
// desktop session.
type DeviceChangeIpDesktopDetails struct {
	// DeviceSessionInfo : Device's session logged information.
	DeviceSessionInfo IsDeviceSessionLogInfo `json:"device_session_info"`
}

// NewDeviceChangeIpDesktopDetails returns a new DeviceChangeIpDesktopDetails instance
func NewDeviceChangeIpDesktopDetails(DeviceSessionInfo IsDeviceSessionLogInfo) *DeviceChangeIpDesktopDetails {
	s := new(DeviceChangeIpDesktopDetails)
	s.DeviceSessionInfo = DeviceSessionInfo
	return s
}

// UnmarshalJSON deserializes into a DeviceChangeIpDesktopDetails instance
func (u *DeviceChangeIpDesktopDetails) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// DeviceSessionInfo : Device's session logged information.
		DeviceSessionInfo json.RawMessage `json:"device_session_info"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	DeviceSessionInfo, err := IsDeviceSessionLogInfoFromJSON(w.DeviceSessionInfo)
	if err != nil {
		return err
	}
	u.DeviceSessionInfo = DeviceSessionInfo
	return nil
}

// DeviceChangeIpDesktopType : has no documentation (yet)
type DeviceChangeIpDesktopType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceChangeIpDesktopType returns a new DeviceChangeIpDesktopType instance
func NewDeviceChangeIpDesktopType(Description string) *DeviceChangeIpDesktopType {
	s := new(DeviceChangeIpDesktopType)
	s.Description = Description
	return s
}

// DeviceChangeIpMobileDetails : Changed IP address associated with active
// mobile session.
type DeviceChangeIpMobileDetails struct {
	// DeviceSessionInfo : Device's session logged information.
	DeviceSessionInfo IsDeviceSessionLogInfo `json:"device_session_info,omitempty"`
}

// NewDeviceChangeIpMobileDetails returns a new DeviceChangeIpMobileDetails instance
func NewDeviceChangeIpMobileDetails() *DeviceChangeIpMobileDetails {
	s := new(DeviceChangeIpMobileDetails)
	return s
}

// DeviceChangeIpMobileType : has no documentation (yet)
type DeviceChangeIpMobileType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceChangeIpMobileType returns a new DeviceChangeIpMobileType instance
func NewDeviceChangeIpMobileType(Description string) *DeviceChangeIpMobileType {
	s := new(DeviceChangeIpMobileType)
	s.Description = Description
	return s
}

// DeviceChangeIpWebDetails : Changed IP address associated with active web
// session.
type DeviceChangeIpWebDetails struct {
	// UserAgent : Web browser name.
	UserAgent string `json:"user_agent"`
}

// NewDeviceChangeIpWebDetails returns a new DeviceChangeIpWebDetails instance
func NewDeviceChangeIpWebDetails(UserAgent string) *DeviceChangeIpWebDetails {
	s := new(DeviceChangeIpWebDetails)
	s.UserAgent = UserAgent
	return s
}

// DeviceChangeIpWebType : has no documentation (yet)
type DeviceChangeIpWebType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceChangeIpWebType returns a new DeviceChangeIpWebType instance
func NewDeviceChangeIpWebType(Description string) *DeviceChangeIpWebType {
	s := new(DeviceChangeIpWebType)
	s.Description = Description
	return s
}

// DeviceDeleteOnUnlinkFailDetails : Failed to delete all files from unlinked
// device.
type DeviceDeleteOnUnlinkFailDetails struct {
	// SessionInfo : Session unique id. Might be missing due to historical data
	// gap.
	SessionInfo IsSessionLogInfo `json:"session_info,omitempty"`
	// DisplayName : The device name. Might be missing due to historical data
	// gap.
	DisplayName string `json:"display_name,omitempty"`
	// NumFailures : The number of times that remote file deletion failed.
	NumFailures int64 `json:"num_failures"`
}

// NewDeviceDeleteOnUnlinkFailDetails returns a new DeviceDeleteOnUnlinkFailDetails instance
func NewDeviceDeleteOnUnlinkFailDetails(NumFailures int64) *DeviceDeleteOnUnlinkFailDetails {
	s := new(DeviceDeleteOnUnlinkFailDetails)
	s.NumFailures = NumFailures
	return s
}

// DeviceDeleteOnUnlinkFailType : has no documentation (yet)
type DeviceDeleteOnUnlinkFailType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceDeleteOnUnlinkFailType returns a new DeviceDeleteOnUnlinkFailType instance
func NewDeviceDeleteOnUnlinkFailType(Description string) *DeviceDeleteOnUnlinkFailType {
	s := new(DeviceDeleteOnUnlinkFailType)
	s.Description = Description
	return s
}

// DeviceDeleteOnUnlinkSuccessDetails : Deleted all files from unlinked device.
type DeviceDeleteOnUnlinkSuccessDetails struct {
	// SessionInfo : Session unique id. Might be missing due to historical data
	// gap.
	SessionInfo IsSessionLogInfo `json:"session_info,omitempty"`
	// DisplayName : The device name. Might be missing due to historical data
	// gap.
	DisplayName string `json:"display_name,omitempty"`
}

// NewDeviceDeleteOnUnlinkSuccessDetails returns a new DeviceDeleteOnUnlinkSuccessDetails instance
func NewDeviceDeleteOnUnlinkSuccessDetails() *DeviceDeleteOnUnlinkSuccessDetails {
	s := new(DeviceDeleteOnUnlinkSuccessDetails)
	return s
}

// DeviceDeleteOnUnlinkSuccessType : has no documentation (yet)
type DeviceDeleteOnUnlinkSuccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceDeleteOnUnlinkSuccessType returns a new DeviceDeleteOnUnlinkSuccessType instance
func NewDeviceDeleteOnUnlinkSuccessType(Description string) *DeviceDeleteOnUnlinkSuccessType {
	s := new(DeviceDeleteOnUnlinkSuccessType)
	s.Description = Description
	return s
}

// DeviceLinkFailDetails : Failed to link device.
type DeviceLinkFailDetails struct {
	// IpAddress : IP address. Might be missing due to historical data gap.
	IpAddress string `json:"ip_address,omitempty"`
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

// DeviceLinkFailType : has no documentation (yet)
type DeviceLinkFailType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceLinkFailType returns a new DeviceLinkFailType instance
func NewDeviceLinkFailType(Description string) *DeviceLinkFailType {
	s := new(DeviceLinkFailType)
	s.Description = Description
	return s
}

// DeviceLinkSuccessDetails : Linked device.
type DeviceLinkSuccessDetails struct {
	// DeviceSessionInfo : Device's session logged information.
	DeviceSessionInfo IsDeviceSessionLogInfo `json:"device_session_info,omitempty"`
}

// NewDeviceLinkSuccessDetails returns a new DeviceLinkSuccessDetails instance
func NewDeviceLinkSuccessDetails() *DeviceLinkSuccessDetails {
	s := new(DeviceLinkSuccessDetails)
	return s
}

// DeviceLinkSuccessType : has no documentation (yet)
type DeviceLinkSuccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceLinkSuccessType returns a new DeviceLinkSuccessType instance
func NewDeviceLinkSuccessType(Description string) *DeviceLinkSuccessType {
	s := new(DeviceLinkSuccessType)
	s.Description = Description
	return s
}

// DeviceManagementDisabledDetails : Disabled device management.
type DeviceManagementDisabledDetails struct {
}

// NewDeviceManagementDisabledDetails returns a new DeviceManagementDisabledDetails instance
func NewDeviceManagementDisabledDetails() *DeviceManagementDisabledDetails {
	s := new(DeviceManagementDisabledDetails)
	return s
}

// DeviceManagementDisabledType : has no documentation (yet)
type DeviceManagementDisabledType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceManagementDisabledType returns a new DeviceManagementDisabledType instance
func NewDeviceManagementDisabledType(Description string) *DeviceManagementDisabledType {
	s := new(DeviceManagementDisabledType)
	s.Description = Description
	return s
}

// DeviceManagementEnabledDetails : Enabled device management.
type DeviceManagementEnabledDetails struct {
}

// NewDeviceManagementEnabledDetails returns a new DeviceManagementEnabledDetails instance
func NewDeviceManagementEnabledDetails() *DeviceManagementEnabledDetails {
	s := new(DeviceManagementEnabledDetails)
	return s
}

// DeviceManagementEnabledType : has no documentation (yet)
type DeviceManagementEnabledType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceManagementEnabledType returns a new DeviceManagementEnabledType instance
func NewDeviceManagementEnabledType(Description string) *DeviceManagementEnabledType {
	s := new(DeviceManagementEnabledType)
	s.Description = Description
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

// DeviceUnlinkDetails : Disconnected device.
type DeviceUnlinkDetails struct {
	// SessionInfo : Session unique id.
	SessionInfo IsSessionLogInfo `json:"session_info,omitempty"`
	// DisplayName : The device name. Might be missing due to historical data
	// gap.
	DisplayName string `json:"display_name,omitempty"`
	// DeleteData : True if the user requested to delete data after device
	// unlink, false otherwise.
	DeleteData bool `json:"delete_data"`
}

// NewDeviceUnlinkDetails returns a new DeviceUnlinkDetails instance
func NewDeviceUnlinkDetails(DeleteData bool) *DeviceUnlinkDetails {
	s := new(DeviceUnlinkDetails)
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

// DeviceUnlinkType : has no documentation (yet)
type DeviceUnlinkType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDeviceUnlinkType returns a new DeviceUnlinkType instance
func NewDeviceUnlinkType(Description string) *DeviceUnlinkType {
	s := new(DeviceUnlinkType)
	s.Description = Description
	return s
}

// DirectoryRestrictionsAddMembersDetails : Added members to directory
// restrictions list.
type DirectoryRestrictionsAddMembersDetails struct {
}

// NewDirectoryRestrictionsAddMembersDetails returns a new DirectoryRestrictionsAddMembersDetails instance
func NewDirectoryRestrictionsAddMembersDetails() *DirectoryRestrictionsAddMembersDetails {
	s := new(DirectoryRestrictionsAddMembersDetails)
	return s
}

// DirectoryRestrictionsAddMembersType : has no documentation (yet)
type DirectoryRestrictionsAddMembersType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDirectoryRestrictionsAddMembersType returns a new DirectoryRestrictionsAddMembersType instance
func NewDirectoryRestrictionsAddMembersType(Description string) *DirectoryRestrictionsAddMembersType {
	s := new(DirectoryRestrictionsAddMembersType)
	s.Description = Description
	return s
}

// DirectoryRestrictionsRemoveMembersDetails : Removed members from directory
// restrictions list.
type DirectoryRestrictionsRemoveMembersDetails struct {
}

// NewDirectoryRestrictionsRemoveMembersDetails returns a new DirectoryRestrictionsRemoveMembersDetails instance
func NewDirectoryRestrictionsRemoveMembersDetails() *DirectoryRestrictionsRemoveMembersDetails {
	s := new(DirectoryRestrictionsRemoveMembersDetails)
	return s
}

// DirectoryRestrictionsRemoveMembersType : has no documentation (yet)
type DirectoryRestrictionsRemoveMembersType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDirectoryRestrictionsRemoveMembersType returns a new DirectoryRestrictionsRemoveMembersType instance
func NewDirectoryRestrictionsRemoveMembersType(Description string) *DirectoryRestrictionsRemoveMembersType {
	s := new(DirectoryRestrictionsRemoveMembersType)
	s.Description = Description
	return s
}

// DisabledDomainInvitesDetails : Disabled domain invites.
type DisabledDomainInvitesDetails struct {
}

// NewDisabledDomainInvitesDetails returns a new DisabledDomainInvitesDetails instance
func NewDisabledDomainInvitesDetails() *DisabledDomainInvitesDetails {
	s := new(DisabledDomainInvitesDetails)
	return s
}

// DisabledDomainInvitesType : has no documentation (yet)
type DisabledDomainInvitesType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDisabledDomainInvitesType returns a new DisabledDomainInvitesType instance
func NewDisabledDomainInvitesType(Description string) *DisabledDomainInvitesType {
	s := new(DisabledDomainInvitesType)
	s.Description = Description
	return s
}

// DomainInvitesApproveRequestToJoinTeamDetails : Approved user's request to
// join team.
type DomainInvitesApproveRequestToJoinTeamDetails struct {
}

// NewDomainInvitesApproveRequestToJoinTeamDetails returns a new DomainInvitesApproveRequestToJoinTeamDetails instance
func NewDomainInvitesApproveRequestToJoinTeamDetails() *DomainInvitesApproveRequestToJoinTeamDetails {
	s := new(DomainInvitesApproveRequestToJoinTeamDetails)
	return s
}

// DomainInvitesApproveRequestToJoinTeamType : has no documentation (yet)
type DomainInvitesApproveRequestToJoinTeamType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainInvitesApproveRequestToJoinTeamType returns a new DomainInvitesApproveRequestToJoinTeamType instance
func NewDomainInvitesApproveRequestToJoinTeamType(Description string) *DomainInvitesApproveRequestToJoinTeamType {
	s := new(DomainInvitesApproveRequestToJoinTeamType)
	s.Description = Description
	return s
}

// DomainInvitesDeclineRequestToJoinTeamDetails : Declined user's request to
// join team.
type DomainInvitesDeclineRequestToJoinTeamDetails struct {
}

// NewDomainInvitesDeclineRequestToJoinTeamDetails returns a new DomainInvitesDeclineRequestToJoinTeamDetails instance
func NewDomainInvitesDeclineRequestToJoinTeamDetails() *DomainInvitesDeclineRequestToJoinTeamDetails {
	s := new(DomainInvitesDeclineRequestToJoinTeamDetails)
	return s
}

// DomainInvitesDeclineRequestToJoinTeamType : has no documentation (yet)
type DomainInvitesDeclineRequestToJoinTeamType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainInvitesDeclineRequestToJoinTeamType returns a new DomainInvitesDeclineRequestToJoinTeamType instance
func NewDomainInvitesDeclineRequestToJoinTeamType(Description string) *DomainInvitesDeclineRequestToJoinTeamType {
	s := new(DomainInvitesDeclineRequestToJoinTeamType)
	s.Description = Description
	return s
}

// DomainInvitesEmailExistingUsersDetails : Sent domain invites to existing
// domain accounts.
type DomainInvitesEmailExistingUsersDetails struct {
	// DomainName : Domain names.
	DomainName string `json:"domain_name"`
	// NumRecipients : Number of recipients.
	NumRecipients uint64 `json:"num_recipients"`
}

// NewDomainInvitesEmailExistingUsersDetails returns a new DomainInvitesEmailExistingUsersDetails instance
func NewDomainInvitesEmailExistingUsersDetails(DomainName string, NumRecipients uint64) *DomainInvitesEmailExistingUsersDetails {
	s := new(DomainInvitesEmailExistingUsersDetails)
	s.DomainName = DomainName
	s.NumRecipients = NumRecipients
	return s
}

// DomainInvitesEmailExistingUsersType : has no documentation (yet)
type DomainInvitesEmailExistingUsersType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainInvitesEmailExistingUsersType returns a new DomainInvitesEmailExistingUsersType instance
func NewDomainInvitesEmailExistingUsersType(Description string) *DomainInvitesEmailExistingUsersType {
	s := new(DomainInvitesEmailExistingUsersType)
	s.Description = Description
	return s
}

// DomainInvitesRequestToJoinTeamDetails : Requested to join team.
type DomainInvitesRequestToJoinTeamDetails struct {
}

// NewDomainInvitesRequestToJoinTeamDetails returns a new DomainInvitesRequestToJoinTeamDetails instance
func NewDomainInvitesRequestToJoinTeamDetails() *DomainInvitesRequestToJoinTeamDetails {
	s := new(DomainInvitesRequestToJoinTeamDetails)
	return s
}

// DomainInvitesRequestToJoinTeamType : has no documentation (yet)
type DomainInvitesRequestToJoinTeamType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainInvitesRequestToJoinTeamType returns a new DomainInvitesRequestToJoinTeamType instance
func NewDomainInvitesRequestToJoinTeamType(Description string) *DomainInvitesRequestToJoinTeamType {
	s := new(DomainInvitesRequestToJoinTeamType)
	s.Description = Description
	return s
}

// DomainInvitesSetInviteNewUserPrefToNoDetails : Disabled "Automatically invite
// new users".
type DomainInvitesSetInviteNewUserPrefToNoDetails struct {
}

// NewDomainInvitesSetInviteNewUserPrefToNoDetails returns a new DomainInvitesSetInviteNewUserPrefToNoDetails instance
func NewDomainInvitesSetInviteNewUserPrefToNoDetails() *DomainInvitesSetInviteNewUserPrefToNoDetails {
	s := new(DomainInvitesSetInviteNewUserPrefToNoDetails)
	return s
}

// DomainInvitesSetInviteNewUserPrefToNoType : has no documentation (yet)
type DomainInvitesSetInviteNewUserPrefToNoType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainInvitesSetInviteNewUserPrefToNoType returns a new DomainInvitesSetInviteNewUserPrefToNoType instance
func NewDomainInvitesSetInviteNewUserPrefToNoType(Description string) *DomainInvitesSetInviteNewUserPrefToNoType {
	s := new(DomainInvitesSetInviteNewUserPrefToNoType)
	s.Description = Description
	return s
}

// DomainInvitesSetInviteNewUserPrefToYesDetails : Enabled "Automatically invite
// new users".
type DomainInvitesSetInviteNewUserPrefToYesDetails struct {
}

// NewDomainInvitesSetInviteNewUserPrefToYesDetails returns a new DomainInvitesSetInviteNewUserPrefToYesDetails instance
func NewDomainInvitesSetInviteNewUserPrefToYesDetails() *DomainInvitesSetInviteNewUserPrefToYesDetails {
	s := new(DomainInvitesSetInviteNewUserPrefToYesDetails)
	return s
}

// DomainInvitesSetInviteNewUserPrefToYesType : has no documentation (yet)
type DomainInvitesSetInviteNewUserPrefToYesType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainInvitesSetInviteNewUserPrefToYesType returns a new DomainInvitesSetInviteNewUserPrefToYesType instance
func NewDomainInvitesSetInviteNewUserPrefToYesType(Description string) *DomainInvitesSetInviteNewUserPrefToYesType {
	s := new(DomainInvitesSetInviteNewUserPrefToYesType)
	s.Description = Description
	return s
}

// DomainVerificationAddDomainFailDetails : Failed to verify team domain.
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

// DomainVerificationAddDomainFailType : has no documentation (yet)
type DomainVerificationAddDomainFailType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainVerificationAddDomainFailType returns a new DomainVerificationAddDomainFailType instance
func NewDomainVerificationAddDomainFailType(Description string) *DomainVerificationAddDomainFailType {
	s := new(DomainVerificationAddDomainFailType)
	s.Description = Description
	return s
}

// DomainVerificationAddDomainSuccessDetails : Verified team domain.
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

// DomainVerificationAddDomainSuccessType : has no documentation (yet)
type DomainVerificationAddDomainSuccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainVerificationAddDomainSuccessType returns a new DomainVerificationAddDomainSuccessType instance
func NewDomainVerificationAddDomainSuccessType(Description string) *DomainVerificationAddDomainSuccessType {
	s := new(DomainVerificationAddDomainSuccessType)
	s.Description = Description
	return s
}

// DomainVerificationRemoveDomainDetails : Removed domain from list of verified
// team domains.
type DomainVerificationRemoveDomainDetails struct {
	// DomainNames : Domain names.
	DomainNames []string `json:"domain_names"`
}

// NewDomainVerificationRemoveDomainDetails returns a new DomainVerificationRemoveDomainDetails instance
func NewDomainVerificationRemoveDomainDetails(DomainNames []string) *DomainVerificationRemoveDomainDetails {
	s := new(DomainVerificationRemoveDomainDetails)
	s.DomainNames = DomainNames
	return s
}

// DomainVerificationRemoveDomainType : has no documentation (yet)
type DomainVerificationRemoveDomainType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewDomainVerificationRemoveDomainType returns a new DomainVerificationRemoveDomainType instance
func NewDomainVerificationRemoveDomainType(Description string) *DomainVerificationRemoveDomainType {
	s := new(DomainVerificationRemoveDomainType)
	s.Description = Description
	return s
}

// DownloadPolicyType : Shared content downloads policy
type DownloadPolicyType struct {
	dropbox.Tagged
}

// Valid tag values for DownloadPolicyType
const (
	DownloadPolicyTypeAllow    = "allow"
	DownloadPolicyTypeDisallow = "disallow"
	DownloadPolicyTypeOther    = "other"
)

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

// EmmAddExceptionDetails : Added members to EMM exception list.
type EmmAddExceptionDetails struct {
}

// NewEmmAddExceptionDetails returns a new EmmAddExceptionDetails instance
func NewEmmAddExceptionDetails() *EmmAddExceptionDetails {
	s := new(EmmAddExceptionDetails)
	return s
}

// EmmAddExceptionType : has no documentation (yet)
type EmmAddExceptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmAddExceptionType returns a new EmmAddExceptionType instance
func NewEmmAddExceptionType(Description string) *EmmAddExceptionType {
	s := new(EmmAddExceptionType)
	s.Description = Description
	return s
}

// EmmChangePolicyDetails : Enabled/disabled enterprise mobility management for
// members.
type EmmChangePolicyDetails struct {
	// NewValue : New enterprise mobility management policy.
	NewValue *team_policies.EmmState `json:"new_value"`
	// PreviousValue : Previous enterprise mobility management policy. Might be
	// missing due to historical data gap.
	PreviousValue *team_policies.EmmState `json:"previous_value,omitempty"`
}

// NewEmmChangePolicyDetails returns a new EmmChangePolicyDetails instance
func NewEmmChangePolicyDetails(NewValue *team_policies.EmmState) *EmmChangePolicyDetails {
	s := new(EmmChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// EmmChangePolicyType : has no documentation (yet)
type EmmChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmChangePolicyType returns a new EmmChangePolicyType instance
func NewEmmChangePolicyType(Description string) *EmmChangePolicyType {
	s := new(EmmChangePolicyType)
	s.Description = Description
	return s
}

// EmmCreateExceptionsReportDetails : Created EMM-excluded users report.
type EmmCreateExceptionsReportDetails struct {
}

// NewEmmCreateExceptionsReportDetails returns a new EmmCreateExceptionsReportDetails instance
func NewEmmCreateExceptionsReportDetails() *EmmCreateExceptionsReportDetails {
	s := new(EmmCreateExceptionsReportDetails)
	return s
}

// EmmCreateExceptionsReportType : has no documentation (yet)
type EmmCreateExceptionsReportType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmCreateExceptionsReportType returns a new EmmCreateExceptionsReportType instance
func NewEmmCreateExceptionsReportType(Description string) *EmmCreateExceptionsReportType {
	s := new(EmmCreateExceptionsReportType)
	s.Description = Description
	return s
}

// EmmCreateUsageReportDetails : Created EMM mobile app usage report.
type EmmCreateUsageReportDetails struct {
}

// NewEmmCreateUsageReportDetails returns a new EmmCreateUsageReportDetails instance
func NewEmmCreateUsageReportDetails() *EmmCreateUsageReportDetails {
	s := new(EmmCreateUsageReportDetails)
	return s
}

// EmmCreateUsageReportType : has no documentation (yet)
type EmmCreateUsageReportType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmCreateUsageReportType returns a new EmmCreateUsageReportType instance
func NewEmmCreateUsageReportType(Description string) *EmmCreateUsageReportType {
	s := new(EmmCreateUsageReportType)
	s.Description = Description
	return s
}

// EmmErrorDetails : Failed to sign in via EMM.
type EmmErrorDetails struct {
	// ErrorDetails : Error details.
	ErrorDetails *FailureDetailsLogInfo `json:"error_details"`
}

// NewEmmErrorDetails returns a new EmmErrorDetails instance
func NewEmmErrorDetails(ErrorDetails *FailureDetailsLogInfo) *EmmErrorDetails {
	s := new(EmmErrorDetails)
	s.ErrorDetails = ErrorDetails
	return s
}

// EmmErrorType : has no documentation (yet)
type EmmErrorType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmErrorType returns a new EmmErrorType instance
func NewEmmErrorType(Description string) *EmmErrorType {
	s := new(EmmErrorType)
	s.Description = Description
	return s
}

// EmmRefreshAuthTokenDetails : Refreshed auth token used for setting up
// enterprise mobility management.
type EmmRefreshAuthTokenDetails struct {
}

// NewEmmRefreshAuthTokenDetails returns a new EmmRefreshAuthTokenDetails instance
func NewEmmRefreshAuthTokenDetails() *EmmRefreshAuthTokenDetails {
	s := new(EmmRefreshAuthTokenDetails)
	return s
}

// EmmRefreshAuthTokenType : has no documentation (yet)
type EmmRefreshAuthTokenType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmRefreshAuthTokenType returns a new EmmRefreshAuthTokenType instance
func NewEmmRefreshAuthTokenType(Description string) *EmmRefreshAuthTokenType {
	s := new(EmmRefreshAuthTokenType)
	s.Description = Description
	return s
}

// EmmRemoveExceptionDetails : Removed members from EMM exception list.
type EmmRemoveExceptionDetails struct {
}

// NewEmmRemoveExceptionDetails returns a new EmmRemoveExceptionDetails instance
func NewEmmRemoveExceptionDetails() *EmmRemoveExceptionDetails {
	s := new(EmmRemoveExceptionDetails)
	return s
}

// EmmRemoveExceptionType : has no documentation (yet)
type EmmRemoveExceptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEmmRemoveExceptionType returns a new EmmRemoveExceptionType instance
func NewEmmRemoveExceptionType(Description string) *EmmRemoveExceptionType {
	s := new(EmmRemoveExceptionType)
	s.Description = Description
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

// EnabledDomainInvitesType : has no documentation (yet)
type EnabledDomainInvitesType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewEnabledDomainInvitesType returns a new EnabledDomainInvitesType instance
func NewEnabledDomainInvitesType(Description string) *EnabledDomainInvitesType {
	s := new(EnabledDomainInvitesType)
	s.Description = Description
	return s
}

// EventCategory : Category of events in event audit log.
type EventCategory struct {
	dropbox.Tagged
}

// Valid tag values for EventCategory
const (
	EventCategoryApps           = "apps"
	EventCategoryComments       = "comments"
	EventCategoryDevices        = "devices"
	EventCategoryDomains        = "domains"
	EventCategoryFileOperations = "file_operations"
	EventCategoryFileRequests   = "file_requests"
	EventCategoryGroups         = "groups"
	EventCategoryLogins         = "logins"
	EventCategoryMembers        = "members"
	EventCategoryPaper          = "paper"
	EventCategoryPasswords      = "passwords"
	EventCategoryReports        = "reports"
	EventCategorySharing        = "sharing"
	EventCategoryShowcase       = "showcase"
	EventCategorySso            = "sso"
	EventCategoryTeamFolders    = "team_folders"
	EventCategoryTeamPolicies   = "team_policies"
	EventCategoryTeamProfile    = "team_profile"
	EventCategoryTfa            = "tfa"
	EventCategoryOther          = "other"
)

// EventDetails : Additional fields depending on the event type.
type EventDetails struct {
	dropbox.Tagged
	// AppLinkTeamDetails : has no documentation (yet)
	AppLinkTeamDetails *AppLinkTeamDetails `json:"app_link_team_details,omitempty"`
	// AppLinkUserDetails : has no documentation (yet)
	AppLinkUserDetails *AppLinkUserDetails `json:"app_link_user_details,omitempty"`
	// AppUnlinkTeamDetails : has no documentation (yet)
	AppUnlinkTeamDetails *AppUnlinkTeamDetails `json:"app_unlink_team_details,omitempty"`
	// AppUnlinkUserDetails : has no documentation (yet)
	AppUnlinkUserDetails *AppUnlinkUserDetails `json:"app_unlink_user_details,omitempty"`
	// FileAddCommentDetails : has no documentation (yet)
	FileAddCommentDetails *FileAddCommentDetails `json:"file_add_comment_details,omitempty"`
	// FileChangeCommentSubscriptionDetails : has no documentation (yet)
	FileChangeCommentSubscriptionDetails *FileChangeCommentSubscriptionDetails `json:"file_change_comment_subscription_details,omitempty"`
	// FileDeleteCommentDetails : has no documentation (yet)
	FileDeleteCommentDetails *FileDeleteCommentDetails `json:"file_delete_comment_details,omitempty"`
	// FileLikeCommentDetails : has no documentation (yet)
	FileLikeCommentDetails *FileLikeCommentDetails `json:"file_like_comment_details,omitempty"`
	// FileResolveCommentDetails : has no documentation (yet)
	FileResolveCommentDetails *FileResolveCommentDetails `json:"file_resolve_comment_details,omitempty"`
	// FileUnlikeCommentDetails : has no documentation (yet)
	FileUnlikeCommentDetails *FileUnlikeCommentDetails `json:"file_unlike_comment_details,omitempty"`
	// FileUnresolveCommentDetails : has no documentation (yet)
	FileUnresolveCommentDetails *FileUnresolveCommentDetails `json:"file_unresolve_comment_details,omitempty"`
	// DeviceChangeIpDesktopDetails : has no documentation (yet)
	DeviceChangeIpDesktopDetails *DeviceChangeIpDesktopDetails `json:"device_change_ip_desktop_details,omitempty"`
	// DeviceChangeIpMobileDetails : has no documentation (yet)
	DeviceChangeIpMobileDetails *DeviceChangeIpMobileDetails `json:"device_change_ip_mobile_details,omitempty"`
	// DeviceChangeIpWebDetails : has no documentation (yet)
	DeviceChangeIpWebDetails *DeviceChangeIpWebDetails `json:"device_change_ip_web_details,omitempty"`
	// DeviceDeleteOnUnlinkFailDetails : has no documentation (yet)
	DeviceDeleteOnUnlinkFailDetails *DeviceDeleteOnUnlinkFailDetails `json:"device_delete_on_unlink_fail_details,omitempty"`
	// DeviceDeleteOnUnlinkSuccessDetails : has no documentation (yet)
	DeviceDeleteOnUnlinkSuccessDetails *DeviceDeleteOnUnlinkSuccessDetails `json:"device_delete_on_unlink_success_details,omitempty"`
	// DeviceLinkFailDetails : has no documentation (yet)
	DeviceLinkFailDetails *DeviceLinkFailDetails `json:"device_link_fail_details,omitempty"`
	// DeviceLinkSuccessDetails : has no documentation (yet)
	DeviceLinkSuccessDetails *DeviceLinkSuccessDetails `json:"device_link_success_details,omitempty"`
	// DeviceManagementDisabledDetails : has no documentation (yet)
	DeviceManagementDisabledDetails *DeviceManagementDisabledDetails `json:"device_management_disabled_details,omitempty"`
	// DeviceManagementEnabledDetails : has no documentation (yet)
	DeviceManagementEnabledDetails *DeviceManagementEnabledDetails `json:"device_management_enabled_details,omitempty"`
	// DeviceUnlinkDetails : has no documentation (yet)
	DeviceUnlinkDetails *DeviceUnlinkDetails `json:"device_unlink_details,omitempty"`
	// EmmRefreshAuthTokenDetails : has no documentation (yet)
	EmmRefreshAuthTokenDetails *EmmRefreshAuthTokenDetails `json:"emm_refresh_auth_token_details,omitempty"`
	// AccountCaptureChangeAvailabilityDetails : has no documentation (yet)
	AccountCaptureChangeAvailabilityDetails *AccountCaptureChangeAvailabilityDetails `json:"account_capture_change_availability_details,omitempty"`
	// AccountCaptureMigrateAccountDetails : has no documentation (yet)
	AccountCaptureMigrateAccountDetails *AccountCaptureMigrateAccountDetails `json:"account_capture_migrate_account_details,omitempty"`
	// AccountCaptureNotificationEmailsSentDetails : has no documentation (yet)
	AccountCaptureNotificationEmailsSentDetails *AccountCaptureNotificationEmailsSentDetails `json:"account_capture_notification_emails_sent_details,omitempty"`
	// AccountCaptureRelinquishAccountDetails : has no documentation (yet)
	AccountCaptureRelinquishAccountDetails *AccountCaptureRelinquishAccountDetails `json:"account_capture_relinquish_account_details,omitempty"`
	// DisabledDomainInvitesDetails : has no documentation (yet)
	DisabledDomainInvitesDetails *DisabledDomainInvitesDetails `json:"disabled_domain_invites_details,omitempty"`
	// DomainInvitesApproveRequestToJoinTeamDetails : has no documentation (yet)
	DomainInvitesApproveRequestToJoinTeamDetails *DomainInvitesApproveRequestToJoinTeamDetails `json:"domain_invites_approve_request_to_join_team_details,omitempty"`
	// DomainInvitesDeclineRequestToJoinTeamDetails : has no documentation (yet)
	DomainInvitesDeclineRequestToJoinTeamDetails *DomainInvitesDeclineRequestToJoinTeamDetails `json:"domain_invites_decline_request_to_join_team_details,omitempty"`
	// DomainInvitesEmailExistingUsersDetails : has no documentation (yet)
	DomainInvitesEmailExistingUsersDetails *DomainInvitesEmailExistingUsersDetails `json:"domain_invites_email_existing_users_details,omitempty"`
	// DomainInvitesRequestToJoinTeamDetails : has no documentation (yet)
	DomainInvitesRequestToJoinTeamDetails *DomainInvitesRequestToJoinTeamDetails `json:"domain_invites_request_to_join_team_details,omitempty"`
	// DomainInvitesSetInviteNewUserPrefToNoDetails : has no documentation (yet)
	DomainInvitesSetInviteNewUserPrefToNoDetails *DomainInvitesSetInviteNewUserPrefToNoDetails `json:"domain_invites_set_invite_new_user_pref_to_no_details,omitempty"`
	// DomainInvitesSetInviteNewUserPrefToYesDetails : has no documentation
	// (yet)
	DomainInvitesSetInviteNewUserPrefToYesDetails *DomainInvitesSetInviteNewUserPrefToYesDetails `json:"domain_invites_set_invite_new_user_pref_to_yes_details,omitempty"`
	// DomainVerificationAddDomainFailDetails : has no documentation (yet)
	DomainVerificationAddDomainFailDetails *DomainVerificationAddDomainFailDetails `json:"domain_verification_add_domain_fail_details,omitempty"`
	// DomainVerificationAddDomainSuccessDetails : has no documentation (yet)
	DomainVerificationAddDomainSuccessDetails *DomainVerificationAddDomainSuccessDetails `json:"domain_verification_add_domain_success_details,omitempty"`
	// DomainVerificationRemoveDomainDetails : has no documentation (yet)
	DomainVerificationRemoveDomainDetails *DomainVerificationRemoveDomainDetails `json:"domain_verification_remove_domain_details,omitempty"`
	// EnabledDomainInvitesDetails : has no documentation (yet)
	EnabledDomainInvitesDetails *EnabledDomainInvitesDetails `json:"enabled_domain_invites_details,omitempty"`
	// CreateFolderDetails : has no documentation (yet)
	CreateFolderDetails *CreateFolderDetails `json:"create_folder_details,omitempty"`
	// FileAddDetails : has no documentation (yet)
	FileAddDetails *FileAddDetails `json:"file_add_details,omitempty"`
	// FileCopyDetails : has no documentation (yet)
	FileCopyDetails *FileCopyDetails `json:"file_copy_details,omitempty"`
	// FileDeleteDetails : has no documentation (yet)
	FileDeleteDetails *FileDeleteDetails `json:"file_delete_details,omitempty"`
	// FileDownloadDetails : has no documentation (yet)
	FileDownloadDetails *FileDownloadDetails `json:"file_download_details,omitempty"`
	// FileEditDetails : has no documentation (yet)
	FileEditDetails *FileEditDetails `json:"file_edit_details,omitempty"`
	// FileGetCopyReferenceDetails : has no documentation (yet)
	FileGetCopyReferenceDetails *FileGetCopyReferenceDetails `json:"file_get_copy_reference_details,omitempty"`
	// FileMoveDetails : has no documentation (yet)
	FileMoveDetails *FileMoveDetails `json:"file_move_details,omitempty"`
	// FilePermanentlyDeleteDetails : has no documentation (yet)
	FilePermanentlyDeleteDetails *FilePermanentlyDeleteDetails `json:"file_permanently_delete_details,omitempty"`
	// FilePreviewDetails : has no documentation (yet)
	FilePreviewDetails *FilePreviewDetails `json:"file_preview_details,omitempty"`
	// FileRenameDetails : has no documentation (yet)
	FileRenameDetails *FileRenameDetails `json:"file_rename_details,omitempty"`
	// FileRestoreDetails : has no documentation (yet)
	FileRestoreDetails *FileRestoreDetails `json:"file_restore_details,omitempty"`
	// FileRevertDetails : has no documentation (yet)
	FileRevertDetails *FileRevertDetails `json:"file_revert_details,omitempty"`
	// FileRollbackChangesDetails : has no documentation (yet)
	FileRollbackChangesDetails *FileRollbackChangesDetails `json:"file_rollback_changes_details,omitempty"`
	// FileSaveCopyReferenceDetails : has no documentation (yet)
	FileSaveCopyReferenceDetails *FileSaveCopyReferenceDetails `json:"file_save_copy_reference_details,omitempty"`
	// FileRequestChangeDetails : has no documentation (yet)
	FileRequestChangeDetails *FileRequestChangeDetails `json:"file_request_change_details,omitempty"`
	// FileRequestCloseDetails : has no documentation (yet)
	FileRequestCloseDetails *FileRequestCloseDetails `json:"file_request_close_details,omitempty"`
	// FileRequestCreateDetails : has no documentation (yet)
	FileRequestCreateDetails *FileRequestCreateDetails `json:"file_request_create_details,omitempty"`
	// FileRequestReceiveFileDetails : has no documentation (yet)
	FileRequestReceiveFileDetails *FileRequestReceiveFileDetails `json:"file_request_receive_file_details,omitempty"`
	// GroupAddExternalIdDetails : has no documentation (yet)
	GroupAddExternalIdDetails *GroupAddExternalIdDetails `json:"group_add_external_id_details,omitempty"`
	// GroupAddMemberDetails : has no documentation (yet)
	GroupAddMemberDetails *GroupAddMemberDetails `json:"group_add_member_details,omitempty"`
	// GroupChangeExternalIdDetails : has no documentation (yet)
	GroupChangeExternalIdDetails *GroupChangeExternalIdDetails `json:"group_change_external_id_details,omitempty"`
	// GroupChangeManagementTypeDetails : has no documentation (yet)
	GroupChangeManagementTypeDetails *GroupChangeManagementTypeDetails `json:"group_change_management_type_details,omitempty"`
	// GroupChangeMemberRoleDetails : has no documentation (yet)
	GroupChangeMemberRoleDetails *GroupChangeMemberRoleDetails `json:"group_change_member_role_details,omitempty"`
	// GroupCreateDetails : has no documentation (yet)
	GroupCreateDetails *GroupCreateDetails `json:"group_create_details,omitempty"`
	// GroupDeleteDetails : has no documentation (yet)
	GroupDeleteDetails *GroupDeleteDetails `json:"group_delete_details,omitempty"`
	// GroupDescriptionUpdatedDetails : has no documentation (yet)
	GroupDescriptionUpdatedDetails *GroupDescriptionUpdatedDetails `json:"group_description_updated_details,omitempty"`
	// GroupJoinPolicyUpdatedDetails : has no documentation (yet)
	GroupJoinPolicyUpdatedDetails *GroupJoinPolicyUpdatedDetails `json:"group_join_policy_updated_details,omitempty"`
	// GroupMovedDetails : has no documentation (yet)
	GroupMovedDetails *GroupMovedDetails `json:"group_moved_details,omitempty"`
	// GroupRemoveExternalIdDetails : has no documentation (yet)
	GroupRemoveExternalIdDetails *GroupRemoveExternalIdDetails `json:"group_remove_external_id_details,omitempty"`
	// GroupRemoveMemberDetails : has no documentation (yet)
	GroupRemoveMemberDetails *GroupRemoveMemberDetails `json:"group_remove_member_details,omitempty"`
	// GroupRenameDetails : has no documentation (yet)
	GroupRenameDetails *GroupRenameDetails `json:"group_rename_details,omitempty"`
	// EmmErrorDetails : has no documentation (yet)
	EmmErrorDetails *EmmErrorDetails `json:"emm_error_details,omitempty"`
	// LoginFailDetails : has no documentation (yet)
	LoginFailDetails *LoginFailDetails `json:"login_fail_details,omitempty"`
	// LoginSuccessDetails : has no documentation (yet)
	LoginSuccessDetails *LoginSuccessDetails `json:"login_success_details,omitempty"`
	// LogoutDetails : has no documentation (yet)
	LogoutDetails *LogoutDetails `json:"logout_details,omitempty"`
	// ResellerSupportSessionEndDetails : has no documentation (yet)
	ResellerSupportSessionEndDetails *ResellerSupportSessionEndDetails `json:"reseller_support_session_end_details,omitempty"`
	// ResellerSupportSessionStartDetails : has no documentation (yet)
	ResellerSupportSessionStartDetails *ResellerSupportSessionStartDetails `json:"reseller_support_session_start_details,omitempty"`
	// SignInAsSessionEndDetails : has no documentation (yet)
	SignInAsSessionEndDetails *SignInAsSessionEndDetails `json:"sign_in_as_session_end_details,omitempty"`
	// SignInAsSessionStartDetails : has no documentation (yet)
	SignInAsSessionStartDetails *SignInAsSessionStartDetails `json:"sign_in_as_session_start_details,omitempty"`
	// SsoErrorDetails : has no documentation (yet)
	SsoErrorDetails *SsoErrorDetails `json:"sso_error_details,omitempty"`
	// MemberAddNameDetails : has no documentation (yet)
	MemberAddNameDetails *MemberAddNameDetails `json:"member_add_name_details,omitempty"`
	// MemberChangeAdminRoleDetails : has no documentation (yet)
	MemberChangeAdminRoleDetails *MemberChangeAdminRoleDetails `json:"member_change_admin_role_details,omitempty"`
	// MemberChangeEmailDetails : has no documentation (yet)
	MemberChangeEmailDetails *MemberChangeEmailDetails `json:"member_change_email_details,omitempty"`
	// MemberChangeMembershipTypeDetails : has no documentation (yet)
	MemberChangeMembershipTypeDetails *MemberChangeMembershipTypeDetails `json:"member_change_membership_type_details,omitempty"`
	// MemberChangeNameDetails : has no documentation (yet)
	MemberChangeNameDetails *MemberChangeNameDetails `json:"member_change_name_details,omitempty"`
	// MemberChangeStatusDetails : has no documentation (yet)
	MemberChangeStatusDetails *MemberChangeStatusDetails `json:"member_change_status_details,omitempty"`
	// MemberPermanentlyDeleteAccountContentsDetails : has no documentation
	// (yet)
	MemberPermanentlyDeleteAccountContentsDetails *MemberPermanentlyDeleteAccountContentsDetails `json:"member_permanently_delete_account_contents_details,omitempty"`
	// MemberSpaceLimitsAddCustomQuotaDetails : has no documentation (yet)
	MemberSpaceLimitsAddCustomQuotaDetails *MemberSpaceLimitsAddCustomQuotaDetails `json:"member_space_limits_add_custom_quota_details,omitempty"`
	// MemberSpaceLimitsChangeCustomQuotaDetails : has no documentation (yet)
	MemberSpaceLimitsChangeCustomQuotaDetails *MemberSpaceLimitsChangeCustomQuotaDetails `json:"member_space_limits_change_custom_quota_details,omitempty"`
	// MemberSpaceLimitsChangeStatusDetails : has no documentation (yet)
	MemberSpaceLimitsChangeStatusDetails *MemberSpaceLimitsChangeStatusDetails `json:"member_space_limits_change_status_details,omitempty"`
	// MemberSpaceLimitsRemoveCustomQuotaDetails : has no documentation (yet)
	MemberSpaceLimitsRemoveCustomQuotaDetails *MemberSpaceLimitsRemoveCustomQuotaDetails `json:"member_space_limits_remove_custom_quota_details,omitempty"`
	// MemberSuggestDetails : has no documentation (yet)
	MemberSuggestDetails *MemberSuggestDetails `json:"member_suggest_details,omitempty"`
	// MemberTransferAccountContentsDetails : has no documentation (yet)
	MemberTransferAccountContentsDetails *MemberTransferAccountContentsDetails `json:"member_transfer_account_contents_details,omitempty"`
	// SecondaryMailsPolicyChangedDetails : has no documentation (yet)
	SecondaryMailsPolicyChangedDetails *SecondaryMailsPolicyChangedDetails `json:"secondary_mails_policy_changed_details,omitempty"`
	// PaperContentAddMemberDetails : has no documentation (yet)
	PaperContentAddMemberDetails *PaperContentAddMemberDetails `json:"paper_content_add_member_details,omitempty"`
	// PaperContentAddToFolderDetails : has no documentation (yet)
	PaperContentAddToFolderDetails *PaperContentAddToFolderDetails `json:"paper_content_add_to_folder_details,omitempty"`
	// PaperContentArchiveDetails : has no documentation (yet)
	PaperContentArchiveDetails *PaperContentArchiveDetails `json:"paper_content_archive_details,omitempty"`
	// PaperContentCreateDetails : has no documentation (yet)
	PaperContentCreateDetails *PaperContentCreateDetails `json:"paper_content_create_details,omitempty"`
	// PaperContentPermanentlyDeleteDetails : has no documentation (yet)
	PaperContentPermanentlyDeleteDetails *PaperContentPermanentlyDeleteDetails `json:"paper_content_permanently_delete_details,omitempty"`
	// PaperContentRemoveFromFolderDetails : has no documentation (yet)
	PaperContentRemoveFromFolderDetails *PaperContentRemoveFromFolderDetails `json:"paper_content_remove_from_folder_details,omitempty"`
	// PaperContentRemoveMemberDetails : has no documentation (yet)
	PaperContentRemoveMemberDetails *PaperContentRemoveMemberDetails `json:"paper_content_remove_member_details,omitempty"`
	// PaperContentRenameDetails : has no documentation (yet)
	PaperContentRenameDetails *PaperContentRenameDetails `json:"paper_content_rename_details,omitempty"`
	// PaperContentRestoreDetails : has no documentation (yet)
	PaperContentRestoreDetails *PaperContentRestoreDetails `json:"paper_content_restore_details,omitempty"`
	// PaperDocAddCommentDetails : has no documentation (yet)
	PaperDocAddCommentDetails *PaperDocAddCommentDetails `json:"paper_doc_add_comment_details,omitempty"`
	// PaperDocChangeMemberRoleDetails : has no documentation (yet)
	PaperDocChangeMemberRoleDetails *PaperDocChangeMemberRoleDetails `json:"paper_doc_change_member_role_details,omitempty"`
	// PaperDocChangeSharingPolicyDetails : has no documentation (yet)
	PaperDocChangeSharingPolicyDetails *PaperDocChangeSharingPolicyDetails `json:"paper_doc_change_sharing_policy_details,omitempty"`
	// PaperDocChangeSubscriptionDetails : has no documentation (yet)
	PaperDocChangeSubscriptionDetails *PaperDocChangeSubscriptionDetails `json:"paper_doc_change_subscription_details,omitempty"`
	// PaperDocDeletedDetails : has no documentation (yet)
	PaperDocDeletedDetails *PaperDocDeletedDetails `json:"paper_doc_deleted_details,omitempty"`
	// PaperDocDeleteCommentDetails : has no documentation (yet)
	PaperDocDeleteCommentDetails *PaperDocDeleteCommentDetails `json:"paper_doc_delete_comment_details,omitempty"`
	// PaperDocDownloadDetails : has no documentation (yet)
	PaperDocDownloadDetails *PaperDocDownloadDetails `json:"paper_doc_download_details,omitempty"`
	// PaperDocEditDetails : has no documentation (yet)
	PaperDocEditDetails *PaperDocEditDetails `json:"paper_doc_edit_details,omitempty"`
	// PaperDocEditCommentDetails : has no documentation (yet)
	PaperDocEditCommentDetails *PaperDocEditCommentDetails `json:"paper_doc_edit_comment_details,omitempty"`
	// PaperDocFollowedDetails : has no documentation (yet)
	PaperDocFollowedDetails *PaperDocFollowedDetails `json:"paper_doc_followed_details,omitempty"`
	// PaperDocMentionDetails : has no documentation (yet)
	PaperDocMentionDetails *PaperDocMentionDetails `json:"paper_doc_mention_details,omitempty"`
	// PaperDocRequestAccessDetails : has no documentation (yet)
	PaperDocRequestAccessDetails *PaperDocRequestAccessDetails `json:"paper_doc_request_access_details,omitempty"`
	// PaperDocResolveCommentDetails : has no documentation (yet)
	PaperDocResolveCommentDetails *PaperDocResolveCommentDetails `json:"paper_doc_resolve_comment_details,omitempty"`
	// PaperDocRevertDetails : has no documentation (yet)
	PaperDocRevertDetails *PaperDocRevertDetails `json:"paper_doc_revert_details,omitempty"`
	// PaperDocSlackShareDetails : has no documentation (yet)
	PaperDocSlackShareDetails *PaperDocSlackShareDetails `json:"paper_doc_slack_share_details,omitempty"`
	// PaperDocTeamInviteDetails : has no documentation (yet)
	PaperDocTeamInviteDetails *PaperDocTeamInviteDetails `json:"paper_doc_team_invite_details,omitempty"`
	// PaperDocTrashedDetails : has no documentation (yet)
	PaperDocTrashedDetails *PaperDocTrashedDetails `json:"paper_doc_trashed_details,omitempty"`
	// PaperDocUnresolveCommentDetails : has no documentation (yet)
	PaperDocUnresolveCommentDetails *PaperDocUnresolveCommentDetails `json:"paper_doc_unresolve_comment_details,omitempty"`
	// PaperDocUntrashedDetails : has no documentation (yet)
	PaperDocUntrashedDetails *PaperDocUntrashedDetails `json:"paper_doc_untrashed_details,omitempty"`
	// PaperDocViewDetails : has no documentation (yet)
	PaperDocViewDetails *PaperDocViewDetails `json:"paper_doc_view_details,omitempty"`
	// PaperExternalViewAllowDetails : has no documentation (yet)
	PaperExternalViewAllowDetails *PaperExternalViewAllowDetails `json:"paper_external_view_allow_details,omitempty"`
	// PaperExternalViewDefaultTeamDetails : has no documentation (yet)
	PaperExternalViewDefaultTeamDetails *PaperExternalViewDefaultTeamDetails `json:"paper_external_view_default_team_details,omitempty"`
	// PaperExternalViewForbidDetails : has no documentation (yet)
	PaperExternalViewForbidDetails *PaperExternalViewForbidDetails `json:"paper_external_view_forbid_details,omitempty"`
	// PaperFolderChangeSubscriptionDetails : has no documentation (yet)
	PaperFolderChangeSubscriptionDetails *PaperFolderChangeSubscriptionDetails `json:"paper_folder_change_subscription_details,omitempty"`
	// PaperFolderDeletedDetails : has no documentation (yet)
	PaperFolderDeletedDetails *PaperFolderDeletedDetails `json:"paper_folder_deleted_details,omitempty"`
	// PaperFolderFollowedDetails : has no documentation (yet)
	PaperFolderFollowedDetails *PaperFolderFollowedDetails `json:"paper_folder_followed_details,omitempty"`
	// PaperFolderTeamInviteDetails : has no documentation (yet)
	PaperFolderTeamInviteDetails *PaperFolderTeamInviteDetails `json:"paper_folder_team_invite_details,omitempty"`
	// PasswordChangeDetails : has no documentation (yet)
	PasswordChangeDetails *PasswordChangeDetails `json:"password_change_details,omitempty"`
	// PasswordResetDetails : has no documentation (yet)
	PasswordResetDetails *PasswordResetDetails `json:"password_reset_details,omitempty"`
	// PasswordResetAllDetails : has no documentation (yet)
	PasswordResetAllDetails *PasswordResetAllDetails `json:"password_reset_all_details,omitempty"`
	// EmmCreateExceptionsReportDetails : has no documentation (yet)
	EmmCreateExceptionsReportDetails *EmmCreateExceptionsReportDetails `json:"emm_create_exceptions_report_details,omitempty"`
	// EmmCreateUsageReportDetails : has no documentation (yet)
	EmmCreateUsageReportDetails *EmmCreateUsageReportDetails `json:"emm_create_usage_report_details,omitempty"`
	// ExportMembersReportDetails : has no documentation (yet)
	ExportMembersReportDetails *ExportMembersReportDetails `json:"export_members_report_details,omitempty"`
	// PaperAdminExportStartDetails : has no documentation (yet)
	PaperAdminExportStartDetails *PaperAdminExportStartDetails `json:"paper_admin_export_start_details,omitempty"`
	// SmartSyncCreateAdminPrivilegeReportDetails : has no documentation (yet)
	SmartSyncCreateAdminPrivilegeReportDetails *SmartSyncCreateAdminPrivilegeReportDetails `json:"smart_sync_create_admin_privilege_report_details,omitempty"`
	// TeamActivityCreateReportDetails : has no documentation (yet)
	TeamActivityCreateReportDetails *TeamActivityCreateReportDetails `json:"team_activity_create_report_details,omitempty"`
	// CollectionShareDetails : has no documentation (yet)
	CollectionShareDetails *CollectionShareDetails `json:"collection_share_details,omitempty"`
	// NoteAclInviteOnlyDetails : has no documentation (yet)
	NoteAclInviteOnlyDetails *NoteAclInviteOnlyDetails `json:"note_acl_invite_only_details,omitempty"`
	// NoteAclLinkDetails : has no documentation (yet)
	NoteAclLinkDetails *NoteAclLinkDetails `json:"note_acl_link_details,omitempty"`
	// NoteAclTeamLinkDetails : has no documentation (yet)
	NoteAclTeamLinkDetails *NoteAclTeamLinkDetails `json:"note_acl_team_link_details,omitempty"`
	// NoteSharedDetails : has no documentation (yet)
	NoteSharedDetails *NoteSharedDetails `json:"note_shared_details,omitempty"`
	// NoteShareReceiveDetails : has no documentation (yet)
	NoteShareReceiveDetails *NoteShareReceiveDetails `json:"note_share_receive_details,omitempty"`
	// OpenNoteSharedDetails : has no documentation (yet)
	OpenNoteSharedDetails *OpenNoteSharedDetails `json:"open_note_shared_details,omitempty"`
	// SfAddGroupDetails : has no documentation (yet)
	SfAddGroupDetails *SfAddGroupDetails `json:"sf_add_group_details,omitempty"`
	// SfAllowNonMembersToViewSharedLinksDetails : has no documentation (yet)
	SfAllowNonMembersToViewSharedLinksDetails *SfAllowNonMembersToViewSharedLinksDetails `json:"sf_allow_non_members_to_view_shared_links_details,omitempty"`
	// SfExternalInviteWarnDetails : has no documentation (yet)
	SfExternalInviteWarnDetails *SfExternalInviteWarnDetails `json:"sf_external_invite_warn_details,omitempty"`
	// SfFbInviteDetails : has no documentation (yet)
	SfFbInviteDetails *SfFbInviteDetails `json:"sf_fb_invite_details,omitempty"`
	// SfFbInviteChangeRoleDetails : has no documentation (yet)
	SfFbInviteChangeRoleDetails *SfFbInviteChangeRoleDetails `json:"sf_fb_invite_change_role_details,omitempty"`
	// SfFbUninviteDetails : has no documentation (yet)
	SfFbUninviteDetails *SfFbUninviteDetails `json:"sf_fb_uninvite_details,omitempty"`
	// SfInviteGroupDetails : has no documentation (yet)
	SfInviteGroupDetails *SfInviteGroupDetails `json:"sf_invite_group_details,omitempty"`
	// SfTeamGrantAccessDetails : has no documentation (yet)
	SfTeamGrantAccessDetails *SfTeamGrantAccessDetails `json:"sf_team_grant_access_details,omitempty"`
	// SfTeamInviteDetails : has no documentation (yet)
	SfTeamInviteDetails *SfTeamInviteDetails `json:"sf_team_invite_details,omitempty"`
	// SfTeamInviteChangeRoleDetails : has no documentation (yet)
	SfTeamInviteChangeRoleDetails *SfTeamInviteChangeRoleDetails `json:"sf_team_invite_change_role_details,omitempty"`
	// SfTeamJoinDetails : has no documentation (yet)
	SfTeamJoinDetails *SfTeamJoinDetails `json:"sf_team_join_details,omitempty"`
	// SfTeamJoinFromOobLinkDetails : has no documentation (yet)
	SfTeamJoinFromOobLinkDetails *SfTeamJoinFromOobLinkDetails `json:"sf_team_join_from_oob_link_details,omitempty"`
	// SfTeamUninviteDetails : has no documentation (yet)
	SfTeamUninviteDetails *SfTeamUninviteDetails `json:"sf_team_uninvite_details,omitempty"`
	// SharedContentAddInviteesDetails : has no documentation (yet)
	SharedContentAddInviteesDetails *SharedContentAddInviteesDetails `json:"shared_content_add_invitees_details,omitempty"`
	// SharedContentAddLinkExpiryDetails : has no documentation (yet)
	SharedContentAddLinkExpiryDetails *SharedContentAddLinkExpiryDetails `json:"shared_content_add_link_expiry_details,omitempty"`
	// SharedContentAddLinkPasswordDetails : has no documentation (yet)
	SharedContentAddLinkPasswordDetails *SharedContentAddLinkPasswordDetails `json:"shared_content_add_link_password_details,omitempty"`
	// SharedContentAddMemberDetails : has no documentation (yet)
	SharedContentAddMemberDetails *SharedContentAddMemberDetails `json:"shared_content_add_member_details,omitempty"`
	// SharedContentChangeDownloadsPolicyDetails : has no documentation (yet)
	SharedContentChangeDownloadsPolicyDetails *SharedContentChangeDownloadsPolicyDetails `json:"shared_content_change_downloads_policy_details,omitempty"`
	// SharedContentChangeInviteeRoleDetails : has no documentation (yet)
	SharedContentChangeInviteeRoleDetails *SharedContentChangeInviteeRoleDetails `json:"shared_content_change_invitee_role_details,omitempty"`
	// SharedContentChangeLinkAudienceDetails : has no documentation (yet)
	SharedContentChangeLinkAudienceDetails *SharedContentChangeLinkAudienceDetails `json:"shared_content_change_link_audience_details,omitempty"`
	// SharedContentChangeLinkExpiryDetails : has no documentation (yet)
	SharedContentChangeLinkExpiryDetails *SharedContentChangeLinkExpiryDetails `json:"shared_content_change_link_expiry_details,omitempty"`
	// SharedContentChangeLinkPasswordDetails : has no documentation (yet)
	SharedContentChangeLinkPasswordDetails *SharedContentChangeLinkPasswordDetails `json:"shared_content_change_link_password_details,omitempty"`
	// SharedContentChangeMemberRoleDetails : has no documentation (yet)
	SharedContentChangeMemberRoleDetails *SharedContentChangeMemberRoleDetails `json:"shared_content_change_member_role_details,omitempty"`
	// SharedContentChangeViewerInfoPolicyDetails : has no documentation (yet)
	SharedContentChangeViewerInfoPolicyDetails *SharedContentChangeViewerInfoPolicyDetails `json:"shared_content_change_viewer_info_policy_details,omitempty"`
	// SharedContentClaimInvitationDetails : has no documentation (yet)
	SharedContentClaimInvitationDetails *SharedContentClaimInvitationDetails `json:"shared_content_claim_invitation_details,omitempty"`
	// SharedContentCopyDetails : has no documentation (yet)
	SharedContentCopyDetails *SharedContentCopyDetails `json:"shared_content_copy_details,omitempty"`
	// SharedContentDownloadDetails : has no documentation (yet)
	SharedContentDownloadDetails *SharedContentDownloadDetails `json:"shared_content_download_details,omitempty"`
	// SharedContentRelinquishMembershipDetails : has no documentation (yet)
	SharedContentRelinquishMembershipDetails *SharedContentRelinquishMembershipDetails `json:"shared_content_relinquish_membership_details,omitempty"`
	// SharedContentRemoveInviteesDetails : has no documentation (yet)
	SharedContentRemoveInviteesDetails *SharedContentRemoveInviteesDetails `json:"shared_content_remove_invitees_details,omitempty"`
	// SharedContentRemoveLinkExpiryDetails : has no documentation (yet)
	SharedContentRemoveLinkExpiryDetails *SharedContentRemoveLinkExpiryDetails `json:"shared_content_remove_link_expiry_details,omitempty"`
	// SharedContentRemoveLinkPasswordDetails : has no documentation (yet)
	SharedContentRemoveLinkPasswordDetails *SharedContentRemoveLinkPasswordDetails `json:"shared_content_remove_link_password_details,omitempty"`
	// SharedContentRemoveMemberDetails : has no documentation (yet)
	SharedContentRemoveMemberDetails *SharedContentRemoveMemberDetails `json:"shared_content_remove_member_details,omitempty"`
	// SharedContentRequestAccessDetails : has no documentation (yet)
	SharedContentRequestAccessDetails *SharedContentRequestAccessDetails `json:"shared_content_request_access_details,omitempty"`
	// SharedContentUnshareDetails : has no documentation (yet)
	SharedContentUnshareDetails *SharedContentUnshareDetails `json:"shared_content_unshare_details,omitempty"`
	// SharedContentViewDetails : has no documentation (yet)
	SharedContentViewDetails *SharedContentViewDetails `json:"shared_content_view_details,omitempty"`
	// SharedFolderChangeLinkPolicyDetails : has no documentation (yet)
	SharedFolderChangeLinkPolicyDetails *SharedFolderChangeLinkPolicyDetails `json:"shared_folder_change_link_policy_details,omitempty"`
	// SharedFolderChangeMembersInheritancePolicyDetails : has no documentation
	// (yet)
	SharedFolderChangeMembersInheritancePolicyDetails *SharedFolderChangeMembersInheritancePolicyDetails `json:"shared_folder_change_members_inheritance_policy_details,omitempty"`
	// SharedFolderChangeMembersManagementPolicyDetails : has no documentation
	// (yet)
	SharedFolderChangeMembersManagementPolicyDetails *SharedFolderChangeMembersManagementPolicyDetails `json:"shared_folder_change_members_management_policy_details,omitempty"`
	// SharedFolderChangeMembersPolicyDetails : has no documentation (yet)
	SharedFolderChangeMembersPolicyDetails *SharedFolderChangeMembersPolicyDetails `json:"shared_folder_change_members_policy_details,omitempty"`
	// SharedFolderCreateDetails : has no documentation (yet)
	SharedFolderCreateDetails *SharedFolderCreateDetails `json:"shared_folder_create_details,omitempty"`
	// SharedFolderDeclineInvitationDetails : has no documentation (yet)
	SharedFolderDeclineInvitationDetails *SharedFolderDeclineInvitationDetails `json:"shared_folder_decline_invitation_details,omitempty"`
	// SharedFolderMountDetails : has no documentation (yet)
	SharedFolderMountDetails *SharedFolderMountDetails `json:"shared_folder_mount_details,omitempty"`
	// SharedFolderNestDetails : has no documentation (yet)
	SharedFolderNestDetails *SharedFolderNestDetails `json:"shared_folder_nest_details,omitempty"`
	// SharedFolderTransferOwnershipDetails : has no documentation (yet)
	SharedFolderTransferOwnershipDetails *SharedFolderTransferOwnershipDetails `json:"shared_folder_transfer_ownership_details,omitempty"`
	// SharedFolderUnmountDetails : has no documentation (yet)
	SharedFolderUnmountDetails *SharedFolderUnmountDetails `json:"shared_folder_unmount_details,omitempty"`
	// SharedLinkAddExpiryDetails : has no documentation (yet)
	SharedLinkAddExpiryDetails *SharedLinkAddExpiryDetails `json:"shared_link_add_expiry_details,omitempty"`
	// SharedLinkChangeExpiryDetails : has no documentation (yet)
	SharedLinkChangeExpiryDetails *SharedLinkChangeExpiryDetails `json:"shared_link_change_expiry_details,omitempty"`
	// SharedLinkChangeVisibilityDetails : has no documentation (yet)
	SharedLinkChangeVisibilityDetails *SharedLinkChangeVisibilityDetails `json:"shared_link_change_visibility_details,omitempty"`
	// SharedLinkCopyDetails : has no documentation (yet)
	SharedLinkCopyDetails *SharedLinkCopyDetails `json:"shared_link_copy_details,omitempty"`
	// SharedLinkCreateDetails : has no documentation (yet)
	SharedLinkCreateDetails *SharedLinkCreateDetails `json:"shared_link_create_details,omitempty"`
	// SharedLinkDisableDetails : has no documentation (yet)
	SharedLinkDisableDetails *SharedLinkDisableDetails `json:"shared_link_disable_details,omitempty"`
	// SharedLinkDownloadDetails : has no documentation (yet)
	SharedLinkDownloadDetails *SharedLinkDownloadDetails `json:"shared_link_download_details,omitempty"`
	// SharedLinkRemoveExpiryDetails : has no documentation (yet)
	SharedLinkRemoveExpiryDetails *SharedLinkRemoveExpiryDetails `json:"shared_link_remove_expiry_details,omitempty"`
	// SharedLinkShareDetails : has no documentation (yet)
	SharedLinkShareDetails *SharedLinkShareDetails `json:"shared_link_share_details,omitempty"`
	// SharedLinkViewDetails : has no documentation (yet)
	SharedLinkViewDetails *SharedLinkViewDetails `json:"shared_link_view_details,omitempty"`
	// SharedNoteOpenedDetails : has no documentation (yet)
	SharedNoteOpenedDetails *SharedNoteOpenedDetails `json:"shared_note_opened_details,omitempty"`
	// ShmodelGroupShareDetails : has no documentation (yet)
	ShmodelGroupShareDetails *ShmodelGroupShareDetails `json:"shmodel_group_share_details,omitempty"`
	// ShowcaseAccessGrantedDetails : has no documentation (yet)
	ShowcaseAccessGrantedDetails *ShowcaseAccessGrantedDetails `json:"showcase_access_granted_details,omitempty"`
	// ShowcaseAddMemberDetails : has no documentation (yet)
	ShowcaseAddMemberDetails *ShowcaseAddMemberDetails `json:"showcase_add_member_details,omitempty"`
	// ShowcaseArchivedDetails : has no documentation (yet)
	ShowcaseArchivedDetails *ShowcaseArchivedDetails `json:"showcase_archived_details,omitempty"`
	// ShowcaseCreatedDetails : has no documentation (yet)
	ShowcaseCreatedDetails *ShowcaseCreatedDetails `json:"showcase_created_details,omitempty"`
	// ShowcaseDeleteCommentDetails : has no documentation (yet)
	ShowcaseDeleteCommentDetails *ShowcaseDeleteCommentDetails `json:"showcase_delete_comment_details,omitempty"`
	// ShowcaseEditedDetails : has no documentation (yet)
	ShowcaseEditedDetails *ShowcaseEditedDetails `json:"showcase_edited_details,omitempty"`
	// ShowcaseEditCommentDetails : has no documentation (yet)
	ShowcaseEditCommentDetails *ShowcaseEditCommentDetails `json:"showcase_edit_comment_details,omitempty"`
	// ShowcaseFileAddedDetails : has no documentation (yet)
	ShowcaseFileAddedDetails *ShowcaseFileAddedDetails `json:"showcase_file_added_details,omitempty"`
	// ShowcaseFileDownloadDetails : has no documentation (yet)
	ShowcaseFileDownloadDetails *ShowcaseFileDownloadDetails `json:"showcase_file_download_details,omitempty"`
	// ShowcaseFileRemovedDetails : has no documentation (yet)
	ShowcaseFileRemovedDetails *ShowcaseFileRemovedDetails `json:"showcase_file_removed_details,omitempty"`
	// ShowcaseFileViewDetails : has no documentation (yet)
	ShowcaseFileViewDetails *ShowcaseFileViewDetails `json:"showcase_file_view_details,omitempty"`
	// ShowcasePermanentlyDeletedDetails : has no documentation (yet)
	ShowcasePermanentlyDeletedDetails *ShowcasePermanentlyDeletedDetails `json:"showcase_permanently_deleted_details,omitempty"`
	// ShowcasePostCommentDetails : has no documentation (yet)
	ShowcasePostCommentDetails *ShowcasePostCommentDetails `json:"showcase_post_comment_details,omitempty"`
	// ShowcaseRemoveMemberDetails : has no documentation (yet)
	ShowcaseRemoveMemberDetails *ShowcaseRemoveMemberDetails `json:"showcase_remove_member_details,omitempty"`
	// ShowcaseRenamedDetails : has no documentation (yet)
	ShowcaseRenamedDetails *ShowcaseRenamedDetails `json:"showcase_renamed_details,omitempty"`
	// ShowcaseRequestAccessDetails : has no documentation (yet)
	ShowcaseRequestAccessDetails *ShowcaseRequestAccessDetails `json:"showcase_request_access_details,omitempty"`
	// ShowcaseResolveCommentDetails : has no documentation (yet)
	ShowcaseResolveCommentDetails *ShowcaseResolveCommentDetails `json:"showcase_resolve_comment_details,omitempty"`
	// ShowcaseRestoredDetails : has no documentation (yet)
	ShowcaseRestoredDetails *ShowcaseRestoredDetails `json:"showcase_restored_details,omitempty"`
	// ShowcaseTrashedDetails : has no documentation (yet)
	ShowcaseTrashedDetails *ShowcaseTrashedDetails `json:"showcase_trashed_details,omitempty"`
	// ShowcaseTrashedDeprecatedDetails : has no documentation (yet)
	ShowcaseTrashedDeprecatedDetails *ShowcaseTrashedDeprecatedDetails `json:"showcase_trashed_deprecated_details,omitempty"`
	// ShowcaseUnresolveCommentDetails : has no documentation (yet)
	ShowcaseUnresolveCommentDetails *ShowcaseUnresolveCommentDetails `json:"showcase_unresolve_comment_details,omitempty"`
	// ShowcaseUntrashedDetails : has no documentation (yet)
	ShowcaseUntrashedDetails *ShowcaseUntrashedDetails `json:"showcase_untrashed_details,omitempty"`
	// ShowcaseUntrashedDeprecatedDetails : has no documentation (yet)
	ShowcaseUntrashedDeprecatedDetails *ShowcaseUntrashedDeprecatedDetails `json:"showcase_untrashed_deprecated_details,omitempty"`
	// ShowcaseViewDetails : has no documentation (yet)
	ShowcaseViewDetails *ShowcaseViewDetails `json:"showcase_view_details,omitempty"`
	// SsoAddCertDetails : has no documentation (yet)
	SsoAddCertDetails *SsoAddCertDetails `json:"sso_add_cert_details,omitempty"`
	// SsoAddLoginUrlDetails : has no documentation (yet)
	SsoAddLoginUrlDetails *SsoAddLoginUrlDetails `json:"sso_add_login_url_details,omitempty"`
	// SsoAddLogoutUrlDetails : has no documentation (yet)
	SsoAddLogoutUrlDetails *SsoAddLogoutUrlDetails `json:"sso_add_logout_url_details,omitempty"`
	// SsoChangeCertDetails : has no documentation (yet)
	SsoChangeCertDetails *SsoChangeCertDetails `json:"sso_change_cert_details,omitempty"`
	// SsoChangeLoginUrlDetails : has no documentation (yet)
	SsoChangeLoginUrlDetails *SsoChangeLoginUrlDetails `json:"sso_change_login_url_details,omitempty"`
	// SsoChangeLogoutUrlDetails : has no documentation (yet)
	SsoChangeLogoutUrlDetails *SsoChangeLogoutUrlDetails `json:"sso_change_logout_url_details,omitempty"`
	// SsoChangeSamlIdentityModeDetails : has no documentation (yet)
	SsoChangeSamlIdentityModeDetails *SsoChangeSamlIdentityModeDetails `json:"sso_change_saml_identity_mode_details,omitempty"`
	// SsoRemoveCertDetails : has no documentation (yet)
	SsoRemoveCertDetails *SsoRemoveCertDetails `json:"sso_remove_cert_details,omitempty"`
	// SsoRemoveLoginUrlDetails : has no documentation (yet)
	SsoRemoveLoginUrlDetails *SsoRemoveLoginUrlDetails `json:"sso_remove_login_url_details,omitempty"`
	// SsoRemoveLogoutUrlDetails : has no documentation (yet)
	SsoRemoveLogoutUrlDetails *SsoRemoveLogoutUrlDetails `json:"sso_remove_logout_url_details,omitempty"`
	// TeamFolderChangeStatusDetails : has no documentation (yet)
	TeamFolderChangeStatusDetails *TeamFolderChangeStatusDetails `json:"team_folder_change_status_details,omitempty"`
	// TeamFolderCreateDetails : has no documentation (yet)
	TeamFolderCreateDetails *TeamFolderCreateDetails `json:"team_folder_create_details,omitempty"`
	// TeamFolderDowngradeDetails : has no documentation (yet)
	TeamFolderDowngradeDetails *TeamFolderDowngradeDetails `json:"team_folder_downgrade_details,omitempty"`
	// TeamFolderPermanentlyDeleteDetails : has no documentation (yet)
	TeamFolderPermanentlyDeleteDetails *TeamFolderPermanentlyDeleteDetails `json:"team_folder_permanently_delete_details,omitempty"`
	// TeamFolderRenameDetails : has no documentation (yet)
	TeamFolderRenameDetails *TeamFolderRenameDetails `json:"team_folder_rename_details,omitempty"`
	// TeamSelectiveSyncSettingsChangedDetails : has no documentation (yet)
	TeamSelectiveSyncSettingsChangedDetails *TeamSelectiveSyncSettingsChangedDetails `json:"team_selective_sync_settings_changed_details,omitempty"`
	// AccountCaptureChangePolicyDetails : has no documentation (yet)
	AccountCaptureChangePolicyDetails *AccountCaptureChangePolicyDetails `json:"account_capture_change_policy_details,omitempty"`
	// AllowDownloadDisabledDetails : has no documentation (yet)
	AllowDownloadDisabledDetails *AllowDownloadDisabledDetails `json:"allow_download_disabled_details,omitempty"`
	// AllowDownloadEnabledDetails : has no documentation (yet)
	AllowDownloadEnabledDetails *AllowDownloadEnabledDetails `json:"allow_download_enabled_details,omitempty"`
	// DataPlacementRestrictionChangePolicyDetails : has no documentation (yet)
	DataPlacementRestrictionChangePolicyDetails *DataPlacementRestrictionChangePolicyDetails `json:"data_placement_restriction_change_policy_details,omitempty"`
	// DataPlacementRestrictionSatisfyPolicyDetails : has no documentation (yet)
	DataPlacementRestrictionSatisfyPolicyDetails *DataPlacementRestrictionSatisfyPolicyDetails `json:"data_placement_restriction_satisfy_policy_details,omitempty"`
	// DeviceApprovalsChangeDesktopPolicyDetails : has no documentation (yet)
	DeviceApprovalsChangeDesktopPolicyDetails *DeviceApprovalsChangeDesktopPolicyDetails `json:"device_approvals_change_desktop_policy_details,omitempty"`
	// DeviceApprovalsChangeMobilePolicyDetails : has no documentation (yet)
	DeviceApprovalsChangeMobilePolicyDetails *DeviceApprovalsChangeMobilePolicyDetails `json:"device_approvals_change_mobile_policy_details,omitempty"`
	// DeviceApprovalsChangeOverageActionDetails : has no documentation (yet)
	DeviceApprovalsChangeOverageActionDetails *DeviceApprovalsChangeOverageActionDetails `json:"device_approvals_change_overage_action_details,omitempty"`
	// DeviceApprovalsChangeUnlinkActionDetails : has no documentation (yet)
	DeviceApprovalsChangeUnlinkActionDetails *DeviceApprovalsChangeUnlinkActionDetails `json:"device_approvals_change_unlink_action_details,omitempty"`
	// DirectoryRestrictionsAddMembersDetails : has no documentation (yet)
	DirectoryRestrictionsAddMembersDetails *DirectoryRestrictionsAddMembersDetails `json:"directory_restrictions_add_members_details,omitempty"`
	// DirectoryRestrictionsRemoveMembersDetails : has no documentation (yet)
	DirectoryRestrictionsRemoveMembersDetails *DirectoryRestrictionsRemoveMembersDetails `json:"directory_restrictions_remove_members_details,omitempty"`
	// EmmAddExceptionDetails : has no documentation (yet)
	EmmAddExceptionDetails *EmmAddExceptionDetails `json:"emm_add_exception_details,omitempty"`
	// EmmChangePolicyDetails : has no documentation (yet)
	EmmChangePolicyDetails *EmmChangePolicyDetails `json:"emm_change_policy_details,omitempty"`
	// EmmRemoveExceptionDetails : has no documentation (yet)
	EmmRemoveExceptionDetails *EmmRemoveExceptionDetails `json:"emm_remove_exception_details,omitempty"`
	// ExtendedVersionHistoryChangePolicyDetails : has no documentation (yet)
	ExtendedVersionHistoryChangePolicyDetails *ExtendedVersionHistoryChangePolicyDetails `json:"extended_version_history_change_policy_details,omitempty"`
	// FileCommentsChangePolicyDetails : has no documentation (yet)
	FileCommentsChangePolicyDetails *FileCommentsChangePolicyDetails `json:"file_comments_change_policy_details,omitempty"`
	// FileRequestsChangePolicyDetails : has no documentation (yet)
	FileRequestsChangePolicyDetails *FileRequestsChangePolicyDetails `json:"file_requests_change_policy_details,omitempty"`
	// FileRequestsEmailsEnabledDetails : has no documentation (yet)
	FileRequestsEmailsEnabledDetails *FileRequestsEmailsEnabledDetails `json:"file_requests_emails_enabled_details,omitempty"`
	// FileRequestsEmailsRestrictedToTeamOnlyDetails : has no documentation
	// (yet)
	FileRequestsEmailsRestrictedToTeamOnlyDetails *FileRequestsEmailsRestrictedToTeamOnlyDetails `json:"file_requests_emails_restricted_to_team_only_details,omitempty"`
	// GoogleSsoChangePolicyDetails : has no documentation (yet)
	GoogleSsoChangePolicyDetails *GoogleSsoChangePolicyDetails `json:"google_sso_change_policy_details,omitempty"`
	// GroupUserManagementChangePolicyDetails : has no documentation (yet)
	GroupUserManagementChangePolicyDetails *GroupUserManagementChangePolicyDetails `json:"group_user_management_change_policy_details,omitempty"`
	// MemberRequestsChangePolicyDetails : has no documentation (yet)
	MemberRequestsChangePolicyDetails *MemberRequestsChangePolicyDetails `json:"member_requests_change_policy_details,omitempty"`
	// MemberSpaceLimitsAddExceptionDetails : has no documentation (yet)
	MemberSpaceLimitsAddExceptionDetails *MemberSpaceLimitsAddExceptionDetails `json:"member_space_limits_add_exception_details,omitempty"`
	// MemberSpaceLimitsChangeCapsTypePolicyDetails : has no documentation (yet)
	MemberSpaceLimitsChangeCapsTypePolicyDetails *MemberSpaceLimitsChangeCapsTypePolicyDetails `json:"member_space_limits_change_caps_type_policy_details,omitempty"`
	// MemberSpaceLimitsChangePolicyDetails : has no documentation (yet)
	MemberSpaceLimitsChangePolicyDetails *MemberSpaceLimitsChangePolicyDetails `json:"member_space_limits_change_policy_details,omitempty"`
	// MemberSpaceLimitsRemoveExceptionDetails : has no documentation (yet)
	MemberSpaceLimitsRemoveExceptionDetails *MemberSpaceLimitsRemoveExceptionDetails `json:"member_space_limits_remove_exception_details,omitempty"`
	// MemberSuggestionsChangePolicyDetails : has no documentation (yet)
	MemberSuggestionsChangePolicyDetails *MemberSuggestionsChangePolicyDetails `json:"member_suggestions_change_policy_details,omitempty"`
	// MicrosoftOfficeAddinChangePolicyDetails : has no documentation (yet)
	MicrosoftOfficeAddinChangePolicyDetails *MicrosoftOfficeAddinChangePolicyDetails `json:"microsoft_office_addin_change_policy_details,omitempty"`
	// NetworkControlChangePolicyDetails : has no documentation (yet)
	NetworkControlChangePolicyDetails *NetworkControlChangePolicyDetails `json:"network_control_change_policy_details,omitempty"`
	// PaperChangeDeploymentPolicyDetails : has no documentation (yet)
	PaperChangeDeploymentPolicyDetails *PaperChangeDeploymentPolicyDetails `json:"paper_change_deployment_policy_details,omitempty"`
	// PaperChangeMemberLinkPolicyDetails : has no documentation (yet)
	PaperChangeMemberLinkPolicyDetails *PaperChangeMemberLinkPolicyDetails `json:"paper_change_member_link_policy_details,omitempty"`
	// PaperChangeMemberPolicyDetails : has no documentation (yet)
	PaperChangeMemberPolicyDetails *PaperChangeMemberPolicyDetails `json:"paper_change_member_policy_details,omitempty"`
	// PaperChangePolicyDetails : has no documentation (yet)
	PaperChangePolicyDetails *PaperChangePolicyDetails `json:"paper_change_policy_details,omitempty"`
	// PaperEnabledUsersGroupAdditionDetails : has no documentation (yet)
	PaperEnabledUsersGroupAdditionDetails *PaperEnabledUsersGroupAdditionDetails `json:"paper_enabled_users_group_addition_details,omitempty"`
	// PaperEnabledUsersGroupRemovalDetails : has no documentation (yet)
	PaperEnabledUsersGroupRemovalDetails *PaperEnabledUsersGroupRemovalDetails `json:"paper_enabled_users_group_removal_details,omitempty"`
	// PermanentDeleteChangePolicyDetails : has no documentation (yet)
	PermanentDeleteChangePolicyDetails *PermanentDeleteChangePolicyDetails `json:"permanent_delete_change_policy_details,omitempty"`
	// SharingChangeFolderJoinPolicyDetails : has no documentation (yet)
	SharingChangeFolderJoinPolicyDetails *SharingChangeFolderJoinPolicyDetails `json:"sharing_change_folder_join_policy_details,omitempty"`
	// SharingChangeLinkPolicyDetails : has no documentation (yet)
	SharingChangeLinkPolicyDetails *SharingChangeLinkPolicyDetails `json:"sharing_change_link_policy_details,omitempty"`
	// SharingChangeMemberPolicyDetails : has no documentation (yet)
	SharingChangeMemberPolicyDetails *SharingChangeMemberPolicyDetails `json:"sharing_change_member_policy_details,omitempty"`
	// ShowcaseChangeDownloadPolicyDetails : has no documentation (yet)
	ShowcaseChangeDownloadPolicyDetails *ShowcaseChangeDownloadPolicyDetails `json:"showcase_change_download_policy_details,omitempty"`
	// ShowcaseChangeEnabledPolicyDetails : has no documentation (yet)
	ShowcaseChangeEnabledPolicyDetails *ShowcaseChangeEnabledPolicyDetails `json:"showcase_change_enabled_policy_details,omitempty"`
	// ShowcaseChangeExternalSharingPolicyDetails : has no documentation (yet)
	ShowcaseChangeExternalSharingPolicyDetails *ShowcaseChangeExternalSharingPolicyDetails `json:"showcase_change_external_sharing_policy_details,omitempty"`
	// SmartSyncChangePolicyDetails : has no documentation (yet)
	SmartSyncChangePolicyDetails *SmartSyncChangePolicyDetails `json:"smart_sync_change_policy_details,omitempty"`
	// SmartSyncNotOptOutDetails : has no documentation (yet)
	SmartSyncNotOptOutDetails *SmartSyncNotOptOutDetails `json:"smart_sync_not_opt_out_details,omitempty"`
	// SmartSyncOptOutDetails : has no documentation (yet)
	SmartSyncOptOutDetails *SmartSyncOptOutDetails `json:"smart_sync_opt_out_details,omitempty"`
	// SsoChangePolicyDetails : has no documentation (yet)
	SsoChangePolicyDetails *SsoChangePolicyDetails `json:"sso_change_policy_details,omitempty"`
	// TfaChangePolicyDetails : has no documentation (yet)
	TfaChangePolicyDetails *TfaChangePolicyDetails `json:"tfa_change_policy_details,omitempty"`
	// TwoAccountChangePolicyDetails : has no documentation (yet)
	TwoAccountChangePolicyDetails *TwoAccountChangePolicyDetails `json:"two_account_change_policy_details,omitempty"`
	// WebSessionsChangeFixedLengthPolicyDetails : has no documentation (yet)
	WebSessionsChangeFixedLengthPolicyDetails *WebSessionsChangeFixedLengthPolicyDetails `json:"web_sessions_change_fixed_length_policy_details,omitempty"`
	// WebSessionsChangeIdleLengthPolicyDetails : has no documentation (yet)
	WebSessionsChangeIdleLengthPolicyDetails *WebSessionsChangeIdleLengthPolicyDetails `json:"web_sessions_change_idle_length_policy_details,omitempty"`
	// TeamMergeFromDetails : has no documentation (yet)
	TeamMergeFromDetails *TeamMergeFromDetails `json:"team_merge_from_details,omitempty"`
	// TeamMergeToDetails : has no documentation (yet)
	TeamMergeToDetails *TeamMergeToDetails `json:"team_merge_to_details,omitempty"`
	// TeamProfileAddLogoDetails : has no documentation (yet)
	TeamProfileAddLogoDetails *TeamProfileAddLogoDetails `json:"team_profile_add_logo_details,omitempty"`
	// TeamProfileChangeDefaultLanguageDetails : has no documentation (yet)
	TeamProfileChangeDefaultLanguageDetails *TeamProfileChangeDefaultLanguageDetails `json:"team_profile_change_default_language_details,omitempty"`
	// TeamProfileChangeLogoDetails : has no documentation (yet)
	TeamProfileChangeLogoDetails *TeamProfileChangeLogoDetails `json:"team_profile_change_logo_details,omitempty"`
	// TeamProfileChangeNameDetails : has no documentation (yet)
	TeamProfileChangeNameDetails *TeamProfileChangeNameDetails `json:"team_profile_change_name_details,omitempty"`
	// TeamProfileRemoveLogoDetails : has no documentation (yet)
	TeamProfileRemoveLogoDetails *TeamProfileRemoveLogoDetails `json:"team_profile_remove_logo_details,omitempty"`
	// TfaAddBackupPhoneDetails : has no documentation (yet)
	TfaAddBackupPhoneDetails *TfaAddBackupPhoneDetails `json:"tfa_add_backup_phone_details,omitempty"`
	// TfaAddSecurityKeyDetails : has no documentation (yet)
	TfaAddSecurityKeyDetails *TfaAddSecurityKeyDetails `json:"tfa_add_security_key_details,omitempty"`
	// TfaChangeBackupPhoneDetails : has no documentation (yet)
	TfaChangeBackupPhoneDetails *TfaChangeBackupPhoneDetails `json:"tfa_change_backup_phone_details,omitempty"`
	// TfaChangeStatusDetails : has no documentation (yet)
	TfaChangeStatusDetails *TfaChangeStatusDetails `json:"tfa_change_status_details,omitempty"`
	// TfaRemoveBackupPhoneDetails : has no documentation (yet)
	TfaRemoveBackupPhoneDetails *TfaRemoveBackupPhoneDetails `json:"tfa_remove_backup_phone_details,omitempty"`
	// TfaRemoveSecurityKeyDetails : has no documentation (yet)
	TfaRemoveSecurityKeyDetails *TfaRemoveSecurityKeyDetails `json:"tfa_remove_security_key_details,omitempty"`
	// TfaResetDetails : has no documentation (yet)
	TfaResetDetails *TfaResetDetails `json:"tfa_reset_details,omitempty"`
	// MissingDetails : Hints that this event was returned with missing details
	// due to an internal error.
	MissingDetails *MissingDetails `json:"missing_details,omitempty"`
}

// Valid tag values for EventDetails
const (
	EventDetailsAppLinkTeamDetails                                = "app_link_team_details"
	EventDetailsAppLinkUserDetails                                = "app_link_user_details"
	EventDetailsAppUnlinkTeamDetails                              = "app_unlink_team_details"
	EventDetailsAppUnlinkUserDetails                              = "app_unlink_user_details"
	EventDetailsFileAddCommentDetails                             = "file_add_comment_details"
	EventDetailsFileChangeCommentSubscriptionDetails              = "file_change_comment_subscription_details"
	EventDetailsFileDeleteCommentDetails                          = "file_delete_comment_details"
	EventDetailsFileLikeCommentDetails                            = "file_like_comment_details"
	EventDetailsFileResolveCommentDetails                         = "file_resolve_comment_details"
	EventDetailsFileUnlikeCommentDetails                          = "file_unlike_comment_details"
	EventDetailsFileUnresolveCommentDetails                       = "file_unresolve_comment_details"
	EventDetailsDeviceChangeIpDesktopDetails                      = "device_change_ip_desktop_details"
	EventDetailsDeviceChangeIpMobileDetails                       = "device_change_ip_mobile_details"
	EventDetailsDeviceChangeIpWebDetails                          = "device_change_ip_web_details"
	EventDetailsDeviceDeleteOnUnlinkFailDetails                   = "device_delete_on_unlink_fail_details"
	EventDetailsDeviceDeleteOnUnlinkSuccessDetails                = "device_delete_on_unlink_success_details"
	EventDetailsDeviceLinkFailDetails                             = "device_link_fail_details"
	EventDetailsDeviceLinkSuccessDetails                          = "device_link_success_details"
	EventDetailsDeviceManagementDisabledDetails                   = "device_management_disabled_details"
	EventDetailsDeviceManagementEnabledDetails                    = "device_management_enabled_details"
	EventDetailsDeviceUnlinkDetails                               = "device_unlink_details"
	EventDetailsEmmRefreshAuthTokenDetails                        = "emm_refresh_auth_token_details"
	EventDetailsAccountCaptureChangeAvailabilityDetails           = "account_capture_change_availability_details"
	EventDetailsAccountCaptureMigrateAccountDetails               = "account_capture_migrate_account_details"
	EventDetailsAccountCaptureNotificationEmailsSentDetails       = "account_capture_notification_emails_sent_details"
	EventDetailsAccountCaptureRelinquishAccountDetails            = "account_capture_relinquish_account_details"
	EventDetailsDisabledDomainInvitesDetails                      = "disabled_domain_invites_details"
	EventDetailsDomainInvitesApproveRequestToJoinTeamDetails      = "domain_invites_approve_request_to_join_team_details"
	EventDetailsDomainInvitesDeclineRequestToJoinTeamDetails      = "domain_invites_decline_request_to_join_team_details"
	EventDetailsDomainInvitesEmailExistingUsersDetails            = "domain_invites_email_existing_users_details"
	EventDetailsDomainInvitesRequestToJoinTeamDetails             = "domain_invites_request_to_join_team_details"
	EventDetailsDomainInvitesSetInviteNewUserPrefToNoDetails      = "domain_invites_set_invite_new_user_pref_to_no_details"
	EventDetailsDomainInvitesSetInviteNewUserPrefToYesDetails     = "domain_invites_set_invite_new_user_pref_to_yes_details"
	EventDetailsDomainVerificationAddDomainFailDetails            = "domain_verification_add_domain_fail_details"
	EventDetailsDomainVerificationAddDomainSuccessDetails         = "domain_verification_add_domain_success_details"
	EventDetailsDomainVerificationRemoveDomainDetails             = "domain_verification_remove_domain_details"
	EventDetailsEnabledDomainInvitesDetails                       = "enabled_domain_invites_details"
	EventDetailsCreateFolderDetails                               = "create_folder_details"
	EventDetailsFileAddDetails                                    = "file_add_details"
	EventDetailsFileCopyDetails                                   = "file_copy_details"
	EventDetailsFileDeleteDetails                                 = "file_delete_details"
	EventDetailsFileDownloadDetails                               = "file_download_details"
	EventDetailsFileEditDetails                                   = "file_edit_details"
	EventDetailsFileGetCopyReferenceDetails                       = "file_get_copy_reference_details"
	EventDetailsFileMoveDetails                                   = "file_move_details"
	EventDetailsFilePermanentlyDeleteDetails                      = "file_permanently_delete_details"
	EventDetailsFilePreviewDetails                                = "file_preview_details"
	EventDetailsFileRenameDetails                                 = "file_rename_details"
	EventDetailsFileRestoreDetails                                = "file_restore_details"
	EventDetailsFileRevertDetails                                 = "file_revert_details"
	EventDetailsFileRollbackChangesDetails                        = "file_rollback_changes_details"
	EventDetailsFileSaveCopyReferenceDetails                      = "file_save_copy_reference_details"
	EventDetailsFileRequestChangeDetails                          = "file_request_change_details"
	EventDetailsFileRequestCloseDetails                           = "file_request_close_details"
	EventDetailsFileRequestCreateDetails                          = "file_request_create_details"
	EventDetailsFileRequestReceiveFileDetails                     = "file_request_receive_file_details"
	EventDetailsGroupAddExternalIdDetails                         = "group_add_external_id_details"
	EventDetailsGroupAddMemberDetails                             = "group_add_member_details"
	EventDetailsGroupChangeExternalIdDetails                      = "group_change_external_id_details"
	EventDetailsGroupChangeManagementTypeDetails                  = "group_change_management_type_details"
	EventDetailsGroupChangeMemberRoleDetails                      = "group_change_member_role_details"
	EventDetailsGroupCreateDetails                                = "group_create_details"
	EventDetailsGroupDeleteDetails                                = "group_delete_details"
	EventDetailsGroupDescriptionUpdatedDetails                    = "group_description_updated_details"
	EventDetailsGroupJoinPolicyUpdatedDetails                     = "group_join_policy_updated_details"
	EventDetailsGroupMovedDetails                                 = "group_moved_details"
	EventDetailsGroupRemoveExternalIdDetails                      = "group_remove_external_id_details"
	EventDetailsGroupRemoveMemberDetails                          = "group_remove_member_details"
	EventDetailsGroupRenameDetails                                = "group_rename_details"
	EventDetailsEmmErrorDetails                                   = "emm_error_details"
	EventDetailsLoginFailDetails                                  = "login_fail_details"
	EventDetailsLoginSuccessDetails                               = "login_success_details"
	EventDetailsLogoutDetails                                     = "logout_details"
	EventDetailsResellerSupportSessionEndDetails                  = "reseller_support_session_end_details"
	EventDetailsResellerSupportSessionStartDetails                = "reseller_support_session_start_details"
	EventDetailsSignInAsSessionEndDetails                         = "sign_in_as_session_end_details"
	EventDetailsSignInAsSessionStartDetails                       = "sign_in_as_session_start_details"
	EventDetailsSsoErrorDetails                                   = "sso_error_details"
	EventDetailsMemberAddNameDetails                              = "member_add_name_details"
	EventDetailsMemberChangeAdminRoleDetails                      = "member_change_admin_role_details"
	EventDetailsMemberChangeEmailDetails                          = "member_change_email_details"
	EventDetailsMemberChangeMembershipTypeDetails                 = "member_change_membership_type_details"
	EventDetailsMemberChangeNameDetails                           = "member_change_name_details"
	EventDetailsMemberChangeStatusDetails                         = "member_change_status_details"
	EventDetailsMemberPermanentlyDeleteAccountContentsDetails     = "member_permanently_delete_account_contents_details"
	EventDetailsMemberSpaceLimitsAddCustomQuotaDetails            = "member_space_limits_add_custom_quota_details"
	EventDetailsMemberSpaceLimitsChangeCustomQuotaDetails         = "member_space_limits_change_custom_quota_details"
	EventDetailsMemberSpaceLimitsChangeStatusDetails              = "member_space_limits_change_status_details"
	EventDetailsMemberSpaceLimitsRemoveCustomQuotaDetails         = "member_space_limits_remove_custom_quota_details"
	EventDetailsMemberSuggestDetails                              = "member_suggest_details"
	EventDetailsMemberTransferAccountContentsDetails              = "member_transfer_account_contents_details"
	EventDetailsSecondaryMailsPolicyChangedDetails                = "secondary_mails_policy_changed_details"
	EventDetailsPaperContentAddMemberDetails                      = "paper_content_add_member_details"
	EventDetailsPaperContentAddToFolderDetails                    = "paper_content_add_to_folder_details"
	EventDetailsPaperContentArchiveDetails                        = "paper_content_archive_details"
	EventDetailsPaperContentCreateDetails                         = "paper_content_create_details"
	EventDetailsPaperContentPermanentlyDeleteDetails              = "paper_content_permanently_delete_details"
	EventDetailsPaperContentRemoveFromFolderDetails               = "paper_content_remove_from_folder_details"
	EventDetailsPaperContentRemoveMemberDetails                   = "paper_content_remove_member_details"
	EventDetailsPaperContentRenameDetails                         = "paper_content_rename_details"
	EventDetailsPaperContentRestoreDetails                        = "paper_content_restore_details"
	EventDetailsPaperDocAddCommentDetails                         = "paper_doc_add_comment_details"
	EventDetailsPaperDocChangeMemberRoleDetails                   = "paper_doc_change_member_role_details"
	EventDetailsPaperDocChangeSharingPolicyDetails                = "paper_doc_change_sharing_policy_details"
	EventDetailsPaperDocChangeSubscriptionDetails                 = "paper_doc_change_subscription_details"
	EventDetailsPaperDocDeletedDetails                            = "paper_doc_deleted_details"
	EventDetailsPaperDocDeleteCommentDetails                      = "paper_doc_delete_comment_details"
	EventDetailsPaperDocDownloadDetails                           = "paper_doc_download_details"
	EventDetailsPaperDocEditDetails                               = "paper_doc_edit_details"
	EventDetailsPaperDocEditCommentDetails                        = "paper_doc_edit_comment_details"
	EventDetailsPaperDocFollowedDetails                           = "paper_doc_followed_details"
	EventDetailsPaperDocMentionDetails                            = "paper_doc_mention_details"
	EventDetailsPaperDocRequestAccessDetails                      = "paper_doc_request_access_details"
	EventDetailsPaperDocResolveCommentDetails                     = "paper_doc_resolve_comment_details"
	EventDetailsPaperDocRevertDetails                             = "paper_doc_revert_details"
	EventDetailsPaperDocSlackShareDetails                         = "paper_doc_slack_share_details"
	EventDetailsPaperDocTeamInviteDetails                         = "paper_doc_team_invite_details"
	EventDetailsPaperDocTrashedDetails                            = "paper_doc_trashed_details"
	EventDetailsPaperDocUnresolveCommentDetails                   = "paper_doc_unresolve_comment_details"
	EventDetailsPaperDocUntrashedDetails                          = "paper_doc_untrashed_details"
	EventDetailsPaperDocViewDetails                               = "paper_doc_view_details"
	EventDetailsPaperExternalViewAllowDetails                     = "paper_external_view_allow_details"
	EventDetailsPaperExternalViewDefaultTeamDetails               = "paper_external_view_default_team_details"
	EventDetailsPaperExternalViewForbidDetails                    = "paper_external_view_forbid_details"
	EventDetailsPaperFolderChangeSubscriptionDetails              = "paper_folder_change_subscription_details"
	EventDetailsPaperFolderDeletedDetails                         = "paper_folder_deleted_details"
	EventDetailsPaperFolderFollowedDetails                        = "paper_folder_followed_details"
	EventDetailsPaperFolderTeamInviteDetails                      = "paper_folder_team_invite_details"
	EventDetailsPasswordChangeDetails                             = "password_change_details"
	EventDetailsPasswordResetDetails                              = "password_reset_details"
	EventDetailsPasswordResetAllDetails                           = "password_reset_all_details"
	EventDetailsEmmCreateExceptionsReportDetails                  = "emm_create_exceptions_report_details"
	EventDetailsEmmCreateUsageReportDetails                       = "emm_create_usage_report_details"
	EventDetailsExportMembersReportDetails                        = "export_members_report_details"
	EventDetailsPaperAdminExportStartDetails                      = "paper_admin_export_start_details"
	EventDetailsSmartSyncCreateAdminPrivilegeReportDetails        = "smart_sync_create_admin_privilege_report_details"
	EventDetailsTeamActivityCreateReportDetails                   = "team_activity_create_report_details"
	EventDetailsCollectionShareDetails                            = "collection_share_details"
	EventDetailsNoteAclInviteOnlyDetails                          = "note_acl_invite_only_details"
	EventDetailsNoteAclLinkDetails                                = "note_acl_link_details"
	EventDetailsNoteAclTeamLinkDetails                            = "note_acl_team_link_details"
	EventDetailsNoteSharedDetails                                 = "note_shared_details"
	EventDetailsNoteShareReceiveDetails                           = "note_share_receive_details"
	EventDetailsOpenNoteSharedDetails                             = "open_note_shared_details"
	EventDetailsSfAddGroupDetails                                 = "sf_add_group_details"
	EventDetailsSfAllowNonMembersToViewSharedLinksDetails         = "sf_allow_non_members_to_view_shared_links_details"
	EventDetailsSfExternalInviteWarnDetails                       = "sf_external_invite_warn_details"
	EventDetailsSfFbInviteDetails                                 = "sf_fb_invite_details"
	EventDetailsSfFbInviteChangeRoleDetails                       = "sf_fb_invite_change_role_details"
	EventDetailsSfFbUninviteDetails                               = "sf_fb_uninvite_details"
	EventDetailsSfInviteGroupDetails                              = "sf_invite_group_details"
	EventDetailsSfTeamGrantAccessDetails                          = "sf_team_grant_access_details"
	EventDetailsSfTeamInviteDetails                               = "sf_team_invite_details"
	EventDetailsSfTeamInviteChangeRoleDetails                     = "sf_team_invite_change_role_details"
	EventDetailsSfTeamJoinDetails                                 = "sf_team_join_details"
	EventDetailsSfTeamJoinFromOobLinkDetails                      = "sf_team_join_from_oob_link_details"
	EventDetailsSfTeamUninviteDetails                             = "sf_team_uninvite_details"
	EventDetailsSharedContentAddInviteesDetails                   = "shared_content_add_invitees_details"
	EventDetailsSharedContentAddLinkExpiryDetails                 = "shared_content_add_link_expiry_details"
	EventDetailsSharedContentAddLinkPasswordDetails               = "shared_content_add_link_password_details"
	EventDetailsSharedContentAddMemberDetails                     = "shared_content_add_member_details"
	EventDetailsSharedContentChangeDownloadsPolicyDetails         = "shared_content_change_downloads_policy_details"
	EventDetailsSharedContentChangeInviteeRoleDetails             = "shared_content_change_invitee_role_details"
	EventDetailsSharedContentChangeLinkAudienceDetails            = "shared_content_change_link_audience_details"
	EventDetailsSharedContentChangeLinkExpiryDetails              = "shared_content_change_link_expiry_details"
	EventDetailsSharedContentChangeLinkPasswordDetails            = "shared_content_change_link_password_details"
	EventDetailsSharedContentChangeMemberRoleDetails              = "shared_content_change_member_role_details"
	EventDetailsSharedContentChangeViewerInfoPolicyDetails        = "shared_content_change_viewer_info_policy_details"
	EventDetailsSharedContentClaimInvitationDetails               = "shared_content_claim_invitation_details"
	EventDetailsSharedContentCopyDetails                          = "shared_content_copy_details"
	EventDetailsSharedContentDownloadDetails                      = "shared_content_download_details"
	EventDetailsSharedContentRelinquishMembershipDetails          = "shared_content_relinquish_membership_details"
	EventDetailsSharedContentRemoveInviteesDetails                = "shared_content_remove_invitees_details"
	EventDetailsSharedContentRemoveLinkExpiryDetails              = "shared_content_remove_link_expiry_details"
	EventDetailsSharedContentRemoveLinkPasswordDetails            = "shared_content_remove_link_password_details"
	EventDetailsSharedContentRemoveMemberDetails                  = "shared_content_remove_member_details"
	EventDetailsSharedContentRequestAccessDetails                 = "shared_content_request_access_details"
	EventDetailsSharedContentUnshareDetails                       = "shared_content_unshare_details"
	EventDetailsSharedContentViewDetails                          = "shared_content_view_details"
	EventDetailsSharedFolderChangeLinkPolicyDetails               = "shared_folder_change_link_policy_details"
	EventDetailsSharedFolderChangeMembersInheritancePolicyDetails = "shared_folder_change_members_inheritance_policy_details"
	EventDetailsSharedFolderChangeMembersManagementPolicyDetails  = "shared_folder_change_members_management_policy_details"
	EventDetailsSharedFolderChangeMembersPolicyDetails            = "shared_folder_change_members_policy_details"
	EventDetailsSharedFolderCreateDetails                         = "shared_folder_create_details"
	EventDetailsSharedFolderDeclineInvitationDetails              = "shared_folder_decline_invitation_details"
	EventDetailsSharedFolderMountDetails                          = "shared_folder_mount_details"
	EventDetailsSharedFolderNestDetails                           = "shared_folder_nest_details"
	EventDetailsSharedFolderTransferOwnershipDetails              = "shared_folder_transfer_ownership_details"
	EventDetailsSharedFolderUnmountDetails                        = "shared_folder_unmount_details"
	EventDetailsSharedLinkAddExpiryDetails                        = "shared_link_add_expiry_details"
	EventDetailsSharedLinkChangeExpiryDetails                     = "shared_link_change_expiry_details"
	EventDetailsSharedLinkChangeVisibilityDetails                 = "shared_link_change_visibility_details"
	EventDetailsSharedLinkCopyDetails                             = "shared_link_copy_details"
	EventDetailsSharedLinkCreateDetails                           = "shared_link_create_details"
	EventDetailsSharedLinkDisableDetails                          = "shared_link_disable_details"
	EventDetailsSharedLinkDownloadDetails                         = "shared_link_download_details"
	EventDetailsSharedLinkRemoveExpiryDetails                     = "shared_link_remove_expiry_details"
	EventDetailsSharedLinkShareDetails                            = "shared_link_share_details"
	EventDetailsSharedLinkViewDetails                             = "shared_link_view_details"
	EventDetailsSharedNoteOpenedDetails                           = "shared_note_opened_details"
	EventDetailsShmodelGroupShareDetails                          = "shmodel_group_share_details"
	EventDetailsShowcaseAccessGrantedDetails                      = "showcase_access_granted_details"
	EventDetailsShowcaseAddMemberDetails                          = "showcase_add_member_details"
	EventDetailsShowcaseArchivedDetails                           = "showcase_archived_details"
	EventDetailsShowcaseCreatedDetails                            = "showcase_created_details"
	EventDetailsShowcaseDeleteCommentDetails                      = "showcase_delete_comment_details"
	EventDetailsShowcaseEditedDetails                             = "showcase_edited_details"
	EventDetailsShowcaseEditCommentDetails                        = "showcase_edit_comment_details"
	EventDetailsShowcaseFileAddedDetails                          = "showcase_file_added_details"
	EventDetailsShowcaseFileDownloadDetails                       = "showcase_file_download_details"
	EventDetailsShowcaseFileRemovedDetails                        = "showcase_file_removed_details"
	EventDetailsShowcaseFileViewDetails                           = "showcase_file_view_details"
	EventDetailsShowcasePermanentlyDeletedDetails                 = "showcase_permanently_deleted_details"
	EventDetailsShowcasePostCommentDetails                        = "showcase_post_comment_details"
	EventDetailsShowcaseRemoveMemberDetails                       = "showcase_remove_member_details"
	EventDetailsShowcaseRenamedDetails                            = "showcase_renamed_details"
	EventDetailsShowcaseRequestAccessDetails                      = "showcase_request_access_details"
	EventDetailsShowcaseResolveCommentDetails                     = "showcase_resolve_comment_details"
	EventDetailsShowcaseRestoredDetails                           = "showcase_restored_details"
	EventDetailsShowcaseTrashedDetails                            = "showcase_trashed_details"
	EventDetailsShowcaseTrashedDeprecatedDetails                  = "showcase_trashed_deprecated_details"
	EventDetailsShowcaseUnresolveCommentDetails                   = "showcase_unresolve_comment_details"
	EventDetailsShowcaseUntrashedDetails                          = "showcase_untrashed_details"
	EventDetailsShowcaseUntrashedDeprecatedDetails                = "showcase_untrashed_deprecated_details"
	EventDetailsShowcaseViewDetails                               = "showcase_view_details"
	EventDetailsSsoAddCertDetails                                 = "sso_add_cert_details"
	EventDetailsSsoAddLoginUrlDetails                             = "sso_add_login_url_details"
	EventDetailsSsoAddLogoutUrlDetails                            = "sso_add_logout_url_details"
	EventDetailsSsoChangeCertDetails                              = "sso_change_cert_details"
	EventDetailsSsoChangeLoginUrlDetails                          = "sso_change_login_url_details"
	EventDetailsSsoChangeLogoutUrlDetails                         = "sso_change_logout_url_details"
	EventDetailsSsoChangeSamlIdentityModeDetails                  = "sso_change_saml_identity_mode_details"
	EventDetailsSsoRemoveCertDetails                              = "sso_remove_cert_details"
	EventDetailsSsoRemoveLoginUrlDetails                          = "sso_remove_login_url_details"
	EventDetailsSsoRemoveLogoutUrlDetails                         = "sso_remove_logout_url_details"
	EventDetailsTeamFolderChangeStatusDetails                     = "team_folder_change_status_details"
	EventDetailsTeamFolderCreateDetails                           = "team_folder_create_details"
	EventDetailsTeamFolderDowngradeDetails                        = "team_folder_downgrade_details"
	EventDetailsTeamFolderPermanentlyDeleteDetails                = "team_folder_permanently_delete_details"
	EventDetailsTeamFolderRenameDetails                           = "team_folder_rename_details"
	EventDetailsTeamSelectiveSyncSettingsChangedDetails           = "team_selective_sync_settings_changed_details"
	EventDetailsAccountCaptureChangePolicyDetails                 = "account_capture_change_policy_details"
	EventDetailsAllowDownloadDisabledDetails                      = "allow_download_disabled_details"
	EventDetailsAllowDownloadEnabledDetails                       = "allow_download_enabled_details"
	EventDetailsDataPlacementRestrictionChangePolicyDetails       = "data_placement_restriction_change_policy_details"
	EventDetailsDataPlacementRestrictionSatisfyPolicyDetails      = "data_placement_restriction_satisfy_policy_details"
	EventDetailsDeviceApprovalsChangeDesktopPolicyDetails         = "device_approvals_change_desktop_policy_details"
	EventDetailsDeviceApprovalsChangeMobilePolicyDetails          = "device_approvals_change_mobile_policy_details"
	EventDetailsDeviceApprovalsChangeOverageActionDetails         = "device_approvals_change_overage_action_details"
	EventDetailsDeviceApprovalsChangeUnlinkActionDetails          = "device_approvals_change_unlink_action_details"
	EventDetailsDirectoryRestrictionsAddMembersDetails            = "directory_restrictions_add_members_details"
	EventDetailsDirectoryRestrictionsRemoveMembersDetails         = "directory_restrictions_remove_members_details"
	EventDetailsEmmAddExceptionDetails                            = "emm_add_exception_details"
	EventDetailsEmmChangePolicyDetails                            = "emm_change_policy_details"
	EventDetailsEmmRemoveExceptionDetails                         = "emm_remove_exception_details"
	EventDetailsExtendedVersionHistoryChangePolicyDetails         = "extended_version_history_change_policy_details"
	EventDetailsFileCommentsChangePolicyDetails                   = "file_comments_change_policy_details"
	EventDetailsFileRequestsChangePolicyDetails                   = "file_requests_change_policy_details"
	EventDetailsFileRequestsEmailsEnabledDetails                  = "file_requests_emails_enabled_details"
	EventDetailsFileRequestsEmailsRestrictedToTeamOnlyDetails     = "file_requests_emails_restricted_to_team_only_details"
	EventDetailsGoogleSsoChangePolicyDetails                      = "google_sso_change_policy_details"
	EventDetailsGroupUserManagementChangePolicyDetails            = "group_user_management_change_policy_details"
	EventDetailsMemberRequestsChangePolicyDetails                 = "member_requests_change_policy_details"
	EventDetailsMemberSpaceLimitsAddExceptionDetails              = "member_space_limits_add_exception_details"
	EventDetailsMemberSpaceLimitsChangeCapsTypePolicyDetails      = "member_space_limits_change_caps_type_policy_details"
	EventDetailsMemberSpaceLimitsChangePolicyDetails              = "member_space_limits_change_policy_details"
	EventDetailsMemberSpaceLimitsRemoveExceptionDetails           = "member_space_limits_remove_exception_details"
	EventDetailsMemberSuggestionsChangePolicyDetails              = "member_suggestions_change_policy_details"
	EventDetailsMicrosoftOfficeAddinChangePolicyDetails           = "microsoft_office_addin_change_policy_details"
	EventDetailsNetworkControlChangePolicyDetails                 = "network_control_change_policy_details"
	EventDetailsPaperChangeDeploymentPolicyDetails                = "paper_change_deployment_policy_details"
	EventDetailsPaperChangeMemberLinkPolicyDetails                = "paper_change_member_link_policy_details"
	EventDetailsPaperChangeMemberPolicyDetails                    = "paper_change_member_policy_details"
	EventDetailsPaperChangePolicyDetails                          = "paper_change_policy_details"
	EventDetailsPaperEnabledUsersGroupAdditionDetails             = "paper_enabled_users_group_addition_details"
	EventDetailsPaperEnabledUsersGroupRemovalDetails              = "paper_enabled_users_group_removal_details"
	EventDetailsPermanentDeleteChangePolicyDetails                = "permanent_delete_change_policy_details"
	EventDetailsSharingChangeFolderJoinPolicyDetails              = "sharing_change_folder_join_policy_details"
	EventDetailsSharingChangeLinkPolicyDetails                    = "sharing_change_link_policy_details"
	EventDetailsSharingChangeMemberPolicyDetails                  = "sharing_change_member_policy_details"
	EventDetailsShowcaseChangeDownloadPolicyDetails               = "showcase_change_download_policy_details"
	EventDetailsShowcaseChangeEnabledPolicyDetails                = "showcase_change_enabled_policy_details"
	EventDetailsShowcaseChangeExternalSharingPolicyDetails        = "showcase_change_external_sharing_policy_details"
	EventDetailsSmartSyncChangePolicyDetails                      = "smart_sync_change_policy_details"
	EventDetailsSmartSyncNotOptOutDetails                         = "smart_sync_not_opt_out_details"
	EventDetailsSmartSyncOptOutDetails                            = "smart_sync_opt_out_details"
	EventDetailsSsoChangePolicyDetails                            = "sso_change_policy_details"
	EventDetailsTfaChangePolicyDetails                            = "tfa_change_policy_details"
	EventDetailsTwoAccountChangePolicyDetails                     = "two_account_change_policy_details"
	EventDetailsWebSessionsChangeFixedLengthPolicyDetails         = "web_sessions_change_fixed_length_policy_details"
	EventDetailsWebSessionsChangeIdleLengthPolicyDetails          = "web_sessions_change_idle_length_policy_details"
	EventDetailsTeamMergeFromDetails                              = "team_merge_from_details"
	EventDetailsTeamMergeToDetails                                = "team_merge_to_details"
	EventDetailsTeamProfileAddLogoDetails                         = "team_profile_add_logo_details"
	EventDetailsTeamProfileChangeDefaultLanguageDetails           = "team_profile_change_default_language_details"
	EventDetailsTeamProfileChangeLogoDetails                      = "team_profile_change_logo_details"
	EventDetailsTeamProfileChangeNameDetails                      = "team_profile_change_name_details"
	EventDetailsTeamProfileRemoveLogoDetails                      = "team_profile_remove_logo_details"
	EventDetailsTfaAddBackupPhoneDetails                          = "tfa_add_backup_phone_details"
	EventDetailsTfaAddSecurityKeyDetails                          = "tfa_add_security_key_details"
	EventDetailsTfaChangeBackupPhoneDetails                       = "tfa_change_backup_phone_details"
	EventDetailsTfaChangeStatusDetails                            = "tfa_change_status_details"
	EventDetailsTfaRemoveBackupPhoneDetails                       = "tfa_remove_backup_phone_details"
	EventDetailsTfaRemoveSecurityKeyDetails                       = "tfa_remove_security_key_details"
	EventDetailsTfaResetDetails                                   = "tfa_reset_details"
	EventDetailsMissingDetails                                    = "missing_details"
	EventDetailsOther                                             = "other"
)

// UnmarshalJSON deserializes into a EventDetails instance
func (u *EventDetails) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AppLinkTeamDetails : has no documentation (yet)
		AppLinkTeamDetails json.RawMessage `json:"app_link_team_details,omitempty"`
		// AppLinkUserDetails : has no documentation (yet)
		AppLinkUserDetails json.RawMessage `json:"app_link_user_details,omitempty"`
		// AppUnlinkTeamDetails : has no documentation (yet)
		AppUnlinkTeamDetails json.RawMessage `json:"app_unlink_team_details,omitempty"`
		// AppUnlinkUserDetails : has no documentation (yet)
		AppUnlinkUserDetails json.RawMessage `json:"app_unlink_user_details,omitempty"`
		// FileAddCommentDetails : has no documentation (yet)
		FileAddCommentDetails json.RawMessage `json:"file_add_comment_details,omitempty"`
		// FileChangeCommentSubscriptionDetails : has no documentation (yet)
		FileChangeCommentSubscriptionDetails json.RawMessage `json:"file_change_comment_subscription_details,omitempty"`
		// FileDeleteCommentDetails : has no documentation (yet)
		FileDeleteCommentDetails json.RawMessage `json:"file_delete_comment_details,omitempty"`
		// FileLikeCommentDetails : has no documentation (yet)
		FileLikeCommentDetails json.RawMessage `json:"file_like_comment_details,omitempty"`
		// FileResolveCommentDetails : has no documentation (yet)
		FileResolveCommentDetails json.RawMessage `json:"file_resolve_comment_details,omitempty"`
		// FileUnlikeCommentDetails : has no documentation (yet)
		FileUnlikeCommentDetails json.RawMessage `json:"file_unlike_comment_details,omitempty"`
		// FileUnresolveCommentDetails : has no documentation (yet)
		FileUnresolveCommentDetails json.RawMessage `json:"file_unresolve_comment_details,omitempty"`
		// DeviceChangeIpDesktopDetails : has no documentation (yet)
		DeviceChangeIpDesktopDetails json.RawMessage `json:"device_change_ip_desktop_details,omitempty"`
		// DeviceChangeIpMobileDetails : has no documentation (yet)
		DeviceChangeIpMobileDetails json.RawMessage `json:"device_change_ip_mobile_details,omitempty"`
		// DeviceChangeIpWebDetails : has no documentation (yet)
		DeviceChangeIpWebDetails json.RawMessage `json:"device_change_ip_web_details,omitempty"`
		// DeviceDeleteOnUnlinkFailDetails : has no documentation (yet)
		DeviceDeleteOnUnlinkFailDetails json.RawMessage `json:"device_delete_on_unlink_fail_details,omitempty"`
		// DeviceDeleteOnUnlinkSuccessDetails : has no documentation (yet)
		DeviceDeleteOnUnlinkSuccessDetails json.RawMessage `json:"device_delete_on_unlink_success_details,omitempty"`
		// DeviceLinkFailDetails : has no documentation (yet)
		DeviceLinkFailDetails json.RawMessage `json:"device_link_fail_details,omitempty"`
		// DeviceLinkSuccessDetails : has no documentation (yet)
		DeviceLinkSuccessDetails json.RawMessage `json:"device_link_success_details,omitempty"`
		// DeviceManagementDisabledDetails : has no documentation (yet)
		DeviceManagementDisabledDetails json.RawMessage `json:"device_management_disabled_details,omitempty"`
		// DeviceManagementEnabledDetails : has no documentation (yet)
		DeviceManagementEnabledDetails json.RawMessage `json:"device_management_enabled_details,omitempty"`
		// DeviceUnlinkDetails : has no documentation (yet)
		DeviceUnlinkDetails json.RawMessage `json:"device_unlink_details,omitempty"`
		// EmmRefreshAuthTokenDetails : has no documentation (yet)
		EmmRefreshAuthTokenDetails json.RawMessage `json:"emm_refresh_auth_token_details,omitempty"`
		// AccountCaptureChangeAvailabilityDetails : has no documentation (yet)
		AccountCaptureChangeAvailabilityDetails json.RawMessage `json:"account_capture_change_availability_details,omitempty"`
		// AccountCaptureMigrateAccountDetails : has no documentation (yet)
		AccountCaptureMigrateAccountDetails json.RawMessage `json:"account_capture_migrate_account_details,omitempty"`
		// AccountCaptureNotificationEmailsSentDetails : has no documentation
		// (yet)
		AccountCaptureNotificationEmailsSentDetails json.RawMessage `json:"account_capture_notification_emails_sent_details,omitempty"`
		// AccountCaptureRelinquishAccountDetails : has no documentation (yet)
		AccountCaptureRelinquishAccountDetails json.RawMessage `json:"account_capture_relinquish_account_details,omitempty"`
		// DisabledDomainInvitesDetails : has no documentation (yet)
		DisabledDomainInvitesDetails json.RawMessage `json:"disabled_domain_invites_details,omitempty"`
		// DomainInvitesApproveRequestToJoinTeamDetails : has no documentation
		// (yet)
		DomainInvitesApproveRequestToJoinTeamDetails json.RawMessage `json:"domain_invites_approve_request_to_join_team_details,omitempty"`
		// DomainInvitesDeclineRequestToJoinTeamDetails : has no documentation
		// (yet)
		DomainInvitesDeclineRequestToJoinTeamDetails json.RawMessage `json:"domain_invites_decline_request_to_join_team_details,omitempty"`
		// DomainInvitesEmailExistingUsersDetails : has no documentation (yet)
		DomainInvitesEmailExistingUsersDetails json.RawMessage `json:"domain_invites_email_existing_users_details,omitempty"`
		// DomainInvitesRequestToJoinTeamDetails : has no documentation (yet)
		DomainInvitesRequestToJoinTeamDetails json.RawMessage `json:"domain_invites_request_to_join_team_details,omitempty"`
		// DomainInvitesSetInviteNewUserPrefToNoDetails : has no documentation
		// (yet)
		DomainInvitesSetInviteNewUserPrefToNoDetails json.RawMessage `json:"domain_invites_set_invite_new_user_pref_to_no_details,omitempty"`
		// DomainInvitesSetInviteNewUserPrefToYesDetails : has no documentation
		// (yet)
		DomainInvitesSetInviteNewUserPrefToYesDetails json.RawMessage `json:"domain_invites_set_invite_new_user_pref_to_yes_details,omitempty"`
		// DomainVerificationAddDomainFailDetails : has no documentation (yet)
		DomainVerificationAddDomainFailDetails json.RawMessage `json:"domain_verification_add_domain_fail_details,omitempty"`
		// DomainVerificationAddDomainSuccessDetails : has no documentation
		// (yet)
		DomainVerificationAddDomainSuccessDetails json.RawMessage `json:"domain_verification_add_domain_success_details,omitempty"`
		// DomainVerificationRemoveDomainDetails : has no documentation (yet)
		DomainVerificationRemoveDomainDetails json.RawMessage `json:"domain_verification_remove_domain_details,omitempty"`
		// EnabledDomainInvitesDetails : has no documentation (yet)
		EnabledDomainInvitesDetails json.RawMessage `json:"enabled_domain_invites_details,omitempty"`
		// CreateFolderDetails : has no documentation (yet)
		CreateFolderDetails json.RawMessage `json:"create_folder_details,omitempty"`
		// FileAddDetails : has no documentation (yet)
		FileAddDetails json.RawMessage `json:"file_add_details,omitempty"`
		// FileCopyDetails : has no documentation (yet)
		FileCopyDetails json.RawMessage `json:"file_copy_details,omitempty"`
		// FileDeleteDetails : has no documentation (yet)
		FileDeleteDetails json.RawMessage `json:"file_delete_details,omitempty"`
		// FileDownloadDetails : has no documentation (yet)
		FileDownloadDetails json.RawMessage `json:"file_download_details,omitempty"`
		// FileEditDetails : has no documentation (yet)
		FileEditDetails json.RawMessage `json:"file_edit_details,omitempty"`
		// FileGetCopyReferenceDetails : has no documentation (yet)
		FileGetCopyReferenceDetails json.RawMessage `json:"file_get_copy_reference_details,omitempty"`
		// FileMoveDetails : has no documentation (yet)
		FileMoveDetails json.RawMessage `json:"file_move_details,omitempty"`
		// FilePermanentlyDeleteDetails : has no documentation (yet)
		FilePermanentlyDeleteDetails json.RawMessage `json:"file_permanently_delete_details,omitempty"`
		// FilePreviewDetails : has no documentation (yet)
		FilePreviewDetails json.RawMessage `json:"file_preview_details,omitempty"`
		// FileRenameDetails : has no documentation (yet)
		FileRenameDetails json.RawMessage `json:"file_rename_details,omitempty"`
		// FileRestoreDetails : has no documentation (yet)
		FileRestoreDetails json.RawMessage `json:"file_restore_details,omitempty"`
		// FileRevertDetails : has no documentation (yet)
		FileRevertDetails json.RawMessage `json:"file_revert_details,omitempty"`
		// FileRollbackChangesDetails : has no documentation (yet)
		FileRollbackChangesDetails json.RawMessage `json:"file_rollback_changes_details,omitempty"`
		// FileSaveCopyReferenceDetails : has no documentation (yet)
		FileSaveCopyReferenceDetails json.RawMessage `json:"file_save_copy_reference_details,omitempty"`
		// FileRequestChangeDetails : has no documentation (yet)
		FileRequestChangeDetails json.RawMessage `json:"file_request_change_details,omitempty"`
		// FileRequestCloseDetails : has no documentation (yet)
		FileRequestCloseDetails json.RawMessage `json:"file_request_close_details,omitempty"`
		// FileRequestCreateDetails : has no documentation (yet)
		FileRequestCreateDetails json.RawMessage `json:"file_request_create_details,omitempty"`
		// FileRequestReceiveFileDetails : has no documentation (yet)
		FileRequestReceiveFileDetails json.RawMessage `json:"file_request_receive_file_details,omitempty"`
		// GroupAddExternalIdDetails : has no documentation (yet)
		GroupAddExternalIdDetails json.RawMessage `json:"group_add_external_id_details,omitempty"`
		// GroupAddMemberDetails : has no documentation (yet)
		GroupAddMemberDetails json.RawMessage `json:"group_add_member_details,omitempty"`
		// GroupChangeExternalIdDetails : has no documentation (yet)
		GroupChangeExternalIdDetails json.RawMessage `json:"group_change_external_id_details,omitempty"`
		// GroupChangeManagementTypeDetails : has no documentation (yet)
		GroupChangeManagementTypeDetails json.RawMessage `json:"group_change_management_type_details,omitempty"`
		// GroupChangeMemberRoleDetails : has no documentation (yet)
		GroupChangeMemberRoleDetails json.RawMessage `json:"group_change_member_role_details,omitempty"`
		// GroupCreateDetails : has no documentation (yet)
		GroupCreateDetails json.RawMessage `json:"group_create_details,omitempty"`
		// GroupDeleteDetails : has no documentation (yet)
		GroupDeleteDetails json.RawMessage `json:"group_delete_details,omitempty"`
		// GroupDescriptionUpdatedDetails : has no documentation (yet)
		GroupDescriptionUpdatedDetails json.RawMessage `json:"group_description_updated_details,omitempty"`
		// GroupJoinPolicyUpdatedDetails : has no documentation (yet)
		GroupJoinPolicyUpdatedDetails json.RawMessage `json:"group_join_policy_updated_details,omitempty"`
		// GroupMovedDetails : has no documentation (yet)
		GroupMovedDetails json.RawMessage `json:"group_moved_details,omitempty"`
		// GroupRemoveExternalIdDetails : has no documentation (yet)
		GroupRemoveExternalIdDetails json.RawMessage `json:"group_remove_external_id_details,omitempty"`
		// GroupRemoveMemberDetails : has no documentation (yet)
		GroupRemoveMemberDetails json.RawMessage `json:"group_remove_member_details,omitempty"`
		// GroupRenameDetails : has no documentation (yet)
		GroupRenameDetails json.RawMessage `json:"group_rename_details,omitempty"`
		// EmmErrorDetails : has no documentation (yet)
		EmmErrorDetails json.RawMessage `json:"emm_error_details,omitempty"`
		// LoginFailDetails : has no documentation (yet)
		LoginFailDetails json.RawMessage `json:"login_fail_details,omitempty"`
		// LoginSuccessDetails : has no documentation (yet)
		LoginSuccessDetails json.RawMessage `json:"login_success_details,omitempty"`
		// LogoutDetails : has no documentation (yet)
		LogoutDetails json.RawMessage `json:"logout_details,omitempty"`
		// ResellerSupportSessionEndDetails : has no documentation (yet)
		ResellerSupportSessionEndDetails json.RawMessage `json:"reseller_support_session_end_details,omitempty"`
		// ResellerSupportSessionStartDetails : has no documentation (yet)
		ResellerSupportSessionStartDetails json.RawMessage `json:"reseller_support_session_start_details,omitempty"`
		// SignInAsSessionEndDetails : has no documentation (yet)
		SignInAsSessionEndDetails json.RawMessage `json:"sign_in_as_session_end_details,omitempty"`
		// SignInAsSessionStartDetails : has no documentation (yet)
		SignInAsSessionStartDetails json.RawMessage `json:"sign_in_as_session_start_details,omitempty"`
		// SsoErrorDetails : has no documentation (yet)
		SsoErrorDetails json.RawMessage `json:"sso_error_details,omitempty"`
		// MemberAddNameDetails : has no documentation (yet)
		MemberAddNameDetails json.RawMessage `json:"member_add_name_details,omitempty"`
		// MemberChangeAdminRoleDetails : has no documentation (yet)
		MemberChangeAdminRoleDetails json.RawMessage `json:"member_change_admin_role_details,omitempty"`
		// MemberChangeEmailDetails : has no documentation (yet)
		MemberChangeEmailDetails json.RawMessage `json:"member_change_email_details,omitempty"`
		// MemberChangeMembershipTypeDetails : has no documentation (yet)
		MemberChangeMembershipTypeDetails json.RawMessage `json:"member_change_membership_type_details,omitempty"`
		// MemberChangeNameDetails : has no documentation (yet)
		MemberChangeNameDetails json.RawMessage `json:"member_change_name_details,omitempty"`
		// MemberChangeStatusDetails : has no documentation (yet)
		MemberChangeStatusDetails json.RawMessage `json:"member_change_status_details,omitempty"`
		// MemberPermanentlyDeleteAccountContentsDetails : has no documentation
		// (yet)
		MemberPermanentlyDeleteAccountContentsDetails json.RawMessage `json:"member_permanently_delete_account_contents_details,omitempty"`
		// MemberSpaceLimitsAddCustomQuotaDetails : has no documentation (yet)
		MemberSpaceLimitsAddCustomQuotaDetails json.RawMessage `json:"member_space_limits_add_custom_quota_details,omitempty"`
		// MemberSpaceLimitsChangeCustomQuotaDetails : has no documentation
		// (yet)
		MemberSpaceLimitsChangeCustomQuotaDetails json.RawMessage `json:"member_space_limits_change_custom_quota_details,omitempty"`
		// MemberSpaceLimitsChangeStatusDetails : has no documentation (yet)
		MemberSpaceLimitsChangeStatusDetails json.RawMessage `json:"member_space_limits_change_status_details,omitempty"`
		// MemberSpaceLimitsRemoveCustomQuotaDetails : has no documentation
		// (yet)
		MemberSpaceLimitsRemoveCustomQuotaDetails json.RawMessage `json:"member_space_limits_remove_custom_quota_details,omitempty"`
		// MemberSuggestDetails : has no documentation (yet)
		MemberSuggestDetails json.RawMessage `json:"member_suggest_details,omitempty"`
		// MemberTransferAccountContentsDetails : has no documentation (yet)
		MemberTransferAccountContentsDetails json.RawMessage `json:"member_transfer_account_contents_details,omitempty"`
		// SecondaryMailsPolicyChangedDetails : has no documentation (yet)
		SecondaryMailsPolicyChangedDetails json.RawMessage `json:"secondary_mails_policy_changed_details,omitempty"`
		// PaperContentAddMemberDetails : has no documentation (yet)
		PaperContentAddMemberDetails json.RawMessage `json:"paper_content_add_member_details,omitempty"`
		// PaperContentAddToFolderDetails : has no documentation (yet)
		PaperContentAddToFolderDetails json.RawMessage `json:"paper_content_add_to_folder_details,omitempty"`
		// PaperContentArchiveDetails : has no documentation (yet)
		PaperContentArchiveDetails json.RawMessage `json:"paper_content_archive_details,omitempty"`
		// PaperContentCreateDetails : has no documentation (yet)
		PaperContentCreateDetails json.RawMessage `json:"paper_content_create_details,omitempty"`
		// PaperContentPermanentlyDeleteDetails : has no documentation (yet)
		PaperContentPermanentlyDeleteDetails json.RawMessage `json:"paper_content_permanently_delete_details,omitempty"`
		// PaperContentRemoveFromFolderDetails : has no documentation (yet)
		PaperContentRemoveFromFolderDetails json.RawMessage `json:"paper_content_remove_from_folder_details,omitempty"`
		// PaperContentRemoveMemberDetails : has no documentation (yet)
		PaperContentRemoveMemberDetails json.RawMessage `json:"paper_content_remove_member_details,omitempty"`
		// PaperContentRenameDetails : has no documentation (yet)
		PaperContentRenameDetails json.RawMessage `json:"paper_content_rename_details,omitempty"`
		// PaperContentRestoreDetails : has no documentation (yet)
		PaperContentRestoreDetails json.RawMessage `json:"paper_content_restore_details,omitempty"`
		// PaperDocAddCommentDetails : has no documentation (yet)
		PaperDocAddCommentDetails json.RawMessage `json:"paper_doc_add_comment_details,omitempty"`
		// PaperDocChangeMemberRoleDetails : has no documentation (yet)
		PaperDocChangeMemberRoleDetails json.RawMessage `json:"paper_doc_change_member_role_details,omitempty"`
		// PaperDocChangeSharingPolicyDetails : has no documentation (yet)
		PaperDocChangeSharingPolicyDetails json.RawMessage `json:"paper_doc_change_sharing_policy_details,omitempty"`
		// PaperDocChangeSubscriptionDetails : has no documentation (yet)
		PaperDocChangeSubscriptionDetails json.RawMessage `json:"paper_doc_change_subscription_details,omitempty"`
		// PaperDocDeletedDetails : has no documentation (yet)
		PaperDocDeletedDetails json.RawMessage `json:"paper_doc_deleted_details,omitempty"`
		// PaperDocDeleteCommentDetails : has no documentation (yet)
		PaperDocDeleteCommentDetails json.RawMessage `json:"paper_doc_delete_comment_details,omitempty"`
		// PaperDocDownloadDetails : has no documentation (yet)
		PaperDocDownloadDetails json.RawMessage `json:"paper_doc_download_details,omitempty"`
		// PaperDocEditDetails : has no documentation (yet)
		PaperDocEditDetails json.RawMessage `json:"paper_doc_edit_details,omitempty"`
		// PaperDocEditCommentDetails : has no documentation (yet)
		PaperDocEditCommentDetails json.RawMessage `json:"paper_doc_edit_comment_details,omitempty"`
		// PaperDocFollowedDetails : has no documentation (yet)
		PaperDocFollowedDetails json.RawMessage `json:"paper_doc_followed_details,omitempty"`
		// PaperDocMentionDetails : has no documentation (yet)
		PaperDocMentionDetails json.RawMessage `json:"paper_doc_mention_details,omitempty"`
		// PaperDocRequestAccessDetails : has no documentation (yet)
		PaperDocRequestAccessDetails json.RawMessage `json:"paper_doc_request_access_details,omitempty"`
		// PaperDocResolveCommentDetails : has no documentation (yet)
		PaperDocResolveCommentDetails json.RawMessage `json:"paper_doc_resolve_comment_details,omitempty"`
		// PaperDocRevertDetails : has no documentation (yet)
		PaperDocRevertDetails json.RawMessage `json:"paper_doc_revert_details,omitempty"`
		// PaperDocSlackShareDetails : has no documentation (yet)
		PaperDocSlackShareDetails json.RawMessage `json:"paper_doc_slack_share_details,omitempty"`
		// PaperDocTeamInviteDetails : has no documentation (yet)
		PaperDocTeamInviteDetails json.RawMessage `json:"paper_doc_team_invite_details,omitempty"`
		// PaperDocTrashedDetails : has no documentation (yet)
		PaperDocTrashedDetails json.RawMessage `json:"paper_doc_trashed_details,omitempty"`
		// PaperDocUnresolveCommentDetails : has no documentation (yet)
		PaperDocUnresolveCommentDetails json.RawMessage `json:"paper_doc_unresolve_comment_details,omitempty"`
		// PaperDocUntrashedDetails : has no documentation (yet)
		PaperDocUntrashedDetails json.RawMessage `json:"paper_doc_untrashed_details,omitempty"`
		// PaperDocViewDetails : has no documentation (yet)
		PaperDocViewDetails json.RawMessage `json:"paper_doc_view_details,omitempty"`
		// PaperExternalViewAllowDetails : has no documentation (yet)
		PaperExternalViewAllowDetails json.RawMessage `json:"paper_external_view_allow_details,omitempty"`
		// PaperExternalViewDefaultTeamDetails : has no documentation (yet)
		PaperExternalViewDefaultTeamDetails json.RawMessage `json:"paper_external_view_default_team_details,omitempty"`
		// PaperExternalViewForbidDetails : has no documentation (yet)
		PaperExternalViewForbidDetails json.RawMessage `json:"paper_external_view_forbid_details,omitempty"`
		// PaperFolderChangeSubscriptionDetails : has no documentation (yet)
		PaperFolderChangeSubscriptionDetails json.RawMessage `json:"paper_folder_change_subscription_details,omitempty"`
		// PaperFolderDeletedDetails : has no documentation (yet)
		PaperFolderDeletedDetails json.RawMessage `json:"paper_folder_deleted_details,omitempty"`
		// PaperFolderFollowedDetails : has no documentation (yet)
		PaperFolderFollowedDetails json.RawMessage `json:"paper_folder_followed_details,omitempty"`
		// PaperFolderTeamInviteDetails : has no documentation (yet)
		PaperFolderTeamInviteDetails json.RawMessage `json:"paper_folder_team_invite_details,omitempty"`
		// PasswordChangeDetails : has no documentation (yet)
		PasswordChangeDetails json.RawMessage `json:"password_change_details,omitempty"`
		// PasswordResetDetails : has no documentation (yet)
		PasswordResetDetails json.RawMessage `json:"password_reset_details,omitempty"`
		// PasswordResetAllDetails : has no documentation (yet)
		PasswordResetAllDetails json.RawMessage `json:"password_reset_all_details,omitempty"`
		// EmmCreateExceptionsReportDetails : has no documentation (yet)
		EmmCreateExceptionsReportDetails json.RawMessage `json:"emm_create_exceptions_report_details,omitempty"`
		// EmmCreateUsageReportDetails : has no documentation (yet)
		EmmCreateUsageReportDetails json.RawMessage `json:"emm_create_usage_report_details,omitempty"`
		// ExportMembersReportDetails : has no documentation (yet)
		ExportMembersReportDetails json.RawMessage `json:"export_members_report_details,omitempty"`
		// PaperAdminExportStartDetails : has no documentation (yet)
		PaperAdminExportStartDetails json.RawMessage `json:"paper_admin_export_start_details,omitempty"`
		// SmartSyncCreateAdminPrivilegeReportDetails : has no documentation
		// (yet)
		SmartSyncCreateAdminPrivilegeReportDetails json.RawMessage `json:"smart_sync_create_admin_privilege_report_details,omitempty"`
		// TeamActivityCreateReportDetails : has no documentation (yet)
		TeamActivityCreateReportDetails json.RawMessage `json:"team_activity_create_report_details,omitempty"`
		// CollectionShareDetails : has no documentation (yet)
		CollectionShareDetails json.RawMessage `json:"collection_share_details,omitempty"`
		// NoteAclInviteOnlyDetails : has no documentation (yet)
		NoteAclInviteOnlyDetails json.RawMessage `json:"note_acl_invite_only_details,omitempty"`
		// NoteAclLinkDetails : has no documentation (yet)
		NoteAclLinkDetails json.RawMessage `json:"note_acl_link_details,omitempty"`
		// NoteAclTeamLinkDetails : has no documentation (yet)
		NoteAclTeamLinkDetails json.RawMessage `json:"note_acl_team_link_details,omitempty"`
		// NoteSharedDetails : has no documentation (yet)
		NoteSharedDetails json.RawMessage `json:"note_shared_details,omitempty"`
		// NoteShareReceiveDetails : has no documentation (yet)
		NoteShareReceiveDetails json.RawMessage `json:"note_share_receive_details,omitempty"`
		// OpenNoteSharedDetails : has no documentation (yet)
		OpenNoteSharedDetails json.RawMessage `json:"open_note_shared_details,omitempty"`
		// SfAddGroupDetails : has no documentation (yet)
		SfAddGroupDetails json.RawMessage `json:"sf_add_group_details,omitempty"`
		// SfAllowNonMembersToViewSharedLinksDetails : has no documentation
		// (yet)
		SfAllowNonMembersToViewSharedLinksDetails json.RawMessage `json:"sf_allow_non_members_to_view_shared_links_details,omitempty"`
		// SfExternalInviteWarnDetails : has no documentation (yet)
		SfExternalInviteWarnDetails json.RawMessage `json:"sf_external_invite_warn_details,omitempty"`
		// SfFbInviteDetails : has no documentation (yet)
		SfFbInviteDetails json.RawMessage `json:"sf_fb_invite_details,omitempty"`
		// SfFbInviteChangeRoleDetails : has no documentation (yet)
		SfFbInviteChangeRoleDetails json.RawMessage `json:"sf_fb_invite_change_role_details,omitempty"`
		// SfFbUninviteDetails : has no documentation (yet)
		SfFbUninviteDetails json.RawMessage `json:"sf_fb_uninvite_details,omitempty"`
		// SfInviteGroupDetails : has no documentation (yet)
		SfInviteGroupDetails json.RawMessage `json:"sf_invite_group_details,omitempty"`
		// SfTeamGrantAccessDetails : has no documentation (yet)
		SfTeamGrantAccessDetails json.RawMessage `json:"sf_team_grant_access_details,omitempty"`
		// SfTeamInviteDetails : has no documentation (yet)
		SfTeamInviteDetails json.RawMessage `json:"sf_team_invite_details,omitempty"`
		// SfTeamInviteChangeRoleDetails : has no documentation (yet)
		SfTeamInviteChangeRoleDetails json.RawMessage `json:"sf_team_invite_change_role_details,omitempty"`
		// SfTeamJoinDetails : has no documentation (yet)
		SfTeamJoinDetails json.RawMessage `json:"sf_team_join_details,omitempty"`
		// SfTeamJoinFromOobLinkDetails : has no documentation (yet)
		SfTeamJoinFromOobLinkDetails json.RawMessage `json:"sf_team_join_from_oob_link_details,omitempty"`
		// SfTeamUninviteDetails : has no documentation (yet)
		SfTeamUninviteDetails json.RawMessage `json:"sf_team_uninvite_details,omitempty"`
		// SharedContentAddInviteesDetails : has no documentation (yet)
		SharedContentAddInviteesDetails json.RawMessage `json:"shared_content_add_invitees_details,omitempty"`
		// SharedContentAddLinkExpiryDetails : has no documentation (yet)
		SharedContentAddLinkExpiryDetails json.RawMessage `json:"shared_content_add_link_expiry_details,omitempty"`
		// SharedContentAddLinkPasswordDetails : has no documentation (yet)
		SharedContentAddLinkPasswordDetails json.RawMessage `json:"shared_content_add_link_password_details,omitempty"`
		// SharedContentAddMemberDetails : has no documentation (yet)
		SharedContentAddMemberDetails json.RawMessage `json:"shared_content_add_member_details,omitempty"`
		// SharedContentChangeDownloadsPolicyDetails : has no documentation
		// (yet)
		SharedContentChangeDownloadsPolicyDetails json.RawMessage `json:"shared_content_change_downloads_policy_details,omitempty"`
		// SharedContentChangeInviteeRoleDetails : has no documentation (yet)
		SharedContentChangeInviteeRoleDetails json.RawMessage `json:"shared_content_change_invitee_role_details,omitempty"`
		// SharedContentChangeLinkAudienceDetails : has no documentation (yet)
		SharedContentChangeLinkAudienceDetails json.RawMessage `json:"shared_content_change_link_audience_details,omitempty"`
		// SharedContentChangeLinkExpiryDetails : has no documentation (yet)
		SharedContentChangeLinkExpiryDetails json.RawMessage `json:"shared_content_change_link_expiry_details,omitempty"`
		// SharedContentChangeLinkPasswordDetails : has no documentation (yet)
		SharedContentChangeLinkPasswordDetails json.RawMessage `json:"shared_content_change_link_password_details,omitempty"`
		// SharedContentChangeMemberRoleDetails : has no documentation (yet)
		SharedContentChangeMemberRoleDetails json.RawMessage `json:"shared_content_change_member_role_details,omitempty"`
		// SharedContentChangeViewerInfoPolicyDetails : has no documentation
		// (yet)
		SharedContentChangeViewerInfoPolicyDetails json.RawMessage `json:"shared_content_change_viewer_info_policy_details,omitempty"`
		// SharedContentClaimInvitationDetails : has no documentation (yet)
		SharedContentClaimInvitationDetails json.RawMessage `json:"shared_content_claim_invitation_details,omitempty"`
		// SharedContentCopyDetails : has no documentation (yet)
		SharedContentCopyDetails json.RawMessage `json:"shared_content_copy_details,omitempty"`
		// SharedContentDownloadDetails : has no documentation (yet)
		SharedContentDownloadDetails json.RawMessage `json:"shared_content_download_details,omitempty"`
		// SharedContentRelinquishMembershipDetails : has no documentation (yet)
		SharedContentRelinquishMembershipDetails json.RawMessage `json:"shared_content_relinquish_membership_details,omitempty"`
		// SharedContentRemoveInviteesDetails : has no documentation (yet)
		SharedContentRemoveInviteesDetails json.RawMessage `json:"shared_content_remove_invitees_details,omitempty"`
		// SharedContentRemoveLinkExpiryDetails : has no documentation (yet)
		SharedContentRemoveLinkExpiryDetails json.RawMessage `json:"shared_content_remove_link_expiry_details,omitempty"`
		// SharedContentRemoveLinkPasswordDetails : has no documentation (yet)
		SharedContentRemoveLinkPasswordDetails json.RawMessage `json:"shared_content_remove_link_password_details,omitempty"`
		// SharedContentRemoveMemberDetails : has no documentation (yet)
		SharedContentRemoveMemberDetails json.RawMessage `json:"shared_content_remove_member_details,omitempty"`
		// SharedContentRequestAccessDetails : has no documentation (yet)
		SharedContentRequestAccessDetails json.RawMessage `json:"shared_content_request_access_details,omitempty"`
		// SharedContentUnshareDetails : has no documentation (yet)
		SharedContentUnshareDetails json.RawMessage `json:"shared_content_unshare_details,omitempty"`
		// SharedContentViewDetails : has no documentation (yet)
		SharedContentViewDetails json.RawMessage `json:"shared_content_view_details,omitempty"`
		// SharedFolderChangeLinkPolicyDetails : has no documentation (yet)
		SharedFolderChangeLinkPolicyDetails json.RawMessage `json:"shared_folder_change_link_policy_details,omitempty"`
		// SharedFolderChangeMembersInheritancePolicyDetails : has no
		// documentation (yet)
		SharedFolderChangeMembersInheritancePolicyDetails json.RawMessage `json:"shared_folder_change_members_inheritance_policy_details,omitempty"`
		// SharedFolderChangeMembersManagementPolicyDetails : has no
		// documentation (yet)
		SharedFolderChangeMembersManagementPolicyDetails json.RawMessage `json:"shared_folder_change_members_management_policy_details,omitempty"`
		// SharedFolderChangeMembersPolicyDetails : has no documentation (yet)
		SharedFolderChangeMembersPolicyDetails json.RawMessage `json:"shared_folder_change_members_policy_details,omitempty"`
		// SharedFolderCreateDetails : has no documentation (yet)
		SharedFolderCreateDetails json.RawMessage `json:"shared_folder_create_details,omitempty"`
		// SharedFolderDeclineInvitationDetails : has no documentation (yet)
		SharedFolderDeclineInvitationDetails json.RawMessage `json:"shared_folder_decline_invitation_details,omitempty"`
		// SharedFolderMountDetails : has no documentation (yet)
		SharedFolderMountDetails json.RawMessage `json:"shared_folder_mount_details,omitempty"`
		// SharedFolderNestDetails : has no documentation (yet)
		SharedFolderNestDetails json.RawMessage `json:"shared_folder_nest_details,omitempty"`
		// SharedFolderTransferOwnershipDetails : has no documentation (yet)
		SharedFolderTransferOwnershipDetails json.RawMessage `json:"shared_folder_transfer_ownership_details,omitempty"`
		// SharedFolderUnmountDetails : has no documentation (yet)
		SharedFolderUnmountDetails json.RawMessage `json:"shared_folder_unmount_details,omitempty"`
		// SharedLinkAddExpiryDetails : has no documentation (yet)
		SharedLinkAddExpiryDetails json.RawMessage `json:"shared_link_add_expiry_details,omitempty"`
		// SharedLinkChangeExpiryDetails : has no documentation (yet)
		SharedLinkChangeExpiryDetails json.RawMessage `json:"shared_link_change_expiry_details,omitempty"`
		// SharedLinkChangeVisibilityDetails : has no documentation (yet)
		SharedLinkChangeVisibilityDetails json.RawMessage `json:"shared_link_change_visibility_details,omitempty"`
		// SharedLinkCopyDetails : has no documentation (yet)
		SharedLinkCopyDetails json.RawMessage `json:"shared_link_copy_details,omitempty"`
		// SharedLinkCreateDetails : has no documentation (yet)
		SharedLinkCreateDetails json.RawMessage `json:"shared_link_create_details,omitempty"`
		// SharedLinkDisableDetails : has no documentation (yet)
		SharedLinkDisableDetails json.RawMessage `json:"shared_link_disable_details,omitempty"`
		// SharedLinkDownloadDetails : has no documentation (yet)
		SharedLinkDownloadDetails json.RawMessage `json:"shared_link_download_details,omitempty"`
		// SharedLinkRemoveExpiryDetails : has no documentation (yet)
		SharedLinkRemoveExpiryDetails json.RawMessage `json:"shared_link_remove_expiry_details,omitempty"`
		// SharedLinkShareDetails : has no documentation (yet)
		SharedLinkShareDetails json.RawMessage `json:"shared_link_share_details,omitempty"`
		// SharedLinkViewDetails : has no documentation (yet)
		SharedLinkViewDetails json.RawMessage `json:"shared_link_view_details,omitempty"`
		// SharedNoteOpenedDetails : has no documentation (yet)
		SharedNoteOpenedDetails json.RawMessage `json:"shared_note_opened_details,omitempty"`
		// ShmodelGroupShareDetails : has no documentation (yet)
		ShmodelGroupShareDetails json.RawMessage `json:"shmodel_group_share_details,omitempty"`
		// ShowcaseAccessGrantedDetails : has no documentation (yet)
		ShowcaseAccessGrantedDetails json.RawMessage `json:"showcase_access_granted_details,omitempty"`
		// ShowcaseAddMemberDetails : has no documentation (yet)
		ShowcaseAddMemberDetails json.RawMessage `json:"showcase_add_member_details,omitempty"`
		// ShowcaseArchivedDetails : has no documentation (yet)
		ShowcaseArchivedDetails json.RawMessage `json:"showcase_archived_details,omitempty"`
		// ShowcaseCreatedDetails : has no documentation (yet)
		ShowcaseCreatedDetails json.RawMessage `json:"showcase_created_details,omitempty"`
		// ShowcaseDeleteCommentDetails : has no documentation (yet)
		ShowcaseDeleteCommentDetails json.RawMessage `json:"showcase_delete_comment_details,omitempty"`
		// ShowcaseEditedDetails : has no documentation (yet)
		ShowcaseEditedDetails json.RawMessage `json:"showcase_edited_details,omitempty"`
		// ShowcaseEditCommentDetails : has no documentation (yet)
		ShowcaseEditCommentDetails json.RawMessage `json:"showcase_edit_comment_details,omitempty"`
		// ShowcaseFileAddedDetails : has no documentation (yet)
		ShowcaseFileAddedDetails json.RawMessage `json:"showcase_file_added_details,omitempty"`
		// ShowcaseFileDownloadDetails : has no documentation (yet)
		ShowcaseFileDownloadDetails json.RawMessage `json:"showcase_file_download_details,omitempty"`
		// ShowcaseFileRemovedDetails : has no documentation (yet)
		ShowcaseFileRemovedDetails json.RawMessage `json:"showcase_file_removed_details,omitempty"`
		// ShowcaseFileViewDetails : has no documentation (yet)
		ShowcaseFileViewDetails json.RawMessage `json:"showcase_file_view_details,omitempty"`
		// ShowcasePermanentlyDeletedDetails : has no documentation (yet)
		ShowcasePermanentlyDeletedDetails json.RawMessage `json:"showcase_permanently_deleted_details,omitempty"`
		// ShowcasePostCommentDetails : has no documentation (yet)
		ShowcasePostCommentDetails json.RawMessage `json:"showcase_post_comment_details,omitempty"`
		// ShowcaseRemoveMemberDetails : has no documentation (yet)
		ShowcaseRemoveMemberDetails json.RawMessage `json:"showcase_remove_member_details,omitempty"`
		// ShowcaseRenamedDetails : has no documentation (yet)
		ShowcaseRenamedDetails json.RawMessage `json:"showcase_renamed_details,omitempty"`
		// ShowcaseRequestAccessDetails : has no documentation (yet)
		ShowcaseRequestAccessDetails json.RawMessage `json:"showcase_request_access_details,omitempty"`
		// ShowcaseResolveCommentDetails : has no documentation (yet)
		ShowcaseResolveCommentDetails json.RawMessage `json:"showcase_resolve_comment_details,omitempty"`
		// ShowcaseRestoredDetails : has no documentation (yet)
		ShowcaseRestoredDetails json.RawMessage `json:"showcase_restored_details,omitempty"`
		// ShowcaseTrashedDetails : has no documentation (yet)
		ShowcaseTrashedDetails json.RawMessage `json:"showcase_trashed_details,omitempty"`
		// ShowcaseTrashedDeprecatedDetails : has no documentation (yet)
		ShowcaseTrashedDeprecatedDetails json.RawMessage `json:"showcase_trashed_deprecated_details,omitempty"`
		// ShowcaseUnresolveCommentDetails : has no documentation (yet)
		ShowcaseUnresolveCommentDetails json.RawMessage `json:"showcase_unresolve_comment_details,omitempty"`
		// ShowcaseUntrashedDetails : has no documentation (yet)
		ShowcaseUntrashedDetails json.RawMessage `json:"showcase_untrashed_details,omitempty"`
		// ShowcaseUntrashedDeprecatedDetails : has no documentation (yet)
		ShowcaseUntrashedDeprecatedDetails json.RawMessage `json:"showcase_untrashed_deprecated_details,omitempty"`
		// ShowcaseViewDetails : has no documentation (yet)
		ShowcaseViewDetails json.RawMessage `json:"showcase_view_details,omitempty"`
		// SsoAddCertDetails : has no documentation (yet)
		SsoAddCertDetails json.RawMessage `json:"sso_add_cert_details,omitempty"`
		// SsoAddLoginUrlDetails : has no documentation (yet)
		SsoAddLoginUrlDetails json.RawMessage `json:"sso_add_login_url_details,omitempty"`
		// SsoAddLogoutUrlDetails : has no documentation (yet)
		SsoAddLogoutUrlDetails json.RawMessage `json:"sso_add_logout_url_details,omitempty"`
		// SsoChangeCertDetails : has no documentation (yet)
		SsoChangeCertDetails json.RawMessage `json:"sso_change_cert_details,omitempty"`
		// SsoChangeLoginUrlDetails : has no documentation (yet)
		SsoChangeLoginUrlDetails json.RawMessage `json:"sso_change_login_url_details,omitempty"`
		// SsoChangeLogoutUrlDetails : has no documentation (yet)
		SsoChangeLogoutUrlDetails json.RawMessage `json:"sso_change_logout_url_details,omitempty"`
		// SsoChangeSamlIdentityModeDetails : has no documentation (yet)
		SsoChangeSamlIdentityModeDetails json.RawMessage `json:"sso_change_saml_identity_mode_details,omitempty"`
		// SsoRemoveCertDetails : has no documentation (yet)
		SsoRemoveCertDetails json.RawMessage `json:"sso_remove_cert_details,omitempty"`
		// SsoRemoveLoginUrlDetails : has no documentation (yet)
		SsoRemoveLoginUrlDetails json.RawMessage `json:"sso_remove_login_url_details,omitempty"`
		// SsoRemoveLogoutUrlDetails : has no documentation (yet)
		SsoRemoveLogoutUrlDetails json.RawMessage `json:"sso_remove_logout_url_details,omitempty"`
		// TeamFolderChangeStatusDetails : has no documentation (yet)
		TeamFolderChangeStatusDetails json.RawMessage `json:"team_folder_change_status_details,omitempty"`
		// TeamFolderCreateDetails : has no documentation (yet)
		TeamFolderCreateDetails json.RawMessage `json:"team_folder_create_details,omitempty"`
		// TeamFolderDowngradeDetails : has no documentation (yet)
		TeamFolderDowngradeDetails json.RawMessage `json:"team_folder_downgrade_details,omitempty"`
		// TeamFolderPermanentlyDeleteDetails : has no documentation (yet)
		TeamFolderPermanentlyDeleteDetails json.RawMessage `json:"team_folder_permanently_delete_details,omitempty"`
		// TeamFolderRenameDetails : has no documentation (yet)
		TeamFolderRenameDetails json.RawMessage `json:"team_folder_rename_details,omitempty"`
		// TeamSelectiveSyncSettingsChangedDetails : has no documentation (yet)
		TeamSelectiveSyncSettingsChangedDetails json.RawMessage `json:"team_selective_sync_settings_changed_details,omitempty"`
		// AccountCaptureChangePolicyDetails : has no documentation (yet)
		AccountCaptureChangePolicyDetails json.RawMessage `json:"account_capture_change_policy_details,omitempty"`
		// AllowDownloadDisabledDetails : has no documentation (yet)
		AllowDownloadDisabledDetails json.RawMessage `json:"allow_download_disabled_details,omitempty"`
		// AllowDownloadEnabledDetails : has no documentation (yet)
		AllowDownloadEnabledDetails json.RawMessage `json:"allow_download_enabled_details,omitempty"`
		// DataPlacementRestrictionChangePolicyDetails : has no documentation
		// (yet)
		DataPlacementRestrictionChangePolicyDetails json.RawMessage `json:"data_placement_restriction_change_policy_details,omitempty"`
		// DataPlacementRestrictionSatisfyPolicyDetails : has no documentation
		// (yet)
		DataPlacementRestrictionSatisfyPolicyDetails json.RawMessage `json:"data_placement_restriction_satisfy_policy_details,omitempty"`
		// DeviceApprovalsChangeDesktopPolicyDetails : has no documentation
		// (yet)
		DeviceApprovalsChangeDesktopPolicyDetails json.RawMessage `json:"device_approvals_change_desktop_policy_details,omitempty"`
		// DeviceApprovalsChangeMobilePolicyDetails : has no documentation (yet)
		DeviceApprovalsChangeMobilePolicyDetails json.RawMessage `json:"device_approvals_change_mobile_policy_details,omitempty"`
		// DeviceApprovalsChangeOverageActionDetails : has no documentation
		// (yet)
		DeviceApprovalsChangeOverageActionDetails json.RawMessage `json:"device_approvals_change_overage_action_details,omitempty"`
		// DeviceApprovalsChangeUnlinkActionDetails : has no documentation (yet)
		DeviceApprovalsChangeUnlinkActionDetails json.RawMessage `json:"device_approvals_change_unlink_action_details,omitempty"`
		// DirectoryRestrictionsAddMembersDetails : has no documentation (yet)
		DirectoryRestrictionsAddMembersDetails json.RawMessage `json:"directory_restrictions_add_members_details,omitempty"`
		// DirectoryRestrictionsRemoveMembersDetails : has no documentation
		// (yet)
		DirectoryRestrictionsRemoveMembersDetails json.RawMessage `json:"directory_restrictions_remove_members_details,omitempty"`
		// EmmAddExceptionDetails : has no documentation (yet)
		EmmAddExceptionDetails json.RawMessage `json:"emm_add_exception_details,omitempty"`
		// EmmChangePolicyDetails : has no documentation (yet)
		EmmChangePolicyDetails json.RawMessage `json:"emm_change_policy_details,omitempty"`
		// EmmRemoveExceptionDetails : has no documentation (yet)
		EmmRemoveExceptionDetails json.RawMessage `json:"emm_remove_exception_details,omitempty"`
		// ExtendedVersionHistoryChangePolicyDetails : has no documentation
		// (yet)
		ExtendedVersionHistoryChangePolicyDetails json.RawMessage `json:"extended_version_history_change_policy_details,omitempty"`
		// FileCommentsChangePolicyDetails : has no documentation (yet)
		FileCommentsChangePolicyDetails json.RawMessage `json:"file_comments_change_policy_details,omitempty"`
		// FileRequestsChangePolicyDetails : has no documentation (yet)
		FileRequestsChangePolicyDetails json.RawMessage `json:"file_requests_change_policy_details,omitempty"`
		// FileRequestsEmailsEnabledDetails : has no documentation (yet)
		FileRequestsEmailsEnabledDetails json.RawMessage `json:"file_requests_emails_enabled_details,omitempty"`
		// FileRequestsEmailsRestrictedToTeamOnlyDetails : has no documentation
		// (yet)
		FileRequestsEmailsRestrictedToTeamOnlyDetails json.RawMessage `json:"file_requests_emails_restricted_to_team_only_details,omitempty"`
		// GoogleSsoChangePolicyDetails : has no documentation (yet)
		GoogleSsoChangePolicyDetails json.RawMessage `json:"google_sso_change_policy_details,omitempty"`
		// GroupUserManagementChangePolicyDetails : has no documentation (yet)
		GroupUserManagementChangePolicyDetails json.RawMessage `json:"group_user_management_change_policy_details,omitempty"`
		// MemberRequestsChangePolicyDetails : has no documentation (yet)
		MemberRequestsChangePolicyDetails json.RawMessage `json:"member_requests_change_policy_details,omitempty"`
		// MemberSpaceLimitsAddExceptionDetails : has no documentation (yet)
		MemberSpaceLimitsAddExceptionDetails json.RawMessage `json:"member_space_limits_add_exception_details,omitempty"`
		// MemberSpaceLimitsChangeCapsTypePolicyDetails : has no documentation
		// (yet)
		MemberSpaceLimitsChangeCapsTypePolicyDetails json.RawMessage `json:"member_space_limits_change_caps_type_policy_details,omitempty"`
		// MemberSpaceLimitsChangePolicyDetails : has no documentation (yet)
		MemberSpaceLimitsChangePolicyDetails json.RawMessage `json:"member_space_limits_change_policy_details,omitempty"`
		// MemberSpaceLimitsRemoveExceptionDetails : has no documentation (yet)
		MemberSpaceLimitsRemoveExceptionDetails json.RawMessage `json:"member_space_limits_remove_exception_details,omitempty"`
		// MemberSuggestionsChangePolicyDetails : has no documentation (yet)
		MemberSuggestionsChangePolicyDetails json.RawMessage `json:"member_suggestions_change_policy_details,omitempty"`
		// MicrosoftOfficeAddinChangePolicyDetails : has no documentation (yet)
		MicrosoftOfficeAddinChangePolicyDetails json.RawMessage `json:"microsoft_office_addin_change_policy_details,omitempty"`
		// NetworkControlChangePolicyDetails : has no documentation (yet)
		NetworkControlChangePolicyDetails json.RawMessage `json:"network_control_change_policy_details,omitempty"`
		// PaperChangeDeploymentPolicyDetails : has no documentation (yet)
		PaperChangeDeploymentPolicyDetails json.RawMessage `json:"paper_change_deployment_policy_details,omitempty"`
		// PaperChangeMemberLinkPolicyDetails : has no documentation (yet)
		PaperChangeMemberLinkPolicyDetails json.RawMessage `json:"paper_change_member_link_policy_details,omitempty"`
		// PaperChangeMemberPolicyDetails : has no documentation (yet)
		PaperChangeMemberPolicyDetails json.RawMessage `json:"paper_change_member_policy_details,omitempty"`
		// PaperChangePolicyDetails : has no documentation (yet)
		PaperChangePolicyDetails json.RawMessage `json:"paper_change_policy_details,omitempty"`
		// PaperEnabledUsersGroupAdditionDetails : has no documentation (yet)
		PaperEnabledUsersGroupAdditionDetails json.RawMessage `json:"paper_enabled_users_group_addition_details,omitempty"`
		// PaperEnabledUsersGroupRemovalDetails : has no documentation (yet)
		PaperEnabledUsersGroupRemovalDetails json.RawMessage `json:"paper_enabled_users_group_removal_details,omitempty"`
		// PermanentDeleteChangePolicyDetails : has no documentation (yet)
		PermanentDeleteChangePolicyDetails json.RawMessage `json:"permanent_delete_change_policy_details,omitempty"`
		// SharingChangeFolderJoinPolicyDetails : has no documentation (yet)
		SharingChangeFolderJoinPolicyDetails json.RawMessage `json:"sharing_change_folder_join_policy_details,omitempty"`
		// SharingChangeLinkPolicyDetails : has no documentation (yet)
		SharingChangeLinkPolicyDetails json.RawMessage `json:"sharing_change_link_policy_details,omitempty"`
		// SharingChangeMemberPolicyDetails : has no documentation (yet)
		SharingChangeMemberPolicyDetails json.RawMessage `json:"sharing_change_member_policy_details,omitempty"`
		// ShowcaseChangeDownloadPolicyDetails : has no documentation (yet)
		ShowcaseChangeDownloadPolicyDetails json.RawMessage `json:"showcase_change_download_policy_details,omitempty"`
		// ShowcaseChangeEnabledPolicyDetails : has no documentation (yet)
		ShowcaseChangeEnabledPolicyDetails json.RawMessage `json:"showcase_change_enabled_policy_details,omitempty"`
		// ShowcaseChangeExternalSharingPolicyDetails : has no documentation
		// (yet)
		ShowcaseChangeExternalSharingPolicyDetails json.RawMessage `json:"showcase_change_external_sharing_policy_details,omitempty"`
		// SmartSyncChangePolicyDetails : has no documentation (yet)
		SmartSyncChangePolicyDetails json.RawMessage `json:"smart_sync_change_policy_details,omitempty"`
		// SmartSyncNotOptOutDetails : has no documentation (yet)
		SmartSyncNotOptOutDetails json.RawMessage `json:"smart_sync_not_opt_out_details,omitempty"`
		// SmartSyncOptOutDetails : has no documentation (yet)
		SmartSyncOptOutDetails json.RawMessage `json:"smart_sync_opt_out_details,omitempty"`
		// SsoChangePolicyDetails : has no documentation (yet)
		SsoChangePolicyDetails json.RawMessage `json:"sso_change_policy_details,omitempty"`
		// TfaChangePolicyDetails : has no documentation (yet)
		TfaChangePolicyDetails json.RawMessage `json:"tfa_change_policy_details,omitempty"`
		// TwoAccountChangePolicyDetails : has no documentation (yet)
		TwoAccountChangePolicyDetails json.RawMessage `json:"two_account_change_policy_details,omitempty"`
		// WebSessionsChangeFixedLengthPolicyDetails : has no documentation
		// (yet)
		WebSessionsChangeFixedLengthPolicyDetails json.RawMessage `json:"web_sessions_change_fixed_length_policy_details,omitempty"`
		// WebSessionsChangeIdleLengthPolicyDetails : has no documentation (yet)
		WebSessionsChangeIdleLengthPolicyDetails json.RawMessage `json:"web_sessions_change_idle_length_policy_details,omitempty"`
		// TeamMergeFromDetails : has no documentation (yet)
		TeamMergeFromDetails json.RawMessage `json:"team_merge_from_details,omitempty"`
		// TeamMergeToDetails : has no documentation (yet)
		TeamMergeToDetails json.RawMessage `json:"team_merge_to_details,omitempty"`
		// TeamProfileAddLogoDetails : has no documentation (yet)
		TeamProfileAddLogoDetails json.RawMessage `json:"team_profile_add_logo_details,omitempty"`
		// TeamProfileChangeDefaultLanguageDetails : has no documentation (yet)
		TeamProfileChangeDefaultLanguageDetails json.RawMessage `json:"team_profile_change_default_language_details,omitempty"`
		// TeamProfileChangeLogoDetails : has no documentation (yet)
		TeamProfileChangeLogoDetails json.RawMessage `json:"team_profile_change_logo_details,omitempty"`
		// TeamProfileChangeNameDetails : has no documentation (yet)
		TeamProfileChangeNameDetails json.RawMessage `json:"team_profile_change_name_details,omitempty"`
		// TeamProfileRemoveLogoDetails : has no documentation (yet)
		TeamProfileRemoveLogoDetails json.RawMessage `json:"team_profile_remove_logo_details,omitempty"`
		// TfaAddBackupPhoneDetails : has no documentation (yet)
		TfaAddBackupPhoneDetails json.RawMessage `json:"tfa_add_backup_phone_details,omitempty"`
		// TfaAddSecurityKeyDetails : has no documentation (yet)
		TfaAddSecurityKeyDetails json.RawMessage `json:"tfa_add_security_key_details,omitempty"`
		// TfaChangeBackupPhoneDetails : has no documentation (yet)
		TfaChangeBackupPhoneDetails json.RawMessage `json:"tfa_change_backup_phone_details,omitempty"`
		// TfaChangeStatusDetails : has no documentation (yet)
		TfaChangeStatusDetails json.RawMessage `json:"tfa_change_status_details,omitempty"`
		// TfaRemoveBackupPhoneDetails : has no documentation (yet)
		TfaRemoveBackupPhoneDetails json.RawMessage `json:"tfa_remove_backup_phone_details,omitempty"`
		// TfaRemoveSecurityKeyDetails : has no documentation (yet)
		TfaRemoveSecurityKeyDetails json.RawMessage `json:"tfa_remove_security_key_details,omitempty"`
		// TfaResetDetails : has no documentation (yet)
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
	case "file_add_comment_details":
		err = json.Unmarshal(body, &u.FileAddCommentDetails)

		if err != nil {
			return err
		}
	case "file_change_comment_subscription_details":
		err = json.Unmarshal(body, &u.FileChangeCommentSubscriptionDetails)

		if err != nil {
			return err
		}
	case "file_delete_comment_details":
		err = json.Unmarshal(body, &u.FileDeleteCommentDetails)

		if err != nil {
			return err
		}
	case "file_like_comment_details":
		err = json.Unmarshal(body, &u.FileLikeCommentDetails)

		if err != nil {
			return err
		}
	case "file_resolve_comment_details":
		err = json.Unmarshal(body, &u.FileResolveCommentDetails)

		if err != nil {
			return err
		}
	case "file_unlike_comment_details":
		err = json.Unmarshal(body, &u.FileUnlikeCommentDetails)

		if err != nil {
			return err
		}
	case "file_unresolve_comment_details":
		err = json.Unmarshal(body, &u.FileUnresolveCommentDetails)

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
	case "account_capture_notification_emails_sent_details":
		err = json.Unmarshal(body, &u.AccountCaptureNotificationEmailsSentDetails)

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
	case "file_request_change_details":
		err = json.Unmarshal(body, &u.FileRequestChangeDetails)

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
	case "emm_error_details":
		err = json.Unmarshal(body, &u.EmmErrorDetails)

		if err != nil {
			return err
		}
	case "login_fail_details":
		err = json.Unmarshal(body, &u.LoginFailDetails)

		if err != nil {
			return err
		}
	case "login_success_details":
		err = json.Unmarshal(body, &u.LoginSuccessDetails)

		if err != nil {
			return err
		}
	case "logout_details":
		err = json.Unmarshal(body, &u.LogoutDetails)

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
	case "sso_error_details":
		err = json.Unmarshal(body, &u.SsoErrorDetails)

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
	case "member_change_membership_type_details":
		err = json.Unmarshal(body, &u.MemberChangeMembershipTypeDetails)

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
	case "member_permanently_delete_account_contents_details":
		err = json.Unmarshal(body, &u.MemberPermanentlyDeleteAccountContentsDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_add_custom_quota_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsAddCustomQuotaDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_change_custom_quota_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeCustomQuotaDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_change_status_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeStatusDetails)

		if err != nil {
			return err
		}
	case "member_space_limits_remove_custom_quota_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsRemoveCustomQuotaDetails)

		if err != nil {
			return err
		}
	case "member_suggest_details":
		err = json.Unmarshal(body, &u.MemberSuggestDetails)

		if err != nil {
			return err
		}
	case "member_transfer_account_contents_details":
		err = json.Unmarshal(body, &u.MemberTransferAccountContentsDetails)

		if err != nil {
			return err
		}
	case "secondary_mails_policy_changed_details":
		err = json.Unmarshal(body, &u.SecondaryMailsPolicyChangedDetails)

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
	case "paper_doc_change_subscription_details":
		err = json.Unmarshal(body, &u.PaperDocChangeSubscriptionDetails)

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
	case "paper_doc_trashed_details":
		err = json.Unmarshal(body, &u.PaperDocTrashedDetails)

		if err != nil {
			return err
		}
	case "paper_doc_unresolve_comment_details":
		err = json.Unmarshal(body, &u.PaperDocUnresolveCommentDetails)

		if err != nil {
			return err
		}
	case "paper_doc_untrashed_details":
		err = json.Unmarshal(body, &u.PaperDocUntrashedDetails)

		if err != nil {
			return err
		}
	case "paper_doc_view_details":
		err = json.Unmarshal(body, &u.PaperDocViewDetails)

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
	case "paper_folder_change_subscription_details":
		err = json.Unmarshal(body, &u.PaperFolderChangeSubscriptionDetails)

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
	case "export_members_report_details":
		err = json.Unmarshal(body, &u.ExportMembersReportDetails)

		if err != nil {
			return err
		}
	case "paper_admin_export_start_details":
		err = json.Unmarshal(body, &u.PaperAdminExportStartDetails)

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
	case "sf_external_invite_warn_details":
		err = json.Unmarshal(body, &u.SfExternalInviteWarnDetails)

		if err != nil {
			return err
		}
	case "sf_fb_invite_details":
		err = json.Unmarshal(body, &u.SfFbInviteDetails)

		if err != nil {
			return err
		}
	case "sf_fb_invite_change_role_details":
		err = json.Unmarshal(body, &u.SfFbInviteChangeRoleDetails)

		if err != nil {
			return err
		}
	case "sf_fb_uninvite_details":
		err = json.Unmarshal(body, &u.SfFbUninviteDetails)

		if err != nil {
			return err
		}
	case "sf_invite_group_details":
		err = json.Unmarshal(body, &u.SfInviteGroupDetails)

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
	case "shared_content_remove_invitees_details":
		err = json.Unmarshal(body, &u.SharedContentRemoveInviteesDetails)

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
	case "shared_folder_change_link_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeLinkPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_members_inheritance_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeMembersInheritancePolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_members_management_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeMembersManagementPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_change_members_policy_details":
		err = json.Unmarshal(body, &u.SharedFolderChangeMembersPolicyDetails)

		if err != nil {
			return err
		}
	case "shared_folder_create_details":
		err = json.Unmarshal(body, &u.SharedFolderCreateDetails)

		if err != nil {
			return err
		}
	case "shared_folder_decline_invitation_details":
		err = json.Unmarshal(body, &u.SharedFolderDeclineInvitationDetails)

		if err != nil {
			return err
		}
	case "shared_folder_mount_details":
		err = json.Unmarshal(body, &u.SharedFolderMountDetails)

		if err != nil {
			return err
		}
	case "shared_folder_nest_details":
		err = json.Unmarshal(body, &u.SharedFolderNestDetails)

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
	case "shared_link_add_expiry_details":
		err = json.Unmarshal(body, &u.SharedLinkAddExpiryDetails)

		if err != nil {
			return err
		}
	case "shared_link_change_expiry_details":
		err = json.Unmarshal(body, &u.SharedLinkChangeExpiryDetails)

		if err != nil {
			return err
		}
	case "shared_link_change_visibility_details":
		err = json.Unmarshal(body, &u.SharedLinkChangeVisibilityDetails)

		if err != nil {
			return err
		}
	case "shared_link_copy_details":
		err = json.Unmarshal(body, &u.SharedLinkCopyDetails)

		if err != nil {
			return err
		}
	case "shared_link_create_details":
		err = json.Unmarshal(body, &u.SharedLinkCreateDetails)

		if err != nil {
			return err
		}
	case "shared_link_disable_details":
		err = json.Unmarshal(body, &u.SharedLinkDisableDetails)

		if err != nil {
			return err
		}
	case "shared_link_download_details":
		err = json.Unmarshal(body, &u.SharedLinkDownloadDetails)

		if err != nil {
			return err
		}
	case "shared_link_remove_expiry_details":
		err = json.Unmarshal(body, &u.SharedLinkRemoveExpiryDetails)

		if err != nil {
			return err
		}
	case "shared_link_share_details":
		err = json.Unmarshal(body, &u.SharedLinkShareDetails)

		if err != nil {
			return err
		}
	case "shared_link_view_details":
		err = json.Unmarshal(body, &u.SharedLinkViewDetails)

		if err != nil {
			return err
		}
	case "shared_note_opened_details":
		err = json.Unmarshal(body, &u.SharedNoteOpenedDetails)

		if err != nil {
			return err
		}
	case "shmodel_group_share_details":
		err = json.Unmarshal(body, &u.ShmodelGroupShareDetails)

		if err != nil {
			return err
		}
	case "showcase_access_granted_details":
		err = json.Unmarshal(body, &u.ShowcaseAccessGrantedDetails)

		if err != nil {
			return err
		}
	case "showcase_add_member_details":
		err = json.Unmarshal(body, &u.ShowcaseAddMemberDetails)

		if err != nil {
			return err
		}
	case "showcase_archived_details":
		err = json.Unmarshal(body, &u.ShowcaseArchivedDetails)

		if err != nil {
			return err
		}
	case "showcase_created_details":
		err = json.Unmarshal(body, &u.ShowcaseCreatedDetails)

		if err != nil {
			return err
		}
	case "showcase_delete_comment_details":
		err = json.Unmarshal(body, &u.ShowcaseDeleteCommentDetails)

		if err != nil {
			return err
		}
	case "showcase_edited_details":
		err = json.Unmarshal(body, &u.ShowcaseEditedDetails)

		if err != nil {
			return err
		}
	case "showcase_edit_comment_details":
		err = json.Unmarshal(body, &u.ShowcaseEditCommentDetails)

		if err != nil {
			return err
		}
	case "showcase_file_added_details":
		err = json.Unmarshal(body, &u.ShowcaseFileAddedDetails)

		if err != nil {
			return err
		}
	case "showcase_file_download_details":
		err = json.Unmarshal(body, &u.ShowcaseFileDownloadDetails)

		if err != nil {
			return err
		}
	case "showcase_file_removed_details":
		err = json.Unmarshal(body, &u.ShowcaseFileRemovedDetails)

		if err != nil {
			return err
		}
	case "showcase_file_view_details":
		err = json.Unmarshal(body, &u.ShowcaseFileViewDetails)

		if err != nil {
			return err
		}
	case "showcase_permanently_deleted_details":
		err = json.Unmarshal(body, &u.ShowcasePermanentlyDeletedDetails)

		if err != nil {
			return err
		}
	case "showcase_post_comment_details":
		err = json.Unmarshal(body, &u.ShowcasePostCommentDetails)

		if err != nil {
			return err
		}
	case "showcase_remove_member_details":
		err = json.Unmarshal(body, &u.ShowcaseRemoveMemberDetails)

		if err != nil {
			return err
		}
	case "showcase_renamed_details":
		err = json.Unmarshal(body, &u.ShowcaseRenamedDetails)

		if err != nil {
			return err
		}
	case "showcase_request_access_details":
		err = json.Unmarshal(body, &u.ShowcaseRequestAccessDetails)

		if err != nil {
			return err
		}
	case "showcase_resolve_comment_details":
		err = json.Unmarshal(body, &u.ShowcaseResolveCommentDetails)

		if err != nil {
			return err
		}
	case "showcase_restored_details":
		err = json.Unmarshal(body, &u.ShowcaseRestoredDetails)

		if err != nil {
			return err
		}
	case "showcase_trashed_details":
		err = json.Unmarshal(body, &u.ShowcaseTrashedDetails)

		if err != nil {
			return err
		}
	case "showcase_trashed_deprecated_details":
		err = json.Unmarshal(body, &u.ShowcaseTrashedDeprecatedDetails)

		if err != nil {
			return err
		}
	case "showcase_unresolve_comment_details":
		err = json.Unmarshal(body, &u.ShowcaseUnresolveCommentDetails)

		if err != nil {
			return err
		}
	case "showcase_untrashed_details":
		err = json.Unmarshal(body, &u.ShowcaseUntrashedDetails)

		if err != nil {
			return err
		}
	case "showcase_untrashed_deprecated_details":
		err = json.Unmarshal(body, &u.ShowcaseUntrashedDeprecatedDetails)

		if err != nil {
			return err
		}
	case "showcase_view_details":
		err = json.Unmarshal(body, &u.ShowcaseViewDetails)

		if err != nil {
			return err
		}
	case "sso_add_cert_details":
		err = json.Unmarshal(body, &u.SsoAddCertDetails)

		if err != nil {
			return err
		}
	case "sso_add_login_url_details":
		err = json.Unmarshal(body, &u.SsoAddLoginUrlDetails)

		if err != nil {
			return err
		}
	case "sso_add_logout_url_details":
		err = json.Unmarshal(body, &u.SsoAddLogoutUrlDetails)

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
	case "sso_remove_cert_details":
		err = json.Unmarshal(body, &u.SsoRemoveCertDetails)

		if err != nil {
			return err
		}
	case "sso_remove_login_url_details":
		err = json.Unmarshal(body, &u.SsoRemoveLoginUrlDetails)

		if err != nil {
			return err
		}
	case "sso_remove_logout_url_details":
		err = json.Unmarshal(body, &u.SsoRemoveLogoutUrlDetails)

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
	case "team_selective_sync_settings_changed_details":
		err = json.Unmarshal(body, &u.TeamSelectiveSyncSettingsChangedDetails)

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
	case "directory_restrictions_add_members_details":
		err = json.Unmarshal(body, &u.DirectoryRestrictionsAddMembersDetails)

		if err != nil {
			return err
		}
	case "directory_restrictions_remove_members_details":
		err = json.Unmarshal(body, &u.DirectoryRestrictionsRemoveMembersDetails)

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
	case "member_space_limits_change_caps_type_policy_details":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeCapsTypePolicyDetails)

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
	case "paper_change_member_link_policy_details":
		err = json.Unmarshal(body, &u.PaperChangeMemberLinkPolicyDetails)

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
	case "showcase_change_download_policy_details":
		err = json.Unmarshal(body, &u.ShowcaseChangeDownloadPolicyDetails)

		if err != nil {
			return err
		}
	case "showcase_change_enabled_policy_details":
		err = json.Unmarshal(body, &u.ShowcaseChangeEnabledPolicyDetails)

		if err != nil {
			return err
		}
	case "showcase_change_external_sharing_policy_details":
		err = json.Unmarshal(body, &u.ShowcaseChangeExternalSharingPolicyDetails)

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
	case "team_profile_add_logo_details":
		err = json.Unmarshal(body, &u.TeamProfileAddLogoDetails)

		if err != nil {
			return err
		}
	case "team_profile_change_default_language_details":
		err = json.Unmarshal(body, &u.TeamProfileChangeDefaultLanguageDetails)

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
	// AppLinkTeam : (apps) Linked app for team
	AppLinkTeam *AppLinkTeamType `json:"app_link_team,omitempty"`
	// AppLinkUser : (apps) Linked app for member
	AppLinkUser *AppLinkUserType `json:"app_link_user,omitempty"`
	// AppUnlinkTeam : (apps) Unlinked app for team
	AppUnlinkTeam *AppUnlinkTeamType `json:"app_unlink_team,omitempty"`
	// AppUnlinkUser : (apps) Unlinked app for member
	AppUnlinkUser *AppUnlinkUserType `json:"app_unlink_user,omitempty"`
	// FileAddComment : (comments) Added file comment
	FileAddComment *FileAddCommentType `json:"file_add_comment,omitempty"`
	// FileChangeCommentSubscription : (comments) Subscribed to or unsubscribed
	// from comment notifications for file
	FileChangeCommentSubscription *FileChangeCommentSubscriptionType `json:"file_change_comment_subscription,omitempty"`
	// FileDeleteComment : (comments) Deleted file comment
	FileDeleteComment *FileDeleteCommentType `json:"file_delete_comment,omitempty"`
	// FileLikeComment : (comments) Liked file comment (deprecated, no longer
	// logged)
	FileLikeComment *FileLikeCommentType `json:"file_like_comment,omitempty"`
	// FileResolveComment : (comments) Resolved file comment
	FileResolveComment *FileResolveCommentType `json:"file_resolve_comment,omitempty"`
	// FileUnlikeComment : (comments) Unliked file comment (deprecated, no
	// longer logged)
	FileUnlikeComment *FileUnlikeCommentType `json:"file_unlike_comment,omitempty"`
	// FileUnresolveComment : (comments) Unresolved file comment
	FileUnresolveComment *FileUnresolveCommentType `json:"file_unresolve_comment,omitempty"`
	// DeviceChangeIpDesktop : (devices) Changed IP address associated with
	// active desktop session
	DeviceChangeIpDesktop *DeviceChangeIpDesktopType `json:"device_change_ip_desktop,omitempty"`
	// DeviceChangeIpMobile : (devices) Changed IP address associated with
	// active mobile session
	DeviceChangeIpMobile *DeviceChangeIpMobileType `json:"device_change_ip_mobile,omitempty"`
	// DeviceChangeIpWeb : (devices) Changed IP address associated with active
	// web session
	DeviceChangeIpWeb *DeviceChangeIpWebType `json:"device_change_ip_web,omitempty"`
	// DeviceDeleteOnUnlinkFail : (devices) Failed to delete all files from
	// unlinked device
	DeviceDeleteOnUnlinkFail *DeviceDeleteOnUnlinkFailType `json:"device_delete_on_unlink_fail,omitempty"`
	// DeviceDeleteOnUnlinkSuccess : (devices) Deleted all files from unlinked
	// device
	DeviceDeleteOnUnlinkSuccess *DeviceDeleteOnUnlinkSuccessType `json:"device_delete_on_unlink_success,omitempty"`
	// DeviceLinkFail : (devices) Failed to link device
	DeviceLinkFail *DeviceLinkFailType `json:"device_link_fail,omitempty"`
	// DeviceLinkSuccess : (devices) Linked device
	DeviceLinkSuccess *DeviceLinkSuccessType `json:"device_link_success,omitempty"`
	// DeviceManagementDisabled : (devices) Disabled device management
	// (deprecated, no longer logged)
	DeviceManagementDisabled *DeviceManagementDisabledType `json:"device_management_disabled,omitempty"`
	// DeviceManagementEnabled : (devices) Enabled device management
	// (deprecated, no longer logged)
	DeviceManagementEnabled *DeviceManagementEnabledType `json:"device_management_enabled,omitempty"`
	// DeviceUnlink : (devices) Disconnected device
	DeviceUnlink *DeviceUnlinkType `json:"device_unlink,omitempty"`
	// EmmRefreshAuthToken : (devices) Refreshed auth token used for setting up
	// enterprise mobility management
	EmmRefreshAuthToken *EmmRefreshAuthTokenType `json:"emm_refresh_auth_token,omitempty"`
	// AccountCaptureChangeAvailability : (domains) Granted/revoked option to
	// enable account capture on team domains
	AccountCaptureChangeAvailability *AccountCaptureChangeAvailabilityType `json:"account_capture_change_availability,omitempty"`
	// AccountCaptureMigrateAccount : (domains) Account-captured user migrated
	// account to team
	AccountCaptureMigrateAccount *AccountCaptureMigrateAccountType `json:"account_capture_migrate_account,omitempty"`
	// AccountCaptureNotificationEmailsSent : (domains) Sent proactive account
	// capture email to all unmanaged members
	AccountCaptureNotificationEmailsSent *AccountCaptureNotificationEmailsSentType `json:"account_capture_notification_emails_sent,omitempty"`
	// AccountCaptureRelinquishAccount : (domains) Account-captured user changed
	// account email to personal email
	AccountCaptureRelinquishAccount *AccountCaptureRelinquishAccountType `json:"account_capture_relinquish_account,omitempty"`
	// DisabledDomainInvites : (domains) Disabled domain invites (deprecated, no
	// longer logged)
	DisabledDomainInvites *DisabledDomainInvitesType `json:"disabled_domain_invites,omitempty"`
	// DomainInvitesApproveRequestToJoinTeam : (domains) Approved user's request
	// to join team
	DomainInvitesApproveRequestToJoinTeam *DomainInvitesApproveRequestToJoinTeamType `json:"domain_invites_approve_request_to_join_team,omitempty"`
	// DomainInvitesDeclineRequestToJoinTeam : (domains) Declined user's request
	// to join team
	DomainInvitesDeclineRequestToJoinTeam *DomainInvitesDeclineRequestToJoinTeamType `json:"domain_invites_decline_request_to_join_team,omitempty"`
	// DomainInvitesEmailExistingUsers : (domains) Sent domain invites to
	// existing domain accounts (deprecated, no longer logged)
	DomainInvitesEmailExistingUsers *DomainInvitesEmailExistingUsersType `json:"domain_invites_email_existing_users,omitempty"`
	// DomainInvitesRequestToJoinTeam : (domains) Requested to join team
	DomainInvitesRequestToJoinTeam *DomainInvitesRequestToJoinTeamType `json:"domain_invites_request_to_join_team,omitempty"`
	// DomainInvitesSetInviteNewUserPrefToNo : (domains) Disabled "Automatically
	// invite new users" (deprecated, no longer logged)
	DomainInvitesSetInviteNewUserPrefToNo *DomainInvitesSetInviteNewUserPrefToNoType `json:"domain_invites_set_invite_new_user_pref_to_no,omitempty"`
	// DomainInvitesSetInviteNewUserPrefToYes : (domains) Enabled "Automatically
	// invite new users" (deprecated, no longer logged)
	DomainInvitesSetInviteNewUserPrefToYes *DomainInvitesSetInviteNewUserPrefToYesType `json:"domain_invites_set_invite_new_user_pref_to_yes,omitempty"`
	// DomainVerificationAddDomainFail : (domains) Failed to verify team domain
	DomainVerificationAddDomainFail *DomainVerificationAddDomainFailType `json:"domain_verification_add_domain_fail,omitempty"`
	// DomainVerificationAddDomainSuccess : (domains) Verified team domain
	DomainVerificationAddDomainSuccess *DomainVerificationAddDomainSuccessType `json:"domain_verification_add_domain_success,omitempty"`
	// DomainVerificationRemoveDomain : (domains) Removed domain from list of
	// verified team domains
	DomainVerificationRemoveDomain *DomainVerificationRemoveDomainType `json:"domain_verification_remove_domain,omitempty"`
	// EnabledDomainInvites : (domains) Enabled domain invites (deprecated, no
	// longer logged)
	EnabledDomainInvites *EnabledDomainInvitesType `json:"enabled_domain_invites,omitempty"`
	// CreateFolder : (file_operations) Created folders (deprecated, no longer
	// logged)
	CreateFolder *CreateFolderType `json:"create_folder,omitempty"`
	// FileAdd : (file_operations) Added files and/or folders
	FileAdd *FileAddType `json:"file_add,omitempty"`
	// FileCopy : (file_operations) Copied files and/or folders
	FileCopy *FileCopyType `json:"file_copy,omitempty"`
	// FileDelete : (file_operations) Deleted files and/or folders
	FileDelete *FileDeleteType `json:"file_delete,omitempty"`
	// FileDownload : (file_operations) Downloaded files and/or folders
	FileDownload *FileDownloadType `json:"file_download,omitempty"`
	// FileEdit : (file_operations) Edited files
	FileEdit *FileEditType `json:"file_edit,omitempty"`
	// FileGetCopyReference : (file_operations) Created copy reference to
	// file/folder
	FileGetCopyReference *FileGetCopyReferenceType `json:"file_get_copy_reference,omitempty"`
	// FileMove : (file_operations) Moved files and/or folders
	FileMove *FileMoveType `json:"file_move,omitempty"`
	// FilePermanentlyDelete : (file_operations) Permanently deleted files
	// and/or folders
	FilePermanentlyDelete *FilePermanentlyDeleteType `json:"file_permanently_delete,omitempty"`
	// FilePreview : (file_operations) Previewed files and/or folders
	FilePreview *FilePreviewType `json:"file_preview,omitempty"`
	// FileRename : (file_operations) Renamed files and/or folders
	FileRename *FileRenameType `json:"file_rename,omitempty"`
	// FileRestore : (file_operations) Restored deleted files and/or folders
	FileRestore *FileRestoreType `json:"file_restore,omitempty"`
	// FileRevert : (file_operations) Reverted files to previous version
	FileRevert *FileRevertType `json:"file_revert,omitempty"`
	// FileRollbackChanges : (file_operations) Rolled back file actions
	FileRollbackChanges *FileRollbackChangesType `json:"file_rollback_changes,omitempty"`
	// FileSaveCopyReference : (file_operations) Saved file/folder using copy
	// reference
	FileSaveCopyReference *FileSaveCopyReferenceType `json:"file_save_copy_reference,omitempty"`
	// FileRequestChange : (file_requests) Changed file request
	FileRequestChange *FileRequestChangeType `json:"file_request_change,omitempty"`
	// FileRequestClose : (file_requests) Closed file request
	FileRequestClose *FileRequestCloseType `json:"file_request_close,omitempty"`
	// FileRequestCreate : (file_requests) Created file request
	FileRequestCreate *FileRequestCreateType `json:"file_request_create,omitempty"`
	// FileRequestReceiveFile : (file_requests) Received files for file request
	FileRequestReceiveFile *FileRequestReceiveFileType `json:"file_request_receive_file,omitempty"`
	// GroupAddExternalId : (groups) Added external ID for group
	GroupAddExternalId *GroupAddExternalIdType `json:"group_add_external_id,omitempty"`
	// GroupAddMember : (groups) Added team members to group
	GroupAddMember *GroupAddMemberType `json:"group_add_member,omitempty"`
	// GroupChangeExternalId : (groups) Changed external ID for group
	GroupChangeExternalId *GroupChangeExternalIdType `json:"group_change_external_id,omitempty"`
	// GroupChangeManagementType : (groups) Changed group management type
	GroupChangeManagementType *GroupChangeManagementTypeType `json:"group_change_management_type,omitempty"`
	// GroupChangeMemberRole : (groups) Changed manager permissions of group
	// member
	GroupChangeMemberRole *GroupChangeMemberRoleType `json:"group_change_member_role,omitempty"`
	// GroupCreate : (groups) Created group
	GroupCreate *GroupCreateType `json:"group_create,omitempty"`
	// GroupDelete : (groups) Deleted group
	GroupDelete *GroupDeleteType `json:"group_delete,omitempty"`
	// GroupDescriptionUpdated : (groups) Updated group (deprecated, no longer
	// logged)
	GroupDescriptionUpdated *GroupDescriptionUpdatedType `json:"group_description_updated,omitempty"`
	// GroupJoinPolicyUpdated : (groups) Updated group join policy (deprecated,
	// no longer logged)
	GroupJoinPolicyUpdated *GroupJoinPolicyUpdatedType `json:"group_join_policy_updated,omitempty"`
	// GroupMoved : (groups) Moved group (deprecated, no longer logged)
	GroupMoved *GroupMovedType `json:"group_moved,omitempty"`
	// GroupRemoveExternalId : (groups) Removed external ID for group
	GroupRemoveExternalId *GroupRemoveExternalIdType `json:"group_remove_external_id,omitempty"`
	// GroupRemoveMember : (groups) Removed team members from group
	GroupRemoveMember *GroupRemoveMemberType `json:"group_remove_member,omitempty"`
	// GroupRename : (groups) Renamed group
	GroupRename *GroupRenameType `json:"group_rename,omitempty"`
	// EmmError : (logins) Failed to sign in via EMM (deprecated, replaced by
	// 'Failed to sign in')
	EmmError *EmmErrorType `json:"emm_error,omitempty"`
	// LoginFail : (logins) Failed to sign in
	LoginFail *LoginFailType `json:"login_fail,omitempty"`
	// LoginSuccess : (logins) Signed in
	LoginSuccess *LoginSuccessType `json:"login_success,omitempty"`
	// Logout : (logins) Signed out
	Logout *LogoutType `json:"logout,omitempty"`
	// ResellerSupportSessionEnd : (logins) Ended reseller support session
	ResellerSupportSessionEnd *ResellerSupportSessionEndType `json:"reseller_support_session_end,omitempty"`
	// ResellerSupportSessionStart : (logins) Started reseller support session
	ResellerSupportSessionStart *ResellerSupportSessionStartType `json:"reseller_support_session_start,omitempty"`
	// SignInAsSessionEnd : (logins) Ended admin sign-in-as session
	SignInAsSessionEnd *SignInAsSessionEndType `json:"sign_in_as_session_end,omitempty"`
	// SignInAsSessionStart : (logins) Started admin sign-in-as session
	SignInAsSessionStart *SignInAsSessionStartType `json:"sign_in_as_session_start,omitempty"`
	// SsoError : (logins) Failed to sign in via SSO (deprecated, replaced by
	// 'Failed to sign in')
	SsoError *SsoErrorType `json:"sso_error,omitempty"`
	// MemberAddName : (members) Added team member name
	MemberAddName *MemberAddNameType `json:"member_add_name,omitempty"`
	// MemberChangeAdminRole : (members) Changed team member admin role
	MemberChangeAdminRole *MemberChangeAdminRoleType `json:"member_change_admin_role,omitempty"`
	// MemberChangeEmail : (members) Changed team member email
	MemberChangeEmail *MemberChangeEmailType `json:"member_change_email,omitempty"`
	// MemberChangeMembershipType : (members) Changed membership type
	// (limited/full) of member (deprecated, no longer logged)
	MemberChangeMembershipType *MemberChangeMembershipTypeType `json:"member_change_membership_type,omitempty"`
	// MemberChangeName : (members) Changed team member name
	MemberChangeName *MemberChangeNameType `json:"member_change_name,omitempty"`
	// MemberChangeStatus : (members) Changed member status (invited, joined,
	// suspended, etc.)
	MemberChangeStatus *MemberChangeStatusType `json:"member_change_status,omitempty"`
	// MemberPermanentlyDeleteAccountContents : (members) Permanently deleted
	// contents of deleted team member account
	MemberPermanentlyDeleteAccountContents *MemberPermanentlyDeleteAccountContentsType `json:"member_permanently_delete_account_contents,omitempty"`
	// MemberSpaceLimitsAddCustomQuota : (members) Set custom member space limit
	MemberSpaceLimitsAddCustomQuota *MemberSpaceLimitsAddCustomQuotaType `json:"member_space_limits_add_custom_quota,omitempty"`
	// MemberSpaceLimitsChangeCustomQuota : (members) Changed custom member
	// space limit
	MemberSpaceLimitsChangeCustomQuota *MemberSpaceLimitsChangeCustomQuotaType `json:"member_space_limits_change_custom_quota,omitempty"`
	// MemberSpaceLimitsChangeStatus : (members) Changed space limit status
	MemberSpaceLimitsChangeStatus *MemberSpaceLimitsChangeStatusType `json:"member_space_limits_change_status,omitempty"`
	// MemberSpaceLimitsRemoveCustomQuota : (members) Removed custom member
	// space limit
	MemberSpaceLimitsRemoveCustomQuota *MemberSpaceLimitsRemoveCustomQuotaType `json:"member_space_limits_remove_custom_quota,omitempty"`
	// MemberSuggest : (members) Suggested person to add to team
	MemberSuggest *MemberSuggestType `json:"member_suggest,omitempty"`
	// MemberTransferAccountContents : (members) Transferred contents of deleted
	// member account to another member
	MemberTransferAccountContents *MemberTransferAccountContentsType `json:"member_transfer_account_contents,omitempty"`
	// SecondaryMailsPolicyChanged : (members) Secondary mails policy changed
	SecondaryMailsPolicyChanged *SecondaryMailsPolicyChangedType `json:"secondary_mails_policy_changed,omitempty"`
	// PaperContentAddMember : (paper) Added team member to Paper doc/folder
	PaperContentAddMember *PaperContentAddMemberType `json:"paper_content_add_member,omitempty"`
	// PaperContentAddToFolder : (paper) Added Paper doc/folder to folder
	PaperContentAddToFolder *PaperContentAddToFolderType `json:"paper_content_add_to_folder,omitempty"`
	// PaperContentArchive : (paper) Archived Paper doc/folder
	PaperContentArchive *PaperContentArchiveType `json:"paper_content_archive,omitempty"`
	// PaperContentCreate : (paper) Created Paper doc/folder
	PaperContentCreate *PaperContentCreateType `json:"paper_content_create,omitempty"`
	// PaperContentPermanentlyDelete : (paper) Permanently deleted Paper
	// doc/folder
	PaperContentPermanentlyDelete *PaperContentPermanentlyDeleteType `json:"paper_content_permanently_delete,omitempty"`
	// PaperContentRemoveFromFolder : (paper) Removed Paper doc/folder from
	// folder
	PaperContentRemoveFromFolder *PaperContentRemoveFromFolderType `json:"paper_content_remove_from_folder,omitempty"`
	// PaperContentRemoveMember : (paper) Removed team member from Paper
	// doc/folder
	PaperContentRemoveMember *PaperContentRemoveMemberType `json:"paper_content_remove_member,omitempty"`
	// PaperContentRename : (paper) Renamed Paper doc/folder
	PaperContentRename *PaperContentRenameType `json:"paper_content_rename,omitempty"`
	// PaperContentRestore : (paper) Restored archived Paper doc/folder
	PaperContentRestore *PaperContentRestoreType `json:"paper_content_restore,omitempty"`
	// PaperDocAddComment : (paper) Added Paper doc comment
	PaperDocAddComment *PaperDocAddCommentType `json:"paper_doc_add_comment,omitempty"`
	// PaperDocChangeMemberRole : (paper) Changed team member permissions for
	// Paper doc
	PaperDocChangeMemberRole *PaperDocChangeMemberRoleType `json:"paper_doc_change_member_role,omitempty"`
	// PaperDocChangeSharingPolicy : (paper) Changed sharing setting for Paper
	// doc
	PaperDocChangeSharingPolicy *PaperDocChangeSharingPolicyType `json:"paper_doc_change_sharing_policy,omitempty"`
	// PaperDocChangeSubscription : (paper) Followed/unfollowed Paper doc
	PaperDocChangeSubscription *PaperDocChangeSubscriptionType `json:"paper_doc_change_subscription,omitempty"`
	// PaperDocDeleted : (paper) Archived Paper doc (deprecated, no longer
	// logged)
	PaperDocDeleted *PaperDocDeletedType `json:"paper_doc_deleted,omitempty"`
	// PaperDocDeleteComment : (paper) Deleted Paper doc comment
	PaperDocDeleteComment *PaperDocDeleteCommentType `json:"paper_doc_delete_comment,omitempty"`
	// PaperDocDownload : (paper) Downloaded Paper doc in specific format
	PaperDocDownload *PaperDocDownloadType `json:"paper_doc_download,omitempty"`
	// PaperDocEdit : (paper) Edited Paper doc
	PaperDocEdit *PaperDocEditType `json:"paper_doc_edit,omitempty"`
	// PaperDocEditComment : (paper) Edited Paper doc comment
	PaperDocEditComment *PaperDocEditCommentType `json:"paper_doc_edit_comment,omitempty"`
	// PaperDocFollowed : (paper) Followed Paper doc (deprecated, replaced by
	// 'Followed/unfollowed Paper doc')
	PaperDocFollowed *PaperDocFollowedType `json:"paper_doc_followed,omitempty"`
	// PaperDocMention : (paper) Mentioned team member in Paper doc
	PaperDocMention *PaperDocMentionType `json:"paper_doc_mention,omitempty"`
	// PaperDocRequestAccess : (paper) Requested access to Paper doc
	PaperDocRequestAccess *PaperDocRequestAccessType `json:"paper_doc_request_access,omitempty"`
	// PaperDocResolveComment : (paper) Resolved Paper doc comment
	PaperDocResolveComment *PaperDocResolveCommentType `json:"paper_doc_resolve_comment,omitempty"`
	// PaperDocRevert : (paper) Restored Paper doc to previous version
	PaperDocRevert *PaperDocRevertType `json:"paper_doc_revert,omitempty"`
	// PaperDocSlackShare : (paper) Shared Paper doc via Slack
	PaperDocSlackShare *PaperDocSlackShareType `json:"paper_doc_slack_share,omitempty"`
	// PaperDocTeamInvite : (paper) Shared Paper doc with team member
	// (deprecated, no longer logged)
	PaperDocTeamInvite *PaperDocTeamInviteType `json:"paper_doc_team_invite,omitempty"`
	// PaperDocTrashed : (paper) Deleted Paper doc
	PaperDocTrashed *PaperDocTrashedType `json:"paper_doc_trashed,omitempty"`
	// PaperDocUnresolveComment : (paper) Unresolved Paper doc comment
	PaperDocUnresolveComment *PaperDocUnresolveCommentType `json:"paper_doc_unresolve_comment,omitempty"`
	// PaperDocUntrashed : (paper) Restored Paper doc
	PaperDocUntrashed *PaperDocUntrashedType `json:"paper_doc_untrashed,omitempty"`
	// PaperDocView : (paper) Viewed Paper doc
	PaperDocView *PaperDocViewType `json:"paper_doc_view,omitempty"`
	// PaperExternalViewAllow : (paper) Changed Paper external sharing setting
	// to anyone (deprecated, no longer logged)
	PaperExternalViewAllow *PaperExternalViewAllowType `json:"paper_external_view_allow,omitempty"`
	// PaperExternalViewDefaultTeam : (paper) Changed Paper external sharing
	// setting to default team (deprecated, no longer logged)
	PaperExternalViewDefaultTeam *PaperExternalViewDefaultTeamType `json:"paper_external_view_default_team,omitempty"`
	// PaperExternalViewForbid : (paper) Changed Paper external sharing setting
	// to team-only (deprecated, no longer logged)
	PaperExternalViewForbid *PaperExternalViewForbidType `json:"paper_external_view_forbid,omitempty"`
	// PaperFolderChangeSubscription : (paper) Followed/unfollowed Paper folder
	PaperFolderChangeSubscription *PaperFolderChangeSubscriptionType `json:"paper_folder_change_subscription,omitempty"`
	// PaperFolderDeleted : (paper) Archived Paper folder (deprecated, no longer
	// logged)
	PaperFolderDeleted *PaperFolderDeletedType `json:"paper_folder_deleted,omitempty"`
	// PaperFolderFollowed : (paper) Followed Paper folder (deprecated, replaced
	// by 'Followed/unfollowed Paper folder')
	PaperFolderFollowed *PaperFolderFollowedType `json:"paper_folder_followed,omitempty"`
	// PaperFolderTeamInvite : (paper) Shared Paper folder with member
	// (deprecated, no longer logged)
	PaperFolderTeamInvite *PaperFolderTeamInviteType `json:"paper_folder_team_invite,omitempty"`
	// PasswordChange : (passwords) Changed password
	PasswordChange *PasswordChangeType `json:"password_change,omitempty"`
	// PasswordReset : (passwords) Reset password
	PasswordReset *PasswordResetType `json:"password_reset,omitempty"`
	// PasswordResetAll : (passwords) Reset all team member passwords
	PasswordResetAll *PasswordResetAllType `json:"password_reset_all,omitempty"`
	// EmmCreateExceptionsReport : (reports) Created EMM-excluded users report
	EmmCreateExceptionsReport *EmmCreateExceptionsReportType `json:"emm_create_exceptions_report,omitempty"`
	// EmmCreateUsageReport : (reports) Created EMM mobile app usage report
	EmmCreateUsageReport *EmmCreateUsageReportType `json:"emm_create_usage_report,omitempty"`
	// ExportMembersReport : (reports) Created member data report
	ExportMembersReport *ExportMembersReportType `json:"export_members_report,omitempty"`
	// PaperAdminExportStart : (reports) Exported all team Paper docs
	PaperAdminExportStart *PaperAdminExportStartType `json:"paper_admin_export_start,omitempty"`
	// SmartSyncCreateAdminPrivilegeReport : (reports) Created Smart Sync
	// non-admin devices report
	SmartSyncCreateAdminPrivilegeReport *SmartSyncCreateAdminPrivilegeReportType `json:"smart_sync_create_admin_privilege_report,omitempty"`
	// TeamActivityCreateReport : (reports) Created team activity report
	TeamActivityCreateReport *TeamActivityCreateReportType `json:"team_activity_create_report,omitempty"`
	// CollectionShare : (sharing) Shared album
	CollectionShare *CollectionShareType `json:"collection_share,omitempty"`
	// NoteAclInviteOnly : (sharing) Changed Paper doc to invite-only
	// (deprecated, no longer logged)
	NoteAclInviteOnly *NoteAclInviteOnlyType `json:"note_acl_invite_only,omitempty"`
	// NoteAclLink : (sharing) Changed Paper doc to link-accessible (deprecated,
	// no longer logged)
	NoteAclLink *NoteAclLinkType `json:"note_acl_link,omitempty"`
	// NoteAclTeamLink : (sharing) Changed Paper doc to link-accessible for team
	// (deprecated, no longer logged)
	NoteAclTeamLink *NoteAclTeamLinkType `json:"note_acl_team_link,omitempty"`
	// NoteShared : (sharing) Shared Paper doc (deprecated, no longer logged)
	NoteShared *NoteSharedType `json:"note_shared,omitempty"`
	// NoteShareReceive : (sharing) Shared received Paper doc (deprecated, no
	// longer logged)
	NoteShareReceive *NoteShareReceiveType `json:"note_share_receive,omitempty"`
	// OpenNoteShared : (sharing) Opened shared Paper doc (deprecated, no longer
	// logged)
	OpenNoteShared *OpenNoteSharedType `json:"open_note_shared,omitempty"`
	// SfAddGroup : (sharing) Added team to shared folder (deprecated, no longer
	// logged)
	SfAddGroup *SfAddGroupType `json:"sf_add_group,omitempty"`
	// SfAllowNonMembersToViewSharedLinks : (sharing) Allowed non-collaborators
	// to view links to files in shared folder (deprecated, no longer logged)
	SfAllowNonMembersToViewSharedLinks *SfAllowNonMembersToViewSharedLinksType `json:"sf_allow_non_members_to_view_shared_links,omitempty"`
	// SfExternalInviteWarn : (sharing) Set team members to see warning before
	// sharing folders outside team (deprecated, no longer logged)
	SfExternalInviteWarn *SfExternalInviteWarnType `json:"sf_external_invite_warn,omitempty"`
	// SfFbInvite : (sharing) Invited Facebook users to shared folder
	// (deprecated, no longer logged)
	SfFbInvite *SfFbInviteType `json:"sf_fb_invite,omitempty"`
	// SfFbInviteChangeRole : (sharing) Changed Facebook user's role in shared
	// folder (deprecated, no longer logged)
	SfFbInviteChangeRole *SfFbInviteChangeRoleType `json:"sf_fb_invite_change_role,omitempty"`
	// SfFbUninvite : (sharing) Uninvited Facebook user from shared folder
	// (deprecated, no longer logged)
	SfFbUninvite *SfFbUninviteType `json:"sf_fb_uninvite,omitempty"`
	// SfInviteGroup : (sharing) Invited group to shared folder (deprecated, no
	// longer logged)
	SfInviteGroup *SfInviteGroupType `json:"sf_invite_group,omitempty"`
	// SfTeamGrantAccess : (sharing) Granted access to shared folder
	// (deprecated, no longer logged)
	SfTeamGrantAccess *SfTeamGrantAccessType `json:"sf_team_grant_access,omitempty"`
	// SfTeamInvite : (sharing) Invited team members to shared folder
	// (deprecated, replaced by 'Invited user to Dropbox and added them to
	// shared file/folder')
	SfTeamInvite *SfTeamInviteType `json:"sf_team_invite,omitempty"`
	// SfTeamInviteChangeRole : (sharing) Changed team member's role in shared
	// folder (deprecated, no longer logged)
	SfTeamInviteChangeRole *SfTeamInviteChangeRoleType `json:"sf_team_invite_change_role,omitempty"`
	// SfTeamJoin : (sharing) Joined team member's shared folder (deprecated, no
	// longer logged)
	SfTeamJoin *SfTeamJoinType `json:"sf_team_join,omitempty"`
	// SfTeamJoinFromOobLink : (sharing) Joined team member's shared folder from
	// link (deprecated, no longer logged)
	SfTeamJoinFromOobLink *SfTeamJoinFromOobLinkType `json:"sf_team_join_from_oob_link,omitempty"`
	// SfTeamUninvite : (sharing) Unshared folder with team member (deprecated,
	// replaced by 'Removed invitee from shared file/folder before invite was
	// accepted')
	SfTeamUninvite *SfTeamUninviteType `json:"sf_team_uninvite,omitempty"`
	// SharedContentAddInvitees : (sharing) Invited user to Dropbox and added
	// them to shared file/folder
	SharedContentAddInvitees *SharedContentAddInviteesType `json:"shared_content_add_invitees,omitempty"`
	// SharedContentAddLinkExpiry : (sharing) Added expiration date to link for
	// shared file/folder
	SharedContentAddLinkExpiry *SharedContentAddLinkExpiryType `json:"shared_content_add_link_expiry,omitempty"`
	// SharedContentAddLinkPassword : (sharing) Added password to link for
	// shared file/folder
	SharedContentAddLinkPassword *SharedContentAddLinkPasswordType `json:"shared_content_add_link_password,omitempty"`
	// SharedContentAddMember : (sharing) Added users and/or groups to shared
	// file/folder
	SharedContentAddMember *SharedContentAddMemberType `json:"shared_content_add_member,omitempty"`
	// SharedContentChangeDownloadsPolicy : (sharing) Changed whether members
	// can download shared file/folder
	SharedContentChangeDownloadsPolicy *SharedContentChangeDownloadsPolicyType `json:"shared_content_change_downloads_policy,omitempty"`
	// SharedContentChangeInviteeRole : (sharing) Changed access type of invitee
	// to shared file/folder before invite was accepted
	SharedContentChangeInviteeRole *SharedContentChangeInviteeRoleType `json:"shared_content_change_invitee_role,omitempty"`
	// SharedContentChangeLinkAudience : (sharing) Changed link audience of
	// shared file/folder
	SharedContentChangeLinkAudience *SharedContentChangeLinkAudienceType `json:"shared_content_change_link_audience,omitempty"`
	// SharedContentChangeLinkExpiry : (sharing) Changed link expiration of
	// shared file/folder
	SharedContentChangeLinkExpiry *SharedContentChangeLinkExpiryType `json:"shared_content_change_link_expiry,omitempty"`
	// SharedContentChangeLinkPassword : (sharing) Changed link password of
	// shared file/folder
	SharedContentChangeLinkPassword *SharedContentChangeLinkPasswordType `json:"shared_content_change_link_password,omitempty"`
	// SharedContentChangeMemberRole : (sharing) Changed access type of shared
	// file/folder member
	SharedContentChangeMemberRole *SharedContentChangeMemberRoleType `json:"shared_content_change_member_role,omitempty"`
	// SharedContentChangeViewerInfoPolicy : (sharing) Changed whether members
	// can see who viewed shared file/folder
	SharedContentChangeViewerInfoPolicy *SharedContentChangeViewerInfoPolicyType `json:"shared_content_change_viewer_info_policy,omitempty"`
	// SharedContentClaimInvitation : (sharing) Acquired membership of shared
	// file/folder by accepting invite
	SharedContentClaimInvitation *SharedContentClaimInvitationType `json:"shared_content_claim_invitation,omitempty"`
	// SharedContentCopy : (sharing) Copied shared file/folder to own Dropbox
	SharedContentCopy *SharedContentCopyType `json:"shared_content_copy,omitempty"`
	// SharedContentDownload : (sharing) Downloaded shared file/folder
	SharedContentDownload *SharedContentDownloadType `json:"shared_content_download,omitempty"`
	// SharedContentRelinquishMembership : (sharing) Left shared file/folder
	SharedContentRelinquishMembership *SharedContentRelinquishMembershipType `json:"shared_content_relinquish_membership,omitempty"`
	// SharedContentRemoveInvitees : (sharing) Removed invitee from shared
	// file/folder before invite was accepted
	SharedContentRemoveInvitees *SharedContentRemoveInviteesType `json:"shared_content_remove_invitees,omitempty"`
	// SharedContentRemoveLinkExpiry : (sharing) Removed link expiration date of
	// shared file/folder
	SharedContentRemoveLinkExpiry *SharedContentRemoveLinkExpiryType `json:"shared_content_remove_link_expiry,omitempty"`
	// SharedContentRemoveLinkPassword : (sharing) Removed link password of
	// shared file/folder
	SharedContentRemoveLinkPassword *SharedContentRemoveLinkPasswordType `json:"shared_content_remove_link_password,omitempty"`
	// SharedContentRemoveMember : (sharing) Removed user/group from shared
	// file/folder
	SharedContentRemoveMember *SharedContentRemoveMemberType `json:"shared_content_remove_member,omitempty"`
	// SharedContentRequestAccess : (sharing) Requested access to shared
	// file/folder
	SharedContentRequestAccess *SharedContentRequestAccessType `json:"shared_content_request_access,omitempty"`
	// SharedContentUnshare : (sharing) Unshared file/folder by clearing
	// membership and turning off link
	SharedContentUnshare *SharedContentUnshareType `json:"shared_content_unshare,omitempty"`
	// SharedContentView : (sharing) Previewed shared file/folder
	SharedContentView *SharedContentViewType `json:"shared_content_view,omitempty"`
	// SharedFolderChangeLinkPolicy : (sharing) Changed who can access shared
	// folder via link
	SharedFolderChangeLinkPolicy *SharedFolderChangeLinkPolicyType `json:"shared_folder_change_link_policy,omitempty"`
	// SharedFolderChangeMembersInheritancePolicy : (sharing) Changed whether
	// shared folder inherits members from parent folder
	SharedFolderChangeMembersInheritancePolicy *SharedFolderChangeMembersInheritancePolicyType `json:"shared_folder_change_members_inheritance_policy,omitempty"`
	// SharedFolderChangeMembersManagementPolicy : (sharing) Changed who can
	// add/remove members of shared folder
	SharedFolderChangeMembersManagementPolicy *SharedFolderChangeMembersManagementPolicyType `json:"shared_folder_change_members_management_policy,omitempty"`
	// SharedFolderChangeMembersPolicy : (sharing) Changed who can become member
	// of shared folder
	SharedFolderChangeMembersPolicy *SharedFolderChangeMembersPolicyType `json:"shared_folder_change_members_policy,omitempty"`
	// SharedFolderCreate : (sharing) Created shared folder
	SharedFolderCreate *SharedFolderCreateType `json:"shared_folder_create,omitempty"`
	// SharedFolderDeclineInvitation : (sharing) Declined team member's invite
	// to shared folder
	SharedFolderDeclineInvitation *SharedFolderDeclineInvitationType `json:"shared_folder_decline_invitation,omitempty"`
	// SharedFolderMount : (sharing) Added shared folder to own Dropbox
	SharedFolderMount *SharedFolderMountType `json:"shared_folder_mount,omitempty"`
	// SharedFolderNest : (sharing) Changed parent of shared folder
	SharedFolderNest *SharedFolderNestType `json:"shared_folder_nest,omitempty"`
	// SharedFolderTransferOwnership : (sharing) Transferred ownership of shared
	// folder to another member
	SharedFolderTransferOwnership *SharedFolderTransferOwnershipType `json:"shared_folder_transfer_ownership,omitempty"`
	// SharedFolderUnmount : (sharing) Deleted shared folder from Dropbox
	SharedFolderUnmount *SharedFolderUnmountType `json:"shared_folder_unmount,omitempty"`
	// SharedLinkAddExpiry : (sharing) Added shared link expiration date
	SharedLinkAddExpiry *SharedLinkAddExpiryType `json:"shared_link_add_expiry,omitempty"`
	// SharedLinkChangeExpiry : (sharing) Changed shared link expiration date
	SharedLinkChangeExpiry *SharedLinkChangeExpiryType `json:"shared_link_change_expiry,omitempty"`
	// SharedLinkChangeVisibility : (sharing) Changed visibility of shared link
	SharedLinkChangeVisibility *SharedLinkChangeVisibilityType `json:"shared_link_change_visibility,omitempty"`
	// SharedLinkCopy : (sharing) Added file/folder to Dropbox from shared link
	SharedLinkCopy *SharedLinkCopyType `json:"shared_link_copy,omitempty"`
	// SharedLinkCreate : (sharing) Created shared link
	SharedLinkCreate *SharedLinkCreateType `json:"shared_link_create,omitempty"`
	// SharedLinkDisable : (sharing) Removed shared link
	SharedLinkDisable *SharedLinkDisableType `json:"shared_link_disable,omitempty"`
	// SharedLinkDownload : (sharing) Downloaded file/folder from shared link
	SharedLinkDownload *SharedLinkDownloadType `json:"shared_link_download,omitempty"`
	// SharedLinkRemoveExpiry : (sharing) Removed shared link expiration date
	SharedLinkRemoveExpiry *SharedLinkRemoveExpiryType `json:"shared_link_remove_expiry,omitempty"`
	// SharedLinkShare : (sharing) Added members as audience of shared link
	SharedLinkShare *SharedLinkShareType `json:"shared_link_share,omitempty"`
	// SharedLinkView : (sharing) Opened shared link
	SharedLinkView *SharedLinkViewType `json:"shared_link_view,omitempty"`
	// SharedNoteOpened : (sharing) Opened shared Paper doc (deprecated, no
	// longer logged)
	SharedNoteOpened *SharedNoteOpenedType `json:"shared_note_opened,omitempty"`
	// ShmodelGroupShare : (sharing) Shared link with group (deprecated, no
	// longer logged)
	ShmodelGroupShare *ShmodelGroupShareType `json:"shmodel_group_share,omitempty"`
	// ShowcaseAccessGranted : (showcase) Granted access to showcase
	ShowcaseAccessGranted *ShowcaseAccessGrantedType `json:"showcase_access_granted,omitempty"`
	// ShowcaseAddMember : (showcase) Added member to showcase
	ShowcaseAddMember *ShowcaseAddMemberType `json:"showcase_add_member,omitempty"`
	// ShowcaseArchived : (showcase) Archived showcase
	ShowcaseArchived *ShowcaseArchivedType `json:"showcase_archived,omitempty"`
	// ShowcaseCreated : (showcase) Created showcase
	ShowcaseCreated *ShowcaseCreatedType `json:"showcase_created,omitempty"`
	// ShowcaseDeleteComment : (showcase) Deleted showcase comment
	ShowcaseDeleteComment *ShowcaseDeleteCommentType `json:"showcase_delete_comment,omitempty"`
	// ShowcaseEdited : (showcase) Edited showcase
	ShowcaseEdited *ShowcaseEditedType `json:"showcase_edited,omitempty"`
	// ShowcaseEditComment : (showcase) Edited showcase comment
	ShowcaseEditComment *ShowcaseEditCommentType `json:"showcase_edit_comment,omitempty"`
	// ShowcaseFileAdded : (showcase) Added file to showcase
	ShowcaseFileAdded *ShowcaseFileAddedType `json:"showcase_file_added,omitempty"`
	// ShowcaseFileDownload : (showcase) Downloaded file from showcase
	ShowcaseFileDownload *ShowcaseFileDownloadType `json:"showcase_file_download,omitempty"`
	// ShowcaseFileRemoved : (showcase) Removed file from showcase
	ShowcaseFileRemoved *ShowcaseFileRemovedType `json:"showcase_file_removed,omitempty"`
	// ShowcaseFileView : (showcase) Viewed file in showcase
	ShowcaseFileView *ShowcaseFileViewType `json:"showcase_file_view,omitempty"`
	// ShowcasePermanentlyDeleted : (showcase) Permanently deleted showcase
	ShowcasePermanentlyDeleted *ShowcasePermanentlyDeletedType `json:"showcase_permanently_deleted,omitempty"`
	// ShowcasePostComment : (showcase) Added showcase comment
	ShowcasePostComment *ShowcasePostCommentType `json:"showcase_post_comment,omitempty"`
	// ShowcaseRemoveMember : (showcase) Removed member from showcase
	ShowcaseRemoveMember *ShowcaseRemoveMemberType `json:"showcase_remove_member,omitempty"`
	// ShowcaseRenamed : (showcase) Renamed showcase
	ShowcaseRenamed *ShowcaseRenamedType `json:"showcase_renamed,omitempty"`
	// ShowcaseRequestAccess : (showcase) Requested access to showcase
	ShowcaseRequestAccess *ShowcaseRequestAccessType `json:"showcase_request_access,omitempty"`
	// ShowcaseResolveComment : (showcase) Resolved showcase comment
	ShowcaseResolveComment *ShowcaseResolveCommentType `json:"showcase_resolve_comment,omitempty"`
	// ShowcaseRestored : (showcase) Unarchived showcase
	ShowcaseRestored *ShowcaseRestoredType `json:"showcase_restored,omitempty"`
	// ShowcaseTrashed : (showcase) Deleted showcase
	ShowcaseTrashed *ShowcaseTrashedType `json:"showcase_trashed,omitempty"`
	// ShowcaseTrashedDeprecated : (showcase) Deleted showcase (old version)
	// (deprecated, replaced by 'Deleted showcase')
	ShowcaseTrashedDeprecated *ShowcaseTrashedDeprecatedType `json:"showcase_trashed_deprecated,omitempty"`
	// ShowcaseUnresolveComment : (showcase) Unresolved showcase comment
	ShowcaseUnresolveComment *ShowcaseUnresolveCommentType `json:"showcase_unresolve_comment,omitempty"`
	// ShowcaseUntrashed : (showcase) Restored showcase
	ShowcaseUntrashed *ShowcaseUntrashedType `json:"showcase_untrashed,omitempty"`
	// ShowcaseUntrashedDeprecated : (showcase) Restored showcase (old version)
	// (deprecated, replaced by 'Restored showcase')
	ShowcaseUntrashedDeprecated *ShowcaseUntrashedDeprecatedType `json:"showcase_untrashed_deprecated,omitempty"`
	// ShowcaseView : (showcase) Viewed showcase
	ShowcaseView *ShowcaseViewType `json:"showcase_view,omitempty"`
	// SsoAddCert : (sso) Added X.509 certificate for SSO
	SsoAddCert *SsoAddCertType `json:"sso_add_cert,omitempty"`
	// SsoAddLoginUrl : (sso) Added sign-in URL for SSO
	SsoAddLoginUrl *SsoAddLoginUrlType `json:"sso_add_login_url,omitempty"`
	// SsoAddLogoutUrl : (sso) Added sign-out URL for SSO
	SsoAddLogoutUrl *SsoAddLogoutUrlType `json:"sso_add_logout_url,omitempty"`
	// SsoChangeCert : (sso) Changed X.509 certificate for SSO
	SsoChangeCert *SsoChangeCertType `json:"sso_change_cert,omitempty"`
	// SsoChangeLoginUrl : (sso) Changed sign-in URL for SSO
	SsoChangeLoginUrl *SsoChangeLoginUrlType `json:"sso_change_login_url,omitempty"`
	// SsoChangeLogoutUrl : (sso) Changed sign-out URL for SSO
	SsoChangeLogoutUrl *SsoChangeLogoutUrlType `json:"sso_change_logout_url,omitempty"`
	// SsoChangeSamlIdentityMode : (sso) Changed SAML identity mode for SSO
	SsoChangeSamlIdentityMode *SsoChangeSamlIdentityModeType `json:"sso_change_saml_identity_mode,omitempty"`
	// SsoRemoveCert : (sso) Removed X.509 certificate for SSO
	SsoRemoveCert *SsoRemoveCertType `json:"sso_remove_cert,omitempty"`
	// SsoRemoveLoginUrl : (sso) Removed sign-in URL for SSO
	SsoRemoveLoginUrl *SsoRemoveLoginUrlType `json:"sso_remove_login_url,omitempty"`
	// SsoRemoveLogoutUrl : (sso) Removed sign-out URL for SSO
	SsoRemoveLogoutUrl *SsoRemoveLogoutUrlType `json:"sso_remove_logout_url,omitempty"`
	// TeamFolderChangeStatus : (team_folders) Changed archival status of team
	// folder
	TeamFolderChangeStatus *TeamFolderChangeStatusType `json:"team_folder_change_status,omitempty"`
	// TeamFolderCreate : (team_folders) Created team folder in active status
	TeamFolderCreate *TeamFolderCreateType `json:"team_folder_create,omitempty"`
	// TeamFolderDowngrade : (team_folders) Downgraded team folder to regular
	// shared folder
	TeamFolderDowngrade *TeamFolderDowngradeType `json:"team_folder_downgrade,omitempty"`
	// TeamFolderPermanentlyDelete : (team_folders) Permanently deleted archived
	// team folder
	TeamFolderPermanentlyDelete *TeamFolderPermanentlyDeleteType `json:"team_folder_permanently_delete,omitempty"`
	// TeamFolderRename : (team_folders) Renamed active/archived team folder
	TeamFolderRename *TeamFolderRenameType `json:"team_folder_rename,omitempty"`
	// TeamSelectiveSyncSettingsChanged : (team_folders) Changed sync default
	TeamSelectiveSyncSettingsChanged *TeamSelectiveSyncSettingsChangedType `json:"team_selective_sync_settings_changed,omitempty"`
	// AccountCaptureChangePolicy : (team_policies) Changed account capture
	// setting on team domain
	AccountCaptureChangePolicy *AccountCaptureChangePolicyType `json:"account_capture_change_policy,omitempty"`
	// AllowDownloadDisabled : (team_policies) Disabled downloads (deprecated,
	// no longer logged)
	AllowDownloadDisabled *AllowDownloadDisabledType `json:"allow_download_disabled,omitempty"`
	// AllowDownloadEnabled : (team_policies) Enabled downloads (deprecated, no
	// longer logged)
	AllowDownloadEnabled *AllowDownloadEnabledType `json:"allow_download_enabled,omitempty"`
	// DataPlacementRestrictionChangePolicy : (team_policies) Set restrictions
	// on data center locations where team data resides
	DataPlacementRestrictionChangePolicy *DataPlacementRestrictionChangePolicyType `json:"data_placement_restriction_change_policy,omitempty"`
	// DataPlacementRestrictionSatisfyPolicy : (team_policies) Completed
	// restrictions on data center locations where team data resides
	DataPlacementRestrictionSatisfyPolicy *DataPlacementRestrictionSatisfyPolicyType `json:"data_placement_restriction_satisfy_policy,omitempty"`
	// DeviceApprovalsChangeDesktopPolicy : (team_policies) Set/removed limit on
	// number of computers member can link to team Dropbox account
	DeviceApprovalsChangeDesktopPolicy *DeviceApprovalsChangeDesktopPolicyType `json:"device_approvals_change_desktop_policy,omitempty"`
	// DeviceApprovalsChangeMobilePolicy : (team_policies) Set/removed limit on
	// number of mobile devices member can link to team Dropbox account
	DeviceApprovalsChangeMobilePolicy *DeviceApprovalsChangeMobilePolicyType `json:"device_approvals_change_mobile_policy,omitempty"`
	// DeviceApprovalsChangeOverageAction : (team_policies) Changed device
	// approvals setting when member is over limit
	DeviceApprovalsChangeOverageAction *DeviceApprovalsChangeOverageActionType `json:"device_approvals_change_overage_action,omitempty"`
	// DeviceApprovalsChangeUnlinkAction : (team_policies) Changed device
	// approvals setting when member unlinks approved device
	DeviceApprovalsChangeUnlinkAction *DeviceApprovalsChangeUnlinkActionType `json:"device_approvals_change_unlink_action,omitempty"`
	// DirectoryRestrictionsAddMembers : (team_policies) Added members to
	// directory restrictions list
	DirectoryRestrictionsAddMembers *DirectoryRestrictionsAddMembersType `json:"directory_restrictions_add_members,omitempty"`
	// DirectoryRestrictionsRemoveMembers : (team_policies) Removed members from
	// directory restrictions list
	DirectoryRestrictionsRemoveMembers *DirectoryRestrictionsRemoveMembersType `json:"directory_restrictions_remove_members,omitempty"`
	// EmmAddException : (team_policies) Added members to EMM exception list
	EmmAddException *EmmAddExceptionType `json:"emm_add_exception,omitempty"`
	// EmmChangePolicy : (team_policies) Enabled/disabled enterprise mobility
	// management for members
	EmmChangePolicy *EmmChangePolicyType `json:"emm_change_policy,omitempty"`
	// EmmRemoveException : (team_policies) Removed members from EMM exception
	// list
	EmmRemoveException *EmmRemoveExceptionType `json:"emm_remove_exception,omitempty"`
	// ExtendedVersionHistoryChangePolicy : (team_policies) Accepted/opted out
	// of extended version history
	ExtendedVersionHistoryChangePolicy *ExtendedVersionHistoryChangePolicyType `json:"extended_version_history_change_policy,omitempty"`
	// FileCommentsChangePolicy : (team_policies) Enabled/disabled commenting on
	// team files
	FileCommentsChangePolicy *FileCommentsChangePolicyType `json:"file_comments_change_policy,omitempty"`
	// FileRequestsChangePolicy : (team_policies) Enabled/disabled file requests
	FileRequestsChangePolicy *FileRequestsChangePolicyType `json:"file_requests_change_policy,omitempty"`
	// FileRequestsEmailsEnabled : (team_policies) Enabled file request emails
	// for everyone (deprecated, no longer logged)
	FileRequestsEmailsEnabled *FileRequestsEmailsEnabledType `json:"file_requests_emails_enabled,omitempty"`
	// FileRequestsEmailsRestrictedToTeamOnly : (team_policies) Enabled file
	// request emails for team (deprecated, no longer logged)
	FileRequestsEmailsRestrictedToTeamOnly *FileRequestsEmailsRestrictedToTeamOnlyType `json:"file_requests_emails_restricted_to_team_only,omitempty"`
	// GoogleSsoChangePolicy : (team_policies) Enabled/disabled Google single
	// sign-on for team
	GoogleSsoChangePolicy *GoogleSsoChangePolicyType `json:"google_sso_change_policy,omitempty"`
	// GroupUserManagementChangePolicy : (team_policies) Changed who can create
	// groups
	GroupUserManagementChangePolicy *GroupUserManagementChangePolicyType `json:"group_user_management_change_policy,omitempty"`
	// MemberRequestsChangePolicy : (team_policies) Changed whether users can
	// find team when not invited
	MemberRequestsChangePolicy *MemberRequestsChangePolicyType `json:"member_requests_change_policy,omitempty"`
	// MemberSpaceLimitsAddException : (team_policies) Added members to member
	// space limit exception list
	MemberSpaceLimitsAddException *MemberSpaceLimitsAddExceptionType `json:"member_space_limits_add_exception,omitempty"`
	// MemberSpaceLimitsChangeCapsTypePolicy : (team_policies) Changed member
	// space limit type for team
	MemberSpaceLimitsChangeCapsTypePolicy *MemberSpaceLimitsChangeCapsTypePolicyType `json:"member_space_limits_change_caps_type_policy,omitempty"`
	// MemberSpaceLimitsChangePolicy : (team_policies) Changed team default
	// member space limit
	MemberSpaceLimitsChangePolicy *MemberSpaceLimitsChangePolicyType `json:"member_space_limits_change_policy,omitempty"`
	// MemberSpaceLimitsRemoveException : (team_policies) Removed members from
	// member space limit exception list
	MemberSpaceLimitsRemoveException *MemberSpaceLimitsRemoveExceptionType `json:"member_space_limits_remove_exception,omitempty"`
	// MemberSuggestionsChangePolicy : (team_policies) Enabled/disabled option
	// for team members to suggest people to add to team
	MemberSuggestionsChangePolicy *MemberSuggestionsChangePolicyType `json:"member_suggestions_change_policy,omitempty"`
	// MicrosoftOfficeAddinChangePolicy : (team_policies) Enabled/disabled
	// Microsoft Office add-in
	MicrosoftOfficeAddinChangePolicy *MicrosoftOfficeAddinChangePolicyType `json:"microsoft_office_addin_change_policy,omitempty"`
	// NetworkControlChangePolicy : (team_policies) Enabled/disabled network
	// control
	NetworkControlChangePolicy *NetworkControlChangePolicyType `json:"network_control_change_policy,omitempty"`
	// PaperChangeDeploymentPolicy : (team_policies) Changed whether Dropbox
	// Paper, when enabled, is deployed to all members or to specific members
	PaperChangeDeploymentPolicy *PaperChangeDeploymentPolicyType `json:"paper_change_deployment_policy,omitempty"`
	// PaperChangeMemberLinkPolicy : (team_policies) Changed whether non-members
	// can view Paper docs with link (deprecated, no longer logged)
	PaperChangeMemberLinkPolicy *PaperChangeMemberLinkPolicyType `json:"paper_change_member_link_policy,omitempty"`
	// PaperChangeMemberPolicy : (team_policies) Changed whether members can
	// share Paper docs outside team, and if docs are accessible only by team
	// members or anyone by default
	PaperChangeMemberPolicy *PaperChangeMemberPolicyType `json:"paper_change_member_policy,omitempty"`
	// PaperChangePolicy : (team_policies) Enabled/disabled Dropbox Paper for
	// team
	PaperChangePolicy *PaperChangePolicyType `json:"paper_change_policy,omitempty"`
	// PaperEnabledUsersGroupAddition : (team_policies) Added users to
	// Paper-enabled users list
	PaperEnabledUsersGroupAddition *PaperEnabledUsersGroupAdditionType `json:"paper_enabled_users_group_addition,omitempty"`
	// PaperEnabledUsersGroupRemoval : (team_policies) Removed users from
	// Paper-enabled users list
	PaperEnabledUsersGroupRemoval *PaperEnabledUsersGroupRemovalType `json:"paper_enabled_users_group_removal,omitempty"`
	// PermanentDeleteChangePolicy : (team_policies) Enabled/disabled ability of
	// team members to permanently delete content
	PermanentDeleteChangePolicy *PermanentDeleteChangePolicyType `json:"permanent_delete_change_policy,omitempty"`
	// SharingChangeFolderJoinPolicy : (team_policies) Changed whether team
	// members can join shared folders owned outside team
	SharingChangeFolderJoinPolicy *SharingChangeFolderJoinPolicyType `json:"sharing_change_folder_join_policy,omitempty"`
	// SharingChangeLinkPolicy : (team_policies) Changed whether members can
	// share links outside team, and if links are accessible only by team
	// members or anyone by default
	SharingChangeLinkPolicy *SharingChangeLinkPolicyType `json:"sharing_change_link_policy,omitempty"`
	// SharingChangeMemberPolicy : (team_policies) Changed whether members can
	// share files/folders outside team
	SharingChangeMemberPolicy *SharingChangeMemberPolicyType `json:"sharing_change_member_policy,omitempty"`
	// ShowcaseChangeDownloadPolicy : (team_policies) Enabled/disabled
	// downloading files from Dropbox Showcase for team
	ShowcaseChangeDownloadPolicy *ShowcaseChangeDownloadPolicyType `json:"showcase_change_download_policy,omitempty"`
	// ShowcaseChangeEnabledPolicy : (team_policies) Enabled/disabled Dropbox
	// Showcase for team
	ShowcaseChangeEnabledPolicy *ShowcaseChangeEnabledPolicyType `json:"showcase_change_enabled_policy,omitempty"`
	// ShowcaseChangeExternalSharingPolicy : (team_policies) Enabled/disabled
	// sharing Dropbox Showcase externally for team
	ShowcaseChangeExternalSharingPolicy *ShowcaseChangeExternalSharingPolicyType `json:"showcase_change_external_sharing_policy,omitempty"`
	// SmartSyncChangePolicy : (team_policies) Changed default Smart Sync
	// setting for team members
	SmartSyncChangePolicy *SmartSyncChangePolicyType `json:"smart_sync_change_policy,omitempty"`
	// SmartSyncNotOptOut : (team_policies) Opted team into Smart Sync
	SmartSyncNotOptOut *SmartSyncNotOptOutType `json:"smart_sync_not_opt_out,omitempty"`
	// SmartSyncOptOut : (team_policies) Opted team out of Smart Sync
	SmartSyncOptOut *SmartSyncOptOutType `json:"smart_sync_opt_out,omitempty"`
	// SsoChangePolicy : (team_policies) Changed single sign-on setting for team
	SsoChangePolicy *SsoChangePolicyType `json:"sso_change_policy,omitempty"`
	// TfaChangePolicy : (team_policies) Changed two-step verification setting
	// for team
	TfaChangePolicy *TfaChangePolicyType `json:"tfa_change_policy,omitempty"`
	// TwoAccountChangePolicy : (team_policies) Enabled/disabled option for
	// members to link personal Dropbox account and team account to same
	// computer
	TwoAccountChangePolicy *TwoAccountChangePolicyType `json:"two_account_change_policy,omitempty"`
	// WebSessionsChangeFixedLengthPolicy : (team_policies) Changed how long
	// members can stay signed in to Dropbox.com
	WebSessionsChangeFixedLengthPolicy *WebSessionsChangeFixedLengthPolicyType `json:"web_sessions_change_fixed_length_policy,omitempty"`
	// WebSessionsChangeIdleLengthPolicy : (team_policies) Changed how long team
	// members can be idle while signed in to Dropbox.com
	WebSessionsChangeIdleLengthPolicy *WebSessionsChangeIdleLengthPolicyType `json:"web_sessions_change_idle_length_policy,omitempty"`
	// TeamMergeFrom : (team_profile) Merged another team into this team
	TeamMergeFrom *TeamMergeFromType `json:"team_merge_from,omitempty"`
	// TeamMergeTo : (team_profile) Merged this team into another team
	TeamMergeTo *TeamMergeToType `json:"team_merge_to,omitempty"`
	// TeamProfileAddLogo : (team_profile) Added team logo to display on shared
	// link headers
	TeamProfileAddLogo *TeamProfileAddLogoType `json:"team_profile_add_logo,omitempty"`
	// TeamProfileChangeDefaultLanguage : (team_profile) Changed default
	// language for team
	TeamProfileChangeDefaultLanguage *TeamProfileChangeDefaultLanguageType `json:"team_profile_change_default_language,omitempty"`
	// TeamProfileChangeLogo : (team_profile) Changed team logo displayed on
	// shared link headers
	TeamProfileChangeLogo *TeamProfileChangeLogoType `json:"team_profile_change_logo,omitempty"`
	// TeamProfileChangeName : (team_profile) Changed team name
	TeamProfileChangeName *TeamProfileChangeNameType `json:"team_profile_change_name,omitempty"`
	// TeamProfileRemoveLogo : (team_profile) Removed team logo displayed on
	// shared link headers
	TeamProfileRemoveLogo *TeamProfileRemoveLogoType `json:"team_profile_remove_logo,omitempty"`
	// TfaAddBackupPhone : (tfa) Added backup phone for two-step verification
	TfaAddBackupPhone *TfaAddBackupPhoneType `json:"tfa_add_backup_phone,omitempty"`
	// TfaAddSecurityKey : (tfa) Added security key for two-step verification
	TfaAddSecurityKey *TfaAddSecurityKeyType `json:"tfa_add_security_key,omitempty"`
	// TfaChangeBackupPhone : (tfa) Changed backup phone for two-step
	// verification
	TfaChangeBackupPhone *TfaChangeBackupPhoneType `json:"tfa_change_backup_phone,omitempty"`
	// TfaChangeStatus : (tfa) Enabled/disabled/changed two-step verification
	// setting
	TfaChangeStatus *TfaChangeStatusType `json:"tfa_change_status,omitempty"`
	// TfaRemoveBackupPhone : (tfa) Removed backup phone for two-step
	// verification
	TfaRemoveBackupPhone *TfaRemoveBackupPhoneType `json:"tfa_remove_backup_phone,omitempty"`
	// TfaRemoveSecurityKey : (tfa) Removed security key for two-step
	// verification
	TfaRemoveSecurityKey *TfaRemoveSecurityKeyType `json:"tfa_remove_security_key,omitempty"`
	// TfaReset : (tfa) Reset two-step verification for team member
	TfaReset *TfaResetType `json:"tfa_reset,omitempty"`
}

// Valid tag values for EventType
const (
	EventTypeAppLinkTeam                                = "app_link_team"
	EventTypeAppLinkUser                                = "app_link_user"
	EventTypeAppUnlinkTeam                              = "app_unlink_team"
	EventTypeAppUnlinkUser                              = "app_unlink_user"
	EventTypeFileAddComment                             = "file_add_comment"
	EventTypeFileChangeCommentSubscription              = "file_change_comment_subscription"
	EventTypeFileDeleteComment                          = "file_delete_comment"
	EventTypeFileLikeComment                            = "file_like_comment"
	EventTypeFileResolveComment                         = "file_resolve_comment"
	EventTypeFileUnlikeComment                          = "file_unlike_comment"
	EventTypeFileUnresolveComment                       = "file_unresolve_comment"
	EventTypeDeviceChangeIpDesktop                      = "device_change_ip_desktop"
	EventTypeDeviceChangeIpMobile                       = "device_change_ip_mobile"
	EventTypeDeviceChangeIpWeb                          = "device_change_ip_web"
	EventTypeDeviceDeleteOnUnlinkFail                   = "device_delete_on_unlink_fail"
	EventTypeDeviceDeleteOnUnlinkSuccess                = "device_delete_on_unlink_success"
	EventTypeDeviceLinkFail                             = "device_link_fail"
	EventTypeDeviceLinkSuccess                          = "device_link_success"
	EventTypeDeviceManagementDisabled                   = "device_management_disabled"
	EventTypeDeviceManagementEnabled                    = "device_management_enabled"
	EventTypeDeviceUnlink                               = "device_unlink"
	EventTypeEmmRefreshAuthToken                        = "emm_refresh_auth_token"
	EventTypeAccountCaptureChangeAvailability           = "account_capture_change_availability"
	EventTypeAccountCaptureMigrateAccount               = "account_capture_migrate_account"
	EventTypeAccountCaptureNotificationEmailsSent       = "account_capture_notification_emails_sent"
	EventTypeAccountCaptureRelinquishAccount            = "account_capture_relinquish_account"
	EventTypeDisabledDomainInvites                      = "disabled_domain_invites"
	EventTypeDomainInvitesApproveRequestToJoinTeam      = "domain_invites_approve_request_to_join_team"
	EventTypeDomainInvitesDeclineRequestToJoinTeam      = "domain_invites_decline_request_to_join_team"
	EventTypeDomainInvitesEmailExistingUsers            = "domain_invites_email_existing_users"
	EventTypeDomainInvitesRequestToJoinTeam             = "domain_invites_request_to_join_team"
	EventTypeDomainInvitesSetInviteNewUserPrefToNo      = "domain_invites_set_invite_new_user_pref_to_no"
	EventTypeDomainInvitesSetInviteNewUserPrefToYes     = "domain_invites_set_invite_new_user_pref_to_yes"
	EventTypeDomainVerificationAddDomainFail            = "domain_verification_add_domain_fail"
	EventTypeDomainVerificationAddDomainSuccess         = "domain_verification_add_domain_success"
	EventTypeDomainVerificationRemoveDomain             = "domain_verification_remove_domain"
	EventTypeEnabledDomainInvites                       = "enabled_domain_invites"
	EventTypeCreateFolder                               = "create_folder"
	EventTypeFileAdd                                    = "file_add"
	EventTypeFileCopy                                   = "file_copy"
	EventTypeFileDelete                                 = "file_delete"
	EventTypeFileDownload                               = "file_download"
	EventTypeFileEdit                                   = "file_edit"
	EventTypeFileGetCopyReference                       = "file_get_copy_reference"
	EventTypeFileMove                                   = "file_move"
	EventTypeFilePermanentlyDelete                      = "file_permanently_delete"
	EventTypeFilePreview                                = "file_preview"
	EventTypeFileRename                                 = "file_rename"
	EventTypeFileRestore                                = "file_restore"
	EventTypeFileRevert                                 = "file_revert"
	EventTypeFileRollbackChanges                        = "file_rollback_changes"
	EventTypeFileSaveCopyReference                      = "file_save_copy_reference"
	EventTypeFileRequestChange                          = "file_request_change"
	EventTypeFileRequestClose                           = "file_request_close"
	EventTypeFileRequestCreate                          = "file_request_create"
	EventTypeFileRequestReceiveFile                     = "file_request_receive_file"
	EventTypeGroupAddExternalId                         = "group_add_external_id"
	EventTypeGroupAddMember                             = "group_add_member"
	EventTypeGroupChangeExternalId                      = "group_change_external_id"
	EventTypeGroupChangeManagementType                  = "group_change_management_type"
	EventTypeGroupChangeMemberRole                      = "group_change_member_role"
	EventTypeGroupCreate                                = "group_create"
	EventTypeGroupDelete                                = "group_delete"
	EventTypeGroupDescriptionUpdated                    = "group_description_updated"
	EventTypeGroupJoinPolicyUpdated                     = "group_join_policy_updated"
	EventTypeGroupMoved                                 = "group_moved"
	EventTypeGroupRemoveExternalId                      = "group_remove_external_id"
	EventTypeGroupRemoveMember                          = "group_remove_member"
	EventTypeGroupRename                                = "group_rename"
	EventTypeEmmError                                   = "emm_error"
	EventTypeLoginFail                                  = "login_fail"
	EventTypeLoginSuccess                               = "login_success"
	EventTypeLogout                                     = "logout"
	EventTypeResellerSupportSessionEnd                  = "reseller_support_session_end"
	EventTypeResellerSupportSessionStart                = "reseller_support_session_start"
	EventTypeSignInAsSessionEnd                         = "sign_in_as_session_end"
	EventTypeSignInAsSessionStart                       = "sign_in_as_session_start"
	EventTypeSsoError                                   = "sso_error"
	EventTypeMemberAddName                              = "member_add_name"
	EventTypeMemberChangeAdminRole                      = "member_change_admin_role"
	EventTypeMemberChangeEmail                          = "member_change_email"
	EventTypeMemberChangeMembershipType                 = "member_change_membership_type"
	EventTypeMemberChangeName                           = "member_change_name"
	EventTypeMemberChangeStatus                         = "member_change_status"
	EventTypeMemberPermanentlyDeleteAccountContents     = "member_permanently_delete_account_contents"
	EventTypeMemberSpaceLimitsAddCustomQuota            = "member_space_limits_add_custom_quota"
	EventTypeMemberSpaceLimitsChangeCustomQuota         = "member_space_limits_change_custom_quota"
	EventTypeMemberSpaceLimitsChangeStatus              = "member_space_limits_change_status"
	EventTypeMemberSpaceLimitsRemoveCustomQuota         = "member_space_limits_remove_custom_quota"
	EventTypeMemberSuggest                              = "member_suggest"
	EventTypeMemberTransferAccountContents              = "member_transfer_account_contents"
	EventTypeSecondaryMailsPolicyChanged                = "secondary_mails_policy_changed"
	EventTypePaperContentAddMember                      = "paper_content_add_member"
	EventTypePaperContentAddToFolder                    = "paper_content_add_to_folder"
	EventTypePaperContentArchive                        = "paper_content_archive"
	EventTypePaperContentCreate                         = "paper_content_create"
	EventTypePaperContentPermanentlyDelete              = "paper_content_permanently_delete"
	EventTypePaperContentRemoveFromFolder               = "paper_content_remove_from_folder"
	EventTypePaperContentRemoveMember                   = "paper_content_remove_member"
	EventTypePaperContentRename                         = "paper_content_rename"
	EventTypePaperContentRestore                        = "paper_content_restore"
	EventTypePaperDocAddComment                         = "paper_doc_add_comment"
	EventTypePaperDocChangeMemberRole                   = "paper_doc_change_member_role"
	EventTypePaperDocChangeSharingPolicy                = "paper_doc_change_sharing_policy"
	EventTypePaperDocChangeSubscription                 = "paper_doc_change_subscription"
	EventTypePaperDocDeleted                            = "paper_doc_deleted"
	EventTypePaperDocDeleteComment                      = "paper_doc_delete_comment"
	EventTypePaperDocDownload                           = "paper_doc_download"
	EventTypePaperDocEdit                               = "paper_doc_edit"
	EventTypePaperDocEditComment                        = "paper_doc_edit_comment"
	EventTypePaperDocFollowed                           = "paper_doc_followed"
	EventTypePaperDocMention                            = "paper_doc_mention"
	EventTypePaperDocRequestAccess                      = "paper_doc_request_access"
	EventTypePaperDocResolveComment                     = "paper_doc_resolve_comment"
	EventTypePaperDocRevert                             = "paper_doc_revert"
	EventTypePaperDocSlackShare                         = "paper_doc_slack_share"
	EventTypePaperDocTeamInvite                         = "paper_doc_team_invite"
	EventTypePaperDocTrashed                            = "paper_doc_trashed"
	EventTypePaperDocUnresolveComment                   = "paper_doc_unresolve_comment"
	EventTypePaperDocUntrashed                          = "paper_doc_untrashed"
	EventTypePaperDocView                               = "paper_doc_view"
	EventTypePaperExternalViewAllow                     = "paper_external_view_allow"
	EventTypePaperExternalViewDefaultTeam               = "paper_external_view_default_team"
	EventTypePaperExternalViewForbid                    = "paper_external_view_forbid"
	EventTypePaperFolderChangeSubscription              = "paper_folder_change_subscription"
	EventTypePaperFolderDeleted                         = "paper_folder_deleted"
	EventTypePaperFolderFollowed                        = "paper_folder_followed"
	EventTypePaperFolderTeamInvite                      = "paper_folder_team_invite"
	EventTypePasswordChange                             = "password_change"
	EventTypePasswordReset                              = "password_reset"
	EventTypePasswordResetAll                           = "password_reset_all"
	EventTypeEmmCreateExceptionsReport                  = "emm_create_exceptions_report"
	EventTypeEmmCreateUsageReport                       = "emm_create_usage_report"
	EventTypeExportMembersReport                        = "export_members_report"
	EventTypePaperAdminExportStart                      = "paper_admin_export_start"
	EventTypeSmartSyncCreateAdminPrivilegeReport        = "smart_sync_create_admin_privilege_report"
	EventTypeTeamActivityCreateReport                   = "team_activity_create_report"
	EventTypeCollectionShare                            = "collection_share"
	EventTypeNoteAclInviteOnly                          = "note_acl_invite_only"
	EventTypeNoteAclLink                                = "note_acl_link"
	EventTypeNoteAclTeamLink                            = "note_acl_team_link"
	EventTypeNoteShared                                 = "note_shared"
	EventTypeNoteShareReceive                           = "note_share_receive"
	EventTypeOpenNoteShared                             = "open_note_shared"
	EventTypeSfAddGroup                                 = "sf_add_group"
	EventTypeSfAllowNonMembersToViewSharedLinks         = "sf_allow_non_members_to_view_shared_links"
	EventTypeSfExternalInviteWarn                       = "sf_external_invite_warn"
	EventTypeSfFbInvite                                 = "sf_fb_invite"
	EventTypeSfFbInviteChangeRole                       = "sf_fb_invite_change_role"
	EventTypeSfFbUninvite                               = "sf_fb_uninvite"
	EventTypeSfInviteGroup                              = "sf_invite_group"
	EventTypeSfTeamGrantAccess                          = "sf_team_grant_access"
	EventTypeSfTeamInvite                               = "sf_team_invite"
	EventTypeSfTeamInviteChangeRole                     = "sf_team_invite_change_role"
	EventTypeSfTeamJoin                                 = "sf_team_join"
	EventTypeSfTeamJoinFromOobLink                      = "sf_team_join_from_oob_link"
	EventTypeSfTeamUninvite                             = "sf_team_uninvite"
	EventTypeSharedContentAddInvitees                   = "shared_content_add_invitees"
	EventTypeSharedContentAddLinkExpiry                 = "shared_content_add_link_expiry"
	EventTypeSharedContentAddLinkPassword               = "shared_content_add_link_password"
	EventTypeSharedContentAddMember                     = "shared_content_add_member"
	EventTypeSharedContentChangeDownloadsPolicy         = "shared_content_change_downloads_policy"
	EventTypeSharedContentChangeInviteeRole             = "shared_content_change_invitee_role"
	EventTypeSharedContentChangeLinkAudience            = "shared_content_change_link_audience"
	EventTypeSharedContentChangeLinkExpiry              = "shared_content_change_link_expiry"
	EventTypeSharedContentChangeLinkPassword            = "shared_content_change_link_password"
	EventTypeSharedContentChangeMemberRole              = "shared_content_change_member_role"
	EventTypeSharedContentChangeViewerInfoPolicy        = "shared_content_change_viewer_info_policy"
	EventTypeSharedContentClaimInvitation               = "shared_content_claim_invitation"
	EventTypeSharedContentCopy                          = "shared_content_copy"
	EventTypeSharedContentDownload                      = "shared_content_download"
	EventTypeSharedContentRelinquishMembership          = "shared_content_relinquish_membership"
	EventTypeSharedContentRemoveInvitees                = "shared_content_remove_invitees"
	EventTypeSharedContentRemoveLinkExpiry              = "shared_content_remove_link_expiry"
	EventTypeSharedContentRemoveLinkPassword            = "shared_content_remove_link_password"
	EventTypeSharedContentRemoveMember                  = "shared_content_remove_member"
	EventTypeSharedContentRequestAccess                 = "shared_content_request_access"
	EventTypeSharedContentUnshare                       = "shared_content_unshare"
	EventTypeSharedContentView                          = "shared_content_view"
	EventTypeSharedFolderChangeLinkPolicy               = "shared_folder_change_link_policy"
	EventTypeSharedFolderChangeMembersInheritancePolicy = "shared_folder_change_members_inheritance_policy"
	EventTypeSharedFolderChangeMembersManagementPolicy  = "shared_folder_change_members_management_policy"
	EventTypeSharedFolderChangeMembersPolicy            = "shared_folder_change_members_policy"
	EventTypeSharedFolderCreate                         = "shared_folder_create"
	EventTypeSharedFolderDeclineInvitation              = "shared_folder_decline_invitation"
	EventTypeSharedFolderMount                          = "shared_folder_mount"
	EventTypeSharedFolderNest                           = "shared_folder_nest"
	EventTypeSharedFolderTransferOwnership              = "shared_folder_transfer_ownership"
	EventTypeSharedFolderUnmount                        = "shared_folder_unmount"
	EventTypeSharedLinkAddExpiry                        = "shared_link_add_expiry"
	EventTypeSharedLinkChangeExpiry                     = "shared_link_change_expiry"
	EventTypeSharedLinkChangeVisibility                 = "shared_link_change_visibility"
	EventTypeSharedLinkCopy                             = "shared_link_copy"
	EventTypeSharedLinkCreate                           = "shared_link_create"
	EventTypeSharedLinkDisable                          = "shared_link_disable"
	EventTypeSharedLinkDownload                         = "shared_link_download"
	EventTypeSharedLinkRemoveExpiry                     = "shared_link_remove_expiry"
	EventTypeSharedLinkShare                            = "shared_link_share"
	EventTypeSharedLinkView                             = "shared_link_view"
	EventTypeSharedNoteOpened                           = "shared_note_opened"
	EventTypeShmodelGroupShare                          = "shmodel_group_share"
	EventTypeShowcaseAccessGranted                      = "showcase_access_granted"
	EventTypeShowcaseAddMember                          = "showcase_add_member"
	EventTypeShowcaseArchived                           = "showcase_archived"
	EventTypeShowcaseCreated                            = "showcase_created"
	EventTypeShowcaseDeleteComment                      = "showcase_delete_comment"
	EventTypeShowcaseEdited                             = "showcase_edited"
	EventTypeShowcaseEditComment                        = "showcase_edit_comment"
	EventTypeShowcaseFileAdded                          = "showcase_file_added"
	EventTypeShowcaseFileDownload                       = "showcase_file_download"
	EventTypeShowcaseFileRemoved                        = "showcase_file_removed"
	EventTypeShowcaseFileView                           = "showcase_file_view"
	EventTypeShowcasePermanentlyDeleted                 = "showcase_permanently_deleted"
	EventTypeShowcasePostComment                        = "showcase_post_comment"
	EventTypeShowcaseRemoveMember                       = "showcase_remove_member"
	EventTypeShowcaseRenamed                            = "showcase_renamed"
	EventTypeShowcaseRequestAccess                      = "showcase_request_access"
	EventTypeShowcaseResolveComment                     = "showcase_resolve_comment"
	EventTypeShowcaseRestored                           = "showcase_restored"
	EventTypeShowcaseTrashed                            = "showcase_trashed"
	EventTypeShowcaseTrashedDeprecated                  = "showcase_trashed_deprecated"
	EventTypeShowcaseUnresolveComment                   = "showcase_unresolve_comment"
	EventTypeShowcaseUntrashed                          = "showcase_untrashed"
	EventTypeShowcaseUntrashedDeprecated                = "showcase_untrashed_deprecated"
	EventTypeShowcaseView                               = "showcase_view"
	EventTypeSsoAddCert                                 = "sso_add_cert"
	EventTypeSsoAddLoginUrl                             = "sso_add_login_url"
	EventTypeSsoAddLogoutUrl                            = "sso_add_logout_url"
	EventTypeSsoChangeCert                              = "sso_change_cert"
	EventTypeSsoChangeLoginUrl                          = "sso_change_login_url"
	EventTypeSsoChangeLogoutUrl                         = "sso_change_logout_url"
	EventTypeSsoChangeSamlIdentityMode                  = "sso_change_saml_identity_mode"
	EventTypeSsoRemoveCert                              = "sso_remove_cert"
	EventTypeSsoRemoveLoginUrl                          = "sso_remove_login_url"
	EventTypeSsoRemoveLogoutUrl                         = "sso_remove_logout_url"
	EventTypeTeamFolderChangeStatus                     = "team_folder_change_status"
	EventTypeTeamFolderCreate                           = "team_folder_create"
	EventTypeTeamFolderDowngrade                        = "team_folder_downgrade"
	EventTypeTeamFolderPermanentlyDelete                = "team_folder_permanently_delete"
	EventTypeTeamFolderRename                           = "team_folder_rename"
	EventTypeTeamSelectiveSyncSettingsChanged           = "team_selective_sync_settings_changed"
	EventTypeAccountCaptureChangePolicy                 = "account_capture_change_policy"
	EventTypeAllowDownloadDisabled                      = "allow_download_disabled"
	EventTypeAllowDownloadEnabled                       = "allow_download_enabled"
	EventTypeDataPlacementRestrictionChangePolicy       = "data_placement_restriction_change_policy"
	EventTypeDataPlacementRestrictionSatisfyPolicy      = "data_placement_restriction_satisfy_policy"
	EventTypeDeviceApprovalsChangeDesktopPolicy         = "device_approvals_change_desktop_policy"
	EventTypeDeviceApprovalsChangeMobilePolicy          = "device_approvals_change_mobile_policy"
	EventTypeDeviceApprovalsChangeOverageAction         = "device_approvals_change_overage_action"
	EventTypeDeviceApprovalsChangeUnlinkAction          = "device_approvals_change_unlink_action"
	EventTypeDirectoryRestrictionsAddMembers            = "directory_restrictions_add_members"
	EventTypeDirectoryRestrictionsRemoveMembers         = "directory_restrictions_remove_members"
	EventTypeEmmAddException                            = "emm_add_exception"
	EventTypeEmmChangePolicy                            = "emm_change_policy"
	EventTypeEmmRemoveException                         = "emm_remove_exception"
	EventTypeExtendedVersionHistoryChangePolicy         = "extended_version_history_change_policy"
	EventTypeFileCommentsChangePolicy                   = "file_comments_change_policy"
	EventTypeFileRequestsChangePolicy                   = "file_requests_change_policy"
	EventTypeFileRequestsEmailsEnabled                  = "file_requests_emails_enabled"
	EventTypeFileRequestsEmailsRestrictedToTeamOnly     = "file_requests_emails_restricted_to_team_only"
	EventTypeGoogleSsoChangePolicy                      = "google_sso_change_policy"
	EventTypeGroupUserManagementChangePolicy            = "group_user_management_change_policy"
	EventTypeMemberRequestsChangePolicy                 = "member_requests_change_policy"
	EventTypeMemberSpaceLimitsAddException              = "member_space_limits_add_exception"
	EventTypeMemberSpaceLimitsChangeCapsTypePolicy      = "member_space_limits_change_caps_type_policy"
	EventTypeMemberSpaceLimitsChangePolicy              = "member_space_limits_change_policy"
	EventTypeMemberSpaceLimitsRemoveException           = "member_space_limits_remove_exception"
	EventTypeMemberSuggestionsChangePolicy              = "member_suggestions_change_policy"
	EventTypeMicrosoftOfficeAddinChangePolicy           = "microsoft_office_addin_change_policy"
	EventTypeNetworkControlChangePolicy                 = "network_control_change_policy"
	EventTypePaperChangeDeploymentPolicy                = "paper_change_deployment_policy"
	EventTypePaperChangeMemberLinkPolicy                = "paper_change_member_link_policy"
	EventTypePaperChangeMemberPolicy                    = "paper_change_member_policy"
	EventTypePaperChangePolicy                          = "paper_change_policy"
	EventTypePaperEnabledUsersGroupAddition             = "paper_enabled_users_group_addition"
	EventTypePaperEnabledUsersGroupRemoval              = "paper_enabled_users_group_removal"
	EventTypePermanentDeleteChangePolicy                = "permanent_delete_change_policy"
	EventTypeSharingChangeFolderJoinPolicy              = "sharing_change_folder_join_policy"
	EventTypeSharingChangeLinkPolicy                    = "sharing_change_link_policy"
	EventTypeSharingChangeMemberPolicy                  = "sharing_change_member_policy"
	EventTypeShowcaseChangeDownloadPolicy               = "showcase_change_download_policy"
	EventTypeShowcaseChangeEnabledPolicy                = "showcase_change_enabled_policy"
	EventTypeShowcaseChangeExternalSharingPolicy        = "showcase_change_external_sharing_policy"
	EventTypeSmartSyncChangePolicy                      = "smart_sync_change_policy"
	EventTypeSmartSyncNotOptOut                         = "smart_sync_not_opt_out"
	EventTypeSmartSyncOptOut                            = "smart_sync_opt_out"
	EventTypeSsoChangePolicy                            = "sso_change_policy"
	EventTypeTfaChangePolicy                            = "tfa_change_policy"
	EventTypeTwoAccountChangePolicy                     = "two_account_change_policy"
	EventTypeWebSessionsChangeFixedLengthPolicy         = "web_sessions_change_fixed_length_policy"
	EventTypeWebSessionsChangeIdleLengthPolicy          = "web_sessions_change_idle_length_policy"
	EventTypeTeamMergeFrom                              = "team_merge_from"
	EventTypeTeamMergeTo                                = "team_merge_to"
	EventTypeTeamProfileAddLogo                         = "team_profile_add_logo"
	EventTypeTeamProfileChangeDefaultLanguage           = "team_profile_change_default_language"
	EventTypeTeamProfileChangeLogo                      = "team_profile_change_logo"
	EventTypeTeamProfileChangeName                      = "team_profile_change_name"
	EventTypeTeamProfileRemoveLogo                      = "team_profile_remove_logo"
	EventTypeTfaAddBackupPhone                          = "tfa_add_backup_phone"
	EventTypeTfaAddSecurityKey                          = "tfa_add_security_key"
	EventTypeTfaChangeBackupPhone                       = "tfa_change_backup_phone"
	EventTypeTfaChangeStatus                            = "tfa_change_status"
	EventTypeTfaRemoveBackupPhone                       = "tfa_remove_backup_phone"
	EventTypeTfaRemoveSecurityKey                       = "tfa_remove_security_key"
	EventTypeTfaReset                                   = "tfa_reset"
	EventTypeOther                                      = "other"
)

// UnmarshalJSON deserializes into a EventType instance
func (u *EventType) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// AppLinkTeam : (apps) Linked app for team
		AppLinkTeam json.RawMessage `json:"app_link_team,omitempty"`
		// AppLinkUser : (apps) Linked app for member
		AppLinkUser json.RawMessage `json:"app_link_user,omitempty"`
		// AppUnlinkTeam : (apps) Unlinked app for team
		AppUnlinkTeam json.RawMessage `json:"app_unlink_team,omitempty"`
		// AppUnlinkUser : (apps) Unlinked app for member
		AppUnlinkUser json.RawMessage `json:"app_unlink_user,omitempty"`
		// FileAddComment : (comments) Added file comment
		FileAddComment json.RawMessage `json:"file_add_comment,omitempty"`
		// FileChangeCommentSubscription : (comments) Subscribed to or
		// unsubscribed from comment notifications for file
		FileChangeCommentSubscription json.RawMessage `json:"file_change_comment_subscription,omitempty"`
		// FileDeleteComment : (comments) Deleted file comment
		FileDeleteComment json.RawMessage `json:"file_delete_comment,omitempty"`
		// FileLikeComment : (comments) Liked file comment (deprecated, no
		// longer logged)
		FileLikeComment json.RawMessage `json:"file_like_comment,omitempty"`
		// FileResolveComment : (comments) Resolved file comment
		FileResolveComment json.RawMessage `json:"file_resolve_comment,omitempty"`
		// FileUnlikeComment : (comments) Unliked file comment (deprecated, no
		// longer logged)
		FileUnlikeComment json.RawMessage `json:"file_unlike_comment,omitempty"`
		// FileUnresolveComment : (comments) Unresolved file comment
		FileUnresolveComment json.RawMessage `json:"file_unresolve_comment,omitempty"`
		// DeviceChangeIpDesktop : (devices) Changed IP address associated with
		// active desktop session
		DeviceChangeIpDesktop json.RawMessage `json:"device_change_ip_desktop,omitempty"`
		// DeviceChangeIpMobile : (devices) Changed IP address associated with
		// active mobile session
		DeviceChangeIpMobile json.RawMessage `json:"device_change_ip_mobile,omitempty"`
		// DeviceChangeIpWeb : (devices) Changed IP address associated with
		// active web session
		DeviceChangeIpWeb json.RawMessage `json:"device_change_ip_web,omitempty"`
		// DeviceDeleteOnUnlinkFail : (devices) Failed to delete all files from
		// unlinked device
		DeviceDeleteOnUnlinkFail json.RawMessage `json:"device_delete_on_unlink_fail,omitempty"`
		// DeviceDeleteOnUnlinkSuccess : (devices) Deleted all files from
		// unlinked device
		DeviceDeleteOnUnlinkSuccess json.RawMessage `json:"device_delete_on_unlink_success,omitempty"`
		// DeviceLinkFail : (devices) Failed to link device
		DeviceLinkFail json.RawMessage `json:"device_link_fail,omitempty"`
		// DeviceLinkSuccess : (devices) Linked device
		DeviceLinkSuccess json.RawMessage `json:"device_link_success,omitempty"`
		// DeviceManagementDisabled : (devices) Disabled device management
		// (deprecated, no longer logged)
		DeviceManagementDisabled json.RawMessage `json:"device_management_disabled,omitempty"`
		// DeviceManagementEnabled : (devices) Enabled device management
		// (deprecated, no longer logged)
		DeviceManagementEnabled json.RawMessage `json:"device_management_enabled,omitempty"`
		// DeviceUnlink : (devices) Disconnected device
		DeviceUnlink json.RawMessage `json:"device_unlink,omitempty"`
		// EmmRefreshAuthToken : (devices) Refreshed auth token used for setting
		// up enterprise mobility management
		EmmRefreshAuthToken json.RawMessage `json:"emm_refresh_auth_token,omitempty"`
		// AccountCaptureChangeAvailability : (domains) Granted/revoked option
		// to enable account capture on team domains
		AccountCaptureChangeAvailability json.RawMessage `json:"account_capture_change_availability,omitempty"`
		// AccountCaptureMigrateAccount : (domains) Account-captured user
		// migrated account to team
		AccountCaptureMigrateAccount json.RawMessage `json:"account_capture_migrate_account,omitempty"`
		// AccountCaptureNotificationEmailsSent : (domains) Sent proactive
		// account capture email to all unmanaged members
		AccountCaptureNotificationEmailsSent json.RawMessage `json:"account_capture_notification_emails_sent,omitempty"`
		// AccountCaptureRelinquishAccount : (domains) Account-captured user
		// changed account email to personal email
		AccountCaptureRelinquishAccount json.RawMessage `json:"account_capture_relinquish_account,omitempty"`
		// DisabledDomainInvites : (domains) Disabled domain invites
		// (deprecated, no longer logged)
		DisabledDomainInvites json.RawMessage `json:"disabled_domain_invites,omitempty"`
		// DomainInvitesApproveRequestToJoinTeam : (domains) Approved user's
		// request to join team
		DomainInvitesApproveRequestToJoinTeam json.RawMessage `json:"domain_invites_approve_request_to_join_team,omitempty"`
		// DomainInvitesDeclineRequestToJoinTeam : (domains) Declined user's
		// request to join team
		DomainInvitesDeclineRequestToJoinTeam json.RawMessage `json:"domain_invites_decline_request_to_join_team,omitempty"`
		// DomainInvitesEmailExistingUsers : (domains) Sent domain invites to
		// existing domain accounts (deprecated, no longer logged)
		DomainInvitesEmailExistingUsers json.RawMessage `json:"domain_invites_email_existing_users,omitempty"`
		// DomainInvitesRequestToJoinTeam : (domains) Requested to join team
		DomainInvitesRequestToJoinTeam json.RawMessage `json:"domain_invites_request_to_join_team,omitempty"`
		// DomainInvitesSetInviteNewUserPrefToNo : (domains) Disabled
		// "Automatically invite new users" (deprecated, no longer logged)
		DomainInvitesSetInviteNewUserPrefToNo json.RawMessage `json:"domain_invites_set_invite_new_user_pref_to_no,omitempty"`
		// DomainInvitesSetInviteNewUserPrefToYes : (domains) Enabled
		// "Automatically invite new users" (deprecated, no longer logged)
		DomainInvitesSetInviteNewUserPrefToYes json.RawMessage `json:"domain_invites_set_invite_new_user_pref_to_yes,omitempty"`
		// DomainVerificationAddDomainFail : (domains) Failed to verify team
		// domain
		DomainVerificationAddDomainFail json.RawMessage `json:"domain_verification_add_domain_fail,omitempty"`
		// DomainVerificationAddDomainSuccess : (domains) Verified team domain
		DomainVerificationAddDomainSuccess json.RawMessage `json:"domain_verification_add_domain_success,omitempty"`
		// DomainVerificationRemoveDomain : (domains) Removed domain from list
		// of verified team domains
		DomainVerificationRemoveDomain json.RawMessage `json:"domain_verification_remove_domain,omitempty"`
		// EnabledDomainInvites : (domains) Enabled domain invites (deprecated,
		// no longer logged)
		EnabledDomainInvites json.RawMessage `json:"enabled_domain_invites,omitempty"`
		// CreateFolder : (file_operations) Created folders (deprecated, no
		// longer logged)
		CreateFolder json.RawMessage `json:"create_folder,omitempty"`
		// FileAdd : (file_operations) Added files and/or folders
		FileAdd json.RawMessage `json:"file_add,omitempty"`
		// FileCopy : (file_operations) Copied files and/or folders
		FileCopy json.RawMessage `json:"file_copy,omitempty"`
		// FileDelete : (file_operations) Deleted files and/or folders
		FileDelete json.RawMessage `json:"file_delete,omitempty"`
		// FileDownload : (file_operations) Downloaded files and/or folders
		FileDownload json.RawMessage `json:"file_download,omitempty"`
		// FileEdit : (file_operations) Edited files
		FileEdit json.RawMessage `json:"file_edit,omitempty"`
		// FileGetCopyReference : (file_operations) Created copy reference to
		// file/folder
		FileGetCopyReference json.RawMessage `json:"file_get_copy_reference,omitempty"`
		// FileMove : (file_operations) Moved files and/or folders
		FileMove json.RawMessage `json:"file_move,omitempty"`
		// FilePermanentlyDelete : (file_operations) Permanently deleted files
		// and/or folders
		FilePermanentlyDelete json.RawMessage `json:"file_permanently_delete,omitempty"`
		// FilePreview : (file_operations) Previewed files and/or folders
		FilePreview json.RawMessage `json:"file_preview,omitempty"`
		// FileRename : (file_operations) Renamed files and/or folders
		FileRename json.RawMessage `json:"file_rename,omitempty"`
		// FileRestore : (file_operations) Restored deleted files and/or folders
		FileRestore json.RawMessage `json:"file_restore,omitempty"`
		// FileRevert : (file_operations) Reverted files to previous version
		FileRevert json.RawMessage `json:"file_revert,omitempty"`
		// FileRollbackChanges : (file_operations) Rolled back file actions
		FileRollbackChanges json.RawMessage `json:"file_rollback_changes,omitempty"`
		// FileSaveCopyReference : (file_operations) Saved file/folder using
		// copy reference
		FileSaveCopyReference json.RawMessage `json:"file_save_copy_reference,omitempty"`
		// FileRequestChange : (file_requests) Changed file request
		FileRequestChange json.RawMessage `json:"file_request_change,omitempty"`
		// FileRequestClose : (file_requests) Closed file request
		FileRequestClose json.RawMessage `json:"file_request_close,omitempty"`
		// FileRequestCreate : (file_requests) Created file request
		FileRequestCreate json.RawMessage `json:"file_request_create,omitempty"`
		// FileRequestReceiveFile : (file_requests) Received files for file
		// request
		FileRequestReceiveFile json.RawMessage `json:"file_request_receive_file,omitempty"`
		// GroupAddExternalId : (groups) Added external ID for group
		GroupAddExternalId json.RawMessage `json:"group_add_external_id,omitempty"`
		// GroupAddMember : (groups) Added team members to group
		GroupAddMember json.RawMessage `json:"group_add_member,omitempty"`
		// GroupChangeExternalId : (groups) Changed external ID for group
		GroupChangeExternalId json.RawMessage `json:"group_change_external_id,omitempty"`
		// GroupChangeManagementType : (groups) Changed group management type
		GroupChangeManagementType json.RawMessage `json:"group_change_management_type,omitempty"`
		// GroupChangeMemberRole : (groups) Changed manager permissions of group
		// member
		GroupChangeMemberRole json.RawMessage `json:"group_change_member_role,omitempty"`
		// GroupCreate : (groups) Created group
		GroupCreate json.RawMessage `json:"group_create,omitempty"`
		// GroupDelete : (groups) Deleted group
		GroupDelete json.RawMessage `json:"group_delete,omitempty"`
		// GroupDescriptionUpdated : (groups) Updated group (deprecated, no
		// longer logged)
		GroupDescriptionUpdated json.RawMessage `json:"group_description_updated,omitempty"`
		// GroupJoinPolicyUpdated : (groups) Updated group join policy
		// (deprecated, no longer logged)
		GroupJoinPolicyUpdated json.RawMessage `json:"group_join_policy_updated,omitempty"`
		// GroupMoved : (groups) Moved group (deprecated, no longer logged)
		GroupMoved json.RawMessage `json:"group_moved,omitempty"`
		// GroupRemoveExternalId : (groups) Removed external ID for group
		GroupRemoveExternalId json.RawMessage `json:"group_remove_external_id,omitempty"`
		// GroupRemoveMember : (groups) Removed team members from group
		GroupRemoveMember json.RawMessage `json:"group_remove_member,omitempty"`
		// GroupRename : (groups) Renamed group
		GroupRename json.RawMessage `json:"group_rename,omitempty"`
		// EmmError : (logins) Failed to sign in via EMM (deprecated, replaced
		// by 'Failed to sign in')
		EmmError json.RawMessage `json:"emm_error,omitempty"`
		// LoginFail : (logins) Failed to sign in
		LoginFail json.RawMessage `json:"login_fail,omitempty"`
		// LoginSuccess : (logins) Signed in
		LoginSuccess json.RawMessage `json:"login_success,omitempty"`
		// Logout : (logins) Signed out
		Logout json.RawMessage `json:"logout,omitempty"`
		// ResellerSupportSessionEnd : (logins) Ended reseller support session
		ResellerSupportSessionEnd json.RawMessage `json:"reseller_support_session_end,omitempty"`
		// ResellerSupportSessionStart : (logins) Started reseller support
		// session
		ResellerSupportSessionStart json.RawMessage `json:"reseller_support_session_start,omitempty"`
		// SignInAsSessionEnd : (logins) Ended admin sign-in-as session
		SignInAsSessionEnd json.RawMessage `json:"sign_in_as_session_end,omitempty"`
		// SignInAsSessionStart : (logins) Started admin sign-in-as session
		SignInAsSessionStart json.RawMessage `json:"sign_in_as_session_start,omitempty"`
		// SsoError : (logins) Failed to sign in via SSO (deprecated, replaced
		// by 'Failed to sign in')
		SsoError json.RawMessage `json:"sso_error,omitempty"`
		// MemberAddName : (members) Added team member name
		MemberAddName json.RawMessage `json:"member_add_name,omitempty"`
		// MemberChangeAdminRole : (members) Changed team member admin role
		MemberChangeAdminRole json.RawMessage `json:"member_change_admin_role,omitempty"`
		// MemberChangeEmail : (members) Changed team member email
		MemberChangeEmail json.RawMessage `json:"member_change_email,omitempty"`
		// MemberChangeMembershipType : (members) Changed membership type
		// (limited/full) of member (deprecated, no longer logged)
		MemberChangeMembershipType json.RawMessage `json:"member_change_membership_type,omitempty"`
		// MemberChangeName : (members) Changed team member name
		MemberChangeName json.RawMessage `json:"member_change_name,omitempty"`
		// MemberChangeStatus : (members) Changed member status (invited,
		// joined, suspended, etc.)
		MemberChangeStatus json.RawMessage `json:"member_change_status,omitempty"`
		// MemberPermanentlyDeleteAccountContents : (members) Permanently
		// deleted contents of deleted team member account
		MemberPermanentlyDeleteAccountContents json.RawMessage `json:"member_permanently_delete_account_contents,omitempty"`
		// MemberSpaceLimitsAddCustomQuota : (members) Set custom member space
		// limit
		MemberSpaceLimitsAddCustomQuota json.RawMessage `json:"member_space_limits_add_custom_quota,omitempty"`
		// MemberSpaceLimitsChangeCustomQuota : (members) Changed custom member
		// space limit
		MemberSpaceLimitsChangeCustomQuota json.RawMessage `json:"member_space_limits_change_custom_quota,omitempty"`
		// MemberSpaceLimitsChangeStatus : (members) Changed space limit status
		MemberSpaceLimitsChangeStatus json.RawMessage `json:"member_space_limits_change_status,omitempty"`
		// MemberSpaceLimitsRemoveCustomQuota : (members) Removed custom member
		// space limit
		MemberSpaceLimitsRemoveCustomQuota json.RawMessage `json:"member_space_limits_remove_custom_quota,omitempty"`
		// MemberSuggest : (members) Suggested person to add to team
		MemberSuggest json.RawMessage `json:"member_suggest,omitempty"`
		// MemberTransferAccountContents : (members) Transferred contents of
		// deleted member account to another member
		MemberTransferAccountContents json.RawMessage `json:"member_transfer_account_contents,omitempty"`
		// SecondaryMailsPolicyChanged : (members) Secondary mails policy
		// changed
		SecondaryMailsPolicyChanged json.RawMessage `json:"secondary_mails_policy_changed,omitempty"`
		// PaperContentAddMember : (paper) Added team member to Paper doc/folder
		PaperContentAddMember json.RawMessage `json:"paper_content_add_member,omitempty"`
		// PaperContentAddToFolder : (paper) Added Paper doc/folder to folder
		PaperContentAddToFolder json.RawMessage `json:"paper_content_add_to_folder,omitempty"`
		// PaperContentArchive : (paper) Archived Paper doc/folder
		PaperContentArchive json.RawMessage `json:"paper_content_archive,omitempty"`
		// PaperContentCreate : (paper) Created Paper doc/folder
		PaperContentCreate json.RawMessage `json:"paper_content_create,omitempty"`
		// PaperContentPermanentlyDelete : (paper) Permanently deleted Paper
		// doc/folder
		PaperContentPermanentlyDelete json.RawMessage `json:"paper_content_permanently_delete,omitempty"`
		// PaperContentRemoveFromFolder : (paper) Removed Paper doc/folder from
		// folder
		PaperContentRemoveFromFolder json.RawMessage `json:"paper_content_remove_from_folder,omitempty"`
		// PaperContentRemoveMember : (paper) Removed team member from Paper
		// doc/folder
		PaperContentRemoveMember json.RawMessage `json:"paper_content_remove_member,omitempty"`
		// PaperContentRename : (paper) Renamed Paper doc/folder
		PaperContentRename json.RawMessage `json:"paper_content_rename,omitempty"`
		// PaperContentRestore : (paper) Restored archived Paper doc/folder
		PaperContentRestore json.RawMessage `json:"paper_content_restore,omitempty"`
		// PaperDocAddComment : (paper) Added Paper doc comment
		PaperDocAddComment json.RawMessage `json:"paper_doc_add_comment,omitempty"`
		// PaperDocChangeMemberRole : (paper) Changed team member permissions
		// for Paper doc
		PaperDocChangeMemberRole json.RawMessage `json:"paper_doc_change_member_role,omitempty"`
		// PaperDocChangeSharingPolicy : (paper) Changed sharing setting for
		// Paper doc
		PaperDocChangeSharingPolicy json.RawMessage `json:"paper_doc_change_sharing_policy,omitempty"`
		// PaperDocChangeSubscription : (paper) Followed/unfollowed Paper doc
		PaperDocChangeSubscription json.RawMessage `json:"paper_doc_change_subscription,omitempty"`
		// PaperDocDeleted : (paper) Archived Paper doc (deprecated, no longer
		// logged)
		PaperDocDeleted json.RawMessage `json:"paper_doc_deleted,omitempty"`
		// PaperDocDeleteComment : (paper) Deleted Paper doc comment
		PaperDocDeleteComment json.RawMessage `json:"paper_doc_delete_comment,omitempty"`
		// PaperDocDownload : (paper) Downloaded Paper doc in specific format
		PaperDocDownload json.RawMessage `json:"paper_doc_download,omitempty"`
		// PaperDocEdit : (paper) Edited Paper doc
		PaperDocEdit json.RawMessage `json:"paper_doc_edit,omitempty"`
		// PaperDocEditComment : (paper) Edited Paper doc comment
		PaperDocEditComment json.RawMessage `json:"paper_doc_edit_comment,omitempty"`
		// PaperDocFollowed : (paper) Followed Paper doc (deprecated, replaced
		// by 'Followed/unfollowed Paper doc')
		PaperDocFollowed json.RawMessage `json:"paper_doc_followed,omitempty"`
		// PaperDocMention : (paper) Mentioned team member in Paper doc
		PaperDocMention json.RawMessage `json:"paper_doc_mention,omitempty"`
		// PaperDocRequestAccess : (paper) Requested access to Paper doc
		PaperDocRequestAccess json.RawMessage `json:"paper_doc_request_access,omitempty"`
		// PaperDocResolveComment : (paper) Resolved Paper doc comment
		PaperDocResolveComment json.RawMessage `json:"paper_doc_resolve_comment,omitempty"`
		// PaperDocRevert : (paper) Restored Paper doc to previous version
		PaperDocRevert json.RawMessage `json:"paper_doc_revert,omitempty"`
		// PaperDocSlackShare : (paper) Shared Paper doc via Slack
		PaperDocSlackShare json.RawMessage `json:"paper_doc_slack_share,omitempty"`
		// PaperDocTeamInvite : (paper) Shared Paper doc with team member
		// (deprecated, no longer logged)
		PaperDocTeamInvite json.RawMessage `json:"paper_doc_team_invite,omitempty"`
		// PaperDocTrashed : (paper) Deleted Paper doc
		PaperDocTrashed json.RawMessage `json:"paper_doc_trashed,omitempty"`
		// PaperDocUnresolveComment : (paper) Unresolved Paper doc comment
		PaperDocUnresolveComment json.RawMessage `json:"paper_doc_unresolve_comment,omitempty"`
		// PaperDocUntrashed : (paper) Restored Paper doc
		PaperDocUntrashed json.RawMessage `json:"paper_doc_untrashed,omitempty"`
		// PaperDocView : (paper) Viewed Paper doc
		PaperDocView json.RawMessage `json:"paper_doc_view,omitempty"`
		// PaperExternalViewAllow : (paper) Changed Paper external sharing
		// setting to anyone (deprecated, no longer logged)
		PaperExternalViewAllow json.RawMessage `json:"paper_external_view_allow,omitempty"`
		// PaperExternalViewDefaultTeam : (paper) Changed Paper external sharing
		// setting to default team (deprecated, no longer logged)
		PaperExternalViewDefaultTeam json.RawMessage `json:"paper_external_view_default_team,omitempty"`
		// PaperExternalViewForbid : (paper) Changed Paper external sharing
		// setting to team-only (deprecated, no longer logged)
		PaperExternalViewForbid json.RawMessage `json:"paper_external_view_forbid,omitempty"`
		// PaperFolderChangeSubscription : (paper) Followed/unfollowed Paper
		// folder
		PaperFolderChangeSubscription json.RawMessage `json:"paper_folder_change_subscription,omitempty"`
		// PaperFolderDeleted : (paper) Archived Paper folder (deprecated, no
		// longer logged)
		PaperFolderDeleted json.RawMessage `json:"paper_folder_deleted,omitempty"`
		// PaperFolderFollowed : (paper) Followed Paper folder (deprecated,
		// replaced by 'Followed/unfollowed Paper folder')
		PaperFolderFollowed json.RawMessage `json:"paper_folder_followed,omitempty"`
		// PaperFolderTeamInvite : (paper) Shared Paper folder with member
		// (deprecated, no longer logged)
		PaperFolderTeamInvite json.RawMessage `json:"paper_folder_team_invite,omitempty"`
		// PasswordChange : (passwords) Changed password
		PasswordChange json.RawMessage `json:"password_change,omitempty"`
		// PasswordReset : (passwords) Reset password
		PasswordReset json.RawMessage `json:"password_reset,omitempty"`
		// PasswordResetAll : (passwords) Reset all team member passwords
		PasswordResetAll json.RawMessage `json:"password_reset_all,omitempty"`
		// EmmCreateExceptionsReport : (reports) Created EMM-excluded users
		// report
		EmmCreateExceptionsReport json.RawMessage `json:"emm_create_exceptions_report,omitempty"`
		// EmmCreateUsageReport : (reports) Created EMM mobile app usage report
		EmmCreateUsageReport json.RawMessage `json:"emm_create_usage_report,omitempty"`
		// ExportMembersReport : (reports) Created member data report
		ExportMembersReport json.RawMessage `json:"export_members_report,omitempty"`
		// PaperAdminExportStart : (reports) Exported all team Paper docs
		PaperAdminExportStart json.RawMessage `json:"paper_admin_export_start,omitempty"`
		// SmartSyncCreateAdminPrivilegeReport : (reports) Created Smart Sync
		// non-admin devices report
		SmartSyncCreateAdminPrivilegeReport json.RawMessage `json:"smart_sync_create_admin_privilege_report,omitempty"`
		// TeamActivityCreateReport : (reports) Created team activity report
		TeamActivityCreateReport json.RawMessage `json:"team_activity_create_report,omitempty"`
		// CollectionShare : (sharing) Shared album
		CollectionShare json.RawMessage `json:"collection_share,omitempty"`
		// NoteAclInviteOnly : (sharing) Changed Paper doc to invite-only
		// (deprecated, no longer logged)
		NoteAclInviteOnly json.RawMessage `json:"note_acl_invite_only,omitempty"`
		// NoteAclLink : (sharing) Changed Paper doc to link-accessible
		// (deprecated, no longer logged)
		NoteAclLink json.RawMessage `json:"note_acl_link,omitempty"`
		// NoteAclTeamLink : (sharing) Changed Paper doc to link-accessible for
		// team (deprecated, no longer logged)
		NoteAclTeamLink json.RawMessage `json:"note_acl_team_link,omitempty"`
		// NoteShared : (sharing) Shared Paper doc (deprecated, no longer
		// logged)
		NoteShared json.RawMessage `json:"note_shared,omitempty"`
		// NoteShareReceive : (sharing) Shared received Paper doc (deprecated,
		// no longer logged)
		NoteShareReceive json.RawMessage `json:"note_share_receive,omitempty"`
		// OpenNoteShared : (sharing) Opened shared Paper doc (deprecated, no
		// longer logged)
		OpenNoteShared json.RawMessage `json:"open_note_shared,omitempty"`
		// SfAddGroup : (sharing) Added team to shared folder (deprecated, no
		// longer logged)
		SfAddGroup json.RawMessage `json:"sf_add_group,omitempty"`
		// SfAllowNonMembersToViewSharedLinks : (sharing) Allowed
		// non-collaborators to view links to files in shared folder
		// (deprecated, no longer logged)
		SfAllowNonMembersToViewSharedLinks json.RawMessage `json:"sf_allow_non_members_to_view_shared_links,omitempty"`
		// SfExternalInviteWarn : (sharing) Set team members to see warning
		// before sharing folders outside team (deprecated, no longer logged)
		SfExternalInviteWarn json.RawMessage `json:"sf_external_invite_warn,omitempty"`
		// SfFbInvite : (sharing) Invited Facebook users to shared folder
		// (deprecated, no longer logged)
		SfFbInvite json.RawMessage `json:"sf_fb_invite,omitempty"`
		// SfFbInviteChangeRole : (sharing) Changed Facebook user's role in
		// shared folder (deprecated, no longer logged)
		SfFbInviteChangeRole json.RawMessage `json:"sf_fb_invite_change_role,omitempty"`
		// SfFbUninvite : (sharing) Uninvited Facebook user from shared folder
		// (deprecated, no longer logged)
		SfFbUninvite json.RawMessage `json:"sf_fb_uninvite,omitempty"`
		// SfInviteGroup : (sharing) Invited group to shared folder (deprecated,
		// no longer logged)
		SfInviteGroup json.RawMessage `json:"sf_invite_group,omitempty"`
		// SfTeamGrantAccess : (sharing) Granted access to shared folder
		// (deprecated, no longer logged)
		SfTeamGrantAccess json.RawMessage `json:"sf_team_grant_access,omitempty"`
		// SfTeamInvite : (sharing) Invited team members to shared folder
		// (deprecated, replaced by 'Invited user to Dropbox and added them to
		// shared file/folder')
		SfTeamInvite json.RawMessage `json:"sf_team_invite,omitempty"`
		// SfTeamInviteChangeRole : (sharing) Changed team member's role in
		// shared folder (deprecated, no longer logged)
		SfTeamInviteChangeRole json.RawMessage `json:"sf_team_invite_change_role,omitempty"`
		// SfTeamJoin : (sharing) Joined team member's shared folder
		// (deprecated, no longer logged)
		SfTeamJoin json.RawMessage `json:"sf_team_join,omitempty"`
		// SfTeamJoinFromOobLink : (sharing) Joined team member's shared folder
		// from link (deprecated, no longer logged)
		SfTeamJoinFromOobLink json.RawMessage `json:"sf_team_join_from_oob_link,omitempty"`
		// SfTeamUninvite : (sharing) Unshared folder with team member
		// (deprecated, replaced by 'Removed invitee from shared file/folder
		// before invite was accepted')
		SfTeamUninvite json.RawMessage `json:"sf_team_uninvite,omitempty"`
		// SharedContentAddInvitees : (sharing) Invited user to Dropbox and
		// added them to shared file/folder
		SharedContentAddInvitees json.RawMessage `json:"shared_content_add_invitees,omitempty"`
		// SharedContentAddLinkExpiry : (sharing) Added expiration date to link
		// for shared file/folder
		SharedContentAddLinkExpiry json.RawMessage `json:"shared_content_add_link_expiry,omitempty"`
		// SharedContentAddLinkPassword : (sharing) Added password to link for
		// shared file/folder
		SharedContentAddLinkPassword json.RawMessage `json:"shared_content_add_link_password,omitempty"`
		// SharedContentAddMember : (sharing) Added users and/or groups to
		// shared file/folder
		SharedContentAddMember json.RawMessage `json:"shared_content_add_member,omitempty"`
		// SharedContentChangeDownloadsPolicy : (sharing) Changed whether
		// members can download shared file/folder
		SharedContentChangeDownloadsPolicy json.RawMessage `json:"shared_content_change_downloads_policy,omitempty"`
		// SharedContentChangeInviteeRole : (sharing) Changed access type of
		// invitee to shared file/folder before invite was accepted
		SharedContentChangeInviteeRole json.RawMessage `json:"shared_content_change_invitee_role,omitempty"`
		// SharedContentChangeLinkAudience : (sharing) Changed link audience of
		// shared file/folder
		SharedContentChangeLinkAudience json.RawMessage `json:"shared_content_change_link_audience,omitempty"`
		// SharedContentChangeLinkExpiry : (sharing) Changed link expiration of
		// shared file/folder
		SharedContentChangeLinkExpiry json.RawMessage `json:"shared_content_change_link_expiry,omitempty"`
		// SharedContentChangeLinkPassword : (sharing) Changed link password of
		// shared file/folder
		SharedContentChangeLinkPassword json.RawMessage `json:"shared_content_change_link_password,omitempty"`
		// SharedContentChangeMemberRole : (sharing) Changed access type of
		// shared file/folder member
		SharedContentChangeMemberRole json.RawMessage `json:"shared_content_change_member_role,omitempty"`
		// SharedContentChangeViewerInfoPolicy : (sharing) Changed whether
		// members can see who viewed shared file/folder
		SharedContentChangeViewerInfoPolicy json.RawMessage `json:"shared_content_change_viewer_info_policy,omitempty"`
		// SharedContentClaimInvitation : (sharing) Acquired membership of
		// shared file/folder by accepting invite
		SharedContentClaimInvitation json.RawMessage `json:"shared_content_claim_invitation,omitempty"`
		// SharedContentCopy : (sharing) Copied shared file/folder to own
		// Dropbox
		SharedContentCopy json.RawMessage `json:"shared_content_copy,omitempty"`
		// SharedContentDownload : (sharing) Downloaded shared file/folder
		SharedContentDownload json.RawMessage `json:"shared_content_download,omitempty"`
		// SharedContentRelinquishMembership : (sharing) Left shared file/folder
		SharedContentRelinquishMembership json.RawMessage `json:"shared_content_relinquish_membership,omitempty"`
		// SharedContentRemoveInvitees : (sharing) Removed invitee from shared
		// file/folder before invite was accepted
		SharedContentRemoveInvitees json.RawMessage `json:"shared_content_remove_invitees,omitempty"`
		// SharedContentRemoveLinkExpiry : (sharing) Removed link expiration
		// date of shared file/folder
		SharedContentRemoveLinkExpiry json.RawMessage `json:"shared_content_remove_link_expiry,omitempty"`
		// SharedContentRemoveLinkPassword : (sharing) Removed link password of
		// shared file/folder
		SharedContentRemoveLinkPassword json.RawMessage `json:"shared_content_remove_link_password,omitempty"`
		// SharedContentRemoveMember : (sharing) Removed user/group from shared
		// file/folder
		SharedContentRemoveMember json.RawMessage `json:"shared_content_remove_member,omitempty"`
		// SharedContentRequestAccess : (sharing) Requested access to shared
		// file/folder
		SharedContentRequestAccess json.RawMessage `json:"shared_content_request_access,omitempty"`
		// SharedContentUnshare : (sharing) Unshared file/folder by clearing
		// membership and turning off link
		SharedContentUnshare json.RawMessage `json:"shared_content_unshare,omitempty"`
		// SharedContentView : (sharing) Previewed shared file/folder
		SharedContentView json.RawMessage `json:"shared_content_view,omitempty"`
		// SharedFolderChangeLinkPolicy : (sharing) Changed who can access
		// shared folder via link
		SharedFolderChangeLinkPolicy json.RawMessage `json:"shared_folder_change_link_policy,omitempty"`
		// SharedFolderChangeMembersInheritancePolicy : (sharing) Changed
		// whether shared folder inherits members from parent folder
		SharedFolderChangeMembersInheritancePolicy json.RawMessage `json:"shared_folder_change_members_inheritance_policy,omitempty"`
		// SharedFolderChangeMembersManagementPolicy : (sharing) Changed who can
		// add/remove members of shared folder
		SharedFolderChangeMembersManagementPolicy json.RawMessage `json:"shared_folder_change_members_management_policy,omitempty"`
		// SharedFolderChangeMembersPolicy : (sharing) Changed who can become
		// member of shared folder
		SharedFolderChangeMembersPolicy json.RawMessage `json:"shared_folder_change_members_policy,omitempty"`
		// SharedFolderCreate : (sharing) Created shared folder
		SharedFolderCreate json.RawMessage `json:"shared_folder_create,omitempty"`
		// SharedFolderDeclineInvitation : (sharing) Declined team member's
		// invite to shared folder
		SharedFolderDeclineInvitation json.RawMessage `json:"shared_folder_decline_invitation,omitempty"`
		// SharedFolderMount : (sharing) Added shared folder to own Dropbox
		SharedFolderMount json.RawMessage `json:"shared_folder_mount,omitempty"`
		// SharedFolderNest : (sharing) Changed parent of shared folder
		SharedFolderNest json.RawMessage `json:"shared_folder_nest,omitempty"`
		// SharedFolderTransferOwnership : (sharing) Transferred ownership of
		// shared folder to another member
		SharedFolderTransferOwnership json.RawMessage `json:"shared_folder_transfer_ownership,omitempty"`
		// SharedFolderUnmount : (sharing) Deleted shared folder from Dropbox
		SharedFolderUnmount json.RawMessage `json:"shared_folder_unmount,omitempty"`
		// SharedLinkAddExpiry : (sharing) Added shared link expiration date
		SharedLinkAddExpiry json.RawMessage `json:"shared_link_add_expiry,omitempty"`
		// SharedLinkChangeExpiry : (sharing) Changed shared link expiration
		// date
		SharedLinkChangeExpiry json.RawMessage `json:"shared_link_change_expiry,omitempty"`
		// SharedLinkChangeVisibility : (sharing) Changed visibility of shared
		// link
		SharedLinkChangeVisibility json.RawMessage `json:"shared_link_change_visibility,omitempty"`
		// SharedLinkCopy : (sharing) Added file/folder to Dropbox from shared
		// link
		SharedLinkCopy json.RawMessage `json:"shared_link_copy,omitempty"`
		// SharedLinkCreate : (sharing) Created shared link
		SharedLinkCreate json.RawMessage `json:"shared_link_create,omitempty"`
		// SharedLinkDisable : (sharing) Removed shared link
		SharedLinkDisable json.RawMessage `json:"shared_link_disable,omitempty"`
		// SharedLinkDownload : (sharing) Downloaded file/folder from shared
		// link
		SharedLinkDownload json.RawMessage `json:"shared_link_download,omitempty"`
		// SharedLinkRemoveExpiry : (sharing) Removed shared link expiration
		// date
		SharedLinkRemoveExpiry json.RawMessage `json:"shared_link_remove_expiry,omitempty"`
		// SharedLinkShare : (sharing) Added members as audience of shared link
		SharedLinkShare json.RawMessage `json:"shared_link_share,omitempty"`
		// SharedLinkView : (sharing) Opened shared link
		SharedLinkView json.RawMessage `json:"shared_link_view,omitempty"`
		// SharedNoteOpened : (sharing) Opened shared Paper doc (deprecated, no
		// longer logged)
		SharedNoteOpened json.RawMessage `json:"shared_note_opened,omitempty"`
		// ShmodelGroupShare : (sharing) Shared link with group (deprecated, no
		// longer logged)
		ShmodelGroupShare json.RawMessage `json:"shmodel_group_share,omitempty"`
		// ShowcaseAccessGranted : (showcase) Granted access to showcase
		ShowcaseAccessGranted json.RawMessage `json:"showcase_access_granted,omitempty"`
		// ShowcaseAddMember : (showcase) Added member to showcase
		ShowcaseAddMember json.RawMessage `json:"showcase_add_member,omitempty"`
		// ShowcaseArchived : (showcase) Archived showcase
		ShowcaseArchived json.RawMessage `json:"showcase_archived,omitempty"`
		// ShowcaseCreated : (showcase) Created showcase
		ShowcaseCreated json.RawMessage `json:"showcase_created,omitempty"`
		// ShowcaseDeleteComment : (showcase) Deleted showcase comment
		ShowcaseDeleteComment json.RawMessage `json:"showcase_delete_comment,omitempty"`
		// ShowcaseEdited : (showcase) Edited showcase
		ShowcaseEdited json.RawMessage `json:"showcase_edited,omitempty"`
		// ShowcaseEditComment : (showcase) Edited showcase comment
		ShowcaseEditComment json.RawMessage `json:"showcase_edit_comment,omitempty"`
		// ShowcaseFileAdded : (showcase) Added file to showcase
		ShowcaseFileAdded json.RawMessage `json:"showcase_file_added,omitempty"`
		// ShowcaseFileDownload : (showcase) Downloaded file from showcase
		ShowcaseFileDownload json.RawMessage `json:"showcase_file_download,omitempty"`
		// ShowcaseFileRemoved : (showcase) Removed file from showcase
		ShowcaseFileRemoved json.RawMessage `json:"showcase_file_removed,omitempty"`
		// ShowcaseFileView : (showcase) Viewed file in showcase
		ShowcaseFileView json.RawMessage `json:"showcase_file_view,omitempty"`
		// ShowcasePermanentlyDeleted : (showcase) Permanently deleted showcase
		ShowcasePermanentlyDeleted json.RawMessage `json:"showcase_permanently_deleted,omitempty"`
		// ShowcasePostComment : (showcase) Added showcase comment
		ShowcasePostComment json.RawMessage `json:"showcase_post_comment,omitempty"`
		// ShowcaseRemoveMember : (showcase) Removed member from showcase
		ShowcaseRemoveMember json.RawMessage `json:"showcase_remove_member,omitempty"`
		// ShowcaseRenamed : (showcase) Renamed showcase
		ShowcaseRenamed json.RawMessage `json:"showcase_renamed,omitempty"`
		// ShowcaseRequestAccess : (showcase) Requested access to showcase
		ShowcaseRequestAccess json.RawMessage `json:"showcase_request_access,omitempty"`
		// ShowcaseResolveComment : (showcase) Resolved showcase comment
		ShowcaseResolveComment json.RawMessage `json:"showcase_resolve_comment,omitempty"`
		// ShowcaseRestored : (showcase) Unarchived showcase
		ShowcaseRestored json.RawMessage `json:"showcase_restored,omitempty"`
		// ShowcaseTrashed : (showcase) Deleted showcase
		ShowcaseTrashed json.RawMessage `json:"showcase_trashed,omitempty"`
		// ShowcaseTrashedDeprecated : (showcase) Deleted showcase (old version)
		// (deprecated, replaced by 'Deleted showcase')
		ShowcaseTrashedDeprecated json.RawMessage `json:"showcase_trashed_deprecated,omitempty"`
		// ShowcaseUnresolveComment : (showcase) Unresolved showcase comment
		ShowcaseUnresolveComment json.RawMessage `json:"showcase_unresolve_comment,omitempty"`
		// ShowcaseUntrashed : (showcase) Restored showcase
		ShowcaseUntrashed json.RawMessage `json:"showcase_untrashed,omitempty"`
		// ShowcaseUntrashedDeprecated : (showcase) Restored showcase (old
		// version) (deprecated, replaced by 'Restored showcase')
		ShowcaseUntrashedDeprecated json.RawMessage `json:"showcase_untrashed_deprecated,omitempty"`
		// ShowcaseView : (showcase) Viewed showcase
		ShowcaseView json.RawMessage `json:"showcase_view,omitempty"`
		// SsoAddCert : (sso) Added X.509 certificate for SSO
		SsoAddCert json.RawMessage `json:"sso_add_cert,omitempty"`
		// SsoAddLoginUrl : (sso) Added sign-in URL for SSO
		SsoAddLoginUrl json.RawMessage `json:"sso_add_login_url,omitempty"`
		// SsoAddLogoutUrl : (sso) Added sign-out URL for SSO
		SsoAddLogoutUrl json.RawMessage `json:"sso_add_logout_url,omitempty"`
		// SsoChangeCert : (sso) Changed X.509 certificate for SSO
		SsoChangeCert json.RawMessage `json:"sso_change_cert,omitempty"`
		// SsoChangeLoginUrl : (sso) Changed sign-in URL for SSO
		SsoChangeLoginUrl json.RawMessage `json:"sso_change_login_url,omitempty"`
		// SsoChangeLogoutUrl : (sso) Changed sign-out URL for SSO
		SsoChangeLogoutUrl json.RawMessage `json:"sso_change_logout_url,omitempty"`
		// SsoChangeSamlIdentityMode : (sso) Changed SAML identity mode for SSO
		SsoChangeSamlIdentityMode json.RawMessage `json:"sso_change_saml_identity_mode,omitempty"`
		// SsoRemoveCert : (sso) Removed X.509 certificate for SSO
		SsoRemoveCert json.RawMessage `json:"sso_remove_cert,omitempty"`
		// SsoRemoveLoginUrl : (sso) Removed sign-in URL for SSO
		SsoRemoveLoginUrl json.RawMessage `json:"sso_remove_login_url,omitempty"`
		// SsoRemoveLogoutUrl : (sso) Removed sign-out URL for SSO
		SsoRemoveLogoutUrl json.RawMessage `json:"sso_remove_logout_url,omitempty"`
		// TeamFolderChangeStatus : (team_folders) Changed archival status of
		// team folder
		TeamFolderChangeStatus json.RawMessage `json:"team_folder_change_status,omitempty"`
		// TeamFolderCreate : (team_folders) Created team folder in active
		// status
		TeamFolderCreate json.RawMessage `json:"team_folder_create,omitempty"`
		// TeamFolderDowngrade : (team_folders) Downgraded team folder to
		// regular shared folder
		TeamFolderDowngrade json.RawMessage `json:"team_folder_downgrade,omitempty"`
		// TeamFolderPermanentlyDelete : (team_folders) Permanently deleted
		// archived team folder
		TeamFolderPermanentlyDelete json.RawMessage `json:"team_folder_permanently_delete,omitempty"`
		// TeamFolderRename : (team_folders) Renamed active/archived team folder
		TeamFolderRename json.RawMessage `json:"team_folder_rename,omitempty"`
		// TeamSelectiveSyncSettingsChanged : (team_folders) Changed sync
		// default
		TeamSelectiveSyncSettingsChanged json.RawMessage `json:"team_selective_sync_settings_changed,omitempty"`
		// AccountCaptureChangePolicy : (team_policies) Changed account capture
		// setting on team domain
		AccountCaptureChangePolicy json.RawMessage `json:"account_capture_change_policy,omitempty"`
		// AllowDownloadDisabled : (team_policies) Disabled downloads
		// (deprecated, no longer logged)
		AllowDownloadDisabled json.RawMessage `json:"allow_download_disabled,omitempty"`
		// AllowDownloadEnabled : (team_policies) Enabled downloads (deprecated,
		// no longer logged)
		AllowDownloadEnabled json.RawMessage `json:"allow_download_enabled,omitempty"`
		// DataPlacementRestrictionChangePolicy : (team_policies) Set
		// restrictions on data center locations where team data resides
		DataPlacementRestrictionChangePolicy json.RawMessage `json:"data_placement_restriction_change_policy,omitempty"`
		// DataPlacementRestrictionSatisfyPolicy : (team_policies) Completed
		// restrictions on data center locations where team data resides
		DataPlacementRestrictionSatisfyPolicy json.RawMessage `json:"data_placement_restriction_satisfy_policy,omitempty"`
		// DeviceApprovalsChangeDesktopPolicy : (team_policies) Set/removed
		// limit on number of computers member can link to team Dropbox account
		DeviceApprovalsChangeDesktopPolicy json.RawMessage `json:"device_approvals_change_desktop_policy,omitempty"`
		// DeviceApprovalsChangeMobilePolicy : (team_policies) Set/removed limit
		// on number of mobile devices member can link to team Dropbox account
		DeviceApprovalsChangeMobilePolicy json.RawMessage `json:"device_approvals_change_mobile_policy,omitempty"`
		// DeviceApprovalsChangeOverageAction : (team_policies) Changed device
		// approvals setting when member is over limit
		DeviceApprovalsChangeOverageAction json.RawMessage `json:"device_approvals_change_overage_action,omitempty"`
		// DeviceApprovalsChangeUnlinkAction : (team_policies) Changed device
		// approvals setting when member unlinks approved device
		DeviceApprovalsChangeUnlinkAction json.RawMessage `json:"device_approvals_change_unlink_action,omitempty"`
		// DirectoryRestrictionsAddMembers : (team_policies) Added members to
		// directory restrictions list
		DirectoryRestrictionsAddMembers json.RawMessage `json:"directory_restrictions_add_members,omitempty"`
		// DirectoryRestrictionsRemoveMembers : (team_policies) Removed members
		// from directory restrictions list
		DirectoryRestrictionsRemoveMembers json.RawMessage `json:"directory_restrictions_remove_members,omitempty"`
		// EmmAddException : (team_policies) Added members to EMM exception list
		EmmAddException json.RawMessage `json:"emm_add_exception,omitempty"`
		// EmmChangePolicy : (team_policies) Enabled/disabled enterprise
		// mobility management for members
		EmmChangePolicy json.RawMessage `json:"emm_change_policy,omitempty"`
		// EmmRemoveException : (team_policies) Removed members from EMM
		// exception list
		EmmRemoveException json.RawMessage `json:"emm_remove_exception,omitempty"`
		// ExtendedVersionHistoryChangePolicy : (team_policies) Accepted/opted
		// out of extended version history
		ExtendedVersionHistoryChangePolicy json.RawMessage `json:"extended_version_history_change_policy,omitempty"`
		// FileCommentsChangePolicy : (team_policies) Enabled/disabled
		// commenting on team files
		FileCommentsChangePolicy json.RawMessage `json:"file_comments_change_policy,omitempty"`
		// FileRequestsChangePolicy : (team_policies) Enabled/disabled file
		// requests
		FileRequestsChangePolicy json.RawMessage `json:"file_requests_change_policy,omitempty"`
		// FileRequestsEmailsEnabled : (team_policies) Enabled file request
		// emails for everyone (deprecated, no longer logged)
		FileRequestsEmailsEnabled json.RawMessage `json:"file_requests_emails_enabled,omitempty"`
		// FileRequestsEmailsRestrictedToTeamOnly : (team_policies) Enabled file
		// request emails for team (deprecated, no longer logged)
		FileRequestsEmailsRestrictedToTeamOnly json.RawMessage `json:"file_requests_emails_restricted_to_team_only,omitempty"`
		// GoogleSsoChangePolicy : (team_policies) Enabled/disabled Google
		// single sign-on for team
		GoogleSsoChangePolicy json.RawMessage `json:"google_sso_change_policy,omitempty"`
		// GroupUserManagementChangePolicy : (team_policies) Changed who can
		// create groups
		GroupUserManagementChangePolicy json.RawMessage `json:"group_user_management_change_policy,omitempty"`
		// MemberRequestsChangePolicy : (team_policies) Changed whether users
		// can find team when not invited
		MemberRequestsChangePolicy json.RawMessage `json:"member_requests_change_policy,omitempty"`
		// MemberSpaceLimitsAddException : (team_policies) Added members to
		// member space limit exception list
		MemberSpaceLimitsAddException json.RawMessage `json:"member_space_limits_add_exception,omitempty"`
		// MemberSpaceLimitsChangeCapsTypePolicy : (team_policies) Changed
		// member space limit type for team
		MemberSpaceLimitsChangeCapsTypePolicy json.RawMessage `json:"member_space_limits_change_caps_type_policy,omitempty"`
		// MemberSpaceLimitsChangePolicy : (team_policies) Changed team default
		// member space limit
		MemberSpaceLimitsChangePolicy json.RawMessage `json:"member_space_limits_change_policy,omitempty"`
		// MemberSpaceLimitsRemoveException : (team_policies) Removed members
		// from member space limit exception list
		MemberSpaceLimitsRemoveException json.RawMessage `json:"member_space_limits_remove_exception,omitempty"`
		// MemberSuggestionsChangePolicy : (team_policies) Enabled/disabled
		// option for team members to suggest people to add to team
		MemberSuggestionsChangePolicy json.RawMessage `json:"member_suggestions_change_policy,omitempty"`
		// MicrosoftOfficeAddinChangePolicy : (team_policies) Enabled/disabled
		// Microsoft Office add-in
		MicrosoftOfficeAddinChangePolicy json.RawMessage `json:"microsoft_office_addin_change_policy,omitempty"`
		// NetworkControlChangePolicy : (team_policies) Enabled/disabled network
		// control
		NetworkControlChangePolicy json.RawMessage `json:"network_control_change_policy,omitempty"`
		// PaperChangeDeploymentPolicy : (team_policies) Changed whether Dropbox
		// Paper, when enabled, is deployed to all members or to specific
		// members
		PaperChangeDeploymentPolicy json.RawMessage `json:"paper_change_deployment_policy,omitempty"`
		// PaperChangeMemberLinkPolicy : (team_policies) Changed whether
		// non-members can view Paper docs with link (deprecated, no longer
		// logged)
		PaperChangeMemberLinkPolicy json.RawMessage `json:"paper_change_member_link_policy,omitempty"`
		// PaperChangeMemberPolicy : (team_policies) Changed whether members can
		// share Paper docs outside team, and if docs are accessible only by
		// team members or anyone by default
		PaperChangeMemberPolicy json.RawMessage `json:"paper_change_member_policy,omitempty"`
		// PaperChangePolicy : (team_policies) Enabled/disabled Dropbox Paper
		// for team
		PaperChangePolicy json.RawMessage `json:"paper_change_policy,omitempty"`
		// PaperEnabledUsersGroupAddition : (team_policies) Added users to
		// Paper-enabled users list
		PaperEnabledUsersGroupAddition json.RawMessage `json:"paper_enabled_users_group_addition,omitempty"`
		// PaperEnabledUsersGroupRemoval : (team_policies) Removed users from
		// Paper-enabled users list
		PaperEnabledUsersGroupRemoval json.RawMessage `json:"paper_enabled_users_group_removal,omitempty"`
		// PermanentDeleteChangePolicy : (team_policies) Enabled/disabled
		// ability of team members to permanently delete content
		PermanentDeleteChangePolicy json.RawMessage `json:"permanent_delete_change_policy,omitempty"`
		// SharingChangeFolderJoinPolicy : (team_policies) Changed whether team
		// members can join shared folders owned outside team
		SharingChangeFolderJoinPolicy json.RawMessage `json:"sharing_change_folder_join_policy,omitempty"`
		// SharingChangeLinkPolicy : (team_policies) Changed whether members can
		// share links outside team, and if links are accessible only by team
		// members or anyone by default
		SharingChangeLinkPolicy json.RawMessage `json:"sharing_change_link_policy,omitempty"`
		// SharingChangeMemberPolicy : (team_policies) Changed whether members
		// can share files/folders outside team
		SharingChangeMemberPolicy json.RawMessage `json:"sharing_change_member_policy,omitempty"`
		// ShowcaseChangeDownloadPolicy : (team_policies) Enabled/disabled
		// downloading files from Dropbox Showcase for team
		ShowcaseChangeDownloadPolicy json.RawMessage `json:"showcase_change_download_policy,omitempty"`
		// ShowcaseChangeEnabledPolicy : (team_policies) Enabled/disabled
		// Dropbox Showcase for team
		ShowcaseChangeEnabledPolicy json.RawMessage `json:"showcase_change_enabled_policy,omitempty"`
		// ShowcaseChangeExternalSharingPolicy : (team_policies)
		// Enabled/disabled sharing Dropbox Showcase externally for team
		ShowcaseChangeExternalSharingPolicy json.RawMessage `json:"showcase_change_external_sharing_policy,omitempty"`
		// SmartSyncChangePolicy : (team_policies) Changed default Smart Sync
		// setting for team members
		SmartSyncChangePolicy json.RawMessage `json:"smart_sync_change_policy,omitempty"`
		// SmartSyncNotOptOut : (team_policies) Opted team into Smart Sync
		SmartSyncNotOptOut json.RawMessage `json:"smart_sync_not_opt_out,omitempty"`
		// SmartSyncOptOut : (team_policies) Opted team out of Smart Sync
		SmartSyncOptOut json.RawMessage `json:"smart_sync_opt_out,omitempty"`
		// SsoChangePolicy : (team_policies) Changed single sign-on setting for
		// team
		SsoChangePolicy json.RawMessage `json:"sso_change_policy,omitempty"`
		// TfaChangePolicy : (team_policies) Changed two-step verification
		// setting for team
		TfaChangePolicy json.RawMessage `json:"tfa_change_policy,omitempty"`
		// TwoAccountChangePolicy : (team_policies) Enabled/disabled option for
		// members to link personal Dropbox account and team account to same
		// computer
		TwoAccountChangePolicy json.RawMessage `json:"two_account_change_policy,omitempty"`
		// WebSessionsChangeFixedLengthPolicy : (team_policies) Changed how long
		// members can stay signed in to Dropbox.com
		WebSessionsChangeFixedLengthPolicy json.RawMessage `json:"web_sessions_change_fixed_length_policy,omitempty"`
		// WebSessionsChangeIdleLengthPolicy : (team_policies) Changed how long
		// team members can be idle while signed in to Dropbox.com
		WebSessionsChangeIdleLengthPolicy json.RawMessage `json:"web_sessions_change_idle_length_policy,omitempty"`
		// TeamMergeFrom : (team_profile) Merged another team into this team
		TeamMergeFrom json.RawMessage `json:"team_merge_from,omitempty"`
		// TeamMergeTo : (team_profile) Merged this team into another team
		TeamMergeTo json.RawMessage `json:"team_merge_to,omitempty"`
		// TeamProfileAddLogo : (team_profile) Added team logo to display on
		// shared link headers
		TeamProfileAddLogo json.RawMessage `json:"team_profile_add_logo,omitempty"`
		// TeamProfileChangeDefaultLanguage : (team_profile) Changed default
		// language for team
		TeamProfileChangeDefaultLanguage json.RawMessage `json:"team_profile_change_default_language,omitempty"`
		// TeamProfileChangeLogo : (team_profile) Changed team logo displayed on
		// shared link headers
		TeamProfileChangeLogo json.RawMessage `json:"team_profile_change_logo,omitempty"`
		// TeamProfileChangeName : (team_profile) Changed team name
		TeamProfileChangeName json.RawMessage `json:"team_profile_change_name,omitempty"`
		// TeamProfileRemoveLogo : (team_profile) Removed team logo displayed on
		// shared link headers
		TeamProfileRemoveLogo json.RawMessage `json:"team_profile_remove_logo,omitempty"`
		// TfaAddBackupPhone : (tfa) Added backup phone for two-step
		// verification
		TfaAddBackupPhone json.RawMessage `json:"tfa_add_backup_phone,omitempty"`
		// TfaAddSecurityKey : (tfa) Added security key for two-step
		// verification
		TfaAddSecurityKey json.RawMessage `json:"tfa_add_security_key,omitempty"`
		// TfaChangeBackupPhone : (tfa) Changed backup phone for two-step
		// verification
		TfaChangeBackupPhone json.RawMessage `json:"tfa_change_backup_phone,omitempty"`
		// TfaChangeStatus : (tfa) Enabled/disabled/changed two-step
		// verification setting
		TfaChangeStatus json.RawMessage `json:"tfa_change_status,omitempty"`
		// TfaRemoveBackupPhone : (tfa) Removed backup phone for two-step
		// verification
		TfaRemoveBackupPhone json.RawMessage `json:"tfa_remove_backup_phone,omitempty"`
		// TfaRemoveSecurityKey : (tfa) Removed security key for two-step
		// verification
		TfaRemoveSecurityKey json.RawMessage `json:"tfa_remove_security_key,omitempty"`
		// TfaReset : (tfa) Reset two-step verification for team member
		TfaReset json.RawMessage `json:"tfa_reset,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "app_link_team":
		err = json.Unmarshal(body, &u.AppLinkTeam)

		if err != nil {
			return err
		}
	case "app_link_user":
		err = json.Unmarshal(body, &u.AppLinkUser)

		if err != nil {
			return err
		}
	case "app_unlink_team":
		err = json.Unmarshal(body, &u.AppUnlinkTeam)

		if err != nil {
			return err
		}
	case "app_unlink_user":
		err = json.Unmarshal(body, &u.AppUnlinkUser)

		if err != nil {
			return err
		}
	case "file_add_comment":
		err = json.Unmarshal(body, &u.FileAddComment)

		if err != nil {
			return err
		}
	case "file_change_comment_subscription":
		err = json.Unmarshal(body, &u.FileChangeCommentSubscription)

		if err != nil {
			return err
		}
	case "file_delete_comment":
		err = json.Unmarshal(body, &u.FileDeleteComment)

		if err != nil {
			return err
		}
	case "file_like_comment":
		err = json.Unmarshal(body, &u.FileLikeComment)

		if err != nil {
			return err
		}
	case "file_resolve_comment":
		err = json.Unmarshal(body, &u.FileResolveComment)

		if err != nil {
			return err
		}
	case "file_unlike_comment":
		err = json.Unmarshal(body, &u.FileUnlikeComment)

		if err != nil {
			return err
		}
	case "file_unresolve_comment":
		err = json.Unmarshal(body, &u.FileUnresolveComment)

		if err != nil {
			return err
		}
	case "device_change_ip_desktop":
		err = json.Unmarshal(body, &u.DeviceChangeIpDesktop)

		if err != nil {
			return err
		}
	case "device_change_ip_mobile":
		err = json.Unmarshal(body, &u.DeviceChangeIpMobile)

		if err != nil {
			return err
		}
	case "device_change_ip_web":
		err = json.Unmarshal(body, &u.DeviceChangeIpWeb)

		if err != nil {
			return err
		}
	case "device_delete_on_unlink_fail":
		err = json.Unmarshal(body, &u.DeviceDeleteOnUnlinkFail)

		if err != nil {
			return err
		}
	case "device_delete_on_unlink_success":
		err = json.Unmarshal(body, &u.DeviceDeleteOnUnlinkSuccess)

		if err != nil {
			return err
		}
	case "device_link_fail":
		err = json.Unmarshal(body, &u.DeviceLinkFail)

		if err != nil {
			return err
		}
	case "device_link_success":
		err = json.Unmarshal(body, &u.DeviceLinkSuccess)

		if err != nil {
			return err
		}
	case "device_management_disabled":
		err = json.Unmarshal(body, &u.DeviceManagementDisabled)

		if err != nil {
			return err
		}
	case "device_management_enabled":
		err = json.Unmarshal(body, &u.DeviceManagementEnabled)

		if err != nil {
			return err
		}
	case "device_unlink":
		err = json.Unmarshal(body, &u.DeviceUnlink)

		if err != nil {
			return err
		}
	case "emm_refresh_auth_token":
		err = json.Unmarshal(body, &u.EmmRefreshAuthToken)

		if err != nil {
			return err
		}
	case "account_capture_change_availability":
		err = json.Unmarshal(body, &u.AccountCaptureChangeAvailability)

		if err != nil {
			return err
		}
	case "account_capture_migrate_account":
		err = json.Unmarshal(body, &u.AccountCaptureMigrateAccount)

		if err != nil {
			return err
		}
	case "account_capture_notification_emails_sent":
		err = json.Unmarshal(body, &u.AccountCaptureNotificationEmailsSent)

		if err != nil {
			return err
		}
	case "account_capture_relinquish_account":
		err = json.Unmarshal(body, &u.AccountCaptureRelinquishAccount)

		if err != nil {
			return err
		}
	case "disabled_domain_invites":
		err = json.Unmarshal(body, &u.DisabledDomainInvites)

		if err != nil {
			return err
		}
	case "domain_invites_approve_request_to_join_team":
		err = json.Unmarshal(body, &u.DomainInvitesApproveRequestToJoinTeam)

		if err != nil {
			return err
		}
	case "domain_invites_decline_request_to_join_team":
		err = json.Unmarshal(body, &u.DomainInvitesDeclineRequestToJoinTeam)

		if err != nil {
			return err
		}
	case "domain_invites_email_existing_users":
		err = json.Unmarshal(body, &u.DomainInvitesEmailExistingUsers)

		if err != nil {
			return err
		}
	case "domain_invites_request_to_join_team":
		err = json.Unmarshal(body, &u.DomainInvitesRequestToJoinTeam)

		if err != nil {
			return err
		}
	case "domain_invites_set_invite_new_user_pref_to_no":
		err = json.Unmarshal(body, &u.DomainInvitesSetInviteNewUserPrefToNo)

		if err != nil {
			return err
		}
	case "domain_invites_set_invite_new_user_pref_to_yes":
		err = json.Unmarshal(body, &u.DomainInvitesSetInviteNewUserPrefToYes)

		if err != nil {
			return err
		}
	case "domain_verification_add_domain_fail":
		err = json.Unmarshal(body, &u.DomainVerificationAddDomainFail)

		if err != nil {
			return err
		}
	case "domain_verification_add_domain_success":
		err = json.Unmarshal(body, &u.DomainVerificationAddDomainSuccess)

		if err != nil {
			return err
		}
	case "domain_verification_remove_domain":
		err = json.Unmarshal(body, &u.DomainVerificationRemoveDomain)

		if err != nil {
			return err
		}
	case "enabled_domain_invites":
		err = json.Unmarshal(body, &u.EnabledDomainInvites)

		if err != nil {
			return err
		}
	case "create_folder":
		err = json.Unmarshal(body, &u.CreateFolder)

		if err != nil {
			return err
		}
	case "file_add":
		err = json.Unmarshal(body, &u.FileAdd)

		if err != nil {
			return err
		}
	case "file_copy":
		err = json.Unmarshal(body, &u.FileCopy)

		if err != nil {
			return err
		}
	case "file_delete":
		err = json.Unmarshal(body, &u.FileDelete)

		if err != nil {
			return err
		}
	case "file_download":
		err = json.Unmarshal(body, &u.FileDownload)

		if err != nil {
			return err
		}
	case "file_edit":
		err = json.Unmarshal(body, &u.FileEdit)

		if err != nil {
			return err
		}
	case "file_get_copy_reference":
		err = json.Unmarshal(body, &u.FileGetCopyReference)

		if err != nil {
			return err
		}
	case "file_move":
		err = json.Unmarshal(body, &u.FileMove)

		if err != nil {
			return err
		}
	case "file_permanently_delete":
		err = json.Unmarshal(body, &u.FilePermanentlyDelete)

		if err != nil {
			return err
		}
	case "file_preview":
		err = json.Unmarshal(body, &u.FilePreview)

		if err != nil {
			return err
		}
	case "file_rename":
		err = json.Unmarshal(body, &u.FileRename)

		if err != nil {
			return err
		}
	case "file_restore":
		err = json.Unmarshal(body, &u.FileRestore)

		if err != nil {
			return err
		}
	case "file_revert":
		err = json.Unmarshal(body, &u.FileRevert)

		if err != nil {
			return err
		}
	case "file_rollback_changes":
		err = json.Unmarshal(body, &u.FileRollbackChanges)

		if err != nil {
			return err
		}
	case "file_save_copy_reference":
		err = json.Unmarshal(body, &u.FileSaveCopyReference)

		if err != nil {
			return err
		}
	case "file_request_change":
		err = json.Unmarshal(body, &u.FileRequestChange)

		if err != nil {
			return err
		}
	case "file_request_close":
		err = json.Unmarshal(body, &u.FileRequestClose)

		if err != nil {
			return err
		}
	case "file_request_create":
		err = json.Unmarshal(body, &u.FileRequestCreate)

		if err != nil {
			return err
		}
	case "file_request_receive_file":
		err = json.Unmarshal(body, &u.FileRequestReceiveFile)

		if err != nil {
			return err
		}
	case "group_add_external_id":
		err = json.Unmarshal(body, &u.GroupAddExternalId)

		if err != nil {
			return err
		}
	case "group_add_member":
		err = json.Unmarshal(body, &u.GroupAddMember)

		if err != nil {
			return err
		}
	case "group_change_external_id":
		err = json.Unmarshal(body, &u.GroupChangeExternalId)

		if err != nil {
			return err
		}
	case "group_change_management_type":
		err = json.Unmarshal(body, &u.GroupChangeManagementType)

		if err != nil {
			return err
		}
	case "group_change_member_role":
		err = json.Unmarshal(body, &u.GroupChangeMemberRole)

		if err != nil {
			return err
		}
	case "group_create":
		err = json.Unmarshal(body, &u.GroupCreate)

		if err != nil {
			return err
		}
	case "group_delete":
		err = json.Unmarshal(body, &u.GroupDelete)

		if err != nil {
			return err
		}
	case "group_description_updated":
		err = json.Unmarshal(body, &u.GroupDescriptionUpdated)

		if err != nil {
			return err
		}
	case "group_join_policy_updated":
		err = json.Unmarshal(body, &u.GroupJoinPolicyUpdated)

		if err != nil {
			return err
		}
	case "group_moved":
		err = json.Unmarshal(body, &u.GroupMoved)

		if err != nil {
			return err
		}
	case "group_remove_external_id":
		err = json.Unmarshal(body, &u.GroupRemoveExternalId)

		if err != nil {
			return err
		}
	case "group_remove_member":
		err = json.Unmarshal(body, &u.GroupRemoveMember)

		if err != nil {
			return err
		}
	case "group_rename":
		err = json.Unmarshal(body, &u.GroupRename)

		if err != nil {
			return err
		}
	case "emm_error":
		err = json.Unmarshal(body, &u.EmmError)

		if err != nil {
			return err
		}
	case "login_fail":
		err = json.Unmarshal(body, &u.LoginFail)

		if err != nil {
			return err
		}
	case "login_success":
		err = json.Unmarshal(body, &u.LoginSuccess)

		if err != nil {
			return err
		}
	case "logout":
		err = json.Unmarshal(body, &u.Logout)

		if err != nil {
			return err
		}
	case "reseller_support_session_end":
		err = json.Unmarshal(body, &u.ResellerSupportSessionEnd)

		if err != nil {
			return err
		}
	case "reseller_support_session_start":
		err = json.Unmarshal(body, &u.ResellerSupportSessionStart)

		if err != nil {
			return err
		}
	case "sign_in_as_session_end":
		err = json.Unmarshal(body, &u.SignInAsSessionEnd)

		if err != nil {
			return err
		}
	case "sign_in_as_session_start":
		err = json.Unmarshal(body, &u.SignInAsSessionStart)

		if err != nil {
			return err
		}
	case "sso_error":
		err = json.Unmarshal(body, &u.SsoError)

		if err != nil {
			return err
		}
	case "member_add_name":
		err = json.Unmarshal(body, &u.MemberAddName)

		if err != nil {
			return err
		}
	case "member_change_admin_role":
		err = json.Unmarshal(body, &u.MemberChangeAdminRole)

		if err != nil {
			return err
		}
	case "member_change_email":
		err = json.Unmarshal(body, &u.MemberChangeEmail)

		if err != nil {
			return err
		}
	case "member_change_membership_type":
		err = json.Unmarshal(body, &u.MemberChangeMembershipType)

		if err != nil {
			return err
		}
	case "member_change_name":
		err = json.Unmarshal(body, &u.MemberChangeName)

		if err != nil {
			return err
		}
	case "member_change_status":
		err = json.Unmarshal(body, &u.MemberChangeStatus)

		if err != nil {
			return err
		}
	case "member_permanently_delete_account_contents":
		err = json.Unmarshal(body, &u.MemberPermanentlyDeleteAccountContents)

		if err != nil {
			return err
		}
	case "member_space_limits_add_custom_quota":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsAddCustomQuota)

		if err != nil {
			return err
		}
	case "member_space_limits_change_custom_quota":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeCustomQuota)

		if err != nil {
			return err
		}
	case "member_space_limits_change_status":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeStatus)

		if err != nil {
			return err
		}
	case "member_space_limits_remove_custom_quota":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsRemoveCustomQuota)

		if err != nil {
			return err
		}
	case "member_suggest":
		err = json.Unmarshal(body, &u.MemberSuggest)

		if err != nil {
			return err
		}
	case "member_transfer_account_contents":
		err = json.Unmarshal(body, &u.MemberTransferAccountContents)

		if err != nil {
			return err
		}
	case "secondary_mails_policy_changed":
		err = json.Unmarshal(body, &u.SecondaryMailsPolicyChanged)

		if err != nil {
			return err
		}
	case "paper_content_add_member":
		err = json.Unmarshal(body, &u.PaperContentAddMember)

		if err != nil {
			return err
		}
	case "paper_content_add_to_folder":
		err = json.Unmarshal(body, &u.PaperContentAddToFolder)

		if err != nil {
			return err
		}
	case "paper_content_archive":
		err = json.Unmarshal(body, &u.PaperContentArchive)

		if err != nil {
			return err
		}
	case "paper_content_create":
		err = json.Unmarshal(body, &u.PaperContentCreate)

		if err != nil {
			return err
		}
	case "paper_content_permanently_delete":
		err = json.Unmarshal(body, &u.PaperContentPermanentlyDelete)

		if err != nil {
			return err
		}
	case "paper_content_remove_from_folder":
		err = json.Unmarshal(body, &u.PaperContentRemoveFromFolder)

		if err != nil {
			return err
		}
	case "paper_content_remove_member":
		err = json.Unmarshal(body, &u.PaperContentRemoveMember)

		if err != nil {
			return err
		}
	case "paper_content_rename":
		err = json.Unmarshal(body, &u.PaperContentRename)

		if err != nil {
			return err
		}
	case "paper_content_restore":
		err = json.Unmarshal(body, &u.PaperContentRestore)

		if err != nil {
			return err
		}
	case "paper_doc_add_comment":
		err = json.Unmarshal(body, &u.PaperDocAddComment)

		if err != nil {
			return err
		}
	case "paper_doc_change_member_role":
		err = json.Unmarshal(body, &u.PaperDocChangeMemberRole)

		if err != nil {
			return err
		}
	case "paper_doc_change_sharing_policy":
		err = json.Unmarshal(body, &u.PaperDocChangeSharingPolicy)

		if err != nil {
			return err
		}
	case "paper_doc_change_subscription":
		err = json.Unmarshal(body, &u.PaperDocChangeSubscription)

		if err != nil {
			return err
		}
	case "paper_doc_deleted":
		err = json.Unmarshal(body, &u.PaperDocDeleted)

		if err != nil {
			return err
		}
	case "paper_doc_delete_comment":
		err = json.Unmarshal(body, &u.PaperDocDeleteComment)

		if err != nil {
			return err
		}
	case "paper_doc_download":
		err = json.Unmarshal(body, &u.PaperDocDownload)

		if err != nil {
			return err
		}
	case "paper_doc_edit":
		err = json.Unmarshal(body, &u.PaperDocEdit)

		if err != nil {
			return err
		}
	case "paper_doc_edit_comment":
		err = json.Unmarshal(body, &u.PaperDocEditComment)

		if err != nil {
			return err
		}
	case "paper_doc_followed":
		err = json.Unmarshal(body, &u.PaperDocFollowed)

		if err != nil {
			return err
		}
	case "paper_doc_mention":
		err = json.Unmarshal(body, &u.PaperDocMention)

		if err != nil {
			return err
		}
	case "paper_doc_request_access":
		err = json.Unmarshal(body, &u.PaperDocRequestAccess)

		if err != nil {
			return err
		}
	case "paper_doc_resolve_comment":
		err = json.Unmarshal(body, &u.PaperDocResolveComment)

		if err != nil {
			return err
		}
	case "paper_doc_revert":
		err = json.Unmarshal(body, &u.PaperDocRevert)

		if err != nil {
			return err
		}
	case "paper_doc_slack_share":
		err = json.Unmarshal(body, &u.PaperDocSlackShare)

		if err != nil {
			return err
		}
	case "paper_doc_team_invite":
		err = json.Unmarshal(body, &u.PaperDocTeamInvite)

		if err != nil {
			return err
		}
	case "paper_doc_trashed":
		err = json.Unmarshal(body, &u.PaperDocTrashed)

		if err != nil {
			return err
		}
	case "paper_doc_unresolve_comment":
		err = json.Unmarshal(body, &u.PaperDocUnresolveComment)

		if err != nil {
			return err
		}
	case "paper_doc_untrashed":
		err = json.Unmarshal(body, &u.PaperDocUntrashed)

		if err != nil {
			return err
		}
	case "paper_doc_view":
		err = json.Unmarshal(body, &u.PaperDocView)

		if err != nil {
			return err
		}
	case "paper_external_view_allow":
		err = json.Unmarshal(body, &u.PaperExternalViewAllow)

		if err != nil {
			return err
		}
	case "paper_external_view_default_team":
		err = json.Unmarshal(body, &u.PaperExternalViewDefaultTeam)

		if err != nil {
			return err
		}
	case "paper_external_view_forbid":
		err = json.Unmarshal(body, &u.PaperExternalViewForbid)

		if err != nil {
			return err
		}
	case "paper_folder_change_subscription":
		err = json.Unmarshal(body, &u.PaperFolderChangeSubscription)

		if err != nil {
			return err
		}
	case "paper_folder_deleted":
		err = json.Unmarshal(body, &u.PaperFolderDeleted)

		if err != nil {
			return err
		}
	case "paper_folder_followed":
		err = json.Unmarshal(body, &u.PaperFolderFollowed)

		if err != nil {
			return err
		}
	case "paper_folder_team_invite":
		err = json.Unmarshal(body, &u.PaperFolderTeamInvite)

		if err != nil {
			return err
		}
	case "password_change":
		err = json.Unmarshal(body, &u.PasswordChange)

		if err != nil {
			return err
		}
	case "password_reset":
		err = json.Unmarshal(body, &u.PasswordReset)

		if err != nil {
			return err
		}
	case "password_reset_all":
		err = json.Unmarshal(body, &u.PasswordResetAll)

		if err != nil {
			return err
		}
	case "emm_create_exceptions_report":
		err = json.Unmarshal(body, &u.EmmCreateExceptionsReport)

		if err != nil {
			return err
		}
	case "emm_create_usage_report":
		err = json.Unmarshal(body, &u.EmmCreateUsageReport)

		if err != nil {
			return err
		}
	case "export_members_report":
		err = json.Unmarshal(body, &u.ExportMembersReport)

		if err != nil {
			return err
		}
	case "paper_admin_export_start":
		err = json.Unmarshal(body, &u.PaperAdminExportStart)

		if err != nil {
			return err
		}
	case "smart_sync_create_admin_privilege_report":
		err = json.Unmarshal(body, &u.SmartSyncCreateAdminPrivilegeReport)

		if err != nil {
			return err
		}
	case "team_activity_create_report":
		err = json.Unmarshal(body, &u.TeamActivityCreateReport)

		if err != nil {
			return err
		}
	case "collection_share":
		err = json.Unmarshal(body, &u.CollectionShare)

		if err != nil {
			return err
		}
	case "note_acl_invite_only":
		err = json.Unmarshal(body, &u.NoteAclInviteOnly)

		if err != nil {
			return err
		}
	case "note_acl_link":
		err = json.Unmarshal(body, &u.NoteAclLink)

		if err != nil {
			return err
		}
	case "note_acl_team_link":
		err = json.Unmarshal(body, &u.NoteAclTeamLink)

		if err != nil {
			return err
		}
	case "note_shared":
		err = json.Unmarshal(body, &u.NoteShared)

		if err != nil {
			return err
		}
	case "note_share_receive":
		err = json.Unmarshal(body, &u.NoteShareReceive)

		if err != nil {
			return err
		}
	case "open_note_shared":
		err = json.Unmarshal(body, &u.OpenNoteShared)

		if err != nil {
			return err
		}
	case "sf_add_group":
		err = json.Unmarshal(body, &u.SfAddGroup)

		if err != nil {
			return err
		}
	case "sf_allow_non_members_to_view_shared_links":
		err = json.Unmarshal(body, &u.SfAllowNonMembersToViewSharedLinks)

		if err != nil {
			return err
		}
	case "sf_external_invite_warn":
		err = json.Unmarshal(body, &u.SfExternalInviteWarn)

		if err != nil {
			return err
		}
	case "sf_fb_invite":
		err = json.Unmarshal(body, &u.SfFbInvite)

		if err != nil {
			return err
		}
	case "sf_fb_invite_change_role":
		err = json.Unmarshal(body, &u.SfFbInviteChangeRole)

		if err != nil {
			return err
		}
	case "sf_fb_uninvite":
		err = json.Unmarshal(body, &u.SfFbUninvite)

		if err != nil {
			return err
		}
	case "sf_invite_group":
		err = json.Unmarshal(body, &u.SfInviteGroup)

		if err != nil {
			return err
		}
	case "sf_team_grant_access":
		err = json.Unmarshal(body, &u.SfTeamGrantAccess)

		if err != nil {
			return err
		}
	case "sf_team_invite":
		err = json.Unmarshal(body, &u.SfTeamInvite)

		if err != nil {
			return err
		}
	case "sf_team_invite_change_role":
		err = json.Unmarshal(body, &u.SfTeamInviteChangeRole)

		if err != nil {
			return err
		}
	case "sf_team_join":
		err = json.Unmarshal(body, &u.SfTeamJoin)

		if err != nil {
			return err
		}
	case "sf_team_join_from_oob_link":
		err = json.Unmarshal(body, &u.SfTeamJoinFromOobLink)

		if err != nil {
			return err
		}
	case "sf_team_uninvite":
		err = json.Unmarshal(body, &u.SfTeamUninvite)

		if err != nil {
			return err
		}
	case "shared_content_add_invitees":
		err = json.Unmarshal(body, &u.SharedContentAddInvitees)

		if err != nil {
			return err
		}
	case "shared_content_add_link_expiry":
		err = json.Unmarshal(body, &u.SharedContentAddLinkExpiry)

		if err != nil {
			return err
		}
	case "shared_content_add_link_password":
		err = json.Unmarshal(body, &u.SharedContentAddLinkPassword)

		if err != nil {
			return err
		}
	case "shared_content_add_member":
		err = json.Unmarshal(body, &u.SharedContentAddMember)

		if err != nil {
			return err
		}
	case "shared_content_change_downloads_policy":
		err = json.Unmarshal(body, &u.SharedContentChangeDownloadsPolicy)

		if err != nil {
			return err
		}
	case "shared_content_change_invitee_role":
		err = json.Unmarshal(body, &u.SharedContentChangeInviteeRole)

		if err != nil {
			return err
		}
	case "shared_content_change_link_audience":
		err = json.Unmarshal(body, &u.SharedContentChangeLinkAudience)

		if err != nil {
			return err
		}
	case "shared_content_change_link_expiry":
		err = json.Unmarshal(body, &u.SharedContentChangeLinkExpiry)

		if err != nil {
			return err
		}
	case "shared_content_change_link_password":
		err = json.Unmarshal(body, &u.SharedContentChangeLinkPassword)

		if err != nil {
			return err
		}
	case "shared_content_change_member_role":
		err = json.Unmarshal(body, &u.SharedContentChangeMemberRole)

		if err != nil {
			return err
		}
	case "shared_content_change_viewer_info_policy":
		err = json.Unmarshal(body, &u.SharedContentChangeViewerInfoPolicy)

		if err != nil {
			return err
		}
	case "shared_content_claim_invitation":
		err = json.Unmarshal(body, &u.SharedContentClaimInvitation)

		if err != nil {
			return err
		}
	case "shared_content_copy":
		err = json.Unmarshal(body, &u.SharedContentCopy)

		if err != nil {
			return err
		}
	case "shared_content_download":
		err = json.Unmarshal(body, &u.SharedContentDownload)

		if err != nil {
			return err
		}
	case "shared_content_relinquish_membership":
		err = json.Unmarshal(body, &u.SharedContentRelinquishMembership)

		if err != nil {
			return err
		}
	case "shared_content_remove_invitees":
		err = json.Unmarshal(body, &u.SharedContentRemoveInvitees)

		if err != nil {
			return err
		}
	case "shared_content_remove_link_expiry":
		err = json.Unmarshal(body, &u.SharedContentRemoveLinkExpiry)

		if err != nil {
			return err
		}
	case "shared_content_remove_link_password":
		err = json.Unmarshal(body, &u.SharedContentRemoveLinkPassword)

		if err != nil {
			return err
		}
	case "shared_content_remove_member":
		err = json.Unmarshal(body, &u.SharedContentRemoveMember)

		if err != nil {
			return err
		}
	case "shared_content_request_access":
		err = json.Unmarshal(body, &u.SharedContentRequestAccess)

		if err != nil {
			return err
		}
	case "shared_content_unshare":
		err = json.Unmarshal(body, &u.SharedContentUnshare)

		if err != nil {
			return err
		}
	case "shared_content_view":
		err = json.Unmarshal(body, &u.SharedContentView)

		if err != nil {
			return err
		}
	case "shared_folder_change_link_policy":
		err = json.Unmarshal(body, &u.SharedFolderChangeLinkPolicy)

		if err != nil {
			return err
		}
	case "shared_folder_change_members_inheritance_policy":
		err = json.Unmarshal(body, &u.SharedFolderChangeMembersInheritancePolicy)

		if err != nil {
			return err
		}
	case "shared_folder_change_members_management_policy":
		err = json.Unmarshal(body, &u.SharedFolderChangeMembersManagementPolicy)

		if err != nil {
			return err
		}
	case "shared_folder_change_members_policy":
		err = json.Unmarshal(body, &u.SharedFolderChangeMembersPolicy)

		if err != nil {
			return err
		}
	case "shared_folder_create":
		err = json.Unmarshal(body, &u.SharedFolderCreate)

		if err != nil {
			return err
		}
	case "shared_folder_decline_invitation":
		err = json.Unmarshal(body, &u.SharedFolderDeclineInvitation)

		if err != nil {
			return err
		}
	case "shared_folder_mount":
		err = json.Unmarshal(body, &u.SharedFolderMount)

		if err != nil {
			return err
		}
	case "shared_folder_nest":
		err = json.Unmarshal(body, &u.SharedFolderNest)

		if err != nil {
			return err
		}
	case "shared_folder_transfer_ownership":
		err = json.Unmarshal(body, &u.SharedFolderTransferOwnership)

		if err != nil {
			return err
		}
	case "shared_folder_unmount":
		err = json.Unmarshal(body, &u.SharedFolderUnmount)

		if err != nil {
			return err
		}
	case "shared_link_add_expiry":
		err = json.Unmarshal(body, &u.SharedLinkAddExpiry)

		if err != nil {
			return err
		}
	case "shared_link_change_expiry":
		err = json.Unmarshal(body, &u.SharedLinkChangeExpiry)

		if err != nil {
			return err
		}
	case "shared_link_change_visibility":
		err = json.Unmarshal(body, &u.SharedLinkChangeVisibility)

		if err != nil {
			return err
		}
	case "shared_link_copy":
		err = json.Unmarshal(body, &u.SharedLinkCopy)

		if err != nil {
			return err
		}
	case "shared_link_create":
		err = json.Unmarshal(body, &u.SharedLinkCreate)

		if err != nil {
			return err
		}
	case "shared_link_disable":
		err = json.Unmarshal(body, &u.SharedLinkDisable)

		if err != nil {
			return err
		}
	case "shared_link_download":
		err = json.Unmarshal(body, &u.SharedLinkDownload)

		if err != nil {
			return err
		}
	case "shared_link_remove_expiry":
		err = json.Unmarshal(body, &u.SharedLinkRemoveExpiry)

		if err != nil {
			return err
		}
	case "shared_link_share":
		err = json.Unmarshal(body, &u.SharedLinkShare)

		if err != nil {
			return err
		}
	case "shared_link_view":
		err = json.Unmarshal(body, &u.SharedLinkView)

		if err != nil {
			return err
		}
	case "shared_note_opened":
		err = json.Unmarshal(body, &u.SharedNoteOpened)

		if err != nil {
			return err
		}
	case "shmodel_group_share":
		err = json.Unmarshal(body, &u.ShmodelGroupShare)

		if err != nil {
			return err
		}
	case "showcase_access_granted":
		err = json.Unmarshal(body, &u.ShowcaseAccessGranted)

		if err != nil {
			return err
		}
	case "showcase_add_member":
		err = json.Unmarshal(body, &u.ShowcaseAddMember)

		if err != nil {
			return err
		}
	case "showcase_archived":
		err = json.Unmarshal(body, &u.ShowcaseArchived)

		if err != nil {
			return err
		}
	case "showcase_created":
		err = json.Unmarshal(body, &u.ShowcaseCreated)

		if err != nil {
			return err
		}
	case "showcase_delete_comment":
		err = json.Unmarshal(body, &u.ShowcaseDeleteComment)

		if err != nil {
			return err
		}
	case "showcase_edited":
		err = json.Unmarshal(body, &u.ShowcaseEdited)

		if err != nil {
			return err
		}
	case "showcase_edit_comment":
		err = json.Unmarshal(body, &u.ShowcaseEditComment)

		if err != nil {
			return err
		}
	case "showcase_file_added":
		err = json.Unmarshal(body, &u.ShowcaseFileAdded)

		if err != nil {
			return err
		}
	case "showcase_file_download":
		err = json.Unmarshal(body, &u.ShowcaseFileDownload)

		if err != nil {
			return err
		}
	case "showcase_file_removed":
		err = json.Unmarshal(body, &u.ShowcaseFileRemoved)

		if err != nil {
			return err
		}
	case "showcase_file_view":
		err = json.Unmarshal(body, &u.ShowcaseFileView)

		if err != nil {
			return err
		}
	case "showcase_permanently_deleted":
		err = json.Unmarshal(body, &u.ShowcasePermanentlyDeleted)

		if err != nil {
			return err
		}
	case "showcase_post_comment":
		err = json.Unmarshal(body, &u.ShowcasePostComment)

		if err != nil {
			return err
		}
	case "showcase_remove_member":
		err = json.Unmarshal(body, &u.ShowcaseRemoveMember)

		if err != nil {
			return err
		}
	case "showcase_renamed":
		err = json.Unmarshal(body, &u.ShowcaseRenamed)

		if err != nil {
			return err
		}
	case "showcase_request_access":
		err = json.Unmarshal(body, &u.ShowcaseRequestAccess)

		if err != nil {
			return err
		}
	case "showcase_resolve_comment":
		err = json.Unmarshal(body, &u.ShowcaseResolveComment)

		if err != nil {
			return err
		}
	case "showcase_restored":
		err = json.Unmarshal(body, &u.ShowcaseRestored)

		if err != nil {
			return err
		}
	case "showcase_trashed":
		err = json.Unmarshal(body, &u.ShowcaseTrashed)

		if err != nil {
			return err
		}
	case "showcase_trashed_deprecated":
		err = json.Unmarshal(body, &u.ShowcaseTrashedDeprecated)

		if err != nil {
			return err
		}
	case "showcase_unresolve_comment":
		err = json.Unmarshal(body, &u.ShowcaseUnresolveComment)

		if err != nil {
			return err
		}
	case "showcase_untrashed":
		err = json.Unmarshal(body, &u.ShowcaseUntrashed)

		if err != nil {
			return err
		}
	case "showcase_untrashed_deprecated":
		err = json.Unmarshal(body, &u.ShowcaseUntrashedDeprecated)

		if err != nil {
			return err
		}
	case "showcase_view":
		err = json.Unmarshal(body, &u.ShowcaseView)

		if err != nil {
			return err
		}
	case "sso_add_cert":
		err = json.Unmarshal(body, &u.SsoAddCert)

		if err != nil {
			return err
		}
	case "sso_add_login_url":
		err = json.Unmarshal(body, &u.SsoAddLoginUrl)

		if err != nil {
			return err
		}
	case "sso_add_logout_url":
		err = json.Unmarshal(body, &u.SsoAddLogoutUrl)

		if err != nil {
			return err
		}
	case "sso_change_cert":
		err = json.Unmarshal(body, &u.SsoChangeCert)

		if err != nil {
			return err
		}
	case "sso_change_login_url":
		err = json.Unmarshal(body, &u.SsoChangeLoginUrl)

		if err != nil {
			return err
		}
	case "sso_change_logout_url":
		err = json.Unmarshal(body, &u.SsoChangeLogoutUrl)

		if err != nil {
			return err
		}
	case "sso_change_saml_identity_mode":
		err = json.Unmarshal(body, &u.SsoChangeSamlIdentityMode)

		if err != nil {
			return err
		}
	case "sso_remove_cert":
		err = json.Unmarshal(body, &u.SsoRemoveCert)

		if err != nil {
			return err
		}
	case "sso_remove_login_url":
		err = json.Unmarshal(body, &u.SsoRemoveLoginUrl)

		if err != nil {
			return err
		}
	case "sso_remove_logout_url":
		err = json.Unmarshal(body, &u.SsoRemoveLogoutUrl)

		if err != nil {
			return err
		}
	case "team_folder_change_status":
		err = json.Unmarshal(body, &u.TeamFolderChangeStatus)

		if err != nil {
			return err
		}
	case "team_folder_create":
		err = json.Unmarshal(body, &u.TeamFolderCreate)

		if err != nil {
			return err
		}
	case "team_folder_downgrade":
		err = json.Unmarshal(body, &u.TeamFolderDowngrade)

		if err != nil {
			return err
		}
	case "team_folder_permanently_delete":
		err = json.Unmarshal(body, &u.TeamFolderPermanentlyDelete)

		if err != nil {
			return err
		}
	case "team_folder_rename":
		err = json.Unmarshal(body, &u.TeamFolderRename)

		if err != nil {
			return err
		}
	case "team_selective_sync_settings_changed":
		err = json.Unmarshal(body, &u.TeamSelectiveSyncSettingsChanged)

		if err != nil {
			return err
		}
	case "account_capture_change_policy":
		err = json.Unmarshal(body, &u.AccountCaptureChangePolicy)

		if err != nil {
			return err
		}
	case "allow_download_disabled":
		err = json.Unmarshal(body, &u.AllowDownloadDisabled)

		if err != nil {
			return err
		}
	case "allow_download_enabled":
		err = json.Unmarshal(body, &u.AllowDownloadEnabled)

		if err != nil {
			return err
		}
	case "data_placement_restriction_change_policy":
		err = json.Unmarshal(body, &u.DataPlacementRestrictionChangePolicy)

		if err != nil {
			return err
		}
	case "data_placement_restriction_satisfy_policy":
		err = json.Unmarshal(body, &u.DataPlacementRestrictionSatisfyPolicy)

		if err != nil {
			return err
		}
	case "device_approvals_change_desktop_policy":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeDesktopPolicy)

		if err != nil {
			return err
		}
	case "device_approvals_change_mobile_policy":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeMobilePolicy)

		if err != nil {
			return err
		}
	case "device_approvals_change_overage_action":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeOverageAction)

		if err != nil {
			return err
		}
	case "device_approvals_change_unlink_action":
		err = json.Unmarshal(body, &u.DeviceApprovalsChangeUnlinkAction)

		if err != nil {
			return err
		}
	case "directory_restrictions_add_members":
		err = json.Unmarshal(body, &u.DirectoryRestrictionsAddMembers)

		if err != nil {
			return err
		}
	case "directory_restrictions_remove_members":
		err = json.Unmarshal(body, &u.DirectoryRestrictionsRemoveMembers)

		if err != nil {
			return err
		}
	case "emm_add_exception":
		err = json.Unmarshal(body, &u.EmmAddException)

		if err != nil {
			return err
		}
	case "emm_change_policy":
		err = json.Unmarshal(body, &u.EmmChangePolicy)

		if err != nil {
			return err
		}
	case "emm_remove_exception":
		err = json.Unmarshal(body, &u.EmmRemoveException)

		if err != nil {
			return err
		}
	case "extended_version_history_change_policy":
		err = json.Unmarshal(body, &u.ExtendedVersionHistoryChangePolicy)

		if err != nil {
			return err
		}
	case "file_comments_change_policy":
		err = json.Unmarshal(body, &u.FileCommentsChangePolicy)

		if err != nil {
			return err
		}
	case "file_requests_change_policy":
		err = json.Unmarshal(body, &u.FileRequestsChangePolicy)

		if err != nil {
			return err
		}
	case "file_requests_emails_enabled":
		err = json.Unmarshal(body, &u.FileRequestsEmailsEnabled)

		if err != nil {
			return err
		}
	case "file_requests_emails_restricted_to_team_only":
		err = json.Unmarshal(body, &u.FileRequestsEmailsRestrictedToTeamOnly)

		if err != nil {
			return err
		}
	case "google_sso_change_policy":
		err = json.Unmarshal(body, &u.GoogleSsoChangePolicy)

		if err != nil {
			return err
		}
	case "group_user_management_change_policy":
		err = json.Unmarshal(body, &u.GroupUserManagementChangePolicy)

		if err != nil {
			return err
		}
	case "member_requests_change_policy":
		err = json.Unmarshal(body, &u.MemberRequestsChangePolicy)

		if err != nil {
			return err
		}
	case "member_space_limits_add_exception":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsAddException)

		if err != nil {
			return err
		}
	case "member_space_limits_change_caps_type_policy":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangeCapsTypePolicy)

		if err != nil {
			return err
		}
	case "member_space_limits_change_policy":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsChangePolicy)

		if err != nil {
			return err
		}
	case "member_space_limits_remove_exception":
		err = json.Unmarshal(body, &u.MemberSpaceLimitsRemoveException)

		if err != nil {
			return err
		}
	case "member_suggestions_change_policy":
		err = json.Unmarshal(body, &u.MemberSuggestionsChangePolicy)

		if err != nil {
			return err
		}
	case "microsoft_office_addin_change_policy":
		err = json.Unmarshal(body, &u.MicrosoftOfficeAddinChangePolicy)

		if err != nil {
			return err
		}
	case "network_control_change_policy":
		err = json.Unmarshal(body, &u.NetworkControlChangePolicy)

		if err != nil {
			return err
		}
	case "paper_change_deployment_policy":
		err = json.Unmarshal(body, &u.PaperChangeDeploymentPolicy)

		if err != nil {
			return err
		}
	case "paper_change_member_link_policy":
		err = json.Unmarshal(body, &u.PaperChangeMemberLinkPolicy)

		if err != nil {
			return err
		}
	case "paper_change_member_policy":
		err = json.Unmarshal(body, &u.PaperChangeMemberPolicy)

		if err != nil {
			return err
		}
	case "paper_change_policy":
		err = json.Unmarshal(body, &u.PaperChangePolicy)

		if err != nil {
			return err
		}
	case "paper_enabled_users_group_addition":
		err = json.Unmarshal(body, &u.PaperEnabledUsersGroupAddition)

		if err != nil {
			return err
		}
	case "paper_enabled_users_group_removal":
		err = json.Unmarshal(body, &u.PaperEnabledUsersGroupRemoval)

		if err != nil {
			return err
		}
	case "permanent_delete_change_policy":
		err = json.Unmarshal(body, &u.PermanentDeleteChangePolicy)

		if err != nil {
			return err
		}
	case "sharing_change_folder_join_policy":
		err = json.Unmarshal(body, &u.SharingChangeFolderJoinPolicy)

		if err != nil {
			return err
		}
	case "sharing_change_link_policy":
		err = json.Unmarshal(body, &u.SharingChangeLinkPolicy)

		if err != nil {
			return err
		}
	case "sharing_change_member_policy":
		err = json.Unmarshal(body, &u.SharingChangeMemberPolicy)

		if err != nil {
			return err
		}
	case "showcase_change_download_policy":
		err = json.Unmarshal(body, &u.ShowcaseChangeDownloadPolicy)

		if err != nil {
			return err
		}
	case "showcase_change_enabled_policy":
		err = json.Unmarshal(body, &u.ShowcaseChangeEnabledPolicy)

		if err != nil {
			return err
		}
	case "showcase_change_external_sharing_policy":
		err = json.Unmarshal(body, &u.ShowcaseChangeExternalSharingPolicy)

		if err != nil {
			return err
		}
	case "smart_sync_change_policy":
		err = json.Unmarshal(body, &u.SmartSyncChangePolicy)

		if err != nil {
			return err
		}
	case "smart_sync_not_opt_out":
		err = json.Unmarshal(body, &u.SmartSyncNotOptOut)

		if err != nil {
			return err
		}
	case "smart_sync_opt_out":
		err = json.Unmarshal(body, &u.SmartSyncOptOut)

		if err != nil {
			return err
		}
	case "sso_change_policy":
		err = json.Unmarshal(body, &u.SsoChangePolicy)

		if err != nil {
			return err
		}
	case "tfa_change_policy":
		err = json.Unmarshal(body, &u.TfaChangePolicy)

		if err != nil {
			return err
		}
	case "two_account_change_policy":
		err = json.Unmarshal(body, &u.TwoAccountChangePolicy)

		if err != nil {
			return err
		}
	case "web_sessions_change_fixed_length_policy":
		err = json.Unmarshal(body, &u.WebSessionsChangeFixedLengthPolicy)

		if err != nil {
			return err
		}
	case "web_sessions_change_idle_length_policy":
		err = json.Unmarshal(body, &u.WebSessionsChangeIdleLengthPolicy)

		if err != nil {
			return err
		}
	case "team_merge_from":
		err = json.Unmarshal(body, &u.TeamMergeFrom)

		if err != nil {
			return err
		}
	case "team_merge_to":
		err = json.Unmarshal(body, &u.TeamMergeTo)

		if err != nil {
			return err
		}
	case "team_profile_add_logo":
		err = json.Unmarshal(body, &u.TeamProfileAddLogo)

		if err != nil {
			return err
		}
	case "team_profile_change_default_language":
		err = json.Unmarshal(body, &u.TeamProfileChangeDefaultLanguage)

		if err != nil {
			return err
		}
	case "team_profile_change_logo":
		err = json.Unmarshal(body, &u.TeamProfileChangeLogo)

		if err != nil {
			return err
		}
	case "team_profile_change_name":
		err = json.Unmarshal(body, &u.TeamProfileChangeName)

		if err != nil {
			return err
		}
	case "team_profile_remove_logo":
		err = json.Unmarshal(body, &u.TeamProfileRemoveLogo)

		if err != nil {
			return err
		}
	case "tfa_add_backup_phone":
		err = json.Unmarshal(body, &u.TfaAddBackupPhone)

		if err != nil {
			return err
		}
	case "tfa_add_security_key":
		err = json.Unmarshal(body, &u.TfaAddSecurityKey)

		if err != nil {
			return err
		}
	case "tfa_change_backup_phone":
		err = json.Unmarshal(body, &u.TfaChangeBackupPhone)

		if err != nil {
			return err
		}
	case "tfa_change_status":
		err = json.Unmarshal(body, &u.TfaChangeStatus)

		if err != nil {
			return err
		}
	case "tfa_remove_backup_phone":
		err = json.Unmarshal(body, &u.TfaRemoveBackupPhone)

		if err != nil {
			return err
		}
	case "tfa_remove_security_key":
		err = json.Unmarshal(body, &u.TfaRemoveSecurityKey)

		if err != nil {
			return err
		}
	case "tfa_reset":
		err = json.Unmarshal(body, &u.TfaReset)

		if err != nil {
			return err
		}
	}
	return nil
}

// ExportMembersReportDetails : Created member data report.
type ExportMembersReportDetails struct {
}

// NewExportMembersReportDetails returns a new ExportMembersReportDetails instance
func NewExportMembersReportDetails() *ExportMembersReportDetails {
	s := new(ExportMembersReportDetails)
	return s
}

// ExportMembersReportType : has no documentation (yet)
type ExportMembersReportType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewExportMembersReportType returns a new ExportMembersReportType instance
func NewExportMembersReportType(Description string) *ExportMembersReportType {
	s := new(ExportMembersReportType)
	s.Description = Description
	return s
}

// ExtendedVersionHistoryChangePolicyDetails : Accepted/opted out of extended
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

// ExtendedVersionHistoryChangePolicyType : has no documentation (yet)
type ExtendedVersionHistoryChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewExtendedVersionHistoryChangePolicyType returns a new ExtendedVersionHistoryChangePolicyType instance
func NewExtendedVersionHistoryChangePolicyType(Description string) *ExtendedVersionHistoryChangePolicyType {
	s := new(ExtendedVersionHistoryChangePolicyType)
	s.Description = Description
	return s
}

// ExtendedVersionHistoryPolicy : has no documentation (yet)
type ExtendedVersionHistoryPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ExtendedVersionHistoryPolicy
const (
	ExtendedVersionHistoryPolicyExplicitlyLimited   = "explicitly_limited"
	ExtendedVersionHistoryPolicyExplicitlyUnlimited = "explicitly_unlimited"
	ExtendedVersionHistoryPolicyImplicitlyLimited   = "implicitly_limited"
	ExtendedVersionHistoryPolicyImplicitlyUnlimited = "implicitly_unlimited"
	ExtendedVersionHistoryPolicyOther               = "other"
)

// ExternalUserLogInfo : A user without a Dropbox account.
type ExternalUserLogInfo struct {
	// UserIdentifier : An external user identifier.
	UserIdentifier string `json:"user_identifier"`
	// IdentifierType : Identifier type.
	IdentifierType *IdentifierType `json:"identifier_type"`
}

// NewExternalUserLogInfo returns a new ExternalUserLogInfo instance
func NewExternalUserLogInfo(UserIdentifier string, IdentifierType *IdentifierType) *ExternalUserLogInfo {
	s := new(ExternalUserLogInfo)
	s.UserIdentifier = UserIdentifier
	s.IdentifierType = IdentifierType
	return s
}

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

// FileAddCommentDetails : Added file comment.
type FileAddCommentDetails struct {
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileAddCommentDetails returns a new FileAddCommentDetails instance
func NewFileAddCommentDetails() *FileAddCommentDetails {
	s := new(FileAddCommentDetails)
	return s
}

// FileAddCommentType : has no documentation (yet)
type FileAddCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileAddCommentType returns a new FileAddCommentType instance
func NewFileAddCommentType(Description string) *FileAddCommentType {
	s := new(FileAddCommentType)
	s.Description = Description
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

// FileAddType : has no documentation (yet)
type FileAddType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileAddType returns a new FileAddType instance
func NewFileAddType(Description string) *FileAddType {
	s := new(FileAddType)
	s.Description = Description
	return s
}

// FileChangeCommentSubscriptionDetails : Subscribed to or unsubscribed from
// comment notifications for file.
type FileChangeCommentSubscriptionDetails struct {
	// NewValue : New file comment subscription.
	NewValue *FileCommentNotificationPolicy `json:"new_value"`
	// PreviousValue : Previous file comment subscription. Might be missing due
	// to historical data gap.
	PreviousValue *FileCommentNotificationPolicy `json:"previous_value,omitempty"`
}

// NewFileChangeCommentSubscriptionDetails returns a new FileChangeCommentSubscriptionDetails instance
func NewFileChangeCommentSubscriptionDetails(NewValue *FileCommentNotificationPolicy) *FileChangeCommentSubscriptionDetails {
	s := new(FileChangeCommentSubscriptionDetails)
	s.NewValue = NewValue
	return s
}

// FileChangeCommentSubscriptionType : has no documentation (yet)
type FileChangeCommentSubscriptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileChangeCommentSubscriptionType returns a new FileChangeCommentSubscriptionType instance
func NewFileChangeCommentSubscriptionType(Description string) *FileChangeCommentSubscriptionType {
	s := new(FileChangeCommentSubscriptionType)
	s.Description = Description
	return s
}

// FileCommentNotificationPolicy : Enable or disable file comments notifications
type FileCommentNotificationPolicy struct {
	dropbox.Tagged
}

// Valid tag values for FileCommentNotificationPolicy
const (
	FileCommentNotificationPolicyDisabled = "disabled"
	FileCommentNotificationPolicyEnabled  = "enabled"
	FileCommentNotificationPolicyOther    = "other"
)

// FileCommentsChangePolicyDetails : Enabled/disabled commenting on team files.
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

// FileCommentsChangePolicyType : has no documentation (yet)
type FileCommentsChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileCommentsChangePolicyType returns a new FileCommentsChangePolicyType instance
func NewFileCommentsChangePolicyType(Description string) *FileCommentsChangePolicyType {
	s := new(FileCommentsChangePolicyType)
	s.Description = Description
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

// FileCopyType : has no documentation (yet)
type FileCopyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileCopyType returns a new FileCopyType instance
func NewFileCopyType(Description string) *FileCopyType {
	s := new(FileCopyType)
	s.Description = Description
	return s
}

// FileDeleteCommentDetails : Deleted file comment.
type FileDeleteCommentDetails struct {
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileDeleteCommentDetails returns a new FileDeleteCommentDetails instance
func NewFileDeleteCommentDetails() *FileDeleteCommentDetails {
	s := new(FileDeleteCommentDetails)
	return s
}

// FileDeleteCommentType : has no documentation (yet)
type FileDeleteCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileDeleteCommentType returns a new FileDeleteCommentType instance
func NewFileDeleteCommentType(Description string) *FileDeleteCommentType {
	s := new(FileDeleteCommentType)
	s.Description = Description
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

// FileDeleteType : has no documentation (yet)
type FileDeleteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileDeleteType returns a new FileDeleteType instance
func NewFileDeleteType(Description string) *FileDeleteType {
	s := new(FileDeleteType)
	s.Description = Description
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

// FileDownloadType : has no documentation (yet)
type FileDownloadType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileDownloadType returns a new FileDownloadType instance
func NewFileDownloadType(Description string) *FileDownloadType {
	s := new(FileDownloadType)
	s.Description = Description
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

// FileEditType : has no documentation (yet)
type FileEditType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileEditType returns a new FileEditType instance
func NewFileEditType(Description string) *FileEditType {
	s := new(FileEditType)
	s.Description = Description
	return s
}

// FileGetCopyReferenceDetails : Created copy reference to file/folder.
type FileGetCopyReferenceDetails struct {
}

// NewFileGetCopyReferenceDetails returns a new FileGetCopyReferenceDetails instance
func NewFileGetCopyReferenceDetails() *FileGetCopyReferenceDetails {
	s := new(FileGetCopyReferenceDetails)
	return s
}

// FileGetCopyReferenceType : has no documentation (yet)
type FileGetCopyReferenceType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileGetCopyReferenceType returns a new FileGetCopyReferenceType instance
func NewFileGetCopyReferenceType(Description string) *FileGetCopyReferenceType {
	s := new(FileGetCopyReferenceType)
	s.Description = Description
	return s
}

// FileLikeCommentDetails : Liked file comment.
type FileLikeCommentDetails struct {
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileLikeCommentDetails returns a new FileLikeCommentDetails instance
func NewFileLikeCommentDetails() *FileLikeCommentDetails {
	s := new(FileLikeCommentDetails)
	return s
}

// FileLikeCommentType : has no documentation (yet)
type FileLikeCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileLikeCommentType returns a new FileLikeCommentType instance
func NewFileLikeCommentType(Description string) *FileLikeCommentType {
	s := new(FileLikeCommentType)
	s.Description = Description
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

// FileMoveType : has no documentation (yet)
type FileMoveType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileMoveType returns a new FileMoveType instance
func NewFileMoveType(Description string) *FileMoveType {
	s := new(FileMoveType)
	s.Description = Description
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

// FilePermanentlyDeleteType : has no documentation (yet)
type FilePermanentlyDeleteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFilePermanentlyDeleteType returns a new FilePermanentlyDeleteType instance
func NewFilePermanentlyDeleteType(Description string) *FilePermanentlyDeleteType {
	s := new(FilePermanentlyDeleteType)
	s.Description = Description
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

// FilePreviewType : has no documentation (yet)
type FilePreviewType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFilePreviewType returns a new FilePreviewType instance
func NewFilePreviewType(Description string) *FilePreviewType {
	s := new(FilePreviewType)
	s.Description = Description
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

// FileRenameType : has no documentation (yet)
type FileRenameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRenameType returns a new FileRenameType instance
func NewFileRenameType(Description string) *FileRenameType {
	s := new(FileRenameType)
	s.Description = Description
	return s
}

// FileRequestChangeDetails : Changed file request.
type FileRequestChangeDetails struct {
	// FileRequestId : File request id. Might be missing due to historical data
	// gap.
	FileRequestId string `json:"file_request_id,omitempty"`
	// PreviousDetails : Previous file request details. Might be missing due to
	// historical data gap.
	PreviousDetails *FileRequestDetails `json:"previous_details,omitempty"`
	// NewDetails : New file request details.
	NewDetails *FileRequestDetails `json:"new_details"`
}

// NewFileRequestChangeDetails returns a new FileRequestChangeDetails instance
func NewFileRequestChangeDetails(NewDetails *FileRequestDetails) *FileRequestChangeDetails {
	s := new(FileRequestChangeDetails)
	s.NewDetails = NewDetails
	return s
}

// FileRequestChangeType : has no documentation (yet)
type FileRequestChangeType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestChangeType returns a new FileRequestChangeType instance
func NewFileRequestChangeType(Description string) *FileRequestChangeType {
	s := new(FileRequestChangeType)
	s.Description = Description
	return s
}

// FileRequestCloseDetails : Closed file request.
type FileRequestCloseDetails struct {
	// FileRequestId : File request id. Might be missing due to historical data
	// gap.
	FileRequestId string `json:"file_request_id,omitempty"`
	// PreviousDetails : Previous file request details. Might be missing due to
	// historical data gap.
	PreviousDetails *FileRequestDetails `json:"previous_details,omitempty"`
}

// NewFileRequestCloseDetails returns a new FileRequestCloseDetails instance
func NewFileRequestCloseDetails() *FileRequestCloseDetails {
	s := new(FileRequestCloseDetails)
	return s
}

// FileRequestCloseType : has no documentation (yet)
type FileRequestCloseType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestCloseType returns a new FileRequestCloseType instance
func NewFileRequestCloseType(Description string) *FileRequestCloseType {
	s := new(FileRequestCloseType)
	s.Description = Description
	return s
}

// FileRequestCreateDetails : Created file request.
type FileRequestCreateDetails struct {
	// FileRequestId : File request id. Might be missing due to historical data
	// gap.
	FileRequestId string `json:"file_request_id,omitempty"`
	// RequestDetails : File request details. Might be missing due to historical
	// data gap.
	RequestDetails *FileRequestDetails `json:"request_details,omitempty"`
}

// NewFileRequestCreateDetails returns a new FileRequestCreateDetails instance
func NewFileRequestCreateDetails() *FileRequestCreateDetails {
	s := new(FileRequestCreateDetails)
	return s
}

// FileRequestCreateType : has no documentation (yet)
type FileRequestCreateType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestCreateType returns a new FileRequestCreateType instance
func NewFileRequestCreateType(Description string) *FileRequestCreateType {
	s := new(FileRequestCreateType)
	s.Description = Description
	return s
}

// FileRequestDeadline : File request deadline
type FileRequestDeadline struct {
	// Deadline : The deadline for this file request. Might be missing due to
	// historical data gap.
	Deadline time.Time `json:"deadline,omitempty"`
	// AllowLateUploads : If set, allow uploads after the deadline has passed.
	// Might be missing due to historical data gap.
	AllowLateUploads string `json:"allow_late_uploads,omitempty"`
}

// NewFileRequestDeadline returns a new FileRequestDeadline instance
func NewFileRequestDeadline() *FileRequestDeadline {
	s := new(FileRequestDeadline)
	return s
}

// FileRequestDetails : File request details
type FileRequestDetails struct {
	// AssetIndex : Asset position in the Assets list.
	AssetIndex uint64 `json:"asset_index"`
	// Deadline : File request deadline. Might be missing due to historical data
	// gap.
	Deadline *FileRequestDeadline `json:"deadline,omitempty"`
}

// NewFileRequestDetails returns a new FileRequestDetails instance
func NewFileRequestDetails(AssetIndex uint64) *FileRequestDetails {
	s := new(FileRequestDetails)
	s.AssetIndex = AssetIndex
	return s
}

// FileRequestReceiveFileDetails : Received files for file request.
type FileRequestReceiveFileDetails struct {
	// FileRequestId : File request id. Might be missing due to historical data
	// gap.
	FileRequestId string `json:"file_request_id,omitempty"`
	// FileRequestDetails : File request details. Might be missing due to
	// historical data gap.
	FileRequestDetails *FileRequestDetails `json:"file_request_details,omitempty"`
	// SubmittedFileNames : Submitted file names.
	SubmittedFileNames []string `json:"submitted_file_names"`
	// SubmitterName : The name as provided by the submitter. Might be missing
	// due to historical data gap.
	SubmitterName string `json:"submitter_name,omitempty"`
	// SubmitterEmail : The email as provided by the submitter. Might be missing
	// due to historical data gap.
	SubmitterEmail string `json:"submitter_email,omitempty"`
}

// NewFileRequestReceiveFileDetails returns a new FileRequestReceiveFileDetails instance
func NewFileRequestReceiveFileDetails(SubmittedFileNames []string) *FileRequestReceiveFileDetails {
	s := new(FileRequestReceiveFileDetails)
	s.SubmittedFileNames = SubmittedFileNames
	return s
}

// FileRequestReceiveFileType : has no documentation (yet)
type FileRequestReceiveFileType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestReceiveFileType returns a new FileRequestReceiveFileType instance
func NewFileRequestReceiveFileType(Description string) *FileRequestReceiveFileType {
	s := new(FileRequestReceiveFileType)
	s.Description = Description
	return s
}

// FileRequestsChangePolicyDetails : Enabled/disabled file requests.
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

// FileRequestsChangePolicyType : has no documentation (yet)
type FileRequestsChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestsChangePolicyType returns a new FileRequestsChangePolicyType instance
func NewFileRequestsChangePolicyType(Description string) *FileRequestsChangePolicyType {
	s := new(FileRequestsChangePolicyType)
	s.Description = Description
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

// FileRequestsEmailsEnabledType : has no documentation (yet)
type FileRequestsEmailsEnabledType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestsEmailsEnabledType returns a new FileRequestsEmailsEnabledType instance
func NewFileRequestsEmailsEnabledType(Description string) *FileRequestsEmailsEnabledType {
	s := new(FileRequestsEmailsEnabledType)
	s.Description = Description
	return s
}

// FileRequestsEmailsRestrictedToTeamOnlyDetails : Enabled file request emails
// for team.
type FileRequestsEmailsRestrictedToTeamOnlyDetails struct {
}

// NewFileRequestsEmailsRestrictedToTeamOnlyDetails returns a new FileRequestsEmailsRestrictedToTeamOnlyDetails instance
func NewFileRequestsEmailsRestrictedToTeamOnlyDetails() *FileRequestsEmailsRestrictedToTeamOnlyDetails {
	s := new(FileRequestsEmailsRestrictedToTeamOnlyDetails)
	return s
}

// FileRequestsEmailsRestrictedToTeamOnlyType : has no documentation (yet)
type FileRequestsEmailsRestrictedToTeamOnlyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRequestsEmailsRestrictedToTeamOnlyType returns a new FileRequestsEmailsRestrictedToTeamOnlyType instance
func NewFileRequestsEmailsRestrictedToTeamOnlyType(Description string) *FileRequestsEmailsRestrictedToTeamOnlyType {
	s := new(FileRequestsEmailsRestrictedToTeamOnlyType)
	s.Description = Description
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

// FileResolveCommentDetails : Resolved file comment.
type FileResolveCommentDetails struct {
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileResolveCommentDetails returns a new FileResolveCommentDetails instance
func NewFileResolveCommentDetails() *FileResolveCommentDetails {
	s := new(FileResolveCommentDetails)
	return s
}

// FileResolveCommentType : has no documentation (yet)
type FileResolveCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileResolveCommentType returns a new FileResolveCommentType instance
func NewFileResolveCommentType(Description string) *FileResolveCommentType {
	s := new(FileResolveCommentType)
	s.Description = Description
	return s
}

// FileRestoreDetails : Restored deleted files and/or folders.
type FileRestoreDetails struct {
}

// NewFileRestoreDetails returns a new FileRestoreDetails instance
func NewFileRestoreDetails() *FileRestoreDetails {
	s := new(FileRestoreDetails)
	return s
}

// FileRestoreType : has no documentation (yet)
type FileRestoreType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRestoreType returns a new FileRestoreType instance
func NewFileRestoreType(Description string) *FileRestoreType {
	s := new(FileRestoreType)
	s.Description = Description
	return s
}

// FileRevertDetails : Reverted files to previous version.
type FileRevertDetails struct {
}

// NewFileRevertDetails returns a new FileRevertDetails instance
func NewFileRevertDetails() *FileRevertDetails {
	s := new(FileRevertDetails)
	return s
}

// FileRevertType : has no documentation (yet)
type FileRevertType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRevertType returns a new FileRevertType instance
func NewFileRevertType(Description string) *FileRevertType {
	s := new(FileRevertType)
	s.Description = Description
	return s
}

// FileRollbackChangesDetails : Rolled back file actions.
type FileRollbackChangesDetails struct {
}

// NewFileRollbackChangesDetails returns a new FileRollbackChangesDetails instance
func NewFileRollbackChangesDetails() *FileRollbackChangesDetails {
	s := new(FileRollbackChangesDetails)
	return s
}

// FileRollbackChangesType : has no documentation (yet)
type FileRollbackChangesType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileRollbackChangesType returns a new FileRollbackChangesType instance
func NewFileRollbackChangesType(Description string) *FileRollbackChangesType {
	s := new(FileRollbackChangesType)
	s.Description = Description
	return s
}

// FileSaveCopyReferenceDetails : Saved file/folder using copy reference.
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

// FileSaveCopyReferenceType : has no documentation (yet)
type FileSaveCopyReferenceType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileSaveCopyReferenceType returns a new FileSaveCopyReferenceType instance
func NewFileSaveCopyReferenceType(Description string) *FileSaveCopyReferenceType {
	s := new(FileSaveCopyReferenceType)
	s.Description = Description
	return s
}

// FileUnlikeCommentDetails : Unliked file comment.
type FileUnlikeCommentDetails struct {
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileUnlikeCommentDetails returns a new FileUnlikeCommentDetails instance
func NewFileUnlikeCommentDetails() *FileUnlikeCommentDetails {
	s := new(FileUnlikeCommentDetails)
	return s
}

// FileUnlikeCommentType : has no documentation (yet)
type FileUnlikeCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileUnlikeCommentType returns a new FileUnlikeCommentType instance
func NewFileUnlikeCommentType(Description string) *FileUnlikeCommentType {
	s := new(FileUnlikeCommentType)
	s.Description = Description
	return s
}

// FileUnresolveCommentDetails : Unresolved file comment.
type FileUnresolveCommentDetails struct {
	// CommentText : Comment text. Might be missing due to historical data gap.
	CommentText string `json:"comment_text,omitempty"`
}

// NewFileUnresolveCommentDetails returns a new FileUnresolveCommentDetails instance
func NewFileUnresolveCommentDetails() *FileUnresolveCommentDetails {
	s := new(FileUnresolveCommentDetails)
	return s
}

// FileUnresolveCommentType : has no documentation (yet)
type FileUnresolveCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewFileUnresolveCommentType returns a new FileUnresolveCommentType instance
func NewFileUnresolveCommentType(Description string) *FileUnresolveCommentType {
	s := new(FileUnresolveCommentType)
	s.Description = Description
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

// GoogleSsoChangePolicyDetails : Enabled/disabled Google single sign-on for
// team.
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

// GoogleSsoChangePolicyType : has no documentation (yet)
type GoogleSsoChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGoogleSsoChangePolicyType returns a new GoogleSsoChangePolicyType instance
func NewGoogleSsoChangePolicyType(Description string) *GoogleSsoChangePolicyType {
	s := new(GoogleSsoChangePolicyType)
	s.Description = Description
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

// GroupAddExternalIdDetails : Added external ID for group.
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

// GroupAddExternalIdType : has no documentation (yet)
type GroupAddExternalIdType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupAddExternalIdType returns a new GroupAddExternalIdType instance
func NewGroupAddExternalIdType(Description string) *GroupAddExternalIdType {
	s := new(GroupAddExternalIdType)
	s.Description = Description
	return s
}

// GroupAddMemberDetails : Added team members to group.
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

// GroupAddMemberType : has no documentation (yet)
type GroupAddMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupAddMemberType returns a new GroupAddMemberType instance
func NewGroupAddMemberType(Description string) *GroupAddMemberType {
	s := new(GroupAddMemberType)
	s.Description = Description
	return s
}

// GroupChangeExternalIdDetails : Changed external ID for group.
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

// GroupChangeExternalIdType : has no documentation (yet)
type GroupChangeExternalIdType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupChangeExternalIdType returns a new GroupChangeExternalIdType instance
func NewGroupChangeExternalIdType(Description string) *GroupChangeExternalIdType {
	s := new(GroupChangeExternalIdType)
	s.Description = Description
	return s
}

// GroupChangeManagementTypeDetails : Changed group management type.
type GroupChangeManagementTypeDetails struct {
	// NewValue : New group management type.
	NewValue *team_common.GroupManagementType `json:"new_value"`
	// PreviousValue : Previous group management type. Might be missing due to
	// historical data gap.
	PreviousValue *team_common.GroupManagementType `json:"previous_value,omitempty"`
}

// NewGroupChangeManagementTypeDetails returns a new GroupChangeManagementTypeDetails instance
func NewGroupChangeManagementTypeDetails(NewValue *team_common.GroupManagementType) *GroupChangeManagementTypeDetails {
	s := new(GroupChangeManagementTypeDetails)
	s.NewValue = NewValue
	return s
}

// GroupChangeManagementTypeType : has no documentation (yet)
type GroupChangeManagementTypeType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupChangeManagementTypeType returns a new GroupChangeManagementTypeType instance
func NewGroupChangeManagementTypeType(Description string) *GroupChangeManagementTypeType {
	s := new(GroupChangeManagementTypeType)
	s.Description = Description
	return s
}

// GroupChangeMemberRoleDetails : Changed manager permissions of group member.
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

// GroupChangeMemberRoleType : has no documentation (yet)
type GroupChangeMemberRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupChangeMemberRoleType returns a new GroupChangeMemberRoleType instance
func NewGroupChangeMemberRoleType(Description string) *GroupChangeMemberRoleType {
	s := new(GroupChangeMemberRoleType)
	s.Description = Description
	return s
}

// GroupCreateDetails : Created group.
type GroupCreateDetails struct {
	// IsCompanyManaged : Is company managed group. Might be missing due to
	// historical data gap.
	IsCompanyManaged bool `json:"is_company_managed,omitempty"`
	// JoinPolicy : Group join policy.
	JoinPolicy *GroupJoinPolicy `json:"join_policy,omitempty"`
}

// NewGroupCreateDetails returns a new GroupCreateDetails instance
func NewGroupCreateDetails() *GroupCreateDetails {
	s := new(GroupCreateDetails)
	return s
}

// GroupCreateType : has no documentation (yet)
type GroupCreateType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupCreateType returns a new GroupCreateType instance
func NewGroupCreateType(Description string) *GroupCreateType {
	s := new(GroupCreateType)
	s.Description = Description
	return s
}

// GroupDeleteDetails : Deleted group.
type GroupDeleteDetails struct {
	// IsCompanyManaged : Is company managed group. Might be missing due to
	// historical data gap.
	IsCompanyManaged bool `json:"is_company_managed,omitempty"`
}

// NewGroupDeleteDetails returns a new GroupDeleteDetails instance
func NewGroupDeleteDetails() *GroupDeleteDetails {
	s := new(GroupDeleteDetails)
	return s
}

// GroupDeleteType : has no documentation (yet)
type GroupDeleteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupDeleteType returns a new GroupDeleteType instance
func NewGroupDeleteType(Description string) *GroupDeleteType {
	s := new(GroupDeleteType)
	s.Description = Description
	return s
}

// GroupDescriptionUpdatedDetails : Updated group.
type GroupDescriptionUpdatedDetails struct {
}

// NewGroupDescriptionUpdatedDetails returns a new GroupDescriptionUpdatedDetails instance
func NewGroupDescriptionUpdatedDetails() *GroupDescriptionUpdatedDetails {
	s := new(GroupDescriptionUpdatedDetails)
	return s
}

// GroupDescriptionUpdatedType : has no documentation (yet)
type GroupDescriptionUpdatedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupDescriptionUpdatedType returns a new GroupDescriptionUpdatedType instance
func NewGroupDescriptionUpdatedType(Description string) *GroupDescriptionUpdatedType {
	s := new(GroupDescriptionUpdatedType)
	s.Description = Description
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

// GroupJoinPolicyUpdatedDetails : Updated group join policy.
type GroupJoinPolicyUpdatedDetails struct {
	// IsCompanyManaged : Is company managed group. Might be missing due to
	// historical data gap.
	IsCompanyManaged bool `json:"is_company_managed,omitempty"`
	// JoinPolicy : Group join policy.
	JoinPolicy *GroupJoinPolicy `json:"join_policy,omitempty"`
}

// NewGroupJoinPolicyUpdatedDetails returns a new GroupJoinPolicyUpdatedDetails instance
func NewGroupJoinPolicyUpdatedDetails() *GroupJoinPolicyUpdatedDetails {
	s := new(GroupJoinPolicyUpdatedDetails)
	return s
}

// GroupJoinPolicyUpdatedType : has no documentation (yet)
type GroupJoinPolicyUpdatedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupJoinPolicyUpdatedType returns a new GroupJoinPolicyUpdatedType instance
func NewGroupJoinPolicyUpdatedType(Description string) *GroupJoinPolicyUpdatedType {
	s := new(GroupJoinPolicyUpdatedType)
	s.Description = Description
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

// GroupMovedDetails : Moved group.
type GroupMovedDetails struct {
}

// NewGroupMovedDetails returns a new GroupMovedDetails instance
func NewGroupMovedDetails() *GroupMovedDetails {
	s := new(GroupMovedDetails)
	return s
}

// GroupMovedType : has no documentation (yet)
type GroupMovedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupMovedType returns a new GroupMovedType instance
func NewGroupMovedType(Description string) *GroupMovedType {
	s := new(GroupMovedType)
	s.Description = Description
	return s
}

// GroupRemoveExternalIdDetails : Removed external ID for group.
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

// GroupRemoveExternalIdType : has no documentation (yet)
type GroupRemoveExternalIdType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupRemoveExternalIdType returns a new GroupRemoveExternalIdType instance
func NewGroupRemoveExternalIdType(Description string) *GroupRemoveExternalIdType {
	s := new(GroupRemoveExternalIdType)
	s.Description = Description
	return s
}

// GroupRemoveMemberDetails : Removed team members from group.
type GroupRemoveMemberDetails struct {
}

// NewGroupRemoveMemberDetails returns a new GroupRemoveMemberDetails instance
func NewGroupRemoveMemberDetails() *GroupRemoveMemberDetails {
	s := new(GroupRemoveMemberDetails)
	return s
}

// GroupRemoveMemberType : has no documentation (yet)
type GroupRemoveMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupRemoveMemberType returns a new GroupRemoveMemberType instance
func NewGroupRemoveMemberType(Description string) *GroupRemoveMemberType {
	s := new(GroupRemoveMemberType)
	s.Description = Description
	return s
}

// GroupRenameDetails : Renamed group.
type GroupRenameDetails struct {
	// PreviousValue : Previous display name.
	PreviousValue string `json:"previous_value"`
	// NewValue : New display name.
	NewValue string `json:"new_value"`
}

// NewGroupRenameDetails returns a new GroupRenameDetails instance
func NewGroupRenameDetails(PreviousValue string, NewValue string) *GroupRenameDetails {
	s := new(GroupRenameDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// GroupRenameType : has no documentation (yet)
type GroupRenameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupRenameType returns a new GroupRenameType instance
func NewGroupRenameType(Description string) *GroupRenameType {
	s := new(GroupRenameType)
	s.Description = Description
	return s
}

// GroupUserManagementChangePolicyDetails : Changed who can create groups.
type GroupUserManagementChangePolicyDetails struct {
	// NewValue : New group users management policy.
	NewValue *team_policies.GroupCreation `json:"new_value"`
	// PreviousValue : Previous group users management policy. Might be missing
	// due to historical data gap.
	PreviousValue *team_policies.GroupCreation `json:"previous_value,omitempty"`
}

// NewGroupUserManagementChangePolicyDetails returns a new GroupUserManagementChangePolicyDetails instance
func NewGroupUserManagementChangePolicyDetails(NewValue *team_policies.GroupCreation) *GroupUserManagementChangePolicyDetails {
	s := new(GroupUserManagementChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// GroupUserManagementChangePolicyType : has no documentation (yet)
type GroupUserManagementChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewGroupUserManagementChangePolicyType returns a new GroupUserManagementChangePolicyType instance
func NewGroupUserManagementChangePolicyType(Description string) *GroupUserManagementChangePolicyType {
	s := new(GroupUserManagementChangePolicyType)
	s.Description = Description
	return s
}

// IdentifierType : has no documentation (yet)
type IdentifierType struct {
	dropbox.Tagged
}

// Valid tag values for IdentifierType
const (
	IdentifierTypeEmail               = "email"
	IdentifierTypeFacebookProfileName = "facebook_profile_name"
	IdentifierTypeOther               = "other"
)

// JoinTeamDetails : Additional information relevant when a new member joins the
// team.
type JoinTeamDetails struct {
	// LinkedApps : Linked applications.
	LinkedApps []*UserLinkedAppLogInfo `json:"linked_apps"`
	// LinkedDevices : Linked devices.
	LinkedDevices []*LinkedDeviceLogInfo `json:"linked_devices"`
	// LinkedSharedFolders : Linked shared folders.
	LinkedSharedFolders []*FolderLogInfo `json:"linked_shared_folders"`
}

// NewJoinTeamDetails returns a new JoinTeamDetails instance
func NewJoinTeamDetails(LinkedApps []*UserLinkedAppLogInfo, LinkedDevices []*LinkedDeviceLogInfo, LinkedSharedFolders []*FolderLogInfo) *JoinTeamDetails {
	s := new(JoinTeamDetails)
	s.LinkedApps = LinkedApps
	s.LinkedDevices = LinkedDevices
	s.LinkedSharedFolders = LinkedSharedFolders
	return s
}

// LegacyDeviceSessionLogInfo : Information on sessions, in legacy format
type LegacyDeviceSessionLogInfo struct {
	DeviceSessionLogInfo
	// SessionInfo : Session unique id. Might be missing due to historical data
	// gap.
	SessionInfo IsSessionLogInfo `json:"session_info,omitempty"`
	// DisplayName : The device name. Might be missing due to historical data
	// gap.
	DisplayName string `json:"display_name,omitempty"`
	// IsEmmManaged : Is device managed by emm. Might be missing due to
	// historical data gap.
	IsEmmManaged bool `json:"is_emm_managed,omitempty"`
	// Platform : Information on the hosting platform. Might be missing due to
	// historical data gap.
	Platform string `json:"platform,omitempty"`
	// MacAddress : The mac address of the last activity from this session.
	// Might be missing due to historical data gap.
	MacAddress string `json:"mac_address,omitempty"`
	// OsVersion : The hosting OS version. Might be missing due to historical
	// data gap.
	OsVersion string `json:"os_version,omitempty"`
	// DeviceType : Information on the hosting device type. Might be missing due
	// to historical data gap.
	DeviceType string `json:"device_type,omitempty"`
	// ClientVersion : The Dropbox client version. Might be missing due to
	// historical data gap.
	ClientVersion string `json:"client_version,omitempty"`
	// LegacyUniqId : Alternative unique device session id, instead of session
	// id field. Might be missing due to historical data gap.
	LegacyUniqId string `json:"legacy_uniq_id,omitempty"`
}

// NewLegacyDeviceSessionLogInfo returns a new LegacyDeviceSessionLogInfo instance
func NewLegacyDeviceSessionLogInfo() *LegacyDeviceSessionLogInfo {
	s := new(LegacyDeviceSessionLogInfo)
	return s
}

// LinkedDeviceLogInfo : The device sessions that user is linked to.
type LinkedDeviceLogInfo struct {
	dropbox.Tagged
	// MobileDeviceSession : mobile device session's details.
	MobileDeviceSession *MobileDeviceSessionLogInfo `json:"mobile_device_session,omitempty"`
	// DesktopDeviceSession : desktop device session's details.
	DesktopDeviceSession *DesktopDeviceSessionLogInfo `json:"desktop_device_session,omitempty"`
	// WebDeviceSession : web device session's details.
	WebDeviceSession *WebDeviceSessionLogInfo `json:"web_device_session,omitempty"`
	// LegacyDeviceSession : legacy device session's details.
	LegacyDeviceSession *LegacyDeviceSessionLogInfo `json:"legacy_device_session,omitempty"`
}

// Valid tag values for LinkedDeviceLogInfo
const (
	LinkedDeviceLogInfoMobileDeviceSession  = "mobile_device_session"
	LinkedDeviceLogInfoDesktopDeviceSession = "desktop_device_session"
	LinkedDeviceLogInfoWebDeviceSession     = "web_device_session"
	LinkedDeviceLogInfoLegacyDeviceSession  = "legacy_device_session"
	LinkedDeviceLogInfoOther                = "other"
)

// UnmarshalJSON deserializes into a LinkedDeviceLogInfo instance
func (u *LinkedDeviceLogInfo) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// MobileDeviceSession : mobile device session's details.
		MobileDeviceSession json.RawMessage `json:"mobile_device_session,omitempty"`
		// DesktopDeviceSession : desktop device session's details.
		DesktopDeviceSession json.RawMessage `json:"desktop_device_session,omitempty"`
		// WebDeviceSession : web device session's details.
		WebDeviceSession json.RawMessage `json:"web_device_session,omitempty"`
		// LegacyDeviceSession : legacy device session's details.
		LegacyDeviceSession json.RawMessage `json:"legacy_device_session,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "mobile_device_session":
		err = json.Unmarshal(body, &u.MobileDeviceSession)

		if err != nil {
			return err
		}
	case "desktop_device_session":
		err = json.Unmarshal(body, &u.DesktopDeviceSession)

		if err != nil {
			return err
		}
	case "web_device_session":
		err = json.Unmarshal(body, &u.WebDeviceSession)

		if err != nil {
			return err
		}
	case "legacy_device_session":
		err = json.Unmarshal(body, &u.LegacyDeviceSession)

		if err != nil {
			return err
		}
	}
	return nil
}

// LoginFailDetails : Failed to sign in.
type LoginFailDetails struct {
	// IsEmmManaged : Tells if the login device is EMM managed. Might be missing
	// due to historical data gap.
	IsEmmManaged bool `json:"is_emm_managed,omitempty"`
	// LoginMethod : Login method.
	LoginMethod *LoginMethod `json:"login_method"`
	// ErrorDetails : Error details.
	ErrorDetails *FailureDetailsLogInfo `json:"error_details"`
}

// NewLoginFailDetails returns a new LoginFailDetails instance
func NewLoginFailDetails(LoginMethod *LoginMethod, ErrorDetails *FailureDetailsLogInfo) *LoginFailDetails {
	s := new(LoginFailDetails)
	s.LoginMethod = LoginMethod
	s.ErrorDetails = ErrorDetails
	return s
}

// LoginFailType : has no documentation (yet)
type LoginFailType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewLoginFailType returns a new LoginFailType instance
func NewLoginFailType(Description string) *LoginFailType {
	s := new(LoginFailType)
	s.Description = Description
	return s
}

// LoginMethod : has no documentation (yet)
type LoginMethod struct {
	dropbox.Tagged
}

// Valid tag values for LoginMethod
const (
	LoginMethodPassword                = "password"
	LoginMethodTwoFactorAuthentication = "two_factor_authentication"
	LoginMethodSaml                    = "saml"
	LoginMethodOther                   = "other"
)

// LoginSuccessDetails : Signed in.
type LoginSuccessDetails struct {
	// IsEmmManaged : Tells if the login device is EMM managed. Might be missing
	// due to historical data gap.
	IsEmmManaged bool `json:"is_emm_managed,omitempty"`
	// LoginMethod : Login method.
	LoginMethod *LoginMethod `json:"login_method"`
}

// NewLoginSuccessDetails returns a new LoginSuccessDetails instance
func NewLoginSuccessDetails(LoginMethod *LoginMethod) *LoginSuccessDetails {
	s := new(LoginSuccessDetails)
	s.LoginMethod = LoginMethod
	return s
}

// LoginSuccessType : has no documentation (yet)
type LoginSuccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewLoginSuccessType returns a new LoginSuccessType instance
func NewLoginSuccessType(Description string) *LoginSuccessType {
	s := new(LoginSuccessType)
	s.Description = Description
	return s
}

// LogoutDetails : Signed out.
type LogoutDetails struct {
}

// NewLogoutDetails returns a new LogoutDetails instance
func NewLogoutDetails() *LogoutDetails {
	s := new(LogoutDetails)
	return s
}

// LogoutType : has no documentation (yet)
type LogoutType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewLogoutType returns a new LogoutType instance
func NewLogoutType(Description string) *LogoutType {
	s := new(LogoutType)
	s.Description = Description
	return s
}

// MemberAddNameDetails : Added team member name.
type MemberAddNameDetails struct {
	// NewValue : New user's name.
	NewValue *UserNameLogInfo `json:"new_value"`
}

// NewMemberAddNameDetails returns a new MemberAddNameDetails instance
func NewMemberAddNameDetails(NewValue *UserNameLogInfo) *MemberAddNameDetails {
	s := new(MemberAddNameDetails)
	s.NewValue = NewValue
	return s
}

// MemberAddNameType : has no documentation (yet)
type MemberAddNameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberAddNameType returns a new MemberAddNameType instance
func NewMemberAddNameType(Description string) *MemberAddNameType {
	s := new(MemberAddNameType)
	s.Description = Description
	return s
}

// MemberChangeAdminRoleDetails : Changed team member admin role.
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

// MemberChangeAdminRoleType : has no documentation (yet)
type MemberChangeAdminRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberChangeAdminRoleType returns a new MemberChangeAdminRoleType instance
func NewMemberChangeAdminRoleType(Description string) *MemberChangeAdminRoleType {
	s := new(MemberChangeAdminRoleType)
	s.Description = Description
	return s
}

// MemberChangeEmailDetails : Changed team member email.
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

// MemberChangeEmailType : has no documentation (yet)
type MemberChangeEmailType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberChangeEmailType returns a new MemberChangeEmailType instance
func NewMemberChangeEmailType(Description string) *MemberChangeEmailType {
	s := new(MemberChangeEmailType)
	s.Description = Description
	return s
}

// MemberChangeMembershipTypeDetails : Changed membership type (limited/full) of
// member.
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

// MemberChangeMembershipTypeType : has no documentation (yet)
type MemberChangeMembershipTypeType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberChangeMembershipTypeType returns a new MemberChangeMembershipTypeType instance
func NewMemberChangeMembershipTypeType(Description string) *MemberChangeMembershipTypeType {
	s := new(MemberChangeMembershipTypeType)
	s.Description = Description
	return s
}

// MemberChangeNameDetails : Changed team member name.
type MemberChangeNameDetails struct {
	// NewValue : New user's name.
	NewValue *UserNameLogInfo `json:"new_value"`
	// PreviousValue : Previous user's name. Might be missing due to historical
	// data gap.
	PreviousValue *UserNameLogInfo `json:"previous_value,omitempty"`
}

// NewMemberChangeNameDetails returns a new MemberChangeNameDetails instance
func NewMemberChangeNameDetails(NewValue *UserNameLogInfo) *MemberChangeNameDetails {
	s := new(MemberChangeNameDetails)
	s.NewValue = NewValue
	return s
}

// MemberChangeNameType : has no documentation (yet)
type MemberChangeNameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberChangeNameType returns a new MemberChangeNameType instance
func NewMemberChangeNameType(Description string) *MemberChangeNameType {
	s := new(MemberChangeNameType)
	s.Description = Description
	return s
}

// MemberChangeStatusDetails : Changed member status (invited, joined,
// suspended, etc.).
type MemberChangeStatusDetails struct {
	// PreviousValue : Previous member status. Might be missing due to
	// historical data gap.
	PreviousValue *MemberStatus `json:"previous_value,omitempty"`
	// NewValue : New member status.
	NewValue *MemberStatus `json:"new_value"`
	// Action : Additional information indicating the action taken that caused
	// status change.
	Action *ActionDetails `json:"action,omitempty"`
}

// NewMemberChangeStatusDetails returns a new MemberChangeStatusDetails instance
func NewMemberChangeStatusDetails(NewValue *MemberStatus) *MemberChangeStatusDetails {
	s := new(MemberChangeStatusDetails)
	s.NewValue = NewValue
	return s
}

// MemberChangeStatusType : has no documentation (yet)
type MemberChangeStatusType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberChangeStatusType returns a new MemberChangeStatusType instance
func NewMemberChangeStatusType(Description string) *MemberChangeStatusType {
	s := new(MemberChangeStatusType)
	s.Description = Description
	return s
}

// MemberPermanentlyDeleteAccountContentsDetails : Permanently deleted contents
// of deleted team member account.
type MemberPermanentlyDeleteAccountContentsDetails struct {
}

// NewMemberPermanentlyDeleteAccountContentsDetails returns a new MemberPermanentlyDeleteAccountContentsDetails instance
func NewMemberPermanentlyDeleteAccountContentsDetails() *MemberPermanentlyDeleteAccountContentsDetails {
	s := new(MemberPermanentlyDeleteAccountContentsDetails)
	return s
}

// MemberPermanentlyDeleteAccountContentsType : has no documentation (yet)
type MemberPermanentlyDeleteAccountContentsType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberPermanentlyDeleteAccountContentsType returns a new MemberPermanentlyDeleteAccountContentsType instance
func NewMemberPermanentlyDeleteAccountContentsType(Description string) *MemberPermanentlyDeleteAccountContentsType {
	s := new(MemberPermanentlyDeleteAccountContentsType)
	s.Description = Description
	return s
}

// MemberRemoveActionType : has no documentation (yet)
type MemberRemoveActionType struct {
	dropbox.Tagged
}

// Valid tag values for MemberRemoveActionType
const (
	MemberRemoveActionTypeDelete   = "delete"
	MemberRemoveActionTypeOffboard = "offboard"
	MemberRemoveActionTypeLeave    = "leave"
	MemberRemoveActionTypeOther    = "other"
)

// MemberRequestsChangePolicyDetails : Changed whether users can find team when
// not invited.
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

// MemberRequestsChangePolicyType : has no documentation (yet)
type MemberRequestsChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberRequestsChangePolicyType returns a new MemberRequestsChangePolicyType instance
func NewMemberRequestsChangePolicyType(Description string) *MemberRequestsChangePolicyType {
	s := new(MemberRequestsChangePolicyType)
	s.Description = Description
	return s
}

// MemberRequestsPolicy : has no documentation (yet)
type MemberRequestsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for MemberRequestsPolicy
const (
	MemberRequestsPolicyAutoAccept      = "auto_accept"
	MemberRequestsPolicyDisabled        = "disabled"
	MemberRequestsPolicyRequireApproval = "require_approval"
	MemberRequestsPolicyOther           = "other"
)

// MemberSpaceLimitsAddCustomQuotaDetails : Set custom member space limit.
type MemberSpaceLimitsAddCustomQuotaDetails struct {
	// NewValue : New custom quota value in bytes.
	NewValue uint64 `json:"new_value"`
}

// NewMemberSpaceLimitsAddCustomQuotaDetails returns a new MemberSpaceLimitsAddCustomQuotaDetails instance
func NewMemberSpaceLimitsAddCustomQuotaDetails(NewValue uint64) *MemberSpaceLimitsAddCustomQuotaDetails {
	s := new(MemberSpaceLimitsAddCustomQuotaDetails)
	s.NewValue = NewValue
	return s
}

// MemberSpaceLimitsAddCustomQuotaType : has no documentation (yet)
type MemberSpaceLimitsAddCustomQuotaType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsAddCustomQuotaType returns a new MemberSpaceLimitsAddCustomQuotaType instance
func NewMemberSpaceLimitsAddCustomQuotaType(Description string) *MemberSpaceLimitsAddCustomQuotaType {
	s := new(MemberSpaceLimitsAddCustomQuotaType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsAddExceptionDetails : Added members to member space limit
// exception list.
type MemberSpaceLimitsAddExceptionDetails struct {
}

// NewMemberSpaceLimitsAddExceptionDetails returns a new MemberSpaceLimitsAddExceptionDetails instance
func NewMemberSpaceLimitsAddExceptionDetails() *MemberSpaceLimitsAddExceptionDetails {
	s := new(MemberSpaceLimitsAddExceptionDetails)
	return s
}

// MemberSpaceLimitsAddExceptionType : has no documentation (yet)
type MemberSpaceLimitsAddExceptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsAddExceptionType returns a new MemberSpaceLimitsAddExceptionType instance
func NewMemberSpaceLimitsAddExceptionType(Description string) *MemberSpaceLimitsAddExceptionType {
	s := new(MemberSpaceLimitsAddExceptionType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsChangeCapsTypePolicyDetails : Changed member space limit
// type for team.
type MemberSpaceLimitsChangeCapsTypePolicyDetails struct {
	// PreviousValue : Previous space limit type.
	PreviousValue *SpaceCapsType `json:"previous_value"`
	// NewValue : New space limit type.
	NewValue *SpaceCapsType `json:"new_value"`
}

// NewMemberSpaceLimitsChangeCapsTypePolicyDetails returns a new MemberSpaceLimitsChangeCapsTypePolicyDetails instance
func NewMemberSpaceLimitsChangeCapsTypePolicyDetails(PreviousValue *SpaceCapsType, NewValue *SpaceCapsType) *MemberSpaceLimitsChangeCapsTypePolicyDetails {
	s := new(MemberSpaceLimitsChangeCapsTypePolicyDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// MemberSpaceLimitsChangeCapsTypePolicyType : has no documentation (yet)
type MemberSpaceLimitsChangeCapsTypePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsChangeCapsTypePolicyType returns a new MemberSpaceLimitsChangeCapsTypePolicyType instance
func NewMemberSpaceLimitsChangeCapsTypePolicyType(Description string) *MemberSpaceLimitsChangeCapsTypePolicyType {
	s := new(MemberSpaceLimitsChangeCapsTypePolicyType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsChangeCustomQuotaDetails : Changed custom member space
// limit.
type MemberSpaceLimitsChangeCustomQuotaDetails struct {
	// PreviousValue : Previous custom quota value in bytes.
	PreviousValue uint64 `json:"previous_value"`
	// NewValue : New custom quota value in bytes.
	NewValue uint64 `json:"new_value"`
}

// NewMemberSpaceLimitsChangeCustomQuotaDetails returns a new MemberSpaceLimitsChangeCustomQuotaDetails instance
func NewMemberSpaceLimitsChangeCustomQuotaDetails(PreviousValue uint64, NewValue uint64) *MemberSpaceLimitsChangeCustomQuotaDetails {
	s := new(MemberSpaceLimitsChangeCustomQuotaDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// MemberSpaceLimitsChangeCustomQuotaType : has no documentation (yet)
type MemberSpaceLimitsChangeCustomQuotaType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsChangeCustomQuotaType returns a new MemberSpaceLimitsChangeCustomQuotaType instance
func NewMemberSpaceLimitsChangeCustomQuotaType(Description string) *MemberSpaceLimitsChangeCustomQuotaType {
	s := new(MemberSpaceLimitsChangeCustomQuotaType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsChangePolicyDetails : Changed team default member space
// limit.
type MemberSpaceLimitsChangePolicyDetails struct {
	// PreviousValue : Previous team default limit value in bytes. Might be
	// missing due to historical data gap.
	PreviousValue uint64 `json:"previous_value,omitempty"`
	// NewValue : New team default limit value in bytes. Might be missing due to
	// historical data gap.
	NewValue uint64 `json:"new_value,omitempty"`
}

// NewMemberSpaceLimitsChangePolicyDetails returns a new MemberSpaceLimitsChangePolicyDetails instance
func NewMemberSpaceLimitsChangePolicyDetails() *MemberSpaceLimitsChangePolicyDetails {
	s := new(MemberSpaceLimitsChangePolicyDetails)
	return s
}

// MemberSpaceLimitsChangePolicyType : has no documentation (yet)
type MemberSpaceLimitsChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsChangePolicyType returns a new MemberSpaceLimitsChangePolicyType instance
func NewMemberSpaceLimitsChangePolicyType(Description string) *MemberSpaceLimitsChangePolicyType {
	s := new(MemberSpaceLimitsChangePolicyType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsChangeStatusDetails : Changed space limit status.
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

// MemberSpaceLimitsChangeStatusType : has no documentation (yet)
type MemberSpaceLimitsChangeStatusType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsChangeStatusType returns a new MemberSpaceLimitsChangeStatusType instance
func NewMemberSpaceLimitsChangeStatusType(Description string) *MemberSpaceLimitsChangeStatusType {
	s := new(MemberSpaceLimitsChangeStatusType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsRemoveCustomQuotaDetails : Removed custom member space
// limit.
type MemberSpaceLimitsRemoveCustomQuotaDetails struct {
}

// NewMemberSpaceLimitsRemoveCustomQuotaDetails returns a new MemberSpaceLimitsRemoveCustomQuotaDetails instance
func NewMemberSpaceLimitsRemoveCustomQuotaDetails() *MemberSpaceLimitsRemoveCustomQuotaDetails {
	s := new(MemberSpaceLimitsRemoveCustomQuotaDetails)
	return s
}

// MemberSpaceLimitsRemoveCustomQuotaType : has no documentation (yet)
type MemberSpaceLimitsRemoveCustomQuotaType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsRemoveCustomQuotaType returns a new MemberSpaceLimitsRemoveCustomQuotaType instance
func NewMemberSpaceLimitsRemoveCustomQuotaType(Description string) *MemberSpaceLimitsRemoveCustomQuotaType {
	s := new(MemberSpaceLimitsRemoveCustomQuotaType)
	s.Description = Description
	return s
}

// MemberSpaceLimitsRemoveExceptionDetails : Removed members from member space
// limit exception list.
type MemberSpaceLimitsRemoveExceptionDetails struct {
}

// NewMemberSpaceLimitsRemoveExceptionDetails returns a new MemberSpaceLimitsRemoveExceptionDetails instance
func NewMemberSpaceLimitsRemoveExceptionDetails() *MemberSpaceLimitsRemoveExceptionDetails {
	s := new(MemberSpaceLimitsRemoveExceptionDetails)
	return s
}

// MemberSpaceLimitsRemoveExceptionType : has no documentation (yet)
type MemberSpaceLimitsRemoveExceptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSpaceLimitsRemoveExceptionType returns a new MemberSpaceLimitsRemoveExceptionType instance
func NewMemberSpaceLimitsRemoveExceptionType(Description string) *MemberSpaceLimitsRemoveExceptionType {
	s := new(MemberSpaceLimitsRemoveExceptionType)
	s.Description = Description
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

// MemberSuggestDetails : Suggested person to add to team.
type MemberSuggestDetails struct {
	// SuggestedMembers : suggested users emails.
	SuggestedMembers []string `json:"suggested_members"`
}

// NewMemberSuggestDetails returns a new MemberSuggestDetails instance
func NewMemberSuggestDetails(SuggestedMembers []string) *MemberSuggestDetails {
	s := new(MemberSuggestDetails)
	s.SuggestedMembers = SuggestedMembers
	return s
}

// MemberSuggestType : has no documentation (yet)
type MemberSuggestType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSuggestType returns a new MemberSuggestType instance
func NewMemberSuggestType(Description string) *MemberSuggestType {
	s := new(MemberSuggestType)
	s.Description = Description
	return s
}

// MemberSuggestionsChangePolicyDetails : Enabled/disabled option for team
// members to suggest people to add to team.
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

// MemberSuggestionsChangePolicyType : has no documentation (yet)
type MemberSuggestionsChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberSuggestionsChangePolicyType returns a new MemberSuggestionsChangePolicyType instance
func NewMemberSuggestionsChangePolicyType(Description string) *MemberSuggestionsChangePolicyType {
	s := new(MemberSuggestionsChangePolicyType)
	s.Description = Description
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

// MemberTransferAccountContentsDetails : Transferred contents of deleted member
// account to another member.
type MemberTransferAccountContentsDetails struct {
}

// NewMemberTransferAccountContentsDetails returns a new MemberTransferAccountContentsDetails instance
func NewMemberTransferAccountContentsDetails() *MemberTransferAccountContentsDetails {
	s := new(MemberTransferAccountContentsDetails)
	return s
}

// MemberTransferAccountContentsType : has no documentation (yet)
type MemberTransferAccountContentsType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMemberTransferAccountContentsType returns a new MemberTransferAccountContentsType instance
func NewMemberTransferAccountContentsType(Description string) *MemberTransferAccountContentsType {
	s := new(MemberTransferAccountContentsType)
	s.Description = Description
	return s
}

// MicrosoftOfficeAddinChangePolicyDetails : Enabled/disabled Microsoft Office
// add-in.
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

// MicrosoftOfficeAddinChangePolicyType : has no documentation (yet)
type MicrosoftOfficeAddinChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewMicrosoftOfficeAddinChangePolicyType returns a new MicrosoftOfficeAddinChangePolicyType instance
func NewMicrosoftOfficeAddinChangePolicyType(Description string) *MicrosoftOfficeAddinChangePolicyType {
	s := new(MicrosoftOfficeAddinChangePolicyType)
	s.Description = Description
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

// MissingDetails : An indication that an error occurred while retrieving the
// event. Some attributes of the event may be omitted as a result.
type MissingDetails struct {
	// SourceEventFields : All the data that could be retrieved and converted
	// from the source event.
	SourceEventFields string `json:"source_event_fields,omitempty"`
}

// NewMissingDetails returns a new MissingDetails instance
func NewMissingDetails() *MissingDetails {
	s := new(MissingDetails)
	return s
}

// MobileDeviceSessionLogInfo : Information about linked Dropbox mobile client
// sessions
type MobileDeviceSessionLogInfo struct {
	DeviceSessionLogInfo
	// SessionInfo : Mobile session unique id. Might be missing due to
	// historical data gap.
	SessionInfo *MobileSessionLogInfo `json:"session_info,omitempty"`
	// DeviceName : The device name.
	DeviceName string `json:"device_name"`
	// ClientType : The mobile application type.
	ClientType *team.MobileClientPlatform `json:"client_type"`
	// ClientVersion : The Dropbox client version.
	ClientVersion string `json:"client_version,omitempty"`
	// OsVersion : The hosting OS version.
	OsVersion string `json:"os_version,omitempty"`
	// LastCarrier : last carrier used by the device.
	LastCarrier string `json:"last_carrier,omitempty"`
}

// NewMobileDeviceSessionLogInfo returns a new MobileDeviceSessionLogInfo instance
func NewMobileDeviceSessionLogInfo(DeviceName string, ClientType *team.MobileClientPlatform) *MobileDeviceSessionLogInfo {
	s := new(MobileDeviceSessionLogInfo)
	s.DeviceName = DeviceName
	s.ClientType = ClientType
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

// NetworkControlChangePolicyDetails : Enabled/disabled network control.
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

// NetworkControlChangePolicyType : has no documentation (yet)
type NetworkControlChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewNetworkControlChangePolicyType returns a new NetworkControlChangePolicyType instance
func NewNetworkControlChangePolicyType(Description string) *NetworkControlChangePolicyType {
	s := new(NetworkControlChangePolicyType)
	s.Description = Description
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

// NoteAclInviteOnlyDetails : Changed Paper doc to invite-only.
type NoteAclInviteOnlyDetails struct {
}

// NewNoteAclInviteOnlyDetails returns a new NoteAclInviteOnlyDetails instance
func NewNoteAclInviteOnlyDetails() *NoteAclInviteOnlyDetails {
	s := new(NoteAclInviteOnlyDetails)
	return s
}

// NoteAclInviteOnlyType : has no documentation (yet)
type NoteAclInviteOnlyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewNoteAclInviteOnlyType returns a new NoteAclInviteOnlyType instance
func NewNoteAclInviteOnlyType(Description string) *NoteAclInviteOnlyType {
	s := new(NoteAclInviteOnlyType)
	s.Description = Description
	return s
}

// NoteAclLinkDetails : Changed Paper doc to link-accessible.
type NoteAclLinkDetails struct {
}

// NewNoteAclLinkDetails returns a new NoteAclLinkDetails instance
func NewNoteAclLinkDetails() *NoteAclLinkDetails {
	s := new(NoteAclLinkDetails)
	return s
}

// NoteAclLinkType : has no documentation (yet)
type NoteAclLinkType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewNoteAclLinkType returns a new NoteAclLinkType instance
func NewNoteAclLinkType(Description string) *NoteAclLinkType {
	s := new(NoteAclLinkType)
	s.Description = Description
	return s
}

// NoteAclTeamLinkDetails : Changed Paper doc to link-accessible for team.
type NoteAclTeamLinkDetails struct {
}

// NewNoteAclTeamLinkDetails returns a new NoteAclTeamLinkDetails instance
func NewNoteAclTeamLinkDetails() *NoteAclTeamLinkDetails {
	s := new(NoteAclTeamLinkDetails)
	return s
}

// NoteAclTeamLinkType : has no documentation (yet)
type NoteAclTeamLinkType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewNoteAclTeamLinkType returns a new NoteAclTeamLinkType instance
func NewNoteAclTeamLinkType(Description string) *NoteAclTeamLinkType {
	s := new(NoteAclTeamLinkType)
	s.Description = Description
	return s
}

// NoteShareReceiveDetails : Shared received Paper doc.
type NoteShareReceiveDetails struct {
}

// NewNoteShareReceiveDetails returns a new NoteShareReceiveDetails instance
func NewNoteShareReceiveDetails() *NoteShareReceiveDetails {
	s := new(NoteShareReceiveDetails)
	return s
}

// NoteShareReceiveType : has no documentation (yet)
type NoteShareReceiveType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewNoteShareReceiveType returns a new NoteShareReceiveType instance
func NewNoteShareReceiveType(Description string) *NoteShareReceiveType {
	s := new(NoteShareReceiveType)
	s.Description = Description
	return s
}

// NoteSharedDetails : Shared Paper doc.
type NoteSharedDetails struct {
}

// NewNoteSharedDetails returns a new NoteSharedDetails instance
func NewNoteSharedDetails() *NoteSharedDetails {
	s := new(NoteSharedDetails)
	return s
}

// NoteSharedType : has no documentation (yet)
type NoteSharedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewNoteSharedType returns a new NoteSharedType instance
func NewNoteSharedType(Description string) *NoteSharedType {
	s := new(NoteSharedType)
	s.Description = Description
	return s
}

// OpenNoteSharedDetails : Opened shared Paper doc.
type OpenNoteSharedDetails struct {
}

// NewOpenNoteSharedDetails returns a new OpenNoteSharedDetails instance
func NewOpenNoteSharedDetails() *OpenNoteSharedDetails {
	s := new(OpenNoteSharedDetails)
	return s
}

// OpenNoteSharedType : has no documentation (yet)
type OpenNoteSharedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewOpenNoteSharedType returns a new OpenNoteSharedType instance
func NewOpenNoteSharedType(Description string) *OpenNoteSharedType {
	s := new(OpenNoteSharedType)
	s.Description = Description
	return s
}

// OriginLogInfo : The origin from which the actor performed the action.
type OriginLogInfo struct {
	// GeoLocation : Geographic location details.
	GeoLocation *GeoLocationLogInfo `json:"geo_location,omitempty"`
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

// PaperAdminExportStartDetails : Exported all team Paper docs.
type PaperAdminExportStartDetails struct {
}

// NewPaperAdminExportStartDetails returns a new PaperAdminExportStartDetails instance
func NewPaperAdminExportStartDetails() *PaperAdminExportStartDetails {
	s := new(PaperAdminExportStartDetails)
	return s
}

// PaperAdminExportStartType : has no documentation (yet)
type PaperAdminExportStartType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperAdminExportStartType returns a new PaperAdminExportStartType instance
func NewPaperAdminExportStartType(Description string) *PaperAdminExportStartType {
	s := new(PaperAdminExportStartType)
	s.Description = Description
	return s
}

// PaperChangeDeploymentPolicyDetails : Changed whether Dropbox Paper, when
// enabled, is deployed to all members or to specific members.
type PaperChangeDeploymentPolicyDetails struct {
	// NewValue : New Dropbox Paper deployment policy.
	NewValue *team_policies.PaperDeploymentPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Paper deployment policy. Might be
	// missing due to historical data gap.
	PreviousValue *team_policies.PaperDeploymentPolicy `json:"previous_value,omitempty"`
}

// NewPaperChangeDeploymentPolicyDetails returns a new PaperChangeDeploymentPolicyDetails instance
func NewPaperChangeDeploymentPolicyDetails(NewValue *team_policies.PaperDeploymentPolicy) *PaperChangeDeploymentPolicyDetails {
	s := new(PaperChangeDeploymentPolicyDetails)
	s.NewValue = NewValue
	return s
}

// PaperChangeDeploymentPolicyType : has no documentation (yet)
type PaperChangeDeploymentPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperChangeDeploymentPolicyType returns a new PaperChangeDeploymentPolicyType instance
func NewPaperChangeDeploymentPolicyType(Description string) *PaperChangeDeploymentPolicyType {
	s := new(PaperChangeDeploymentPolicyType)
	s.Description = Description
	return s
}

// PaperChangeMemberLinkPolicyDetails : Changed whether non-members can view
// Paper docs with link.
type PaperChangeMemberLinkPolicyDetails struct {
	// NewValue : New paper external link accessibility policy.
	NewValue *PaperMemberPolicy `json:"new_value"`
}

// NewPaperChangeMemberLinkPolicyDetails returns a new PaperChangeMemberLinkPolicyDetails instance
func NewPaperChangeMemberLinkPolicyDetails(NewValue *PaperMemberPolicy) *PaperChangeMemberLinkPolicyDetails {
	s := new(PaperChangeMemberLinkPolicyDetails)
	s.NewValue = NewValue
	return s
}

// PaperChangeMemberLinkPolicyType : has no documentation (yet)
type PaperChangeMemberLinkPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperChangeMemberLinkPolicyType returns a new PaperChangeMemberLinkPolicyType instance
func NewPaperChangeMemberLinkPolicyType(Description string) *PaperChangeMemberLinkPolicyType {
	s := new(PaperChangeMemberLinkPolicyType)
	s.Description = Description
	return s
}

// PaperChangeMemberPolicyDetails : Changed whether members can share Paper docs
// outside team, and if docs are accessible only by team members or anyone by
// default.
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

// PaperChangeMemberPolicyType : has no documentation (yet)
type PaperChangeMemberPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperChangeMemberPolicyType returns a new PaperChangeMemberPolicyType instance
func NewPaperChangeMemberPolicyType(Description string) *PaperChangeMemberPolicyType {
	s := new(PaperChangeMemberPolicyType)
	s.Description = Description
	return s
}

// PaperChangePolicyDetails : Enabled/disabled Dropbox Paper for team.
type PaperChangePolicyDetails struct {
	// NewValue : New Dropbox Paper policy.
	NewValue *team_policies.PaperEnabledPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Paper policy. Might be missing due to
	// historical data gap.
	PreviousValue *team_policies.PaperEnabledPolicy `json:"previous_value,omitempty"`
}

// NewPaperChangePolicyDetails returns a new PaperChangePolicyDetails instance
func NewPaperChangePolicyDetails(NewValue *team_policies.PaperEnabledPolicy) *PaperChangePolicyDetails {
	s := new(PaperChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// PaperChangePolicyType : has no documentation (yet)
type PaperChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperChangePolicyType returns a new PaperChangePolicyType instance
func NewPaperChangePolicyType(Description string) *PaperChangePolicyType {
	s := new(PaperChangePolicyType)
	s.Description = Description
	return s
}

// PaperContentAddMemberDetails : Added team member to Paper doc/folder.
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

// PaperContentAddMemberType : has no documentation (yet)
type PaperContentAddMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentAddMemberType returns a new PaperContentAddMemberType instance
func NewPaperContentAddMemberType(Description string) *PaperContentAddMemberType {
	s := new(PaperContentAddMemberType)
	s.Description = Description
	return s
}

// PaperContentAddToFolderDetails : Added Paper doc/folder to folder.
type PaperContentAddToFolderDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// ParentAssetIndex : Parent asset position in the Assets list.
	ParentAssetIndex uint64 `json:"parent_asset_index"`
}

// NewPaperContentAddToFolderDetails returns a new PaperContentAddToFolderDetails instance
func NewPaperContentAddToFolderDetails(EventUuid string, TargetAssetIndex uint64, ParentAssetIndex uint64) *PaperContentAddToFolderDetails {
	s := new(PaperContentAddToFolderDetails)
	s.EventUuid = EventUuid
	s.TargetAssetIndex = TargetAssetIndex
	s.ParentAssetIndex = ParentAssetIndex
	return s
}

// PaperContentAddToFolderType : has no documentation (yet)
type PaperContentAddToFolderType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentAddToFolderType returns a new PaperContentAddToFolderType instance
func NewPaperContentAddToFolderType(Description string) *PaperContentAddToFolderType {
	s := new(PaperContentAddToFolderType)
	s.Description = Description
	return s
}

// PaperContentArchiveDetails : Archived Paper doc/folder.
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

// PaperContentArchiveType : has no documentation (yet)
type PaperContentArchiveType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentArchiveType returns a new PaperContentArchiveType instance
func NewPaperContentArchiveType(Description string) *PaperContentArchiveType {
	s := new(PaperContentArchiveType)
	s.Description = Description
	return s
}

// PaperContentCreateDetails : Created Paper doc/folder.
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

// PaperContentCreateType : has no documentation (yet)
type PaperContentCreateType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentCreateType returns a new PaperContentCreateType instance
func NewPaperContentCreateType(Description string) *PaperContentCreateType {
	s := new(PaperContentCreateType)
	s.Description = Description
	return s
}

// PaperContentPermanentlyDeleteDetails : Permanently deleted Paper doc/folder.
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

// PaperContentPermanentlyDeleteType : has no documentation (yet)
type PaperContentPermanentlyDeleteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentPermanentlyDeleteType returns a new PaperContentPermanentlyDeleteType instance
func NewPaperContentPermanentlyDeleteType(Description string) *PaperContentPermanentlyDeleteType {
	s := new(PaperContentPermanentlyDeleteType)
	s.Description = Description
	return s
}

// PaperContentRemoveFromFolderDetails : Removed Paper doc/folder from folder.
type PaperContentRemoveFromFolderDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// ParentAssetIndex : Parent asset position in the Assets list.
	ParentAssetIndex uint64 `json:"parent_asset_index"`
}

// NewPaperContentRemoveFromFolderDetails returns a new PaperContentRemoveFromFolderDetails instance
func NewPaperContentRemoveFromFolderDetails(EventUuid string, TargetAssetIndex uint64, ParentAssetIndex uint64) *PaperContentRemoveFromFolderDetails {
	s := new(PaperContentRemoveFromFolderDetails)
	s.EventUuid = EventUuid
	s.TargetAssetIndex = TargetAssetIndex
	s.ParentAssetIndex = ParentAssetIndex
	return s
}

// PaperContentRemoveFromFolderType : has no documentation (yet)
type PaperContentRemoveFromFolderType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentRemoveFromFolderType returns a new PaperContentRemoveFromFolderType instance
func NewPaperContentRemoveFromFolderType(Description string) *PaperContentRemoveFromFolderType {
	s := new(PaperContentRemoveFromFolderType)
	s.Description = Description
	return s
}

// PaperContentRemoveMemberDetails : Removed team member from Paper doc/folder.
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

// PaperContentRemoveMemberType : has no documentation (yet)
type PaperContentRemoveMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentRemoveMemberType returns a new PaperContentRemoveMemberType instance
func NewPaperContentRemoveMemberType(Description string) *PaperContentRemoveMemberType {
	s := new(PaperContentRemoveMemberType)
	s.Description = Description
	return s
}

// PaperContentRenameDetails : Renamed Paper doc/folder.
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

// PaperContentRenameType : has no documentation (yet)
type PaperContentRenameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentRenameType returns a new PaperContentRenameType instance
func NewPaperContentRenameType(Description string) *PaperContentRenameType {
	s := new(PaperContentRenameType)
	s.Description = Description
	return s
}

// PaperContentRestoreDetails : Restored archived Paper doc/folder.
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

// PaperContentRestoreType : has no documentation (yet)
type PaperContentRestoreType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperContentRestoreType returns a new PaperContentRestoreType instance
func NewPaperContentRestoreType(Description string) *PaperContentRestoreType {
	s := new(PaperContentRestoreType)
	s.Description = Description
	return s
}

// PaperDocAddCommentDetails : Added Paper doc comment.
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

// PaperDocAddCommentType : has no documentation (yet)
type PaperDocAddCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocAddCommentType returns a new PaperDocAddCommentType instance
func NewPaperDocAddCommentType(Description string) *PaperDocAddCommentType {
	s := new(PaperDocAddCommentType)
	s.Description = Description
	return s
}

// PaperDocChangeMemberRoleDetails : Changed team member permissions for Paper
// doc.
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

// PaperDocChangeMemberRoleType : has no documentation (yet)
type PaperDocChangeMemberRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocChangeMemberRoleType returns a new PaperDocChangeMemberRoleType instance
func NewPaperDocChangeMemberRoleType(Description string) *PaperDocChangeMemberRoleType {
	s := new(PaperDocChangeMemberRoleType)
	s.Description = Description
	return s
}

// PaperDocChangeSharingPolicyDetails : Changed sharing setting for Paper doc.
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

// PaperDocChangeSharingPolicyType : has no documentation (yet)
type PaperDocChangeSharingPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocChangeSharingPolicyType returns a new PaperDocChangeSharingPolicyType instance
func NewPaperDocChangeSharingPolicyType(Description string) *PaperDocChangeSharingPolicyType {
	s := new(PaperDocChangeSharingPolicyType)
	s.Description = Description
	return s
}

// PaperDocChangeSubscriptionDetails : Followed/unfollowed Paper doc.
type PaperDocChangeSubscriptionDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// NewSubscriptionLevel : New doc subscription level.
	NewSubscriptionLevel string `json:"new_subscription_level"`
	// PreviousSubscriptionLevel : Previous doc subscription level. Might be
	// missing due to historical data gap.
	PreviousSubscriptionLevel string `json:"previous_subscription_level,omitempty"`
}

// NewPaperDocChangeSubscriptionDetails returns a new PaperDocChangeSubscriptionDetails instance
func NewPaperDocChangeSubscriptionDetails(EventUuid string, NewSubscriptionLevel string) *PaperDocChangeSubscriptionDetails {
	s := new(PaperDocChangeSubscriptionDetails)
	s.EventUuid = EventUuid
	s.NewSubscriptionLevel = NewSubscriptionLevel
	return s
}

// PaperDocChangeSubscriptionType : has no documentation (yet)
type PaperDocChangeSubscriptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocChangeSubscriptionType returns a new PaperDocChangeSubscriptionType instance
func NewPaperDocChangeSubscriptionType(Description string) *PaperDocChangeSubscriptionType {
	s := new(PaperDocChangeSubscriptionType)
	s.Description = Description
	return s
}

// PaperDocDeleteCommentDetails : Deleted Paper doc comment.
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

// PaperDocDeleteCommentType : has no documentation (yet)
type PaperDocDeleteCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocDeleteCommentType returns a new PaperDocDeleteCommentType instance
func NewPaperDocDeleteCommentType(Description string) *PaperDocDeleteCommentType {
	s := new(PaperDocDeleteCommentType)
	s.Description = Description
	return s
}

// PaperDocDeletedDetails : Archived Paper doc.
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

// PaperDocDeletedType : has no documentation (yet)
type PaperDocDeletedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocDeletedType returns a new PaperDocDeletedType instance
func NewPaperDocDeletedType(Description string) *PaperDocDeletedType {
	s := new(PaperDocDeletedType)
	s.Description = Description
	return s
}

// PaperDocDownloadDetails : Downloaded Paper doc in specific format.
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

// PaperDocDownloadType : has no documentation (yet)
type PaperDocDownloadType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocDownloadType returns a new PaperDocDownloadType instance
func NewPaperDocDownloadType(Description string) *PaperDocDownloadType {
	s := new(PaperDocDownloadType)
	s.Description = Description
	return s
}

// PaperDocEditCommentDetails : Edited Paper doc comment.
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

// PaperDocEditCommentType : has no documentation (yet)
type PaperDocEditCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocEditCommentType returns a new PaperDocEditCommentType instance
func NewPaperDocEditCommentType(Description string) *PaperDocEditCommentType {
	s := new(PaperDocEditCommentType)
	s.Description = Description
	return s
}

// PaperDocEditDetails : Edited Paper doc.
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

// PaperDocEditType : has no documentation (yet)
type PaperDocEditType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocEditType returns a new PaperDocEditType instance
func NewPaperDocEditType(Description string) *PaperDocEditType {
	s := new(PaperDocEditType)
	s.Description = Description
	return s
}

// PaperDocFollowedDetails : Followed Paper doc.
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

// PaperDocFollowedType : has no documentation (yet)
type PaperDocFollowedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocFollowedType returns a new PaperDocFollowedType instance
func NewPaperDocFollowedType(Description string) *PaperDocFollowedType {
	s := new(PaperDocFollowedType)
	s.Description = Description
	return s
}

// PaperDocMentionDetails : Mentioned team member in Paper doc.
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

// PaperDocMentionType : has no documentation (yet)
type PaperDocMentionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocMentionType returns a new PaperDocMentionType instance
func NewPaperDocMentionType(Description string) *PaperDocMentionType {
	s := new(PaperDocMentionType)
	s.Description = Description
	return s
}

// PaperDocRequestAccessDetails : Requested access to Paper doc.
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

// PaperDocRequestAccessType : has no documentation (yet)
type PaperDocRequestAccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocRequestAccessType returns a new PaperDocRequestAccessType instance
func NewPaperDocRequestAccessType(Description string) *PaperDocRequestAccessType {
	s := new(PaperDocRequestAccessType)
	s.Description = Description
	return s
}

// PaperDocResolveCommentDetails : Resolved Paper doc comment.
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

// PaperDocResolveCommentType : has no documentation (yet)
type PaperDocResolveCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocResolveCommentType returns a new PaperDocResolveCommentType instance
func NewPaperDocResolveCommentType(Description string) *PaperDocResolveCommentType {
	s := new(PaperDocResolveCommentType)
	s.Description = Description
	return s
}

// PaperDocRevertDetails : Restored Paper doc to previous version.
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

// PaperDocRevertType : has no documentation (yet)
type PaperDocRevertType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocRevertType returns a new PaperDocRevertType instance
func NewPaperDocRevertType(Description string) *PaperDocRevertType {
	s := new(PaperDocRevertType)
	s.Description = Description
	return s
}

// PaperDocSlackShareDetails : Shared Paper doc via Slack.
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

// PaperDocSlackShareType : has no documentation (yet)
type PaperDocSlackShareType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocSlackShareType returns a new PaperDocSlackShareType instance
func NewPaperDocSlackShareType(Description string) *PaperDocSlackShareType {
	s := new(PaperDocSlackShareType)
	s.Description = Description
	return s
}

// PaperDocTeamInviteDetails : Shared Paper doc with team member.
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

// PaperDocTeamInviteType : has no documentation (yet)
type PaperDocTeamInviteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocTeamInviteType returns a new PaperDocTeamInviteType instance
func NewPaperDocTeamInviteType(Description string) *PaperDocTeamInviteType {
	s := new(PaperDocTeamInviteType)
	s.Description = Description
	return s
}

// PaperDocTrashedDetails : Deleted Paper doc.
type PaperDocTrashedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocTrashedDetails returns a new PaperDocTrashedDetails instance
func NewPaperDocTrashedDetails(EventUuid string) *PaperDocTrashedDetails {
	s := new(PaperDocTrashedDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocTrashedType : has no documentation (yet)
type PaperDocTrashedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocTrashedType returns a new PaperDocTrashedType instance
func NewPaperDocTrashedType(Description string) *PaperDocTrashedType {
	s := new(PaperDocTrashedType)
	s.Description = Description
	return s
}

// PaperDocUnresolveCommentDetails : Unresolved Paper doc comment.
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

// PaperDocUnresolveCommentType : has no documentation (yet)
type PaperDocUnresolveCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocUnresolveCommentType returns a new PaperDocUnresolveCommentType instance
func NewPaperDocUnresolveCommentType(Description string) *PaperDocUnresolveCommentType {
	s := new(PaperDocUnresolveCommentType)
	s.Description = Description
	return s
}

// PaperDocUntrashedDetails : Restored Paper doc.
type PaperDocUntrashedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperDocUntrashedDetails returns a new PaperDocUntrashedDetails instance
func NewPaperDocUntrashedDetails(EventUuid string) *PaperDocUntrashedDetails {
	s := new(PaperDocUntrashedDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperDocUntrashedType : has no documentation (yet)
type PaperDocUntrashedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocUntrashedType returns a new PaperDocUntrashedType instance
func NewPaperDocUntrashedType(Description string) *PaperDocUntrashedType {
	s := new(PaperDocUntrashedType)
	s.Description = Description
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

// PaperDocViewType : has no documentation (yet)
type PaperDocViewType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperDocViewType returns a new PaperDocViewType instance
func NewPaperDocViewType(Description string) *PaperDocViewType {
	s := new(PaperDocViewType)
	s.Description = Description
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

// PaperEnabledUsersGroupAdditionDetails : Added users to Paper-enabled users
// list.
type PaperEnabledUsersGroupAdditionDetails struct {
}

// NewPaperEnabledUsersGroupAdditionDetails returns a new PaperEnabledUsersGroupAdditionDetails instance
func NewPaperEnabledUsersGroupAdditionDetails() *PaperEnabledUsersGroupAdditionDetails {
	s := new(PaperEnabledUsersGroupAdditionDetails)
	return s
}

// PaperEnabledUsersGroupAdditionType : has no documentation (yet)
type PaperEnabledUsersGroupAdditionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperEnabledUsersGroupAdditionType returns a new PaperEnabledUsersGroupAdditionType instance
func NewPaperEnabledUsersGroupAdditionType(Description string) *PaperEnabledUsersGroupAdditionType {
	s := new(PaperEnabledUsersGroupAdditionType)
	s.Description = Description
	return s
}

// PaperEnabledUsersGroupRemovalDetails : Removed users from Paper-enabled users
// list.
type PaperEnabledUsersGroupRemovalDetails struct {
}

// NewPaperEnabledUsersGroupRemovalDetails returns a new PaperEnabledUsersGroupRemovalDetails instance
func NewPaperEnabledUsersGroupRemovalDetails() *PaperEnabledUsersGroupRemovalDetails {
	s := new(PaperEnabledUsersGroupRemovalDetails)
	return s
}

// PaperEnabledUsersGroupRemovalType : has no documentation (yet)
type PaperEnabledUsersGroupRemovalType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperEnabledUsersGroupRemovalType returns a new PaperEnabledUsersGroupRemovalType instance
func NewPaperEnabledUsersGroupRemovalType(Description string) *PaperEnabledUsersGroupRemovalType {
	s := new(PaperEnabledUsersGroupRemovalType)
	s.Description = Description
	return s
}

// PaperExternalViewAllowDetails : Changed Paper external sharing setting to
// anyone.
type PaperExternalViewAllowDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperExternalViewAllowDetails returns a new PaperExternalViewAllowDetails instance
func NewPaperExternalViewAllowDetails(EventUuid string) *PaperExternalViewAllowDetails {
	s := new(PaperExternalViewAllowDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperExternalViewAllowType : has no documentation (yet)
type PaperExternalViewAllowType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperExternalViewAllowType returns a new PaperExternalViewAllowType instance
func NewPaperExternalViewAllowType(Description string) *PaperExternalViewAllowType {
	s := new(PaperExternalViewAllowType)
	s.Description = Description
	return s
}

// PaperExternalViewDefaultTeamDetails : Changed Paper external sharing setting
// to default team.
type PaperExternalViewDefaultTeamDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperExternalViewDefaultTeamDetails returns a new PaperExternalViewDefaultTeamDetails instance
func NewPaperExternalViewDefaultTeamDetails(EventUuid string) *PaperExternalViewDefaultTeamDetails {
	s := new(PaperExternalViewDefaultTeamDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperExternalViewDefaultTeamType : has no documentation (yet)
type PaperExternalViewDefaultTeamType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperExternalViewDefaultTeamType returns a new PaperExternalViewDefaultTeamType instance
func NewPaperExternalViewDefaultTeamType(Description string) *PaperExternalViewDefaultTeamType {
	s := new(PaperExternalViewDefaultTeamType)
	s.Description = Description
	return s
}

// PaperExternalViewForbidDetails : Changed Paper external sharing setting to
// team-only.
type PaperExternalViewForbidDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewPaperExternalViewForbidDetails returns a new PaperExternalViewForbidDetails instance
func NewPaperExternalViewForbidDetails(EventUuid string) *PaperExternalViewForbidDetails {
	s := new(PaperExternalViewForbidDetails)
	s.EventUuid = EventUuid
	return s
}

// PaperExternalViewForbidType : has no documentation (yet)
type PaperExternalViewForbidType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperExternalViewForbidType returns a new PaperExternalViewForbidType instance
func NewPaperExternalViewForbidType(Description string) *PaperExternalViewForbidType {
	s := new(PaperExternalViewForbidType)
	s.Description = Description
	return s
}

// PaperFolderChangeSubscriptionDetails : Followed/unfollowed Paper folder.
type PaperFolderChangeSubscriptionDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// NewSubscriptionLevel : New folder subscription level.
	NewSubscriptionLevel string `json:"new_subscription_level"`
	// PreviousSubscriptionLevel : Previous folder subscription level. Might be
	// missing due to historical data gap.
	PreviousSubscriptionLevel string `json:"previous_subscription_level,omitempty"`
}

// NewPaperFolderChangeSubscriptionDetails returns a new PaperFolderChangeSubscriptionDetails instance
func NewPaperFolderChangeSubscriptionDetails(EventUuid string, NewSubscriptionLevel string) *PaperFolderChangeSubscriptionDetails {
	s := new(PaperFolderChangeSubscriptionDetails)
	s.EventUuid = EventUuid
	s.NewSubscriptionLevel = NewSubscriptionLevel
	return s
}

// PaperFolderChangeSubscriptionType : has no documentation (yet)
type PaperFolderChangeSubscriptionType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperFolderChangeSubscriptionType returns a new PaperFolderChangeSubscriptionType instance
func NewPaperFolderChangeSubscriptionType(Description string) *PaperFolderChangeSubscriptionType {
	s := new(PaperFolderChangeSubscriptionType)
	s.Description = Description
	return s
}

// PaperFolderDeletedDetails : Archived Paper folder.
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

// PaperFolderDeletedType : has no documentation (yet)
type PaperFolderDeletedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperFolderDeletedType returns a new PaperFolderDeletedType instance
func NewPaperFolderDeletedType(Description string) *PaperFolderDeletedType {
	s := new(PaperFolderDeletedType)
	s.Description = Description
	return s
}

// PaperFolderFollowedDetails : Followed Paper folder.
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

// PaperFolderFollowedType : has no documentation (yet)
type PaperFolderFollowedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperFolderFollowedType returns a new PaperFolderFollowedType instance
func NewPaperFolderFollowedType(Description string) *PaperFolderFollowedType {
	s := new(PaperFolderFollowedType)
	s.Description = Description
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

// PaperFolderTeamInviteDetails : Shared Paper folder with member.
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

// PaperFolderTeamInviteType : has no documentation (yet)
type PaperFolderTeamInviteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPaperFolderTeamInviteType returns a new PaperFolderTeamInviteType instance
func NewPaperFolderTeamInviteType(Description string) *PaperFolderTeamInviteType {
	s := new(PaperFolderTeamInviteType)
	s.Description = Description
	return s
}

// PaperMemberPolicy : Policy for controlling if team members can share Paper
// documents externally.
type PaperMemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperMemberPolicy
const (
	PaperMemberPolicyAnyoneWithLink          = "anyone_with_link"
	PaperMemberPolicyOnlyTeam                = "only_team"
	PaperMemberPolicyTeamAndExplicitlyShared = "team_and_explicitly_shared"
	PaperMemberPolicyOther                   = "other"
)

// ParticipantLogInfo : A user or group
type ParticipantLogInfo struct {
	dropbox.Tagged
	// User : A user with a Dropbox account.
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
		// User : A user with a Dropbox account.
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

// PasswordChangeType : has no documentation (yet)
type PasswordChangeType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPasswordChangeType returns a new PasswordChangeType instance
func NewPasswordChangeType(Description string) *PasswordChangeType {
	s := new(PasswordChangeType)
	s.Description = Description
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

// PasswordResetAllType : has no documentation (yet)
type PasswordResetAllType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPasswordResetAllType returns a new PasswordResetAllType instance
func NewPasswordResetAllType(Description string) *PasswordResetAllType {
	s := new(PasswordResetAllType)
	s.Description = Description
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

// PasswordResetType : has no documentation (yet)
type PasswordResetType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPasswordResetType returns a new PasswordResetType instance
func NewPasswordResetType(Description string) *PasswordResetType {
	s := new(PasswordResetType)
	s.Description = Description
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

// PermanentDeleteChangePolicyDetails : Enabled/disabled ability of team members
// to permanently delete content.
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

// PermanentDeleteChangePolicyType : has no documentation (yet)
type PermanentDeleteChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewPermanentDeleteChangePolicyType returns a new PermanentDeleteChangePolicyType instance
func NewPermanentDeleteChangePolicyType(Description string) *PermanentDeleteChangePolicyType {
	s := new(PermanentDeleteChangePolicyType)
	s.Description = Description
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
	// SrcAssetIndex : Source asset position in the Assets list.
	SrcAssetIndex uint64 `json:"src_asset_index"`
	// DestAssetIndex : Destination asset position in the Assets list.
	DestAssetIndex uint64 `json:"dest_asset_index"`
}

// NewRelocateAssetReferencesLogInfo returns a new RelocateAssetReferencesLogInfo instance
func NewRelocateAssetReferencesLogInfo(SrcAssetIndex uint64, DestAssetIndex uint64) *RelocateAssetReferencesLogInfo {
	s := new(RelocateAssetReferencesLogInfo)
	s.SrcAssetIndex = SrcAssetIndex
	s.DestAssetIndex = DestAssetIndex
	return s
}

// ResellerLogInfo : Reseller information.
type ResellerLogInfo struct {
	// ResellerName : Reseller name.
	ResellerName string `json:"reseller_name"`
	// ResellerEmail : Reseller email.
	ResellerEmail string `json:"reseller_email"`
}

// NewResellerLogInfo returns a new ResellerLogInfo instance
func NewResellerLogInfo(ResellerName string, ResellerEmail string) *ResellerLogInfo {
	s := new(ResellerLogInfo)
	s.ResellerName = ResellerName
	s.ResellerEmail = ResellerEmail
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

// ResellerSupportSessionEndType : has no documentation (yet)
type ResellerSupportSessionEndType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewResellerSupportSessionEndType returns a new ResellerSupportSessionEndType instance
func NewResellerSupportSessionEndType(Description string) *ResellerSupportSessionEndType {
	s := new(ResellerSupportSessionEndType)
	s.Description = Description
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

// ResellerSupportSessionStartType : has no documentation (yet)
type ResellerSupportSessionStartType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewResellerSupportSessionStartType returns a new ResellerSupportSessionStartType instance
func NewResellerSupportSessionStartType(Description string) *ResellerSupportSessionStartType {
	s := new(ResellerSupportSessionStartType)
	s.Description = Description
	return s
}

// SecondaryMailsPolicy : has no documentation (yet)
type SecondaryMailsPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SecondaryMailsPolicy
const (
	SecondaryMailsPolicyDisabled = "disabled"
	SecondaryMailsPolicyEnabled  = "enabled"
	SecondaryMailsPolicyOther    = "other"
)

// SecondaryMailsPolicyChangedDetails : Secondary mails policy changed.
type SecondaryMailsPolicyChangedDetails struct {
	// PreviousValue : Previous secondary mails policy.
	PreviousValue *SecondaryMailsPolicy `json:"previous_value"`
	// NewValue : New secondary mails policy.
	NewValue *SecondaryMailsPolicy `json:"new_value"`
}

// NewSecondaryMailsPolicyChangedDetails returns a new SecondaryMailsPolicyChangedDetails instance
func NewSecondaryMailsPolicyChangedDetails(PreviousValue *SecondaryMailsPolicy, NewValue *SecondaryMailsPolicy) *SecondaryMailsPolicyChangedDetails {
	s := new(SecondaryMailsPolicyChangedDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// SecondaryMailsPolicyChangedType : has no documentation (yet)
type SecondaryMailsPolicyChangedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSecondaryMailsPolicyChangedType returns a new SecondaryMailsPolicyChangedType instance
func NewSecondaryMailsPolicyChangedType(Description string) *SecondaryMailsPolicyChangedType {
	s := new(SecondaryMailsPolicyChangedType)
	s.Description = Description
	return s
}

// SfAddGroupDetails : Added team to shared folder.
type SfAddGroupDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
	// TeamName : Team name.
	TeamName string `json:"team_name"`
}

// NewSfAddGroupDetails returns a new SfAddGroupDetails instance
func NewSfAddGroupDetails(TargetAssetIndex uint64, OriginalFolderName string, TeamName string) *SfAddGroupDetails {
	s := new(SfAddGroupDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	s.TeamName = TeamName
	return s
}

// SfAddGroupType : has no documentation (yet)
type SfAddGroupType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfAddGroupType returns a new SfAddGroupType instance
func NewSfAddGroupType(Description string) *SfAddGroupType {
	s := new(SfAddGroupType)
	s.Description = Description
	return s
}

// SfAllowNonMembersToViewSharedLinksDetails : Allowed non-collaborators to view
// links to files in shared folder.
type SfAllowNonMembersToViewSharedLinksDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharedFolderType : Shared folder type. Might be missing due to historical
	// data gap.
	SharedFolderType string `json:"shared_folder_type,omitempty"`
}

// NewSfAllowNonMembersToViewSharedLinksDetails returns a new SfAllowNonMembersToViewSharedLinksDetails instance
func NewSfAllowNonMembersToViewSharedLinksDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfAllowNonMembersToViewSharedLinksDetails {
	s := new(SfAllowNonMembersToViewSharedLinksDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfAllowNonMembersToViewSharedLinksType : has no documentation (yet)
type SfAllowNonMembersToViewSharedLinksType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfAllowNonMembersToViewSharedLinksType returns a new SfAllowNonMembersToViewSharedLinksType instance
func NewSfAllowNonMembersToViewSharedLinksType(Description string) *SfAllowNonMembersToViewSharedLinksType {
	s := new(SfAllowNonMembersToViewSharedLinksType)
	s.Description = Description
	return s
}

// SfExternalInviteWarnDetails : Set team members to see warning before sharing
// folders outside team.
type SfExternalInviteWarnDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// NewSharingPermission : New sharing permission. Might be missing due to
	// historical data gap.
	NewSharingPermission string `json:"new_sharing_permission,omitempty"`
	// PreviousSharingPermission : Previous sharing permission. Might be missing
	// due to historical data gap.
	PreviousSharingPermission string `json:"previous_sharing_permission,omitempty"`
}

// NewSfExternalInviteWarnDetails returns a new SfExternalInviteWarnDetails instance
func NewSfExternalInviteWarnDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfExternalInviteWarnDetails {
	s := new(SfExternalInviteWarnDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfExternalInviteWarnType : has no documentation (yet)
type SfExternalInviteWarnType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfExternalInviteWarnType returns a new SfExternalInviteWarnType instance
func NewSfExternalInviteWarnType(Description string) *SfExternalInviteWarnType {
	s := new(SfExternalInviteWarnType)
	s.Description = Description
	return s
}

// SfFbInviteChangeRoleDetails : Changed Facebook user's role in shared folder.
type SfFbInviteChangeRoleDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// PreviousSharingPermission : Previous sharing permission. Might be missing
	// due to historical data gap.
	PreviousSharingPermission string `json:"previous_sharing_permission,omitempty"`
	// NewSharingPermission : New sharing permission. Might be missing due to
	// historical data gap.
	NewSharingPermission string `json:"new_sharing_permission,omitempty"`
}

// NewSfFbInviteChangeRoleDetails returns a new SfFbInviteChangeRoleDetails instance
func NewSfFbInviteChangeRoleDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfFbInviteChangeRoleDetails {
	s := new(SfFbInviteChangeRoleDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfFbInviteChangeRoleType : has no documentation (yet)
type SfFbInviteChangeRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfFbInviteChangeRoleType returns a new SfFbInviteChangeRoleType instance
func NewSfFbInviteChangeRoleType(Description string) *SfFbInviteChangeRoleType {
	s := new(SfFbInviteChangeRoleType)
	s.Description = Description
	return s
}

// SfFbInviteDetails : Invited Facebook users to shared folder.
type SfFbInviteDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
}

// NewSfFbInviteDetails returns a new SfFbInviteDetails instance
func NewSfFbInviteDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfFbInviteDetails {
	s := new(SfFbInviteDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfFbInviteType : has no documentation (yet)
type SfFbInviteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfFbInviteType returns a new SfFbInviteType instance
func NewSfFbInviteType(Description string) *SfFbInviteType {
	s := new(SfFbInviteType)
	s.Description = Description
	return s
}

// SfFbUninviteDetails : Uninvited Facebook user from shared folder.
type SfFbUninviteDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfFbUninviteDetails returns a new SfFbUninviteDetails instance
func NewSfFbUninviteDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfFbUninviteDetails {
	s := new(SfFbUninviteDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfFbUninviteType : has no documentation (yet)
type SfFbUninviteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfFbUninviteType returns a new SfFbUninviteType instance
func NewSfFbUninviteType(Description string) *SfFbUninviteType {
	s := new(SfFbUninviteType)
	s.Description = Description
	return s
}

// SfInviteGroupDetails : Invited group to shared folder.
type SfInviteGroupDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
}

// NewSfInviteGroupDetails returns a new SfInviteGroupDetails instance
func NewSfInviteGroupDetails(TargetAssetIndex uint64) *SfInviteGroupDetails {
	s := new(SfInviteGroupDetails)
	s.TargetAssetIndex = TargetAssetIndex
	return s
}

// SfInviteGroupType : has no documentation (yet)
type SfInviteGroupType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfInviteGroupType returns a new SfInviteGroupType instance
func NewSfInviteGroupType(Description string) *SfInviteGroupType {
	s := new(SfInviteGroupType)
	s.Description = Description
	return s
}

// SfTeamGrantAccessDetails : Granted access to shared folder.
type SfTeamGrantAccessDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamGrantAccessDetails returns a new SfTeamGrantAccessDetails instance
func NewSfTeamGrantAccessDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfTeamGrantAccessDetails {
	s := new(SfTeamGrantAccessDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamGrantAccessType : has no documentation (yet)
type SfTeamGrantAccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfTeamGrantAccessType returns a new SfTeamGrantAccessType instance
func NewSfTeamGrantAccessType(Description string) *SfTeamGrantAccessType {
	s := new(SfTeamGrantAccessType)
	s.Description = Description
	return s
}

// SfTeamInviteChangeRoleDetails : Changed team member's role in shared folder.
type SfTeamInviteChangeRoleDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
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
func NewSfTeamInviteChangeRoleDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfTeamInviteChangeRoleDetails {
	s := new(SfTeamInviteChangeRoleDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamInviteChangeRoleType : has no documentation (yet)
type SfTeamInviteChangeRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfTeamInviteChangeRoleType returns a new SfTeamInviteChangeRoleType instance
func NewSfTeamInviteChangeRoleType(Description string) *SfTeamInviteChangeRoleType {
	s := new(SfTeamInviteChangeRoleType)
	s.Description = Description
	return s
}

// SfTeamInviteDetails : Invited team members to shared folder.
type SfTeamInviteDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
}

// NewSfTeamInviteDetails returns a new SfTeamInviteDetails instance
func NewSfTeamInviteDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfTeamInviteDetails {
	s := new(SfTeamInviteDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamInviteType : has no documentation (yet)
type SfTeamInviteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfTeamInviteType returns a new SfTeamInviteType instance
func NewSfTeamInviteType(Description string) *SfTeamInviteType {
	s := new(SfTeamInviteType)
	s.Description = Description
	return s
}

// SfTeamJoinDetails : Joined team member's shared folder.
type SfTeamJoinDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamJoinDetails returns a new SfTeamJoinDetails instance
func NewSfTeamJoinDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfTeamJoinDetails {
	s := new(SfTeamJoinDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamJoinFromOobLinkDetails : Joined team member's shared folder from link.
type SfTeamJoinFromOobLinkDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
	// TokenKey : Shared link token key.
	TokenKey string `json:"token_key,omitempty"`
	// SharingPermission : Sharing permission. Might be missing due to
	// historical data gap.
	SharingPermission string `json:"sharing_permission,omitempty"`
}

// NewSfTeamJoinFromOobLinkDetails returns a new SfTeamJoinFromOobLinkDetails instance
func NewSfTeamJoinFromOobLinkDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfTeamJoinFromOobLinkDetails {
	s := new(SfTeamJoinFromOobLinkDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamJoinFromOobLinkType : has no documentation (yet)
type SfTeamJoinFromOobLinkType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfTeamJoinFromOobLinkType returns a new SfTeamJoinFromOobLinkType instance
func NewSfTeamJoinFromOobLinkType(Description string) *SfTeamJoinFromOobLinkType {
	s := new(SfTeamJoinFromOobLinkType)
	s.Description = Description
	return s
}

// SfTeamJoinType : has no documentation (yet)
type SfTeamJoinType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfTeamJoinType returns a new SfTeamJoinType instance
func NewSfTeamJoinType(Description string) *SfTeamJoinType {
	s := new(SfTeamJoinType)
	s.Description = Description
	return s
}

// SfTeamUninviteDetails : Unshared folder with team member.
type SfTeamUninviteDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
	// OriginalFolderName : Original shared folder name.
	OriginalFolderName string `json:"original_folder_name"`
}

// NewSfTeamUninviteDetails returns a new SfTeamUninviteDetails instance
func NewSfTeamUninviteDetails(TargetAssetIndex uint64, OriginalFolderName string) *SfTeamUninviteDetails {
	s := new(SfTeamUninviteDetails)
	s.TargetAssetIndex = TargetAssetIndex
	s.OriginalFolderName = OriginalFolderName
	return s
}

// SfTeamUninviteType : has no documentation (yet)
type SfTeamUninviteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSfTeamUninviteType returns a new SfTeamUninviteType instance
func NewSfTeamUninviteType(Description string) *SfTeamUninviteType {
	s := new(SfTeamUninviteType)
	s.Description = Description
	return s
}

// SharedContentAddInviteesDetails : Invited user to Dropbox and added them to
// shared file/folder.
type SharedContentAddInviteesDetails struct {
	// SharedContentAccessLevel : Shared content access level.
	SharedContentAccessLevel *sharing.AccessLevel `json:"shared_content_access_level"`
	// Invitees : A list of invitees.
	Invitees []string `json:"invitees"`
}

// NewSharedContentAddInviteesDetails returns a new SharedContentAddInviteesDetails instance
func NewSharedContentAddInviteesDetails(SharedContentAccessLevel *sharing.AccessLevel, Invitees []string) *SharedContentAddInviteesDetails {
	s := new(SharedContentAddInviteesDetails)
	s.SharedContentAccessLevel = SharedContentAccessLevel
	s.Invitees = Invitees
	return s
}

// SharedContentAddInviteesType : has no documentation (yet)
type SharedContentAddInviteesType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentAddInviteesType returns a new SharedContentAddInviteesType instance
func NewSharedContentAddInviteesType(Description string) *SharedContentAddInviteesType {
	s := new(SharedContentAddInviteesType)
	s.Description = Description
	return s
}

// SharedContentAddLinkExpiryDetails : Added expiration date to link for shared
// file/folder.
type SharedContentAddLinkExpiryDetails struct {
	// NewValue : New shared content link expiration date. Might be missing due
	// to historical data gap.
	NewValue time.Time `json:"new_value,omitempty"`
}

// NewSharedContentAddLinkExpiryDetails returns a new SharedContentAddLinkExpiryDetails instance
func NewSharedContentAddLinkExpiryDetails() *SharedContentAddLinkExpiryDetails {
	s := new(SharedContentAddLinkExpiryDetails)
	return s
}

// SharedContentAddLinkExpiryType : has no documentation (yet)
type SharedContentAddLinkExpiryType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentAddLinkExpiryType returns a new SharedContentAddLinkExpiryType instance
func NewSharedContentAddLinkExpiryType(Description string) *SharedContentAddLinkExpiryType {
	s := new(SharedContentAddLinkExpiryType)
	s.Description = Description
	return s
}

// SharedContentAddLinkPasswordDetails : Added password to link for shared
// file/folder.
type SharedContentAddLinkPasswordDetails struct {
}

// NewSharedContentAddLinkPasswordDetails returns a new SharedContentAddLinkPasswordDetails instance
func NewSharedContentAddLinkPasswordDetails() *SharedContentAddLinkPasswordDetails {
	s := new(SharedContentAddLinkPasswordDetails)
	return s
}

// SharedContentAddLinkPasswordType : has no documentation (yet)
type SharedContentAddLinkPasswordType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentAddLinkPasswordType returns a new SharedContentAddLinkPasswordType instance
func NewSharedContentAddLinkPasswordType(Description string) *SharedContentAddLinkPasswordType {
	s := new(SharedContentAddLinkPasswordType)
	s.Description = Description
	return s
}

// SharedContentAddMemberDetails : Added users and/or groups to shared
// file/folder.
type SharedContentAddMemberDetails struct {
	// SharedContentAccessLevel : Shared content access level.
	SharedContentAccessLevel *sharing.AccessLevel `json:"shared_content_access_level"`
}

// NewSharedContentAddMemberDetails returns a new SharedContentAddMemberDetails instance
func NewSharedContentAddMemberDetails(SharedContentAccessLevel *sharing.AccessLevel) *SharedContentAddMemberDetails {
	s := new(SharedContentAddMemberDetails)
	s.SharedContentAccessLevel = SharedContentAccessLevel
	return s
}

// SharedContentAddMemberType : has no documentation (yet)
type SharedContentAddMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentAddMemberType returns a new SharedContentAddMemberType instance
func NewSharedContentAddMemberType(Description string) *SharedContentAddMemberType {
	s := new(SharedContentAddMemberType)
	s.Description = Description
	return s
}

// SharedContentChangeDownloadsPolicyDetails : Changed whether members can
// download shared file/folder.
type SharedContentChangeDownloadsPolicyDetails struct {
	// NewValue : New downloads policy.
	NewValue *DownloadPolicyType `json:"new_value"`
	// PreviousValue : Previous downloads policy. Might be missing due to
	// historical data gap.
	PreviousValue *DownloadPolicyType `json:"previous_value,omitempty"`
}

// NewSharedContentChangeDownloadsPolicyDetails returns a new SharedContentChangeDownloadsPolicyDetails instance
func NewSharedContentChangeDownloadsPolicyDetails(NewValue *DownloadPolicyType) *SharedContentChangeDownloadsPolicyDetails {
	s := new(SharedContentChangeDownloadsPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharedContentChangeDownloadsPolicyType : has no documentation (yet)
type SharedContentChangeDownloadsPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeDownloadsPolicyType returns a new SharedContentChangeDownloadsPolicyType instance
func NewSharedContentChangeDownloadsPolicyType(Description string) *SharedContentChangeDownloadsPolicyType {
	s := new(SharedContentChangeDownloadsPolicyType)
	s.Description = Description
	return s
}

// SharedContentChangeInviteeRoleDetails : Changed access type of invitee to
// shared file/folder before invite was accepted.
type SharedContentChangeInviteeRoleDetails struct {
	// PreviousAccessLevel : Previous access level. Might be missing due to
	// historical data gap.
	PreviousAccessLevel *sharing.AccessLevel `json:"previous_access_level,omitempty"`
	// NewAccessLevel : New access level.
	NewAccessLevel *sharing.AccessLevel `json:"new_access_level"`
	// Invitee : The invitee whose role was changed.
	Invitee string `json:"invitee"`
}

// NewSharedContentChangeInviteeRoleDetails returns a new SharedContentChangeInviteeRoleDetails instance
func NewSharedContentChangeInviteeRoleDetails(NewAccessLevel *sharing.AccessLevel, Invitee string) *SharedContentChangeInviteeRoleDetails {
	s := new(SharedContentChangeInviteeRoleDetails)
	s.NewAccessLevel = NewAccessLevel
	s.Invitee = Invitee
	return s
}

// SharedContentChangeInviteeRoleType : has no documentation (yet)
type SharedContentChangeInviteeRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeInviteeRoleType returns a new SharedContentChangeInviteeRoleType instance
func NewSharedContentChangeInviteeRoleType(Description string) *SharedContentChangeInviteeRoleType {
	s := new(SharedContentChangeInviteeRoleType)
	s.Description = Description
	return s
}

// SharedContentChangeLinkAudienceDetails : Changed link audience of shared
// file/folder.
type SharedContentChangeLinkAudienceDetails struct {
	// NewValue : New link audience value.
	NewValue *sharing.LinkAudience `json:"new_value"`
	// PreviousValue : Previous link audience value.
	PreviousValue *sharing.LinkAudience `json:"previous_value,omitempty"`
}

// NewSharedContentChangeLinkAudienceDetails returns a new SharedContentChangeLinkAudienceDetails instance
func NewSharedContentChangeLinkAudienceDetails(NewValue *sharing.LinkAudience) *SharedContentChangeLinkAudienceDetails {
	s := new(SharedContentChangeLinkAudienceDetails)
	s.NewValue = NewValue
	return s
}

// SharedContentChangeLinkAudienceType : has no documentation (yet)
type SharedContentChangeLinkAudienceType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeLinkAudienceType returns a new SharedContentChangeLinkAudienceType instance
func NewSharedContentChangeLinkAudienceType(Description string) *SharedContentChangeLinkAudienceType {
	s := new(SharedContentChangeLinkAudienceType)
	s.Description = Description
	return s
}

// SharedContentChangeLinkExpiryDetails : Changed link expiration of shared
// file/folder.
type SharedContentChangeLinkExpiryDetails struct {
	// NewValue : New shared content link expiration date. Might be missing due
	// to historical data gap.
	NewValue time.Time `json:"new_value,omitempty"`
	// PreviousValue : Previous shared content link expiration date. Might be
	// missing due to historical data gap.
	PreviousValue time.Time `json:"previous_value,omitempty"`
}

// NewSharedContentChangeLinkExpiryDetails returns a new SharedContentChangeLinkExpiryDetails instance
func NewSharedContentChangeLinkExpiryDetails() *SharedContentChangeLinkExpiryDetails {
	s := new(SharedContentChangeLinkExpiryDetails)
	return s
}

// SharedContentChangeLinkExpiryType : has no documentation (yet)
type SharedContentChangeLinkExpiryType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeLinkExpiryType returns a new SharedContentChangeLinkExpiryType instance
func NewSharedContentChangeLinkExpiryType(Description string) *SharedContentChangeLinkExpiryType {
	s := new(SharedContentChangeLinkExpiryType)
	s.Description = Description
	return s
}

// SharedContentChangeLinkPasswordDetails : Changed link password of shared
// file/folder.
type SharedContentChangeLinkPasswordDetails struct {
}

// NewSharedContentChangeLinkPasswordDetails returns a new SharedContentChangeLinkPasswordDetails instance
func NewSharedContentChangeLinkPasswordDetails() *SharedContentChangeLinkPasswordDetails {
	s := new(SharedContentChangeLinkPasswordDetails)
	return s
}

// SharedContentChangeLinkPasswordType : has no documentation (yet)
type SharedContentChangeLinkPasswordType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeLinkPasswordType returns a new SharedContentChangeLinkPasswordType instance
func NewSharedContentChangeLinkPasswordType(Description string) *SharedContentChangeLinkPasswordType {
	s := new(SharedContentChangeLinkPasswordType)
	s.Description = Description
	return s
}

// SharedContentChangeMemberRoleDetails : Changed access type of shared
// file/folder member.
type SharedContentChangeMemberRoleDetails struct {
	// PreviousAccessLevel : Previous access level. Might be missing due to
	// historical data gap.
	PreviousAccessLevel *sharing.AccessLevel `json:"previous_access_level,omitempty"`
	// NewAccessLevel : New access level.
	NewAccessLevel *sharing.AccessLevel `json:"new_access_level"`
}

// NewSharedContentChangeMemberRoleDetails returns a new SharedContentChangeMemberRoleDetails instance
func NewSharedContentChangeMemberRoleDetails(NewAccessLevel *sharing.AccessLevel) *SharedContentChangeMemberRoleDetails {
	s := new(SharedContentChangeMemberRoleDetails)
	s.NewAccessLevel = NewAccessLevel
	return s
}

// SharedContentChangeMemberRoleType : has no documentation (yet)
type SharedContentChangeMemberRoleType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeMemberRoleType returns a new SharedContentChangeMemberRoleType instance
func NewSharedContentChangeMemberRoleType(Description string) *SharedContentChangeMemberRoleType {
	s := new(SharedContentChangeMemberRoleType)
	s.Description = Description
	return s
}

// SharedContentChangeViewerInfoPolicyDetails : Changed whether members can see
// who viewed shared file/folder.
type SharedContentChangeViewerInfoPolicyDetails struct {
	// NewValue : New viewer info policy.
	NewValue *sharing.ViewerInfoPolicy `json:"new_value"`
	// PreviousValue : Previous view info policy. Might be missing due to
	// historical data gap.
	PreviousValue *sharing.ViewerInfoPolicy `json:"previous_value,omitempty"`
}

// NewSharedContentChangeViewerInfoPolicyDetails returns a new SharedContentChangeViewerInfoPolicyDetails instance
func NewSharedContentChangeViewerInfoPolicyDetails(NewValue *sharing.ViewerInfoPolicy) *SharedContentChangeViewerInfoPolicyDetails {
	s := new(SharedContentChangeViewerInfoPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharedContentChangeViewerInfoPolicyType : has no documentation (yet)
type SharedContentChangeViewerInfoPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentChangeViewerInfoPolicyType returns a new SharedContentChangeViewerInfoPolicyType instance
func NewSharedContentChangeViewerInfoPolicyType(Description string) *SharedContentChangeViewerInfoPolicyType {
	s := new(SharedContentChangeViewerInfoPolicyType)
	s.Description = Description
	return s
}

// SharedContentClaimInvitationDetails : Acquired membership of shared
// file/folder by accepting invite.
type SharedContentClaimInvitationDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link,omitempty"`
}

// NewSharedContentClaimInvitationDetails returns a new SharedContentClaimInvitationDetails instance
func NewSharedContentClaimInvitationDetails() *SharedContentClaimInvitationDetails {
	s := new(SharedContentClaimInvitationDetails)
	return s
}

// SharedContentClaimInvitationType : has no documentation (yet)
type SharedContentClaimInvitationType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentClaimInvitationType returns a new SharedContentClaimInvitationType instance
func NewSharedContentClaimInvitationType(Description string) *SharedContentClaimInvitationType {
	s := new(SharedContentClaimInvitationType)
	s.Description = Description
	return s
}

// SharedContentCopyDetails : Copied shared file/folder to own Dropbox.
type SharedContentCopyDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link"`
	// SharedContentOwner : The shared content owner.
	SharedContentOwner IsUserLogInfo `json:"shared_content_owner,omitempty"`
	// SharedContentAccessLevel : Shared content access level.
	SharedContentAccessLevel *sharing.AccessLevel `json:"shared_content_access_level"`
	// DestinationPath : The path where the member saved the content.
	DestinationPath string `json:"destination_path"`
}

// NewSharedContentCopyDetails returns a new SharedContentCopyDetails instance
func NewSharedContentCopyDetails(SharedContentLink string, SharedContentAccessLevel *sharing.AccessLevel, DestinationPath string) *SharedContentCopyDetails {
	s := new(SharedContentCopyDetails)
	s.SharedContentLink = SharedContentLink
	s.SharedContentAccessLevel = SharedContentAccessLevel
	s.DestinationPath = DestinationPath
	return s
}

// SharedContentCopyType : has no documentation (yet)
type SharedContentCopyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentCopyType returns a new SharedContentCopyType instance
func NewSharedContentCopyType(Description string) *SharedContentCopyType {
	s := new(SharedContentCopyType)
	s.Description = Description
	return s
}

// SharedContentDownloadDetails : Downloaded shared file/folder.
type SharedContentDownloadDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link"`
	// SharedContentOwner : The shared content owner.
	SharedContentOwner IsUserLogInfo `json:"shared_content_owner,omitempty"`
	// SharedContentAccessLevel : Shared content access level.
	SharedContentAccessLevel *sharing.AccessLevel `json:"shared_content_access_level"`
}

// NewSharedContentDownloadDetails returns a new SharedContentDownloadDetails instance
func NewSharedContentDownloadDetails(SharedContentLink string, SharedContentAccessLevel *sharing.AccessLevel) *SharedContentDownloadDetails {
	s := new(SharedContentDownloadDetails)
	s.SharedContentLink = SharedContentLink
	s.SharedContentAccessLevel = SharedContentAccessLevel
	return s
}

// SharedContentDownloadType : has no documentation (yet)
type SharedContentDownloadType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentDownloadType returns a new SharedContentDownloadType instance
func NewSharedContentDownloadType(Description string) *SharedContentDownloadType {
	s := new(SharedContentDownloadType)
	s.Description = Description
	return s
}

// SharedContentRelinquishMembershipDetails : Left shared file/folder.
type SharedContentRelinquishMembershipDetails struct {
}

// NewSharedContentRelinquishMembershipDetails returns a new SharedContentRelinquishMembershipDetails instance
func NewSharedContentRelinquishMembershipDetails() *SharedContentRelinquishMembershipDetails {
	s := new(SharedContentRelinquishMembershipDetails)
	return s
}

// SharedContentRelinquishMembershipType : has no documentation (yet)
type SharedContentRelinquishMembershipType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentRelinquishMembershipType returns a new SharedContentRelinquishMembershipType instance
func NewSharedContentRelinquishMembershipType(Description string) *SharedContentRelinquishMembershipType {
	s := new(SharedContentRelinquishMembershipType)
	s.Description = Description
	return s
}

// SharedContentRemoveInviteesDetails : Removed invitee from shared file/folder
// before invite was accepted.
type SharedContentRemoveInviteesDetails struct {
	// Invitees : A list of invitees.
	Invitees []string `json:"invitees"`
}

// NewSharedContentRemoveInviteesDetails returns a new SharedContentRemoveInviteesDetails instance
func NewSharedContentRemoveInviteesDetails(Invitees []string) *SharedContentRemoveInviteesDetails {
	s := new(SharedContentRemoveInviteesDetails)
	s.Invitees = Invitees
	return s
}

// SharedContentRemoveInviteesType : has no documentation (yet)
type SharedContentRemoveInviteesType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentRemoveInviteesType returns a new SharedContentRemoveInviteesType instance
func NewSharedContentRemoveInviteesType(Description string) *SharedContentRemoveInviteesType {
	s := new(SharedContentRemoveInviteesType)
	s.Description = Description
	return s
}

// SharedContentRemoveLinkExpiryDetails : Removed link expiration date of shared
// file/folder.
type SharedContentRemoveLinkExpiryDetails struct {
	// PreviousValue : Previous shared content link expiration date. Might be
	// missing due to historical data gap.
	PreviousValue time.Time `json:"previous_value,omitempty"`
}

// NewSharedContentRemoveLinkExpiryDetails returns a new SharedContentRemoveLinkExpiryDetails instance
func NewSharedContentRemoveLinkExpiryDetails() *SharedContentRemoveLinkExpiryDetails {
	s := new(SharedContentRemoveLinkExpiryDetails)
	return s
}

// SharedContentRemoveLinkExpiryType : has no documentation (yet)
type SharedContentRemoveLinkExpiryType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentRemoveLinkExpiryType returns a new SharedContentRemoveLinkExpiryType instance
func NewSharedContentRemoveLinkExpiryType(Description string) *SharedContentRemoveLinkExpiryType {
	s := new(SharedContentRemoveLinkExpiryType)
	s.Description = Description
	return s
}

// SharedContentRemoveLinkPasswordDetails : Removed link password of shared
// file/folder.
type SharedContentRemoveLinkPasswordDetails struct {
}

// NewSharedContentRemoveLinkPasswordDetails returns a new SharedContentRemoveLinkPasswordDetails instance
func NewSharedContentRemoveLinkPasswordDetails() *SharedContentRemoveLinkPasswordDetails {
	s := new(SharedContentRemoveLinkPasswordDetails)
	return s
}

// SharedContentRemoveLinkPasswordType : has no documentation (yet)
type SharedContentRemoveLinkPasswordType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentRemoveLinkPasswordType returns a new SharedContentRemoveLinkPasswordType instance
func NewSharedContentRemoveLinkPasswordType(Description string) *SharedContentRemoveLinkPasswordType {
	s := new(SharedContentRemoveLinkPasswordType)
	s.Description = Description
	return s
}

// SharedContentRemoveMemberDetails : Removed user/group from shared
// file/folder.
type SharedContentRemoveMemberDetails struct {
	// SharedContentAccessLevel : Shared content access level.
	SharedContentAccessLevel *sharing.AccessLevel `json:"shared_content_access_level,omitempty"`
}

// NewSharedContentRemoveMemberDetails returns a new SharedContentRemoveMemberDetails instance
func NewSharedContentRemoveMemberDetails() *SharedContentRemoveMemberDetails {
	s := new(SharedContentRemoveMemberDetails)
	return s
}

// SharedContentRemoveMemberType : has no documentation (yet)
type SharedContentRemoveMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentRemoveMemberType returns a new SharedContentRemoveMemberType instance
func NewSharedContentRemoveMemberType(Description string) *SharedContentRemoveMemberType {
	s := new(SharedContentRemoveMemberType)
	s.Description = Description
	return s
}

// SharedContentRequestAccessDetails : Requested access to shared file/folder.
type SharedContentRequestAccessDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link,omitempty"`
}

// NewSharedContentRequestAccessDetails returns a new SharedContentRequestAccessDetails instance
func NewSharedContentRequestAccessDetails() *SharedContentRequestAccessDetails {
	s := new(SharedContentRequestAccessDetails)
	return s
}

// SharedContentRequestAccessType : has no documentation (yet)
type SharedContentRequestAccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentRequestAccessType returns a new SharedContentRequestAccessType instance
func NewSharedContentRequestAccessType(Description string) *SharedContentRequestAccessType {
	s := new(SharedContentRequestAccessType)
	s.Description = Description
	return s
}

// SharedContentUnshareDetails : Unshared file/folder by clearing membership and
// turning off link.
type SharedContentUnshareDetails struct {
}

// NewSharedContentUnshareDetails returns a new SharedContentUnshareDetails instance
func NewSharedContentUnshareDetails() *SharedContentUnshareDetails {
	s := new(SharedContentUnshareDetails)
	return s
}

// SharedContentUnshareType : has no documentation (yet)
type SharedContentUnshareType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentUnshareType returns a new SharedContentUnshareType instance
func NewSharedContentUnshareType(Description string) *SharedContentUnshareType {
	s := new(SharedContentUnshareType)
	s.Description = Description
	return s
}

// SharedContentViewDetails : Previewed shared file/folder.
type SharedContentViewDetails struct {
	// SharedContentLink : Shared content link.
	SharedContentLink string `json:"shared_content_link"`
	// SharedContentOwner : The shared content owner.
	SharedContentOwner IsUserLogInfo `json:"shared_content_owner,omitempty"`
	// SharedContentAccessLevel : Shared content access level.
	SharedContentAccessLevel *sharing.AccessLevel `json:"shared_content_access_level"`
}

// NewSharedContentViewDetails returns a new SharedContentViewDetails instance
func NewSharedContentViewDetails(SharedContentLink string, SharedContentAccessLevel *sharing.AccessLevel) *SharedContentViewDetails {
	s := new(SharedContentViewDetails)
	s.SharedContentLink = SharedContentLink
	s.SharedContentAccessLevel = SharedContentAccessLevel
	return s
}

// SharedContentViewType : has no documentation (yet)
type SharedContentViewType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedContentViewType returns a new SharedContentViewType instance
func NewSharedContentViewType(Description string) *SharedContentViewType {
	s := new(SharedContentViewType)
	s.Description = Description
	return s
}

// SharedFolderChangeLinkPolicyDetails : Changed who can access shared folder
// via link.
type SharedFolderChangeLinkPolicyDetails struct {
	// NewValue : New shared folder link policy.
	NewValue *sharing.SharedLinkPolicy `json:"new_value"`
	// PreviousValue : Previous shared folder link policy. Might be missing due
	// to historical data gap.
	PreviousValue *sharing.SharedLinkPolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeLinkPolicyDetails returns a new SharedFolderChangeLinkPolicyDetails instance
func NewSharedFolderChangeLinkPolicyDetails(NewValue *sharing.SharedLinkPolicy) *SharedFolderChangeLinkPolicyDetails {
	s := new(SharedFolderChangeLinkPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeLinkPolicyType : has no documentation (yet)
type SharedFolderChangeLinkPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderChangeLinkPolicyType returns a new SharedFolderChangeLinkPolicyType instance
func NewSharedFolderChangeLinkPolicyType(Description string) *SharedFolderChangeLinkPolicyType {
	s := new(SharedFolderChangeLinkPolicyType)
	s.Description = Description
	return s
}

// SharedFolderChangeMembersInheritancePolicyDetails : Changed whether shared
// folder inherits members from parent folder.
type SharedFolderChangeMembersInheritancePolicyDetails struct {
	// NewValue : New member inheritance policy.
	NewValue *SharedFolderMembersInheritancePolicy `json:"new_value"`
	// PreviousValue : Previous member inheritance policy. Might be missing due
	// to historical data gap.
	PreviousValue *SharedFolderMembersInheritancePolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeMembersInheritancePolicyDetails returns a new SharedFolderChangeMembersInheritancePolicyDetails instance
func NewSharedFolderChangeMembersInheritancePolicyDetails(NewValue *SharedFolderMembersInheritancePolicy) *SharedFolderChangeMembersInheritancePolicyDetails {
	s := new(SharedFolderChangeMembersInheritancePolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeMembersInheritancePolicyType : has no documentation (yet)
type SharedFolderChangeMembersInheritancePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderChangeMembersInheritancePolicyType returns a new SharedFolderChangeMembersInheritancePolicyType instance
func NewSharedFolderChangeMembersInheritancePolicyType(Description string) *SharedFolderChangeMembersInheritancePolicyType {
	s := new(SharedFolderChangeMembersInheritancePolicyType)
	s.Description = Description
	return s
}

// SharedFolderChangeMembersManagementPolicyDetails : Changed who can add/remove
// members of shared folder.
type SharedFolderChangeMembersManagementPolicyDetails struct {
	// NewValue : New members management policy.
	NewValue *sharing.AclUpdatePolicy `json:"new_value"`
	// PreviousValue : Previous members management policy. Might be missing due
	// to historical data gap.
	PreviousValue *sharing.AclUpdatePolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeMembersManagementPolicyDetails returns a new SharedFolderChangeMembersManagementPolicyDetails instance
func NewSharedFolderChangeMembersManagementPolicyDetails(NewValue *sharing.AclUpdatePolicy) *SharedFolderChangeMembersManagementPolicyDetails {
	s := new(SharedFolderChangeMembersManagementPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeMembersManagementPolicyType : has no documentation (yet)
type SharedFolderChangeMembersManagementPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderChangeMembersManagementPolicyType returns a new SharedFolderChangeMembersManagementPolicyType instance
func NewSharedFolderChangeMembersManagementPolicyType(Description string) *SharedFolderChangeMembersManagementPolicyType {
	s := new(SharedFolderChangeMembersManagementPolicyType)
	s.Description = Description
	return s
}

// SharedFolderChangeMembersPolicyDetails : Changed who can become member of
// shared folder.
type SharedFolderChangeMembersPolicyDetails struct {
	// NewValue : New external invite policy.
	NewValue *sharing.MemberPolicy `json:"new_value"`
	// PreviousValue : Previous external invite policy. Might be missing due to
	// historical data gap.
	PreviousValue *sharing.MemberPolicy `json:"previous_value,omitempty"`
}

// NewSharedFolderChangeMembersPolicyDetails returns a new SharedFolderChangeMembersPolicyDetails instance
func NewSharedFolderChangeMembersPolicyDetails(NewValue *sharing.MemberPolicy) *SharedFolderChangeMembersPolicyDetails {
	s := new(SharedFolderChangeMembersPolicyDetails)
	s.NewValue = NewValue
	return s
}

// SharedFolderChangeMembersPolicyType : has no documentation (yet)
type SharedFolderChangeMembersPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderChangeMembersPolicyType returns a new SharedFolderChangeMembersPolicyType instance
func NewSharedFolderChangeMembersPolicyType(Description string) *SharedFolderChangeMembersPolicyType {
	s := new(SharedFolderChangeMembersPolicyType)
	s.Description = Description
	return s
}

// SharedFolderCreateDetails : Created shared folder.
type SharedFolderCreateDetails struct {
	// TargetNsId : Target namespace ID. Might be missing due to historical data
	// gap.
	TargetNsId string `json:"target_ns_id,omitempty"`
}

// NewSharedFolderCreateDetails returns a new SharedFolderCreateDetails instance
func NewSharedFolderCreateDetails() *SharedFolderCreateDetails {
	s := new(SharedFolderCreateDetails)
	return s
}

// SharedFolderCreateType : has no documentation (yet)
type SharedFolderCreateType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderCreateType returns a new SharedFolderCreateType instance
func NewSharedFolderCreateType(Description string) *SharedFolderCreateType {
	s := new(SharedFolderCreateType)
	s.Description = Description
	return s
}

// SharedFolderDeclineInvitationDetails : Declined team member's invite to
// shared folder.
type SharedFolderDeclineInvitationDetails struct {
}

// NewSharedFolderDeclineInvitationDetails returns a new SharedFolderDeclineInvitationDetails instance
func NewSharedFolderDeclineInvitationDetails() *SharedFolderDeclineInvitationDetails {
	s := new(SharedFolderDeclineInvitationDetails)
	return s
}

// SharedFolderDeclineInvitationType : has no documentation (yet)
type SharedFolderDeclineInvitationType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderDeclineInvitationType returns a new SharedFolderDeclineInvitationType instance
func NewSharedFolderDeclineInvitationType(Description string) *SharedFolderDeclineInvitationType {
	s := new(SharedFolderDeclineInvitationType)
	s.Description = Description
	return s
}

// SharedFolderMembersInheritancePolicy : Specifies if a shared folder inherits
// its members from the parent folder.
type SharedFolderMembersInheritancePolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderMembersInheritancePolicy
const (
	SharedFolderMembersInheritancePolicyInheritMembers     = "inherit_members"
	SharedFolderMembersInheritancePolicyDontInheritMembers = "dont_inherit_members"
	SharedFolderMembersInheritancePolicyOther              = "other"
)

// SharedFolderMountDetails : Added shared folder to own Dropbox.
type SharedFolderMountDetails struct {
}

// NewSharedFolderMountDetails returns a new SharedFolderMountDetails instance
func NewSharedFolderMountDetails() *SharedFolderMountDetails {
	s := new(SharedFolderMountDetails)
	return s
}

// SharedFolderMountType : has no documentation (yet)
type SharedFolderMountType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderMountType returns a new SharedFolderMountType instance
func NewSharedFolderMountType(Description string) *SharedFolderMountType {
	s := new(SharedFolderMountType)
	s.Description = Description
	return s
}

// SharedFolderNestDetails : Changed parent of shared folder.
type SharedFolderNestDetails struct {
	// PreviousParentNsId : Previous parent namespace ID. Might be missing due
	// to historical data gap.
	PreviousParentNsId string `json:"previous_parent_ns_id,omitempty"`
	// NewParentNsId : New parent namespace ID. Might be missing due to
	// historical data gap.
	NewParentNsId string `json:"new_parent_ns_id,omitempty"`
}

// NewSharedFolderNestDetails returns a new SharedFolderNestDetails instance
func NewSharedFolderNestDetails() *SharedFolderNestDetails {
	s := new(SharedFolderNestDetails)
	return s
}

// SharedFolderNestType : has no documentation (yet)
type SharedFolderNestType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderNestType returns a new SharedFolderNestType instance
func NewSharedFolderNestType(Description string) *SharedFolderNestType {
	s := new(SharedFolderNestType)
	s.Description = Description
	return s
}

// SharedFolderTransferOwnershipDetails : Transferred ownership of shared folder
// to another member.
type SharedFolderTransferOwnershipDetails struct {
	// PreviousOwnerEmail : The email address of the previous shared folder
	// owner.
	PreviousOwnerEmail string `json:"previous_owner_email,omitempty"`
	// NewOwnerEmail : The email address of the new shared folder owner.
	NewOwnerEmail string `json:"new_owner_email"`
}

// NewSharedFolderTransferOwnershipDetails returns a new SharedFolderTransferOwnershipDetails instance
func NewSharedFolderTransferOwnershipDetails(NewOwnerEmail string) *SharedFolderTransferOwnershipDetails {
	s := new(SharedFolderTransferOwnershipDetails)
	s.NewOwnerEmail = NewOwnerEmail
	return s
}

// SharedFolderTransferOwnershipType : has no documentation (yet)
type SharedFolderTransferOwnershipType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderTransferOwnershipType returns a new SharedFolderTransferOwnershipType instance
func NewSharedFolderTransferOwnershipType(Description string) *SharedFolderTransferOwnershipType {
	s := new(SharedFolderTransferOwnershipType)
	s.Description = Description
	return s
}

// SharedFolderUnmountDetails : Deleted shared folder from Dropbox.
type SharedFolderUnmountDetails struct {
}

// NewSharedFolderUnmountDetails returns a new SharedFolderUnmountDetails instance
func NewSharedFolderUnmountDetails() *SharedFolderUnmountDetails {
	s := new(SharedFolderUnmountDetails)
	return s
}

// SharedFolderUnmountType : has no documentation (yet)
type SharedFolderUnmountType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedFolderUnmountType returns a new SharedFolderUnmountType instance
func NewSharedFolderUnmountType(Description string) *SharedFolderUnmountType {
	s := new(SharedFolderUnmountType)
	s.Description = Description
	return s
}

// SharedLinkAccessLevel : Shared link access level.
type SharedLinkAccessLevel struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkAccessLevel
const (
	SharedLinkAccessLevelNone   = "none"
	SharedLinkAccessLevelReader = "reader"
	SharedLinkAccessLevelWriter = "writer"
	SharedLinkAccessLevelOther  = "other"
)

// SharedLinkAddExpiryDetails : Added shared link expiration date.
type SharedLinkAddExpiryDetails struct {
	// NewValue : New shared link expiration date.
	NewValue time.Time `json:"new_value"`
}

// NewSharedLinkAddExpiryDetails returns a new SharedLinkAddExpiryDetails instance
func NewSharedLinkAddExpiryDetails(NewValue time.Time) *SharedLinkAddExpiryDetails {
	s := new(SharedLinkAddExpiryDetails)
	s.NewValue = NewValue
	return s
}

// SharedLinkAddExpiryType : has no documentation (yet)
type SharedLinkAddExpiryType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkAddExpiryType returns a new SharedLinkAddExpiryType instance
func NewSharedLinkAddExpiryType(Description string) *SharedLinkAddExpiryType {
	s := new(SharedLinkAddExpiryType)
	s.Description = Description
	return s
}

// SharedLinkChangeExpiryDetails : Changed shared link expiration date.
type SharedLinkChangeExpiryDetails struct {
	// NewValue : New shared link expiration date. Might be missing due to
	// historical data gap.
	NewValue time.Time `json:"new_value,omitempty"`
	// PreviousValue : Previous shared link expiration date. Might be missing
	// due to historical data gap.
	PreviousValue time.Time `json:"previous_value,omitempty"`
}

// NewSharedLinkChangeExpiryDetails returns a new SharedLinkChangeExpiryDetails instance
func NewSharedLinkChangeExpiryDetails() *SharedLinkChangeExpiryDetails {
	s := new(SharedLinkChangeExpiryDetails)
	return s
}

// SharedLinkChangeExpiryType : has no documentation (yet)
type SharedLinkChangeExpiryType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkChangeExpiryType returns a new SharedLinkChangeExpiryType instance
func NewSharedLinkChangeExpiryType(Description string) *SharedLinkChangeExpiryType {
	s := new(SharedLinkChangeExpiryType)
	s.Description = Description
	return s
}

// SharedLinkChangeVisibilityDetails : Changed visibility of shared link.
type SharedLinkChangeVisibilityDetails struct {
	// NewValue : New shared link visibility.
	NewValue *SharedLinkVisibility `json:"new_value"`
	// PreviousValue : Previous shared link visibility. Might be missing due to
	// historical data gap.
	PreviousValue *SharedLinkVisibility `json:"previous_value,omitempty"`
}

// NewSharedLinkChangeVisibilityDetails returns a new SharedLinkChangeVisibilityDetails instance
func NewSharedLinkChangeVisibilityDetails(NewValue *SharedLinkVisibility) *SharedLinkChangeVisibilityDetails {
	s := new(SharedLinkChangeVisibilityDetails)
	s.NewValue = NewValue
	return s
}

// SharedLinkChangeVisibilityType : has no documentation (yet)
type SharedLinkChangeVisibilityType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkChangeVisibilityType returns a new SharedLinkChangeVisibilityType instance
func NewSharedLinkChangeVisibilityType(Description string) *SharedLinkChangeVisibilityType {
	s := new(SharedLinkChangeVisibilityType)
	s.Description = Description
	return s
}

// SharedLinkCopyDetails : Added file/folder to Dropbox from shared link.
type SharedLinkCopyDetails struct {
	// SharedLinkOwner : Shared link owner details. Might be missing due to
	// historical data gap.
	SharedLinkOwner IsUserLogInfo `json:"shared_link_owner,omitempty"`
}

// NewSharedLinkCopyDetails returns a new SharedLinkCopyDetails instance
func NewSharedLinkCopyDetails() *SharedLinkCopyDetails {
	s := new(SharedLinkCopyDetails)
	return s
}

// SharedLinkCopyType : has no documentation (yet)
type SharedLinkCopyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkCopyType returns a new SharedLinkCopyType instance
func NewSharedLinkCopyType(Description string) *SharedLinkCopyType {
	s := new(SharedLinkCopyType)
	s.Description = Description
	return s
}

// SharedLinkCreateDetails : Created shared link.
type SharedLinkCreateDetails struct {
	// SharedLinkAccessLevel : Defines who can access the shared link. Might be
	// missing due to historical data gap.
	SharedLinkAccessLevel *SharedLinkAccessLevel `json:"shared_link_access_level,omitempty"`
}

// NewSharedLinkCreateDetails returns a new SharedLinkCreateDetails instance
func NewSharedLinkCreateDetails() *SharedLinkCreateDetails {
	s := new(SharedLinkCreateDetails)
	return s
}

// SharedLinkCreateType : has no documentation (yet)
type SharedLinkCreateType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkCreateType returns a new SharedLinkCreateType instance
func NewSharedLinkCreateType(Description string) *SharedLinkCreateType {
	s := new(SharedLinkCreateType)
	s.Description = Description
	return s
}

// SharedLinkDisableDetails : Removed shared link.
type SharedLinkDisableDetails struct {
	// SharedLinkOwner : Shared link owner details. Might be missing due to
	// historical data gap.
	SharedLinkOwner IsUserLogInfo `json:"shared_link_owner,omitempty"`
}

// NewSharedLinkDisableDetails returns a new SharedLinkDisableDetails instance
func NewSharedLinkDisableDetails() *SharedLinkDisableDetails {
	s := new(SharedLinkDisableDetails)
	return s
}

// SharedLinkDisableType : has no documentation (yet)
type SharedLinkDisableType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkDisableType returns a new SharedLinkDisableType instance
func NewSharedLinkDisableType(Description string) *SharedLinkDisableType {
	s := new(SharedLinkDisableType)
	s.Description = Description
	return s
}

// SharedLinkDownloadDetails : Downloaded file/folder from shared link.
type SharedLinkDownloadDetails struct {
	// SharedLinkOwner : Shared link owner details. Might be missing due to
	// historical data gap.
	SharedLinkOwner IsUserLogInfo `json:"shared_link_owner,omitempty"`
}

// NewSharedLinkDownloadDetails returns a new SharedLinkDownloadDetails instance
func NewSharedLinkDownloadDetails() *SharedLinkDownloadDetails {
	s := new(SharedLinkDownloadDetails)
	return s
}

// SharedLinkDownloadType : has no documentation (yet)
type SharedLinkDownloadType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkDownloadType returns a new SharedLinkDownloadType instance
func NewSharedLinkDownloadType(Description string) *SharedLinkDownloadType {
	s := new(SharedLinkDownloadType)
	s.Description = Description
	return s
}

// SharedLinkRemoveExpiryDetails : Removed shared link expiration date.
type SharedLinkRemoveExpiryDetails struct {
	// PreviousValue : Previous shared link expiration date. Might be missing
	// due to historical data gap.
	PreviousValue time.Time `json:"previous_value,omitempty"`
}

// NewSharedLinkRemoveExpiryDetails returns a new SharedLinkRemoveExpiryDetails instance
func NewSharedLinkRemoveExpiryDetails() *SharedLinkRemoveExpiryDetails {
	s := new(SharedLinkRemoveExpiryDetails)
	return s
}

// SharedLinkRemoveExpiryType : has no documentation (yet)
type SharedLinkRemoveExpiryType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkRemoveExpiryType returns a new SharedLinkRemoveExpiryType instance
func NewSharedLinkRemoveExpiryType(Description string) *SharedLinkRemoveExpiryType {
	s := new(SharedLinkRemoveExpiryType)
	s.Description = Description
	return s
}

// SharedLinkShareDetails : Added members as audience of shared link.
type SharedLinkShareDetails struct {
	// SharedLinkOwner : Shared link owner details. Might be missing due to
	// historical data gap.
	SharedLinkOwner IsUserLogInfo `json:"shared_link_owner,omitempty"`
	// ExternalUsers : Users without a Dropbox account that were added as shared
	// link audience.
	ExternalUsers []*ExternalUserLogInfo `json:"external_users,omitempty"`
}

// NewSharedLinkShareDetails returns a new SharedLinkShareDetails instance
func NewSharedLinkShareDetails() *SharedLinkShareDetails {
	s := new(SharedLinkShareDetails)
	return s
}

// SharedLinkShareType : has no documentation (yet)
type SharedLinkShareType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkShareType returns a new SharedLinkShareType instance
func NewSharedLinkShareType(Description string) *SharedLinkShareType {
	s := new(SharedLinkShareType)
	s.Description = Description
	return s
}

// SharedLinkViewDetails : Opened shared link.
type SharedLinkViewDetails struct {
	// SharedLinkOwner : Shared link owner details. Might be missing due to
	// historical data gap.
	SharedLinkOwner IsUserLogInfo `json:"shared_link_owner,omitempty"`
}

// NewSharedLinkViewDetails returns a new SharedLinkViewDetails instance
func NewSharedLinkViewDetails() *SharedLinkViewDetails {
	s := new(SharedLinkViewDetails)
	return s
}

// SharedLinkViewType : has no documentation (yet)
type SharedLinkViewType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedLinkViewType returns a new SharedLinkViewType instance
func NewSharedLinkViewType(Description string) *SharedLinkViewType {
	s := new(SharedLinkViewType)
	s.Description = Description
	return s
}

// SharedLinkVisibility : Defines who has access to a shared link.
type SharedLinkVisibility struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkVisibility
const (
	SharedLinkVisibilityPassword = "password"
	SharedLinkVisibilityPublic   = "public"
	SharedLinkVisibilityTeamOnly = "team_only"
	SharedLinkVisibilityOther    = "other"
)

// SharedNoteOpenedDetails : Opened shared Paper doc.
type SharedNoteOpenedDetails struct {
}

// NewSharedNoteOpenedDetails returns a new SharedNoteOpenedDetails instance
func NewSharedNoteOpenedDetails() *SharedNoteOpenedDetails {
	s := new(SharedNoteOpenedDetails)
	return s
}

// SharedNoteOpenedType : has no documentation (yet)
type SharedNoteOpenedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharedNoteOpenedType returns a new SharedNoteOpenedType instance
func NewSharedNoteOpenedType(Description string) *SharedNoteOpenedType {
	s := new(SharedNoteOpenedType)
	s.Description = Description
	return s
}

// SharingChangeFolderJoinPolicyDetails : Changed whether team members can join
// shared folders owned outside team.
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

// SharingChangeFolderJoinPolicyType : has no documentation (yet)
type SharingChangeFolderJoinPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharingChangeFolderJoinPolicyType returns a new SharingChangeFolderJoinPolicyType instance
func NewSharingChangeFolderJoinPolicyType(Description string) *SharingChangeFolderJoinPolicyType {
	s := new(SharingChangeFolderJoinPolicyType)
	s.Description = Description
	return s
}

// SharingChangeLinkPolicyDetails : Changed whether members can share links
// outside team, and if links are accessible only by team members or anyone by
// default.
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

// SharingChangeLinkPolicyType : has no documentation (yet)
type SharingChangeLinkPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharingChangeLinkPolicyType returns a new SharingChangeLinkPolicyType instance
func NewSharingChangeLinkPolicyType(Description string) *SharingChangeLinkPolicyType {
	s := new(SharingChangeLinkPolicyType)
	s.Description = Description
	return s
}

// SharingChangeMemberPolicyDetails : Changed whether members can share
// files/folders outside team.
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

// SharingChangeMemberPolicyType : has no documentation (yet)
type SharingChangeMemberPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSharingChangeMemberPolicyType returns a new SharingChangeMemberPolicyType instance
func NewSharingChangeMemberPolicyType(Description string) *SharingChangeMemberPolicyType {
	s := new(SharingChangeMemberPolicyType)
	s.Description = Description
	return s
}

// SharingFolderJoinPolicy : Policy for controlling if team members can join
// shared folders owned by non team members.
type SharingFolderJoinPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharingFolderJoinPolicy
const (
	SharingFolderJoinPolicyFromAnyone   = "from_anyone"
	SharingFolderJoinPolicyFromTeamOnly = "from_team_only"
	SharingFolderJoinPolicyOther        = "other"
)

// SharingLinkPolicy : Policy for controlling if team members can share links
// externally
type SharingLinkPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharingLinkPolicy
const (
	SharingLinkPolicyDefaultPrivate = "default_private"
	SharingLinkPolicyDefaultPublic  = "default_public"
	SharingLinkPolicyOnlyPrivate    = "only_private"
	SharingLinkPolicyOther          = "other"
)

// SharingMemberPolicy : External sharing policy
type SharingMemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharingMemberPolicy
const (
	SharingMemberPolicyAllow  = "allow"
	SharingMemberPolicyForbid = "forbid"
	SharingMemberPolicyOther  = "other"
)

// ShmodelGroupShareDetails : Shared link with group.
type ShmodelGroupShareDetails struct {
}

// NewShmodelGroupShareDetails returns a new ShmodelGroupShareDetails instance
func NewShmodelGroupShareDetails() *ShmodelGroupShareDetails {
	s := new(ShmodelGroupShareDetails)
	return s
}

// ShmodelGroupShareType : has no documentation (yet)
type ShmodelGroupShareType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShmodelGroupShareType returns a new ShmodelGroupShareType instance
func NewShmodelGroupShareType(Description string) *ShmodelGroupShareType {
	s := new(ShmodelGroupShareType)
	s.Description = Description
	return s
}

// ShowcaseAccessGrantedDetails : Granted access to showcase.
type ShowcaseAccessGrantedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseAccessGrantedDetails returns a new ShowcaseAccessGrantedDetails instance
func NewShowcaseAccessGrantedDetails(EventUuid string) *ShowcaseAccessGrantedDetails {
	s := new(ShowcaseAccessGrantedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseAccessGrantedType : has no documentation (yet)
type ShowcaseAccessGrantedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseAccessGrantedType returns a new ShowcaseAccessGrantedType instance
func NewShowcaseAccessGrantedType(Description string) *ShowcaseAccessGrantedType {
	s := new(ShowcaseAccessGrantedType)
	s.Description = Description
	return s
}

// ShowcaseAddMemberDetails : Added member to showcase.
type ShowcaseAddMemberDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseAddMemberDetails returns a new ShowcaseAddMemberDetails instance
func NewShowcaseAddMemberDetails(EventUuid string) *ShowcaseAddMemberDetails {
	s := new(ShowcaseAddMemberDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseAddMemberType : has no documentation (yet)
type ShowcaseAddMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseAddMemberType returns a new ShowcaseAddMemberType instance
func NewShowcaseAddMemberType(Description string) *ShowcaseAddMemberType {
	s := new(ShowcaseAddMemberType)
	s.Description = Description
	return s
}

// ShowcaseArchivedDetails : Archived showcase.
type ShowcaseArchivedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseArchivedDetails returns a new ShowcaseArchivedDetails instance
func NewShowcaseArchivedDetails(EventUuid string) *ShowcaseArchivedDetails {
	s := new(ShowcaseArchivedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseArchivedType : has no documentation (yet)
type ShowcaseArchivedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseArchivedType returns a new ShowcaseArchivedType instance
func NewShowcaseArchivedType(Description string) *ShowcaseArchivedType {
	s := new(ShowcaseArchivedType)
	s.Description = Description
	return s
}

// ShowcaseChangeDownloadPolicyDetails : Enabled/disabled downloading files from
// Dropbox Showcase for team.
type ShowcaseChangeDownloadPolicyDetails struct {
	// NewValue : New Dropbox Showcase download policy.
	NewValue *ShowcaseDownloadPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Showcase download policy.
	PreviousValue *ShowcaseDownloadPolicy `json:"previous_value"`
}

// NewShowcaseChangeDownloadPolicyDetails returns a new ShowcaseChangeDownloadPolicyDetails instance
func NewShowcaseChangeDownloadPolicyDetails(NewValue *ShowcaseDownloadPolicy, PreviousValue *ShowcaseDownloadPolicy) *ShowcaseChangeDownloadPolicyDetails {
	s := new(ShowcaseChangeDownloadPolicyDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// ShowcaseChangeDownloadPolicyType : has no documentation (yet)
type ShowcaseChangeDownloadPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseChangeDownloadPolicyType returns a new ShowcaseChangeDownloadPolicyType instance
func NewShowcaseChangeDownloadPolicyType(Description string) *ShowcaseChangeDownloadPolicyType {
	s := new(ShowcaseChangeDownloadPolicyType)
	s.Description = Description
	return s
}

// ShowcaseChangeEnabledPolicyDetails : Enabled/disabled Dropbox Showcase for
// team.
type ShowcaseChangeEnabledPolicyDetails struct {
	// NewValue : New Dropbox Showcase policy.
	NewValue *ShowcaseEnabledPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Showcase policy.
	PreviousValue *ShowcaseEnabledPolicy `json:"previous_value"`
}

// NewShowcaseChangeEnabledPolicyDetails returns a new ShowcaseChangeEnabledPolicyDetails instance
func NewShowcaseChangeEnabledPolicyDetails(NewValue *ShowcaseEnabledPolicy, PreviousValue *ShowcaseEnabledPolicy) *ShowcaseChangeEnabledPolicyDetails {
	s := new(ShowcaseChangeEnabledPolicyDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// ShowcaseChangeEnabledPolicyType : has no documentation (yet)
type ShowcaseChangeEnabledPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseChangeEnabledPolicyType returns a new ShowcaseChangeEnabledPolicyType instance
func NewShowcaseChangeEnabledPolicyType(Description string) *ShowcaseChangeEnabledPolicyType {
	s := new(ShowcaseChangeEnabledPolicyType)
	s.Description = Description
	return s
}

// ShowcaseChangeExternalSharingPolicyDetails : Enabled/disabled sharing Dropbox
// Showcase externally for team.
type ShowcaseChangeExternalSharingPolicyDetails struct {
	// NewValue : New Dropbox Showcase external sharing policy.
	NewValue *ShowcaseExternalSharingPolicy `json:"new_value"`
	// PreviousValue : Previous Dropbox Showcase external sharing policy.
	PreviousValue *ShowcaseExternalSharingPolicy `json:"previous_value"`
}

// NewShowcaseChangeExternalSharingPolicyDetails returns a new ShowcaseChangeExternalSharingPolicyDetails instance
func NewShowcaseChangeExternalSharingPolicyDetails(NewValue *ShowcaseExternalSharingPolicy, PreviousValue *ShowcaseExternalSharingPolicy) *ShowcaseChangeExternalSharingPolicyDetails {
	s := new(ShowcaseChangeExternalSharingPolicyDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// ShowcaseChangeExternalSharingPolicyType : has no documentation (yet)
type ShowcaseChangeExternalSharingPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseChangeExternalSharingPolicyType returns a new ShowcaseChangeExternalSharingPolicyType instance
func NewShowcaseChangeExternalSharingPolicyType(Description string) *ShowcaseChangeExternalSharingPolicyType {
	s := new(ShowcaseChangeExternalSharingPolicyType)
	s.Description = Description
	return s
}

// ShowcaseCreatedDetails : Created showcase.
type ShowcaseCreatedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseCreatedDetails returns a new ShowcaseCreatedDetails instance
func NewShowcaseCreatedDetails(EventUuid string) *ShowcaseCreatedDetails {
	s := new(ShowcaseCreatedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseCreatedType : has no documentation (yet)
type ShowcaseCreatedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseCreatedType returns a new ShowcaseCreatedType instance
func NewShowcaseCreatedType(Description string) *ShowcaseCreatedType {
	s := new(ShowcaseCreatedType)
	s.Description = Description
	return s
}

// ShowcaseDeleteCommentDetails : Deleted showcase comment.
type ShowcaseDeleteCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text.
	CommentText string `json:"comment_text,omitempty"`
}

// NewShowcaseDeleteCommentDetails returns a new ShowcaseDeleteCommentDetails instance
func NewShowcaseDeleteCommentDetails(EventUuid string) *ShowcaseDeleteCommentDetails {
	s := new(ShowcaseDeleteCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseDeleteCommentType : has no documentation (yet)
type ShowcaseDeleteCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseDeleteCommentType returns a new ShowcaseDeleteCommentType instance
func NewShowcaseDeleteCommentType(Description string) *ShowcaseDeleteCommentType {
	s := new(ShowcaseDeleteCommentType)
	s.Description = Description
	return s
}

// ShowcaseDocumentLogInfo : Showcase document's logged information.
type ShowcaseDocumentLogInfo struct {
	// ShowcaseId : Showcase document Id.
	ShowcaseId string `json:"showcase_id"`
	// ShowcaseTitle : Showcase document title.
	ShowcaseTitle string `json:"showcase_title"`
}

// NewShowcaseDocumentLogInfo returns a new ShowcaseDocumentLogInfo instance
func NewShowcaseDocumentLogInfo(ShowcaseId string, ShowcaseTitle string) *ShowcaseDocumentLogInfo {
	s := new(ShowcaseDocumentLogInfo)
	s.ShowcaseId = ShowcaseId
	s.ShowcaseTitle = ShowcaseTitle
	return s
}

// ShowcaseDownloadPolicy : Policy for controlling if files can be downloaded
// from Showcases by team members
type ShowcaseDownloadPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ShowcaseDownloadPolicy
const (
	ShowcaseDownloadPolicyDisabled = "disabled"
	ShowcaseDownloadPolicyEnabled  = "enabled"
	ShowcaseDownloadPolicyOther    = "other"
)

// ShowcaseEditCommentDetails : Edited showcase comment.
type ShowcaseEditCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text.
	CommentText string `json:"comment_text,omitempty"`
}

// NewShowcaseEditCommentDetails returns a new ShowcaseEditCommentDetails instance
func NewShowcaseEditCommentDetails(EventUuid string) *ShowcaseEditCommentDetails {
	s := new(ShowcaseEditCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseEditCommentType : has no documentation (yet)
type ShowcaseEditCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseEditCommentType returns a new ShowcaseEditCommentType instance
func NewShowcaseEditCommentType(Description string) *ShowcaseEditCommentType {
	s := new(ShowcaseEditCommentType)
	s.Description = Description
	return s
}

// ShowcaseEditedDetails : Edited showcase.
type ShowcaseEditedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseEditedDetails returns a new ShowcaseEditedDetails instance
func NewShowcaseEditedDetails(EventUuid string) *ShowcaseEditedDetails {
	s := new(ShowcaseEditedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseEditedType : has no documentation (yet)
type ShowcaseEditedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseEditedType returns a new ShowcaseEditedType instance
func NewShowcaseEditedType(Description string) *ShowcaseEditedType {
	s := new(ShowcaseEditedType)
	s.Description = Description
	return s
}

// ShowcaseEnabledPolicy : Policy for controlling whether Showcase is enabled.
type ShowcaseEnabledPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ShowcaseEnabledPolicy
const (
	ShowcaseEnabledPolicyDisabled = "disabled"
	ShowcaseEnabledPolicyEnabled  = "enabled"
	ShowcaseEnabledPolicyOther    = "other"
)

// ShowcaseExternalSharingPolicy : Policy for controlling if team members can
// share Showcases externally.
type ShowcaseExternalSharingPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ShowcaseExternalSharingPolicy
const (
	ShowcaseExternalSharingPolicyDisabled = "disabled"
	ShowcaseExternalSharingPolicyEnabled  = "enabled"
	ShowcaseExternalSharingPolicyOther    = "other"
)

// ShowcaseFileAddedDetails : Added file to showcase.
type ShowcaseFileAddedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseFileAddedDetails returns a new ShowcaseFileAddedDetails instance
func NewShowcaseFileAddedDetails(EventUuid string) *ShowcaseFileAddedDetails {
	s := new(ShowcaseFileAddedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseFileAddedType : has no documentation (yet)
type ShowcaseFileAddedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseFileAddedType returns a new ShowcaseFileAddedType instance
func NewShowcaseFileAddedType(Description string) *ShowcaseFileAddedType {
	s := new(ShowcaseFileAddedType)
	s.Description = Description
	return s
}

// ShowcaseFileDownloadDetails : Downloaded file from showcase.
type ShowcaseFileDownloadDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// DownloadType : Showcase download type.
	DownloadType string `json:"download_type"`
}

// NewShowcaseFileDownloadDetails returns a new ShowcaseFileDownloadDetails instance
func NewShowcaseFileDownloadDetails(EventUuid string, DownloadType string) *ShowcaseFileDownloadDetails {
	s := new(ShowcaseFileDownloadDetails)
	s.EventUuid = EventUuid
	s.DownloadType = DownloadType
	return s
}

// ShowcaseFileDownloadType : has no documentation (yet)
type ShowcaseFileDownloadType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseFileDownloadType returns a new ShowcaseFileDownloadType instance
func NewShowcaseFileDownloadType(Description string) *ShowcaseFileDownloadType {
	s := new(ShowcaseFileDownloadType)
	s.Description = Description
	return s
}

// ShowcaseFileRemovedDetails : Removed file from showcase.
type ShowcaseFileRemovedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseFileRemovedDetails returns a new ShowcaseFileRemovedDetails instance
func NewShowcaseFileRemovedDetails(EventUuid string) *ShowcaseFileRemovedDetails {
	s := new(ShowcaseFileRemovedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseFileRemovedType : has no documentation (yet)
type ShowcaseFileRemovedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseFileRemovedType returns a new ShowcaseFileRemovedType instance
func NewShowcaseFileRemovedType(Description string) *ShowcaseFileRemovedType {
	s := new(ShowcaseFileRemovedType)
	s.Description = Description
	return s
}

// ShowcaseFileViewDetails : Viewed file in showcase.
type ShowcaseFileViewDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseFileViewDetails returns a new ShowcaseFileViewDetails instance
func NewShowcaseFileViewDetails(EventUuid string) *ShowcaseFileViewDetails {
	s := new(ShowcaseFileViewDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseFileViewType : has no documentation (yet)
type ShowcaseFileViewType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseFileViewType returns a new ShowcaseFileViewType instance
func NewShowcaseFileViewType(Description string) *ShowcaseFileViewType {
	s := new(ShowcaseFileViewType)
	s.Description = Description
	return s
}

// ShowcasePermanentlyDeletedDetails : Permanently deleted showcase.
type ShowcasePermanentlyDeletedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcasePermanentlyDeletedDetails returns a new ShowcasePermanentlyDeletedDetails instance
func NewShowcasePermanentlyDeletedDetails(EventUuid string) *ShowcasePermanentlyDeletedDetails {
	s := new(ShowcasePermanentlyDeletedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcasePermanentlyDeletedType : has no documentation (yet)
type ShowcasePermanentlyDeletedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcasePermanentlyDeletedType returns a new ShowcasePermanentlyDeletedType instance
func NewShowcasePermanentlyDeletedType(Description string) *ShowcasePermanentlyDeletedType {
	s := new(ShowcasePermanentlyDeletedType)
	s.Description = Description
	return s
}

// ShowcasePostCommentDetails : Added showcase comment.
type ShowcasePostCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text.
	CommentText string `json:"comment_text,omitempty"`
}

// NewShowcasePostCommentDetails returns a new ShowcasePostCommentDetails instance
func NewShowcasePostCommentDetails(EventUuid string) *ShowcasePostCommentDetails {
	s := new(ShowcasePostCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcasePostCommentType : has no documentation (yet)
type ShowcasePostCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcasePostCommentType returns a new ShowcasePostCommentType instance
func NewShowcasePostCommentType(Description string) *ShowcasePostCommentType {
	s := new(ShowcasePostCommentType)
	s.Description = Description
	return s
}

// ShowcaseRemoveMemberDetails : Removed member from showcase.
type ShowcaseRemoveMemberDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseRemoveMemberDetails returns a new ShowcaseRemoveMemberDetails instance
func NewShowcaseRemoveMemberDetails(EventUuid string) *ShowcaseRemoveMemberDetails {
	s := new(ShowcaseRemoveMemberDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseRemoveMemberType : has no documentation (yet)
type ShowcaseRemoveMemberType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseRemoveMemberType returns a new ShowcaseRemoveMemberType instance
func NewShowcaseRemoveMemberType(Description string) *ShowcaseRemoveMemberType {
	s := new(ShowcaseRemoveMemberType)
	s.Description = Description
	return s
}

// ShowcaseRenamedDetails : Renamed showcase.
type ShowcaseRenamedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseRenamedDetails returns a new ShowcaseRenamedDetails instance
func NewShowcaseRenamedDetails(EventUuid string) *ShowcaseRenamedDetails {
	s := new(ShowcaseRenamedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseRenamedType : has no documentation (yet)
type ShowcaseRenamedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseRenamedType returns a new ShowcaseRenamedType instance
func NewShowcaseRenamedType(Description string) *ShowcaseRenamedType {
	s := new(ShowcaseRenamedType)
	s.Description = Description
	return s
}

// ShowcaseRequestAccessDetails : Requested access to showcase.
type ShowcaseRequestAccessDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseRequestAccessDetails returns a new ShowcaseRequestAccessDetails instance
func NewShowcaseRequestAccessDetails(EventUuid string) *ShowcaseRequestAccessDetails {
	s := new(ShowcaseRequestAccessDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseRequestAccessType : has no documentation (yet)
type ShowcaseRequestAccessType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseRequestAccessType returns a new ShowcaseRequestAccessType instance
func NewShowcaseRequestAccessType(Description string) *ShowcaseRequestAccessType {
	s := new(ShowcaseRequestAccessType)
	s.Description = Description
	return s
}

// ShowcaseResolveCommentDetails : Resolved showcase comment.
type ShowcaseResolveCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text.
	CommentText string `json:"comment_text,omitempty"`
}

// NewShowcaseResolveCommentDetails returns a new ShowcaseResolveCommentDetails instance
func NewShowcaseResolveCommentDetails(EventUuid string) *ShowcaseResolveCommentDetails {
	s := new(ShowcaseResolveCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseResolveCommentType : has no documentation (yet)
type ShowcaseResolveCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseResolveCommentType returns a new ShowcaseResolveCommentType instance
func NewShowcaseResolveCommentType(Description string) *ShowcaseResolveCommentType {
	s := new(ShowcaseResolveCommentType)
	s.Description = Description
	return s
}

// ShowcaseRestoredDetails : Unarchived showcase.
type ShowcaseRestoredDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseRestoredDetails returns a new ShowcaseRestoredDetails instance
func NewShowcaseRestoredDetails(EventUuid string) *ShowcaseRestoredDetails {
	s := new(ShowcaseRestoredDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseRestoredType : has no documentation (yet)
type ShowcaseRestoredType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseRestoredType returns a new ShowcaseRestoredType instance
func NewShowcaseRestoredType(Description string) *ShowcaseRestoredType {
	s := new(ShowcaseRestoredType)
	s.Description = Description
	return s
}

// ShowcaseTrashedDeprecatedDetails : Deleted showcase (old version).
type ShowcaseTrashedDeprecatedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseTrashedDeprecatedDetails returns a new ShowcaseTrashedDeprecatedDetails instance
func NewShowcaseTrashedDeprecatedDetails(EventUuid string) *ShowcaseTrashedDeprecatedDetails {
	s := new(ShowcaseTrashedDeprecatedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseTrashedDeprecatedType : has no documentation (yet)
type ShowcaseTrashedDeprecatedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseTrashedDeprecatedType returns a new ShowcaseTrashedDeprecatedType instance
func NewShowcaseTrashedDeprecatedType(Description string) *ShowcaseTrashedDeprecatedType {
	s := new(ShowcaseTrashedDeprecatedType)
	s.Description = Description
	return s
}

// ShowcaseTrashedDetails : Deleted showcase.
type ShowcaseTrashedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseTrashedDetails returns a new ShowcaseTrashedDetails instance
func NewShowcaseTrashedDetails(EventUuid string) *ShowcaseTrashedDetails {
	s := new(ShowcaseTrashedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseTrashedType : has no documentation (yet)
type ShowcaseTrashedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseTrashedType returns a new ShowcaseTrashedType instance
func NewShowcaseTrashedType(Description string) *ShowcaseTrashedType {
	s := new(ShowcaseTrashedType)
	s.Description = Description
	return s
}

// ShowcaseUnresolveCommentDetails : Unresolved showcase comment.
type ShowcaseUnresolveCommentDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
	// CommentText : Comment text.
	CommentText string `json:"comment_text,omitempty"`
}

// NewShowcaseUnresolveCommentDetails returns a new ShowcaseUnresolveCommentDetails instance
func NewShowcaseUnresolveCommentDetails(EventUuid string) *ShowcaseUnresolveCommentDetails {
	s := new(ShowcaseUnresolveCommentDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseUnresolveCommentType : has no documentation (yet)
type ShowcaseUnresolveCommentType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseUnresolveCommentType returns a new ShowcaseUnresolveCommentType instance
func NewShowcaseUnresolveCommentType(Description string) *ShowcaseUnresolveCommentType {
	s := new(ShowcaseUnresolveCommentType)
	s.Description = Description
	return s
}

// ShowcaseUntrashedDeprecatedDetails : Restored showcase (old version).
type ShowcaseUntrashedDeprecatedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseUntrashedDeprecatedDetails returns a new ShowcaseUntrashedDeprecatedDetails instance
func NewShowcaseUntrashedDeprecatedDetails(EventUuid string) *ShowcaseUntrashedDeprecatedDetails {
	s := new(ShowcaseUntrashedDeprecatedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseUntrashedDeprecatedType : has no documentation (yet)
type ShowcaseUntrashedDeprecatedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseUntrashedDeprecatedType returns a new ShowcaseUntrashedDeprecatedType instance
func NewShowcaseUntrashedDeprecatedType(Description string) *ShowcaseUntrashedDeprecatedType {
	s := new(ShowcaseUntrashedDeprecatedType)
	s.Description = Description
	return s
}

// ShowcaseUntrashedDetails : Restored showcase.
type ShowcaseUntrashedDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseUntrashedDetails returns a new ShowcaseUntrashedDetails instance
func NewShowcaseUntrashedDetails(EventUuid string) *ShowcaseUntrashedDetails {
	s := new(ShowcaseUntrashedDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseUntrashedType : has no documentation (yet)
type ShowcaseUntrashedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseUntrashedType returns a new ShowcaseUntrashedType instance
func NewShowcaseUntrashedType(Description string) *ShowcaseUntrashedType {
	s := new(ShowcaseUntrashedType)
	s.Description = Description
	return s
}

// ShowcaseViewDetails : Viewed showcase.
type ShowcaseViewDetails struct {
	// EventUuid : Event unique identifier.
	EventUuid string `json:"event_uuid"`
}

// NewShowcaseViewDetails returns a new ShowcaseViewDetails instance
func NewShowcaseViewDetails(EventUuid string) *ShowcaseViewDetails {
	s := new(ShowcaseViewDetails)
	s.EventUuid = EventUuid
	return s
}

// ShowcaseViewType : has no documentation (yet)
type ShowcaseViewType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewShowcaseViewType returns a new ShowcaseViewType instance
func NewShowcaseViewType(Description string) *ShowcaseViewType {
	s := new(ShowcaseViewType)
	s.Description = Description
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

// SignInAsSessionEndType : has no documentation (yet)
type SignInAsSessionEndType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSignInAsSessionEndType returns a new SignInAsSessionEndType instance
func NewSignInAsSessionEndType(Description string) *SignInAsSessionEndType {
	s := new(SignInAsSessionEndType)
	s.Description = Description
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

// SignInAsSessionStartType : has no documentation (yet)
type SignInAsSessionStartType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSignInAsSessionStartType returns a new SignInAsSessionStartType instance
func NewSignInAsSessionStartType(Description string) *SignInAsSessionStartType {
	s := new(SignInAsSessionStartType)
	s.Description = Description
	return s
}

// SmartSyncChangePolicyDetails : Changed default Smart Sync setting for team
// members.
type SmartSyncChangePolicyDetails struct {
	// NewValue : New smart sync policy.
	NewValue *team_policies.SmartSyncPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous smart sync policy.
	PreviousValue *team_policies.SmartSyncPolicy `json:"previous_value,omitempty"`
}

// NewSmartSyncChangePolicyDetails returns a new SmartSyncChangePolicyDetails instance
func NewSmartSyncChangePolicyDetails() *SmartSyncChangePolicyDetails {
	s := new(SmartSyncChangePolicyDetails)
	return s
}

// SmartSyncChangePolicyType : has no documentation (yet)
type SmartSyncChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSmartSyncChangePolicyType returns a new SmartSyncChangePolicyType instance
func NewSmartSyncChangePolicyType(Description string) *SmartSyncChangePolicyType {
	s := new(SmartSyncChangePolicyType)
	s.Description = Description
	return s
}

// SmartSyncCreateAdminPrivilegeReportDetails : Created Smart Sync non-admin
// devices report.
type SmartSyncCreateAdminPrivilegeReportDetails struct {
}

// NewSmartSyncCreateAdminPrivilegeReportDetails returns a new SmartSyncCreateAdminPrivilegeReportDetails instance
func NewSmartSyncCreateAdminPrivilegeReportDetails() *SmartSyncCreateAdminPrivilegeReportDetails {
	s := new(SmartSyncCreateAdminPrivilegeReportDetails)
	return s
}

// SmartSyncCreateAdminPrivilegeReportType : has no documentation (yet)
type SmartSyncCreateAdminPrivilegeReportType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSmartSyncCreateAdminPrivilegeReportType returns a new SmartSyncCreateAdminPrivilegeReportType instance
func NewSmartSyncCreateAdminPrivilegeReportType(Description string) *SmartSyncCreateAdminPrivilegeReportType {
	s := new(SmartSyncCreateAdminPrivilegeReportType)
	s.Description = Description
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

// SmartSyncNotOptOutType : has no documentation (yet)
type SmartSyncNotOptOutType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSmartSyncNotOptOutType returns a new SmartSyncNotOptOutType instance
func NewSmartSyncNotOptOutType(Description string) *SmartSyncNotOptOutType {
	s := new(SmartSyncNotOptOutType)
	s.Description = Description
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
	SmartSyncOptOutPolicyDefault  = "default"
	SmartSyncOptOutPolicyOptedOut = "opted_out"
	SmartSyncOptOutPolicyOther    = "other"
)

// SmartSyncOptOutType : has no documentation (yet)
type SmartSyncOptOutType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSmartSyncOptOutType returns a new SmartSyncOptOutType instance
func NewSmartSyncOptOutType(Description string) *SmartSyncOptOutType {
	s := new(SmartSyncOptOutType)
	s.Description = Description
	return s
}

// SpaceCapsType : Space limit alert policy
type SpaceCapsType struct {
	dropbox.Tagged
}

// Valid tag values for SpaceCapsType
const (
	SpaceCapsTypeHard  = "hard"
	SpaceCapsTypeOff   = "off"
	SpaceCapsTypeSoft  = "soft"
	SpaceCapsTypeOther = "other"
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

// SsoAddCertDetails : Added X.509 certificate for SSO.
type SsoAddCertDetails struct {
	// CertificateDetails : SSO certificate details.
	CertificateDetails *Certificate `json:"certificate_details"`
}

// NewSsoAddCertDetails returns a new SsoAddCertDetails instance
func NewSsoAddCertDetails(CertificateDetails *Certificate) *SsoAddCertDetails {
	s := new(SsoAddCertDetails)
	s.CertificateDetails = CertificateDetails
	return s
}

// SsoAddCertType : has no documentation (yet)
type SsoAddCertType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoAddCertType returns a new SsoAddCertType instance
func NewSsoAddCertType(Description string) *SsoAddCertType {
	s := new(SsoAddCertType)
	s.Description = Description
	return s
}

// SsoAddLoginUrlDetails : Added sign-in URL for SSO.
type SsoAddLoginUrlDetails struct {
	// NewValue : New single sign-on login URL.
	NewValue string `json:"new_value"`
}

// NewSsoAddLoginUrlDetails returns a new SsoAddLoginUrlDetails instance
func NewSsoAddLoginUrlDetails(NewValue string) *SsoAddLoginUrlDetails {
	s := new(SsoAddLoginUrlDetails)
	s.NewValue = NewValue
	return s
}

// SsoAddLoginUrlType : has no documentation (yet)
type SsoAddLoginUrlType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoAddLoginUrlType returns a new SsoAddLoginUrlType instance
func NewSsoAddLoginUrlType(Description string) *SsoAddLoginUrlType {
	s := new(SsoAddLoginUrlType)
	s.Description = Description
	return s
}

// SsoAddLogoutUrlDetails : Added sign-out URL for SSO.
type SsoAddLogoutUrlDetails struct {
	// NewValue : New single sign-on logout URL. Might be missing due to
	// historical data gap.
	NewValue string `json:"new_value,omitempty"`
}

// NewSsoAddLogoutUrlDetails returns a new SsoAddLogoutUrlDetails instance
func NewSsoAddLogoutUrlDetails() *SsoAddLogoutUrlDetails {
	s := new(SsoAddLogoutUrlDetails)
	return s
}

// SsoAddLogoutUrlType : has no documentation (yet)
type SsoAddLogoutUrlType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoAddLogoutUrlType returns a new SsoAddLogoutUrlType instance
func NewSsoAddLogoutUrlType(Description string) *SsoAddLogoutUrlType {
	s := new(SsoAddLogoutUrlType)
	s.Description = Description
	return s
}

// SsoChangeCertDetails : Changed X.509 certificate for SSO.
type SsoChangeCertDetails struct {
	// PreviousCertificateDetails : Previous SSO certificate details. Might be
	// missing due to historical data gap.
	PreviousCertificateDetails *Certificate `json:"previous_certificate_details,omitempty"`
	// NewCertificateDetails : New SSO certificate details.
	NewCertificateDetails *Certificate `json:"new_certificate_details"`
}

// NewSsoChangeCertDetails returns a new SsoChangeCertDetails instance
func NewSsoChangeCertDetails(NewCertificateDetails *Certificate) *SsoChangeCertDetails {
	s := new(SsoChangeCertDetails)
	s.NewCertificateDetails = NewCertificateDetails
	return s
}

// SsoChangeCertType : has no documentation (yet)
type SsoChangeCertType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoChangeCertType returns a new SsoChangeCertType instance
func NewSsoChangeCertType(Description string) *SsoChangeCertType {
	s := new(SsoChangeCertType)
	s.Description = Description
	return s
}

// SsoChangeLoginUrlDetails : Changed sign-in URL for SSO.
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

// SsoChangeLoginUrlType : has no documentation (yet)
type SsoChangeLoginUrlType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoChangeLoginUrlType returns a new SsoChangeLoginUrlType instance
func NewSsoChangeLoginUrlType(Description string) *SsoChangeLoginUrlType {
	s := new(SsoChangeLoginUrlType)
	s.Description = Description
	return s
}

// SsoChangeLogoutUrlDetails : Changed sign-out URL for SSO.
type SsoChangeLogoutUrlDetails struct {
	// PreviousValue : Previous single sign-on logout URL. Might be missing due
	// to historical data gap.
	PreviousValue string `json:"previous_value,omitempty"`
	// NewValue : New single sign-on logout URL. Might be missing due to
	// historical data gap.
	NewValue string `json:"new_value,omitempty"`
}

// NewSsoChangeLogoutUrlDetails returns a new SsoChangeLogoutUrlDetails instance
func NewSsoChangeLogoutUrlDetails() *SsoChangeLogoutUrlDetails {
	s := new(SsoChangeLogoutUrlDetails)
	return s
}

// SsoChangeLogoutUrlType : has no documentation (yet)
type SsoChangeLogoutUrlType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoChangeLogoutUrlType returns a new SsoChangeLogoutUrlType instance
func NewSsoChangeLogoutUrlType(Description string) *SsoChangeLogoutUrlType {
	s := new(SsoChangeLogoutUrlType)
	s.Description = Description
	return s
}

// SsoChangePolicyDetails : Changed single sign-on setting for team.
type SsoChangePolicyDetails struct {
	// NewValue : New single sign-on policy.
	NewValue *team_policies.SsoPolicy `json:"new_value"`
	// PreviousValue : Previous single sign-on policy. Might be missing due to
	// historical data gap.
	PreviousValue *team_policies.SsoPolicy `json:"previous_value,omitempty"`
}

// NewSsoChangePolicyDetails returns a new SsoChangePolicyDetails instance
func NewSsoChangePolicyDetails(NewValue *team_policies.SsoPolicy) *SsoChangePolicyDetails {
	s := new(SsoChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// SsoChangePolicyType : has no documentation (yet)
type SsoChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoChangePolicyType returns a new SsoChangePolicyType instance
func NewSsoChangePolicyType(Description string) *SsoChangePolicyType {
	s := new(SsoChangePolicyType)
	s.Description = Description
	return s
}

// SsoChangeSamlIdentityModeDetails : Changed SAML identity mode for SSO.
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

// SsoChangeSamlIdentityModeType : has no documentation (yet)
type SsoChangeSamlIdentityModeType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoChangeSamlIdentityModeType returns a new SsoChangeSamlIdentityModeType instance
func NewSsoChangeSamlIdentityModeType(Description string) *SsoChangeSamlIdentityModeType {
	s := new(SsoChangeSamlIdentityModeType)
	s.Description = Description
	return s
}

// SsoErrorDetails : Failed to sign in via SSO.
type SsoErrorDetails struct {
	// ErrorDetails : Error details.
	ErrorDetails *FailureDetailsLogInfo `json:"error_details"`
}

// NewSsoErrorDetails returns a new SsoErrorDetails instance
func NewSsoErrorDetails(ErrorDetails *FailureDetailsLogInfo) *SsoErrorDetails {
	s := new(SsoErrorDetails)
	s.ErrorDetails = ErrorDetails
	return s
}

// SsoErrorType : has no documentation (yet)
type SsoErrorType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoErrorType returns a new SsoErrorType instance
func NewSsoErrorType(Description string) *SsoErrorType {
	s := new(SsoErrorType)
	s.Description = Description
	return s
}

// SsoRemoveCertDetails : Removed X.509 certificate for SSO.
type SsoRemoveCertDetails struct {
}

// NewSsoRemoveCertDetails returns a new SsoRemoveCertDetails instance
func NewSsoRemoveCertDetails() *SsoRemoveCertDetails {
	s := new(SsoRemoveCertDetails)
	return s
}

// SsoRemoveCertType : has no documentation (yet)
type SsoRemoveCertType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoRemoveCertType returns a new SsoRemoveCertType instance
func NewSsoRemoveCertType(Description string) *SsoRemoveCertType {
	s := new(SsoRemoveCertType)
	s.Description = Description
	return s
}

// SsoRemoveLoginUrlDetails : Removed sign-in URL for SSO.
type SsoRemoveLoginUrlDetails struct {
	// PreviousValue : Previous single sign-on login URL.
	PreviousValue string `json:"previous_value"`
}

// NewSsoRemoveLoginUrlDetails returns a new SsoRemoveLoginUrlDetails instance
func NewSsoRemoveLoginUrlDetails(PreviousValue string) *SsoRemoveLoginUrlDetails {
	s := new(SsoRemoveLoginUrlDetails)
	s.PreviousValue = PreviousValue
	return s
}

// SsoRemoveLoginUrlType : has no documentation (yet)
type SsoRemoveLoginUrlType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoRemoveLoginUrlType returns a new SsoRemoveLoginUrlType instance
func NewSsoRemoveLoginUrlType(Description string) *SsoRemoveLoginUrlType {
	s := new(SsoRemoveLoginUrlType)
	s.Description = Description
	return s
}

// SsoRemoveLogoutUrlDetails : Removed sign-out URL for SSO.
type SsoRemoveLogoutUrlDetails struct {
	// PreviousValue : Previous single sign-on logout URL.
	PreviousValue string `json:"previous_value"`
}

// NewSsoRemoveLogoutUrlDetails returns a new SsoRemoveLogoutUrlDetails instance
func NewSsoRemoveLogoutUrlDetails(PreviousValue string) *SsoRemoveLogoutUrlDetails {
	s := new(SsoRemoveLogoutUrlDetails)
	s.PreviousValue = PreviousValue
	return s
}

// SsoRemoveLogoutUrlType : has no documentation (yet)
type SsoRemoveLogoutUrlType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewSsoRemoveLogoutUrlType returns a new SsoRemoveLogoutUrlType instance
func NewSsoRemoveLogoutUrlType(Description string) *SsoRemoveLogoutUrlType {
	s := new(SsoRemoveLogoutUrlType)
	s.Description = Description
	return s
}

// TeamActivityCreateReportDetails : Created team activity report.
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

// TeamActivityCreateReportType : has no documentation (yet)
type TeamActivityCreateReportType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamActivityCreateReportType returns a new TeamActivityCreateReportType instance
func NewTeamActivityCreateReportType(Description string) *TeamActivityCreateReportType {
	s := new(TeamActivityCreateReportType)
	s.Description = Description
	return s
}

// TeamEvent : An audit log event.
type TeamEvent struct {
	// Timestamp : The Dropbox timestamp representing when the action was taken.
	Timestamp time.Time `json:"timestamp"`
	// EventCategory : The category that this type of action belongs to.
	EventCategory *EventCategory `json:"event_category"`
	// Actor : The entity who actually performed the action. Might be missing
	// due to historical data gap.
	Actor *ActorLogInfo `json:"actor,omitempty"`
	// Origin : The origin from which the actor performed the action including
	// information about host, ip address, location, session, etc. If the action
	// was performed programmatically via the API the origin represents the API
	// client.
	Origin *OriginLogInfo `json:"origin,omitempty"`
	// InvolveNonTeamMember : True if the action involved a non team member
	// either as the actor or as one of the affected users. Might be missing due
	// to historical data gap.
	InvolveNonTeamMember bool `json:"involve_non_team_member,omitempty"`
	// Context : The user or team on whose behalf the actor performed the
	// action. Might be missing due to historical data gap.
	Context *ContextLogInfo `json:"context,omitempty"`
	// Participants : Zero or more users and/or groups that are affected by the
	// action. Note that this list doesn't include any actors or users in
	// context.
	Participants []*ParticipantLogInfo `json:"participants,omitempty"`
	// Assets : Zero or more content assets involved in the action. Currently
	// these include Dropbox files and folders but in the future we might add
	// other asset types such as Paper documents, folders, projects, etc.
	Assets []*AssetLogInfo `json:"assets,omitempty"`
	// EventType : The particular type of action taken.
	EventType *EventType `json:"event_type"`
	// Details : The variable event schema applicable to this type of action,
	// instantiated with respect to this particular action.
	Details *EventDetails `json:"details"`
}

// NewTeamEvent returns a new TeamEvent instance
func NewTeamEvent(Timestamp time.Time, EventCategory *EventCategory, EventType *EventType, Details *EventDetails) *TeamEvent {
	s := new(TeamEvent)
	s.Timestamp = Timestamp
	s.EventCategory = EventCategory
	s.EventType = EventType
	s.Details = Details
	return s
}

// TeamFolderChangeStatusDetails : Changed archival status of team folder.
type TeamFolderChangeStatusDetails struct {
	// NewValue : New team folder status.
	NewValue *team.TeamFolderStatus `json:"new_value"`
	// PreviousValue : Previous team folder status. Might be missing due to
	// historical data gap.
	PreviousValue *team.TeamFolderStatus `json:"previous_value,omitempty"`
}

// NewTeamFolderChangeStatusDetails returns a new TeamFolderChangeStatusDetails instance
func NewTeamFolderChangeStatusDetails(NewValue *team.TeamFolderStatus) *TeamFolderChangeStatusDetails {
	s := new(TeamFolderChangeStatusDetails)
	s.NewValue = NewValue
	return s
}

// TeamFolderChangeStatusType : has no documentation (yet)
type TeamFolderChangeStatusType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamFolderChangeStatusType returns a new TeamFolderChangeStatusType instance
func NewTeamFolderChangeStatusType(Description string) *TeamFolderChangeStatusType {
	s := new(TeamFolderChangeStatusType)
	s.Description = Description
	return s
}

// TeamFolderCreateDetails : Created team folder in active status.
type TeamFolderCreateDetails struct {
}

// NewTeamFolderCreateDetails returns a new TeamFolderCreateDetails instance
func NewTeamFolderCreateDetails() *TeamFolderCreateDetails {
	s := new(TeamFolderCreateDetails)
	return s
}

// TeamFolderCreateType : has no documentation (yet)
type TeamFolderCreateType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamFolderCreateType returns a new TeamFolderCreateType instance
func NewTeamFolderCreateType(Description string) *TeamFolderCreateType {
	s := new(TeamFolderCreateType)
	s.Description = Description
	return s
}

// TeamFolderDowngradeDetails : Downgraded team folder to regular shared folder.
type TeamFolderDowngradeDetails struct {
	// TargetAssetIndex : Target asset position in the Assets list.
	TargetAssetIndex uint64 `json:"target_asset_index"`
}

// NewTeamFolderDowngradeDetails returns a new TeamFolderDowngradeDetails instance
func NewTeamFolderDowngradeDetails(TargetAssetIndex uint64) *TeamFolderDowngradeDetails {
	s := new(TeamFolderDowngradeDetails)
	s.TargetAssetIndex = TargetAssetIndex
	return s
}

// TeamFolderDowngradeType : has no documentation (yet)
type TeamFolderDowngradeType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamFolderDowngradeType returns a new TeamFolderDowngradeType instance
func NewTeamFolderDowngradeType(Description string) *TeamFolderDowngradeType {
	s := new(TeamFolderDowngradeType)
	s.Description = Description
	return s
}

// TeamFolderPermanentlyDeleteDetails : Permanently deleted archived team
// folder.
type TeamFolderPermanentlyDeleteDetails struct {
}

// NewTeamFolderPermanentlyDeleteDetails returns a new TeamFolderPermanentlyDeleteDetails instance
func NewTeamFolderPermanentlyDeleteDetails() *TeamFolderPermanentlyDeleteDetails {
	s := new(TeamFolderPermanentlyDeleteDetails)
	return s
}

// TeamFolderPermanentlyDeleteType : has no documentation (yet)
type TeamFolderPermanentlyDeleteType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamFolderPermanentlyDeleteType returns a new TeamFolderPermanentlyDeleteType instance
func NewTeamFolderPermanentlyDeleteType(Description string) *TeamFolderPermanentlyDeleteType {
	s := new(TeamFolderPermanentlyDeleteType)
	s.Description = Description
	return s
}

// TeamFolderRenameDetails : Renamed active/archived team folder.
type TeamFolderRenameDetails struct {
	// PreviousFolderName : Previous folder name.
	PreviousFolderName string `json:"previous_folder_name"`
	// NewFolderName : New folder name.
	NewFolderName string `json:"new_folder_name"`
}

// NewTeamFolderRenameDetails returns a new TeamFolderRenameDetails instance
func NewTeamFolderRenameDetails(PreviousFolderName string, NewFolderName string) *TeamFolderRenameDetails {
	s := new(TeamFolderRenameDetails)
	s.PreviousFolderName = PreviousFolderName
	s.NewFolderName = NewFolderName
	return s
}

// TeamFolderRenameType : has no documentation (yet)
type TeamFolderRenameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamFolderRenameType returns a new TeamFolderRenameType instance
func NewTeamFolderRenameType(Description string) *TeamFolderRenameType {
	s := new(TeamFolderRenameType)
	s.Description = Description
	return s
}

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

// TeamMergeFromType : has no documentation (yet)
type TeamMergeFromType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamMergeFromType returns a new TeamMergeFromType instance
func NewTeamMergeFromType(Description string) *TeamMergeFromType {
	s := new(TeamMergeFromType)
	s.Description = Description
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

// TeamMergeToType : has no documentation (yet)
type TeamMergeToType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamMergeToType returns a new TeamMergeToType instance
func NewTeamMergeToType(Description string) *TeamMergeToType {
	s := new(TeamMergeToType)
	s.Description = Description
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

// TeamProfileAddLogoDetails : Added team logo to display on shared link
// headers.
type TeamProfileAddLogoDetails struct {
}

// NewTeamProfileAddLogoDetails returns a new TeamProfileAddLogoDetails instance
func NewTeamProfileAddLogoDetails() *TeamProfileAddLogoDetails {
	s := new(TeamProfileAddLogoDetails)
	return s
}

// TeamProfileAddLogoType : has no documentation (yet)
type TeamProfileAddLogoType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamProfileAddLogoType returns a new TeamProfileAddLogoType instance
func NewTeamProfileAddLogoType(Description string) *TeamProfileAddLogoType {
	s := new(TeamProfileAddLogoType)
	s.Description = Description
	return s
}

// TeamProfileChangeDefaultLanguageDetails : Changed default language for team.
type TeamProfileChangeDefaultLanguageDetails struct {
	// NewValue : New team's default language.
	NewValue string `json:"new_value"`
	// PreviousValue : Previous team's default language.
	PreviousValue string `json:"previous_value"`
}

// NewTeamProfileChangeDefaultLanguageDetails returns a new TeamProfileChangeDefaultLanguageDetails instance
func NewTeamProfileChangeDefaultLanguageDetails(NewValue string, PreviousValue string) *TeamProfileChangeDefaultLanguageDetails {
	s := new(TeamProfileChangeDefaultLanguageDetails)
	s.NewValue = NewValue
	s.PreviousValue = PreviousValue
	return s
}

// TeamProfileChangeDefaultLanguageType : has no documentation (yet)
type TeamProfileChangeDefaultLanguageType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamProfileChangeDefaultLanguageType returns a new TeamProfileChangeDefaultLanguageType instance
func NewTeamProfileChangeDefaultLanguageType(Description string) *TeamProfileChangeDefaultLanguageType {
	s := new(TeamProfileChangeDefaultLanguageType)
	s.Description = Description
	return s
}

// TeamProfileChangeLogoDetails : Changed team logo displayed on shared link
// headers.
type TeamProfileChangeLogoDetails struct {
}

// NewTeamProfileChangeLogoDetails returns a new TeamProfileChangeLogoDetails instance
func NewTeamProfileChangeLogoDetails() *TeamProfileChangeLogoDetails {
	s := new(TeamProfileChangeLogoDetails)
	return s
}

// TeamProfileChangeLogoType : has no documentation (yet)
type TeamProfileChangeLogoType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamProfileChangeLogoType returns a new TeamProfileChangeLogoType instance
func NewTeamProfileChangeLogoType(Description string) *TeamProfileChangeLogoType {
	s := new(TeamProfileChangeLogoType)
	s.Description = Description
	return s
}

// TeamProfileChangeNameDetails : Changed team name.
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

// TeamProfileChangeNameType : has no documentation (yet)
type TeamProfileChangeNameType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamProfileChangeNameType returns a new TeamProfileChangeNameType instance
func NewTeamProfileChangeNameType(Description string) *TeamProfileChangeNameType {
	s := new(TeamProfileChangeNameType)
	s.Description = Description
	return s
}

// TeamProfileRemoveLogoDetails : Removed team logo displayed on shared link
// headers.
type TeamProfileRemoveLogoDetails struct {
}

// NewTeamProfileRemoveLogoDetails returns a new TeamProfileRemoveLogoDetails instance
func NewTeamProfileRemoveLogoDetails() *TeamProfileRemoveLogoDetails {
	s := new(TeamProfileRemoveLogoDetails)
	return s
}

// TeamProfileRemoveLogoType : has no documentation (yet)
type TeamProfileRemoveLogoType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamProfileRemoveLogoType returns a new TeamProfileRemoveLogoType instance
func NewTeamProfileRemoveLogoType(Description string) *TeamProfileRemoveLogoType {
	s := new(TeamProfileRemoveLogoType)
	s.Description = Description
	return s
}

// TeamSelectiveSyncSettingsChangedDetails : Changed sync default.
type TeamSelectiveSyncSettingsChangedDetails struct {
	// PreviousValue : Previous value.
	PreviousValue *files.SyncSetting `json:"previous_value"`
	// NewValue : New value.
	NewValue *files.SyncSetting `json:"new_value"`
}

// NewTeamSelectiveSyncSettingsChangedDetails returns a new TeamSelectiveSyncSettingsChangedDetails instance
func NewTeamSelectiveSyncSettingsChangedDetails(PreviousValue *files.SyncSetting, NewValue *files.SyncSetting) *TeamSelectiveSyncSettingsChangedDetails {
	s := new(TeamSelectiveSyncSettingsChangedDetails)
	s.PreviousValue = PreviousValue
	s.NewValue = NewValue
	return s
}

// TeamSelectiveSyncSettingsChangedType : has no documentation (yet)
type TeamSelectiveSyncSettingsChangedType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTeamSelectiveSyncSettingsChangedType returns a new TeamSelectiveSyncSettingsChangedType instance
func NewTeamSelectiveSyncSettingsChangedType(Description string) *TeamSelectiveSyncSettingsChangedType {
	s := new(TeamSelectiveSyncSettingsChangedType)
	s.Description = Description
	return s
}

// TfaAddBackupPhoneDetails : Added backup phone for two-step verification.
type TfaAddBackupPhoneDetails struct {
}

// NewTfaAddBackupPhoneDetails returns a new TfaAddBackupPhoneDetails instance
func NewTfaAddBackupPhoneDetails() *TfaAddBackupPhoneDetails {
	s := new(TfaAddBackupPhoneDetails)
	return s
}

// TfaAddBackupPhoneType : has no documentation (yet)
type TfaAddBackupPhoneType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaAddBackupPhoneType returns a new TfaAddBackupPhoneType instance
func NewTfaAddBackupPhoneType(Description string) *TfaAddBackupPhoneType {
	s := new(TfaAddBackupPhoneType)
	s.Description = Description
	return s
}

// TfaAddSecurityKeyDetails : Added security key for two-step verification.
type TfaAddSecurityKeyDetails struct {
}

// NewTfaAddSecurityKeyDetails returns a new TfaAddSecurityKeyDetails instance
func NewTfaAddSecurityKeyDetails() *TfaAddSecurityKeyDetails {
	s := new(TfaAddSecurityKeyDetails)
	return s
}

// TfaAddSecurityKeyType : has no documentation (yet)
type TfaAddSecurityKeyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaAddSecurityKeyType returns a new TfaAddSecurityKeyType instance
func NewTfaAddSecurityKeyType(Description string) *TfaAddSecurityKeyType {
	s := new(TfaAddSecurityKeyType)
	s.Description = Description
	return s
}

// TfaChangeBackupPhoneDetails : Changed backup phone for two-step verification.
type TfaChangeBackupPhoneDetails struct {
}

// NewTfaChangeBackupPhoneDetails returns a new TfaChangeBackupPhoneDetails instance
func NewTfaChangeBackupPhoneDetails() *TfaChangeBackupPhoneDetails {
	s := new(TfaChangeBackupPhoneDetails)
	return s
}

// TfaChangeBackupPhoneType : has no documentation (yet)
type TfaChangeBackupPhoneType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaChangeBackupPhoneType returns a new TfaChangeBackupPhoneType instance
func NewTfaChangeBackupPhoneType(Description string) *TfaChangeBackupPhoneType {
	s := new(TfaChangeBackupPhoneType)
	s.Description = Description
	return s
}

// TfaChangePolicyDetails : Changed two-step verification setting for team.
type TfaChangePolicyDetails struct {
	// NewValue : New change policy.
	NewValue *team_policies.TwoStepVerificationPolicy `json:"new_value"`
	// PreviousValue : Previous change policy. Might be missing due to
	// historical data gap.
	PreviousValue *team_policies.TwoStepVerificationPolicy `json:"previous_value,omitempty"`
}

// NewTfaChangePolicyDetails returns a new TfaChangePolicyDetails instance
func NewTfaChangePolicyDetails(NewValue *team_policies.TwoStepVerificationPolicy) *TfaChangePolicyDetails {
	s := new(TfaChangePolicyDetails)
	s.NewValue = NewValue
	return s
}

// TfaChangePolicyType : has no documentation (yet)
type TfaChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaChangePolicyType returns a new TfaChangePolicyType instance
func NewTfaChangePolicyType(Description string) *TfaChangePolicyType {
	s := new(TfaChangePolicyType)
	s.Description = Description
	return s
}

// TfaChangeStatusDetails : Enabled/disabled/changed two-step verification
// setting.
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

// TfaChangeStatusType : has no documentation (yet)
type TfaChangeStatusType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaChangeStatusType returns a new TfaChangeStatusType instance
func NewTfaChangeStatusType(Description string) *TfaChangeStatusType {
	s := new(TfaChangeStatusType)
	s.Description = Description
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

// TfaRemoveBackupPhoneDetails : Removed backup phone for two-step verification.
type TfaRemoveBackupPhoneDetails struct {
}

// NewTfaRemoveBackupPhoneDetails returns a new TfaRemoveBackupPhoneDetails instance
func NewTfaRemoveBackupPhoneDetails() *TfaRemoveBackupPhoneDetails {
	s := new(TfaRemoveBackupPhoneDetails)
	return s
}

// TfaRemoveBackupPhoneType : has no documentation (yet)
type TfaRemoveBackupPhoneType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaRemoveBackupPhoneType returns a new TfaRemoveBackupPhoneType instance
func NewTfaRemoveBackupPhoneType(Description string) *TfaRemoveBackupPhoneType {
	s := new(TfaRemoveBackupPhoneType)
	s.Description = Description
	return s
}

// TfaRemoveSecurityKeyDetails : Removed security key for two-step verification.
type TfaRemoveSecurityKeyDetails struct {
}

// NewTfaRemoveSecurityKeyDetails returns a new TfaRemoveSecurityKeyDetails instance
func NewTfaRemoveSecurityKeyDetails() *TfaRemoveSecurityKeyDetails {
	s := new(TfaRemoveSecurityKeyDetails)
	return s
}

// TfaRemoveSecurityKeyType : has no documentation (yet)
type TfaRemoveSecurityKeyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaRemoveSecurityKeyType returns a new TfaRemoveSecurityKeyType instance
func NewTfaRemoveSecurityKeyType(Description string) *TfaRemoveSecurityKeyType {
	s := new(TfaRemoveSecurityKeyType)
	s.Description = Description
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

// TfaResetType : has no documentation (yet)
type TfaResetType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTfaResetType returns a new TfaResetType instance
func NewTfaResetType(Description string) *TfaResetType {
	s := new(TfaResetType)
	s.Description = Description
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

// TwoAccountChangePolicyDetails : Enabled/disabled option for members to link
// personal Dropbox account and team account to same computer.
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

// TwoAccountChangePolicyType : has no documentation (yet)
type TwoAccountChangePolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewTwoAccountChangePolicyType returns a new TwoAccountChangePolicyType instance
func NewTwoAccountChangePolicyType(Description string) *TwoAccountChangePolicyType {
	s := new(TwoAccountChangePolicyType)
	s.Description = Description
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

// WebDeviceSessionLogInfo : Information on active web sessions
type WebDeviceSessionLogInfo struct {
	DeviceSessionLogInfo
	// SessionInfo : Web session unique id. Might be missing due to historical
	// data gap.
	SessionInfo *WebSessionLogInfo `json:"session_info,omitempty"`
	// UserAgent : Information on the hosting device.
	UserAgent string `json:"user_agent"`
	// Os : Information on the hosting operating system.
	Os string `json:"os"`
	// Browser : Information on the browser used for this web session.
	Browser string `json:"browser"`
}

// NewWebDeviceSessionLogInfo returns a new WebDeviceSessionLogInfo instance
func NewWebDeviceSessionLogInfo(UserAgent string, Os string, Browser string) *WebDeviceSessionLogInfo {
	s := new(WebDeviceSessionLogInfo)
	s.UserAgent = UserAgent
	s.Os = Os
	s.Browser = Browser
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

// WebSessionsChangeFixedLengthPolicyDetails : Changed how long members can stay
// signed in to Dropbox.com.
type WebSessionsChangeFixedLengthPolicyDetails struct {
	// NewValue : New session length policy. Might be missing due to historical
	// data gap.
	NewValue *WebSessionsFixedLengthPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous session length policy. Might be missing due to
	// historical data gap.
	PreviousValue *WebSessionsFixedLengthPolicy `json:"previous_value,omitempty"`
}

// NewWebSessionsChangeFixedLengthPolicyDetails returns a new WebSessionsChangeFixedLengthPolicyDetails instance
func NewWebSessionsChangeFixedLengthPolicyDetails() *WebSessionsChangeFixedLengthPolicyDetails {
	s := new(WebSessionsChangeFixedLengthPolicyDetails)
	return s
}

// WebSessionsChangeFixedLengthPolicyType : has no documentation (yet)
type WebSessionsChangeFixedLengthPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewWebSessionsChangeFixedLengthPolicyType returns a new WebSessionsChangeFixedLengthPolicyType instance
func NewWebSessionsChangeFixedLengthPolicyType(Description string) *WebSessionsChangeFixedLengthPolicyType {
	s := new(WebSessionsChangeFixedLengthPolicyType)
	s.Description = Description
	return s
}

// WebSessionsChangeIdleLengthPolicyDetails : Changed how long team members can
// be idle while signed in to Dropbox.com.
type WebSessionsChangeIdleLengthPolicyDetails struct {
	// NewValue : New idle length policy. Might be missing due to historical
	// data gap.
	NewValue *WebSessionsIdleLengthPolicy `json:"new_value,omitempty"`
	// PreviousValue : Previous idle length policy. Might be missing due to
	// historical data gap.
	PreviousValue *WebSessionsIdleLengthPolicy `json:"previous_value,omitempty"`
}

// NewWebSessionsChangeIdleLengthPolicyDetails returns a new WebSessionsChangeIdleLengthPolicyDetails instance
func NewWebSessionsChangeIdleLengthPolicyDetails() *WebSessionsChangeIdleLengthPolicyDetails {
	s := new(WebSessionsChangeIdleLengthPolicyDetails)
	return s
}

// WebSessionsChangeIdleLengthPolicyType : has no documentation (yet)
type WebSessionsChangeIdleLengthPolicyType struct {
	// Description : has no documentation (yet)
	Description string `json:"description"`
}

// NewWebSessionsChangeIdleLengthPolicyType returns a new WebSessionsChangeIdleLengthPolicyType instance
func NewWebSessionsChangeIdleLengthPolicyType(Description string) *WebSessionsChangeIdleLengthPolicyType {
	s := new(WebSessionsChangeIdleLengthPolicyType)
	s.Description = Description
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
