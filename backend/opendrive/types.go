package opendrive

import (
	"encoding/json"
	"fmt"
)

// Error describes an openDRIVE error response
type Error struct {
	Info struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Error satisfies the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%s (Error %d)", e.Info.Message, e.Info.Code)
}

// Account describes an OpenDRIVE account
type Account struct {
	Username string `json:"username"`
	Password string `json:"passwd"`
}

// UserSessionInfo describes an OpenDRIVE session
type UserSessionInfo struct {
	Username string `json:"username"`
	Password string `json:"passwd"`

	SessionID          string          `json:"SessionID"`
	UserName           string          `json:"UserName"`
	UserFirstName      string          `json:"UserFirstName"`
	UserLastName       string          `json:"UserLastName"`
	AccType            string          `json:"AccType"`
	UserLang           string          `json:"UserLang"`
	UserID             string          `json:"UserID"`
	IsAccountUser      json.RawMessage `json:"IsAccountUser"`
	DriveName          string          `json:"DriveName"`
	UserLevel          string          `json:"UserLevel"`
	UserPlan           string          `json:"UserPlan"`
	FVersioning        string          `json:"FVersioning"`
	UserDomain         string          `json:"UserDomain"`
	PartnerUsersDomain string          `json:"PartnerUsersDomain"`
}

// FolderList describes an OpenDRIVE listing
type FolderList struct {
	// DirUpdateTime    string   `json:"DirUpdateTime,string"`
	Name             string   `json:"Name"`
	ParentFolderID   string   `json:"ParentFolderID"`
	DirectFolderLink string   `json:"DirectFolderLink"`
	ResponseType     int      `json:"ResponseType"`
	Folders          []Folder `json:"Folders"`
	Files            []File   `json:"Files"`
}

// Folder describes an OpenDRIVE folder
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

type createFolder struct {
	SessionID           string `json:"session_id"`
	FolderName          string `json:"folder_name"`
	FolderSubParent     string `json:"folder_sub_parent"`
	FolderIsPublic      int64  `json:"folder_is_public"`      // (0 = private, 1 = public, 2 = hidden)
	FolderPublicUpl     int64  `json:"folder_public_upl"`     // (0 = disabled, 1 = enabled)
	FolderPublicDisplay int64  `json:"folder_public_display"` // (0 = disabled, 1 = enabled)
	FolderPublicDnl     int64  `json:"folder_public_dnl"`     // (0 = disabled, 1 = enabled).
}

type createFolderResponse struct {
	FolderID      string `json:"FolderID"`
	Name          string `json:"Name"`
	DateCreated   int    `json:"DateCreated"`
	DirUpdateTime int    `json:"DirUpdateTime"`
	Access        int    `json:"Access"`
	DateModified  int    `json:"DateModified"`
	Shared        string `json:"Shared"`
	Description   string `json:"Description"`
	Link          string `json:"Link"`
}

type moveCopyFolder struct {
	SessionID     string `json:"session_id"`
	FolderID      string `json:"folder_id"`
	DstFolderID   string `json:"dst_folder_id"`
	Move          string `json:"move"`
	NewFolderName string `json:"new_folder_name"` // New name for destination folder.
}

type renameFolder struct {
	SessionID  string `json:"session_id"`
	FolderID   string `json:"folder_id"`
	FolderName string `json:"folder_name"` // New name for destination folder (max 255).
	SharingID  string `json:"sharing_id"`
}

type moveCopyFolderResponse struct {
	FolderID string `json:"FolderID"`
}

type removeFolder struct {
	SessionID string `json:"session_id"`
	FolderID  string `json:"folder_id"`
}

// File describes an OpenDRIVE file
type File struct {
	FileID            string `json:"FileId"`
	FileHash          string `json:"FileHash"`
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

type moveCopyFile struct {
	SessionID         string `json:"session_id"`
	SrcFileID         string `json:"src_file_id"`
	DstFolderID       string `json:"dst_folder_id"`
	Move              string `json:"move"`
	OverwriteIfExists string `json:"overwrite_if_exists"`
	NewFileName       string `json:"new_file_name"` // New name for destination file.
}

type moveCopyFileResponse struct {
	FileID string `json:"FileID"`
	Size   string `json:"Size"`
}

type renameFile struct {
	SessionID      string `json:"session_id"`
	NewFileName    string `json:"new_file_name"` // New name for destination file.
	FileID         string `json:"file_id"`
	AccessFolderID string `json:"access_folder_id"`
	SharingID      string `json:"sharing_id"`
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

type modTimeFile struct {
	SessionID            string `json:"session_id"`
	FileID               string `json:"file_id"`
	FileModificationTime string `json:"file_modification_time"`
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
	FileID   string `json:"FileID"`
	FileHash string `json:"FileHash"`
	Size     int64  `json:"Size"`
}

type permissions struct {
	SessionID    string `json:"session_id"`
	FileID       string `json:"file_id"`
	FileIsPublic int64  `json:"file_ispublic"`
}

type uploadFileChunkReply struct {
	TotalWritten int64 `json:"TotalWritten"`
}
