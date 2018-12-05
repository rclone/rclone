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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/auth"
)

// Client interface describes all routes in this namespace
type Client interface {
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

//GetAccountAPIError is an error-wrapper for the get_account route
type GetAccountAPIError struct {
	dropbox.APIError
	EndpointError *GetAccountError `json:"error"`
}

func (dbx *apiImpl) GetAccount(arg *GetAccountArg) (res *BasicAccount, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "users", "get_account", headers, bytes.NewReader(b))
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
		var apiError GetAccountAPIError
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

//GetAccountBatchAPIError is an error-wrapper for the get_account_batch route
type GetAccountBatchAPIError struct {
	dropbox.APIError
	EndpointError *GetAccountBatchError `json:"error"`
}

func (dbx *apiImpl) GetAccountBatch(arg *GetAccountBatchArg) (res []*BasicAccount, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "users", "get_account_batch", headers, bytes.NewReader(b))
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
		var apiError GetAccountBatchAPIError
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

//GetCurrentAccountAPIError is an error-wrapper for the get_current_account route
type GetCurrentAccountAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetCurrentAccount() (res *FullAccount, err error) {
	cli := dbx.Client

	headers := map[string]string{}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "users", "get_current_account", headers, nil)
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
		var apiError GetCurrentAccountAPIError
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

//GetSpaceUsageAPIError is an error-wrapper for the get_space_usage route
type GetSpaceUsageAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetSpaceUsage() (res *SpaceUsage, err error) {
	cli := dbx.Client

	headers := map[string]string{}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "users", "get_space_usage", headers, nil)
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
		var apiError GetSpaceUsageAPIError
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
