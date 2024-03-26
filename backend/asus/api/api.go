package api

import (
	"bytes"
	"context"
	"crypto/des"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

func getAsusHashedPassword(password string) string {
	lowerCase := strings.ToLower(password)
	hash := md5.Sum([]byte(lowerCase))
	hashedPassword := hex.EncodeToString(hash[:])
	return hashedPassword
}

func getAsusSymmetryPassword(password string, key string) string {
	c, _ := des.NewTripleDESCipher([]byte(key[:24]))

	input := []byte(password)
	padding := des.BlockSize - len(input)%des.BlockSize
	for i := 0; i < padding; i++ {
		input = append(input, byte(padding))
	}

	// Split plaintext into blocks and encrypt each block using ECB mode
	ciphertext := make([]byte, len(input))
	for i := 0; i < len(input); i += des.BlockSize {
		c.Encrypt(ciphertext[i:i+des.BlockSize], input[i:i+des.BlockSize])
	}

	return base64.StdEncoding.EncodeToString(ciphertext)
}

type AsusAPI struct {
	srv            *rest.Client
	userid         string
	password       string
	symmetrypwd    string
	sid            int
	servicegateway string
	sessionConfig  RequestTokenResponse
	pacer          *fs.Pacer
	cookie         string
}

func NewAsusAPI(ctx context.Context, userid string, password string, pacer *fs.Pacer) (AsusAPI, error) {
	// instead generating unique sid and calculate key for it, official linux client uses next hard-coded values
	sid := 86408316
	key := "C9B01DCA7CB144FBAD58AE1222FA1334"

	client := fshttp.NewClient(ctx)
	api := AsusAPI{
		srv:         rest.NewClient(client),
		userid:      userid,
		password:    password,
		symmetrypwd: getAsusSymmetryPassword(password, key),
		sid:         sid,
		pacer:       pacer,
	}

	api.cookie = fmt.Sprintf("sid=%v", api.sid)
	// api.cookie = fmt.Sprintf("sid=%v;c=0;v=1.0.0.3_1001;EEE_MANU=;EEE_PROD=;OS_VER=", api.sid)

	_, err := api.RequestServiceGateway(ctx)
	if err != nil {
		return api, err
	}

	_, err = api.RequestToken(ctx)
	if err != nil {
		return api, err
	}

	return api, nil
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	authRetry := false

	// if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Contains(resp.Header["Www-Authenticate"][0], "expired_token") {
	// 	authRetry = true
	// 	fs.Debugf(nil, "Should retry: %v", err)
	// }
	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func (a *AsusAPI) callXML(ctx context.Context, host string, path string, req any, res any) (*http.Response, error) {
	opts := rest.Opts{
		RootURL: fmt.Sprintf("https://%s", host),
		Method:  "POST",
		Path:    path,
		ExtraHeaders: map[string]string{
			"Cookie": a.cookie,
		},
	}
	resp := &http.Response{}
	err := error(nil)

	if a.pacer != nil {
		err = a.pacer.Call(func() (bool, error) {
			resp, err := a.srv.CallXML(ctx, &opts, &req, &res)
			return shouldRetry(ctx, resp, err)
		})
	} else {
		resp, err = a.srv.CallXML(ctx, &opts, &req, &res)
	}
	if err != nil {
		return nil, err
	}
	return resp, err
}

type RequestServiceGatewayRequest struct {
	XMLName  xml.Name `xml:"requestservicegateway"`
	UserId   string   `xml:"userid"`
	Password string   `xml:"password"`
	Language string   `xml:"language"`
	Service  int      `xml:"service"`
}

type RequestServiceGatewayResponse struct {
	XMLName          xml.Name `xml:"requestservicegateway"`
	Status           int      `xml:"status"`
	ServiceGateway   string   `xml:"servicegateway"`
	AccountModel     string   `xml:"accountmodel"`
	AccountSyncState int      `xml:"accountsyncstate"`
	LiveUpdateUri    string   `xml:"liveupdateuri"`
	Time             int64    `xml:"time"`
}

func (a *AsusAPI) RequestServiceGateway(ctx context.Context) (RequestServiceGatewayResponse, error) {
	req := RequestServiceGatewayRequest{
		UserId:   a.userid,
		Password: getAsusHashedPassword(a.password),
		Language: "zh_TW",
		Service:  1,
	}
	var resp RequestServiceGatewayResponse
	resp.Status = -1
	_, err := a.callXML(ctx, "syncportal01.asuswebstorage.com", "/member/requestservicegateway/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting RequestServiceGateway, return code %v", resp.Status)
	}
	a.servicegateway = resp.ServiceGateway

	return resp, nil
}

type RequestTokenRequest struct {
	XMLName     xml.Name `xml:"aaa"`
	UserId      string   `xml:"userid"`
	Password    string   `xml:"password,omitempty"`
	SymmetryPwd string   `xml:"symmetrypwd,omitempty"`
}

type RequestTokenResponse struct {
	XMLName       xml.Name    `xml:"aaa"`
	Status        int         `xml:"status"`
	UserId        string      `xml:"userid"`
	Token         string      `xml:"token"`
	TimeToLive    int         `xml:"timetolive"`
	Navigate      string      `xml:"navigate"`
	MailRelay     string      `xml:"mailrelay"`
	FileRelay     string      `xml:"filerelay"`
	Starton       string      `xml:"starton"`
	WebRelay      string      `xml:"webrelay"`
	OmniSearch    string      `xml:"omnisearch"`
	InfoRelay     string      `xml:"inforelay"`
	CoralFacade   string      `xml:"coralfacade"`
	Oao           string      `xml:"oao"`
	ChameleonDB   string      `xml:"chameleondb"`
	SearchServer  string      `xml:"searchserver"`
	ManagerStudio string      `xml:"managerstudio"`
	Wopi          string      `xml:"wopi"`
	LQS           string      `xml:"LQS"`
	Tsdbase       string      `xml:"tsdbase"`
	Package       AsusPackage `xml:"package"`
	Time          int64       `xml:"time"`
}

type AsusPackage struct {
	Id                string `xml:"id"`
	Display           string `xml:"display"`
	Capacity          int    `xml:"capacity"`
	UploadBandwidth   int    `xml:"uploadbandwidth"`
	DownloadBandwidth int    `xml:"downloadbandwidth"`
	Upload            int    `xml:"upload"`
	Download          int    `xml:"download"`
	ConcurrentSession int    `xml:"concurrentsession"`
	MaxFileSize       int    `xml:"maxfilesize"`
	ShareGroup        int    `xml:"sharegroup"`
	HasEncryption     int    `xml:"hasencryption"`
	Expire            string `xml:"expire"`
	MaxBackupPC       int    `xml:"maxbackuppc"`
	FeatureList       struct {
		Features []struct {
			Name       string `xml:"name,attr"`
			Enable     int    `xml:"enable,attr"`
			Properties []struct {
				Name  string `xml:"name,attr"`
				Value string `xml:"value,attr"`
			} `xml:"property"`
		} `xml:"feature"`
	} `xml:"featurelist"`
	PackageAttrs string `xml:"packageattrs"`
}

func (a *AsusAPI) RequestToken(ctx context.Context) (RequestTokenResponse, error) {
	req := RequestTokenRequest{
		UserId: a.userid,
	}

	if a.symmetrypwd != "" {
		req.SymmetryPwd = a.symmetrypwd
	} else {
		req.Password = getAsusHashedPassword(a.password)
	}

	var resp RequestTokenResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.servicegateway, "/member/requesttoken/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting RequestToken, return code %v", resp.Status)
	}
	a.sessionConfig = resp

	return resp, nil
}

