package hdfs

import (
	"os"
	"path"
	"time"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

// FileInfo implements os.FileInfo, and provides information about a file or
// directory in HDFS.
type FileInfo struct {
	name   string
	status *FileStatus
}

type FileStatus = hdfs.HdfsFileStatusProto

// Stat returns an os.FileInfo describing the named file or directory.
func (c *Client) Stat(name string) (os.FileInfo, error) {
	fi, err := c.getFileInfo(name)
	if err != nil {
		err = &os.PathError{"stat", name, interpretException(err)}
	}

	return fi, err
}

func (c *Client) getFileInfo(name string) (os.FileInfo, error) {
	req := &hdfs.GetFileInfoRequestProto{Src: proto.String(name)}
	resp := &hdfs.GetFileInfoResponseProto{}

	err := c.namenode.Execute("getFileInfo", req, resp)
	if err != nil {
		return nil, err
	}

	if resp.GetFs() == nil {
		return nil, os.ErrNotExist
	}

	return newFileInfo(resp.GetFs(), name), nil
}

func newFileInfo(status *hdfs.HdfsFileStatusProto, name string) *FileInfo {
	fi := &FileInfo{status: status}

	var fullName string
	if string(status.GetPath()) != "" {
		fullName = string(status.GetPath())
	} else {
		fullName = name
	}

	fi.name = path.Base(fullName)
	return fi
}

func (fi *FileInfo) Name() string {
	return fi.name
}

func (fi *FileInfo) Size() int64 {
	return int64(fi.status.GetLength())
}

func (fi *FileInfo) Mode() os.FileMode {
	mode := os.FileMode(fi.status.GetPermission().GetPerm())
	if fi.IsDir() {
		mode |= os.ModeDir
	}

	return mode
}

func (fi *FileInfo) ModTime() time.Time {
	return time.Unix(0, int64(fi.status.GetModificationTime())*int64(time.Millisecond))
}

func (fi *FileInfo) IsDir() bool {
	return fi.status.GetFileType() == hdfs.HdfsFileStatusProto_IS_DIR
}

// Sys returns the raw *hadoop_hdfs.HdfsFileStatusProto message from the
// namenode.
func (fi *FileInfo) Sys() interface{} {
	return fi.status
}

// Owner returns the name of the user that owns the file or directory. It's not
// part of the os.FileInfo interface.
func (fi *FileInfo) Owner() string {
	return fi.status.GetOwner()
}

// OwnerGroup returns the name of the group that owns the file or directory.
// It's not part of the os.FileInfo interface.
func (fi *FileInfo) OwnerGroup() string {
	return fi.status.GetGroup()
}

// AccessTime returns the last time the file was accessed. It's not part of the
// os.FileInfo interface.
func (fi *FileInfo) AccessTime() time.Time {
	return time.Unix(int64(fi.status.GetAccessTime())/1000, 0)
}
