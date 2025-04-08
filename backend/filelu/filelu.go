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
			Sensitive: true, // Hides the key when displayed
		}},
	})
}

// Options defines the configuration for the FileLu backend
type Options struct {
	Key string `config:"key"`
}

// Fs represents the FileLu file system
type Fs struct {
	name       string       // name of the remote
	root       string       // root folder path
	opt        Options      // backend options
	features   *fs.Features // optional features
	endpoint   string       // FileLu endpoint
	client     *http.Client // HTTP client
	isFile     bool         // whether this fs points to a specific file
	targetFile string       // specific file being targeted in single-file operations
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

	if strings.TrimSpace(root) == "." || strings.TrimSpace(root) == "" {
		root = "Rclone"
	}
	// If the root points to a specific file, extract just the directory part
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
		isFile:     isFile,
		targetFile: filename,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
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

// Mkdir creates a new folder on FileLu
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Mkdir: Starting directory creation for dir=%q, root=%q", dir, f.root)

	// Assume root directory if dir is empty, preventing empty directory creation
	if dir == "" {
		dir = f.root
		if dir == "" {
			return fmt.Errorf("directory name cannot be empty")
		}
	}

	// Prepare full path with root verified and normalized
	fullPath := path.Clean("/" + dir)

	// Construct the API URL
	apiURL := fmt.Sprintf("%s/folder/create?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
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
	apiURL := fmt.Sprintf("%s/account/info?key=%s", f.endpoint, url.QueryEscape(f.opt.Key))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", fserrors.FsError(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("received HTTP status %d", resp.StatusCode)
	}

	var result api.AccountInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", "", fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return "", "", fmt.Errorf("error: %s", result.Msg)
	}

	return result.Result.Storage, result.Result.StorageUsed, nil
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// DeleteFile sends an API request to remove a file from FileLu
func (f *Fs) DeleteFile(ctx context.Context, filePath string) error {
	fs.Debugf(f, "DeleteFile: Attempting to delete file at path %q", filePath)

	// Ensure filePath starts with a forward slash and remove any trailing slashes
	filePath = "/" + strings.Trim(filePath, "/")

	// Construct the API URL for deletion
	apiURL := fmt.Sprintf("%s/file/remove?file_path=%s&restore=1&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "DeleteFile: Sending DELETE request to %s", apiURL)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	// Read and log the full response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	fs.Debugf(f, "DeleteFile: Response body: %s", string(body))

	// Parse response
	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding delete response: %w", err)
	}

	// Check API response status
	if result.Status != 200 {
		return fmt.Errorf("error while deleting file: %s", result.Msg)
	}

	fs.Infof(f, "Successfully deleted file: %s", filePath)
	return nil
}

// Rename a file using file path
func (f *Fs) renameFile(ctx context.Context, filePath, newName string) error {
	// Ensure filePath starts with a forward slash
	filePath = "/" + strings.Trim(filePath, "/")

	apiURL := fmt.Sprintf("%s/file/rename?file_path=%s&name=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(newName),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "renameFile: Sending rename request to %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create rename request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send rename request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding rename response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while renaming file: %s", result.Msg)
	}

	fs.Infof(f, "Successfully renamed file at path: %s to %s", filePath, newName)
	return nil
}

// renameFolder handles folder renaming using folder paths
func (f *Fs) renameFolder(ctx context.Context, folderPath string, newName string) error {
	// Ensure the folder path starts with a forward slash
	folderPath = "/" + strings.Trim(folderPath, "/")

	apiURL := fmt.Sprintf("%s/folder/rename?folder_path=%s&name=%s&key=%s",
		f.endpoint,
		url.QueryEscape(folderPath),
		url.QueryEscape(newName),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "renameFolder: Sending rename request to %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create rename folder request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send rename folder request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding rename folder response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while renaming folder: %s", result.Msg)
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
	// Ensure paths start with forward slashes
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

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send move folder request: %w", err)
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
	// Ensure paths start with forward slashes
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

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send move request: %w", err)
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
	// Check if the path is a file or directory and remove accordingly
	fldID, err := f.getFolderID(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to get folder ID for %q: %w", dir, err)
	}

	// Delete folder
	apiURL := fmt.Sprintf("%s/folder/delete?fld_id=%d&key=%s", f.endpoint, fldID, url.QueryEscape(f.opt.Key))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
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
			FldID json.Number `json:"fld_id"` // Adjust to json.Number for flexibility in decoding
		} `json:"files"`
		Folders []struct {
			Name  string      `json:"name"`
			FldID json.Number `json:"fld_id"` // Adjust this as well
		} `json:"folders"`
	} `json:"result"`
}

