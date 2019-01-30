// Package qingstor provides an interface to QingStor object storage
// Home: https://www.qingcloud.com/

// +build !plan9

package qingstor

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/pkg/errors"
	qsConfig "github.com/yunify/qingstor-sdk-go/config"
	qsErr "github.com/yunify/qingstor-sdk-go/request/errors"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "qingstor",
		Description: "QingCloud Object Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:    "env_auth",
			Help:    "Get QingStor credentials from runtime. Only applies if access_key_id and secret_access_key is blank.",
			Default: false,
			Examples: []fs.OptionExample{{
				Value: "false",
				Help:  "Enter QingStor credentials in the next step",
			}, {
				Value: "true",
				Help:  "Get QingStor credentials from the environment (env vars or IAM)",
			}},
		}, {
			Name: "access_key_id",
			Help: "QingStor Access Key ID\nLeave blank for anonymous access or runtime credentials.",
		}, {
			Name: "secret_access_key",
			Help: "QingStor Secret Access Key (password)\nLeave blank for anonymous access or runtime credentials.",
		}, {
			Name: "endpoint",
			Help: "Enter a endpoint URL to connection QingStor API.\nLeave blank will use the default value \"https://qingstor.com:443\"",
		}, {
			Name: "zone",
			Help: "Zone to connect to.\nDefault is \"pek3a\".",
			Examples: []fs.OptionExample{{
				Value: "pek3a",
				Help:  "The Beijing (China) Three Zone\nNeeds location constraint pek3a.",
			}, {
				Value: "sh1a",
				Help:  "The Shanghai (China) First Zone\nNeeds location constraint sh1a.",
			}, {
				Value: "gd2a",
				Help:  "The Guangdong (China) Second Zone\nNeeds location constraint gd2a.",
			}},
		}, {
			Name:     "connection_retries",
			Help:     "Number of connection retries.",
			Default:  3,
			Advanced: true,
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5GB.`,
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Chunk size to use for uploading.

When uploading files larger than upload_cutoff they will be uploaded
as multipart uploads using this chunk size.

Note that "--qingstor-upload-concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high speed links and you have
enough memory, then increasing this will speed up the transfers.`,
			Default:  minChunkSize,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

NB if you set this to > 1 then the checksums of multpart uploads
become corrupted (the uploads themselves are not corrupted though).

If you are uploading small numbers of large file over high speed link
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
			Default:  1,
			Advanced: true,
		}},
	})
}

// Constants
const (
	listLimitSize       = 1000                   // Number of items to read at once
	maxSizeForCopy      = 1024 * 1024 * 1024 * 5 // The maximum size of object we can COPY
	minChunkSize        = fs.SizeSuffix(minMultiPartSize)
	defaultUploadCutoff = fs.SizeSuffix(200 * 1024 * 1024)
	maxUploadCutoff     = fs.SizeSuffix(5 * 1024 * 1024 * 1024)
)

// Globals
func timestampToTime(tp int64) time.Time {
	timeLayout := time.RFC3339Nano
	ts := time.Unix(tp, 0).Format(timeLayout)
	tm, _ := time.Parse(timeLayout, ts)
	return tm.UTC()
}

// Options defines the configuration for this backend
type Options struct {
	EnvAuth           bool          `config:"env_auth"`
	AccessKeyID       string        `config:"access_key_id"`
	SecretAccessKey   string        `config:"secret_access_key"`
	Endpoint          string        `config:"endpoint"`
	Zone              string        `config:"zone"`
	ConnectionRetries int           `config:"connection_retries"`
	UploadCutoff      fs.SizeSuffix `config:"upload_cutoff"`
	ChunkSize         fs.SizeSuffix `config:"chunk_size"`
	UploadConcurrency int           `config:"upload_concurrency"`
}

// Fs represents a remote qingstor server
type Fs struct {
	name          string       // The name of the remote
	root          string       // The root is a subdir, is a special object
	opt           Options      // parsed options
	features      *fs.Features // optional features
	svc           *qs.Service  // The connection to the qingstor server
	zone          string       // The zone we are working on
	bucket        string       // The bucket we are working on
	bucketOKMu    sync.Mutex   // mutex to protect bucketOK and bucketDeleted
	bucketOK      bool         // true if we have created the bucket
	bucketDeleted bool         // true if we have deleted the bucket
}

