package hdfs

import (
	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
)

// AllowSnapshots marks a directory as available for snapshots.
// This is required to make a snapshot of a directory as snapshottable
// directories work as a whitelist.
//
// This requires superuser privileges.
func (c *Client) AllowSnapshots(dir string) error {
	allowSnapshotReq := &hdfs.AllowSnapshotRequestProto{SnapshotRoot: &dir}
	allowSnapshotRes := &hdfs.AllowSnapshotResponseProto{}

	err := c.namenode.Execute("allowSnapshot", allowSnapshotReq, allowSnapshotRes)
	if err != nil {
		return interpretException(err)
	}

	return nil
}

// DisallowSnapshots marks a directory as unavailable for snapshots.
//
// This requires superuser privileges.
func (c *Client) DisallowSnapshots(dir string) error {
	disallowSnapshotReq := &hdfs.DisallowSnapshotRequestProto{SnapshotRoot: &dir}
	disallowSnapshotRes := &hdfs.DisallowSnapshotResponseProto{}

	err := c.namenode.Execute("disallowSnapshot", disallowSnapshotReq, disallowSnapshotRes)
	if err != nil {
		return interpretException(err)
	}

	return nil
}

// CreateSnapshots creates a snapshot of a given directory and name, and
// returns the path containing the snapshot. Snapshot names must be unique.
//
// This requires superuser privileges.
func (c *Client) CreateSnapshot(dir, name string) (string, error) {
	allowSnapshotReq := &hdfs.CreateSnapshotRequestProto{
		SnapshotRoot: &dir,
		SnapshotName: &name,
	}
	allowSnapshotRes := &hdfs.CreateSnapshotResponseProto{}

	err := c.namenode.Execute("createSnapshot", allowSnapshotReq, allowSnapshotRes)
	if err != nil {
		return "", interpretException(err)
	}

	return allowSnapshotRes.GetSnapshotPath(), nil
}

// CreateSnapshots deletes a snapshot with a given directory and name.
//
// This requires superuser privileges.
func (c *Client) DeleteSnapshot(dir, name string) error {
	allowSnapshotReq := &hdfs.DeleteSnapshotRequestProto{
		SnapshotRoot: &dir,
		SnapshotName: &name,
	}
	allowSnapshotRes := &hdfs.DeleteSnapshotResponseProto{}

	err := c.namenode.Execute("deleteSnapshot", allowSnapshotReq, allowSnapshotRes)
	if err != nil {
		return interpretException(err)
	}
	return nil
}
