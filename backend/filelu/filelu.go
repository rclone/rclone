// Package filelu provides an interface to the FileLu storage system.
package filelu

import (
    "bytes"
    "context"
    "crypto/md5"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "net/url"
    "os"
    "path"
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
		Options: []fs.Option{
			{
				Name:      "FileLu Rclone Key",
				Help:      "Get your FileLu Rclone key in My Account",
				Required:  true,
				Sensitive: true, // Hides the key when displayed
			},
		},
	})
}

// Options defines the configuration for the FileLu backend
type Options struct {
	RcloneKey string `config:"FileLu Rclone Key"`
}

// Fs represents the FileLu file system
type Fs struct {
	name     string       // name of the remote
	root     string       // root folder path
                 folderID string       // folder ID for hashing
	opt      Options      // backend options
	endpoint string       // FileLu endpoint
	client   *http.Client // HTTP client
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

    if opt.RcloneKey == "" {
        return nil, fmt.Errorf("FileLu Rclone Key is required")
    }

    client := fshttp.NewClient(ctx)

    // Ensure the root is properly formatted
    trimmedRoot := strings.Trim(root, "/")
    folderID, parsedRoot := parseFolderID(trimmedRoot)

    if folderID == "" {
        folderID = "0"
        fs.Debugf(nil, "NewFs: No folder ID found in path, defaulting to 0")
    } else {
        fs.Debugf(nil, "NewFs: Found folder ID %q", folderID)
    }

    // Additional debugging information
    fs.Debugf(nil, "NewFs: Parsed Root %q", parsedRoot)

    f := &Fs{
        name:     name,
        root:     parsedRoot,
        folderID: folderID,
        opt:      *opt,
        endpoint: "https://filelu.com/rclone",
        client:   client,
    }

    fs.Debugf(nil, "NewFs: Created filesystem with folder ID %q", f.folderID)
    return f, nil
}


// Simplified parseFolderID function to handle direct folder ID
func parseFolderID(root string) (folderID, parsedRoot string) {
    // Check if the root is purely numeric, indicating a direct folder ID
    if isNumeric(root) {
        return root, "" // This assumes the root is just the folder ID
    }
  
    parts := strings.Split(root, ":")
    if len(parts) == 2 {
        return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
    }
    return "", root
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

// resolveFolderPath takes a path and returns the folder ID, creating the folder if it doesn't exist
func (f *Fs) resolveFolderPath(ctx context.Context, path string) (int, error) {
	if path == "" {
		return 0, nil // Root directory
	}

	parts := strings.Split(path, "/")
	currentID := 0 // Start from root

	for _, part := range parts {
		if part == "" {
			continue
		}

		    // Extract folder ID from format "(id) name"
folderID := 0
if strings.HasPrefix(part, "(") {
    end := strings.Index(part, ")")
    if end != -1 {
        idStr := part[1:end]
        if id, err := strconv.Atoi(idStr); err == nil {
            folderID = id
            part = strings.TrimSpace(part[end+1:])
        }
    }
}

		if folderID != 0 {
			currentID = folderID
			continue
		}

		apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s",
			f.endpoint,
			currentID,
			url.QueryEscape(f.opt.RcloneKey))

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return 0, err
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return 0, err
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
					FldID int    `json:"fld_id"` // Changed to int
				} `json:"folders"`
			} `json:"result"`
		}

		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return 0, err
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

	return currentID, nil
}

// File: filelu.go

// GetAccountInfo fetches the account information including storage usage
func (f *Fs) GetAccountInfo(ctx context.Context) (string, string, error) {
	apiURL := fmt.Sprintf("%s/account/info?key=%s", f.endpoint, url.QueryEscape(f.opt.RcloneKey))

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
	return &fs.Features{
		About:                   f.About,
		Command:                 f.Command,
		DirMove:                 nil,
		CanHaveEmptyDirectories: true,
	}
}

// DeleteFile deletes a file from FileLu using the provided file_code
func (f *Fs) DeleteFile(ctx context.Context, fileCode string) error {
	apiURL := fmt.Sprintf("%s/file/remove?file_code=%s&remove=1&key=%s",
		f.endpoint,
		url.QueryEscape(fileCode),
		url.QueryEscape(f.opt.RcloneKey),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
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
		return fmt.Errorf("error decoding delete response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while deleting file: %s", result.Msg)
	}

	fs.Infof(f, "Successfully deleted file with code: %s", fileCode)
	return nil
}

