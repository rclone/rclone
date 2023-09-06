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
	"encoding/json"
	"io"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
)

// Client interface describes all routes in this namespace
type Client interface {
	// TokenFromOauth1 : Creates an OAuth 2.0 access token from the supplied
	// OAuth 1.0 access token.
	TokenFromOauth1(arg *TokenFromOAuth1Arg) (res *TokenFromOAuth1Result, err error)
	// TokenRevoke : Disables the access token used to authenticate the call. If
	// there is a corresponding refresh token for the access token, this
	// disables that refresh token, as well as any other access tokens for that
	// refresh token.
	TokenRevoke() (err error)
}

type apiImpl dropbox.Context

//TokenFromOauth1APIError is an error-wrapper for the token/from_oauth1 route
type TokenFromOauth1APIError struct {
	dropbox.APIError
	EndpointError *TokenFromOAuth1Error `json:"error"`
}

func (dbx *apiImpl) TokenFromOauth1(arg *TokenFromOAuth1Arg) (res *TokenFromOAuth1Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "auth",
		Route:        "token/from_oauth1",
		Auth:         "app",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TokenFromOauth1APIError
		err = ParseError(err, &appErr)
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

//TokenRevokeAPIError is an error-wrapper for the token/revoke route
type TokenRevokeAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) TokenRevoke() (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "auth",
		Route:        "token/revoke",
		Auth:         "user",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TokenRevokeAPIError
		err = ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

// New returns a Client implementation for this namespace
func New(c dropbox.Config) Client {
	ctx := apiImpl(dropbox.NewContext(c))
	return &ctx
}