// List function to use json.Number instead of int
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	fs.Debugf(f, "List: Starting for directory %q with root %q", dir, f.root)

	// If we're targeting a specific file, we should only list that file
	if f.isFile {
		fs.Debugf(f, "List: Single file mode, targeting file %q", f.targetFile)
		obj, err := f.NewObject(ctx, f.targetFile)
		if err != nil {
			return nil, err
		}
		return []fs.DirEntry{obj}, nil
	}

	// Construct the full path for directory listing
	fullPath := path.Join(f.root, dir)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}

	apiURL := fmt.Sprintf("%s/folder/list?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "List: Fetching folder contents from URL: %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	fs.Debugf(f, "List: Response body: %s", string(body))

	var result FolderListResponse
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	entries := make([]fs.DirEntry, 0)

	// Add files
	for _, file := range result.Result.Files {
		remote := path.Join(dir, file.Name)
		filePath := path.Join(fullPath, file.Name)

		if file.FldID.String() != "" {
			fldID, err := file.FldID.Int64()
			if err != nil {
				fs.Debugf(f, "Error converting fld_id for %q: %v", filePath, err)
			} else {
				fs.Debugf(f, "Parsed fld_id for %q: %d", filePath, fldID)
			}
		}

		size, err := f.getFileSize(ctx, filePath)
		if err != nil {
			fs.Debugf(f, "Error getting file size for %q: %v", filePath, err)
			size = 0 // Set default size to 0 if there's an error
		}

		obj := &Object{
			fs:      f,
			remote:  remote,
			size:    size,
			modTime: time.Now(),
		}
		entries = append(entries, obj)
	}

	// Add folders
	for _, folder := range result.Result.Folders {
		remote := path.Join(dir, folder.Name)
		if folder.FldID.String() != "" {
			fldID, err := folder.FldID.Int64()
			if err == nil {
				fs.Debugf(f, "Parsed fld_id for custom directory logic: %d", fldID)
			}
		}
		entries = append(entries, fs.NewDir(remote, time.Now()))
	}

	return entries, nil
}

// ConvertSizeStringToInt64 parses a string size to int64, returning 0 if the parsing fails.
func ConvertSizeStringToInt64(sizeStr string) int64 {
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		// Handle the error gracefully by logging it once
		fs.Debugf(nil, "Error parsing size '%s': %v", sizeStr, err)
		return 0 // Return default value when there's an error
	}
	return size
}