// Command handles various commands including delete
func (f *Fs) Command(ctx context.Context, name string, args []string, opt map[string]string) (interface{}, error) {
	switch name {
	case "delete":
		if len(args) == 0 {
			return nil, fmt.Errorf("delete requires at least one path")
		}
		fs.Infof(f, "Deleting files: %v", args)
		for _, remote := range args {
			obj, err := f.NewObject(ctx, remote)
			if err != nil {
				return nil, fmt.Errorf("failed to find object %q: %w", remote, err)
			}
			err = obj.Remove(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to delete %q: %w", remote, err)
			}
		}
		return nil, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
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

// isNumeric checks if a string contains only numeric characters
//
//nolint:unused
// Keep this function definition in one location and remove the other.
func isNumeric(s string) bool {
    _, err := strconv.Atoi(s)
    return err == nil
}

// Mkdir creates a new folder on FileLu
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Mkdir: Starting directory creation for dir=%q, root=%q", dir, f.root)

	// If dir is empty, assume root directory
	if dir == "" {
		dir = f.root
		if dir == "" {
			return fmt.Errorf("directory name cannot be empty")
		}
	}

	// Resolve parent folder ID
	parentID := 0
	parentDir := path.Dir(dir) // Get the parent directory path
	if parentDir != "." && parentDir != "/" {
		var err error
		parentID, err = f.resolveFolderPath(ctx, parentDir)
		if err != nil {
			return fmt.Errorf("failed to resolve parent folder path: %w", err)
		}
	}

	// Create the directory
	apiURL := fmt.Sprintf("%s/folder/create?parent_id=%d&name=%s&key=%s",
		f.endpoint,
		parentID,
		url.QueryEscape(path.Base(dir)), // Use the base name of the path
		url.QueryEscape(f.opt.RcloneKey),
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
			FldID string `json:"fld_id"`
		} `json:"result"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error: %s", result.Msg)
	}

	fs.Infof(f, "Successfully created folder %q with ID %q", dir, result.Result.FldID)
	return nil
}

// Remove deletes the object from FileLu
func (f *Fs) Remove(ctx context.Context, dir string) error {
	// Check if the path is a file or directory and remove accordingly
	fldID, err := f.getFolderID(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to get folder ID for %q: %w", dir, err)
	}

	// Delete folder
	apiURL := fmt.Sprintf("%s/folder/delete?fld_id=%d&key=%s", f.endpoint, fldID, url.QueryEscape(f.opt.RcloneKey))
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

// List lists the objects and directories in a remote directory.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	fs.Debugf(f, "List: Starting for directory %q", dir)

	// If the root is a file code, handle it as a single file
	if isFileCode(f.root) {
		fs.Debugf(f, "List: root is a file code %q, returning file object", f.root)

		// Fetch the direct link and file size
		directLink, size, err := f.getDirectLink(ctx, f.root)
		if err != nil {
			return nil, fmt.Errorf("failed to get direct link: %w", err)
		}

		// Create an Object for the file
		fileObject := &Object{
			fs:      f,
			remote:  extractFileName(directLink),
			size:    size,
			modTime: time.Now(), // Optionally fetch the actual mod time if available
		}

		return fs.DirEntries{fileObject}, nil
	}

	// Handle normal directories
	folderID, err := f.resolveFolderPath(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve folder path: %w", err)
	}

	return f.listDirectory(ctx, folderID, dir)
}

func (f *Fs) listDirectory(ctx context.Context, folderID int, dir string) (fs.DirEntries, error) {
	apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s", f.endpoint, folderID, url.QueryEscape(f.opt.RcloneKey))
	fs.Debugf(f, "listDirectory: Fetching files and folders for fld_id=%d (directory=%q)", folderID, dir)

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

	var result struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
		Result struct {
			Files []struct {
				Name string `json:"name"`
				Code string `json:"file_code"`
				Size int64  `json:"size"`
			} `json:"files"`
			Folders []struct {
				Name   string `json:"name"`
				FldID  int    `json:"fld_id"`
				Parent int    `json:"parent_fld_id"`
			} `json:"folders"`
		} `json:"result"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("error: %s", result.Msg)
	}

	entries := fs.DirEntries{}

	// Build the current directory path
	currentDir := dir
	if currentDir != "" && !strings.HasSuffix(currentDir, "/") {
		currentDir += "/"
	}

	for _, folder := range result.Result.Folders {
		nameWithID := fmt.Sprintf("(%d) %s", folder.FldID, folder.Name)
		// For directories, combine the current path with the folder name
		fullPath := nameWithID
		if currentDir != "" {
			fullPath = path.Join(currentDir, nameWithID)
		}
		d := fs.NewDir(fullPath, time.Now())
		entries = append(entries, d)
	}

	for _, file := range result.Result.Files {
		nameWithCode := fmt.Sprintf("(%s) %s", file.Code, file.Name)
		// For files, combine the current path with the file name
		fullPath := nameWithCode
		if currentDir != "" {
			fullPath = path.Join(currentDir, nameWithCode)
		}
		entries = append(entries, &Object{
			fs:      f,
			remote:  fullPath,
			size:    file.Size,
			modTime: time.Now(),
		})
	}

	fs.Debugf(f, "listDirectory: Successfully listed contents for folder ID: %d", folderID)
	return entries, nil
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
		apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s", f.endpoint, currentID, url.QueryEscape(f.opt.RcloneKey))
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

func (f *Fs) getDirectLink(ctx context.Context, fileCode string) (string, int64, error) {
	fileCode = strings.TrimSpace(fileCode)
	if fileCode == "" {
		return "", 0, fmt.Errorf("empty file code")
	}

	apiURL := fmt.Sprintf("%s/file/direct_link?file_code=%s&key=%s", f.endpoint, url.QueryEscape(fileCode), url.QueryEscape(f.opt.RcloneKey))
	fs.Debugf(f, "getDirectLink: fetching direct link for file code %q", fileCode)

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
/*func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    fs.Debugf(f, "NewObject: called with remote=%q", remote)

    // Clean the remote path
    remote = strings.TrimPrefix(remote, "/")

    // If remote is empty, return error
    if remote == "" {
        return nil, fmt.Errorf("empty remote path")
    }

    // For new files, just return the object without trying to get info
    // This allows Put to handle the actual file creation
    fs.Debugf(f, "NewObject: creating new object for path=%q without validation", remote)
    return &Object{
        fs:      f,
        remote:  remote,
        modTime: time.Now(),
    }, nil
}*/

// NewObject creates a new Object for the given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fs.Debugf(f, "NewObject: called with remote=%q", remote)

	// If the root is a file code, handle it as a single file
	if isFileCode(f.root) {
		fs.Debugf(f, "NewObject: root is a file code %q", f.root)

		directLink, size, err := f.getDirectLink(ctx, f.root)
		if err != nil {
			return nil, fmt.Errorf("failed to get direct link: %w", err)
		}

		return &Object{
			fs:      f,
			remote:  extractFileName(directLink),
			size:    size,
			modTime: time.Now(), // Optionally fetch the actual mod time if available
		}, nil
	}

	// Handle normal remote paths
	return &Object{
		fs:      f,
		remote:  remote,
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
	// Step 1: Get upload server
	apiURL := fmt.Sprintf("%s/upload/server?key=%s", f.endpoint, url.QueryEscape(f.opt.RcloneKey))

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
// Put uploads a file to the storage backend.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
    fs.Debugf(f, "Put: Starting upload for %q", src.Remote())

    // Convert the input reader to a temp file to compute the MD5 hash.
    tempFile, err := createTempFileFromReader(in)
    if err != nil {
        return nil, fmt.Errorf("failed to create temp file: %w", err)
    }
  defer func() {
    if err := os.Remove("file_path"); err != nil {
       fs.Logf(nil, "Failed to remove file: %v", err.Error())
    }
}()

   // Compute the MD5 hash of the file
    hash, err := ComputeMD5(tempFile.Name())
   if err != nil {
       return nil, fmt.Errorf("failed to compute file hash: %w", err)
    }

    // Print the local computed hash for debugging
    fs.Debugf(f, "Local file hash for %q: %s", src.Remote(), hash)

    // Fetch existing remote hashes for the given folder
    folderID, err := strconv.Atoi(f.folderID)
    if err != nil {
        folderID = 0 // Default to root folder if conversion fails
    }
    fs.Debugf(f, "Folder ID: %d", folderID)

    // Generate the combined hash
    combinedHash := fmt.Sprintf("%s%d", hash, folderID)
    fs.Debugf(f, "Combined file and folder hash: %s", combinedHash)

    existingHashes, err := f.FetchRemoteFileHashes(ctx, folderID)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch remote file hashes: %w", err)
    }

    // Compare the combined hash with remote hashes
    if _, exists := existingHashes[combinedHash]; exists {
        fs.Infof(f, "Detected duplicate file %q, skipping upload.", src.Remote())
        //return nil, fmt.Errorf("file %q is a duplicate", src.Remote())
    }

    // Proceed with file upload if not a duplicate
    uploadURL, sessID, err := f.getUploadServer(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve upload server: %w", err)
    }

    fileCode, err := f.uploadFile(ctx, uploadURL, sessID, src.Remote(), tempFile)
    if err != nil {
        return nil, fmt.Errorf("failed to upload file: %w", err)
    }

    // Move the file if needed
    if folderID != 0 {
        moveErr := f.moveFileToFolder(ctx, fileCode, folderID)
        if moveErr != nil {
            return nil, fmt.Errorf("failed to move file to folder ID %d: %w", folderID, moveErr)
        }
    }

    obj := &Object{
        fs:      f,
        remote:  src.Remote(),
        size:    src.Size(),
        modTime: src.ModTime(ctx),
    }
    return obj, nil
}

// createTempFileFromReader writes the content of the 'in' reader into a temporary file
func createTempFileFromReader(in io.Reader) (*os.File, error) {
    // Create a temporary file
    tempFile, err := os.CreateTemp("", "upload-*.tmp")
    if err != nil {
        return nil, fmt.Errorf("failed to create temp file: %w", err)
    }

    // Copy the content from the reader into the temporary file
    _, err = io.Copy(tempFile, in)
    if err != nil {
        tempFile.Close() // Make sure to close the file even if an error occurs during copying
        return nil, fmt.Errorf("failed to write to temp file: %w", err)
    }

    // Seek back to the start of the file for further reading
    if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
        tempFile.Close() // Close the file before returning the error
        return nil, fmt.Errorf("failed to seek file: %w", err)
    }

    // Ensure the file is closed when the function exits, handle errors if any
    if err := tempFile.Close(); err != nil {
        fs.Logf(nil, "Failed to close temporary file: %v", err)
        return nil, fmt.Errorf("failed to close temp file: %w", err)
    }

    // Return the temporary file
    return tempFile, nil
}

