// Package s3 implements an s3 server for rclone
package s3

import (
	"context"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/ncw/swift/v2"
	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

var (
	emptyPrefix = &gofakes3.Prefix{}
	timeFormat  = "Mon, 2 Jan 2006 15:04:05 GMT"
)

// s3Backend implements the gofacess3.Backend interface to make an S3
// backend for gofakes3
type s3Backend struct {
	opt  *Options
	s    *Server
	meta *sync.Map
}

// newBackend creates a new SimpleBucketBackend.
func newBackend(s *Server, opt *Options) gofakes3.Backend {
	return &s3Backend{
		opt:  opt,
		s:    s,
		meta: new(sync.Map),
	}
}

// ListBuckets always returns the default bucket.
func (b *s3Backend) ListBuckets(ctx context.Context) ([]gofakes3.BucketInfo, error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return nil, err
	}
	dirEntries, err := getDirEntries("/", _vfs)
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
		// FIXME: handle files in root dir
	}

	return response, nil
}

// ListBucket lists the objects in the given bucket.
func (b *s3Backend) ListBucket(ctx context.Context, bucket string, prefix *gofakes3.Prefix, page gofakes3.ListBucketPage) (*gofakes3.ObjectList, error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return nil, err
	}
	_, err = _vfs.Stat(bucket)
	if err != nil {
		return nil, gofakes3.BucketNotFound(bucket)
	}
	if prefix == nil {
		prefix = emptyPrefix
	}

	// workaround
	if strings.TrimSpace(prefix.Prefix) == "" {
		prefix.HasPrefix = false
	}
	if strings.TrimSpace(prefix.Delimiter) == "" {
		prefix.HasDelimiter = false
	}

	response := gofakes3.NewObjectList()
	path, remaining := prefixParser(prefix)

	err = b.entryListR(_vfs, bucket, path, remaining, prefix.HasDelimiter, response)
	if err == gofakes3.ErrNoSuchKey {
		// AWS just returns an empty list
		response = gofakes3.NewObjectList()
	} else if err != nil {
		return nil, err
	}

	return b.pager(response, page)
}

