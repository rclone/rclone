// Package internetarchive provides an interface to Internet Archive's Item
// via their native API than using S3-compatible endpoints.
package internetarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "internetarchive",
		Description: "Internet Archive",
		NewFs:       NewFs,

		MetadataInfo: &fs.MetadataInfo{
			System: map[string]fs.MetadataHelp{
				"name": {
					Help:     "Full file path, without the bucket part",
					Type:     "filename",
					Example:  "backend/internetarchive/internetarchive.go",
					ReadOnly: true,
				},
				"source": {
					Help:     "The source of the file",
					Type:     "string",
					Example:  "original",
					ReadOnly: true,
				},
				"mtime": {
					Help:     "Time of last modification, managed by Rclone",
					Type:     "RFC 3339",
					Example:  "2006-01-02T15:04:05.999999999Z",
					ReadOnly: true,
				},
				"size": {
					Help:     "File size in bytes",
					Type:     "decimal number",
					Example:  "123456",
					ReadOnly: true,
				},
				"md5": {
					Help:     "MD5 hash calculated by Internet Archive",
					Type:     "string",
					Example:  "01234567012345670123456701234567",
					ReadOnly: true,
				},
				"crc32": {
					Help:     "CRC32 calculated by Internet Archive",
					Type:     "string",
					Example:  "01234567",
					ReadOnly: true,
				},
				"sha1": {
					Help:     "SHA1 hash calculated by Internet Archive",
					Type:     "string",
					Example:  "0123456701234567012345670123456701234567",
					ReadOnly: true,
				},
				"format": {
					Help:     "Name of format identified by Internet Archive",
					Type:     "string",
					Example:  "Comma-Separated Values",
					ReadOnly: true,
				},
				"old_version": {
					Help:     "Whether the file was replaced and moved by keep-old-version flag",
					Type:     "boolean",
					Example:  "true",
					ReadOnly: true,
				},
				"viruscheck": {
					Help:     "The last time viruscheck process was run for the file (?)",
					Type:     "unixtime",
					Example:  "1654191352",
					ReadOnly: true,
				},
				"summation": {
					Help:     "Check https://forum.rclone.org/t/31922 for how it is used",
					Type:     "string",
					Example:  "md5",
					ReadOnly: true,
				},

				"rclone-ia-mtime": {
					Help:    "Time of last modification, managed by Internet Archive",
					Type:    "RFC 3339",
					Example: "2006-01-02T15:04:05.999999999Z",
				},
				"rclone-mtime": {
					Help:    "Time of last modification, managed by Rclone",
					Type:    "RFC 3339",
					Example: "2006-01-02T15:04:05.999999999Z",
				},
				"rclone-update-track": {
					Help:    "Random value used by Rclone for tracking changes inside Internet Archive",
					Type:    "string",
					Example: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
			},
			Help: `Metadata fields provided by Internet Archive.
If there are multiple values for a key, only the first one is returned.
This is a limitation of Rclone, that supports one value per one key.

Owner is able to add custom keys. Metadata feature grabs all the keys including them.
`,
		},

		Options: []fs.Option{{
			Name:      "access_key_id",
			Help:      "IAS3 Access Key.\n\nLeave blank for anonymous access.\nYou can find one here: https://archive.org/account/s3.php",
			Sensitive: true,
		}, {
			Name:      "secret_access_key",
			Help:      "IAS3 Secret Key (password).\n\nLeave blank for anonymous access.",
			Sensitive: true,
		}, {
			// their official client (https://github.com/jjjake/internetarchive) hardcodes following the two
			Name:     "endpoint",
			Help:     "IAS3 Endpoint.\n\nLeave blank for default value.",
			Default:  "https://s3.us.archive.org",
			Advanced: true,
		}, {
			Name:     "front_endpoint",
			Help:     "Host of InternetArchive Frontend.\n\nLeave blank for default value.",
			Default:  "https://archive.org",
			Advanced: true,
		}, {
			Name: "disable_checksum",
			Help: `Don't ask the server to test against MD5 checksum calculated by rclone.
Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can ask the server to check the object against checksum.
This is great for data integrity checking but can cause long delays for
large files to start uploading.`,
			Default:  true,
			Advanced: true,
		}, {
			Name: "wait_archive",
			Help: `Timeout for waiting the server's processing tasks (specifically archive and book_op) to finish.
Only enable if you need to be guaranteed to be reflected after write operations.
0 to disable waiting. No errors to be thrown in case of timeout.`,
			Default:  fs.Duration(0),
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: encoder.EncodeZero |
				encoder.EncodeSlash |
				encoder.EncodeLtGt |
				encoder.EncodeCrLf |
				encoder.EncodeDel |
				encoder.EncodeCtl |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeDot,
		},
		}})
}

