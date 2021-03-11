package seafile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/seafile/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
)

// Start of the API URLs
const (
	APIv20 = "api2/repos/"
	APIv21 = "api/v2.1/repos/"
)

// Errors specific to seafile fs
var (
	ErrorInternalDuringUpload = errors.New("Internal server error during file upload")
)

// ==================== Seafile API ====================

func (f *Fs) getAuthorizationToken(ctx context.Context) (string, error) {
	return getAuthorizationToken(ctx, f.srv, f.opt.User, f.opt.Password, "")
}

// getAuthorizationToken can be called outside of an fs (during configuration of the remote to get the authentication token)
// it's doing a single call (no pacer involved)
func getAuthorizationToken(ctx context.Context, srv *rest.Client, user, password, oneTimeCode string) (string, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/home.md#user-content-Quick%20Start
	opts := rest.Opts{
		Method:       "POST",
		Path:         "api2/auth-token/",
		ExtraHeaders: map[string]string{"Authorization": ""}, // unset the Authorization for this request
		IgnoreStatus: true,                                   // so we can load the error messages back into result
	}

	// 2FA
	if oneTimeCode != "" {
		opts.ExtraHeaders["X-SEAFILE-OTP"] = oneTimeCode
	}

	request := api.AuthenticationRequest{
		Username: user,
		Password: password,
	}
	result := api.AuthenticationResult{}

	_, err := srv.CallJSON(ctx, &opts, &request, &result)
	if err != nil {
		// This is only going to be http errors here
		return "", errors.Wrap(err, "failed to authenticate")
	}
	if result.Errors != nil && len(result.Errors) > 0 {
		return "", errors.New(strings.Join(result.Errors, ", "))
	}
	if result.Token == "" {
		// No error in "non_field_errors" field but still empty token
		return "", errors.New("failed to authenticate")
	}
	return result.Token, nil
}

func (f *Fs) getServerInfo(ctx context.Context) (account *api.ServerInfo, err error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/server-info.md#user-content-Get%20Server%20Information
	opts := rest.Opts{
		Method: "GET",
		Path:   "api2/server-info/",
	}

	result := api.ServerInfo{}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to get server info")
	}
	return &result, nil
}

func (f *Fs) getUserAccountInfo(ctx context.Context) (account *api.AccountInfo, err error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/account.md#user-content-Check%20Account%20Info
	opts := rest.Opts{
		Method: "GET",
		Path:   "api2/account/info/",
	}

	result := api.AccountInfo{}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to get account info")
	}
	return &result, nil
}

func (f *Fs) getLibraries(ctx context.Context) ([]api.Library, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/libraries.md#user-content-List%20Libraries
	opts := rest.Opts{
		Method: "GET",
		Path:   APIv20,
	}

	result := make([]api.Library, 1)

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to get libraries")
	}
	return result, nil
}

func (f *Fs) createLibrary(ctx context.Context, libraryName, password string) (library *api.CreateLibrary, err error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/libraries.md#user-content-Create%20Library
	opts := rest.Opts{
		Method: "POST",
		Path:   APIv20,
	}

	request := api.CreateLibraryRequest{
		Name:        f.opt.Enc.FromStandardName(libraryName),
		Description: "Created by rclone",
		Password:    password,
	}
	result := &api.CreateLibrary{}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to create library")
	}
	return result, nil
}

func (f *Fs) deleteLibrary(ctx context.Context, libraryID string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/libraries.md#user-content-Create%20Library
	opts := rest.Opts{
		Method: "DELETE",
		Path:   APIv20 + libraryID + "/",
	}

	result := ""

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
		}
		return errors.Wrap(err, "failed to delete library")
	}
	return nil
}

func (f *Fs) decryptLibrary(ctx context.Context, libraryID, password string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/library-encryption.md#user-content-Decrypt%20Library
	if libraryID == "" {
		return errors.New("cannot list files without a library")
	}
	// This is another call that cannot accept a JSON input so we have to build it manually
	opts := rest.Opts{
		Method:      "POST",
		Path:        APIv20 + libraryID + "/",
		ContentType: "application/x-www-form-urlencoded",
		Body:        bytes.NewBuffer([]byte("password=" + f.opt.Enc.FromStandardName(password))),
		NoResponse:  true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 400 {
				return errors.New("incorrect password")
			}
			if resp.StatusCode == 409 {
				fs.Debugf(nil, "library is not encrypted")
				return nil
			}
		}
		return errors.Wrap(err, "failed to decrypt library")
	}
	return nil
}

