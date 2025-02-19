package api

import (
	"fmt"
	"time"
)

const (
	// 2017-05-03T07:26:10-07:00
	timeFormat = `"` + time.RFC3339 + `"`
)

// Time represents date and time information for the
// box API, by using RFC3339
type Time time.Time

// MarshalJSON turns a Time into JSON (in UTC)
func (t *Time) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).Format(timeFormat)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	newT, err := time.Parse(timeFormat, string(data))
	if err != nil {
		return err
	}
	*t = Time(newT)
	return nil
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q (%s)", e.Message, e.Code)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

type AuthorizeRequest struct {
	ClientID     string  `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
	GrantType    string  `json:"grant_type"`
	Code         *string `json:"code"`
	RefreshToken *string `json:"refresh_token"`
	CodeVerifier *string `json:"code_verifier"`
}

type Token struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Expiry returns expiry from expires in, so it should be called on retrieval
// e must be non-nil.
func (e *Token) Expiry() (t time.Time) {
	if v := e.ExpiresIn; v != 0 {
		return time.Now().Add(time.Duration(v) * time.Second)
	}
	return
}

type User struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Avatar string  `json:"avatar"`
	Phone  *string `json:"phone"`
}

type DriveInfo struct {
	UserID          string  `json:"user_id"`
	Name            string  `json:"name"`
	Avatar          string  `json:"avatar"`
	DefaultDriveID  string  `json:"default_drive_id" default:"drive"`
	ResourceDriveID *string `json:"resource_drive_id,omitempty"`
	BackupDriveID   *string `json:"backup_drive_id,omitempty"`
	FolderID        *string `json:"folder_id,omitempty"`
}

type SpaceInfo struct {
	UsedSize  int64 `json:"used_size"`
	TotalSize int64 `json:"total_size"`
}

type VipInfo struct {
	Identity            string    `json:"identity"`
	Level               *string   `json:"level,omitempty"`
	Expire              time.Time `json:"expire"`
	ThirdPartyVip       bool      `json:"third_party_vip"`
	ThirdPartyVipExpire *string   `json:"third_party_vip_expire,omitempty"`
}

type Item struct {
	DriveID       string    `json:"drive_id"`
	FileID        string    `json:"file_id"`
	ParentFileID  string    `json:"parent_file_id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	FileExtension string    `json:"file_extension"`
	ContentHash   string    `json:"content_hash"`
	Category      string    `json:"category"`
	Type          string    `json:"type"`
	Thumbnail     *string   `json:"thumbnail,omitempty"`
	URL           *string   `json:"url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type List struct {
	Items      []Item  `json:"items"`
	NextMarker *string `json:"next_marker"`
}

type ListRequest struct {
	DriveID      string `json:"drive_id"`
	ParentFileID string `json:"parent_file_id"`
}

type DeleteFile struct {
	DriveID string `json:"drive_id"`
	FileID  string `json:"file_id"`
}

type FileMoveCopy struct {
	DriveID        string  `json:"drive_id"`
	FileID         string  `json:"file_id"`
	ToParentFileID string  `json:"to_parent_file_id"`
	CheckNameMode  *string `json:"check_name_mode"`
	NewName        *string `json:"new_name"`
}

type FileCopyResp struct {
	DriveID     string `json:"drive_id"`
	FileID      string `json:"file_id"`
	AsyncTaskID string `json:"async_task_id"`
	Exist       bool   `json:"exist"`
}

type CreateFolder struct {
	DriveID       string `json:"drive_id"`
	ParentFileID  string `json:"parent_file_id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	CheckNameMode string `json:"check_name_mode"`
}
