package api

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

const (
	defaultZone        = "com.apple.CloudDocs"
	statusOk           = "OK"
	statusEtagConflict = "ETAG_CONFLICT"
)

// DriveService represents an iCloud Drive service.
type DriveService struct {
	icloud       *Client
	RootID       string
	endpoint     string
	docsEndpoint string
}

// NewDriveService creates a new DriveService instance.
func NewDriveService(icloud *Client) (*DriveService, error) {
	return &DriveService{icloud: icloud, RootID: "FOLDER::com.apple.CloudDocs::root", endpoint: icloud.Session.AccountInfo.Webservices["drivews"].URL, docsEndpoint: icloud.Session.AccountInfo.Webservices["docws"].URL}, nil
}

// GetItemByDriveID retrieves a DriveItem by its Drive ID.
func (d *DriveService) GetItemByDriveID(ctx context.Context, id string, includeChildren bool) (*DriveItem, *http.Response, error) {
	items, resp, err := d.GetItemsByDriveID(ctx, []string{id}, includeChildren)
	if err != nil {
		return nil, resp, err
	}
	return items[0], resp, err
}

// GetItemsByDriveID retrieves DriveItems by their Drive IDs.
func (d *DriveService) GetItemsByDriveID(ctx context.Context, ids []string, includeChildren bool) ([]*DriveItem, *http.Response, error) {
	var err error
	_items := []map[string]any{}
	for _, id := range ids {
		_items = append(_items, map[string]any{
			"drivewsid":        id,
			"partialData":      false,
			"includeHierarchy": false,
		})
	}

	var body *bytes.Reader
	var path string
	if !includeChildren {
		values := []map[string]any{{
			"items": _items,
		}}
		body, err = IntoReader(values)
		if err != nil {
			return nil, nil, err
		}
		path = "/retrieveItemDetails"
	} else {
		values := _items
		body, err = IntoReader(values)
		if err != nil {
			return nil, nil, err
		}
		path = "/retrieveItemDetailsInFolders"
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         path,
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.endpoint,
		Body:         body,
	}
	var items []*DriveItem
	resp, err := d.icloud.Request(ctx, opts, nil, &items)
	if err != nil {
		return nil, resp, err
	}

	return items, resp, err
}

// GetDocByPath retrieves a document by its path.
func (d *DriveService) GetDocByPath(ctx context.Context, path string) (*Document, *http.Response, error) {
	values := url.Values{}
	values.Set("unified_format", "false")
	body, err := IntoReader(path)
	if err != nil {
		return nil, nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/ws/" + defaultZone + "/list/lookup_by_path",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Parameters:   values,
		Body:         body,
	}
	var item []*Document
	resp, err := d.icloud.Request(ctx, opts, nil, &item)
	if err != nil {
		return nil, resp, err
	}

	return item[0], resp, err
}

// GetItemByPath retrieves a DriveItem by its path.
func (d *DriveService) GetItemByPath(ctx context.Context, path string) (*DriveItem, *http.Response, error) {
	values := url.Values{}
	values.Set("unified_format", "true")

	body, err := IntoReader(path)
	if err != nil {
		return nil, nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/ws/" + defaultZone + "/list/lookup_by_path",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Parameters:   values,
		Body:         body,
	}
	var item []*DriveItem
	resp, err := d.icloud.Request(ctx, opts, nil, &item)
	if err != nil {
		return nil, resp, err
	}

	return item[0], resp, err
}

// GetDocByItemID retrieves a document by its item ID.
func (d *DriveService) GetDocByItemID(ctx context.Context, id string) (*Document, *http.Response, error) {
	values := url.Values{}
	values.Set("document_id", id)
	values.Set("unified_format", "false") // important
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/ws/" + defaultZone + "/list/lookup_by_id",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Parameters:   values,
	}
	var item *Document
	resp, err := d.icloud.Request(ctx, opts, nil, &item)
	if err != nil {
		return nil, resp, err
	}

	return item, resp, err
}

