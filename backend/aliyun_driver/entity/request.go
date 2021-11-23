package entity

// list 查询参数
type ListIn struct {
	Marker         string `json:"marker"`
	Limit          int    `json:"limit"`
	OrderBy        string `json:"order_by"`
	OrderDirection string `json:"order_direction"`
	DriveId        string `json:"drive_id"`
	ParentFileId   string `json:"parent_file_id"`
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

type RmDirIn struct {
	DriveId string `json:"drive_id"`
	FileId  string `json:"file_id"`
}