// maximum size of an item. this is constant across all items
const iaItemMaxSize int64 = 1099511627776

// metadata keys that are not writeable
var roMetadataKey = map[string]interface{}{
	// do not add mtime here, it's a documented exception
	"name": nil, "source": nil, "size": nil, "md5": nil,
	"crc32": nil, "sha1": nil, "format": nil, "old_version": nil,
	"viruscheck": nil, "summation": nil,
}

// Options defines the configuration for this backend
type Options struct {
	AccessKeyID     string               `config:"access_key_id"`
	SecretAccessKey string               `config:"secret_access_key"`
	Endpoint        string               `config:"endpoint"`
	FrontEndpoint   string               `config:"front_endpoint"`
	DisableChecksum bool                 `config:"disable_checksum"`
	WaitArchive     fs.Duration          `config:"wait_archive"`
	Enc             encoder.MultiEncoder `config:"encoding"`
}

// Fs represents an IAS3 remote
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on if any
	opt      Options      // parsed config options
	features *fs.Features // optional features
	srv      *rest.Client // the connection to IAS3
	front    *rest.Client // the connection to frontend
	pacer    *fs.Pacer    // pacer for API calls
	ctx      context.Context
}

// Object describes a file at IA
type Object struct {
	fs      *Fs       // reference to Fs
	remote  string    // the remote path
	modTime time.Time // last modified time
	size    int64     // size of the file in bytes
	md5     string    // md5 hash of the file presented by the server
	sha1    string    // sha1 hash of the file presented by the server
	crc32   string    // crc32 of the file presented by the server
	rawData json.RawMessage
}

// IAFile represents a subset of object in MetadataResponse.Files
type IAFile struct {
	Name string `json:"name"`
	// Source     string `json:"source"`
	Mtime       string          `json:"mtime"`
	RcloneMtime json.RawMessage `json:"rclone-mtime"`
	UpdateTrack json.RawMessage `json:"rclone-update-track"`
	Size        string          `json:"size"`
	Md5         string          `json:"md5"`
	Crc32       string          `json:"crc32"`
	Sha1        string          `json:"sha1"`
	Summation   string          `json:"summation"`

	rawData json.RawMessage
}

// MetadataResponse represents subset of the JSON object returned by (frontend)/metadata/
type MetadataResponse struct {
	Files    []IAFile `json:"files"`
	ItemSize int64    `json:"item_size"`
}

// MetadataResponseRaw is the form of MetadataResponse to deal with metadata
type MetadataResponseRaw struct {
	Files    []json.RawMessage `json:"files"`
	ItemSize int64             `json:"item_size"`
}

// ModMetadataResponse represents response for amending metadata
type ModMetadataResponse struct {
	// https://archive.org/services/docs/api/md-write.html#example
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	bucket, file := f.split("")
	if bucket == "" {
		return "Internet Archive root"
	}
	if file == "" {
		return fmt.Sprintf("Internet Archive item %s", bucket)
	}
	return fmt.Sprintf("Internet Archive item %s path %s", bucket, file)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns type of hashes supported by IA
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5, hash.SHA1, hash.CRC32)
}

// Precision returns the precision of mtime that the server responds
func (f *Fs) Precision() time.Duration {
	if f.opt.WaitArchive == 0 {
		return fs.ModTimeNotSupported
	}
	return time.Nanosecond
}

