/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

// Package dropbox implements the Dropbox core and datastore API.
package dropbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// ErrNotAuth is the error returned when the OAuth token is not provided
var ErrNotAuth = errors.New("authentication required")

// Account represents information about the user account.
type Account struct {
	ReferralLink string `json:"referral_link,omitempty"` // URL for referral.
	DisplayName  string `json:"display_name,omitempty"`  // User name.
	UID          int    `json:"uid,omitempty"`           // User account ID.
	Country      string `json:"country,omitempty"`       // Country ISO code.
	QuotaInfo    struct {
		Shared int64 `json:"shared,omitempty"` // Quota for shared files.
		Quota  int64 `json:"quota,omitempty"`  // Quota in bytes.
		Normal int64 `json:"normal,omitempty"` // Quota for non-shared files.
	} `json:"quota_info"`
}

// CopyRef represents the reply of CopyRef.
type CopyRef struct {
	CopyRef string `json:"copy_ref"` // Reference to use on fileops/copy.
	Expires string `json:"expires"`  // Expiration date.
}

// DeltaPage represents the reply of delta.
type DeltaPage struct {
	Reset   bool         // if true the local state must be cleared.
	HasMore bool         // if true an other call to delta should be made.
	Cursor               // Tag of the current state.
	Entries []DeltaEntry // List of changed entries.
}

// DeltaEntry represents the list of changes for a given path.
type DeltaEntry struct {
	Path  string // Path of this entry in lowercase.
	Entry *Entry // nil when this entry does not exists.
}

// DeltaPoll represents the reply of longpoll_delta.
type DeltaPoll struct {
	Changes bool `json:"changes"` // true if the polled path has changed.
	Backoff int  `json:"backoff"` // time in second before calling poll again.
}

// ChunkUploadResponse represents the reply of chunked_upload.
type ChunkUploadResponse struct {
	UploadID string `json:"upload_id"` // Unique ID of this upload.
	Offset   int64  `json:"offset"`    // Size in bytes of already sent data.
	Expires  DBTime `json:"expires"`   // Expiration time of this upload.
}

// Cursor represents the tag of a server state at a given moment.
type Cursor struct {
	Cursor string `json:"cursor"`
}

// Format of reply when http error code is not 200.
// Format may be:
// {"error": "reason"}
// {"error": {"param": "reason"}}
type requestError struct {
	Error interface{} `json:"error"` // Description of this error.
}

const (
	// PollMinTimeout is the minimum timeout for longpoll.
	PollMinTimeout = 30
	// PollMaxTimeout is the maximum timeout for longpoll.
	PollMaxTimeout = 480
	// DefaultChunkSize is the maximum size of a file sendable using files_put.
	DefaultChunkSize = 4 * 1024 * 1024
	// MaxPutFileSize is the maximum size of a file sendable using files_put.
	MaxPutFileSize = 150 * 1024 * 1024
	// MetadataLimitMax is the maximum number of entries returned by metadata.
	MetadataLimitMax = 25000
	// MetadataLimitDefault is the default number of entries returned by metadata.
	MetadataLimitDefault = 10000
	// RevisionsLimitMax is the maximum number of revisions returned by revisions.
	RevisionsLimitMax = 1000
	// RevisionsLimitDefault is the default number of revisions returned by revisions.
	RevisionsLimitDefault = 10
	// SearchLimitMax is the maximum number of entries returned by search.
	SearchLimitMax = 1000
	// SearchLimitDefault is the default number of entries returned by search.
	SearchLimitDefault = 1000
	// DateFormat is the format to use when decoding a time.
	DateFormat = time.RFC1123Z
)

// DBTime allow marshalling and unmarshalling of time.
type DBTime time.Time

// UnmarshalJSON unmarshals a time according to the Dropbox format.
func (dbt *DBTime) UnmarshalJSON(data []byte) error {
	var s string
	var err error
	var t time.Time

	if err = json.Unmarshal(data, &s); err != nil {
		return err
	}
	if t, err = time.ParseInLocation(DateFormat, s, time.UTC); err != nil {
		return err
	}
	if t.IsZero() {
		*dbt = DBTime(time.Time{})
	} else {
		*dbt = DBTime(t)
	}
	return nil
}

// MarshalJSON marshals a time according to the Dropbox format.
func (dbt DBTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(dbt).Format(DateFormat))
}

