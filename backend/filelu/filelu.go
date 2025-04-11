// Package filelu provides an interface to the FileLu storage system.
package filelu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/filelu/api"
	"github.com/rclone/rclone/lib/rest"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
)

// Register the backend with Rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "filelu",
		Description: "FileLu Cloud Storage",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:      "key",
			Help:      "Your FileLu Rclone key from My Account",
			Required:  true,
			Sensitive: true,
		}},
	})
}

// Options defines the configuration for the FileLu backend
type Options struct {
	Key string `config:"key"`
}

// Fs represents the FileLu file system
type Fs struct {
	name       string
	root       string
	opt        Options
	features   *fs.Features
	endpoint   string
	pacer      *pacer.Pacer
	srv        *rest.Client
	client     *http.Client
	isFile     bool
	targetFile string
}

// Object describes a FileLu object
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
}

// NewFs creates a new Fs object for FileLu
func NewFs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	fs.Debugf(nil, "NewFs: Starting with root = %q, name = %q", root, name)

	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if opt.Key == "" {
		return nil, fmt.Errorf("FileLu Rclone Key is required")
	}

	client := fshttp.NewClient(ctx)

	if strings.TrimSpace(root) == "" {
    root = ""
}

	isFile := false
	filename := ""
	cleanRoot := strings.Trim(root, "/")

	if strings.Contains(cleanRoot, ".") {
		isFile = true
		filename = path.Base(cleanRoot)
		cleanRoot = path.Dir(cleanRoot)
		if cleanRoot == "." {
			cleanRoot = ""
		}
	}

	f := &Fs{
		name:       name,
		root:       cleanRoot,
		opt:        *opt,
		endpoint:   "https://filelu.com/rclone",
		client:     client,
		srv:        rest.NewClient(client).SetRoot("https://filelu.com/rclone"),
		pacer: pacer.New(),
		isFile:     isFile,
		targetFile: filename,
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
                                      ListR:                   f.ListR,
	}).Fill(ctx, f)

	fs.Debugf(nil, "NewFs: Created filesystem with root path %q, isFile=%v, targetFile=%q", f.root, isFile, filename)
	return f, nil
}

// isFileCode checks if a string looks like a file code
func isFileCode(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Mkdir to create directory on remote server.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Mkdir: Starting directory creation for dir=%q, root=%q", dir, f.root)

	if dir == "" {
		dir = f.root
		if dir == "" {
			return fmt.Errorf("directory name cannot be empty")
		}
	}

	fullPath := path.Clean("/" + dir)
	apiURL := fmt.Sprintf("%s/folder/create?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key), // assuming f.opt.Key is the correct field
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var innerErr error
		resp, innerErr = f.client.Do(req)
		return fserrors.ShouldRetry(innerErr), innerErr
	})
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Logf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result struct {
			FldID int `json:"fld_id"`
		} `json:"result"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully created folder %q with ID %d", dir, result.Result.FldID)
	return nil
}

// GetAccountInfo fetches the account information including storage usage
func (f *Fs) GetAccountInfo(ctx context.Context) (string, string, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/account/info",
		Parameters: url.Values{
			"key": {f.opt.Key},
		},
	}

	var result api.AccountInfoResponse
	err := f.pacer.Call(func() (bool, error) {
		_, callErr := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(callErr), callErr
	})

	if err != nil {
		return "", "", err
	}

	if result.Status != 200 {
		return "", "", fmt.Errorf("error: %s", result.Msg)
	}

	return result.Result.Storage, result.Result.StorageUsed, nil
}

// DeleteFile sends an API request to remove a file from FileLu
func (f *Fs) DeleteFile(ctx context.Context, filePath string) error {
	fs.Debugf(f, "DeleteFile: Attempting to delete file at path %q", filePath)

	filePath = "/" + strings.Trim(filePath, "/")

	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/remove",
		Parameters: url.Values{
			"file_path": {filePath},
			"restore":   {"1"},
			"key":       {f.opt.Key},
		},
	}

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
	_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
	return fserrors.ShouldRetry(err), err
})

	if err != nil {
		return fmt.Errorf("DeleteFile failed: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("DeleteFile API error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully deleted file: %s", filePath)
	return nil
}

// Rename a file using file path
func (f *Fs) renameFile(ctx context.Context, filePath, newName string) error {
	filePath = "/" + strings.Trim(filePath, "/")

	opts := rest.Opts{
		Method: "GET",
		Path:   "/file/rename",
		Parameters: url.Values{
			"file_path": {filePath},
			"name":      {newName},
			"key":       {f.opt.Key},
		},
	}

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})

	if err != nil {
		return fmt.Errorf("renameFile failed: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("renameFile API error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully renamed file at path: %s to %s", filePath, newName)
	return nil
}

// renameFolder handles folder renaming using folder paths
func (f *Fs) renameFolder(ctx context.Context, folderPath, newName string) error {
	folderPath = "/" + strings.Trim(folderPath, "/")

	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/rename",
		Parameters: url.Values{
			"folder_path": {folderPath},
			"name":        {newName},
			"key":         {f.opt.Key},
		},
	}

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		_, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return fserrors.ShouldRetry(err), err
	})

	if err != nil {
		return fmt.Errorf("renameFolder failed: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("renameFolder API error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully renamed folder at path: %s to %s", folderPath, newName)
	return nil
}

var commandHelp = []fs.CommandHelp{{
	Name:  "rename",
	Short: "Rename a file in a FileLu directory",
	Long: `
