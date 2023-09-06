package hdfs

import (
	"os"
	"time"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

// Chmod changes the mode of the named file to mode.
func (c *Client) Chmod(name string, perm os.FileMode) error {
	req := &hdfs.SetPermissionRequestProto{
		Src:        proto.String(name),
		Permission: &hdfs.FsPermissionProto{Perm: proto.Uint32(uint32(perm))},
	}
	resp := &hdfs.SetPermissionResponseProto{}

	err := c.namenode.Execute("setPermission", req, resp)
	if err != nil {
		return &os.PathError{"chmod", name, interpretException(err)}
	}

	return nil
}

// Chown changes the user and group of the file. Unlike os.Chown, this takes
// a string username and group (since that's what HDFS uses.)
//
// If an empty string is passed for user or group, that field will not be
// changed remotely.
func (c *Client) Chown(name string, user, group string) error {
	req := &hdfs.SetOwnerRequestProto{
		Src:       proto.String(name),
		Username:  proto.String(user),
		Groupname: proto.String(group),
	}
	resp := &hdfs.SetOwnerResponseProto{}

	err := c.namenode.Execute("setOwner", req, resp)
	if err != nil {
		return &os.PathError{"chown", name, interpretException(err)}
	}

	return nil
}

// Chtimes changes the access and modification times of the named file.
func (c *Client) Chtimes(name string, atime time.Time, mtime time.Time) error {
	req := &hdfs.SetTimesRequestProto{
		Src:   proto.String(name),
		Mtime: proto.Uint64(uint64(mtime.Unix()) * 1000),
		Atime: proto.Uint64(uint64(atime.Unix()) * 1000),
	}
	resp := &hdfs.SetTimesResponseProto{}

	err := c.namenode.Execute("setTimes", req, resp)
	if err != nil {
		return &os.PathError{"chtimes", name, interpretException(err)}
	}

	return nil
}
