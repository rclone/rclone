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

// Package users : This namespace contains endpoints and data types for user
// management.
package users

import (
	"encoding/json"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/common"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team_common"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team_policies"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/users_common"
)

// Account : The amount of detail revealed about an account depends on the user
// being queried and the user making the query.
type Account struct {
	// AccountId : The user's unique Dropbox ID.
	AccountId string `json:"account_id"`
	// Name : Details of a user's name.
	Name *Name `json:"name"`
	// Email : The user's e-mail address. Do not rely on this without checking
	// the `email_verified` field. Even then, it's possible that the user has
	// since lost access to their e-mail.
	Email string `json:"email"`
	// EmailVerified : Whether the user has verified their e-mail address.
	EmailVerified bool `json:"email_verified"`
	// ProfilePhotoUrl : URL for the photo representing the user, if one is set.
	ProfilePhotoUrl string `json:"profile_photo_url,omitempty"`
	// Disabled : Whether the user has been disabled.
	Disabled bool `json:"disabled"`
}

// NewAccount returns a new Account instance
func NewAccount(AccountId string, Name *Name, Email string, EmailVerified bool, Disabled bool) *Account {
	s := new(Account)
	s.AccountId = AccountId
	s.Name = Name
	s.Email = Email
	s.EmailVerified = EmailVerified
	s.Disabled = Disabled
	return s
}

// BasicAccount : Basic information about any account.
type BasicAccount struct {
	Account
	// IsTeammate : Whether this user is a teammate of the current user. If this
	// account is the current user's account, then this will be true.
	IsTeammate bool `json:"is_teammate"`
	// TeamMemberId : The user's unique team member id. This field will only be
	// present if the user is part of a team and `is_teammate` is true.
	TeamMemberId string `json:"team_member_id,omitempty"`
}

// NewBasicAccount returns a new BasicAccount instance
func NewBasicAccount(AccountId string, Name *Name, Email string, EmailVerified bool, Disabled bool, IsTeammate bool) *BasicAccount {
	s := new(BasicAccount)
	s.AccountId = AccountId
	s.Name = Name
	s.Email = Email
	s.EmailVerified = EmailVerified
	s.Disabled = Disabled
	s.IsTeammate = IsTeammate
	return s
}

// FullAccount : Detailed information about the current user's account.
type FullAccount struct {
	Account
	// Country : The user's two-letter country code, if available. Country codes
	// are based on `ISO 3166-1` <http://en.wikipedia.org/wiki/ISO_3166-1>.
	Country string `json:"country,omitempty"`
	// Locale : The language that the user specified. Locale tags will be `IETF
	// language tags` <http://en.wikipedia.org/wiki/IETF_language_tag>.
	Locale string `json:"locale"`
	// ReferralLink : The user's `referral link`
	// <https://www.dropbox.com/referrals>.
	ReferralLink string `json:"referral_link"`
	// Team : If this account is a member of a team, information about that
	// team.
	Team *FullTeam `json:"team,omitempty"`
	// TeamMemberId : This account's unique team member id. This field will only
	// be present if `team` is present.
	TeamMemberId string `json:"team_member_id,omitempty"`
	// IsPaired : Whether the user has a personal and work account. If the
	// current account is personal, then `team` will always be nil, but
	// `is_paired` will indicate if a work account is linked.
	IsPaired bool `json:"is_paired"`
	// AccountType : What type of account this user has.
	AccountType *users_common.AccountType `json:"account_type"`
	// RootInfo : The root info for this account.
	RootInfo common.IsRootInfo `json:"root_info"`
}

// NewFullAccount returns a new FullAccount instance
func NewFullAccount(AccountId string, Name *Name, Email string, EmailVerified bool, Disabled bool, Locale string, ReferralLink string, IsPaired bool, AccountType *users_common.AccountType, RootInfo common.IsRootInfo) *FullAccount {
	s := new(FullAccount)
	s.AccountId = AccountId
	s.Name = Name
	s.Email = Email
	s.EmailVerified = EmailVerified
	s.Disabled = Disabled
	s.Locale = Locale
	s.ReferralLink = ReferralLink
	s.IsPaired = IsPaired
	s.AccountType = AccountType
	s.RootInfo = RootInfo
	return s
}

