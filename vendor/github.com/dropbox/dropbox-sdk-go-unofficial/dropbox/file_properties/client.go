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

package file_properties

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/auth"
)

// Client interface describes all routes in this namespace
type Client interface {
	// PropertiesAdd : Add property groups to a Dropbox file. See
	// `templatesAddForUser` or `templatesAddForTeam` to create new templates.
	PropertiesAdd(arg *AddPropertiesArg) (err error)
	// PropertiesOverwrite : Overwrite property groups associated with a file.
	// This endpoint should be used instead of `propertiesUpdate` when property
	// groups are being updated via a "snapshot" instead of via a "delta". In
	// other words, this endpoint will delete all omitted fields from a property
	// group, whereas `propertiesUpdate` will only delete fields that are
	// explicitly marked for deletion.
	PropertiesOverwrite(arg *OverwritePropertyGroupArg) (err error)
	// PropertiesRemove : Permanently removes the specified property group from
	// the file. To remove specific property field key value pairs, see
	// `propertiesUpdate`. To update a template, see `templatesUpdateForUser` or
	// `templatesUpdateForTeam`. To remove a template, see
	// `templatesRemoveForUser` or `templatesRemoveForTeam`.
	PropertiesRemove(arg *RemovePropertiesArg) (err error)
	// PropertiesSearch : Search across property templates for particular
	// property field values.
	PropertiesSearch(arg *PropertiesSearchArg) (res *PropertiesSearchResult, err error)
	// PropertiesSearchContinue : Once a cursor has been retrieved from
	// `propertiesSearch`, use this to paginate through all search results.
	PropertiesSearchContinue(arg *PropertiesSearchContinueArg) (res *PropertiesSearchResult, err error)
	// PropertiesUpdate : Add, update or remove properties associated with the
	// supplied file and templates. This endpoint should be used instead of
	// `propertiesOverwrite` when property groups are being updated via a
	// "delta" instead of via a "snapshot" . In other words, this endpoint will
	// not delete any omitted fields from a property group, whereas
	// `propertiesOverwrite` will delete any fields that are omitted from a
	// property group.
	PropertiesUpdate(arg *UpdatePropertiesArg) (err error)
	// TemplatesAddForTeam : Add a template associated with a team. See
	// `propertiesAdd` to add properties to a file or folder. Note: this
	// endpoint will create team-owned templates.
	TemplatesAddForTeam(arg *AddTemplateArg) (res *AddTemplateResult, err error)
	// TemplatesAddForUser : Add a template associated with a user. See
	// `propertiesAdd` to add properties to a file. This endpoint can't be
	// called on a team member or admin's behalf.
	TemplatesAddForUser(arg *AddTemplateArg) (res *AddTemplateResult, err error)
	// TemplatesGetForTeam : Get the schema for a specified template.
	TemplatesGetForTeam(arg *GetTemplateArg) (res *GetTemplateResult, err error)
	// TemplatesGetForUser : Get the schema for a specified template. This
	// endpoint can't be called on a team member or admin's behalf.
	TemplatesGetForUser(arg *GetTemplateArg) (res *GetTemplateResult, err error)
	// TemplatesListForTeam : Get the template identifiers for a team. To get
	// the schema of each template use `templatesGetForTeam`.
	TemplatesListForTeam() (res *ListTemplateResult, err error)
	// TemplatesListForUser : Get the template identifiers for a team. To get
	// the schema of each template use `templatesGetForUser`. This endpoint
	// can't be called on a team member or admin's behalf.
	TemplatesListForUser() (res *ListTemplateResult, err error)
	// TemplatesRemoveForTeam : Permanently removes the specified template
	// created from `templatesAddForUser`. All properties associated with the
	// template will also be removed. This action cannot be undone.
	TemplatesRemoveForTeam(arg *RemoveTemplateArg) (err error)
	// TemplatesRemoveForUser : Permanently removes the specified template
	// created from `templatesAddForUser`. All properties associated with the
	// template will also be removed. This action cannot be undone.
	TemplatesRemoveForUser(arg *RemoveTemplateArg) (err error)
	// TemplatesUpdateForTeam : Update a template associated with a team. This
	// route can update the template name, the template description and add
	// optional properties to templates.
	TemplatesUpdateForTeam(arg *UpdateTemplateArg) (res *UpdateTemplateResult, err error)
	// TemplatesUpdateForUser : Update a template associated with a user. This
	// route can update the template name, the template description and add
	// optional properties to templates. This endpoint can't be called on a team
	// member or admin's behalf.
	TemplatesUpdateForUser(arg *UpdateTemplateArg) (res *UpdateTemplateResult, err error)
}

type apiImpl dropbox.Context

//PropertiesAddAPIError is an error-wrapper for the properties/add route
type PropertiesAddAPIError struct {
	dropbox.APIError
	EndpointError *AddPropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesAdd(arg *AddPropertiesArg) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "properties/add", headers, bytes.NewReader(b))
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
		var apiError PropertiesAddAPIError
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

//PropertiesOverwriteAPIError is an error-wrapper for the properties/overwrite route
type PropertiesOverwriteAPIError struct {
	dropbox.APIError
	EndpointError *InvalidPropertyGroupError `json:"error"`
}

