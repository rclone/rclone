// Package ente provides an interface to Ente.io E2EE cloud storage.
package ente

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	pathModule "path"
	"strconv"
	"time"

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
	minSleep        = 10 * time.Millisecond
	maxSleep        = 20 * time.Second
	decayConstant   = 1
	defaultEndpoint = "https://api.ente.com"
	headerClientPkg = "X-Client-Package"
	headerAuthToken = "X-Auth-Token"
	appPhotos        = "com.ente.photos"
)

// Register with rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "ente",
		Description: "Ente.io E2EE Cloud Storage",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "email",
				Help:     "Email of your Ente account",
				Required: false,
			},
			{
				Name:       "password",
				Help:       "Password of your Ente account",
				Required:   false,
				IsPassword: true,
				Sensitive:  true,
			},
			{
				Name:       "session_token",
				Help:       "Session Auth Token for Ente API (X-Auth-Token)",
				Required:   false,
				IsPassword: true,
				Sensitive:  true,
			},
			{
				Name:       "account_key",
				Help:       "Master encryption key for account (hex encoded)",
				Required:   false,
				IsPassword: true,
				Sensitive:  true,
			},
			{
				Name:     "endpoint",
				Help:     "Ente API server URL",
				Default:  defaultEndpoint,
				Advanced: true,
			},
			{
				Name:     "app",
				Help:     "Ente app type (photos, drive, secrets)",
				Default:  "photos",
				Advanced: true,
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Display |
					encoder.EncodeDoubleQuote |
					encoder.EncodeBackSlash |
					encoder.EncodeInvalidUtf8),
			},
		},
	})
}

