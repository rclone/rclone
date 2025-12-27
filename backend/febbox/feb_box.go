// feb_box.go - FINAL WORKING VERSION WITH STREAMING SUPPORT
package feb_box

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "strings"
    "time"

    "github.com/rclone/rclone/fs"
    "github.com/rclone/rclone/fs/config/configmap"
    "github.com/rclone/rclone/fs/config/configstruct"
    "github.com/rclone/rclone/fs/hash"
    "github.com/rclone/rclone/lib/rest"
)

func init() {
    fs.Register(&fs.RegInfo{
        Name:        "febbox",
        Description: "Febbox Cloud Storage",
        NewFs:       NewFs,
        Options: []fs.Option{{
            Name:     "cookies",
            Help:     "ALL cookies from browser (PHPSESSID, ui, cf_clearance, etc.)",
            Required: true,
        }, {
            Name:     "share_key",
            Help:     "Share key from Febbox share URL",
            Required: true,
        }},
    })
}

type Options struct {
    Cookies  string `config:"cookies"`
    ShareKey string `config:"share_key"`
}

type FebboxFileItem struct {
    FID           int64  `json:"fid"`
    FileName      string `json:"file_name"`
    FileSize      string `json:"file_size"`
    FileSizeBytes int64  `json:"file_size_bytes"`
    IsDir         int    `json:"is_dir"`
    Ext           string `json:"ext"`
    AddTime       string `json:"add_time"`
    UpdateTime    int64  `json:"update_time"`
    OssFID        int64  `json:"oss_fid"`
    Hash          string `json:"hash"`
    HashType      string `json:"hash_type"`
}

type FebboxResponse struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
    Data struct {
        FileList []FebboxFileItem `json:"file_list"`
    } `json:"data"`
}

type FileDownloadResponse struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
    Data []struct {
        Error       int    `json:"error"`
        DownloadURL string `json:"download_url"`
        Hash        string `json:"hash"`
        HashType    string `json:"hash_type"`
        FID         int64  `json:"fid"`
        FileName    string `json:"file_name"`
        FileSize    int64  `json:"file_size"`
        Ext         string `json:"ext"`
        QualityList []struct {
            Quality     string `json:"quality"`
            DownloadURL string `json:"download_url"`
            OssFID      int64  `json:"oss_fid"`
            FileSize    int64  `json:"file_size"`
            Bitrate     string `json:"bitrate"`
            Runtime     int    `json:"runtime"`
            Is265       int    `json:"is_265"`
        } `json:"quality_list"`
    } `json:"data"`
}

type Fs struct {
    name     string
    root     string
    opt      Options
    features *fs.Features
    api      *rest.Client
    shareKey string
    cookieJar *cookiejar.Jar
}

type Object struct {
    fs       *Fs
    remote   string
    fid      int64
    ossFid   int64
    hash     string
    hashType string
    name     string
    size     int64
    modTime  time.Time
    isDir    bool
    mimeType string
}

// StreamingResponse wraps the HTTP response for proper streaming
type StreamingResponse struct {
    io.ReadCloser
    Header     http.Header
    StatusCode int
    ContentLength int64
}

func (sr *StreamingResponse) Read(p []byte) (n int, err error) {
    return sr.ReadCloser.Read(p)
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    opt := new(Options)
    if err := configstruct.Set(m, opt); err != nil {
        return nil, err
    }

    if opt.Cookies == "" {
        return nil, fmt.Errorf("cookies are required - get them from browser dev tools")
    }
    if opt.ShareKey == "" {
        return nil, fmt.Errorf("share_key is required - get it from the share URL")
    }

    root = strings.Trim(root, "/")
    f := &Fs{
        name:     name,
        root:     root,
        opt:      *opt,
        shareKey: opt.ShareKey,
    }

    jar, err := cookiejar.New(nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create cookie jar: %w", err)
    }
    f.cookieJar = jar
    
    cookies := ParseCookieString(opt.Cookies)
    febboxURL, _ := url.Parse("https://www.febbox.com")
    jar.SetCookies(febboxURL, cookies)

    httpClient := &http.Client{
        Jar: jar,
        Timeout: 30 * time.Second,
    }
    
    f.api = rest.NewClient(httpClient).SetRoot("https://www.febbox.com")
    
    f.api.SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
    f.api.SetHeader("X-Requested-With", "XMLHttpRequest")
    f.api.SetHeader("Cookie", opt.Cookies)
    f.api.SetHeader("Referer", "https://www.febbox.com/console")
    f.api.SetHeader("Origin", "https://www.febbox.com")

    var apiResp FebboxResponse
    opts := rest.Opts{
        Method: "GET",
        Path:   fmt.Sprintf("/file/file_share_list?share_key=%s&parent_id=0&is_html=0", f.shareKey),
    }

    _, err = f.api.CallJSON(ctx, &opts, nil, &apiResp)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to Febbox: %w", err)
    }

    if apiResp.Code != 1 {
        return nil, fmt.Errorf("Febbox API error (code %d): %s", apiResp.Code, apiResp.Msg)
    }

    f.features = (&fs.Features{
        CaseInsensitive:         true,
        ReadMimeType:           true,
        CanHaveEmptyDirectories: true,
    }).Fill(ctx, f)

    return f, nil
}