type GetInfoRequest struct {
	XMLName xml.Name `xml:"getinfo"`
	UserId  string   `xml:"userid"`
	Token   string   `xml:"token"`
}

type GetInfoResponse struct {
	XMLName          xml.Name    `xml:"getinfo"`
	Status           int         `xml:"status"`
	Email            string      `xml:"email"`
	RegYear          int         `xml:"regyear"`
	Language         string      `xml:"language"`
	ActivateDate     string      `xml:"activatedate"`
	CredentialState  int         `xml:"credentialstate"`
	UsedBackupPC     int         `xml:"usedbackuppc"`
	Package          AsusPackage `xml:"package"`
	UsedCapacity     int         `xml:"usedcapacity"`
	FreeCapacity     int         `xml:"freecapacity"`
	IsEmailConfirmed int         `xml:"isemailconfirmed"`
	DiskFreeSpace    int         `xml:"diskfreespace"`
	PackageAttrs     string      `xml:"packageattrs"`
}

func (a *AsusAPI) GetInfo(ctx context.Context) (GetInfoResponse, error) {
	req := GetInfoRequest{
		UserId: a.userid,
		Token:  a.sessionConfig.Token,
	}
	var resp GetInfoResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.servicegateway, "/member/getinfo/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting GetInfo, return code %v", resp.Status)
	}

	return resp, nil
}

