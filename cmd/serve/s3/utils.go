package s3

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/vfs"
)

func getDirEntries(prefix string, VFS *vfs.VFS) (vfs.Nodes, error) {
	node, err := VFS.Stat(prefix)

	if err == vfs.ENOENT {
		return nil, gofakes3.ErrNoSuchKey
	} else if err != nil {
		return nil, err
	}

	if !node.IsDir() {
		return nil, gofakes3.ErrNoSuchKey
	}

	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		return nil, err
	}

	return dirEntries, nil
}

func getFileHashByte(node any, hashType hash.Type) []byte {
	b, err := hex.DecodeString(getFileHash(node, hashType))
	if err != nil {
		return nil
	}
	return b
}

// emptyHash returns the hash of no data.
func emptyHash(hashType hash.Type) []byte {
	if hashType == hash.None {
		return nil
	}
	hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(hashType))
	if err != nil {
		return nil
	}
	b, err := hex.DecodeString(hasher.Sums()[hashType])
	if err != nil {
		return nil
	}
	return b
}

func getFileHash(node any, hashType hash.Type) string {
	if hashType == hash.None {
		return ""
	}

	var o fs.Object

	switch b := node.(type) {
	case vfs.Node:
		fsObj, ok := b.DirEntry().(fs.Object)
		if !ok {
			fs.Debugf("serve s3", "File uploading - reading hash from VFS cache")
			in, err := b.Open(os.O_RDONLY)
			if err != nil {
				return ""
			}
			defer func() {
				_ = in.Close()
			}()
			h, err := hash.NewMultiHasherTypes(hash.NewHashSet(hashType))
			if err != nil {
				return ""
			}
			_, err = io.Copy(h, in)
			if err != nil {
				return ""
			}
			return h.Sums()[hashType]
		}
		o = fsObj
	case fs.Object:
		o = b
	}

	hash, err := o.Hash(context.Background(), hashType)
	if err != nil {
		return ""
	}
	return hash
}

// canonicalKey reports whether key is a non-empty path in canonical form: it
// is equal to the path it cleans to, so it has no ".", ".." or empty
// (repeated, leading or trailing slash) segments.
//
// The key is anchored with a leading "/" before cleaning so that a leading
// ".." cannot silently escape upwards and still compare equal.
func canonicalKey(key string) bool {
	return key != "" && "/"+key == path.Clean("/"+key)
}

// errInvalidObjectName is returned for object keys that cannot be represented
// as a backend path, for example because they contain "..", "." or "//"
// segments. Like MinIO, this is reported as a 400 Bad Request rather than
// silently resolving the key to a different object (or one outside the
// bucket).
func errInvalidObjectName(key string) error {
	return gofakes3.ErrorMessagef(gofakes3.ErrInvalidArgument, "Object name contains unsupported characters: %q", key)
}

// bucketObjectPath joins the bucket name and object key into a backend path.
//
// S3 object keys are opaque, so rclone treats "dir/../file" and "file" as
// distinct keys and refuses to normalise one into the other. Keys that are
// not in canonical form (or that would escape the bucket) are rejected with
// errInvalidObjectName rather than resolved.
func bucketObjectPath(bucketName, objectName string) (string, error) {
	if !canonicalKey(objectName) {
		return "", errInvalidObjectName(objectName)
	}
	return path.Join(bucketName, objectName), nil
}

// bucketDirMarkerPath returns the backend path of the directory a
// directory marker key, one ending in "/" such as "dir/", stands for.
// isMarker is false if objectName isn't a directory marker key.
func bucketDirMarkerPath(bucketName, objectName string) (fp string, isMarker bool, err error) {
	dir, isMarker := strings.CutSuffix(objectName, "/")
	if !isMarker {
		return "", false, nil
	}
	fp, err = bucketObjectPath(bucketName, dir)
	if err != nil {
		return "", true, errInvalidObjectName(objectName)
	}
	return fp, true, nil
}

// errIsDirectory is returned for an object key which is the name of a
// directory, which can't be an object too.
func errIsDirectory(key string) error {
	return gofakes3.ErrorMessagef(gofakes3.ErrInvalidRequest, "Object name %q is the name of a directory", key)
}

// errParentIsObject is returned for an object key whose directory, or
// one of its parents, is the name of an object, so can't be a directory
// too.
func errParentIsObject(dir string) error {
	return gofakes3.ErrorMessagef(gofakes3.ErrInvalidRequest, "Directory %q is the name of an object", dir)
}

// bucketDirPath joins the bucket name and a directory path (a listing prefix)
// into a backend path. It is like bucketObjectPath except that the empty path
// is allowed and addresses the bucket root. Every other non-canonical path,
// including one with a trailing slash, is rejected - it is not normalised, so
// "dir" and "dir/" are not treated as the same directory.
func bucketDirPath(bucketName, dirName string) (string, error) {
	if dirName == "" {
		return bucketName, nil
	}
	return bucketObjectPath(bucketName, dirName)
}

