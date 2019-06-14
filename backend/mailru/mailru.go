package mailru

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ncw/rclone/backend/mailru/api"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"

	"github.com/ncw/rclone/lib/oauthutil"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// Global constants
const (
	minSleepPacer   = 10 * time.Millisecond
	maxSleepPacer   = 2 * time.Second
	decayConstPacer = 2          // bigger for slower decay, exponential
	metaExpirySec   = 20 * 60    // meta server expiration time
	serverExpirySec = 3 * 60     // download server expiration time
	shardExpirySec  = 30 * 60    // upload server expiration time
	maxServerLocks  = 8          // maximum number of locks per single download server
	maxInt32        = 2147483647 // used as limit in directory list request
)

// Global errors
var (
	ErrorDirAlreadyExists              = errors.New("directory already exists")
	ErrorDirAlreadyExistsDifferentCase = errors.New("directory already exists (different case)")
	ErrorDirSourceNotExists            = errors.New("directory source does not exist")
)

// Description of how to authorize
var oauthConfig = &oauth2.Config{
	ClientID:     api.OAuthClientID,
	ClientSecret: "",
	Endpoint: oauth2.Endpoint{
		AuthURL:   api.OAuthURL,
		TokenURL:  api.OAuthURL,
		AuthStyle: oauth2.AuthStyleInParams,
	},
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "mailru",
		Description: "Mail.ru Cloud",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "username",
			Help: "User name",
		}, {
			Name:       "password",
			Help:       "Password",
			IsPassword: true,
		}, {
			Name:     "user_agent",
			Default:  api.DefaultUserAgent,
			Help:     "User agent used by client (internal)",
			Advanced: true,
			Hide:     fs.OptionHideBoth,
		}, {
			Name: "debug",
			Help: `Comma separated list of debugging options. This is for debugging purposes only.
List of supported options: nogzip, insecure, binlist, lsnames.`,
			Default:  "",
			Advanced: true,
			Hide:     fs.OptionHideBoth,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Username  string `config:"username"`
	Password  string `config:"password"`
	UserAgent string `config:"user_agent"`
	Debug     string `config:"debug"`
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

// shouldRetry returns a boolean as to whether this response and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(res *http.Response, err error) (bool, error) {
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(res, retryErrorCodes), err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(res *http.Response) (err error) {
	// TODO: also handle api.ServerErrorResponse
	response := new(api.FileErrorResponse)
	//body, err := rest.ReadBody(res)
	//fs.Debugf(nil, "Full error response: %s", string(body))
	err = rest.DecodeJSON(res, &response)
	if err != nil {
		fs.Debugf(nil, "Unknown error response: %v", err)
	}
	response.Message = response.Body.Home.Error
	if response.Message == "" {
		response.Message = res.Status
	}
	if response.Status == 0 {
		response.Status = res.StatusCode
	}
	return response
}

// Fs represents a remote mail.ru
type Fs struct {
	name        string
	root        string             // root path
	opt         Options            // parsed options
	features    *fs.Features       // optional features
	srv         *rest.Client       // REST API client
	cli         *http.Client       // underlying HTTP client (for authorize)
	m           configmap.Mapper   // config reader (for authorize)
	source      oauth2.TokenSource // OAuth token refresher
	pacer       *fs.Pacer          // pacer for API calls
	metaMu      sync.Mutex         // lock for meta server switcher
	metaURL     string             // URL of meta server
	metaExpiry  time.Time          // time to refresh meta server
	shardMu     sync.Mutex         // lock for upload shard switcher
	shardURL    string             // URL of upload shard
	shardExpiry time.Time          // time to refresh upload shard
	fileServers serverList         // file server switcher
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	fs.Debugf(nil, ">>> NewFs %q %q", name, root)

	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.Password != "" {
		opt.Password = obscure.MustReveal(opt.Password)
	}

	// Trailing slash signals us to optimize out one file check
	rootIsDir := strings.HasSuffix(root, "/")
	// However the f.root string should not have leading or trailing slashes
	root = strings.Trim(root, "/")

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		m:     m,
		pacer: fs.NewPacer(pacer.NewDefault(pacer.MinSleep(minSleepPacer), pacer.MaxSleep(maxSleepPacer), pacer.DecayConstant(decayConstPacer))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		// Can copy/move across mailru configs (almost, thus true here), but
		// only when they share common account (this is checked in Copy/Move).
		ServerSideAcrossConfigs: true,
	}).Fill(f)

	// Override few config settings and create a client
	clientConfig := *fs.Config
	clientConfig.UserAgent = opt.UserAgent
	if strings.Contains(opt.Debug, "nogzip") {
		clientConfig.NoGzip = true
	}
	f.cli = fshttp.NewClient(&clientConfig)
	f.srv = rest.NewClient(f.cli).SetRoot(api.APIServerURL)
	f.srv.SetErrorHandler(errorHandler)

	if strings.Contains(opt.Debug, "insecure") {
		transport := f.cli.Transport.(*fshttp.Transport).Transport
		transport.TLSClientConfig.InsecureSkipVerify = true
		transport.ProxyConnectHeader = http.Header{"User-Agent": {clientConfig.UserAgent}}
	}

	if err = f.authorize(false); err != nil {
		return nil, err
	}

	f.fileServers = serverList{
		fs:        f,
		path:      "/d",
		expirySec: serverExpirySec,
	}

	if !rootIsDir {
		_, dirSize, err := f.readItemMetaData(f.root)
		rootIsDir = (dirSize >= 0)
		// Ignore non-existing item and other errors
		if err == nil && !rootIsDir {
			root = path.Dir(f.root)
			if root == "." {
				root = ""
			}
			f.root = root
			// Return fs that points to the parent and signal rclone to do filtering
			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// Note: authorize() is not safe for concurrent access, as it updates the token source
func (f *Fs) authorize(force bool) (err error) {
	var t *oauth2.Token
	if !force {
		t, err = oauthutil.GetToken(f.name, f.m)
	}

	if err != nil || !tokenIsValid(t) {
		fs.Infof(f, "Valid token not found, authorizing.")
		ctx := oauthutil.Context(f.cli)
		t, err = oauthConfig.PasswordCredentialsToken(ctx, f.opt.Username, f.opt.Password)
	}
	if err == nil && !tokenIsValid(t) {
		err = errors.New("Invalid token")
	}
	if err != nil {
		return errors.Wrap(err, "Failed to authenticate")
	}

	if err = oauthutil.PutToken(f.name, f.m, t, false); err != nil {
		return err
	}

	// Mailru API server expects access token not in the request header but
	// in the URL query string, so we must use a bare token source rather than
	// client provided by oauthutil.
	//
	// WARNING: direct use of the returned token source triggers a bug in the
	// `(*token != *ts.token)` comparison in oauthutil.TokenSource.Token()
	// crashing with panic `comparing uncomparable type map[string]interface{}`
	// As a workaround, mimic oauth2.NewClient() wrapping token source in
	// oauth2.ReuseTokenSource
	_, ts, err := oauthutil.NewClientWithBaseClient(f.name, f.m, oauthConfig, f.cli)
	if err == nil {
		f.source = oauth2.ReuseTokenSource(nil, ts)
	}
	return err
}

func tokenIsValid(t *oauth2.Token) bool {
	return t.Valid() && t.RefreshToken != "" && t.Type() == "Bearer"
}

// accessToken() returns OAuth token and possibly refreshes it
func (f *Fs) accessToken() (string, error) {
	token, err := f.source.Token()
	if err != nil {
		return "", errors.Wrap(err, "cannot refresh access token")
	}
	return token.AccessToken, nil
}

// absPath converts root-relative remote to absolute home path
func (f *Fs) absPath(remote string) string {
	return "/" + path.Join(f.root, strings.Trim(remote, "/"))
}

// relPath converts absolute home path to root-relative remote
// Note that f.root can not have leading and trailing slashes
func (f *Fs) relPath(absPath string) (string, error) {
	target := strings.Trim(absPath, "/")
	if f.root == "" {
		return target, nil
	}
	if target == f.root {
		return "", nil
	}
	if strings.HasPrefix(target+"/", f.root+"/") {
		return target[len(f.root)+1:], nil
	}
	return "", fmt.Errorf("path %q should be under %q", absPath, f.root)
}

// metaServer ...
func (f *Fs) metaServer() (string, error) {
	f.metaMu.Lock()
	defer f.metaMu.Unlock()

	if f.metaURL != "" && time.Now().Before(f.metaExpiry) {
		return f.metaURL, nil
	}

	opts := rest.Opts{
		RootURL: api.DispatchServerURL,
		Method:  "GET",
		Path:    "/m",
	}

	var (
		res *http.Response
		url string
		err error
	)
	err = f.pacer.Call(func() (bool, error) {
		res, err = f.srv.Call(&opts)
		if err == nil {
			url, err = readBodyWord(res)
		}
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		closeBody(res)
		return "", err
	}
	f.metaURL = url
	f.metaExpiry = time.Now().Add(metaExpirySec * time.Second)
	fs.Debugf(f, "new meta server: %s", f.metaURL)
	return f.metaURL, nil
}

// readBodyWord reads the single line response to completion
// and extracts the first word from the first line.
func readBodyWord(res *http.Response) (word string, err error) {
	var body []byte
	body, err = rest.ReadBody(res)
	if err == nil {
		line := strings.Trim(string(body), " \r\n")
		word = strings.Split(line, " ")[0]
	}
	if word == "" {
		return "", errors.New("Empty reply from dispatcher")
	}
	return word, nil
}

// readItemMetaData returns a file/directory info at given full path
// If it can't be found it fails with fs.ErrorObjectNotFound
// For the return value `dirSize` please see Fs.itemToEntry()
func (f *Fs) readItemMetaData(path string) (entry fs.DirEntry, dirSize int, err error) {
	token, err := f.accessToken()
	if err != nil {
		return nil, -1, err
	}

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/m1/file",
		Parameters: url.Values{
			"access_token": {token},
			"home":         {path},
			"offset":       {"0"},
			"limit":        {strconv.Itoa(maxInt32)},
		},
	}

	var info api.ItemInfoResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(res, err)
	})

	if err != nil {
		apiErr, ok := err.(*api.FileErrorResponse)
		if ok && apiErr.Status == 404 {
			err = fs.ErrorObjectNotFound
		}
		return
	}

	entry, dirSize, err = f.itemToDirEntry(&info.Body)
	return
}

// itemToEntry converts API item to rclone directory entry
// The dirSize return value is:
//   <0 - for a file or in case of error
//   =0 - for an empty directory
//   >0 - for a non-empty directory
func (f *Fs) itemToDirEntry(item *api.ListItem) (entry fs.DirEntry, dirSize int, err error) {
	remote, err := f.relPath(item.Home)
	if err != nil {
		return nil, -1, err
	}
	switch item.Kind {
	case "folder":
		dir := fs.NewDir(remote, time.Unix(item.Mtime, 0)).SetSize(item.Size)
		dirSize := item.Count.Files + item.Count.Folders
		return dir, dirSize, nil
	case "file":
		file := &Object{
			fs:          f,
			remote:      remote,
			hasMetaData: true,
			size:        item.Size,
			mrHash:      item.Hash,
			modTime:     time.Unix(item.Mtime, 0),
		}
		return file, -1, nil
	default:
		return nil, -1, fmt.Errorf("Unknown resource type %q", item.Kind)
	}
}

// List the objects and directories in dir into entries.
// The entries can be returned in any order but should be for a complete directory.
// dir should be "" to list the root, and should not have trailing slashes.
// This should return ErrDirNotFound if the directory isn't found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	fs.Debugf(f, ">>> List: %q\n", dir)

	if strings.Contains(f.opt.Debug, "binlist") {
		entries, err = f.listBin(f.absPath(dir), 1)
	} else {
		entries, err = f.listM1(f.absPath(dir), 0, maxInt32)
	}

	if err == nil && strings.Contains(f.opt.Debug, "lsnames") {
		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Remote())
		}
		sort.Strings(names)
		fs.Debugf(f, "List(%q): %v", dir, names)
	}

	return
}

