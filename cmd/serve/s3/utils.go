package s3

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/Mikubill/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

func getDirEntries(prefix string, fs *vfs.VFS) (vfs.Nodes, error) {
	node, err := fs.Stat(prefix)

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

func getFileHashByte(node interface{}) []byte {
	b, err := hex.DecodeString(getFileHash(node))
	if err != nil {
		return nil
	}
	return b
}

func getFileHash(node interface{}) string {
	var o fs.Object

	switch b := node.(type) {
	case vfs.Node:
		o = b.DirEntry().(fs.Object)
	case fs.DirEntry:
		o = b.(fs.Object)
	}

	hash, err := o.Hash(context.Background(), Opt.hashType)
	if err != nil {
		return ""
	}
	return hash
}

func prefixParser(p *gofakes3.Prefix) (path, remaining string) {

	idx := strings.LastIndexByte(p.Prefix, '/')
	if idx < 0 {
		return "", p.Prefix
	}
	return p.Prefix[:idx], p.Prefix[idx+1:]
}

func mkdirRecursive(path string, fs *vfs.VFS) error {
	path = strings.Trim(path, "/")
	dirs := strings.Split(path, "/")
	dir := ""
	for _, d := range dirs {
		dir += "/" + d
		if _, err := fs.Stat(dir); err != nil {
			err := fs.Mkdir(dir, 0777)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func rmdirRecursive(p string, fs *vfs.VFS) {
	dir := path.Dir(p)
	if !strings.ContainsAny(dir, "/\\") {
		// might be bucket(root)
		return
	}
	if _, err := fs.Stat(dir); err == nil {
		err := fs.Remove(dir)
		if err != nil {
			return
		}
		rmdirRecursive(dir, fs)
	}
}

func authlistResolver(list []string) map[string]string {
	authList := make(map[string]string)
	for _, v := range list {
		splited := strings.Split(v, ",")
		if len(splited) != 2 {
			fs.Infof(nil, fmt.Sprintf("Ignored: invalid auth pair %s", v))
			continue
		}
		authList[splited[0]] = splited[1]
	}
	return authList
}
