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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/async"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/auth"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/file_properties"
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
	MembersAdd(arg *MembersAddArg) (res *MembersAddLaunch, err error)
	// MembersAddJobStatusGet : Once an async_job_id is returned from
	// `membersAdd` , use this to poll the status of the asynchronous request.
	// Permission : Team member management.
	MembersAddJobStatusGet(arg *async.PollArg) (res *MembersAddJobStatus, err error)
	// MembersGetInfo : Returns information about multiple team members.
	// Permission : Team information This endpoint will return
	// `MembersGetInfoItem.id_not_found`, for IDs (or emails) that cannot be
	// matched to a valid team member.
	MembersGetInfo(arg *MembersGetInfoArgs) (res []*MembersGetInfoItem, err error)
	// MembersList : Lists members of a team. Permission : Team information.
	MembersList(arg *MembersListArg) (res *MembersListResult, err error)
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
	// time, based on the version history length associated with the team (120
	// days for most teams). This endpoint may initiate an asynchronous job. To
	// obtain the final result of the job, the client should periodically poll
	// `membersRemoveJobStatusGet`.
	MembersRemove(arg *MembersRemoveArg) (res *async.LaunchEmptyResult, err error)
	// MembersRemoveJobStatusGet : Once an async_job_id is returned from
	// `membersRemove` , use this to poll the status of the asynchronous
	// request. Permission : Team member management.
	MembersRemoveJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error)
	// MembersSendWelcomeEmail : Sends welcome email to pending team member.
	// Permission : Team member management Exactly one of team_member_id, email,
	// or external_id must be provided to identify the user account. No-op if
	// team member is not pending.
	MembersSendWelcomeEmail(arg *UserSelectorArg) (err error)
	// MembersSetAdminPermissions : Updates a team member's permissions.
	// Permission : Team member management.
	MembersSetAdminPermissions(arg *MembersSetPermissionsArg) (res *MembersSetPermissionsResult, err error)
	// MembersSetProfile : Updates a team member's profile. Permission : Team
	// member management.
	MembersSetProfile(arg *MembersSetProfileArg) (res *TeamMemberInfo, err error)
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
	// PropertiesTemplateGet : Permission : Team member file access.
	// Deprecated:
	PropertiesTemplateGet(arg *file_properties.GetTemplateArg) (res *file_properties.GetTemplateResult, err error)
	// PropertiesTemplateList : Permission : Team member file access.
	// Deprecated:
	PropertiesTemplateList() (res *file_properties.ListTemplateResult, err error)
	// PropertiesTemplateUpdate : Permission : Team member file access.
	// Deprecated:
	PropertiesTemplateUpdate(arg *file_properties.UpdateTemplateArg) (res *file_properties.UpdateTemplateResult, err error)
	// ReportsGetActivity : Retrieves reporting data about a team's user
	// activity.
	ReportsGetActivity(arg *DateRange) (res *GetActivityReport, err error)
	// ReportsGetDevices : Retrieves reporting data about a team's linked
	// devices.
	ReportsGetDevices(arg *DateRange) (res *GetDevicesReport, err error)
	// ReportsGetMembership : Retrieves reporting data about a team's
	// membership.
	ReportsGetMembership(arg *DateRange) (res *GetMembershipReport, err error)
	// ReportsGetStorage : Retrieves reporting data about a team's storage
	// usage.
	ReportsGetStorage(arg *DateRange) (res *GetStorageReport, err error)
	// TeamFolderActivate : Sets an archived team folder's status to active.
	// Permission : Team member file access.
	TeamFolderActivate(arg *TeamFolderIdArg) (res *TeamFolderMetadata, err error)
	// TeamFolderArchive : Sets an active team folder's status to archived and
	// removes all folder and file members. Permission : Team member file
	// access.
	TeamFolderArchive(arg *TeamFolderArchiveArg) (res *TeamFolderArchiveLaunch, err error)
	// TeamFolderArchiveCheck : Returns the status of an asynchronous job for
	// archiving a team folder. Permission : Team member file access.
	TeamFolderArchiveCheck(arg *async.PollArg) (res *TeamFolderArchiveJobStatus, err error)
	// TeamFolderCreate : Creates a new, active, team folder with no members.
	// Permission : Team member file access.
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
	// folder. Permission : Team member file access.
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
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "devices/list_member_devices", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DevicesListMemberDevicesAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//DevicesListMembersDevicesAPIError is an error-wrapper for the devices/list_members_devices route
type DevicesListMembersDevicesAPIError struct {
	dropbox.APIError
	EndpointError *ListMembersDevicesError `json:"error"`
}

