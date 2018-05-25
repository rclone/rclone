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

// Package team_common : has no documentation (yet)
package team_common

import (
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// GroupManagementType : The group type determines how a group is managed.
type GroupManagementType struct {
	dropbox.Tagged
}

// Valid tag values for GroupManagementType
const (
	GroupManagementTypeUserManaged    = "user_managed"
	GroupManagementTypeCompanyManaged = "company_managed"
	GroupManagementTypeSystemManaged  = "system_managed"
	GroupManagementTypeOther          = "other"
)

// GroupSummary : Information about a group.
type GroupSummary struct {
	// GroupName : has no documentation (yet)
	GroupName string `json:"group_name"`
	// GroupId : has no documentation (yet)
	GroupId string `json:"group_id"`
	// GroupExternalId : External ID of group. This is an arbitrary ID that an
	// admin can attach to a group.
	GroupExternalId string `json:"group_external_id,omitempty"`
	// MemberCount : The number of members in the group.
	MemberCount uint32 `json:"member_count,omitempty"`
	// GroupManagementType : Who is allowed to manage the group.
	GroupManagementType *GroupManagementType `json:"group_management_type"`
}

// NewGroupSummary returns a new GroupSummary instance
func NewGroupSummary(GroupName string, GroupId string, GroupManagementType *GroupManagementType) *GroupSummary {
	s := new(GroupSummary)
	s.GroupName = GroupName
	s.GroupId = GroupId
	s.GroupManagementType = GroupManagementType
	return s
}

// GroupType : The group type determines how a group is created and managed.
type GroupType struct {
	dropbox.Tagged
}

// Valid tag values for GroupType
const (
	GroupTypeTeam        = "team"
	GroupTypeUserManaged = "user_managed"
	GroupTypeOther       = "other"
)

// MemberSpaceLimitType : The type of the space limit imposed on a team member.
type MemberSpaceLimitType struct {
	dropbox.Tagged
}

// Valid tag values for MemberSpaceLimitType
const (
	MemberSpaceLimitTypeOff       = "off"
	MemberSpaceLimitTypeAlertOnly = "alert_only"
	MemberSpaceLimitTypeStopSync  = "stop_sync"
	MemberSpaceLimitTypeOther     = "other"
)

// TimeRange : Time range.
type TimeRange struct {
	// StartTime : Optional starting time (inclusive).
	StartTime time.Time `json:"start_time,omitempty"`
	// EndTime : Optional ending time (exclusive).
	EndTime time.Time `json:"end_time,omitempty"`
}

// NewTimeRange returns a new TimeRange instance
func NewTimeRange() *TimeRange {
	s := new(TimeRange)
	return s
}
