// Package api contains types for the FebBox API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ID is a FebBox file or directory identifier.
type ID string

// UnmarshalJSON accepts identifiers encoded as strings or numbers.
func (id *ID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*id = ID(value)
		return nil
	}
	var value json.Number
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid FebBox ID: %w", err)
	}
	*id = ID(value.String())
	return nil
}

// Response is the common FebBox response envelope.
type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// Token contains OAuth client-credentials tokens.
type Token struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
}

// Item describes a file or directory.
type Item struct {
	FID        ID     `json:"fid"`
	ParentID   ID     `json:"parent_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	Extension  string `json:"ext"`
	Hash       string `json:"hash"`
	HashType   string `json:"hash_type"`
	UpdateTime int64  `json:"update_time"`
	AddTime    int64  `json:"add_time"`
	IsDir      int    `json:"is_dir"`
}

// FileList contains one page of directory entries.
type FileList struct {
	Files []Item `json:"file_list"`
}

// Download describes a generated download URL.
type Download struct {
	FID         ID     `json:"fid"`
	DownloadURL string `json:"download_url"`
	Error       int    `json:"error"`
}

// CreateDir contains the created directory identifier.
type CreateDir struct {
	FID      ID     `json:"fid"`
	ParentID ID     `json:"pid"`
	FileName string `json:"file_name"`
}

// AddURL contains an asynchronous URL ingestion task identifier.
type AddURL struct {
	TaskID ID `json:"task_id"`
}
