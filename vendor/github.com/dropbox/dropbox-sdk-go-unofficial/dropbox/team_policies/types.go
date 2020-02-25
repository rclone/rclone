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

// CameraUploadsPolicyState : has no documentation (yet)
type CameraUploadsPolicyState struct {
	dropbox.Tagged
}

// Valid tag values for CameraUploadsPolicyState
const (
	CameraUploadsPolicyStateDisabled = "disabled"
	CameraUploadsPolicyStateEnabled  = "enabled"
	CameraUploadsPolicyStateOther    = "other"
)

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

// GroupCreation : has no documentation (yet)
type GroupCreation struct {
	dropbox.Tagged
}

// Valid tag values for GroupCreation
const (
	GroupCreationAdminsAndMembers = "admins_and_members"
	GroupCreationAdminsOnly       = "admins_only"
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

// PaperDefaultFolderPolicy : has no documentation (yet)
type PaperDefaultFolderPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperDefaultFolderPolicy
const (
	PaperDefaultFolderPolicyEveryoneInTeam = "everyone_in_team"
	PaperDefaultFolderPolicyInviteOnly     = "invite_only"
	PaperDefaultFolderPolicyOther          = "other"
)

// PaperDeploymentPolicy : has no documentation (yet)
type PaperDeploymentPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperDeploymentPolicy
const (
	PaperDeploymentPolicyFull    = "full"
	PaperDeploymentPolicyPartial = "partial"
	PaperDeploymentPolicyOther   = "other"
)

// PaperDesktopPolicy : has no documentation (yet)
type PaperDesktopPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperDesktopPolicy
const (
	PaperDesktopPolicyDisabled = "disabled"
	PaperDesktopPolicyEnabled  = "enabled"
	PaperDesktopPolicyOther    = "other"
)

// PaperEnabledPolicy : has no documentation (yet)
type PaperEnabledPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PaperEnabledPolicy
const (
	PaperEnabledPolicyDisabled    = "disabled"
	PaperEnabledPolicyEnabled     = "enabled"
	PaperEnabledPolicyUnspecified = "unspecified"
	PaperEnabledPolicyOther       = "other"
)

// PasswordStrengthPolicy : has no documentation (yet)
type PasswordStrengthPolicy struct {
	dropbox.Tagged
}

// Valid tag values for PasswordStrengthPolicy
const (
	PasswordStrengthPolicyMinimalRequirements = "minimal_requirements"
	PasswordStrengthPolicyModeratePassword    = "moderate_password"
	PasswordStrengthPolicyStrongPassword      = "strong_password"
	PasswordStrengthPolicyOther               = "other"
)

// RolloutMethod : has no documentation (yet)
type RolloutMethod struct {
	dropbox.Tagged
}

// Valid tag values for RolloutMethod
const (
	RolloutMethodUnlinkAll             = "unlink_all"
	RolloutMethodUnlinkMostInactive    = "unlink_most_inactive"
	RolloutMethodAddMemberToExceptions = "add_member_to_exceptions"
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

// ShowcaseDownloadPolicy : has no documentation (yet)
type ShowcaseDownloadPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ShowcaseDownloadPolicy
const (
	ShowcaseDownloadPolicyDisabled = "disabled"
	ShowcaseDownloadPolicyEnabled  = "enabled"
	ShowcaseDownloadPolicyOther    = "other"
)

// ShowcaseEnabledPolicy : has no documentation (yet)
type ShowcaseEnabledPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ShowcaseEnabledPolicy
const (
	ShowcaseEnabledPolicyDisabled = "disabled"
	ShowcaseEnabledPolicyEnabled  = "enabled"
	ShowcaseEnabledPolicyOther    = "other"
)

// ShowcaseExternalSharingPolicy : has no documentation (yet)
type ShowcaseExternalSharingPolicy struct {
	dropbox.Tagged
}

// Valid tag values for ShowcaseExternalSharingPolicy
const (
	ShowcaseExternalSharingPolicyDisabled = "disabled"
	ShowcaseExternalSharingPolicyEnabled  = "enabled"
	ShowcaseExternalSharingPolicyOther    = "other"
)

// SmartSyncPolicy : has no documentation (yet)
type SmartSyncPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SmartSyncPolicy
const (
	SmartSyncPolicyLocal    = "local"
	SmartSyncPolicyOnDemand = "on_demand"
	SmartSyncPolicyOther    = "other"
)

// SsoPolicy : has no documentation (yet)
type SsoPolicy struct {
	dropbox.Tagged
}

// Valid tag values for SsoPolicy
const (
	SsoPolicyDisabled = "disabled"
	SsoPolicyOptional = "optional"
	SsoPolicyRequired = "required"
	SsoPolicyOther    = "other"
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

// TwoStepVerificationPolicy : has no documentation (yet)
type TwoStepVerificationPolicy struct {
	dropbox.Tagged
}

// Valid tag values for TwoStepVerificationPolicy
const (
	TwoStepVerificationPolicyRequireTfaEnable  = "require_tfa_enable"
	TwoStepVerificationPolicyRequireTfaDisable = "require_tfa_disable"
	TwoStepVerificationPolicyOther             = "other"
)

// TwoStepVerificationState : has no documentation (yet)
type TwoStepVerificationState struct {
	dropbox.Tagged
}

// Valid tag values for TwoStepVerificationState
const (
	TwoStepVerificationStateRequired = "required"
	TwoStepVerificationStateOptional = "optional"
	TwoStepVerificationStateOther    = "other"
)
