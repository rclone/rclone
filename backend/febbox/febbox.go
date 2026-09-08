// Package febbox provides an interface to FebBox cloud storage.
package febbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/rclone/rclone/backend/febbox/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	defaultEndpoint  = "https://api.febbox.com/oauth"
	rootID           = "0"
	minSleep         = 100 * time.Millisecond
	maxSleep         = 10 * time.Second
	browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

var retryErrorCodes = []int{429, 500, 502, 503, 504}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "febbox",
		Description: "FebBox",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:      "client_id",
			Help:      "OAuth client ID.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "client_secret",
			Help:      "OAuth client secret.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "list_chunk",
			Help:     "Number of entries requested per page.",
			Default:  100,
			Advanced: true,
		}, {
			Name:     "endpoint",
			Help:     "API endpoint.",
			Default:  defaultEndpoint,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend.
type Options struct {
	ClientID     string               `config:"client_id"`
	ClientSecret string               `config:"client_secret"`
	ListChunk    int                  `config:"list_chunk"`
	Endpoint     string               `config:"endpoint"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a FebBox filesystem.
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	srv      *rest.Client
	download *rest.Client
	dirCache *dircache.DirCache
	pacer    *fs.Pacer
}

// Object describes a FebBox file.
type Object struct {
	fs       *Fs
	remote   string
	id       string
	parentID string
	size     int64
	modTime  time.Time
	mimeType string
}

type tokenSource struct {
	mu           sync.Mutex
	ctx          context.Context
	client       *http.Client
	tokenURL     string
	clientID     string
	clientSecret string
	token        *oauth2.Token
}

func (s *tokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token.Valid() {
		return s.token, nil
	}
	form := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"grant_type":    {"client_credentials"},
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", browserUserAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token request failed: %s", resp.Status)
	}
	var result api.Response[api.Token]
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if err = apiError("authenticate", result.Code, result.Msg); err != nil {
		return nil, err
	}
	if result.Data.AccessToken == "" {
		return nil, errors.New("authenticate: response contains no access token")
	}
	tokenType := result.Data.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	s.token = &oauth2.Token{
		AccessToken:  result.Data.AccessToken,
		TokenType:    tokenType,
		RefreshToken: result.Data.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(result.Data.ExpiresIn) * time.Second),
	}
	return s.token, nil
}

func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func apiError(operation string, code int, message string) error {
	if code == 1 {
		return nil
	}
	if code == -403 {
		return fmt.Errorf("%s: %s (code %d): %w", operation, message, code, fs.ErrorPermissionDenied)
	}
	return fmt.Errorf("%s: %s (code %d)", operation, message, code)
}

// NewFs constructs a FebBox filesystem.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.ClientID == "" || opt.ClientSecret == "" {
		return nil, errors.New("client_id and client_secret are required")
	}
	if opt.ListChunk == 0 {
		opt.ListChunk = 100
	}
	if opt.ListChunk < 0 {
		return nil, errors.New("list_chunk must be greater than zero")
	}
	if opt.Endpoint == "" {
		opt.Endpoint = defaultEndpoint
	}
	opt.Endpoint = strings.TrimRight(opt.Endpoint, "/")
	root = strings.Trim(root, "/")
	baseClient := fshttp.NewClient(ctx)
	if transport, ok := baseClient.Transport.(*fshttp.Transport); ok {
		transport.SetRequestFilter(func(req *http.Request) {
			req.Header.Set("User-Agent", browserUserAgent)
		})
	}
	tokenClient := oauth2.NewClient(context.WithValue(ctx, oauth2.HTTPClient, baseClient), &tokenSource{
		ctx:          ctx,
		client:       baseClient,
		tokenURL:     opt.Endpoint + "/token",
		clientID:     opt.ClientID,
		clientSecret: opt.ClientSecret,
	})
	f := &Fs{
		name:     name,
		root:     root,
		opt:      *opt,
		srv:      rest.NewClient(tokenClient).SetRoot(opt.Endpoint),
		download: rest.NewClient(baseClient),
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep))),
	}
	f.srv.SetHeader("Accept", "application/json")
	f.srv.SetHeader("User-Agent", browserUserAgent)
	f.download.SetHeader("User-Agent", browserUserAgent)
	f.dirCache = dircache.New(root, rootID, f)
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)
	err := f.dirCache.FindRoot(ctx, false)
	if err == nil {
		return f, nil
	}
	newRoot, remote := dircache.SplitPath(root)
	tempF := *f
	tempF.root = newRoot
	tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
	if err = tempF.dirCache.FindRoot(ctx, false); err != nil {
		return f, nil
	}
	if _, err = tempF.NewObject(ctx, remote); err != nil {
		if err == fs.ErrorObjectNotFound {
			return f, nil
		}
		return nil, err
	}
	f.dirCache = tempF.dirCache
	f.root = tempF.root
	return f, fs.ErrorIsFile
}

// Name returns the configured remote name.
func (f *Fs) Name() string { return f.name }

// Root returns the configured root path.
func (f *Fs) Root() string { return f.root }

// String describes the filesystem.
func (f *Fs) String() string { return fmt.Sprintf("FebBox root %q", f.root) }

// Precision returns the modification-time precision.
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns the supported hashes.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

// Features returns optional filesystem features.
func (f *Fs) Features() *fs.Features { return f.features }

func (f *Fs) call(ctx context.Context, operation string, params url.Values, result any) error {
	return f.callWithRetry(ctx, operation, params, result, true)
}

func (f *Fs) callNoRetry(ctx context.Context, operation string, params url.Values, result any) error {
	return f.callWithRetry(ctx, operation, params, result, false)
}

func (f *Fs) callWithRetry(ctx context.Context, operation string, params url.Values, result any, retry bool) error {
	params.Set("module", operation)
	opts := rest.Opts{Method: http.MethodPost, MultipartParams: params}
	var resp *http.Response
	var err error
	call := f.pacer.Call
	if !retry {
		call = f.pacer.CallNoRetry
	}
	return call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, result)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) listPage(ctx context.Context, parentID string, pageNumber int) ([]api.Item, error) {
	var result api.Response[api.FileList]
	err := f.call(ctx, "file_list", url.Values{
		"parent_id": {parentID},
		"page":      {strconv.Itoa(pageNumber)},
		"pagelimit": {strconv.Itoa(f.opt.ListChunk)},
		"order":     {"name_asc"},
	}, &result)
	if err != nil {
		return nil, err
	}
	if err = apiError("list", result.Code, result.Msg); err != nil {
		return nil, err
	}
	return result.Data.Files, nil
}

func (f *Fs) listAll(ctx context.Context, parentID string) ([]api.Item, error) {
	var items []api.Item
	for pageNumber := 1; ; pageNumber++ {
		pageItems, err := f.listPage(ctx, parentID, pageNumber)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(pageItems) < f.opt.ListChunk {
			return items, nil
		}
	}
}

// FindLeaf finds a directory below pathID.
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	items, err := f.listAll(ctx, pathID)
	if err != nil {
		return "", false, err
	}
	encodedLeaf := f.opt.Enc.FromStandardName(leaf)
	for _, item := range items {
		if item.IsDir != 0 && item.FileName == encodedLeaf {
			return string(item.FID), true, nil
		}
	}
	return "", false, nil
}

// CreateDir creates a directory below pathID.
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	var result api.Response[api.CreateDir]
	err := f.callNoRetry(ctx, "create_dir", url.Values{
		"parent_id": {pathID},
		"name":      {f.opt.Enc.FromStandardName(leaf)},
	}, &result)
	if err != nil {
		return "", err
	}
	if err = apiError("create directory", result.Code, result.Msg); err != nil {
		return "", err
	}
	if result.Data.FID == "" {
		return "", errors.New("create directory: response contains no ID")
	}
	return string(result.Data.FID), nil
}

func (f *Fs) newObject(remote string, item *api.Item) *Object {
	modTime := time.Unix(item.UpdateTime, 0)
	if item.UpdateTime == 0 {
		modTime = time.Unix(item.AddTime, 0)
	}
	return &Object{
		fs:       f,
		remote:   remote,
		id:       string(item.FID),
		parentID: string(item.ParentID),
		size:     item.FileSize,
		modTime:  modTime,
		mimeType: mime.TypeByExtension("." + item.Extension),
	}
}

// List returns the entries in dir.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	parentID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	items, err := f.listAll(ctx, parentID)
	if err != nil {
		return nil, err
	}
	entries := make(fs.DirEntries, 0, len(items))
	for i := range items {
		item := &items[i]
		name := f.opt.Enc.ToStandardName(item.FileName)
		remote := path.Join(dir, name)
		if item.IsDir != 0 {
			id := string(item.FID)
			f.dirCache.Put(remote, id)
			entries = append(entries, fs.NewDir(remote, time.Unix(item.UpdateTime, 0)).SetID(id))
			continue
		}
		entries = append(entries, f.newObject(remote, item))
	}
	return entries, nil
}

// NewObject finds the file at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	leaf, parentID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	items, err := f.listAll(ctx, parentID)
	if err != nil {
		return nil, err
	}
	encodedLeaf := f.opt.Enc.FromStandardName(leaf)
	for i := range items {
		item := &items[i]
		if item.IsDir == 0 && item.FileName == encodedLeaf {
			return f.newObject(remote, item), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Put is unsupported because FebBox accepts only HTTP source URLs.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fmt.Errorf("FebBox accepts only HTTP source URLs: %w", fs.ErrorNotImplemented)
}

// Mkdir creates dir and its parents.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

func (f *Fs) delete(ctx context.Context, id string) error {
	var result api.Response[any]
	err := f.call(ctx, "file_delete", url.Values{"fids[]": {id}}, &result)
	if err != nil {
		return err
	}
	return apiError("delete", result.Code, result.Msg)
}

// DirCacheFlush resets the directory cache.
func (f *Fs) DirCacheFlush() { f.dirCache.ResetRoot() }

// Rmdir removes an empty directory.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	if path.Join(f.root, dir) == "" {
		return errors.New("can't remove account root directory")
	}
	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	items, err := f.listPage(ctx, id, 1)
	if err != nil {
		return err
	}
	if len(items) != 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	if err = f.delete(ctx, id); err != nil {
		return err
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Fs returns the owning filesystem.
func (o *Object) Fs() fs.Info { return o.fs }

// String returns the remote path.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path.
func (o *Object) Remote() string { return o.remote }

// ModTime returns the modification time.
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// Size returns the file size.
func (o *Object) Size() int64 { return o.size }

// Storable reports whether the object can be stored.
func (o *Object) Storable() bool { return true }

// Hash returns no hash because FebBox hash algorithms are unspecified.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime reports that modification times cannot be set.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// MimeType returns the detected MIME type.
func (o *Object) MimeType(ctx context.Context) string { return o.mimeType }

// ID returns the FebBox file identifier.
func (o *Object) ID() string { return o.id }

func (f *Fs) downloadURL(ctx context.Context, id string) (string, error) {
	var result api.Response[[]api.Download]
	err := f.call(ctx, "file_get_download_url", url.Values{"fids[]": {id}}, &result)
	if err != nil {
		return "", err
	}
	if err = apiError("get download URL", result.Code, result.Msg); err != nil {
		return "", err
	}
	if len(result.Data) == 0 || result.Data[0].DownloadURL == "" {
		return "", errors.New("get download URL: response contains no URL")
	}
	return result.Data[0].DownloadURL, nil
}

// Open opens the object for reading.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	downloadURL, err := o.fs.downloadURL(ctx, o.id)
	if err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{Method: http.MethodGet, RootURL: downloadURL, Options: options}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.download.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update is unsupported because FebBox accepts only HTTP source URLs.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fmt.Errorf("FebBox accepts only HTTP source URLs: %w", fs.ErrorNotImplemented)
}

// Remove removes the object.
func (o *Object) Remove(ctx context.Context) error { return o.fs.delete(ctx, o.id) }

func (f *Fs) rename(ctx context.Context, id, leaf string) error {
	var result api.Response[any]
	err := f.call(ctx, "file_rename", url.Values{
		"fid":  {id},
		"name": {f.opt.Enc.FromStandardName(leaf)},
	}, &result)
	if err != nil {
		return err
	}
	return apiError("rename", result.Code, result.Msg)
}

func (f *Fs) moveID(ctx context.Context, id, parentID string) error {
	var result api.Response[any]
	err := f.call(ctx, "file_move", url.Values{
		"fids[]": {id},
		"to":     {parentID},
	}, &result)
	if err != nil {
		return err
	}
	return apiError("move", result.Code, result.Msg)
}

// Move moves src to remote using FebBox server-side operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObject, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	leaf, parentID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	if srcObject.parentID != parentID {
		if err = f.moveID(ctx, srcObject.id, parentID); err != nil {
			return nil, err
		}
	}
	if path.Base(srcObject.remote) != leaf {
		if err = f.rename(ctx, srcObject.id, leaf); err != nil {
			return nil, err
		}
	}
	dst := *srcObject
	dst.remote = remote
	dst.parentID = parentID
	return &dst, nil
}

// Copy copies src to remote using FebBox server-side operations.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObject, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	leaf, parentID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	itemsBefore, err := f.listAll(ctx, parentID)
	if err != nil {
		return nil, err
	}
	previousIDs := make(map[string]struct{}, len(itemsBefore))
	for i := range itemsBefore {
		previousIDs[string(itemsBefore[i].FID)] = struct{}{}
	}
	var result api.Response[any]
	err = f.callNoRetry(ctx, "file_copy", url.Values{
		"fids[]": {srcObject.id},
		"to":     {parentID},
	}, &result)
	if err != nil {
		return nil, err
	}
	if err = apiError("copy", result.Code, result.Msg); err != nil {
		return nil, err
	}
	items, err := f.listAll(ctx, parentID)
	if err != nil {
		return nil, err
	}
	sourceLeaf := f.opt.Enc.FromStandardName(path.Base(srcObject.remote))
	for i := range items {
		item := &items[i]
		if item.IsDir != 0 || item.FileName != sourceLeaf {
			continue
		}
		if _, existed := previousIDs[string(item.FID)]; existed {
			continue
		}
		if path.Base(srcObject.remote) != leaf {
			if err = f.rename(ctx, string(item.FID), leaf); err != nil {
				return nil, err
			}
		}
		return f.newObject(remote, item), nil
	}
	return nil, fs.ErrorObjectNotFound
}

// DirMove moves a directory using FebBox server-side operations.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}
	srcID, _, srcLeaf, dstParentID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	if err = f.moveID(ctx, srcID, dstParentID); err != nil {
		return err
	}
	if srcLeaf != dstLeaf {
		if err = f.rename(ctx, srcID, dstLeaf); err != nil {
			return err
		}
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

func (f *Fs) addURL(ctx context.Context, rawURL string) (*api.AddURL, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid HTTP source URL %q", rawURL)
	}
	encodedRoot := f.opt.Enc.FromStandardPath(strings.Trim(f.root, "/"))
	var result api.Response[api.AddURL]
	err = f.callNoRetry(ctx, "file_add", url.Values{
		"url":  {rawURL},
		"path": {"/" + encodedRoot},
	}, &result)
	if err != nil {
		return nil, err
	}
	if err = apiError("add URL", result.Code, result.Msg); err != nil {
		return nil, err
	}
	if result.Data.TaskID == "" {
		return nil, errors.New("add URL: response contains no task ID")
	}
	return &result.Data, nil
}

var commandHelp = []fs.CommandHelp{{
	Name:  "addurl",
	Short: "Add a file from an HTTP URL.",
	Long: `This command asks FebBox to ingest a file from an HTTP or HTTPS URL.

` + "```console" + `
rclone backend addurl febbox:path https://example.com/file
` + "```",
}}

// Command runs a FebBox backend command.
func (f *Fs) Command(ctx context.Context, name string, args []string, opts map[string]string) (any, error) {
	switch name {
	case "addurl":
		if len(args) != 1 {
			return nil, errors.New("need exactly one URL argument")
		}
		return f.addURL(ctx, args[0])
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.Commander       = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
)
