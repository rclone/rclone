package transfer

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
)

// ChecksumReader provides an interface for reading the "MD5CRC32" checksums of
// individual blocks. It abstracts over reading from multiple datanodes, in
// order to be robust to failures.
type ChecksumReader struct {
	// Block is the block location provided by the namenode.
	Block *hdfs.LocatedBlockProto
	// UseDatanodeHostname specifies whether the datanodes should be connected to
	// via their hostnames (if true) or IP addresses (if false).
	UseDatanodeHostname bool
	// DialFunc is used to connect to the datanodes. If nil, then (&net.Dialer{}).DialContext is used
	DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

	deadline  time.Time
	datanodes *datanodeFailover
}

// SetDeadline sets the deadline for future ReadChecksum calls. A zero value
// for t means Read will not time out.
func (cr *ChecksumReader) SetDeadline(t time.Time) error {
	cr.deadline = t
	// Return the error at connection time.
	return nil
}

// ReadChecksum returns the checksum of the block.
func (cr *ChecksumReader) ReadChecksum() ([]byte, error) {
	if cr.datanodes == nil {
		locs := cr.Block.GetLocs()
		datanodes := make([]string, len(locs))
		for i, loc := range locs {
			dn := loc.GetId()
			datanodes[i] = getDatanodeAddress(dn, cr.UseDatanodeHostname)
		}

		cr.datanodes = newDatanodeFailover(datanodes)
	}

	for cr.datanodes.numRemaining() > 0 {
		address := cr.datanodes.next()
		checksum, err := cr.readChecksum(address)
		if err != nil {
			cr.datanodes.recordFailure(err)
			continue
		}

		return checksum, nil
	}

	err := cr.datanodes.lastError()
	if err != nil {
		err = errors.New("No available datanodes for block.")
	}

	return nil, err
}

func (cr *ChecksumReader) readChecksum(address string) ([]byte, error) {
	if cr.DialFunc == nil {
		cr.DialFunc = (&net.Dialer{}).DialContext
	}

	conn, err := cr.DialFunc(context.Background(), "tcp", address)
	if err != nil {
		return nil, err
	}

	err = conn.SetDeadline(cr.deadline)
	if err != nil {
		return nil, err
	}

	err = cr.writeBlockChecksumRequest(conn)
	if err != nil {
		return nil, err
	}

	resp, err := cr.readBlockChecksumResponse(conn)
	if err != nil {
		return nil, err
	}

	return resp.GetChecksumResponse().GetBlockChecksum(), nil
}

// A checksum request to a datanode:
// +-----------------------------------------------------------+
// |  Data Transfer Protocol Version, int16                    |
// +-----------------------------------------------------------+
// |  Op code, 1 byte (CHECKSUM_BLOCK = 0x55)                  |
// +-----------------------------------------------------------+
// |  varint length + OpReadBlockProto                         |
// +-----------------------------------------------------------+
func (cr *ChecksumReader) writeBlockChecksumRequest(w io.Writer) error {
	header := []byte{0x00, dataTransferVersion, checksumBlockOp}

	op := newChecksumBlockOp(cr.Block)
	opBytes, err := makePrefixedMessage(op)
	if err != nil {
		return err
	}

	req := append(header, opBytes...)
	_, err = w.Write(req)
	if err != nil {
		return err
	}

	return nil
}

// The response from the datanode:
// +-----------------------------------------------------------+
// |  varint length + BlockOpResponseProto                     |
// +-----------------------------------------------------------+
func (cr *ChecksumReader) readBlockChecksumResponse(r io.Reader) (*hdfs.BlockOpResponseProto, error) {
	resp := &hdfs.BlockOpResponseProto{}
	err := readPrefixedMessage(r, resp)
	return resp, err
}

func newChecksumBlockOp(block *hdfs.LocatedBlockProto) *hdfs.OpBlockChecksumProto {
	return &hdfs.OpBlockChecksumProto{
		Header: &hdfs.BaseHeaderProto{
			Block: block.GetB(),
			Token: block.GetBlockToken(),
		},
	}
}
