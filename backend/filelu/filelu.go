// Package filelu provides an interface to the FileLu storage system.
package filelu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
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
					encoder.EncodeExclamation |
					encoder.EncodeDoubleQuote |
					encoder.EncodeSingleQuote |
					encoder.EncodeBackQuote |
					encoder.EncodeQuestion |
					encoder.EncodeDollar |
					encoder.EncodeColon |
					encoder.EncodeAsterisk |
					encoder.EncodePipe |
					encoder.EncodeHash |
					encoder.EncodePercent |
					encoder.EncodeBackSlash |
					encoder.EncodeCrLf |
					encoder.EncodeDel |
					encoder.EncodeCtl |
					encoder.EncodeLeftSpace |
					encoder.EncodeLeftPeriod |
					encoder.EncodeLeftTilde |
					encoder.EncodeLeftCrLfHtVt |
					encoder.EncodeRightPeriod |
					encoder.EncodeRightCrLfHtVt |
					encoder.EncodeSquareBracket |
					encoder.EncodeSemicolon |
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
	root = strings.Trim(root, "/")

	filename := ""

	f := &Fs{
		name:       name,
		opt:        *opt,
		endpoint:   "https://filelu.com/rclone",
		client:     client,
		srv:        rest.NewClient(client).SetRoot("https://filelu.com/rclone"),
		pacer:      pacer.New(),
		targetFile: filename,
		root:       root,
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		WriteMetadata:           false,
		SlowHash:                true,
	}).Fill(ctx, f)

	rootContainer, rootDirectory := rootSplit(f.root)
	if rootContainer != "" && rootDirectory != "" {
		// Check to see if the (container,directory) is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		f.root = strings.Trim(newRoot, "/")
		_, err := f.NewObject(ctx, leaf)
		if err != nil {
			if err == fs.ErrorObjectNotFound || err == fs.ErrorNotAFile {
				// File doesn't exist or is a directory so return old f
				f.root = strings.Trim(oldRoot, "/")
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Mkdir to create directory on remote server.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fullPath := path.Clean(f.root + "/" + dir)
	_, err := f.createFolder(ctx, fullPath)
	return err
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

// Purge deletes the directory and all its contents
func (f *Fs) Purge(ctx context.Context, dir string) error {
	fullPath := path.Join(f.root, dir)
	if fullPath != "" {
		fullPath = "/" + strings.Trim(fullPath, "/")
	}
	return f.deleteFolder(ctx, fullPath)
}

// List returns a list of files and folders
// List returns a list of files and folders for the given directory
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
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

		paths := strings.Split(folder.Path, fullPath+"/")
		remote := paths[0]
		if len(paths) > 1 {
			remote = paths[1]
		}

		if strings.Contains(remote, "/") {
			continue
		}

		pathsWithoutRoot := strings.Split(folder.Path, "/"+f.root+"/")
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
		obj := &Object{
			fs:      f,
			remote:  remote,
			size:    file.Size,
			modTime: time.Now(),
		}
		entries = append(entries, obj)
	}
	return entries, nil
}

// Put uploads a file directly to the destination folder in the FileLu storage system.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if src.Size() == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	err := f.uploadFile(ctx, in, src.Remote())
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

// Move moves the file to the specified location
func (f *Fs) Move(ctx context.Context, src fs.Object, destinationPath string) (fs.Object, error) {

	if strings.HasPrefix(destinationPath, "/") || strings.Contains(destinationPath, ":\\") {
		dir := path.Dir(destinationPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}

		reader, err := src.Open(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to open source file: %w", err)
		}
		defer func() {
			if err := reader.Close(); err != nil {
				fs.Logf(nil, "Failed to close file body: %v", err)
			}
		}()

		dest, err := os.Create(destinationPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create destination file: %w", err)
		}
		defer func() {
			if err := dest.Close(); err != nil {
				fs.Logf(nil, "Failed to close file body: %v", err)
			}
		}()

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
	defer func() {
		if err := reader.Close(); err != nil {
			fs.Logf(nil, "Failed to close file body: %v", err)
		}
	}()

	err = f.uploadFile(ctx, reader, destinationPath)
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
	_ fs.Fs      = (*Fs)(nil)
	_ fs.Purger  = (*Fs)(nil)
	_ fs.Abouter = (*Fs)(nil)
	_ fs.Mover   = (*Fs)(nil)
	_ fs.Object  = (*Object)(nil)
)
