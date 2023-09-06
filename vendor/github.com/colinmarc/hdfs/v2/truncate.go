package hdfs

import (
	"errors"
	"os"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

// Truncate truncates the file specified by name to the given size, and returns
// the status any error encountered. The returned status will false in the case
// of any error or, if the error is nil, if HDFS indicated that the operation
// will be performed asynchronously and is not yet complete.
func (c *Client) Truncate(name string, size int64) (bool, error) {
	req := &hdfs.TruncateRequestProto{
		Src:        proto.String(name),
		NewLength:  proto.Uint64(uint64(size)),
		ClientName: proto.String(c.namenode.ClientName),
	}
	resp := &hdfs.TruncateResponseProto{}

	err := c.namenode.Execute("truncate", req, resp)
	if err != nil {
		return false, &os.PathError{"truncate", name, interpretException(err)}
	} else if resp.Result == nil {
		return false, &os.PathError{"truncate", name, errors.New("unexpected empty response")}
	}

	return resp.GetResult(), nil
}
