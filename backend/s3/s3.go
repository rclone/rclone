// Package s3 provides an interface to Amazon S3 oject storage
package s3

// FIXME need to prevent anything but ListDir working for s3://

/*
Progress of port to aws-sdk

 * Don't really need o.meta at all?

What happens if you CTRL-C a multipart upload
  * get an incomplete upload
  * disappears when you delete the bucket
*/

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/rest"
	"github.com/ncw/swift"
	"github.com/pkg/errors"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "s3",
		Description: "Amazon S3 (also Dreamhost, Ceph, Minio, IBM COS)",
		NewFs:       NewFs,
		// AWS endpoints: http://docs.amazonwebservices.com/general/latest/gr/rande.html#s3_region
		Options: []fs.Option{{
			Name: "env_auth",
			Help: "Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars). Only applies if access_key_id and secret_access_key is blank.",
			Examples: []fs.OptionExample{
				{
					Value: "false",
					Help:  "Enter AWS credentials in the next step",
				}, {
					Value: "true",
					Help:  "Get AWS credentials from the environment (env vars or IAM)",
				},
			},
		}, {
			Name: "access_key_id",
			Help: "AWS Access Key ID - leave blank for anonymous access or runtime credentials.",
		}, {
			Name: "secret_access_key",
			Help: "AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.",
		}, {
			Name: "region",
			Help: "Region to connect to.  Leave blank if you are using an S3 clone and you don't have a region.",
			Examples: []fs.OptionExample{{
				Value: "us-east-1",
				Help:  "The default endpoint - a good choice if you are unsure.\nUS Region, Northern Virginia or Pacific Northwest.\nLeave location constraint empty.",
			}, {
				Value: "us-east-2",
				Help:  "US East (Ohio) Region\nNeeds location constraint us-east-2.",
			}, {
				Value: "us-west-2",
				Help:  "US West (Oregon) Region\nNeeds location constraint us-west-2.",
			}, {
				Value: "us-west-1",
				Help:  "US West (Northern California) Region\nNeeds location constraint us-west-1.",
			}, {
				Value: "ca-central-1",
				Help:  "Canada (Central) Region\nNeeds location constraint ca-central-1.",
			}, {
				Value: "eu-west-1",
				Help:  "EU (Ireland) Region\nNeeds location constraint EU or eu-west-1.",
			}, {
				Value: "eu-west-2",
				Help:  "EU (London) Region\nNeeds location constraint eu-west-2.",
			}, {
				Value: "eu-central-1",
				Help:  "EU (Frankfurt) Region\nNeeds location constraint eu-central-1.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region\nNeeds location constraint ap-southeast-1.",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region\nNeeds location constraint ap-southeast-2.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Tokyo) Region\nNeeds location constraint ap-northeast-1.",
			}, {
				Value: "ap-northeast-2",
				Help:  "Asia Pacific (Seoul)\nNeeds location constraint ap-northeast-2.",
			}, {
				Value: "ap-south-1",
				Help:  "Asia Pacific (Mumbai)\nNeeds location constraint ap-south-1.",
			}, {
				Value: "sa-east-1",
				Help:  "South America (Sao Paulo) Region\nNeeds location constraint sa-east-1.",
			}, {
				Value: "other-v2-signature",
				Help:  "Use this only if v4 signatures don't work, eg pre Jewel/v10 CEPH.\nSet this and make sure you set the endpoint.",
			}},
		}, {
			Name: "endpoint",
			Help: "Endpoint for S3 API.\nLeave blank if using AWS to use the default endpoint for the region.\nSpecify if using an S3 clone such as Ceph.",
		}, {
			Name: "location_constraint",
			Help: "Location constraint - must be set to match the Region. Used when creating buckets only.",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Empty for US Region, Northern Virginia or Pacific Northwest.",
			}, {
				Value: "us-east-2",
				Help:  "US East (Ohio) Region.",
			}, {
				Value: "us-west-2",
				Help:  "US West (Oregon) Region.",
			}, {
				Value: "us-west-1",
				Help:  "US West (Northern California) Region.",
			}, {
				Value: "ca-central-1",
				Help:  "Canada (Central) Region.",
			}, {
				Value: "eu-west-1",
				Help:  "EU (Ireland) Region.",
			}, {
				Value: "eu-west-2",
				Help:  "EU (London) Region.",
			}, {
				Value: "EU",
				Help:  "EU Region.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region.",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Tokyo) Region.",
			}, {
				Value: "ap-northeast-2",
				Help:  "Asia Pacific (Seoul)",
			}, {
				Value: "ap-south-1",
				Help:  "Asia Pacific (Mumbai)",
			}, {
				Value: "sa-east-1",
				Help:  "South America (Sao Paulo) Region.",
			}},
		}, {
			Name: "acl",
			Help: "Canned ACL used when creating buckets and/or storing objects in S3.\nFor more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl",
			Examples: []fs.OptionExample{{
				Value: "private",
				Help:  "Owner gets FULL_CONTROL. No one else has access rights (default).",
			}, {
				Value: "public-read",
				Help:  "Owner gets FULL_CONTROL. The AllUsers group gets READ access.",
			}, {
				Value: "public-read-write",
				Help:  "Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.\nGranting this on a bucket is generally not recommended.",
			}, {
				Value: "authenticated-read",
				Help:  "Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.",
			}, {
				Value: "bucket-owner-read",
				Help:  "Object owner gets FULL_CONTROL. Bucket owner gets READ access.\nIf you specify this canned ACL when creating a bucket, Amazon S3 ignores it.",
			}, {
				Value: "bucket-owner-full-control",
				Help:  "Both the object owner and the bucket owner get FULL_CONTROL over the object.\nIf you specify this canned ACL when creating a bucket, Amazon S3 ignores it.",
			}},
		}, {
			Name: "server_side_encryption",
			Help: "The server-side encryption algorithm used when storing this object in S3.",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}, {
				Value: "AES256",
				Help:  "AES256",
			}},
		}, {
			Name: "storage_class",
			Help: "The storage class to use when storing objects in S3.",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "STANDARD",
				Help:  "Standard storage class",
			}, {
				Value: "REDUCED_REDUNDANCY",
				Help:  "Reduced redundancy storage class",
			}, {
				Value: "STANDARD_IA",
				Help:  "Standard Infrequent Access storage class",
			}},
		}},
	})
	flags.VarP(&s3ChunkSize, "s3-chunk-size", "", "Chunk size to use for uploading")
}

