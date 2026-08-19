// Package api defines Quark Drive API request and response types.
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Response contains the fields shared by Quark Drive responses.
type Response struct {
	Status  int    `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Check returns an error when the API did not report success.
func (r Response) Check() error {
	if r.Code == 0 && (r.Status == 0 || r.Status == 200) {
		return nil
	}
	return fmt.Errorf("quark API error: status=%d code=%d message=%q", r.Status, r.Code, r.Message)
}

// Error describes a non-2xx HTTP response from Quark Drive.
type Error struct {
	Response
	HTTPStatus int `json:"-"`
}

// Error returns a readable API error.
func (e *Error) Error() string {
	return fmt.Sprintf("quark HTTP error: http_status=%d status=%d code=%d message=%q", e.HTTPStatus, e.Status, e.Code, e.Message)
}

// Item describes a file or directory returned by the file APIs.
type Item struct {
	FID          string `json:"fid"`
	ParentFID    string `json:"pdir_fid"`
	FileName     string `json:"file_name"`
	Size         int64  `json:"size"`
	Dir          bool   `json:"dir"`
	File         bool   `json:"file"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	LCreatedAt   int64  `json:"l_created_at"`
	LUpdatedAt   int64  `json:"l_updated_at"`
	MD5          string `json:"md5"`
	SHA1         string `json:"sha1"`
	FormatType   string `json:"format_type"`
	DownloadURL  string `json:"download_url"`
	FileType     int    `json:"file_type"`
	Category     int    `json:"category"`
	UpdatedAtSec int64  `json:"updated_at_sec"`
}

// Name returns the API file name.
func (i Item) Name() string { return i.FileName }

// IsDir reports whether the item is a directory.
func (i Item) IsDir() bool { return i.Dir }

// ModTime returns the best modification time supplied by the API.
func (i Item) ModTime() time.Time {
	switch {
	case i.LUpdatedAt > 0:
		return time.UnixMilli(i.LUpdatedAt)
	case i.UpdatedAt > 0:
		return time.UnixMilli(i.UpdatedAt)
	case i.UpdatedAtSec > 0:
		return time.Unix(i.UpdatedAtSec, 0)
	case i.CreatedAt > 0:
		return time.UnixMilli(i.CreatedAt)
	case i.LCreatedAt > 0:
		return time.UnixMilli(i.LCreatedAt)
	default:
		return time.Time{}
	}
}

// ListResponse is returned by the directory listing endpoint.
type ListResponse struct {
	Response
	Data struct {
		List []Item `json:"list"`
	} `json:"data"`
}

// IDResponse is returned by simple file operations.
type IDResponse struct {
	Response
	Data struct {
		FID string `json:"fid"`
	} `json:"data"`
}

// UploadPreResponse describes an initialized OSS multipart upload.
type UploadPreResponse struct {
	Response
	Data struct {
		AuthInfo  json.RawMessage `json:"auth_info"`
		Bucket    string          `json:"bucket"`
		Callback  json.RawMessage `json:"callback"`
		FID       string          `json:"fid"`
		Finish    bool            `json:"finish"`
		ObjKey    string          `json:"obj_key"`
		TaskID    string          `json:"task_id"`
		UploadID  string          `json:"upload_id"`
		UploadURL string          `json:"upload_url"`
	} `json:"data"`
	Metadata struct {
		PartSize   int64 `json:"part_size"`
		PartThread int   `json:"part_thread"`
	} `json:"metadata"`
}

// UploadHashResponse reports whether a server-side hash match completed an upload.
type UploadHashResponse struct {
	Response
	Data struct {
		Finish bool   `json:"finish"`
		FID    string `json:"fid"`
	} `json:"data"`
}

// UploadAuthResponse contains an OSS authorization value.
type UploadAuthResponse struct {
	Response
	Data struct {
		AuthKey string `json:"auth_key"`
	} `json:"data"`
}

// UploadFinishResponse describes the completed file.
type UploadFinishResponse struct {
	Response
	Data Item `json:"data"`
}

// DownloadItem contains a signed file download URL.
type DownloadItem struct {
	FID         string `json:"fid"`
	DownloadURL string `json:"download_url"`
}

// DownloadResponse retains the polymorphic download response data.
type DownloadResponse struct {
	Response
	Data json.RawMessage `json:"data"`
}

// DownloadTask describes an asynchronous download request.
type DownloadTask struct {
	TaskID   string `json:"task_id"`
	TaskSync bool   `json:"task_sync"`
	TaskResp *struct {
		Data []DownloadItem `json:"data"`
	} `json:"task_resp"`
}

// TaskResponse is returned while polling asynchronous operations.
type TaskResponse struct {
	Response
	Data struct {
		Status      int            `json:"status"`
		FID         string         `json:"fid"`
		ShareID     string         `json:"share_id"`
		DownloadURL string         `json:"download_url"`
		Data        []DownloadItem `json:"data"`
	} `json:"data"`
}

// MemberResponse contains account capacity information.
type MemberResponse struct {
	Response
	Data struct {
		Used       int64  `json:"use_capacity"`
		Total      int64  `json:"total_capacity"`
		MemberType string `json:"member_type"`
	} `json:"data"`
}

// AccountResponse is returned by the account information endpoint.
type AccountResponse struct {
	Success bool           `json:"success"`
	Code    string         `json:"code"`
	Message string         `json:"msg"`
	Data    map[string]any `json:"data"`
}

// ShareResponse describes share creation, which may be asynchronous.
type ShareResponse struct {
	Response
	Data struct {
		TaskID   string `json:"task_id"`
		TaskSync bool   `json:"task_sync"`
		TaskResp *struct {
			Data struct {
				Status  int    `json:"status"`
				ShareID string `json:"share_id"`
			} `json:"data"`
		} `json:"task_resp"`
	} `json:"data"`
}

// SharePasswordResponse contains the final public URL.
type SharePasswordResponse struct {
	Response
	Data struct {
		ShareURL string `json:"share_url"`
	} `json:"data"`
}

// ShareEntry describes one public link owned by the account.
type ShareEntry struct {
	ShareID  string `json:"share_id"`
	FirstFID string `json:"first_fid"`
	Status   int    `json:"status"`
}

// ShareListResponse is returned by the account public-link listing endpoint.
type ShareListResponse struct {
	Response
	Data struct {
		List []ShareEntry `json:"list"`
	} `json:"data"`
}

// QRResponse is returned by both QR login endpoints.
type QRResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Members struct {
			Token         string `json:"token"`
			ServiceTicket string `json:"service_ticket"`
		} `json:"members"`
	} `json:"data"`
}
