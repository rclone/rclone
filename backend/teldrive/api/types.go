// Package api provides types used by the Teldrive API.
package api

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
	Id       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	ParentId string `json:"parentId"`
	Type     string `json:"type"`
	ModTime  string `json:"updatedAt"`
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
	Path      string     `json:"path"`
	MimeType  string     `json:"mimeType"`
	Size      int64      `json:"size"`
	ChannelID int64      `json:"channelId"`
	Encrypted bool       `json:"encrypted"`
	Parts     []FilePart `json:"parts"`
	CreatedAt string     `json:"createdAt,omitempty"`
	UpdatedAt string     `json:"updatedAt,omitempty"`
}

type MoveFileRequest struct {
	Files       []string `json:"files"`
	Destination string   `json:"destination"`
}
type DirMove struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type UpdateFileInformation struct {
	Name      string     `json:"name,omitempty"`
	UpdatedAt string     `json:"updatedAt,omitempty"`
	Parts     []FilePart `json:"parts,omitempty"`
	Size      int64      `json:"size,omitempty"`
	UploadId  string     `json:"uploadId,omitempty"`
	ChannelID int64      `json:"channelId,omitempty"`
}

type RemoveFileRequest struct {
	Source string   `json:"source,omitempty"`
	Files  []string `json:"files,omitempty"`
}
type CopyFile struct {
	ID          string `json:"id"`
	Newname     string `json:"newName"`
	Destination string `json:"destination"`
}

type Session struct {
	UserName string `json:"userName"`
	UserId   int64  `json:"userId"`
	Hash     string `json:"hash"`
}
