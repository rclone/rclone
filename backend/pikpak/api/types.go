// Package api has type definitions for pikpak
//
// Converted from the API docs with help from https://mholt.github.io/json-to-go/
package api

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

const (
	// "2022-09-17T14:31:06.056+08:00"
	timeFormat = `"` + time.RFC3339 + `"`
)

// Time represents date and time information for the pikpak API, by using RFC3339
type Time time.Time

// MarshalJSON turns a Time into JSON (in UTC)
func (t *Time) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).Format(timeFormat)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Time
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	newT, err := time.Parse(timeFormat, string(data))
	if err != nil {
		return err
	}
	*t = Time(newT)
	return nil
}

// Types of things in Item
const (
	KindOfFolder        = "drive#folder"
	KindOfFile          = "drive#file"
	KindOfFileList      = "drive#fileList"
	KindOfResumable     = "drive#resumable"
	KindOfForm          = "drive#form"
	ThumbnailSizeS      = "SIZE_SMALL"
	ThumbnailSizeM      = "SIZE_MEDIUM"
	ThumbnailSizeL      = "SIZE_LARGE"
	PhaseTypeComplete   = "PHASE_TYPE_COMPLETE"
	PhaseTypeRunning    = "PHASE_TYPE_RUNNING"
	PhaseTypeError      = "PHASE_TYPE_ERROR"
	PhaseTypePending    = "PHASE_TYPE_PENDING"
	UploadTypeForm      = "UPLOAD_TYPE_FORM"
	UploadTypeResumable = "UPLOAD_TYPE_RESUMABLE"
	ListLimit           = 100
)

// ------------------------------------------------------------

type Error struct {
	Reason  string `json:"error"` // short description of the reason, e.g. "file_name_empty" "invalid_request"
	Code    int    `json:"error_code"`
	Url     string `json:"error_url,omitempty"`
	Message string `json:"error_description,omitempty"`
	// can have either of `error_details` or `details``
	ErrorDetails []*ErrorDetails `json:"error_details,omitempty"`
	Details      []*ErrorDetails `json:"details,omitempty"`
}

