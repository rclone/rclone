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

package sharing

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/async"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/auth"
)

// Client interface describes all routes in this namespace
type Client interface {
	// AddFileMember : Adds specified members to a file.
	AddFileMember(arg *AddFileMemberArgs) (res []*FileMemberActionResult, err error)
	// AddFolderMember : Allows an owner or editor (if the ACL update policy
	// allows) of a shared folder to add another member. For the new member to
	// get access to all the functionality for this folder, you will need to
	// call `mountFolder` on their behalf.
	AddFolderMember(arg *AddFolderMemberArg) (err error)
	// ChangeFileMemberAccess : Identical to update_file_member but with less
	// information returned.
	// Deprecated: Use `UpdateFileMember` instead
	ChangeFileMemberAccess(arg *ChangeFileMemberAccessArgs) (res *FileMemberActionResult, err error)
	// CheckJobStatus : Returns the status of an asynchronous job.
	CheckJobStatus(arg *async.PollArg) (res *JobStatus, err error)
	// CheckRemoveMemberJobStatus : Returns the status of an asynchronous job
	// for sharing a folder.
	CheckRemoveMemberJobStatus(arg *async.PollArg) (res *RemoveMemberJobStatus, err error)
	// CheckShareJobStatus : Returns the status of an asynchronous job for
	// sharing a folder.
	CheckShareJobStatus(arg *async.PollArg) (res *ShareFolderJobStatus, err error)
	// CreateSharedLink : Create a shared link. If a shared link already exists
	// for the given path, that link is returned. Note that in the returned
	// `PathLinkMetadata`, the `PathLinkMetadata.url` field is the shortened URL
	// if `CreateSharedLinkArg.short_url` argument is set to true. Previously,
	// it was technically possible to break a shared link by moving or renaming
	// the corresponding file or folder. In the future, this will no longer be
	// the case, so your app shouldn't rely on this behavior. Instead, if your
	// app needs to revoke a shared link, use `revokeSharedLink`.
	// Deprecated: Use `CreateSharedLinkWithSettings` instead
	CreateSharedLink(arg *CreateSharedLinkArg) (res *PathLinkMetadata, err error)
	// CreateSharedLinkWithSettings : Create a shared link with custom settings.
	// If no settings are given then the default visibility is
	// `RequestedVisibility.public` (The resolved visibility, though, may depend
	// on other aspects such as team and shared folder settings).
	CreateSharedLinkWithSettings(arg *CreateSharedLinkWithSettingsArg) (res IsSharedLinkMetadata, err error)
	// GetFileMetadata : Returns shared file metadata.
	GetFileMetadata(arg *GetFileMetadataArg) (res *SharedFileMetadata, err error)
	// GetFileMetadataBatch : Returns shared file metadata.
	GetFileMetadataBatch(arg *GetFileMetadataBatchArg) (res []*GetFileMetadataBatchResult, err error)
	// GetFolderMetadata : Returns shared folder metadata by its folder ID.
	GetFolderMetadata(arg *GetMetadataArgs) (res *SharedFolderMetadata, err error)
	// GetSharedLinkFile : Download the shared link's file from a user's
	// Dropbox.
	GetSharedLinkFile(arg *GetSharedLinkMetadataArg) (res IsSharedLinkMetadata, content io.ReadCloser, err error)
	// GetSharedLinkMetadata : Get the shared link's metadata.
	GetSharedLinkMetadata(arg *GetSharedLinkMetadataArg) (res IsSharedLinkMetadata, err error)
	// GetSharedLinks : Returns a list of `LinkMetadata` objects for this user,
	// including collection links. If no path is given, returns a list of all
	// shared links for the current user, including collection links, up to a
	// maximum of 1000 links. If a non-empty path is given, returns a list of
	// all shared links that allow access to the given path.  Collection links
	// are never returned in this case. Note that the url field in the response
	// is never the shortened URL.
	// Deprecated: Use `ListSharedLinks` instead
	GetSharedLinks(arg *GetSharedLinksArg) (res *GetSharedLinksResult, err error)
	// ListFileMembers : Use to obtain the members who have been invited to a
	// file, both inherited and uninherited members.
	ListFileMembers(arg *ListFileMembersArg) (res *SharedFileMembers, err error)
	// ListFileMembersBatch : Get members of multiple files at once. The
	// arguments to this route are more limited, and the limit on query result
	// size per file is more strict. To customize the results more, use the
	// individual file endpoint. Inherited users and groups are not included in
	// the result, and permissions are not returned for this endpoint.
	ListFileMembersBatch(arg *ListFileMembersBatchArg) (res []*ListFileMembersBatchResult, err error)
	// ListFileMembersContinue : Once a cursor has been retrieved from
	// `listFileMembers` or `listFileMembersBatch`, use this to paginate through
	// all shared file members.
	ListFileMembersContinue(arg *ListFileMembersContinueArg) (res *SharedFileMembers, err error)
	// ListFolderMembers : Returns shared folder membership by its folder ID.
	ListFolderMembers(arg *ListFolderMembersArgs) (res *SharedFolderMembers, err error)
	// ListFolderMembersContinue : Once a cursor has been retrieved from
	// `listFolderMembers`, use this to paginate through all shared folder
	// members.
	ListFolderMembersContinue(arg *ListFolderMembersContinueArg) (res *SharedFolderMembers, err error)
	// ListFolders : Return the list of all shared folders the current user has
	// access to.
	ListFolders(arg *ListFoldersArgs) (res *ListFoldersResult, err error)
	// ListFoldersContinue : Once a cursor has been retrieved from
	// `listFolders`, use this to paginate through all shared folders. The
	// cursor must come from a previous call to `listFolders` or
	// `listFoldersContinue`.
	ListFoldersContinue(arg *ListFoldersContinueArg) (res *ListFoldersResult, err error)
	// ListMountableFolders : Return the list of all shared folders the current
	// user can mount or unmount.
	ListMountableFolders(arg *ListFoldersArgs) (res *ListFoldersResult, err error)
	// ListMountableFoldersContinue : Once a cursor has been retrieved from
	// `listMountableFolders`, use this to paginate through all mountable shared
	// folders. The cursor must come from a previous call to
	// `listMountableFolders` or `listMountableFoldersContinue`.
	ListMountableFoldersContinue(arg *ListFoldersContinueArg) (res *ListFoldersResult, err error)
	// ListReceivedFiles : Returns a list of all files shared with current user.
	// Does not include files the user has received via shared folders, and does
	// not include unclaimed invitations.
	ListReceivedFiles(arg *ListFilesArg) (res *ListFilesResult, err error)
	// ListReceivedFilesContinue : Get more results with a cursor from
	// `listReceivedFiles`.
	ListReceivedFilesContinue(arg *ListFilesContinueArg) (res *ListFilesResult, err error)
	// ListSharedLinks : List shared links of this user. If no path is given,
	// returns a list of all shared links for the current user. If a non-empty
	// path is given, returns a list of all shared links that allow access to
	// the given path - direct links to the given path and links to parent
	// folders of the given path. Links to parent folders can be suppressed by
	// setting direct_only to true.
	ListSharedLinks(arg *ListSharedLinksArg) (res *ListSharedLinksResult, err error)
	// ModifySharedLinkSettings : Modify the shared link's settings. If the
	// requested visibility conflict with the shared links policy of the team or
	// the shared folder (in case the linked file is part of a shared folder)
	// then the `LinkPermissions.resolved_visibility` of the returned
	// `SharedLinkMetadata` will reflect the actual visibility of the shared
	// link and the `LinkPermissions.requested_visibility` will reflect the
	// requested visibility.
	ModifySharedLinkSettings(arg *ModifySharedLinkSettingsArgs) (res IsSharedLinkMetadata, err error)
	// MountFolder : The current user mounts the designated folder. Mount a
	// shared folder for a user after they have been added as a member. Once
	// mounted, the shared folder will appear in their Dropbox.
	MountFolder(arg *MountFolderArg) (res *SharedFolderMetadata, err error)
	// RelinquishFileMembership : The current user relinquishes their membership
	// in the designated file. Note that the current user may still have
	// inherited access to this file through the parent folder.
	RelinquishFileMembership(arg *RelinquishFileMembershipArg) (err error)
	// RelinquishFolderMembership : The current user relinquishes their
	// membership in the designated shared folder and will no longer have access
	// to the folder.  A folder owner cannot relinquish membership in their own
	// folder. This will run synchronously if leave_a_copy is false, and
	// asynchronously if leave_a_copy is true.
	RelinquishFolderMembership(arg *RelinquishFolderMembershipArg) (res *async.LaunchEmptyResult, err error)
	// RemoveFileMember : Identical to remove_file_member_2 but with less
	// information returned.
	// Deprecated: Use `RemoveFileMember2` instead
	RemoveFileMember(arg *RemoveFileMemberArg) (res *FileMemberActionIndividualResult, err error)
	// RemoveFileMember2 : Removes a specified member from the file.
	RemoveFileMember2(arg *RemoveFileMemberArg) (res *FileMemberRemoveActionResult, err error)
	// RemoveFolderMember : Allows an owner or editor (if the ACL update policy
	// allows) of a shared folder to remove another member.
	RemoveFolderMember(arg *RemoveFolderMemberArg) (res *async.LaunchResultBase, err error)
	// RevokeSharedLink : Revoke a shared link. Note that even after revoking a
	// shared link to a file, the file may be accessible if there are shared
	// links leading to any of the file parent folders. To list all shared links
	// that enable access to a specific file, you can use the `listSharedLinks`
	// with the file as the `ListSharedLinksArg.path` argument.
	RevokeSharedLink(arg *RevokeSharedLinkArg) (err error)
	// SetAccessInheritance : Change the inheritance policy of an existing
	// Shared Folder. Only permitted for shared folders in a shared team root.
	// If a `ShareFolderLaunch.async_job_id` is returned, you'll need to call
	// `checkShareJobStatus` until the action completes to get the metadata for
	// the folder.
	SetAccessInheritance(arg *SetAccessInheritanceArg) (res *ShareFolderLaunch, err error)
	// ShareFolder : Share a folder with collaborators. Most sharing will be
	// completed synchronously. Large folders will be completed asynchronously.
	// To make testing the async case repeatable, set
	// `ShareFolderArg.force_async`. If a `ShareFolderLaunch.async_job_id` is
	// returned, you'll need to call `checkShareJobStatus` until the action
	// completes to get the metadata for the folder.
	ShareFolder(arg *ShareFolderArg) (res *ShareFolderLaunch, err error)
	// TransferFolder : Transfer ownership of a shared folder to a member of the
	// shared folder. User must have `AccessLevel.owner` access to the shared
	// folder to perform a transfer.
	TransferFolder(arg *TransferFolderArg) (err error)
	// UnmountFolder : The current user unmounts the designated folder. They can
	// re-mount the folder at a later time using `mountFolder`.
	UnmountFolder(arg *UnmountFolderArg) (err error)
	// UnshareFile : Remove all members from this file. Does not remove
	// inherited members.
	UnshareFile(arg *UnshareFileArg) (err error)
	// UnshareFolder : Allows a shared folder owner to unshare the folder.
	// You'll need to call `checkJobStatus` to determine if the action has
	// completed successfully.
	UnshareFolder(arg *UnshareFolderArg) (res *async.LaunchEmptyResult, err error)
	// UpdateFileMember : Changes a member's access on a shared file.
	UpdateFileMember(arg *UpdateFileMemberArgs) (res *MemberAccessLevelResult, err error)
	// UpdateFolderMember : Allows an owner or editor of a shared folder to
	// update another member's permissions.
	UpdateFolderMember(arg *UpdateFolderMemberArg) (res *MemberAccessLevelResult, err error)
	// UpdateFolderPolicy : Update the sharing policies for a shared folder.
	// User must have `AccessLevel.owner` access to the shared folder to update
	// its policies.
	UpdateFolderPolicy(arg *UpdateFolderPolicyArg) (res *SharedFolderMetadata, err error)
}

