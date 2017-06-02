package opendrive

// Account describes a OpenDRIVE account
type Account struct {
	Username string `json:"username"`
	Password string `json:"passwd"`
}

// UserSessionInfo describes a OpenDRIVE session
type UserSessionInfo struct {
	Username string `json:"username"`
	Password string `json:"passwd"`

	SessionID          string `json:"SessionID"`
	UserName           string `json:"UserName"`
	UserFirstName      string `json:"UserFirstName"`
	UserLastName       string `json:"UserLastName"`
	AccType            string `json:"AccType"`
	UserLang           string `json:"UserLang"`
	UserID             string `json:"UserID"`
	IsAccountUser      int    `json:"IsAccountUser"`
	DriveName          string `json:"DriveName"`
	UserLevel          string `json:"UserLevel"`
	UserPlan           string `json:"UserPlan"`
	FVersioning        string `json:"FVersioning"`
	UserDomain         string `json:"UserDomain"`
	PartnerUsersDomain string `json:"PartnerUsersDomain"`
}

// FolderList describes a OpenDRIVE listing
type FolderList struct {
	// DirUpdateTime    string   `json:"DirUpdateTime,string"`
	Name             string   `json:"Name"`
	ParentFolderID   string   `json:"ParentFolderID"`
	DirectFolderLink string   `json:"DirectFolderLink"`
	ResponseType     int      `json:"ResponseType"`
	Folders          []Folder `json:"Folders"`
	Files            []File   `json:"Files"`
}

// Folder describes a OpenDRIVE folder
type Folder struct {
	FolderID      string `json:"FolderID"`
	Name          string `json:"Name"`
	DateCreated   int    `json:"DateCreated"`
	DirUpdateTime int    `json:"DirUpdateTime"`
	Access        int    `json:"Access"`
	DateModified  int64  `json:"DateModified"`
	Shared        string `json:"Shared"`
	ChildFolders  int    `json:"ChildFolders"`
	Link          string `json:"Link"`
	Encrypted     string `json:"Encrypted"`
}

// File describes a OpenDRIVE file
type File struct {
	FileID            string `json:"FileId"`
	Name              string `json:"Name"`
	GroupID           int    `json:"GroupID"`
	Extension         string `json:"Extension"`
	Size              int64  `json:"Size,string"`
	Views             string `json:"Views"`
	Version           string `json:"Version"`
	Downloads         string `json:"Downloads"`
	DateModified      int64  `json:"DateModified,string"`
	Access            string `json:"Access"`
	Link              string `json:"Link"`
	DownloadLink      string `json:"DownloadLink"`
	StreamingLink     string `json:"StreamingLink"`
	TempStreamingLink string `json:"TempStreamingLink"`
	EditLink          string `json:"EditLink"`
	ThumbLink         string `json:"ThumbLink"`
	Password          string `json:"Password"`
	EditOnline        int    `json:"EditOnline"`
}

type createFile struct {
	SessionID string `json:"session_id"`
	FolderID  string `json:"folder_id"`
	Name      string `json:"file_name"`
}

type createFileResponse struct {
	FileID             string `json:"FileId"`
	Name               string `json:"Name"`
	GroupID            int    `json:"GroupID"`
	Extension          string `json:"Extension"`
	Size               string `json:"Size"`
	Views              string `json:"Views"`
	Downloads          string `json:"Downloads"`
	DateModified       string `json:"DateModified"`
	Access             string `json:"Access"`
	Link               string `json:"Link"`
	DownloadLink       string `json:"DownloadLink"`
	StreamingLink      string `json:"StreamingLink"`
	TempStreamingLink  string `json:"TempStreamingLink"`
	DirUpdateTime      int    `json:"DirUpdateTime"`
	TempLocation       string `json:"TempLocation"`
	SpeedLimit         int    `json:"SpeedLimit"`
	RequireCompression int    `json:"RequireCompression"`
	RequireHash        int    `json:"RequireHash"`
	RequireHashOnly    int    `json:"RequireHashOnly"`
}

type openUpload struct {
	SessionID string `json:"session_id"`
	FileID    string `json:"file_id"`
	Size      int64  `json:"file_size"`
}

type openUploadResponse struct {
	TempLocation       string `json:"TempLocation"`
	RequireCompression bool   `json:"RequireCompression"`
	RequireHash        bool   `json:"RequireHash"`
	RequireHashOnly    bool   `json:"RequireHashOnly"`
	SpeedLimit         int    `json:"SpeedLimit"`
}

type closeUpload struct {
	SessionID    string `json:"session_id"`
	FileID       string `json:"file_id"`
	Size         int64  `json:"file_size"`
	TempLocation string `json:"temp_location"`
}

type closeUploadResponse struct {
	FileHash string `json:"FileHash"`
	Size     int64  `json:"Size"`
}
