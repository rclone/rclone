// Package api provides types used by the Terabox API.
package api

// ResponseDefault - check only error
type ResponseDefault struct {
	ErrorAPI
}

// ResponseItemInfo - data or dir information
type ResponseItemInfo struct {
	ErrorAPI
	List []*struct {
		ErrorAPI
		Item
	} `json:"info"`
}

// ResponseList - list dir information
type ResponseList struct {
	ErrorAPI
	List []*Item `json:"list"`
}

// Item information
type Item struct {
	ID                 uint64 `json:"fs_id"` // The file ID; the unique identification of the file in the cloud
	Name               string `json:"server_filename"`
	Path               string `json:"path"`
	MD5                string `json:"md5"`
	Size               int64  `json:"size"`
	Category           int    `json:"category"` // File type: 1: video; 2: audio; 3: image; 4: document; 5: application; 6: others; 7: seed
	Isdir              int    `json:"isdir"`
	Share              int    `json:"share"`           // if share > 0,the file is shared
	ServerCreateTime   int64  `json:"server_ctime"`    // The server-side creation time of the file [unix timestamp]
	ServerModifiedTime int64  `json:"server_mtime"`    // The server-side modification time of the file [unix timestamp]
	DownloadLink       string `json:"dlink,omitempty"` // Download link for Item Info response, not awailable for list
}

// OperationalItem operation on Item
type OperationalItem struct {
	Path        string `json:"path"`
	Destination string `json:"dest,omitempty"`
	NewName     string `json:"newname,omitempty"`
	OnDuplicate string `json:"ondup,omitempty"` // foor copy or move, can be `overwrite`, `newcopy` - will add `(1)` to filename
}

// ResponseOperational result of operation on Item
type ResponseOperational struct {
	ErrorAPI
	Info []struct {
		ErrorAPI
		Path string `json:"path"`
	} `json:"info"`
}

// ResponseDownload contain Download link
type ResponseDownload struct {
	ErrorAPI
	DownloadLink []struct {
		ID  string `json:"fs_id"`
		URL string `json:"dlink"`
	} `json:"dlink"`
	FileInfo struct {
		Size int64  `json:"size"`
		Name string `json:"filename"`
	} `json:"file_info"`
}

// ResponseHomeInfo download file sign
type ResponseHomeInfo struct {
	ErrorAPI
	Data struct {
		Sign1     string `json:"sign1"`
		Sign3     string `json:"sign3"`
		Timestamp int64  `json:"timestamp"`
	} `json:"data"`
}

// ResponseQuota storage info
type ResponseQuota struct {
	ErrorAPI
	Total      int64 `json:"total"`
	Used       int64 `json:"used"`
	Free       int64 `json:"free"`
	Expire     bool  `json:"expire"`
	SboxUsed   int64 `json:"sbox_used"`
	ServerTime int64 `json:"server_time"`
}

// ResponseFileLocateUpload host for file uploading
type ResponseFileLocateUpload struct {
	ErrorAPI
	Host string `json:"host"`
}

// ResponsePrecreate params of created file
type ResponsePrecreate struct {
	ErrorAPI
	UploadID  string  `json:"uploadid"`
	Type      int     `json:"return_type"`
	Path      string  `json:"path"`
	BlockList []int64 `json:"block_list"`
}

// ResponseUploadedChunk information about uploaded chunk
type ResponseUploadedChunk struct {
	UploadID string `json:"uploadid"`
	PartSeq  int    `json:"partseq"`
	MD5      string `json:"md5"`
}

// ResponseCreate file creation result
type ResponseCreate struct {
	ErrorAPI
	MD5  string `json:"md5"`
	Size int64  `json:"size"`
	Name string `json:"filename"`
}

// ResponseUser check user VIP status
type ResponseUser struct {
	ErrorAPI
	Data struct {
		MemberInfo struct {
			IsVIP int64 `json:"is_vip"`
		} `json:"member_info"`
	} `json:"data"`
}
