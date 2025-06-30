// Package piqlconnect provides an interface to piqlConnect
package piqlconnect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/piqlconnect/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "piqlconnect",
		Description: "piqlConnect",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:      "api_key",
				Help:      "piqlConnect API key obtained from web interface",
				Required:  true,
				Sensitive: true,
			},
			{
				Name:     "root_url",
				Help:     "piqlConnect API url",
				Required: true,
				Default:  "https://app.piql.com/api",
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ApiKey  string `config:"api_key"`
	RootURL string `config:"root_url"`
}

// Fs represents a remote piqlConnect path
type Fs struct {
	name           string            // name of this remote
	root           string            // the path we are working on
	organisationId string            // the organisation ID in piqlConnect
	httpClient     *http.Client      // http Client used for external HTTP calls (file downloads / uploads)
	client         *rest.Client      // rest Client used for API calls
	packageIdCache map[string]string // map of package name to package ID
}

// Object represents a piqlConnect object
type Object struct {
	fs   *Fs
	file api.Item
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

func (f *Fs) String() string {
	return fmt.Sprintf("piqlConnect '%s'", f.root)
}

func (f *Fs) Features() *fs.Features {
	return &fs.Features{
		CanHaveEmptyDirectories: true,
	}
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	if strings.Index(root, "/") == 0 {
		root = root[1:]
	}
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[0 : len(root)-1]
	}
	// Parse config into Options struct
	opt := Options{}
	err := configstruct.Set(m, &opt)
	if err != nil {
		return nil, err
	}

	httpclient := fshttp.NewClient(ctx)
	client := rest.NewClient(httpclient)
	client.SetRoot(opt.RootURL)
	client.SetHeader("Authorization", "Bearer "+opt.ApiKey)
	resp, err := client.Call(ctx, &rest.Opts{Path: "/user/api-key/organisation"})
	if err != nil {
		return nil, err
	}
	organisationIdBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:           name,
		root:           root,
		organisationId: string(organisationIdBytes),
		httpClient:     httpclient,
		client:         client,
		packageIdCache: make(map[string]string),
	}
	return f, nil
}

// NewObject finds the Object at remote. If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// TODO
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	segments := f.getPathSegments(dir)
	if len(segments) == 1 {
		return f.listPackages(ctx, dir)
	}
	if len(segments) >= 2 {
		return f.listFiles(ctx, segments[1], strings.Join(segments[2:], "/"))
	}

	return entries, nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.file.Path
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.file.Path
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.file.Size
}

// ModTime returns the modification time of the object
//
// It attempts to parse the objects mtime
func (o *Object) ModTime(ctx context.Context) time.Time {
	mtime, err := time.Parse(time.RFC3339, o.file.UpdatedAt)
	if err != nil {
		return time.Now()
	}
	return mtime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return errors.New("failed to set modtime")
}

// Storable returns a boolean showing whether this object is storable
func (o *Object) Storable() bool {
	return true
}

func (f *Fs) getPathSegments(dir string) []string {
	return strings.Split(path.Join(f.root, dir), "/")
}

func (f *Fs) adjustPackagePath(dir string, packageName string) string {
	r, _ := strings.CutPrefix(path.Join(dir, packageName), f.root+"/")
	return r
}

func (f *Fs) listPackages(ctx context.Context, path string) (entries fs.DirEntries, err error) {
	segments := f.getPathSegments(path)
	values := url.Values{}
	values.Set("organisationId", f.organisationId)

	ps := []api.Package{}
	_, err = f.client.CallJSON(ctx, &rest.Opts{Path: "/packages", Parameters: values}, nil, &ps)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		f.packageIdCache[p.Name] = p.Id
		if segments[0] == "Workspace" && p.Status == "ACTIVE" {
			mtime, err := time.Parse(time.RFC3339, p.UpdatedAt)
			if err != nil {
				return nil, err
			}
			entries = append(entries, fs.NewDir(f.adjustPackagePath(segments[0], p.Name), mtime))
		}
	}
	return
}