For example:

    rclone backend rename filelu:/file-path/hello.txt "hello_new_name.txt"
`,
}, {
	Name:  "movefile",
	Short: "Move file within the remote FileLu directory",
	Long: `
For example:

    rclone backend movefile filelu:/source-path/hello.txt /destination-path/
`,
}, {
	Name:  "movefolder",
	Short: "Move a folder on remote FileLu",
	Long: `
For example:

    rclone backend movefolder filelu:/sorce-fld-path/hello-folder/ /destication-fld-path/hello-folder/
`,
}, {
	Name:  "renamefolder",
	Short: "Rename a folder on FileLu",
	Long: `
For example:

    rclone backend renamefolder filelu:/folder-path/folder-name "new-folder-name"
`,
}}

// Command method to handle file and folder rename
func (f *Fs) Command(ctx context.Context, name string, args []string, opt map[string]string) (interface{}, error) {
	switch name {
	case "rename":
		if len(args) != 1 {
			return nil, fmt.Errorf("rename command requires new_name argument")
		}

		// For file operations, construct the full path using f.root and f.targetFile
		var filePath string
		if f.isFile {
			filePath = path.Join(f.root, f.targetFile)
		} else {
			return nil, fmt.Errorf("please specify a file to rename")
		}

		// Ensure the path starts with a forward slash
		filePath = "/" + strings.Trim(filePath, "/")

		newName := args[0]
		// Remove any directory path from new name
		newName = path.Base(newName)

		fs.Debugf(f, "Command rename: Renaming file at path %q to %q", filePath, newName)

		// Perform the rename operation
		err := f.renameFile(ctx, filePath, newName)
		if err != nil {
			return nil, fmt.Errorf("rename failed: %w", err)
		}

		return nil, nil

	case "movefile":
		if len(args) != 1 {
			return nil, fmt.Errorf("movefile command requires destination_folder_path argument")
		}

		// For file operations, construct the full source path using f.root and f.targetFile
		var sourcePath string
		if f.isFile {
			sourcePath = path.Join(f.root, f.targetFile)
			fs.Debugf(f, "Command movefile: Source path constructed as %q", sourcePath)
		} else {
			return nil, fmt.Errorf("please specify a file to move")
		}

		destinationPath := args[0]
		fs.Debugf(f, "Command movefile: Moving file from %q to folder %q", sourcePath, destinationPath)

		err := f.moveFileToDestination(ctx, sourcePath, destinationPath)
		if err != nil {
			return nil, fmt.Errorf("move failed: %w", err)
		}

		return nil, nil

	// Handle move folder case in Command method
	case "movefolder":
		if len(args) != 1 {
			return nil, fmt.Errorf("movefolder command requires destination_folder_path argument")
		}

		if f.isFile {
			return nil, fmt.Errorf("cannot move a file with movefolder command, use movefile instead")
		}

		sourcePath := f.root
		destinationPath := args[0]

		fs.Debugf(f, "Command movefolder: Moving folder from %q to folder %q", sourcePath, destinationPath)

		err := f.moveFolderToDestination(ctx, sourcePath, destinationPath)
		if err != nil {
			return nil, fmt.Errorf("folder move failed: %w", err)
		}

		return nil, nil

	// Handle renamefolder case in Command method
	case "renamefolder":
		fs.Debugf(f, "renamefolder: Received arguments: %+v", args)

		if len(args) != 1 {
			return nil, fmt.Errorf("renamefolder command requires new_name argument")
		}

		folderPath := f.root
		newName := args[0]

		fs.Debugf(f, "renamefolder: Renaming folder at path %q to %q", folderPath, newName)

		// Perform the folder rename operation
		err := f.renameFolder(ctx, folderPath, newName)
		if err != nil {
			return nil, fmt.Errorf("folder rename failed: %w", err)
		}

		return nil, nil

	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// moveFolderToDestination moves a folder to a different location within FileLu
func (f *Fs) moveFolderToDestination(ctx context.Context, folderPath string, destFolderPath string) error {
	folderPath = "/" + strings.Trim(folderPath, "/")
	destFolderPath = "/" + strings.Trim(destFolderPath, "/")

	apiURL := fmt.Sprintf("%s/folder/move?folder_path=%s&dest_folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(folderPath),
		url.QueryEscape(destFolderPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "moveFolderToDestination: Sending move request to %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create move folder request: %w", err)
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send move folder request: %w", err)
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status      int    `json:"status"`
		Msg         string `json:"msg"`
		SourceFldID string `json:"source_fld_id"`
		DestFldID   string `json:"dest_fld_id"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding move folder response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while moving folder: %s", result.Msg)
	}

	fs.Infof(f, "Successfully moved folder from %s to %s", folderPath, destFolderPath)
	return nil
}

// moveFileToDestination moves a file to a different folder using file paths
func (f *Fs) moveFileToDestination(ctx context.Context, filePath string, destinationFolderPath string) error {
	filePath = "/" + strings.Trim(filePath, "/")
	destinationFolderPath = "/" + strings.Trim(destinationFolderPath, "/")

	apiURL := fmt.Sprintf("%s/file/set_folder?file_path=%s&destination_folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(destinationFolderPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "moveFileToDestination: Sending move request to %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create move request: %w", err)
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send move request: %w", err)
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding move response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while moving file: %s", result.Msg)
	}

	fs.Infof(f, "Successfully moved file from %s to folder %s", filePath, destinationFolderPath)
	return nil
}