// Constants
const (
	metaMtime      = "Mtime"                       // the meta key to store mtime in - eg X-Amz-Meta-Mtime
	metaMD5Hash    = "Md5chksum"                   // the meta key to store md5hash in
	listChunkSize  = 1000                          // number of items to read at once
	maxRetries     = 10                            // number of retries to make of operations
	maxSizeForCopy = 5 * 1024 * 1024 * 1024        // The maximum size of object we can COPY
	maxFileSize    = 5 * 1024 * 1024 * 1024 * 1024 // largest possible upload file size
)

// Globals
var (
	// Flags
	s3ACL          = flags.StringP("s3-acl", "", "", "Canned ACL used when creating buckets and/or storing objects in S3")
	s3StorageClass = flags.StringP("s3-storage-class", "", "", "Storage class to use when uploading S3 objects (STANDARD|REDUCED_REDUNDANCY|STANDARD_IA)")
	s3ChunkSize    = fs.SizeSuffix(s3manager.MinUploadPartSize)
)

// Fs represents a remote s3 server
type Fs struct {
	name               string           // the name of the remote
	root               string           // root of the bucket - ignore all objects above this
	features           *fs.Features     // optional features
	c                  *s3.S3           // the connection to the s3 server
	ses                *session.Session // the s3 session
	bucket             string           // the bucket we are working on
	bucketOKMu         sync.Mutex       // mutex to protect bucket OK
	bucketOK           bool             // true if we have created the bucket
	bucketDeleted      bool             // true if we have deleted the bucket
	acl                string           // ACL for new buckets / objects
	locationConstraint string           // location constraint of new buckets
	sse                string           // the type of server-side encryption
	storageClass       string           // storage class
}

// Object describes a s3 object
type Object struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta & mimeType - to fill
	// that in you need to call readMetaData
	fs           *Fs                // what this object is part of
	remote       string             // The remote path
	etag         string             // md5sum of the object
	bytes        int64              // size of the object
	lastModified time.Time          // Last modified
	meta         map[string]*string // The object metadata if known - may be nil
	mimeType     string             // MimeType of object - may be ""
}