// retryErrorCodes is a slice of error codes that we will retry
// See: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error - "We encountered an internal error. Please try again."
	503, // Service Unavailable/Slow Down - "Reduce your request rate"
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Parse the endpoints
	ep, err := url.Parse(opt.Endpoint)
	if err != nil {
		return nil, err
	}
	fe, err := url.Parse(opt.FrontEndpoint)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name: name,
		opt:  *opt,
		ctx:  ctx,
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		BucketBased:   true,
		ReadMetadata:  true,
		WriteMetadata: true,
		UserMetadata:  true,
	}).Fill(ctx, f)

	f.srv = rest.NewClient(fshttp.NewClient(ctx))
	f.srv.SetRoot(ep.String())

	f.front = rest.NewClient(fshttp.NewClient(ctx))
	f.front.SetRoot(fe.String())

	if opt.AccessKeyID != "" && opt.SecretAccessKey != "" {
		auth := fmt.Sprintf("LOW %s:%s", opt.AccessKeyID, opt.SecretAccessKey)
		f.srv.SetHeader("Authorization", auth)
		f.front.SetHeader("Authorization", auth)
	}

	f.pacer = fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(10*time.Millisecond)))

	// test if the root exists as a file
	_, err = f.NewObject(ctx, "/")
	if err == nil {
		f.setRoot(betterPathDir(root))
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = strings.Trim(root, "/")
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.size
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the hash value presented by IA
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty == hash.MD5 {
		return o.md5, nil
	}
	if ty == hash.SHA1 {
		return o.sha1, nil
	}
	if ty == hash.CRC32 {
		return o.crc32, nil
	}
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets modTime on a particular file
func (o *Object) SetModTime(ctx context.Context, t time.Time) (err error) {
	bucket, reqDir := o.split()
	if bucket == "" {
		return fs.ErrorCantSetModTime
	}
	if reqDir == "" {
		return fs.ErrorCantSetModTime
	}

	// https://archive.org/services/docs/api/md-write.html
	// the following code might be useful for modifying metadata of an uploaded file
	patch := []map[string]string{
		// we should drop it first to clear all rclone-provided mtimes
		{
			"op":   "remove",
			"path": "/rclone-mtime",
		}, {
			"op":    "add",
			"path":  "/rclone-mtime",
			"value": t.Format(time.RFC3339Nano),
		}}
	res, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Add("-target", fmt.Sprintf("files/%s", reqDir))
	params.Add("-patch", string(res))
	body := []byte(params.Encode())
	bodyLen := int64(len(body))

	var resp *http.Response
	var result ModMetadataResponse
	// make a POST request to (frontend)/metadata/:item/
	opts := rest.Opts{
		Method:        "POST",
		Path:          path.Join("/metadata/", bucket),
		Body:          bytes.NewReader(body),
		ContentLength: &bodyLen,
		ContentType:   "application/x-www-form-urlencoded",
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.front.CallJSON(ctx, &opts, nil, &result)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}

	if result.Success {
		o.modTime = t
		return nil
	}

	return errors.New(result.Error)
}

// List files and directories in a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	bucket, reqDir := f.split(dir)
	if bucket == "" {
		if reqDir != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return entries, nil
	}
	grandparent := f.opt.Enc.ToStandardPath(strings.Trim(path.Join(bucket, reqDir), "/") + "/")

	allEntries, err := f.listAllUnconstrained(ctx, bucket)
	if err != nil {
		return entries, err
	}
	for _, ent := range allEntries {
		obj, ok := ent.(*Object)
		if ok && strings.HasPrefix(obj.remote, grandparent) {
			path := trimPathPrefix(obj.remote, grandparent, f.opt.Enc)
			if !strings.Contains(path, "/") {
				obj.remote = trimPathPrefix(obj.remote, f.root, f.opt.Enc)
				entries = append(entries, obj)
			}
		}
		dire, ok := ent.(*fs.Dir)
		if ok && strings.HasPrefix(dire.Remote(), grandparent) {
			path := trimPathPrefix(dire.Remote(), grandparent, f.opt.Enc)
			if !strings.Contains(path, "/") {
				dire.SetRemote(trimPathPrefix(dire.Remote(), f.root, f.opt.Enc))
				entries = append(entries, dire)
			}
		}
	}

	return entries, nil
}

// Mkdir can't be performed on IA like git repositories
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	return nil
}

