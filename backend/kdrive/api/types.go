// Package api has type definitions for kDrive
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"fmt"
	"strconv"
	"time"
)

const (
	// Sun, 16 Mar 2014 17:26:04 +0000
	timeFormat = `"` + time.RFC1123Z + `"`
)

// Time represents date and time information for the
// kdrive API, by using RFC1123Z
type Time time.Time

// MarshalJSON turns a Time into JSON (in UTC)
func (t *Time) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).Format(timeFormat)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	timestamp, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	newT := time.Unix(timestamp, 0)
	*t = Time(newT)
	return nil
}

// Error is returned from kdrive when things go wrong
//
// If result is 0 then everything is OK
type ResultStatus struct {
	Status      string `json:"result"`
	ErrorDetail struct {
		Result      string `json:"code"`
		ErrorString string `json:"description"`
		Errors      []struct {
			Result      string `json:"code"`
			ErrorString string `json:"description"`
		} `json:"errors"`
	} `json:"error"`
}

// Error returns a string for the error and satisfies the error interface
func (e *ResultStatus) Error() string {
	var details string
	for i := range e.ErrorDetail.Errors {
		details += "|" + e.ErrorDetail.Errors[i].Result
	}

	return fmt.Sprintf("kDrive error: %s (%s: %s %s)", e.Status, e.ErrorDetail.Result, e.ErrorDetail.ErrorString, details)
}

// IsError returns true if there is an error
func (e ResultStatus) IsError() bool {
	return e.Status != "success"
}

// Update returns err directly if it was != nil, otherwise it returns
// an Error or nil if no error was detected
func (e *ResultStatus) Update(err error) error {
	if err != nil {
		return err
	}
	if e.IsError() {
		return e
	}
	return nil
}

// Check ResultStatus satisfies the error interface
var _ error = (*ResultStatus)(nil)

// Profile describes a profile, as returned by the "/profile" root API
type Profile struct {
	ResultStatus
	Data struct {
		ID                                int    `json:"id"`
		UserID                            int    `json:"user_id"`
		Login                             string `json:"login"`
		Firstname                         string `json:"firstname"`
		Lastname                          string `json:"lastname"`
		DisplayName                       string `json:"display_name"`
		DateLastChangePassword            int    `json:"date_last_change_password"`
		Otp                               bool   `json:"otp"`
		Sms                               bool   `json:"sms"`
		SmsPhone                          any    `json:"sms_phone"`
		Yubikey                           bool   `json:"yubikey"`
		InfomaniakApplication             bool   `json:"infomaniak_application"`
		DoubleAuth                        bool   `json:"double_auth"`
		DoubleAuthMethod                  string `json:"double_auth_method"`
		RemainingRescueCode               int    `json:"remaining_rescue_code"`
		SecurityAssistant                 int    `json:"security_assistant"`
		SecurityCheck                     bool   `json:"security_check"`
		OpenRenewalWarrantyInvoiceGroupID []any  `json:"open_renewal_warranty_invoice_group_id"`
		AuthDevices                       []any  `json:"auth_devices"`
		ValidatedAt                       any    `json:"validated_at"`
		LastLoginAt                       int    `json:"last_login_at"`
		AdministrationLastLoginAt         int    `json:"administration_last_login_at"`
		InvalidEmail                      bool   `json:"invalid_email"`
		Avatar                            string `json:"avatar"`
		Locale                            string `json:"locale"`
		LanguageID                        int    `json:"language_id"`
		Timezone                          string `json:"timezone"`
		Country                           struct {
			ID      int    `json:"id"`
			Short   string `json:"short"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"country"`
		UnsuccessfulConnexionLimit        bool `json:"unsuccessful_connexion_limit"`
		UnsuccessfulConnexionRateLimit    int  `json:"unsuccessful_connexion_rate_limit"`
		UnsuccessfulConnexionNotification bool `json:"unsuccessful_connexion_notification"`
		SuccessfulConnexionNotification   bool `json:"successful_connexion_notification"`
		CurrentAccountID                  int  `json:"current_account_id"`
	} `json:"data"`
}

// ListDrives describes a list of available drives for a user
type ListDrives struct {
	ResultStatus
	Data []struct {
		ID               int    `json:"id"`
		DisplayName      string `json:"display_name"`
		FirstName        string `json:"first_name"`
		LastName         string `json:"last_name"`
		Email            string `json:"email"`
		IsSso            bool   `json:"is_sso"`
		Avatar           string `json:"avatar"`
		DeletedAt        any    `json:"deleted_at"`
		DriveID          int    `json:"drive_id"`
		DriveName        string `json:"drive_name"`
		AccountID        int    `json:"account_id"`
		CreatedAt        int    `json:"created_at"`
		UpdatedAt        int    `json:"updated_at"`
		LastConnectionAt int    `json:"last_connection_at"`
		ProductID        int    `json:"product_id"`
		Status           string `json:"status"`
		Role             string `json:"role"`
		Type             string `json:"type"`
		Preference       struct {
			Color       string `json:"color"`
			Hide        bool   `json:"hide"`
			Default     bool   `json:"default"`
			DefaultPage string `json:"default_page"`
		} `json:"preference"`
	} `json:"data"`
}

// Item describes a folder or a file as returned by Get Folder Items and others
type Item struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	FullPath       string `json:"path"`
	Status         string `json:"status"`
	Hash           string `json:"hash"`
	Size           int64  `json:"size"`
	Visibility     string `json:"visibility"`
	DriveID        int    `json:"drive_id"`
	Depth          int    `json:"depth"`
	CreatedBy      int    `json:"created_by"`
	CreatedAt      Time   `json:"created_at"`
	AddedAt        int    `json:"added_at"`
	LastModifiedAt Time   `json:"last_modified_at"`
	LastModifiedBy int    `json:"last_modified_by"`
	RevisedAt      int    `json:"revised_at"`
	UpdatedAt      int    `json:"updated_at"`
	MimeType       string `json:"mime_type"`
	ParentID       int    `json:"parent_id"`
	Color          string `json:"color"`
}