type GetMySyncFolderRequest struct {
	XMLName xml.Name `xml:"getmysyncfolder"`
	UserId  string   `xml:"userid"`
	Token   string   `xml:"token"`
}

type GetMySyncFolderResponse struct {
	XMLName xml.Name `xml:"getmysyncfolder"`
	Status  int      `xml:"status"`
	Id      string   `xml:"id"`
}

func (a *AsusAPI) GetMySyncFolder(ctx context.Context) (GetMySyncFolderResponse, error) {
	req := GetMySyncFolderRequest{
		UserId: a.userid,
		Token:  a.sessionConfig.Token,
	}
	var resp GetMySyncFolderResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/folder/getmysyncfolder/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting GetMySyncFolder, return code %v", resp.Status)
	}

	return resp, nil
}

type BrowseFolderRequest struct {
	XMLName   xml.Name    `xml:"browse"`
	UserId    string      `xml:"userid"`
	Token     string      `xml:"token"`
	Language  string      `xml:"language"`
	FolderId  string      `xml:"folderid"`
	Page      RequestPage `xml:"page"`
	IsBilling int         `xml:"issibiling"`
}

type RequestPage struct {
	PageNo   int `xml:"pageno"`
	PageSize int `xml:"pagesize"`
	Enable   int `xml:"enable"`
}

type BrowseFolderResponse struct {
	XMLName       xml.Name     `xml:"browse"`
	Status        int          `xml:"status"`
	LogMessage    string       `xml:"logmessage"`
	Scrip         int64        `xml:"scrip"`
	RawFolderName string       `xml:"rawfoldername"`
	ParentId      string       `xml:"parent"`
	RootFolderId  string       `xml:"rootfolderid"`
	Page          PageResponse `xml:"page"`
	File          []File       `xml:"file"`
	Folder        []Folder     `xml:"folder"`
	Owner         string       `xml:"owner"`
}

type PageResponse struct {
	PageNo      int `xml:"pageno"`
	PageSize    int `xml:"pagesize"`
	TotalCount  int `xml:"totalcount"`
	HasNextPage int `xml:"hasnextpage"`
}

type File struct {
	Id               string `xml:"id"`
	IsGroupAware     int    `xml:"isgroupaware"`
	RawFileName      string `xml:"rawfilename"`
	Size             uint64 `xml:"size"`
	IsBackup         int    `xml:"isbackup"`
	IsOrigDeleted    int    `xml:"isorigdeleted"`
	IsInfected       int    `xml:"isinfected"`
	IsPublic         int    `xml:"ispublic"`
	HeadVersion      int    `xml:"headversion"`
	CreatedTime      string `xml:"createdtime"`
	LastModifiedTime string `xml:"lastmodifiedtime"`
	Contributor      string `xml:"contributor"`
	IsPrivacyRisk    int    `xml:"isprivacyrisk"`
	IsPrivacySuspect int    `xml:"isprivacysuspect"`
	StorageType      int    `xml:"storageType"`
}

