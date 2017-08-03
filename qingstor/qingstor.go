// Package qingstor provides an interface to QingStor object storage
// Home: https://www.qingcloud.com/

// +build !plan9

package qingstor

import (
	"bytes"
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
	"github.com/pkg/errors"
	"github.com/yunify/qingstor-sdk-go/config"
	qsErr "github.com/yunify/qingstor-sdk-go/request/errors"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "qingstor",
		Description: "QingClound Object Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "env_auth",
			Help: "Get QingStor credentials from runtime. Only applies if access_key_id and secret_access_key is blank.",
			Examples: []fs.OptionExample{
				{
					Value: "false",
					Help:  "Enter QingStor credentials in the next step",
				}, {
					Value: "true",
					Help:  "Get QingStor credentials from the environment (env vars or IAM)",
				},
			},
		}, {
			Name: "access_key_id",
			Help: "QingStor Access Key ID - leave blank for anonymous access or runtime credentials.",
		}, {
			Name: "secret_access_key",
			Help: "QingStor Secret Access Key (password) - leave blank for anonymous access or runtime credentials.",
		}, {
			Name: "endpoint",
			Help: "Enter a endpoint URL to connection QingStor API.\nLeave blank will use the default value \"https://qingstor.com:443\"",
		}, {
			Name: "zone",
			Help: "Choose or Enter a zone to connect. Default is \"pek3a\".",
			Examples: []fs.OptionExample{
				{
					Value: "pek3a",

					Help: "The Beijing (China) Three Zone\nNeeds location constraint pek3a.",
				},
				{
					Value: "sh1a",

					Help: "The Shanghai (China) First Zone\nNeeds location constraint sh1a.",
				},
			},
		}, {
			Name: "connection_retries",
			Help: "Number of connnection retry.\nLeave blank will use the default value \"3\".",
		}},
	})
}

// Constants
const (
	listLimitSize       = 1000                   // Number of items to read at once
	maxSizeForCopy      = 1024 * 1024 * 1024 * 5 // The maximum size of object we can COPY
	maxSizeForPart      = 1024 * 1024 * 1024 * 1 // The maximum size of object we can Upload in Multipart Upload API
	multipartUploadSize = 1024 * 1024 * 64       // The size of multipart upload object as once.
	MaxMultipleParts    = 10000                  // The maximum number of upload multiple parts
)

// Globals
func timestampToTime(tp int64) time.Time {
	timeLayout := time.RFC3339Nano
	ts := time.Unix(tp, 0).Format(timeLayout)
	tm, _ := time.Parse(timeLayout, ts)
	return tm.UTC()
}

