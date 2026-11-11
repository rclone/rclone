package s3

import (
	"context"
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/vfs"
)

// legacyMultipartUploadPrefix marked the temporary objects of in-progress
// multipart uploads before the tempObjectPrefix namespace was reserved
// (rclone v1.75); leftovers from an older server are still hidden.
const legacyMultipartUploadPrefix = ".rclone_multipart_upload_"

// errPageFull is returned by the listing walk once the page has been filled
// and one further key has been seen, so the walk can unwind early. It never
// escapes ListBucket.
var errPageFull = errors.New("listing page full")

// lister walks a directory tree emitting one page of an S3 listing.
//
// The walk emits keys in flat S3 order and stops as soon as the page is full,
// so the cost of a page is proportional to the keys it returns rather than to
// the size of the subtree below the prefix.
type lister struct {
	vfs       *vfs.VFS
	t         *tenant                                            // the user listing
	hashType  hash.Type                                          // hash used for ETags
	etag      func(fp string, entry vfs.Node) []byte             // returns the ETag of the object at fp
	meta      func(fp string, entry vfs.Node) (objectMeta, bool) // returns the stored metadata of the object at fp
	bucket    string
	marker    string // keys at or before this one have already been returned
	hasMarker bool
	max       int // number of keys wanted in this page
	response  *gofakes3.ObjectList
	keys      int    // keys added to the page so far
	lastKey   string // the last key added to the page
	dirsRead  int    // directories read, for debug logging

	// With a delimiter other than "/", which can't be found by
	// listing directories, the whole tree is walked and the keys are
	// rolled up into common prefixes.
	prefix     string // the prefix of the listing
	delimiter  string // the delimiter, or "" if none or "/"
	lastPrefix string // the last common prefix added to the page
}

// newLister makes a lister which fills response with at most one page of the
// listing of bucket, as described by page.
func newLister(ctx context.Context, b *s3Backend, _vfs *vfs.VFS, bucket string, page gofakes3.ListBucketPage, response *gofakes3.ObjectList) *lister {
	max := int(page.MaxKeys)
	if max <= 0 {
		// A missing or zero max-keys means the client didn't ask for a
		// particular page size, so use the S3 maximum.
		max = 1000
	}
	t := b.tenant(ctx)
	return &lister{
		vfs:       _vfs,
		t:         t,
		hashType:  b.s.etagHash(_vfs),
		bucket:    bucket,
		marker:    page.Marker,
		hasMarker: page.HasMarker,
		max:       max,
		response:  response,
		etag: func(fp string, entry vfs.Node) []byte {
			return b.objectHash(ctx, _vfs, fp, entry)
		},
		meta: func(fp string, entry vfs.Node) (objectMeta, bool) {
			return t.getMeta(ctx, fp, entry)
		},
	}
}

// truncate marks the page as truncated after the last key added to it, so
// that the next page resumes from there, and returns errPageFull.
func (l *lister) truncate() error {
	l.response.IsTruncated = true
	l.response.NextMarker = l.lastKey
	return errPageFull
}

// sortKey returns the S3 key an entry contributes to the listing.
//
// Directories sort as if they carried their trailing slash, which is what
// makes a depth first walk emit keys in the same order as the flat keyspace
// of a real S3 bucket.
func sortKey(entry vfs.Node) string {
	if entry.IsDir() {
		return entry.Name() + "/"
	}
	return entry.Name()
}

// skipKey reports whether key has already been returned by an earlier page.
func (l *lister) skipKey(key string) bool {
	return l.hasMarker && key <= l.marker
}

// skipDir reports whether every key under the directory prefix p was returned
// by an earlier page, so the subtree need not be read at all.
//
// A marker inside the subtree has p as a prefix and is never skipped: that
// subtree is descended and filtered key by key.
func (l *lister) skipDir(p string) bool {
	return l.hasMarker && l.marker > p && !strings.HasPrefix(l.marker, p)
}

// addCommonPrefix adds a common prefix to the page, returning errPageFull if
// the page is now complete.
func (l *lister) addCommonPrefix(prefix string) error {
	if l.keys >= l.max {
		return l.truncate()
	}
	l.response.AddPrefix(prefix)
	l.keys++
	l.lastKey = prefix
	return nil
}

