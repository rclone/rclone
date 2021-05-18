// Package api has type definitions for filefabric
//
// Converted from the API responses with help from https://mholt.github.io/json-to-go/
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	// TimeFormat for parameters (UTC)
	timeFormatParameters = `2006-01-02 15:04:05`
	// "2020-08-11 10:10:04" for JSON parsing
	timeFormatJSON = `"` + timeFormatParameters + `"`
)

// Time represents represents date and time information for the
// filefabric API
type Time time.Time

// MarshalJSON turns a Time into JSON (in UTC)
func (t *Time) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).UTC().Format(timeFormatJSON)
	return []byte(timeString), nil
}

var zeroTime = []byte(`"0000-00-00 00:00:00"`)

// UnmarshalJSON turns JSON into a Time (in UTC)
func (t *Time) UnmarshalJSON(data []byte) error {
	// Set a Zero time.Time if we receive a zero time input
	if bytes.Equal(data, zeroTime) {
		*t = Time(time.Time{})
		return nil
	}
	newT, err := time.Parse(timeFormatJSON, string(data))
	if err != nil {
		return err
	}
	*t = Time(newT)
	return nil
}

// String turns a Time into a string in UTC suitable for the API
// parameters
func (t Time) String() string {
	return time.Time(t).UTC().Format(timeFormatParameters)
}

// Int represents an integer which can be represented in JSON as a
// quoted integer or an integer.
type Int int

// MarshalJSON turns a Int into JSON
func (i *Int) MarshalJSON() (out []byte, err error) {
	return json.Marshal((*int)(i))
}

// UnmarshalJSON turns JSON into a Int
func (i *Int) UnmarshalJSON(data []byte) error {
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	return json.Unmarshal(data, (*int)(i))
}

// Status return returned in all status responses
type Status struct {
	Code    string `json:"status"`
	Message string `json:"statusmessage"`
	TaskID  string `json:"taskid"`
	// Warning string `json:"warning"` // obsolete
}

// Status statisfies the error interface
func (e *Status) Error() string {
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// OK returns true if the status is all good
func (e *Status) OK() bool {
	return e.Code == "ok"
}

// GetCode returns the status code if any
func (e *Status) GetCode() string {
	return e.Code
}

// OKError defines an interface for items which can be OK or be an error
type OKError interface {
	error
	OK() bool
	GetCode() string
}

// Check Status satisfies the OKError interface
var _ OKError = (*Status)(nil)

// EmptyResponse is response which just returns the error condition
type EmptyResponse struct {
	Status
}

// GetTokenByAuthTokenResponse is the response to getTokenByAuthToken
type GetTokenByAuthTokenResponse struct {
	Status
	Token              string `json:"token"`
	UserID             string `json:"userid"`
	AllowLoginRemember string `json:"allowloginremember"`
	LastLogin          Time   `json:"lastlogin"`
	AutoLoginCode      string `json:"autologincode"`
}

// ApplianceInfo is the response to getApplianceInfo
type ApplianceInfo struct {
	Status
	Sitetitle            string `json:"sitetitle"`
	OauthLoginSupport    string `json:"oauthloginsupport"`
	IsAppliance          string `json:"isappliance"`
	SoftwareVersion      string `json:"softwareversion"`
	SoftwareVersionLabel string `json:"softwareversionlabel"`
}

// GetFolderContentsResponse is returned from getFolderContents
type GetFolderContentsResponse struct {
	Status
	Total  int    `json:"total,string"`
	Items  []Item `json:"filelist"`
	Folder Item   `json:"folder"`
	From   Int    `json:"from"`
	//Count         int    `json:"count"`
	Pid           string `json:"pid"`
	RefreshResult Status `json:"refreshresult"`
	// Curfolder         Item              `json:"curfolder"` - sometimes returned as "ROOT"?
	Parents           []Item            `json:"parents"`
	CustomPermissions CustomPermissions `json:"custompermissions"`
}

// ItemType determine whether it is a file or a folder
type ItemType uint8

// Types of things in Item
const (
	ItemTypeFile   ItemType = 0
	ItemTypeFolder ItemType = 1
)

// Item ia a File or a Folder
type Item struct {
	ID  string `json:"fi_id"`
	PID string `json:"fi_pid"`
	// UID             string   `json:"fi_uid"`
	Name string `json:"fi_name"`
	// S3Name          string   `json:"fi_s3name"`
	// Extension       string   `json:"fi_extension"`
	// Description     string   `json:"fi_description"`
	Type ItemType `json:"fi_type,string"`
	// Created         Time     `json:"fi_created"`
	Size        int64  `json:"fi_size,string"`
	ContentType string `json:"fi_contenttype"`
	// Tags            string   `json:"fi_tags"`
	// MainCode        string   `json:"fi_maincode"`
	// Public          int      `json:"fi_public,string"`
	// Provider        string   `json:"fi_provider"`
	// ProviderFolder  string   `json:"fi_providerfolder"` // folder
	// Encrypted       int      `json:"fi_encrypted,string"`
	// StructType      string   `json:"fi_structtype"`
	// Bname           string   `json:"fi_bname"` // folder
	// OrgID           string   `json:"fi_orgid"`
	// Favorite        int      `json:"fi_favorite,string"`
	// IspartOf        string   `json:"fi_ispartof"` // folder
	Modified Time `json:"fi_modified"`
	// LastAccessed    Time     `json:"fi_lastaccessed"`
	// Hits            int64    `json:"fi_hits,string"`
	// IP              string   `json:"fi_ip"` // folder
	// BigDescription  string   `json:"fi_bigdescription"`
	LocalTime Time `json:"fi_localtime"`
	// OrgfolderID     string   `json:"fi_orgfolderid"`
	// StorageIP       string   `json:"fi_storageip"` // folder
	// RemoteTime      Time     `json:"fi_remotetime"`
	// ProviderOptions string   `json:"fi_provideroptions"`
	// Access          string   `json:"fi_access"`
	// Hidden          string   `json:"fi_hidden"` // folder
	// VersionOf       string   `json:"fi_versionof"`
	Trash bool `json:"trash"`
	// Isbucket        string   `json:"isbucket"` // filelist
	SubFolders int64 `json:"subfolders"` // folder
}

// ItemFields is a | separated list of fields in Item
var ItemFields = mustFields(Item{})

// fields returns the JSON fields in use by opt as a | separated
// string.
func fields(opt interface{}) (pipeTags string, err error) {
	var tags []string
	def := reflect.ValueOf(opt)
	defType := def.Type()
	for i := 0; i < def.NumField(); i++ {
		field := defType.Field(i)
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			continue
		}
		if comma := strings.IndexRune(tag, ','); comma >= 0 {
			tag = tag[:comma]
		}
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}
	return strings.Join(tags, "|"), nil
}