func (f *Fs) getDirectoryEntriesAPIv21(ctx context.Context, libraryID, dirPath string, recursive bool) ([]api.DirEntry, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/directories.md#user-content-List%20Items%20in%20Directory
	// This is using the undocumented version 2.1 of the API (so we can use the recursive option which is not available in the version 2)
	if libraryID == "" {
		return nil, errors.New("cannot list files without a library")
	}
	dirPath = path.Join("/", dirPath)

	recursiveFlag := "0"
	if recursive {
		recursiveFlag = "1"
	}
	opts := rest.Opts{
		Method: "GET",
		Path:   APIv21 + libraryID + "/dir/",
		Parameters: url.Values{
			"recursive": {recursiveFlag},
			"p":         {f.opt.Enc.FromStandardPath(dirPath)},
		},
	}
	result := &api.DirEntries{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return nil, fs.ErrorDirNotFound
			}
			if resp.StatusCode == 440 {
				// Encrypted library and password not provided
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to get directory contents")
	}

	// Clean up encoded names
	for index, fileInfo := range result.Entries {
		fileInfo.Name = f.opt.Enc.ToStandardName(fileInfo.Name)
		fileInfo.Path = f.opt.Enc.ToStandardPath(fileInfo.Path)
		result.Entries[index] = fileInfo
	}
	return result.Entries, nil
}

func (f *Fs) getDirectoryDetails(ctx context.Context, libraryID, dirPath string) (*api.DirectoryDetail, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/directories.md#user-content-Get%20Directory%20Detail
	if libraryID == "" {
		return nil, errors.New("cannot read directory without a library")
	}
	dirPath = path.Join("/", dirPath)

	opts := rest.Opts{
		Method:     "GET",
		Path:       APIv21 + libraryID + "/dir/detail/",
		Parameters: url.Values{"path": {f.opt.Enc.FromStandardPath(dirPath)}},
	}
	result := &api.DirectoryDetail{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return nil, fs.ErrorDirNotFound
			}
		}
		return nil, errors.Wrap(err, "failed to get directory details")
	}
	result.Name = f.opt.Enc.ToStandardName(result.Name)
	result.Path = f.opt.Enc.ToStandardPath(result.Path)
	return result, nil
}

// createDir creates a new directory. The API will add a number to the directory name if it already exist
func (f *Fs) createDir(ctx context.Context, libraryID, dirPath string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/directories.md#user-content-Create%20New%20Directory
	if libraryID == "" {
		return errors.New("cannot create directory without a library")
	}
	dirPath = path.Join("/", dirPath)

	// This call *cannot* handle json parameters in the body, so we have to build the request body manually
	opts := rest.Opts{
		Method:      "POST",
		Path:        APIv20 + libraryID + "/dir/",
		Parameters:  url.Values{"p": {f.opt.Enc.FromStandardPath(dirPath)}},
		NoRedirect:  true,
		ContentType: "application/x-www-form-urlencoded",
		Body:        bytes.NewBuffer([]byte("operation=mkdir")),
		NoResponse:  true,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
		}
		return errors.Wrap(err, "failed to create directory")
	}
	return nil
}

func (f *Fs) renameDir(ctx context.Context, libraryID, dirPath, newName string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/directories.md#user-content-Rename%20Directory
	if libraryID == "" {
		return errors.New("cannot rename directory without a library")
	}
	dirPath = path.Join("/", dirPath)

	// This call *cannot* handle json parameters in the body, so we have to build the request body manually
	postParameters := url.Values{
		"operation": {"rename"},
		"newname":   {f.opt.Enc.FromStandardPath(newName)},
	}

	opts := rest.Opts{
		Method:      "POST",
		Path:        APIv20 + libraryID + "/dir/",
		Parameters:  url.Values{"p": {f.opt.Enc.FromStandardPath(dirPath)}},
		ContentType: "application/x-www-form-urlencoded",
		Body:        bytes.NewBuffer([]byte(postParameters.Encode())),
		NoResponse:  true,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
		}
		return errors.Wrap(err, "failed to rename directory")
	}
	return nil
}

