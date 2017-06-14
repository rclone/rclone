// Package qingstor provides an interface to QingClound object storage that name is QingStor
// Home: https://www.qingcloud.com/
package qingstor

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
	"strconv"
	"net/http"
	"bytes"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/yunify/qingstor-sdk-go/config"
	qs "github.com/yunify/qingstor-sdk-go/service"
	qsErr "github.com/yunify/qingstor-sdk-go/request/errors"
)

// Register with Fs
func init()  {
	fs.Register(&fs.RegInfo{
		Name:        "qingstor",
		Description: "QingClound Object Storage",
		NewFs:        NewFs,
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
		},{
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
	listLimitSize        = 1000                      // Number of items to read at once
	maxSizeForCopy       = 1024 * 1024 * 1024 * 5    // The maximum size of object we can COPY
	maxSizeForPart       = 1024 * 1024 * 1024 * 1    // The maximum size of object we can Upload in Multipart Upload API
	multipartUploadSize  = 1024 * 1024 * 64          // The size of multipart upload object as once.
	MaxMultipleParts     = 10000                     // The maximum number of upload multiple parts
)

// Globals
func timestampToTime(ti int64) time.Time {
	timeLayout := time.RFC3339Nano
	ts := time.Unix(ti, 0).Format(timeLayout)
	tm, _ := time.Parse(timeLayout, ts)
	return tm.UTC()
}

// Fs represents a remote qingstor server
type Fs struct {
	name               string           // The name of the remote
	zone               string           // The zone we are working on
	bucket             string           // The bucket we are working on
	root               string           // The root is a subdir, is a special object
	features           *fs.Features     // optional features
	svc                *qs.Service      // The connection to the qingstor server
	bucketInit         *qs.Bucket       // Initialize a qingstor bucket server
	sse                string           // The type of server-side encryption
}

// Object describes a qingstor object
type Object struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta & mimeType - to fill
	// that in you need to call readMetaData
	fs            *Fs                // what this object is part of
	remote        string             // object of remote
	etag          string             // md5sum of the object
	size 	      int64              // length of the object content
	mimeType      string             // ContentType of object - may be ""
	lastModified  time.Time          // Last modified
	encrypted     bool               // whether the object is encryption
	algo          string             // Custom encryption algorithms
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
func qsParseEndpoint(endpoint string) (protocol, host, port string, err error)  {
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
	accessKeyID     := fs.ConfigFileGet(name, "access_key_id")
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
			return nil, errors.New(fmt.Sprintf("The endpoint \"%s\" format error", endpoint))
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

	bucketInit, err := svc.Bucket(bucket, zone)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:               name,
		zone:               zone,
		root:               key,
		bucket:             bucket,
		svc:                svc,
		bucketInit:         bucketInit,
		sse:                fs.ConfigFileGet(name, "server_side_encryption"),
	}
	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)

	if f.root != "" {
		if ! strings.HasSuffix(f.root, "/") {
			f.root += "/"
		}
		 //Check to see if the object exists
		_, err := f.bucketInit.HeadObject(key, &qs.HeadObjectInput{})
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
		return fmt.Sprintf("QingStor bucket \"%s\"", f.bucket)
	}
	return fmt.Sprintf("QingStor bucket \"%s\" root \"%s\"", f.bucket, f.root)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	//return time.Nanosecond
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Created a new object
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)  {
	// Temporary Object under construction
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcFs := srcObj.fs
	key := f.root + remote
	source := path.Join("/" + srcFs.bucket, srcFs.root, srcObj.remote)
	fs.Debugf(f, fmt.Sprintf("Copied, source key is: %s, and dst key is: %s", source, key))
	req := qs.PutObjectInput{
		XQSCopySource: &source,
	}
	_, err := f.bucketInit.PutObject(key, &req)
	if err != nil {
		fs.Debugf(f, fmt.Sprintf("Copied Faild, API Error: %s", err))
		return nil, err
	}
	return f.NewObject(remote)
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error)  {
	return f.newObjectWithInfo(remote, nil)
}

// Return an Object from a path
//
//If it can't be found it returns the error ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *qs.KeyType) (fs.Object, error) {
	o := &Object{
		fs:            f,
		remote:        remote,
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
			o.encrypted = qs.BoolValue(info.Encrypted )
		}

	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// lists files and directories to out
