// Package s3 implements a fake s3 server for rclone
package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/Mikubill/gofakes3"
	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

var (
	emptyPrefix    = &gofakes3.Prefix{}
	timeFormat     = "Mon, 2 Jan 2006 15:04:05.999999999 GMT"
	tmpMetaStorage = new(sync.Map)
)

type s3Backend struct {
	opt  *Options
	lock sync.Mutex
	fs   *vfs.VFS
}

// newBackend creates a new SimpleBucketBackend.
func newBackend(fs *vfs.VFS, opt *Options) gofakes3.Backend {
	return &s3Backend{
		fs:  fs,
		opt: opt,
	}
}

// ListBuckets always returns the default bucket.
func (db *s3Backend) ListBuckets() ([]gofakes3.BucketInfo, error) {
	dirEntries, err := getDirEntries("/", db.fs)
	if err != nil {
		return nil, err
	}
	var response []gofakes3.BucketInfo
	for _, entry := range dirEntries {
		if entry.IsDir() {
			response = append(response, gofakes3.BucketInfo{
				Name:         gofakes3.URLEncode(entry.Name()),
				CreationDate: gofakes3.NewContentTime(entry.ModTime()),
			})
		}
		// todo: handle files in root dir
	}

	return response, nil
}

// ListBucket lists the objects in the given bucket.
func (db *s3Backend) ListBucket(bucket string, prefix *gofakes3.Prefix, page gofakes3.ListBucketPage) (*gofakes3.ObjectList, error) {

	_, err := db.fs.Stat(bucket)
	if err != nil {
		return nil, gofakes3.BucketNotFound(bucket)
	}
	if prefix == nil {
		prefix = emptyPrefix
	}

	db.lock.Lock()
	defer db.lock.Unlock()

	// workaround
	if strings.TrimSpace(prefix.Prefix) == "" {
		prefix.HasPrefix = false
	}
	if strings.TrimSpace(prefix.Delimiter) == "" {
		prefix.HasDelimiter = false
	}

	response := gofakes3.NewObjectList()
	if db.fs.Fs().Features().BucketBased || prefix.HasDelimiter && prefix.Delimiter != "/" {
		err = db.getObjectsListArbitrary(bucket, prefix, response)
	} else {
		path, remaining := prefixParser(prefix)
		err = db.entryListR(bucket, path, remaining, prefix.HasDelimiter, response)
	}

	if err != nil {
		return nil, err
	}

	return db.pager(response, page)
}

// HeadObject returns the fileinfo for the given object name.
//
// Note that the metadata is not supported yet.
func (db *s3Backend) HeadObject(bucketName, objectName string) (*gofakes3.Object, error) {

	_, err := db.fs.Stat(bucketName)
	if err != nil {
		return nil, gofakes3.BucketNotFound(bucketName)
	}

	db.lock.Lock()
	defer db.lock.Unlock()

	fp := path.Join(bucketName, objectName)
	node, err := db.fs.Stat(fp)
	if err != nil {
		return nil, gofakes3.KeyNotFound(objectName)
	}

	if !node.IsFile() {
		return nil, gofakes3.KeyNotFound(objectName)
	}

	entry := node.DirEntry()
	if entry == nil {
		return nil, gofakes3.KeyNotFound(objectName)
	}

	fobj := entry.(fs.Object)
	size := node.Size()
	hash := getFileHashByte(fobj)

	meta := map[string]string{
		"Last-Modified": node.ModTime().Format(timeFormat),
		"Content-Type":  fs.MimeType(context.Background(), fobj),
	}

	if val, ok := tmpMetaStorage.Load(fp); ok {
		metaMap := val.(map[string]string)
		for k, v := range metaMap {
			meta[k] = v
		}
	}

	return &gofakes3.Object{
		Name:     objectName,
		Hash:     hash,
		Metadata: meta,
		Size:     size,
		Contents: noOpReadCloser{},
	}, nil
}