// Modifier represents the user who made a change on a particular file
type Modifier struct {
	UID         int64  `json:"uid"`
	DisplayName string `json:"display_name"`
}

// Entry represents the metadata of a file or folder.
type Entry struct {
	Bytes                int64     `json:"bytes,omitempty"`        // Size of the file in bytes.
	ClientMtime          DBTime    `json:"client_mtime,omitempty"` // Modification time set by the client when added.
	Contents             []Entry   `json:"contents,omitempty"`     // List of children for a directory.
	Hash                 string    `json:"hash,omitempty"`         // Hash of this entry.
	Icon                 string    `json:"icon,omitempty"`         // Name of the icon displayed for this entry.
	IsDeleted            bool      `json:"is_deleted,omitempty"`   // true if this entry was deleted.
	IsDir                bool      `json:"is_dir,omitempty"`       // true if this entry is a directory.
	MimeType             string    `json:"mime_type,omitempty"`    // MimeType of this entry.
	Modified             DBTime    `json:"modified,omitempty"`     // Date of last modification.
	Path                 string    `json:"path,omitempty"`         // Absolute path of this entry.
	Revision             string    `json:"rev,omitempty"`          // Unique ID for this file revision.
	Root                 string    `json:"root,omitempty"`         // dropbox or sandbox.
	Size                 string    `json:"size,omitempty"`         // Size of the file humanized/localized.
	ThumbExists          bool      `json:"thumb_exists,omitempty"` // true if a thumbnail is available for this entry.
	Modifier             *Modifier `json:"modifier"`               // last user to edit the file if in a shared folder
	ParentSharedFolderID string    `json:"parent_shared_folder_id,omitempty"`
}

// Link for sharing a file.
type Link struct {
	Expires DBTime `json:"expires"` // Expiration date of this link.
	URL     string `json:"url"`     // URL to share.
}

// User represents a Dropbox user.
type User struct {
	UID         int64  `json:"uid"`
	DisplayName string `json:"display_name"`
}

// SharedFolderMember represents access right associated with a Dropbox user.
type SharedFolderMember struct {
	User       User   `json:"user"`
	Active     bool   `json:"active"`
	AccessType string `json:"access_type"`
}

// SharedFolder reprensents a directory with a specific sharing policy.
type SharedFolder struct {
	SharedFolderID   string               `json:"shared_folder_id"`
	SharedFolderName string               `json:"shared_folder_name"`
	Path             string               `json:"path"`
	AccessType       string               `json:"access_type"`
	SharedLinkPolicy string               `json:"shared_link_policy"`
	Owner            User                 `json:"owner"`
	Membership       []SharedFolderMember `json:"membership"`
}

// Dropbox client.
type Dropbox struct {
	RootDirectory string // dropbox or sandbox.
	Locale        string // Locale sent to the API to translate/format messages.
	APIURL        string // Normal API URL.
	APIContentURL string // URL for transferring files.
	APINotifyURL  string // URL for realtime notification.
	config        *oauth2.Config
	token         *oauth2.Token
	ctx           context.Context
}

// NewDropbox returns a new Dropbox configured.
func NewDropbox() *Dropbox {
	db := &Dropbox{
		RootDirectory: "auto", // auto (recommended), dropbox or sandbox.
		Locale:        "en",
		APIURL:        "https://api.dropbox.com/1",
		APIContentURL: "https://api-content.dropbox.com/1",
		APINotifyURL:  "https://api-notify.dropbox.com/1",
		ctx:           oauth2.NoContext,
	}
	return db
}

// SetAppInfo sets the clientid (app_key) and clientsecret (app_secret).
// You have to register an application on https://www.dropbox.com/developers/apps.
func (db *Dropbox) SetAppInfo(clientid, clientsecret string) error {

	db.config = &oauth2.Config{
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.dropbox.com/1/oauth2/authorize",
			TokenURL: "https://api.dropbox.com/1/oauth2/token",
		},
	}
	return nil
}

// SetAccessToken sets access token to avoid calling Auth method.
func (db *Dropbox) SetAccessToken(accesstoken string) {
	db.token = &oauth2.Token{AccessToken: accesstoken}
}

// SetContext allow to set a custom context.
func (db *Dropbox) SetContext(ctx context.Context) {
	db.ctx = ctx
}

// AccessToken returns the OAuth access token.
func (db *Dropbox) AccessToken() string {
	return db.token.AccessToken
}

