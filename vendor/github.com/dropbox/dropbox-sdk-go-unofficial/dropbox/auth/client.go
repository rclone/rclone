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

package auth

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// Client interface describes all routes in this namespace
type Client interface {
	// TokenFromOauth1 : Creates an OAuth 2.0 access token from the supplied
	// OAuth 1.0 access token.
	TokenFromOauth1(arg *TokenFromOAuth1Arg) (res *TokenFromOAuth1Result, err error)
	// TokenRevoke : Disables the access token used to authenticate the call.
	TokenRevoke() (err error)
}

type apiImpl dropbox.Context

//TokenFromOauth1APIError is an error-wrapper for the token/from_oauth1 route
type TokenFromOauth1APIError struct {
	dropbox.APIError
	EndpointError *TokenFromOAuth1Error `json:"error"`
}

func (dbx *apiImpl) TokenFromOauth1(arg *TokenFromOAuth1Arg) (res *TokenFromOAuth1Result, err error) {
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

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "auth", "token/from_oauth1", headers, bytes.NewReader(b))
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
		var apiError TokenFromOauth1APIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = HandleCommonAuthErrors(dbx.Config, resp, body)
	if err != nil {
		return
	}
	err = dropbox.HandleCommonAPIErrors(dbx.Config, resp, body)
	return
}

//TokenRevokeAPIError is an error-wrapper for the token/revoke route
type TokenRevokeAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) TokenRevoke() (err error) {
	cli := dbx.Client

	headers := map[string]string{}
	if dbx.Config.AsMemberID != "" {
		headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID
	}

	req, err := (*dropbox.Context)(dbx).NewRequest("api", "rpc", true, "auth", "token/revoke", headers, nil)
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
		var apiError TokenRevokeAPIError
		err = json.Unmarshal(body, &apiError)
		if err != nil {
			return
		}
		err = apiError
		return
	}
	err = HandleCommonAuthErrors(dbx.Config, resp, body)
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