// list using protocol "m1"
func (f *Fs) listM1(dirPath string, offset int, limit int) (entries fs.DirEntries, err error) {
	token, err := f.accessToken()
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("access_token", token)
	params.Set("offset", strconv.Itoa(offset))
	params.Set("limit", strconv.Itoa(limit))

	data := url.Values{}
	data.Set("home", dirPath)

	opts := rest.Opts{
		Method:      "POST",
		Path:        "/api/m1/folder",
		Parameters:  params,
		Body:        strings.NewReader(data.Encode()),
		ContentType: api.BinContentType,
	}

	var (
		info api.FolderInfoResponse
		res  *http.Response
	)
	err = f.pacer.Call(func() (bool, error) {
		res, err = f.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(res, err)
	})

	if err != nil {
		apiErr, ok := err.(*api.FileErrorResponse)
		if ok && apiErr.Status == 404 {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}

	if info.Body.Kind != "folder" {
		return nil, fs.ErrorIsFile
	}

	for _, item := range info.Body.List {
		entry, _, err := f.itemToDirEntry(&item)
		if err == nil {
			entries = append(entries, entry)
		} else {
			fs.Debugf(f, "Excluding path %q from list: %v", item.Home, err)
		}
	}
	return entries, nil
}

// list using protocol "bin"
func (f *Fs) listBin(dirPath string, depth int) (entries fs.DirEntries, err error) {
	options := api.ListOptDefaults

	req := api.NewBinWriter()
	req.WritePu16(api.OperationFolderList)
	req.WriteString(dirPath)
	req.WritePu32(int64(depth))
	req.WritePu32(int64(options))
	req.WritePu32(0)

	token, err := f.accessToken()
	if err != nil {
		return nil, err
	}
	metaURL, err := f.metaServer()
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method:  "POST",
		RootURL: metaURL,
		Parameters: url.Values{
			"client_id": {api.OAuthClientID},
			"token":     {token},
		},
		ContentType: api.BinContentType,
		Body:        req.Reader(),
	}

	var res *http.Response
	err = f.pacer.Call(func() (bool, error) {
		res, err = f.srv.Call(&opts)
		return shouldRetry(res, err)
	})
	if err != nil {
		closeBody(res)
		return nil, err
	}

	r := api.NewBinReader(res.Body)
	defer closeBody(res)

	// read status
	switch status := r.ReadByteAsInt(); status {
	case api.ListResultOK:
		// go on...
	case api.ListResultNotExists:
		return nil, fs.ErrorDirNotFound
	default:
		return nil, fmt.Errorf("directory list error %d", status)
	}

	t := &treeState{
		f:       f,
		r:       r,
		options: options,
		rootDir: parentDir(dirPath),
		lastDir: "",
		level:   0,
	}
	t.currDir = t.rootDir

	// read revision
	if err := t.revision.Read(r); err != nil {
		return nil, err
	}

	// read space
	if (options & api.ListOptTotalSpace) != 0 {
		t.totalSpace = int64(r.ReadULong())
	}
	if (options & api.ListOptUsedSpace) != 0 {
		t.usedSpace = int64(r.ReadULong())
	}

	t.fingerprint = r.ReadBytesByLength()

	// deserialize
	for {
		entry, err := t.NextRecord()
		if err != nil {
			break
		}
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	if err != nil && err != fs.ErrorListAborted {
		fs.Debugf(f, "listBin failed at offset %d: %v", r.Count(), err)
		return nil, err
	}
	return entries, nil
}

func (t *treeState) NextRecord() (fs.DirEntry, error) {
	r := t.r
	parseOp := r.ReadByteAsShort()
	if r.Error() != nil {
		return nil, r.Error()
	}

	switch parseOp {
	case api.ListParseDone:
		return nil, fs.ErrorListAborted
	case api.ListParsePin:
		if t.lastDir == "" {
			return nil, errors.New("last folder is null")
		}
		t.currDir = t.lastDir
		t.level++
		return nil, nil
	case api.ListParsePinUpper:
		if t.currDir == t.rootDir {
			return nil, nil
		}
		if t.level <= 0 {
			return nil, errors.New("no parent folder")
		}
		t.currDir = parentDir(t.currDir)
		t.level--
		return nil, nil
	case api.ListParseUnknown15:
		skip := int(r.ReadPu32())
		for i := 0; i < skip; i++ {
			r.ReadPu32()
			r.ReadPu32()
		}
		return nil, nil
	case api.ListParseReadItem:
		// get item (see below)
	default:
		return nil, fmt.Errorf("unknown parse operation %d", parseOp)
	}

	// get item
	head := r.ReadIntSpl()
	itemType := head & 3
	if (head & 4096) != 0 {
		t.dunnoNodeID = r.ReadNBytes(api.DunnoNodeIDLength)
	}
	name := string(r.ReadBytesByLength())
	t.dunno1 = int(r.ReadULong())
	t.dunno2 = 0
	t.dunno3 = 0

	if r.Error() != nil {
		return nil, r.Error()
	}

	var (
		modTime time.Time
		size    int64
		binHash []byte
		dirSize int64
		isDir   = true
	)

	switch itemType {
	case api.ListItemMountPoint:
		t.treeID = r.ReadNBytes(api.TreeIDLength)
		t.dunno2 = int(r.ReadULong())
		t.dunno3 = int(r.ReadULong())
	case api.ListItemFolder:
		t.dunno2 = int(r.ReadULong())
	case api.ListItemSharedFolder:
		t.dunno2 = int(r.ReadULong())
		t.treeID = r.ReadNBytes(api.TreeIDLength)
	case api.ListItemFile:
		isDir = false
		modTime = r.ReadDate()
		size = int64(r.ReadULong())
		binHash = r.ReadNBytes(api.HashLength)
	default:
		return nil, fmt.Errorf("unknown item type %d", itemType)
	}

	if isDir {
		t.lastDir = path.Join(t.currDir, name)
		if (t.options & api.ListOptDelete) != 0 {
			t.dunnoDel1 = int(r.ReadPu32())
			t.dunnoDel2 = int(r.ReadPu32())
		}
		if (t.options & api.ListOptFolderSize) != 0 {
			dirSize = int64(r.ReadULong())
		}
	}

	if r.Error() != nil {
		return nil, r.Error()
	}

	if fs.Config.LogLevel >= fs.LogLevelDebug {
		ctime, _ := modTime.MarshalJSON()
		fs.Debugf(t.f, "binDir %d.%d %q %q (%d) %s", t.level, itemType, t.currDir, name, size, ctime)
	}

	if t.level != 1 {
		// TODO: implement recursion and ListR
		// Note: recursion is broken because maximum buffer size is 8K
		return nil, nil
	}

	remote, err := t.f.relPath(path.Join(t.currDir, name))
	if err != nil {
		return nil, err
	}
	if isDir {
		return fs.NewDir(remote, modTime).SetSize(dirSize), nil
	}
	obj := &Object{
		fs:          t.f,
		remote:      remote,
		hasMetaData: true,
		size:        size,
		mrHash:      hex.EncodeToString(binHash),
		modTime:     modTime,
	}
	return obj, nil
}

type treeState struct {
	f           *Fs
	r           *api.BinReader
	options     int
	rootDir     string
	currDir     string
	lastDir     string
	level       int
	revision    treeRevision
	totalSpace  int64
	usedSpace   int64
	fingerprint []byte
	dunno1      int
	dunno2      int
	dunno3      int
	dunnoDel1   int
	dunnoDel2   int
	dunnoNodeID []byte
	treeID      []byte
}

type treeRevision struct {
	ver       int16
	treeID    []byte
	treeIDNew []byte
	bgn       uint64
	bgnNew    uint64
}

func (rev *treeRevision) Read(data *api.BinReader) error {
	rev.ver = data.ReadByteAsShort()
	switch rev.ver {
	case 0:
		// Revision()
	case 1, 2:
		rev.treeID = data.ReadNBytes(api.TreeIDLength)
		rev.bgn = data.ReadULong()
	case 3, 4:
		rev.treeID = data.ReadNBytes(api.TreeIDLength)
		rev.bgn = data.ReadULong()
		rev.treeIDNew = data.ReadNBytes(api.TreeIDLength)
		rev.bgnNew = data.ReadULong()
	case 5:
		rev.treeID = data.ReadNBytes(api.TreeIDLength)
		rev.bgn = data.ReadULong()
		rev.treeIDNew = data.ReadNBytes(api.TreeIDLength)
	default:
		return fmt.Errorf("unknown directory revision %d", rev.ver)
	}
	return data.Error()
}

// CreateDir makes a directory (parent must exist)
func (f *Fs) CreateDir(path string) error {
	fs.Debugf(f, ">>> CreateDir %q\n", path)

	req := api.NewBinWriter()
	req.WritePu16(api.OperationCreateFolder)
	req.WritePu16(0) // revision
	req.WriteString(path)
	req.WritePu32(0)

	token, err := f.accessToken()
	if err != nil {
		return err
	}
	metaURL, err := f.metaServer()
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:  "POST",
		RootURL: metaURL,
		Parameters: url.Values{
			"client_id": {api.OAuthClientID},
			"token":     {token},
		},
		ContentType: api.BinContentType,
		Body:        req.Reader(),
	}

	var res *http.Response
	err = f.pacer.Call(func() (bool, error) {
		res, err = f.srv.Call(&opts)
		return shouldRetry(res, err)
	})
	if err != nil {
		closeBody(res)
		return err
	}

	reply := api.NewBinReader(res.Body)
	defer closeBody(res)

	switch status := reply.ReadByteAsInt(); status {
	case api.MkdirResultOK:
		return nil
	case api.MkdirResultAlreadyExists:
		return ErrorDirAlreadyExists
	case api.MkdirResultAlreadyExistsDifferentCase:
		return ErrorDirAlreadyExistsDifferentCase
	case api.MkdirResultSourceNotExists:
		return ErrorDirSourceNotExists
	default:
		return fmt.Errorf("create directory error %d", status)
	}
}