// About provides usage statistics for the remote
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	storage, storageUsed, err := f.GetAccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	totalStorage, err := parseStorageToBytes(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total storage: %w", err)
	}

	usedStorage, err := parseStorageToBytes(storageUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse used storage: %w", err)
	}

	return &fs.Usage{
		Total: fs.NewUsageValue(totalStorage), // Total bytes available
		Used:  fs.NewUsageValue(usedStorage),  // Total bytes used
		Free:  fs.NewUsageValue(totalStorage - usedStorage),
	}, nil
}

// Hashes returns an empty hash set, indicating no hash support
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet() // Properly creates an empty hash set
}

// Remove deletes the object from FileLu
func (f *Fs) Remove(ctx context.Context, dir string) error {
	fldID, err := f.getFolderID(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to get folder ID for %q: %w", dir, err)
	}

	apiURL := fmt.Sprintf("%s/folder/delete?fld_id=%d&key=%s", f.endpoint, fldID, url.QueryEscape(f.opt.Key))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to delete folder: %w", err)
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error: %s", result.Msg)
	}

	fs.Infof(f, "Removed directory %q successfully", dir)
	return nil
}
// Precision returns the precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// FolderListResponse represents the response for listing folders.
type FolderListResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		Files []struct {
			Name  string      `json:"name"`
			FldID json.Number `json:"fld_id"`
		} `json:"files"`
		Folders []struct {
			Name  string      `json:"name"`
			FldID json.Number `json:"fld_id"`
		} `json:"folders"`
	} `json:"result"`
}

// List returns a list of files and folders
// List returns a list of files and folders for the given directory
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	fs.Debugf(f, "List: Starting for directory %q with root %q", dir, f.root)

	if f.isFile {
		obj, err := f.NewObject(ctx, f.targetFile)
		if err != nil {
			return nil, err
		}
		return []fs.DirEntry{obj}, nil
	}

	// Compose full path for API call
	fullPath := path.Join(f.root, dir)
	fullPath = "/" + strings.Trim(fullPath, "/")
	if fullPath == "/" {
		fullPath = ""
	}

	apiURL := fmt.Sprintf("%s/folder/list?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "List: Fetching from URL: %s", apiURL)

	var body []byte
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to list directory: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return nil, err
	}

	fs.Debugf(f, "List: Response body: %s", string(body))

	var result FolderListResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	var entries fs.DirEntries

	for _, folder := range result.Result.Folders {
		remote := path.Join(dir, folder.Name)
		entries = append(entries, fs.NewDir(remote, time.Now()))
	}

	for _, file := range result.Result.Files {
		remote := path.Join(dir, file.Name)
		size := int64(0)

		filePath := path.Join(fullPath, file.Name)
		if sz, err := f.getFileSize(ctx, filePath); err == nil {
			size = sz
		}

		obj := &Object{
			fs:      f,
			remote:  remote,
			size:    size,
			modTime: time.Now(),
		}
		entries = append(entries, obj)
	}

	return entries, nil
}

// ListR lists the objects and directories of the Fs recursively
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	fs.Debugf(f, "ListR: Starting recursive listing from %q", dir)

	return f.walkDir(ctx, dir, callback)
}

func (f *Fs) walkDir(ctx context.Context, dir string, callback fs.ListRCallback) error {
	entries, err := f.List(ctx, dir)
	if err != nil {
		return err
	}

	if err := callback(entries); err != nil {
		return err
	}

	for _, entry := range entries {
		if d, ok := entry.(fs.Directory); ok {
			err := f.walkDir(ctx, path.Join(dir, d.Remote()), callback)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// listFolderRaw to list folder with its full path.
func (f *Fs) listFolderRaw(ctx context.Context, folderPath string) (*FolderListResponse, error) {
	apiURL := fmt.Sprintf("%s/folder/list?folder_path=%s&&key=%s",
		f.endpoint,
		url.QueryEscape(folderPath),
		url.QueryEscape(f.opt.Key),
	)

	var body []byte
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), err
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		return shouldRetryHTTP(resp.StatusCode), err
	})
	if err != nil {
		return nil, err
	}

	var result FolderListResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ConvertSizeStringToInt64 parses a string size to int64, returning 0 if the parsing fails.
func ConvertSizeStringToInt64(sizeStr string) int64 {
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		fs.Debugf(nil, "Error parsing size '%s': %v", sizeStr, err)
		return 0
	}
	return size
}

