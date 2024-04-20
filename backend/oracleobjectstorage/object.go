//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// ------------------------------------------------------------
// Object Interface Implementation
// ------------------------------------------------------------

const (
	metaMtime   = "mtime"     // the meta key to store mtime in - e.g. X-Amz-Meta-Mtime
	metaMD5Hash = "md5chksum" // the meta key to store md5hash in
	// StandardTier object storage tier
	ociMetaPrefix = "opc-meta-"
)

var archive = "archive"
var infrequentAccess = "infrequentaccess"
var standard = "standard"

var storageTierMap = map[string]*string{
	archive:          &archive,
	infrequentAccess: &infrequentAccess,
	standard:         &standard,
}

var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Object describes a oci bucket object
type Object struct {
	fs           *Fs               // what this object is part of
	remote       string            // The remote path
	md5          string            // MD5 hash if known
	bytes        int64             // Size of the object
	lastModified time.Time         // The modified time of the object if known
	meta         map[string]string // The object metadata if known - may be nil
	mimeType     string            // Content-Type of the object

	// Metadata as pointers to strings as they often won't be present
	storageTier *string // e.g. Standard
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	return o.fs.split(o.remote)
}

// readMetaData gets the metadata if it hasn't already been fetched
func (o *Object) readMetaData(ctx context.Context) (err error) {
	fs.Debugf(o, "trying to read metadata %v", o.remote)
	if o.meta != nil {
		return nil
	}
	info, err := o.headObject(ctx)
	if err != nil {
		return err
	}
	return o.decodeMetaDataHead(info)
}

// headObject gets the metadata from the object unconditionally
func (o *Object) headObject(ctx context.Context) (info *objectstorage.HeadObjectResponse, err error) {
	bucketName, objectPath := o.split()
	req := objectstorage.HeadObjectRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(objectPath),
	}
	useBYOKHeadObject(o.fs, &req)
	var response objectstorage.HeadObjectResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		var err error
		response, err = o.fs.srv.HeadObject(ctx, req)
		return shouldRetry(ctx, response.HTTPResponse(), err)
	})
	if err != nil {
		if svcErr, ok := err.(common.ServiceError); ok {
			if svcErr.GetHTTPStatusCode() == http.StatusNotFound {
				return nil, fs.ErrorObjectNotFound
			}
		}
		fs.Errorf(o, "Failed to head object: %v", err)
		return nil, err
	}
	o.fs.cache.MarkOK(bucketName)
	return &response, err
}

func (o *Object) decodeMetaDataHead(info *objectstorage.HeadObjectResponse) (err error) {
	return o.setMetaData(
		info.ContentLength,
		info.ContentMd5,
		info.ContentType,
		info.LastModified,
		info.StorageTier,
		info.OpcMeta)
}

func (o *Object) decodeMetaDataObject(info *objectstorage.GetObjectResponse) (err error) {
	return o.setMetaData(
		info.ContentLength,
		info.ContentMd5,
		info.ContentType,
		info.LastModified,
		info.StorageTier,
		info.OpcMeta)
}

func (o *Object) setMetaData(
	contentLength *int64,
	contentMd5 *string,
	contentType *string,
	lastModified *common.SDKTime,
	storageTier interface{},
	meta map[string]string) error {

	if contentLength != nil {
		o.bytes = *contentLength
	}
	if contentMd5 != nil {
		md5, err := o.base64ToMd5(*contentMd5)
		if err == nil {
			o.md5 = md5
		}
	}
	o.meta = meta
	if o.meta == nil {
		o.meta = map[string]string{}
	}
	// Read MD5 from metadata if present
	if md5sumBase64, ok := o.meta[metaMD5Hash]; ok {
		md5, err := o.base64ToMd5(md5sumBase64)
		if err != nil {
			o.md5 = md5
		}
	}
	if lastModified == nil {
		o.lastModified = time.Now()
		fs.Logf(o, "Failed to read last modified")
	} else {
		o.lastModified = lastModified.Time
	}
	if contentType != nil {
		o.mimeType = *contentType
	}
	if storageTier == nil || storageTier == "" {
		o.storageTier = storageTierMap[standard]
	} else {
		tier := strings.ToLower(fmt.Sprintf("%v", storageTier))
		o.storageTier = storageTierMap[tier]
	}
	return nil
}