// Mkdir creates the container (and its parents) if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	fs.Debugf(f, ">>> Mkdir %q\n", dir)
	return f.mkDirs(f.absPath(dir))
}

func (f *Fs) mkDirs(path string) error {
	err := f.CreateDir(path)
	switch err {
	case nil, ErrorDirAlreadyExists, ErrorDirAlreadyExistsDifferentCase:
		return nil
	case ErrorDirSourceNotExists:
		fs.Debugf(f, "mkDirs by part %q", path)
	default:
		return err
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	path = ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		path += "/" + part
		err = f.CreateDir(path)
		switch err {
		case nil, ErrorDirAlreadyExists, ErrorDirAlreadyExistsDifferentCase:
			continue
		default:
			return err
		}
	}
	return nil
}

func parentDir(absPath string) string {
	parent := path.Dir(strings.TrimRight(absPath, "/"))
	if parent == "." {
		parent = ""
	}
	return parent
}

func (f *Fs) mkParentDirs(path string) error {
	return f.mkDirs(parentDir(path))
}

// Rmdir deletes a directory.
// Returns an error if it isn't empty.
func (f *Fs) Rmdir(dir string) error {
	fs.Debugf(f, ">>> Rmdir %q\n", dir)
	return f.purgeWithCheck(dir, true)
}