// mkdirRecursive makes the directory path and any of its parents which
// don't exist, returning the top-most directory it made, or "" if path
// existed already.
//
// It returns errParentIsObject if path or one of its parents is a file.
//
// FIXME this could be implemented by VFS.MkdirAll()
func mkdirRecursive(path string, VFS *vfs.VFS) (made string, err error) {
	path = strings.Trim(path, "/")
	dirs := strings.Split(path, "/")
	var dir strings.Builder
	for _, d := range dirs {
		dir.WriteString("/" + d)
		if node, err := VFS.Stat(dir.String()); err == nil {
			if !node.IsDir() {
				// Report the directory without its bucket
				_, inBucket, _ := strings.Cut(strings.TrimPrefix(dir.String(), "/"), "/")
				return made, errParentIsObject(inBucket)
			}
		} else {
			err := VFS.Mkdir(dir.String(), 0777)
			if err != nil {
				return made, err
			}
			if made == "" {
				made = strings.TrimPrefix(dir.String(), "/")
			}
		}
	}
	return made, nil
}

// isEmptyDir reports whether dir is an empty directory.
func isEmptyDir(dir string, VFS *vfs.VFS) bool {
	entries, err := VFS.ReadDir(dir)
	return err == nil && len(entries) == 0
}

// removeMadeDirs removes the directory dir, which mkdirRecursive
// returned made for, and its parents while they are empty, up to and
// including made. It does nothing if made is "".
func removeMadeDirs(dir, made string, VFS *vfs.VFS) {
	if made == "" {
		return
	}
	for ; isEmptyDir(dir, VFS); dir = path.Dir(dir) {
		if VFS.Remove(dir) != nil || dir == made {
			return
		}
	}
}

// rmdirRecursive removes the parent directories of p which are empty,
// stopping at the bucket or at a directory with a directory marker of
// the user t, and forgets what t remembers of the directories removed.
func rmdirRecursive(p string, VFS *vfs.VFS, t *tenant) {
	dir := path.Dir(p)
	if !strings.ContainsAny(dir, "/\\") {
		// might be bucket(root)
		return
	}
	// Check the directory is empty first as the VFS logs an error
	// removing one which isn't
	if t.hasDirMarker(dir) || !isEmptyDir(dir, VFS) {
		return
	}
	if err := VFS.Remove(dir); err != nil {
		return
	}
	t.removeMeta(dir)
	rmdirRecursive(dir, VFS, t)
}

// pathLocks are locks on paths, made as needed.
type pathLocks struct {
	mu    sync.Mutex
	locks map[string]*pathLock
}

// pathLock is a lock on a path wanted by refs users.
type pathLock struct {
	mu   sync.Mutex
	refs int
}

// lock locks path, returning a function to unlock it.
func (p *pathLocks) lock(path string) (unlock func()) {
	p.mu.Lock()
	if p.locks == nil {
		p.locks = map[string]*pathLock{}
	}
	l := p.locks[path]
	if l == nil {
		l = &pathLock{}
		p.locks[path] = l
	}
	l.refs++
	p.mu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		p.mu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(p.locks, path)
		}
		p.mu.Unlock()
	}
}

// errNotEmpty is returned by removeEmptyTree for a directory holding
// files.
var errNotEmpty = errors.New("directory not empty")

// isEmptyTree reports whether dir holds nothing but empty directories,
// returning errNotEmpty if it holds any files.
func isEmptyTree(dir string, VFS *vfs.VFS) error {
	entries, err := VFS.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return errNotEmpty
		}
		if err := isEmptyTree(path.Join(dir, entry.Name()), VFS); err != nil {
			return err
		}
	}
	return nil
}

// removeEmptyTree removes dir if it holds nothing but empty
// directories, along with them, returning errNotEmpty if it holds any
// files.
func removeEmptyTree(dir string, VFS *vfs.VFS) error {
	// Check the whole tree first so nothing is removed if it isn't
	// empty
	if err := isEmptyTree(dir, VFS); err != nil {
		return err
	}
	var remove func(dir string) error
	remove = func(dir string) error {
		entries, err := VFS.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := remove(path.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
		return VFS.Remove(dir)
	}
	return remove(dir)
}

func authlistResolver(list []string) (map[string]string, error) {
	authList := make(map[string]string)
	for _, v := range list {
		parts := strings.Split(v, ",")
		if len(parts) != 2 {
			return nil, errors.New("invalid auth pair: expecting a single comma")
		}
		authList[parts[0]] = parts[1]
	}
	return authList, nil
}
