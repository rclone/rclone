package fichier

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/rest"
)

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	403, // Forbidden (may happen when request limit is exceeded)
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(resp *http.Response, err error) (bool, error) {
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

var isAlphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString

func (f *Fs) getDownloadToken(ctx context.Context, url string) (*GetTokenResponse, error) {
	request := DownloadRequest{
		URL:    url,
		Single: 1,
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/download/get_token.cgi",
	}

	var token GetTokenResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, &token)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list files")
	}

	return &token, nil
}

func fileFromSharedFile(file *SharedFile) File {
	return File{
		URL:      file.Link,
		Filename: file.Filename,
		Size:     file.Size,
	}
}

func (f *Fs) listSharedFiles(ctx context.Context, id string) (entries fs.DirEntries, err error) {
	opts := rest.Opts{
		Method:     "GET",
		RootURL:    "https://1fichier.com/dir/",
		Path:       id,
		Parameters: map[string][]string{"json": {"1"}},
	}

	var sharedFiles SharedFolderResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, &sharedFiles)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list files")
	}

	entries = make([]fs.DirEntry, len(sharedFiles))

	for i, sharedFile := range sharedFiles {
		entries[i] = f.newObjectFromFile(ctx, "", fileFromSharedFile(&sharedFile))
	}

	return entries, nil
}

func (f *Fs) listFiles(ctx context.Context, directoryID int) (filesList *FilesList, err error) {
	// fs.Debugf(f, "Requesting files for dir `%s`", directoryID)
	request := ListFilesRequest{
		FolderID: directoryID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/file/ls.cgi",
	}

	filesList = &FilesList{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, filesList)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list files")
	}
	for i := range filesList.Items {
		item := &filesList.Items[i]
		item.Filename = f.opt.Enc.ToStandardName(item.Filename)
	}

	return filesList, nil
}

func (f *Fs) listFolders(ctx context.Context, directoryID int) (foldersList *FoldersList, err error) {
	// fs.Debugf(f, "Requesting folders for id `%s`", directoryID)

	request := ListFolderRequest{
		FolderID: directoryID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/folder/ls.cgi",
	}

	foldersList = &FoldersList{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, foldersList)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list folders")
	}
	foldersList.Name = f.opt.Enc.ToStandardName(foldersList.Name)
	for i := range foldersList.SubFolders {
		folder := &foldersList.SubFolders[i]
		folder.Name = f.opt.Enc.ToStandardName(folder.Name)
	}

	// fs.Debugf(f, "Got FoldersList for id `%s`", directoryID)

	return foldersList, err
}

func (f *Fs) listDir(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	folderID, err := strconv.Atoi(directoryID)
	if err != nil {
		return nil, err
	}

	files, err := f.listFiles(ctx, folderID)
	if err != nil {
		return nil, err
	}

	folders, err := f.listFolders(ctx, folderID)
	if err != nil {
		return nil, err
	}

	entries = make([]fs.DirEntry, len(files.Items)+len(folders.SubFolders))

	for i, item := range files.Items {
		entries[i] = f.newObjectFromFile(ctx, dir, item)
	}

	for i, folder := range folders.SubFolders {
		createDate, err := time.Parse("2006-01-02 15:04:05", folder.CreateDate)
		if err != nil {
			return nil, err
		}

		fullPath := getRemote(dir, folder.Name)
		folderID := strconv.Itoa(folder.ID)

		entries[len(files.Items)+i] = fs.NewDir(fullPath, createDate).SetID(folderID)

		// fs.Debugf(f, "Put Path `%s` for id `%d` into dircache", fullPath, folder.ID)
		f.dirCache.Put(fullPath, folderID)
	}

	return entries, nil
}

func (f *Fs) newObjectFromFile(ctx context.Context, dir string, item File) *Object {
	return &Object{
		fs:     f,
		remote: getRemote(dir, item.Filename),
		file:   item,
	}
}

func getRemote(dir, fileName string) string {
	if dir == "" {
		return fileName
	}

	return dir + "/" + fileName
}