// Rmdir as well, unless we're asked for recursive deletion
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (ret fs.Object, err error) {
	bucket, filepath := f.split(remote)
	filepath = strings.Trim(filepath, "/")
	if bucket == "" {
		if filepath != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return nil, fs.ErrorIsDir
	}

	grandparent := f.opt.Enc.ToStandardPath(strings.Trim(path.Join(bucket, filepath), "/"))

	allEntries, err := f.listAllUnconstrained(ctx, bucket)
	if err != nil {
		return nil, err
	}
	for _, ent := range allEntries {
		obj, ok := ent.(*Object)
		if ok && obj.remote == grandparent {
			obj.remote = trimPathPrefix(obj.remote, f.root, f.opt.Enc)
			return obj, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:      f,
		remote:  src.Remote(),
		modTime: src.ModTime(ctx),
		size:    src.Size(),
	}

	err := o.Update(ctx, in, src, options...)
	if err == nil {
		return o, nil
	}

	return nil, err
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	if strings.HasSuffix(remote, "/") {
		return "", fs.ErrorCantShareDirectories
	}
	if _, err := f.NewObject(ctx, remote); err != nil {
		return "", err
	}
	bucket, bucketPath := f.split(remote)
	return path.Join(f.opt.FrontEndpoint, "/download/", bucket, quotePath(bucketPath)), nil
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (_ fs.Object, err error) {
	dstBucket, dstPath := f.split(remote)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcBucket, srcPath := srcObj.split()

	if dstBucket == srcBucket && dstPath == srcPath {
		// https://github.com/jjjake/internetarchive/blob/2456376533251df9d05e0a14d796ec1ced4959f5/internetarchive/cli/ia_copy.py#L68
		fs.Debugf(src, "Can't copy - the source and destination files cannot be the same!")
		return nil, fs.ErrorCantCopy
	}

	updateTracker := random.String(32)
	headers := map[string]string{
		"x-archive-auto-make-bucket": "1",
		"x-archive-queue-derive":     "0",
		"x-archive-keep-old-version": "0",
		"x-amz-copy-source":          quotePath(path.Join("/", srcBucket, srcPath)),
		"x-amz-metadata-directive":   "COPY",
		"x-archive-filemeta-sha1":    srcObj.sha1,
		"x-archive-filemeta-md5":     srcObj.md5,
		"x-archive-filemeta-crc32":   srcObj.crc32,
		"x-archive-filemeta-size":    fmt.Sprint(srcObj.size),
		// add this too for sure
		"x-archive-filemeta-rclone-mtime":        srcObj.modTime.Format(time.RFC3339Nano),
		"x-archive-filemeta-rclone-update-track": updateTracker,
	}

	// make a PUT request at (IAS3)/:item/:path without body
	var resp *http.Response
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/" + url.PathEscape(path.Join(dstBucket, dstPath)),
		ExtraHeaders: headers,
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	// we can't update/find metadata here as IA will also
	// queue server-side copy as well as upload/delete.
	return f.waitFileUpload(ctx, trimPathPrefix(path.Join(dstBucket, dstPath), f.root, f.opt.Enc), updateTracker, srcObj.size)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	var allEntries, entries fs.DirEntries
	bucket, reqDir := f.split(dir)
	if bucket == "" {
		if reqDir != "" {
			return fs.ErrorListBucketRequired
		}
		return callback(entries)
	}
	grandparent := f.opt.Enc.ToStandardPath(strings.Trim(path.Join(bucket, reqDir), "/") + "/")

	allEntries, err = f.listAllUnconstrained(ctx, bucket)
	if err != nil {
		return err
	}
	for _, ent := range allEntries {
		obj, ok := ent.(*Object)
		if ok && strings.HasPrefix(obj.remote, grandparent) {
			obj.remote = trimPathPrefix(obj.remote, f.root, f.opt.Enc)
			entries = append(entries, obj)
		}
		dire, ok := ent.(*fs.Dir)
		if ok && strings.HasPrefix(dire.Remote(), grandparent) {
			dire.SetRemote(trimPathPrefix(dire.Remote(), f.root, f.opt.Enc))
			entries = append(entries, dire)
		}
	}

	return callback(entries)
}

// CleanUp removes all files inside history/
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	bucket, _ := f.split("/")
	if bucket == "" {
		return fs.ErrorListBucketRequired
	}
	entries, err := f.listAllUnconstrained(ctx, bucket)
	if err != nil {
		return err
	}

	for _, ent := range entries {
		obj, ok := ent.(*Object)
		if ok && strings.HasPrefix(obj.remote, bucket+"/history/") {
			err = obj.Remove(ctx)
			if err != nil {
				return err
			}
		}
		// we can fully ignore directories, as they're just virtual entries to
		// comply with rclone's requirement
	}

	return nil
}

// About returns things about remaining and used spaces
func (f *Fs) About(ctx context.Context) (_ *fs.Usage, err error) {
	bucket, _ := f.split("/")
	if bucket == "" {
		return nil, fs.ErrorListBucketRequired
	}

	result, err := f.requestMetadata(ctx, bucket)
	if err != nil {
		return nil, err
	}

	// perform low-level operation here since it's ridiculous to make 2 same requests
	var historySize int64
	for _, ent := range result.Files {
		if strings.HasPrefix(ent.Name, "history/") {
			size := parseSize(ent.Size)
			if size < 0 {
				// parse error can be ignored since it's not fatal
				continue
			}
			historySize += size
		}
	}

	usage := &fs.Usage{
		Total:   fs.NewUsageValue(iaItemMaxSize),
		Free:    fs.NewUsageValue(iaItemMaxSize - result.ItemSize),
		Used:    fs.NewUsageValue(result.ItemSize),
		Trashed: fs.NewUsageValue(historySize), // bytes in trash
	}
	return usage, nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var optionsFixed []fs.OpenOption
	for _, opt := range options {
		if optRange, ok := opt.(*fs.RangeOption); ok {
			// Ignore range option if file is empty
			if o.Size() == 0 && optRange.Start == 0 && optRange.End > 0 {
				continue
			}
		}
		optionsFixed = append(optionsFixed, opt)
	}

	var resp *http.Response
	// make a GET request to (frontend)/download/:item/:path
	opts := rest.Opts{
		Method:  "GET",
		Path:    path.Join("/download/", o.fs.root, quotePath(o.fs.opt.Enc.FromStandardPath(o.remote))),
		Options: optionsFixed,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.front.Call(ctx, &opts)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update the Object from in with modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	bucket, bucketPath := o.split()
	modTime := src.ModTime(ctx)
	size := src.Size()
	updateTracker := random.String(32)

	// Set the mtime in the metadata
	// internetarchive backend builds at header level as IAS3 has extension outside X-Amz-
	headers := map[string]string{
		// https://github.com/jjjake/internetarchive/blob/2456376533251df9d05e0a14d796ec1ced4959f5/internetarchive/iarequest.py#L158
		"x-amz-filemeta-rclone-mtime":        modTime.Format(time.RFC3339Nano),
		"x-amz-filemeta-rclone-update-track": updateTracker,

		// we add some more headers for intuitive actions
		"x-amz-auto-make-bucket":     "1",    // create an item if does not exist, do nothing if already
		"x-archive-auto-make-bucket": "1",    // same as above in IAS3 original way
		"x-archive-keep-old-version": "0",    // do not keep old versions (a.k.a. trashes in other clouds)
		"x-archive-meta-mediatype":   "data", // mark media type of the uploading file as "data"
		"x-archive-queue-derive":     "0",    // skip derivation process (e.g. encoding to smaller files, OCR on PDFs)
		"x-archive-cascade-delete":   "1",    // enable "cascate delete" (delete all derived files in addition to the file itself)
	}
	if size >= 0 {
		headers["Content-Length"] = fmt.Sprintf("%d", size)
		headers["x-archive-size-hint"] = fmt.Sprintf("%d", size)
	}
	var mdata fs.Metadata
	mdata, err = fs.GetMetadataOptions(ctx, o.fs, src, options)
	if err == nil && mdata != nil {
		for mk, mv := range mdata {
			mk = strings.ToLower(mk)
			if strings.HasPrefix(mk, "rclone-") {
				fs.LogPrintf(fs.LogLevelWarning, o, "reserved metadata key %s is about to set", mk)
			} else if _, ok := roMetadataKey[mk]; ok {
				fs.LogPrintf(fs.LogLevelWarning, o, "setting or modifying read-only key %s is requested, skipping", mk)
				continue
			} else if mk == "mtime" {
				// redirect to make it work
				mk = "rclone-mtime"
			}
			headers[fmt.Sprintf("x-amz-filemeta-%s", mk)] = mv
		}
	}

	// read the md5sum if available
	var md5sumHex string
	if !o.fs.opt.DisableChecksum {
		md5sumHex, err = src.Hash(ctx, hash.MD5)
		if err == nil && matchMd5.MatchString(md5sumHex) {
			// Set the md5sum in header on the object if
			// the user wants it
			// https://github.com/jjjake/internetarchive/blob/245637653/internetarchive/item.py#L969
			headers["Content-MD5"] = md5sumHex
		}
	}

	// make a PUT request at (IAS3)/encoded(:item/:path)
	var resp *http.Response
	opts := rest.Opts{
		Method:        "PUT",
		Path:          "/" + url.PathEscape(path.Join(bucket, bucketPath)),
		Body:          in,
		ContentLength: &size,
		ExtraHeaders:  headers,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(resp, err)
	})

	// we can't update/find metadata here as IA will "ingest" uploaded file(s)
	// upon uploads. (you can find its progress at https://archive.org/history/ItemNameHere )
	// or we have to wait for finish? (needs polling (frontend)/metadata/:item or scraping (frontend)/history/:item)
	var newObj *Object
	if err == nil {
		newObj, err = o.fs.waitFileUpload(ctx, o.remote, updateTracker, size)
	} else {
		newObj = &Object{}
	}
	o.crc32 = newObj.crc32
	o.md5 = newObj.md5
	o.sha1 = newObj.sha1
	o.modTime = newObj.modTime
	o.size = newObj.size
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	bucket, bucketPath := o.split()

	// make a DELETE request at (IAS3)/:item/:path
	var resp *http.Response
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/" + url.PathEscape(path.Join(bucket, bucketPath)),
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(resp, err)
	})

	// deleting files can take bit longer as
	// it'll be processed on same queue as uploads
	if err == nil {
		err = o.fs.waitDelete(ctx, bucket, bucketPath)
	}
	return err
}

