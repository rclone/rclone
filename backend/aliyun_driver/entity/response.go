package entity

import "time"

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AccessTokenOut 获取accessToken 的返回数据
type AccessTokenOut struct {
	DefaultSboxDriveId string        `json:"default_sbox_drive_id"`
	Role               string        `json:"role"`
	DeviceId           string        `json:"device_id"`
	UserName           string        `json:"user_name"`
	NeedLink           bool          `json:"need_link"`
	ExpireTime         time.Time     `json:"expire_time"`
	PinSetup           bool          `json:"pin_setup"`
	NeedRpVerify       bool          `json:"need_rp_verify"`
	Avatar             string        `json:"avatar"`
	TokenType          string        `json:"token_type"`
	AccessToken        string        `json:"access_token"`
	DefaultDriveId     string        `json:"default_drive_id"`
	DomainId           string        `json:"domain_id"`
	RefreshToken       string        `json:"refresh_token"`
	IsFirstLogin       bool          `json:"is_first_login"`
	UserId             string        `json:"user_id"`
	NickName           string        `json:"nick_name"`
	ExistLink          []interface{} `json:"exist_link"`
	State              string        `json:"state"`
	ExpiresIn          int           `json:"expires_in"`
	Status             string        `json:"status"`
}

// 返回输出
type ListOut struct {
	Items             []ItemsOut `json:"items"`
	NextMarker        string     `json:"next_marker"`
	PunishedFileCount int        `json:"punished_file_count"`
}
type CroppingBoundary struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Top    int `json:"top"`
	Left   int `json:"left"`
}
type CroppingSuggestion struct {
	AspectRatio      string           `json:"aspect_ratio"`
	Score            float64          `json:"score"`
	CroppingBoundary CroppingBoundary `json:"cropping_boundary"`
}
type ImageMediaMetadata struct {
	Exif               string               `json:"exif"`
	CroppingSuggestion []CroppingSuggestion `json:"cropping_suggestion"`
}
type Heic struct {
	Crc64Hash   string `json:"crc64_hash"`
	Size        int    `json:"size"`
	DownloadURL string `json:"download_url"`
	URL         string `json:"url"`
	Thumbnail   string `json:"thumbnail"`
}
type Mov struct {
	Crc64Hash   string `json:"crc64_hash"`
	Size        int    `json:"size"`
	DownloadURL string `json:"download_url"`
	URL         string `json:"url"`
	Thumbnail   string `json:"thumbnail"`
}
type StreamsInfo struct {
	Heic Heic `json:"heic"`
	Mov  Mov  `json:"mov"`
}
type ItemsOut struct {
	DriveId         string    `json:"drive_id"`
	DomainId        string    `json:"domain_id"`
	FileId          string    `json:"file_id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Hidden          bool      `json:"hidden"`
	Starred         bool      `json:"starred"`
	Status          string    `json:"status"`
	ParentFileId    string    `json:"parent_file_id"`
	EncryptMode     string    `json:"encrypt_mode"`
	RevisionId      string    `json:"revision_id"`
	FileExtension   string    `json:"file_extension,omitempty"`
	MimeType        string    `json:"mime_type,omitempty"`
	MimeExtension   string    `json:"mime_extension,omitempty"`
	Size            int       `json:"size,omitempty"`
	ContentHash     string    `json:"content_hash,omitempty"`
	ContentHashName string    `json:"content_hash_name,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	URL             string    `json:"url,omitempty"`
	Thumbnail       string    `json:"thumbnail,omitempty"`
	Category        string    `json:"category,omitempty"`
	PunishFlag      int       `json:"punish_flag,omitempty"`
	// StreamsInfo        StreamsInfo        `json:"streams_info,omitempty"`
	// ImageMediaMetadata ImageMediaMetadata `json:"image_media_metadata,omitempty"`
}

type MkdirOut struct {
	DomainId     string `json:"domain_id"`
	DriveId      string `json:"drive_id"`
	EncryptMode  string `json:"encrypt_mode"`
	FileId       string `json:"file_id"`
	FileName     string `json:"file_name"`
	ParentFileId string `json:"parent_file_id"`
	Type         string `json:"type"`
}

type DeleteOut struct {
	AsyncTaskId string `json:"async_task_id"`
	DriveId     string `json:"drive_id"`
	DomainId    string `json:"domain_id"`
	FileId      string `json:"file_id"`
}

type PersonalInfoOut struct {
	PersonalRightsInfo struct {
		SpuID      string `json:"spu_id"`
		Name       string `json:"name"`
		IsExpires  bool   `json:"is_expires"`
		Privileges []struct {
			FeatureID     string `json:"feature_id"`
			FeatureAttrID string `json:"feature_attr_id"`
			Quota         int    `json:"quota"`
		} `json:"privileges"`
	} `json:"personal_rights_info"`
	PersonalSpaceInfo struct {
		UsedSize  int64 `json:"used_size"`
		TotalSize int64 `json:"total_size"`
	} `json:"personal_space_info"`
}

type PreUploadOut struct {
	FileId       string     `json:"file_id"`
	FileName     string     `json:"file_name"`
	Location     string     `json:"location"`
	RapidUpload  bool       `json:"rapid_upload"`
	Type         string     `json:"type"`
	UploadId     string     `json:"upload_id"`
	PartInfoList []PartInfo `json:"part_info_list"`
}

type DownloadInfo struct {
	Method          string    `json:"method"`
	URL             string    `json:"url"`
	InternalURL     string    `json:"internal_url"`
	Expiration      time.Time `json:"expiration"`
	Size            int       `json:"size"`
	ContentHash     string    `json:"content_hash"`
	ContentHashName string    `json:"content_hash_name"`
	StreamsURL      struct {
		Heic string `json:"heic"`
		Mov  string `json:"mov"`
	} `json:"streams_url"`
	Ratelimit struct {
		PartSpeed int `json:"part_speed"`
		PartSize  int `json:"part_size"`
	} `json:"ratelimit"`
}
