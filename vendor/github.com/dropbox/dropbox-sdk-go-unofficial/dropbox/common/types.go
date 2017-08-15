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

// Package common : has no documentation (yet)
package common

import (
	"encoding/json"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// InvalidPathRootError : has no documentation (yet)
type InvalidPathRootError struct {
	// PathRoot : The latest path root id for user's team if the user is still
	// in a team.
	PathRoot string `json:"path_root,omitempty"`
}

// NewInvalidPathRootError returns a new InvalidPathRootError instance
func NewInvalidPathRootError() *InvalidPathRootError {
	s := new(InvalidPathRootError)
	return s
}

// PathRoot : has no documentation (yet)
type PathRoot struct {
	dropbox.Tagged
	// Team : Paths are relative to the given team directory. (This results in
	// `PathRootError.invalid` if the user is not a member of the team
	// associated with that path root id.)
	Team string `json:"team,omitempty"`
	// NamespaceId : Paths are relative to given namespace id (This results in
	// `PathRootError.no_permission` if you don't have access to this
	// namespace.)
	NamespaceId string `json:"namespace_id,omitempty"`
}

// Valid tag values for PathRoot
const (
	PathRootHome        = "home"
	PathRootMemberHome  = "member_home"
	PathRootTeam        = "team"
	PathRootUserHome    = "user_home"
	PathRootNamespaceId = "namespace_id"
	PathRootOther       = "other"
)

// UnmarshalJSON deserializes into a PathRoot instance
func (u *PathRoot) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "team":
		err = json.Unmarshal(body, &u.Team)

		if err != nil {
			return err
		}
	case "namespace_id":
		err = json.Unmarshal(body, &u.NamespaceId)

		if err != nil {
			return err
		}
	}
	return nil
}

// PathRootError : has no documentation (yet)
type PathRootError struct {
	dropbox.Tagged
	// Invalid : The path root id value in Dropbox-API-Path-Root header is no
	// longer valid.
	Invalid *InvalidPathRootError `json:"invalid,omitempty"`
}

// Valid tag values for PathRootError
const (
	PathRootErrorInvalid      = "invalid"
	PathRootErrorNoPermission = "no_permission"
	PathRootErrorOther        = "other"
)

// UnmarshalJSON deserializes into a PathRootError instance
func (u *PathRootError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Invalid : The path root id value in Dropbox-API-Path-Root header is
		// no longer valid.
		Invalid json.RawMessage `json:"invalid,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "invalid":
		err = json.Unmarshal(body, &u.Invalid)

		if err != nil {
			return err
		}
	}
	return nil
}
