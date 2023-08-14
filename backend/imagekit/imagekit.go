package imagekit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/imagekit/client"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/version"
)

const (
	minSleep      = 1 * time.Second
	maxSleep      = 60 * time.Second
	decayConstant = 2
)

var systemMetadataInfo = map[string]fs.MetadataHelp{
	"btime": {
		Help:     "Time of file birth (creation) read from Last-Modified header",
		Type:     "RFC 3339",
		Example:  "2006-01-02T15:04:05.999999999Z07:00",
		ReadOnly: true,
	},
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "imagekit",
		Description: "ImageKit.io",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			System: systemMetadataInfo,
			Help:   `Any metadata supported by the underlying remote is read and written.`,
		},
		Options: []fs.Option{
			{
				Name:     "endpoint",
				Help:     "You can find your ImageKit.io URL endpoint in your [dashboard](https://imagekit.io/dashboard/developer/api-keys)",
				Required: true,
			},
			{
				Name:     "public_key",
				Help:     "You can find your ImageKit.io public key in your [dashboard](https://imagekit.io/dashboard/developer/api-keys)",
				Required: true,
			},
			{
				Name:     "private_key",
				Help:     "You can find your ImageKit.io private key in your [dashboard](https://imagekit.io/dashboard/developer/api-keys)",
				Required: true,
			},
			{
				Name:     "only_signed",
				Help:     "If you have configured `Restrict unsigned image URLs` in your dashboard settings, set this to true.",
				Default:  false,
				Advanced: true,
			},
			{
				Name:     "versions",
				Help:     "Include old versions in directory listings.",
				Default:  false,
				Advanced: true,
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.EncodeZero |
					encoder.EncodeSlash |
					encoder.EncodeQuestion |
					encoder.EncodeHashPercent |
					encoder.EncodeCtl |
					encoder.EncodeDel |
					encoder.EncodeDot |
					encoder.EncodeDoubleQuote |
					encoder.EncodePercent |
					encoder.EncodeBackSlash |
					encoder.EncodeDollar |
					encoder.EncodeLtGt |
					encoder.EncodeSquareBracket |
					encoder.EncodeInvalidUtf8),
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Endpoint   string               `config:"endpoint"`
	PublicKey  string               `config:"public_key"`
	PrivateKey string               `config:"private_key"`
	OnlySigned bool                 `config:"only_signed"`
	Versions   bool                 `config:"versions"`
	Enc        encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote to ImageKit
type Fs struct {
	name     string           // name of remote
	root     string           // root path
	opt      *Options         // parsed options
	features *fs.Features     // optional features
	ik       *client.ImageKit // ImageKit client
	pacer    *fs.Pacer        // pacer for API calls
}

// Object describes a ImageKit file
type Object struct {
	fs          *Fs               // The Fs this object is part of
	remote      string            // The remote path
	filePath    string            // The path to the file
	contentType string            // The content type of the object if known - may be ""
	timestamp   time.Time         // The timestamp of the object if known - may be zero
	file        client.File       // The media file if known - may be nil
	versionID   string            // If present this points to an object version
	meta        map[string]string // The object metadata if known - may be nil - with lower case keys
}

// Check the interfaces are satisfied.
var (
	_ fs.Fs = &Fs{}
	// _ fs.Purger       = &Fs{}
	// _ fs.PublicLinker = &Fs{}
	_ fs.Object = &Object{}
	// _ fs.Copier       = &Fs{}
)

func NewFs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)

	if err != nil {
		return nil, err
	}

	ik, err := client.New(ctx, client.NewParams{
		UrlEndpoint: opt.Endpoint,
		PublicKey:   opt.PublicKey,
		PrivateKey:  opt.PrivateKey,
	})

	if err != nil {
		fs.Debugf(err, "Failed to create ImageKit client")
		return nil, err
	}

	fs.Debugf(ik, "ImageKit client created")

	f := &Fs{
		name:  name,
		opt:   opt,
		ik:    ik,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.root = path.Join("/", root)

	fs.Debugf("Root is %q", f.root)

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           false,
		CanHaveEmptyDirectories: true,
		BucketBased:             false,
		ServerSideAcrossConfigs: false,
		IsLocal:                 false,
		SlowHash:                true,
		ReadMetadata:            true,
		WriteMetadata:           false,
		UserMetadata:            false,
		FilterAware:             true,
		PartialUploads:          false,
		NoMultiThreading:        false,
	}).Fill(ctx, f)

	if f.root != "/" {

		r := f.root

		folderPath := f.EncodePath(r[:strings.LastIndex(r, "/")+1])
		fileName := f.EncodeFileName(r[strings.LastIndex(r, "/")+1:])

		file := f.getFileByName(ctx, folderPath, fileName)

		if file != nil {
			newRoot := path.Dir(f.root)
			f.root = newRoot
			return f, fs.ErrorIsFile
		}

	}
	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return strings.TrimLeft(f.root, "/")
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("FS imagekit: %s", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash types of the filesystem.
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Features returns the optional features of this Fs.
func (f *Fs) Features() *fs.Features {
	return f.features
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

	remote := path.Join(f.root, dir)

	remote = f.EncodePath(remote)

	fs.Debugf(remote, "Listing directory")

	if remote != "/" {

		parentFolderPath := remote[:strings.LastIndex(remote, "/")+1]
		folderName := remote[strings.LastIndex(remote, "/")+1:]

		folderExists := f.getFolderByName(ctx, parentFolderPath, folderName)

		if folderExists == nil {
			return make(fs.DirEntries, 0), fs.ErrorDirNotFound
		}
	}

	folders, folderError := f.getFolders(ctx, remote)

	if folderError != nil {
		return make(fs.DirEntries, 0), folderError
	}

	files, fileError := f.getFiles(ctx, remote, f.opt.Versions)

	if fileError != nil {
		return make(fs.DirEntries, 0), fileError
	}

	res := make([]fs.DirEntry, 0, len(folders)+len(files))

	for _, folder := range folders {
		folderPath := f.DecodePath(strings.TrimLeft(strings.Replace(folder.FolderPath, f.EncodePath(f.root), "", 1), "/"))
		res = append(res, fs.NewDir(folderPath, folder.UpdatedAt))
	}

	for _, file := range files {
		res = append(res, f.newObject(ctx, remote, file))
	}

	return res, nil
}

func (f *Fs) newObject(ctx context.Context, remote string, file client.File) *Object {
	remoteFile := strings.TrimLeft(strings.Replace(file.FilePath, f.EncodePath(f.root), "", 1), "/")

	folderPath := f.DecodePath(remoteFile[:strings.LastIndex(remoteFile, "/")+1])
	fileName := f.DecodeFileName(remoteFile[strings.LastIndex(remoteFile, "/")+1:])

	remoteFile = path.Join(folderPath, fileName)

	if file.Type == "file-version" {
		remoteFile = version.Add(remoteFile, file.UpdatedAt)

		return &Object{
			fs:          f,
			remote:      remoteFile,
			filePath:    file.FilePath,
			contentType: file.Mime,
			timestamp:   file.UpdatedAt,
			file:        file,
			versionID:   file.VersionInfo["id"],
		}
	}

	return &Object{
		fs:          f,
		remote:      remoteFile,
		filePath:    file.FilePath,
		contentType: file.Mime,
		timestamp:   file.UpdatedAt,
		file:        file,
	}
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	r := path.Join(f.root, remote)

	fs.Debugf(r, "Getting object")

	folderPath := f.EncodePath(r[:strings.LastIndex(r, "/")+1])
	fileName := f.EncodeFileName(r[strings.LastIndex(r, "/")+1:])

	isFolder := f.getFolderByName(ctx, folderPath, fileName)

	if isFolder != nil {
		return nil, fs.ErrorIsDir
	}

	file := f.getFileByName(ctx, folderPath, fileName)

	if file == nil {
		return nil, fs.ErrorObjectNotFound
	}

	return f.newObject(ctx, r, *file), nil
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
	return uploadFile(ctx, f, in, src.Remote(), options...)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	remote := path.Join(f.root, dir)

	fs.Debugf(remote, "Creating directory")

	parentFolderPath := f.EncodePath(remote[:strings.LastIndex(remote, "/")+1])
	folderName := f.EncodePath(remote[strings.LastIndex(remote, "/")+1:])

	_, e := f.ik.CreateFolder(ctx, client.CreateFolderParam{
		ParentFolderPath: parentFolderPath,
		FolderName:       folderName,
	})

	if e != nil {
		return e
	}

	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fs.Debugf(dir, "Removing directory")

	entries, err := f.List(ctx, dir)

	if err != nil {
		return err
	}

	if len(entries) > 0 {
		return errors.New("directory is not empty")
	}

	res, e := f.ik.DeleteFolder(ctx, client.DeleteFolderParam{
		FolderPath: f.EncodePath(path.Join(f.root, dir)),
	})

	if res.StatusCode == http.StatusNotFound {
		return fs.ErrorDirNotFound
	}

	if e != nil {
		return e
	}

	return nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	fs.Debugf(dir, "Purging directory")

	remote := path.Join(f.root, dir)

	res, e := f.ik.DeleteFolder(ctx, client.DeleteFolderParam{
		FolderPath: f.EncodePath(remote),
	})

	if res.StatusCode == http.StatusNotFound {
		return fs.ErrorDirNotFound
	}

	if e != nil {
		return e
	}

	return nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {

	duration := time.Duration(math.Abs(float64(expire)))

	expireSeconds := duration.Seconds()

	fileRemote := path.Join(f.root, remote)

	fs.Debugf(fileRemote, "Getting object")

	folderPath := f.EncodePath(fileRemote[:strings.LastIndex(fileRemote, "/")+1])
	fileName := f.EncodeFileName(fileRemote[strings.LastIndex(fileRemote, "/")+1:])

	file := f.getFileByName(ctx, folderPath, fileName)

	if file == nil {
		return "", fs.ErrorObjectNotFound
	}

	url, err := f.ik.Url(client.UrlParam{
		Src:           file.Url,
		Signed:        *file.IsPrivateFile || f.opt.OnlySigned,
		ExpireSeconds: int64(expireSeconds),
		QueryParameters: map[string]string{
			"updatedAt": file.UpdatedAt.String(),
		},
	})

	if err != nil {
		return "", err
	}

	return url, nil
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.file.Name
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(context.Context) time.Time {
	return o.file.UpdatedAt
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return int64(o.file.Size)
}

// MimeType returns the MIME type of the file
func (o *Object) MimeType(context.Context) string {
	return o.contentType
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Offset and Count for range download
	var offset int64
	var count int64

	fs.FixRangeOption(options, -1)
	partialContent := false
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, count = x.Decode(-1)
			partialContent = true
		case *fs.SeekOption:
			offset = x.Offset
			partialContent = true
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	url, err := o.fs.ik.Url(client.UrlParam{
		Src:    o.file.Url,
		Signed: *o.file.IsPrivateFile || o.fs.opt.OnlySigned,
		QueryParameters: map[string]string{
			"tr":        "orig-true",
			"updatedAt": o.file.UpdatedAt.String(),
		},
	})

	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+count-1))
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	end := resp.ContentLength

	if partialContent && resp.StatusCode == http.StatusOK {
		skip := offset

		if offset < 0 {
			skip = end + offset + 1
		}

		_, err = io.CopyN(io.Discard, resp.Body, skip)
		if err != nil {
			if resp != nil {
				_ = resp.Body.Close()
			}
			return nil, err
		}

		return readers.NewLimitedReadCloser(resp.Body, end-skip), nil
	}

	return resp.Body, nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	fs.Debugf(o.remote, "Updating file")

	srcRemote := o.Remote()

	remote := path.Join(o.fs.root, srcRemote)
	folderPath := o.fs.EncodePath(remote[:strings.LastIndex(remote, "/")+1])
	fileName := o.fs.EncodeFileName(remote[strings.LastIndex(remote, "/")+1:])

	UseUniqueFileName := new(bool)
	*UseUniqueFileName = false

	_, res, err := o.fs.ik.Upload(ctx, in, client.UploadParam{
		FileName:          fileName,
		Folder:            folderPath,
		UseUniqueFileName: UseUniqueFileName,
	})

	if err != nil {
		return err
	}

	fileId := res.FileId

	_, file, err := o.fs.ik.File(ctx, fileId)

	if err != nil {
		return err
	}

	o.file = *file

	return nil
}

// Removes this object
func (o *Object) Remove(ctx context.Context) error {

	fs.Debugf(o.remote, "Removing file: %s", o.file.FileId)

	_, e := o.fs.ik.DeleteFile(ctx, o.file.FileId)

	if e != nil {
		return e
	}

	return nil
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

func uploadFile(ctx context.Context, f *Fs, in io.Reader, srcRemote string, options ...fs.OpenOption) (fs.Object, error) {

	fs.Debugf(srcRemote, "Uploading file")

	remote := path.Join(f.root, srcRemote)
	folderPath := f.EncodePath(remote[:strings.LastIndex(remote, "/")+1])
	fileName := f.EncodeFileName(remote[strings.LastIndex(remote, "/")+1:])

	UseUniqueFileName := new(bool)
	*UseUniqueFileName = false

	_, _, err := f.ik.Upload(ctx, in, client.UploadParam{
		FileName:          fileName,
		Folder:            folderPath,
		UseUniqueFileName: UseUniqueFileName,
	})

	if err != nil {
		return nil, err
	}

	return f.NewObject(ctx, srcRemote)
}

func (o *Object) readMetaData(ctx context.Context) error {

	if o.meta != nil {
		return nil
	}

	meta := make(map[string]string)

	meta["btime"] = o.file.CreatedAt.Format(time.RFC3339)

	meta["size"] = strconv.FormatUint(o.file.Size, 10)

	meta["fileType"] = string(o.file.FileType)
	meta["height"] = strconv.Itoa(o.file.Height)
	meta["width"] = strconv.Itoa(o.file.Width)
	meta["hasAlpha"] = strconv.FormatBool(o.file.HasAlpha)

	for k, v := range o.file.EmbeddedMetadata {
		switch v := v.(type) {
		case string:
			meta[k] = v
		case float64:
			meta[k] = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			meta[k] = strconv.FormatBool(v)
		default:
			meta[k] = fmt.Sprintf("%v", v)
		}
	}

	if o.file.Tags != nil {
		meta["tags"] = strings.Join(o.file.Tags, ",")
	}

	if o.file.CustomCoordinates != nil {
		meta["customCoordinates"] = *o.file.CustomCoordinates
	}

	if o.file.IsPrivateFile != nil {
		meta["isPrivateFile"] = strconv.FormatBool(*o.file.IsPrivateFile)
	}

	if o.file.AITags != nil {
		googleTags := []string{}
		awsTags := []string{}

		for _, tag := range o.file.AITags {
			if tag.Source == "google-auto-tagging" {
				googleTags = append(googleTags, tag.Name)
			} else if tag.Source == "aws-auto-tagging" {
				awsTags = append(awsTags, tag.Name)
			}
		}

		if len(googleTags) > 0 {
			meta["googleTags"] = strings.Join(googleTags, ",")
		}

		if len(awsTags) > 0 {
			meta["awsTags"] = strings.Join(awsTags, ",")
		}
	}

	o.meta = meta

	return nil

}

func (o *Object) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {

	err = o.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	metadata = make(fs.Metadata, len(o.meta))

	for k, v := range o.meta {
		switch k {
		default:
			metadata[k] = v
		}
	}

	return metadata, nil
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantMove
	}

	file, err := srcObj.Open(ctx)

	if err != nil {
		return nil, err
	}

	return uploadFile(ctx, f, file, remote)
}