// Purge deletes all the files and the root directory
// Optional interface: Only implement this if you have a way of deleting
// all the files quicker than just running Remove() on the result of List()
func (f *Fs) Purge() error {
	fs.Debugf(f, ">>> Purge\n")
	return f.purgeWithCheck("", false)
}

// purgeWithCheck() removes the root directory.
// Refuses if `check` is set and directory has anything in.
func (f *Fs) purgeWithCheck(dir string, check bool) error {
	path := f.absPath(dir)
	if check {
		_, dirSize, err := f.readItemMetaData(path)
		if err != nil {
			return errors.Wrap(err, "rmdir failed")
		}
		if dirSize > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}
	return f.delete(path, false)
}

func (f *Fs) delete(path string, hardDelete bool) error {
	token, err := f.accessToken()
	if err != nil {
		return err
	}

	// use POST to send path due to possible unprintable Unicode characters
	data := url.Values{"home": {path}}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/m1/file/remove",
		Parameters: url.Values{
			"access_token": {token},
		},
		Body:        strings.NewReader(data.Encode()),
		ContentType: api.BinContentType,
	}

	var response api.GenericOperationResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.srv.CallJSON(&opts, nil, &response)
		return shouldRetry(res, err)
	})

	switch {
	case err != nil:
		return err
	case response.Status == 200:
		return nil
	default:
		return fmt.Errorf("delete failed with code %d", response.Status)
	}
}

