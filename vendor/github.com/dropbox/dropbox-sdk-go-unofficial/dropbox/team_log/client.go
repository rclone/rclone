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

package team_log

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// Client interface describes all routes in this namespace
type Client interface {
	// GetEvents : Retrieves team events. Permission : Team Auditing.
	GetEvents(arg *GetTeamEventsArg) (res *GetTeamEventsResult, err error)
	// GetEventsContinue : Once a cursor has been retrieved from `getEvents`,
	// use this to paginate through all events. Permission : Team Auditing.
	GetEventsContinue(arg *GetTeamEventsContinueArg) (res *GetTeamEventsResult, err error)
}

type apiImpl dropbox.Context

//GetEventsAPIError is an error-wrapper for the get_events route
type GetEventsAPIError struct {
	dropbox.APIError
	EndpointError *GetTeamEventsError `json:"error"`
}

func (dbx *apiImpl) GetEvents(arg *GetTeamEventsArg) (res *GetTeamEventsResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team_log", "get_events", headers, bytes.NewReader(b))
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
		var apiError GetEventsAPIError
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

//GetEventsContinueAPIError is an error-wrapper for the get_events/continue route
type GetEventsContinueAPIError struct {
	dropbox.APIError
	EndpointError *GetTeamEventsContinueError `json:"error"`
}

func (dbx *apiImpl) GetEventsContinue(arg *GetTeamEventsContinueArg) (res *GetTeamEventsResult, err error) {
	cli := dbx.Client

	dbx.Config.LogDebug("arg: %v", arg)
	b, err := json.Marshal(arg)
	if err != nil {
		return
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "team_log", "get_events/continue", headers, bytes.NewReader(b))
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
		var apiError GetEventsContinueAPIError
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