// mustFields returns the JSON fields in use by opt as a | separated
// string. It panics on failure.
func mustFields(opt interface{}) string {
	tags, err := fields(opt)
	if err != nil {
		panic(err)
	}
	return tags
}

// CustomPermissions is returned as part of GetFolderContentsResponse
type CustomPermissions struct {
	Upload            string `json:"upload"`
	CreateSubFolder   string `json:"createsubfolder"`
	Rename            string `json:"rename"`
	Delete            string `json:"delete"`
	Move              string `json:"move"`
	ManagePermissions string `json:"managepermissions"`
	ListOnly          string `json:"listonly"`
	VisibleInTrash    string `json:"visibleintrash"`
}

// DoCreateNewFolderResponse is response from foCreateNewFolder
type DoCreateNewFolderResponse struct {
	Status
	Item Item `json:"file"`
}

// DoInitUploadResponse is response from doInitUpload
type DoInitUploadResponse struct {
	Status
	ProviderID          string `json:"providerid"`
	UploadCode          string `json:"uploadcode"`
	FileType            string `json:"filetype"`
	DirectUploadSupport string `json:"directuploadsupport"`
	ResumeAllowed       string `json:"resumeallowed"`
}

// UploaderResponse is returned from /cgi-bin/uploader/uploader1.cgi
//
// Sometimes the response is returned as XML and sometimes as JSON
type UploaderResponse struct {
	FileSize int64  `xml:"filesize" json:"filesize,string"`
	MD5      string `xml:"md5" json:"md5"`
	Success  string `xml:"success" json:"success"`
}

// UploadStatus is returned from getUploadStatus
type UploadStatus struct {
	Status
	UploadCode     string `json:"uploadcode"`
	Metafile       string `json:"metafile"`
	Percent        int    `json:"percent,string"`
	Uploaded       int64  `json:"uploaded,string"`
	Size           int64  `json:"size,string"`
	Filename       string `json:"filename"`
	Nofile         string `json:"nofile"`
	Completed      string `json:"completed"`
	Completsuccess string `json:"completsuccess"`
	Completerror   string `json:"completerror"`
}

