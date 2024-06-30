// Package api has type definitions for pcloud
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"fmt"
	"time"
)

const (
	// Sun, 16 Mar 2014 17:26:04 +0000
	timeFormat = `"` + time.RFC1123Z + `"`
)

// Time represents date and time information for the
// pcloud API, by using RFC1123Z
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

// Error is returned from pcloud when things go wrong
//
// If result is 0 then everything is OK
type Error struct {
	Result      int    `json:"result"`
	ErrorString string `json:"error"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("pcloud error: %s (%d)", e.ErrorString, e.Result)
}

// Update returns err directly if it was != nil, otherwise it returns
// an Error or nil if no error was detected
func (e *Error) Update(err error) error {
	if err != nil {
		return err
	}
	if e.Result == 0 {
		return nil
	}
	return e
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// Item describes a folder or a file as returned by Get Folder Items and others
type Item struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Created        Time   `json:"created"`
	IsMine         bool   `json:"ismine"`
	Thumb          bool   `json:"thumb"`
	Modified       Time   `json:"modified"`
	Comments       int    `json:"comments"`
	ID             string `json:"id"`
	IsShared       bool   `json:"isshared"`
	IsDeleted      bool   `json:"isdeleted"`
	Icon           string `json:"icon"`
	IsFolder       bool   `json:"isfolder"`
	ParentFolderID int64  `json:"parentfolderid"`
	FolderID       int64  `json:"folderid,omitempty"`
	Height         int    `json:"height,omitempty"`
	FileID         int64  `json:"fileid,omitempty"`
	Width          int    `json:"width,omitempty"`
	Hash           uint64 `json:"hash,omitempty"`
	Category       int    `json:"category,omitempty"`
	Size           int64  `json:"size,omitempty"`
	ContentType    string `json:"contenttype,omitempty"`
	Contents       []Item `json:"contents"`
}

// ModTime returns the modification time of the item
func (i *Item) ModTime() (t time.Time) {
	t = time.Time(i.Modified)
	if t.IsZero() {
		t = time.Time(i.Created)
	}
	return t
}

// ItemResult is returned from the /listfolder, /createfolder, /deletefolder, /deletefile, etc. methods
type ItemResult struct {
	Error
	Metadata Item `json:"metadata"`
}

// Hashes contains the supported hashes
type Hashes struct {
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// FileTruncateResponse is the response from /file_truncate
type FileTruncateResponse struct {
	Error
}

// FileCloseResponse is the response from /file_close
type FileCloseResponse struct {
	Error
}

// FileOpenResponse is the response from /file_open
type FileOpenResponse struct {
	Error
	Fileid         int64 `json:"fileid"`
	FileDescriptor int64 `json:"fd"`
}

// FileChecksumResponse is the response from /file_checksum
type FileChecksumResponse struct {
	Error
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
	SHA256 string `json:"sha256"`
}

// FilePWriteResponse is the response from /file_pwrite
type FilePWriteResponse struct {
	Error
	Bytes int64 `json:"bytes"`
}

// UploadFileResponse is the response from /uploadfile
type UploadFileResponse struct {
	Error
	Items     []Item   `json:"metadata"`
	Checksums []Hashes `json:"checksums"`
	Fileids   []int64  `json:"fileids"`
}

// GetFileLinkResult is returned from /getfilelink
type GetFileLinkResult struct {
	Error
	Dwltag  string   `json:"dwltag"`
	Hash    uint64   `json:"hash"`
	Size    int64    `json:"size"`
	Expires Time     `json:"expires"`
	Path    string   `json:"path"`
	Hosts   []string `json:"hosts"`
}

// IsValid returns whether the link is valid and has not expired
func (g *GetFileLinkResult) IsValid() bool {
	if g == nil {
		return false
	}
	if len(g.Hosts) == 0 {
		return false
	}
	return time.Until(time.Time(g.Expires)) > 30*time.Second
}

// URL returns a URL from the Path and Hosts.  Check with IsValid
// before calling.
func (g *GetFileLinkResult) URL() string {
	// FIXME rotate the hosts?
	return "https://" + g.Hosts[0] + g.Path
}

// ChecksumFileResult is returned from /checksumfile
type ChecksumFileResult struct {
	Error
	Hashes
	Metadata Item `json:"metadata"`
}

// PubLinkResult is returned from /getfilepublink and /getfolderpublink
type PubLinkResult struct {
	Error
	LinkID   int    `json:"linkid"`
	Link     string `json:"link"`
	LinkCode string `json:"code"`
}

// UserInfo is returned from /userinfo
type UserInfo struct {
	Error
	Cryptosetup           bool   `json:"cryptosetup"`
	Plan                  int    `json:"plan"`
	CryptoSubscription    bool   `json:"cryptosubscription"`
	PublicLinkQuota       int64  `json:"publiclinkquota"`
	Email                 string `json:"email"`
	UserID                int    `json:"userid"`
	Quota                 int64  `json:"quota"`
	TrashRevretentionDays int    `json:"trashrevretentiondays"`
	Premium               bool   `json:"premium"`
	PremiumLifetime       bool   `json:"premiumlifetime"`
	EmailVerified         bool   `json:"emailverified"`
	UsedQuota             int64  `json:"usedquota"`
	Language              string `json:"language"`
	Business              bool   `json:"business"`
	CryptoLifetime        bool   `json:"cryptolifetime"`
	Registered            string `json:"registered"`
	Journey               struct {
		Claimed bool `json:"claimed"`
		Steps   struct {
			VerifyMail    bool `json:"verifymail"`
			UploadFile    bool `json:"uploadfile"`
			AutoUpload    bool `json:"autoupload"`
			DownloadApp   bool `json:"downloadapp"`
			DownloadDrive bool `json:"downloaddrive"`
		} `json:"steps"`
	} `json:"journey"`
}