// Object describes a qingstor object
type Object struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta & mimeType - to fill
	// that in you need to call readMetaData
	fs           *Fs       // what this object is part of
	remote       string    // object of remote
	etag         string    // md5sum of the object
	size         int64     // length of the object content
	mimeType     string    // ContentType of object - may be ""
	lastModified time.Time // Last modified
	encrypted    bool      // whether the object is encryption
	algo         string    // Custom encryption algorithms
}

// ------------------------------------------------------------

// Pattern to match a qingstor path
var matcher = regexp.MustCompile(`^/*([^/]*)(.*)$`)

// parseParse parses a qingstor 'url'
func qsParsePath(path string) (bucket, key string, err error) {
	// Pattern to match a qingstor path
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("Couldn't parse bucket out of qingstor path %q", path)
	} else {
		bucket, key = parts[1], parts[2]
		key = strings.Trim(key, "/")
	}
	return
}

// Split an URL into three parts: protocol host and port
func qsParseEndpoint(endpoint string) (protocol, host, port string, err error) {
	/*
	  Pattern to match a endpoint,
	  eg: "http(s)://qingstor.com:443" --> "http(s)", "qingstor.com", 443
	      "http(s)//qingstor.com"      --> "http(s)", "qingstor.com", ""
	      "qingstor.com"               --> "", "qingstor.com", ""
	*/
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case error:
				err = x
			default:
				err = nil
			}
		}
	}()
	var mather = regexp.MustCompile(`^(?:(http|https)://)*(\w+\.(?:[\w\.])*)(?::(\d{0,5}))*$`)
	parts := mather.FindStringSubmatch(endpoint)
	protocol, host, port = parts[1], parts[2], parts[3]
	return
}

