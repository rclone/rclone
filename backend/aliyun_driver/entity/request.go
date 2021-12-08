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