func (f *Fs) list(out fs.ListOpts, dir string) error {
	prefix := f.root
	if dir != "" {
		prefix += dir + "/"
	}

	delimiter := ""
	level := out.Level()
	switch level {
	case 1:
		delimiter = "/"
	case fs.MaxLevel:
		//
	default:
		return fs.ErrorLevelNotSupported
	}

	maxLimit := int(listLimitSize)
	var marker *string

	for {
		// FIXME need to implement ALL loop
		req := qs.ListObjectsInput{
			Delimiter: &delimiter,
			Prefix:    &prefix,
			Limit:     &maxLimit,
			Marker:    marker,
		}
		resp, err := f.bucketInit.ListObjects(&req)
		if err != nil {
			return err
		}
		rootLength := len(f.root)
		if level == 1 {
			for _, commonPrefix := range resp.CommonPrefixes {
				if commonPrefix == nil {
					fs.Logf(f, "Nil common prefix received")
					continue
				}
				remote := *commonPrefix

				if ! strings.HasPrefix(remote, f.root) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[rootLength:]
				if strings.HasSuffix(remote, "/") {
					remote = remote[:len(remote)-1]
				}
				dir := &fs.Dir{
					Name:  remote,
					Bytes: -1,
					Count: -1,
				}
				if out.AddDir(dir) {
					return fs.ErrorListAborted
				}
			}
		}

		for _, object := range resp.Keys {
			key := qs.StringValue(object.Key)
			if ! strings.HasPrefix(key, f.root) {
				fs.Logf(f, "Odd name received %q", key)
				continue
			}
			remote := key[rootLength:]

			o, _ := f.newObjectWithInfo(remote, object)
			if out.Add(o) {
				return fs.ErrorListAborted
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

func (f *Fs) listFiles(out fs.ListOpts, dir string) {
	defer out.Finished()
	if f.bucket == "" {
		// Return no objects at top level list
		out.SetError(errors.New("can't list objects at root - choose a bucket using lsd"))
		return
	}
	err := f.list(out, dir)
	if err != nil {
		if e, ok := err.(*qsErr.QingStorError); ok {
			if e.StatusCode == http.StatusNotFound {
				err = fs.ErrorDirNotFound
			}
		}
		out.SetError(err)
		return
	}
}

// Check if the bucket exists
func (f *Fs) dirExists() (bool, error) {
	_, err := f.bucketInit.Head()
	if err == nil {
		return true, nil
	}

	if e, ok := err.(*qsErr.QingStorError); ok {
		if e.StatusCode == http.StatusNotFound {
			return false, nil
		}
	}
	return false, err
}

// Create a new bucket
func (f *Fs) Mkdir(dir string) error  {
	// Can't create subdirs
	if dir != "" {
		return nil
	}
	exists, err := f.dirExists()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = f.bucketInit.Put()
	if e, ok := err.(*qsErr.QingStorError); ok {
		if e.StatusCode == http.StatusConflict {
			return nil
		}
	}
	return err
}

// Check if the bucket empty
func (f *Fs) dirIsEmpty() (bool, error)  {
	limit := 10
	req := qs.ListObjectsInput{
		Limit: &limit,
	}
	rsp, err := f.bucketInit.ListObjects(&req)

	if err != nil {
		return false, err
	}
	if len(rsp.Keys) == 0 {
		return true, nil
	}
	return false, nil
}

// Delete a bucket
func (f *Fs) Rmdir(dir string) error  {
	if f.root != "" || dir != "" {
		return nil
	}

	isEmpty, err := f.dirIsEmpty()
	if err != nil {
		return err
	}
	if ! isEmpty {
		fs.Debugf(f, fmt.Sprintf("The bucket %s you tried to delete not empty.", f.bucket))
		return errors.New("BucketNotEmpty: The bucket you tried to delete is not empty")
	}

	fs.Debugf(f, fmt.Sprintf("Tried to delete the bucket %s", f.bucket))
	_, err = f.bucketInit.Delete()
	return err
}

// listBuckets lists the buckets to out
func (f *Fs) listBuckets(out fs.ListOpts, dir string) {
	defer out.Finished()
	if dir != "" {
		out.SetError(fs.ErrorListOnlyRoot)
		return
	}

	req := qs.ListBucketsInput{
		Location: &f.zone,
	}
	resp, err := f.svc.ListBuckets(&req)
	if err != nil {
		out.SetError(err)
		return
	}

	for _, bucket := range resp.Buckets {
		dir := &fs.Dir{
			Name:  qs.StringValue(bucket.Name),
			When:  qs.TimeValue(bucket.Created),
			Bytes: -1,
			Count: -1,
		}
		if out.AddDir(dir) {
			break
		}
	}
}

// List lists files and directories to out
func (f *Fs) List(out fs.ListOpts, dir string) {
	if f.bucket == "" {
		f.listBuckets(out, dir)
	} else {
		f.listFiles(out, dir)
	}
	return
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	key := o.fs.root + o.remote
	fs.Debugf(o, fmt.Sprintf("Read metadata of key: %s", key))
	resp, err := o.fs.bucketInit.HeadObject(key, &qs.HeadObjectInput{})
	if err != nil {
		fs.Debugf(o, fmt.Sprintf("Read metadata faild, API Error: %s", err))
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
func (o *Object) ModTime() time.Time  {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata, %v", err)
		return time.Now()
	}
	modTime := o.lastModified
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error  {
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
	sourceKey :=  path.Join("/" + o.fs.bucket, o.fs.root, o.remote)

	req := qs.PutObjectInput{
		XQSCopySource: &sourceKey,
		ContentType:   &mimeType,
	}
	_ , err = o.fs.bucketInit.PutObject(key, &req)

	return err
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error)  {
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
	resp, err := o.fs.bucketInit.GetObject(key, &req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update in to the object
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error  {
	// The maximum size of upload object is multipartUploadSize * MaxMultipleParts
	key := o.fs.root + o.remote

	// Initiate Upload Multipart
	fs.Debugf(o, fmt.Sprintf("Initiate Upload Multipart, key: %s", key))
	var uploadID  *string
	mimeType := fs.MimeType(src)

	initReq := qs.InitiateMultipartUploadInput{
		ContentType: &mimeType,
	}
	rsp, err := o.fs.bucketInit.InitiateMultipartUpload(key, &initReq)
	if err != nil {
		return err
	}
	uploadID = rsp.UploadID

	// Start Upload Part
	var n int = 0
	var objectParts = [] *qs.ObjectPartType{}

	// Create an new buffer
	buffer := new(bytes.Buffer)

	for {
		size, _ := io.CopyN(buffer, in, multipartUploadSize)
		if size == 0 {
			break
		}
		// Upload Multipart Object
		number := n
		req := qs.UploadMultipartInput{
			PartNumber:    &number,
			UploadID:      uploadID,
			ContentLength: &size,
			Body:          buffer,
		}
		fs.Debugf(o, fmt.Sprintf("Upload Multipart, upload_id: %s, part_number: %d", *uploadID, number))
		_, err = o.fs.bucketInit.UploadMultipart(key, &req)
		if err != nil {
			return err
		}
		part := qs.ObjectPartType{
			PartNumber: &number,
			Size: &size,
		}
		objectParts = append(objectParts, &part)
		n += 1
	}

	// Complete Multipart Upload
	fs.Debugf(o, fmt.Sprintf("Complete Upload Multipart, upload_id: %s, part_numbers: %d", *uploadID, n))
	completeReq := qs.CompleteMultipartUploadInput{
		UploadID:      uploadID,
		ObjectParts:   objectParts,
	}
	_, err = o.fs.bucketInit.CompleteMultipartUpload(key, &completeReq)
	if err != nil{
		return err
	}
	defer func() {
		if err != nil {
			fs.Errorf(o, fmt.Sprintf("Create Object Faild, API ERROR: %s", err))
			// Abort Upload when init success and upload failed
			if uploadID != nil {
				fs.Debugf(o, fmt.Sprintf("Abort Upload Multipart, upload_id: %s, part_numbers: %d", *uploadID, n) )
				abortReq := qs.AbortMultipartUploadInput{
					UploadID: uploadID,
				}
				_, _ = o.fs.bucketInit.AbortMultipartUpload(key, &abortReq)
			}
		}
	}()

	// Read Metadata of object
	err = o.readMetaData()
	return err
}

// Removes this object
func (o *Object) Remove() error  {
	key := o.fs.root + o.remote
	_, err := o.fs.bucketInit.DeleteObject(key)
	return err
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info  {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(t fs.HashType) (string, error)  {
	var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)
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
func (o *Object) Storable() bool  {
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
func (o *Object) Size() int64  {
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
	_ fs.MimeTyper = &Object{}
)