// SetRedirectURL updates the configuration with the given redirection URL.
func (db *Dropbox) SetRedirectURL(url string) {
	db.config.RedirectURL = url
}

func (db *Dropbox) client() *http.Client {
	return db.config.Client(db.ctx, db.token)
}

// Auth displays the URL to authorize this application to connect to your account.
func (db *Dropbox) Auth() error {
	var code string

	fmt.Printf("Please visit:\n%s\nEnter the code: ",
		db.config.AuthCodeURL(""))
	fmt.Scanln(&code)
	return db.AuthCode(code)
}

// AuthCode gets the token associated with the given code.
func (db *Dropbox) AuthCode(code string) error {
	t, err := db.config.Exchange(oauth2.NoContext, code)
	if err != nil {
		return err
	}

	db.token = t
	db.token.TokenType = "Bearer"
	return nil
}

// Error - all errors generated by HTTP transactions are of this type.
// Other error may be passed on from library functions though.
type Error struct {
	StatusCode int // HTTP status code
	Text       string
}

// Error satisfy the error interface.
func (e *Error) Error() string {
	return e.Text
}

// newError make a new error from a string.
func newError(StatusCode int, Text string) *Error {
	return &Error{
		StatusCode: StatusCode,
		Text:       Text,
	}
}

// newErrorf makes a new error from sprintf parameters.
func newErrorf(StatusCode int, Text string, Parameters ...interface{}) *Error {
	return newError(StatusCode, fmt.Sprintf(Text, Parameters...))
}

func getResponse(r *http.Response) ([]byte, error) {
	var e requestError
	var b []byte
	var err error

	if b, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, err
	}
	if r.StatusCode == http.StatusOK {
		return b, nil
	}
	if err = json.Unmarshal(b, &e); err == nil {
		switch v := e.Error.(type) {
		case string:
			return nil, newErrorf(r.StatusCode, "%s", v)
		case map[string]interface{}:
			for param, reason := range v {
				if reasonstr, ok := reason.(string); ok {
					return nil, newErrorf(r.StatusCode, "%s: %s", param, reasonstr)
				}
			}
			return nil, newErrorf(r.StatusCode, "wrong parameter")
		}
	}
	return nil, newErrorf(r.StatusCode, "unexpected HTTP status code %d", r.StatusCode)
}

// urlEncode encodes s for url
func urlEncode(s string) string {
	// Would like to call url.escape(value, encodePath) here
	encoded := url.QueryEscape(s)
	encoded = strings.Replace(encoded, "+", "%20", -1)
	return encoded
}

// CommitChunkedUpload ends the chunked upload by giving a name to the UploadID.
func (db *Dropbox) CommitChunkedUpload(uploadid, dst string, overwrite bool, parentRev string) (*Entry, error) {
	var err error
	var rawurl string
	var response *http.Response
	var params *url.Values
	var body []byte
	var rv Entry

	if dst[0] == '/' {
		dst = dst[1:]
	}

	params = &url.Values{
		"locale":    {db.Locale},
		"upload_id": {uploadid},
		"overwrite": {strconv.FormatBool(overwrite)},
	}
	if len(parentRev) != 0 {
		params.Set("parent_rev", parentRev)
	}
	rawurl = fmt.Sprintf("%s/commit_chunked_upload/%s/%s?%s", db.APIContentURL, db.RootDirectory, urlEncode(dst), params.Encode())

	if response, err = db.client().Post(rawurl, "", nil); err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if body, err = getResponse(response); err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &rv)
	return &rv, err
}

// ChunkedUpload sends a chunk with a maximum size of chunksize, if there is no session a new one is created.
func (db *Dropbox) ChunkedUpload(session *ChunkUploadResponse, input io.ReadCloser, chunksize int) (*ChunkUploadResponse, error) {
	var err error
	var rawurl string
	var cur ChunkUploadResponse
	var response *http.Response
	var body []byte
	var r *io.LimitedReader

	if chunksize <= 0 {
		chunksize = DefaultChunkSize
	} else if chunksize > MaxPutFileSize {
		chunksize = MaxPutFileSize
	}

	if session != nil {
		rawurl = fmt.Sprintf("%s/chunked_upload?upload_id=%s&offset=%d", db.APIContentURL, session.UploadID, session.Offset)
	} else {
		rawurl = fmt.Sprintf("%s/chunked_upload", db.APIContentURL)
	}
	r = &io.LimitedReader{R: input, N: int64(chunksize)}

	if response, err = db.client().Post(rawurl, "application/octet-stream", r); err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if body, err = getResponse(response); err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &cur)
	if r.N != 0 {
		err = io.EOF
	}
	return &cur, err
}

