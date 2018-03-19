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

package paper

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// Client interface describes all routes in this namespace
type Client interface {
	// DocsArchive : Marks the given Paper doc as archived. Note: This action
	// can be performed or undone by anyone with edit permissions to the doc.
	DocsArchive(arg *RefPaperDoc) (err error)
	// DocsCreate : Creates a new Paper doc with the provided content.
	DocsCreate(arg *PaperDocCreateArgs, content io.Reader) (res *PaperDocCreateUpdateResult, err error)
	// DocsDownload : Exports and downloads Paper doc either as HTML or
	// markdown.
	DocsDownload(arg *PaperDocExport) (res *PaperDocExportResult, content io.ReadCloser, err error)
	// DocsFolderUsersList : Lists the users who are explicitly invited to the
	// Paper folder in which the Paper doc is contained. For private folders all
	// users (including owner) shared on the folder are listed and for team
	// folders all non-team users shared on the folder are returned.
	DocsFolderUsersList(arg *ListUsersOnFolderArgs) (res *ListUsersOnFolderResponse, err error)
	// DocsFolderUsersListContinue : Once a cursor has been retrieved from
	// `docsFolderUsersList`, use this to paginate through all users on the
	// Paper folder.
	DocsFolderUsersListContinue(arg *ListUsersOnFolderContinueArgs) (res *ListUsersOnFolderResponse, err error)
	// DocsGetFolderInfo : Retrieves folder information for the given Paper doc.
	// This includes:   - folder sharing policy; permissions for subfolders are
	// set by the top-level folder.   - full 'filepath', i.e. the list of
	// folders (both folderId and folderName) from the root folder to the folder
	// directly containing the Paper doc.  Note: If the Paper doc is not in any
	// folder (aka unfiled) the response will be empty.
	DocsGetFolderInfo(arg *RefPaperDoc) (res *FoldersContainingPaperDoc, err error)
	// DocsList : Return the list of all Paper docs according to the argument
	// specifications. To iterate over through the full pagination, pass the
	// cursor to `docsListContinue`.
	DocsList(arg *ListPaperDocsArgs) (res *ListPaperDocsResponse, err error)
	// DocsListContinue : Once a cursor has been retrieved from `docsList`, use
	// this to paginate through all Paper doc.
	DocsListContinue(arg *ListPaperDocsContinueArgs) (res *ListPaperDocsResponse, err error)
	// DocsPermanentlyDelete : Permanently deletes the given Paper doc. This
	// operation is final as the doc cannot be recovered.  Note: This action can
	// be performed only by the doc owner.
	DocsPermanentlyDelete(arg *RefPaperDoc) (err error)
	// DocsSharingPolicyGet : Gets the default sharing policy for the given
	// Paper doc.
	DocsSharingPolicyGet(arg *RefPaperDoc) (res *SharingPolicy, err error)
	// DocsSharingPolicySet : Sets the default sharing policy for the given
	// Paper doc. The default 'team_sharing_policy' can be changed only by
	// teams, omit this field for personal accounts.  Note:
	// 'public_sharing_policy' cannot be set to the value 'disabled' because
	// this setting can be changed only via the team admin console.
	DocsSharingPolicySet(arg *PaperDocSharingPolicy) (err error)
	// DocsUpdate : Updates an existing Paper doc with the provided content.
	DocsUpdate(arg *PaperDocUpdateArgs, content io.Reader) (res *PaperDocCreateUpdateResult, err error)
	// DocsUsersAdd : Allows an owner or editor to add users to a Paper doc or
	// change their permissions using their email address or Dropbox account ID.
	// Note: The Doc owner's permissions cannot be changed.
	DocsUsersAdd(arg *AddPaperDocUser) (res []*AddPaperDocUserMemberResult, err error)
	// DocsUsersList : Lists all users who visited the Paper doc or users with
	// explicit access. This call excludes users who have been removed. The list
	// is sorted by the date of the visit or the share date. The list will
	// include both users, the explicitly shared ones as well as those who came
	// in using the Paper url link.
	DocsUsersList(arg *ListUsersOnPaperDocArgs) (res *ListUsersOnPaperDocResponse, err error)
	// DocsUsersListContinue : Once a cursor has been retrieved from
	// `docsUsersList`, use this to paginate through all users on the Paper doc.
	DocsUsersListContinue(arg *ListUsersOnPaperDocContinueArgs) (res *ListUsersOnPaperDocResponse, err error)
	// DocsUsersRemove : Allows an owner or editor to remove users from a Paper
	// doc using their email address or Dropbox account ID.  Note: Doc owner
	// cannot be removed.
	DocsUsersRemove(arg *RemovePaperDocUser) (err error)
}

type apiImpl dropbox.Context

//DocsArchiveAPIError is an error-wrapper for the docs/archive route
type DocsArchiveAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsArchive(arg *RefPaperDoc) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/archive", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsArchiveAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsCreateAPIError is an error-wrapper for the docs/create route
type DocsCreateAPIError struct {
	dropbox.APIError
	EndpointError *PaperDocCreateError `json:"error"`
}