func (o *Object) base64ToMd5(md5sumBase64 string) (md5 string, err error) {
	md5sumBytes, err := base64.StdEncoding.DecodeString(md5sumBase64)
	if err != nil {
		fs.Debugf(o, "Failed to read md5sum from metadata %q: %v", md5sumBase64, err)
		return "", err
	} else if len(md5sumBytes) != 16 {
		fs.Debugf(o, "failed to read md5sum from metadata %q: wrong length", md5sumBase64)
		return "", fmt.Errorf("failed to read md5sum from metadata %q: wrong length", md5sumBase64)
	}
	return hex.EncodeToString(md5sumBytes), nil
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

// GetTier returns storage class as string
func (o *Object) GetTier() string {
	if o.storageTier == nil || *o.storageTier == "" {
		return standard
	}
	return *o.storageTier
}

// SetTier performs changing storage class
func (o *Object) SetTier(tier string) (err error) {
	ctx := context.TODO()
	tier = strings.ToLower(tier)
	bucketName, bucketPath := o.split()
	tierEnum, ok := objectstorage.GetMappingStorageTierEnum(tier)
	if !ok {
		return fmt.Errorf("not a valid storage tier %v ", tier)
	}

	req := objectstorage.UpdateObjectStorageTierRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucketName),
		UpdateObjectStorageTierDetails: objectstorage.UpdateObjectStorageTierDetails{
			ObjectName:  common.String(bucketPath),
			StorageTier: tierEnum,
		},
	}
	_, err = o.fs.srv.UpdateObjectStorageTier(ctx, req)
	if err != nil {
		return err
	}
	o.storageTier = storageTierMap[tier]
	return err
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	// Convert base64 encoded md5 into lower case hex
	if o.md5 == "" {
		err := o.readMetaData(ctx)
		if err != nil {
			return "", err
		}
	}
	return o.md5, nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned to the http headers
func (o *Object) ModTime(ctx context.Context) (result time.Time) {
	if o.fs.ci.UseServerModTime {
		return o.lastModified
	}
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	// read mtime out of metadata if available
	d, ok := o.meta[metaMtime]
	if !ok || d == "" {
		return o.lastModified
	}
	modTime, err := swift.FloatStringToTime(d)
	if err != nil {
		fs.Logf(o, "Failed to read mtime from object: %v", err)
		return o.lastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	err := o.readMetaData(ctx)
	if err != nil {
		return err
	}
	o.meta[metaMtime] = swift.TimeToFloatString(modTime)
	_, err = o.fs.Copy(ctx, o, o.remote)
	return err
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	bucketName, bucketPath := o.split()
	req := objectstorage.DeleteObjectRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(bucketPath),
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.DeleteObject(ctx, req)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	return err
}

// Open object file
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	bucketName, bucketPath := o.split()
	req := objectstorage.GetObjectRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(bucketPath),
	}
	o.applyGetObjectOptions(&req, options...)
	useBYOKGetObject(o.fs, &req)
	var resp objectstorage.GetObjectResponse
	err := o.fs.pacer.Call(func() (bool, error) {
		var err error
		resp, err = o.fs.srv.GetObject(ctx, req)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	if err != nil {
		return nil, err
	}
	// read size from ContentLength or ContentRange
	bytes := resp.ContentLength
	if resp.ContentRange != nil {
		var contentRange = *resp.ContentRange
		slash := strings.IndexRune(contentRange, '/')
		if slash >= 0 {
			i, err := strconv.ParseInt(contentRange[slash+1:], 10, 64)
			if err == nil {
				bytes = &i
			} else {
				fs.Debugf(o, "Failed to find parse integer from in %q: %v", contentRange, err)
			}
		} else {
			fs.Debugf(o, "Failed to find length in %q", contentRange)
		}
	}
	err = o.decodeMetaDataObject(&resp)
	if err != nil {
		return nil, err
	}
	o.bytes = *bytes
	return resp.HTTPResponse().Body, nil
}

func isZeroLength(streamReader io.Reader) bool {
	switch v := streamReader.(type) {
	case *bytes.Buffer:
		return v.Len() == 0
	case *bytes.Reader:
		return v.Len() == 0
	case *strings.Reader:
		return v.Len() == 0
	case *os.File:
		fi, err := v.Stat()
		if err != nil {
			return false
		}
		return fi.Size() == 0
	default:
		return false
	}
}

// Update an object if it has changed
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	bucketName, _ := o.split()
	err = o.fs.makeBucket(ctx, bucketName)
	if err != nil {
		return err
	}

	// determine if we like upload single or multipart.
	size := src.Size()
	multipart := size < 0 || size >= int64(o.fs.opt.UploadCutoff)
	if isZeroLength(in) {
		multipart = false
	}
	if multipart {
		err = o.uploadMultipart(ctx, src, in, options...)
		if err != nil {
			return err
		}
	} else {
		ui, err := o.prepareUpload(ctx, src, options)
		if err != nil {
			return fmt.Errorf("failed to prepare upload: %w", err)
		}
		var resp objectstorage.PutObjectResponse
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			ui.req.PutObjectBody = io.NopCloser(in)
			resp, err = o.fs.srv.PutObject(ctx, *ui.req)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		if err != nil {
			fs.Errorf(o, "put object failed %v", err)
			return err
		}
	}
	// Read the metadata from the newly created object
	o.meta = nil // wipe old metadata
	return o.readMetaData(ctx)
}