// qsConnection makes a connection to qingstor
func qsServiceConnection(opt *Options) (*qs.Service, error) {
	accessKeyID := opt.AccessKeyID
	secretAccessKey := opt.SecretAccessKey

	switch {
	case opt.EnvAuth:
		// No need for empty checks if "env_auth" is true
	case accessKeyID == "" && secretAccessKey == "":
		// if no access key/secret and iam is explicitly disabled then fall back to anon interaction
	case accessKeyID == "":
		return nil, errors.New("access_key_id not found")
	case secretAccessKey == "":
		return nil, errors.New("secret_access_key not found")
	}

	protocol := "https"
	host := "qingstor.com"
	port := 443

	endpoint := opt.Endpoint
	if endpoint != "" {
		_protocol, _host, _port, err := qsParseEndpoint(endpoint)

		if err != nil {
			return nil, fmt.Errorf("The endpoint \"%s\" format error", endpoint)
		}

		if _protocol != "" {
			protocol = _protocol
		}
		host = _host
		if _port != "" {
			port, _ = strconv.Atoi(_port)
		} else if protocol == "http" {
			port = 80
		}

	}

	cf, err := qsConfig.NewDefault()
	if err != nil {
		return nil, err
	}
	cf.AccessKeyID = accessKeyID
	cf.SecretAccessKey = secretAccessKey
	cf.Protocol = protocol
	cf.Host = host
	cf.Port = port
	cf.ConnectionRetries = opt.ConnectionRetries
	cf.Connection = fshttp.NewClient(fs.Config)

	return qs.Init(cf)
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minChunkSize {
		return errors.Errorf("%s is less than %s", cs, minChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	}
	return
}

func checkUploadCutoff(cs fs.SizeSuffix) error {
	if cs > maxUploadCutoff {
		return errors.Errorf("%s is greater than %s", cs, maxUploadCutoff)
	}
	return nil
}

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadCutoff(cs)
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "qingstor: chunk size")
	}
	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, errors.Wrap(err, "qingstor: upload cutoff")
	}
	bucket, key, err := qsParsePath(root)
	if err != nil {
		return nil, err
	}
	svc, err := qsServiceConnection(opt)
	if err != nil {
		return nil, err
	}

	if opt.Zone == "" {
		opt.Zone = "pek3a"
	}

	f := &Fs{
		name:   name,
		root:   key,
		opt:    *opt,
		svc:    svc,
		zone:   opt.Zone,
		bucket: bucket,
	}
	f.features = (&fs.Features{
		ReadMimeType:  true,
		WriteMimeType: true,
		BucketBased:   true,
	}).Fill(f)

	if f.root != "" {
		if !strings.HasSuffix(f.root, "/") {
			f.root += "/"
		}
		//Check to see if the object exists
		bucketInit, err := svc.Bucket(bucket, opt.Zone)
		if err != nil {
			return nil, err
		}
		_, err = bucketInit.HeadObject(key, &qs.HeadObjectInput{})
		if err == nil {
			f.root = path.Dir(key)
			if f.root == "." {
				f.root = ""
			} else {
				f.root += "/"
			}
			// return an error with an fs which points to the parent
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
	if f.root == "" {
		return f.bucket
	}
	return f.bucket + "/" + f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	if f.root == "" {
		return fmt.Sprintf("QingStor bucket %s", f.bucket)
	}
	return fmt.Sprintf("QingStor bucket %s root %s", f.bucket, f.root)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	//return time.Nanosecond
	//Not supported temporary
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
	//return hash.HashSet(hash.HashNone)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Put created a new object
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fsObj := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fsObj, fsObj.Update(in, src, options...)
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	err := f.Mkdir("")
	if err != nil {
		return nil, err
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcFs := srcObj.fs
	key := f.root + remote
	source := path.Join("/"+srcFs.bucket, srcFs.root+srcObj.remote)

	fs.Debugf(f, "Copied, source key is: %s, and dst key is: %s", source, key)
	req := qs.PutObjectInput{
		XQSCopySource: &source,
	}
	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)

	if err != nil {
		return nil, err
	}
	_, err = bucketInit.PutObject(key, &req)
	if err != nil {
		fs.Debugf(f, "Copied Faild, API Error: %v", err)
		return nil, err
	}
	return f.NewObject(remote)
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// Return an Object from a path
//
//If it can't be found it returns the error ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *qs.KeyType) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info
		if info.Size != nil {
			o.size = *info.Size
		}

		if info.Etag != nil {
			o.etag = qs.StringValue(info.Etag)
		}
		if info.Modified == nil {
			fs.Logf(o, "Failed to read last modified")
			o.lastModified = time.Now()
		} else {
			o.lastModified = timestampToTime(int64(*info.Modified))
		}

		if info.MimeType != nil {
			o.mimeType = qs.StringValue(info.MimeType)
		}

		if info.Encrypted != nil {
			o.encrypted = qs.BoolValue(info.Encrypted)
		}

	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *qs.KeyType, isDirectory bool) error

// list the objects into the function supplied
//
// dir is the starting directory, "" for root
//
// Set recurse to read sub directories
func (f *Fs) list(dir string, recurse bool, fn listFn) error {
	prefix := f.root
	if dir != "" {
		prefix += dir + "/"
	}

	delimiter := ""
	if !recurse {
		delimiter = "/"
	}

	maxLimit := int(listLimitSize)
	var marker *string

	for {
		bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
		if err != nil {
			return err
		}
		// FIXME need to implement ALL loop
		req := qs.ListObjectsInput{
			Delimiter: &delimiter,
			Prefix:    &prefix,
			Limit:     &maxLimit,
			Marker:    marker,
		}
		resp, err := bucketInit.ListObjects(&req)
		if err != nil {
			if e, ok := err.(*qsErr.QingStorError); ok {
				if e.StatusCode == http.StatusNotFound {
					err = fs.ErrorDirNotFound
				}
			}
			return err
		}
		rootLength := len(f.root)
		if !recurse {
			for _, commonPrefix := range resp.CommonPrefixes {
				if commonPrefix == nil {
					fs.Logf(f, "Nil common prefix received")
					continue
				}
				remote := *commonPrefix
				if !strings.HasPrefix(remote, f.root) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[rootLength:]
				if strings.HasSuffix(remote, "/") {
					remote = remote[:len(remote)-1]
				}

				err = fn(remote, &qs.KeyType{Key: &remote}, true)
				if err != nil {
					return err
				}
			}
		}

		for _, object := range resp.Keys {
			key := qs.StringValue(object.Key)
			if !strings.HasPrefix(key, f.root) {
				fs.Logf(f, "Odd name received %q", key)
				continue
			}
			remote := key[rootLength:]
			err = fn(remote, object, false)
			if err != nil {
				return err
			}
		}
		// Use NextMarker if set, otherwise use last Key
		if resp.NextMarker == nil || *resp.NextMarker == "" {
			//marker = resp.Keys[len(resp.Keys)-1].Key
			break
		} else {
			marker = resp.NextMarker
		}
	}
	return nil
}

// Convert a list item into a BasicInfo
func (f *Fs) itemToDirEntry(remote string, object *qs.KeyType, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		size := int64(0)
		if object.Size != nil {
			size = *object.Size
		}
		d := fs.NewDir(remote, time.Time{}).SetSize(size)
		return d, nil
	}
	o, err := f.newObjectWithInfo(remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// mark the bucket as being OK
func (f *Fs) markBucketOK() {
	if f.bucket != "" {
		f.bucketOKMu.Lock()
		f.bucketOK = true
		f.bucketDeleted = false
		f.bucketOKMu.Unlock()
	}
}

// listDir lists files and directories to out
func (f *Fs) listDir(dir string) (entries fs.DirEntries, err error) {
	// List the objects and directories
	err = f.list(dir, false, func(remote string, object *qs.KeyType, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// bucket must be present if listing succeeded
	f.markBucketOK()
	return entries, nil
}

// listBuckets lists the buckets to out
func (f *Fs) listBuckets(dir string) (entries fs.DirEntries, err error) {
	if dir != "" {
		return nil, fs.ErrorListBucketRequired
	}

	req := qs.ListBucketsInput{
		Location: &f.zone,
	}
	resp, err := f.svc.ListBuckets(&req)
	if err != nil {
		return nil, err
	}

	for _, bucket := range resp.Buckets {
		d := fs.NewDir(qs.StringValue(bucket.Name), qs.TimeValue(bucket.Created))
		entries = append(entries, d)
	}
	return entries, nil
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
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	if f.bucket == "" {
		return f.listBuckets(dir)
	}
	return f.listDir(dir)
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
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	if f.bucket == "" {
		return fs.ErrorListBucketRequired
	}
	list := walk.NewListRHelper(callback)
	err = f.list(dir, true, func(remote string, object *qs.KeyType, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
		if err != nil {
			return err
		}
		return list.Add(entry)
	})
	if err != nil {
		return err
	}
	// bucket must be present if listing succeeded
	f.markBucketOK()
	return list.Flush()
}

// Check if the bucket exists
func (f *Fs) dirExists() (bool, error) {
	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
	if err != nil {
		return false, err
	}

	_, err = bucketInit.Head()
	if err == nil {
		return true, nil
	}

	if e, ok := err.(*qsErr.QingStorError); ok {
		if e.StatusCode == http.StatusNotFound {
			err = nil
		}
	}
	return false, err
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.bucketOK {
		return nil
	}

	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
	if err != nil {
		return err
	}
	/* When delete a bucket, qingstor need about 60 second to sync status;
	So, need wait for it sync end if we try to operation a just deleted bucket
	*/
	retries := 0
	for retries <= 120 {
		statistics, err := bucketInit.GetStatistics()
		if statistics == nil || err != nil {
			break
		}
		switch *statistics.Status {
		case "deleted":
			fs.Debugf(f, "Wiat for qingstor sync bucket status, retries: %d", retries)
			time.Sleep(time.Second * 1)
			retries++
			continue
		default:
			break
		}
		break
	}

	if !f.bucketDeleted {
		exists, err := f.dirExists()
		if err == nil {
			f.bucketOK = exists
		}
		if err != nil || exists {
			return err
		}
	}

	_, err = bucketInit.Put()
	if e, ok := err.(*qsErr.QingStorError); ok {
		if e.StatusCode == http.StatusConflict {
			err = nil
		}
	}

	if err == nil {
		f.bucketOK = true
		f.bucketDeleted = false
	}

	return err
}

// dirIsEmpty check if the bucket empty
func (f *Fs) dirIsEmpty() (bool, error) {
	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
	if err != nil {
		return true, err
	}

	statistics, err := bucketInit.GetStatistics()
	if err != nil {
		return true, err
	}

	if *statistics.Count == 0 {
		return true, nil
	}
	return false, nil
}

// Rmdir delete a bucket
func (f *Fs) Rmdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.root != "" || dir != "" {
		return nil
	}

	isEmpty, err := f.dirIsEmpty()
	if err != nil {
		return err
	}
	if !isEmpty {
		fs.Debugf(f, "The bucket %s you tried to delete not empty.", f.bucket)
		return errors.New("BucketNotEmpty: The bucket you tried to delete is not empty")
	}

	fs.Debugf(f, "Tried to delete the bucket %s", f.bucket)
	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
	if err != nil {
		return err
	}
	retries := 0
	for retries <= 10 {
		_, delErr := bucketInit.Delete()
		if delErr != nil {
			if e, ok := delErr.(*qsErr.QingStorError); ok {
				switch e.Code {
				// The status of "lease" takes a few seconds to "ready" when creating a new bucket
				// wait for lease status ready
				case "lease_not_ready":
					fs.Debugf(f, "QingStor bucket lease not ready, retries: %d", retries)
					retries++
					time.Sleep(time.Second * 1)
					continue
				default:
					err = e
					break
				}
			}
		} else {
			err = delErr
		}
		break
	}

	if err == nil {
		f.bucketOK = false
		f.bucketDeleted = true
	}
	return err
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	bucketInit, err := o.fs.svc.Bucket(o.fs.bucket, o.fs.zone)
	if err != nil {
		return err
	}

	key := o.fs.root + o.remote
	fs.Debugf(o, "Read metadata of key: %s", key)
	resp, err := bucketInit.HeadObject(key, &qs.HeadObjectInput{})
	if err != nil {
		fs.Debugf(o, "Read metadata faild, API Error: %v", err)
		if e, ok := err.(*qsErr.QingStorError); ok {
			if e.StatusCode == http.StatusNotFound {
				return fs.ErrorObjectNotFound
			}
		}
		return err
	}
	// Ignore missing Content-Length assuming it is 0
	if resp.ContentLength != nil {
		o.size = *resp.ContentLength
	}

	if resp.ETag != nil {
		o.etag = qs.StringValue(resp.ETag)
	}

	if resp.LastModified == nil {
		fs.Logf(o, "Failed to read last modified from HEAD: %v", err)
		o.lastModified = time.Now()
	} else {
		o.lastModified = *resp.LastModified
	}

	if resp.ContentType != nil {
		o.mimeType = qs.StringValue(resp.ContentType)
	}

	if resp.XQSEncryptionCustomerAlgorithm != nil {
		o.algo = qs.StringValue(resp.XQSEncryptionCustomerAlgorithm)
		o.encrypted = true
	}

	return nil
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata, %v", err)
		return time.Now()
	}
	modTime := o.lastModified
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	err := o.readMetaData()
	if err != nil {
		return err
	}
	o.lastModified = modTime
	mimeType := fs.MimeType(o)

	if o.size >= maxSizeForCopy {
		fs.Debugf(o, "SetModTime is unsupported for objects bigger than %v bytes", fs.SizeSuffix(maxSizeForCopy))
		return nil
	}
	// Copy the object to itself to update the metadata
	key := o.fs.root + o.remote
	sourceKey := path.Join("/", o.fs.bucket, key)

	bucketInit, err := o.fs.svc.Bucket(o.fs.bucket, o.fs.zone)
	if err != nil {
		return err
	}

	req := qs.PutObjectInput{
		XQSCopySource: &sourceKey,
		ContentType:   &mimeType,
	}
	_, err = bucketInit.PutObject(key, &req)

	return err
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	bucketInit, err := o.fs.svc.Bucket(o.fs.bucket, o.fs.zone)
	if err != nil {
		return nil, err
	}

	key := o.fs.root + o.remote
	req := qs.GetObjectInput{}
	for _, option := range options {
		switch option.(type) {
		case *fs.RangeOption, *fs.SeekOption:
			_, value := option.Header()
			req.Range = &value
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	resp, err := bucketInit.GetObject(key, &req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update in to the object
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// The maximum size of upload object is multipartUploadSize * MaxMultipleParts
	err := o.fs.Mkdir("")
	if err != nil {
		return err
	}

	key := o.fs.root + o.remote
	// Guess the content type
	mimeType := fs.MimeType(src)

	req := uploadInput{
		body:        in,
		qsSvc:       o.fs.svc,
		bucket:      o.fs.bucket,
		zone:        o.fs.zone,
		key:         key,
		mimeType:    mimeType,
		partSize:    int64(o.fs.opt.ChunkSize),
		concurrency: o.fs.opt.UploadConcurrency,
	}
	uploader := newUploader(&req)

	size := src.Size()
	multipart := size < 0 || size >= int64(o.fs.opt.UploadCutoff)
	if multipart {
		err = uploader.upload()
	} else {
		err = uploader.singlePartUpload(in, size)
	}
	if err != nil {
		return err
	}
	// Read Metadata of object
	err = o.readMetaData()
	return err
}

// Remove this object
func (o *Object) Remove() error {
	bucketInit, err := o.fs.svc.Bucket(o.fs.bucket, o.fs.zone)
	if err != nil {
		return err
	}

	key := o.fs.root + o.remote
	_, err = bucketInit.DeleteObject(key)
	return err
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	etag := strings.Trim(strings.ToLower(o.etag), `"`)
	// Check the etag is a valid md5sum
	if !matchMd5.MatchString(etag) {
		fs.Debugf(o, "Invalid md5sum (probably multipart uploaded) - ignoring: %q", etag)
		return "", nil
	}
	return etag, nil
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
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = &Fs{}
	_ fs.Copier    = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.ListRer   = &Fs{}
	_ fs.MimeTyper = &Object{}
)