type ErrorDetails struct {
	Type     string `json:"@type,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Metadata struct {
	} `json:"metadata,omitempty"` // TODO: undiscovered yet
	Locale       string        `json:"locale,omitempty"` // e.g. "en"
	Message      string        `json:"message,omitempty"`
	StackEntries []interface{} `json:"stack_entries,omitempty"` // TODO: undiscovered yet
	Detail       string        `json:"detail,omitempty"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := fmt.Sprintf("Error %q (%d)", e.Reason, e.Code)
	if e.Message != "" {
		out += ": " + e.Message
	}
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// ------------------------------------------------------------

// Filters contains parameters for filters when listing.
//
// possible operators
// * in: a list of comma-separated string
// * eq: "true" or "false"
// * gt or lt: time format string, e.g. "2023-01-28T10:56:49.757+08:00"
type Filters struct {
	Phase        *map[string]string `json:"phase,omitempty"`         // "in" or "eq"
	Trashed      *map[string]bool   `json:"trashed,omitempty"`       // "eq"
	Kind         *map[string]string `json:"kind,omitempty"`          // "eq"
	Starred      *map[string]bool   `json:"starred,omitempty"`       // "eq"
	ModifiedTime *map[string]string `json:"modified_time,omitempty"` // "gt" or "lt"
}

func (f *Filters) Set(field, operator, value string) {
	if value == "" {
		// UNSET for empty values
		return
	}
	r := reflect.ValueOf(f)
	fd := reflect.Indirect(r).FieldByName(field)
	if v, err := strconv.ParseBool(value); err == nil {
		fd.Set(reflect.ValueOf(&map[string]bool{operator: v}))
	} else {
		fd.Set(reflect.ValueOf(&map[string]string{operator: value}))
	}
}

// ------------------------------------------------------------
// Common Elements

type Link struct {
	Url    string `json:"url"`
	Token  string `json:"token"`
	Expire Time   `json:"expire"`
	Type   string `json:"type,omitempty"`
}

type Url struct {
	Kind string `json:"kind,omitempty"` // e.g. "upload#url"
	Url  string `json:"url,omitempty"`
}

// ------------------------------------------------------------
// Base Elements

type FileList struct {
	Kind            string  `json:"kind,omitempty"` // drive#fileList
	Files           []*File `json:"files,omitempty"`
	NextPageToken   string  `json:"next_page_token"`
	Version         string  `json:"version,omitempty"`
	VersionOutdated bool    `json:"version_outdated,omitempty"`
}

// There are two types of download links,
// 1) one from api.WebContentLink or api.Links.ApplicationOctetStream.Url and
// 2) the other from api.Medias[].Link.Url.
// Empirically, 2) is less restrictive to multiple concurrent range-requests
// for a single file, i.e. supports for higher `--multi-thread-streams=N`.
// However, it is not generally applicable as it is only for meadia.
type File struct {
	Apps              []*FileApp    `json:"apps,omitempty"`
	Audit             *FileAudit    `json:"audit,omitempty"`
	Collection        string        `json:"collection,omitempty"` // TODO
	CreatedTime       Time          `json:"created_time,omitempty"`
	DeleteTime        Time          `json:"delete_time,omitempty"`
	FileCategory      string        `json:"file_category,omitempty"`
	FileExtension     string        `json:"file_extension,omitempty"`
	FolderType        string        `json:"folder_type,omitempty"`
	Hash              string        `json:"hash,omitempty"` // sha1 but NOT a valid file hash. looks like a torrent hash
	IconLink          string        `json:"icon_link,omitempty"`
	Id                string        `json:"id,omitempty"`
	Kind              string        `json:"kind,omitempty"` // "drive#file"
	Links             *FileLinks    `json:"links,omitempty"`
	Md5Checksum       string        `json:"md5_checksum,omitempty"`
	Medias            []*Media      `json:"medias,omitempty"`
	MimeType          string        `json:"mime_type,omitempty"`
	ModifiedTime      Time          `json:"modified_time,omitempty"` // updated when renamed or moved
	Name              string        `json:"name,omitempty"`
	OriginalFileIndex int           `json:"original_file_index,omitempty"` // TODO
	OriginalUrl       string        `json:"original_url,omitempty"`
	Params            *FileParams   `json:"params,omitempty"`
	ParentId          string        `json:"parent_id,omitempty"`
	Phase             string        `json:"phase,omitempty"`
	Revision          int           `json:"revision,omitempty,string"`
	Size              int64         `json:"size,omitempty,string"`
	SortName          string        `json:"sort_name,omitempty"`
	Space             string        `json:"space,omitempty"`
	SpellName         []interface{} `json:"spell_name,omitempty"` // TODO maybe list of something?
	Starred           bool          `json:"starred,omitempty"`
	ThumbnailLink     string        `json:"thumbnail_link,omitempty"`
	Trashed           bool          `json:"trashed,omitempty"`
	UserId            string        `json:"user_id,omitempty"`
	UserModifiedTime  Time          `json:"user_modified_time,omitempty"`
	WebContentLink    string        `json:"web_content_link,omitempty"`
	Writable          bool          `json:"writable,omitempty"`
}

type FileLinks struct {
	ApplicationOctetStream *Link `json:"application/octet-stream,omitempty"`
}

type FileAudit struct {
	Status  string `json:"status,omitempty"` // "STATUS_OK"
	Message string `json:"message,omitempty"`
	Title   string `json:"title,omitempty"`
}

type Media struct {
	MediaId   string `json:"media_id,omitempty"`
	MediaName string `json:"media_name,omitempty"`
	Video     struct {
		Height     int    `json:"height,omitempty"`
		Width      int    `json:"width,omitempty"`
		Duration   int64  `json:"duration,omitempty"`
		BitRate    int    `json:"bit_rate,omitempty"`
		FrameRate  int    `json:"frame_rate,omitempty"`
		VideoCodec string `json:"video_codec,omitempty"`
		AudioCodec string `json:"audio_codec,omitempty"`
		VideoType  string `json:"video_type,omitempty"`
	} `json:"video,omitempty"`
	Link           *Link         `json:"link,omitempty"`
	NeedMoreQuota  bool          `json:"need_more_quota,omitempty"`
	VipTypes       []interface{} `json:"vip_types,omitempty"` // TODO maybe list of something?
	RedirectLink   string        `json:"redirect_link,omitempty"`
	IconLink       string        `json:"icon_link,omitempty"`
	IsDefault      bool          `json:"is_default,omitempty"`
	Priority       int           `json:"priority,omitempty"`
	IsOrigin       bool          `json:"is_origin,omitempty"`
	ResolutionName string        `json:"resolution_name,omitempty"`
	IsVisible      bool          `json:"is_visible,omitempty"`
	Category       string        `json:"category,omitempty"`
}

type FileParams struct {
	Duration     int64  `json:"duration,omitempty,string"` // in seconds
	Height       int    `json:"height,omitempty,string"`
	Platform     string `json:"platform,omitempty"` // "Upload"
	PlatformIcon string `json:"platform_icon,omitempty"`
	Url          string `json:"url,omitempty"`
	Width        int    `json:"width,omitempty,string"`
}

type FileApp struct {
	ID            string        `json:"id,omitempty"`   // "decompress" for rar files
	Name          string        `json:"name,omitempty"` // decompress" for rar files
	Access        []interface{} `json:"access,omitempty"`
	Link          string        `json:"link,omitempty"` // "https://mypikpak.com/drive/decompression/{File.Id}?gcid={File.Hash}\u0026wv-style=topbar%3Ahide"
	RedirectLink  string        `json:"redirect_link,omitempty"`
	VipTypes      []interface{} `json:"vip_types,omitempty"`
	NeedMoreQuota bool          `json:"need_more_quota,omitempty"`
	IconLink      string        `json:"icon_link,omitempty"`
	IsDefault     bool          `json:"is_default,omitempty"`
	Params        struct {
	} `json:"params,omitempty"` // TODO
	CategoryIds []interface{} `json:"category_ids,omitempty"`
	AdSceneType int           `json:"ad_scene_type,omitempty"`
	Space       string        `json:"space,omitempty"`
	Links       struct {
	} `json:"links,omitempty"` // TODO
}

// ------------------------------------------------------------

type TaskList struct {
	Tasks         []*Task `json:"tasks,omitempty"` // "drive#task"
	NextPageToken string  `json:"next_page_token"`
	ExpiresIn     int     `json:"expires_in,omitempty"`
}

type Task struct {
	Kind              string        `json:"kind,omitempty"` // "drive#task"
	Id                string        `json:"id,omitempty"`   // task id?
	Name              string        `json:"name,omitempty"` // torrent name?
	Type              string        `json:"type,omitempty"` // "offline"
	UserId            string        `json:"user_id,omitempty"`
	Statuses          []interface{} `json:"statuses,omitempty"`    // TODO
	StatusSize        int           `json:"status_size,omitempty"` // TODO
	Params            *TaskParams   `json:"params,omitempty"`      // TODO
	FileId            string        `json:"file_id,omitempty"`
	FileName          string        `json:"file_name,omitempty"`
	FileSize          string        `json:"file_size,omitempty"`
	Message           string        `json:"message,omitempty"` // e.g. "Saving"
	CreatedTime       Time          `json:"created_time,omitempty"`
	UpdatedTime       Time          `json:"updated_time,omitempty"`
	ThirdTaskId       string        `json:"third_task_id,omitempty"` // TODO
	Phase             string        `json:"phase,omitempty"`         // e.g. "PHASE_TYPE_RUNNING"
	Progress          int           `json:"progress,omitempty"`
	IconLink          string        `json:"icon_link,omitempty"`
	Callback          string        `json:"callback,omitempty"`
	ReferenceResource interface{}   `json:"reference_resource,omitempty"` // TODO
	Space             string        `json:"space,omitempty"`
}

type TaskParams struct {
	Age          string `json:"age,omitempty"`
	PredictSpeed string `json:"predict_speed,omitempty"`
	PredictType  string `json:"predict_type,omitempty"`
	Url          string `json:"url,omitempty"`
}

type Form struct {
	Headers    struct{} `json:"headers"`
	Kind       string   `json:"kind"`   // "drive#form"
	Method     string   `json:"method"` // "POST"
	MultiParts struct {
		OSSAccessKeyId string `json:"OSSAccessKeyId"`
		Signature      string `json:"Signature"`
		Callback       string `json:"callback"`
		Key            string `json:"key"`
		Policy         string `json:"policy"`
		XUserData      string `json:"x:user_data"`
	} `json:"multi_parts"`
	Url string `json:"url"`
}

type Resumable struct {
	Kind     string           `json:"kind,omitempty"`     // "drive#resumable"
	Provider string           `json:"provider,omitempty"` // e.g. "PROVIDER_ALIYUN"
	Params   *ResumableParams `json:"params,omitempty"`
}

type ResumableParams struct {
	AccessKeyId     string `json:"access_key_id,omitempty"`
	AccessKeySecret string `json:"access_key_secret,omitempty"`
	Bucket          string `json:"bucket,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	Expiration      Time   `json:"expiration,omitempty"`
	Key             string `json:"key,omitempty"`
	SecurityToken   string `json:"security_token,omitempty"`
}

type FileInArchive struct {
	Index    int    `json:"index,omitempty"`
	Filename string `json:"filename,omitempty"`
	Filesize string `json:"filesize,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Gcid     string `json:"gcid,omitempty"`
	Kind     string `json:"kind,omitempty"`
	IconLink string `json:"icon_link,omitempty"`
	Path     string `json:"path,omitempty"`
}

// ------------------------------------------------------------

type NewFile struct {
	File       *File      `json:"file,omitempty"`
	Form       *Form      `json:"form,omitempty"`
	Resumable  *Resumable `json:"resumable,omitempty"`
	Task       *Task      `json:"task,omitempty"`        // null in this case
	UploadType string     `json:"upload_type,omitempty"` // "UPLOAD_TYPE_FORM" or "UPLOAD_TYPE_RESUMABLE"
}

type NewTask struct {
	UploadType string `json:"upload_type,omitempty"` // "UPLOAD_TYPE_URL"
	File       *File  `json:"file,omitempty"`        // null in this case
	Task       *Task  `json:"task,omitempty"`
	Url        *Url   `json:"url,omitempty"` // {"kind": "upload#url"}
}

type About struct {
	Kind      string `json:"kind,omitempty"` // "drive#about"
	Quota     *Quota `json:"quota,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Quotas    struct {
	} `json:"quotas,omitempty"` // maybe []*Quota?
}

type Quota struct {
	Kind           string `json:"kind,omitempty"`                  // "drive#quota"
	Limit          int64  `json:"limit,omitempty,string"`          // limit in bytes
	Usage          int64  `json:"usage,omitempty,string"`          // bytes in use
	UsageInTrash   int64  `json:"usage_in_trash,omitempty,string"` // bytes in trash but this seems not working
	PlayTimesLimit string `json:"play_times_limit,omitempty"`      // maybe in seconds
	PlayTimesUsage string `json:"play_times_usage,omitempty"`      // maybe in seconds
}

// used in PublicLink()
type Share struct {
	ShareId   string `json:"share_id,omitempty"`
	ShareUrl  string `json:"share_url,omitempty"`
	PassCode  string `json:"pass_code,omitempty"`
	ShareText string `json:"share_text,omitempty"`
}

// GET https://user.mypikpak.com/v1/user/me
type User struct {
	Sub               string          `json:"sub,omitempty"`       // userid for internal use
	Name              string          `json:"name,omitempty"`      // Username
	Picture           string          `json:"picture,omitempty"`   // URL to Avatar image
	Email             string          `json:"email,omitempty"`     // redacted email address
	Providers         *[]UserProvider `json:"providers,omitempty"` // OAuth provider
	PhoneNumber       string          `json:"phone_number,omitempty"`
	Password          string          `json:"password,omitempty"` // "SET" if configured
	Status            string          `json:"status,omitempty"`   // "ACTIVE"
	CreatedAt         Time            `json:"created_at,omitempty"`
	PasswordUpdatedAt Time            `json:"password_updated_at,omitempty"`
}

type UserProvider struct {
	Id             string `json:"id,omitempty"` // e.g. "google.com"
	ProviderUserId string `json:"provider_user_id,omitempty"`
	Name           string `json:"name,omitempty"` // username
}

type DecompressResult struct {
	Status       string `json:"status,omitempty"` // "OK"
	StatusText   string `json:"status_text,omitempty"`
	TaskId       string `json:"task_id,omitempty"`   // same as File.Id
	FilesNum     int    `json:"files_num,omitempty"` // number of files in archive
	RedirectLink string `json:"redirect_link,omitempty"`
}

// ------------------------------------------------------------

type RequestShare struct {
	FileIds        []string `json:"file_ids,omitempty"`
	ShareTo        string   `json:"share_to,omitempty"`         // "publiclink",
	ExpirationDays int      `json:"expiration_days,omitempty"`  // -1 = 'forever'
	PassCodeOption string   `json:"pass_code_option,omitempty"` // "NOT_REQUIRED"
}

type RequestBatch struct {
	Ids []string           `json:"ids,omitempty"`
	To  *map[string]string `json:"to,omitempty"`
}

// used for creating `drive#folder`
type RequestNewFile struct {
	// always required
	Kind       string `json:"kind"` // "drive#folder" or "drive#file"
	Name       string `json:"name"`
	ParentId   string `json:"parent_id"`
	FolderType string `json:"folder_type"`
	// only when uploading a new file
	Hash       string             `json:"hash,omitempty"`      // sha1sum
	Resumable  *map[string]string `json:"resumable,omitempty"` // {"provider": "PROVIDER_ALIYUN"}
	Size       int64              `json:"size,omitempty"`
	UploadType string             `json:"upload_type,omitempty"` // "UPLOAD_TYPE_FORM" or "UPLOAD_TYPE_RESUMABLE"
}

// async request for a new task uploading (offline downloading) files by Urls
//
// Name and ParentId can be left empty.
type RequestNewTask struct {
	Kind       string `json:"kind,omitempty"` // "drive#file"
	Name       string `json:"name,omitempty"`
	ParentId   string `json:"parent_id,omitempty"`
	UploadType string `json:"upload_type,omitempty"` // "UPLOAD_TYPE_URL"
	Url        *Url   `json:"url,omitempty"`         // {"url": downloadUrl}
	FolderType string `json:"folder_type,omitempty"` // "" if parent_id else "DOWNLOAD"
}

type RequestDecompress struct {
	Gcid          string           `json:"gcid,omitempty"`     // same as File.Hash
	Password      string           `json:"password,omitempty"` // ""
	FileId        string           `json:"file_id,omitempty"`
	Files         []*FileInArchive `json:"files,omitempty"` // can request selected files to be decompressed
	DefaultParent bool             `json:"default_parent,omitempty"`
}

// ------------------------------------------------------------

// NOT implemented YET

// GET https://api-drive.mypikpak.com/drive/v1/privilege/vip
type VIP struct {
	Result      string `json:"result,omitempty"` // "ACCEPTED"
	Message     string `json:"message,omitempty"`
	RedirectUri string `json:"redirect_uri,omitempty"`
	Data        struct {
		Expire Time   `json:"expire,omitempty"`
		Status string `json:"status,omitempty"`  // "invalid"
		Type   string `json:"type,omitempty"`    // "novip"
		UserID string `json:"user_id,omitempty"` // same as User.Sub
	} `json:"data,omitempty"`
}

// POST https://api-drive.mypikpak.com/decompress/v1/list
type RequestArchiveFileList struct {
	Gcid     string `json:"gcid,omitempty"`     // same as api.File.Hash
	Path     string `json:"path,omitempty"`     // "" by default
	Password string `json:"password,omitempty"` // "" by default
	FileId   string `json:"file_id,omitempty"`
}

type ArchiveFileList struct {
	Status      string           `json:"status,omitempty"`       // "OK"
	StatusText  string           `json:"status_text,omitempty"`  // ""
	TaskId      string           `json:"task_id,omitempty"`      // ""
	CurrentPath string           `json:"current_path,omitempty"` // ""
	Title       string           `json:"title,omitempty"`
	FileSize    int64            `json:"file_size,omitempty"`
	Gcid        string           `json:"gcid,omitempty"` // same as File.Hash
	Files       []*FileInArchive `json:"files,omitempty"`
}
