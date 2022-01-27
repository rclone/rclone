package entity

// list 查询参数
type ListIn struct {
	Marker                string `json:"marker"`                  //分页游标
	Limit                 int    `json:"limit"`                   //单次请求返回
	OrderBy               string `json:"order_by"`                //排序字段
	OrderDirection        string `json:"order_direction"`         //排序方式
	DriveId               string `json:"drive_id"`                // 驱动Id
	ParentFileId          string `json:"parent_file_id"`          //父目录 Id
	Fields                string `json:"fields"`                  //获取的字段，默认 *
	UrlExpireSec          int64  `json:"url_expire_sec"`          //过期时间 默认 1600
	All                   bool   `json:"all"`                     //是否全部 ，默认 false
	ImageThumbnailProcess string `json:"image_thumbnail_process"` //
	ImageUrlProcess       string `json:"image_url_process"`       //
	VideoThumbnailProcess string `json:"video_thumbnail_process"` //
}

// 获取accessToken 的入参
type AccessTokenIn struct {
	RefreshToken string `json:"refresh_token"`
}

type MakeDirIn struct {
	DriveId       string `json:"drive_id"`
	Name          string `json:"name"`
	ParentFileId  string `json:"parent_file_id"`
	Type          string `json:"type"`
	CheckNameMode string `json:"check_name_mode"`
}

type DeleteIn struct {
	DriveId string `json:"drive_id"`
	FileId  string `json:"file_id"`
}

type PreUploadIn struct {
	CheckNameMode   string     `json:"check_name_mode"`
	ContentHash     string     `json:"content_hash"`
	ContentHashName string     `json:"content_hash_name"`
	DriveId         string     `json:"drive_id"`
	Name            string     `json:"name"`
	ParentFileId    string     `json:"parent_file_id"`
	ProofCode       string     `json:"proof_code"`
	ProofVersion    string     `json:"proof_version"`
	Size            int64      `json:"size"`
	PartInfoList    []PartInfo `json:"part_info_list"`
	Type            string     `json:"type"`
}

type PartInfo struct {
	PartNumber int    `json:"part_number"`
	UploadUrl  string `json:"upload_url"`
}

type CompleteUploadIn struct {
	DriveId  string `json:"drive_id"`
	FileId   string `json:"file_id"`
	UploadId string `json:"upload_id"`
}

type DownloadIn struct {
	DriveId   string `json:"drive_id"`
	FileId    string `json:"file_id"`
	ExpireSec int64  `json:"expire_sec"`
}
