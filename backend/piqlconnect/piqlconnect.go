// Package piqlconnect provides an interface to piqlConnect
package piqlconnect

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
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
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	size    int64     // size of the object
	modTime time.Time // modification time of the object
	id      string    // ID of the object
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

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("piqlConnect '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return &fs.Features{
		CanHaveEmptyDirectories: true,
	}
}

// parsePath parses a piqlconnect 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := Options{}
	err := configstruct.Set(m, &opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote. If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

func (f *Fs) getFiles(ctx context.Context, segments []string) (files []api.Item, err error) {
	packageName := segments[1]
	values := url.Values{}
	values.Set("organisationId", f.organisationId)
	if f.packageIdCache[packageName] == "" {
		f.listPackages(ctx, segments[0])
	}
	packageId := f.packageIdCache[packageName]
	values.Set("packageId", packageId)
	packageRelativePath := strings.Join(segments[2:], "/")
	values.Set("path", packageRelativePath)
	resp, err := f.client.CallJSON(ctx, &rest.Opts{Path: "/files", Parameters: values}, nil, &files)
	if err != nil {
		if resp.StatusCode == 404 {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}
	return files, nil
}

func (f *Fs) listFiles(ctx context.Context, absolutePath string) (entries fs.DirEntries, err error) {
	segments := getPathSegments(absolutePath)
	files, err := f.getFiles(ctx, segments)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		file.Path = segments[0] + "/" + segments[1] + "/" + file.Path
		if file.Size == 0 && file.Path[len(file.Path)-1] == '/' {
			modTime, err := time.Parse(time.RFC3339, file.UpdatedAt)
			if err != nil {
				return nil, err
			}
			dir := fs.NewDir(f.absolutePathToRclone(file.Path[0:len(file.Path)-1]), modTime)
			entries = append(entries, dir)
		} else {
			o, err := f.newObjectWithInfo(ctx, f.absolutePathToRclone(file.Path), &file)
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
	}
	return entries, nil
}

func (f *Fs) listPackages(ctx context.Context, absolutePath string) (entries fs.DirEntries, err error) {
	segments := getPathSegments(absolutePath)
	values := url.Values{}
	values.Set("organisationId", f.organisationId)

	ps := []api.Package{}
	_, err = f.client.CallJSON(ctx, &rest.Opts{Path: "/packages", Parameters: values}, nil, &ps)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		f.packageIdCache[p.Name] = p.Id
		mtime, err := time.Parse(time.RFC3339, p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entry := fs.NewDir(f.absolutePathToRclone(segments[0]+"/"+p.Name), mtime)
		if segments[0] == "Workspace" && p.Status == "ACTIVE" {
			entries = append(entries, entry)
		} else if segments[0] == "In Progress" && (p.Status == "PENDING_PAYMENT" || p.Status == "PREPARING" || p.Status == "PROCESSING") {
			entries = append(entries, entry)
		} else if segments[0] == "Archive" && p.Status == "ARCHIVED" {
			entries = append(entries, entry)
		}
	}
	return
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This returns ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	absolutePath := f.rclonePathToAbsolute(dir)
	if len(absolutePath) == 0 {
		entries = append(
			entries,
			fs.NewDir(f.absolutePathToRclone(absolutePath+"/Workspace"), time.Unix(0, 0)),
			fs.NewDir(f.absolutePathToRclone(absolutePath+"/In Progress"), time.Unix(0, 0)),
			fs.NewDir(f.absolutePathToRclone(absolutePath+"/Archive"), time.Unix(0, 0)),
		)
		return entries, nil
	}
	segments := getPathSegments(absolutePath)
	if len(segments) >= 2 {
		return f.listFiles(ctx, absolutePath)
	}
	return f.listPackages(ctx, segments[0])
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, optons ...fs.OpenOption) (fs.Object, error) {
	dir := path.Join(f.root, src.Remote())
	segments := f.getAbsolutePathSegments(src.Remote())
	if f.packageIdCache[segments[1]] == "" {
		f.listPackages(ctx, dir)
	}
	reqBody := api.CreateFileUrl{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		Files:          [1]api.FilePath{{Path: strings.Join(segments[2:], "/")}},
		Method:         "OVERWRITE",
	}
	var results [1]string
	_, err := f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/sas-url"}, &reqBody, &results)
	if err != nil {
		return nil, err
	}
	size := src.Size()
	if size > 4000*1024*1024 {
		i := 0
		bytes_left := size
		for bytes_left > 0 {
			i++
			chunkSize := min(4000*1024*1024, bytes_left)
			bytes_left -= chunkSize
			url := results[0] + "&comp=block&blockid=" + url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(i))))
			req, err := http.NewRequestWithContext(ctx, "PUT", url, io.LimitReader(in, chunkSize))
			if err != nil {
				return nil, err
			}
			req.ContentLength = chunkSize
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
		}

		blocklist := "<?xml version=\"1.0\" encoding=\"utf-8\"?><BlockList>"
		for j := 1; j <= i; j++ {
			blocklist += "<Uncommitted>" + base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(j))) + "</Uncommitted>"
		}
		blocklist += "</BlockList>"
		req, err := http.NewRequestWithContext(ctx, "PUT", results[0]+"&comp=blocklist", strings.NewReader(blocklist))
		if err != nil {
			return nil, err
		}
		req.ContentLength = int64(len(blocklist))
		resp, err := f.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			respBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("invalid http response code from azure: " + resp.Status + "\n" + string(respBytes))
		}
	} else {
		if size == 0 {
			in = nil
		}
		req, err := http.NewRequestWithContext(ctx, "PUT", results[0], in)
		if err != nil {
			return nil, err
		}
		req.ContentLength = size
		req.Header.Add("x-ms-blob-type", "BlockBlob")
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

	}

	filesReqBody := api.CreateFile{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		Files:          [1]api.FilePathSize{{Path: strings.Join(segments[2:], "/"), Size: size}},
	}
	var fileIds [1]string
	_, err = f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/files"}, &filesReqBody, &fileIds)
	if err != nil {
		return nil, err
	}
	return &Object{
		fs:      f,
		remote:  src.Remote(),
		size:    size,
		modTime: time.Now(),
		id:      fileIds[0],
	}, nil

}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	segments := f.getAbsolutePathSegments(dir)
	if f.packageIdCache[segments[1]] == "" {
		f.listPackages(ctx, segments[0])
	}
	createFolder := api.CreateFolder{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		FolderPath:     strings.Join(segments[2:], "/"),
	}

	resp, err := f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/folders"}, &createFolder, nil)
	if err != nil {
		return err
	}
	folderIdBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	folderId := string(folderIdBytes)
	_ = folderId
	return nil
}

