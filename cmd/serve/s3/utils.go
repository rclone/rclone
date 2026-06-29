package s3

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"strings"

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

func prefixParser(p *gofakes3.Prefix) (path, remaining string) {
	idx := strings.LastIndexByte(p.Prefix, '/')
	if idx < 0 {
		return "", p.Prefix
	}
	return p.Prefix[:idx], p.Prefix[idx+1:]
}

// FIXME this could be implemented by VFS.MkdirAll()
func mkdirRecursive(path string, VFS *vfs.VFS) error {
	path = strings.Trim(path, "/")
	dirs := strings.Split(path, "/")
	dir := ""
	for _, d := range dirs {
		dir += "/" + d
		if _, err := VFS.Stat(dir); err != nil {
			err := VFS.Mkdir(dir, 0777)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func rmdirRecursive(p string, VFS *vfs.VFS) {
	dir := path.Dir(p)
	if !strings.ContainsAny(dir, "/\\") {
		// might be bucket(root)
		return
	}
	if _, err := VFS.Stat(dir); err == nil {
		err := VFS.Remove(dir)
		if err != nil {
			return
		}
		rmdirRecursive(dir, VFS)
	}
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