// UnmarshalJSON deserializes into a FullAccount instance
func (u *FullAccount) UnmarshalJSON(b []byte) error {
	type wrap struct {
		// AccountId : The user's unique Dropbox ID.
		AccountId string `json:"account_id"`
		// Name : Details of a user's name.
		Name *Name `json:"name"`
		// Email : The user's e-mail address. Do not rely on this without
		// checking the `email_verified` field. Even then, it's possible that
		// the user has since lost access to their e-mail.
		Email string `json:"email"`
		// EmailVerified : Whether the user has verified their e-mail address.
		EmailVerified bool `json:"email_verified"`
		// Disabled : Whether the user has been disabled.
		Disabled bool `json:"disabled"`
		// Locale : The language that the user specified. Locale tags will be
		// `IETF language tags`
		// <http://en.wikipedia.org/wiki/IETF_language_tag>.
		Locale string `json:"locale"`
		// ReferralLink : The user's `referral link`
		// <https://www.dropbox.com/referrals>.
		ReferralLink string `json:"referral_link"`
		// IsPaired : Whether the user has a personal and work account. If the
		// current account is personal, then `team` will always be nil, but
		// `is_paired` will indicate if a work account is linked.
		IsPaired bool `json:"is_paired"`
		// AccountType : What type of account this user has.
		AccountType *users_common.AccountType `json:"account_type"`
		// RootInfo : The root info for this account.
		RootInfo json.RawMessage `json:"root_info"`
		// ProfilePhotoUrl : URL for the photo representing the user, if one is
		// set.
		ProfilePhotoUrl string `json:"profile_photo_url,omitempty"`
		// Country : The user's two-letter country code, if available. Country
		// codes are based on `ISO 3166-1`
		// <http://en.wikipedia.org/wiki/ISO_3166-1>.
		Country string `json:"country,omitempty"`
		// Team : If this account is a member of a team, information about that
		// team.
		Team *FullTeam `json:"team,omitempty"`
		// TeamMemberId : This account's unique team member id. This field will
		// only be present if `team` is present.
		TeamMemberId string `json:"team_member_id,omitempty"`
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	u.AccountId = w.AccountId
	u.Name = w.Name
	u.Email = w.Email
	u.EmailVerified = w.EmailVerified
	u.Disabled = w.Disabled
	u.Locale = w.Locale
	u.ReferralLink = w.ReferralLink
	u.IsPaired = w.IsPaired
	u.AccountType = w.AccountType
	RootInfo, err := common.IsRootInfoFromJSON(w.RootInfo)
	if err != nil {
		return err
	}
	u.RootInfo = RootInfo
	u.ProfilePhotoUrl = w.ProfilePhotoUrl
	u.Country = w.Country
	u.Team = w.Team
	u.TeamMemberId = w.TeamMemberId
	return nil
}

// Team : Information about a team.
type Team struct {
	// Id : The team's unique ID.
	Id string `json:"id"`
	// Name : The name of the team.
	Name string `json:"name"`
}

// NewTeam returns a new Team instance
func NewTeam(Id string, Name string) *Team {
	s := new(Team)
	s.Id = Id
	s.Name = Name
	return s
}

// FullTeam : Detailed information about a team.
type FullTeam struct {
	Team
	// SharingPolicies : Team policies governing sharing.
	SharingPolicies *team_policies.TeamSharingPolicies `json:"sharing_policies"`
	// OfficeAddinPolicy : Team policy governing the use of the Office Add-In.
	OfficeAddinPolicy *team_policies.OfficeAddInPolicy `json:"office_addin_policy"`
}

// NewFullTeam returns a new FullTeam instance
func NewFullTeam(Id string, Name string, SharingPolicies *team_policies.TeamSharingPolicies, OfficeAddinPolicy *team_policies.OfficeAddInPolicy) *FullTeam {
	s := new(FullTeam)
	s.Id = Id
	s.Name = Name
	s.SharingPolicies = SharingPolicies
	s.OfficeAddinPolicy = OfficeAddinPolicy
	return s
}

// GetAccountArg : has no documentation (yet)
type GetAccountArg struct {
	// AccountId : A user's account identifier.
	AccountId string `json:"account_id"`
}

// NewGetAccountArg returns a new GetAccountArg instance
func NewGetAccountArg(AccountId string) *GetAccountArg {
	s := new(GetAccountArg)
	s.AccountId = AccountId
	return s
}

// GetAccountBatchArg : has no documentation (yet)
type GetAccountBatchArg struct {
	// AccountIds : List of user account identifiers.  Should not contain any
	// duplicate account IDs.
	AccountIds []string `json:"account_ids"`
}

// NewGetAccountBatchArg returns a new GetAccountBatchArg instance
func NewGetAccountBatchArg(AccountIds []string) *GetAccountBatchArg {
	s := new(GetAccountBatchArg)
	s.AccountIds = AccountIds
	return s
}

// GetAccountBatchError : has no documentation (yet)
type GetAccountBatchError struct {
	dropbox.Tagged
	// NoAccount : The value is an account ID specified in
	// `GetAccountBatchArg.account_ids` that does not exist.
	NoAccount string `json:"no_account,omitempty"`
}

// Valid tag values for GetAccountBatchError
const (
	GetAccountBatchErrorNoAccount = "no_account"
	GetAccountBatchErrorOther     = "other"
)

// UnmarshalJSON deserializes into a GetAccountBatchError instance
func (u *GetAccountBatchError) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// NoAccount : The value is an account ID specified in
		// `GetAccountBatchArg.account_ids` that does not exist.
		NoAccount string `json:"no_account,omitempty"`
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "no_account":
		u.NoAccount = w.NoAccount

		if err != nil {
			return err
		}
	}
	return nil
}

