package seafile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

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
	ErrorInternalDuringUpload = errors.New("internal server error during file upload")
)

// ==================== Seafile API ====================

func (f *Fs) getAuthorizationToken(ctx context.Context) (string, error) {
	opts, request := prepareAuthorizationRequest(f.opt.User, f.opt.Password, "")
	result := api.AuthenticationResult{}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		// This is only going to be http errors here
		return "", fmt.Errorf("failed to authenticate: %w", err)
	}
	if len(result.Errors) > 0 {
		return "", errors.New(strings.Join(result.Errors, ", "))
	}
	if result.Token == "" {
		// No error in "non_field_errors" field but still empty token
		return "", errors.New("failed to authenticate")
	}
	return result.Token, nil
}

func prepareAuthorizationRequest(user, password, oneTimeCode string) (rest.Opts, api.AuthenticationRequest) {
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
	return opts, request
}

// getAuthorizationToken is called outside of an fs (during configuration of the remote to get the authentication token)
// it's doing a single call (no pacer involved)
func getAuthorizationToken(ctx context.Context, srv *rest.Client, user, password, oneTimeCode string) (string, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/home.md#user-content-Quick%20Start
	opts, request := prepareAuthorizationRequest(user, password, oneTimeCode)
	result := api.AuthenticationResult{}

	_, err := srv.CallJSON(ctx, &opts, &request, &result)
	if err != nil {
		// This is only going to be http errors here
		return "", fmt.Errorf("failed to authenticate: %w", err)
	}
	if len(result.Errors) > 0 {
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
		return nil, fmt.Errorf("failed to get server info: %w", err)
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
		return nil, fmt.Errorf("failed to get account info: %w", err)
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
		return nil, fmt.Errorf("failed to get libraries: %w", err)
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
		return nil, fmt.Errorf("failed to create library: %w", err)
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
		return fmt.Errorf("failed to delete library: %w", err)
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
		return fmt.Errorf("failed to decrypt library: %w", err)
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
		return nil, fmt.Errorf("failed to get directory contents: %w", err)
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
		return nil, fmt.Errorf("failed to get directory details: %w", err)
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
		return fmt.Errorf("failed to create directory: %w", err)
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
		return fmt.Errorf("failed to rename directory: %w", err)
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
		return fmt.Errorf("failed to move directory '%s' from '%s' to '%s': %w", srcName, srcDir, dstPath, err)
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
			if resp.StatusCode == 404 {
				return fs.ErrorDirNotFound
			}
		}
		return fmt.Errorf("failed to delete directory: %w", err)
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
		return nil, fmt.Errorf("failed to get file details: %w", err)
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
		return fmt.Errorf("failed to delete file: %w", err)
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
		return "", fmt.Errorf("failed to get download link: %w", err)
	}
	return result, nil
}

func (f *Fs) download(ctx context.Context, downloadLink string, size int64, options ...fs.OpenOption) (io.ReadCloser, error) {
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
		Options: options,
	}
	parsedURL, err := url.Parse(downloadLink)
	if err != nil {
		return nil, fmt.Errorf("failed to parse download url: %w", err)
	}
	if parsedURL.IsAbs() {
		opts.RootURL = downloadLink
	} else {
		opts.Path = downloadLink
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 404 {
				return nil, fmt.Errorf("file not found '%s'", downloadLink)
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
			_, err = io.CopyN(io.Discard, resp.Body, start)
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
		return "", fmt.Errorf("failed to get upload link: %w", err)
	}
	return result, nil
}

// getFileUploadedSize returns the size already uploaded on the server
//
//nolint:unused
func (f *Fs) getFileUploadedSize(ctx context.Context, libraryID, filePath string) (int64, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file-upload.md
	if libraryID == "" {
		return 0, errors.New("cannot get file uploaded size without a library")
	}
	fs.Debugf(nil, "filePath=%q", filePath)
	fileDir, filename := path.Split(filePath)
	fileDir = "/" + strings.TrimSuffix(fileDir, "/")
	if fileDir == "" {
		fileDir = "/"
	}
	opts := rest.Opts{
		Method: "GET",
		Path:   APIv21 + libraryID + "/file-uploaded-bytes/",
		Parameters: url.Values{
			"parent_dir": {f.opt.Enc.FromStandardPath(fileDir)},
			"file_name":  {f.opt.Enc.FromStandardPath(filename)},
		},
	}

	result := api.FileUploadedBytes{}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return 0, fs.ErrorPermissionDenied
			}
		}
		return 0, fmt.Errorf("failed to get file uploaded size for parent_dir=%q and file_name=%q: %w", fileDir, filename, err)
	}
	return result.FileUploadedBytes, nil
}

func (f *Fs) prepareFileUpload(ctx context.Context, in io.Reader, uploadLink, filePath string, contentRange contentRanger) (*rest.Opts, error) {
	fileDir, filename := path.Split(filePath)
	safeFilename := f.opt.Enc.FromStandardName(filename)
	parameters := url.Values{
		"parent_dir":        {"/"},
		"relative_path":     {f.opt.Enc.FromStandardPath(fileDir)},
		"need_idx_progress": {"true"},
		"replace":           {"1"},
	}

	contentRangeHeader := contentRange.getContentRangeHeader()
	opts := &rest.Opts{
		Method:               http.MethodPost,
		Body:                 in,
		ContentRange:         contentRangeHeader,
		Parameters:           url.Values{"ret-json": {"1"}}, // It needs to be on the url, not in the body parameters
		MultipartParams:      parameters,
		MultipartContentName: "file",
		MultipartFileName:    safeFilename,
	}
	if contentRangeHeader != "" {
		// When using resumable upload, the name of the file is no longer retrieved from the "file" field of the form.
		// It's instead retrieved from the header.
		opts.ExtraHeaders = map[string]string{
			"Content-Disposition": "attachment; filename=\"" + safeFilename + "\"",
		}
	}

	parsedURL, err := url.Parse(uploadLink)
	if err != nil {
		return nil, fmt.Errorf("failed to parse upload url: %w", err)
	}
	if parsedURL.IsAbs() {
		opts.RootURL = uploadLink
	} else {
		opts.Path = uploadLink
	}

	chunkSize := contentRange.getChunkSize()
	if chunkSize > 0 {
		// seafile might not make use of the Content-Length header but a proxy (or reverse proxy) in the middle might
		opts.ContentLength = &chunkSize
	}
	return opts, nil
}