// Options defines configuration options
type Options struct {
	Email        string               `config:"email"`
	Password     string               `config:"password"`
	SessionToken string               `config:"session_token"`
	AccountKey   string               `config:"account_key"`
	Endpoint     string               `config:"endpoint"`
	App          string               `config:"app"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents an Ente remote
type Fs struct {
	name      string
	root      string
	opt       Options
	features  *fs.Features
	srv       *rest.Client
	dirCache  *dircache.DirCache
	pacer     *fs.Pacer
	masterKey []byte
	authToken string
}

// Object represents an Ente file
type Object struct {
	fs           *Fs
	remote       string
	size         int64
	modTime      time.Time
	id           int64
	collectionID int64
	fileKey      []byte
	hashMD5      string
}

// Ente API data structures
type Collection struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	EncryptedKey       string `json:"encryptedKey"`
	KeyDecryptionNonce string `json:"keyDecryptionNonce"`
	UpdationTime       int64  `json:"updationTime"`
}

type FileItem struct {
	ID           int64        `json:"id"`
	CollectionID int64        `json:"collectionId"`
	Size         int64        `json:"fileSize,omitempty"`
	EncryptedKey string       `json:"encryptedKey"`
	KeyNonce     string       `json:"keyDecryptionNonce"`
	UpdationTime int64        `json:"updationTime"`
	Metadata     FileMetadata `json:"metadata"`
}

type FileMetadata struct {
	Title string `json:"title"`
}

type CollectionsResponse struct {
	Collections []Collection `json:"collections"`
}

type FilesResponse struct {
	Files []FileItem `json:"files"`
}

type CreateCollectionRequest struct {
	Name               string `json:"name"`
	EncryptedKey       string `json:"encryptedKey"`
	KeyDecryptionNonce string `json:"keyDecryptionNonce"`
}

type CreateCollectionResponse struct {
	Collection Collection `json:"collection"`
}

// Name of remote
func (f *Fs) Name() string {
	return f.name
}

// Root of remote
func (f *Fs) Root() string {
	return f.root
}

// String representation
func (f *Fs) String() string {
	return fmt.Sprintf("ente root '%s'", f.root)
}

// Features returns optional features
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision of mod time
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// Hashes supported
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// NewFs creates a new Fs instance
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.Endpoint == "" {
		opt.Endpoint = defaultEndpoint
	}

	client := fshttp.NewClient(ctx)
	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		srv:   rest.NewClient(client).SetRoot(opt.Endpoint),
	}

	f.srv.SetHeader(headerClientPkg, appPhotos)
	if opt.SessionToken != "" {
		f.authToken = opt.SessionToken
		f.srv.SetHeader(headerAuthToken, f.authToken)
	}

	if opt.AccountKey != "" {
		keyBytes, err := hex.DecodeString(opt.AccountKey)
		if err == nil && len(keyBytes) == 32 {
			f.masterKey = keyBytes
		}
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	f.dirCache = dircache.New(root, "0", f)

	// Test connection
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil && err != fs.ErrorDirNotFound {
		fs.Debugf(f, "FindRoot error: %v", err)
	}

	return f, nil
}

// FindLeaf finds directory leaf ID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	collections, err := f.getCollections(ctx)
	if err != nil {
		return "", false, err
	}

	for _, c := range collections {
		if c.Name == leaf {
			return strconv.FormatInt(c.ID, 10), true, nil
		}
	}

	return "", false, nil
}

// CreateDir creates a new collection/directory
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	dummyKey := make([]byte, 32)
	rand.Read(dummyKey)
	req := CreateCollectionRequest{
		Name:               leaf,
		EncryptedKey:       hex.EncodeToString(dummyKey),
		KeyDecryptionNonce: hex.EncodeToString(make([]byte, 12)),
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/collections",
	}

	var resp CreateCollectionResponse
	err = f.pacer.Call(func() (bool, error) {
		r, err := f.srv.CallJSON(ctx, &opts, &req, &resp)
		return shouldRetry(ctx, r, err)
	})
	if err != nil {
		return "", fmt.Errorf("failed to create collection: %w", err)
	}

	return strconv.FormatInt(resp.Collection.ID, 10), nil
}

// getCollections gets all collections
func (f *Fs) getCollections(ctx context.Context) ([]Collection, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/collections",
	}

	var resp CollectionsResponse
	err := f.pacer.Call(func() (bool, error) {
		r, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, r, err)
	})
	if err != nil {
		return nil, err
	}

	return resp.Collections, nil
}

// List lists directory contents
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	dirID, _ := strconv.ParseInt(directoryID, 10, 64)

	opts := rest.Opts{
		Method: "GET",
		Path:   fmt.Sprintf("/files/collection/%d", dirID),
	}

	var resp FilesResponse
	err = f.pacer.Call(func() (bool, error) {
		r, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, r, err)
	})
	if err != nil {
		return nil, err
	}

	for _, file := range resp.Files {
		title := file.Metadata.Title
		if title == "" {
			title = fmt.Sprintf("file_%d", file.ID)
		}
		remote := pathModule.Join(dir, title)
		obj := &Object{
			fs:           f,
			remote:       remote,
			id:           file.ID,
			collectionID: file.CollectionID,
			size:         file.Size,
			modTime:      time.Unix(file.UpdationTime/1000000000, 0),
		}
		entries = append(entries, obj)
	}

	return entries, nil
}

// NewObject creates a new Object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	dir, leaf := pathModule.Split(remote)
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	dirID, _ := strconv.ParseInt(directoryID, 10, 64)

	opts := rest.Opts{
		Method: "GET",
		Path:   fmt.Sprintf("/files/collection/%d", dirID),
	}

	var resp FilesResponse
	err = f.pacer.Call(func() (bool, error) {
		r, err := f.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, r, err)
	})
	if err != nil {
		return nil, err
	}

	for _, file := range resp.Files {
		title := file.Metadata.Title
		if title == leaf {
			return &Object{
				fs:           f,
				remote:       remote,
				id:           file.ID,
				collectionID: file.CollectionID,
				size:         file.Size,
				modTime:      time.Unix(file.UpdationTime/1000000000, 0),
			}, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// Put uploads an object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	dir, _ := pathModule.Split(src.Remote())
	directoryID, err := f.dirCache.FindDir(ctx, dir, true)
	if err != nil {
		return nil, err
	}

	dirID, _ := strconv.ParseInt(directoryID, 10, 64)

	obj := &Object{
		fs:           f,
		remote:       src.Remote(),
		size:         src.Size(),
		modTime:      src.ModTime(ctx),
		collectionID: dirID,
	}

	err = obj.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// Mkdir creates a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	dirID, _ := strconv.ParseInt(directoryID, 10, 64)

	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/collections/%d", dirID),
	}

	err = f.pacer.Call(func() (bool, error) {
		r, err := f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, r, err)
	})
	if err != nil {
		return err
	}

	f.dirCache.Flush()
	return nil
}

// Object methods
func (o *Object) Fs() fs.Info {
	return o.fs
}

func (o *Object) ReturnFs() *Fs {
	return o.fs
}

func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

func (o *Object) Remote() string {
	return o.remote
}

func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

func (o *Object) Size() int64 {
	return o.size
}

func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty == hash.MD5 {
		return o.hashMD5, nil
	}
	return "", hash.ErrUnsupported
}

func (o *Object) Storable() bool {
	return true
}

func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	o.modTime = modTime
	return nil
}

func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	opts := rest.Opts{
		Method:  "GET",
		Path:    fmt.Sprintf("/files/data/%d", o.id),
		Options: options,
	}

	resp, err := o.fs.srv.Call(ctx, &opts)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, in)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:      "POST",
		Path:        fmt.Sprintf("/files/data?collectionId=%d", o.collectionID),
		Body:        buf,
		ContentType: "application/octet-stream",
	}

	var resp FileItem
	err = o.fs.pacer.Call(func() (bool, error) {
		r, err := o.fs.srv.CallJSON(ctx, &opts, nil, &resp)
		return shouldRetry(ctx, r, err)
	})
	if err != nil {
		return err
	}

	o.id = resp.ID
	o.size = src.Size()
	o.modTime = src.ModTime(ctx)
	return nil
}

func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/files/%d", o.id),
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		r, err := o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, r, err)
	})
	return err
}

// shouldRetry checks if response/err should be retried
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, []int{429, 500, 502, 503, 504}), err
}
