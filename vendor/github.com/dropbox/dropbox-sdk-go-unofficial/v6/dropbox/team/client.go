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

package team

import (
	"encoding/json"
	"io"
	"log"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/async"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/auth"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/file_properties"
)

// Client interface describes all routes in this namespace
type Client interface {
	// DevicesListMemberDevices : List all device sessions of a team's member.
	DevicesListMemberDevices(arg *ListMemberDevicesArg) (res *ListMemberDevicesResult, err error)
	// DevicesListMembersDevices : List all device sessions of a team.
	// Permission : Team member file access.
	DevicesListMembersDevices(arg *ListMembersDevicesArg) (res *ListMembersDevicesResult, err error)
	// DevicesListTeamDevices : List all device sessions of a team. Permission :
	// Team member file access.
	// Deprecated: Use `DevicesListMembersDevices` instead
	DevicesListTeamDevices(arg *ListTeamDevicesArg) (res *ListTeamDevicesResult, err error)
	// DevicesRevokeDeviceSession : Revoke a device session of a team's member.
	DevicesRevokeDeviceSession(arg *RevokeDeviceSessionArg) (err error)
	// DevicesRevokeDeviceSessionBatch : Revoke a list of device sessions of
	// team members.
	DevicesRevokeDeviceSessionBatch(arg *RevokeDeviceSessionBatchArg) (res *RevokeDeviceSessionBatchResult, err error)
	// FeaturesGetValues : Get the values for one or more featues. This route
	// allows you to check your account's capability for what feature you can
	// access or what value you have for certain features. Permission : Team
	// information.
	FeaturesGetValues(arg *FeaturesGetValuesBatchArg) (res *FeaturesGetValuesBatchResult, err error)
	// GetInfo : Retrieves information about a team.
	GetInfo() (res *TeamGetInfoResult, err error)
	// GroupsCreate : Creates a new, empty group, with a requested name.
	// Permission : Team member management.
	GroupsCreate(arg *GroupCreateArg) (res *GroupFullInfo, err error)
	// GroupsDelete : Deletes a group. The group is deleted immediately. However
	// the revoking of group-owned resources may take additional time. Use the
	// `groupsJobStatusGet` to determine whether this process has completed.
	// Permission : Team member management.
	GroupsDelete(arg *GroupSelector) (res *async.LaunchEmptyResult, err error)
	// GroupsGetInfo : Retrieves information about one or more groups. Note that
	// the optional field  `GroupFullInfo.members` is not returned for
	// system-managed groups. Permission : Team Information.
	GroupsGetInfo(arg *GroupsSelector) (res []*GroupsGetInfoItem, err error)
	// GroupsJobStatusGet : Once an async_job_id is returned from
	// `groupsDelete`, `groupsMembersAdd` , or `groupsMembersRemove` use this
	// method to poll the status of granting/revoking group members' access to
	// group-owned resources. Permission : Team member management.
	GroupsJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error)
	// GroupsList : Lists groups on a team. Permission : Team Information.
	GroupsList(arg *GroupsListArg) (res *GroupsListResult, err error)
	// GroupsListContinue : Once a cursor has been retrieved from `groupsList`,
	// use this to paginate through all groups. Permission : Team Information.
	GroupsListContinue(arg *GroupsListContinueArg) (res *GroupsListResult, err error)
	// GroupsMembersAdd : Adds members to a group. The members are added
	// immediately. However the granting of group-owned resources may take
	// additional time. Use the `groupsJobStatusGet` to determine whether this
	// process has completed. Permission : Team member management.
	GroupsMembersAdd(arg *GroupMembersAddArg) (res *GroupMembersChangeResult, err error)
	// GroupsMembersList : Lists members of a group. Permission : Team
	// Information.
	GroupsMembersList(arg *GroupsMembersListArg) (res *GroupsMembersListResult, err error)
	// GroupsMembersListContinue : Once a cursor has been retrieved from
	// `groupsMembersList`, use this to paginate through all members of the
	// group. Permission : Team information.
	GroupsMembersListContinue(arg *GroupsMembersListContinueArg) (res *GroupsMembersListResult, err error)
	// GroupsMembersRemove : Removes members from a group. The members are
	// removed immediately. However the revoking of group-owned resources may
	// take additional time. Use the `groupsJobStatusGet` to determine whether
	// this process has completed. This method permits removing the only owner
	// of a group, even in cases where this is not possible via the web client.
	// Permission : Team member management.
	GroupsMembersRemove(arg *GroupMembersRemoveArg) (res *GroupMembersChangeResult, err error)
	// GroupsMembersSetAccessType : Sets a member's access type in a group.
	// Permission : Team member management.
	GroupsMembersSetAccessType(arg *GroupMembersSetAccessTypeArg) (res []*GroupsGetInfoItem, err error)
	// GroupsUpdate : Updates a group's name and/or external ID. Permission :
	// Team member management.
	GroupsUpdate(arg *GroupUpdateArgs) (res *GroupFullInfo, err error)
	// LegalHoldsCreatePolicy : Creates new legal hold policy. Note: Legal Holds
	// is a paid add-on. Not all teams have the feature. Permission : Team
	// member file access.
	LegalHoldsCreatePolicy(arg *LegalHoldsPolicyCreateArg) (res *LegalHoldPolicy, err error)
	// LegalHoldsGetPolicy : Gets a legal hold by Id. Note: Legal Holds is a
	// paid add-on. Not all teams have the feature. Permission : Team member
	// file access.
	LegalHoldsGetPolicy(arg *LegalHoldsGetPolicyArg) (res *LegalHoldPolicy, err error)
	// LegalHoldsListHeldRevisions : List the file metadata that's under the
	// hold. Note: Legal Holds is a paid add-on. Not all teams have the feature.
	// Permission : Team member file access.
	LegalHoldsListHeldRevisions(arg *LegalHoldsListHeldRevisionsArg) (res *LegalHoldsListHeldRevisionResult, err error)
	// LegalHoldsListHeldRevisionsContinue : Continue listing the file metadata
	// that's under the hold. Note: Legal Holds is a paid add-on. Not all teams
	// have the feature. Permission : Team member file access.
	LegalHoldsListHeldRevisionsContinue(arg *LegalHoldsListHeldRevisionsContinueArg) (res *LegalHoldsListHeldRevisionResult, err error)
	// LegalHoldsListPolicies : Lists legal holds on a team. Note: Legal Holds
	// is a paid add-on. Not all teams have the feature. Permission : Team
	// member file access.
	LegalHoldsListPolicies(arg *LegalHoldsListPoliciesArg) (res *LegalHoldsListPoliciesResult, err error)
	// LegalHoldsReleasePolicy : Releases a legal hold by Id. Note: Legal Holds
	// is a paid add-on. Not all teams have the feature. Permission : Team
	// member file access.
	LegalHoldsReleasePolicy(arg *LegalHoldsPolicyReleaseArg) (err error)
	// LegalHoldsUpdatePolicy : Updates a legal hold. Note: Legal Holds is a
	// paid add-on. Not all teams have the feature. Permission : Team member
	// file access.
	LegalHoldsUpdatePolicy(arg *LegalHoldsPolicyUpdateArg) (res *LegalHoldPolicy, err error)
	// LinkedAppsListMemberLinkedApps : List all linked applications of the team
	// member. Note, this endpoint does not list any team-linked applications.
	LinkedAppsListMemberLinkedApps(arg *ListMemberAppsArg) (res *ListMemberAppsResult, err error)
	// LinkedAppsListMembersLinkedApps : List all applications linked to the
	// team members' accounts. Note, this endpoint does not list any team-linked
	// applications.
	LinkedAppsListMembersLinkedApps(arg *ListMembersAppsArg) (res *ListMembersAppsResult, err error)
	// LinkedAppsListTeamLinkedApps : List all applications linked to the team
	// members' accounts. Note, this endpoint doesn't list any team-linked
	// applications.
	// Deprecated: Use `LinkedAppsListMembersLinkedApps` instead
	LinkedAppsListTeamLinkedApps(arg *ListTeamAppsArg) (res *ListTeamAppsResult, err error)
	// LinkedAppsRevokeLinkedApp : Revoke a linked application of the team
	// member.
	LinkedAppsRevokeLinkedApp(arg *RevokeLinkedApiAppArg) (err error)
	// LinkedAppsRevokeLinkedAppBatch : Revoke a list of linked applications of
	// the team members.
	LinkedAppsRevokeLinkedAppBatch(arg *RevokeLinkedApiAppBatchArg) (res *RevokeLinkedAppBatchResult, err error)
	// MemberSpaceLimitsExcludedUsersAdd : Add users to member space limits
	// excluded users list.
	MemberSpaceLimitsExcludedUsersAdd(arg *ExcludedUsersUpdateArg) (res *ExcludedUsersUpdateResult, err error)
	// MemberSpaceLimitsExcludedUsersList : List member space limits excluded
	// users.
	MemberSpaceLimitsExcludedUsersList(arg *ExcludedUsersListArg) (res *ExcludedUsersListResult, err error)
	// MemberSpaceLimitsExcludedUsersListContinue : Continue listing member
	// space limits excluded users.
	MemberSpaceLimitsExcludedUsersListContinue(arg *ExcludedUsersListContinueArg) (res *ExcludedUsersListResult, err error)
	// MemberSpaceLimitsExcludedUsersRemove : Remove users from member space
	// limits excluded users list.
	MemberSpaceLimitsExcludedUsersRemove(arg *ExcludedUsersUpdateArg) (res *ExcludedUsersUpdateResult, err error)
	// MemberSpaceLimitsGetCustomQuota : Get users custom quota. Returns none as
	// the custom quota if none was set. A maximum of 1000 members can be
	// specified in a single call.
	MemberSpaceLimitsGetCustomQuota(arg *CustomQuotaUsersArg) (res []*CustomQuotaResult, err error)
	// MemberSpaceLimitsRemoveCustomQuota : Remove users custom quota. A maximum
	// of 1000 members can be specified in a single call.
	MemberSpaceLimitsRemoveCustomQuota(arg *CustomQuotaUsersArg) (res []*RemoveCustomQuotaResult, err error)
	// MemberSpaceLimitsSetCustomQuota : Set users custom quota. Custom quota
	// has to be at least 15GB. A maximum of 1000 members can be specified in a
	// single call.
	MemberSpaceLimitsSetCustomQuota(arg *SetCustomQuotaArg) (res []*CustomQuotaResult, err error)
	// MembersAdd : Adds members to a team. Permission : Team member management
	// A maximum of 20 members can be specified in a single call. If no Dropbox
	// account exists with the email address specified, a new Dropbox account
	// will be created with the given email address, and that account will be
	// invited to the team. If a personal Dropbox account exists with the email
	// address specified in the call, this call will create a placeholder
	// Dropbox account for the user on the team and send an email inviting the
	// user to migrate their existing personal account onto the team. Team
	// member management apps are required to set an initial given_name and
	// surname for a user to use in the team invitation and for 'Perform as team
	// member' actions taken on the user before they become 'active'.
	MembersAddV2(arg *MembersAddV2Arg) (res *MembersAddLaunchV2Result, err error)
	// MembersAdd : Adds members to a team. Permission : Team member management
	// A maximum of 20 members can be specified in a single call. If no Dropbox
	// account exists with the email address specified, a new Dropbox account
	// will be created with the given email address, and that account will be
	// invited to the team. If a personal Dropbox account exists with the email
	// address specified in the call, this call will create a placeholder
	// Dropbox account for the user on the team and send an email inviting the
	// user to migrate their existing personal account onto the team. Team
	// member management apps are required to set an initial given_name and
	// surname for a user to use in the team invitation and for 'Perform as team
	// member' actions taken on the user before they become 'active'.
	MembersAdd(arg *MembersAddArg) (res *MembersAddLaunch, err error)
	// MembersAddJobStatusGet : Once an async_job_id is returned from
	// `membersAdd` , use this to poll the status of the asynchronous request.
	// Permission : Team member management.
	MembersAddJobStatusGetV2(arg *async.PollArg) (res *MembersAddJobStatusV2Result, err error)
	// MembersAddJobStatusGet : Once an async_job_id is returned from
	// `membersAdd` , use this to poll the status of the asynchronous request.
	// Permission : Team member management.
	MembersAddJobStatusGet(arg *async.PollArg) (res *MembersAddJobStatus, err error)
	// MembersDeleteProfilePhoto : Deletes a team member's profile photo.
	// Permission : Team member management.
	MembersDeleteProfilePhotoV2(arg *MembersDeleteProfilePhotoArg) (res *TeamMemberInfoV2Result, err error)
	// MembersDeleteProfilePhoto : Deletes a team member's profile photo.
	// Permission : Team member management.
	MembersDeleteProfilePhoto(arg *MembersDeleteProfilePhotoArg) (res *TeamMemberInfo, err error)
	// MembersGetAvailableTeamMemberRoles : Get available TeamMemberRoles for
	// the connected team. To be used with `membersSetAdminPermissions`.
	// Permission : Team member management.
	MembersGetAvailableTeamMemberRoles() (res *MembersGetAvailableTeamMemberRolesResult, err error)
	// MembersGetInfo : Returns information about multiple team members.
	// Permission : Team information This endpoint will return
	// `MembersGetInfoItem.id_not_found`, for IDs (or emails) that cannot be
	// matched to a valid team member.
	MembersGetInfoV2(arg *MembersGetInfoV2Arg) (res *MembersGetInfoV2Result, err error)
	// MembersGetInfo : Returns information about multiple team members.
	// Permission : Team information This endpoint will return
	// `MembersGetInfoItem.id_not_found`, for IDs (or emails) that cannot be
	// matched to a valid team member.
	MembersGetInfo(arg *MembersGetInfoArgs) (res []*MembersGetInfoItem, err error)
	// MembersList : Lists members of a team. Permission : Team information.
	MembersListV2(arg *MembersListArg) (res *MembersListV2Result, err error)
	// MembersList : Lists members of a team. Permission : Team information.
	MembersList(arg *MembersListArg) (res *MembersListResult, err error)
	// MembersListContinue : Once a cursor has been retrieved from
	// `membersList`, use this to paginate through all team members. Permission
	// : Team information.
	MembersListContinueV2(arg *MembersListContinueArg) (res *MembersListV2Result, err error)
	// MembersListContinue : Once a cursor has been retrieved from
	// `membersList`, use this to paginate through all team members. Permission
	// : Team information.
	MembersListContinue(arg *MembersListContinueArg) (res *MembersListResult, err error)
	// MembersMoveFormerMemberFiles : Moves removed member's files to a
	// different member. This endpoint initiates an asynchronous job. To obtain
	// the final result of the job, the client should periodically poll
	// `membersMoveFormerMemberFilesJobStatusCheck`. Permission : Team member
	// management.
	MembersMoveFormerMemberFiles(arg *MembersDataTransferArg) (res *async.LaunchEmptyResult, err error)
	// MembersMoveFormerMemberFilesJobStatusCheck : Once an async_job_id is
	// returned from `membersMoveFormerMemberFiles` , use this to poll the
	// status of the asynchronous request. Permission : Team member management.
	MembersMoveFormerMemberFilesJobStatusCheck(arg *async.PollArg) (res *async.PollEmptyResult, err error)
	// MembersRecover : Recover a deleted member. Permission : Team member
	// management Exactly one of team_member_id, email, or external_id must be
	// provided to identify the user account.
	MembersRecover(arg *MembersRecoverArg) (err error)
	// MembersRemove : Removes a member from a team. Permission : Team member
	// management Exactly one of team_member_id, email, or external_id must be
	// provided to identify the user account. Accounts can be recovered via
	// `membersRecover` for a 7 day period or until the account has been
	// permanently deleted or transferred to another account (whichever comes
	// first). Calling `membersAdd` while a user is still recoverable on your
	// team will return with `MemberAddResult.user_already_on_team`. Accounts
	// can have their files transferred via the admin console for a limited
	// time, based on the version history length associated with the team (180
	// days for most teams). This endpoint may initiate an asynchronous job. To
	// obtain the final result of the job, the client should periodically poll
	// `membersRemoveJobStatusGet`.
	MembersRemove(arg *MembersRemoveArg) (res *async.LaunchEmptyResult, err error)
	// MembersRemoveJobStatusGet : Once an async_job_id is returned from
	// `membersRemove` , use this to poll the status of the asynchronous
	// request. Permission : Team member management.
	MembersRemoveJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error)
	// MembersSecondaryEmailsAdd : Add secondary emails to users. Permission :
	// Team member management. Emails that are on verified domains will be
	// verified automatically. For each email address not on a verified domain a
	// verification email will be sent.
	MembersSecondaryEmailsAdd(arg *AddSecondaryEmailsArg) (res *AddSecondaryEmailsResult, err error)
	// MembersSecondaryEmailsDelete : Delete secondary emails from users
	// Permission : Team member management. Users will be notified of deletions
	// of verified secondary emails at both the secondary email and their
	// primary email.
	MembersSecondaryEmailsDelete(arg *DeleteSecondaryEmailsArg) (res *DeleteSecondaryEmailsResult, err error)
	// MembersSecondaryEmailsResendVerificationEmails : Resend secondary email
	// verification emails. Permission : Team member management.
	MembersSecondaryEmailsResendVerificationEmails(arg *ResendVerificationEmailArg) (res *ResendVerificationEmailResult, err error)
	// MembersSendWelcomeEmail : Sends welcome email to pending team member.
	// Permission : Team member management Exactly one of team_member_id, email,
	// or external_id must be provided to identify the user account. No-op if
	// team member is not pending.
	MembersSendWelcomeEmail(arg *UserSelectorArg) (err error)
	// MembersSetAdminPermissions : Updates a team member's permissions.
	// Permission : Team member management.
	MembersSetAdminPermissionsV2(arg *MembersSetPermissions2Arg) (res *MembersSetPermissions2Result, err error)
	// MembersSetAdminPermissions : Updates a team member's permissions.
	// Permission : Team member management.
	MembersSetAdminPermissions(arg *MembersSetPermissionsArg) (res *MembersSetPermissionsResult, err error)
	// MembersSetProfile : Updates a team member's profile. Permission : Team
	// member management.
	MembersSetProfileV2(arg *MembersSetProfileArg) (res *TeamMemberInfoV2Result, err error)
	// MembersSetProfile : Updates a team member's profile. Permission : Team
	// member management.
	MembersSetProfile(arg *MembersSetProfileArg) (res *TeamMemberInfo, err error)
	// MembersSetProfilePhoto : Updates a team member's profile photo.
	// Permission : Team member management.
	MembersSetProfilePhotoV2(arg *MembersSetProfilePhotoArg) (res *TeamMemberInfoV2Result, err error)
	// MembersSetProfilePhoto : Updates a team member's profile photo.
	// Permission : Team member management.
	MembersSetProfilePhoto(arg *MembersSetProfilePhotoArg) (res *TeamMemberInfo, err error)
	// MembersSuspend : Suspend a member from a team. Permission : Team member
	// management Exactly one of team_member_id, email, or external_id must be
	// provided to identify the user account.
	MembersSuspend(arg *MembersDeactivateArg) (err error)
	// MembersUnsuspend : Unsuspend a member from a team. Permission : Team
	// member management Exactly one of team_member_id, email, or external_id
	// must be provided to identify the user account.
	MembersUnsuspend(arg *MembersUnsuspendArg) (err error)
	// NamespacesList : Returns a list of all team-accessible namespaces. This
	// list includes team folders, shared folders containing team members, team
	// members' home namespaces, and team members' app folders. Home namespaces
	// and app folders are always owned by this team or members of the team, but
	// shared folders may be owned by other users or other teams. Duplicates may
	// occur in the list.
	NamespacesList(arg *TeamNamespacesListArg) (res *TeamNamespacesListResult, err error)
	// NamespacesListContinue : Once a cursor has been retrieved from
	// `namespacesList`, use this to paginate through all team-accessible
	// namespaces. Duplicates may occur in the list.
	NamespacesListContinue(arg *TeamNamespacesListContinueArg) (res *TeamNamespacesListResult, err error)
	// PropertiesTemplateAdd : Permission : Team member file access.
	// Deprecated:
	PropertiesTemplateAdd(arg *file_properties.AddTemplateArg) (res *file_properties.AddTemplateResult, err error)
	// PropertiesTemplateGet : Permission : Team member file access. The scope
	// for the route is files.team_metadata.write.
	// Deprecated:
	PropertiesTemplateGet(arg *file_properties.GetTemplateArg) (res *file_properties.GetTemplateResult, err error)
	// PropertiesTemplateList : Permission : Team member file access. The scope
	// for the route is files.team_metadata.write.
	// Deprecated:
	PropertiesTemplateList() (res *file_properties.ListTemplateResult, err error)
	// PropertiesTemplateUpdate : Permission : Team member file access.
	// Deprecated:
	PropertiesTemplateUpdate(arg *file_properties.UpdateTemplateArg) (res *file_properties.UpdateTemplateResult, err error)
	// ReportsGetActivity : Retrieves reporting data about a team's user
	// activity. Deprecated: Will be removed on July 1st 2021.
	// Deprecated:
	ReportsGetActivity(arg *DateRange) (res *GetActivityReport, err error)
	// ReportsGetDevices : Retrieves reporting data about a team's linked
	// devices. Deprecated: Will be removed on July 1st 2021.
	// Deprecated:
	ReportsGetDevices(arg *DateRange) (res *GetDevicesReport, err error)
	// ReportsGetMembership : Retrieves reporting data about a team's
	// membership. Deprecated: Will be removed on July 1st 2021.
	// Deprecated:
	ReportsGetMembership(arg *DateRange) (res *GetMembershipReport, err error)
	// ReportsGetStorage : Retrieves reporting data about a team's storage
	// usage. Deprecated: Will be removed on July 1st 2021.
	// Deprecated:
	ReportsGetStorage(arg *DateRange) (res *GetStorageReport, err error)
	// TeamFolderActivate : Sets an archived team folder's status to active.
	// Permission : Team member file access.
	TeamFolderActivate(arg *TeamFolderIdArg) (res *TeamFolderMetadata, err error)
	// TeamFolderArchive : Sets an active team folder's status to archived and
	// removes all folder and file members. This endpoint cannot be used for
	// teams that have a shared team space. Permission : Team member file
	// access.
	TeamFolderArchive(arg *TeamFolderArchiveArg) (res *TeamFolderArchiveLaunch, err error)
	// TeamFolderArchiveCheck : Returns the status of an asynchronous job for
	// archiving a team folder. Permission : Team member file access.
	TeamFolderArchiveCheck(arg *async.PollArg) (res *TeamFolderArchiveJobStatus, err error)
	// TeamFolderCreate : Creates a new, active, team folder with no members.
	// This endpoint can only be used for teams that do not already have a
	// shared team space. Permission : Team member file access.
	TeamFolderCreate(arg *TeamFolderCreateArg) (res *TeamFolderMetadata, err error)
	// TeamFolderGetInfo : Retrieves metadata for team folders. Permission :
	// Team member file access.
	TeamFolderGetInfo(arg *TeamFolderIdListArg) (res []*TeamFolderGetInfoItem, err error)
	// TeamFolderList : Lists all team folders. Permission : Team member file
	// access.
	TeamFolderList(arg *TeamFolderListArg) (res *TeamFolderListResult, err error)
	// TeamFolderListContinue : Once a cursor has been retrieved from
	// `teamFolderList`, use this to paginate through all team folders.
	// Permission : Team member file access.
	TeamFolderListContinue(arg *TeamFolderListContinueArg) (res *TeamFolderListResult, err error)
	// TeamFolderPermanentlyDelete : Permanently deletes an archived team
	// folder. This endpoint cannot be used for teams that have a shared team
	// space. Permission : Team member file access.
	TeamFolderPermanentlyDelete(arg *TeamFolderIdArg) (err error)
	// TeamFolderRename : Changes an active team folder's name. Permission :
	// Team member file access.
	TeamFolderRename(arg *TeamFolderRenameArg) (res *TeamFolderMetadata, err error)
	// TeamFolderUpdateSyncSettings : Updates the sync settings on a team folder
	// or its contents.  Use of this endpoint requires that the team has team
	// selective sync enabled.
	TeamFolderUpdateSyncSettings(arg *TeamFolderUpdateSyncSettingsArg) (res *TeamFolderMetadata, err error)
	// TokenGetAuthenticatedAdmin : Returns the member profile of the admin who
	// generated the team access token used to make the call.
	TokenGetAuthenticatedAdmin() (res *TokenGetAuthenticatedAdminResult, err error)
}

