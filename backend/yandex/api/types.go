package api

import (
	"fmt"
	"strings"
)

// DiskInfo contains disk metadata
type DiskInfo struct {
	TotalSpace int64 `json:"total_space"`
	UsedSpace  int64 `json:"used_space"`
	TrashSize  int64 `json:"trash_size"`
}

// ResourceInfoRequestOptions struct
type ResourceInfoRequestOptions struct {
	SortMode *SortMode
	Limit    uint64
	Offset   uint64
	Fields   []string
}

//ResourceInfoResponse struct is returned by the API for metadata requests.
type ResourceInfoResponse struct {
	PublicKey        string                 `json:"public_key"`
	Name             string                 `json:"name"`
	Created          string                 `json:"created"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
	Preview          string                 `json:"preview"`
	PublicURL        string                 `json:"public_url"`
	OriginPath       string                 `json:"origin_path"`
	Modified         string                 `json:"modified"`
	Path             string                 `json:"path"`
	Md5              string                 `json:"md5"`
	ResourceType     string                 `json:"type"`
	MimeType         string                 `json:"mime_type"`
	Size             int64                  `json:"size"`
	Embedded         *ResourceListResponse  `json:"_embedded"`
}

// ResourceListResponse struct
type ResourceListResponse struct {
	Sort      *SortMode              `json:"sort"`
	PublicKey string                 `json:"public_key"`
	Items     []ResourceInfoResponse `json:"items"`
	Path      string                 `json:"path"`
	Limit     *uint64                `json:"limit"`
	Offset    *uint64                `json:"offset"`
	Total     *uint64                `json:"total"`
}

// AsyncInfo struct is returned by the API for various async operations.
type AsyncInfo struct {
	HRef      string `json:"href"`
	Method    string `json:"method"`
	Templated bool   `json:"templated"`
}

// AsyncStatus is returned when requesting the status of an async operations. Possible values in-progress, success, failure
type AsyncStatus struct {
	Status string `json:"status"`
}

//CustomPropertyResponse struct we send and is returned by the API for CustomProperty request.
type CustomPropertyResponse struct {
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// SortMode struct - sort mode
type SortMode struct {
	mode string
}

// Default - sort mode
func (m *SortMode) Default() *SortMode {
	return &SortMode{
		mode: "",
	}
}

// ByName - sort mode
func (m *SortMode) ByName() *SortMode {
	return &SortMode{
		mode: "name",
	}
}

// ByPath - sort mode
func (m *SortMode) ByPath() *SortMode {
	return &SortMode{
		mode: "path",
	}
}

// ByCreated - sort mode
func (m *SortMode) ByCreated() *SortMode {
	return &SortMode{
		mode: "created",
	}
}

// ByModified - sort mode
func (m *SortMode) ByModified() *SortMode {
	return &SortMode{
		mode: "modified",
	}
}

// BySize - sort mode
func (m *SortMode) BySize() *SortMode {
	return &SortMode{
		mode: "size",
	}
}

// Reverse - sort mode
func (m *SortMode) Reverse() *SortMode {
	if strings.HasPrefix(m.mode, "-") {
		return &SortMode{
			mode: m.mode[1:],
		}
	}
	return &SortMode{
		mode: "-" + m.mode,
	}
}

func (m *SortMode) String() string {
	return m.mode
}

// UnmarshalJSON sort mode
func (m *SortMode) UnmarshalJSON(value []byte) error {
	if len(value) == 0 {
		m.mode = ""
		return nil
	}
	m.mode = string(value)
	if strings.HasPrefix(m.mode, "\"") && strings.HasSuffix(m.mode, "\"") {
		m.mode = m.mode[1 : len(m.mode)-1]
	}
	return nil
}

// ErrorResponse represents erroneous API response.
// Implements go's built in `error`.
type ErrorResponse struct {
	ErrorName   string `json:"error"`
	Description string `json:"description"`
	Message     string `json:"message"`

	StatusCode int `json:""`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("[%d - %s] %s (%s)", e.StatusCode, e.ErrorName, e.Description, e.Message)
}