func (dbx *apiImpl) PropertiesOverwrite(arg *OverwritePropertyGroupArg) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "properties/overwrite", headers, bytes.NewReader(b))
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
		var apiError PropertiesOverwriteAPIError
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

//PropertiesRemoveAPIError is an error-wrapper for the properties/remove route
type PropertiesRemoveAPIError struct {
	dropbox.APIError
	EndpointError *RemovePropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesRemove(arg *RemovePropertiesArg) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "properties/remove", headers, bytes.NewReader(b))
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
		var apiError PropertiesRemoveAPIError
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

//PropertiesSearchAPIError is an error-wrapper for the properties/search route
type PropertiesSearchAPIError struct {
	dropbox.APIError
	EndpointError *PropertiesSearchError `json:"error"`
}

func (dbx *apiImpl) PropertiesSearch(arg *PropertiesSearchArg) (res *PropertiesSearchResult, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "properties/search", headers, bytes.NewReader(b))
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
		var apiError PropertiesSearchAPIError
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

//PropertiesSearchContinueAPIError is an error-wrapper for the properties/search/continue route
type PropertiesSearchContinueAPIError struct {
	dropbox.APIError
	EndpointError *PropertiesSearchContinueError `json:"error"`
}

func (dbx *apiImpl) PropertiesSearchContinue(arg *PropertiesSearchContinueArg) (res *PropertiesSearchResult, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "properties/search/continue", headers, bytes.NewReader(b))
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
		var apiError PropertiesSearchContinueAPIError
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

//PropertiesUpdateAPIError is an error-wrapper for the properties/update route
type PropertiesUpdateAPIError struct {
	dropbox.APIError
	EndpointError *UpdatePropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesUpdate(arg *UpdatePropertiesArg) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "properties/update", headers, bytes.NewReader(b))
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
		var apiError PropertiesUpdateAPIError
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

//TemplatesAddForTeamAPIError is an error-wrapper for the templates/add_for_team route
type TemplatesAddForTeamAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesAddForTeam(arg *AddTemplateArg) (res *AddTemplateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/add_for_team", headers, bytes.NewReader(b))
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
		var apiError TemplatesAddForTeamAPIError
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

//TemplatesAddForUserAPIError is an error-wrapper for the templates/add_for_user route
type TemplatesAddForUserAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesAddForUser(arg *AddTemplateArg) (res *AddTemplateResult, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/add_for_user", headers, bytes.NewReader(b))
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
		var apiError TemplatesAddForUserAPIError
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

//TemplatesGetForTeamAPIError is an error-wrapper for the templates/get_for_team route
type TemplatesGetForTeamAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesGetForTeam(arg *GetTemplateArg) (res *GetTemplateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/get_for_team", headers, bytes.NewReader(b))
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
		var apiError TemplatesGetForTeamAPIError
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

//TemplatesGetForUserAPIError is an error-wrapper for the templates/get_for_user route
type TemplatesGetForUserAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesGetForUser(arg *GetTemplateArg) (res *GetTemplateResult, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/get_for_user", headers, bytes.NewReader(b))
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
		var apiError TemplatesGetForUserAPIError
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

//TemplatesListForTeamAPIError is an error-wrapper for the templates/list_for_team route
type TemplatesListForTeamAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesListForTeam() (res *ListTemplateResult, err error) {
	cli := dbx.Client

	headers := map[string]string{}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/list_for_team", headers, nil)
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
		var apiError TemplatesListForTeamAPIError
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

//TemplatesListForUserAPIError is an error-wrapper for the templates/list_for_user route
type TemplatesListForUserAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesListForUser() (res *ListTemplateResult, err error) {
	cli := dbx.Client

	headers := map[string]string{}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/list_for_user", headers, nil)
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
		var apiError TemplatesListForUserAPIError
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

//TemplatesRemoveForTeamAPIError is an error-wrapper for the templates/remove_for_team route
type TemplatesRemoveForTeamAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesRemoveForTeam(arg *RemoveTemplateArg) (err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/remove_for_team", headers, bytes.NewReader(b))
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
		var apiError TemplatesRemoveForTeamAPIError
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

//TemplatesRemoveForUserAPIError is an error-wrapper for the templates/remove_for_user route
type TemplatesRemoveForUserAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesRemoveForUser(arg *RemoveTemplateArg) (err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/remove_for_user", headers, bytes.NewReader(b))
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
		var apiError TemplatesRemoveForUserAPIError
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

//TemplatesUpdateForTeamAPIError is an error-wrapper for the templates/update_for_team route
type TemplatesUpdateForTeamAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesUpdateForTeam(arg *UpdateTemplateArg) (res *UpdateTemplateResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/update_for_team", headers, bytes.NewReader(b))
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
		var apiError TemplatesUpdateForTeamAPIError
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

//TemplatesUpdateForUserAPIError is an error-wrapper for the templates/update_for_user route
type TemplatesUpdateForUserAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesUpdateForUser(arg *UpdateTemplateArg) (res *UpdateTemplateResult, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "file_properties", "templates/update_for_user", headers, bytes.NewReader(b))
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
		var apiError TemplatesUpdateForUserAPIError
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