// String converts this Fs to a string
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Metadata returns all file metadata provided by Internet Archive
func (o *Object) Metadata(ctx context.Context) (m fs.Metadata, err error) {
	if o.rawData == nil {
		return nil, nil
	}
	raw := make(map[string]json.RawMessage)
	err = json.Unmarshal(o.rawData, &raw)
	if err != nil {
		// fatal: json parsing failed
		return
	}
	for k, v := range raw {
		items, err := listOrString(v)
		if len(items) == 0 || err != nil {
			// skip: an entry failed to parse
			continue
		}
		m.Set(k, items[0])
	}
	// move the old mtime to an another key
	if v, ok := m["mtime"]; ok {
		m["rclone-ia-mtime"] = v
	}
	// overwrite with a correct mtime
	m["mtime"] = o.modTime.Format(time.RFC3339Nano)
	return
}

func (f *Fs) shouldRetry(resp *http.Response, err error) (bool, error) {
	if resp != nil {
		for _, e := range retryErrorCodes {
			if resp.StatusCode == e {
				return true, err
			}
		}
	}
	// Ok, not an awserr, check for generic failure conditions
	return fserrors.ShouldRetry(err), err
}

var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// split returns bucket and bucketPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (bucketName, bucketPath string) {
	bucketName, bucketPath = bucket.Split(path.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(bucketName), f.opt.Enc.FromStandardPath(bucketPath)
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	return o.fs.split(o.remote)
}

