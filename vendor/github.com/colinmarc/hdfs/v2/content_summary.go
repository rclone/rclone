package hdfs

import (
	"os"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

// ContentSummary represents a set of information about a file or directory in
// HDFS. It's provided directly by the namenode, and has no unix filesystem
// analogue.
type ContentSummary struct {
	name           string
	contentSummary *hdfs.ContentSummaryProto
}

// GetContentSummary returns a ContentSummary representing the named file or
// directory. The summary contains information about the entire tree rooted
// in the named file; for instance, it can return the total size of all
func (c *Client) GetContentSummary(name string) (*ContentSummary, error) {
	cs, err := c.getContentSummary(name)
	if err != nil {
		err = &os.PathError{"content summary", name, interpretException(err)}
	}

	return cs, err
}

func (c *Client) getContentSummary(name string) (*ContentSummary, error) {
	req := &hdfs.GetContentSummaryRequestProto{Path: proto.String(name)}
	resp := &hdfs.GetContentSummaryResponseProto{}

	err := c.namenode.Execute("getContentSummary", req, resp)
	if err != nil {
		return nil, err
	}

	return &ContentSummary{name, resp.GetSummary()}, nil
}

// Size returns the total size of the named path, including any subdirectories.
func (cs *ContentSummary) Size() int64 {
	return int64(cs.contentSummary.GetLength())
}

// SizeAfterReplication returns the total size of the named path, including any
// subdirectories. Unlike Size, it counts the total replicated size of each
// file, and represents the total on-disk footprint for a tree in HDFS.
func (cs *ContentSummary) SizeAfterReplication() int64 {
	return int64(cs.contentSummary.GetSpaceConsumed())
}

// FileCount returns the number of files under the named path, including any
// subdirectories. If the named path is a file, FileCount returns 1.
func (cs *ContentSummary) FileCount() int {
	return int(cs.contentSummary.GetFileCount())
}

// DirectoryCount returns the number of directories under the named one,
// including any subdirectories, and including the root directory itself. If
// the named path is a file, this returns 0.
func (cs *ContentSummary) DirectoryCount() int {
	return int(cs.contentSummary.GetDirectoryCount())
}

// NameQuota returns the HDFS configured "name quota" for the named path. The
// name quota is a hard limit on the number of directories and files inside a
// directory; see http://goo.gl/sOSJmJ for more information.
func (cs *ContentSummary) NameQuota() int {
	return int(cs.contentSummary.GetQuota())
}

// SpaceQuota returns the HDFS configured "name quota" for the named path. The
// name quota is a hard limit on the number of directories and files inside
// a directory; see http://goo.gl/sOSJmJ for more information.
func (cs *ContentSummary) SpaceQuota() int64 {
	return int64(cs.contentSummary.GetSpaceQuota())
}