// getFileSize retrieves the size of a file
func (f *Fs) getFileSize(ctx context.Context, filePath string) (int64, error) {
	filePath = "/" + strings.Trim(filePath, "/")

	apiURL := fmt.Sprintf("%s/file/info?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "getFileSize: Fetching file info from %s", apiURL)

	var sizeStr string
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch file info: %w", err)
		}
		defer resp.Body.Close()

		var result struct {
			Status int    `json:"status"`
			Msg    string `json:"msg"`
			Result []struct {
				Size string `json:"size"`
			} `json:"result"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("error decoding response: %w", err)
		}

		if result.Status != 200 || len(result.Result) == 0 {
			return false, fmt.Errorf("error fetching file info: %s", result.Msg)
		}

		sizeStr = result.Result[0].Size
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return 0, err
	}

	return ConvertSizeStringToInt64(sizeStr), nil
}

func shouldRetry(err error) bool {
	return fserrors.ShouldRetry(err)
}

func shouldRetryHTTP(code int) bool {
	return code == 429 || code >= 500
}

// getFolderID resolves and returns the folder ID for a given directory name or path
func (f *Fs) getFolderID(ctx context.Context, dir string) (int, error) {
	if dir == "" {
		rootID, err := strconv.Atoi(f.root)
		if err != nil {
			return 0, fmt.Errorf("invalid root directory ID: %w", err)
		}
		return rootID, nil
	}

	if folderID, err := strconv.Atoi(dir); err == nil {
		return folderID, nil
	}

	fs.Debugf(f, "getFolderID: Resolving folder ID for directory=%q", dir)

	parts := strings.Split(dir, "/")
	currentID := 0

	for _, part := range parts {
		if part == "" {
			continue
		}

		apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s", f.endpoint, currentID, url.QueryEscape(f.opt.Key))

		var body []byte
		err := f.pacer.Call(func() (bool, error) {
			req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				return false, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := f.client.Do(req)
			if err != nil {
				return shouldRetry(err), fmt.Errorf("failed to list directory: %w", err)
			}
			defer resp.Body.Close()

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return false, fmt.Errorf("failed to read response body: %w", err)
			}

			return shouldRetryHTTP(resp.StatusCode), nil
		})
		if err != nil {
			return 0, err
		}

		var result struct {
			Status int    `json:"status"`
			Msg    string `json:"msg"`
			Result struct {
				Folders []struct {
					Name  string `json:"name"`
					FldID int    `json:"fld_id"`
				} `json:"folders"`
			} `json:"result"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return 0, fmt.Errorf("error decoding response: %w", err)
		}

		if result.Status != 200 {
			return 0, fmt.Errorf("error: %s", result.Msg)
		}

		found := false
		for _, folder := range result.Result.Folders {
			if folder.Name == part {
				currentID = folder.FldID
				found = true
				break
			}
		}

		if !found {
			return 0, fs.ErrorDirNotFound
		}
	}

	fs.Debugf(f, "getFolderID: Resolved folder ID=%d for directory=%q", currentID, dir)
	return currentID, nil
}

//getDirectLink of files from FileLu to download.
func (f *Fs) getDirectLink(ctx context.Context, filePath string) (string, int64, error) {
	filePath = "/" + strings.Trim(filePath, "/")

	apiURL := fmt.Sprintf("%s/file/direct_link?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "getDirectLink: fetching direct link for file path %q", filePath)

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result struct {
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"result"`
	}

	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch direct link: %w", err)
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("error decoding response: %w", err)
		}

		if result.Status != 200 {
			return false, fmt.Errorf("API error: %s", result.Msg)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return "", 0, err
	}

	fs.Debugf(f, "getDirectLink: obtained URL %q with size %d", result.Result.URL, result.Result.Size)
	return result.Result.URL, result.Result.Size, nil
}

// NewObject creates a new Object for the given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fs.Debugf(f, "NewObject: called with remote=%q", remote)

	var filePath string
	if f.isFile {
		filePath = path.Join(f.root, f.targetFile)
	} else {
		filePath = path.Join(f.root, remote)
	}
	filePath = "/" + strings.Trim(filePath, "/")

	apiURL := fmt.Sprintf("%s/file/info?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "NewObject: Fetching file info from %s", apiURL)

	var body []byte
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to fetch file info: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return nil, err
	}

	fs.Debugf(f, "NewObject: Response body: %s", string(body))

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Size     string `json:"size"`
			Name     string `json:"name"`
			FileCode string `json:"filecode"`
			Hash     string `json:"hash"`
			Status   int    `json:"status"`
		} `json:"result"`
	}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 || len(result.Result) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	fileInfo := result.Result[0]
	size, err := strconv.ParseInt(fileInfo.Size, 10, 64)
	if err != nil {
		fs.Debugf(f, "Error parsing file size %q: %v", fileInfo.Size, err)
		size = 0
	}
	fs.Debugf(f, "File %q size parsed: %d from string: %q", filePath, size, fileInfo.Size)

	returnedRemote := remote
	if f.isFile {
		returnedRemote = f.targetFile
	}

	return &Object{
		fs:      f,
		remote:  returnedRemote,
		size:    size,
		modTime: time.Now(),
	}, nil
}

// Helper function to handle duplicate files
//
//nolint:unused
func (f *Fs) handleDuplicate(ctx context.Context, remote string) error {
	// List files in destination
	entries, err := f.List(ctx, path.Dir(remote))
	if err != nil {
		return err
	}

	// Check if file exists
	for _, entry := range entries {
		if entry.Remote() == remote {
			// Type assert to Object
			obj, ok := entry.(fs.Object)
			if !ok {
				return fmt.Errorf("entry is not an Object")
			}
			// If file exists, remove it first
			err = obj.Remove(ctx)
			if err != nil {
				return fmt.Errorf("failed to remove existing file: %w", err)
			}
			break
		}
	}
	return nil
}

// getUploadServer gets the upload server URL with proper key authentication
func (f *Fs) getUploadServer(ctx context.Context) (string, string, error) {
	apiURL := fmt.Sprintf("%s/upload/server?key=%s", f.endpoint, url.QueryEscape(f.opt.Key))

	var result struct {
		Status int    `json:"status"`
		SessID string `json:"sess_id"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to get upload server: %w", err)
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("error decoding response: %w", err)
		}

		if result.Status != 200 {
			return false, fmt.Errorf("API error: %s", result.Msg)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})

	if err != nil {
		return "", "", err
	}

	fs.Debugf(f, "Got upload server URL=%s and session ID=%s", result.Result, result.SessID)
	return result.Result, result.SessID, nil
}