// GetItemRawByItemID retrieves a DriveItemRaw by its item ID.
func (d *DriveService) GetItemRawByItemID(ctx context.Context, id string) (*DriveItemRaw, *http.Response, error) {
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/v1/item/" + id,
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
	}
	var item *DriveItemRaw
	resp, err := d.icloud.Request(ctx, opts, nil, &item)
	if err != nil {
		return nil, resp, err
	}

	return item, resp, err
}

// GetItemsInFolder retrieves a list of DriveItemRaw objects in a folder with the given ID.
func (d *DriveService) GetItemsInFolder(ctx context.Context, id string, limit int64) ([]*DriveItemRaw, *http.Response, error) {
	values := url.Values{}
	values.Set("limit", strconv.FormatInt(limit, 10))

	opts := rest.Opts{
		Method:       "GET",
		Path:         "/v1/enumerate/" + id,
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Parameters:   values,
	}

	items := struct {
		Items []*DriveItemRaw `json:"drive_item"`
	}{}

	resp, err := d.icloud.Request(ctx, opts, nil, &items)
	if err != nil {
		return nil, resp, err
	}

	return items.Items, resp, err
}

// GetDownloadURLByDriveID retrieves the download URL for a file in the DriveService.
func (d *DriveService) GetDownloadURLByDriveID(ctx context.Context, id string) (string, *http.Response, error) {
	_, zone, docid := DeconstructDriveID(id)
	values := url.Values{}
	values.Set("document_id", docid)

	if zone == "" {
		zone = defaultZone
	}

	opts := rest.Opts{
		Method:       "GET",
		Path:         "/ws/" + zone + "/download/by_id",
		Parameters:   values,
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
	}

	var filer *FileRequest
	resp, err := d.icloud.Request(ctx, opts, nil, &filer)

	if err != nil {
		return "", resp, err
	}

	var url string
	if filer.DataToken != nil {
		url = filer.DataToken.URL
	} else {
		url = filer.PackageToken.URL
	}

	return url, resp, err
}

// DownloadFile downloads a file from the given URL using the provided options.
func (d *DriveService) DownloadFile(ctx context.Context, url string, opt []fs.OpenOption) (*http.Response, error) {
	opts := &rest.Opts{
		Method:       "GET",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      url,
		Options:      opt,
	}

	resp, err := d.icloud.srv.Call(ctx, opts)
	// icloud has some weird http codes
	if err != nil && resp != nil && resp.StatusCode == 330 {
		loc, err := resp.Location()
		if err == nil {
			return d.DownloadFile(ctx, loc.String(), opt)
		}
	}
	return resp, err
}

// MoveItemToTrashByItemID moves an item to the trash based on the item ID.
func (d *DriveService) MoveItemToTrashByItemID(ctx context.Context, id, etag string, force bool) (*DriveItem, *http.Response, error) {
	doc, resp, err := d.GetDocByItemID(ctx, id)
	if err != nil {
		return nil, resp, err
	}
	return d.MoveItemToTrashByID(ctx, doc.DriveID(), etag, force)
}

// MoveItemToTrashByID moves an item to the trash based on the item ID.
func (d *DriveService) MoveItemToTrashByID(ctx context.Context, drivewsid, etag string, force bool) (*DriveItem, *http.Response, error) {
	values := map[string]any{
		"items": []map[string]any{{
			"drivewsid": drivewsid,
			"etag":      etag,
			"clientId":  drivewsid,
		}}}

	body, err := IntoReader(values)
	if err != nil {
		return nil, nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/moveItemsToTrash",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.endpoint,
		Body:         body,
	}

	item := struct {
		Items []*DriveItem `json:"items"`
	}{}
	resp, err := d.icloud.Request(ctx, opts, nil, &item)

	if err != nil {
		return nil, resp, err
	}

	if item.Items[0].Status != statusOk {
		// rerun with latest etag
		if force && item.Items[0].Status == "ETAG_CONFLICT" {
			return d.MoveItemToTrashByID(ctx, drivewsid, item.Items[0].Etag, false)
		}

		err = newRequestError(item.Items[0].Status, "unknown request status")
	}

	return item.Items[0], resp, err
}