// getFileSize to get the file size of objects on the remote
func (f *Fs) getFileSize(ctx context.Context, filePath string) (int64, error) {
	// Ensure filePath starts with a forward slash
	filePath = "/" + strings.Trim(filePath, "/")

	apiURL := fmt.Sprintf("%s/file/info?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "getFileSize: Fetching file info from %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch file info: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Size string `json:"size"` // Size is still a string here
		} `json:"result"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 || len(result.Result) == 0 {
		return 0, fmt.Errorf("error fetching file info: %s", result.Msg)
	}

	// Convert size from string to int64
	fileSize, err := strconv.ParseInt(result.Result[0].Size, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse file size: %w", err)
	}

	return fileSize, nil
}

// getFolderID resolves and returns the folder ID for a given directory name or path
func (f *Fs) getFolderID(ctx context.Context, dir string) (int, error) {
	// If the directory is empty, return the root directory ID
	if dir == "" {
		rootID, err := strconv.Atoi(f.root)
		if err != nil {
			return 0, fmt.Errorf("invalid root directory ID: %w", err)
		}
		return rootID, nil
	}

	// If the directory is a valid numeric ID, return it directly
	if folderID, err := strconv.Atoi(dir); err == nil {
		return folderID, nil
	}

	fs.Debugf(f, "getFolderID: Resolving folder ID for directory=%q", dir)

	// Fallback: Resolve folder ID based on folder name/path
	parts := strings.Split(dir, "/")
	currentID := 0 // Start from the root directory

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Fetch folders in the current directory
		apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s", f.endpoint, currentID, url.QueryEscape(f.opt.Key))
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("failed to list directory: %w", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Fatalf(nil, "Failed to close response body: %v", err)
			}
		}()

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

		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
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

func (f *Fs) getDirectLink(ctx context.Context, filePath string) (string, int64, error) {
	// Ensure filePath starts with a forward slash
	filePath = "/" + strings.Trim(filePath, "/")

	// Construct the API URL with file_path parameter
	apiURL := fmt.Sprintf("%s/file/direct_link?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "getDirectLink: fetching direct link for file path %q", filePath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch direct link: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result struct {
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"result"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", 0, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return "", 0, fmt.Errorf("error: %s", result.Msg)
	}

	fs.Debugf(f, "getDirectLink: obtained URL %q with size %d", result.Result.URL, result.Result.Size)
	return result.Result.URL, result.Result.Size, nil
}

// NewObject creates a new Object for the given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fs.Debugf(f, "NewObject: called with remote=%q", remote)

	// Determine the proper remote path
	var filePath string
	if f.isFile {
		// If we're in single file mode, use the target file path
		filePath = path.Join(f.root, f.targetFile)
	} else {
		// Otherwise use the provided remote path
		filePath = path.Join(f.root, remote)
	}
	filePath = "/" + strings.Trim(filePath, "/")

	fs.Debugf(f, "NewObject: Using file path %q", filePath)

	// Use the FileLu API to fetch file info
	apiURL := fmt.Sprintf("%s/file/info?file_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "NewObject: Fetching file info from %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file info: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	// Read and log the response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	fs.Debugf(f, "NewObject: Response body: %s", string(body))

	// Parse response
	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Size     string `json:"size"` // API returns size as string
			Name     string `json:"name"`
			FileCode string `json:"filecode"`
			Hash     string `json:"hash"`
			Status   int    `json:"status"`
		} `json:"result"`
	}

	err = json.NewDecoder(bytes.NewReader(body)).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 || len(result.Result) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	// Get the first matching file
	fileInfo := result.Result[0]
	size, err := strconv.ParseInt(fileInfo.Size, 10, 64)
	if err != nil {
		fs.Debugf(f, "Error parsing file size %q: %v", fileInfo.Size, err)
		size = 0 // Set default size to 0 if parsing fails
	}
	fs.Debugf(f, "File %q size parsed: %d from string: %q", filePath, size, fileInfo.Size)

	// Use the correct remote path for the object
	returnedRemote := remote
	if f.isFile {
		returnedRemote = f.targetFile
	}

	return &Object{
		fs:      f,
		remote:  returnedRemote,
		size:    size,
		modTime: time.Now(), // Consider parsing upload time if available in API response
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
	// Step 1: Get upload server
	apiURL := fmt.Sprintf("%s/upload/server?key=%s", f.endpoint, url.QueryEscape(f.opt.Key))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to get upload server: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		SessID string `json:"sess_id"`
		Result string `json:"result"`
		Msg    string `json:"msg"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", "", fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return "", "", fmt.Errorf("error: %s", result.Msg)
	}

	fs.Debugf(f, "Got upload server URL=%s and session ID=%s", result.Result, result.SessID)
	return result.Result, result.SessID, nil
}