func (f *Fs) deleteObject(ctx context.Context, packageName string, id string) error {
	removeFile := api.RemoveFile{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[packageName],
		FileIds:        [1]string{id},
	}
	_, err := f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/files/delete"}, &removeFile, nil)
	if err != nil {
		return err
	}
	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	segments := f.getAbsolutePathSegments(dir)
	if f.packageIdCache[segments[1]] == "" {
		f.listPackages(ctx, segments[0])
	}
	removeFolder := api.RemoveFolder{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		FolderPaths:    [1]string{strings.Join(segments[2:], "/")},
	}
	_, err := f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/folders/delete"}, &removeFolder, nil)
	if err != nil {
		return err
	}
	return nil
}

// Purge deletes all the files and the container
func (f *Fs) Purge(ctx context.Context, dir string) error {
	segments := f.getAbsolutePathSegments(dir)
	if f.packageIdCache[segments[1]] == "" {
		f.listPackages(ctx, segments[0])
	}
	removeFolder := api.RemoveFolder{
		OrganisationId: f.organisationId,
		PackageId:      f.packageIdCache[segments[1]],
		FolderPaths:    [1]string{strings.Join(segments[2:], "/")},
	}
	_, err := f.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/folders/delete"}, &removeFolder, nil)
	if err != nil {
		return err
	}
	return nil
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
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the MD5 of an object as a lowercase hex string
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	segments := o.fs.getAbsolutePathSegments(o.remote)
	if o.fs.packageIdCache[segments[1]] == "" {
		o.fs.listPackages(ctx, segments[0])
	}
	reqBody := api.CreateFileUrl{
		OrganisationId: o.fs.organisationId,
		PackageId:      o.fs.packageIdCache[segments[1]],
		Files:          [1]api.FilePath{{Path: path.Join(segments[2:]...)}},
		Method:         "READ",
	}
	var results [1]string
	_, err := o.fs.client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/sas-url"}, &reqBody, &results)
	if err != nil {
		return "", err
	}
	resp, err := o.fs.httpClient.Head(results[0])
	if err != nil {
		return "", err
	}
	md5header := resp.Header.Get("Content-MD5")
	if md5header == "" {
		return "", nil
	}
	md5bytes, err := base64.StdEncoding.DecodeString(md5header)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(md5bytes), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) error {
	if info.Size == 0 && info.Path[len(info.Path)-1] == '/' {
		return fs.ErrorIsDir
	}
	modTime, err := time.Parse(time.RFC3339, info.UpdatedAt)
	if err != nil {
		return err
	}
	o.size = info.Size
	o.modTime = modTime
	o.id = info.Id
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.id != "" {
		return nil
	}
	segments := o.fs.getAbsolutePathSegments(path.Dir(o.remote))
	entries, err := o.fs.getFiles(ctx, segments)
	if err != nil {
		return err
	}
	packageRelativePath := strings.Join(segments[2:], "/")
	for _, entry := range entries {
		if packageRelativePath == entry.Path {
			o.setMetaData(&entry)
			return nil
		}
	}
	return fs.ErrorObjectNotFound
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	segments := o.fs.getAbsolutePathSegments(o.remote)
	if o.fs.packageIdCache[segments[1]] == "" {
		o.fs.listPackages(ctx, segments[0])
	}

	touchFile := api.TouchFile{
		OrganisationId: o.fs.organisationId,
		PackageId:      o.fs.packageIdCache[segments[1]],
		Files:          [1]string{path.Join(segments[2:]...)},
	}
	_, err := o.fs.client.CallJSON(ctx, &rest.Opts{
		Method: "POST",
		Path:   "/files/touch",
	}, &touchFile, nil)
	if err != nil {
		return err
	}
	return nil
}