// CreateNewFolderByItemID creates a new folder by item ID.
func (d *DriveService) CreateNewFolderByItemID(ctx context.Context, id, name string) (*DriveItem, *http.Response, error) {
	doc, resp, err := d.GetDocByItemID(ctx, id)
	if err != nil {
		return nil, resp, err
	}
	return d.CreateNewFolderByDriveID(ctx, doc.DriveID(), name)
}

// CreateNewFolderByDriveID creates a new folder by its Drive ID.
func (d *DriveService) CreateNewFolderByDriveID(ctx context.Context, drivewsid, name string) (*DriveItem, *http.Response, error) {
	values := map[string]any{
		"destinationDrivewsId": drivewsid,
		"folders": []map[string]any{{
			"clientId": "FOLDER::UNKNOWN_ZONE::TempId-" + uuid.New().String(),
			"name":     name,
		}},
	}

	body, err := IntoReader(values)
	if err != nil {
		return nil, nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/createFolders",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.endpoint,
		Body:         body,
	}
	var fResp *CreateFoldersResponse
	resp, err := d.icloud.Request(ctx, opts, nil, &fResp)
	if err != nil {
		return nil, resp, err
	}
	status := fResp.Folders[0].Status
	if status != statusOk {
		err = newRequestError(status, "unknown request status")
	}

	return fResp.Folders[0], resp, err
}

// RenameItemByItemID renames a DriveItem by its item ID.
func (d *DriveService) RenameItemByItemID(ctx context.Context, id, etag, name string, force bool) (*DriveItem, *http.Response, error) {
	doc, resp, err := d.GetDocByItemID(ctx, id)
	if err != nil {
		return nil, resp, err
	}
	return d.RenameItemByDriveID(ctx, doc.DriveID(), doc.Etag, name, force)
}

// RenameItemByDriveID renames a DriveItem by its drive ID.
func (d *DriveService) RenameItemByDriveID(ctx context.Context, id, etag, name string, force bool) (*DriveItem, *http.Response, error) {
	values := map[string]any{
		"items": []map[string]any{{
			"drivewsid": id,
			"name":      name,
			"etag":      etag,
			// "extension": split[1],
		}},
	}

	body, err := IntoReader(values)
	if err != nil {
		return nil, nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/renameItems",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.endpoint,
		Body:         body,
	}
	var items *DriveItem
	resp, err := d.icloud.Request(ctx, opts, nil, &items)

	if err != nil {
		return nil, resp, err
	}

	status := items.Items[0].Status
	if status != statusOk {
		// rerun with latest etag
		if force && status == "ETAG_CONFLICT" {
			return d.RenameItemByDriveID(ctx, id, items.Items[0].Etag, name, false)
		}
		err = newRequestErrorf(status, "unknown inner status for: %s %s", opts.Method, resp.Request.URL)
	}

	return items.Items[0], resp, err
}

// MoveItemByItemID moves an item by its item ID to a destination item ID.
func (d *DriveService) MoveItemByItemID(ctx context.Context, id, etag, dstID string, force bool) (*DriveItem, *http.Response, error) {
	docSrc, resp, err := d.GetDocByItemID(ctx, id)
	if err != nil {
		return nil, resp, err
	}
	docDst, resp, err := d.GetDocByItemID(ctx, dstID)
	if err != nil {
		return nil, resp, err
	}
	return d.MoveItemByDriveID(ctx, docSrc.DriveID(), docSrc.Etag, docDst.DriveID(), force)
}

// MoveItemByDocID moves an item by its doc ID.
// func (d *DriveService) MoveItemByDocID(ctx context.Context, srcDocID, srcEtag, dstDocID string, force bool) (*DriveItem, *http.Response, error) {
// 	return d.MoveItemByDriveID(ctx, srcDocID, srcEtag, docDst.DriveID(), force)
// }