type Folder struct {
	Id            string `xml:"id"`
	TreeSize      int    `xml:"treesize"`
	IsGroupAware  int    `xml:"isgroupaware"`
	RawFolderName string `xml:"rawfoldername"`
	IsBackup      int    `xml:"isbackup"`
	IsOrigDeleted int    `xml:"isorigdeleted"`
	IsPublic      int    `xml:"ispublic"`
	CreatedTime   string `xml:"createdtime"`
	Contributor   string `xml:"contributor"`
}

func (a *AsusAPI) BrowseFolder(ctx context.Context, id string, offset int, itemsperpage int) (BrowseFolderResponse, error) {
	req := BrowseFolderRequest{
		UserId:   a.userid,
		Token:    a.sessionConfig.Token,
		FolderId: id,
		Page: RequestPage{
			PageNo:   offset + 1,   // 1
			PageSize: itemsperpage, // 200
			Enable:   0,
		},
	}
	var resp BrowseFolderResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/inforelay/browsefolder/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting BrowseFolder, return code %v", resp.Status)
	}

	return resp, nil
}

type FolderTreeResponse struct {
	XMLName xml.Name      `xml:"tree"`
	Status  int           `xml:"status"`
	Entries []FolderEntry `xml:"entries>entry"`
}

type FolderEntry struct {
	FolderId int           `xml:"folderid"`
	Display  string        `xml:"display"` //base64-encoded value
	Entries  []FolderEntry `xml:"entries"`
}

// FolderTree call should send malformed xml, so such is implemented
func (a *AsusAPI) FolderTree(ctx context.Context, folderid string, depth int) (FolderTreeResponse, error) {
	xmlstr := fmt.Sprintf("<tree><userid>%s</userid><token>%s</token><folderid>%v<depth>%v</depth></folderid></tree>",
		a.userid, a.sessionConfig.Token, folderid, depth)

	opts := rest.Opts{
		RootURL: fmt.Sprintf("https://%s", a.sessionConfig.InfoRelay),
		Method:  "POST",
		Path:    "/folder/tree/",
		ExtraHeaders: map[string]string{
			"Cookie": a.cookie,
		},
		Body: bytes.NewBufferString(xmlstr),
	}

	rawresp, err := a.srv.Call(ctx, &opts)

	var resp FolderTreeResponse
	resp.Status = -1
	err = rest.DecodeXML(rawresp, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting FolderTree, return code %v", resp.Status)
	}

	return resp, nil
}

type GetEntryInfoRequest struct {
	XMLName  xml.Name `xml:"getentryinfo"`
	UserId   string   `xml:"userid"`
	Token    string   `xml:"token"`
	EntryId  string   `xml:"entryid"`
	IsFolder int      `xml:"isfolder"`
}

type GetEntryInfoResponse struct {
	XMLName          xml.Name        `xml:"getentryinfo"`
	Status           int             `xml:"status"`
	ScrIp            int64           `xml:"scrip"`
	Isfolder         int             `xml:"isfolder"`
	Display          string          `xml:"display"`
	ParentId         string          `xml:"parent"`
	HeadVersion      int             `xml:"headversion"`
	Attribute        ObjectAttribute `xml:"attribute"`
	MimeType         string          `xml:"mimetype"`
	FileSize         int             `xml:"filesize"`
	IsInfected       int             `xml:"isinfected"`
	IsBackup         int             `xml:"isbackup"`
	IsOrigDeleted    int             `xml:"isorigdeleted"`
	IsPublic         int             `xml:"ispublic"`
	CreatedTime      string          `xml:"createdtime"`
	Contributor      string          `xml:"contributor"`
	IsGroupAware     int             `xml:"isgroupaware"`
	IsPrivacyRisk    int             `xml:"isprivacyrisk"`
	Owner            string          `xml:"owner"`
	IsPrivacySuspect int             `xml:"isprivacysuspect"`
}

type ObjectAttribute struct {
	CreationTime      int    `xml:"creationtime"`
	LastAccessTime    int    `xml:"lastaccesstime"`
	LastWriteTime     int    `xml:"lastwritetime"`
	Finfo             string `xml:"finfo,omitempty"`
	XTimeForSyncCheck int    `xml:"x-timeforsynccheck,omitempty"`
	XMachineName      string `xml:"x-machinename,omitempty"`
}