func (o *Object) applyPutOptions(req *objectstorage.PutObjectRequest, options ...fs.OpenOption) {
	// Apply upload options
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			req.CacheControl = common.String(value)
		case "content-disposition":
			req.ContentDisposition = common.String(value)
		case "content-encoding":
			req.ContentEncoding = common.String(value)
		case "content-language":
			req.ContentLanguage = common.String(value)
		case "content-type":
			req.ContentType = common.String(value)
		default:
			if strings.HasPrefix(lowerKey, ociMetaPrefix) {
				req.OpcMeta[lowerKey] = value
			} else {
				fs.Errorf(o, "Don't know how to set key %q on upload", key)
			}
		}
	}
}

func (o *Object) applyGetObjectOptions(req *objectstorage.GetObjectRequest, options ...fs.OpenOption) {
	fs.FixRangeOption(options, o.bytes)
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
	// Apply upload options
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			req.HttpResponseCacheControl = common.String(value)
		case "content-disposition":
			req.HttpResponseContentDisposition = common.String(value)
		case "content-encoding":
			req.HttpResponseContentEncoding = common.String(value)
		case "content-language":
			req.HttpResponseContentLanguage = common.String(value)
		case "content-type":
			req.HttpResponseContentType = common.String(value)
		case "range":
			// do nothing
		default:
			fs.Errorf(o, "Don't know how to set key %q on upload", key)
		}
	}
}

func (o *Object) applyMultipartUploadOptions(putReq *objectstorage.PutObjectRequest, req *objectstorage.CreateMultipartUploadRequest) {
	req.ContentType = putReq.ContentType
	req.ContentLanguage = putReq.ContentLanguage
	req.ContentEncoding = putReq.ContentEncoding
	req.ContentDisposition = putReq.ContentDisposition
	req.CacheControl = putReq.CacheControl
	req.Metadata = metadataWithOpcPrefix(putReq.OpcMeta)
	req.OpcSseCustomerAlgorithm = putReq.OpcSseCustomerAlgorithm
	req.OpcSseCustomerKey = putReq.OpcSseCustomerKey
	req.OpcSseCustomerKeySha256 = putReq.OpcSseCustomerKeySha256
	req.OpcSseKmsKeyId = putReq.OpcSseKmsKeyId
}

func (o *Object) applyPartUploadOptions(putReq *objectstorage.PutObjectRequest, req *objectstorage.UploadPartRequest) {
	req.OpcSseCustomerAlgorithm = putReq.OpcSseCustomerAlgorithm
	req.OpcSseCustomerKey = putReq.OpcSseCustomerKey
	req.OpcSseCustomerKeySha256 = putReq.OpcSseCustomerKeySha256
	req.OpcSseKmsKeyId = putReq.OpcSseKmsKeyId
}

func metadataWithOpcPrefix(src map[string]string) map[string]string {
	dst := make(map[string]string)
	for lowerKey, value := range src {
		if !strings.HasPrefix(lowerKey, ociMetaPrefix) {
			dst[ociMetaPrefix+lowerKey] = value
		}
	}
	return dst
}