func (f *Fs) Name() string           { return f.name }
func (f *Fs) Root() string           { return f.root }
func (f *Fs) String() string         { return fmt.Sprintf("Febbox share '%s'", f.shareKey) }
func (f *Fs) Precision() time.Duration { return time.Second }
func (f *Fs) Hashes() hash.Set       { return hash.Set(hash.None) }
func (f *Fs) Features() *fs.Features { return f.features }

func (f *Fs) getFileList(ctx context.Context, parentID string) ([]FebboxFileItem, error) {
    var apiResp FebboxResponse
    opts := rest.Opts{
        Method: "GET",
        Path:   fmt.Sprintf("/file/file_share_list?share_key=%s&parent_id=%s&is_html=0", f.shareKey, parentID),
    }

    _, err := f.api.CallJSON(ctx, &opts, nil, &apiResp)
    if err != nil {
        return nil, err
    }

    if apiResp.Code != 1 {
        return nil, fmt.Errorf("API error (code %d): %s", apiResp.Code, apiResp.Msg)
    }

    return apiResp.Data.FileList, nil
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    remote = strings.TrimPrefix(remote, "/")
    if remote == "" {
        return nil, fs.ErrorIsDir
    }

    fileList, err := f.getFileList(ctx, "0")
    if err != nil {
        return nil, err
    }

    for _, item := range fileList {
        if item.FileName == remote {
            modTime, _ := time.Parse("Jan 2,2006 15:04", item.AddTime)
            if modTime.IsZero() {
                modTime = time.Now()
            }

            return &Object{
                fs:       f,
                remote:   remote,
                fid:      item.FID,
                ossFid:   item.OssFID,
                hash:     item.Hash,
                hashType: item.HashType,
                name:     item.FileName,
                size:     item.FileSizeBytes,
                modTime:  modTime,
                isDir:    item.IsDir == 1,
                mimeType: getMimeType(item.Ext),
            }, nil
        }
    }

    return nil, fs.ErrorObjectNotFound
}

func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
    if dir != "" && dir != "." {
        return nil, fs.ErrorNotImplemented
    }

    fileList, err := f.getFileList(ctx, "0")
    if err != nil {
        return nil, err
    }

    var entries fs.DirEntries
    for _, item := range fileList {
        modTime, _ := time.Parse("Jan 2,2006 15:04", item.AddTime)
        if modTime.IsZero() {
            modTime = time.Now()
        }

        if item.IsDir == 1 {
            entries = append(entries, fs.NewDir(item.FileName, modTime))
        } else {
            entries = append(entries, &Object{
                fs:       f,
                remote:   item.FileName,
                fid:      item.FID,
                ossFid:   item.OssFID,
                hash:     item.Hash,
                hashType: item.HashType,
                name:     item.FileName,
                size:     item.FileSizeBytes,
                modTime:  modTime,
                isDir:    false,
                mimeType: getMimeType(item.Ext),
            })
        }
    }

    return entries, nil
}

// getDownloadURL returns download URL for a file
func (f *Fs) getDownloadURL(ctx context.Context, fid int64) (string, error) {
    fidsJSON := fmt.Sprintf(`["%d"]`, fid)
    encodedFids := url.QueryEscape(fidsJSON)
    
    var downloadResp FileDownloadResponse
    opts := rest.Opts{
        Method: "GET",
        Path:   fmt.Sprintf("/console/file_download?fids=%s&share=", encodedFids),
    }

    _, err := f.api.CallJSON(ctx, &opts, nil, &downloadResp)
    if err != nil {
        return "", fmt.Errorf("failed to get download URL: %w", err)
    }

    if downloadResp.Code != 1 || len(downloadResp.Data) == 0 {
        return "", fmt.Errorf("API error (code %d): %s", downloadResp.Code, downloadResp.Msg)
    }

    if downloadResp.Data[0].Error != 0 {
        return "", fmt.Errorf("download error: %d", downloadResp.Data[0].Error)
    }

    downloadURL := downloadResp.Data[0].DownloadURL
    if downloadURL == "" && len(downloadResp.Data[0].QualityList) > 0 {
        downloadURL = downloadResp.Data[0].QualityList[0].DownloadURL
    }

    if downloadURL == "" {
        return "", fmt.Errorf("no download URL found")
    }

    return downloadURL, nil
}