// MoveItemByDriveID moves an item by its drive ID.
func (d *DriveService) MoveItemByDriveID(ctx context.Context, id, etag, dstID string, force bool) (*DriveItem, *http.Response, error) {
	values := map[string]any{
		"destinationDrivewsId": dstID,
		"items": []map[string]any{{
			"drivewsid": id,
			"etag":      etag,
			"clientId":  id,
		}},
	}

	body, err := IntoReader(values)
	if err != nil {
		return nil, nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/moveItems",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.endpoint,
		Body:         body,
	}

	var items *DriveItem
	resp, err := d.icloud.Request(ctx, opts, nil, &items)

	if err != nil {
		return nil, resp, err
	}

	status := items.Items[0].Status
	if status != statusOk {
		// rerun with latest etag
		if force && status == "ETAG_CONFLICT" {
			return d.MoveItemByDriveID(ctx, id, items.Items[0].Etag, dstID, false)
		}
		err = newRequestErrorf(status, "unknown inner status for: %s %s", opts.Method, resp.Request.URL)
	}

	return items.Items[0], resp, err
}

// CopyDocByItemID copies a document by its item ID.
func (d *DriveService) CopyDocByItemID(ctx context.Context, itemID string) (*DriveItemRaw, *http.Response, error) {
	// putting name in info doesn't work. extension does work so assume this is a bug in the endpoint
	values := map[string]any{
		"info_to_update": map[string]any{},
	}

	body, err := IntoReader(values)
	if err != nil {
		return nil, nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/v1/item/copy/" + itemID,
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Body:         body,
	}

	var info *DriveItemRaw
	resp, err := d.icloud.Request(ctx, opts, nil, &info)
	if err != nil {
		return nil, resp, err
	}
	return info, resp, err
}

// CreateUpload creates an url for an upload.
func (d *DriveService) CreateUpload(ctx context.Context, size int64, name string) (*UploadResponse, *http.Response, error) {
	// first we need to request an upload url
	values := map[string]any{
		"filename":     name,
		"type":         "FILE",
		"size":         strconv.FormatInt(size, 10),
		"content_type": GetContentTypeForFile(name),
	}
	body, err := IntoReader(values)
	if err != nil {
		return nil, nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/ws/" + defaultZone + "/upload/web",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Body:         body,
	}
	var responseInfo []*UploadResponse
	resp, err := d.icloud.Request(ctx, opts, nil, &responseInfo)
	if err != nil {
		return nil, resp, err
	}
	return responseInfo[0], resp, err
}

// Upload uploads a file to the given url
func (d *DriveService) Upload(ctx context.Context, in io.Reader, size int64, name, uploadURL string) (*SingleFileResponse, *http.Response, error) {
	// TODO: implement multipart upload
	opts := rest.Opts{
		Method:        "POST",
		ExtraHeaders:  d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:       uploadURL,
		Body:          in,
		ContentLength: &size,
		ContentType:   GetContentTypeForFile(name),
		// MultipartContentName: "files",
		MultipartFileName: name,
	}
	var singleFileResponse *SingleFileResponse
	resp, err := d.icloud.Request(ctx, opts, nil, &singleFileResponse)
	if err != nil {
		return nil, resp, err
	}
	return singleFileResponse, resp, err
}

// UpdateFile updates a file in the DriveService.
//
// ctx: the context.Context object for the request.
// r: a pointer to the UpdateFileInfo struct containing the information for the file update.
// Returns a pointer to the DriveItem struct representing the updated file, the http.Response object, and an error if any.
func (d *DriveService) UpdateFile(ctx context.Context, r *UpdateFileInfo) (*DriveItem, *http.Response, error) {
	body, err := IntoReader(r)
	if err != nil {
		return nil, nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/ws/" + defaultZone + "/update/documents",
		ExtraHeaders: d.icloud.Session.GetHeaders(map[string]string{}),
		RootURL:      d.docsEndpoint,
		Body:         body,
	}
	var responseInfo *DocumentUpdateResponse
	resp, err := d.icloud.Request(ctx, opts, nil, &responseInfo)
	if err != nil {
		return nil, resp, err
	}

	doc := responseInfo.Results[0].Document
	item := DriveItem{
		Drivewsid:    "FILE::com.apple.CloudDocs::" + doc.DocumentID,
		Docwsid:      doc.DocumentID,
		Itemid:       doc.ItemID,
		Etag:         doc.Etag,
		ParentID:     doc.ParentID,
		DateModified: time.Unix(r.Mtime, 0),
		DateCreated:  time.Unix(r.Mtime, 0),
		Type:         doc.Type,
		Name:         doc.Name,
		Size:         doc.Size,
	}

	return &item, resp, err
}

