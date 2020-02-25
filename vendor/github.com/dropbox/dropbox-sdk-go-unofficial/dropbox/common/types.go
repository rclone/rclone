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

// PathRoot : has no documentation (yet)
type PathRoot struct {
	dropbox.Tagged
	// Root : Paths are relative to the authenticating user's root namespace
	// (This results in `PathRootError.invalid_root` if the user's root
	// namespace has changed.).
	Root string `json:"root,omitempty"`
	// NamespaceId : Paths are relative to given namespace id (This results in
	// `PathRootError.no_permission` if you don't have access to this
	// namespace.).
	NamespaceId string `json:"namespace_id,omitempty"`
}

// Valid tag values for PathRoot
const (
	PathRootHome        = "home"
	PathRootRoot        = "root"
	PathRootNamespaceId = "namespace_id"
	PathRootOther       = "other"
)

// UnmarshalJSON deserializes into a PathRoot instance
func (u *PathRoot) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// Root : Paths are relative to the authenticating user's root namespace
		// (This results in `PathRootError.invalid_root` if the user's root
		// namespace has changed.).
		Root string `json:"root,omitempty"`
		// NamespaceId : Paths are relative to given namespace id (This results
		// in `PathRootError.no_permission` if you don't have access to this
		// namespace.).
		NamespaceId string `json:"namespace_id,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "root":
		u.Root = w.Root

		if err != nil {
			return err
		}
	case "namespace_id":
		u.NamespaceId = w.NamespaceId

		if err != nil {
			return err
		}
	}
	return nil
}

// PathRootError : has no documentation (yet)
type PathRootError struct {
	dropbox.Tagged
	// InvalidRoot : The root namespace id in Dropbox-API-Path-Root header is
	// not valid. The value of this error is use's latest root info.
	InvalidRoot IsRootInfo `json:"invalid_root,omitempty"`
}

// Valid tag values for PathRootError
const (
	PathRootErrorInvalidRoot  = "invalid_root"
	PathRootErrorNoPermission = "no_permission"
	PathRootErrorOther        = "other"
)

// UnmarshalJSON deserializes into a PathRootError instance
func (u *PathRootError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// InvalidRoot : The root namespace id in Dropbox-API-Path-Root header
		// is not valid. The value of this error is use's latest root info.
		InvalidRoot json.RawMessage `json:"invalid_root,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "invalid_root":
		u.InvalidRoot, err = IsRootInfoFromJSON(w.InvalidRoot)

		if err != nil {
			return err
		}
	}
	return nil
}

// RootInfo : Information about current user's root.
type RootInfo struct {
	// RootNamespaceId : The namespace ID for user's root namespace. It will be
	// the namespace ID of the shared team root if the user is member of a team
	// with a separate team root. Otherwise it will be same as
	// `RootInfo.home_namespace_id`.
	RootNamespaceId string `json:"root_namespace_id"`
	// HomeNamespaceId : The namespace ID for user's home namespace.
	HomeNamespaceId string `json:"home_namespace_id"`
}

// NewRootInfo returns a new RootInfo instance
func NewRootInfo(RootNamespaceId string, HomeNamespaceId string) *RootInfo {
	s := new(RootInfo)
	s.RootNamespaceId = RootNamespaceId
	s.HomeNamespaceId = HomeNamespaceId
	return s
}

// IsRootInfo is the interface type for RootInfo and its subtypes
type IsRootInfo interface {
	IsRootInfo()
}

// IsRootInfo implements the IsRootInfo interface
func (u *RootInfo) IsRootInfo() {}

type rootInfoUnion struct {
	dropbox.Tagged
	// Team : has no documentation (yet)
	Team *TeamRootInfo `json:"team,omitempty"`
	// User : has no documentation (yet)
	User *UserRootInfo `json:"user,omitempty"`
}

// Valid tag values for RootInfo
const (
	RootInfoTeam = "team"
	RootInfoUser = "user"
)

// UnmarshalJSON deserializes into a rootInfoUnion instance
func (u *rootInfoUnion) UnmarshalJSON(body []byte) error {
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
	case "user":
		err = json.Unmarshal(body, &u.User)

		if err != nil {
			return err
		}
	}
	return nil
}

// IsRootInfoFromJSON converts JSON to a concrete IsRootInfo instance
func IsRootInfoFromJSON(data []byte) (IsRootInfo, error) {
	var t rootInfoUnion
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	switch t.Tag {
	case "team":
		return t.Team, nil

	case "user":
		return t.User, nil

	}
	return nil, nil
}

// TeamRootInfo : Root info when user is member of a team with a separate root
// namespace ID.
type TeamRootInfo struct {
	RootInfo
	// HomePath : The path for user's home directory under the shared team root.
	HomePath string `json:"home_path"`
}

// NewTeamRootInfo returns a new TeamRootInfo instance
func NewTeamRootInfo(RootNamespaceId string, HomeNamespaceId string, HomePath string) *TeamRootInfo {
	s := new(TeamRootInfo)
	s.RootNamespaceId = RootNamespaceId
	s.HomeNamespaceId = HomeNamespaceId
	s.HomePath = HomePath
	return s
}

// UserRootInfo : Root info when user is not member of a team or the user is a
// member of a team and the team does not have a separate root namespace.
type UserRootInfo struct {
	RootInfo
}

// NewUserRootInfo returns a new UserRootInfo instance
func NewUserRootInfo(RootNamespaceId string, HomeNamespaceId string) *UserRootInfo {
	s := new(UserRootInfo)
	s.RootNamespaceId = RootNamespaceId
	s.HomeNamespaceId = HomeNamespaceId
	return s
}
