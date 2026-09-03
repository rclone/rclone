package s3

import (
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
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
	b         *s3Backend
	vfs       *vfs.VFS
	bucket    string
	marker    string // keys at or before this one have already been returned
	hasMarker bool
	max       int // number of keys wanted in this page
	response  *gofakes3.ObjectList
	keys      int    // keys added to the page so far
	lastKey   string // the last key added to the page
	dirsRead  int    // directories read, for debug logging
}

// newLister makes a lister which fills response with at most one page of the
// listing of bucket, as described by page.
func newLister(b *s3Backend, _vfs *vfs.VFS, bucket string, page gofakes3.ListBucketPage, response *gofakes3.ObjectList) *lister {
	max := int(page.MaxKeys)
	if max <= 0 {
		// A missing or zero max-keys means the client didn't ask for a
		// particular page size, so use the S3 maximum.
		max = 1000
	}
	return &lister{
		b:         b,
		vfs:       _vfs,
		bucket:    bucket,
		marker:    page.Marker,
		hasMarker: page.HasMarker,
		max:       max,
		response:  response,
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
	l.response.Add(&gofakes3.Content{
		Key:          key,
		LastModified: gofakes3.NewContentTime(entry.ModTime()),
		ETag:         getFileHash(entry, l.b.s.etagHashType),
		Size:         entry.Size(),
		StorageClass: gofakes3.StorageStandard,
	})
	l.keys++
	l.lastKey = key
	return nil
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

	// Emit the entries in the order their keys have in a flat keyspace,
	// which is not the plain name order getDirEntries returns.
	slices.SortFunc(dirEntries, func(a, b vfs.Node) int {
		return strings.Compare(sortKey(a), sortKey(b))
	})

	for _, entry := range dirEntries {
		object := entry.Name()

		// Hide the temporary objects of in-progress uploads
		if strings.HasPrefix(object, tempObjectPrefix) || strings.HasPrefix(object, legacyMultipartUploadPrefix) {
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
			err := l.list(objectPath, "", false)
			if errors.Is(err, gofakes3.ErrNoSuchKey) {
				// The directory went away while we were listing it, so
				// there is nothing below it to report.
				continue
			} else if err != nil {
				return err
			}
		} else {
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

// listPage fills response with one page of the listing of the directory
// fdPath in bucket, as described by page.
//
// Only entries whose leaf name starts with name are listed. If addPrefix is
// set, subdirectories are reported as common prefixes, otherwise the whole
// subtree below fdPath is listed.
//
// It returns gofakes3.ErrNoSuchKey if fdPath is not a directory.
func (b *s3Backend) listPage(_vfs *vfs.VFS, bucket, fdPath, name string, addPrefix bool, page gofakes3.ListBucketPage, response *gofakes3.ObjectList) error {
	l := newLister(b, _vfs, bucket, page, response)
	err := l.list(fdPath, name, addPrefix)
	if err != nil && !errors.Is(err, errPageFull) {
		return err
	}
	fs.Debugf("serve s3", "Listed %d keys from %d directories (truncated=%v)", l.keys, l.dirsRead, response.IsTruncated)
	return nil
}