// Put uploads a file directly to the destination folder in the FileLu storage system.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs.Debugf(f, "Put: Starting upload for %q", src.Remote())

	destinationFolderPath := path.Join(f.root, path.Dir(src.Remote()))
	destinationFolderPath = "/" + strings.Trim(destinationFolderPath, "/")

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
		defer func() {
			if cerr := pw.Close(); cerr != nil {
				fmt.Printf("Error closing pipe writer: %v\n", cerr)
			}
		}()
		_ = writer.WriteField("sess_id", sessID)
		_ = writer.WriteField("utype", "prem")
		_ = writer.WriteField("fld_path", destinationPath)

		part, err := writer.CreateFormFile("file_0", fileName)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to create form file: %w", err))
			return
		}

		_, err = io.Copy(part, fileContent)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to copy file content: %w", err))
			return
		}

		if cerr := writer.Close(); cerr != nil {
			pw.CloseWithError(fmt.Errorf("failed to close writer: %w", cerr))
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, pr)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	// Use the custom respBodyClose function to handle error
	defer respBodyClose(resp.Body)

	var result []struct {
		FileCode   string `json:"file_code"`
		FileStatus string `json:"file_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result) == 0 || result[0].FileStatus != "OK" {
		return "", fmt.Errorf("upload failed with status: %s", result[0].FileStatus)
	}

	return result[0].FileCode, nil
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
	// Ensure paths start with forward slashes
	filePath = "/" + strings.Trim(filePath, "/")
	destinationPath = "/" + strings.Trim(destinationPath, "/")

	apiURL := fmt.Sprintf("%s/file/set_folder?file_path=%s&destination_folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(filePath),
		url.QueryEscape(destinationPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "moveFileToFolder: Sending move request to %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create move request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send move request: %w", err)
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

	// Check if this is a remote-to-local move
	if strings.HasPrefix(remote, "/") || strings.Contains(remote, ":\\") {
		// This is a remote-to-local move
		// Create the destination directory if it doesn't exist
		dir := path.Dir(remote)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}

		// Open source file for reading
		reader, err := src.Open(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to open source file: %w", err)
		}
		defer func() {
			if err := reader.Close(); err != nil {
				fs.Logf(nil, "Failed to close reader: %v", err)
			}
		}()

		// Create destination file
		dest, err := os.Create(remote)
		if err != nil {
			return nil, fmt.Errorf("failed to create destination file: %w", err)
		}
		defer func() {
			if err := dest.Close(); err != nil {
				fs.Logf(nil, "Failed to close destination file: %v", err)
			}
		}()
		// Copy the content
		_, err = io.Copy(dest, reader)
		if err != nil {
			return nil, fmt.Errorf("failed to copy file content: %w", err)
		}

		// Delete the source file after successful copy
		err = src.Remove(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to remove source file: %w", err)
		}

		return nil, nil
	}

	// This is a local-to-remote move
	reader, err := src.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open source object: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fs.Logf(nil, "Failed to close reader: %v", err)
		}
	}()
	// Get upload server details
	uploadURL, sessID, err := f.getUploadServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve upload server: %w", err)
	}

	// Use the original filename for upload
	fileName := path.Base(src.Remote())
	fs.Debugf(f, "MoveTo: Using filename %q for upload", fileName)

	// Upload file to root directory first
	fileCode, err := f.uploadFile(ctx, uploadURL, sessID, fileName, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	fs.Debugf(f, "MoveTo: File uploaded with code: %s", fileCode)

	// Move the file to destination folder
	sourcePath := "/" + fileName
	destinationPath := "/" + strings.Trim(f.root, "/")

	fs.Debugf(f, "MoveTo: Moving file from %q to folder %q", sourcePath, destinationPath)
	err = f.moveFileToFolder(ctx, sourcePath, destinationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to move file to destination folder: %w", err)
	}

	// Delete the source file after successful move
	err = src.Remove(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to delete source file: %w", err)
	}

	// Create and return the destination object
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

	// Download file from FileLu
	obj, err := f.NewObject(ctx, remote)
	if err != nil {
		return fmt.Errorf("failed to find object in FileLu: %w", err)
	}

	reader, err := obj.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open file for download: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fs.Logf(nil, "Failed to close reader: %v", err)
		}
	}()

	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %q: %w", localPath, err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fs.Logf(nil, "Failed to close reader: %v", err)
		}
	}()

	_, err = io.Copy(outFile, reader)
	if err != nil {
		return fmt.Errorf("failed to copy data to local file: %w", err)
	}

	// Verify download and delete file from FileLu
	err = obj.Remove(ctx)
	if err != nil {
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
	fs.Debugf(f, "Rmdir: Starting with dir=%q", dir)

	// Construct the full folder path
	fullPath := path.Join(f.root, dir)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}
	fs.Debugf(f, "Rmdir: Using folder path %q", fullPath)

	// First check if the folder is empty using folder/list
	listURL := fmt.Sprintf("%s/folder/list?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to create list request: %w", err))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to check directory contents: %w", err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Logf(nil, "Failed to close response body: %v", err)
		}
	}()

	// Read and log response for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("error reading list response body: %w", err))
	}
	fs.Debugf(f, "Rmdir: List response: %s", string(body))

	var listResult struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result struct {
			Files   []interface{} `json:"files"`
			Folders []interface{} `json:"folders"`
		} `json:"result"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&listResult); err != nil {
		return fserrors.NoRetryError(fmt.Errorf("error decoding list response: %w", err))
	}

	// Check if folder exists and is empty
	if listResult.Status != 200 {
		return fserrors.NoRetryError(fmt.Errorf("folder not found: %s", listResult.Msg))
	}

	if len(listResult.Result.Files) > 0 || len(listResult.Result.Folders) > 0 {
		return fserrors.NoRetryError(fmt.Errorf("directory is not empty"))
	}

	// Delete the folder using the new folder_path API
	deleteURL := fmt.Sprintf("%s/folder/delete?folder_path=%s&key=%s",
		f.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(f.opt.Key),
	)

	fs.Debugf(f, "Rmdir: Sending delete request to %s", deleteURL)

	req, err = http.NewRequestWithContext(ctx, "GET", deleteURL, nil)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to create delete request: %w", err))
	}

	resp, err = f.client.Do(req)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to delete directory: %w", err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Logf(nil, "Failed to close response body: %v", err)
		}
	}()

	// Read and log response for debugging
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("error reading delete response body: %w", err))
	}
	fs.Debugf(f, "Rmdir: Delete response: %s", string(body))

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return fserrors.NoRetryError(fmt.Errorf("error decoding delete response: %w", err))
	}

	if result.Status != 200 {
		return fserrors.NoRetryError(fmt.Errorf("error deleting directory: %s", result.Msg))
	}

	fs.Infof(f, "Successfully deleted directory %q", fullPath)
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
	// Construct the full file path
	filePath := path.Join(o.fs.root, o.remote)

	directLink, size, err := o.fs.getDirectLink(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct link: %w", err)
	}

	o.size = size // Update the object size with the value from API

	req, err := http.NewRequestWithContext(ctx, "GET", directLink, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := o.fs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fs.Fatalf(nil, "Failed to close response body: %v", err)
			}
		}()
		return nil, fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Update updates the object with new data
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	fs.Debugf(o.fs, "Update: Starting update for %q", o.remote)

	// Fetch existing object on remote
	existingObject, err := o.fs.NewObject(ctx, o.remote)
	if err == nil {
		// Compare size
		if existingObject.Size() == src.Size() {
			fs.Infof(o.fs, "Update: File %q already exists with the same size, skipping upload.", o.remote)
			return nil
		}
	} else {
		fs.Debugf(o.fs, "No existing object found for %q, proceeding with upload...", o.remote)
	}

	// Proceed with the upload if no match is found
	tempPath, err := createTempFileFromReader(in)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempPath); err != nil {
			fs.Logf(nil, "Failed to remove file %q: %v", tempPath, err)
		}
	}()

	tempFile, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer func() {
		if err := tempFile.Close(); err != nil {
			fs.Logf(nil, "Failed to close temporary file: %v", err)
		}
	}()

	uploadURL, sessID, err := o.fs.getUploadServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get upload server: %w", err)
	}
	fs.Debugf(o.fs, "Update: Got upload server URL=%q and session ID=%q", uploadURL, sessID)

	fileName := path.Base(o.remote)
	destinationFolderPath := path.Join(o.fs.root, path.Dir(o.remote))
	destinationFolderPath = "/" + strings.Trim(destinationFolderPath, "/")
	fs.Debugf(o.fs, "Update: Uploading file to folder path: %s", destinationFolderPath)

	fileCode, err := o.fs.uploadFileWithDestination(ctx, uploadURL, sessID, fileName, tempFile, destinationFolderPath)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	fs.Debugf(o.fs, "Update: File uploaded successfully with code: %s", fileCode)

	o.size = src.Size()
	o.modTime = src.ModTime(ctx)

	fs.Debugf(o.fs, "Update: Finished update for %q", o.remote)
	return nil
}