// GetObject fetchs the object from the filesystem.
func (db *s3Backend) GetObject(bucketName, objectName string, rangeRequest *gofakes3.ObjectRangeRequest) (obj *gofakes3.Object, err error) {

	_, err = db.fs.Stat(bucketName)
	if err != nil {
		return nil, gofakes3.BucketNotFound(bucketName)
	}

	db.lock.Lock()
	defer db.lock.Unlock()

	fp := path.Join(bucketName, objectName)
	node, err := db.fs.Stat(fp)
	if err != nil {
		return nil, gofakes3.KeyNotFound(objectName)
	}

	if !node.IsFile() {
		return nil, gofakes3.KeyNotFound(objectName)
	}

	entry := node.DirEntry()
	if entry == nil {
		return nil, gofakes3.KeyNotFound(objectName)
	}

	fobj := entry.(fs.Object)
	file := node.(*vfs.File)

	size := node.Size()
	hash := getFileHashByte(fobj)

	in, err := file.Open(os.O_RDONLY)
	if err != nil {
		return nil, gofakes3.ErrInternal
	}
	defer func() {
		// If an error occurs, the caller may not have access to Object.Body in order to close it:
		if err != nil {
			_ = in.Close()
		}
	}()

	var rdr io.ReadCloser = in
	rnge, err := rangeRequest.Range(size)
	if err != nil {
		return nil, err
	}

	if rnge != nil {
		if _, err := in.Seek(rnge.Start, io.SeekStart); err != nil {
			return nil, err
		}
		rdr = limitReadCloser(rdr, in.Close, rnge.Length)
	}

	meta := map[string]string{
		"Last-Modified": node.ModTime().Format(timeFormat),
		"Content-Type":  fs.MimeType(context.Background(), fobj),
	}

	if val, ok := tmpMetaStorage.Load(fp); ok {
		metaMap := val.(map[string]string)
		for k, v := range metaMap {
			meta[k] = v
		}
	}

	return &gofakes3.Object{
		Name:     gofakes3.URLEncode(objectName),
		Hash:     hash,
		Metadata: meta,
		Size:     size,
		Range:    rnge,
		Contents: rdr,
	}, nil
}

