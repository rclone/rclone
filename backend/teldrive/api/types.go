// Package api provides types used by the Teldrive API.
package api

import "time"

type Error struct {
	Code    bool   `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e Error) Error() string {
	out := "api error"
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

type Part struct {
	Id    int64
	Size  int64
	Name  string
	Start int64
	End   int64
}

// FileInfo represents a file when listing folder contents
type FileInfo struct {
	Id       string    `json:"id"`
	Name     string    `json:"name"`
	MimeType string    `json:"mimeType"`
	Size     int64     `json:"size"`
	ParentId string    `json:"parentId"`
	Type     string    `json:"type"`
	ModTime  time.Time `json:"updatedAt"`
}

type Meta struct {
	Count       int `json:"count,omitempty"`
	TotalPages  int `json:"totalPages,omitempty"`
	CurrentPage int `json:"currentPage,omitempty"`
}

type ReadMetadataResponse struct {
	Files []FileInfo `json:"items"`
	Meta  Meta       `json:"meta"`
}

// MetadataRequestOptions represents all the options when listing folder contents
type MetadataRequestOptions struct {
	Page  int64
	Limit int64
}

type CreateDirRequest struct {
	Path string `json:"path"`
}

type PartFile struct {
	Name       string `json:"name"`
	PartId     int    `json:"partId"`
	PartNo     int    `json:"partNo"`
	TotalParts int    `json:"totalParts"`
	Size       int64  `json:"size"`
	ChannelID  int64  `json:"channelId"`
	Encrypted  bool   `json:"encrypted"`
	Salt       string `json:"salt"`
}

type FilePart struct {
	ID   int    `json:"id"`
	Salt string `json:"salt,omitempty"`
}

type CreateFileRequest struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Path      string     `json:"path,omitempty"`
	MimeType  string     `json:"mimeType,omitempty"`
	Size      int64      `json:"size,omitempty"`
	ChannelID int64      `json:"channelId,omitempty"`
	Encrypted bool       `json:"encrypted,omitempty"`
	Parts     []FilePart `json:"parts,omitempty"`
	ParentId  string     `json:"parentId,omitempty"`
	ModTime   time.Time  `json:"updatedAt,omitempty"`
}

type MoveFileRequest struct {
	Destination     string   `json:"destinationParent,omitempty"`
	DestinationLeaf string   `json:"destinationName,omitempty"`
	Files           []string `json:"ids,omitempty"`
}
type DirMove struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type UpdateFileInformation struct {
	Name      string     `json:"name,omitempty"`
	ModTime   *time.Time `json:"updatedAt,omitempty"`
	Parts     []FilePart `json:"parts,omitempty"`
	Size      int64      `json:"size,omitempty"`
	UploadId  string     `json:"uploadId,omitempty"`
	ChannelID int64      `json:"channelId,omitempty"`
	ParentID  string     `json:"parentId,omitempty"`
}

type RemoveFileRequest struct {
	Source string   `json:"source,omitempty"`
	Files  []string `json:"ids,omitempty"`
}
type CopyFile struct {
	Newname     string    `json:"newName"`
	Destination string    `json:"destination"`
	ModTime     time.Time `json:"updatedAt,omitempty"`
}

type Session struct {
	UserName string `json:"userName"`
	UserId   int64  `json:"userId"`
	Hash     string `json:"hash"`
}

type FileShare struct {
	ID        string     `json:"id,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type CategorySize struct {
	Size int64 `json:"totalSize"`
}

type EventSource struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ParentId     string `json:"parentId"`
	DestParentId string `json:"destParentId"`
}

type Event struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	CreatedAt time.Time   `json:"createdAt"`
	Source    EventSource `json:"source"`
}