// GetAccountError : has no documentation (yet)
type GetAccountError struct {
	dropbox.Tagged
}

// Valid tag values for GetAccountError
const (
	GetAccountErrorNoAccount = "no_account"
	GetAccountErrorOther     = "other"
)

// IndividualSpaceAllocation : has no documentation (yet)
type IndividualSpaceAllocation struct {
	// Allocated : The total space allocated to the user's account (bytes).
	Allocated uint64 `json:"allocated"`
}

// NewIndividualSpaceAllocation returns a new IndividualSpaceAllocation instance
func NewIndividualSpaceAllocation(Allocated uint64) *IndividualSpaceAllocation {
	s := new(IndividualSpaceAllocation)
	s.Allocated = Allocated
	return s
}

// Name : Representations for a person's name to assist with
// internationalization.
type Name struct {
	// GivenName : Also known as a first name.
	GivenName string `json:"given_name"`
	// Surname : Also known as a last name or family name.
	Surname string `json:"surname"`
	// FamiliarName : Locale-dependent name. In the US, a person's familiar name
	// is their `given_name`, but elsewhere, it could be any combination of a
	// person's `given_name` and `surname`.
	FamiliarName string `json:"familiar_name"`
	// DisplayName : A name that can be used directly to represent the name of a
	// user's Dropbox account.
	DisplayName string `json:"display_name"`
	// AbbreviatedName : An abbreviated form of the person's name. Their
	// initials in most locales.
	AbbreviatedName string `json:"abbreviated_name"`
}

// NewName returns a new Name instance
func NewName(GivenName string, Surname string, FamiliarName string, DisplayName string, AbbreviatedName string) *Name {
	s := new(Name)
	s.GivenName = GivenName
	s.Surname = Surname
	s.FamiliarName = FamiliarName
	s.DisplayName = DisplayName
	s.AbbreviatedName = AbbreviatedName
	return s
}

// SpaceAllocation : Space is allocated differently based on the type of
// account.
type SpaceAllocation struct {
	dropbox.Tagged
	// Individual : The user's space allocation applies only to their individual
	// account.
	Individual *IndividualSpaceAllocation `json:"individual,omitempty"`
	// Team : The user shares space with other members of their team.
	Team *TeamSpaceAllocation `json:"team,omitempty"`
}

// Valid tag values for SpaceAllocation
const (
	SpaceAllocationIndividual = "individual"
	SpaceAllocationTeam       = "team"
	SpaceAllocationOther      = "other"
)

// UnmarshalJSON deserializes into a SpaceAllocation instance
func (u *SpaceAllocation) UnmarshalJSON(body []byte) error {
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
	case "individual":
		err = json.Unmarshal(body, &u.Individual)

		if err != nil {
			return err
		}
	case "team":
		err = json.Unmarshal(body, &u.Team)

		if err != nil {
			return err
		}
	}
	return nil
}

// SpaceUsage : Information about a user's space usage and quota.
type SpaceUsage struct {
	// Used : The user's total space usage (bytes).
	Used uint64 `json:"used"`
	// Allocation : The user's space allocation.
	Allocation *SpaceAllocation `json:"allocation"`
}

// NewSpaceUsage returns a new SpaceUsage instance
func NewSpaceUsage(Used uint64, Allocation *SpaceAllocation) *SpaceUsage {
	s := new(SpaceUsage)
	s.Used = Used
	s.Allocation = Allocation
	return s
}

// TeamSpaceAllocation : has no documentation (yet)
type TeamSpaceAllocation struct {
	// Used : The total space currently used by the user's team (bytes).
	Used uint64 `json:"used"`
	// Allocated : The total space allocated to the user's team (bytes).
	Allocated uint64 `json:"allocated"`
	// UserWithinTeamSpaceAllocated : The total space allocated to the user
	// within its team allocated space (0 means that no restriction is imposed
	// on the user's quota within its team).
	UserWithinTeamSpaceAllocated uint64 `json:"user_within_team_space_allocated"`
	// UserWithinTeamSpaceLimitType : The type of the space limit imposed on the
	// team member (off, alert_only, stop_sync).
	UserWithinTeamSpaceLimitType *team_common.MemberSpaceLimitType `json:"user_within_team_space_limit_type"`
}

// NewTeamSpaceAllocation returns a new TeamSpaceAllocation instance
func NewTeamSpaceAllocation(Used uint64, Allocated uint64, UserWithinTeamSpaceAllocated uint64, UserWithinTeamSpaceLimitType *team_common.MemberSpaceLimitType) *TeamSpaceAllocation {
	s := new(TeamSpaceAllocation)
	s.Used = Used
	s.Allocated = Allocated
	s.UserWithinTeamSpaceAllocated = UserWithinTeamSpaceAllocated
	s.UserWithinTeamSpaceLimitType = UserWithinTeamSpaceLimitType
	return s
}