func (a *AsusAPI) GetEntryInfo(ctx context.Context, id string) (GetEntryInfoResponse, error) {
	req := GetEntryInfoRequest{
		UserId:   a.userid,
		Token:    a.sessionConfig.Token,
		EntryId:  id,
		IsFolder: 0,
	}
	var resp GetEntryInfoResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/fsentry/getentryinfo/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting GetEntryInfo, return code %v", resp.Status)
	}

	return resp, nil
}

type PropFindRequest struct {
	XMLName  xml.Name `xml:"propfind"`
	UserId   string   `xml:"userid"`
	Token    string   `xml:"token"`
	Find     string   `xml:"find"` // base64-encoded name
	ParentId string   `xml:"parent"`
	Type     string   `xml:"type"`
}

type PropFindResponse struct {
	XMLName      xml.Name `xml:"propfind"`
	Status       int      `xml:"status"`
	IsEncrypt    int      `xml:"isencrypt"`
	Size         int      `xml:"size"`
	ScrIp        int64    `xml:"scrip"`
	Type         string   `xml:"type"`
	Id           string   `xml:"id"`
	IsGroupaware int      `xml:"isgroupaware"`
	Owner        string   `xml:"owner"`
}

func (a *AsusAPI) PropFind(ctx context.Context, name string, parentid string, objtype string) (PropFindResponse, error) {
	req := PropFindRequest{
		UserId:   a.userid,
		Token:    a.sessionConfig.Token,
		Find:     base64.StdEncoding.EncodeToString([]byte(name)),
		ParentId: parentid,
		Type:     objtype,
	}
	var resp PropFindResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/find/propfind/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting PropFind, return code %v", resp.Status)
	}

	return resp, nil
}

type CreateFolderRequest struct {
	XMLName        xml.Name `xml:"create"`
	UserId         string   `xml:"userid"`
	Token          string   `xml:"token"`
	ParentId       string   `xml:"parent"`
	IsEncrypted    int      `xml:"isencrypted"`
	Display        string   `xml:"display"`
	CreationTime   int      `xml:"attribute>creationtime"`
	LastAccessTime int      `xml:"attribute>lastaccesstime"`
	LastWriteTime  int      `xml:"attribute>lastwritetime"`
	// Attribute   ObjectAttribute `xml:"attribute"`
}

type CreateFolderResponse struct {
	XMLName xml.Name `xml:"create"`
	Status  int      `xml:"status"`
	SrcIp   int      `xml:"srcip"`
	Message string   `xml:"message"`
	Id      string   `xml:"id"`
}

func (a *AsusAPI) CreateFolder(ctx context.Context, name string, parentid string, timestamp int) (CreateFolderResponse, error) {
	req := CreateFolderRequest{
		UserId:         a.userid,
		Token:          a.sessionConfig.Token,
		ParentId:       parentid,
		Display:        base64.StdEncoding.EncodeToString([]byte(name)),
		CreationTime:   timestamp,
		LastAccessTime: timestamp,
		LastWriteTime:  timestamp,
	}
	var resp CreateFolderResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/folder/create/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting CreateFolder, return code %v", resp.Status)
	}

	return resp, nil
}

type RemoveFolderRequest struct {
	XMLName     xml.Name `xml:"remove"`
	UserId      string   `xml:"userid"`
	Token       string   `xml:"token"`
	Id          string   `xml:"id"`
	IsEncrypted int      `xml:"isencrypted"`
	IsSharing   int      `xml:"issharing"`
}

type RemoveFolderResponse struct {
	XMLName    xml.Name `xml:"remove"`
	Status     int      `xml:"status"`
	SrcIp      int      `xml:"srcip"`
	Message    string   `xml:"message"`
	LogMessage string   `xml:"logmessage"`
}

func (a *AsusAPI) RemoveFolder(ctx context.Context, id string) (RemoveFolderResponse, error) {
	req := RemoveFolderRequest{
		UserId: a.userid,
		Token:  a.sessionConfig.Token,
		Id:     id,
	}
	var resp RemoveFolderResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/folder/remove/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting RemoveFolder, return code %v", resp.Status)
	}

	return resp, nil
}

