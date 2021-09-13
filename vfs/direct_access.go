package vfs

import (
	"context"
	"path"
	"time"
	"github.com/rclone/rclone/backend/s3"
	"github.com/rclone/rclone/fs"
)



// Direct Access is an optimization of VFS where a file /dir1/dir2/dir3/file can be accessed
// without populating dir1, dir2 and dir3 children items.
// It can speed things up in deep hirearchies and/or for large directories
// cf https://github.com/rclone/rclone/issues/5553

// import (
// 	"context"
// 	"fmt"
// 	"os"
// 	"path"
// 	"sort"
// 	"strings"
// 	"sync"
// 	"sync/atomic"
// 	"time"

// 	"github.com/pkg/errors"
// 	"github.com/rclone/rclone/backend/s3"
// 	"github.com/rclone/rclone/fs"
// 	"github.com/rclone/rclone/fs/dirtree"
// 	"github.com/rclone/rclone/fs/list"
// 	"github.com/rclone/rclone/fs/log"
// 	"github.com/rclone/rclone/fs/operations"
// 	"github.com/rclone/rclone/fs/walk"
// 	"github.com/rclone/rclone/vfs/vfscommon"
// )

// DirectAccessNodeItem represents a directory entry (file or dir) that has been directl accessed
type DirectAccessNodeItem struct {
	node     *Node     // the file or dir accessed
	accessed time.Time // time directory entry last read
}

// manage the dictionary of directly accessed items
type DirectAccessManager struct {
	items map[string]DirectAccessNodeItem // managed items
}

// =================== DirectAccessNodeItem methods ==============================

func newDirectAccessNodeItem(node *Node, accessed time.Time) *DirectAccessNodeItem {
	return &DirectAccessNodeItem{
		node:     node,
		accessed: accessed,
	}
}

func (item *DirectAccessNodeItem) age(when time.Time) time.Duration {
	return when.Sub(item.accessed)
}

func (item *DirectAccessNodeItem) valid(when time.Time, cache_time time.Duration) bool {
	return item.age(when) <= cache_time
}

// // String converts it to printable
// func (item *DirectAccessNodeItem) String() string {
// 	if item == nil {
// 		return "<nil *DirectAccessNodeItem>"
// 	}
// 	return *item.node.String()
// }

// =================== DirectAccessManager methods ==============================
func newDirectAccessManager() *DirectAccessManager {
	return &DirectAccessManager{
		items: make(map[string]DirectAccessNodeItem),
	}
}

func (dam *DirectAccessManager) String() string {
	if dam == nil {
		return "<nil *DirectAccessManager>"
	}

	return "DirectAccessManager: " + string(len(dam.items))
}

func (dam *DirectAccessManager) clear() {
	dam.items = make(map[string]DirectAccessNodeItem)
}

func (dam *DirectAccessManager) fetch(filename string, when time.Time, cache_time time.Duration) *DirectAccessNodeItem {
	item, ok := dam.items[filename]
	if ok && item.valid(when, cache_time) {
		return &item
	}

	return nil
}

// =================== Dir extra methods ==============================

// currently only applies to S3 and if the option is set
func (d *Dir) isDirectAccessHeuristicOn() bool {
	fs := asS3(d.f)
	if fs != nil && fs.Options().UseLazyDirListHack {
		return true
	}

	return false
}

// fetch if the file is among the valid directly accessed items
// N.B: do not modify d nor d.accessed
func (d *Dir) directAccessLookupInCache(filename string, when time.Time) *DirectAccessNodeItem {
	item := d.accessed.fetch(filename, when, d.vfs.Opt.DirCacheTime)
	if item != nil { // found it!
		return item
	}

	return nil
}

func (d *Dir) directAccessLookup(filename string) (*DirectAccessNodeItem, error) {
	when := time.Now()
	item := d.accessed.fetch(filename, when, d.vfs.Opt.DirCacheTime)
	if item != nil { // found it in cache !!
		fs.Debugf(d.path, "vfs::direct_access::directAccessLookup filename=%v -->  found in cache", filename)
		return item, nil
	}

	// now must look for it
	node, err := createNodeForFile(d, filename)
	if err != nil { // N.B: if not found, err is ENOENT
		return nil, err
	}

	item = newDirectAccessNodeItem(&node, when)

	// store it
	d.accessed.items[filename] = *item

	return item, nil 
}

// look in remote storage if the file or dir exists.
// if it does, return the corresponding node/fs object
func createNodeForFile(d *Dir, filename string) (Node, error) {
	fullPath := path.Join(d.path, filename)

	fs.Debugf("--------------------------------------------------", "")
	fs.Debugf("vfs::direct_access:createNodeForFile", fullPath)

	// if len(d.path) == 0 { // not sure, probably filename is a bucket them
	// 	return nil, ENOENT
	// }

	fs.Debugf("vfs::direct_access:createNodeForFile", "calling NewObject(%s)", fullPath)
	if len(d.path) > 0 { // otherwise it can not be a regular file (could be a bucket)
		node, err := d.f.NewObject(context.TODO(), fullPath)

		if node != nil { // found remote object, it is a File !
			fs.Debugf("vfs::direct_access:createNodeForFile", "---> found file %v!!", fullPath)
			return newFile(d, d.path, node, filename), nil
		}

		if err != nil && err.Error() != "object not found" { // unknown error
			fs.Debugf("vfs::direct_access:createNodeForFile", "NewObject FAILED for %v: %v", fullPath, err)
			return nil, err
		}
	}

	fs.Debugf("vfs::direct_access:createNodeForFile", "calling IsDirectory(%v)", fullPath)
	ok, err := IsDirectory(d.f, context.TODO(), fullPath)
	if err != nil {
		return nil, err
	}

	if ok {
		fs.Debugf("vfs::direct_access:createNodeForFile", "--> creating new Dir entry for %v", fullPath)
		entry := fs.NewDir(fullPath, time.Now())
		dir := newDir(d.vfs, d.f, d, entry)
		return dir, nil
	}

	fs.Debugf("vfs::direct_access:createNodeForFile", "--> DID NOT FIND FILE %v", fullPath)
	return nil, ENOENT
}

//  =================== vfs::dir extra methods ==============================

// return a s3.FS object if S3, nil otherwise
func asS3(f fs.Fs) *s3.Fs {
	s3fs, ok := f.(*s3.Fs)
	if ok {
		return s3fs
	}
	return nil
}

// extension of the
func IsDirectory(f fs.Fs, ctx context.Context, path string) (inDir bool, err error) {
	s3fs := asS3(f)
	if s3fs == nil {
		return false, ENOSYS
	}

	return s3fs.IsDirectoryS3(ctx, path)
}

//  =================== backend::s3 extra methods ==============================
// does not support buckets for now
