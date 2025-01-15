// Package api has type definitions for gofile
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
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
// gofile API, by using RFC3339
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

// Error is returned from gofile when things go wrong
type Error struct {
	Status string `json:"status"`
}

// Error returns a string for the error and satisfies the error interface
func (e Error) Error() string {
	out := fmt.Sprintf("Error %q", e.Status)
	return out
}

// IsError returns true if there is an error
func (e Error) IsError() bool {
	return e.Status != "ok"
}

// Err returns err if not nil, or e if IsError or nil
func (e Error) Err(err error) error {
	if err != nil {
		return err
	}
	if e.IsError() {
		return e
	}
	return nil
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// Types of things in Item
const (
	ItemTypeFolder = "folder"
	ItemTypeFile   = "file"
)

// Item describes a folder or a file as returned by /contents
type Item struct {
	ID            string                 `json:"id"`
	ParentFolder  string                 `json:"parentFolder"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Size          int64                  `json:"size"`
	Code          string                 `json:"code"`
	CreateTime    int64                  `json:"createTime"`
	ModTime       int64                  `json:"modTime"`
	Link          string                 `json:"link"`
	MD5           string                 `json:"md5"`
	MimeType      string                 `json:"mimetype"`
	ChildrenCount int                    `json:"childrenCount"`
	DirectLinks   map[string]*DirectLink `json:"directLinks"`
	//Public         bool     `json:"public"`
	//ServerSelected string   `json:"serverSelected"`
	//Thumbnail      string   `json:"thumbnail"`
	//DownloadCount int      `json:"downloadCount"`
	//TotalDownloadCount int64            `json:"totalDownloadCount"`
	//TotalSize int64            `json:"totalSize"`
	//ChildrenIDs   []string               `json:"childrenIds"`
	Children map[string]*Item `json:"children"`
}

// ToNativeTime converts a go time to a native time
func ToNativeTime(t time.Time) int64 {
	return t.Unix()
}

// FromNativeTime converts native time to a go time
func FromNativeTime(t int64) time.Time {
	return time.Unix(t, 0)
}

// DirectLink describes a direct link to a file so it can be
// downloaded by third parties.
type DirectLink struct {
	ExpireTime       int64  `json:"expireTime"`
	SourceIpsAllowed []any  `json:"sourceIpsAllowed"`
	DomainsAllowed   []any  `json:"domainsAllowed"`
	Auth             []any  `json:"auth"`
	IsReqLink        bool   `json:"isReqLink"`
	DirectLink       string `json:"directLink"`
}

// Contents is returned from the /contents call
type Contents struct {
	Error
	Data struct {
		Item
	} `json:"data"`
	Metadata Metadata `json:"metadata"`
}

// Metadata is returned when paging is in use
type Metadata struct {
	TotalCount  int  `json:"totalCount"`
	TotalPages  int  `json:"totalPages"`
	Page        int  `json:"page"`
	PageSize    int  `json:"pageSize"`
	HasNextPage bool `json:"hasNextPage"`
}

// AccountsGetID is the result of /accounts/getid
type AccountsGetID struct {
	Error
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Stats of storage and traffic
type Stats struct {
	FolderCount            int64 `json:"folderCount"`
	FileCount              int64 `json:"fileCount"`
	Storage                int64 `json:"storage"`
	TrafficDirectGenerated int64 `json:"trafficDirectGenerated"`
	TrafficReqDownloaded   int64 `json:"trafficReqDownloaded"`
	TrafficWebDownloaded   int64 `json:"trafficWebDownloaded"`
}

// AccountsGet is the result of /accounts/{id}
type AccountsGet struct {
	Error
	Data struct {
		ID                             string `json:"id"`
		Email                          string `json:"email"`
		Tier                           string `json:"tier"`
		PremiumType                    string `json:"premiumType"`
		Token                          string `json:"token"`
		RootFolder                     string `json:"rootFolder"`
		SubscriptionProvider           string `json:"subscriptionProvider"`
		SubscriptionEndDate            int    `json:"subscriptionEndDate"`
		SubscriptionLimitDirectTraffic int64  `json:"subscriptionLimitDirectTraffic"`
		SubscriptionLimitStorage       int64  `json:"subscriptionLimitStorage"`
		StatsCurrent                   Stats  `json:"statsCurrent"`
		// StatsHistory                   map[int]map[int]map[int]Stats `json:"statsHistory"`
	} `json:"data"`
}

// CreateFolderRequest is the input to /contents/createFolder
type CreateFolderRequest struct {
	ParentFolderID string `json:"parentFolderId"`
	FolderName     string `json:"folderName"`
	ModTime        int64  `json:"modTime,omitempty"`
}

// CreateFolderResponse is the output from /contents/createFolder
type CreateFolderResponse struct {
	Error
	Data Item `json:"data"`
}

// DeleteRequest is the input to DELETE /contents
type DeleteRequest struct {
	ContentsID string `json:"contentsId"` // comma separated list of IDs
}

// DeleteResponse is the input to DELETE /contents
type DeleteResponse struct {
	Error
	Data map[string]Error
}

// Server is an upload server
type Server struct {
	Name string `json:"name"`
	Zone string `json:"zone"`
}

// String returns a string representation of the Server
func (s *Server) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.Zone)
}

// Root returns the root URL for the server
func (s *Server) Root() string {
	return fmt.Sprintf("https://%s.gofile.io/", s.Name)
}

// URL returns the upload URL for the server
func (s *Server) URL() string {
	return fmt.Sprintf("https://%s.gofile.io/contents/uploadfile", s.Name)
}

// ServersResponse is the output from /servers
type ServersResponse struct {
	Error
	Data struct {
		Servers []Server `json:"servers"`
	} `json:"data"`
}

// UploadResponse is returned by POST /contents/uploadfile
type UploadResponse struct {
	Error
	Data Item `json:"data"`
}

// DirectLinksRequest specifies the parameters for the direct link
type DirectLinksRequest struct {
	ExpireTime       int64 `json:"expireTime,omitempty"`
	SourceIpsAllowed []any `json:"sourceIpsAllowed,omitempty"`
	DomainsAllowed   []any `json:"domainsAllowed,omitempty"`
	Auth             []any `json:"auth,omitempty"`
}

// DirectLinksResult is returned from POST /contents/{id}/directlinks
type DirectLinksResult struct {
	Error
	Data struct {
		ExpireTime       int64  `json:"expireTime"`
		SourceIpsAllowed []any  `json:"sourceIpsAllowed"`
		DomainsAllowed   []any  `json:"domainsAllowed"`
		Auth             []any  `json:"auth"`
		IsReqLink        bool   `json:"isReqLink"`
		ID               string `json:"id"`
		DirectLink       string `json:"directLink"`
	} `json:"data"`
}

// UpdateItemRequest describes the updates to be done to an item for PUT /contents/{id}/update
//
// The Value of the attribute to define :
// For Attribute "name" : The name of the content (file or folder)
// For Attribute "description" : The description displayed on the download page (folder only)
// For Attribute "tags" : A comma-separated list of tags (folder only)
// For Attribute "public" : either true or false (folder only)
// For Attribute "expiry" : A unix timestamp of the expiration date (folder only)
// For Attribute "password" : The password to set (folder only)
type UpdateItemRequest struct {
	Attribute string `json:"attribute"`
	Value     any    `json:"attributeValue"`
}

// UpdateItemResponse is returned by PUT /contents/{id}/update
type UpdateItemResponse struct {
	Error
	Data Item `json:"data"`
}

// MoveRequest is the input to /contents/move
type MoveRequest struct {
	FolderID   string `json:"folderId"`
	ContentsID string `json:"contentsId"` // comma separated list of IDs
}

// MoveResponse is returned by POST /contents/move
type MoveResponse struct {
	Error
	Data map[string]struct {
		Error
		Item `json:"data"`
	} `json:"data"`
}

// CopyRequest is the input to /contents/copy
type CopyRequest struct {
	FolderID   string `json:"folderId"`
	ContentsID string `json:"contentsId"` // comma separated list of IDs
}

// CopyResponse is returned by POST /contents/copy
type CopyResponse struct {
	Error
	Data map[string]struct {
		Error
		Item `json:"data"`
	} `json:"data"`
}

// UploadServerStatus is returned when fetching the root of an upload server
type UploadServerStatus struct {
	Error
	Data struct {
		Server string `json:"server"`
		Test   string `json:"test"`
	} `json:"data"`
}