func (dbx *apiImpl) DocsCreate(arg *PaperDocCreateArgs, content io.Reader) (res *PaperDocCreateUpdateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type":    "application/octet-stream",
		"Dropbox-API-Arg": string(b),
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "upload", true, "paper", "docs/create", headers, content)
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsCreateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsDownloadAPIError is an error-wrapper for the docs/download route
type DocsDownloadAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsDownload(arg *PaperDocExport) (res *PaperDocExportResult, content io.ReadCloser, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Dropbox-API-Arg": string(b),
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "download", true, "paper", "docs/download", headers, bytes.NewReader(b))
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
	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		defer resp.Body.Close()
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		var apiError DocsDownloadAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsFolderUsersListAPIError is an error-wrapper for the docs/folder_users/list route
type DocsFolderUsersListAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsFolderUsersList(arg *ListUsersOnFolderArgs) (res *ListUsersOnFolderResponse, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/folder_users/list", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsFolderUsersListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsFolderUsersListContinueAPIError is an error-wrapper for the docs/folder_users/list/continue route
type DocsFolderUsersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListUsersCursorError `json:"error"`
}

func (dbx *apiImpl) DocsFolderUsersListContinue(arg *ListUsersOnFolderContinueArgs) (res *ListUsersOnFolderResponse, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/folder_users/list/continue", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsFolderUsersListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsGetFolderInfoAPIError is an error-wrapper for the docs/get_folder_info route
type DocsGetFolderInfoAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsGetFolderInfo(arg *RefPaperDoc) (res *FoldersContainingPaperDoc, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/get_folder_info", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsGetFolderInfoAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsListAPIError is an error-wrapper for the docs/list route
type DocsListAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) DocsList(arg *ListPaperDocsArgs) (res *ListPaperDocsResponse, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/list", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsListContinueAPIError is an error-wrapper for the docs/list/continue route
type DocsListContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListDocsCursorError `json:"error"`
}

func (dbx *apiImpl) DocsListContinue(arg *ListPaperDocsContinueArgs) (res *ListPaperDocsResponse, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/list/continue", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsPermanentlyDeleteAPIError is an error-wrapper for the docs/permanently_delete route
type DocsPermanentlyDeleteAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsPermanentlyDelete(arg *RefPaperDoc) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/permanently_delete", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsPermanentlyDeleteAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsSharingPolicyGetAPIError is an error-wrapper for the docs/sharing_policy/get route
type DocsSharingPolicyGetAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsSharingPolicyGet(arg *RefPaperDoc) (res *SharingPolicy, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/sharing_policy/get", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsSharingPolicyGetAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsSharingPolicySetAPIError is an error-wrapper for the docs/sharing_policy/set route
type DocsSharingPolicySetAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsSharingPolicySet(arg *PaperDocSharingPolicy) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/sharing_policy/set", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsSharingPolicySetAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsUpdateAPIError is an error-wrapper for the docs/update route
type DocsUpdateAPIError struct {
	dropbox.APIError
	EndpointError *PaperDocUpdateError `json:"error"`
}

func (dbx *apiImpl) DocsUpdate(arg *PaperDocUpdateArgs, content io.Reader) (res *PaperDocCreateUpdateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type":    "application/octet-stream",
		"Dropbox-API-Arg": string(b),
	}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "upload", true, "paper", "docs/update", headers, content)
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsUpdateAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsUsersAddAPIError is an error-wrapper for the docs/users/add route
type DocsUsersAddAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsUsersAdd(arg *AddPaperDocUser) (res []*AddPaperDocUserMemberResult, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/users/add", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsUsersAddAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsUsersListAPIError is an error-wrapper for the docs/users/list route
type DocsUsersListAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsUsersList(arg *ListUsersOnPaperDocArgs) (res *ListUsersOnPaperDocResponse, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/users/list", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsUsersListAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsUsersListContinueAPIError is an error-wrapper for the docs/users/list/continue route
type DocsUsersListContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListUsersCursorError `json:"error"`
}

func (dbx *apiImpl) DocsUsersListContinue(arg *ListUsersOnPaperDocContinueArgs) (res *ListUsersOnPaperDocResponse, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/users/list/continue", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &res)
		if err != nil {
			return
		}

		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsUsersListContinueAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

//DocsUsersRemoveAPIError is an error-wrapper for the docs/users/remove route
type DocsUsersRemoveAPIError struct {
	dropbox.APIError
	EndpointError *DocLookupError `json:"error"`
}

func (dbx *apiImpl) DocsUsersRemove(arg *RemovePaperDocUser) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "paper", "docs/users/remove", headers, bytes.NewReader(b))
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

	dbx.Config.LogDebug("body: %v", body)
	if resp.StatusCode == http.StatusOK {
		return
	}
	if resp.StatusCode == http.StatusConflict {
		var apiError DocsUsersRemoveAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	var apiError dropbox.APIError
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		apiError.ErrorSummary = string(body)
		err = apiError
		return
	}
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return
	}
	err = apiError
	return
}

// New returns a Client implementation for this namespace
func New(c dropbox.Config) Client {
	ctx := apiImpl(dropbox.NewContext(c))
	return &ctx
}
