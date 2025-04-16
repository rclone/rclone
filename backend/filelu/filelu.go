// Package filelu provides an interface to the FileLu storage system.
package filelu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
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
		},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Base | //  Slash,LtGt,DoubleQuote,Question,Asterisk,Pipe,Hash,Percent,BackSlash,Del,Ctl,RightSpace,InvalidUtf8,Dot
					encoder.EncodeSlash |
					encoder.EncodeLtGt |
					encoder.EncodeDoubleQuote |
					encoder.EncodeQuestion |
					encoder.EncodeAsterisk |
					encoder.EncodePipe |
					encoder.EncodeHash |
					encoder.EncodePercent |
					encoder.EncodeBackSlash |
					encoder.EncodeDel |
					encoder.EncodeCtl |
					encoder.EncodeRightSpace |
					encoder.EncodeInvalidUtf8 |
					encoder.EncodeDot),
			},
		}})
}

// Options defines the configuration for the FileLu backend
type Options struct {
	Key string               `config:"key"`
	Enc encoder.MultiEncoder `config:"encoding"`
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

// NewFs creates a new Fs object for FileLu
func NewFs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
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
		opt:        *opt,
		endpoint:   "https://filelu.com/rclone",
		client:     client,
		srv:        rest.NewClient(client).SetRoot("https://filelu.com/rclone"),
		pacer:      pacer.New(),
		isFile:     isFile,
		targetFile: filename,
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		WriteMetadata:           false,
		ListR:                   f.ListR,
	}).Fill(ctx, f)

	f.root = cleanRoot

	if isFile {
		code, err := f.getFileCode(ctx, root)
		if errors.Is(err, FileNotFound) || errors.Is(err, fs.ErrorDirNotFound) {
			return f, nil
		}

		if err != nil {
			return f, err
		}

		if code != "" {
			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// Mkdir to create directory on remote server.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fullPath := path.Clean(f.root + "/" + dir)
	_, err := f.createFolder(ctx, fullPath)
	return err
}

// DeleteFile sends an API request to remove a file from FileLu
func (f *Fs) DeleteFile(ctx context.Context, filePath string) error {

	filePath = "/" + strings.Trim(filePath, "/")
	return f.deleteFile(ctx, filePath)
}

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
		} else {
			return nil, fmt.Errorf("please specify a file to move")
		}

		destinationPath := args[0]

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

		err := f.moveFolderToDestination(ctx, sourcePath, destinationPath)
		if err != nil {
			return nil, fmt.Errorf("folder move failed: %w", err)
		}

		return nil, nil

	// Handle renamefolder case in Command method
	case "renamefolder":

		if len(args) != 1 {
			return nil, fmt.Errorf("renamefolder command requires new_name argument")
		}

		folderPath := f.root
		newName := args[0]

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

// About provides usage statistics for the remote
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	accountInfo, err := f.getAccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	totalStorage, err := parseStorageToBytes(accountInfo.Result.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total storage: %w", err)
	}

	usedStorage, err := parseStorageToBytes(accountInfo.Result.StorageUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse used storage: %w", err)
	}

	return &fs.Usage{
		Total: fs.NewUsageValue(totalStorage), // Total bytes available
		Used:  fs.NewUsageValue(usedStorage),  // Total bytes used
		Free:  fs.NewUsageValue(totalStorage - usedStorage),
	}, nil
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

func (f *Fs) Purge(ctx context.Context, dir string) error {

	fullPath := path.Join(f.root, dir)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}

	// Step 1: Check if folder is empty
	_, err := f.getFolderList(ctx, fullPath)
	if err != nil {
		return err
	}

	// Step 2: Delete the folder
	return f.deleteFolder(ctx, fullPath)
}

