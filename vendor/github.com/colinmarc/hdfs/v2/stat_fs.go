package hdfs

import (
	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
)

// FsInfo provides information about HDFS
type FsInfo struct {
	Capacity              uint64
	Used                  uint64
	Remaining             uint64
	UnderReplicated       uint64
	CorruptBlocks         uint64
	MissingBlocks         uint64
	MissingReplOneBlocks  uint64
	BlocksInFuture        uint64
	PendingDeletionBlocks uint64
}

func (c *Client) StatFs() (FsInfo, error) {
	req := &hdfs.GetFsStatusRequestProto{}
	resp := &hdfs.GetFsStatsResponseProto{}

	err := c.namenode.Execute("getFsStats", req, resp)
	if err != nil {
		return FsInfo{}, err
	}

	var fs FsInfo
	fs.Capacity = resp.GetCapacity()
	fs.Used = resp.GetUsed()
	fs.Remaining = resp.GetRemaining()
	fs.UnderReplicated = resp.GetUnderReplicated()
	fs.CorruptBlocks = resp.GetCorruptBlocks()
	fs.MissingBlocks = resp.GetMissingBlocks()
	fs.MissingReplOneBlocks = resp.GetMissingReplOneBlocks()
	fs.BlocksInFuture = resp.GetBlocksInFuture()
	fs.PendingDeletionBlocks = resp.GetPendingDeletionBlocks()

	return fs, nil
}
