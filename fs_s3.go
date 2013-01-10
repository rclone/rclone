// S3 interface
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/ncw/swift"
	"io"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Constants
const (
	metaMtime = "X-Amz-Meta-Mtime" // the meta key to store mtime in
)

// FsS3 represents a remote s3 server
type FsS3 struct {
	c      *s3.S3     // the connection to the s3 server
	b      *s3.Bucket // the connection to the bucket
	bucket string     // the bucket we are working on
	perm   s3.ACL     // permissions for new buckets / objects
}

// FsObjectS3 describes a s3 object
type FsObjectS3 struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta - to fill that in need to call
	// readMetaData
	s3           *FsS3      // what this object is part of
	remote       string     // The remote path
	etag         string     // md5sum of the object
	bytes        int64      // size of the object
	lastModified time.Time  // Last modified
	meta         s3.Headers // The object metadata if known - may be nil
}

// ------------------------------------------------------------

// Globals
var (
	// Flags
	awsAccessKeyId     = flag.String("aws-access-key-id", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS Access Key ID. Defaults to environment var AWS_ACCESS_KEY_ID.")
	awsSecretAccessKey = flag.String("aws-secret-access-key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS Secret Access Key (password). Defaults to environment var AWS_SECRET_ACCESS_KEY.")
	// AWS endpoints: http://docs.amazonwebservices.com/general/latest/gr/rande.html#s3_region
	s3Endpoint           = flag.String("s3-endpoint", os.Getenv("S3_ENDPOINT"), "S3 Endpoint. Defaults to environment var S3_ENDPOINT then https://s3.amazonaws.com/.")
	s3LocationConstraint = flag.String("s3-location-constraint", os.Getenv("S3_LOCATION_CONSTRAINT"), "Location constraint for creating buckets only. Defaults to environment var S3_LOCATION_CONSTRAINT.")
)

// String converts this FsS3 to a string
func (f *FsS3) String() string {
	return fmt.Sprintf("S3 bucket %s", f.bucket)
}

// Pattern to match a s3 url
var s3Match = regexp.MustCompile(`^s3://([^/]+)(.*)$`)

// parseParse parses a s3 'url'
func s3ParsePath(path string) (bucket, directory string, err error) {
	parts := s3Match.FindAllStringSubmatch(path, -1)
	if len(parts) != 1 || len(parts[0]) != 3 {
		err = fmt.Errorf("Couldn't parse s3 url %q", path)
	} else {
		bucket, directory = parts[0][1], parts[0][2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// s3Connection makes a connection to s3
func s3Connection() (*s3.S3, error) {
	// Make the auth
	if *awsAccessKeyId == "" {
		return nil, errors.New("Need -aws-access-key-id or environmental variable AWS_ACCESS_KEY_ID")
	}
	if *awsSecretAccessKey == "" {
		return nil, errors.New("Need -aws-secret-access-key or environmental variable AWS_SECRET_ACCESS_KEY")
	}
	auth := aws.Auth{*awsAccessKeyId, *awsSecretAccessKey}

	// FIXME look through all the regions by name and use one of them if found

	// Synthesize the region
	if *s3Endpoint == "" {
		*s3Endpoint = "https://s3.amazonaws.com/"
	}
	region := aws.Region{
		Name:                 "s3",
		S3Endpoint:           *s3Endpoint,
		S3LocationConstraint: false,
	}
	if *s3LocationConstraint != "" {
		region.Name = *s3LocationConstraint
		region.S3LocationConstraint = true
	}

	c := s3.New(auth, region)
	return c, nil
}

// NewFsS3 contstructs an FsS3 from the path, bucket:path
func NewFsS3(path string) (*FsS3, error) {
	bucket, directory, err := s3ParsePath(path)
	if err != nil {
		return nil, err
	}
	if directory != "" {
		return nil, fmt.Errorf("Directories not supported yet in %q: %q", path, directory)
	}
	c, err := s3Connection()
	if err != nil {
		return nil, err
	}
	f := &FsS3{
		c:      c,
		bucket: bucket,
		b:      c.Bucket(bucket),
		perm:   s3.Private, // FIXME need user to specify
	}
	return f, nil
}

// Lists the buckets
func S3Buckets() {
	c, err := s3Connection()
	if err != nil {
		stats.Error()
		log.Fatalf("Couldn't connect: %s", err)
	}
	buckets, err := c.List()
	if err != nil {
		stats.Error()
		log.Fatalf("Couldn't list buckets: %s", err)
	}
	for _, bucket := range buckets.Buckets {
		fmt.Printf("%12s %s\n", bucket.CreationDate, bucket.Name)
	}
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsS3) NewFsObjectWithInfo(remote string, info *s3.Key) FsObject {
	fs := &FsObjectS3{
		s3:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		var err error
		fs.lastModified, err = time.Parse(time.RFC3339, info.LastModified)
		if err != nil {
			FsLog(fs, "Failed to read last modified: %s", err)
			fs.lastModified = time.Now()
		}
		fs.etag = info.ETag
		fs.bytes = info.Size
	} else {
		err := fs.readMetaData() // reads info and meta, returning an error
		if err != nil {
			// logged already FsDebug("Failed to read info: %s", err)
			return nil
		}
	}
	return fs
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsS3) NewFsObject(remote string) FsObject {
	return f.NewFsObjectWithInfo(remote, nil)
}

// Walk the path returning a channel of FsObjects
func (f *FsS3) List() FsObjectsChan {
	out := make(FsObjectsChan, *checkers)
	go func() {
		// FIXME need to implement ALL loop
		objects, err := f.b.List("", "", "", 10000)
		if err != nil {
			stats.Error()
			log.Printf("Couldn't read bucket %q: %s", f.bucket, err)
		} else {
			for i := range objects.Contents {
				object := &objects.Contents[i]
				if fs := f.NewFsObjectWithInfo(object.Key, object); fs != nil {
					out <- fs
				}
			}
		}
		close(out)
	}()
	return out
}

// Put the FsObject into the bucket
func (f *FsS3) Put(in io.Reader, remote string, modTime time.Time, size int64) (FsObject, error) {
	// Temporary FsObject under construction
	fs := &FsObjectS3{s3: f, remote: remote}

	// Set the mtime in the headers
	headers := s3.Headers{
		metaMtime: swift.TimeToFloatString(modTime),
	}

	// Guess the content type
	contentType := mime.TypeByExtension(path.Ext(remote))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := fs.s3.b.PutReaderHeaders(remote, in, size, contentType, f.perm, headers)
	return fs, err
}

// Mkdir creates the bucket if it doesn't exist
func (f *FsS3) Mkdir() error {
	err := f.b.PutBucket(f.perm)
	if err, ok := err.(*s3.Error); ok {
		if err.Code == "BucketAlreadyOwnedByYou" {
			return nil
		}
	}
	return err
}

// Rmdir deletes the bucket
//
// Returns an error if it isn't empty
func (f *FsS3) Rmdir() error {
	return f.b.DelBucket()
}

// ------------------------------------------------------------

// Return the remote path
func (fs *FsObjectS3) Remote() string {
	return fs.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (fs *FsObjectS3) Md5sum() (string, error) {
	return strings.Trim(strings.ToLower(fs.etag), `"`), nil
}

// Size returns the size of an object in bytes
func (fs *FsObjectS3) Size() int64 {
	return fs.bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (fs *FsObjectS3) readMetaData() (err error) {
	if fs.meta != nil {
		return nil
	}

	headers, err := fs.s3.b.Head(fs.remote, nil)
	if err != nil {
		FsDebug(fs, "Failed to read info: %s", err)
		return err
	}
	size, err := strconv.ParseInt(headers["Content-Length"], 10, 64)
	if err != nil {
		FsDebug(fs, "Failed to read size from: %q", headers)
		return err
	}
	fs.etag = headers["Etag"]
	fs.bytes = size
	fs.meta = headers
	if fs.lastModified, err = time.Parse(http.TimeFormat, headers["Last-Modified"]); err != nil {
		FsLog(fs, "Failed to read last modified from HEAD: %s", err)
		fs.lastModified = time.Now()
	}
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (fs *FsObjectS3) ModTime() time.Time {
	err := fs.readMetaData()
	if err != nil {
		FsLog(fs, "Failed to read metadata: %s", err)
		return time.Now()
	}
	// read mtime out of metadata if available
	d, ok := fs.meta[metaMtime]
	if !ok {
		// FsDebug(fs, "No metadata")
		return fs.lastModified
	}
	modTime, err := swift.FloatStringToTime(d)
	if err != nil {
		FsLog(fs, "Failed to read mtime from object: %s", err)
		return fs.lastModified
	}
	return modTime
}

// Sets the modification time of the local fs object
func (fs *FsObjectS3) SetModTime(modTime time.Time) {
	err := fs.readMetaData()
	if err != nil {
		stats.Error()
		FsLog(fs, "Failed to read metadata: %s", err)
		return
	}
	fs.meta[metaMtime] = swift.TimeToFloatString(modTime)
	_, err = fs.s3.b.Update(fs.remote, fs.s3.perm, fs.meta)
	if err != nil {
		stats.Error()
		FsLog(fs, "Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (fs *FsObjectS3) Storable() bool {
	return true
}

// Open an object for read
func (fs *FsObjectS3) Open() (in io.ReadCloser, err error) {
	in, err = fs.s3.b.GetReader(fs.remote)
	return
}

// Remove an object
func (fs *FsObjectS3) Remove() error {
	return fs.s3.b.Del(fs.remote)
}

// Check the interfaces are satisfied
var _ Fs = &FsS3{}
var _ FsObject = &FsObjectS3{}