type apiImpl dropbox.Context

//DevicesListMemberDevicesAPIError is an error-wrapper for the devices/list_member_devices route
type DevicesListMemberDevicesAPIError struct {
	dropbox.APIError
	EndpointError *ListMemberDevicesError `json:"error"`
}

func (dbx *apiImpl) DevicesListMemberDevices(arg *ListMemberDevicesArg) (res *ListMemberDevicesResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "devices/list_member_devices",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DevicesListMemberDevicesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DevicesListMembersDevicesAPIError is an error-wrapper for the devices/list_members_devices route
type DevicesListMembersDevicesAPIError struct {
	dropbox.APIError
	EndpointError *ListMembersDevicesError `json:"error"`
}

func (dbx *apiImpl) DevicesListMembersDevices(arg *ListMembersDevicesArg) (res *ListMembersDevicesResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "devices/list_members_devices",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DevicesListMembersDevicesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DevicesListTeamDevicesAPIError is an error-wrapper for the devices/list_team_devices route
type DevicesListTeamDevicesAPIError struct {
	dropbox.APIError
	EndpointError *ListTeamDevicesError `json:"error"`
}

func (dbx *apiImpl) DevicesListTeamDevices(arg *ListTeamDevicesArg) (res *ListTeamDevicesResult, err error) {
	log.Printf("WARNING: API `DevicesListTeamDevices` is deprecated")
	log.Printf("Use API `DevicesListMembersDevices` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "devices/list_team_devices",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DevicesListTeamDevicesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DevicesRevokeDeviceSessionAPIError is an error-wrapper for the devices/revoke_device_session route
type DevicesRevokeDeviceSessionAPIError struct {
	dropbox.APIError
	EndpointError *RevokeDeviceSessionError `json:"error"`
}

func (dbx *apiImpl) DevicesRevokeDeviceSession(arg *RevokeDeviceSessionArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "devices/revoke_device_session",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DevicesRevokeDeviceSessionAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//DevicesRevokeDeviceSessionBatchAPIError is an error-wrapper for the devices/revoke_device_session_batch route
type DevicesRevokeDeviceSessionBatchAPIError struct {
	dropbox.APIError
	EndpointError *RevokeDeviceSessionBatchError `json:"error"`
}

func (dbx *apiImpl) DevicesRevokeDeviceSessionBatch(arg *RevokeDeviceSessionBatchArg) (res *RevokeDeviceSessionBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "devices/revoke_device_session_batch",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DevicesRevokeDeviceSessionBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//FeaturesGetValuesAPIError is an error-wrapper for the features/get_values route
type FeaturesGetValuesAPIError struct {
	dropbox.APIError
	EndpointError *FeaturesGetValuesBatchError `json:"error"`
}

func (dbx *apiImpl) FeaturesGetValues(arg *FeaturesGetValuesBatchArg) (res *FeaturesGetValuesBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "features/get_values",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr FeaturesGetValuesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GetInfoAPIError is an error-wrapper for the get_info route
type GetInfoAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetInfo() (res *TeamGetInfoResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "get_info",
		Auth:         "team",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetInfoAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsCreateAPIError is an error-wrapper for the groups/create route
type GroupsCreateAPIError struct {
	dropbox.APIError
	EndpointError *GroupCreateError `json:"error"`
}

func (dbx *apiImpl) GroupsCreate(arg *GroupCreateArg) (res *GroupFullInfo, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/create",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsCreateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsDeleteAPIError is an error-wrapper for the groups/delete route
type GroupsDeleteAPIError struct {
	dropbox.APIError
	EndpointError *GroupDeleteError `json:"error"`
}

func (dbx *apiImpl) GroupsDelete(arg *GroupSelector) (res *async.LaunchEmptyResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/delete",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsDeleteAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsGetInfoAPIError is an error-wrapper for the groups/get_info route
type GroupsGetInfoAPIError struct {
	dropbox.APIError
	EndpointError *GroupsGetInfoError `json:"error"`
}

func (dbx *apiImpl) GroupsGetInfo(arg *GroupsSelector) (res []*GroupsGetInfoItem, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/get_info",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsGetInfoAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsJobStatusGetAPIError is an error-wrapper for the groups/job_status/get route
type GroupsJobStatusGetAPIError struct {
	dropbox.APIError
	EndpointError *GroupsPollError `json:"error"`
}

func (dbx *apiImpl) GroupsJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/job_status/get",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsJobStatusGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsListAPIError is an error-wrapper for the groups/list route
type GroupsListAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GroupsList(arg *GroupsListArg) (res *GroupsListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsListContinueAPIError is an error-wrapper for the groups/list/continue route
type GroupsListContinueAPIError struct {
	dropbox.APIError
	EndpointError *GroupsListContinueError `json:"error"`
}

func (dbx *apiImpl) GroupsListContinue(arg *GroupsListContinueArg) (res *GroupsListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/list/continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsListContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsMembersAddAPIError is an error-wrapper for the groups/members/add route
type GroupsMembersAddAPIError struct {
	dropbox.APIError
	EndpointError *GroupMembersAddError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersAdd(arg *GroupMembersAddArg) (res *GroupMembersChangeResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/members/add",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsMembersAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsMembersListAPIError is an error-wrapper for the groups/members/list route
type GroupsMembersListAPIError struct {
	dropbox.APIError
	EndpointError *GroupSelectorError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersList(arg *GroupsMembersListArg) (res *GroupsMembersListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/members/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsMembersListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsMembersListContinueAPIError is an error-wrapper for the groups/members/list/continue route
type GroupsMembersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *GroupsMembersListContinueError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersListContinue(arg *GroupsMembersListContinueArg) (res *GroupsMembersListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/members/list/continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsMembersListContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsMembersRemoveAPIError is an error-wrapper for the groups/members/remove route
type GroupsMembersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *GroupMembersRemoveError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersRemove(arg *GroupMembersRemoveArg) (res *GroupMembersChangeResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/members/remove",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsMembersRemoveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsMembersSetAccessTypeAPIError is an error-wrapper for the groups/members/set_access_type route
type GroupsMembersSetAccessTypeAPIError struct {
	dropbox.APIError
	EndpointError *GroupMemberSetAccessTypeError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersSetAccessType(arg *GroupMembersSetAccessTypeArg) (res []*GroupsGetInfoItem, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/members/set_access_type",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsMembersSetAccessTypeAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GroupsUpdateAPIError is an error-wrapper for the groups/update route
type GroupsUpdateAPIError struct {
	dropbox.APIError
	EndpointError *GroupUpdateError `json:"error"`
}

func (dbx *apiImpl) GroupsUpdate(arg *GroupUpdateArgs) (res *GroupFullInfo, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "groups/update",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GroupsUpdateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LegalHoldsCreatePolicyAPIError is an error-wrapper for the legal_holds/create_policy route
type LegalHoldsCreatePolicyAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsPolicyCreateError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsCreatePolicy(arg *LegalHoldsPolicyCreateArg) (res *LegalHoldPolicy, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/create_policy",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsCreatePolicyAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LegalHoldsGetPolicyAPIError is an error-wrapper for the legal_holds/get_policy route
type LegalHoldsGetPolicyAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsGetPolicyError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsGetPolicy(arg *LegalHoldsGetPolicyArg) (res *LegalHoldPolicy, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/get_policy",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsGetPolicyAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LegalHoldsListHeldRevisionsAPIError is an error-wrapper for the legal_holds/list_held_revisions route
type LegalHoldsListHeldRevisionsAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsListHeldRevisionsError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsListHeldRevisions(arg *LegalHoldsListHeldRevisionsArg) (res *LegalHoldsListHeldRevisionResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/list_held_revisions",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsListHeldRevisionsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LegalHoldsListHeldRevisionsContinueAPIError is an error-wrapper for the legal_holds/list_held_revisions_continue route
type LegalHoldsListHeldRevisionsContinueAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsListHeldRevisionsError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsListHeldRevisionsContinue(arg *LegalHoldsListHeldRevisionsContinueArg) (res *LegalHoldsListHeldRevisionResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/list_held_revisions_continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsListHeldRevisionsContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LegalHoldsListPoliciesAPIError is an error-wrapper for the legal_holds/list_policies route
type LegalHoldsListPoliciesAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsListPoliciesError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsListPolicies(arg *LegalHoldsListPoliciesArg) (res *LegalHoldsListPoliciesResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/list_policies",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsListPoliciesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LegalHoldsReleasePolicyAPIError is an error-wrapper for the legal_holds/release_policy route
type LegalHoldsReleasePolicyAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsPolicyReleaseError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsReleasePolicy(arg *LegalHoldsPolicyReleaseArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/release_policy",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsReleasePolicyAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//LegalHoldsUpdatePolicyAPIError is an error-wrapper for the legal_holds/update_policy route
type LegalHoldsUpdatePolicyAPIError struct {
	dropbox.APIError
	EndpointError *LegalHoldsPolicyUpdateError `json:"error"`
}

func (dbx *apiImpl) LegalHoldsUpdatePolicy(arg *LegalHoldsPolicyUpdateArg) (res *LegalHoldPolicy, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "legal_holds/update_policy",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LegalHoldsUpdatePolicyAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LinkedAppsListMemberLinkedAppsAPIError is an error-wrapper for the linked_apps/list_member_linked_apps route
type LinkedAppsListMemberLinkedAppsAPIError struct {
	dropbox.APIError
	EndpointError *ListMemberAppsError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsListMemberLinkedApps(arg *ListMemberAppsArg) (res *ListMemberAppsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "linked_apps/list_member_linked_apps",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LinkedAppsListMemberLinkedAppsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LinkedAppsListMembersLinkedAppsAPIError is an error-wrapper for the linked_apps/list_members_linked_apps route
type LinkedAppsListMembersLinkedAppsAPIError struct {
	dropbox.APIError
	EndpointError *ListMembersAppsError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsListMembersLinkedApps(arg *ListMembersAppsArg) (res *ListMembersAppsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "linked_apps/list_members_linked_apps",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LinkedAppsListMembersLinkedAppsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LinkedAppsListTeamLinkedAppsAPIError is an error-wrapper for the linked_apps/list_team_linked_apps route
type LinkedAppsListTeamLinkedAppsAPIError struct {
	dropbox.APIError
	EndpointError *ListTeamAppsError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsListTeamLinkedApps(arg *ListTeamAppsArg) (res *ListTeamAppsResult, err error) {
	log.Printf("WARNING: API `LinkedAppsListTeamLinkedApps` is deprecated")
	log.Printf("Use API `LinkedAppsListMembersLinkedApps` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "linked_apps/list_team_linked_apps",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LinkedAppsListTeamLinkedAppsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LinkedAppsRevokeLinkedAppAPIError is an error-wrapper for the linked_apps/revoke_linked_app route
type LinkedAppsRevokeLinkedAppAPIError struct {
	dropbox.APIError
	EndpointError *RevokeLinkedAppError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsRevokeLinkedApp(arg *RevokeLinkedApiAppArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "linked_apps/revoke_linked_app",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LinkedAppsRevokeLinkedAppAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//LinkedAppsRevokeLinkedAppBatchAPIError is an error-wrapper for the linked_apps/revoke_linked_app_batch route
type LinkedAppsRevokeLinkedAppBatchAPIError struct {
	dropbox.APIError
	EndpointError *RevokeLinkedAppBatchError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsRevokeLinkedAppBatch(arg *RevokeLinkedApiAppBatchArg) (res *RevokeLinkedAppBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "linked_apps/revoke_linked_app_batch",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LinkedAppsRevokeLinkedAppBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsExcludedUsersAddAPIError is an error-wrapper for the member_space_limits/excluded_users/add route
type MemberSpaceLimitsExcludedUsersAddAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersUpdateError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersAdd(arg *ExcludedUsersUpdateArg) (res *ExcludedUsersUpdateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/excluded_users/add",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsExcludedUsersAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsExcludedUsersListAPIError is an error-wrapper for the member_space_limits/excluded_users/list route
type MemberSpaceLimitsExcludedUsersListAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersListError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersList(arg *ExcludedUsersListArg) (res *ExcludedUsersListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/excluded_users/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsExcludedUsersListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsExcludedUsersListContinueAPIError is an error-wrapper for the member_space_limits/excluded_users/list/continue route
type MemberSpaceLimitsExcludedUsersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersListContinueError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersListContinue(arg *ExcludedUsersListContinueArg) (res *ExcludedUsersListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/excluded_users/list/continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsExcludedUsersListContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsExcludedUsersRemoveAPIError is an error-wrapper for the member_space_limits/excluded_users/remove route
type MemberSpaceLimitsExcludedUsersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersUpdateError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersRemove(arg *ExcludedUsersUpdateArg) (res *ExcludedUsersUpdateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/excluded_users/remove",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsExcludedUsersRemoveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsGetCustomQuotaAPIError is an error-wrapper for the member_space_limits/get_custom_quota route
type MemberSpaceLimitsGetCustomQuotaAPIError struct {
	dropbox.APIError
	EndpointError *CustomQuotaError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsGetCustomQuota(arg *CustomQuotaUsersArg) (res []*CustomQuotaResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/get_custom_quota",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsGetCustomQuotaAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsRemoveCustomQuotaAPIError is an error-wrapper for the member_space_limits/remove_custom_quota route
type MemberSpaceLimitsRemoveCustomQuotaAPIError struct {
	dropbox.APIError
	EndpointError *CustomQuotaError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsRemoveCustomQuota(arg *CustomQuotaUsersArg) (res []*RemoveCustomQuotaResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/remove_custom_quota",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsRemoveCustomQuotaAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MemberSpaceLimitsSetCustomQuotaAPIError is an error-wrapper for the member_space_limits/set_custom_quota route
type MemberSpaceLimitsSetCustomQuotaAPIError struct {
	dropbox.APIError
	EndpointError *SetCustomQuotaError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsSetCustomQuota(arg *SetCustomQuotaArg) (res []*CustomQuotaResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "member_space_limits/set_custom_quota",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MemberSpaceLimitsSetCustomQuotaAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersAddV2APIError is an error-wrapper for the members/add_v2 route
type MembersAddV2APIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MembersAddV2(arg *MembersAddV2Arg) (res *MembersAddLaunchV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/add_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersAddV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersAddAPIError is an error-wrapper for the members/add route
type MembersAddAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MembersAdd(arg *MembersAddArg) (res *MembersAddLaunch, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/add",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersAddJobStatusGetV2APIError is an error-wrapper for the members/add/job_status/get_v2 route
type MembersAddJobStatusGetV2APIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersAddJobStatusGetV2(arg *async.PollArg) (res *MembersAddJobStatusV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/add/job_status/get_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersAddJobStatusGetV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersAddJobStatusGetAPIError is an error-wrapper for the members/add/job_status/get route
type MembersAddJobStatusGetAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersAddJobStatusGet(arg *async.PollArg) (res *MembersAddJobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/add/job_status/get",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersAddJobStatusGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersDeleteProfilePhotoV2APIError is an error-wrapper for the members/delete_profile_photo_v2 route
type MembersDeleteProfilePhotoV2APIError struct {
	dropbox.APIError
	EndpointError *MembersDeleteProfilePhotoError `json:"error"`
}

func (dbx *apiImpl) MembersDeleteProfilePhotoV2(arg *MembersDeleteProfilePhotoArg) (res *TeamMemberInfoV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/delete_profile_photo_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersDeleteProfilePhotoV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersDeleteProfilePhotoAPIError is an error-wrapper for the members/delete_profile_photo route
type MembersDeleteProfilePhotoAPIError struct {
	dropbox.APIError
	EndpointError *MembersDeleteProfilePhotoError `json:"error"`
}

func (dbx *apiImpl) MembersDeleteProfilePhoto(arg *MembersDeleteProfilePhotoArg) (res *TeamMemberInfo, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/delete_profile_photo",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersDeleteProfilePhotoAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersGetAvailableTeamMemberRolesAPIError is an error-wrapper for the members/get_available_team_member_roles route
type MembersGetAvailableTeamMemberRolesAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MembersGetAvailableTeamMemberRoles() (res *MembersGetAvailableTeamMemberRolesResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/get_available_team_member_roles",
		Auth:         "team",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersGetAvailableTeamMemberRolesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersGetInfoV2APIError is an error-wrapper for the members/get_info_v2 route
type MembersGetInfoV2APIError struct {
	dropbox.APIError
	EndpointError *MembersGetInfoError `json:"error"`
}

func (dbx *apiImpl) MembersGetInfoV2(arg *MembersGetInfoV2Arg) (res *MembersGetInfoV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/get_info_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersGetInfoV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersGetInfoAPIError is an error-wrapper for the members/get_info route
type MembersGetInfoAPIError struct {
	dropbox.APIError
	EndpointError *MembersGetInfoError `json:"error"`
}

func (dbx *apiImpl) MembersGetInfo(arg *MembersGetInfoArgs) (res []*MembersGetInfoItem, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/get_info",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersGetInfoAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersListV2APIError is an error-wrapper for the members/list_v2 route
type MembersListV2APIError struct {
	dropbox.APIError
	EndpointError *MembersListError `json:"error"`
}

func (dbx *apiImpl) MembersListV2(arg *MembersListArg) (res *MembersListV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/list_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersListV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersListAPIError is an error-wrapper for the members/list route
type MembersListAPIError struct {
	dropbox.APIError
	EndpointError *MembersListError `json:"error"`
}

func (dbx *apiImpl) MembersList(arg *MembersListArg) (res *MembersListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersListContinueV2APIError is an error-wrapper for the members/list/continue_v2 route
type MembersListContinueV2APIError struct {
	dropbox.APIError
	EndpointError *MembersListContinueError `json:"error"`
}

func (dbx *apiImpl) MembersListContinueV2(arg *MembersListContinueArg) (res *MembersListV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/list/continue_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersListContinueV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersListContinueAPIError is an error-wrapper for the members/list/continue route
type MembersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *MembersListContinueError `json:"error"`
}

func (dbx *apiImpl) MembersListContinue(arg *MembersListContinueArg) (res *MembersListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/list/continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersListContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersMoveFormerMemberFilesAPIError is an error-wrapper for the members/move_former_member_files route
type MembersMoveFormerMemberFilesAPIError struct {
	dropbox.APIError
	EndpointError *MembersTransferFormerMembersFilesError `json:"error"`
}

func (dbx *apiImpl) MembersMoveFormerMemberFiles(arg *MembersDataTransferArg) (res *async.LaunchEmptyResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/move_former_member_files",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersMoveFormerMemberFilesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersMoveFormerMemberFilesJobStatusCheckAPIError is an error-wrapper for the members/move_former_member_files/job_status/check route
type MembersMoveFormerMemberFilesJobStatusCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersMoveFormerMemberFilesJobStatusCheck(arg *async.PollArg) (res *async.PollEmptyResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/move_former_member_files/job_status/check",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersMoveFormerMemberFilesJobStatusCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersRecoverAPIError is an error-wrapper for the members/recover route
type MembersRecoverAPIError struct {
	dropbox.APIError
	EndpointError *MembersRecoverError `json:"error"`
}

func (dbx *apiImpl) MembersRecover(arg *MembersRecoverArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/recover",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersRecoverAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//MembersRemoveAPIError is an error-wrapper for the members/remove route
type MembersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *MembersRemoveError `json:"error"`
}

func (dbx *apiImpl) MembersRemove(arg *MembersRemoveArg) (res *async.LaunchEmptyResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/remove",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersRemoveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersRemoveJobStatusGetAPIError is an error-wrapper for the members/remove/job_status/get route
type MembersRemoveJobStatusGetAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersRemoveJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/remove/job_status/get",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersRemoveJobStatusGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSecondaryEmailsAddAPIError is an error-wrapper for the members/secondary_emails/add route
type MembersSecondaryEmailsAddAPIError struct {
	dropbox.APIError
	EndpointError *AddSecondaryEmailsError `json:"error"`
}

func (dbx *apiImpl) MembersSecondaryEmailsAdd(arg *AddSecondaryEmailsArg) (res *AddSecondaryEmailsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/secondary_emails/add",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSecondaryEmailsAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSecondaryEmailsDeleteAPIError is an error-wrapper for the members/secondary_emails/delete route
type MembersSecondaryEmailsDeleteAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MembersSecondaryEmailsDelete(arg *DeleteSecondaryEmailsArg) (res *DeleteSecondaryEmailsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/secondary_emails/delete",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSecondaryEmailsDeleteAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSecondaryEmailsResendVerificationEmailsAPIError is an error-wrapper for the members/secondary_emails/resend_verification_emails route
type MembersSecondaryEmailsResendVerificationEmailsAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MembersSecondaryEmailsResendVerificationEmails(arg *ResendVerificationEmailArg) (res *ResendVerificationEmailResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/secondary_emails/resend_verification_emails",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSecondaryEmailsResendVerificationEmailsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSendWelcomeEmailAPIError is an error-wrapper for the members/send_welcome_email route
type MembersSendWelcomeEmailAPIError struct {
	dropbox.APIError
	EndpointError *MembersSendWelcomeError `json:"error"`
}

func (dbx *apiImpl) MembersSendWelcomeEmail(arg *UserSelectorArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/send_welcome_email",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSendWelcomeEmailAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//MembersSetAdminPermissionsV2APIError is an error-wrapper for the members/set_admin_permissions_v2 route
type MembersSetAdminPermissionsV2APIError struct {
	dropbox.APIError
	EndpointError *MembersSetPermissions2Error `json:"error"`
}

func (dbx *apiImpl) MembersSetAdminPermissionsV2(arg *MembersSetPermissions2Arg) (res *MembersSetPermissions2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/set_admin_permissions_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSetAdminPermissionsV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSetAdminPermissionsAPIError is an error-wrapper for the members/set_admin_permissions route
type MembersSetAdminPermissionsAPIError struct {
	dropbox.APIError
	EndpointError *MembersSetPermissionsError `json:"error"`
}

func (dbx *apiImpl) MembersSetAdminPermissions(arg *MembersSetPermissionsArg) (res *MembersSetPermissionsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/set_admin_permissions",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSetAdminPermissionsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSetProfileV2APIError is an error-wrapper for the members/set_profile_v2 route
type MembersSetProfileV2APIError struct {
	dropbox.APIError
	EndpointError *MembersSetProfileError `json:"error"`
}

func (dbx *apiImpl) MembersSetProfileV2(arg *MembersSetProfileArg) (res *TeamMemberInfoV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/set_profile_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSetProfileV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSetProfileAPIError is an error-wrapper for the members/set_profile route
type MembersSetProfileAPIError struct {
	dropbox.APIError
	EndpointError *MembersSetProfileError `json:"error"`
}

func (dbx *apiImpl) MembersSetProfile(arg *MembersSetProfileArg) (res *TeamMemberInfo, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/set_profile",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSetProfileAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSetProfilePhotoV2APIError is an error-wrapper for the members/set_profile_photo_v2 route
type MembersSetProfilePhotoV2APIError struct {
	dropbox.APIError
	EndpointError *MembersSetProfilePhotoError `json:"error"`
}

func (dbx *apiImpl) MembersSetProfilePhotoV2(arg *MembersSetProfilePhotoArg) (res *TeamMemberInfoV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/set_profile_photo_v2",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSetProfilePhotoV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSetProfilePhotoAPIError is an error-wrapper for the members/set_profile_photo route
type MembersSetProfilePhotoAPIError struct {
	dropbox.APIError
	EndpointError *MembersSetProfilePhotoError `json:"error"`
}

func (dbx *apiImpl) MembersSetProfilePhoto(arg *MembersSetProfilePhotoArg) (res *TeamMemberInfo, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/set_profile_photo",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSetProfilePhotoAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MembersSuspendAPIError is an error-wrapper for the members/suspend route
type MembersSuspendAPIError struct {
	dropbox.APIError
	EndpointError *MembersSuspendError `json:"error"`
}

func (dbx *apiImpl) MembersSuspend(arg *MembersDeactivateArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/suspend",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersSuspendAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//MembersUnsuspendAPIError is an error-wrapper for the members/unsuspend route
type MembersUnsuspendAPIError struct {
	dropbox.APIError
	EndpointError *MembersUnsuspendError `json:"error"`
}

func (dbx *apiImpl) MembersUnsuspend(arg *MembersUnsuspendArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "members/unsuspend",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MembersUnsuspendAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//NamespacesListAPIError is an error-wrapper for the namespaces/list route
type NamespacesListAPIError struct {
	dropbox.APIError
	EndpointError *TeamNamespacesListError `json:"error"`
}

func (dbx *apiImpl) NamespacesList(arg *TeamNamespacesListArg) (res *TeamNamespacesListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "namespaces/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr NamespacesListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//NamespacesListContinueAPIError is an error-wrapper for the namespaces/list/continue route
type NamespacesListContinueAPIError struct {
	dropbox.APIError
	EndpointError *TeamNamespacesListContinueError `json:"error"`
}

func (dbx *apiImpl) NamespacesListContinue(arg *TeamNamespacesListContinueArg) (res *TeamNamespacesListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "namespaces/list/continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr NamespacesListContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PropertiesTemplateAddAPIError is an error-wrapper for the properties/template/add route
type PropertiesTemplateAddAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateAdd(arg *file_properties.AddTemplateArg) (res *file_properties.AddTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateAdd` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "properties/template/add",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesTemplateAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PropertiesTemplateGetAPIError is an error-wrapper for the properties/template/get route
type PropertiesTemplateGetAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.TemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateGet(arg *file_properties.GetTemplateArg) (res *file_properties.GetTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateGet` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "properties/template/get",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesTemplateGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PropertiesTemplateListAPIError is an error-wrapper for the properties/template/list route
type PropertiesTemplateListAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.TemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateList() (res *file_properties.ListTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateList` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "properties/template/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesTemplateListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PropertiesTemplateUpdateAPIError is an error-wrapper for the properties/template/update route
type PropertiesTemplateUpdateAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateUpdate(arg *file_properties.UpdateTemplateArg) (res *file_properties.UpdateTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateUpdate` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "properties/template/update",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesTemplateUpdateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ReportsGetActivityAPIError is an error-wrapper for the reports/get_activity route
type ReportsGetActivityAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetActivity(arg *DateRange) (res *GetActivityReport, err error) {
	log.Printf("WARNING: API `ReportsGetActivity` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "reports/get_activity",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ReportsGetActivityAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ReportsGetDevicesAPIError is an error-wrapper for the reports/get_devices route
type ReportsGetDevicesAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetDevices(arg *DateRange) (res *GetDevicesReport, err error) {
	log.Printf("WARNING: API `ReportsGetDevices` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "reports/get_devices",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ReportsGetDevicesAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ReportsGetMembershipAPIError is an error-wrapper for the reports/get_membership route
type ReportsGetMembershipAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetMembership(arg *DateRange) (res *GetMembershipReport, err error) {
	log.Printf("WARNING: API `ReportsGetMembership` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "reports/get_membership",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ReportsGetMembershipAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ReportsGetStorageAPIError is an error-wrapper for the reports/get_storage route
type ReportsGetStorageAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetStorage(arg *DateRange) (res *GetStorageReport, err error) {
	log.Printf("WARNING: API `ReportsGetStorage` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "reports/get_storage",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ReportsGetStorageAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderActivateAPIError is an error-wrapper for the team_folder/activate route
type TeamFolderActivateAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderActivateError `json:"error"`
}

func (dbx *apiImpl) TeamFolderActivate(arg *TeamFolderIdArg) (res *TeamFolderMetadata, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/activate",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderActivateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderArchiveAPIError is an error-wrapper for the team_folder/archive route
type TeamFolderArchiveAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderArchiveError `json:"error"`
}

func (dbx *apiImpl) TeamFolderArchive(arg *TeamFolderArchiveArg) (res *TeamFolderArchiveLaunch, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/archive",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderArchiveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderArchiveCheckAPIError is an error-wrapper for the team_folder/archive/check route
type TeamFolderArchiveCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) TeamFolderArchiveCheck(arg *async.PollArg) (res *TeamFolderArchiveJobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/archive/check",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderArchiveCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderCreateAPIError is an error-wrapper for the team_folder/create route
type TeamFolderCreateAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderCreateError `json:"error"`
}

func (dbx *apiImpl) TeamFolderCreate(arg *TeamFolderCreateArg) (res *TeamFolderMetadata, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/create",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderCreateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderGetInfoAPIError is an error-wrapper for the team_folder/get_info route
type TeamFolderGetInfoAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) TeamFolderGetInfo(arg *TeamFolderIdListArg) (res []*TeamFolderGetInfoItem, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/get_info",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderGetInfoAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderListAPIError is an error-wrapper for the team_folder/list route
type TeamFolderListAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderListError `json:"error"`
}

func (dbx *apiImpl) TeamFolderList(arg *TeamFolderListArg) (res *TeamFolderListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/list",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderListContinueAPIError is an error-wrapper for the team_folder/list/continue route
type TeamFolderListContinueAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderListContinueError `json:"error"`
}

func (dbx *apiImpl) TeamFolderListContinue(arg *TeamFolderListContinueArg) (res *TeamFolderListResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/list/continue",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderListContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderPermanentlyDeleteAPIError is an error-wrapper for the team_folder/permanently_delete route
type TeamFolderPermanentlyDeleteAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderPermanentlyDeleteError `json:"error"`
}

func (dbx *apiImpl) TeamFolderPermanentlyDelete(arg *TeamFolderIdArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/permanently_delete",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderPermanentlyDeleteAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//TeamFolderRenameAPIError is an error-wrapper for the team_folder/rename route
type TeamFolderRenameAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderRenameError `json:"error"`
}

func (dbx *apiImpl) TeamFolderRename(arg *TeamFolderRenameArg) (res *TeamFolderMetadata, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/rename",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderRenameAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TeamFolderUpdateSyncSettingsAPIError is an error-wrapper for the team_folder/update_sync_settings route
type TeamFolderUpdateSyncSettingsAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderUpdateSyncSettingsError `json:"error"`
}

func (dbx *apiImpl) TeamFolderUpdateSyncSettings(arg *TeamFolderUpdateSyncSettingsArg) (res *TeamFolderMetadata, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "team_folder/update_sync_settings",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TeamFolderUpdateSyncSettingsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TokenGetAuthenticatedAdminAPIError is an error-wrapper for the token/get_authenticated_admin route
type TokenGetAuthenticatedAdminAPIError struct {
	dropbox.APIError
	EndpointError *TokenGetAuthenticatedAdminError `json:"error"`
}

func (dbx *apiImpl) TokenGetAuthenticatedAdmin() (res *TokenGetAuthenticatedAdminResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "team",
		Route:        "token/get_authenticated_admin",
		Auth:         "team",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TokenGetAuthenticatedAdminAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

// New returns a Client implementation for this namespace
func New(c dropbox.Config) Client {
	ctx := apiImpl(dropbox.NewContext(c))
	return &ctx
}