func (f *Fs) moveDir(ctx context.Context, srcLibraryID, srcDir, srcName, dstLibraryID, dstPath string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/files-directories-batch-op.md#user-content-Batch%20Move%20Items%20Synchronously
	if srcLibraryID == "" || dstLibraryID == "" || srcName == "" {
		return errors.New("libraryID and/or file path argument(s) missing")
	}
	srcDir = path.Join("/", srcDir)
	dstPath = path.Join("/", dstPath)

	opts := rest.Opts{
		Method:     "POST",
		Path:       APIv21 + "sync-batch-move-item/",
		NoResponse: true,
	}

	request := &api.BatchSourceDestRequest{
		SrcLibraryID: srcLibraryID,
		SrcParentDir: f.opt.Enc.FromStandardPath(srcDir),
		SrcItems:     []string{f.opt.Enc.FromStandardPath(srcName)},
		DstLibraryID: dstLibraryID,
		DstParentDir: f.opt.Enc.FromStandardPath(dstPath),
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return fs.ErrorObjectNotFound
			}
		}
		return errors.Wrap(err, fmt.Sprintf("failed to move directory '%s' from '%s' to '%s'", srcName, srcDir, dstPath))
	}

	return nil
}

func (f *Fs) deleteDir(ctx context.Context, libraryID, filePath string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/directories.md#user-content-Delete%20Directory
	if libraryID == "" {
		return errors.New("cannot delete directory without a library")
	}
	filePath = path.Join("/", filePath)

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       APIv20 + libraryID + "/dir/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(filePath)}},
		NoResponse: true,
	}

	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
		}
		return errors.Wrap(err, "failed to delete directory")
	}
	return nil
}

func (f *Fs) getFileDetails(ctx context.Context, libraryID, filePath string) (*api.FileDetail, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Get%20File%20Detail
	if libraryID == "" {
		return nil, errors.New("cannot open file without a library")
	}
	filePath = path.Join("/", filePath)

	opts := rest.Opts{
		Method:     "GET",
		Path:       APIv20 + libraryID + "/file/detail/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(filePath)}},
	}
	result := &api.FileDetail{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 404 {
				return nil, fs.ErrorObjectNotFound
			}
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to get file details")
	}
	result.Name = f.opt.Enc.ToStandardName(result.Name)
	result.Parent = f.opt.Enc.ToStandardPath(result.Parent)
	return result, nil
}

func (f *Fs) deleteFile(ctx context.Context, libraryID, filePath string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Delete%20File
	if libraryID == "" {
		return errors.New("cannot delete file without a library")
	}
	filePath = path.Join("/", filePath)

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       APIv20 + libraryID + "/file/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(filePath)}},
		NoResponse: true,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete file")
	}
	return nil
}

func (f *Fs) getDownloadLink(ctx context.Context, libraryID, filePath string) (string, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Download%20File
	if libraryID == "" {
		return "", errors.New("cannot download file without a library")
	}
	filePath = path.Join("/", filePath)

	opts := rest.Opts{
		Method:     "GET",
		Path:       APIv20 + libraryID + "/file/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(filePath)}},
	}
	result := ""
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 404 {
				return "", fs.ErrorObjectNotFound
			}
		}
		return "", errors.Wrap(err, "failed to get download link")
	}
	return result, nil
}