// UploadByChunk uploads data from the input reader to the dst path on Dropbox by sending chunks of chunksize.
func (db *Dropbox) UploadByChunk(input io.ReadCloser, chunksize int, dst string, overwrite bool, parentRev string) (*Entry, error) {
	var err error
	var cur *ChunkUploadResponse

	for err == nil {
		if cur, err = db.ChunkedUpload(cur, input, chunksize); err != nil && err != io.EOF {
			return nil, err
		}
	}
	return db.CommitChunkedUpload(cur.UploadID, dst, overwrite, parentRev)
}

// FilesPut uploads size bytes from the input reader to the dst path on Dropbox.
func (db *Dropbox) FilesPut(input io.ReadCloser, size int64, dst string, overwrite bool, parentRev string) (*Entry, error) {
	var err error
	var rawurl string
	var rv Entry
	var request *http.Request
	var response *http.Response
	var params *url.Values
	var body []byte

	if size > MaxPutFileSize {
		return nil, fmt.Errorf("could not upload files bigger than 150MB using this method, use UploadByChunk instead")
	}
	if dst[0] == '/' {
		dst = dst[1:]
	}

	params = &url.Values{"overwrite": {strconv.FormatBool(overwrite)}, "locale": {db.Locale}}
	if len(parentRev) != 0 {
		params.Set("parent_rev", parentRev)
	}
	rawurl = fmt.Sprintf("%s/files_put/%s/%s?%s", db.APIContentURL, db.RootDirectory, urlEncode(dst), params.Encode())

	if request, err = http.NewRequest("PUT", rawurl, input); err != nil {
		return nil, err
	}
	request.Header.Set("Content-Length", strconv.FormatInt(size, 10))
	if response, err = db.client().Do(request); err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if body, err = getResponse(response); err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &rv)
	return &rv, err
}

// UploadFile uploads the file located in the src path on the local disk to the dst path on Dropbox.
func (db *Dropbox) UploadFile(src, dst string, overwrite bool, parentRev string) (*Entry, error) {
	var err error
	var fd *os.File
	var fsize int64

	if fd, err = os.Open(src); err != nil {
		return nil, err
	}
	defer fd.Close()

	if fi, err := fd.Stat(); err == nil {
		fsize = fi.Size()
	} else {
		return nil, err
	}
	return db.FilesPut(fd, fsize, dst, overwrite, parentRev)
}

// Thumbnails gets a thumbnail for an image.
func (db *Dropbox) Thumbnails(src, format, size string) (io.ReadCloser, int64, *Entry, error) {
	var response *http.Response
	var rawurl string
	var err error
	var entry Entry

	switch format {
	case "":
		format = "jpeg"
	case "jpeg", "png":
		break
	default:
		return nil, 0, nil, fmt.Errorf("unsupported format '%s' must be jpeg or png", format)
	}
	switch size {
	case "":
		size = "s"
	case "xs", "s", "m", "l", "xl":
		break
	default:
		return nil, 0, nil, fmt.Errorf("unsupported size '%s' must be xs, s, m, l or xl", size)

	}
	if src[0] == '/' {
		src = src[1:]
	}
	rawurl = fmt.Sprintf("%s/thumbnails/%s/%s?format=%s&size=%s", db.APIContentURL, db.RootDirectory, urlEncode(src), urlEncode(format), urlEncode(size))
	if response, err = db.client().Get(rawurl); err != nil {
		return nil, 0, nil, err
	}
	if response.StatusCode == http.StatusOK {
		json.Unmarshal([]byte(response.Header.Get("x-dropbox-metadata")), &entry)
		return response.Body, response.ContentLength, &entry, err
	}
	response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotFound:
		return nil, 0, nil, os.ErrNotExist
	case http.StatusUnsupportedMediaType:
		return nil, 0, nil, newErrorf(response.StatusCode, "the image located at '%s' cannot be converted to a thumbnail", src)
	default:
		return nil, 0, nil, newErrorf(response.StatusCode, "unexpected HTTP status code %d", response.StatusCode)
	}
}