type apiImpl dropbox.Context

//AddFileMemberAPIError is an error-wrapper for the add_file_member route
type AddFileMemberAPIError struct {
	dropbox.APIError
	EndpointError *AddFileMemberError `json:"error"`
}

func (dbx *apiImpl) AddFileMember(arg *AddFileMemberArgs) (res []*FileMemberActionResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "add_file_member", headers, bytes.NewReader(b))
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
		var apiError AddFileMemberAPIError
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

//AddFolderMemberAPIError is an error-wrapper for the add_folder_member route
type AddFolderMemberAPIError struct {
	dropbox.APIError
	EndpointError *AddFolderMemberError `json:"error"`
}

func (dbx *apiImpl) AddFolderMember(arg *AddFolderMemberArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "add_folder_member", headers, bytes.NewReader(b))
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
		var apiError AddFolderMemberAPIError
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

//ChangeFileMemberAccessAPIError is an error-wrapper for the change_file_member_access route
type ChangeFileMemberAccessAPIError struct {
	dropbox.APIError
	EndpointError *FileMemberActionError `json:"error"`
}

func (dbx *apiImpl) ChangeFileMemberAccess(arg *ChangeFileMemberAccessArgs) (res *FileMemberActionResult, err error) {
	log.Printf("WARNING: API `ChangeFileMemberAccess` is deprecated")
	log.Printf("Use API `UpdateFileMember` instead")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "change_file_member_access", headers, bytes.NewReader(b))
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
		var apiError ChangeFileMemberAccessAPIError
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