func (f *Fs) requestMetadata(ctx context.Context, bucket string) (result *MetadataResponse, err error) {
	var resp *http.Response
	// make a GET request to (frontend)/metadata/:item/
	opts := rest.Opts{
		Method: "GET",
		Path:   path.Join("/metadata/", bucket),
	}

	var temp MetadataResponseRaw
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.front.CallJSON(ctx, &opts, nil, &temp)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return
	}
	return temp.unraw()
}

// list up all files/directories without any filters
func (f *Fs) listAllUnconstrained(ctx context.Context, bucket string) (entries fs.DirEntries, err error) {
	result, err := f.requestMetadata(ctx, bucket)
	if err != nil {
		return nil, err
	}

	knownDirs := map[string]time.Time{
		"": time.Unix(0, 0),
	}
	for _, file := range result.Files {
		dir := strings.Trim(betterPathDir(file.Name), "/")
		nameWithBucket := path.Join(bucket, file.Name)

		mtimeTime := file.parseMtime()

		// populate children directories
		child := dir
		for {
			if _, ok := knownDirs[child]; ok {
				break
			}
			// directory
			d := fs.NewDir(f.opt.Enc.ToStandardPath(path.Join(bucket, child)), mtimeTime)
			entries = append(entries, d)

			knownDirs[child] = mtimeTime
			child = strings.Trim(betterPathDir(child), "/")
		}
		if _, ok := knownDirs[betterPathDir(file.Name)]; !ok {
			continue
		}

		size := parseSize(file.Size)

		o := makeValidObject(f, f.opt.Enc.ToStandardPath(nameWithBucket), file, mtimeTime, size)
		entries = append(entries, o)
	}

	return entries, nil
}

