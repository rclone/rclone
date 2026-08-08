// Package quark provides an interface to Quark Drive.
package quark

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image/color"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boombuler/barcode/qr"
	"github.com/rclone/rclone/backend/quark/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/sync/errgroup"
)

const (
	rootID                = "0"
	defaultUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"
	defaultListPageSize   = 500
	defaultUploadPartSize = int64(8 * fs.Mebi)
	maxUploadParts        = 10_000
	maxUploadPartThreads  = 32
	maxPartUploadAttempts = 3
	shareListPageSize     = 500
	cookieSaveInterval    = time.Minute
	configQRSession       = "config_qr_session"
	qrSuccessStatus       = 2_000_000
	qrNotScannedStatus    = 50_004_001
	qrScannedStatus       = 50_004_002
	ossUserAgent          = "aliyun-sdk-js/1.0.0 Chrome 127.0.0.0 on OS X 10.15.7 64-bit"
)

var (
	currentEndpoints = endpointSet{
		UOP:   "https://uop.quark.cn",
		Pan:   "https://pan.quark.cn",
		Drive: "https://drive-pc.quark.cn",
		Scan:  "https://su.quark.cn/4_eMHBJ?token=%s&client_id=532&ssb=weblogin&uc_param_str=&uc_biz_str=S%%3Acustom%%7COPT%%3ASAREA%%400%%7COPT%%3AIMMERSIVE%%401%%7COPT%%3ABACK_BTN_STYLE%%400",
	}
	qrPollInterval   = time.Second
	qrPollTimeout    = 2 * time.Minute
	taskPollInterval = 500 * time.Millisecond
	taskPollTimeout  = 2 * time.Minute
)

type endpointSet struct {
	UOP   string
	Pan   string
	Drive string
	Scan  string
}