// ThumbnailsToFile downloads the file located in the src path on the Dropbox to the dst file on the local disk.
func (db *Dropbox) ThumbnailsToFile(src, dst, format, size string) (*Entry, error) {
	var input io.ReadCloser
	var fd *os.File
	var err error
	var entry *Entry

	if fd, err = os.Create(dst); err != nil {
		return nil, err
	}
	defer fd.Close()

	if input, _, entry, err = db.Thumbnails(src, format, size); err != nil {
		os.Remove(dst)
		return nil, err
	}
	defer input.Close()
	if _, err = io.Copy(fd, input); err != nil {
		os.Remove(dst)
	}
	return entry, err
}

// Download requests the file located at src, the specific revision may be given.
// offset is used in case the download was interrupted.
// A io.ReadCloser and the file size is returned.
func (db *Dropbox) Download(src, rev string, offset int64) (io.ReadCloser, int64, error) {
	var request *http.Request
	var response *http.Response
	var rawurl string
	var err error

	if src[0] == '/' {
		src = src[1:]
	}

	rawurl = fmt.Sprintf("%s/files/%s/%s", db.APIContentURL, db.RootDirectory, urlEncode(src))
	if len(rev) != 0 {
		rawurl += fmt.Sprintf("?rev=%s", rev)
	}
	if request, err = http.NewRequest("GET", rawurl, nil); err != nil {
		return nil, 0, err
	}
	if offset != 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	if response, err = db.client().Do(request); err != nil {
		return nil, 0, err
	}
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusPartialContent {
		return response.Body, response.ContentLength, err
	}
	response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotFound:
		return nil, 0, os.ErrNotExist
	default:
		return nil, 0, newErrorf(response.StatusCode, "unexpected HTTP status code %d", response.StatusCode)
	}
}

// DownloadToFileResume resumes the download of the file located in the src path on the Dropbox to the dst file on the local disk.
func (db *Dropbox) DownloadToFileResume(src, dst, rev string) error {
	var input io.ReadCloser
	var fi os.FileInfo
	var fd *os.File
	var offset int64
	var err error

	if fd, err = os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		return err
	}
	defer fd.Close()
	if fi, err = fd.Stat(); err != nil {
		return err
	}
	offset = fi.Size()

	if input, _, err = db.Download(src, rev, offset); err != nil {
		return err
	}
	defer input.Close()
	_, err = io.Copy(fd, input)
	return err
}

// DownloadToFile downloads the file located in the src path on the Dropbox to the dst file on the local disk.
// If the destination file exists it will be truncated.
func (db *Dropbox) DownloadToFile(src, dst, rev string) error {
	var input io.ReadCloser
	var fd *os.File
	var err error

	if fd, err = os.Create(dst); err != nil {
		return err
	}
	defer fd.Close()

	if input, _, err = db.Download(src, rev, 0); err != nil {
		os.Remove(dst)
		return err
	}
	defer input.Close()
	if _, err = io.Copy(fd, input); err != nil {
		os.Remove(dst)
	}
	return err
}

func (db *Dropbox) doRequest(method, path string, params *url.Values, receiver interface{}) error {
	var body []byte
	var rawurl string
	var response *http.Response
	var request *http.Request
	var err error

	if params == nil {
		params = &url.Values{"locale": {db.Locale}}
	} else {
		params.Set("locale", db.Locale)
	}
	rawurl = fmt.Sprintf("%s/%s?%s", db.APIURL, urlEncode(path), params.Encode())
	if request, err = http.NewRequest(method, rawurl, nil); err != nil {
		return err
	}
	if response, err = db.client().Do(request); err != nil {
		return err
	}
	defer response.Body.Close()
	if body, err = getResponse(response); err != nil {
		return err
	}
	err = json.Unmarshal(body, receiver)
	return err
}

// GetAccountInfo gets account information for the user currently authenticated.
func (db *Dropbox) GetAccountInfo() (*Account, error) {
	var rv Account

	err := db.doRequest("GET", "account/info", nil, &rv)
	return &rv, err
}

// Shares shares a file.
func (db *Dropbox) Shares(path string, shortURL bool) (*Link, error) {
	var rv Link
	var params *url.Values

	params = &url.Values{"short_url": {strconv.FormatBool(shortURL)}}
	act := strings.Join([]string{"shares", db.RootDirectory, path}, "/")
	err := db.doRequest("POST", act, params, &rv)
	return &rv, err
}