func (f *Fs) download(ctx context.Context, url string, size int64, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Check if we need to download partial content
	var start, end int64 = 0, size
	partialContent := false
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			start = x.Offset
			partialContent = true
		case *fs.RangeOption:
			if x.Start >= 0 {
				start = x.Start
				if x.End > 0 && x.End < size {
					end = x.End + 1
				}
			} else {
				// {-1, 20} should load the last 20 characters [len-20:len]
				start = size - x.End
			}
			partialContent = true
		default:
			if option.Mandatory() {
				fs.Logf(nil, "Unsupported mandatory option: %v", option)
			}
		}
	}
	// Build the http request
	opts := rest.Opts{
		Method:  "GET",
		RootURL: url,
		Options: options,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 404 {
				return nil, fmt.Errorf("file not found '%s'", url)
			}
		}
		return nil, err
	}
	// Non-encrypted libraries are accepting the HTTP Range header,
	// BUT encrypted libraries are simply ignoring it
	if partialContent && resp.StatusCode == 200 {
		// Partial content was requested through a Range header, but a full content was sent instead
		rangeDownloadNotice.Do(func() {
			fs.Logf(nil, "%s ignored our request of partial content. This is probably because encrypted libraries are not accepting range requests. Loading this file might be slow!", f.String())
		})
		if start > 0 {
			// We need to read and discard the beginning of the data...
			_, err = io.CopyN(ioutil.Discard, resp.Body, start)
			if err != nil {
				return nil, err
			}
		}
		// ... and return a limited reader for the remaining of the data
		return readers.NewLimitedReadCloser(resp.Body, end-start), nil
	}
	return resp.Body, nil
}

func (f *Fs) getUploadLink(ctx context.Context, libraryID string) (string, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file-upload.md
	if libraryID == "" {
		return "", errors.New("cannot upload file without a library")
	}
	opts := rest.Opts{
		Method: "GET",
		Path:   APIv20 + libraryID + "/upload-link/",
	}
	result := ""
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return "", fs.ErrorPermissionDenied
			}
		}
		return "", errors.Wrap(err, "failed to get upload link")
	}
	return result, nil
}

func (f *Fs) upload(ctx context.Context, in io.Reader, uploadLink, filePath string) (*api.FileDetail, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file-upload.md
	fileDir, filename := path.Split(filePath)
	parameters := url.Values{
		"parent_dir":        {"/"},
		"relative_path":     {f.opt.Enc.FromStandardPath(fileDir)},
		"need_idx_progress": {"true"},
		"replace":           {"1"},
	}
	formReader, contentType, _, err := rest.MultipartUpload(in, parameters, "file", f.opt.Enc.FromStandardName(filename))
	if err != nil {
		return nil, errors.Wrap(err, "failed to make multipart upload")
	}

	opts := rest.Opts{
		Method:      "POST",
		RootURL:     uploadLink,
		Body:        formReader,
		ContentType: contentType,
		Parameters:  url.Values{"ret-json": {"1"}}, // It needs to be on the url, not in the body parameters
	}
	result := make([]api.FileDetail, 1)
	var resp *http.Response
	// If an error occurs during the call, do not attempt to retry: The upload link is single use only
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetryUpload(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 500 {
				// This is a temporary error - we will get a new upload link before retrying
				return nil, ErrorInternalDuringUpload
			}
		}
		return nil, errors.Wrap(err, "failed to upload file")
	}
	if len(result) > 0 {
		result[0].Parent = f.opt.Enc.ToStandardPath(result[0].Parent)
		result[0].Name = f.opt.Enc.ToStandardName(result[0].Name)
		return &result[0], nil
	}
	return nil, nil
}

func (f *Fs) listShareLinks(ctx context.Context, libraryID, remote string) ([]api.SharedLink, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/share-links.md#user-content-List%20Share%20Link%20of%20a%20Folder%20(File)
	if libraryID == "" {
		return nil, errors.New("cannot get share links without a library")
	}
	remote = path.Join("/", remote)

	opts := rest.Opts{
		Method:     "GET",
		Path:       "api/v2.1/share-links/",
		Parameters: url.Values{"repo_id": {libraryID}, "path": {f.opt.Enc.FromStandardPath(remote)}},
	}
	result := make([]api.SharedLink, 1)
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, errors.Wrap(err, "failed to list shared links")
	}
	return result, nil
}

// createShareLink will only work with non-encrypted libraries
func (f *Fs) createShareLink(ctx context.Context, libraryID, remote string) (*api.SharedLink, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/share-links.md#user-content-Create%20Share%20Link
	if libraryID == "" {
		return nil, errors.New("cannot create a shared link without a library")
	}
	remote = path.Join("/", remote)

	opts := rest.Opts{
		Method: "POST",
		Path:   "api/v2.1/share-links/",
	}
	request := &api.ShareLinkRequest{
		LibraryID: libraryID,
		Path:      f.opt.Enc.FromStandardPath(remote),
	}
	result := &api.SharedLink{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, errors.Wrap(err, "failed to create a shared link")
	}
	return result, nil
}