func (fs *Fs) listFiles(ctx context.Context, packageName string, dir string) (entries fs.DirEntries, err error) {
	values := url.Values{}
	values.Set("organisationId", fs.organisationId)
	if fs.packageIdCache[packageName] == "" {
		fs.listPackages(ctx, dir)
	}
	values.Set("packageId", fs.packageIdCache[packageName])
	values.Set("path", dir)
	files := []api.Item{}
	_, err = fs.client.CallJSON(ctx, &rest.Opts{Path: "/files", Parameters: values}, nil, &files)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		f.Path = fs.adjustPackagePath(dir, packageName) + "/" + f.Path
		if f.Size == 0 && f.Path[len(f.Path)-1] == '/' {
			dir := Directory{
				Path:      f.Path[0 : len(f.Path)-1],
				UpdatedAt: f.UpdatedAt,
			}
			dir.SetFs(fs)
			entries = append(entries, dir)
		} else {
			o := Object{
				fs:   fs,
				file: f,
			}
			entries = append(entries, &o)
		}
	}
	return entries, nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	segments := o.fs.getPathSegments(o.file.Path)
	if o.fs.packageIdCache[segments[1]] == "" {
		o.fs.listPackages(ctx, segments[0])
	}
	downloadFile := api.DownloadFile{
		OrganisationId: o.fs.organisationId,
		PackageId:      o.fs.packageIdCache[segments[1]],
		BlobNames:      [1]string{path.Join(segments[2:]...)},
	}
	resp, err := o.fs.client.CallJSON(ctx, &rest.Opts{
		Method: "POST",
		Path:   "/files/download",
	}, &downloadFile, nil)
	if err != nil {
		return nil, err
	}
	sasUrlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp, err = o.fs.httpClient.Get(string(sasUrlBytes))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	err := o.Remove(ctx)
	if err != nil {
		return err
	}
	_, err = o.fs.Put(ctx, in, src, options...)
	if err != nil {
		return err
	}
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	segments := o.fs.getPathSegments(o.file.Path)
	o.fs.listPackages(ctx, o.file.Path)
	removeFile := api.RemoveFile{
		OrganisationId: o.fs.organisationId,
		PackageId:      o.fs.packageIdCache[segments[1]],
		FileIds:        [1]string{o.file.Id},
	}
	_, err := o.fs.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/files/delete"}, &removeFile, nil)
	if err != nil {
		return err
	}
	return nil
}

type Directory struct {
	fs        fs.Info
	Path      string `json:"path"`
	UpdatedAt string `json:"updatedAt"`
}

func (dir *Directory) SetFs(fs fs.Info) {
	dir.fs = fs
}

func (dir *Directory) SetPath(path string) {
	dir.Path = path
}

// DirEntry
func (dir Directory) Fs() fs.Info {
	return dir.fs
}

func (dir Directory) String() string {
	return path.Base(dir.Path)
}

func (dir Directory) Remote() string {
	return dir.Path
}

func (dir Directory) ModTime(ctx context.Context) time.Time {
	mtime, err := time.Parse(time.RFC3339, dir.UpdatedAt)
	if err != nil {
		panic("Failed to parse time")
	}
	return mtime
}

func (dir Directory) Size() int64 {
	return 0
}

// Directory
func (dir Directory) Items() int64 {
	return -1
}

func (dir Directory) ID() string {
	return ""
}

type TopDirKind uint8

const (
	TopDirWorkspace TopDirKind = iota
	TopDirInProgress
	TopDirArchive
)

func (topDir TopDirKind) name() string {
	if topDir == TopDirWorkspace {
		return "Workspace"
	}
	if topDir == TopDirInProgress {
		return "In Progress"
	}
	if topDir == TopDirArchive {
		return "Archive"
	}
	panic("unreachable")
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	dir := path.Join(f.root, src.Remote())
	segments := f.getPathSegments(src.Remote())
	if f.packageIdCache[segments[1]] == "" {
		f.listPackages(ctx, dir)
	}
	type FilePath struct {
		Path string `json:"path"`
	}
	reqBody := struct {
		OrganisationId string      `json:"organisationId"`
		PackageId      string      `json:"packageId"`
		Files          [1]FilePath `json:"files"`
		Method         string      `json:"method"`
	}{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		Files:          [1]FilePath{{Path: strings.Join(segments[2:], "/")}},
		Method:         "OVERWRITE",
	}
	var results [1]string
	_, err := f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/sas-url"}, &reqBody, &results)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", results[0], in)
	req.ContentLength = src.Size()
	req.Header.Add("x-ms-blob-type", "BlockBlob")
	if err != nil {
		return nil, err
	}
	azureResp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if azureResp.StatusCode < 200 || azureResp.StatusCode > 299 {
		azureRespBytes, err := io.ReadAll(azureResp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("invalid http response code from azure: " + azureResp.Status + "\n" + string(azureRespBytes))
	}
	type FilePathSize struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	filesReqBody := struct {
		OrganisationId string          `json:"organisationId"`
		PackageId      string          `json:"packageId"`
		Files          [1]FilePathSize `json:"files"`
	}{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		Files:          [1]FilePathSize{{Path: strings.Join(segments[2:], "/"), Size: src.Size()}},
	}
	var fileIds [1]string
	_, err = f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/files"}, &filesReqBody, &fileIds)
	if err != nil {
		return nil, err
	}
	adjusted, _ := strings.CutPrefix(segments[0]+"/"+segments[1]+"/"+strings.Join(segments[2:], "/"), f.root+"/")
	return &Object{
		fs: f,
		file: api.Item{
			Id:        fileIds[0],
			Path:      adjusted,
			UpdatedAt: time.Now().Format(time.RFC3339),
			Size:      src.Size(),
		},
	}, nil

}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs = (*Fs)(nil)
	// TODO: Purger is implementable for piqlConnect
	// _ fs.Purger = (*Fs)(nil)
	// TODO: PutStreamer is implementable for piqlConnect
	// _ fs.PutStreamer = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
	// TODO: IDer is implementable for piqlConnect
	// _ fs.IDer = (*Fs)(nil)
)
