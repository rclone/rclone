// Package api has type definitions and code related to API-calls for the HiDrive-API.
package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Time represents date and time information for the API.
type Time time.Time

// MarshalJSON turns Time into JSON (in Unix-time/UTC).
func (t *Time) MarshalJSON() ([]byte, error) {
	secs := time.Time(*t).Unix()
	return []byte(strconv.FormatInt(secs, 10)), nil
}

// UnmarshalJSON turns JSON into Time.
func (t *Time) UnmarshalJSON(data []byte) error {
	secs, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*t = Time(time.Unix(secs, 0))
	return nil
}

// Error is returned from the API when things go wrong.
type Error struct {
	Code        json.Number `json:"code"`
	ContextInfo json.RawMessage
	Message     string `json:"msg"`
}

// Error returns a string for the error and satisfies the error interface.
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q", e.Code.String())
	if e.Message != "" {
		out += ": " + e.Message
	}
	if e.ContextInfo != nil {
		out += fmt.Sprintf(" (%+v)", e.ContextInfo)
	}
	return out
}

// Check Error satisfies the error interface.
var _ error = (*Error)(nil)

// possible types for HiDriveObject
const (
	HiDriveObjectTypeDirectory = "dir"
	HiDriveObjectTypeFile      = "file"
	HiDriveObjectTypeSymlink   = "symlink"
)

// HiDriveObject describes a folder, a symlink or a file.
// Depending on the type and content, not all fields are present.
type HiDriveObject struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	ParentID     string `json:"parent_id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	MemberCount  int64  `json:"nmembers"`
	ModifiedAt   Time   `json:"mtime"`
	ChangedAt    Time   `json:"ctime"`
	MetaHash     string `json:"mhash"`
	MetaOnlyHash string `json:"mohash"`
	NameHash     string `json:"nhash"`
	ContentHash  string `json:"chash"`
	IsTeamfolder bool   `json:"teamfolder"`
	Readable     bool   `json:"readable"`
	Writable     bool   `json:"writable"`
	Shareable    bool   `json:"shareable"`
	MIMEType     string `json:"mime_type"`
}

// ModTime returns the modification time of the HiDriveObject.
func (i *HiDriveObject) ModTime() time.Time {
	t := time.Time(i.ModifiedAt)
	if t.IsZero() {
		t = time.Time(i.ChangedAt)
	}
	return t
}

// UnmarshalJSON turns JSON into HiDriveObject and
// introduces specific default-values where necessary.
func (i *HiDriveObject) UnmarshalJSON(data []byte) error {
	type objectAlias HiDriveObject
	defaultObject := objectAlias{
		Size:        -1,
		MemberCount: -1,
	}

	err := json.Unmarshal(data, &defaultObject)
	if err != nil {
		return err
	}
	name, err := url.PathUnescape(defaultObject.Name)
	if err == nil {
		defaultObject.Name = name
	}

	*i = HiDriveObject(defaultObject)
	return nil
}

// DirectoryContent describes the content of a directory.
type DirectoryContent struct {
	TotalCount int64           `json:"nmembers"`
	Entries    []HiDriveObject `json:"members"`
}

// UnmarshalJSON turns JSON into DirectoryContent and
// introduces specific default-values where necessary.
func (d *DirectoryContent) UnmarshalJSON(data []byte) error {
	type directoryContentAlias DirectoryContent
	defaultDirectoryContent := directoryContentAlias{
		TotalCount: -1,
	}

	err := json.Unmarshal(data, &defaultDirectoryContent)
	if err != nil {
		return err
	}

	*d = DirectoryContent(defaultDirectoryContent)
	return nil
}
