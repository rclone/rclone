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

// Package auth : has no documentation (yet)
package auth

import (
	"encoding/json"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// AccessError : Error occurred because the account doesn't have permission to
// access the resource.
type AccessError struct {
	dropbox.Tagged
	// InvalidAccountType : Current account type cannot access the resource.
	InvalidAccountType *InvalidAccountTypeError `json:"invalid_account_type,omitempty"`
	// PaperAccessDenied : Current account cannot access Paper.
	PaperAccessDenied *PaperAccessError `json:"paper_access_denied,omitempty"`
}

// Valid tag values for AccessError
const (
	AccessErrorInvalidAccountType = "invalid_account_type"
	AccessErrorPaperAccessDenied  = "paper_access_denied"
	AccessErrorOther              = "other"
)

// UnmarshalJSON deserializes into a AccessError instance
func (u *AccessError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// InvalidAccountType : Current account type cannot access the resource.
		InvalidAccountType json.RawMessage `json:"invalid_account_type,omitempty"`
		// PaperAccessDenied : Current account cannot access Paper.
		PaperAccessDenied json.RawMessage `json:"paper_access_denied,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "invalid_account_type":
		err = json.Unmarshal(w.InvalidAccountType, &u.InvalidAccountType)

		if err != nil {
			return err
		}
	case "paper_access_denied":
		err = json.Unmarshal(w.PaperAccessDenied, &u.PaperAccessDenied)

		if err != nil {
			return err
		}
	}
	return nil
}

// AuthError : Errors occurred during authentication.
type AuthError struct {
	dropbox.Tagged
}

// Valid tag values for AuthError
const (
	AuthErrorInvalidAccessToken = "invalid_access_token"
	AuthErrorInvalidSelectUser  = "invalid_select_user"
	AuthErrorInvalidSelectAdmin = "invalid_select_admin"
	AuthErrorUserSuspended      = "user_suspended"
	AuthErrorExpiredAccessToken = "expired_access_token"
	AuthErrorOther              = "other"
)

// InvalidAccountTypeError : has no documentation (yet)
type InvalidAccountTypeError struct {
	dropbox.Tagged
}

// Valid tag values for InvalidAccountTypeError
const (
	InvalidAccountTypeErrorEndpoint = "endpoint"
	InvalidAccountTypeErrorFeature  = "feature"
	InvalidAccountTypeErrorOther    = "other"
)

// PaperAccessError : has no documentation (yet)
type PaperAccessError struct {
	dropbox.Tagged
}

// Valid tag values for PaperAccessError
const (
	PaperAccessErrorPaperDisabled = "paper_disabled"
	PaperAccessErrorNotPaperUser  = "not_paper_user"
	PaperAccessErrorOther         = "other"
)

// RateLimitError : Error occurred because the app is being rate limited.
type RateLimitError struct {
	// Reason : The reason why the app is being rate limited.
	Reason *RateLimitReason `json:"reason"`
	// RetryAfter : The number of seconds that the app should wait before making
	// another request.
	RetryAfter uint64 `json:"retry_after"`
}

// NewRateLimitError returns a new RateLimitError instance
func NewRateLimitError(Reason *RateLimitReason) *RateLimitError {
	s := new(RateLimitError)
	s.Reason = Reason
	s.RetryAfter = 1
	return s
}

// RateLimitReason : has no documentation (yet)
type RateLimitReason struct {
	dropbox.Tagged
}

// Valid tag values for RateLimitReason
const (
	RateLimitReasonTooManyRequests        = "too_many_requests"
	RateLimitReasonTooManyWriteOperations = "too_many_write_operations"
	RateLimitReasonOther                  = "other"
)

// TokenFromOAuth1Arg : has no documentation (yet)
type TokenFromOAuth1Arg struct {
	// Oauth1Token : The supplied OAuth 1.0 access token.
	Oauth1Token string `json:"oauth1_token"`
	// Oauth1TokenSecret : The token secret associated with the supplied access
	// token.
	Oauth1TokenSecret string `json:"oauth1_token_secret"`
}

// NewTokenFromOAuth1Arg returns a new TokenFromOAuth1Arg instance
func NewTokenFromOAuth1Arg(Oauth1Token string, Oauth1TokenSecret string) *TokenFromOAuth1Arg {
	s := new(TokenFromOAuth1Arg)
	s.Oauth1Token = Oauth1Token
	s.Oauth1TokenSecret = Oauth1TokenSecret
	return s
}

// TokenFromOAuth1Error : has no documentation (yet)
type TokenFromOAuth1Error struct {
	dropbox.Tagged
}

// Valid tag values for TokenFromOAuth1Error
const (
	TokenFromOAuth1ErrorInvalidOauth1TokenInfo = "invalid_oauth1_token_info"
	TokenFromOAuth1ErrorAppIdMismatch          = "app_id_mismatch"
	TokenFromOAuth1ErrorOther                  = "other"
)

// TokenFromOAuth1Result : has no documentation (yet)
type TokenFromOAuth1Result struct {
	// Oauth2Token : The OAuth 2.0 token generated from the supplied OAuth 1.0
	// token.
	Oauth2Token string `json:"oauth2_token"`
}

// NewTokenFromOAuth1Result returns a new TokenFromOAuth1Result instance
func NewTokenFromOAuth1Result(Oauth2Token string) *TokenFromOAuth1Result {
	s := new(TokenFromOAuth1Result)
	s.Oauth2Token = Oauth2Token
	return s
}
