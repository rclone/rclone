// response_types.go - API response structures for Febbox
package feb_box

// FileItem represents a file or folder in Febbox
type FileItem struct {
	FID          int64 `json:"fid"`           // File ID
	Name         string `json:"name"`          // File name
	Size         int64  `json:"size"`          // File size in bytes
	IsDir        bool   `json:"is_dir"`        // Is it a directory
	Type         string `json:"type"`          // MIME type
	ModifiedTime string `json:"modified_time"` // Modification time as string
	CreatedTime  string `json:"created_time"`  // Creation time as string
	Thumbnail    string `json:"thumbnail"`     // Thumbnail URL (optional)
	HasPassword  bool   `json:"has_password"`  // Is password protected
	ShareKey     string `json:"share_key"`     // Share key
}

// FileListResponse response for file listing
type FileListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		ShareKey string     `json:"share_key"`
		ParentID string     `json:"parent_id"`
		Files    []FileItem `json:"files"`
		Count    int        `json:"count"`
	} `json:"data"`
}

// DownloadLink represents a single download link with quality info
type DownloadLink struct {
	URL      string `json:"url"`       // Direct download URL
	Quality  string `json:"quality"`   // Quality (e.g., "HD", "SD")
	Size     string `json:"size"`      // Human readable size
	Duration string `json:"duration"`  // Video duration
	Bitrate  string `json:"bitrate"`   // Video bitrate
}

// DownloadLinksResponse response for download links
type DownloadLinksResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		Links []DownloadLink `json:"links"`
	} `json:"data"`
}

// AccountInfoResponse response for account information
type AccountInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar"`
		Space    struct {
			Total int64 `json:"total"` // Total space in bytes
			Used  int64 `json:"used"`  // Used space in bytes
			Free  int64 `json:"free"`  // Free space in bytes
		} `json:"space"`
		Premium struct {
			IsPremium   bool   `json:"is_premium"`
			ExpiryDate  string `json:"expiry_date"`
			MaxFileSize int64  `json:"max_file_size"`
		} `json:"premium"`
	} `json:"data"`
}

// CreateFolderResponse response for folder creation
type CreateFolderResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		FID  string `json:"fid"`
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"data"`
}

// UploadResponse response for upload initiation
type UploadResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		UploadID  string `json:"upload_id"`
		ChunkSize int64  `json:"chunk_size"`
		Endpoints struct {
			Upload   string `json:"upload"`
			Complete string `json:"complete"`
			Status   string `json:"status"`
		} `json:"endpoints"`
	} `json:"data"`
}

// SearchResult represents a search result item
type SearchResult struct {
	FID     string `json:"fid"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Matches []struct {
		Field string `json:"field"`
		Text  string `json:"text"`
	} `json:"matches"`
}

// SearchResponse response for search operations
type SearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		Results []SearchResult `json:"results"`
		Total   int            `json:"total"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
	} `json:"data"`
}