// ------------------------------------------------------------

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
		return fmt.Sprintf("S3 bucket %s", f.bucket)
	}
	return fmt.Sprintf("S3 bucket %s path %s", f.bucket, f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a s3 path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a s3 'url'
func s3ParsePath(path string) (bucket, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("couldn't parse bucket out of s3 path %q", path)
	} else {
		bucket, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// s3Connection makes a connection to s3
func s3Connection(name string) (*s3.S3, *session.Session, error) {
	// Make the auth
	v := credentials.Value{
		AccessKeyID:     config.FileGet(name, "access_key_id"),
		SecretAccessKey: config.FileGet(name, "secret_access_key"),
		SessionToken:    config.FileGet(name, "session_token"),
	}

	lowTimeoutClient := &http.Client{Timeout: 1 * time.Second} // low timeout to ec2 metadata service
	def := defaults.Get()
	def.Config.HTTPClient = lowTimeoutClient

	// first provider to supply a credential set "wins"
	providers := []credentials.Provider{
		// use static credentials if they're present (checked by provider)
		&credentials.StaticProvider{Value: v},

		// * Access Key ID:     AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY
		// * Secret Access Key: AWS_SECRET_ACCESS_KEY or AWS_SECRET_KEY
		&credentials.EnvProvider{},

		// Pick up IAM role if we're in an ECS task
		defaults.RemoteCredProvider(*def.Config, def.Handlers),

		// Pick up IAM role in case we're on EC2
		&ec2rolecreds.EC2RoleProvider{
			Client: ec2metadata.New(session.New(), &aws.Config{
				HTTPClient: lowTimeoutClient,
			}),
			ExpiryWindow: 3,
		},
	}
	cred := credentials.NewChainCredentials(providers)

	switch {
	case config.FileGetBool(name, "env_auth", false):
		// No need for empty checks if "env_auth" is true
	case v.AccessKeyID == "" && v.SecretAccessKey == "":
		// if no access key/secret and iam is explicitly disabled then fall back to anon interaction
		cred = credentials.AnonymousCredentials
	case v.AccessKeyID == "":
		return nil, nil, errors.New("access_key_id not found")
	case v.SecretAccessKey == "":
		return nil, nil, errors.New("secret_access_key not found")
	}

	endpoint := config.FileGet(name, "endpoint")
	region := config.FileGet(name, "region")
	if region == "" && endpoint == "" {
		endpoint = "https://s3.amazonaws.com/"
	}
	if region == "" {
		region = "us-east-1"
	}
	awsConfig := aws.NewConfig().
		WithRegion(region).
		WithMaxRetries(maxRetries).
		WithCredentials(cred).
		WithEndpoint(endpoint).
		WithHTTPClient(fshttp.NewClient(fs.Config)).
		WithS3ForcePathStyle(true)
	// awsConfig.WithLogLevel(aws.LogDebugWithSigning)
	ses := session.New()
	c := s3.New(ses, awsConfig)
	if region == "other-v2-signature" {
		fs.Debugf(name, "Using v2 auth")
		signer := func(req *request.Request) {
			// Ignore AnonymousCredentials object
			if req.Config.Credentials == credentials.AnonymousCredentials {
				return
			}
			sign(v.AccessKeyID, v.SecretAccessKey, req.HTTPRequest)
		}
		c.Handlers.Sign.Clear()
		c.Handlers.Sign.PushBackNamed(corehandlers.BuildContentLengthHandler)
		c.Handlers.Sign.PushBack(signer)
	}
	return c, ses, nil
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	bucket, directory, err := s3ParsePath(root)
	if err != nil {
		return nil, err
	}
	c, ses, err := s3Connection(name)
	if err != nil {
		return nil, err
	}
	f := &Fs{
		name:               name,
		c:                  c,
		bucket:             bucket,
		ses:                ses,
		acl:                config.FileGet(name, "acl"),
		root:               directory,
		locationConstraint: config.FileGet(name, "location_constraint"),
		sse:                config.FileGet(name, "server_side_encryption"),
		storageClass:       config.FileGet(name, "storage_class"),
	}
	f.features = (&fs.Features{
		ReadMimeType:  true,
		WriteMimeType: true,
		BucketBased:   true,
	}).Fill(f)
	if *s3ACL != "" {
		f.acl = *s3ACL
	}
	if *s3StorageClass != "" {
		f.storageClass = *s3StorageClass
	}
	if s3ChunkSize < fs.SizeSuffix(s3manager.MinUploadPartSize) {
		return nil, errors.Errorf("s3 chunk size must be >= %v", fs.SizeSuffix(s3manager.MinUploadPartSize))
	}
	if f.root != "" {
		f.root += "/"
		// Check to see if the object exists
		req := s3.HeadObjectInput{
			Bucket: &f.bucket,
			Key:    &directory,
		}
		_, err = f.c.HeadObject(&req)
		if err == nil {
			f.root = path.Dir(directory)
			if f.root == "." {
				f.root = ""
			} else {
				f.root += "/"
			}
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}
	// f.listMultipartUploads()
	return f, nil
}

// Return an Object from a path
//
//If it can't be found it returns the error ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *s3.Object) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		if info.LastModified == nil {
			fs.Logf(o, "Failed to read last modified")
			o.lastModified = time.Now()
		} else {
			o.lastModified = *info.LastModified
		}
		o.etag = aws.StringValue(info.ETag)
		o.bytes = aws.Int64Value(info.Size)
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *s3.Object, isDirectory bool) error

// list the objects into the function supplied
//
// dir is the starting directory, "" for root
//
// Set recurse to read sub directories
func (f *Fs) list(dir string, recurse bool, fn listFn) error {
	root := f.root
	if dir != "" {
		root += dir + "/"
	}
	maxKeys := int64(listChunkSize)
	delimiter := ""
	if !recurse {
		delimiter = "/"
	}
	var marker *string
	for {
		// FIXME need to implement ALL loop
		req := s3.ListObjectsInput{
			Bucket:    &f.bucket,
			Delimiter: &delimiter,
			Prefix:    &root,
			MaxKeys:   &maxKeys,
			Marker:    marker,
		}
		resp, err := f.c.ListObjects(&req)
		if err != nil {
			if awsErr, ok := err.(awserr.RequestFailure); ok {
				if awsErr.StatusCode() == http.StatusNotFound {
					err = fs.ErrorDirNotFound
				}
			}
			return err
		}
		rootLength := len(f.root)
		if !recurse {
			for _, commonPrefix := range resp.CommonPrefixes {
				if commonPrefix.Prefix == nil {
					fs.Logf(f, "Nil common prefix received")
					continue
				}
				remote := *commonPrefix.Prefix
				if !strings.HasPrefix(remote, f.root) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[rootLength:]
				if strings.HasSuffix(remote, "/") {
					remote = remote[:len(remote)-1]
				}
				err = fn(remote, &s3.Object{Key: &remote}, true)
				if err != nil {
					return err
				}
			}
		}
		for _, object := range resp.Contents {
			key := aws.StringValue(object.Key)
			if !strings.HasPrefix(key, f.root) {
				fs.Logf(f, "Odd name received %q", key)
				continue
			}
			remote := key[rootLength:]
			// is this a directory marker?
			if (strings.HasSuffix(remote, "/") || remote == "") && *object.Size == 0 {
				if recurse {
					// add a directory in if --fast-list since will have no prefixes
					remote = remote[:len(remote)-1]
					err = fn(remote, &s3.Object{Key: &remote}, true)
					if err != nil {
						return err
					}
				}
				continue // skip directory marker
			}
			err = fn(remote, object, false)
			if err != nil {
				return err
			}
		}
		if !aws.BoolValue(resp.IsTruncated) {
			break
		}
		// Use NextMarker if set, otherwise use last Key
		if resp.NextMarker == nil || *resp.NextMarker == "" {
			if len(resp.Contents) == 0 {
				return errors.New("s3 protocol error: received listing with IsTruncated set, no NextMarker and no Contents")
			}
			marker = resp.Contents[len(resp.Contents)-1].Key
		} else {
			marker = resp.NextMarker
		}
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(remote string, object *s3.Object, isDirectory bool) (fs.DirEntry, error) {
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
	err = f.list(dir, false, func(remote string, object *s3.Object, isDirectory bool) error {
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
	req := s3.ListBucketsInput{}
	resp, err := f.c.ListBuckets(&req)
	if err != nil {
		return nil, err
	}
	for _, bucket := range resp.Buckets {
		d := fs.NewDir(aws.StringValue(bucket.Name), aws.TimeValue(bucket.CreationDate))
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
	err = f.list(dir, true, func(remote string, object *s3.Object, isDirectory bool) error {
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

// Put the Object into the bucket
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// Check if the bucket exists
//
// NB this can return incorrect results if called immediately after bucket deletion
func (f *Fs) dirExists() (bool, error) {
	req := s3.HeadBucketInput{
		Bucket: &f.bucket,
	}
	_, err := f.c.HeadBucket(&req)
	if err == nil {
		return true, nil
	}
	if err, ok := err.(awserr.RequestFailure); ok {
		if err.StatusCode() == http.StatusNotFound {
			return false, nil
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
	req := s3.CreateBucketInput{
		Bucket: &f.bucket,
		ACL:    &f.acl,
	}
	if f.locationConstraint != "" {
		req.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
			LocationConstraint: &f.locationConstraint,
		}
	}
	_, err := f.c.CreateBucket(&req)
	if err, ok := err.(awserr.Error); ok {
		if err.Code() == "BucketAlreadyOwnedByYou" {
			err = nil
		}
	}
	if err == nil {
		f.bucketOK = true
		f.bucketDeleted = false
	}
	return err
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.root != "" || dir != "" {
		return nil
	}
	req := s3.DeleteBucketInput{
		Bucket: &f.bucket,
	}
	_, err := f.c.DeleteBucket(&req)
	if err == nil {
		f.bucketOK = false
		f.bucketDeleted = true
	}
	return err
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// pathEscape escapes s as for a URL path.  It uses rest.URLPathEscape
// but also escapes '+' for S3 and Digital Ocean spaces compatibility
func pathEscape(s string) string {
	return strings.Replace(rest.URLPathEscape(s), "+", "%2B", -1)
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
	source := pathEscape(srcFs.bucket + "/" + srcFs.root + srcObj.remote)
	req := s3.CopyObjectInput{
		Bucket:            &f.bucket,
		Key:               &key,
		CopySource:        &source,
		MetadataDirective: aws.String(s3.MetadataDirectiveCopy),
	}
	_, err = f.c.CopyObject(&req)
	if err != nil {
		return nil, err
	}
	return f.NewObject(remote)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
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

var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	hash := strings.Trim(strings.ToLower(o.etag), `"`)
	// Check the etag is a valid md5sum
	if !matchMd5.MatchString(hash) {
		err := o.readMetaData()
		if err != nil {
			return "", err
		}

		if md5sum, ok := o.meta[metaMD5Hash]; ok {
			md5sumBytes, err := base64.StdEncoding.DecodeString(*md5sum)
			if err != nil {
				return "", err
			}
			hash = hex.EncodeToString(md5sumBytes)
		} else {
			hash = ""
		}
	}
	return hash, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.meta != nil {
		return nil
	}
	key := o.fs.root + o.remote
	req := s3.HeadObjectInput{
		Bucket: &o.fs.bucket,
		Key:    &key,
	}
	resp, err := o.fs.c.HeadObject(&req)
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			if awsErr.StatusCode() == http.StatusNotFound {
				return fs.ErrorObjectNotFound
			}
		}
		return err
	}
	var size int64
	// Ignore missing Content-Length assuming it is 0
	// Some versions of ceph do this due their apache proxies
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	o.etag = aws.StringValue(resp.ETag)
	o.bytes = size
	o.meta = resp.Metadata
	if resp.LastModified == nil {
		fs.Logf(o, "Failed to read last modified from HEAD: %v", err)
		o.lastModified = time.Now()
	} else {
		o.lastModified = *resp.LastModified
	}
	o.mimeType = aws.StringValue(resp.ContentType)
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	// read mtime out of metadata if available
	d, ok := o.meta[metaMtime]
	if !ok || d == nil {
		// fs.Debugf(o, "No metadata")
		return o.lastModified
	}
	modTime, err := swift.FloatStringToTime(*d)
	if err != nil {
		fs.Logf(o, "Failed to read mtime from object: %v", err)
		return o.lastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	err := o.readMetaData()
	if err != nil {
		return err
	}
	o.meta[metaMtime] = aws.String(swift.TimeToFloatString(modTime))

	if o.bytes >= maxSizeForCopy {
		fs.Debugf(o, "SetModTime is unsupported for objects bigger than %v bytes", fs.SizeSuffix(maxSizeForCopy))
		return nil
	}

	// Guess the content type
	mimeType := fs.MimeType(o)

	// Copy the object to itself to update the metadata
	key := o.fs.root + o.remote
	sourceKey := o.fs.bucket + "/" + key
	directive := s3.MetadataDirectiveReplace // replace metadata with that passed in
	req := s3.CopyObjectInput{
		Bucket:            &o.fs.bucket,
		ACL:               &o.fs.acl,
		Key:               &key,
		ContentType:       &mimeType,
		CopySource:        aws.String(pathEscape(sourceKey)),
		Metadata:          o.meta,
		MetadataDirective: &directive,
	}
	_, err = o.fs.c.CopyObject(&req)
	return err
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	key := o.fs.root + o.remote
	req := s3.GetObjectInput{
		Bucket: &o.fs.bucket,
		Key:    &key,
	}
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
	resp, err := o.fs.c.GetObject(&req)
	if err, ok := err.(awserr.RequestFailure); ok {
		if err.Code() == "InvalidObjectState" {
			return nil, errors.Errorf("Object in GLACIER, restore first: %v", key)
		}
	}
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update the Object from in with modTime and size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	err := o.fs.Mkdir("")
	if err != nil {
		return err
	}
	modTime := src.ModTime()
	size := src.Size()

	uploader := s3manager.NewUploader(o.fs.ses, func(u *s3manager.Uploader) {
		u.Concurrency = 2
		u.LeavePartsOnError = false
		u.S3 = o.fs.c
		u.PartSize = int64(s3ChunkSize)

		if size == -1 {
			// Make parts as small as possible while still being able to upload to the
			// S3 file size limit. Rounded up to nearest MB.
			u.PartSize = (((maxFileSize / s3manager.MaxUploadParts) >> 20) + 1) << 20
			return
		}
		// Adjust PartSize until the number of parts is small enough.
		if size/u.PartSize >= s3manager.MaxUploadParts {
			// Calculate partition size rounded up to the nearest MB
			u.PartSize = (((size / s3manager.MaxUploadParts) >> 20) + 1) << 20
		}
	})

	// Set the mtime in the meta data
	metadata := map[string]*string{
		metaMtime: aws.String(swift.TimeToFloatString(modTime)),
	}

	if size > uploader.PartSize {
		hash, err := src.Hash(hash.MD5)

		if err == nil && matchMd5.MatchString(hash) {
			hashBytes, err := hex.DecodeString(hash)

			if err == nil {
				metadata[metaMD5Hash] = aws.String(base64.StdEncoding.EncodeToString(hashBytes))
			}
		}
	}

	// Guess the content type
	mimeType := fs.MimeType(src)

	key := o.fs.root + o.remote
	req := s3manager.UploadInput{
		Bucket:      &o.fs.bucket,
		ACL:         &o.fs.acl,
		Key:         &key,
		Body:        in,
		ContentType: &mimeType,
		Metadata:    metadata,
		//ContentLength: &size,
	}
	if o.fs.sse != "" {
		req.ServerSideEncryption = &o.fs.sse
	}
	if o.fs.storageClass != "" {
		req.StorageClass = &o.fs.storageClass
	}
	_, err = uploader.Upload(&req)
	if err != nil {
		return err
	}

	// Read the metadata from the newly created object
	o.meta = nil // wipe old metadata
	err = o.readMetaData()
	return err
}

// Remove an object
func (o *Object) Remove() error {
	key := o.fs.root + o.remote
	req := s3.DeleteObjectInput{
		Bucket: &o.fs.bucket,
		Key:    &key,
	}
	_, err := o.fs.c.DeleteObject(&req)
	return err
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
	_ fs.Fs          = &Fs{}
	_ fs.Copier      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