// Put uploads a file directly to the destination folder in the FileLu storage system.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs.Debugf(f, "Put: Starting upload for %q", src.Remote())

	destPath := path.Dir(src.Remote())
if destPath == "." {
	destPath = ""
}
destinationFolderPath := path.Join(f.root, destPath)
if destinationFolderPath != "" {
	destinationFolderPath = "/" + strings.Trim(destinationFolderPath, "/")
}

	existingEntries, err := f.List(ctx, path.Dir(src.Remote()))
	if err != nil {
		return nil, fmt.Errorf("failed to list existing files: %w", err)
	}

	for _, entry := range existingEntries {
		if entry.Remote() == src.Remote() {
			obj, ok := entry.(fs.Object)
			if !ok || obj.Size() != src.Size() || !obj.ModTime(ctx).Equal(src.ModTime(ctx)) {
				continue
			}
			fs.Infof(f, "Skipping upload for %q, an identical file exists.", src.Remote())
			return obj, nil
		}
	}

	uploadURL, sessID, err := f.getUploadServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve upload server: %w", err)
	}

	fileName := path.Base(src.Remote())

	// Since the fileCode isn't used, just handle the error
	if _, err := f.uploadFileWithDestination(ctx, uploadURL, sessID, fileName, in, destinationFolderPath); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	newObject := &Object{
		fs:      f,
		remote:  src.Remote(),
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}
	fs.Infof(f, "Put: Successfully uploaded new file %q", src.Remote())
	return newObject, nil
}

// respBodyClose to check body response.
func respBodyClose(responseBody io.Closer) {
	if cerr := responseBody.Close(); cerr != nil {
		fmt.Printf("Error closing response body: %v\n", cerr)
	}
}

// uploadFileWithDestination uploads a file directly to a specified folder using file content reader.
func (f *Fs) uploadFileWithDestination(ctx context.Context, uploadURL, sessID, fileName string, fileContent io.Reader, destinationPath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		_ = writer.WriteField("sess_id", sessID)
		_ = writer.WriteField("utype", "prem")
		_ = writer.WriteField("fld_path", destinationPath)

		part, err := writer.CreateFormFile("file_0", fileName)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to create form file: %w", err))
			return
		}

		if _, err := io.Copy(part, fileContent); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to copy file content: %w", err))
			return
		}

		if err := writer.Close(); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to close writer: %w", err))
		}
	}()

	var fileCode string
	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, pr)
		if err != nil {
			return false, fmt.Errorf("failed to create upload request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send upload request: %w", err)
		}
		defer respBodyClose(resp.Body)

		var result []struct {
			FileCode   string `json:"file_code"`
			FileStatus string `json:"file_status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("failed to parse upload response: %w", err)
		}

		if len(result) == 0 || result[0].FileStatus != "OK" {
			return false, fmt.Errorf("upload failed with status: %s", result[0].FileStatus)
		}

		fileCode = result[0].FileCode
		return shouldRetryHTTP(resp.StatusCode), nil
	})

	return fileCode, err
}

// createTempFileFromReader writes the content of the 'in' reader into a temporary file
func createTempFileFromReader(in io.Reader) (string, error) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Defer the closing of the temp file to ensure it gets closed after copying
	defer func() {
		err = tempFile.Close()
		if err != nil {
			fs.Logf(nil, "Failed to close temporary file: %v", err)
		}
	}()

	// Copy the data to the temp file
	_, err = io.Copy(tempFile, in)
	if err != nil {
		// Attempt to remove the file if copy operation fails
		defer func() {
			if err := os.Remove(tempFile.Name()); err != nil {
				fs.Logf(nil, "Failed to remove temp file %q: %v", tempFile.Name(), err)
			}
		}()

		return "", fmt.Errorf("failed to copy data to temp file: %w", err)
	}

	return tempFile.Name(), nil
}

// moveFileToFolder moves a file to a different folder using file paths
func (f *Fs) moveFileToFolder(ctx context.Context, filePath string, destinationPath string) error {
	filePath = "/" + strings.Trim(filePath, "/")
	destinationPath = "/" + strings.Trim(destinationPath, "/")

	apiURL := fmt.Sprintf("%s/file/set_folder?file_path=%s&destination_folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(destinationPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "moveFileToFolder: Sending move request to %s", apiURL)

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}

	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create move request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send move request: %w", err)
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("error decoding move response: %w", err)
		}

		if result.Status != 200 {
			return false, fmt.Errorf("error while moving file: %s", result.Msg)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})

	if err != nil {
		return err
	}

	fs.Debugf(f, "moveFileToFolder: Successfully moved file %q to folder %q", filePath, destinationPath)
	return nil
}

// getFileHash fetches the hash of the uploaded file using its file_code
//
//nolint:unused
func (f *Fs) getFileHash(ctx context.Context, fileCode string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/info?file_code=%s&key=%s", f.endpoint, url.QueryEscape(fileCode), url.QueryEscape(f.opt.Key))

	fmt.Printf("DEBUG: Making API call to get file hash for fileCode: %s\n", fileCode)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fserrors.FsError(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received HTTP status %d", resp.StatusCode)
	}

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Hash string `json:"hash"` // Assuming hash exists
		} `json:"result"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return "", fmt.Errorf("error: %s", result.Msg)
	}

	if len(result.Result) > 0 {
		if result.Result[0].Hash != "" {
			return result.Result[0].Hash, nil
		}
	}

	fmt.Println("DEBUG: Hash not found in API response.")
	return "", nil
}