// UpdateFileInfo represents the information for an update to a file in the DriveService.
type UpdateFileInfo struct {
	AllowConflict   bool   `json:"allow_conflict"`
	Btime           int64  `json:"btime"`
	Command         string `json:"command"`
	CreateShortGUID bool   `json:"create_short_guid"`
	Data            struct {
		Receipt            string `json:"receipt,omitempty"`
		ReferenceSignature string `json:"reference_signature,omitempty"`
		Signature          string `json:"signature,omitempty"`
		Size               int64  `json:"size,omitempty"`
		WrappingKey        string `json:"wrapping_key,omitempty"`
	} `json:"data,omitempty"`
	DocumentID string    `json:"document_id"`
	FileFlags  FileFlags `json:"file_flags"`
	Mtime      int64     `json:"mtime"`
	Path       struct {
		Path               string `json:"path"`
		StartingDocumentID string `json:"starting_document_id"`
	} `json:"path"`
}

// FileFlags defines the file flags for a document.
type FileFlags struct {
	IsExecutable bool `json:"is_executable"`
	IsHidden     bool `json:"is_hidden"`
	IsWritable   bool `json:"is_writable"`
}

// NewUpdateFileInfo creates a new UpdateFileInfo object with default values.
//
// Returns an UpdateFileInfo object.
func NewUpdateFileInfo() UpdateFileInfo {
	return UpdateFileInfo{
		Command:         "add_file",
		CreateShortGUID: true,
		AllowConflict:   true,
		FileFlags: FileFlags{
			IsExecutable: true,
			IsHidden:     false,
			IsWritable:   true,
		},
	}
}

// DriveItemRaw is a raw drive item.
// not suure what to call this but there seems to be a "unified" and non "unified" drive item response. This is the non unified.
type DriveItemRaw struct {
	ItemID   string            `json:"item_id"`
	ItemInfo *DriveItemRawInfo `json:"item_info"`
}

// SplitName splits the name of a DriveItemRaw into its name and extension.
//
// It returns the name and extension as separate strings. If the name ends with a dot,
// it means there is no extension, so an empty string is returned for the extension.
// If the name does not contain a dot, it means
func (d *DriveItemRaw) SplitName() (string, string) {
	name := d.ItemInfo.Name
	// ends with a dot, no extension
	if strings.HasSuffix(name, ".") {
		return name, ""
	}
	lastInd := strings.LastIndex(name, ".")

	if lastInd == -1 {
		return name, ""
	}
	return name[:lastInd], name[lastInd+1:]
}