// Media shares a file for streaming (direct access).
func (db *Dropbox) Media(path string) (*Link, error) {
	var rv Link

	act := strings.Join([]string{"media", db.RootDirectory, path}, "/")
	err := db.doRequest("POST", act, nil, &rv)
	return &rv, err
}

// Search searches the entries matching all the words contained in query in the given path.
// The maximum number of entries and whether to consider deleted file may be given.
func (db *Dropbox) Search(path, query string, fileLimit int, includeDeleted bool) ([]Entry, error) {
	var rv []Entry
	var params *url.Values

	if fileLimit <= 0 || fileLimit > SearchLimitMax {
		fileLimit = SearchLimitDefault
	}
	params = &url.Values{
		"query":           {query},
		"file_limit":      {strconv.FormatInt(int64(fileLimit), 10)},
		"include_deleted": {strconv.FormatBool(includeDeleted)},
	}
	act := strings.Join([]string{"search", db.RootDirectory, path}, "/")
	err := db.doRequest("GET", act, params, &rv)
	return rv, err
}

// Delta gets modifications since the cursor.
func (db *Dropbox) Delta(cursor, pathPrefix string) (*DeltaPage, error) {
	var rv DeltaPage
	var params *url.Values
	type deltaPageParser struct {
		Reset   bool                `json:"reset"`    // if true the local state must be cleared.
		HasMore bool                `json:"has_more"` // if true an other call to delta should be made.
		Cursor                      // Tag of the current state.
		Entries [][]json.RawMessage `json:"entries"` // List of changed entries.
	}
	var dpp deltaPageParser

	params = &url.Values{}
	if len(cursor) != 0 {
		params.Set("cursor", cursor)
	}
	if len(pathPrefix) != 0 {
		params.Set("path_prefix", pathPrefix)
	}
	err := db.doRequest("POST", "delta", params, &dpp)
	rv = DeltaPage{Reset: dpp.Reset, HasMore: dpp.HasMore, Cursor: dpp.Cursor}
	rv.Entries = make([]DeltaEntry, 0, len(dpp.Entries))
	for _, jentry := range dpp.Entries {
		var path string
		var entry Entry

		if len(jentry) != 2 {
			return nil, fmt.Errorf("malformed reply")
		}

		if err = json.Unmarshal(jentry[0], &path); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(jentry[1], &entry); err != nil {
			return nil, err
		}
		if entry.Path == "" {
			rv.Entries = append(rv.Entries, DeltaEntry{Path: path, Entry: nil})
		} else {
			rv.Entries = append(rv.Entries, DeltaEntry{Path: path, Entry: &entry})
		}
	}
	return &rv, err
}

// LongPollDelta waits for a notification to happen.
func (db *Dropbox) LongPollDelta(cursor string, timeout int) (*DeltaPoll, error) {
	var rv DeltaPoll
	var params *url.Values
	var body []byte
	var rawurl string
	var response *http.Response
	var err error
	var client http.Client

	params = &url.Values{}
	if timeout != 0 {
		if timeout < PollMinTimeout || timeout > PollMaxTimeout {
			return nil, fmt.Errorf("timeout out of range [%d; %d]", PollMinTimeout, PollMaxTimeout)
		}
		params.Set("timeout", strconv.FormatInt(int64(timeout), 10))
	}
	params.Set("cursor", cursor)
	rawurl = fmt.Sprintf("%s/longpoll_delta?%s", db.APINotifyURL, params.Encode())
	if response, err = client.Get(rawurl); err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if body, err = getResponse(response); err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &rv)
	return &rv, err
}

// Metadata gets the metadata for a file or a directory.
// If list is true and src is a directory, immediate child will be sent in the Contents field.
// If include_deleted is true, entries deleted will be sent.
// hash is the hash of the contents of a directory, it is used to avoid sending data when directory did not change.
// rev is the specific revision to get the metadata from.
// limit is the maximum number of entries requested.
func (db *Dropbox) Metadata(src string, list bool, includeDeleted bool, hash, rev string, limit int) (*Entry, error) {
	var rv Entry
	var params *url.Values

	if limit <= 0 {
		limit = MetadataLimitDefault
	} else if limit > MetadataLimitMax {
		limit = MetadataLimitMax
	}
	params = &url.Values{
		"list":            {strconv.FormatBool(list)},
		"include_deleted": {strconv.FormatBool(includeDeleted)},
		"file_limit":      {strconv.FormatInt(int64(limit), 10)},
	}
	if len(rev) != 0 {
		params.Set("rev", rev)
	}
	if len(hash) != 0 {
		params.Set("hash", hash)
	}

	src = strings.Trim(src, "/")
	act := strings.Join([]string{"metadata", db.RootDirectory, src}, "/")
	err := db.doRequest("GET", act, params, &rv)
	return &rv, err
}