// Move the objects and directories
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "Move: starting directory move for %q to %q", src.Remote(), remote)

	// Check if the source is a directory
	if srcDir, ok := src.(fs.Directory); ok {
		// Recursively move all contents
		err := f.moveDirectoryContents(ctx, srcDir.Remote(), remote)
		if err != nil {
			return nil, fmt.Errorf("failed to move directory contents: %w", err)
		}
		fs.Debugf(f, "Move: successfully moved directory %q to %q", src.Remote(), remote)
		return src, nil
	}

	// Fall back to single file move
	return f.MoveTo(ctx, src, remote)
}

// Updated recursive directory mover
func (f *Fs) moveDirectoryContents(ctx context.Context, dir string, dest string) error {
	// List all contents of the directory
	entries, err := f.List(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to list directory contents: %w", err)
	}

	for _, entry := range entries {
		switch obj := entry.(type) {
		case fs.Directory:
			// Recursively move subdirectory
			subDirDest := path.Join(dest, obj.Remote())
			err = f.moveDirectoryContents(ctx, obj.Remote(), subDirDest)
			if err != nil {
				return err
			}
		case fs.Object:
			// Move file using MoveTo
			_, err = f.MoveTo(ctx, obj, path.Join(dest, obj.Remote()))
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected entry type: %T", entry)
		}
	}

	return nil
}

// Helper method to move a single file
//
//nolint:unused
func (f *Fs) moveSingleFile(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "MoveSingleFile: moving %q to %q", src.Remote(), remote)

	// Open source object for reading
	reader, err := src.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open source object: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fs.Logf(nil, "Failed to close reader: %v", err)
		}
	}()

	// Upload the file to the destination
	obj, err := f.Put(ctx, reader, src)
	if err != nil {
		return nil, fmt.Errorf("failed to move file to destination: %w", err)
	}

	// Delete the source file
	err = src.Remove(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to delete source file after move: %w", err)
	}

	fs.Debugf(f, "MoveSingleFile: successfully moved %q to %q", src.Remote(), remote)
	return obj, nil
}

