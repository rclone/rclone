package putio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
)

// FilesService is a general service to gather information about user files,
// such as listing, searching, creating new ones, or just fetching a single
// file.
type FilesService struct {
	client *Client
}

// Get fetches file metadata for given file ID.
func (f *FilesService) Get(ctx context.Context, id int64) (File, error) {
	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/"+itoa(id), nil)
	if err != nil {
		return File{}, err
	}

	var r struct {
		File File `json:"file"`
	}
	_, err = f.client.Do(req, &r)
	if err != nil {
		return File{}, err
	}
	return r.File, nil
}

// List fetches children for given directory ID.
func (f *FilesService) List(ctx context.Context, id int64) (children []File, parent File, err error) {
	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/list?parent_id="+itoa(id)+"&per_page=1000", nil)
	if err != nil {
		return
	}
	var r struct {
		Files  []File `json:"files"`
		Parent File   `json:"parent"`
		Cursor string `json:"cursor"`
	}
	_, err = f.client.Do(req, &r)
	if err != nil {
		return
	}
	children = append(children, r.Files...)
	parent = r.Parent
	for r.Cursor != "" {
		body := strings.NewReader(`{"cursor": "` + r.Cursor + `"}`)
		req, err = f.client.NewRequest(ctx, "POST", "/v2/files/list/continue", body)
		if err != nil {
			return
		}
		req.Header.Set("content-type", "application/json")
		r.Files = nil
		r.Cursor = ""
		_, err = f.client.Do(req, &r)
		if err != nil {
			return
		}
		children = append(children, r.Files...)
	}
	return
}

// URL returns a URL of the file for downloading or streaming.
func (f *FilesService) URL(ctx context.Context, id int64, useTunnel bool) (string, error) {
	notunnel := "notunnel=1"
	if useTunnel {
		notunnel = "notunnel=0"
	}

	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/"+itoa(id)+"/url?"+notunnel, nil)
	if err != nil {
		return "", err
	}

	var r struct {
		URL string `json:"url"`
	}

	_, err = f.client.Do(req, &r)
	if err != nil {
		return "", err
	}

	return r.URL, nil
}

