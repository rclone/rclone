package fichier

import (
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/dircache"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	rootID        = "0"
	rootURL       = "https://api.1fichier.com/v1"
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "fichier",
		Description: "1Fichier",
		Config: func(name string, config configmap.Mapper) {
		},
		NewFs: NewFs,
		Options: []fs.Option{
			{
				Name:       "api_key",
				IsPassword: true,
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ApiKey string `config:"api_key"`
}

type Fs struct {
	root       string
	name       string
	dirCache   *dircache.DirCache
	baseClient *http.Client
	options    *Options
	pacer      *pacer.Pacer
	rest       *rest.Client
}

func (f *Fs) RoundTrip(request *http.Request) (response *http.Response, err error) {
	request.Header.Add("Authorization", "Bearer "+f.options.ApiKey)

	return f.baseClient.Do(request)
}

func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	_, err = f.listFolders(pathID)

	if err != nil {
		return "", false, err
	}

	return pathIDOut, found, err
}

func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	panic("implement me")
}

func (f *Fs) Name() string {
	return f.name
}

func (f *Fs) Root() string {
	return f.root
}

func (f *Fs) String() string {
	panic("implement me")
}

func (f *Fs) Precision() time.Duration {
	panic("implement me")
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

func (f *Fs) Features() *fs.Features {
	panic("implement me")
}

func folderValue(id string) url.Values {
	values := url.Values{}
	values.Set("folder_id", id)
	return values
}

func (f *Fs) listFiles(dir string) (entries fs.DirEntries, err error) {
	err = f.dirCache.FindRoot(false)
	if err != nil {
		return nil, err
	}

	directoryID, err := f.dirCache.FindDir(dir, false)
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/file/ls.cgi",
		Parameters: folderValue(directoryID),
	}

	var filesList FilesList
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rest.CallJSON(&opts, nil, &filesList)
		return true, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list files")
	}

	for i := range filesList.Items {
		filesList.Items[i].Path = dir
	}

	entries = make([]fs.DirEntry, len(filesList.Items))

	for i := range entries {
		entries[i] = &FichierObject{
			fs:   f,
			file: &filesList.Items[i],
		}
	}

	return entries, nil
}

func (f *Fs) listFolders(dir string) (entries fs.DirEntries, err error) {
	err = f.dirCache.FindRoot(false)
	if err != nil {
		return nil, err
	}

	directoryID, err := f.dirCache.FindDir(dir, false)
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/folder/ls.cgi",
		Parameters: folderValue(directoryID),
	}

	var foldersList FoldersList
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rest.CallJSON(&opts, nil, &foldersList)
		return true, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list files")
	}

	for i, folder := range foldersList.SubFolders {
		fullPath := folder.Name
		if dir != "" {
			fullPath = dir + "/" + fullPath
		}

		f.dirCache.Put(fullPath, strconv.Itoa(folder.ID))
		foldersList.SubFolders[i].Path = dir
	}

	entries = make([]fs.DirEntry, len(foldersList.SubFolders))

	for i, folder := range foldersList.SubFolders {
		createDate, err := time.Parse("2006-01-02 15:04:05", folder.CreateDate)

		if err != nil {
			return nil, err
		}

		fullPath := folder.Name
		if folder.Path != "" {
			fullPath = folder.Path + "/" + fullPath
		}
		entries[i] = fs.NewDir(fullPath, createDate)
	}

	return entries, nil
}

func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	files, err := f.listFiles(dir)

	if err != nil {
		return nil, err
	}

	folders, err := f.listFolders(dir)

	if err != nil {
		return nil, err
	}

	dirEntries := append(files, folders...)

	return dirEntries, nil
}

func (f *Fs) NewObject(remote string) (fs.Object, error) {
	panic("implement me")
}

func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	panic("implement me")
}

func (f *Fs) Mkdir(dir string) error {
	panic("implement me")
}

func (f *Fs) Rmdir(dir string) error {
	panic("implement me")
}

func NewFs(name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(config, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:       name,
		root:       root,
		options:    opt,
		pacer:      pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
		baseClient: &http.Client{},
	}

	f.rest = rest.NewClient(
		&http.Client{
			Transport: f,
		}).SetRoot(rootURL)

	f.dirCache = dircache.New(root, rootID, f)

	return f, nil
}

type FichierObject struct {
	fs   *Fs
	file *File
}

type File struct {
	Acl         int    `json:"acl"`
	Cdn         int    `json:"cdn"`
	Checksum    string `json:"checksum"`
	ContentType string `json:"content-type"`
	Date        string `json:"date"`
	Filename    string `json:"filename"`
	Path        string
	Pass        int    `json:"pass"`
	Size        int    `json:"size"`
	Url         string `json:"url"`
}

type FilesList struct {
	Items  []File `json:"items"`
	Status string `json:"status"`
}

type Folder struct {
	CreateDate string `json:"create_date"`
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Path       string
	Pass       string `json:"pass"`
}

type FoldersList struct {
	FolderID   string   `json:"folder_id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	SubFolders []Folder `json:"sub_folders"`
}

func (f *FichierObject) String() string {
	return f.file.Filename
}

func (f *FichierObject) Remote() string {
	if f.file.Path == "" {
		return f.file.Filename
	}

	return f.file.Path + "/" + f.file.Filename
}

func (f *FichierObject) ModTime() time.Time {
	modTime, err := time.Parse("", f.file.Date)

	if err != nil {
		return time.Now()
	}

	return modTime
}

func (f *FichierObject) Size() int64 {
	return int64(f.file.Size)
}

func (f *FichierObject) Fs() fs.Info {
	return f.fs
}

func (f *FichierObject) Hash(hash.Type) (string, error) {
	return f.file.Checksum, nil
}

func (f *FichierObject) Storable() bool {
	return false
}

func (f *FichierObject) SetModTime(time.Time) error {
	return nil
}

type Bla struct {
	url     string
	status  string
	message string
}

func urlValue(id string) url.Values {
	values := url.Values{}
	values.Set("url", id)
	return values
}

func (f *Fs) getDownloadLink(url string) (authUrl string, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/download/get_token.cgi",
		Parameters: urlValue(url),
	}

	var token Bla
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rest.CallJSON(&opts, nil, &token)
		return true, nil
	})
	if err != nil {
		return "", errors.Wrap(err, "couldn't list files")
	}

	return token.url, nil
}

func (f *FichierObject) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	downloadLink, err := f.fs.getDownloadLink(f.file.Url)

	if err != nil {
		return nil, err
	}

	res, err := f.fs.baseClient.Get(downloadLink)
	err = statusError(res, err)
	if err != nil {
		return nil, errors.Wrap(err, "Open failed")
	}
	return res.Body, nil
}

func statusError(res *http.Response, err error) error {
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		_ = res.Body.Close()
		return errors.Errorf("HTTP Error %d: %s", res.StatusCode, res.Status)
	}
	return nil
}

func (f *FichierObject) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return nil
}

func (f *FichierObject) Remove() error {
	return nil
}