// ModTime returns the modification time of the DriveItemRaw.
//
// It parses the ModifiedAt field of the ItemInfo struct and converts it to a time.Time value.
// If the parsing fails, it returns the zero value of time.Time.
// The returned time.Time value represents the modification time of the DriveItemRaw.
func (d *DriveItemRaw) ModTime() time.Time {
	i, err := strconv.ParseInt(d.ItemInfo.ModifiedAt, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(i)
}

// CreatedTime returns the creation time of the DriveItemRaw.
//
// It parses the CreatedAt field of the ItemInfo struct and converts it to a time.Time value.
// If the parsing fails, it returns the zero value of time.Time.
// The returned time.Time
func (d *DriveItemRaw) CreatedTime() time.Time {
	i, err := strconv.ParseInt(d.ItemInfo.CreatedAt, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(i)
}

// DriveItemRawInfo is the raw information about a drive item.
type DriveItemRawInfo struct {
	Name string `json:"name"`
	// Extension is absolutely borked on endpoints so dont use it.
	Extension  string `json:"extension"`
	Size       int64  `json:"size,string"`
	Type       string `json:"type"`
	Version    string `json:"version"`
	ModifiedAt string `json:"modified_at"`
	CreatedAt  string `json:"created_at"`
	Urls       struct {
		URLDownload string `json:"url_download"`
	} `json:"urls"`
}

// IntoDriveItem converts a DriveItemRaw into a DriveItem.
//
// It takes no parameters.
// It returns a pointer to a DriveItem.
func (d *DriveItemRaw) IntoDriveItem() *DriveItem {
	name, extension := d.SplitName()
	return &DriveItem{
		Itemid:       d.ItemID,
		Name:         name,
		Extension:    extension,
		Type:         d.ItemInfo.Type,
		Etag:         d.ItemInfo.Version,
		DateModified: d.ModTime(),
		DateCreated:  d.CreatedTime(),
		Size:         d.ItemInfo.Size,
		Urls:         d.ItemInfo.Urls,
	}
}

// DocumentUpdateResponse is the response of a document update request.
type DocumentUpdateResponse struct {
	Status struct {
		StatusCode   int    `json:"status_code"`
		ErrorMessage string `json:"error_message"`
	} `json:"status"`
	Results []struct {
		Status struct {
			StatusCode   int    `json:"status_code"`
			ErrorMessage string `json:"error_message"`
		} `json:"status"`
		OperationID any       `json:"operation_id"`
		Document    *Document `json:"document"`
	} `json:"results"`
}

// Document represents a document on iCloud.
type Document struct {
	Status struct {
		StatusCode   int    `json:"status_code"`
		ErrorMessage string `json:"error_message"`
	} `json:"status"`
	DocumentID string `json:"document_id"`
	ItemID     string `json:"item_id"`
	Urls       struct {
		URLDownload string `json:"url_download"`
	} `json:"urls"`
	Etag           string       `json:"etag"`
	ParentID       string       `json:"parent_id"`
	Name           string       `json:"name"`
	Type           string       `json:"type"`
	Deleted        bool         `json:"deleted"`
	Mtime          int64        `json:"mtime"`
	LastEditorName string       `json:"last_editor_name"`
	Data           DocumentData `json:"data"`
	Size           int64        `json:"size"`
	Btime          int64        `json:"btime"`
	Zone           string       `json:"zone"`
	FileFlags      struct {
		IsExecutable bool `json:"is_executable"`
		IsWritable   bool `json:"is_writable"`
		IsHidden     bool `json:"is_hidden"`
	} `json:"file_flags"`
	LastOpenedTime   int64 `json:"lastOpenedTime"`
	RestorePath      any   `json:"restorePath"`
	HasChainedParent bool  `json:"hasChainedParent"`
}

// DriveID returns the drive ID of the Document.
func (d *Document) DriveID() string {
	if d.Zone == "" {
		d.Zone = defaultZone
	}
	return d.Type + "::" + d.Zone + "::" + d.DocumentID
}

// DocumentData represents the data of a document.
type DocumentData struct {
	Signature          string `json:"signature"`
	Owner              string `json:"owner"`
	Size               int64  `json:"size"`
	ReferenceSignature string `json:"reference_signature"`
	WrappingKey        string `json:"wrapping_key"`
	PcsInfo            string `json:"pcsInfo"`
}

// SingleFileResponse is the response of a single file request.
type SingleFileResponse struct {
	SingleFile *SingleFileInfo `json:"singleFile"`
}

// SingleFileInfo represents the information of a single file.
type SingleFileInfo struct {
	ReferenceSignature string `json:"referenceChecksum"`
	Size               int64  `json:"size"`
	Signature          string `json:"fileChecksum"`
	WrappingKey        string `json:"wrappingKey"`
	Receipt            string `json:"receipt"`
}

// UploadResponse is the response of an upload request.
type UploadResponse struct {
	URL        string `json:"url"`
	DocumentID string `json:"document_id"`
}

// FileRequestToken represents the token of a file request.
type FileRequestToken struct {
	URL                string `json:"url"`
	Token              string `json:"token"`
	Signature          string `json:"signature"`
	WrappingKey        string `json:"wrapping_key"`
	ReferenceSignature string `json:"reference_signature"`
}

// FileRequest represents the request of a file.
type FileRequest struct {
	DocumentID   string            `json:"document_id"`
	ItemID       string            `json:"item_id"`
	OwnerDsid    int64             `json:"owner_dsid"`
	DataToken    *FileRequestToken `json:"data_token,omitempty"`
	PackageToken *FileRequestToken `json:"package_token,omitempty"`
	DoubleEtag   string            `json:"double_etag"`
}

// CreateFoldersResponse is the response of a create folders request.
type CreateFoldersResponse struct {
	Folders []*DriveItem `json:"folders"`
}

// DriveItem represents an item on iCloud.
type DriveItem struct {
	DateCreated         time.Time    `json:"dateCreated"`
	Drivewsid           string       `json:"drivewsid"`
	Docwsid             string       `json:"docwsid"`
	Itemid              string       `json:"item_id"`
	Zone                string       `json:"zone"`
	Name                string       `json:"name"`
	ParentID            string       `json:"parentId"`
	Hierarchy           []DriveItem  `json:"hierarchy"`
	Etag                string       `json:"etag"`
	Type                string       `json:"type"`
	AssetQuota          int64        `json:"assetQuota"`
	FileCount           int64        `json:"fileCount"`
	ShareCount          int64        `json:"shareCount"`
	ShareAliasCount     int64        `json:"shareAliasCount"`
	DirectChildrenCount int64        `json:"directChildrenCount"`
	Items               []*DriveItem `json:"items"`
	NumberOfItems       int64        `json:"numberOfItems"`
	Status              string       `json:"status"`
	Extension           string       `json:"extension,omitempty"`
	DateModified        time.Time    `json:"dateModified,omitempty"`
	DateChanged         time.Time    `json:"dateChanged,omitempty"`
	Size                int64        `json:"size,omitempty"`
	LastOpenTime        time.Time    `json:"lastOpenTime,omitempty"`
	Urls                struct {
		URLDownload string `json:"url_download"`
	} `json:"urls"`
}

// IsFolder returns true if the item is a folder.
func (d *DriveItem) IsFolder() bool {
	return d.Type == "FOLDER" || d.Type == "APP_CONTAINER" || d.Type == "APP_LIBRARY"
}

// DownloadURL returns the download URL of the item.
func (d *DriveItem) DownloadURL() string {
	return d.Urls.URLDownload
}

// FullName returns the full name of the item.
// name + extension
func (d *DriveItem) FullName() string {
	if d.Extension != "" {
		return d.Name + "." + d.Extension
	}
	return d.Name
}

// GetDocIDFromDriveID returns the DocumentID from the drive ID.
func GetDocIDFromDriveID(id string) string {
	split := strings.Split(id, "::")
	return split[len(split)-1]
}

// DeconstructDriveID returns the document type, zone, and document ID from the drive ID.
func DeconstructDriveID(id string) (docType, zone, docid string) {
	split := strings.Split(id, "::")
	if len(split) < 3 {
		return "", "", id
	}
	return split[0], split[1], split[2]
}

// ConstructDriveID constructs a drive ID from the given components.
func ConstructDriveID(id string, zone string, t string) string {
	return strings.Join([]string{t, zone, id}, "::")
}

// GetContentTypeForFile detects content type for given file name.
func GetContentTypeForFile(name string) string {
	// detect MIME type by looking at the filename only
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		// api requires a mime type passed in
		mimeType = "text/plain"
	}
	return strings.Split(mimeType, ";")[0]
}