//CheckJobStatusAPIError is an error-wrapper for the check_job_status route
type CheckJobStatusAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) CheckJobStatus(arg *async.PollArg) (res *JobStatus, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "check_job_status", headers, bytes.NewReader(b))
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
		var apiError CheckJobStatusAPIError
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

//CheckRemoveMemberJobStatusAPIError is an error-wrapper for the check_remove_member_job_status route
type CheckRemoveMemberJobStatusAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) CheckRemoveMemberJobStatus(arg *async.PollArg) (res *RemoveMemberJobStatus, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "check_remove_member_job_status", headers, bytes.NewReader(b))
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
		var apiError CheckRemoveMemberJobStatusAPIError
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

//CheckShareJobStatusAPIError is an error-wrapper for the check_share_job_status route
type CheckShareJobStatusAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) CheckShareJobStatus(arg *async.PollArg) (res *ShareFolderJobStatus, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "check_share_job_status", headers, bytes.NewReader(b))
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
		var apiError CheckShareJobStatusAPIError
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

//CreateSharedLinkAPIError is an error-wrapper for the create_shared_link route
type CreateSharedLinkAPIError struct {
	dropbox.APIError
	EndpointError *CreateSharedLinkError `json:"error"`
}

func (dbx *apiImpl) CreateSharedLink(arg *CreateSharedLinkArg) (res *PathLinkMetadata, err error) {
	log.Printf("WARNING: API `CreateSharedLink` is deprecated")
	log.Printf("Use API `CreateSharedLinkWithSettings` instead")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "create_shared_link", headers, bytes.NewReader(b))
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
		var apiError CreateSharedLinkAPIError
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