func (dbx *apiImpl) DevicesListMembersDevices(arg *ListMembersDevicesArg) (res *ListMembersDevicesResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "devices/list_members_devices", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DevicesListMembersDevicesAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
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

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "devices/list_team_devices", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DevicesListTeamDevicesAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//DevicesRevokeDeviceSessionAPIError is an error-wrapper for the devices/revoke_device_session route
type DevicesRevokeDeviceSessionAPIError struct {
	dropbox.APIError
	EndpointError *RevokeDeviceSessionError `json:"error"`
}

func (dbx *apiImpl) DevicesRevokeDeviceSession(arg *RevokeDeviceSessionArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "devices/revoke_device_session", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DevicesRevokeDeviceSessionAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//DevicesRevokeDeviceSessionBatchAPIError is an error-wrapper for the devices/revoke_device_session_batch route
type DevicesRevokeDeviceSessionBatchAPIError struct {
	dropbox.APIError
	EndpointError *RevokeDeviceSessionBatchError `json:"error"`
}

func (dbx *apiImpl) DevicesRevokeDeviceSessionBatch(arg *RevokeDeviceSessionBatchArg) (res *RevokeDeviceSessionBatchResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "devices/revoke_device_session_batch", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DevicesRevokeDeviceSessionBatchAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//FeaturesGetValuesAPIError is an error-wrapper for the features/get_values route
type FeaturesGetValuesAPIError struct {
	dropbox.APIError
	EndpointError *FeaturesGetValuesBatchError `json:"error"`
}

func (dbx *apiImpl) FeaturesGetValues(arg *FeaturesGetValuesBatchArg) (res *FeaturesGetValuesBatchResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "features/get_values", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError FeaturesGetValuesAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GetInfoAPIError is an error-wrapper for the get_info route
type GetInfoAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetInfo() (res *TeamGetInfoResult, err error) {
	cli := dbx.Client

	headers := map[string]string{}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "get_info", headers, nil)
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GetInfoAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsCreateAPIError is an error-wrapper for the groups/create route
type GroupsCreateAPIError struct {
	dropbox.APIError
	EndpointError *GroupCreateError `json:"error"`
}

func (dbx *apiImpl) GroupsCreate(arg *GroupCreateArg) (res *GroupFullInfo, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/create", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsCreateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsDeleteAPIError is an error-wrapper for the groups/delete route
type GroupsDeleteAPIError struct {
	dropbox.APIError
	EndpointError *GroupDeleteError `json:"error"`
}

func (dbx *apiImpl) GroupsDelete(arg *GroupSelector) (res *async.LaunchEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/delete", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsDeleteAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsGetInfoAPIError is an error-wrapper for the groups/get_info route
type GroupsGetInfoAPIError struct {
	dropbox.APIError
	EndpointError *GroupsGetInfoError `json:"error"`
}

func (dbx *apiImpl) GroupsGetInfo(arg *GroupsSelector) (res []*GroupsGetInfoItem, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/get_info", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsGetInfoAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsJobStatusGetAPIError is an error-wrapper for the groups/job_status/get route
type GroupsJobStatusGetAPIError struct {
	dropbox.APIError
	EndpointError *GroupsPollError `json:"error"`
}

func (dbx *apiImpl) GroupsJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/job_status/get", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsJobStatusGetAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsListAPIError is an error-wrapper for the groups/list route
type GroupsListAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GroupsList(arg *GroupsListArg) (res *GroupsListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/list", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsListContinueAPIError is an error-wrapper for the groups/list/continue route
type GroupsListContinueAPIError struct {
	dropbox.APIError
	EndpointError *GroupsListContinueError `json:"error"`
}

func (dbx *apiImpl) GroupsListContinue(arg *GroupsListContinueArg) (res *GroupsListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/list/continue", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsMembersAddAPIError is an error-wrapper for the groups/members/add route
type GroupsMembersAddAPIError struct {
	dropbox.APIError
	EndpointError *GroupMembersAddError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersAdd(arg *GroupMembersAddArg) (res *GroupMembersChangeResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/members/add", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsMembersAddAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsMembersListAPIError is an error-wrapper for the groups/members/list route
type GroupsMembersListAPIError struct {
	dropbox.APIError
	EndpointError *GroupSelectorError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersList(arg *GroupsMembersListArg) (res *GroupsMembersListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/members/list", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsMembersListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsMembersListContinueAPIError is an error-wrapper for the groups/members/list/continue route
type GroupsMembersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *GroupsMembersListContinueError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersListContinue(arg *GroupsMembersListContinueArg) (res *GroupsMembersListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/members/list/continue", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsMembersListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsMembersRemoveAPIError is an error-wrapper for the groups/members/remove route
type GroupsMembersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *GroupMembersRemoveError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersRemove(arg *GroupMembersRemoveArg) (res *GroupMembersChangeResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/members/remove", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsMembersRemoveAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsMembersSetAccessTypeAPIError is an error-wrapper for the groups/members/set_access_type route
type GroupsMembersSetAccessTypeAPIError struct {
	dropbox.APIError
	EndpointError *GroupMemberSetAccessTypeError `json:"error"`
}

func (dbx *apiImpl) GroupsMembersSetAccessType(arg *GroupMembersSetAccessTypeArg) (res []*GroupsGetInfoItem, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/members/set_access_type", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsMembersSetAccessTypeAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//GroupsUpdateAPIError is an error-wrapper for the groups/update route
type GroupsUpdateAPIError struct {
	dropbox.APIError
	EndpointError *GroupUpdateError `json:"error"`
}

func (dbx *apiImpl) GroupsUpdate(arg *GroupUpdateArgs) (res *GroupFullInfo, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "groups/update", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GroupsUpdateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//LinkedAppsListMemberLinkedAppsAPIError is an error-wrapper for the linked_apps/list_member_linked_apps route
type LinkedAppsListMemberLinkedAppsAPIError struct {
	dropbox.APIError
	EndpointError *ListMemberAppsError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsListMemberLinkedApps(arg *ListMemberAppsArg) (res *ListMemberAppsResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "linked_apps/list_member_linked_apps", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError LinkedAppsListMemberLinkedAppsAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//LinkedAppsListMembersLinkedAppsAPIError is an error-wrapper for the linked_apps/list_members_linked_apps route
type LinkedAppsListMembersLinkedAppsAPIError struct {
	dropbox.APIError
	EndpointError *ListMembersAppsError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsListMembersLinkedApps(arg *ListMembersAppsArg) (res *ListMembersAppsResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "linked_apps/list_members_linked_apps", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError LinkedAppsListMembersLinkedAppsAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
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

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "linked_apps/list_team_linked_apps", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError LinkedAppsListTeamLinkedAppsAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//LinkedAppsRevokeLinkedAppAPIError is an error-wrapper for the linked_apps/revoke_linked_app route
type LinkedAppsRevokeLinkedAppAPIError struct {
	dropbox.APIError
	EndpointError *RevokeLinkedAppError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsRevokeLinkedApp(arg *RevokeLinkedApiAppArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "linked_apps/revoke_linked_app", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError LinkedAppsRevokeLinkedAppAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//LinkedAppsRevokeLinkedAppBatchAPIError is an error-wrapper for the linked_apps/revoke_linked_app_batch route
type LinkedAppsRevokeLinkedAppBatchAPIError struct {
	dropbox.APIError
	EndpointError *RevokeLinkedAppBatchError `json:"error"`
}

func (dbx *apiImpl) LinkedAppsRevokeLinkedAppBatch(arg *RevokeLinkedApiAppBatchArg) (res *RevokeLinkedAppBatchResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "linked_apps/revoke_linked_app_batch", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError LinkedAppsRevokeLinkedAppBatchAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsExcludedUsersAddAPIError is an error-wrapper for the member_space_limits/excluded_users/add route
type MemberSpaceLimitsExcludedUsersAddAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersUpdateError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersAdd(arg *ExcludedUsersUpdateArg) (res *ExcludedUsersUpdateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/excluded_users/add", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsExcludedUsersAddAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsExcludedUsersListAPIError is an error-wrapper for the member_space_limits/excluded_users/list route
type MemberSpaceLimitsExcludedUsersListAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersListError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersList(arg *ExcludedUsersListArg) (res *ExcludedUsersListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/excluded_users/list", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsExcludedUsersListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsExcludedUsersListContinueAPIError is an error-wrapper for the member_space_limits/excluded_users/list/continue route
type MemberSpaceLimitsExcludedUsersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersListContinueError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersListContinue(arg *ExcludedUsersListContinueArg) (res *ExcludedUsersListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/excluded_users/list/continue", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsExcludedUsersListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsExcludedUsersRemoveAPIError is an error-wrapper for the member_space_limits/excluded_users/remove route
type MemberSpaceLimitsExcludedUsersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *ExcludedUsersUpdateError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsExcludedUsersRemove(arg *ExcludedUsersUpdateArg) (res *ExcludedUsersUpdateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/excluded_users/remove", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsExcludedUsersRemoveAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsGetCustomQuotaAPIError is an error-wrapper for the member_space_limits/get_custom_quota route
type MemberSpaceLimitsGetCustomQuotaAPIError struct {
	dropbox.APIError
	EndpointError *CustomQuotaError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsGetCustomQuota(arg *CustomQuotaUsersArg) (res []*CustomQuotaResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/get_custom_quota", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsGetCustomQuotaAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsRemoveCustomQuotaAPIError is an error-wrapper for the member_space_limits/remove_custom_quota route
type MemberSpaceLimitsRemoveCustomQuotaAPIError struct {
	dropbox.APIError
	EndpointError *CustomQuotaError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsRemoveCustomQuota(arg *CustomQuotaUsersArg) (res []*RemoveCustomQuotaResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/remove_custom_quota", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsRemoveCustomQuotaAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MemberSpaceLimitsSetCustomQuotaAPIError is an error-wrapper for the member_space_limits/set_custom_quota route
type MemberSpaceLimitsSetCustomQuotaAPIError struct {
	dropbox.APIError
	EndpointError *SetCustomQuotaError `json:"error"`
}

func (dbx *apiImpl) MemberSpaceLimitsSetCustomQuota(arg *SetCustomQuotaArg) (res []*CustomQuotaResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "member_space_limits/set_custom_quota", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MemberSpaceLimitsSetCustomQuotaAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersAddAPIError is an error-wrapper for the members/add route
type MembersAddAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MembersAdd(arg *MembersAddArg) (res *MembersAddLaunch, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/add", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersAddAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersAddJobStatusGetAPIError is an error-wrapper for the members/add/job_status/get route
type MembersAddJobStatusGetAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersAddJobStatusGet(arg *async.PollArg) (res *MembersAddJobStatus, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/add/job_status/get", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersAddJobStatusGetAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersGetInfoAPIError is an error-wrapper for the members/get_info route
type MembersGetInfoAPIError struct {
	dropbox.APIError
	EndpointError *MembersGetInfoError `json:"error"`
}

func (dbx *apiImpl) MembersGetInfo(arg *MembersGetInfoArgs) (res []*MembersGetInfoItem, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/get_info", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersGetInfoAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersListAPIError is an error-wrapper for the members/list route
type MembersListAPIError struct {
	dropbox.APIError
	EndpointError *MembersListError `json:"error"`
}

func (dbx *apiImpl) MembersList(arg *MembersListArg) (res *MembersListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/list", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersListContinueAPIError is an error-wrapper for the members/list/continue route
type MembersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *MembersListContinueError `json:"error"`
}

func (dbx *apiImpl) MembersListContinue(arg *MembersListContinueArg) (res *MembersListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/list/continue", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersMoveFormerMemberFilesAPIError is an error-wrapper for the members/move_former_member_files route
type MembersMoveFormerMemberFilesAPIError struct {
	dropbox.APIError
	EndpointError *MembersTransferFormerMembersFilesError `json:"error"`
}

func (dbx *apiImpl) MembersMoveFormerMemberFiles(arg *MembersDataTransferArg) (res *async.LaunchEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/move_former_member_files", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersMoveFormerMemberFilesAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersMoveFormerMemberFilesJobStatusCheckAPIError is an error-wrapper for the members/move_former_member_files/job_status/check route
type MembersMoveFormerMemberFilesJobStatusCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersMoveFormerMemberFilesJobStatusCheck(arg *async.PollArg) (res *async.PollEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/move_former_member_files/job_status/check", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersMoveFormerMemberFilesJobStatusCheckAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersRecoverAPIError is an error-wrapper for the members/recover route
type MembersRecoverAPIError struct {
	dropbox.APIError
	EndpointError *MembersRecoverError `json:"error"`
}

func (dbx *apiImpl) MembersRecover(arg *MembersRecoverArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/recover", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersRecoverAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersRemoveAPIError is an error-wrapper for the members/remove route
type MembersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *MembersRemoveError `json:"error"`
}

func (dbx *apiImpl) MembersRemove(arg *MembersRemoveArg) (res *async.LaunchEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/remove", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersRemoveAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersRemoveJobStatusGetAPIError is an error-wrapper for the members/remove/job_status/get route
type MembersRemoveJobStatusGetAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MembersRemoveJobStatusGet(arg *async.PollArg) (res *async.PollEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/remove/job_status/get", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersRemoveJobStatusGetAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersSendWelcomeEmailAPIError is an error-wrapper for the members/send_welcome_email route
type MembersSendWelcomeEmailAPIError struct {
	dropbox.APIError
	EndpointError *MembersSendWelcomeError `json:"error"`
}

func (dbx *apiImpl) MembersSendWelcomeEmail(arg *UserSelectorArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/send_welcome_email", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersSendWelcomeEmailAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersSetAdminPermissionsAPIError is an error-wrapper for the members/set_admin_permissions route
type MembersSetAdminPermissionsAPIError struct {
	dropbox.APIError
	EndpointError *MembersSetPermissionsError `json:"error"`
}

func (dbx *apiImpl) MembersSetAdminPermissions(arg *MembersSetPermissionsArg) (res *MembersSetPermissionsResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/set_admin_permissions", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersSetAdminPermissionsAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersSetProfileAPIError is an error-wrapper for the members/set_profile route
type MembersSetProfileAPIError struct {
	dropbox.APIError
	EndpointError *MembersSetProfileError `json:"error"`
}

func (dbx *apiImpl) MembersSetProfile(arg *MembersSetProfileArg) (res *TeamMemberInfo, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/set_profile", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersSetProfileAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersSuspendAPIError is an error-wrapper for the members/suspend route
type MembersSuspendAPIError struct {
	dropbox.APIError
	EndpointError *MembersSuspendError `json:"error"`
}

func (dbx *apiImpl) MembersSuspend(arg *MembersDeactivateArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/suspend", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersSuspendAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//MembersUnsuspendAPIError is an error-wrapper for the members/unsuspend route
type MembersUnsuspendAPIError struct {
	dropbox.APIError
	EndpointError *MembersUnsuspendError `json:"error"`
}

func (dbx *apiImpl) MembersUnsuspend(arg *MembersUnsuspendArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "members/unsuspend", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError MembersUnsuspendAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//NamespacesListAPIError is an error-wrapper for the namespaces/list route
type NamespacesListAPIError struct {
	dropbox.APIError
	EndpointError *TeamNamespacesListError `json:"error"`
}

func (dbx *apiImpl) NamespacesList(arg *TeamNamespacesListArg) (res *TeamNamespacesListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "namespaces/list", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError NamespacesListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//NamespacesListContinueAPIError is an error-wrapper for the namespaces/list/continue route
type NamespacesListContinueAPIError struct {
	dropbox.APIError
	EndpointError *TeamNamespacesListContinueError `json:"error"`
}

func (dbx *apiImpl) NamespacesListContinue(arg *TeamNamespacesListContinueArg) (res *TeamNamespacesListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "namespaces/list/continue", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError NamespacesListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//PropertiesTemplateAddAPIError is an error-wrapper for the properties/template/add route
type PropertiesTemplateAddAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateAdd(arg *file_properties.AddTemplateArg) (res *file_properties.AddTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateAdd` is deprecated")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "properties/template/add", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError PropertiesTemplateAddAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//PropertiesTemplateGetAPIError is an error-wrapper for the properties/template/get route
type PropertiesTemplateGetAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.TemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateGet(arg *file_properties.GetTemplateArg) (res *file_properties.GetTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateGet` is deprecated")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "properties/template/get", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError PropertiesTemplateGetAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//PropertiesTemplateListAPIError is an error-wrapper for the properties/template/list route
type PropertiesTemplateListAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.TemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateList() (res *file_properties.ListTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateList` is deprecated")

	cli := dbx.Client

	headers := map[string]string{}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "properties/template/list", headers, nil)
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError PropertiesTemplateListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//PropertiesTemplateUpdateAPIError is an error-wrapper for the properties/template/update route
type PropertiesTemplateUpdateAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateUpdate(arg *file_properties.UpdateTemplateArg) (res *file_properties.UpdateTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateUpdate` is deprecated")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "properties/template/update", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError PropertiesTemplateUpdateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//ReportsGetActivityAPIError is an error-wrapper for the reports/get_activity route
type ReportsGetActivityAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetActivity(arg *DateRange) (res *GetActivityReport, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "reports/get_activity", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError ReportsGetActivityAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//ReportsGetDevicesAPIError is an error-wrapper for the reports/get_devices route
type ReportsGetDevicesAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetDevices(arg *DateRange) (res *GetDevicesReport, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "reports/get_devices", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError ReportsGetDevicesAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//ReportsGetMembershipAPIError is an error-wrapper for the reports/get_membership route
type ReportsGetMembershipAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetMembership(arg *DateRange) (res *GetMembershipReport, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "reports/get_membership", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError ReportsGetMembershipAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//ReportsGetStorageAPIError is an error-wrapper for the reports/get_storage route
type ReportsGetStorageAPIError struct {
	dropbox.APIError
	EndpointError *DateRangeError `json:"error"`
}

func (dbx *apiImpl) ReportsGetStorage(arg *DateRange) (res *GetStorageReport, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "reports/get_storage", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError ReportsGetStorageAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderActivateAPIError is an error-wrapper for the team_folder/activate route
type TeamFolderActivateAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderActivateError `json:"error"`
}

func (dbx *apiImpl) TeamFolderActivate(arg *TeamFolderIdArg) (res *TeamFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/activate", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderActivateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderArchiveAPIError is an error-wrapper for the team_folder/archive route
type TeamFolderArchiveAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderArchiveError `json:"error"`
}

func (dbx *apiImpl) TeamFolderArchive(arg *TeamFolderArchiveArg) (res *TeamFolderArchiveLaunch, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/archive", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderArchiveAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderArchiveCheckAPIError is an error-wrapper for the team_folder/archive/check route
type TeamFolderArchiveCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) TeamFolderArchiveCheck(arg *async.PollArg) (res *TeamFolderArchiveJobStatus, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/archive/check", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderArchiveCheckAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderCreateAPIError is an error-wrapper for the team_folder/create route
type TeamFolderCreateAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderCreateError `json:"error"`
}

func (dbx *apiImpl) TeamFolderCreate(arg *TeamFolderCreateArg) (res *TeamFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/create", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderCreateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderGetInfoAPIError is an error-wrapper for the team_folder/get_info route
type TeamFolderGetInfoAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) TeamFolderGetInfo(arg *TeamFolderIdListArg) (res []*TeamFolderGetInfoItem, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/get_info", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderGetInfoAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderListAPIError is an error-wrapper for the team_folder/list route
type TeamFolderListAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderListError `json:"error"`
}

func (dbx *apiImpl) TeamFolderList(arg *TeamFolderListArg) (res *TeamFolderListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/list", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderListContinueAPIError is an error-wrapper for the team_folder/list/continue route
type TeamFolderListContinueAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderListContinueError `json:"error"`
}

func (dbx *apiImpl) TeamFolderListContinue(arg *TeamFolderListContinueArg) (res *TeamFolderListResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/list/continue", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderPermanentlyDeleteAPIError is an error-wrapper for the team_folder/permanently_delete route
type TeamFolderPermanentlyDeleteAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderPermanentlyDeleteError `json:"error"`
}

func (dbx *apiImpl) TeamFolderPermanentlyDelete(arg *TeamFolderIdArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/permanently_delete", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderPermanentlyDeleteAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderRenameAPIError is an error-wrapper for the team_folder/rename route
type TeamFolderRenameAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderRenameError `json:"error"`
}

func (dbx *apiImpl) TeamFolderRename(arg *TeamFolderRenameArg) (res *TeamFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/rename", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderRenameAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TeamFolderUpdateSyncSettingsAPIError is an error-wrapper for the team_folder/update_sync_settings route
type TeamFolderUpdateSyncSettingsAPIError struct {
	dropbox.APIError
	EndpointError *TeamFolderUpdateSyncSettingsError `json:"error"`
}

func (dbx *apiImpl) TeamFolderUpdateSyncSettings(arg *TeamFolderUpdateSyncSettingsArg) (res *TeamFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "team_folder/update_sync_settings", headers, bytes.NewReader(b))
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TeamFolderUpdateSyncSettingsAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TokenGetAuthenticatedAdminAPIError is an error-wrapper for the token/get_authenticated_admin route
type TokenGetAuthenticatedAdminAPIError struct {
	dropbox.APIError
	EndpointError *TokenGetAuthenticatedAdminError `json:"error"`
}

func (dbx *apiImpl) TokenGetAuthenticatedAdmin() (res *TokenGetAuthenticatedAdminResult, err error) {
	cli := dbx.Client

	headers := map[string]string{}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team", "token/get_authenticated_admin", headers, nil)
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError TokenGetAuthenticatedAdminAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = auth.HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

// New returns a Client implementation for this namespace
func New(c dropbox.Config) Client {
	ctx := apiImpl(dropbox.NewContext(c))
	return &ctx
}