// CreateFolder creates a new folder under parent.
func (f *FilesService) CreateFolder(ctx context.Context, name string, parent int64) (File, error) {
	if name == "" {
		return File{}, fmt.Errorf("empty folder name")
	}

	params := url.Values{}
	params.Set("name", name)
	params.Set("parent_id", itoa(parent))

	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/create-folder", strings.NewReader(params.Encode()))
	if err != nil {
		return File{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var r struct {
		File File `json:"file"`
	}
	_, err = f.client.Do(req, &r)
	if err != nil {
		return File{}, err
	}

	return r.File, nil
}

// Delete deletes given files.
func (f *FilesService) Delete(ctx context.Context, files ...int64) error {
	if len(files) == 0 {
		return fmt.Errorf("no file id is given")
	}

	var ids []string
	for _, id := range files {
		ids = append(ids, itoa(id))
	}

	params := url.Values{}
	params.Set("file_ids", strings.Join(ids, ","))

	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/delete", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = f.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}
	return nil
}

// Rename change the name of the file to newname.
func (f *FilesService) Rename(ctx context.Context, id int64, newname string) error {
	if newname == "" {
		return fmt.Errorf("new filename cannot be empty")
	}

	params := url.Values{}
	params.Set("file_id", itoa(id))
	params.Set("name", newname)

	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/rename", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = f.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}

	return nil
}

// Move moves files to the given destination.
func (f *FilesService) Move(ctx context.Context, parent int64, files ...int64) error {
	if len(files) == 0 {
		return fmt.Errorf("no files given")
	}

	var ids []string
	for _, file := range files {
		ids = append(ids, itoa(file))
	}

	params := url.Values{}
	params.Set("file_ids", strings.Join(ids, ","))
	params.Set("parent_id", itoa(parent))

	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/move", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = f.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}
	return nil
}

// Upload reads from given io.Reader and uploads the file contents to Put.io
// servers under directory given by parent. If parent is negative, user's
// preferred folder is used.
//
// If the uploaded file is a torrent file, Put.io will interpret it as a
// transfer and Transfer field will be present to represent the status of the
// tranfer. Likewise, if the uploaded file is a regular file, Transfer field
// would be nil and the uploaded file will be represented by the File field.
//
// This method reads the file contents into the memory, so it should be used for
// <150MB files.
func (f *FilesService) Upload(ctx context.Context, r io.Reader, filename string, parent int64) (Upload, error) {
	if filename == "" {
		return Upload{}, fmt.Errorf("filename cannot be empty")
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// negative parent means use user's preferred download folder.
	if parent >= 0 {
		err := mw.WriteField("parent_id", itoa(parent))
		if err != nil {
			return Upload{}, err
		}
	}

	formfile, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return Upload{}, err
	}

	_, err = io.Copy(formfile, r)
	if err != nil {
		return Upload{}, err
	}

	err = mw.Close()
	if err != nil {
		return Upload{}, err
	}

	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/upload", &buf)
	if err != nil {
		return Upload{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	var response struct {
		Upload
	}
	_, err = f.client.Do(req, &response)
	if err != nil {
		return Upload{}, err
	}
	return response.Upload, nil
}

// Search makes a search request with the given query. Servers return 50
// results at a time. The URL for the next 50 results are in Next field.  If
// page is -1, all results are returned.
func (f *FilesService) Search(ctx context.Context, query string, page int64) (Search, error) {
	if page == 0 || page < -1 {
		return Search{}, fmt.Errorf("invalid page number")
	}
	if query == "" {
		return Search{}, fmt.Errorf("no query given")
	}

	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/search/"+query+"/page/"+itoa(page), nil)
	if err != nil {
		return Search{}, err
	}

	var r Search
	_, err = f.client.Do(req, &r)
	if err != nil {
		return Search{}, err
	}

	return r, nil
}

// Shared returns list of shared files and share information.
func (f *FilesService) shared(ctx context.Context) ([]share, error) {
	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/shared", nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		Shared []share
	}
	_, err = f.client.Do(req, &r)
	if err != nil {
		return nil, err
	}

	return r.Shared, nil
}

// SharedWith returns list of users the given file is shared with.
func (f *FilesService) sharedWith(ctx context.Context, id int64) ([]share, error) {
	// FIXME: shared-with returns different json structure than /shared/
	// endpoint. so it's not an exported method until a common structure is
	// decided
	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/"+itoa(id)+"/shared-with", nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		Shared []share `json:"shared-with"`
	}
	_, err = f.client.Do(req, &r)
	if err != nil {
		return nil, err
	}

	return r.Shared, nil
}

// Subtitles lists available subtitles for the given file for user's preferred
// subtitle language.
func (f *FilesService) Subtitles(ctx context.Context, id int64) ([]Subtitle, error) {
	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/"+itoa(id)+"/subtitles", nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		Subtitles []Subtitle
		Default   string
	}
	_, err = f.client.Do(req, &r)
	if err != nil {
		return nil, err
	}

	return r.Subtitles, nil
}

// DownloadSubtitle sends the contents of the subtitle file. If the key is empty string,
// `default` key is used. This key is used to search for a subtitle in the
// following order and returns the first match:
// - A subtitle file that has identical parent folder and name with the video.
// - Subtitle file extracted from video if the format is MKV.
// - First match from OpenSubtitles.org.
func (f *FilesService) DownloadSubtitle(ctx context.Context, id int64, key string, format string) (io.ReadCloser, error) {
	if key == "" {
		key = "default"
	}
	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/"+itoa(id)+"/subtitles/"+key, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req, nil)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// HLSPlaylist serves a HLS playlist for a video file. Use “all” as
// subtitleKey to get available subtitles for user’s preferred languages.
func (f *FilesService) HLSPlaylist(ctx context.Context, id int64, subtitleKey string) (io.ReadCloser, error) {
	if subtitleKey == "" {
		return nil, fmt.Errorf("empty subtitle key is given")
	}

	req, err := f.client.NewRequest(ctx, "GET", "/v2/files/"+itoa(id)+"/hls/media.m3u8?subtitle_key"+subtitleKey, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req, nil)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// SetVideoPosition sets default video position for a video file.
func (f *FilesService) SetVideoPosition(ctx context.Context, id int64, t int) error {
	if t < 0 {
		return fmt.Errorf("time cannot be negative")
	}

	params := url.Values{}
	params.Set("time", strconv.Itoa(t))

	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/"+itoa(id)+"/start-from", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = f.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}

	return nil
}

// DeleteVideoPosition deletes video position for a video file.
func (f *FilesService) DeleteVideoPosition(ctx context.Context, id int64) error {
	req, err := f.client.NewRequest(ctx, "POST", "/v2/files/"+itoa(id)+"/start-from/delete", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err = f.client.Do(req, &struct{}{})
	if err != nil {
		return err
	}

	return nil
}

func itoa(i int64) string {
	return strconv.FormatInt(i, 10)
}