// TouchObject creates or updates meta on specified object.
func (db *s3Backend) TouchObject(fp string, meta map[string]string) (result gofakes3.PutObjectResult, err error) {

	_, err = db.fs.Stat(fp)
	if err == vfs.ENOENT {
		f, err := db.fs.Create(fp)
		if err != nil {
			return result, err
		}
		_ = f.Close()
		return db.TouchObject(fp, meta)
	} else if err != nil {
		return result, err
	}

	_, err = db.fs.Stat(fp)
	if err != nil {
		return result, err
	}

	tmpMetaStorage.Store(fp, meta)

	if val, ok := meta["X-Amz-Meta-Mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			return result, db.fs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	if val, ok := meta["mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			return result, db.fs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	return result, nil
}

// PutObject creates or overwrites the object with the given name.
func (db *s3Backend) PutObject(
	bucketName, objectName string,
	meta map[string]string,
	input io.Reader, size int64,
) (result gofakes3.PutObjectResult, err error) {

	_, err = db.fs.Stat(bucketName)
	if err != nil {
		return result, gofakes3.BucketNotFound(bucketName)
	}

	db.lock.Lock()
	defer db.lock.Unlock()

	fp := path.Join(bucketName, objectName)
	objectDir := path.Dir(fp)
	// _, err = db.fs.Stat(objectDir)
	// if err == vfs.ENOENT {
	// 	fs.Errorf(objectDir, "PutObject failed: path not found")
	// 	return result, gofakes3.KeyNotFound(objectName)
	// }

	if objectDir != "." {
		if err := mkdirRecursive(objectDir, db.fs); err != nil {
			return result, err
		}
	}

	if size == 0 {
		// maybe a touch operation
		return db.TouchObject(fp, meta)
	}

	f, err := db.fs.Create(fp)
	if err != nil {
		return result, err
	}

	hasher := md5.New()
	w := io.MultiWriter(f, hasher)
	if _, err := io.Copy(w, input); err != nil {
		// remove file when i/o error occurred (FsPutErr)
		_ = f.Close()
		_ = db.fs.Remove(fp)
		return result, err
	}

	if err := f.Close(); err != nil {
		// remove file when close error occurred (FsPutErr)
		_ = db.fs.Remove(fp)
		return result, err
	}

	_, err = db.fs.Stat(fp)
	if err != nil {
		return result, err
	}

	tmpMetaStorage.Store(fp, meta)

	if val, ok := meta["X-Amz-Meta-Mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			return result, db.fs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	if val, ok := meta["mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			return result, db.fs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	return result, nil
}

// DeleteMulti deletes multiple objects in a single request.
func (db *s3Backend) DeleteMulti(bucketName string, objects ...string) (result gofakes3.MultiDeleteResult, rerr error) {
	db.lock.Lock()
	defer db.lock.Unlock()

	for _, object := range objects {
		if err := db.deleteObjectLocked(bucketName, object); err != nil {
			log.Println("delete object failed:", err)
			result.Error = append(result.Error, gofakes3.ErrorResult{
				Code:    gofakes3.ErrInternal,
				Message: gofakes3.ErrInternal.Message(),
				Key:     object,
			})
		} else {
			result.Deleted = append(result.Deleted, gofakes3.ObjectID{
				Key: object,
			})
		}
	}

	return result, nil
}

// DeleteObject deletes the object with the given name.
func (db *s3Backend) DeleteObject(bucketName, objectName string) (result gofakes3.ObjectDeleteResult, rerr error) {
	db.lock.Lock()
	defer db.lock.Unlock()

	return result, db.deleteObjectLocked(bucketName, objectName)
}

// deleteObjectLocked deletes the object from the filesystem.
func (db *s3Backend) deleteObjectLocked(bucketName, objectName string) error {

	_, err := db.fs.Stat(bucketName)
	if err != nil {
		return gofakes3.BucketNotFound(bucketName)
	}

	fp := path.Join(bucketName, objectName)
	// S3 does not report an error when attemping to delete a key that does not exist, so
	// we need to skip IsNotExist errors.
	if err := db.fs.Remove(fp); err != nil && !os.IsNotExist(err) {
		return err
	}

	// fixme: unsafe operation
	if db.fs.Fs().Features().CanHaveEmptyDirectories {
		rmdirRecursive(fp, db.fs)
	}
	return nil
}

// CreateBucket creates a new bucket.
func (db *s3Backend) CreateBucket(name string) error {
	_, err := db.fs.Stat(name)
	if err != nil && err != vfs.ENOENT {
		return gofakes3.ErrInternal
	}

	if err == nil {
		return gofakes3.ErrBucketAlreadyExists
	}

	if err := db.fs.Mkdir(name, 0755); err != nil {
		return gofakes3.ErrInternal
	}
	return nil
}

// DeleteBucket deletes the bucket with the given name.
func (db *s3Backend) DeleteBucket(name string) error {
	_, err := db.fs.Stat(name)
	if err != nil {
		return gofakes3.BucketNotFound(name)
	}

	if err := db.fs.Remove(name); err != nil {
		return gofakes3.ErrBucketNotEmpty
	}

	return nil
}

// BucketExists checks if the bucket exists.
func (db *s3Backend) BucketExists(name string) (exists bool, err error) {
	_, err = db.fs.Stat(name)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// CopyObject copy specified object from srcKey to dstKey.
func (db *s3Backend) CopyObject(srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string) (result gofakes3.CopyObjectResult, err error) {

	fp := path.Join(srcBucket, srcKey)
	if srcBucket == dstBucket && srcKey == dstKey {
		tmpMetaStorage.Store(fp, meta)

		val, ok := meta["X-Amz-Meta-Mtime"]
		if !ok {
			if val, ok = meta["mtime"]; !ok {
				return
			}
		}
		// update modtime
		ti, err := swift.FloatStringToTime(val)
		if err != nil {
			return result, nil
		}

		return result, db.fs.Chtimes(fp, ti, ti)
	}

	cStat, err := db.fs.Stat(fp)
	if err != nil {
		return
	}

	c, err := db.GetObject(srcBucket, srcKey, nil)
	if err != nil {
		return
	}
	defer func() {
		_ = c.Contents.Close()
	}()

	for k, v := range c.Metadata {
		if _, found := meta[k]; !found && k != "X-Amz-Acl" {
			meta[k] = v
		}
	}
	if _, ok := meta["mtime"]; !ok {
		meta["mtime"] = swift.TimeToFloatString(cStat.ModTime())
	}

	_, err = db.PutObject(dstBucket, dstKey, meta, c.Contents, c.Size)
	if err != nil {
		return
	}

	return gofakes3.CopyObjectResult{
		ETag:         `"` + hex.EncodeToString(c.Hash) + `"`,
		LastModified: gofakes3.NewContentTime(cStat.ModTime()),
	}, nil
}
