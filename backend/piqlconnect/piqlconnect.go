// Package piqlconnect provides an interface to piqlConnect
package piqlconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

type Package struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type File struct {
	fs        *Fs
	Id        string `json:"id"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updatedAt"`
	FileSize  int64  `json:"size"`
}

func (file *File) SetFs(fs *Fs) {
	file.fs = fs
}

func (file *File) SetPath(path string) {
	file.Path = path
}

// DirEntry
func (file File) Fs() fs.Info {
	return file.fs
}

func (file File) String() string {
	return path.Base(file.Path)
}

func (file File) Remote() string {
	return file.Path
}

func (file File) ModTime(ctx context.Context) time.Time {
	mtime, err := time.Parse(time.RFC3339, file.UpdatedAt)
	if err != nil {
		panic("Failed to parse time")
	}
	return mtime
}

func (file File) Size() int64 {
	return file.FileSize
}

// ObjectInfo
func (file File) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

func (file File) Storable() bool {
	return true
}

// Object
func (file File) SetModTime(ctx context.Context, t time.Time) error {
	return fmt.Errorf("failed to set modtime")
}

func (file File) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	segments := strings.Split(path.Join(file.fs.RootPath, file.Path), "/")
	if file.fs.PackageIdMap[segments[1]] == "" {
		var topDir TopDirKind
		if segments[0] == "Workspace" {
			topDir = TopDirWorkspace
		}
		if segments[0] == "In Progress" {
			topDir = TopDirInProgress
		}
		if segments[0] == "Archive" {
			topDir = TopDirArchive
		}
		file.fs.listPackages(ctx, topDir)
	}
	json_string, err := json.Marshal(struct {
		OrganisationId string    `json:"organisationId"`
		PackageId      string    `json:"packageId"`
		PackageName    string    `json:"packageName"`
		BlobNames      [1]string `json:"blobNames"`
	}{
		OrganisationId: file.fs.OrganisationId,
		PackageId:      file.fs.PackageIdMap[segments[1]],
		PackageName:    "",
		BlobNames:      [1]string{path.Join(segments[2:]...)},
	})
	if err != nil {
		return nil, err
	}
	resp, err := file.fs.Client.Call(ctx, &rest.Opts{
		Method: "POST",
		Path:   "/api/files/download",
		Body:   strings.NewReader(string(json_string)),
	})
	if err != nil {
		return nil, err
	}
	sasUrlBytes, err := io.ReadAll(resp.Body)
	sasUrl := string(sasUrlBytes)
	if err != nil {
		return nil, err
	}
	resp, err = file.fs.HttpClient.Get(sasUrl)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (file File) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	fmt.Printf("Update(%s)\n", file.Remote())
	err := file.Remove(ctx)
	if err != nil {
		return err
	}
	_, err = file.fs.Put(ctx, in, src, options...)
	if err != nil {
		return err
	}
	return nil
}

