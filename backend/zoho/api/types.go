// Package api provides types used by the Zoho API.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Time represents date and time information for Zoho
// Zoho uses milliseconds since unix epoch (Java currentTimeMillis)
type Time time.Time

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	s := string(data)
	// If the time is a quoted string, strip quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	millis, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*t = Time(time.Unix(0, millis*int64(time.Millisecond)))
	return nil
}

// OAuthUser is a Zoho user we are only interested in the ZUID here
type OAuthUser struct {
	FirstName   string `json:"First_Name"`
	Email       string `json:"Email"`
	LastName    string `json:"Last_Name"`
	DisplayName string `json:"Display_Name"`
	ZUID        int64  `json:"ZUID"`
}

// UserInfoResponse is returned by the user info API.
type UserInfoResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"users"`
		Attributes struct {
			EmailID string `json:"email_id"`
			Edition string `json:"edition"`
		} `json:"attributes"`
	} `json:"data"`
}

// PrivateSpaceInfo gives basic information about a users private folder.
type PrivateSpaceInfo struct {
	Data struct {
		ID   string `json:"id"`
		Type string `json:"string"`
	} `json:"data"`
}

// CurrentTeamInfo gives information about the current user in a team.
type CurrentTeamInfo struct {
	Data struct {
		ID   string `json:"id"`
		Type string `json:"string"`
	}
}

// TeamWorkspace represents a Zoho Team, Workspace or Private Space
// It's actually a VERY large json object that differs between
// Team and Workspace and Private Space but we are only interested in some fields
// that all of them have so we can use the same struct.
type TeamWorkspace struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name    string `json:"name"`
		Created Time   `json:"created_time_in_millisecond"`
		IsPart  bool   `json:"is_partof"`
	} `json:"attributes"`
}

// TeamWorkspaceResponse is the response by the list teams API, list workspace API
// or list team private spaces API.
type TeamWorkspaceResponse struct {
	TeamWorkspace []TeamWorkspace `json:"data"`
}

// Item is may represent a file or a folder in Zoho Workdrive
type Item struct {
	ID         string `json:"id"`
	Attributes struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		IsFolder     bool   `json:"is_folder"`
		CreatedTime  Time   `json:"created_time_in_millisecond"`
		ModifiedTime Time   `json:"modified_time_in_millisecond"`
		UploadedTime Time   `json:"uploaded_time_in_millisecond"`
		StorageInfo  struct {
			Size        int64 `json:"size_in_bytes"`
			FileCount   int64 `json:"files_count"`
			FolderCount int64 `json:"folders_count"`
		} `json:"storage_info"`
	} `json:"attributes"`
}

// ItemInfo contains a single Zoho Item
type ItemInfo struct {
	Item Item `json:"data"`
}

// Links contains Cursor information
type Links struct {
	Cursor struct {
		HasNext bool   `json:"has_next"`
		Next    string `json:"next"`
	} `json:"cursor"`
}

// ItemList contains multiple Zoho Items
type ItemList struct {
	Links Links  `json:"links"`
	Items []Item `json:"data"`
}

// UploadFileInfo is what the FileInfo field in the UnloadInfo struct decodes to
type UploadFileInfo struct {
	OrgID           string `json:"ORG_ID"`
	ResourceID      string `json:"RESOURCE_ID"`
	LibraryID       string `json:"LIBRARY_ID"`
	Md5Checksum     string `json:"MD5_CHECKSUM"`
	ParentModelID   string `json:"PARENT_MODEL_ID"`
	ParentID        string `json:"PARENT_ID"`
	ResourceType    int    `json:"RESOURCE_TYPE"`
	WmsSentTime     string `json:"WMS_SENT_TIME"`
	TabID           string `json:"TAB_ID"`
	Owner           string `json:"OWNER"`
	ResourceGroup   string `json:"RESOURCE_GROUP"`
	ParentModelName string `json:"PARENT_MODEL_NAME"`
	Size            int64  `json:"size"`
	Operation       string `json:"OPERATION"`
	EventID         string `json:"EVENT_ID"`
	AuditInfo       struct {
		VersionInfo struct {
			VersionAuthors    []string `json:"versionAuthors"`
			VersionID         string   `json:"versionId"`
			IsMinorVersion    bool     `json:"isMinorVersion"`
			VersionTime       Time     `json:"versionTime"`
			VersionAuthorZuid []string `json:"versionAuthorZuid"`
			VersionNotes      string   `json:"versionNotes"`
			VersionNumber     string   `json:"versionNumber"`
		} `json:"versionInfo"`
		Resource struct {
			Owner            string `json:"owner"`
			CreatedTime      Time   `json:"created_time"`
			Creator          string `json:"creator"`
			ServiceType      int    `json:"service_type"`
			Extension        string `json:"extension"`
			StatusChangeTime Time   `json:"status_change_time"`
			ResourceType     int    `json:"resource_type"`
			Name             string `json:"name"`
		} `json:"resource"`
		ParentInfo struct {
			ParentName string `json:"parentName"`
			ParentID   string `json:"parentId"`
			ParentType int    `json:"parentType"`
		} `json:"parentInfo"`
		LibraryInfo struct {
			LibraryName string `json:"libraryName"`
			LibraryID   string `json:"libraryId"`
			LibraryType int    `json:"libraryType"`
		} `json:"libraryInfo"`
		UpdateType string `json:"updateType"`
		StatusCode string `json:"statusCode"`
	} `json:"AUDIT_INFO"`
	ZUID   int64  `json:"ZUID"`
	TeamID string `json:"TEAM_ID"`
}

// GetModTime fetches the modification time of the upload
//
// This tries a few places and if all fails returns the current time
func (ufi *UploadFileInfo) GetModTime() Time {
	if t := ufi.AuditInfo.Resource.CreatedTime; !time.Time(t).IsZero() {
		return t
	}
	if t := ufi.AuditInfo.Resource.StatusChangeTime; !time.Time(t).IsZero() {
		return t
	}
	return Time(time.Now())
}

// UploadInfo is a simplified and slightly different version of
// the Item struct only used in the response to uploads
type UploadInfo struct {
	Attributes struct {
		ParentID    string `json:"parent_id"`
		FileName    string `json:"notes.txt"`
		RessourceID string `json:"resource_id"`
		Permalink   string `json:"Permalink"`
		FileInfo    string `json:"File INFO"` // JSON encoded UploadFileInfo
	} `json:"attributes"`
}

// GetUploadFileInfo decodes the embedded FileInfo
func (ui *UploadInfo) GetUploadFileInfo() (*UploadFileInfo, error) {
	var ufi UploadFileInfo
	err := json.Unmarshal([]byte(ui.Attributes.FileInfo), &ufi)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FileInfo: %w", err)
	}
	return &ufi, nil
}

// LargeUploadInfo is once again a slightly different version of UploadInfo
// returned as part of an LargeUploadResponse by the large file upload API.
type LargeUploadInfo struct {
	Attributes struct {
		ParentID    string `json:"parent_id"`
		FileName    string `json:"file_name"`
		RessourceID string `json:"resource_id"`
		FileInfo    string `json:"file_info"`
	} `json:"attributes"`
}

// GetUploadFileInfo decodes the embedded FileInfo
func (ui *LargeUploadInfo) GetUploadFileInfo() (*UploadFileInfo, error) {
	var ufi UploadFileInfo
	err := json.Unmarshal([]byte(ui.Attributes.FileInfo), &ufi)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FileInfo: %w", err)
	}
	return &ufi, nil
}

// UploadResponse is the response to a file Upload
type UploadResponse struct {
	Uploads []UploadInfo `json:"data"`
}

// LargeUploadResponse is the response returned by large file upload API.
type LargeUploadResponse struct {
	Uploads []LargeUploadInfo `json:"data"`
	Status  string            `json:"status"`
}

// WriteMetadataRequest is used to write metadata for a
// single item
type WriteMetadataRequest struct {
	Data WriteMetadata `json:"data"`
}

// WriteMultiMetadataRequest can be used to write metadata for
// multiple items at once but we don't use it that way
type WriteMultiMetadataRequest struct {
	Meta []WriteMetadata `json:"data"`
}

// WriteMetadata is used to write item metadata
type WriteMetadata struct {
	Attributes WriteAttributes `json:"attributes,omitempty"`
	ID         string          `json:"id,omitempty"`
	Type       string          `json:"type"`
}

// WriteAttributes is used to set various attributes for on items
// this is used for Move, Copy, Delete, Rename
type WriteAttributes struct {
	Name        string `json:"name,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	RessourceID string `json:"resource_id,omitempty"`
	Status      string `json:"status,omitempty"`
}