func (f *Fs) moveFileToFolder(ctx context.Context, fileCode string, folderID int) error {
	if folderID == 0 {
		return fmt.Errorf("invalid folder ID")
	}

	apiURL := fmt.Sprintf("%s/file/set_folder?file_code=%s&fld_id=%d&key=%s",
		f.endpoint,
		url.QueryEscape(fileCode),
		folderID,
		url.QueryEscape(f.opt.RcloneKey),
	)

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

	fs.Debugf(f, "moveFileToFolder: File moved successfully to folder ID: %d", folderID)
	return nil
}

// getFileHash fetches the hash of the uploaded file using its file_code
//
//nolint:unused
func (f *Fs) getFileHash(ctx context.Context, fileCode string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/info?file_code=%s&key=%s", f.endpoint, url.QueryEscape(fileCode), url.QueryEscape(f.opt.RcloneKey))

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

// MoveTo moves the file or folder to the specified location.
// It implements the fs.Fs interface and performs the move operation.
func (f *Fs) MoveTo(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	fs.Debugf(f, "MoveTo: Starting move for %q to %q", src.Remote(), remote)

	reader, err := src.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open source object: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fs.Logf(nil, "Failed to close reader: %v", err)
		}
	}()

	uploadURL, sessID, err := f.getUploadServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve upload server: %w", err)
	}

	folderID, err := strconv.Atoi(f.root)
	if err != nil || folderID == 0 {
		folderID = 0 // Default to root folder
	}

	fileCode, uploadErr := f.uploadFileWithFolder(ctx, uploadURL, sessID, src.Remote(), reader, folderID)
	if uploadErr != nil {
		return nil, fmt.Errorf("failed to upload and move file: %w", uploadErr)
	}

	// Add this line to use the variable
	fs.Debugf(f, "Uploaded file has fileCode: %s", fileCode)

	err = src.Remove(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to delete source file after move: %w", err)
	}

	return &Object{
		fs:      f,
		remote:  src.Remote(),
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}, nil
}