func (f *Fs) waitFileUpload(ctx context.Context, reqPath, tracker string, newSize int64) (ret *Object, err error) {
	bucket, bucketPath := f.split(reqPath)

	ret = &Object{
		fs:      f,
		remote:  trimPathPrefix(path.Join(bucket, bucketPath), f.root, f.opt.Enc),
		modTime: time.Unix(0, 0),
		size:    -1,
	}

	if f.opt.WaitArchive == 0 {
		// user doesn't want to poll, let's not
		ret2, err := f.NewObject(ctx, reqPath)
		if err == nil {
			ret2, ok := ret2.(*Object)
			if ok {
				ret = ret2
				ret.crc32 = ""
				ret.md5 = ""
				ret.sha1 = ""
				ret.size = -1
			}
		}
		return ret, nil
	}

	retC := make(chan struct {
		*Object
		error
	}, 1)
	go func() {
		isFirstTime := true
		existed := false
		for {
			if !isFirstTime {
				// depending on the queue, it takes time
				time.Sleep(10 * time.Second)
			}
			metadata, err := f.requestMetadata(ctx, bucket)
			if err != nil {
				retC <- struct {
					*Object
					error
				}{ret, err}
				return
			}

			var iaFile *IAFile
			for _, f := range metadata.Files {
				if f.Name == bucketPath {
					iaFile = &f
					break
				}
			}
			if isFirstTime {
				isFirstTime = false
				existed = iaFile != nil
			}
			if iaFile == nil {
				continue
			}
			if !existed && !isFirstTime {
				// fast path: file wasn't exited before
				retC <- struct {
					*Object
					error
				}{makeValidObject2(f, *iaFile, bucket), nil}
				return
			}

			fileTrackers, _ := listOrString(iaFile.UpdateTrack)
			trackerMatch := false
			for _, v := range fileTrackers {
				if v == tracker {
					trackerMatch = true
					break
				}
			}
			if !trackerMatch {
				continue
			}
			if !compareSize(parseSize(iaFile.Size), newSize) {
				continue
			}

			// voila!
			retC <- struct {
				*Object
				error
			}{makeValidObject2(f, *iaFile, bucket), nil}
			return
		}
	}()

	select {
	case res := <-retC:
		return res.Object, res.error
	case <-time.After(time.Duration(f.opt.WaitArchive)):
		return ret, nil
	}
}

func (f *Fs) waitDelete(ctx context.Context, bucket, bucketPath string) (err error) {
	if f.opt.WaitArchive == 0 {
		// user doesn't want to poll, let's not
		return nil
	}

	retC := make(chan error, 1)
	go func() {
		for {
			metadata, err := f.requestMetadata(ctx, bucket)
			if err != nil {
				retC <- err
				return
			}

			found := false
			for _, f := range metadata.Files {
				if f.Name == bucketPath {
					found = true
					break
				}
			}

			if !found {
				retC <- nil
				return
			}

			// depending on the queue, it takes time
			time.Sleep(10 * time.Second)
		}
	}()

	select {
	case res := <-retC:
		return res
	case <-time.After(time.Duration(f.opt.WaitArchive)):
		return nil
	}
}