// Copy src to this remote using server side copy operations.
// This is stored with the remote path given.
// It returns the destination Object and a possible error.
// Will only be called if src.Fs().Name() == f.Name()
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, ">>> Copy %v --> %q\n", src, remote)

	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	if srcObj.fs.opt.Username != f.opt.Username {
		// Can copy across mailru configs only if they share common account
		fs.Debugf(src, "Can't copy - not same account")
		return nil, fs.ErrorCantCopy
	}

	srcPath := srcObj.absPath()
	dstPath := f.absPath(remote)
	overwrite := false
	fs.Debugf(f, "copy %q -> %q\n", srcPath, dstPath)

	err := f.mkParentDirs(dstPath)
	if err != nil {
		return nil, err
	}

	data := url.Values{}
	data.Set("home", srcPath)
	data.Set("folder", parentDir(dstPath))
	data.Set("email", f.opt.Username)
	data.Set("x-email", f.opt.Username)

	if overwrite {
		data.Set("conflict", "rewrite")
	} else {
		data.Set("conflict", "rename")
	}

	token, err := f.accessToken()
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/m1/file/copy",
		Parameters: url.Values{
			"access_token": {token},
		},
		Body:        strings.NewReader(data.Encode()),
		ContentType: api.BinContentType,
	}

	var response api.CopyResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.srv.CallJSON(&opts, nil, &response)
		return shouldRetry(res, err)
	})

	if err != nil {
		return nil, errors.Wrap(err, "couldn't copy file")
	}
	if response.Status != 200 {
		return nil, fmt.Errorf("copy failed with code %d", response.Status)
	}

	tmpPath := response.Body
	if tmpPath != dstPath {
		fs.Debugf(f, "rename temporary file %q -> %q\n", tmpPath, dstPath)
		err = f.moveItem(tmpPath, dstPath, "rename temporary file")
		if err != nil {
			_ = f.delete(tmpPath, false) // ignore error
			return nil, err
		}
	}

	return f.NewObject(remote)
}