// Options defines the configuration for this backend.
type Options struct {
	Cookie       string               `config:"cookie"`
	UserAgent    string               `config:"user_agent"`
	RootFolderID string               `config:"root_folder_id"`
	ListPageSize int                  `config:"list_page_size"`
	ChunkSize    fs.SizeSuffix        `config:"chunk_size"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a Quark Drive remote.
type Fs struct {
	name      string
	root      string
	opt       Options
	features  *fs.Features
	client    *http.Client
	srv       *rest.Client
	dirCache  *dircache.DirCache
	pacer     *fs.Pacer
	cookies   *cookieStore
	rootID    string
	endpoints endpointSet
	chunkSize fs.SizeSuffix
}

type cookieStore struct {
	mu           sync.RWMutex
	value        string
	lastSaved    string
	lastSave     time.Time
	name         string
	m            configmap.Mapper
	trustedHosts map[string]struct{}
}

type cookieTransport struct {
	base    http.RoundTripper
	cookies *cookieStore
}

// Object describes a Quark Drive file.
type Object struct {
	fs       *Fs
	remote   string
	id       string
	parentID string
	size     int64
	modTime  time.Time
	mimeType string
}

type createDirRequest struct {
	ParentID    string `json:"pdir_fid"`
	FileName    string `json:"file_name"`
	DirPath     string `json:"dir_path"`
	DirInitLock bool   `json:"dir_init_lock"`
}

type moveRequest struct {
	ActionType  int      `json:"action_type"`
	ExcludeFIDs []string `json:"exclude_fids"`
	FileList    []string `json:"filelist"`
	ToParentID  string   `json:"to_pdir_fid"`
}

type copyRequest = moveRequest

type renameRequest struct {
	ID       string `json:"fid"`
	FileName string `json:"file_name"`
}

type deleteRequest struct {
	ActionType  int      `json:"action_type"`
	ExcludeFIDs []string `json:"exclude_fids"`
	FileList    []string `json:"filelist"`
}

type uploadPreRequest struct {
	FileName       string `json:"file_name"`
	Size           int64  `json:"size"`
	ParentID       string `json:"pdir_fid"`
	HashUpdate     bool   `json:"ccp_hash_update"`
	DirName        string `json:"dir_name"`
	FormatType     string `json:"format_type"`
	CreatedAt      int64  `json:"l_created_at"`
	UpdatedAt      int64  `json:"l_updated_at"`
	ParallelUpload bool   `json:"parallel_upload"`
}

type updateHashRequest struct {
	TaskID string `json:"task_id"`
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
}

type uploadAuthRequest struct {
	AuthInfo json.RawMessage `json:"auth_info"`
	TaskID   string          `json:"task_id"`
	AuthMeta string          `json:"auth_meta"`
}

type shareRequest struct {
	FileIDs     []string `json:"fid_list"`
	Title       string   `json:"title"`
	URLType     int      `json:"url_type"`
	ExpiredType int      `json:"expired_type"`
}

type sharePasswordRequest struct {
	ShareID string `json:"share_id"`
}

type shareDeleteRequest struct {
	ShareIDs []string `json:"share_ids"`
}

type uploadedPart struct {
	Number int
	ETag   string
}

type qrCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type qrSession struct {
	RequestID string     `json:"request_id"`
	Token     string     `json:"token"`
	Cookies   []qrCookie `json:"cookies"`
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "quark",
		Description: "Quark Drive",
		NewFs:       NewFs,
		Config:      Config,
		Options: []fs.Option{
			{
				Name:       "cookie",
				Help:       "Quark Drive session cookie generated by QR login.",
				IsPassword: true,
				Sensitive:  true,
				Hide:       fs.OptionHideConfigurator,
			}, {
				Name:     "user_agent",
				Help:     "User agent used for Quark Drive API requests.",
				Default:  defaultUserAgent,
				Advanced: true,
			}, {
				Name: "root_folder_id",
				Help: `ID of the folder to use as the remote root.

Leave blank to use the account root.`,
				Advanced:  true,
				Sensitive: true,
			}, {
				Name:     "list_page_size",
				Help:     "Number of entries requested per directory-list page.",
				Default:  defaultListPageSize,
				Advanced: true,
			}, {
				Name: "chunk_size",
				Help: `Override the multipart upload chunk size.

Leave at zero to use the size recommended by Quark Drive.`,
				Default:  fs.SizeSuffix(0),
				Advanced: true,
			}, {
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Display |
					encoder.EncodeBackSlash |
					encoder.EncodeLeftSpace |
					encoder.EncodeLeftTilde |
					encoder.EncodeRightPeriod |
					encoder.EncodeRightSpace |
					encoder.EncodeWin |
					encoder.EncodeInvalidUtf8),
			},
		},
	})
}

// Config performs QR-code authentication.
func Config(ctx context.Context, name string, m configmap.Mapper, in fs.ConfigIn) (*fs.ConfigOut, error) {
	switch in.State {
	case "":
		if cookie, ok := m.Get("cookie"); ok && cookie != "" {
			return fs.ConfigConfirm("relogin", false, "config_relogin", "A Quark Drive login already exists. Log in again?")
		}
		return fs.ConfigGoto("qr_start")
	case "relogin":
		if in.Result != "true" {
			return nil, nil
		}
		return fs.ConfigGoto("qr_start")
	case "qr_start":
		session, scanURL, err := startQRCodeLogin(ctx, currentEndpoints)
		if err != nil {
			return nil, err
		}
		encoded, err := encodeQRSession(session)
		if err != nil {
			return nil, err
		}
		m.Set(configQRSession, encoded)
		code, err := renderQRCode(scanURL)
		if err != nil {
			return nil, err
		}
		help := fmt.Sprintf("Scan this QR code with the Quark app, then confirm.\n\n%s\nIf the code does not render, open:\n%s", code, scanURL)
		return fs.ConfigConfirm("qr_poll", true, "config_scan_complete", help)
	case "qr_poll":
		if in.Result != "true" {
			return nil, errors.New("quark drive QR login cancelled")
		}
		encoded, ok := m.Get(configQRSession)
		if !ok || encoded == "" {
			return fs.ConfigGoto("qr_start")
		}
		session, err := decodeQRSession(encoded)
		if err != nil {
			return nil, err
		}
		cookie, err := finishQRCodeLogin(ctx, currentEndpoints, session)
		if err != nil {
			return nil, err
		}
		m.Set("cookie", obscure.MustObscure(cookie))
		m.Set(configQRSession, "")
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown Quark Drive config state %q", in.State)
	}
}

func startQRCodeLogin(ctx context.Context, endpoints endpointSet) (qrSession, string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return qrSession{}, "", err
	}
	client := fshttp.NewClient(ctx)
	client.Jar = jar
	requestID := random.String(32)
	u, err := url.Parse(endpoints.UOP + "/cas/ajax/getTokenForQrcodeLogin")
	if err != nil {
		return qrSession{}, "", err
	}
	q := u.Query()
	q.Set("client_id", "532")
	q.Set("v", "1.2")
	q.Set("request_id", requestID)
	u.RawQuery = q.Encode()
	var response api.QRResponse
	if err = loginJSON(ctx, client, http.MethodGet, u.String(), endpoints.Pan, &response); err != nil {
		return qrSession{}, "", err
	}
	if response.Status != qrSuccessStatus || response.Data.Members.Token == "" {
		return qrSession{}, "", fmt.Errorf("failed to start Quark Drive QR login: status=%d message=%q", response.Status, response.Message)
	}
	cookies := make([]qrCookie, 0)
	for _, cookie := range jar.Cookies(u) {
		cookies = append(cookies, qrCookie{Name: cookie.Name, Value: cookie.Value})
	}
	session := qrSession{RequestID: requestID, Token: response.Data.Members.Token, Cookies: cookies}
	return session, fmt.Sprintf(endpoints.Scan, url.QueryEscape(session.Token)), nil
}

func finishQRCodeLogin(ctx context.Context, endpoints endpointSet, session qrSession) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", err
	}
	uopURL, err := url.Parse(endpoints.UOP)
	if err != nil {
		return "", err
	}
	cookies := make([]*http.Cookie, 0, len(session.Cookies))
	for _, cookie := range session.Cookies {
		cookies = append(cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value, Path: "/"})
	}
	jar.SetCookies(uopURL, cookies)
	client := fshttp.NewClient(ctx)
	client.Jar = jar
	deadline := time.NewTimer(qrPollTimeout)
	defer deadline.Stop()

	var serviceTicket string
	for serviceTicket == "" {
		u, err := url.Parse(endpoints.UOP + "/cas/ajax/getServiceTicketByQrcodeToken")
		if err != nil {
			return "", err
		}
		q := u.Query()
		q.Set("client_id", "532")
		q.Set("v", "1.2")
		q.Set("token", session.Token)
		q.Set("request_id", session.RequestID)
		u.RawQuery = q.Encode()
		var response api.QRResponse
		if err = loginJSON(ctx, client, http.MethodGet, u.String(), endpoints.Pan, &response); err != nil {
			return "", err
		}
		switch response.Status {
		case qrSuccessStatus:
			serviceTicket = response.Data.Members.ServiceTicket
			if serviceTicket == "" {
				return "", errors.New("quark drive QR login returned no service ticket")
			}
		case qrNotScannedStatus, qrScannedStatus:
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-deadline.C:
				return "", errors.New("quark drive QR login timed out")
			case <-time.After(qrPollInterval):
			}
		default:
			return "", fmt.Errorf("quark drive QR login failed: status=%d message=%q", response.Status, response.Message)
		}
	}

	accountURL, err := url.Parse(endpoints.Pan + "/account/info")
	if err != nil {
		return "", err
	}
	q := accountURL.Query()
	q.Set("st", serviceTicket)
	q.Set("lw", "scan")
	accountURL.RawQuery = q.Encode()
	var account api.AccountResponse
	if err = loginJSON(ctx, client, http.MethodGet, accountURL.String(), endpoints.Pan, &account); err != nil {
		return "", err
	}
	if !account.Success {
		return "", fmt.Errorf("quark drive account validation failed: code=%q message=%q", account.Code, account.Message)
	}

	listURL, err := url.Parse(endpoints.Drive + "/1/clouddrive/file/sort")
	if err != nil {
		return "", err
	}
	q = listURL.Query()
	q.Set("pr", "ucpro")
	q.Set("fr", "pc")
	q.Set("uc_param_str", "")
	q.Set("pdir_fid", rootID)
	q.Set("_page", "1")
	q.Set("_size", "1")
	listURL.RawQuery = q.Encode()
	var list api.ListResponse
	if err = loginJSON(ctx, client, http.MethodGet, listURL.String(), endpoints.Pan, &list); err != nil {
		return "", err
	}
	if err = list.Response.Check(); err != nil {
		return "", fmt.Errorf("quark drive session bootstrap failed: %w", err)
	}

	seen := map[string]bool{}
	values := make([]string, 0)
	for _, rawURL := range []string{endpoints.Pan, endpoints.Drive, endpoints.UOP} {
		u, parseErr := url.Parse(rawURL)
		if parseErr != nil {
			return "", parseErr
		}
		for _, cookie := range jar.Cookies(u) {
			entry := cookie.Name + "=" + cookie.Value
			if !seen[entry] {
				seen[entry] = true
				values = append(values, entry)
			}
		}
	}
	if len(values) == 0 {
		return "", errors.New("quark drive QR login returned no cookies")
	}
	sort.Strings(values)
	return strings.Join(values, "; "), nil
}

func loginJSON(ctx context.Context, client *http.Client, method, rawURL, origin string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", strings.TrimSuffix(origin, "/"))
	req.Header.Set("Referer", strings.TrimSuffix(origin, "/")+"/")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("quark drive login returned HTTP %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func encodeQRSession(session qrSession) (string, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	return obscure.MustObscure(base64.RawURLEncoding.EncodeToString(data)), nil
}

func decodeQRSession(encoded string) (qrSession, error) {
	revealed, err := obscure.Reveal(encoded)
	if err != nil {
		return qrSession{}, fmt.Errorf("corrupt Quark Drive QR session: %w", err)
	}
	data, err := base64.RawURLEncoding.DecodeString(revealed)
	if err != nil {
		return qrSession{}, fmt.Errorf("corrupt Quark Drive QR session: %w", err)
	}
	var session qrSession
	if err = json.Unmarshal(data, &session); err != nil {
		return qrSession{}, fmt.Errorf("corrupt Quark Drive QR session: %w", err)
	}
	return session, nil
}

func renderQRCode(content string) (string, error) {
	code, err := qr.Encode(content, qr.L, qr.Auto)
	if err != nil {
		return "", err
	}
	bounds := code.Bounds()
	const quiet = 4
	isBlack := func(x, y int) bool {
		if x < 0 || y < 0 || x >= bounds.Dx() || y >= bounds.Dy() {
			return false
		}
		gray := color.GrayModel.Convert(code.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
		return gray.Y < 128
	}
	var out strings.Builder
	for y := -quiet; y < bounds.Dy()+quiet; y += 2 {
		for x := -quiet; x < bounds.Dx()+quiet; x++ {
			top, bottom := isBlack(x, y), isBlack(x, y+1)
			switch {
			case top && bottom:
				out.WriteString("\x1b[40m ")
			case top:
				out.WriteString("\x1b[30;107m▀")
			case bottom:
				out.WriteString("\x1b[97;40m▀")
			default:
				out.WriteString("\x1b[107m ")
			}
		}
		out.WriteString("\x1b[0m\n")
	}
	return out.String(), nil
}

// NewFs constructs an Fs from the path.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	f, err := newFs(ctx, name, root, m)
	if err != nil {
		return nil, err
	}
	err = f.dirCache.FindRoot(ctx, false)
	if errors.Is(err, fs.ErrorDirNotFound) {
		return f, nil
	}
	if err == nil {
		return f, nil
	}

	parent, leaf := dircache.SplitPath(f.root)
	temp := *f
	temp.root = parent
	temp.dirCache = dircache.New(parent, f.rootID, &temp)
	if findErr := temp.dirCache.FindRoot(ctx, false); findErr != nil {
		return f, err
	}
	if _, objectErr := temp.NewObject(ctx, leaf); objectErr != nil {
		return f, err
	}
	f.root = parent
	f.dirCache = temp.dirCache
	f.features.Fill(ctx, f)
	return f, fs.ErrorIsFile
}

func newCookieStore(name, value string, m configmap.Mapper, endpoints endpointSet) *cookieStore {
	trustedHosts := make(map[string]struct{}, 3)
	for _, rawURL := range []string{endpoints.Pan, endpoints.Drive, endpoints.UOP} {
		u, err := url.Parse(rawURL)
		if err == nil && u.Host != "" {
			trustedHosts[strings.ToLower(u.Host)] = struct{}{}
		}
	}
	return &cookieStore{value: value, lastSaved: value, name: name, m: m, trustedHosts: trustedHosts}
}

func (s *cookieStore) trusted(u *url.URL) bool {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "quark.cn" || strings.HasSuffix(host, ".quark.cn") {
		return true
	}
	_, ok := s.trustedHosts[strings.ToLower(u.Host)]
	return ok
}

func (s *cookieStore) header() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func mergeCookies(header string, updates []*http.Cookie) string {
	parts := strings.Split(header, ";")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	for _, update := range updates {
		if update.Name == "" {
			continue
		}
		deleteCookie := update.MaxAge < 0 || (!update.Expires.IsZero() && update.Expires.Before(time.Now()))
		replacement := (&http.Cookie{Name: update.Name, Value: update.Value, Quoted: update.Quoted}).String()
		found := false
		for i, part := range parts {
			name, _, ok := strings.Cut(part, "=")
			if !ok || strings.TrimSpace(name) != update.Name {
				continue
			}
			if !found && !deleteCookie {
				parts[i] = replacement
				found = true
			} else {
				parts[i] = ""
			}
		}
		if !found && !deleteCookie {
			parts = append(parts, replacement)
		}
	}
	merged := parts[:0]
	for _, part := range parts {
		if part != "" {
			merged = append(merged, part)
		}
	}
	return strings.Join(merged, "; ")
}

func (s *cookieStore) update(updates []*http.Cookie) {
	if len(updates) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value := mergeCookies(s.value, updates)
	if value == s.value {
		return
	}
	s.value = value
	if time.Since(s.lastSave) >= cookieSaveInterval {
		s.saveLocked()
	}
}

func (s *cookieStore) saveLocked() {
	if s.value == s.lastSaved {
		return
	}
	s.m.Set("cookie", obscure.MustObscure(s.value))
	s.lastSaved = s.value
	s.lastSave = time.Now()
	fs.Debugf(s.name, "Saved refreshed cookies in config file")
}

func (s *cookieStore) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLocked()
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	request := req.Clone(req.Context())
	request.Header = req.Header.Clone()
	trusted := t.cookies.trusted(request.URL)
	if trusted {
		request.Header.Set("Cookie", t.cookies.header())
	} else {
		request.Header.Del("Cookie")
	}
	resp, err := t.base.RoundTrip(request)
	if resp != nil && trusted {
		t.cookies.update(resp.Cookies())
	}
	return resp, err
}

func newFs(ctx context.Context, name, root string, m configmap.Mapper) (*Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	cookie, err := obscure.Reveal(opt.Cookie)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal Quark Drive cookie: %w", err)
	}
	if cookie == "" {
		return nil, errors.New("quark drive cookie is empty; run rclone config reconnect to scan a QR code")
	}
	if opt.ListPageSize <= 0 {
		opt.ListPageSize = defaultListPageSize
	}
	root = strings.Trim(root, "/")
	newCtx, ci := fs.AddConfig(ctx)
	ci.UserAgent = opt.UserAgent
	client := fshttp.NewClient(newCtx)
	cookies := newCookieStore(name, cookie, m, currentEndpoints)
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = &cookieTransport{base: transport, cookies: cookies}
	srv := rest.NewClient(client).SetRoot(currentEndpoints.Drive)
	srv.SetHeader("Accept", "application/json, text/plain, */*")
	srv.SetHeader("Origin", currentEndpoints.Pan)
	srv.SetHeader("Referer", strings.TrimSuffix(currentEndpoints.Pan, "/")+"/")
	rootFolderID := opt.RootFolderID
	if rootFolderID == "" {
		rootFolderID = rootID
	}
	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		client:    client,
		srv:       srv,
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(100*time.Millisecond), pacer.MaxSleep(2*time.Second), pacer.DecayConstant(2))),
		cookies:   cookies,
		rootID:    rootFolderID,
		endpoints: currentEndpoints,
		chunkSize: opt.ChunkSize,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)
	f.dirCache = dircache.New(root, rootFolderID, f)
	return f, nil
}

func errorHandler(resp *http.Response) error {
	errResponse := new(api.Error)
	errResponse.HTTPStatus = resp.StatusCode
	if err := rest.DecodeJSON(resp, errResponse); err != nil {
		return fmt.Errorf("failed to decode Quark Drive error response: %w", err)
	}
	return errResponse
}

var retryErrorCodes = []int{408, 409, 429, 500, 502, 503, 504, 509}

var errItemWaitTimeout = errors.New("timed out waiting for Quark Drive item state")

func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func apiParameters(values url.Values) url.Values {
	params := make(url.Values, len(values)+3)
	for key, entries := range values {
		params[key] = append([]string(nil), entries...)
	}
	params.Set("pr", "ucpro")
	params.Set("fr", "pc")
	if _, ok := params["uc_param_str"]; !ok {
		params.Set("uc_param_str", "")
	}
	return params
}

func (f *Fs) callJSON(ctx context.Context, method, requestPath string, params url.Values, in, out any) error {
	opts := rest.Opts{Method: method, Path: requestPath, Parameters: apiParameters(params)}
	err := f.pacer.Call(func() (bool, error) {
		resp, callErr := f.srv.CallJSON(ctx, &opts, in, out)
		return f.shouldRetry(ctx, resp, callErr)
	})
	return err
}

// Name returns the configured remote name.
func (f *Fs) Name() string { return f.name }

// Root returns the remote root path.
func (f *Fs) Root() string { return f.root }

// String returns a description of the remote.
func (f *Fs) String() string { return fmt.Sprintf("Quark Drive root %q", f.root) }

// Precision returns the timestamp precision supported by the API.
func (f *Fs) Precision() time.Duration { return time.Millisecond }

// Hashes reports that Quark Drive does not expose persistent file hashes.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

// Features returns optional backend capabilities.
func (f *Fs) Features() *fs.Features { return f.features }

// DirCacheFlush clears cached directory IDs.
func (f *Fs) DirCacheFlush() { f.dirCache.ResetRoot() }

// Shutdown saves the latest cookies received from Quark Drive.
func (f *Fs) Shutdown(ctx context.Context) error {
	f.cookies.flush()
	return nil
}

func decodeFileName(name string, enc encoder.MultiEncoder) string {
	name = html.UnescapeString(name)
	return enc.ToStandardName(name)
}

func encodeFileName(name string, enc encoder.MultiEncoder) string {
	return html.EscapeString(enc.FromStandardName(name))
}

func (f *Fs) listAll(ctx context.Context, parentID string) ([]api.Item, error) {
	items := make([]api.Item, 0)
	for pageNumber := 1; ; pageNumber++ {
		params := url.Values{
			"pdir_fid":             []string{parentID},
			"_page":                []string{strconv.Itoa(pageNumber)},
			"_size":                []string{strconv.Itoa(f.opt.ListPageSize)},
			"_fetch_total":         []string{"1"},
			"_fetch_sub_dirs":      []string{"0"},
			"_sort":                []string{"file_type:asc,updated_at:desc"},
			"fetch_all_file":       []string{"1"},
			"fetch_risk_file_name": []string{"1"},
		}
		var response api.ListResponse
		if err := f.callJSON(ctx, http.MethodGet, "/1/clouddrive/file/sort", params, nil, &response); err != nil {
			return nil, err
		}
		if err := response.Response.Check(); err != nil {
			return nil, err
		}
		for i := range response.Data.List {
			response.Data.List[i].FileName = decodeFileName(response.Data.List[i].FileName, f.opt.Enc)
		}
		items = append(items, response.Data.List...)
		if len(response.Data.List) < f.opt.ListPageSize {
			return items, nil
		}
	}
}

func (f *Fs) findItem(ctx context.Context, parentID, leaf string) (*api.Item, error) {
	items, err := f.listAll(ctx, parentID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].FileName == leaf {
			return &items[i], nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) waitForItem(ctx context.Context, parentID, id, leaf string, present bool) (*api.Item, error) {
	waitCtx, cancel := context.WithTimeout(ctx, taskPollTimeout)
	defer cancel()
	for {
		items, err := f.listAll(waitCtx, parentID)
		if err != nil {
			if waitCtx.Err() != nil {
				return nil, fmt.Errorf("%w: %w", errItemWaitTimeout, err)
			}
			return nil, err
		}
		var found *api.Item
		for i := range items {
			if items[i].FID == id {
				found = &items[i]
				break
			}
		}
		if present && found != nil && (leaf == "" || found.FileName == leaf) {
			return found, nil
		}
		if !present && found == nil {
			return nil, nil
		}
		select {
		case <-waitCtx.Done():
			state := "disappear"
			if present {
				state = "become visible"
			}
			return nil, fmt.Errorf("%w: item %q did not %s: %w", errItemWaitTimeout, id, state, waitCtx.Err())
		case <-time.After(taskPollInterval):
		}
	}
}

// FindLeaf implements dircache.DirCacher.
func (f *Fs) FindLeaf(ctx context.Context, parentID, leaf string) (string, bool, error) {
	item, err := f.findItem(ctx, parentID, leaf)
	if errors.Is(err, fs.ErrorObjectNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !item.IsDir() {
		return "", false, fs.ErrorIsFile
	}
	return item.FID, true, nil
}

func (f *Fs) createDir(ctx context.Context, parentID, leaf string) (string, error) {
	request := createDirRequest{
		ParentID:    parentID,
		FileName:    encodeFileName(leaf, f.opt.Enc),
		DirPath:     "",
		DirInitLock: false,
	}
	var response api.IDResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file", nil, &request, &response); err != nil {
		return "", err
	}
	if err := response.Response.Check(); err != nil {
		return "", err
	}
	if response.Data.FID == "" {
		return "", errors.New("quark drive create directory returned no ID")
	}
	return response.Data.FID, nil
}

// CreateDir implements dircache.DirCacher.
func (f *Fs) CreateDir(ctx context.Context, parentID, leaf string) (string, error) {
	id, err := f.createDir(ctx, parentID, leaf)
	if err != nil {
		return "", err
	}
	_, err = f.waitForItem(ctx, parentID, id, leaf, true)
	return id, err
}

// Mkdir creates a directory and any missing parents.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

func (f *Fs) itemToEntry(ctx context.Context, remote string, info *api.Item) (fs.DirEntry, error) {
	if info.IsDir() {
		f.dirCache.Put(remote, info.FID)
		return fs.NewDir(remote, info.ModTime()).SetID(info.FID).SetParentID(info.ParentFID), nil
	}
	return f.newObjectWithInfo(ctx, remote, info)
}

// List lists the direct children of dir.
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
		entry, entryErr := f.itemToEntry(ctx, path.Join(dir, items[i].FileName), &items[i])
		if entryErr != nil {
			return nil, entryErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// ListR recursively lists dir for --fast-list operations.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	parentID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	var walk func(string, string) error
	walk = func(remoteDir, id string) error {
		items, listErr := f.listAll(ctx, id)
		if listErr != nil {
			return listErr
		}
		entries := make(fs.DirEntries, 0, len(items))
		dirs := make([]struct {
			remote string
			id     string
		}, 0)
		for i := range items {
			remote := path.Join(remoteDir, items[i].FileName)
			entry, entryErr := f.itemToEntry(ctx, remote, &items[i])
			if entryErr != nil {
				return entryErr
			}
			entries = append(entries, entry)
			if items[i].IsDir() {
				dirs = append(dirs, struct {
					remote string
					id     string
				}{remote: remote, id: items[i].FID})
			}
		}
		if len(entries) > 0 {
			if callbackErr := callback(entries); callbackErr != nil {
				return callbackErr
			}
		}
		for _, child := range dirs {
			if walkErr := walk(child.remote, child.id); walkErr != nil {
				return walkErr
			}
		}
		return nil
	}
	return walk(dir, parentID)
}

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (*Object, error) {
	o := &Object{fs: f, remote: remote}
	if info == nil {
		var err error
		info, err = f.readMetaDataForPath(ctx, remote)
		if err != nil {
			return nil, err
		}
	}
	if info.IsDir() {
		return nil, fs.ErrorIsDir
	}
	o.setMetaData(info)
	return o, nil
}

func (f *Fs) readMetaDataForPath(ctx context.Context, remote string) (*api.Item, error) {
	leaf, parentID, err := f.dirCache.FindPath(ctx, remote, false)
	if errors.Is(err, fs.ErrorDirNotFound) {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	item, err := f.findItem(ctx, parentID, leaf)
	if err != nil {
		return nil, err
	}
	if item.IsDir() {
		return nil, fs.ErrorIsDir
	}
	return item, nil
}

// NewObject finds a file by remote path.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.newObjectWithInfo(ctx, remote, nil)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Object) setMetaData(info *api.Item) {
	o.id = info.FID
	o.parentID = info.ParentFID
	o.size = info.Size
	o.modTime = info.ModTime()
	o.mimeType = info.FormatType
}

func (o *Object) copyFrom(other *Object) {
	o.fs = other.fs
	o.remote = other.remote
	o.id = other.id
	o.parentID = other.parentID
	o.size = other.size
	o.modTime = other.modTime
	o.mimeType = other.mimeType
}

// String returns a description of the object.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Fs returns the parent filesystem.
func (o *Object) Fs() fs.Info { return o.fs }

// Remote returns the object path relative to the Fs root.
func (o *Object) Remote() string { return o.remote }

// ModTime returns the best modification time provided by Quark Drive.
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// Size returns the object size in bytes.
func (o *Object) Size() int64 { return o.size }

// Storable reports that this is a regular storable object.
func (o *Object) Storable() bool { return true }

// ID returns the Quark Drive file ID.
func (o *Object) ID() string { return o.id }

// ParentID returns the Quark Drive parent folder ID.
func (o *Object) ParentID() string { return o.parentID }

// MimeType returns the media type reported by Quark Drive.
func (o *Object) MimeType(ctx context.Context) string { return o.mimeType }

// Hash reports that Quark Drive does not expose persistent content hashes.
func (o *Object) Hash(ctx context.Context, hashType hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime reports that existing Quark Drive objects cannot be retimestamped.
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

func (f *Fs) deleteItem(ctx context.Context, id string) error {
	request := deleteRequest{ActionType: 1, ExcludeFIDs: []string{}, FileList: []string{id}}
	var response api.IDResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/delete", nil, &request, &response); err != nil {
		return err
	}
	return response.Response.Check()
}

func (f *Fs) deleteAndWait(ctx context.Context, id, parentID string) error {
	if err := f.deleteItem(ctx, id); err != nil {
		return err
	}
	if parentID == "" {
		return nil
	}
	_, err := f.waitForItem(ctx, parentID, id, "", false)
	return err
}

// Remove moves this object to the Quark Drive recycle bin.
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteAndWait(ctx, o.id, o.parentID)
}

func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	if check {
		items, listErr := f.listAll(ctx, id)
		if listErr != nil {
			return listErr
		}
		if len(items) != 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	if dir == "" && f.root == "" {
		return errors.New("can't remove the Quark Drive account root")
	}
	parentID := ""
	if dir != "" {
		parentDir := path.Dir(dir)
		if parentDir == "." {
			parentDir = ""
		}
		parentID, err = f.dirCache.FindDir(ctx, parentDir, false)
		if err != nil {
			return err
		}
	}
	if err = f.deleteAndWait(ctx, id, parentID); err != nil {
		return err
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir removes an empty directory.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge removes a directory tree with one server-side operation.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

func (f *Fs) moveItem(ctx context.Context, id, parentID string) error {
	request := moveRequest{ActionType: 1, ExcludeFIDs: []string{}, FileList: []string{id}, ToParentID: parentID}
	var response api.IDResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/move", nil, &request, &response); err != nil {
		return err
	}
	return response.Response.Check()
}

func (f *Fs) moveAndWait(ctx context.Context, id, parentID string) (*api.Item, error) {
	if err := f.moveItem(ctx, id, parentID); err != nil {
		return nil, err
	}
	return f.waitForItem(ctx, parentID, id, "", true)
}

func (f *Fs) copyItem(ctx context.Context, id, parentID string) (string, error) {
	request := copyRequest{ActionType: 1, ExcludeFIDs: []string{}, FileList: []string{id}, ToParentID: parentID}
	var response api.IDResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/copy", nil, &request, &response); err != nil {
		return "", err
	}
	if err := response.Response.Check(); err != nil {
		return "", err
	}
	if response.Data.FID == "" {
		return "", errors.New("quark drive copy returned no file ID")
	}
	return response.Data.FID, nil
}

func (f *Fs) renameItem(ctx context.Context, id, newName string) error {
	request := renameRequest{ID: id, FileName: encodeFileName(newName, f.opt.Enc)}
	var response api.IDResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/rename", nil, &request, &response); err != nil {
		return err
	}
	return response.Response.Check()
}

func (f *Fs) renameAndWait(ctx context.Context, id, parentID, newName string) (*api.Item, error) {
	if err := f.renameItem(ctx, id, newName); err != nil {
		return nil, err
	}
	return f.waitForItem(ctx, parentID, id, newName, true)
}

func (f *Fs) removeDestination(ctx context.Context, remote string) error {
	destination, err := f.NewObject(ctx, remote)
	if errors.Is(err, fs.ErrorObjectNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return destination.Remove(ctx)
}

// Move moves and optionally renames a file on the server.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObject, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	if path.Join(srcObject.fs.root, srcObject.remote) == path.Join(f.root, remote) {
		return src, nil
	}
	leaf, parentID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	if err = f.removeDestination(ctx, remote); err != nil {
		return nil, err
	}
	var info *api.Item
	if srcObject.parentID != parentID {
		info, err = f.moveAndWait(ctx, srcObject.id, parentID)
		if err != nil {
			return nil, err
		}
	}
	if path.Base(srcObject.remote) != leaf {
		info, err = f.renameAndWait(ctx, srcObject.id, parentID, leaf)
		if err != nil {
			return nil, err
		}
	}
	destination := *srcObject
	destination.fs = f
	destination.remote = remote
	destination.parentID = parentID
	if info != nil {
		destination.setMetaData(info)
	}
	return &destination, nil
}

// Copy copies and optionally renames a file on the server.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObject, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	leaf, parentID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	if err = f.removeDestination(ctx, remote); err != nil {
		return nil, err
	}
	id, err := f.copyItem(ctx, srcObject.id, parentID)
	if err != nil {
		return nil, err
	}
	// Quark may temporarily suffix a copied name when the source and
	// destination share a directory. Wait for the returned ID, then rename it.
	info, err := f.waitForItem(ctx, parentID, id, "", true)
	if err != nil {
		return nil, err
	}
	if path.Base(srcObject.remote) != leaf {
		info, err = f.renameAndWait(ctx, id, parentID, leaf)
		if err != nil {
			_ = f.deleteAndWait(ctx, id, parentID)
			return nil, err
		}
	}
	destination, err := f.newObjectWithInfo(ctx, remote, info)
	if err != nil {
		return nil, err
	}
	return destination, nil
}

// DirMove moves and optionally renames a directory on the server.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}
	srcID, srcParentID, srcLeaf, dstParentID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	if srcParentID != dstParentID {
		if _, err = f.moveAndWait(ctx, srcID, dstParentID); err != nil {
			return err
		}
	}
	if srcLeaf != dstLeaf {
		if _, err = f.renameAndWait(ctx, srcID, dstParentID, dstLeaf); err != nil {
			return err
		}
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

func (f *Fs) pollTask(ctx context.Context, taskID string) (*api.TaskResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, taskPollTimeout)
	defer cancel()
	for {
		params := url.Values{"task_id": []string{taskID}, "retry_index": []string{"0"}}
		var response api.TaskResponse
		if err := f.callJSON(ctx, http.MethodGet, "/1/clouddrive/task", params, nil, &response); err != nil {
			return nil, err
		}
		if err := response.Response.Check(); err != nil {
			return nil, err
		}
		switch response.Data.Status {
		case 2:
			return &response, nil
		case 3:
			return nil, fmt.Errorf("quark drive task %q failed", taskID)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for Quark Drive task %q: %w", taskID, ctx.Err())
		case <-time.After(taskPollInterval):
		}
	}
}

func (f *Fs) getDownloadURL(ctx context.Context, id string) (string, error) {
	request := map[string]any{"fids": []string{id}}
	var response api.DownloadResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/download", nil, &request, &response); err != nil {
		return "", err
	}
	if err := response.Response.Check(); err != nil {
		return "", err
	}
	var direct []api.DownloadItem
	if err := json.Unmarshal(response.Data, &direct); err == nil && len(direct) > 0 && direct[0].DownloadURL != "" {
		return direct[0].DownloadURL, nil
	}
	var asynchronous api.DownloadTask
	if err := json.Unmarshal(response.Data, &asynchronous); err != nil {
		return "", fmt.Errorf("invalid Quark Drive download response: %w", err)
	}
	if asynchronous.TaskResp != nil && len(asynchronous.TaskResp.Data) > 0 && asynchronous.TaskResp.Data[0].DownloadURL != "" {
		return asynchronous.TaskResp.Data[0].DownloadURL, nil
	}
	if asynchronous.TaskID == "" {
		return "", errors.New("quark drive download response contained no URL or task ID")
	}
	task, err := f.pollTask(ctx, asynchronous.TaskID)
	if err != nil {
		return "", err
	}
	if task.Data.DownloadURL != "" {
		return task.Data.DownloadURL, nil
	}
	if len(task.Data.Data) > 0 && task.Data.Data[0].DownloadURL != "" {
		return task.Data.Data[0].DownloadURL, nil
	}
	return "", errors.New("quark drive download task completed without a URL")
}

func isQuarkHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	return host == "quark.cn" || strings.HasSuffix(host, ".quark.cn")
}

// Open opens the object for reading, including ranged reads.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.size == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	downloadURL, err := o.fs.getDownloadURL(ctx, o.id)
	if err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.size)
	downloadSrv := rest.NewClient(o.fs.client)
	opts := rest.Opts{Method: http.MethodGet, RootURL: downloadURL, Options: options}
	var response *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		var callErr error
		response, callErr = downloadSrv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, response, callErr)
	})
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}

// About returns account storage usage.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	params := url.Values{"fetch_subscribe": []string{"true"}, "fetch_identity": []string{"true"}}
	var response api.MemberResponse
	if err := f.callJSON(ctx, http.MethodGet, "/1/clouddrive/member", params, nil, &response); err != nil {
		return nil, err
	}
	if err := response.Response.Check(); err != nil {
		return nil, err
	}
	usage := &fs.Usage{
		Total: fs.NewUsageValue(response.Data.Total),
		Used:  fs.NewUsageValue(response.Data.Used),
	}
	if response.Data.Total >= response.Data.Used {
		usage.Free = fs.NewUsageValue(response.Data.Total - response.Data.Used)
	}
	return usage, nil
}

func (f *Fs) accountInfo(ctx context.Context) (*api.AccountResponse, error) {
	opts := rest.Opts{
		Method:     http.MethodGet,
		RootURL:    strings.TrimSuffix(f.endpoints.Pan, "/") + "/account/info",
		Parameters: url.Values{"fr": []string{"pc"}, "platform": []string{"pc"}},
	}
	var response api.AccountResponse
	err := f.pacer.Call(func() (bool, error) {
		httpResponse, callErr := f.srv.CallJSON(ctx, &opts, nil, &response)
		return f.shouldRetry(ctx, httpResponse, callErr)
	})
	if err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("quark drive account API error: code=%q message=%q", response.Code, response.Message)
	}
	return &response, nil
}

// UserInfo returns non-sensitive information about the connected account.
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	account, err := f.accountInfo(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	fields := map[string]string{
		"nickname":    "Nickname",
		"member_type": "MemberType",
		"user_id":     "UserID",
		"avatar_url":  "AvatarURL",
	}
	for source, destination := range fields {
		if value, ok := account.Data[source]; ok && value != nil {
			result[destination] = fmt.Sprint(value)
		}
	}
	return result, nil
}

func shareExpiryType(expiry time.Duration) int {
	if expiry <= 0 || expiry >= 365*24*time.Hour {
		return 1
	}
	if expiry <= 24*time.Hour {
		return 2
	}
	if expiry <= 7*24*time.Hour {
		return 3
	}
	return 4
}

func (f *Fs) createShare(ctx context.Context, title string, ids []string, expiry time.Duration) (string, error) {
	request := shareRequest{FileIDs: ids, Title: title, URLType: 1, ExpiredType: shareExpiryType(expiry)}
	var response api.ShareResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/share", nil, &request, &response); err != nil {
		return "", err
	}
	if err := response.Response.Check(); err != nil {
		return "", err
	}
	shareID := ""
	if response.Data.TaskResp != nil {
		shareID = response.Data.TaskResp.Data.ShareID
	}
	if shareID == "" && response.Data.TaskID != "" {
		task, err := f.pollTask(ctx, response.Data.TaskID)
		if err != nil {
			return "", err
		}
		shareID = task.Data.ShareID
	}
	if shareID == "" {
		return "", errors.New("quark drive share creation returned no share ID")
	}
	passwordRequest := sharePasswordRequest{ShareID: shareID}
	var passwordResponse api.SharePasswordResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/share/password", nil, &passwordRequest, &passwordResponse); err != nil {
		return "", err
	}
	if err := passwordResponse.Response.Check(); err != nil {
		return "", err
	}
	if passwordResponse.Data.ShareURL == "" {
		return "", errors.New("quark drive share password response returned no URL")
	}
	return passwordResponse.Data.ShareURL, nil
}

func (f *Fs) unlinkPublicLinks(ctx context.Context, id string) error {
	shareIDs := make([]string, 0)
	for pageNumber := 1; ; pageNumber++ {
		params := url.Values{
			"_page":                []string{strconv.Itoa(pageNumber)},
			"_size":                []string{strconv.Itoa(shareListPageSize)},
			"_order_field":         []string{"created_at"},
			"_order_type":          []string{"desc"},
			"_fetch_total":         []string{"1"},
			"_fetch_notify_follow": []string{"1"},
		}
		var response api.ShareListResponse
		if err := f.callJSON(ctx, http.MethodGet, "/1/clouddrive/share/mypage/detail", params, nil, &response); err != nil {
			return err
		}
		if err := response.Response.Check(); err != nil {
			return err
		}
		for _, share := range response.Data.List {
			if share.FirstFID == id && share.ShareID != "" && share.Status != 3 {
				shareIDs = append(shareIDs, share.ShareID)
			}
		}
		if len(response.Data.List) < shareListPageSize {
			break
		}
	}
	if len(shareIDs) == 0 {
		return nil
	}
	request := shareDeleteRequest{ShareIDs: shareIDs}
	var response api.IDResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/share/delete", nil, &request, &response); err != nil {
		return err
	}
	return response.Response.Check()
}

// PublicLink creates a public Quark Drive share link.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	id, err := f.dirCache.FindDir(ctx, remote, false)
	if err != nil {
		object, objectErr := f.NewObject(ctx, remote)
		if objectErr != nil {
			return "", objectErr
		}
		id = object.(*Object).id
	}
	if unlink {
		return "", f.unlinkPublicLinks(ctx, id)
	}
	title := path.Base(remote)
	if title == "." || title == "/" || title == "" {
		title = "rclone share"
	}
	duration := time.Duration(expire)
	if expire == fs.DurationOff {
		duration = 0
	}
	return f.createShare(ctx, title, []string{id}, duration)
}

func spoolInput(in io.Reader, expectedSize int64) (file *os.File, size int64, md5sum, sha1sum string, cleanup func(), err error) {
	file, err = os.CreateTemp("", "rclone-quark-upload-")
	if err != nil {
		return nil, 0, "", "", func() {}, err
	}
	cleanup = func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}
	hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(hash.MD5, hash.SHA1))
	if err != nil {
		cleanup()
		return nil, 0, "", "", func() {}, err
	}
	size, err = io.Copy(io.MultiWriter(file, hasher), in)
	if err != nil {
		cleanup()
		return nil, 0, "", "", func() {}, err
	}
	if expectedSize >= 0 && size != expectedSize {
		cleanup()
		return nil, 0, "", "", func() {}, fmt.Errorf("source size changed during upload: expected=%d actual=%d", expectedSize, size)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, 0, "", "", func() {}, err
	}
	md5sum, err = hasher.SumString(hash.MD5, false)
	if err != nil {
		cleanup()
		return nil, 0, "", "", func() {}, err
	}
	sha1sum, err = hasher.SumString(hash.SHA1, false)
	if err != nil {
		cleanup()
		return nil, 0, "", "", func() {}, err
	}
	return file, size, md5sum, sha1sum, cleanup, nil
}

func (f *Fs) uploadPre(ctx context.Context, leaf, parentID, mimeType string, size int64, modTime time.Time, hashUpdate bool) (*api.UploadPreResponse, error) {
	if modTime.IsZero() {
		modTime = time.Now()
	}
	request := uploadPreRequest{
		FileName:       encodeFileName(leaf, f.opt.Enc),
		Size:           size,
		ParentID:       parentID,
		HashUpdate:     hashUpdate,
		DirName:        "",
		FormatType:     mimeType,
		CreatedAt:      modTime.UnixMilli(),
		UpdatedAt:      modTime.UnixMilli(),
		ParallelUpload: false,
	}
	var response api.UploadPreResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/upload/pre", nil, &request, &response); err != nil {
		return nil, err
	}
	if err := response.Response.Check(); err != nil {
		return nil, err
	}
	if response.Data.TaskID == "" {
		return nil, errors.New("quark drive pre-upload returned no task ID")
	}
	return &response, nil
}

func (f *Fs) updateUploadHash(ctx context.Context, taskID, md5sum, sha1sum string) (*api.UploadHashResponse, error) {
	request := updateHashRequest{TaskID: taskID, MD5: md5sum, SHA1: sha1sum}
	var response api.UploadHashResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/update/hash", nil, &request, &response); err != nil {
		return nil, err
	}
	if err := response.Response.Check(); err != nil {
		return nil, err
	}
	return &response, nil
}

func canonicalOSSResource(bucket, objectKey string, query url.Values) string {
	resource := "/"
	if bucket != "" {
		resource += bucket + "/"
	}
	resource += strings.TrimPrefix(objectKey, "/")
	if encoded := query.Encode(); encoded != "" {
		resource += "?" + encoded
	}
	return resource
}

func ossAuthMeta(method, contentType, date, resource string, headers map[string]string) string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, strings.ToLower(key))
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteString(method)
	out.WriteString("\n\n")
	out.WriteString(contentType)
	out.WriteByte('\n')
	out.WriteString(date)
	out.WriteByte('\n')
	for _, key := range keys {
		out.WriteString(key)
		out.WriteByte(':')
		out.WriteString(headers[key])
		out.WriteByte('\n')
	}
	out.WriteString(resource)
	return out.String()
}

func (f *Fs) uploadAuthorization(ctx context.Context, pre *api.UploadPreResponse, authMeta string) (string, error) {
	authInfo := pre.Data.AuthInfo
	if len(authInfo) == 0 {
		authInfo = json.RawMessage("null")
	}
	request := uploadAuthRequest{AuthInfo: authInfo, TaskID: pre.Data.TaskID, AuthMeta: authMeta}
	var response api.UploadAuthResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/upload/auth", nil, &request, &response); err != nil {
		return "", err
	}
	if err := response.Response.Check(); err != nil {
		return "", err
	}
	if response.Data.AuthKey == "" {
		return "", errors.New("quark drive upload authorization returned no key")
	}
	return response.Data.AuthKey, nil
}

func uploadURL(pre *api.UploadPreResponse, query url.Values) (*url.URL, error) {
	u, err := url.Parse(pre.Data.UploadURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if pre.Data.Bucket != "" {
		u.Host = pre.Data.Bucket + "." + u.Host
	}
	u.Path = "/" + strings.TrimPrefix(pre.Data.ObjKey, "/")
	u.RawQuery = query.Encode()
	return u, nil
}

func (f *Fs) uploadPart(ctx context.Context, pre *api.UploadPreResponse, mimeType string, partNumber int, file io.ReaderAt, offset, length int64) (string, error) {
	query := url.Values{"partNumber": []string{strconv.Itoa(partNumber)}, "uploadId": []string{pre.Data.UploadID}}
	u, err := uploadURL(pre, query)
	if err != nil {
		return "", err
	}
	var lastErr error
	for attempt := 1; attempt <= maxPartUploadAttempts; attempt++ {
		date := time.Now().UTC().Format(http.TimeFormat)
		xHeaders := map[string]string{"x-oss-date": date, "x-oss-user-agent": ossUserAgent}
		resource := canonicalOSSResource(pre.Data.Bucket, pre.Data.ObjKey, query)
		authMeta := ossAuthMeta(http.MethodPut, mimeType, date, resource, xHeaders)
		auth, authErr := f.uploadAuthorization(ctx, pre, authMeta)
		if authErr != nil {
			return "", authErr
		}
		req, requestErr := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), io.NewSectionReader(file, offset, length))
		if requestErr != nil {
			return "", requestErr
		}
		req.ContentLength = length
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", mimeType)
		req.Header.Set("Date", date)
		req.Header.Set("x-oss-date", date)
		req.Header.Set("x-oss-user-agent", ossUserAgent)
		response, requestErr := f.client.Do(req)
		if requestErr != nil {
			lastErr = requestErr
			if !fserrors.ShouldRetry(requestErr) {
				break
			}
			continue
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, int64(fs.Mebi)))
		closeErr := response.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			etag := response.Header.Get("ETag")
			if etag == "" {
				return "", fmt.Errorf("quark drive upload part %d returned no ETag", partNumber)
			}
			return etag, nil
		}
		lastErr = fmt.Errorf("quark drive upload part %d returned HTTP %s: %s", partNumber, response.Status, strings.TrimSpace(string(responseBody)))
		if !fserrors.ShouldRetryHTTP(response, retryErrorCodes) {
			break
		}
	}
	return "", fmt.Errorf("failed to upload Quark Drive part %d after %d attempts: %w", partNumber, maxPartUploadAttempts, lastErr)
}

func completeMultipartXML(parts []uploadedPart) string {
	sorted := append([]uploadedPart(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number < sorted[j].Number })
	escapeText := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	var payload strings.Builder
	payload.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	payload.WriteByte('\n')
	payload.WriteString("<CompleteMultipartUpload>")
	for _, part := range sorted {
		payload.WriteString("<Part><PartNumber>")
		payload.WriteString(strconv.Itoa(part.Number))
		payload.WriteString("</PartNumber><ETag>")
		payload.WriteString(escapeText.Replace(part.ETag))
		payload.WriteString("</ETag></Part>")
	}
	payload.WriteString("</CompleteMultipartUpload>")
	return payload.String()
}

func rawJSONValue(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	var value string
	if raw[0] == '"' && json.Unmarshal(raw, &value) == nil {
		return []byte(value)
	}
	return raw
}

func (f *Fs) commitUpload(ctx context.Context, pre *api.UploadPreResponse, parts []uploadedPart) error {
	body := completeMultipartXML(parts)
	query := url.Values{"uploadId": []string{pre.Data.UploadID}}
	u, err := uploadURL(pre, query)
	if err != nil {
		return err
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	callback := base64.StdEncoding.EncodeToString(rawJSONValue(pre.Data.Callback))
	xHeaders := map[string]string{
		"x-oss-callback":   callback,
		"x-oss-date":       date,
		"x-oss-user-agent": ossUserAgent,
	}
	resource := canonicalOSSResource(pre.Data.Bucket, pre.Data.ObjKey, query)
	authMeta := ossAuthMeta(http.MethodPost, "application/xml", date, resource, xHeaders)
	auth, err := f.uploadAuthorization(ctx, pre, authMeta)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Date", date)
	req.Header.Set("x-oss-callback", callback)
	req.Header.Set("x-oss-date", date)
	req.Header.Set("x-oss-user-agent", ossUserAgent)
	response, err := f.client.Do(req)
	if err != nil {
		return err
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, int64(fs.Mebi)))
	closeErr := response.Body.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("quark drive multipart commit returned HTTP %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func (f *Fs) finishUpload(ctx context.Context, pre *api.UploadPreResponse) (*api.Item, error) {
	request := map[string]string{"obj_key": pre.Data.ObjKey, "task_id": pre.Data.TaskID}
	var response api.UploadFinishResponse
	if err := f.callJSON(ctx, http.MethodPost, "/1/clouddrive/file/upload/finish", nil, &request, &response); err != nil {
		return nil, err
	}
	if err := response.Response.Check(); err != nil {
		return nil, err
	}
	if response.Data.FID == "" {
		return nil, errors.New("quark drive upload finish returned no file ID")
	}
	return &response.Data, nil
}

func resolvePartSize(size, serverPartSize int64, configured fs.SizeSuffix) int64 {
	partSize := serverPartSize
	if configured > 0 {
		partSize = int64(configured)
	}
	if partSize <= 0 {
		partSize = defaultUploadPartSize
	}
	minimumByCount := (size + maxUploadParts - 1) / maxUploadParts
	if minimumByCount > partSize {
		partSize = minimumByCount
	}
	return partSize
}

func resolvePartThreads(serverThreads, partCount int) int {
	if serverThreads <= 0 {
		serverThreads = 1
	}
	return min(serverThreads, partCount, maxUploadPartThreads)
}

func (f *Fs) upload(ctx context.Context, in io.Reader, src fs.ObjectInfo, leaf, parentID string) (*Object, error) {
	file, size, md5sum, sha1sum, cleanup, err := spoolInput(in, src.Size())
	if err != nil {
		return nil, err
	}
	defer cleanup()
	mimeType := fs.MimeType(ctx, src)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	modTime := src.ModTime(ctx)
	if modTime.IsZero() {
		modTime = time.Now()
	}
	modTime = time.UnixMilli(modTime.UnixMilli())
	pre, err := f.uploadPre(ctx, leaf, parentID, mimeType, size, modTime, true)
	if err != nil {
		return nil, err
	}
	hashResponse, err := f.updateUploadHash(ctx, pre.Data.TaskID, md5sum, sha1sum)
	if err != nil {
		return nil, err
	}
	if hashResponse.Data.Finish {
		id := hashResponse.Data.FID
		if id == "" {
			id = pre.Data.FID
		}
		if id == "" {
			return nil, errors.New("quark drive instant upload returned no file ID")
		}
		if _, err = f.waitForItem(ctx, parentID, id, leaf, true); err == nil {
			return &Object{fs: f, remote: src.Remote(), id: id, parentID: parentID, size: size, modTime: modTime, mimeType: mimeType}, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fs.Infof(src, "Instant upload result was not visible (%v); retrying as a multipart upload", err)
		pre, err = f.uploadPre(ctx, leaf, parentID, mimeType, size, modTime, false)
		if err != nil {
			return nil, err
		}
	}

	partSize := resolvePartSize(size, pre.Metadata.PartSize, f.chunkSize)
	partCount := int((size + partSize - 1) / partSize)
	if partCount == 0 {
		partCount = 1
	}
	parts := make([]uploadedPart, partCount)
	group, groupCtx := errgroup.WithContext(ctx)
	partThreads := resolvePartThreads(pre.Metadata.PartThread, partCount)
	fs.Debugf(src, "Using %d concurrent multipart upload threads for %d parts", partThreads, partCount)
	group.SetLimit(partThreads)
	for partNumber := 1; partNumber <= partCount; partNumber++ {
		partNumber := partNumber
		offset := int64(partNumber-1) * partSize
		length := min(partSize, max(int64(0), size-offset))
		group.Go(func() error {
			etag, uploadErr := f.uploadPart(groupCtx, pre, mimeType, partNumber, file, offset, length)
			if uploadErr != nil {
				return uploadErr
			}
			parts[partNumber-1] = uploadedPart{Number: partNumber, ETag: etag}
			return nil
		})
	}
	if err = group.Wait(); err != nil {
		return nil, err
	}
	if err = f.commitUpload(ctx, pre, parts); err != nil {
		return nil, err
	}
	info, err := f.finishUpload(ctx, pre)
	if err != nil {
		return nil, err
	}
	if info.Size == 0 && size != 0 {
		info.Size = size
	}
	if info.ParentFID == "" {
		info.ParentFID = parentID
	}
	if info.FileName == "" {
		info.FileName = leaf
	}
	if info.UpdatedAt == 0 {
		info.UpdatedAt = modTime.UnixMilli()
	}
	if info.FormatType == "" {
		info.FormatType = mimeType
	}
	if _, err = f.waitForItem(ctx, parentID, info.FID, leaf, true); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fs.Infof(src, "Multipart upload finished but was not yet visible (%v); accepting the finish response", err)
	}
	object, err := f.newObjectWithInfo(ctx, src.Remote(), info)
	if err != nil {
		return nil, err
	}
	return object, nil
}

// Put uploads a new object or safely replaces an existing one.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existing, err := f.NewObject(ctx, src.Remote())
	if err == nil {
		return existing, existing.Update(ctx, in, src, options...)
	}
	if !errors.Is(err, fs.ErrorObjectNotFound) {
		return nil, err
	}
	leaf, parentID, err := f.dirCache.FindPath(ctx, src.Remote(), true)
	if err != nil {
		return nil, err
	}
	return f.upload(ctx, in, src, leaf, parentID)
}

// PutStream uploads an object whose size may be unknown.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Update safely replaces the object contents.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	parentID := o.parentID
	if parentID == "" {
		_, resolvedParent, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
		if err != nil {
			return err
		}
		parentID = resolvedParent
	}
	targetLeaf := path.Base(o.remote)
	temporaryLeaf := targetLeaf + ".rclone-upload-" + random.String(8)
	newObject, err := o.fs.upload(ctx, in, src, temporaryLeaf, parentID)
	if err != nil {
		return err
	}
	oldName := targetLeaf + ".rclone-old-" + random.String(8)
	if _, err = o.fs.renameAndWait(ctx, o.id, parentID, oldName); err != nil {
		_ = o.fs.deleteAndWait(ctx, newObject.id, parentID)
		return err
	}
	if _, err = o.fs.renameAndWait(ctx, newObject.id, parentID, targetLeaf); err != nil {
		_, _ = o.fs.renameAndWait(ctx, o.id, parentID, targetLeaf)
		_ = o.fs.deleteAndWait(ctx, newObject.id, parentID)
		return err
	}
	if err = o.fs.deleteAndWait(ctx, o.id, parentID); err != nil {
		return err
	}
	newObject.remote = o.remote
	o.copyFrom(newObject)
	return nil
}