// CopyRef gets a reference to a file.
// This reference can be used to copy this file to another user's Dropbox by passing it to the Copy method.
func (db *Dropbox) CopyRef(src string) (*CopyRef, error) {
	var rv CopyRef
	act := strings.Join([]string{"copy_ref", db.RootDirectory, src}, "/")
	err := db.doRequest("GET", act, nil, &rv)
	return &rv, err
}

// Revisions gets the list of revisions for a file.
func (db *Dropbox) Revisions(src string, revLimit int) ([]Entry, error) {
	var rv []Entry
	if revLimit <= 0 {
		revLimit = RevisionsLimitDefault
	} else if revLimit > RevisionsLimitMax {
		revLimit = RevisionsLimitMax
	}
	act := strings.Join([]string{"revisions", db.RootDirectory, src}, "/")
	err := db.doRequest("GET", act,
		&url.Values{"rev_limit": {strconv.FormatInt(int64(revLimit), 10)}}, &rv)
	return rv, err
}

// Restore restores a deleted file at the corresponding revision.
func (db *Dropbox) Restore(src string, rev string) (*Entry, error) {
	var rv Entry
	act := strings.Join([]string{"restore", db.RootDirectory, src}, "/")
	err := db.doRequest("POST", act, &url.Values{"rev": {rev}}, &rv)
	return &rv, err
}

// Copy copies a file.
// If isRef is true src must be a reference from CopyRef instead of a path.
func (db *Dropbox) Copy(src, dst string, isRef bool) (*Entry, error) {
	var rv Entry
	params := &url.Values{"root": {db.RootDirectory}, "to_path": {dst}}
	if isRef {
		params.Set("from_copy_ref", src)
	} else {
		params.Set("from_path", src)
	}
	err := db.doRequest("POST", "fileops/copy", params, &rv)
	return &rv, err
}

// CreateFolder creates a new directory.
func (db *Dropbox) CreateFolder(path string) (*Entry, error) {
	var rv Entry
	err := db.doRequest("POST", "fileops/create_folder",
		&url.Values{"root": {db.RootDirectory}, "path": {path}}, &rv)
	return &rv, err
}

// Delete removes a file or directory (it is a recursive delete).
func (db *Dropbox) Delete(path string) (*Entry, error) {
	var rv Entry
	err := db.doRequest("POST", "fileops/delete",
		&url.Values{"root": {db.RootDirectory}, "path": {path}}, &rv)
	return &rv, err
}

// Move moves a file or directory.
func (db *Dropbox) Move(src, dst string) (*Entry, error) {
	var rv Entry
	err := db.doRequest("POST", "fileops/move",
		&url.Values{"root": {db.RootDirectory},
			"from_path": {src},
			"to_path":   {dst}}, &rv)
	return &rv, err
}

// LatestCursor returns the latest cursor without fetching any data.
func (db *Dropbox) LatestCursor(prefix string, mediaInfo bool) (*Cursor, error) {
	var (
		params = &url.Values{}
		cur    Cursor
	)

	if prefix != "" {
		params.Set("path_prefix", prefix)
	}

	if mediaInfo {
		params.Set("include_media_info", "true")
	}

	err := db.doRequest("POST", "delta/latest_cursor", params, &cur)
	return &cur, err
}

// SharedFolders returns the list of allowed shared folders.
func (db *Dropbox) SharedFolders(sharedFolderID string) ([]SharedFolder, error) {
	var sharedFolders []SharedFolder
	var err error

	if sharedFolderID != "" {
		sharedFolders = make([]SharedFolder, 1)
		err = db.doRequest("GET", "/shared_folders/"+sharedFolderID, nil, &sharedFolders[0])
	} else {
		err = db.doRequest("GET", "/shared_folders/", nil, &sharedFolders)
	}
	return sharedFolders, err
}
