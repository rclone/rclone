package ulozto

import (
	"context"
	"fmt"
	"github.com/rclone/rclone/backend/ulozto/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/rest"
	"path"
	"strconv"
	"time"
)

type Options struct {
	RootFolderSlug string               `config:"root_folder_slug"`
	Enc            encoder.MultiEncoder `config:"encoding"`
	ListChunk      int                  `config:"list_chunk"`
	Username       string               `config:"username"`
	AuthToken      string               `config:"token"`
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "box",
		Description: "Box",
		Options: []fs.Option{
			{
				Name:      "root_folder_slug",
				Help:      "If set, rclone will use this folder as the root folder for all operations.",
				Default:   "",
				Advanced:  true,
				Sensitive: true,
			},
			{
				Name:     "list_chunk",
				Default:  500,
				Help:     "The size of a single page for list commands. 1-500",
				Advanced: true,
			},
			{
				Name:      "username",
				Default:   "",
				Help:      "The name of the logged in user.",
				Sensitive: true,
			},
			{
				Name:      "app_token",
				Default:   "",
				Help:      "app token",
				Sensitive: true,
			},
			{
				Name:      "user_token",
				Default:   "",
				Help:      "user token",
				Sensitive: true,
			},
		}})
}

type Fs struct {
	name     string             // name of this remote
	rootSlug string             // the path we are working on
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the server
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *fs.Pacer          // pacer for API calls
}

type File struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	slug        string    // ID of the object
	crc32       int32     // CRC32 of the object content
}

func (f *Fs) FindLeaf(ctx context.Context, folderSlug, leaf string) (pathIDOut string, found bool, err error) {
	folders, err := f.listFolders(ctx, folderSlug, leaf)
	if err != nil {
		return "", false, err
	}

	for _, folder := range folders {
		if folder.Name == leaf {
			return folder.Slug, true, nil
		}
	}
	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	// var resp *http.Response
	var folder *api.Folder
	opts := rest.Opts{
		Method: "POST",
		Path:   "/v6/user/" + f.opt.Username + "/folder",
	}
	mkdir := api.CreateFolderRequest{
		Name:             f.opt.Enc.FromStandardName(leaf),
		ParentFolderSlug: pathID,
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.CallJSON(ctx, &opts, &mkdir, &folder)
		return false, nil
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return folder.Slug, nil
}

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
	o := &File{
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

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.File, err error) {
	// defer log.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	// Use preupload to find the ID
	itemMini, err := f.preUploadCheck(ctx, leaf, directoryID, -1)
	if err != nil {
		return nil, err
	}
	if itemMini == nil {
		return nil, fs.ErrorObjectNotFound
	}

	// Now we have the ID we can look up the object proper
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/files/" + itemMini.ID,
		Parameters: fieldsValue(),
	}
	var item api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &item)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	folderSlug, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	folders, err := f.listFolders(ctx, folderSlug, "")
	if err != nil {
		return nil, err
	}

	for _, folder := range folders {
		remote := path.Join(dir, folder.Name)
		f.dirCache.Put(remote, folder.Slug)
		entries = append(entries, fs.NewDir(remote, folder.LastUserModified))
	}

	files, err := f.listFolders(ctx, folderSlug, "")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		remote := path.Join(dir, file.Name)
		entries = append(entries, fs.(remote, folder.LastUserModified))
	}

	return entries, nil
}

func (f *Fs) fetchListFolderPage(
	ctx context.Context,
	folderSlug string,
	searchQuery string,
	limit int,
	offset int) (folders []api.Folder, err error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/v9/user/" + f.opt.Username + " /folder/" + folderSlug + "/folder-list",
	}
	opts.Parameters.Set("limit", strconv.Itoa(limit))
	if offset > 0 {
		opts.Parameters.Set("offset", strconv.Itoa(offset))
	}

	if searchQuery != "" {
		opts.Parameters.Set("search_query", searchQuery)
	}

	var respBody *api.ListFoldersResponse

	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.CallJSON(ctx, &opts, nil, &respBody)
		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't list files: %w", err)
	}

	for i := range respBody.Subfolders {
		respBody.Subfolders[i].Name = f.opt.Enc.ToStandardName(respBody.Subfolders[i].Name)
	}

	return respBody.Subfolders, nil
}

func (f *Fs) listFolders(
	ctx context.Context,
	folderSlug string,
	searchQuery string) (folders []api.Folder, err error) {

	targetPageSize := f.opt.ListChunk
	lastPageSize := targetPageSize
	offset := 0

	for targetPageSize == lastPageSize {
		page, err := f.fetchListFolderPage(ctx, folderSlug, searchQuery, targetPageSize, offset)
		if err != nil {
			return nil, err
		}
		lastPageSize = len(page)
		offset += lastPageSize
		folders = append(folders, page...)
	}

	return folders, nil
}

func (f *Fs) fetchListFilePage(
	ctx context.Context,
	folderSlug string,
	searchQuery string,
	limit int,
	offset int) (folders []api.File, err error) {

	opts := rest.Opts{
		Method: "GET",
		Path:   "/v9/user/" + f.opt.Username + " /folder/" + folderSlug + "/file-list",
	}
	opts.Parameters.Set("limit", strconv.Itoa(limit))
	if offset > 0 {
		opts.Parameters.Set("offset", strconv.Itoa(offset))
	}

	if searchQuery != "" {
		opts.Parameters.Set("search_query", searchQuery)
	}

	var respBody *api.ListFilesResponse

	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.CallJSON(ctx, &opts, nil, &respBody)
		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't list files: %w", err)
	}

	for i := range respBody.Items {
		respBody.Items[i].Name = f.opt.Enc.ToStandardName(respBody.Items[i].Name)
	}

	return respBody.Items, nil
}

func (f *Fs) listFiles(
	ctx context.Context,
	folderSlug string,
	searchQuery string) (folders []api.File, err error) {

	targetPageSize := f.opt.ListChunk
	lastPageSize := targetPageSize
	offset := 0

	for targetPageSize == lastPageSize {
		page, err := f.fetchListFilePage(ctx, folderSlug, searchQuery, targetPageSize, offset)
		if err != nil {
			return nil, err
		}
		lastPageSize = len(page)
		offset += lastPageSize
		folders = append(folders, page...)
	}

	return folders, nil
}