// List returns a list of files and folders
// List returns a list of files and folders for the given directory
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {

	if f.isFile {
		obj, err := f.NewObject(ctx, f.targetFile)
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return []fs.DirEntry{}, nil
		}
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

	var entries fs.DirEntries
	result, err := f.getFolderList(ctx, fullPath)
	if err != nil {
		return nil, err
	}

	fldMap := map[string]bool{}

	for _, folder := range result.Result.Folders {
		fldMap[folder.FldID.String()] = true
		if f.root == "" && dir == "" && strings.Contains(folder.Path, "/") {
			continue
		}

		decodedPath := f.ToStandardPath(folder.Path)
		paths := strings.Split(decodedPath, fullPath+"/")
		remote := paths[0]
		if len(paths) > 1 {
			remote = paths[1]
		}

		if strings.Contains(remote, "/") {
			continue
		}

		pathsWithoutRoot := strings.Split(decodedPath, "/"+f.root+"/")
		remotePathWithoutRoot := pathsWithoutRoot[0]
		if len(pathsWithoutRoot) > 1 {
			remotePathWithoutRoot = pathsWithoutRoot[1]
		}
		remotePathWithoutRoot = strings.TrimPrefix(remotePathWithoutRoot, "/")
		entries = append(entries, fs.NewDir(remotePathWithoutRoot, time.Now()))
	}

	for _, file := range result.Result.Files {
		if _, ok := fldMap[file.FldID.String()]; ok {
			continue
		}
		remote := path.Join(dir, file.Name)
		// trim leading slashes
		remote = strings.TrimPrefix(remote, "/")
		size := int64(0)

		if fileInfo, err := f.getFileInfo(ctx, file.FileCode); err == nil {
			if len(fileInfo.Result) > 0 {
				size, _ = strconv.ParseInt(fileInfo.Result[0].Size, 10, 64)
			}
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
			err := f.walkDir(ctx, d.Remote(), callback)
			if err != nil {
				return err
			}
		}
	}

	return nil
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
				// fld_id, err := strconv.Atoi()
				// if err != nil {
				// 	continue
				// }
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

// Put uploads a file directly to the destination folder in the FileLu storage system.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if src.Size() == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	err := f.UploadFile(ctx, in, src.Remote())
	if err != nil {
		return nil, err
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

// Move the objects and directories
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	// Check if the source is a directory
	if srcDir, ok := src.(fs.Directory); ok {
		// Recursively move all contents
		err := f.moveDirectoryContents(ctx, srcDir.Remote(), remote)
		if err != nil {
			return nil, fmt.Errorf("failed to move directory contents: %w", err)
		}
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

// MoveTo moves the file to the specified location
func (f *Fs) MoveTo(ctx context.Context, src fs.Object, destinationPath string) (fs.Object, error) {

	if strings.HasPrefix(destinationPath, "/") || strings.Contains(destinationPath, ":\\") {
		dir := path.Dir(destinationPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}

		reader, err := src.Open(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to open source file: %w", err)
		}
		defer reader.Close()

		dest, err := os.Create(destinationPath)
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

	err = f.UploadFile(ctx, reader, destinationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to destination: %w", err)
	}

	if err := src.Remove(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete source file: %w", err)
	}

	return &Object{
		fs:      f,
		remote:  destinationPath,
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}, nil
}

// MoveToLocal moves the file or folder to the local file system.
// It implements the fs.Fs interface and performs the move operation locally.
func (f *Fs) MoveToLocal(ctx context.Context, remote string, localPath string) error {

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

	return nil
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {

	fullPath := path.Join(f.root, dir)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}

	// Step 1: Check if folder is empty
	listResp, err := f.getFolderList(ctx, fullPath)
	if err != nil {
		return err
	}
	if len(listResp.Result.Files) > 0 || len(listResp.Result.Folders) > 0 {
		return fmt.Errorf("Rmdir: directory %q is not empty", fullPath)
	}

	// Step 2: Delete the folder
	return f.deleteFolder(ctx, fullPath)
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
	_ fs.ListRer = (*Fs)(nil)
)
