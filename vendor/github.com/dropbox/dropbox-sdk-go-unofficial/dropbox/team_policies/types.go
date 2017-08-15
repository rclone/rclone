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

// Package team_policies : has no documentation (yet)
package team_policies

import "github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"

// EmmState : has no documentation (yet)
type EmmState struct {
	dropbox.Tagged
}

// Valid tag values for EmmState
const (
	EmmStateDisabled = "disabled"
	EmmStateOptional = "optional"
	EmmStateRequired = "required"
	EmmStateOther    = "other"
)

// OfficeAddInPolicy : has no documentation (yet)
type OfficeAddInPolicy struct {
	dropbox.Tagged
}

// Valid tag values for OfficeAddInPolicy
const (
	OfficeAddInPolicyDisabled = "disabled"
	OfficeAddInPolicyEnabled  = "enabled"
	OfficeAddInPolicyOther    = "other"
)

// SharedFolderJoinPolicy : Policy governing which shared folders a team member
// can join.
type SharedFolderJoinPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderJoinPolicy
const (
	SharedFolderJoinPolicyFromTeamOnly = "from_team_only"
	SharedFolderJoinPolicyFromAnyone   = "from_anyone"
	SharedFolderJoinPolicyOther        = "other"
)

// SharedFolderMemberPolicy : Policy governing who can be a member of a folder
// shared by a team member.
type SharedFolderMemberPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedFolderMemberPolicy
const (
	SharedFolderMemberPolicyTeam   = "team"
	SharedFolderMemberPolicyAnyone = "anyone"
	SharedFolderMemberPolicyOther  = "other"
)

// SharedLinkCreatePolicy : Policy governing the visibility of shared links.
// This policy can apply to newly created shared links, or all shared links.
type SharedLinkCreatePolicy struct {
	dropbox.Tagged
}

// Valid tag values for SharedLinkCreatePolicy
const (
	SharedLinkCreatePolicyDefaultPublic   = "default_public"
	SharedLinkCreatePolicyDefaultTeamOnly = "default_team_only"
	SharedLinkCreatePolicyTeamOnly        = "team_only"
	SharedLinkCreatePolicyOther           = "other"
)

// TeamMemberPolicies : Policies governing team members.
type TeamMemberPolicies struct {
	// Sharing : Policies governing sharing.
	Sharing *TeamSharingPolicies `json:"sharing"`
	// EmmState : This describes the Enterprise Mobility Management (EMM) state
	// for this team. This information can be used to understand if an
	// organization is integrating with a third-party EMM vendor to further
	// manage and apply restrictions upon the team's Dropbox usage on mobile
	// devices. This is a new feature and in the future we'll be adding more new
	// fields and additional documentation.
	EmmState *EmmState `json:"emm_state"`
	// OfficeAddin : The admin policy around the Dropbox Office Add-In for this
	// team.
	OfficeAddin *OfficeAddInPolicy `json:"office_addin"`
}

// NewTeamMemberPolicies returns a new TeamMemberPolicies instance
func NewTeamMemberPolicies(Sharing *TeamSharingPolicies, EmmState *EmmState, OfficeAddin *OfficeAddInPolicy) *TeamMemberPolicies {
	s := new(TeamMemberPolicies)
	s.Sharing = Sharing
	s.EmmState = EmmState
	s.OfficeAddin = OfficeAddin
	return s
}

// TeamSharingPolicies : Policies governing sharing within and outside of the
// team.
type TeamSharingPolicies struct {
	// SharedFolderMemberPolicy : Who can join folders shared by team members.
	SharedFolderMemberPolicy *SharedFolderMemberPolicy `json:"shared_folder_member_policy"`
	// SharedFolderJoinPolicy : Which shared folders team members can join.
	SharedFolderJoinPolicy *SharedFolderJoinPolicy `json:"shared_folder_join_policy"`
	// SharedLinkCreatePolicy : Who can view shared links owned by team members.
	SharedLinkCreatePolicy *SharedLinkCreatePolicy `json:"shared_link_create_policy"`
}

// NewTeamSharingPolicies returns a new TeamSharingPolicies instance
func NewTeamSharingPolicies(SharedFolderMemberPolicy *SharedFolderMemberPolicy, SharedFolderJoinPolicy *SharedFolderJoinPolicy, SharedLinkCreatePolicy *SharedLinkCreatePolicy) *TeamSharingPolicies {
	s := new(TeamSharingPolicies)
	s.SharedFolderMemberPolicy = SharedFolderMemberPolicy
	s.SharedFolderJoinPolicy = SharedFolderJoinPolicy
	s.SharedLinkCreatePolicy = SharedLinkCreatePolicy
	return s
}