func (f *Fs) copyFile(ctx context.Context, srcLibraryID, srcPath, dstLibraryID, dstPath string) (*api.FileInfo, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Copy%20File
	// It's using the api/v2.1 which is not in the documentation (as of Apr 2020) but works better than api2
	if srcLibraryID == "" || dstLibraryID == "" {
		return nil, errors.New("libraryID and/or file path argument(s) missing")
	}
	srcPath = path.Join("/", srcPath)
	dstPath = path.Join("/", dstPath)

	opts := rest.Opts{
		Method:     "POST",
		Path:       APIv21 + srcLibraryID + "/file/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(srcPath)}},
	}
	request := &api.FileOperationRequest{
		Operation:            api.CopyFileOperation,
		DestinationLibraryID: dstLibraryID,
		DestinationPath:      f.opt.Enc.FromStandardPath(dstPath),
	}
	result := &api.FileInfo{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				fs.Debugf(nil, "Copy: %s", err)
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, errors.Wrap(err, fmt.Sprintf("failed to copy file %s:'%s' to %s:'%s'", srcLibraryID, srcPath, dstLibraryID, dstPath))
	}
	return f.decodeFileInfo(result), nil
}

func (f *Fs) moveFile(ctx context.Context, srcLibraryID, srcPath, dstLibraryID, dstPath string) (*api.FileInfo, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Move%20File
	// It's using the api/v2.1 which is not in the documentation (as of Apr 2020) but works better than api2
	if srcLibraryID == "" || dstLibraryID == "" {
		return nil, errors.New("libraryID and/or file path argument(s) missing")
	}
	srcPath = path.Join("/", srcPath)
	dstPath = path.Join("/", dstPath)

	opts := rest.Opts{
		Method:     "POST",
		Path:       APIv21 + srcLibraryID + "/file/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(srcPath)}},
	}
	request := &api.FileOperationRequest{
		Operation:            api.MoveFileOperation,
		DestinationLibraryID: dstLibraryID,
		DestinationPath:      f.opt.Enc.FromStandardPath(dstPath),
	}
	result := &api.FileInfo{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				fs.Debugf(nil, "Move: %s", err)
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, errors.Wrap(err, fmt.Sprintf("failed to move file %s:'%s' to %s:'%s'", srcLibraryID, srcPath, dstLibraryID, dstPath))
	}
	return f.decodeFileInfo(result), nil
}

func (f *Fs) renameFile(ctx context.Context, libraryID, filePath, newname string) (*api.FileInfo, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Rename%20File
	// It's using the api/v2.1 which is not in the documentation (as of Apr 2020) but works better than api2
	if libraryID == "" || newname == "" {
		return nil, errors.New("libraryID and/or file path argument(s) missing")
	}
	filePath = path.Join("/", filePath)

	opts := rest.Opts{
		Method:     "POST",
		Path:       APIv21 + libraryID + "/file/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(filePath)}},
	}
	request := &api.FileOperationRequest{
		Operation: api.RenameFileOperation,
		NewName:   f.opt.Enc.FromStandardName(newname),
	}
	result := &api.FileInfo{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				fs.Debugf(nil, "Rename: %s", err)
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, errors.Wrap(err, fmt.Sprintf("failed to rename file '%s' to '%s'", filePath, newname))
	}
	return f.decodeFileInfo(result), nil
}

func (f *Fs) decodeFileInfo(input *api.FileInfo) *api.FileInfo {
	input.Name = f.opt.Enc.ToStandardName(input.Name)
	input.Path = f.opt.Enc.ToStandardPath(input.Path)
	return input
}

func (f *Fs) emptyLibraryTrash(ctx context.Context, libraryID string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/libraries.md#user-content-Clean%20Library%20Trash
	if libraryID == "" {
		return errors.New("cannot clean up trash without a library")
	}
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       APIv21 + libraryID + "/trash/",
		NoResponse: true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return fs.ErrorObjectNotFound
			}
		}
		return errors.Wrap(err, "failed empty the library trash")
	}
	return nil
}