// MoveTo moves the file to the specified location
func (f *Fs) MoveTo(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "MoveTo: Starting move for %q to %q", src.Remote(), remote)

	if strings.HasPrefix(remote, "/") || strings.Contains(remote, ":\\") {
		dir := path.Dir(remote)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}

		reader, err := src.Open(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to open source file: %w", err)
		}
		defer reader.Close()

		dest, err := os.Create(remote)
		if err != nil {
			return nil, fmt.Errorf("failed to create destination file: %w", err)
		}
		defer dest.Close()

		if _, err := io.Copy(dest, reader); err != nil {
			return nil, fmt.Errorf("failed to copy file content: %w", err)
		}

		if err := src.Remove(ctx); err != nil {
			return nil, fmt.Errorf("failed to remove source file: %w", err)
		}

		return nil, nil
	}

	reader, err := src.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open source object: %w", err)
	}
	defer reader.Close()

	uploadURL, sessID, err := f.getUploadServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve upload server: %w", err)
	}

	fileName := path.Base(src.Remote())
	fs.Debugf(f, "MoveTo: Using filename %q for upload", fileName)

	var fileCode string
	err = f.pacer.Call(func() (bool, error) {
		var uploadErr error
		fileCode, uploadErr = f.uploadFile(ctx, uploadURL, sessID, fileName, reader)
		if uploadErr != nil {
			return shouldRetry(uploadErr), uploadErr
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	fs.Debugf(f, "MoveTo: File uploaded with code: %s", fileCode)

	sourcePath := "/" + fileName
	destinationPath := "/" + strings.Trim(f.root, "/")

	fs.Debugf(f, "MoveTo: Moving file from %q to folder %q", sourcePath, destinationPath)
	if err := f.moveFileToFolder(ctx, sourcePath, destinationPath); err != nil {
		return nil, fmt.Errorf("failed to move file to destination folder: %w", err)
	}

	if err := src.Remove(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete source file: %w", err)
	}

	return &Object{
		fs:      f,
		remote:  path.Join(remote, fileName),
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}, nil
}

// MoveToLocal moves the file or folder to the local file system.
// It implements the fs.Fs interface and performs the move operation locally.
func (f *Fs) MoveToLocal(ctx context.Context, remote string, localPath string) error {
	fs.Debugf(f, "MoveToLocal: starting move from FileLu %q to local %q", remote, localPath)

	obj, err := f.NewObject(ctx, remote)
	if err != nil {
		return fmt.Errorf("failed to find object in FileLu: %w", err)
	}

	var reader io.ReadCloser
	err = f.pacer.Call(func() (bool, error) {
		var openErr error
		reader, openErr = obj.Open(ctx)
		return shouldRetry(openErr), openErr
	})
	if err != nil {
		return fmt.Errorf("failed to open file for download: %w", err)
	}
	defer reader.Close()

	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %q: %w", localPath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, reader); err != nil {
		return fmt.Errorf("failed to copy data to local file: %w", err)
	}

	if err := obj.Remove(ctx); err != nil {
		return fmt.Errorf("failed to delete file from FileLu after move: %w", err)
	}

	fs.Debugf(f, "MoveToLocal: successfully moved file from FileLu %q to local %q", remote, localPath)
	return nil
}

// DeleteLocalFile deletes a file from the local file system.
func DeleteLocalFile(localPath string) error {
	err := os.Remove(localPath)
	if err != nil {
		return fmt.Errorf("failed to delete local file %q: %w", localPath, err)
	}
	fs.Debugf(nil, "DeleteLocalFile: successfully deleted local file %q", localPath)
	return nil
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Rmdir: Starting for dir=%q", dir)

	fullPath := path.Join(f.root, dir)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}
	fs.Debugf(f, "Rmdir: Using folder_path = %q", fullPath)

	// Step 1: Check if folder is empty
	listURL := fmt.Sprintf("%s/folder/list?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	var listResp struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result struct {
			Files   []interface{} `json:"files"`
			Folders []interface{} `json:"folders"`
		} `json:"result"`
	}

	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		fs.Debugf(f, "Rmdir: folder/list response: %s", string(body))

		if err := json.Unmarshal(body, &listResp); err != nil {
			return false, fmt.Errorf("error decoding list response: %w", err)
		}
		if listResp.Status != 200 {
			return false, fmt.Errorf("API error: %s", listResp.Msg)
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if len(listResp.Result.Files) > 0 || len(listResp.Result.Folders) > 0 {
		return fmt.Errorf("Rmdir: directory %q is not empty", fullPath)
	}

	// Step 2: Delete the folder
	deleteURL := fmt.Sprintf("%s/folder/delete?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	var delResp struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}

	err = f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", deleteURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return fserrors.ShouldRetry(err), err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		fs.Debugf(f, "Rmdir: folder/delete response: %s", string(body))

		if err := json.Unmarshal(body, &delResp); err != nil {
			return false, fmt.Errorf("error decoding delete response: %w", err)
		}
		if delResp.Status != 200 {
			return false, fmt.Errorf("delete error: %s", delResp.Msg)
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	fs.Infof(f, "Rmdir: successfully deleted %q", fullPath)
	return nil
}
// Name returns the remote name
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root path
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("FileLu root '%s'", f.root)
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	o.modTime = modTime
	return nil
}

// Storable indicates whether the object is storable
func (o *Object) Storable() bool {
	return true
}

// Open opens the object for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	filePath := path.Join(o.fs.root, o.remote)

	directLink, size, err := o.fs.getDirectLink(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct link: %w", err)
	}

	o.size = size

	var reader io.ReadCloser
	err = o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", directLink, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create download request: %w", err)
		}

		resp, err := o.fs.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to download file: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return false, fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
		}

		reader = resp.Body
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// Update updates the object with new data
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	fs.Debugf(o.fs, "Update: Starting update for %q", o.remote)

	var existingObject fs.Object
	err := o.fs.pacer.Call(func() (bool, error) {
		var err error
		existingObject, err = o.fs.NewObject(ctx, o.remote)
		return shouldRetry(err), err
	})

	if err == nil && existingObject.Size() == src.Size() {
		fs.Infof(o.fs, "Update: File %q already exists with same size, skipping upload.", o.remote)
		return nil
	}

	tempPath, err := createTempFileFromReader(in)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempPath)

	var tempFile *os.File
	err = o.fs.pacer.Call(func() (bool, error) {
		tempFile, err = os.Open(tempPath)
		return shouldRetry(err), err
	})
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer tempFile.Close()

	uploadURL, sessID, err := o.fs.getUploadServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get upload server: %w", err)
	}
	fs.Debugf(o.fs, "Update: Got upload server URL=%q and session ID=%q", uploadURL, sessID)

	fileName := path.Base(o.remote)
	destPath := "/" + strings.Trim(path.Join(o.fs.root, path.Dir(o.remote)), "/")
	fs.Debugf(o.fs, "Update: Uploading to folder path: %s", destPath)

	fileCode, err := o.fs.uploadFileWithDestination(ctx, uploadURL, sessID, fileName, tempFile, destPath)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	fs.Debugf(o.fs, "Update: Upload complete with code: %s", fileCode)

	o.size = src.Size()
	o.modTime = src.ModTime(ctx)
	return nil
}

// Remove deletes the object from FileLu
func (o *Object) Remove(ctx context.Context) error {
	fs.Debugf(o.fs, "Remove: Deleting file %q", o.remote)
	fullPath := "/" + strings.Trim(path.Join(o.fs.root, o.remote), "/")

	apiURL := fmt.Sprintf("%s/file/remove?file_path=%s&restore=1&key=%s",
		o.fs.endpoint, url.QueryEscape(fullPath), url.QueryEscape(o.fs.opt.Key))

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}
	var body []byte

	err := o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create delete request: %w", err)
		}
		resp, err := o.fs.client.Do(req)
		if err != nil {
			return shouldRetry(err), fmt.Errorf("failed to send delete request: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("error reading response body: %w", err)
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return err
	}

	fs.Debugf(o.fs, "Remove: Response body: %s", string(body))
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("error decoding delete response: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("error while deleting file: %s", result.Msg)
	}

	fs.Infof(o.fs, "Successfully deleted file: %s", fullPath)
	return nil
}

