package hdfs

import (
	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
)

// ServerDefaults represents the filesystem configuration stored on the
// Namenode.
type ServerDefaults struct {
	BlockSize           int64
	BytesPerChecksum    int
	WritePacketSize     int
	Replication         int
	FileBufferSize      int
	EncryptDataTransfer bool
	TrashInterval       int64
	KeyProviderURI      string
	PolicyId            int
}

// ServerDefaults fetches the stored defaults from the Namenode and returns
// them and any error encountered.
func (c *Client) ServerDefaults() (ServerDefaults, error) {
	resp, err := c.fetchDefaults()
	if err != nil {
		return ServerDefaults{}, err
	}

	return ServerDefaults{
		BlockSize:           int64(resp.GetBlockSize()),
		BytesPerChecksum:    int(resp.GetBytesPerChecksum()),
		WritePacketSize:     int(resp.GetWritePacketSize()),
		Replication:         int(resp.GetReplication()),
		FileBufferSize:      int(resp.GetFileBufferSize()),
		EncryptDataTransfer: resp.GetEncryptDataTransfer(),
		TrashInterval:       int64(resp.GetTrashInterval()),
		KeyProviderURI:      resp.GetKeyProviderUri(),
		PolicyId:            int(resp.GetPolicyId()),
	}, nil
}

func (c *Client) fetchDefaults() (*hdfs.FsServerDefaultsProto, error) {
	if c.defaults != nil {
		return c.defaults, nil
	}

	req := &hdfs.GetServerDefaultsRequestProto{}
	resp := &hdfs.GetServerDefaultsResponseProto{}

	err := c.namenode.Execute("getServerDefaults", req, resp)
	if err != nil {
		return nil, err
	}

	c.defaults = resp.GetServerDefaults()
	return c.defaults, nil
}