type RemoveFileRequest struct {
	XMLName     xml.Name `xml:"remove"`
	UserId      string   `xml:"userid"`
	Token       string   `xml:"token"`
	Id          string   `xml:"id"`
	IsEncrypted int      `xml:"isencrypted"`
	IsSharing   int      `xml:"issharing"`
}

type RemoveFileResponse struct {
	XMLName    xml.Name `xml:"remove"`
	Status     int      `xml:"status"`
	SrcIp      int      `xml:"srcip"`
	Message    string   `xml:"message"`
	LogMessage string   `xml:"logmessage"`
}

func (a *AsusAPI) RemoveFile(ctx context.Context, id string) (RemoveFileResponse, error) {
	req := RemoveFileRequest{
		UserId: a.userid,
		Token:  a.sessionConfig.Token,
		Id:     id,
	}
	var resp RemoveFileResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/file/remove/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting RemoveFile, return code %v", resp.Status)
	}

	return resp, nil
}

type RenameFolderRequest struct {
	XMLName     xml.Name `xml:"rename"`
	UserId      string   `xml:"userid"`
	Token       string   `xml:"token"`
	Id          string   `xml:"id"`
	Display     string   `xml:"display"`
	IsEncrypted int      `xml:"isencrypted"`
	IsSharing   string   `xml:"issharing"` // empty value
}

type RenameFolderResponse struct {
	XMLName    xml.Name `xml:"rename"`
	Status     int      `xml:"status"`
	SrcIp      int      `xml:"srcip"`
	LogMessage string   `xml:"logmessage"`
}

func (a *AsusAPI) RenameFolder(ctx context.Context, id string, newname string) (RenameFolderResponse, error) {
	req := RenameFolderRequest{
		UserId:  a.userid,
		Token:   a.sessionConfig.Token,
		Id:      id,
		Display: base64.StdEncoding.EncodeToString([]byte(newname)),
	}
	var resp RenameFolderResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/folder/rename/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting RenameFolder, return code %v", resp.Status)
	}

	return resp, nil
}

type RenameFileRequest struct {
	XMLName     xml.Name `xml:"rename"`
	UserId      string   `xml:"userid"`
	Token       string   `xml:"token"`
	Id          string   `xml:"id"`
	Display     string   `xml:"display"`
	IsEncrypted int      `xml:"isencrypted"`
	IsSharing   string   `xml:"issharing"` // empty value, should be integer?
	// SrcIp      int      `xml:"srcip"` ???
}

type RenameFileResponse struct {
	XMLName    xml.Name `xml:"rename"`
	Status     int      `xml:"status"`
	SrcIp      int      `xml:"srcip"`
	LogMessage string   `xml:"logmessage"`
}

func (a *AsusAPI) RenameFile(ctx context.Context, id string, newname string) (RenameFileResponse, error) {
	req := RenameFileRequest{
		UserId:  a.userid,
		Token:   a.sessionConfig.Token,
		Id:      id,
		Display: base64.StdEncoding.EncodeToString([]byte(newname)),
	}
	var resp RenameFileResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/file/rename/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting RenameFile, return code %v", resp.Status)
	}

	return resp, nil
}

type MoveFolderRequest struct {
	XMLName     xml.Name `xml:"move"`
	UserId      string   `xml:"userid"`
	Token       string   `xml:"token"`
	Id          string   `xml:"id"`
	ParentId    string   `xml:"parent"`
	IsEncrypted int      `xml:"isencrypted"`
	IsSharing   string   `xml:"issharing"` // empty value
}

type MoveFolderResponse struct {
	XMLName    xml.Name `xml:"move"`
	Status     int      `xml:"status"`
	SrcIp      int      `xml:"srcip"`
	LogMessage string   `xml:"logmessage"`
}