type SearchResult struct {
	ResultStatus
	Data       []Item `json:"data"`
	Cursor     string `json:"cursor"`
	HasMore    bool   `json:"has_more"`
	ResponseAt int    `json:"response_at"`
}

// ModTime returns the modification time of the item
func (i *Item) ModTime() (t time.Time) {
	t = time.Time(i.LastModifiedAt)
	if t.IsZero() {
		t = time.Time(i.CreatedAt)
	}
	return t
}

type CancelResource struct {
	CancelID   string `json:"cancel_id"`
	ValidUntil int    `json:"valid_until"`
}

type CancellableResponse struct {
	ResultStatus
	Data CancelResource `json:"data"`
}

type CreateDirResult struct {
	ResultStatus
	Data Item `json:"data"`
}

type FileCopyResponse struct {
	ResultStatus
	Data Item `json:"data"`
}

type UploadFileResponse struct {
	ResultStatus
	Data Item `json:"data"`
}

type ChecksumFileResult struct {
	ResultStatus
	Data struct {
		Hash string `json:"hash"`
	} `json:"data"`
}

// currently used, as PublicLink is disabled
type PubLinkResult struct {
	ResultStatus
	Data struct {
		URL          string `json:"url"`
		FileID       int    `json:"file_id"`
		Right        string `json:"right"`
		ValidUntil   int    `json:"valid_until"`
		CreatedBy    int    `json:"created_by"`
		CreatedAt    int    `json:"created_at"`
		UpdatedAt    int    `json:"updated_at"`
		Capabilities struct {
			CanEdit          bool `json:"can_edit"`
			CanSeeStats      bool `json:"can_see_stats"`
			CanSeeInfo       bool `json:"can_see_info"`
			CanDownload      bool `json:"can_download"`
			CanComment       bool `json:"can_comment"`
			CanRequestAccess bool `json:"can_request_access"`
		} `json:"capabilities"`
		AccessBlocked bool `json:"access_blocked"`
	} `json:"data"`
}

type QuotaInfo struct {
	ResultStatus
	Data struct {
		Size     int64 `json:"size"`
		UsedSize int64 `json:"used_size"`
	} `json:"data"`
}