// Remove deletes the object from FileLu
func (o *Object) Remove(ctx context.Context) error {
	fs.Debugf(o.fs, "Remove: Deleting file %q", o.remote)

	// Construct full path
	fullPath := path.Join(o.fs.root, o.remote)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}

	// Construct the API URL for deletion
	apiURL := fmt.Sprintf("%s/file/remove?file_path=%s&restore=1&key=%s",
		o.fs.endpoint,
		url.QueryEscape(fullPath),
		url.QueryEscape(o.fs.opt.Key),
	)

	fs.Debugf(o.fs, "Remove: Sending delete request to %s", apiURL)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	// Execute request
	resp, err := o.fs.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	// Read and log the full response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	fs.Debugf(o.fs, "Remove: Response body: %s", string(body))

	// Parse response
	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	}
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding delete response: %w", err)
	}

	// Check API response status
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
	apiURL := fmt.Sprintf("%s/file/info?name=%s&key=%s", o.fs.endpoint, url.QueryEscape(o.remote), url.QueryEscape(o.fs.opt.Key))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := o.fs.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fs.ErrorObjectNotFound
	}

	var result struct {
		Status  int    `json:"status"`
		Msg     string `json:"msg"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
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
	fs.Debugf(f, "Fetching remote hashes using URL: %s", apiURL) // Log the API URL for verification

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Logf(nil, "Failed to close response body: %v", err.Error())
		}
	}()

	// Log raw HTTP response for debugging
	debugResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	fs.Debugf(f, "Raw API Response: %s", string(debugResp))

	// Reset the reader for JSON decoding
	resp.Body = io.NopCloser(bytes.NewBuffer(debugResp))
	// Define the structure for the API response
	type APIResponse struct {
		Status int `json:"status"`
		Result struct {
			Files []struct {
				Hash string `json:"hash"`
			} `json:"files"`
		} `json:"result"`
	}

	// Decode JSON response
	var apiResponse APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if apiResponse.Status != 200 {
		return nil, fmt.Errorf("error: non-200 status %d", apiResponse.Status)
	}

	hashes := make(map[string]struct{})
	for _, file := range apiResponse.Result.Files {
		fs.Debugf(f, "Fetched remote hash: %s", file.Hash) // Log each hash fetched
		hashes[file.Hash] = struct{}{}
	}

	fs.Debugf(f, "Total fetched remote hashes: %d", len(hashes))
	return hashes, nil
}

// uploadFile to upload objects from local to remote
func (f *Fs) uploadFile(ctx context.Context, uploadURL, sessionID, fileName string, fileContent io.Reader) (string, error) {
	// Create temporary file and get its path
	tempPath, err := createTempFileFromReader(fileContent)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempPath); err != nil {
			fs.Logf(nil, "Failed to remove temp file %q: %v", tempPath, err)
		}
	}()

	// Open the temporary file for the multipart upload
	file, err := os.Open(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to open temp file for upload: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fs.Logf(nil, "Failed to close temp file %q: %v", tempPath, err)
		}
	}()

	// Prepare multipart form data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	if err = writer.WriteField("sess_id", sessionID); err != nil {
		return "", fmt.Errorf("failed to add sess_id field: %w", err)
	}
	if err = writer.WriteField("utype", "prem"); err != nil {
		return "", fmt.Errorf("failed to add utype field: %w", err)
	}

	// Create the file part
	part, err := writer.CreateFormFile("file_0", fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy file content to form
	if _, err = io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to form: %w", err)
	}

	if err = writer.Close(); err != nil {
		return "", fmt.Errorf("error closing writer: %w", err)
	}

	// Send the request
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fs.Debugf(f, "Error closing response body: %v", cerr)
		}
	}()

	// Parse the response
	var result []struct {
		FileCode   string `json:"file_code"`
		FileStatus string `json:"file_status"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result) == 0 || result[0].FileStatus != "OK" {
		return "", fmt.Errorf("upload failed with status: %s", result[0].FileStatus)
	}

	fs.Debugf(f, "uploadFile: File uploaded successfully with file code: %s", result[0].FileCode)
	return result[0].FileCode, nil
}