// readMetaData fetches metadata for the object
//
//nolint:unused
func (o *Object) readMetaData(ctx context.Context) error {
	apiURL := fmt.Sprintf("%s/file/info?name=%s&key=%s",
		o.fs.endpoint, url.QueryEscape(o.remote), url.QueryEscape(o.fs.opt.Key))

	var result struct {
		Status  int    `json:"status"`
		Msg     string `json:"msg"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := o.fs.client.Do(req)
		if err != nil {
			return shouldRetry(err), err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false, fs.ErrorObjectNotFound
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, err
		}

		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil || result.Status != 200 {
		return fs.ErrorObjectNotFound
	}

	o.size = result.Size
	o.modTime, err = time.Parse(time.RFC3339, result.ModTime)
	if err != nil {
		o.modTime = time.Now()
	}
	return nil
}
// FileEntry represents a file entry in the JSON response
type FileEntry struct {
	Hash string `json:"hash"`
}

// APIResponse represents the response from the API.
type APIResponse struct {
	Status int `json:"status"`
	Result struct {
		Files []FileEntry `json:"files"`
	} `json:"result"`
}

// DuplicateFileError is a custom error type for duplicate files
type DuplicateFileError struct {
	Hash string
}

func (e *DuplicateFileError) Error() string {
	return "Duplicate file skipped."
}

// IsDuplicateFileError checks if the given error indicates a duplicate file.
func IsDuplicateFileError(err error) bool {
	_, ok := err.(*DuplicateFileError)
	return ok
}

// fetchRemoteFileHashes retrieves hashes of remote files in a folder
//
//nolint:unused
func (f *Fs) fetchRemoteFileHashes(ctx context.Context, folderID int) (map[string]struct{}, error) {
	apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s", f.endpoint, folderID, url.QueryEscape(f.opt.Key))
	var debugResp []byte
	var result struct {
		Status int `json:"status"`
		Result struct {
			Files []struct {
				Hash string `json:"hash"`
			} `json:"files"`
		} `json:"result"`
	}

	err := f.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), err
		}
		defer resp.Body.Close()

		debugResp, err = io.ReadAll(resp.Body)
		return shouldRetryHTTP(resp.StatusCode), err
	})
	if err != nil {
		return nil, err
	}

	fs.Debugf(f, "Raw API Response: %s", string(debugResp))

	if err := json.Unmarshal(debugResp, &result); err != nil {
		return nil, err
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("error: non-200 status %d", result.Status)
	}

	hashes := make(map[string]struct{})
	for _, file := range result.Result.Files {
		fs.Debugf(f, "Fetched remote hash: %s", file.Hash)
		hashes[file.Hash] = struct{}{}
	}
	fs.Debugf(f, "Total fetched remote hashes: %d", len(hashes))
	return hashes, nil
}

// uploadFile to upload objects from local to remote
func (f *Fs) uploadFile(ctx context.Context, uploadURL, sessionID, fileName string, fileContent io.Reader) (string, error) {
	tempPath, err := createTempFileFromReader(fileContent)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempPath)

	file, err := os.Open(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to open temp file for upload: %w", err)
	}
	defer file.Close()

	var fileCode string
	err = f.pacer.Call(func() (bool, error) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		if err = writer.WriteField("sess_id", sessionID); err != nil {
			return false, err
		}
		if err = writer.WriteField("utype", "prem"); err != nil {
			return false, err
		}

		part, err := writer.CreateFormFile("file_0", fileName)
		if err != nil {
			return false, err
		}
		if _, err = io.Copy(part, file); err != nil {
			return false, err
		}
		if err = writer.Close(); err != nil {
			return false, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &body)
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := f.client.Do(req)
		if err != nil {
			return shouldRetry(err), err
		}
		defer resp.Body.Close()

		var result []struct {
			FileCode   string `json:"file_code"`
			FileStatus string `json:"file_status"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, err
		}
		if len(result) == 0 || result[0].FileStatus != "OK" {
			return false, fmt.Errorf("upload failed with status: %s", result[0].FileStatus)
		}

		fileCode = result[0].FileCode
		return shouldRetryHTTP(resp.StatusCode), nil
	})

	if err != nil {
		return "", err
	}

	fs.Debugf(f, "uploadFile: File uploaded successfully with file code: %s", fileCode)
	return fileCode, nil
}

// Hash returns the MD5 hash of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}

	var fileCode string
	if isFileCode(o.fs.root) {
		fileCode = o.fs.root
	} else {
		matches := regexp.MustCompile(`\((.*?)\)`).FindAllStringSubmatch(o.remote, -1)
		for _, match := range matches {
			if len(match) > 1 && len(match[1]) == 12 {
				fileCode = match[1]
				break
			}
		}
	}
	if fileCode == "" {
		return "", fmt.Errorf("no valid file code found in the remote path")
	}

	apiURL := fmt.Sprintf("%s/file/info?file_code=%s&key=%s",
		o.fs.endpoint, url.QueryEscape(fileCode), url.QueryEscape(o.fs.opt.Key))

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Hash string `json:"hash"`
		} `json:"result"`
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := o.fs.client.Do(req)
		if err != nil {
			return shouldRetry(err), err
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, err
		}
		return shouldRetryHTTP(resp.StatusCode), nil
	})
	if err != nil {
		return "", err
	}
	if result.Status != 200 || len(result.Result) == 0 {
		return "", fmt.Errorf("error: unable to fetch hash: %s", result.Msg)
	}

	return result.Result[0].Hash, nil
}

// String returns a string representation of the object
func (o *Object) String() string {
	return o.remote
}

// Check the interfaces are satisfied
var (
	_ fs.Fs = (*Fs)(nil)
	// _ fs.Purger          = (*Fs)(nil)
	// _ fs.PutStreamer     = (*Fs)(nil)
	// _ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter = (*Fs)(nil)
	_ fs.Mover   = (*Fs)(nil)
	// _ fs.DirMover        = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
	// _ fs.IDer            = (*Object)(nil)
                 _ fs.ListRer  = (*Fs)(nil) 
)
