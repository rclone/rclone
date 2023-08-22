package api

import (
	"strconv"
	"time"
)

// OverwriteOnCopyMode is a conflict resolve mode during copy. Files with conflicting names will be owerwritten
const OverwriteOnCopyMode = "overwrite"

// ProfileInfo is a profile info about quota
type ProfileInfo struct {
	UserUsed  int64 `json:"user_used"`
	UserLimit int64 `json:"user_limit"`
	AccUsed   int64 `json:"acc_used"`
	AccLimit  int64 `json:"acc_limit"`
}

type IDList struct {
	IDs []string `json:"ids"`
}

type DeleteParams struct {
	IDs               []string `json:"ids"`
	DeletePermanently bool     `json:"delete_permanently"`
}

type FileInfoParams struct {
	ParentID string `json:"parent_id,omitempty"`
	Path     string `json:"path"`
}

type FileInfo struct {
	FileID   string `json:"file_id"`
	ParentID string `json:"parent_id"`
	Src      string `json:"src"`
	Type     string `json:"type"`
}

func (fi *FileInfo) IsFile() bool {
	if fi == nil {
		return false
	}

	return fi.Type == "F"
}

func (fi *FileInfo) IsDir() bool {
	if fi == nil {
		return false
	}

	return fi.Type == "D" || fi.Type == "S" || fi.Type == "T"
}

type CreateDirParams struct {
	Target  string `json:"target,omitempty"`
	Name    string `json:"name"`
	Resolve bool   `json:"resolve"`
}

type File struct {
	ID         string   `json:"id"`
	Created    JSONTime `json:"created"`
	Modified   JSONTime `json:"modified"`
	Name       string   `json:"name"`
	ParentID   string   `json:"parent_id"`
	Size       int64    `json:"size"`
	ModifiedMS JSONTime `json:"modified_ms"`
	Type       string   `json:"type"`
	Operations int      `json:"operations"`
	SubType    string   `json:"sub_type"`
	Content    []File   `json:"content"`
}

func (f *File) IsFile() bool {
	if f == nil {
		return false
	}

	return f.Type == "F"
}

func (f *File) IsDir() bool {
	if f == nil {
		return false
	}

	return f.Type == "D" || f.Type == "S" || f.Type == "T"
}

type SetMTimeParams struct {
	ID    string   `json:"id,omitempty"`
	MTime JSONTime `json:"mtime"`
}

type JSONTime time.Time

// MarshalJSON returns time representation in custom format
func (t JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(time.Time(t).UTC().UnixNano())/1e9, 'f', 6, 64)), nil
}

func (u *JSONTime) UnmarshalJSON(data []byte) error {
	f, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}

	t := JSONTime(time.Unix(0, int64(f*1e9)))
	*u = t

	return nil
}

func (t JSONTime) String() string {
	return strconv.FormatInt(time.Time(t).UTC().Unix(), 10)
}

type DownloadLinkResponse struct {
	ID string `json:"id"`
}

type UploadLinkParams struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
	Resolve  bool   `json:"resolve"`
}

type UploadLinkResponse struct {
	Name      string `json:"name"`
	FileID    string `json:"file_id"`
	ParentID  string `json:"parent_id"`
	UploadKey string `json:"upload_key"`
}

type UploadFinalizeResponse struct {
	FileID   string `json:"id"`
	ParentID string `json:"parent_id"`
	Modified int64  `json:"modified"`
	FileSize int64  `json:"size"`
}

type FileModifyParams struct {
	ID       string `json:"id"`
	Truncate int64  `json:"truncate"`
}

type FileCopyMoveParams struct {
	IDs     []string `json:"ids"`
	Target  string   `json:"target"`
	Resolve bool     `json:"resolve"`
}

type FileCopyMoveOneParams struct {
	ID          string   `json:"file_id"`
	Target      string   `json:"target_id"`
	Name        string   `json:"name"`
	MTime       JSONTime `json:"mtime"`
	Resolve     bool     `json:"resolve"`
	ResolveMode string   `json:"resolve_mode"`
}

type FileRenameParams struct {
	Name    string `json:"name"`
	Resolve bool   `json:"resolve"`
}

type JobResponse struct {
	JobID string `json:"job_id"`
}

type JobResult struct {
	State string `json:"state"`
}