// Fs represents a remote qingstor server
type Fs struct {
	name          string       // The name of the remote
	zone          string       // The zone we are working on
	bucket        string       // The bucket we are working on
	bucketOKMu    sync.Mutex   // mutex to protect bucketOK and bucketDeleted
	bucketOK      bool         // true if we have created the bucket
	bucketDeleted bool         // true if we have deleted the bucket
	root          string       // The root is a subdir, is a special object
	features      *fs.Features // optional features
	svc           *qs.Service  // The connection to the qingstor server
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

// parseParse parses a qingstor 'url'
func qsParsePath(path string) (bucket, key string, err error) {
	// Pattern to match a qingstor path
	var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)
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
func qsServiceConnection(name string) (*qs.Service, error) {
	accessKeyID := fs.ConfigFileGet(name, "access_key_id")
	secretAccessKey := fs.ConfigFileGet(name, "secret_access_key")

	switch {
	case fs.ConfigFileGetBool(name, "env_auth", false):
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

	endpoint := fs.ConfigFileGet(name, "endpoint", "")
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
		}

	}

	connectionRetries := 3
	retries := fs.ConfigFileGet(name, "connection_retries", "")
	if retries != "" {
		connectionRetries, _ = strconv.Atoi(retries)
	}

	cf, err := config.NewDefault()
	cf.AccessKeyID = accessKeyID
	cf.SecretAccessKey = secretAccessKey
	cf.Protocol = protocol
	cf.Host = host
	cf.Port = port
	cf.ConnectionRetries = connectionRetries
	cf.Connection = fs.Config.Client()

	svc, _ := qs.Init(cf)

	return svc, err
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	bucket, key, err := qsParsePath(root)
	if err != nil {
		return nil, err
	}
	svc, err := qsServiceConnection(name)
	if err != nil {
		return nil, err
	}

	zone := fs.ConfigFileGet(name, "zone")
	if zone == "" {
		zone = "pek3a"
	}

	f := &Fs{
		name:   name,
		zone:   zone,
		root:   key,
		bucket: bucket,
		svc:    svc,
	}
	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)

	if f.root != "" {
		if !strings.HasSuffix(f.root, "/") {
			f.root += "/"
		}
		//Check to see if the object exists
		bucketInit, err := svc.Bucket(bucket, zone)
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
func (f *Fs) Hashes() fs.HashSet {
	//return fs.HashSet(fs.HashMD5)
	//Not supported temporary
	return fs.HashSet(fs.HashNone)
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
	list := fs.NewListRHelper(callback)
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

	if !f.bucketDeleted {
		exists, err := f.dirExists()
		if err == nil {
			f.bucketOK = exists
		}
		if err != nil || exists {
			return err
		}
	}

	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
	if err != nil {
		return err
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
	limit := 8
	bucketInit, err := f.svc.Bucket(f.bucket, f.zone)
	if err != nil {
		return true, err
	}

	req := qs.ListObjectsInput{
		Limit: &limit,
	}
	rsp, err := bucketInit.ListObjects(&req)

	if err != nil {
		return false, err
	}
	if len(rsp.Keys) == 0 {
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
	_, err = bucketInit.Delete()
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

	bucketInit, err := o.fs.svc.Bucket(o.fs.bucket, o.fs.zone)
	if err != nil {
		return err
	}
	//Initiate Upload Multipart
	key := o.fs.root + o.remote
	var objectParts = []*qs.ObjectPartType{}
	var uploadID *string
	var partNumber int

	defer func() {
		if err != nil {
			fs.Errorf(o, "Create Object Faild, API ERROR: %v", err)
			// Abort Upload when init success and upload failed
			if uploadID != nil {
				fs.Debugf(o, "Abort Upload Multipart, upload_id: %s, objectParts: %s", *uploadID, objectParts)
				abortReq := qs.AbortMultipartUploadInput{
					UploadID: uploadID,
				}
				_, _ = bucketInit.AbortMultipartUpload(key, &abortReq)
			}
		}
	}()

	fs.Debugf(o, "Initiate Upload Multipart, key: %s", key)
	mimeType := fs.MimeType(src)
	initReq := qs.InitiateMultipartUploadInput{
		ContentType: &mimeType,
	}
	rsp, err := bucketInit.InitiateMultipartUpload(key, &initReq)
	if err != nil {
		return err
	}
	uploadID = rsp.UploadID

	// Create an new buffer
	buffer := new(bytes.Buffer)

	for {
		size, er := io.CopyN(buffer, in, multipartUploadSize)
		if er != nil && er != io.EOF {
			err = fmt.Errorf("read upload data failed, error: %s", er)
			return err
		}
		if size == 0 && partNumber > 0 {
			break
		}
		// Upload Multipart Object
		number := partNumber
		req := qs.UploadMultipartInput{
			PartNumber:    &number,
			UploadID:      uploadID,
			ContentLength: &size,
			Body:          buffer,
		}
		fs.Debugf(o, "Upload Multipart, upload_id: %s, part_number: %d", *uploadID, number)
		_, err = bucketInit.UploadMultipart(key, &req)
		if err != nil {
			return err
		}
		part := qs.ObjectPartType{
			PartNumber: &number,
			Size:       &size,
		}
		objectParts = append(objectParts, &part)
		partNumber++
	}

	// Complete Multipart Upload
	fs.Debugf(o, "Complete Upload Multipart, upload_id: %s, objectParts: %d", *uploadID, objectParts)
	completeReq := qs.CompleteMultipartUploadInput{
		UploadID:    uploadID,
		ObjectParts: objectParts,
	}
	_, err = bucketInit.CompleteMultipartUpload(key, &completeReq)
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
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashMD5 {
		return "", fs.ErrHashUnsupported
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