// Storable returns a boolean showing whether this object is storable
func (o *Object) Storable() bool {
	return true
}

func (f *Fs) rclonePathToAbsolute(dir string) string {
	return path.Join(f.root, dir)
}

func getPathSegments(dir string) []string {
	return strings.Split(dir, "/")
}
func (f *Fs) getAbsolutePathSegments(dir string) []string {
	return getPathSegments(f.rclonePathToAbsolute(dir))
}

func (f *Fs) absolutePathToRclone(absolutePath string) string {
	r, _ := strings.CutPrefix(absolutePath, f.root+"/")
	return r
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	segments := o.fs.getAbsolutePathSegments(o.remote)
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
	req, err := http.NewRequestWithContext(ctx, "GET", string(sasUrlBytes), nil)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		req.Header.Set(option.Header())
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
	segments := o.fs.getAbsolutePathSegments(o.remote)
	if o.fs.packageIdCache[segments[1]] == "" {
		o.fs.listPackages(ctx, segments[0])
	}
	return o.fs.deleteObject(ctx, segments[1], o.id)
}

func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	// TODO: PutStreamer is implementable for piqlConnect
	// _ fs.PutStreamer = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
	// TODO: IDer is implementable for piqlConnect
	// _ fs.IDer = (*Object)(nil)
	// TODO: ParentIDer is implementable for piqlConnect
	// _ fs.ParentIDer = (*Object)(nil)
)