//CreateSharedLinkWithSettingsAPIError is an error-wrapper for the create_shared_link_with_settings route
type CreateSharedLinkWithSettingsAPIError struct {
	dropbox.APIError
	EndpointError *CreateSharedLinkWithSettingsError `json:"error"`
}

func (dbx *apiImpl) CreateSharedLinkWithSettings(arg *CreateSharedLinkWithSettingsArg) (res IsSharedLinkMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "create_shared_link_with_settings", headers, bytes.NewReader(b))
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
		var tmp sharedLinkMetadataUnion
		err = json.Unmarshal(body, &tmp)
		if err != nil {
			return
		}
		switch tmp.Tag {
		case "file":
			res = tmp.File

		case "folder":
			res = tmp.Folder

		}
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError CreateSharedLinkWithSettingsAPIError
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

//GetFileMetadataAPIError is an error-wrapper for the get_file_metadata route
type GetFileMetadataAPIError struct {
	dropbox.APIError
	EndpointError *GetFileMetadataError `json:"error"`
}

func (dbx *apiImpl) GetFileMetadata(arg *GetFileMetadataArg) (res *SharedFileMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "get_file_metadata", headers, bytes.NewReader(b))
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
		var apiError GetFileMetadataAPIError
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

//GetFileMetadataBatchAPIError is an error-wrapper for the get_file_metadata/batch route
type GetFileMetadataBatchAPIError struct {
	dropbox.APIError
	EndpointError *SharingUserError `json:"error"`
}