// HeadObject returns the fileinfo for the given object name.
//
// Note that the metadata is not supported yet.
func (b *s3Backend) HeadObject(ctx context.Context, bucketName, objectName string) (*gofakes3.Object, error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return nil, err
	}
	_, err = _vfs.Stat(bucketName)
	if err != nil {
		return nil, gofakes3.BucketNotFound(bucketName)
	}

	fp := path.Join(bucketName, objectName)
	node, err := _vfs.Stat(fp)
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

	if val, ok := b.meta.Load(fp); ok {
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
func (b *s3Backend) GetObject(ctx context.Context, bucketName, objectName string, rangeRequest *gofakes3.ObjectRangeRequest) (obj *gofakes3.Object, err error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return nil, err
	}
	_, err = _vfs.Stat(bucketName)
	if err != nil {
		return nil, gofakes3.BucketNotFound(bucketName)
	}

	fp := path.Join(bucketName, objectName)
	node, err := _vfs.Stat(fp)
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

	if val, ok := b.meta.Load(fp); ok {
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

// storeModtime sets both "mtime" and "X-Amz-Meta-Mtime" to val in b.meta.
// Call this whenever modtime is updated.
func (b *s3Backend) storeModtime(fp string, meta map[string]string, val string) {
	meta["X-Amz-Meta-Mtime"] = val
	meta["mtime"] = val
	b.meta.Store(fp, meta)
}

// TouchObject creates or updates meta on specified object.
func (b *s3Backend) TouchObject(ctx context.Context, fp string, meta map[string]string) (result gofakes3.PutObjectResult, err error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return result, err
	}
	_, err = _vfs.Stat(fp)
	if err == vfs.ENOENT {
		f, err := _vfs.Create(fp)
		if err != nil {
			return result, err
		}
		_ = f.Close()
		return b.TouchObject(ctx, fp, meta)
	} else if err != nil {
		return result, err
	}

	_, err = _vfs.Stat(fp)
	if err != nil {
		return result, err
	}

	b.meta.Store(fp, meta)

	if val, ok := meta["X-Amz-Meta-Mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			b.storeModtime(fp, meta, val)
			return result, _vfs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	if val, ok := meta["mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			b.storeModtime(fp, meta, val)
			return result, _vfs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	return result, nil
}

// PutObject creates or overwrites the object with the given name.
func (b *s3Backend) PutObject(
	ctx context.Context,
	bucketName, objectName string,
	meta map[string]string,
	input io.Reader, size int64,
) (result gofakes3.PutObjectResult, err error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return result, err
	}
	_, err = _vfs.Stat(bucketName)
	if err != nil {
		return result, gofakes3.BucketNotFound(bucketName)
	}

	fp := path.Join(bucketName, objectName)
	objectDir := path.Dir(fp)
	// _, err = db.fs.Stat(objectDir)
	// if err == vfs.ENOENT {
	// 	fs.Errorf(objectDir, "PutObject failed: path not found")
	// 	return result, gofakes3.KeyNotFound(objectName)
	// }

	if objectDir != "." {
		if err := mkdirRecursive(objectDir, _vfs); err != nil {
			return result, err
		}
	}

	f, err := _vfs.Create(fp)
	if err != nil {
		return result, err
	}

	if _, err := io.Copy(f, input); err != nil {
		// remove file when i/o error occurred (FsPutErr)
		_ = f.Close()
		_ = _vfs.Remove(fp)
		return result, err
	}

	if err := f.Close(); err != nil {
		// remove file when close error occurred (FsPutErr)
		_ = _vfs.Remove(fp)
		return result, err
	}

	_, err = _vfs.Stat(fp)
	if err != nil {
		return result, err
	}

	b.meta.Store(fp, meta)

	if val, ok := meta["X-Amz-Meta-Mtime"]; ok {
		ti, err := swift.FloatStringToTime(val)
		if err == nil {
			b.storeModtime(fp, meta, val)
			return result, _vfs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created

		if val, ok := meta["mtime"]; ok {
			b.storeModtime(fp, meta, val)
			return result, _vfs.Chtimes(fp, ti, ti)
		}
		// ignore error since the file is successfully created
	}

	return result, nil
}

// DeleteMulti deletes multiple objects in a single request.
func (b *s3Backend) DeleteMulti(ctx context.Context, bucketName string, objects ...string) (result gofakes3.MultiDeleteResult, rerr error) {
	for _, object := range objects {
		if err := b.deleteObject(ctx, bucketName, object); err != nil {
			fs.Errorf("serve s3", "delete object failed: %v", err)
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
func (b *s3Backend) DeleteObject(ctx context.Context, bucketName, objectName string) (result gofakes3.ObjectDeleteResult, rerr error) {
	return result, b.deleteObject(ctx, bucketName, objectName)
}

// deleteObject deletes the object from the filesystem.
func (b *s3Backend) deleteObject(ctx context.Context, bucketName, objectName string) error {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return err
	}
	_, err = _vfs.Stat(bucketName)
	if err != nil {
		return gofakes3.BucketNotFound(bucketName)
	}

	fp := path.Join(bucketName, objectName)
	// S3 does not report an error when attemping to delete a key that does not exist, so
	// we need to skip IsNotExist errors.
	if err := _vfs.Remove(fp); err != nil && !os.IsNotExist(err) {
		return err
	}

	// FIXME: unsafe operation
	rmdirRecursive(fp, _vfs)
	return nil
}

// CreateBucket creates a new bucket.
func (b *s3Backend) CreateBucket(ctx context.Context, name string) error {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return err
	}
	_, err = _vfs.Stat(name)
	if err != nil && err != vfs.ENOENT {
		return gofakes3.ErrInternal
	}

	if err == nil {
		return gofakes3.ErrBucketAlreadyExists
	}

	if err := _vfs.Mkdir(name, 0755); err != nil {
		return gofakes3.ErrInternal
	}
	return nil
}

// DeleteBucket deletes the bucket with the given name.
func (b *s3Backend) DeleteBucket(ctx context.Context, name string) error {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return err
	}
	_, err = _vfs.Stat(name)
	if err != nil {
		return gofakes3.BucketNotFound(name)
	}

	if err := _vfs.Remove(name); err != nil {
		return gofakes3.ErrBucketNotEmpty
	}

	return nil
}

// BucketExists checks if the bucket exists.
func (b *s3Backend) BucketExists(ctx context.Context, name string) (exists bool, err error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return false, err
	}
	_, err = _vfs.Stat(name)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// CopyObject copy specified object from srcKey to dstKey.
func (b *s3Backend) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string) (result gofakes3.CopyObjectResult, err error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return result, err
	}
	fp := path.Join(srcBucket, srcKey)
	if srcBucket == dstBucket && srcKey == dstKey {
		b.meta.Store(fp, meta)

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
		b.storeModtime(fp, meta, val)

		return result, _vfs.Chtimes(fp, ti, ti)
	}

	cStat, err := _vfs.Stat(fp)
	if err != nil {
		return
	}

	c, err := b.GetObject(ctx, srcBucket, srcKey, nil)
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

	_, err = b.PutObject(ctx, dstBucket, dstKey, meta, c.Contents, c.Size)
	if err != nil {
		return
	}

	return gofakes3.CopyObjectResult{
		ETag:         `"` + hex.EncodeToString(c.Hash) + `"`,
		LastModified: gofakes3.NewContentTime(cStat.ModTime()),
	}, nil
}