// Hash returns the MD5 hash of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}

	var fileCode string

	// Function to check if the extracted code is a valid file code (non-numeric and 12 characters long)
	isValidFileCode := func(code string) bool {
		if len(code) != 12 {
			return false
		}
		// Check if the code contains any non-numeric character
		for _, c := range code {
			if c < '0' || c > '9' {
				return true // Alphanumeric (contains at least one non-numeric character)
			}
		}
		return false // It's purely numeric, not a file code
	}

	// Extract file code directly if available, otherwise from the remote path
	if isFileCode(o.fs.root) {
		fileCode = o.fs.root
	} else {
		// Attempt to extract file code from the remote path
		remote := o.remote
		// Find all substrings inside parentheses
		matches := regexp.MustCompile(`\((.*?)\)`).FindAllStringSubmatch(remote, -1)

		// Loop through all matched substrings and check for a valid file code
		for _, match := range matches {
			if len(match) > 1 {
				extractedCode := match[1]
				if isValidFileCode(extractedCode) {
					fileCode = extractedCode
					break // Found a valid file code, no need to continue
				}
			}
		}
	}

	// If no valid file code was found, return an error
	if fileCode == "" {
		return "", fmt.Errorf("no valid file code found in the remote path")
	}

	// Use the file_code for API queries
	apiURL := fmt.Sprintf("%s/file/info?file_code=%s&key=%s",
		o.fs.endpoint, url.QueryEscape(fileCode), url.QueryEscape(o.fs.opt.Key))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create hash request: %w", err)
	}

	resp, err := o.fs.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("hash request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result []struct {
			Hash string `json:"hash"` // Assuming the hash is here
		} `json:"result"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("error decoding hash response: %w", err)
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
)