func (f *Fs) makeFolder(ctx context.Context, leaf string, folderID int) (response *MakeFolderResponse, err error) {
	name := f.opt.Enc.FromStandardName(leaf)
	// fs.Debugf(f, "Creating folder `%s` in id `%s`", name, directoryID)

	request := MakeFolderRequest{
		FolderID: folderID,
		Name:     name,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/folder/mkdir.cgi",
	}

	response = &MakeFolderResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, &request, response)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create folder")
	}

	// fs.Debugf(f, "Created Folder `%s` in id `%s`", name, directoryID)

	return response, err
}

func (f *Fs) removeFolder(ctx context.Context, name string, folderID int) (response *GenericOKResponse, err error) {
	// fs.Debugf(f, "Removing folder with id `%s`", directoryID)

	request := &RemoveFolderRequest{
		FolderID: folderID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/folder/rm.cgi",
	}

	response = &GenericOKResponse{}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.rest.CallJSON(ctx, &opts, request, response)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't remove folder")
	}
	if response.Status != "OK" {
		return nil, errors.New("Can't remove non-empty dir")
	}

	// fs.Debugf(f, "Removed Folder with id `%s`", directoryID)

	return response, nil
}

func (f *Fs) deleteFile(ctx context.Context, url string) (response *GenericOKResponse, err error) {
	request := &RemoveFileRequest{
		Files: []RmFile{
			{url},
		},
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/file/rm.cgi",
	}

	response = &GenericOKResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, request, response)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, errors.Wrap(err, "couldn't remove file")
	}

	// fs.Debugf(f, "Removed file with url `%s`", url)

	return response, nil
}

func (f *Fs) getUploadNode(ctx context.Context) (response *GetUploadNodeResponse, err error) {
	// fs.Debugf(f, "Requesting Upload node")

	opts := rest.Opts{
		Method:      "GET",
		ContentType: "application/json", // 1Fichier API is bad
		Path:        "/upload/get_upload_server.cgi",
	}

	response = &GetUploadNodeResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, response)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "didnt got an upload node")
	}

	// fs.Debugf(f, "Got Upload node")

	return response, err
}

func (f *Fs) uploadFile(ctx context.Context, in io.Reader, size int64, fileName, folderID, uploadID, node string, options ...fs.OpenOption) (response *http.Response, err error) {
	// fs.Debugf(f, "Uploading File `%s`", fileName)

	fileName = f.opt.Enc.FromStandardName(fileName)

	if len(uploadID) > 10 || !isAlphaNumeric(uploadID) {
		return nil, errors.New("Invalid UploadID")
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/upload.cgi",
		Parameters: map[string][]string{
			"id": {uploadID},
		},
		NoResponse:           true,
		Body:                 in,
		ContentLength:        &size,
		Options:              options,
		MultipartContentName: "file[]",
		MultipartFileName:    fileName,
		MultipartParams: map[string][]string{
			"did": {folderID},
		},
	}

	if node != "" {
		opts.RootURL = "https://" + node
	}

	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, nil)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, errors.Wrap(err, "couldn't upload file")
	}

	// fs.Debugf(f, "Uploaded File `%s`", fileName)

	return response, err
}

func (f *Fs) endUpload(ctx context.Context, uploadID string, nodeurl string) (response *EndFileUploadResponse, err error) {
	// fs.Debugf(f, "Ending File Upload `%s`", uploadID)

	if len(uploadID) > 10 || !isAlphaNumeric(uploadID) {
		return nil, errors.New("Invalid UploadID")
	}

	opts := rest.Opts{
		Method:  "GET",
		Path:    "/end.pl",
		RootURL: "https://" + nodeurl,
		Parameters: map[string][]string{
			"xid": {uploadID},
		},
		ExtraHeaders: map[string]string{
			"JSON": "1",
		},
	}

	response = &EndFileUploadResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.rest.CallJSON(ctx, &opts, nil, response)
		return shouldRetry(resp, err)
	})

	if err != nil {
		return nil, errors.Wrap(err, "couldn't finish file upload")
	}

	return response, err
}