func makeValidObject(f *Fs, remote string, file IAFile, mtime time.Time, size int64) *Object {
	ret := &Object{
		fs:      f,
		remote:  remote,
		modTime: mtime,
		size:    size,
		rawData: file.rawData,
	}
	// hashes from _files.xml (where summation != "") is different from one in other files
	// https://forum.rclone.org/t/internet-archive-md5-tag-in-id-files-xml-interpreted-incorrectly/31922
	if file.Summation == "" {
		ret.md5 = file.Md5
		ret.crc32 = file.Crc32
		ret.sha1 = file.Sha1
	}
	return ret
}

func makeValidObject2(f *Fs, file IAFile, bucket string) *Object {
	mtimeTime := file.parseMtime()

	size := parseSize(file.Size)

	return makeValidObject(f, trimPathPrefix(path.Join(bucket, file.Name), f.root, f.opt.Enc), file, mtimeTime, size)
}

func listOrString(jm json.RawMessage) (rmArray []string, err error) {
	// rclone-metadata can be an array or string
	// try to deserialize it as array first
	err = json.Unmarshal(jm, &rmArray)
	if err != nil {
		// if not, it's a string
		dst := new(string)
		err = json.Unmarshal(jm, dst)
		if err == nil {
			rmArray = []string{*dst}
		}
	}
	return
}

func (file IAFile) parseMtime() (mtime time.Time) {
	// method 1: use metadata added by rclone
	rmArray, err := listOrString(file.RcloneMtime)
	// let's take the first value we can deserialize
	for _, value := range rmArray {
		mtime, err = time.Parse(time.RFC3339Nano, value)
		if err == nil {
			break
		}
	}
	if err != nil {
		// method 2: use metadata added by IAS3
		mtime, err = swift.FloatStringToTime(file.Mtime)
	}
	if err != nil {
		// metadata files don't have some of the fields
		mtime = time.Unix(0, 0)
	}
	return mtime
}

func (mrr *MetadataResponseRaw) unraw() (_ *MetadataResponse, err error) {
	var files []IAFile
	for _, raw := range mrr.Files {
		var parsed IAFile
		err = json.Unmarshal(raw, &parsed)
		if err != nil {
			return nil, err
		}
		parsed.rawData = raw
		files = append(files, parsed)
	}
	return &MetadataResponse{
		Files:    files,
		ItemSize: mrr.ItemSize,
	}, nil
}

func compareSize(a, b int64) bool {
	if a < 0 || b < 0 {
		// we won't compare if any of them is not known
		return true
	}
	return a == b
}

func parseSize(str string) int64 {
	size, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		size = -1
	}
	return size
}

func betterPathDir(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

func betterPathClean(p string) string {
	d := path.Clean(p)
	if d == "." {
		return ""
	}
	return d
}

func trimPathPrefix(s, prefix string, enc encoder.MultiEncoder) string {
	// we need to clean the paths to make tests pass!
	s = betterPathClean(s)
	prefix = betterPathClean(prefix)
	if s == prefix || s == prefix+"/" {
		return ""
	}
	prefix = enc.ToStandardPath(strings.TrimRight(prefix, "/"))
	return enc.ToStandardPath(strings.TrimPrefix(s, prefix+"/"))
}

// mimics urllib.parse.quote() on Python; exclude / from url.PathEscape
func quotePath(s string) string {
	seg := strings.Split(s, "/")
	newValues := []string{}
	for _, v := range seg {
		newValues = append(newValues, url.PathEscape(v))
	}
	return strings.Join(newValues, "/")
}

var (
	_ fs.Fs           = &Fs{}
	_ fs.Copier       = &Fs{}
	_ fs.ListRer      = &Fs{}
	_ fs.CleanUpper   = &Fs{}
	_ fs.PublicLinker = &Fs{}
	_ fs.Abouter      = &Fs{}
	_ fs.Object       = &Object{}
	_ fs.Metadataer   = &Object{}
)
