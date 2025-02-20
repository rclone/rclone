package api

import (
	"fmt"
	"time"
)

const (
	// 2017-05-03T07:26:10-07:00
	timeFormat = `"` + time.RFC3339 + `"`
)

// Time represents date and time information for the
// box API, by using RFC3339
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

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q (%v)", e.Message, e.Code)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

type AuthorizeRequest struct {
	ClientID     string  `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
	GrantType    string  `json:"grant_type"`
	Code         *string `json:"code"`
	RefreshToken *string `json:"refresh_token"`
	CodeVerifier *string `json:"code_verifier"`
}

type Token struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Expiry returns expiry from expires in, so it should be called on retrieval
// e must be non-nil.
func (e *Token) Expiry() (t time.Time) {
	if v := e.ExpiresIn; v != 0 {
		return time.Now().Add(time.Duration(v) * time.Second)
	}
	return
}

// UserInfo represents the user information returned by the /oauth/users/info endpoint
type UserInfo struct {
	ID     string `json:"id"`     // User ID, unique identifier
	Name   string `json:"name"`   // Nickname, returns default nickname if not set
	Avatar string `json:"avatar"` // Avatar URL, empty if not set
	Phone  string `json:"phone"`  // Phone number, requires user:phone permission
}

// DriveInfo represents the user and drive information returned by the /adrive/v1.0/user/getDriveInfo endpoint
type DriveInfo struct {
	UserID          string `json:"user_id"`           // User ID, unique identifier
	Name            string `json:"name"`              // Nickname
	Avatar          string `json:"avatar"`            // Avatar URL
	DefaultDriveID  string `json:"default_drive_id"`  // Default drive ID
	ResourceDriveID string `json:"resource_drive_id"` // Resource library drive ID, only returned if authorized
	BackupDriveID   string `json:"backup_drive_id"`   // Backup drive ID, only returned if authorized
	FolderID        string `json:"folder_id"`         // Folder ID, only returned if authorized
}

// PersonalSpaceInfo represents the user's personal space usage information
type PersonalSpaceInfo struct {
	UsedSize  int64 `json:"used_size"`  // Used space in bytes
	TotalSize int64 `json:"total_size"` // Total space in bytes
}

// SpaceInfo represents the response from /adrive/v1.0/user/getSpaceInfo endpoint
type SpaceInfo struct {
	PersonalSpaceInfo PersonalSpaceInfo `json:"personal_space_info"` // Personal space usage information
}

// VipIdentity represents the VIP status of a user
type VipIdentity string

const (
	VipIdentityMember VipIdentity = "member" // Regular member
	VipIdentityVIP    VipIdentity = "vip"    // VIP member
	VipIdentitySVIP   VipIdentity = "svip"   // Super VIP member
)

// VipInfo represents the response from /business/v1.0/user/getVipInfo endpoint
type VipInfo struct {
	Identity            VipIdentity `json:"identity"`            // VIP status: member, vip, or svip
	Level               string      `json:"level,omitempty"`     // Storage level (e.g., "20TB", "8TB")
	Expire              int64       `json:"expire"`              // Expiration timestamp in seconds
	ThirdPartyVip       bool        `json:"thirdPartyVip"`       // Whether third-party VIP benefits are active
	ThirdPartyVipExpire int64       `json:"thirdPartyVipExpire"` // Third-party VIP benefits expiration timestamp
}

// Scope represents a single permission scope
type Scope struct {
	Scope string `json:"scope"` // Permission scope identifier
}

// UserScopes represents the response from /oauth/users/scopes endpoint
type UserScopes struct {
	ID     string  `json:"id"`     // User ID
	Scopes []Scope `json:"scopes"` // List of permission scopes
}

// TrialStatus represents the trial status of a VIP feature
type TrialStatus string

const (
	TrialStatusNoTrial    TrialStatus = "noTrial"    // Trial not allowed
	TrialStatusOnTrial    TrialStatus = "onTrial"    // Trial in progress
	TrialStatusEndTrial   TrialStatus = "endTrial"   // Trial ended
	TrialStatusAllowTrial TrialStatus = "allowTrial" // Trial allowed but not started
)

// FeatureCode represents the available VIP features
type FeatureCode string

const (
	FeatureCode1080p     FeatureCode = "hd.1080p"      // 1080p HD feature
	FeatureCode1080pPlus FeatureCode = "hd.1080p.plus" // 1440p HD feature
)

// VipFeature represents a single VIP feature with its trial status
type VipFeature struct {
	Code           FeatureCode `json:"code"`           // Feature identifier
	Intercept      bool        `json:"intercept"`      // Whether the feature is intercepted
	TrialStatus    TrialStatus `json:"trialStatus"`    // Current trial status
	TrialDuration  int64       `json:"trialDuration"`  // Trial duration in minutes
	TrialStartTime int64       `json:"trialStartTime"` // Trial start timestamp
}

// VipFeatureList represents the response from /business/v1.0/vip/feature/list endpoint
type VipFeatureList struct {
	Result []VipFeature `json:"result"` // List of VIP features
}

// VipFeatureTrialRequest represents the request body for /business/v1.0/vip/feature/trial endpoint
type VipFeatureTrialRequest struct {
	FeatureCode FeatureCode `json:"featureCode"` // Feature code to start trial for
}

// VipFeatureTrialResponse represents the response from /business/v1.0/vip/feature/trial endpoint
type VipFeatureTrialResponse struct {
	TrialStatus    TrialStatus `json:"trialStatus"`    // Current trial status
	TrialDuration  int64       `json:"trialDuration"`  // Trial duration in minutes
	TrialStartTime int64       `json:"trialStartTime"` // Trial start timestamp
}

// DriveFile represents a file in a specific drive
type DriveFile struct {
	DriveID string `json:"drive_id"` // Drive ID
	FileID  string `json:"file_id"`  // File ID
}

// CreateFastTransferRequest represents the request body for /adrive/v1.0/openFile/createFastTransfer endpoint
type CreateFastTransferRequest struct {
	DriveFileList []DriveFile `json:"drive_file_list"` // List of drive files to share [1,100]
}

// CreateFastTransferResponse represents the response from /adrive/v1.0/openFile/createFastTransfer endpoint
type CreateFastTransferResponse struct {
	Expiration    Time        `json:"expiration"`      // Fast transfer expiration time
	CreatorID     string      `json:"creator_id"`      // ID of the fast transfer creator
	ShareID       string      `json:"share_id"`        // Share ID
	ShareURL      string      `json:"share_url"`       // Share URL
	DriveFileList []DriveFile `json:"drive_file_list"` // List of shared drive files
}

// FileType represents the type of a file
type FileType string

const (
	FileTypeFile   FileType = "file"   // Regular file
	FileTypeFolder FileType = "folder" // Directory/folder
)

// FileCategory represents the category of a file
type FileCategory string

const (
	FileCategoryVideo  FileCategory = "video"  // Video files
	FileCategoryDoc    FileCategory = "doc"    // Document files
	FileCategoryAudio  FileCategory = "audio"  // Audio files
	FileCategoryZip    FileCategory = "zip"    // Archive files
	FileCategoryOthers FileCategory = "others" // Other files
	FileCategoryImage  FileCategory = "image"  // Image files
)

// OrderDirection represents the sort order direction
type OrderDirection string

const (
	OrderDirectionASC  OrderDirection = "ASC"  // Ascending order
	OrderDirectionDESC OrderDirection = "DESC" // Descending order
)

// OrderBy represents the field to sort by
type OrderBy string

const (
	OrderByCreatedAt    OrderBy = "created_at"    // Sort by creation time
	OrderByUpdatedAt    OrderBy = "updated_at"    // Sort by update time
	OrderByName         OrderBy = "name"          // Sort by name
	OrderBySize         OrderBy = "size"          // Sort by size
	OrderByNameEnhanced OrderBy = "name_enhanced" // Sort by name with enhanced number handling
)

// VideoMediaMetadata represents video file metadata
type VideoMediaMetadata struct {
	// Add video metadata fields as needed
}

// VideoPreviewMetadata represents video preview metadata
type VideoPreviewMetadata struct {
	// Add video preview metadata fields as needed
}

// FileItem represents a file or folder item in the drive
type FileItem struct {
	DriveID              string                `json:"drive_id"`                         // Drive ID
	FileID               string                `json:"file_id"`                          // File ID
	ParentFileID         string                `json:"parent_file_id"`                   // Parent folder ID
	Name                 string                `json:"name"`                             // File name
	Size                 int64                 `json:"size"`                             // File size in bytes
	FileExtension        string                `json:"file_extension"`                   // File extension
	ContentHash          string                `json:"content_hash"`                     // File content hash
	Category             FileCategory          `json:"category"`                         // File category
	Type                 FileType              `json:"type"`                             // File type (file/folder)
	Thumbnail            string                `json:"thumbnail,omitempty"`              // Thumbnail URL
	URL                  string                `json:"url,omitempty"`                    // Preview/download URL for files under 5MB
	CreatedAt            Time                  `json:"created_at"`                       // Creation time
	UpdatedAt            Time                  `json:"updated_at"`                       // Last update time
	PlayCursor           string                `json:"play_cursor,omitempty"`            // Playback progress
	VideoMediaMetadata   *VideoMediaMetadata   `json:"video_media_metadata,omitempty"`   // Video metadata
	VideoPreviewMetadata *VideoPreviewMetadata `json:"video_preview_metadata,omitempty"` // Video preview metadata
}

// ListFileRequest represents the request body for /adrive/v1.0/openFile/list endpoint
type ListFileRequest struct {
	DriveID             string         `json:"drive_id"`                        // Drive ID
	Limit               int            `json:"limit,omitempty"`                 // Max items to return (default 50, max 100)
	Marker              string         `json:"marker,omitempty"`                // Pagination marker
	OrderBy             OrderBy        `json:"order_by,omitempty"`              // Sort field
	OrderDirection      OrderDirection `json:"order_direction,omitempty"`       // Sort direction
	ParentFileID        string         `json:"parent_file_id"`                  // Parent folder ID (root for root folder)
	Category            string         `json:"category,omitempty"`              // File categories (comma-separated)
	Type                FileType       `json:"type,omitempty"`                  // Filter by type
	VideoThumbnailTime  int64          `json:"video_thumbnail_time,omitempty"`  // Video thumbnail timestamp (ms)
	VideoThumbnailWidth int            `json:"video_thumbnail_width,omitempty"` // Video thumbnail width
	ImageThumbnailWidth int            `json:"image_thumbnail_width,omitempty"` // Image thumbnail width
	Fields              string         `json:"fields,omitempty"`                // Fields to return
}

// ListFileResponse represents the response from file listing endpoints
type ListFileResponse struct {
	Items      []FileItem `json:"items"`                 // List of files/folders
	NextMarker string     `json:"next_marker,omitempty"` // Next page marker
}

// SearchFileRequest represents the request body for /adrive/v1.0/openFile/search endpoint
type SearchFileRequest struct {
	DriveID             string `json:"drive_id"`                        // Drive ID
	Limit               int    `json:"limit,omitempty"`                 // Max items to return (default 100, max 100)
	Marker              string `json:"marker,omitempty"`                // Pagination marker
	Query               string `json:"query"`                           // Search query
	OrderBy             string `json:"order_by,omitempty"`              // Sort order
	VideoThumbnailTime  int64  `json:"video_thumbnail_time,omitempty"`  // Video thumbnail timestamp (ms)
	VideoThumbnailWidth int    `json:"video_thumbnail_width,omitempty"` // Video thumbnail width
	ImageThumbnailWidth int    `json:"image_thumbnail_width,omitempty"` // Image thumbnail width
	ReturnTotalCount    bool   `json:"return_total_count,omitempty"`    // Whether to return total count
}

// SearchFileResponse represents the response from the search endpoint
type SearchFileResponse struct {
	Items      []FileItem `json:"items"`                 // Search results
	NextMarker string     `json:"next_marker,omitempty"` // Next page marker
	TotalCount int64      `json:"total_count,omitempty"` // Total number of matches
}

// StarredFileRequest represents the request body for /adrive/v1.0/openFile/starredList endpoint
type StarredFileRequest struct {
	DriveID             string         `json:"drive_id"`                        // Drive ID
	Limit               int            `json:"limit,omitempty"`                 // Max items to return (default 100, max 100)
	Marker              string         `json:"marker,omitempty"`                // Pagination marker
	OrderBy             OrderBy        `json:"order_by,omitempty"`              // Sort field
	OrderDirection      OrderDirection `json:"order_direction,omitempty"`       // Sort direction
	Type                FileType       `json:"type,omitempty"`                  // Filter by type
	VideoThumbnailTime  int64          `json:"video_thumbnail_time,omitempty"`  // Video thumbnail timestamp (ms)
	VideoThumbnailWidth int            `json:"video_thumbnail_width,omitempty"` // Video thumbnail width
	ImageThumbnailWidth int            `json:"image_thumbnail_width,omitempty"` // Image thumbnail width
}

// GetFileRequest represents the request body for /adrive/v1.0/openFile/get endpoint
type GetFileRequest struct {
	DriveID             string `json:"drive_id"`                        // Drive ID
	FileID              string `json:"file_id"`                         // File ID
	VideoThumbnailTime  int64  `json:"video_thumbnail_time,omitempty"`  // Video thumbnail timestamp (ms)
	VideoThumbnailWidth int    `json:"video_thumbnail_width,omitempty"` // Video thumbnail width
	ImageThumbnailWidth int    `json:"image_thumbnail_width,omitempty"` // Image thumbnail width
	Fields              string `json:"fields,omitempty"`                // Specific fields to return (comma-separated)
}

// GetFileByPathRequest represents the request body for /adrive/v1.0/openFile/get_by_path endpoint
type GetFileByPathRequest struct {
	DriveID  string `json:"drive_id"`  // Drive ID
	FilePath string `json:"file_path"` // File path (e.g., /folder/file.txt)
}

// BatchGetFileRequest represents the request body for /adrive/v1.0/openFile/batch/get endpoint
type BatchGetFileRequest struct {
	FileList            []DriveFile `json:"file_list"`                       // List of files to get details for
	VideoThumbnailTime  int64       `json:"video_thumbnail_time,omitempty"`  // Video thumbnail timestamp (ms)
	VideoThumbnailWidth int         `json:"video_thumbnail_width,omitempty"` // Video thumbnail width
	ImageThumbnailWidth int         `json:"image_thumbnail_width,omitempty"` // Image thumbnail width
}

// BatchGetFileResponse represents the response from the batch get endpoint
type BatchGetFileResponse struct {
	Items []FileItem `json:"items"` // List of file details
}

// FileDetailExtended represents a file with additional path information
type FileDetailExtended struct {
	FileItem
	IDPath   string `json:"id_path,omitempty"`   // Path using IDs (e.g., root:/64de0fb2...)
	NamePath string `json:"name_path,omitempty"` // Path using names (e.g., root:/folder/file.txt)
}

// GetDownloadURLRequest represents the request body for /adrive/v1.0/openFile/getDownloadUrl endpoint
type GetDownloadURLRequest struct {
	DriveID   string `json:"drive_id"`             // Drive ID
	FileID    string `json:"file_id"`              // File ID
	ExpireSec int64  `json:"expire_sec,omitempty"` // URL expiration time in seconds (default 900, max 14400 for premium apps)
}

// GetDownloadURLResponse represents the response from the download URL endpoint
type GetDownloadURLResponse struct {
	URL         string `json:"url"`         // Download URL
	Expiration  Time   `json:"expiration"`  // URL expiration time
	Method      string `json:"method"`      // Download method
	Description string `json:"description"` // Additional information about download speed and privileges
}

// CheckNameMode represents how to handle naming conflicts
type CheckNameMode string

const (
	CheckNameModeAutoRename CheckNameMode = "auto_rename" // Automatically rename if file exists
	CheckNameModeRefuse     CheckNameMode = "refuse"      // Don't create if file exists
	CheckNameModeIgnore     CheckNameMode = "ignore"      // Create even if file exists
)

// PartInfo represents information about a file part for multipart upload
type PartInfo struct {
	PartNumber int    `json:"part_number"`          // Part sequence number (1-based)
	UploadURL  string `json:"upload_url,omitempty"` // Upload URL for this part
	PartSize   int64  `json:"part_size,omitempty"`  // Size of this part
	Etag       string `json:"etag,omitempty"`       // ETag returned after part upload
}

// StreamInfo represents stream information for special file formats (e.g., livp)
type StreamInfo struct {
	ContentHash     string     `json:"content_hash,omitempty"`      // Content hash
	ContentHashName string     `json:"content_hash_name,omitempty"` // Hash algorithm name
	ProofVersion    string     `json:"proof_version,omitempty"`     // Proof version
	ProofCode       string     `json:"proof_code,omitempty"`        // Proof code
	ContentMD5      string     `json:"content_md5,omitempty"`       // Content MD5
	PreHash         string     `json:"pre_hash,omitempty"`          // Pre-hash for large files
	Size            int64      `json:"size,omitempty"`              // Stream size
	PartInfoList    []PartInfo `json:"part_info_list,omitempty"`    // Part information list
}

// CreateFileRequest represents the request body for /adrive/v1.0/openFile/create endpoint
type CreateFileRequest struct {
	DriveID         string        `json:"drive_id"`                    // Drive ID
	ParentFileID    string        `json:"parent_file_id"`              // Parent folder ID (root for root directory)
	Name            string        `json:"name"`                        // File name (UTF-8, max 1024 bytes)
	Type            FileType      `json:"type"`                        // File type (file/folder)
	CheckNameMode   CheckNameMode `json:"check_name_mode"`             // How to handle name conflicts
	PartInfoList    []PartInfo    `json:"part_info_list,omitempty"`    // Part information for multipart upload (max 10000)
	StreamsInfo     []StreamInfo  `json:"streams_info,omitempty"`      // Stream information (for special formats)
	PreHash         string        `json:"pre_hash,omitempty"`          // First 1KB SHA1 for quick duplicate check
	Size            int64         `json:"size,omitempty"`              // File size in bytes
	ContentHash     string        `json:"content_hash,omitempty"`      // Full file content hash
	ContentHashName string        `json:"content_hash_name,omitempty"` // Hash algorithm (default: sha1)
	ProofCode       string        `json:"proof_code,omitempty"`        // Proof code for duplicate check
	ProofVersion    string        `json:"proof_version,omitempty"`     // Proof version (fixed: v1)
	LocalCreatedAt  *Time         `json:"local_created_at,omitempty"`  // Local creation time
	LocalModifiedAt *Time         `json:"local_modified_at,omitempty"` // Local modification time
}

// CreateFileResponse represents the response from the file creation endpoint
type CreateFileResponse struct {
	DriveID      string     `json:"drive_id"`       // Drive ID
	FileID       string     `json:"file_id"`        // File ID
	FileName     string     `json:"file_name"`      // File name
	ParentFileID string     `json:"parent_file_id"` // Parent folder ID
	Status       string     `json:"status"`         // Status
	UploadID     string     `json:"upload_id"`      // Upload ID (empty for folders)
	Available    bool       `json:"available"`      // Whether the file is available
	Exist        bool       `json:"exist"`          // Whether a file with same name exists
	RapidUpload  bool       `json:"rapid_upload"`   // Whether rapid upload was used
	PartInfoList []PartInfo `json:"part_info_list"` // Part information list
}

// GetUploadURLRequest represents the request body for /adrive/v1.0/openFile/getUploadUrl endpoint
type GetUploadURLRequest struct {
	DriveID      string     `json:"drive_id"`       // Drive ID
	FileID       string     `json:"file_id"`        // File ID
	UploadID     string     `json:"upload_id"`      // Upload ID from file creation
	PartInfoList []PartInfo `json:"part_info_list"` // Part information list
}

// GetUploadURLResponse represents the response from the upload URL endpoint
type GetUploadURLResponse struct {
	DriveID      string     `json:"drive_id"`       // Drive ID
	FileID       string     `json:"file_id"`        // File ID
	UploadID     string     `json:"upload_id"`      // Upload ID
	CreatedAt    Time       `json:"created_at"`     // Creation time
	PartInfoList []PartInfo `json:"part_info_list"` // Part information with URLs
}

// ListUploadedPartsRequest represents the request body for /adrive/v1.0/openFile/listUploadedParts endpoint
type ListUploadedPartsRequest struct {
	DriveID          string `json:"drive_id"`                     // Drive ID
	FileID           string `json:"file_id"`                      // File ID
	UploadID         string `json:"upload_id"`                    // Upload ID
	PartNumberMarker string `json:"part_number_marker,omitempty"` // Marker for pagination
}

// ListUploadedPartsResponse represents the response from the list uploaded parts endpoint
type ListUploadedPartsResponse struct {
	DriveID              string     `json:"drive_id"`                // Drive ID
	UploadID             string     `json:"upload_id"`               // Upload ID
	ParallelUpload       bool       `json:"parallelUpload"`          // Whether parallel upload is enabled
	UploadedParts        []PartInfo `json:"uploaded_parts"`          // List of uploaded parts
	NextPartNumberMarker string     `json:"next_part_number_marker"` // Marker for next page
}

// CompleteUploadRequest represents the request body for /adrive/v1.0/openFile/complete endpoint
type CompleteUploadRequest struct {
	DriveID  string `json:"drive_id"`  // Drive ID
	FileID   string `json:"file_id"`   // File ID
	UploadID string `json:"upload_id"` // Upload ID
}

// CompleteUploadResponse represents the response from the complete upload endpoint
type CompleteUploadResponse struct {
	DriveID       string       `json:"drive_id"`               // Drive ID
	FileID        string       `json:"file_id"`                // File ID
	Name          string       `json:"name"`                   // File name
	Size          int64        `json:"size"`                   // File size
	FileExtension string       `json:"file_extension"`         // File extension
	ContentHash   string       `json:"content_hash"`           // Content hash
	Category      FileCategory `json:"category"`               // File category
	Type          FileType     `json:"type"`                   // File type
	Thumbnail     string       `json:"thumbnail,omitempty"`    // Thumbnail URL
	URL           string       `json:"url,omitempty"`          // Preview URL
	DownloadURL   string       `json:"download_url,omitempty"` // Download URL
	CreatedAt     Time         `json:"created_at"`             // Creation time
	UpdatedAt     Time         `json:"updated_at"`             // Last update time
}

// UpdateFileRequest represents the request body for /adrive/v1.0/openFile/update endpoint
type UpdateFileRequest struct {
	DriveID       string        `json:"drive_id"`                  // Drive ID
	FileID        string        `json:"file_id"`                   // File ID
	Name          string        `json:"name,omitempty"`            // New file name
	CheckNameMode CheckNameMode `json:"check_name_mode,omitempty"` // How to handle name conflicts
	Starred       *bool         `json:"starred,omitempty"`         // Whether to star/unstar the file
}

// UpdateFileResponse represents the response from the file update endpoint
type UpdateFileResponse struct {
	DriveID       string       `json:"drive_id"`       // Drive ID
	FileID        string       `json:"file_id"`        // File ID
	Name          string       `json:"name"`           // File name
	Size          int64        `json:"size"`           // File size
	FileExtension string       `json:"file_extension"` // File extension
	ContentHash   string       `json:"content_hash"`   // Content hash
	Category      FileCategory `json:"category"`       // File category
	Type          FileType     `json:"type"`           // File type (file/folder)
	CreatedAt     Time         `json:"created_at"`     // Creation time
	UpdatedAt     Time         `json:"updated_at"`     // Last update time
}

// MoveFileRequest represents the request body for /adrive/v1.0/openFile/move endpoint
type MoveFileRequest struct {
	DriveID        string        `json:"drive_id"`                  // Current drive ID
	FileID         string        `json:"file_id"`                   // File ID to move
	ToDriveID      string        `json:"to_drive_id,omitempty"`     // Target drive ID (defaults to current drive_id)
	ToParentFileID string        `json:"to_parent_file_id"`         // Target parent folder ID (root for root directory)
	CheckNameMode  CheckNameMode `json:"check_name_mode,omitempty"` // How to handle name conflicts (default: refuse)
	NewName        string        `json:"new_name,omitempty"`        // New name to use if there's a conflict
}

// MoveFileResponse represents the response from the file move endpoint
type MoveFileResponse struct {
	DriveID     string `json:"drive_id"`                // Drive ID
	FileID      string `json:"file_id"`                 // File ID
	AsyncTaskID string `json:"async_task_id,omitempty"` // Async task ID for folder moves
	Exist       bool   `json:"exist"`                   // Whether file already exists in target
}

// CopyFileRequest represents the request body for /adrive/v1.0/openFile/copy endpoint
type CopyFileRequest struct {
	DriveID        string `json:"drive_id"`              // Drive ID
	FileID         string `json:"file_id"`               // File ID to copy
	ToDriveID      string `json:"to_drive_id,omitempty"` // Target drive ID (defaults to current drive_id)
	ToParentFileID string `json:"to_parent_file_id"`     // Target parent folder ID (root for root directory)
	AutoRename     bool   `json:"auto_rename,omitempty"` // Whether to auto rename on conflict (default: false)
}

// CopyFileResponse represents the response from the file copy endpoint
type CopyFileResponse struct {
	DriveID     string `json:"drive_id"`                // Drive ID
	FileID      string `json:"file_id"`                 // File ID
	AsyncTaskID string `json:"async_task_id,omitempty"` // Async task ID for folder copies
}

// TaskState represents the state of an async task
type TaskState string

const (
	TaskStateSucceed TaskState = "Succeed" // Task completed successfully
	TaskStateRunning TaskState = "Running" // Task is still running
	TaskStateFailed  TaskState = "Failed"  // Task failed
)

// TrashFileRequest represents the request body for /adrive/v1.0/openFile/recyclebin/trash endpoint
type TrashFileRequest struct {
	DriveID string `json:"drive_id"` // Drive ID
	FileID  string `json:"file_id"`  // File ID to move to trash
}

// TrashFileResponse represents the response from the trash file endpoint
type TrashFileResponse struct {
	DriveID     string `json:"drive_id"`                // Drive ID
	FileID      string `json:"file_id"`                 // File ID
	AsyncTaskID string `json:"async_task_id,omitempty"` // Async task ID for folder operations
}

// DeleteFileRequest represents the request body for /adrive/v1.0/openFile/delete endpoint
type DeleteFileRequest struct {
	DriveID string `json:"drive_id"` // Drive ID
	FileID  string `json:"file_id"`  // File ID to delete
}

// DeleteFileResponse represents the response from the delete file endpoint
type DeleteFileResponse struct {
	DriveID     string `json:"drive_id"`                // Drive ID
	FileID      string `json:"file_id"`                 // File ID
	AsyncTaskID string `json:"async_task_id,omitempty"` // Async task ID for folder operations
}

// GetAsyncTaskRequest represents the request body for /adrive/v1.0/openFile/async_task/get endpoint
type GetAsyncTaskRequest struct {
	AsyncTaskID string `json:"async_task_id"` // Async task ID to query
}

// GetAsyncTaskResponse represents the response from the get async task endpoint
type GetAsyncTaskResponse struct {
	State       TaskState `json:"state"`         // Task state (Succeed/Running/Failed)
	AsyncTaskID string    `json:"async_task_id"` // Async task ID
}
