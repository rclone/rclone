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

package users

import (
	"encoding/json"
	"io"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/auth"
)

// Client interface describes all routes in this namespace
type Client interface {
	// FeaturesGetValues : Get a list of feature values that may be configured
	// for the current account.
	FeaturesGetValues(arg *UserFeaturesGetValuesBatchArg) (res *UserFeaturesGetValuesBatchResult, err error)
	// GetAccount : Get information about a user's account.
	GetAccount(arg *GetAccountArg) (res *BasicAccount, err error)
	// GetAccountBatch : Get information about multiple user accounts.  At most
	// 300 accounts may be queried per request.
	GetAccountBatch(arg *GetAccountBatchArg) (res []*BasicAccount, err error)
	// GetCurrentAccount : Get information about the current user's account.
	GetCurrentAccount() (res *FullAccount, err error)
	// GetSpaceUsage : Get the space usage information for the current user's
	// account.
	GetSpaceUsage() (res *SpaceUsage, err error)
}

type apiImpl dropbox.Context

//FeaturesGetValuesAPIError is an error-wrapper for the features/get_values route
type FeaturesGetValuesAPIError struct {
	dropbox.APIError
	EndpointError *UserFeaturesGetValuesBatchError `json:"error"`
}

func (dbx *apiImpl) FeaturesGetValues(arg *UserFeaturesGetValuesBatchArg) (res *UserFeaturesGetValuesBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "users",
		Route:        "features/get_values",
		Auth:         "user",
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

//GetAccountAPIError is an error-wrapper for the get_account route
type GetAccountAPIError struct {
	dropbox.APIError
	EndpointError *GetAccountError `json:"error"`
}

func (dbx *apiImpl) GetAccount(arg *GetAccountArg) (res *BasicAccount, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "users",
		Route:        "get_account",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetAccountAPIError
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

//GetAccountBatchAPIError is an error-wrapper for the get_account_batch route
type GetAccountBatchAPIError struct {
	dropbox.APIError
	EndpointError *GetAccountBatchError `json:"error"`
}

func (dbx *apiImpl) GetAccountBatch(arg *GetAccountBatchArg) (res []*BasicAccount, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "users",
		Route:        "get_account_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetAccountBatchAPIError
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

//GetCurrentAccountAPIError is an error-wrapper for the get_current_account route
type GetCurrentAccountAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetCurrentAccount() (res *FullAccount, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "users",
		Route:        "get_current_account",
		Auth:         "user",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetCurrentAccountAPIError
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

//GetSpaceUsageAPIError is an error-wrapper for the get_space_usage route
type GetSpaceUsageAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetSpaceUsage() (res *SpaceUsage, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "users",
		Route:        "get_space_usage",
		Auth:         "user",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetSpaceUsageAPIError
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