// Move src to this remote using server side move operations.
// This is stored with the remote path given.
// It returns the destination Object and a possible error.
// Will only be called if src.Fs().Name() == f.Name()
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, ">>> Move %v --> %q\n", src, remote)

	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	if srcObj.fs.opt.Username != f.opt.Username {
		// Can move across mailru configs only if they share common account
		fs.Debugf(src, "Can't move - not same account")
		return nil, fs.ErrorCantMove
	}

	srcPath := srcObj.absPath()
	dstPath := f.absPath(remote)

	err := f.mkParentDirs(dstPath)
	if err != nil {
		return nil, err
	}

	err = f.moveItem(srcPath, dstPath, "move file")
	if err != nil {
		return nil, err
	}

	return f.NewObject(remote)
}

func (f *Fs) moveItem(srcPath, dstPath, opName string) error {
	token, err := f.accessToken()
	if err != nil {
		return err
	}
	metaURL, err := f.metaServer()
	if err != nil {
		return err
	}

	req := api.NewBinWriter()
	req.WritePu16(api.OperationRename)
	req.WritePu32(0) // old revision
	req.WriteString(srcPath)
	req.WritePu32(0) // new revision
	req.WriteString(dstPath)
	req.WritePu32(0) // dunno

	opts := rest.Opts{
		Method:  "POST",
		RootURL: metaURL,
		Parameters: url.Values{
			"client_id": {api.OAuthClientID},
			"token":     {token},
		},
		ContentType: api.BinContentType,
		Body:        req.Reader(),
	}

	var res *http.Response
	err = f.pacer.Call(func() (bool, error) {
		res, err = f.srv.Call(&opts)
		return shouldRetry(res, err)
	})
	if err != nil {
		closeBody(res)
		return err
	}

	reply := api.NewBinReader(res.Body)
	defer closeBody(res)

	switch status := reply.ReadByteAsInt(); status {
	case api.MoveResultOK:
		return nil
	default:
		return fmt.Errorf("%s failed with error %d", opName, status)
	}
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
// Will only be called if src.Fs().Name() == f.Name()
// If it isn't possible then return fs.ErrorCantDirMove
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	fs.Debugf(f, ">>> DirMove %q --> %q\n", srcRemote, dstRemote)

	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	if srcFs.opt.Username != f.opt.Username {
		// Can move across mailru configs only if they share common account
		fs.Debugf(src, "Can't move - not same account")
		return fs.ErrorCantDirMove
	}
	srcPath := srcFs.absPath(srcRemote)
	dstPath := f.absPath(dstRemote)
	fs.Debugf(srcFs, "DirMove [%s]%q --> [%s]%q\n", srcRemote, srcPath, dstRemote, dstPath)

	// Refuse to move to or from the root
	if len(srcPath) <= len(srcFs.root) || len(dstPath) <= len(f.root) {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	err := f.mkParentDirs(dstPath)
	if err != nil {
		return err
	}

	_, _, err = f.readItemMetaData(dstPath)
	switch err {
	case fs.ErrorObjectNotFound:
		// OK!
	case nil:
		return fs.ErrorDirExists
	default:
		return err
	}

	return f.moveItem(srcPath, dstPath, "directory move")
}

// About gets quota information
func (f *Fs) About() (*fs.Usage, error) {
	fs.Debugf(f, ">>> About\n")

	token, err := f.accessToken()
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/m1/user",
		Parameters: url.Values{
			"access_token": {token},
		},
	}

	var info api.UserInfoResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(res, err)
	})
	if err != nil {
		return nil, err
	}

	total := info.Body.Cloud.Space.BytesTotal
	used := int64(info.Body.Cloud.Space.BytesUsed)

	usage := &fs.Usage{
		Total: fs.NewUsageValue(total),
		Used:  fs.NewUsageValue(used),
		Free:  fs.NewUsageValue(total - used),
	}
	return usage, nil
}

// Put the object
// Copy the reader in to the new object which is returned
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:      f,
		remote:  src.Remote(),
		size:    src.Size(),
		modTime: src.ModTime(),
	}
	fs.Debugf(f, ">>> Put: %q %d '%v'\n", o.remote, o.size, o.modTime)
	return o, o.Update(in, src, options...)
}

// Update an existing object
// Copy the reader into the object updating modTime and size
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errors.New("mail.ru does not support streaming uploads")
	}

	err := o.fs.mkParentDirs(o.absPath())
	if err != nil {
		return err
	}

	if size <= api.HashLength {
		// do not send upload request if file content fits to hash
		var b []byte
		b, err = ioutil.ReadAll(in)
		o.mrHash = hex.EncodeToString(b) + strings.Repeat("00", api.HashLength-len(b))
	} else {
		err = o.upload(in, size, options...)
		// o.mrHash has been calculated by upload server
	}
	if err != nil {
		return err
	}

	o.size = size
	o.modTime = src.ModTime()
	return o.addFileMetaData(true)
}