func (f *Fs) uploadFileWithFolder(ctx context.Context, uploadURL, sessionID, fileName string, fileContent io.Reader, folderID int) (string, error) {
	// Step 1: Upload the file
	fileCode, err := f.uploadFile(ctx, uploadURL, sessionID, fileName, fileContent)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Step 2: Move the file to the specified folder
	if folderID != 0 {
		err = f.moveFileToFolder(ctx, fileCode, folderID)
		if err != nil {
			return "", fmt.Errorf("failed to move file to folder: %w", err)
		}
	}

	return fileCode, nil
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

// Rmdir removes a directory if it is empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// Combine root path with dir if root is not empty
	fullPath := dir
	if f.root != "" {
		fullPath = path.Join(f.root, dir)
	}

	// Clean and validate the path
	fullPath = strings.Trim(fullPath, "/")
	if fullPath == "" {
		return fserrors.NoRetryError(fmt.Errorf("directory name cannot be empty"))
	}

	fs.Debugf(f, "Removing directory: '%s'", fullPath)

	// Get the folder ID for the directory
	fldID, err := f.getFolderID(ctx, fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrorDirNotFound) {
			// If directory doesn't exist, return appropriate error
			return fs.ErrorDirNotFound
		}
		return fserrors.NoRetryError(fmt.Errorf("failed to get folder ID: %w", err))
	}

	// Check if directory is empty
	apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s",
		f.endpoint,
		fldID,
		url.QueryEscape(f.opt.RcloneKey))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to create list request: %w", err))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to check directory contents: %w", err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	var listResult api.FolderListResponse
	err = json.NewDecoder(resp.Body).Decode(&listResult)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("error decoding list response: %w", err))
	}

	// Check if directory is empty
	if len(listResult.Result.Files) > 0 || len(listResult.Result.Folders) > 0 {
		return fserrors.NoRetryError(fmt.Errorf("directory not empty"))
	}

	// Construct delete API URL
	deleteURL := fmt.Sprintf("%s/folder/delete?fld_id=%d&key=%s",
		f.endpoint,
		fldID,
		url.QueryEscape(f.opt.RcloneKey))

	// Make delete API request
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
			fs.Fatalf(nil, "Failed to close response body: %v", err)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fserrors.NoRetryError(fmt.Errorf("failed to read response: %w", err))
	}

	fs.Debugf(f, "Raw API Response: %s", string(body))

	// Parse API response
	var result struct {
		Status     int    `json:"status"`
		Msg        string `json:"msg"`
		Result     string `json:"result"`
		ServerTime string `json:"server_time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fserrors.NoRetryError(fmt.Errorf("error decoding response: %w", err))
	}

	// Handle API errors
	if result.Status != 200 {
		return fserrors.NoRetryError(fmt.Errorf("error: %s", result.Msg))
	}

	fs.Infof(f, "Successfully deleted directory '%s'", fullPath)
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
	fileCode := o.fs.root
	if fileCode == "" {
		fileCode = o.remote
	}

	directLink, size, err := o.fs.getDirectLink(ctx, fileCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct link: %w", err)
	}

	o.size = size
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

// extractFileName helper function to extract filename from URL
func extractFileName(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return path.Base(u.Path)
}

// deleteFileByCode deletes a object from FileLu by its file code
//
//lint:ignore unused
/*func (f *Fs) deleteFileByCode(ctx context.Context, fileCode string) error {
	fs.Debugf(f, "deleteFileByCode: Attempting to delete file with code=%q", fileCode)
	defer fs.Debugf(f, "deleteFileByCode: Finished deleting file with code=%q", fileCode)

	apiURL := fmt.Sprintf("%s/file/remove?file_code=%s&remove=1&key=%s",
		f.endpoint,
		url.QueryEscape(fileCode),
		url.QueryEscape(f.opt.RcloneKey),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
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
		return fmt.Errorf("error decoding delete response: %w", err)
	}

	if result.Status != 200 {
		return fmt.Errorf("error while deleting file: %s", result.Msg)
	}

	return nil
}
*/
// Update updates the object with new data
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	fs.Debugf(o.fs, "Update: Starting update for %q", o.remote)
	defer fs.Debugf(o.fs, "Update: Finished update for %q", o.remote)

	// Step 1: Get upload server details
	uploadURL, sessID, err := o.fs.getUploadServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get upload server: %w", err)
	}
	fs.Debugf(o.fs, "Update: Got upload server URL=%q and session ID=%q", uploadURL, sessID)

	// Step 2: Upload the file
	fileCode, err := o.fs.uploadFile(ctx, uploadURL, sessID, o.remote, in)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	fs.Debugf(o.fs, "Update: File uploaded with file code %q", fileCode)

	// Step 3: Move the file to the specified folder if necessary
	if o.fs.root != "" {
		folderID, err := strconv.Atoi(o.fs.root)
		if err != nil {
			return fmt.Errorf("invalid folder ID in root: %w", err)
		}

		if folderID != 0 { // Only attempt to move if folder ID is valid
			err = o.fs.moveFileToFolder(ctx, fileCode, folderID)
			if err != nil {
				return fserrors.NoRetryError(fmt.Errorf("failed to move file to folder ID %d: %w", folderID, err))
			}
			fs.Debugf(o.fs, "Update: File moved to folder ID %d", folderID)
		} else {
			fs.Debugf(o.fs, "Update: No folder ID specified, keeping file in the root directory")
		}
	}

	// Step 4: Update the object metadata
	o.size = src.Size()
	o.modTime = src.ModTime(ctx)

	return nil
}

// Remove deletes the object from FileLu
func (o *Object) Remove(ctx context.Context) error {
    var fileCode string

    // If the root is a valid file code, use it
    if isFileCode(o.fs.root) {
        fileCode = o.fs.root
    } else {
        // Otherwise, try to extract file code from the remote path
        remote := o.remote
        if strings.HasPrefix(remote, "(") {
            end := strings.Index(remote, ")")
            if end != -1 {
                fileCode = strings.TrimSpace(remote[1:end])
            }
        }
    }

    if !isFileCode(fileCode) {
        return fmt.Errorf("invalid file code: %q", fileCode)
    }

    return o.fs.DeleteFile(ctx, fileCode)
}

// readMetaData fetches metadata for the object
//
//nolint:unused
func (o *Object) readMetaData(ctx context.Context) error {
	apiURL := fmt.Sprintf("%s/file/info?name=%s&key=%s", o.fs.endpoint, url.QueryEscape(o.remote), url.QueryEscape(o.fs.opt.RcloneKey))

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
	return fmt.Sprintf("file hash %s already exists", e.Hash)
}
// IsDuplicateFileError checks if the given error indicates a duplicate file.
func IsDuplicateFileError(err error) bool {
	_, ok := err.(*DuplicateFileError)
	return ok
}

// FetchRemoteFileHashes retrieves hashes of remote files in a folder
func (f *Fs) FetchRemoteFileHashes(ctx context.Context, folderID int) (map[string]struct{}, error) {
    apiURL := fmt.Sprintf("%s/folder/list?fld_id=%d&key=%s", f.endpoint, folderID, url.QueryEscape(f.opt.RcloneKey))
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
// ComputeMD5 computes the MD5 hash of specified file parts
func ComputeMD5(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", fmt.Errorf("failed to open file: %w", err)
    }
    defer func() {
    if err := file.Close(); err != nil {
        fs.Logf(nil, "Failed to close file: %v", err.Error())
    }
}()


    const partSize = 1024
    firstPart := make([]byte, partSize)
    lastPart := make([]byte, partSize)

    // Read the first part of the file
    _, err = io.ReadFull(file, firstPart)
    if err != nil && err != io.EOF {
        return "", fmt.Errorf("failed to read first part: %w", err)
    }

    fileStat, err := file.Stat()
    if err != nil {
        return "", fmt.Errorf("failed to stat file: %w", err)
    }

    if fileStat.Size() > partSize {
        // Read the last part if file size is greater than partSize
        _, err = file.Seek(-partSize, io.SeekEnd)
        if err != nil {
            return "", fmt.Errorf("failed to seek to last part: %w", err)
        }

        _, err = io.ReadFull(file, lastPart)
        if err != nil && err != io.EOF {
            return "", fmt.Errorf("failed to read last part: %w", err)
        }
    } else {
        // If the file is too small, use firstPart for both segments
        copy(lastPart, firstPart)
    }

    // Create buffer containing the two parts
    buffer := append(firstPart, lastPart...)

    // Compute the MD5 hash
    fullHash := md5.Sum(buffer)

    // Convert the hash to a base64 string
    return base64.RawStdEncoding.EncodeToString(fullHash[:]), nil
}
func (f *Fs) uploadFile(ctx context.Context, uploadURL, sessionID, fileName string, fileContent io.Reader) (string, error) {
    // Convert fileContent to a temporary file for hashing and further operations
    tempFile, err := createTempFileFromReader(fileContent)
    if err != nil {
        return "", fmt.Errorf("failed to create temp file: %w", err)
    }
    err = os.Remove("file_path")
if err != nil {
    // Handle the error appropriately
    fs.Logf(nil, "Failed to remove file: %v", err.Error())
}

    // Compute the MD5 hash of the file
    hash, err := ComputeMD5(tempFile.Name())
    if err != nil {
        return "", fmt.Errorf("failed to compute file hash: %w", err)
    }

    // Log the computed hash for debugging
    fs.Debugf(f, "Computed local hash for file %q: %s", fileName, hash)

    // Convert folderID from string to int
folderIDInt, err := strconv.Atoi(f.folderID)
if err != nil {
    fs.Errorf(f, "Error parsing folderID (expected numerical string): %v", err)
    return "", fmt.Errorf("invalid folder ID, cannot proceed")
}
fs.Debugf(f, "Using folder ID: %d", folderIDInt)

    // Ensure folderIDInt is included in the combined hash
    //fmt.Printf("Computed hash: %s\n", hash)
   // fmt.Printf("Folder ID: %d\n", folderIDInt)

    // Combine local hash and folderID for comparison
    combinedHash := fmt.Sprintf("%s%d", hash, folderIDInt)
    fs.Debugf(f, "Combined hash: %s", combinedHash)

    // Fetch existing remote hashes for the given folder ID
    existingHashes, err := f.FetchRemoteFileHashes(ctx, folderIDInt)
    if err != nil {
        return "", fmt.Errorf("failed to fetch remote file hashes: %w", err)
    }

    fs.Debugf(f, "Fetched remote hashes: %v", existingHashes)

    // Check for duplicate file hash using the combined hash
    if _, exists := existingHashes[combinedHash]; exists {
        fs.Infof(f, "Duplicate file detected with combined hash %s, upload skipped.", combinedHash)
        return "", &DuplicateFileError{Hash: combinedHash}
    }

    // Further code for file upload...

    // Build the multipart request to upload the file
    var body bytes.Buffer
    writer := multipart.NewWriter(&body)

    err = writer.WriteField("sess_id", sessionID)
    if err != nil {
        return "", fmt.Errorf("failed to add sess_id field: %w", err)
    }
    err = writer.WriteField("upload_type", "rclone")
    if err != nil {
        return "", fmt.Errorf("failed to add upload_type field: %w", err)
    }
    err = writer.WriteField("utype", "prem")
    if err != nil {
        return "", fmt.Errorf("failed to add utype field: %w", err)
    }

    // Create the file part for the multipart form
    part, err := writer.CreateFormFile("file_0", fileName)
    if err != nil {
        return "", fmt.Errorf("failed to create form file: %w", err)
    }
    _, err = io.Copy(part, tempFile)
    if err != nil {
        return "", fmt.Errorf("failed to copy file content: %w", err)
    }

    err = writer.Close()
    if err != nil {
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
            fmt.Printf("Error closing response body: %v\n", cerr)
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
// Hash returns the hash of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}

	// Fetch hash from FileLu
	apiURL := fmt.Sprintf("%s/file/info?name=%s&key=%s", o.fs.endpoint, url.QueryEscape(o.remote), url.QueryEscape(o.fs.opt.RcloneKey))

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
		Hash   string `json:"hash"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("error decoding hash response: %w", err)
	}

	if result.Status != 200 {
		return "", fmt.Errorf("error: %s", result.Msg)
	}

	return result.Hash, nil
}

// String returns a string representation of the object
func (o *Object) String() string {
	return o.remote
}