func (dbx *apiImpl) GetFileMetadataBatch(arg *GetFileMetadataBatchArg) (res []*GetFileMetadataBatchResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "get_file_metadata/batch", headers, bytes.NewReader(b))
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
		var apiError GetFileMetadataBatchAPIError
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

//GetFolderMetadataAPIError is an error-wrapper for the get_folder_metadata route
type GetFolderMetadataAPIError struct {
	dropbox.APIError
	EndpointError *SharedFolderAccessError `json:"error"`
}

func (dbx *apiImpl) GetFolderMetadata(arg *GetMetadataArgs) (res *SharedFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "get_folder_metadata", headers, bytes.NewReader(b))
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
		var apiError GetFolderMetadataAPIError
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

//GetSharedLinkFileAPIError is an error-wrapper for the get_shared_link_file route
type GetSharedLinkFileAPIError struct {
	dropbox.APIError
	EndpointError *GetSharedLinkFileError `json:"error"`
}

func (dbx *apiImpl) GetSharedLinkFile(arg *GetSharedLinkMetadataArg) (res IsSharedLinkMetadata, content io.ReadCloser, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Dropbox-API-Arg": dropbox.HTTPHeaderSafeJSON(b),
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("content", "download", true, "sharing", "get_shared_link_file", headers, nil)
	if err != nil {
		return
	}
	dbx.Config.LogInfo("req: %v", req)

	resp, err := cli.Do(req)
	if err != nil {
		return
	}

	dbx.Config.LogInfo("resp: %v", resp)
	body := []byte(resp.Header.Get("Dropbox-API-Result"))
	content = resp.Body
	dbx.Config.LogDebug("body: %s", body)
	if resp.StatusCode == http.StatusOK {
		var tmp sharedLinkMetadataUnion
		err = json.Unmarshal(body, &tmp)
		if err != nil {
			return
		}
		switch tmp.Tag {
		case "file":
			res = tmp.File

		case "folder":
			res = tmp.Folder

		}
		return
	}
	if resp.StatusCode == http.StatusConflict {
		defer resp.Body.Close()
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		var apiError GetSharedLinkFileAPIError
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

//GetSharedLinkMetadataAPIError is an error-wrapper for the get_shared_link_metadata route
type GetSharedLinkMetadataAPIError struct {
	dropbox.APIError
	EndpointError *SharedLinkError `json:"error"`
}

func (dbx *apiImpl) GetSharedLinkMetadata(arg *GetSharedLinkMetadataArg) (res IsSharedLinkMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "get_shared_link_metadata", headers, bytes.NewReader(b))
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
		var tmp sharedLinkMetadataUnion
		err = json.Unmarshal(body, &tmp)
		if err != nil {
			return
		}
		switch tmp.Tag {
		case "file":
			res = tmp.File

		case "folder":
			res = tmp.Folder

		}
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError GetSharedLinkMetadataAPIError
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

//GetSharedLinksAPIError is an error-wrapper for the get_shared_links route
type GetSharedLinksAPIError struct {
	dropbox.APIError
	EndpointError *GetSharedLinksError `json:"error"`
}

func (dbx *apiImpl) GetSharedLinks(arg *GetSharedLinksArg) (res *GetSharedLinksResult, err error) {
	log.Printf("WARNING: API `GetSharedLinks` is deprecated")
	log.Printf("Use API `ListSharedLinks` instead")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "get_shared_links", headers, bytes.NewReader(b))
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
		var apiError GetSharedLinksAPIError
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

//ListFileMembersAPIError is an error-wrapper for the list_file_members route
type ListFileMembersAPIError struct {
	dropbox.APIError
	EndpointError *ListFileMembersError `json:"error"`
}

func (dbx *apiImpl) ListFileMembers(arg *ListFileMembersArg) (res *SharedFileMembers, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_file_members", headers, bytes.NewReader(b))
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
		var apiError ListFileMembersAPIError
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

//ListFileMembersBatchAPIError is an error-wrapper for the list_file_members/batch route
type ListFileMembersBatchAPIError struct {
	dropbox.APIError
	EndpointError *SharingUserError `json:"error"`
}

func (dbx *apiImpl) ListFileMembersBatch(arg *ListFileMembersBatchArg) (res []*ListFileMembersBatchResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_file_members/batch", headers, bytes.NewReader(b))
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
		var apiError ListFileMembersBatchAPIError
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

//ListFileMembersContinueAPIError is an error-wrapper for the list_file_members/continue route
type ListFileMembersContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListFileMembersContinueError `json:"error"`
}

func (dbx *apiImpl) ListFileMembersContinue(arg *ListFileMembersContinueArg) (res *SharedFileMembers, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_file_members/continue", headers, bytes.NewReader(b))
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
		var apiError ListFileMembersContinueAPIError
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

//ListFolderMembersAPIError is an error-wrapper for the list_folder_members route
type ListFolderMembersAPIError struct {
	dropbox.APIError
	EndpointError *SharedFolderAccessError `json:"error"`
}

func (dbx *apiImpl) ListFolderMembers(arg *ListFolderMembersArgs) (res *SharedFolderMembers, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_folder_members", headers, bytes.NewReader(b))
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
		var apiError ListFolderMembersAPIError
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

//ListFolderMembersContinueAPIError is an error-wrapper for the list_folder_members/continue route
type ListFolderMembersContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListFolderMembersContinueError `json:"error"`
}

func (dbx *apiImpl) ListFolderMembersContinue(arg *ListFolderMembersContinueArg) (res *SharedFolderMembers, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_folder_members/continue", headers, bytes.NewReader(b))
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
		var apiError ListFolderMembersContinueAPIError
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

//ListFoldersAPIError is an error-wrapper for the list_folders route
type ListFoldersAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) ListFolders(arg *ListFoldersArgs) (res *ListFoldersResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_folders", headers, bytes.NewReader(b))
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
		var apiError ListFoldersAPIError
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

//ListFoldersContinueAPIError is an error-wrapper for the list_folders/continue route
type ListFoldersContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListFoldersContinueError `json:"error"`
}

func (dbx *apiImpl) ListFoldersContinue(arg *ListFoldersContinueArg) (res *ListFoldersResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_folders/continue", headers, bytes.NewReader(b))
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
		var apiError ListFoldersContinueAPIError
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

//ListMountableFoldersAPIError is an error-wrapper for the list_mountable_folders route
type ListMountableFoldersAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) ListMountableFolders(arg *ListFoldersArgs) (res *ListFoldersResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_mountable_folders", headers, bytes.NewReader(b))
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
		var apiError ListMountableFoldersAPIError
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

//ListMountableFoldersContinueAPIError is an error-wrapper for the list_mountable_folders/continue route
type ListMountableFoldersContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListFoldersContinueError `json:"error"`
}

func (dbx *apiImpl) ListMountableFoldersContinue(arg *ListFoldersContinueArg) (res *ListFoldersResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_mountable_folders/continue", headers, bytes.NewReader(b))
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
		var apiError ListMountableFoldersContinueAPIError
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

//ListReceivedFilesAPIError is an error-wrapper for the list_received_files route
type ListReceivedFilesAPIError struct {
	dropbox.APIError
	EndpointError *SharingUserError `json:"error"`
}

func (dbx *apiImpl) ListReceivedFiles(arg *ListFilesArg) (res *ListFilesResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_received_files", headers, bytes.NewReader(b))
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
		var apiError ListReceivedFilesAPIError
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

//ListReceivedFilesContinueAPIError is an error-wrapper for the list_received_files/continue route
type ListReceivedFilesContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListFilesContinueError `json:"error"`
}

func (dbx *apiImpl) ListReceivedFilesContinue(arg *ListFilesContinueArg) (res *ListFilesResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_received_files/continue", headers, bytes.NewReader(b))
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
		var apiError ListReceivedFilesContinueAPIError
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

//ListSharedLinksAPIError is an error-wrapper for the list_shared_links route
type ListSharedLinksAPIError struct {
	dropbox.APIError
	EndpointError *ListSharedLinksError `json:"error"`
}

func (dbx *apiImpl) ListSharedLinks(arg *ListSharedLinksArg) (res *ListSharedLinksResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "list_shared_links", headers, bytes.NewReader(b))
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
		var apiError ListSharedLinksAPIError
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

//ModifySharedLinkSettingsAPIError is an error-wrapper for the modify_shared_link_settings route
type ModifySharedLinkSettingsAPIError struct {
	dropbox.APIError
	EndpointError *ModifySharedLinkSettingsError `json:"error"`
}

func (dbx *apiImpl) ModifySharedLinkSettings(arg *ModifySharedLinkSettingsArgs) (res IsSharedLinkMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "modify_shared_link_settings", headers, bytes.NewReader(b))
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
		var tmp sharedLinkMetadataUnion
		err = json.Unmarshal(body, &tmp)
		if err != nil {
			return
		}
		switch tmp.Tag {
		case "file":
			res = tmp.File

		case "folder":
			res = tmp.Folder

		}
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError ModifySharedLinkSettingsAPIError
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

//MountFolderAPIError is an error-wrapper for the mount_folder route
type MountFolderAPIError struct {
	dropbox.APIError
	EndpointError *MountFolderError `json:"error"`
}

func (dbx *apiImpl) MountFolder(arg *MountFolderArg) (res *SharedFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "mount_folder", headers, bytes.NewReader(b))
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
		var apiError MountFolderAPIError
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

//RelinquishFileMembershipAPIError is an error-wrapper for the relinquish_file_membership route
type RelinquishFileMembershipAPIError struct {
	dropbox.APIError
	EndpointError *RelinquishFileMembershipError `json:"error"`
}

func (dbx *apiImpl) RelinquishFileMembership(arg *RelinquishFileMembershipArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "relinquish_file_membership", headers, bytes.NewReader(b))
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
		var apiError RelinquishFileMembershipAPIError
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

//RelinquishFolderMembershipAPIError is an error-wrapper for the relinquish_folder_membership route
type RelinquishFolderMembershipAPIError struct {
	dropbox.APIError
	EndpointError *RelinquishFolderMembershipError `json:"error"`
}

func (dbx *apiImpl) RelinquishFolderMembership(arg *RelinquishFolderMembershipArg) (res *async.LaunchEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "relinquish_folder_membership", headers, bytes.NewReader(b))
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
		var apiError RelinquishFolderMembershipAPIError
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

//RemoveFileMemberAPIError is an error-wrapper for the remove_file_member route
type RemoveFileMemberAPIError struct {
	dropbox.APIError
	EndpointError *RemoveFileMemberError `json:"error"`
}

func (dbx *apiImpl) RemoveFileMember(arg *RemoveFileMemberArg) (res *FileMemberActionIndividualResult, err error) {
	log.Printf("WARNING: API `RemoveFileMember` is deprecated")
	log.Printf("Use API `RemoveFileMember2` instead")

	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "remove_file_member", headers, bytes.NewReader(b))
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
		var apiError RemoveFileMemberAPIError
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

//RemoveFileMember2APIError is an error-wrapper for the remove_file_member_2 route
type RemoveFileMember2APIError struct {
	dropbox.APIError
	EndpointError *RemoveFileMemberError `json:"error"`
}

func (dbx *apiImpl) RemoveFileMember2(arg *RemoveFileMemberArg) (res *FileMemberRemoveActionResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "remove_file_member_2", headers, bytes.NewReader(b))
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
		var apiError RemoveFileMember2APIError
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

//RemoveFolderMemberAPIError is an error-wrapper for the remove_folder_member route
type RemoveFolderMemberAPIError struct {
	dropbox.APIError
	EndpointError *RemoveFolderMemberError `json:"error"`
}

func (dbx *apiImpl) RemoveFolderMember(arg *RemoveFolderMemberArg) (res *async.LaunchResultBase, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "remove_folder_member", headers, bytes.NewReader(b))
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
		var apiError RemoveFolderMemberAPIError
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

//RevokeSharedLinkAPIError is an error-wrapper for the revoke_shared_link route
type RevokeSharedLinkAPIError struct {
	dropbox.APIError
	EndpointError *RevokeSharedLinkError `json:"error"`
}

func (dbx *apiImpl) RevokeSharedLink(arg *RevokeSharedLinkArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "revoke_shared_link", headers, bytes.NewReader(b))
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
		var apiError RevokeSharedLinkAPIError
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

//SetAccessInheritanceAPIError is an error-wrapper for the set_access_inheritance route
type SetAccessInheritanceAPIError struct {
	dropbox.APIError
	EndpointError *SetAccessInheritanceError `json:"error"`
}

func (dbx *apiImpl) SetAccessInheritance(arg *SetAccessInheritanceArg) (res *ShareFolderLaunch, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "set_access_inheritance", headers, bytes.NewReader(b))
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
		var apiError SetAccessInheritanceAPIError
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

//ShareFolderAPIError is an error-wrapper for the share_folder route
type ShareFolderAPIError struct {
	dropbox.APIError
	EndpointError *ShareFolderError `json:"error"`
}

func (dbx *apiImpl) ShareFolder(arg *ShareFolderArg) (res *ShareFolderLaunch, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "share_folder", headers, bytes.NewReader(b))
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
		var apiError ShareFolderAPIError
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

//TransferFolderAPIError is an error-wrapper for the transfer_folder route
type TransferFolderAPIError struct {
	dropbox.APIError
	EndpointError *TransferFolderError `json:"error"`
}

func (dbx *apiImpl) TransferFolder(arg *TransferFolderArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "transfer_folder", headers, bytes.NewReader(b))
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
		var apiError TransferFolderAPIError
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

//UnmountFolderAPIError is an error-wrapper for the unmount_folder route
type UnmountFolderAPIError struct {
	dropbox.APIError
	EndpointError *UnmountFolderError `json:"error"`
}

func (dbx *apiImpl) UnmountFolder(arg *UnmountFolderArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "unmount_folder", headers, bytes.NewReader(b))
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
		var apiError UnmountFolderAPIError
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

//UnshareFileAPIError is an error-wrapper for the unshare_file route
type UnshareFileAPIError struct {
	dropbox.APIError
	EndpointError *UnshareFileError `json:"error"`
}

func (dbx *apiImpl) UnshareFile(arg *UnshareFileArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "unshare_file", headers, bytes.NewReader(b))
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
		var apiError UnshareFileAPIError
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

//UnshareFolderAPIError is an error-wrapper for the unshare_folder route
type UnshareFolderAPIError struct {
	dropbox.APIError
	EndpointError *UnshareFolderError `json:"error"`
}

func (dbx *apiImpl) UnshareFolder(arg *UnshareFolderArg) (res *async.LaunchEmptyResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "unshare_folder", headers, bytes.NewReader(b))
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
		var apiError UnshareFolderAPIError
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

//UpdateFileMemberAPIError is an error-wrapper for the update_file_member route
type UpdateFileMemberAPIError struct {
	dropbox.APIError
	EndpointError *FileMemberActionError `json:"error"`
}

func (dbx *apiImpl) UpdateFileMember(arg *UpdateFileMemberArgs) (res *MemberAccessLevelResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "update_file_member", headers, bytes.NewReader(b))
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
		var apiError UpdateFileMemberAPIError
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

//UpdateFolderMemberAPIError is an error-wrapper for the update_folder_member route
type UpdateFolderMemberAPIError struct {
	dropbox.APIError
	EndpointError *UpdateFolderMemberError `json:"error"`
}

func (dbx *apiImpl) UpdateFolderMember(arg *UpdateFolderMemberArg) (res *MemberAccessLevelResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "update_folder_member", headers, bytes.NewReader(b))
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
		var apiError UpdateFolderMemberAPIError
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

//UpdateFolderPolicyAPIError is an error-wrapper for the update_folder_policy route
type UpdateFolderPolicyAPIError struct {
	dropbox.APIError
	EndpointError *UpdateFolderPolicyError `json:"error"`
}

func (dbx *apiImpl) UpdateFolderPolicy(arg *UpdateFolderPolicyArg) (res *SharedFolderMetadata, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "sharing", "update_folder_policy", headers, bytes.NewReader(b))
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
		var apiError UpdateFolderPolicyAPIError
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
