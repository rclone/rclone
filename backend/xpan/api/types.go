package api

// Response common response
type Response struct {
	ErrorNumber int `json:"errno"`
	ErrorCode   int `json:"error_code"`
}

// Code the error code
func (r Response) Code() int {
	if r.ErrorNumber != 0 {
		return r.ErrorNumber
	}
	if r.ErrorCode != 0 {
		return r.ErrorCode
	}
	return 0
}

// Item file/directory info
type Item struct {
	FsID             uint64 `json:"fs_id"`           //文件在云端的唯一标识ID
	Path             string `json:"path"`            //文件的绝对路径
	ServerFilename   string `json:"server_filename"` //文件名称
	Size             uint   `json:"size"`            //文件大小，单位B
	ServerModifyTime uint   `json:"server_mtime"`    //文件在服务器修改时间
	ServerCreateTime uint   `json:"server_ctime"`    //文件在服务器创建时间
	LocalModifyTime  uint   `json:"local_mtime"`     //文件在客户端修改时间
	LocalCreateTime  uint   `json:"local_ctime"`     //文件在客户端创建时间
	DirFlag          uint   `json:"isdir"`           //是否为目录，0 文件、1 目录
	DirEmptyFlag     int    `json:"dir_empty"`       //该目录是否存在子目录，只有请求参数web=1且该条目为目录时，该字段才存在， 0为存在， 1为不存在
}

// IsDir Is it a folder?
func (item Item) IsDir() bool {
	return item.DirFlag == 1
}

// ListFilesResponse list api response
type ListFilesResponse struct {
	Response
	RequestID int64  `json:"request_id"`
	GUIDInfo  string `json:"guid_info"`
	List      []Item `json:"list"`
	GUID      int    `json:"guid"`
}

// ListRFilesResponse recursive list api response
type ListRFilesResponse struct {
	Response
	RequestID string `json:"request_id"`
	HasMore   int    `json:"has_more"`
	Cursor    int    `json:"cursor"`
	List      []Item `json:"list"`
}

// SingleUpoadResponse fast upload api response
type SingleUpoadResponse struct {
	Response
	Item
	RequestID int64 `json:"request_id"`
}

// MultipartUploadResponse multipart upload api response
type MultipartUploadResponse struct {
	Response
	RequestID int64  `json:"request_id"`
	UploadID  string `json:"uploadid"`
}

// UploadPartResponse part upload api response
type UploadPartResponse struct {
	Response
	RequestID int64  `json:"request_id"`
	Md5       string `json:"md5"`
}

// MultipartCreateResponse multipart create api response
type MultipartCreateResponse struct {
	Response
	Item
	RequestID int64 `json:"request_id"`
}

// FileMeta file metadata
type FileMeta struct {
	Dlink    string `json:"dlink"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Size     uint64 `json:"size"`
}

// FileMetaResponse file metadata api response
type FileMetaResponse struct {
	Response
	List      []FileMeta `json:"list"`
	RequestID string     `json:"request_id"`
}

// FileManageResponse file manager api response
type FileManageResponse struct {
	Response
	RequestID int64 `json:"request_id"`
}

// CopyOrMove file manager operation
type CopyOrMove struct {
	Path    string `json:"path"`
	Dest    string `json:"dest"`
	Newname string `json:"newname"`
	Ondup   string `json:"ondup"`
}

// QuotaResponse quota api response
type QuotaResponse struct {
	Response
	RequestID int64 `json:"request_id"`
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
}

// UserResponse user info api response
type UserResponse struct {
	Response
	RequestID   string `json:"request_id"`
	VipType     int    `json:"vip_type"`
	BaiduName   string `json:"daidu_name"`
	NetdiskName string `json:"netdisk_name"`
	AvatarURL   string `json:"avatar_url"`
	UK          int    `json:"uk"`
}