func (o *Object) upload(in io.Reader, size int64, options ...fs.OpenOption) error {
	token, err := o.fs.accessToken()
	if err != nil {
		return err
	}
	shardURL, err := o.fs.uploadShard()
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:        "PUT",
		RootURL:       shardURL,
		Body:          in,
		Options:       options,
		ContentLength: &size,
		Parameters: url.Values{
			"client_id": {api.OAuthClientID},
			"token":     {token},
		},
		ExtraHeaders: map[string]string{
			"Accept": "*/*",
		},
	}

	var res *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.srv.Call(&opts)
		if err == nil {
			o.mrHash, err = readBodyWord(res)
		}
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		closeBody(res)
		return err
	}

	switch res.StatusCode {
	case 200, 201:
		fs.Debugf(o, "upload hash %q", o.mrHash)
		return nil
	default:
		return fmt.Errorf("upload failed with code %s (%d)", res.Status, res.StatusCode)
	}
}

func (f *Fs) uploadShard() (string, error) {
	f.shardMu.Lock()
	defer f.shardMu.Unlock()

	if f.shardURL != "" && time.Now().Before(f.shardExpiry) {
		return f.shardURL, nil
	}

	token, err := f.accessToken()
	if err != nil {
		return "", err
	}

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/m1/dispatcher",
		Parameters: url.Values{
			"client_id":    {api.OAuthClientID},
			"access_token": {token},
		},
	}

	var info api.ShardInfoResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(res, err)
	})
	if err != nil {
		return "", err
	}

	f.shardURL = info.Body.Upload[0].URL
	f.shardExpiry = time.Now().Add(shardExpirySec * time.Second)
	fs.Debugf(f, "new upload shard: %s", f.shardURL)

	return f.shardURL, nil
}

// Object describes a mailru object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // Bytes in the object
	modTime     time.Time // Modified time of the object
	mrHash      string    // Mail.ru flavored SHA1 hash of the object
}

// NewObject finds an Object at the remote.
// If object can't be found it fails with fs.ErrorObjectNotFound
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	fs.Debugf(f, "NewObject %q\n", remote)
	o := &Object{
		fs:     f,
		remote: remote,
	}
	err := o.readMetaData(true)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// absPath converts root-relative remote to absolute home path
func (o *Object) absPath() string {
	return o.fs.absPath(o.remote)
}

// Object.readMetaData reads and fills a file info
// If object can't be found it fails with fs.ErrorObjectNotFound
func (o *Object) readMetaData(force bool) error {
	if o.hasMetaData && !force {
		return nil
	}
	entry, dirSize, err := o.fs.readItemMetaData(o.absPath())
	if err != nil {
		return err
	}
	newObj, ok := entry.(*Object)
	if !ok || dirSize >= 0 {
		return fs.ErrorNotAFile
	}
	if newObj.remote != o.remote {
		return fmt.Errorf("File %q path has changed to %q", o.remote, newObj.remote)
	}
	o.hasMetaData = true
	o.size = newObj.size
	o.modTime = newObj.modTime
	o.mrHash = newObj.mrHash
	return nil
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return fmt.Sprintf("[%s]%q", o.fs.root, o.remote)
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the object
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData(false)
	if err != nil {
		fs.Errorf(o, "%v", err)
	}
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(false)
	if err != nil {
		fs.Errorf(o, "%v", err)
	}
	return o.size
}

// Hash returns the MD5 or SHA1 sum of an object
// returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time of the local fs object
//
// Commits the datastore
func (o *Object) SetModTime(modTime time.Time) error {
	fs.Debugf(o, ">>> SetModTime [%v]", modTime)
	o.modTime = modTime
	return o.addFileMetaData(true)
}

func (o *Object) addFileMetaData(overwrite bool) error {
	binHash, err := hex.DecodeString(o.mrHash)
	if err != nil {
		return err
	}

	token, err := o.fs.accessToken()
	if err != nil {
		return err
	}
	metaURL, err := o.fs.metaServer()
	if err != nil {
		return err
	}

	req := api.NewBinWriter()
	req.WritePu16(api.OperationAddFile)
	req.WritePu16(0) // revision
	req.WriteString(o.absPath())
	req.WritePu64(o.size)
	req.WritePu64(o.modTime.Unix())
	req.WritePu32(0)
	req.Write(binHash)

	if overwrite {
		// overwrite
		req.WritePu32(1)
	} else {
		// don't add if not changed, add with rename if changed
		req.WritePu32(55)
		req.Write(binHash)
		req.WritePu64(o.size)
	}

	opts := rest.Opts{
		Method:  "POST",
		RootURL: metaURL,
		Parameters: url.Values{
			"client_id": {api.OAuthClientID},
			"token":     {token},
		},
		ContentType: api.BinContentType,
		Body:        req.Reader(),
	}

	var res *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.srv.Call(&opts)
		return shouldRetry(res, err)
	})
	if err != nil {
		closeBody(res)
		return err
	}

	reply := api.NewBinReader(res.Body)
	defer closeBody(res)

	switch status := reply.ReadByteAsInt(); status {
	case api.AddResultOK, api.AddResultNotModified, api.AddResultDunno04, api.AddResultDunno09:
		return nil
	default:
		return fmt.Errorf("add file error %d", status)
	}
}

// Remove an object
func (o *Object) Remove() error {
	fs.Debugf(o, ">>> Remove")
	return o.fs.delete(o.absPath(), false)
}