func (f *Fs) upload(ctx context.Context, in io.Reader, uploadLink, filePath string, size int64) (*api.FileDetail, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file-upload.md
	contentRange := newStreamedContentRange(size)
	opts, err := f.prepareFileUpload(ctx, in, uploadLink, filePath, contentRange)
	if err != nil {
		return nil, err
	}

	result := make([]api.FileDetail, 1)
	var resp *http.Response
	// We do not attempt to retry if an error occurs during the call, as we don't know the state of the reader
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, opts, nil, &result)
		return f.shouldRetryUpload(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 500 {
				// This is quite common on heavy load
				return nil, ErrorInternalDuringUpload
			}
		}
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	if len(result) > 0 {
		result[0].Parent = f.opt.Enc.ToStandardPath(result[0].Parent)
		result[0].Name = f.opt.Enc.ToStandardName(result[0].Name)
		return &result[0], nil
	}
	// no file results sent back
	return nil, ErrorInternalDuringUpload
}

func (f *Fs) uploadChunk(ctx context.Context, in io.Reader, uploadLink, filePath string, contentRange contentRanger) error {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file-upload.md
	chunkSize := int(contentRange.getChunkSize())
	buffer := f.getBuf(chunkSize)
	defer f.putBuf(buffer)

	read, err := io.ReadFull(in, buffer)
	if err != nil {
		return fmt.Errorf("error reading from source: %w", err)
	}
	if chunkSize > 0 && read != chunkSize {
		return fmt.Errorf("expected to read %d from source, but got %d", chunkSize, read)
	}

	result := api.ChunkUpload{}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		// recreate a reader on the temporary buffer
		in = bytes.NewReader(buffer)
		opts, err := f.prepareFileUpload(ctx, in, uploadLink, filePath, contentRange)
		if err != nil {
			return false, err
		}
		resp, err = f.srv.CallJSON(ctx, opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 500 {
				return fmt.Errorf("chunk upload %s: %w", contentRange.getContentRangeHeader(), ErrorInternalDuringUpload)
			}
		}
		return fmt.Errorf("failed to upload chunk %s: %w", contentRange.getContentRangeHeader(), err)
	}
	if !result.Success {
		return errors.New("upload failed")
	}
	return nil
}

func (f *Fs) uploadLastChunk(ctx context.Context, in io.Reader, uploadLink, filePath string, contentRange contentRanger) (*api.FileDetail, error) {
	// API Documentation
	// https://download.seafile.com/published/web-api/v2.1/file-upload.md
	chunkSize := int(contentRange.getChunkSize())
	buffer := f.getBuf(chunkSize)
	defer f.putBuf(buffer)

	read, err := io.ReadFull(in, buffer)
	if err != nil {
		return nil, fmt.Errorf("error reading from source: %w", err)
	}
	if chunkSize > 0 && read != chunkSize {
		return nil, fmt.Errorf("expected to read %d from source, but got %d", chunkSize, read)
	}

	result := make([]api.FileDetail, 1)
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		// recreate a reader on the buffer
		in = bytes.NewReader(buffer)
		opts, err := f.prepareFileUpload(ctx, in, uploadLink, filePath, contentRange)
		if err != nil {
			return false, err
		}
		resp, err = f.srv.CallJSON(ctx, opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return nil, fs.ErrorPermissionDenied
			}
			if resp.StatusCode == 500 {
				return nil, fmt.Errorf("last chunk: %w", ErrorInternalDuringUpload)
			}
		}
		return nil, fmt.Errorf("failed to upload last chunk: %w", err)
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
		return nil, fmt.Errorf("failed to list shared links: %w", err)
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
		return nil, fmt.Errorf("failed to create a shared link: %w", err)
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
		return nil, fmt.Errorf("failed to copy file %s:'%s' to %s:'%s': %w", srcLibraryID, srcPath, dstLibraryID, dstPath, err)
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
		return nil, fmt.Errorf("failed to move file %s:'%s' to %s:'%s': %w", srcLibraryID, srcPath, dstLibraryID, dstPath, err)
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
		return nil, fmt.Errorf("failed to rename file '%s' to '%s': %w", filePath, newname, err)
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
		return fmt.Errorf("failed empty the library trash: %w", err)
	}
	return nil
}

func (f *Fs) getDirectoryEntriesAPIv2(ctx context.Context, libraryID, dirPath string) ([]api.DirEntry, error) {
	// API v2 from the official documentation, but that have been replaced by the much better v2.1 (undocumented as of Apr 2020)
	// getDirectoryEntriesAPIv2 is needed to keep compatibility with seafile v6.
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
		return nil, fmt.Errorf("failed to get directory contents: %w", err)
	}

	// Clean up encoded names
	for index, fileInfo := range result {
		fileInfo.Name = f.opt.Enc.ToStandardName(fileInfo.Name)
		fileInfo.Path = f.opt.Enc.ToStandardPath(fileInfo.Path)
		result[index] = fileInfo
	}
	return result, nil
}