// === API v2 from the official documentation, but that have been replaced by the much better v2.1 (undocumented as of Apr 2020)
// === getDirectoryEntriesAPIv2 is needed to keep compatibility with seafile v6,
// === the others can probably be removed after the API v2.1 is documented

func (f *Fs) getDirectoryEntriesAPIv2(ctx context.Context, libraryID, dirPath string) ([]api.DirEntry, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/directories.md#user-content-List%20Items%20in%20Directory
	if libraryID == "" {
		return nil, errors.New("cannot list files without a library")
	}
	dirPath = path.Join("/", dirPath)

	opts := rest.Opts{
		Method:     "GET",
		Path:       APIv20 + libraryID + "/dir/",
		Parameters: url.Values{"p": {f.opt.Enc.FromStandardPath(dirPath)}},
	}
	result := make([]api.DirEntry, 1)
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return nil, fs.ErrorDirNotFound
			}
			if resp.StatusCode == 440 {
				// Encrypted library and password not provided
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, "failed to get directory contents")
	}

	// Clean up encoded names
	for index, fileInfo := range result {
		fileInfo.Name = f.opt.Enc.ToStandardName(fileInfo.Name)
		fileInfo.Path = f.opt.Enc.ToStandardPath(fileInfo.Path)
		result[index] = fileInfo
	}
	return result, nil
}

func (f *Fs) copyFileAPIv2(ctx context.Context, srcLibraryID, srcPath, dstLibraryID, dstPath string) (*api.FileInfo, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Copy%20File
	if srcLibraryID == "" || dstLibraryID == "" {
		return nil, errors.New("libraryID and/or file path argument(s) missing")
	}
	srcPath = path.Join("/", srcPath)
	dstPath = path.Join("/", dstPath)

	// Older API does not seem to accept JSON input here either
	postParameters := url.Values{
		"operation": {"copy"},
		"dst_repo":  {dstLibraryID},
		"dst_dir":   {f.opt.Enc.FromStandardPath(dstPath)},
	}
	opts := rest.Opts{
		Method:      "POST",
		Path:        APIv20 + srcLibraryID + "/file/",
		Parameters:  url.Values{"p": {f.opt.Enc.FromStandardPath(srcPath)}},
		ContentType: "application/x-www-form-urlencoded",
		Body:        bytes.NewBuffer([]byte(postParameters.Encode())),
	}
	result := &api.FileInfo{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
		}
		return nil, errors.Wrap(err, fmt.Sprintf("failed to copy file %s:'%s' to %s:'%s'", srcLibraryID, srcPath, dstLibraryID, dstPath))
	}
	err = rest.DecodeJSON(resp, &result)
	if err != nil {
		return nil, err
	}
	return f.decodeFileInfo(result), nil
}

func (f *Fs) renameFileAPIv2(ctx context.Context, libraryID, filePath, newname string) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file.md#user-content-Rename%20File
	if libraryID == "" || newname == "" {
		return errors.New("libraryID and/or file path argument(s) missing")
	}
	filePath = path.Join("/", filePath)

	// No luck with JSON input with the older api2
	postParameters := url.Values{
		"operation": {"rename"},
		"reloaddir": {"true"}, // This is an undocumented trick to avoid an http code 301 response (found in https://github.com/haiwen/seahub/blob/master/seahub/api2/views.py)
		"newname":   {f.opt.Enc.FromStandardName(newname)},
	}

	opts := rest.Opts{
		Method:      "POST",
		Path:        APIv20 + libraryID + "/file/",
		Parameters:  url.Values{"p": {f.opt.Enc.FromStandardPath(filePath)}},
		ContentType: "application/x-www-form-urlencoded",
		Body:        bytes.NewBuffer([]byte(postParameters.Encode())),
		NoRedirect:  true,
		NoResponse:  true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 301 {
				// This is the normal response from the server
				return nil
			}
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 404 {
				return fs.ErrorObjectNotFound
			}
		}
		return errors.Wrap(err, "failed to rename file")
	}
	return nil
}
