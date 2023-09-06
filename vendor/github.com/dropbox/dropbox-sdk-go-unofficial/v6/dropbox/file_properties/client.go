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
	"encoding/json"
	"io"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/auth"
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
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "properties/add",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesAddAPIError
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

//PropertiesOverwriteAPIError is an error-wrapper for the properties/overwrite route
type PropertiesOverwriteAPIError struct {
	dropbox.APIError
	EndpointError *InvalidPropertyGroupError `json:"error"`
}

func (dbx *apiImpl) PropertiesOverwrite(arg *OverwritePropertyGroupArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "properties/overwrite",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesOverwriteAPIError
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

//PropertiesRemoveAPIError is an error-wrapper for the properties/remove route
type PropertiesRemoveAPIError struct {
	dropbox.APIError
	EndpointError *RemovePropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesRemove(arg *RemovePropertiesArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "properties/remove",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesRemoveAPIError
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

//PropertiesSearchAPIError is an error-wrapper for the properties/search route
type PropertiesSearchAPIError struct {
	dropbox.APIError
	EndpointError *PropertiesSearchError `json:"error"`
}

func (dbx *apiImpl) PropertiesSearch(arg *PropertiesSearchArg) (res *PropertiesSearchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "properties/search",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesSearchAPIError
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

//PropertiesSearchContinueAPIError is an error-wrapper for the properties/search/continue route
type PropertiesSearchContinueAPIError struct {
	dropbox.APIError
	EndpointError *PropertiesSearchContinueError `json:"error"`
}

func (dbx *apiImpl) PropertiesSearchContinue(arg *PropertiesSearchContinueArg) (res *PropertiesSearchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "properties/search/continue",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesSearchContinueAPIError
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

//PropertiesUpdateAPIError is an error-wrapper for the properties/update route
type PropertiesUpdateAPIError struct {
	dropbox.APIError
	EndpointError *UpdatePropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesUpdate(arg *UpdatePropertiesArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "properties/update",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesUpdateAPIError
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

//TemplatesAddForTeamAPIError is an error-wrapper for the templates/add_for_team route
type TemplatesAddForTeamAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesAddForTeam(arg *AddTemplateArg) (res *AddTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/add_for_team",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesAddForTeamAPIError
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

//TemplatesAddForUserAPIError is an error-wrapper for the templates/add_for_user route
type TemplatesAddForUserAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesAddForUser(arg *AddTemplateArg) (res *AddTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/add_for_user",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesAddForUserAPIError
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

//TemplatesGetForTeamAPIError is an error-wrapper for the templates/get_for_team route
type TemplatesGetForTeamAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesGetForTeam(arg *GetTemplateArg) (res *GetTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/get_for_team",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesGetForTeamAPIError
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

//TemplatesGetForUserAPIError is an error-wrapper for the templates/get_for_user route
type TemplatesGetForUserAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesGetForUser(arg *GetTemplateArg) (res *GetTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/get_for_user",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesGetForUserAPIError
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

//TemplatesListForTeamAPIError is an error-wrapper for the templates/list_for_team route
type TemplatesListForTeamAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesListForTeam() (res *ListTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/list_for_team",
		Auth:         "team",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesListForTeamAPIError
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

//TemplatesListForUserAPIError is an error-wrapper for the templates/list_for_user route
type TemplatesListForUserAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesListForUser() (res *ListTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/list_for_user",
		Auth:         "user",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesListForUserAPIError
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

//TemplatesRemoveForTeamAPIError is an error-wrapper for the templates/remove_for_team route
type TemplatesRemoveForTeamAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesRemoveForTeam(arg *RemoveTemplateArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/remove_for_team",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesRemoveForTeamAPIError
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

//TemplatesRemoveForUserAPIError is an error-wrapper for the templates/remove_for_user route
type TemplatesRemoveForUserAPIError struct {
	dropbox.APIError
	EndpointError *TemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesRemoveForUser(arg *RemoveTemplateArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/remove_for_user",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesRemoveForUserAPIError
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

//TemplatesUpdateForTeamAPIError is an error-wrapper for the templates/update_for_team route
type TemplatesUpdateForTeamAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesUpdateForTeam(arg *UpdateTemplateArg) (res *UpdateTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/update_for_team",
		Auth:         "team",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesUpdateForTeamAPIError
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

//TemplatesUpdateForUserAPIError is an error-wrapper for the templates/update_for_user route
type TemplatesUpdateForUserAPIError struct {
	dropbox.APIError
	EndpointError *ModifyTemplateError `json:"error"`
}

func (dbx *apiImpl) TemplatesUpdateForUser(arg *UpdateTemplateArg) (res *UpdateTemplateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "file_properties",
		Route:        "templates/update_for_user",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TemplatesUpdateForUserAPIError
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