// DoCompleteUploadResponse is the response to doCompleteUpload
type DoCompleteUploadResponse struct {
	Status
	UploadedSize int64  `json:"uploadedsize,string"`
	StorageIP    string `json:"storageip"`
	UploadedName string `json:"uploadedname"`
	// Versioned    []interface{} `json:"versioned"`
	// VersionedID  int           `json:"versionedid"`
	// Comment      interface{}           `json:"comment"`
	File Item `json:"file"`
	// UsSize       string        `json:"us_size"`
	// PaSize       string        `json:"pa_size"`
	// SpaceInfo    SpaceInfo     `json:"spaceinfo"`
}

// Providers is returned as part of UploadResponse
type Providers struct {
	Max     string `json:"max"`
	Used    string `json:"used"`
	ID      string `json:"id"`
	Private string `json:"private"`
	Limit   string `json:"limit"`
	Percent int    `json:"percent"`
}

// Total is returned as part of UploadResponse
type Total struct {
	Max        string `json:"max"`
	Used       string `json:"used"`
	ID         string `json:"id"`
	Priused    string `json:"priused"`
	Primax     string `json:"primax"`
	Limit      string `json:"limit"`
	Percent    int    `json:"percent"`
	Pripercent int    `json:"pripercent"`
}

// UploadResponse is returned as part of SpaceInfo
type UploadResponse struct {
	Providers []Providers `json:"providers"`
	Total     Total       `json:"total"`
}

// SpaceInfo is returned as part of DoCompleteUploadResponse
type SpaceInfo struct {
	Response UploadResponse `json:"response"`
	Status   string         `json:"status"`
}

// DeleteResponse is returned from doDeleteFile
type DeleteResponse struct {
	Status
	Deleted        []string      `json:"deleted"`
	Errors         []interface{} `json:"errors"`
	ID             string        `json:"fi_id"`
	BackgroundTask int           `json:"backgroundtask"`
	UsSize         string        `json:"us_size"`
	PaSize         string        `json:"pa_size"`
	//SpaceInfo      SpaceInfo     `json:"spaceinfo"`
}

// FileResponse is returned from doRenameFile
type FileResponse struct {
	Status
	Item   Item   `json:"file"`
	Exists string `json:"exists"`
}

// MoveFilesResponse is returned from doMoveFiles
type MoveFilesResponse struct {
	Status
	Filesleft         string   `json:"filesleft"`
	Addedtobackground string   `json:"addedtobackground"`
	Moved             string   `json:"moved"`
	Item              Item     `json:"file"`
	IDs               []string `json:"fi_ids"`
	Length            int      `json:"length"`
	DirID             string   `json:"dir_id"`
	MovedObjects      []Item   `json:"movedobjects"`
	// FolderTasks       []interface{}  `json:"foldertasks"`
}

// TasksResponse is the response to getUserBackgroundTasks
type TasksResponse struct {
	Status
	Tasks []Task `json:"tasks"`
	Total string `json:"total"`
}

// BtData is part of TasksResponse
type BtData struct {
	Callback string `json:"callback"`
}

// Task describes a task returned in TasksResponse
type Task struct {
	BtID             string `json:"bt_id"`
	UsID             string `json:"us_id"`
	BtType           string `json:"bt_type"`
	BtData           BtData `json:"bt_data"`
	BtStatustext     string `json:"bt_statustext"`
	BtStatusdata     string `json:"bt_statusdata"`
	BtMessage        string `json:"bt_message"`
	BtProcent        string `json:"bt_procent"`
	BtAdded          string `json:"bt_added"`
	BtStatus         string `json:"bt_status"`
	BtCompleted      string `json:"bt_completed"`
	BtTitle          string `json:"bt_title"`
	BtCredentials    string `json:"bt_credentials"`
	BtHidden         string `json:"bt_hidden"`
	BtAutoremove     string `json:"bt_autoremove"`
	BtDevsite        string `json:"bt_devsite"`
	BtPriority       string `json:"bt_priority"`
	BtReport         string `json:"bt_report"`
	BtSitemarker     string `json:"bt_sitemarker"`
	BtExecuteafter   string `json:"bt_executeafter"`
	BtCompletestatus string `json:"bt_completestatus"`
	BtSubtype        string `json:"bt_subtype"`
	BtCanceled       string `json:"bt_canceled"`
	Callback         string `json:"callback"`
	CanBeCanceled    bool   `json:"canbecanceled"`
	CanBeRestarted   bool   `json:"canberestarted"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	Settings         string `json:"settings"`
}
