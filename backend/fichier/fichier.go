package fichier

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	rootID         = "0"
	apiBaseURL     = "https://api.1fichier.com/v1"
	minSleep       = 400 * time.Millisecond // api is extremely rate limited now
	maxSleep       = 5 * time.Second
	decayConstant  = 2 // bigger for slower decay, exponential
	attackConstant = 0 // start with max sleep
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "fichier",
		Description: "1Fichier",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Help: "Your API Key, get it from https://1fichier.com/console/params.pl",
			Name: "api_key",
		}, {
			Help:     "If you want to download a shared folder, add this parameter",
			Name:     "shared_folder",
			Required: false,
			Advanced: true,
		}, {
			Help:       "If you want to download a shared file that is password protected, add this parameter",
			Name:       "file_password",
			Required:   false,
			Advanced:   true,
			IsPassword: true,
		}, {
			Help:       "If you want to list the files in a shared folder that is password protected, add this parameter",
			Name:       "folder_password",
			Required:   false,
			Advanced:   true,
			IsPassword: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Characters that need escaping
			//
			// 		'\\': '＼', // FULLWIDTH REVERSE SOLIDUS
			// 		'<':  '＜', // FULLWIDTH LESS-THAN SIGN
			// 		'>':  '＞', // FULLWIDTH GREATER-THAN SIGN
			// 		'"':  '＂', // FULLWIDTH QUOTATION MARK - not on the list but seems to be reserved
			// 		'\'': '＇', // FULLWIDTH APOSTROPHE
			// 		'$':  '＄', // FULLWIDTH DOLLAR SIGN
			// 		'`':  '｀', // FULLWIDTH GRAVE ACCENT
			//
			// Leading space and trailing space
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeSingleQuote |
				encoder.EncodeBackQuote |
				encoder.EncodeDoubleQuote |
				encoder.EncodeLtGt |
				encoder.EncodeDollar |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	APIKey         string               `config:"api_key"`
	SharedFolder   string               `config:"shared_folder"`
	FilePassword   string               `config:"file_password"`
	FolderPassword string               `config:"folder_password"`
	Enc            encoder.MultiEncoder `config:"encoding"`
}