func (file File) Remove(ctx context.Context) error {
	fmt.Printf("Remove(%s)\n", file.Remote())
	dir := path.Join(file.fs.RootPath, file.Remote())
	segments := strings.Split(dir, "/")
	var topDir TopDirKind
	if file.fs.PackageIdMap[segments[1]] == "" {
		if segments[0] == "Workspace" {
			topDir = TopDirWorkspace
		}
		if segments[0] == "In Progress" {
			topDir = TopDirInProgress
		}
		if segments[0] == "Archive" {
			topDir = TopDirArchive
		}
		file.fs.listPackages(ctx, topDir)
	}
	reqBody := struct {
		OrganisationId string    `json:"organisationId"`
		PackageId      string    `json:"packageId"`
		FileIds        [1]string `json:"fileIds"`
	}{
		OrganisationId: file.fs.OrganisationId,
		PackageId:      file.fs.PackageIdMap[segments[1]],
		FileIds:        [1]string{file.Id},
	}
	_, err := file.fs.Client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/api/files/delete"}, &reqBody, nil)
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

const (
	rootURL = "http://192.168.9.248:3000"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "piqlconnect",
		Description: "piqlConnect",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return nil, nil
		},
		Options: []fs.Option{
			{
				Name:      "api_key",
				Help:      "piqlConnect API key obtained from web interface",
				Required:  true,
				Sensitive: true,
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ApiKey string `config:"api_key"`
}

// Fs represents a remote piqlConnect package
type Fs struct {
	OrganisationId string
	HttpClient     *http.Client
	Client         *rest.Client
	PackageIdMap   map[string]string
	RootPath       string
}

func (f *Fs) Name() string {
	return "hello"
}

func (f *Fs) Root() string {
	return f.RootPath
}

func (f *Fs) String() string {
	return "piqlConnect[" + f.OrganisationId + "]"
}

func (f *Fs) Features() *fs.Features {
	return &fs.Features{
		CanHaveEmptyDirectories: true,
	}
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return nil, nil
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

func (f *Fs) listPackages(ctx context.Context, topDir TopDirKind) (entries fs.DirEntries, err error) {
	values := url.Values{}
	values.Set("organisationId", f.OrganisationId)

	ps := []Package{}
	_, err = f.Client.CallJSON(ctx, &rest.Opts{Path: "/api/packages", Parameters: values}, nil, &ps)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		f.PackageIdMap[p.Name] = p.Id
		switch topDir {
		case TopDirWorkspace:
			if p.Status != "ACTIVE" {
				continue
			}
		case TopDirInProgress:
			if p.Status != "PENDING_PAYMENT" && p.Status != "PREPARING" && p.Status != "PROCESSING" {
				continue
			}
		case TopDirArchive:
			if p.Status != "ARCHIVED" {
				continue
			}
		}
		mtime, err := time.Parse(time.RFC3339, p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		adjusted, _ := strings.CutPrefix(path.Join(topDir.name(), p.Name), f.RootPath+"/")
		entries = append(entries, fs.NewDir(adjusted, mtime))
	}
	return entries, nil
}

func (fs *Fs) listFiles(ctx context.Context, topDir TopDirKind, packageName string, dirPath []string) (entries fs.DirEntries, err error) {
	values := url.Values{}
	values.Set("organisationId", fs.OrganisationId)
	if fs.PackageIdMap[packageName] == "" {
		fs.listPackages(ctx, topDir)
	}
	values.Set("packageId", fs.PackageIdMap[packageName])
	values.Set("path", path.Join(dirPath...))
	files := []File{}
	_, err = fs.Client.CallJSON(ctx, &rest.Opts{Path: "/api/files", Parameters: values}, nil, &files)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		f.SetFs(fs)
		adjusted, _ := strings.CutPrefix(topDir.name()+"/"+packageName+"/"+f.Path, fs.RootPath+"/")
		f.SetPath(adjusted)
		if f.FileSize == 0 && f.Path[len(f.Path)-1] == '/' {
			dir := Directory{
				Path:      f.Path[0 : len(f.Path)-1],
				UpdatedAt: f.UpdatedAt,
			}
			dir.SetFs(fs)
			entries = append(entries, dir)
		} else {
			entries = append(entries, f)
		}
	}
	return entries, nil
}

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dir = path.Join(f.RootPath, dir)
	fmt.Printf("List(\"%s\")\n", dir)
	if len(dir) == 0 {
		return append(
			entries,
			fs.NewDir(path.Join(dir, "Workspace"), time.Unix(0, 0)),
			fs.NewDir(path.Join(dir, "In Progress"), time.Unix(0, 0)),
			fs.NewDir(path.Join(dir, "Archive"), time.Unix(0, 0)),
		), nil
	}
	segments := strings.Split(dir, "/")
	var topDir TopDirKind
	if segments[0] == "Workspace" {
		topDir = TopDirWorkspace
	}
	if segments[0] == "In Progress" {
		topDir = TopDirInProgress
	}
	if segments[0] == "Archive" {
		topDir = TopDirArchive
	}
	if len(segments) == 1 {
		return f.listPackages(ctx, topDir)
	}
	if len(segments) >= 2 {
		return f.listFiles(ctx, topDir, segments[1], segments[2:])
	}

	return entries, nil
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fmt.Printf("Put(%s)\n", src.Remote())
	dir := path.Join(f.RootPath, src.Remote())
	segments := strings.Split(dir, "/")
	var topDir TopDirKind
	if f.PackageIdMap[segments[1]] == "" {
		if segments[0] == "Workspace" {
			topDir = TopDirWorkspace
		}
		if segments[0] == "In Progress" {
			topDir = TopDirInProgress
		}
		if segments[0] == "Archive" {
			topDir = TopDirArchive
		}
		f.listPackages(ctx, topDir)
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
		OrganisationId: f.OrganisationId,
		PackageId:      f.PackageIdMap[segments[1]],
		Files:          [1]FilePath{{Path: strings.Join(segments[2:], "/")}},
		Method:         "OVERWRITE",
	}
	var results [1]string
	_, err := f.Client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/api/sas-url"}, &reqBody, &results)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", results[0], in)
	req.ContentLength = src.Size()
	req.Header.Add("x-ms-blob-type", "BlockBlob")
	if err != nil {
		return nil, err
	}
	azureResp, err := f.HttpClient.Do(req)
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
		OrganisationId: f.OrganisationId,
		PackageId:      f.PackageIdMap[segments[1]],
		Files:          [1]FilePathSize{{Path: strings.Join(segments[2:], "/"), Size: src.Size()}},
	}
	var fileIds [1]string
	_, err = f.Client.CallJSON(ctx, &rest.Opts{Method: "POST", Path: "/api/files"}, &filesReqBody, &fileIds)
	if err != nil {
		return nil, err
	}
	adjusted, _ := strings.CutPrefix(topDir.name()+"/"+segments[1]+"/"+strings.Join(segments[2:], "/"), f.RootPath+"/")
	return &File{
		fs:        f,
		Id:        fileIds[0],
		Path:      adjusted,
		UpdatedAt: time.Now().Format(time.RFC3339),
		FileSize:  src.Size(),
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

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	if strings.Index(root, "/") == 0 {
		root = root[1:]
	}
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[0 : len(root)-1]
	}
	fmt.Printf("NewFs(\"%s\")\n", root)
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	httpclient := fshttp.NewClient(ctx)
	client := rest.NewClient(httpclient)
	client.SetRoot(rootURL)
	client.SetHeader("Authorization", "Bearer "+opt.ApiKey)
	resp, err := client.Call(ctx, &rest.Opts{Path: "/api/user/api-key/organisation"})
	if err != nil {
		return nil, err
	}
	organisationIdBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		OrganisationId: string(organisationIdBytes),
		HttpClient:     httpclient,
		Client:         client,
		PackageIdMap:   make(map[string]string),
		RootPath:       root,
	}
	fmt.Println(f)

	return f, nil
}