// Open an object for read and download its content
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.Debugf(o, ">>> Open %q", o.remote)

	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch opt := option.(type) {
		case *fs.SeekOption:
			offset = opt.Offset
		case *fs.RangeOption:
			offset, limit = opt.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if limit < 0 {
		limit = o.size - offset
	}
	end := offset + limit
	if end > o.size {
		end = o.size
	}

	token, err := o.fs.accessToken()
	if err != nil {
		return nil, err
	}

	// TODO: set custom timeouts
	opts := rest.Opts{
		Method:  "GET",
		Options: options,
		Path:    url.PathEscape(strings.TrimLeft(o.absPath(), "/")),
		Parameters: url.Values{
			"client_id": {api.OAuthClientID},
			"token":     {token},
		},
		ExtraHeaders: map[string]string{
			"Accept": "*/*",
			"Range":  fmt.Sprintf("bytes=%d-%d", offset, end-1),
		},
	}

	var res *http.Response
	server := ""
	err = o.fs.pacer.Call(func() (bool, error) {
		server, err = o.fs.fileServers.take(server)
		if err != nil {
			return false, err
		}
		opts.RootURL = server
		res, err = o.fs.srv.Call(&opts)
		return shouldRetry(res, err)
	})
	if err != nil {
		if res != nil && res.Body != nil {
			closeBody(res)
		}
		return nil, err
	}

	bodyWrapper := &serverUnlocker{
		body:   res.Body,
		o:      o,
		server: server,
	}
	return bodyWrapper, nil
}

type serverUnlocker struct {
	body     io.ReadCloser
	o        *Object
	server   string
	unlocked bool
}

func (su *serverUnlocker) Read(p []byte) (n int, err error) {
	n, err = su.body.Read(p)
	if err != nil { // io.Error or EOF
		su.unlockServer()
	}
	return
}

func (su *serverUnlocker) Close() error {
	su.unlockServer()
	return su.body.Close()
}

func (su *serverUnlocker) unlockServer() {
	if !su.unlocked {
		su.unlocked = true
		su.o.fs.fileServers.free(su.server)
	}
}

type pendingServer struct {
	url    string
	locks  int
	expiry time.Time
}

type serverList struct {
	list      []pendingServer
	mu        sync.Mutex
	path      string
	expirySec time.Duration
	fs        *Fs
}

func (sl *serverList) take(current string) (string, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	var server *pendingServer
	i := 0
	if current != "" {
		// avoid current (faulty) server
		for i < len(sl.list) {
			srv := &sl.list[i]
			if srv.url == current {
				i++
				break
			}
			i++
		}
	}
	for i < len(sl.list) {
		srv := &sl.list[i]
		if srv.locks < maxServerLocks && srv.url != "" {
			server = srv
			break
		}
		i++
	}

	f := sl.fs
	now := time.Now()
	if server != nil && (server.locks > 0 || now.Before(server.expiry)) {
		server.locks++
		fs.Debugf(f, "Take server #%d %q with %d locks", i, server.url, server.locks)
		return server.url, nil
	}

	opts := rest.Opts{
		Method:  "GET",
		RootURL: api.DispatchServerURL,
		Path:    sl.path,
	}
	var (
		res *http.Response
		url string
		err error
	)
	err = f.pacer.Call(func() (bool, error) {
		res, err = f.srv.Call(&opts)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		url, err = readBodyWord(res)
		return fserrors.ShouldRetry(err), err
	})
	if err != nil {
		closeBody(res)
		return "", errors.Wrap(err, "Failed to request file server")
	}

	if server == nil {
		// is this server over-locked?
		for i = 0; i < len(sl.list); i++ {
			srv := &sl.list[i]
			if srv.url == url {
				// Dispatcher approves over-locked file server, reuse.
				srv.locks++
				srv.expiry = now.Add(sl.expirySec * time.Second)
				if fs.Config.LogLevel >= fs.LogLevelInfo {
					ctime, _ := srv.expiry.MarshalJSON()
					fs.Infof(sl.fs, "Reuse server #%d %q with %d locks (new expiry %s)", i, url, srv.locks, ctime)
				}
				return url, nil
			}
		}
	}

	if server == nil {
		// look for a disposed server slot
		for i = 0; i < len(sl.list); i++ {
			srv := &sl.list[i]
			if srv.url == "" {
				server = srv
				break
			}
		}
	}

	if server == nil {
		// no free slots - add new one
		sl.list = append(sl.list, pendingServer{})
		i = len(sl.list) - 1
		server = &sl.list[i]
	}
	server.url = url
	server.locks = 1
	server.expiry = now.Add(sl.expirySec * time.Second)
	if fs.Config.LogLevel >= fs.LogLevelDebug {
		ctime, _ := server.expiry.MarshalJSON()
		fs.Debugf(f, "Allocate server #%d %q (expires %s)", i, server.url, ctime)
	}
	return url, nil
}

func (sl *serverList) free(url string) {
	if url == "" {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	for i := 0; i < len(sl.list); i++ {
		server := &sl.list[i]
		if server.url == url {
			if server.locks > 0 {
				fs.Debugf(sl.fs, "Release server #%d %q with %d locks", i, server.url, server.locks)
				server.locks--
			} else {
				fs.Infof(sl.fs, "Dispose of faulty server #%d %q", i, server.url)
				server.url = ""
			}
		}
	}
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("[%s]", f.root)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// close response body ignoring errors
func closeBody(res *http.Response) {
	_ = res.Body.Close()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Purger   = (*Fs)(nil)
	_ fs.Copier   = (*Fs)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.DirMover = (*Fs)(nil)
	_ fs.Abouter  = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
)