// Fs is the interface a cloud storage system must provide
type Fs struct {
	root       string
	name       string
	features   *fs.Features
	opt        Options
	dirCache   *dircache.DirCache
	baseClient *http.Client
	pacer      *fs.Pacer
	rest       *rest.Client
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	folderID, err := strconv.Atoi(pathID)
	if err != nil {
		return "", false, err
	}
	folders, err := f.listFolders(ctx, folderID)
	if err != nil {
		return "", false, err
	}

	for _, folder := range folders.SubFolders {
		if folder.Name == leaf {
			pathIDOut := strconv.Itoa(folder.ID)
			return pathIDOut, true, nil
		}
	}

	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	folderID, err := strconv.Atoi(pathID)
	if err != nil {
		return "", err
	}
	resp, err := f.makeFolder(ctx, leaf, folderID)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(resp.FolderID), err
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("1Fichier root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.Whirlpool)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewFs makes a new Fs object from the path
//
// The path is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
//
// On Windows avoid single character remote names as they can be mixed
// up with drive letters.
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(config, opt)
	if err != nil {
		return nil, err
	}

	// If using a Shared Folder override root
	if opt.SharedFolder != "" {
		root = ""
	}

	//workaround for wonky parser
	root = strings.Trim(root, "/")

	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant), pacer.AttackConstant(attackConstant))),
		baseClient: &http.Client{},
	}

	f.features = (&fs.Features{
		DuplicateFiles:          true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)

	client := fshttp.NewClient(ctx)

	f.rest = rest.NewClient(client).SetRoot(apiBaseURL)

	f.rest.SetHeader("Authorization", "Bearer "+f.opt.APIKey)

	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	if f.opt.SharedFolder != "" {
		return f.listSharedFiles(ctx, f.opt.SharedFolder)
	}

	dirContent, err := f.listDir(ctx, dir)
	if err != nil {
		return nil, err
	}

	return dirContent, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
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

	for _, file := range files.Items {
		if file.Filename == leaf {
			path, ok := f.dirCache.GetInv(directoryID)

			if !ok {
				return nil, errors.New("Cannot find dir in dircache")
			}

			return f.newObjectFromFile(ctx, path, file), nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// putUnchecked uploads the object with the given name and size
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) putUnchecked(ctx context.Context, in io.Reader, remote string, size int64, options ...fs.OpenOption) (fs.Object, error) {
	if size > int64(300e9) {
		return nil, errors.New("File too big, cant upload")
	} else if size == 0 {
		return nil, fs.ErrorCantUploadEmptyFiles
	}

	nodeResponse, err := f.getUploadNode(ctx)
	if err != nil {
		return nil, err
	}

	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	_, err = f.uploadFile(ctx, in, size, leaf, directoryID, nodeResponse.ID, nodeResponse.URL, options...)
	if err != nil {
		return nil, err
	}

	fileUploadResponse, err := f.endUpload(ctx, nodeResponse.ID, nodeResponse.URL)
	if err != nil {
		return nil, err
	}

	if len(fileUploadResponse.Links) == 0 {
		return nil, errors.New("upload response not found")
	} else if len(fileUploadResponse.Links) > 1 {
		fs.Debugf(remote, "Multiple upload responses found, using the first")
	}

	link := fileUploadResponse.Links[0]
	fileSize, err := strconv.ParseInt(link.Size, 10, 64)

	if err != nil {
		return nil, err
	}

	return &Object{
		fs:     f,
		remote: remote,
		file: File{
			CDN:         0,
			Checksum:    link.Whirlpool,
			ContentType: "",
			Date:        time.Now().Format("2006-01-02 15:04:05"),
			Filename:    link.Filename,
			Pass:        0,
			Size:        fileSize,
			URL:         link.Download,
		},
	}, nil
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.putUnchecked(ctx, in, src.Remote(), src.Size(), options...)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	folderID, err := strconv.Atoi(directoryID)
	if err != nil {
		return err
	}

	_, err = f.removeFolder(ctx, dir, folderID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)

	return nil
}

// Move src to this remote using server side move operations.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Find current directory ID
	_, currentDirectoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote)
	if err != nil {
		return nil, err
	}

	// If it is in the correct directory, just rename it
	var url string
	if currentDirectoryID == directoryID {
		resp, err := f.renameFile(ctx, srcObj.file.URL, leaf)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't rename file")
		}
		if resp.Status != "OK" {
			return nil, errors.Errorf("couldn't rename file: %s", resp.Message)
		}
		url = resp.URLs[0].URL
	} else {
		folderID, err := strconv.Atoi(directoryID)
		if err != nil {
			return nil, err
		}
		resp, err := f.moveFile(ctx, srcObj.file.URL, folderID, leaf)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't move file")
		}
		if resp.Status != "OK" {
			return nil, errors.Errorf("couldn't move file: %s", resp.Message)
		}
		url = resp.URLs[0]
	}

	file, err := f.readFileInfo(ctx, url)
	if err != nil {
		return nil, errors.New("couldn't read file data")
	}
	dstObj.setMetaData(*file)
	return dstObj, nil
}

// Copy src to this remote using server side move operations.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote)
	if err != nil {
		return nil, err
	}

	folderID, err := strconv.Atoi(directoryID)
	if err != nil {
		return nil, err
	}
	resp, err := f.copyFile(ctx, srcObj.file.URL, folderID, leaf)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't move file")
	}
	if resp.Status != "OK" {
		return nil, errors.Errorf("couldn't move file: %s", resp.Message)
	}

	file, err := f.readFileInfo(ctx, resp.URLs[0].ToURL)
	if err != nil {
		return nil, errors.New("couldn't read file data")
	}
	dstObj.setMetaData(*file)
	return dstObj, nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return "", err
	}
	return o.(*Object).file.URL, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
)