// addObject adds an object to the page, returning errPageFull if the page is
// now complete.
func (l *lister) addObject(key string, entry vfs.Node) error {
	if l.keys >= l.max {
		return l.truncate()
	}
	fp := path.Join(l.bucket, key)
	storageClass := gofakes3.StorageStandard
	if om, ok := l.meta(fp, entry); ok && om.meta["X-Amz-Storage-Class"] != "" {
		storageClass = gofakes3.StorageClass(om.meta["X-Amz-Storage-Class"])
	}
	l.response.Add(&gofakes3.Content{
		Key:          key,
		LastModified: gofakes3.NewContentTime(entry.ModTime()),
		ETag:         quoteETag(l.etag(fp, entry)),
		Size:         entry.Size(),
		StorageClass: storageClass,
	})
	l.keys++
	l.lastKey = key
	return nil
}

// addDirMarker adds the directory marker key of the directory fp to
// the page, returning errPageFull if the page is now complete.
func (l *lister) addDirMarker(key, fp string) error {
	if l.skipKey(key) {
		return nil
	}
	node, err := l.vfs.Stat(fp)
	if err != nil {
		// The directory went away while we were listing it
		return nil
	}
	if l.keys >= l.max {
		return l.truncate()
	}
	l.response.Add(&gofakes3.Content{
		Key:          key,
		LastModified: gofakes3.NewContentTime(node.ModTime()),
		ETag:         quoteETag(emptyHash(l.hashType)),
		Size:         0,
		StorageClass: gofakes3.StorageStandard,
	})
	l.keys++
	l.lastKey = key
	return nil
}

// commonPrefix returns the common prefix key rolls up into, if the
// listing has a delimiter other than "/" and key has it after the
// listing's prefix.
func (l *lister) commonPrefix(key string) (prefix string, ok bool) {
	if l.delimiter == "" || !strings.HasPrefix(key, l.prefix) {
		return "", false
	}
	i := strings.Index(key[len(l.prefix):], l.delimiter)
	if i < 0 {
		return "", false
	}
	return key[:len(l.prefix)+i+len(l.delimiter)], true
}

// addRolledUp adds the common prefix a key rolls up into to the page
// unless it has been added already, returning errPageFull if the page
// is now complete.
//
// The keys rolling up into a common prefix are contiguous in the
// listing, so it only needs comparing with the last one.
func (l *lister) addRolledUp(prefix string) error {
	if prefix == l.lastPrefix || l.skipKey(prefix) {
		return nil
	}
	if err := l.addCommonPrefix(prefix); err != nil {
		return err
	}
	l.lastPrefix = prefix
	return nil
}

// hasKeys reports whether the directory fdPath in the bucket has any
// keys below it.
func (l *lister) hasKeys(fdPath string) bool {
	fp, err := bucketDirPath(l.bucket, fdPath)
	if err != nil {
		return false
	}
	dirEntries, err := getDirEntries(fp, l.vfs)
	if err != nil {
		return false
	}
	if len(dirEntries) == 0 || l.t.hasDirMarker(fp) {
		// The directory has its directory marker key
		return true
	}
	for _, entry := range dirEntries {
		if isTempObject(entry.Name()) {
			continue
		}
		if !entry.IsDir() || l.hasKeys(path.Join(fdPath, entry.Name())) {
			return true
		}
	}
	return false
}

// isTempObject reports whether name is a temporary object of an
// in-progress upload, which is hidden from listings.
func isTempObject(name string) bool {
	return strings.HasPrefix(name, tempObjectPrefix) || strings.HasPrefix(name, legacyMultipartUploadPrefix)
}

