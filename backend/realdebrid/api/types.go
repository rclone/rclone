// Package api contains definitions for using the premiumize.me API
package api

import "fmt"

// Response is returned by all messages and embedded in the
// structures below
type Response struct {
	Message string `json:"message,omitempty"`
	Status  string `json:"status"`
}

// Error satisfies the error interface
func (e *Response) Error() string {
	return fmt.Sprintf("%s: %s", e.Status, e.Message)
}

// AsErr checks the status and returns an err if bad or nil if good
func (e *Response) AsErr() error {
	if e.Status != "success" {
		return e
	}
	return nil
}

// Item Types
const (
	ItemTypeFolder = "folder"
	ItemTypeFile   = "file"
)

// Item refers to a file or folder
type Item struct {
	Breadcrumbs     []Breadcrumb ``
	CreatedAt       int64        ``
	ID              string       `json:"id,omitempty"`
	ParentID        string       ``
	Link            string       `json:"download,omitempty"`
	OriginalLink    string       `json:"link,omitempty"`
	Name            string       `json:"filename,omitempty"`
	Size            int64        `json:"filesize,omitempty"`
	StreamLink      string       ``
	Type            string       `json:"type,omitempty"`
	TranscodeStatus string       ``
	IP              string       ``
	MimeType        string       `json:"mimeType,omitempty"`
	Ended           string       `json:"ended,omitempty"`
	Generated       string       `json:"generated,omitempty"`
	Links           []string     `json:"links,omitempty"`
	TorrentHash     string       `json:"hash,omitempty"`
}

// Breadcrumb is part the breadcrumb trail for a file or folder.  It
// is returned as part of folder/list if required
type Breadcrumb struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

// FolderListResponse is the response to folder/list
type FolderListResponse struct {
	Response
	Content  []Item ``                          //`json:"content"`
	Name     string `json:"filename,omitempty"` //`json:"name,omitempty"`
	ParentID string ``                          //`json:"parent_id,omitempty"`
	FolderID string ``                          //`json:"folder_id,omitempty"`
}

// FolderCreateResponse is the response to folder/create
type FolderCreateResponse struct {
	Response
	ID string `json:"id,omitempty"`
}

// FolderUploadinfoResponse is the response to folder/uploadinfo
type FolderUploadinfoResponse struct {
	Response
	Token string `json:"token,omitempty"`
	URL   string `json:"url,omitempty"`
}

// AccountInfoResponse is the response to account/info
type AccountInfoResponse struct {
	Response
	CustomerID   string  `json:"customer_id,omitempty"`
	LimitUsed    float64 `json:"limit_used,omitempty"` // fraction 0..1 of download traffic limit
	PremiumUntil int64   `json:"premium_until,omitempty"`
	SpaceUsed    float64 `json:"space_used,omitempty"`
}