func (a *AsusAPI) MoveFolder(ctx context.Context, id string, newparentid string) (MoveFolderResponse, error) {
	req := MoveFolderRequest{
		UserId:   a.userid,
		Token:    a.sessionConfig.Token,
		Id:       id,
		ParentId: newparentid,
	}
	var resp MoveFolderResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/folder/move/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting MoveFolder, return code %v", resp.Status)
	}

	return resp, nil
}

type MoveFileRequest struct {
	XMLName     xml.Name `xml:"move"`
	UserId      string   `xml:"userid"`
	Token       string   `xml:"token"`
	Id          string   `xml:"id"`
	ParentId    string   `xml:"parent"`
	IsEncrypted int      `xml:"isencrypted"`
	IsSharing   string   `xml:"issharing"` // empty value, should be integer?
	// SrcIp      int      `xml:"srcip"` ???
}

type MoveFileResponse struct {
	XMLName    xml.Name `xml:"move"`
	Status     int      `xml:"status"`
	SrcIp      int      `xml:"srcip"`
	LogMessage string   `xml:"logmessage"`
}

func (a *AsusAPI) MoveFile(ctx context.Context, id string, newparentid string) (MoveFileResponse, error) {
	req := MoveFileRequest{
		UserId:   a.userid,
		Token:    a.sessionConfig.Token,
		Id:       id,
		ParentId: newparentid,
	}
	var resp MoveFileResponse
	resp.Status = -1
	_, err := a.callXML(ctx, a.sessionConfig.InfoRelay, "/file/move/", &req, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting MoveFile, return code %v", resp.Status)
	}

	return resp, nil
}

type InitBinaryUploadResponse struct {
	XMLName        xml.Name `xml:"initbinaryupload"`
	Status         int      `xml:"status"`
	TransactionId  string   `xml:"transid"`
	Offset         string   `xml:"offset"`
	LatestCheckSum string   `xml:"latestchecksum"`
}

func (a *AsusAPI) InitBinaryUpload(ctx context.Context, folderid string, name string, id string, filesize *int, timestamp *int) (InitBinaryUploadResponse, error) {
	opts := rest.Opts{
		RootURL: fmt.Sprintf("https://%s", a.sessionConfig.WebRelay),
		Method:  "GET",
		Path:    "/webrelay/initbinaryupload/",
		Parameters: url.Values{
			"tk": []string{a.sessionConfig.Token},
			"na": []string{base64.StdEncoding.EncodeToString([]byte(name))},
			"pa": []string{folderid},
			// "dis": []string{strconv.Itoa(a.sid)},
			"sg": []string{""},
		},
		ExtraHeaders: map[string]string{
			"Cookie": a.cookie,
		},
	}

	if id != "" {
		opts.Parameters["fi"] = []string{id}
	}
	if filesize != nil {
		opts.Parameters["fs"] = []string{strconv.Itoa(*filesize)}
	}

	if timestamp != nil {
		opts.Parameters["at"] = []string{fmt.Sprintf("<creationtime>%v</creationtime><lastwritetime>%v</lastwritetime><lastaccesstime>%v</lastaccesstime>", *timestamp, *timestamp, *timestamp)}
	}

	var resp InitBinaryUploadResponse
	resp.Status = -1

	rawresp := &http.Response{}
	err := error(nil)

	if a.pacer != nil {
		err = a.pacer.Call(func() (bool, error) {
			rawresp, err = a.srv.Call(ctx, &opts)
			return shouldRetry(ctx, rawresp, err)
		})
	} else {
		rawresp, err = a.srv.Call(ctx, &opts)
	}
	if err != nil {
		return resp, err
	}

	err = rest.DecodeXML(rawresp, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting InitBinaryUpload, return code %v", resp.Status)
	}

	return resp, nil
}

type ResumeBinaryUploadResponse struct {
	XMLName xml.Name `xml:"resumebinaryupload"`
	Status  int      `xml:"status"`
}