func getMimeType(ext string) string {
    ext = strings.ToLower(strings.TrimPrefix(ext, "."))
    
    switch ext {
    case "mp4", "m4v":
        return "video/mp4"
    case "mkv":
        return "video/x-matroska"
    case "avi":
        return "video/x-msvideo"
    case "mov":
        return "video/quicktime"
    case "wmv":
        return "video/x-ms-wmv"
    case "flv":
        return "video/x-flv"
    case "webm":
        return "video/webm"
    case "m3u8":
        return "application/x-mpegURL"
    case "mp3":
        return "audio/mpeg"
    case "wav":
        return "audio/wav"
    case "flac":
        return "audio/flac"
    case "jpg", "jpeg":
        return "image/jpeg"
    case "png":
        return "image/png"
    case "gif":
        return "image/gif"
    default:
        return "application/octet-stream"
    }
}

func (o *Object) Fs() fs.Info                         { return o.fs }
func (o *Object) Remote() string                      { return o.remote }
func (o *Object) String() string                      { return o.remote }
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }
func (o *Object) Size() int64                         { return o.size }
func (o *Object) Storable() bool                      { return !o.isDir }
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
    return "", hash.ErrUnsupported
}
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
    return fs.ErrorNotImplemented
}
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
    return fs.ErrorNotImplemented
}
func (o *Object) Remove(ctx context.Context) error {
    return fs.ErrorNotImplemented
}

// Open with PROPER STREAMING SUPPORT
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
    // Get fresh download URL for each request (tokens expire)
    downloadURL, err := o.fs.getDownloadURL(ctx, o.fid)
    if err != nil {
        return nil, fmt.Errorf("failed to get download URL: %w", err)
    }

    client := &http.Client{
        Jar: o.fs.cookieJar,
        Timeout: 0,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            // Update headers for redirects
            req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
            req.Header.Set("Referer", "https://www.febbox.com/console")
            req.Header.Set("Origin", "https://www.febbox.com")
            req.Header.Set("Cookie", o.fs.opt.Cookies)
            return nil
        },
    }

    req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
    if err != nil {
        return nil, err
    }

    // Headers that work for streaming
    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
    req.Header.Set("Referer", "https://www.febbox.com/console")
    req.Header.Set("Origin", "https://www.febbox.com")
    req.Header.Set("Accept", "*/*")
    req.Header.Set("Accept-Encoding", "identity") // CRITICAL for VLC
    req.Header.Set("Accept-Language", "en-US,en;q=0.9")
    req.Header.Set("Connection", "keep-alive")
    req.Header.Set("Cache-Control", "no-cache")
    req.Header.Set("Cookie", o.fs.opt.Cookies)
    
    // IMPORTANT: VLC needs Range support
    req.Header.Set("Accept-Ranges", "bytes")

    // Handle range/seek requests
    var rangeHeader string
    var start, end int64 = 0, o.size - 1
    
    for _, option := range options {
        switch opt := option.(type) {
        case *fs.SeekOption:
            start = opt.Offset
            if start < 0 {
                start = 0
            }
            if start > o.size {
                start = o.size
            }
            end = o.size - 1
            rangeHeader = fmt.Sprintf("bytes=%d-", start)
        case *fs.RangeOption:
            start = opt.Start
            end = opt.End
            if start < 0 {
                start = 0
            }
            if end < 0 || end >= o.size {
                end = o.size - 1
            }
            rangeHeader = fmt.Sprintf("bytes=%d-%d", start, end)
        }
    }

    if rangeHeader != "" {
        req.Header.Set("Range", rangeHeader)
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }

    // Check response
    status := resp.StatusCode
    if status != http.StatusOK && status != http.StatusPartialContent {
        resp.Body.Close()
        
        // If range request failed, try without range header
        if status == http.StatusRequestedRangeNotSatisfiable || status == http.StatusForbidden {
            retryReq, _ := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
            retryReq.Header = req.Header.Clone()
            retryReq.Header.Del("Range")
            
            retryResp, err := client.Do(retryReq)
            if err != nil {
                resp.Body.Close()
                return nil, fmt.Errorf("download failed: %s", resp.Status)
            }
            
            if retryResp.StatusCode == http.StatusOK {
                // Create streaming response with proper headers
                return &StreamingResponse{
                    ReadCloser:     retryResp.Body,
                    Header:         retryResp.Header,
                    StatusCode:     retryResp.StatusCode,
                    ContentLength:  o.size,
                }, nil
            }
            retryResp.Body.Close()
        }
        
        resp.Body.Close()
        return nil, fmt.Errorf("download failed: %s", resp.Status)
    }

    // Ensure proper headers for streaming
    if resp.Header.Get("Content-Type") == "" {
        resp.Header.Set("Content-Type", o.mimeType)
    }
    
    // Set content length if missing
    contentLength := o.size
    if status == http.StatusPartialContent {
        contentLength = end - start + 1
    }

    return &StreamingResponse{
        ReadCloser:     resp.Body,
        Header:         resp.Header,
        StatusCode:     resp.StatusCode,
        ContentLength:  contentLength,
    }, nil
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
    return nil, fs.ErrorNotImplemented
}
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
    return fs.ErrorNotImplemented
}
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    return fs.ErrorNotImplemented
}