// list walks the directory fdPath, adding the entries whose leaf name starts
// with name to the page.
//
// If addPrefix is set, subdirectories are reported as common prefixes,
// otherwise they are descended into.
//
// It returns errPageFull once the page is complete, and gofakes3.ErrNoSuchKey
// if fdPath is not a directory.
func (l *lister) list(fdPath, name string, addPrefix bool) error {
	fp, err := bucketDirPath(l.bucket, fdPath)
	if err != nil {
		// A listing prefix that can't be represented as a path matches nothing.
		return gofakes3.ErrNoSuchKey
	}

	dirEntries, err := getDirEntries(fp, l.vfs)
	if err != nil {
		return err
	}
	l.dirsRead++

	// A directory is listed as its directory marker, which sorts
	// before the keys below it, if it is empty, as S3 would list the
	// marker which made it, or if a marker was put for it. Other
	// directories may have been made implicitly, so aren't.
	if name == "" && fdPath != "" && (len(dirEntries) == 0 || l.t.hasDirMarker(fp)) {
		if err := l.addDirMarker(fdPath+"/", fp); err != nil || len(dirEntries) == 0 {
			return err
		}
	}

	// Emit the entries in the order their keys have in a flat keyspace,
	// which is not the plain name order getDirEntries returns.
	slices.SortFunc(dirEntries, func(a, b vfs.Node) int {
		return strings.Compare(sortKey(a), sortKey(b))
	})

	for _, entry := range dirEntries {
		object := entry.Name()

		// Hide the temporary objects of in-progress uploads
		if isTempObject(object) {
			continue
		}

		// workaround for control-chars detect
		objectPath := path.Join(fdPath, object)

		if !strings.HasPrefix(object, name) {
			continue
		}

		if entry.IsDir() {
			prefixWithTrailingSlash := objectPath + "/"
			if addPrefix {
				if l.skipKey(prefixWithTrailingSlash) {
					continue
				}
				if err := l.addCommonPrefix(prefixWithTrailingSlash); err != nil {
					return err
				}
				continue
			}
			if l.skipDir(prefixWithTrailingSlash) {
				continue
			}
			// If the delimiter is in the directory's path every
			// key below it rolls up into the same common prefix,
			// so the directory needn't be read.
			if prefix, ok := l.commonPrefix(prefixWithTrailingSlash); ok {
				// Only look for keys if the prefix isn't on the page
				// already or on an earlier one
				if prefix != l.lastPrefix && !l.skipKey(prefix) && l.hasKeys(objectPath) {
					if err := l.addRolledUp(prefix); err != nil {
						return err
					}
				}
				continue
			}
			err := l.list(objectPath, "", false)
			if errors.Is(err, gofakes3.ErrNoSuchKey) {
				// The directory went away while we were listing it, so
				// there is nothing below it to report.
				continue
			} else if err != nil {
				return err
			}
		} else {
			if prefix, ok := l.commonPrefix(objectPath); ok {
				if err := l.addRolledUp(prefix); err != nil {
					return err
				}
				continue
			}
			if l.skipKey(objectPath) {
				continue
			}
			if err := l.addObject(objectPath, entry); err != nil {
				return err
			}
		}
	}
	return nil
}

// listPage fills response with one page of the listing of bucket with
// prefix and delimiter, as described by page.
//
// It returns gofakes3.ErrNoSuchKey if the directory the prefix is in
// doesn't exist.
func (b *s3Backend) listPage(ctx context.Context, _vfs *vfs.VFS, bucket, prefix, delimiter string, page gofakes3.ListBucketPage, response *gofakes3.ObjectList) error {
	l := newLister(ctx, b, _vfs, bucket, page, response)
	// The prefix is split into the directory to list and the start of
	// the names in it to list.
	fdPath, name := "", prefix
	if i := strings.LastIndexByte(prefix, '/'); i >= 0 {
		fdPath, name = prefix[:i], prefix[i+1:]
	}
	// A "/" delimiter is found by listing directories, any other by
	// walking the whole tree.
	addPrefix := delimiter == "/"
	if delimiter != "" && delimiter != "/" {
		l.prefix, l.delimiter = prefix, delimiter
	}
	err := l.list(fdPath, name, addPrefix)
	if err != nil && !errors.Is(err, errPageFull) {
		return err
	}
	fs.Debugf("serve s3", "Listed %d keys from %d directories (truncated=%v)", l.keys, l.dirsRead, response.IsTruncated)
	return nil
}