func (a *AsusAPI) ResumeBinaryUpload(ctx context.Context, transaction string, offset int, size int, data io.Reader) (ResumeBinaryUploadResponse, error) {
	opts := rest.Opts{
		RootURL: fmt.Sprintf("https://%s", a.sessionConfig.WebRelay),
		Method:  "POST",
		Path:    "/webrelay/resumebinaryupload/",
		Parameters: url.Values{
			"tk": []string{a.sessionConfig.Token},
			"tx": []string{transaction},
		},
		ExtraHeaders: map[string]string{
			"Cookie": a.cookie,
			// "X-Omni-Stream-Offset": strconv.Itoa(offset),
			// "Content-Length":       strconv.Itoa(size),
		},
		Body: data,
		// ContentLength: size,
	}

	var resp ResumeBinaryUploadResponse
	resp.Status = -1

	rawresp := &http.Response{}
	err := error(nil)

	if a.pacer != nil {
		err = a.pacer.Call(func() (bool, error) {
			rawresp, err = a.srv.Call(ctx, &opts)
			return shouldRetry(ctx, rawresp, err)
		})
	} else {
		rawresp, err = a.srv.Call(ctx, &opts)
	}
	if err != nil {
		return resp, err
	}

	err = rest.DecodeXML(rawresp, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting ResumeBinaryUpload, return code %v", resp.Status)
	}

	return resp, nil
}

type FinishBinaryUploadResponse struct {
	XMLName xml.Name `xml:"finishbinaryupload"`
	Status  int      `xml:"status"`
	FileId  string   `xml:"fileid"`
}

func (a *AsusAPI) FinishBinaryUpload(ctx context.Context, transaction string, recordedchecksum *string) (FinishBinaryUploadResponse, error) {
	opts := rest.Opts{
		RootURL: fmt.Sprintf("https://%s", a.sessionConfig.WebRelay),
		Method:  "GET",
		Path:    "/webrelay/finishbinaryupload/",
		Parameters: url.Values{
			"tk": []string{a.sessionConfig.Token},
			"tx": []string{transaction},
			// "slg": []string{recordedchecksum},
		},
		ExtraHeaders: map[string]string{
			"Cookie": a.cookie,
		},
	}

	if recordedchecksum != nil && *recordedchecksum != "" {
		opts.Parameters["lsg"] = []string{*recordedchecksum}
	}

	var resp FinishBinaryUploadResponse
	resp.Status = -1

	rawresp := &http.Response{}
	err := error(nil)

	if a.pacer != nil {
		err = a.pacer.Call(func() (bool, error) {
			rawresp, err = a.srv.Call(ctx, &opts)
			return shouldRetry(ctx, rawresp, err)
		})
	} else {
		rawresp, err = a.srv.Call(ctx, &opts)
	}
	if err != nil {
		return resp, err
	}

	err = rest.DecodeXML(rawresp, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Status != 0 {
		return resp, fmt.Errorf("Error requesting FinishBinaryUpload, return code %v", resp.Status)
	}

	return resp, nil
}

func (a *AsusAPI) DirectDownload(ctx context.Context, id string, start int64, end int64) (io.ReadCloser, error) {
	opts := rest.Opts{
		RootURL: fmt.Sprintf("https://%s", a.sessionConfig.WebRelay),
		Method:  "GET",
		Path:    fmt.Sprintf("/webrelay/directdownload/%s/", a.sessionConfig.Token),
		Parameters: url.Values{
			"fi":  []string{id},
			"pv":  []string{"0"},
			"u":   []string{""},
			"of":  []string{""},
			"rn":  []string{""},
			"dis": []string{strconv.Itoa(a.sid)},
		},
		ExtraHeaders: map[string]string{
			"Cookie": a.cookie,
			// "Range":  fmt.Sprintf("bytes=%v-%v", offset, offset+size-1),
			"Range": fmt.Sprintf("bytes=%v-%v", start, end),
		},
	}

	rawresp := &http.Response{}
	err := error(nil)

	if a.pacer != nil {
		err = a.pacer.Call(func() (bool, error) {
			rawresp, err = a.srv.Call(ctx, &opts)
			return shouldRetry(ctx, rawresp, err)
		})
	} else {
		rawresp, err = a.srv.Call(ctx, &opts)
	}
	if err != nil {
		return nil, err
	}

	return rawresp.Body, err
}
