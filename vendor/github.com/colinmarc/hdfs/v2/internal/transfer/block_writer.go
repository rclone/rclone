package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

var ErrEndOfBlock = errors.New("end of block")

// BlockWriter implements io.WriteCloser for writing a block to a datanode.
// Given a block location, it handles pipeline construction and failures,
// including communicating with the namenode if need be.
type BlockWriter struct {
	// ClientName is the unique ID used by the NamenodeConnection to initialize
	// the block.
	ClientName string
	// Block is the block location provided by the namenode.
	Block *hdfs.LocatedBlockProto
	// BlockSize is the target size of the new block (or the existing one, if
	// appending). The represents the configured value, not the actual number
	// of bytes currently in the block.
	BlockSize int64
	// Offset is the current write offset in the block.
	Offset int64
	// Append indicates whether this is an append operation on an existing block.
	Append bool
	// UseDatanodeHostname indicates whether the datanodes will be connected to
	// via hostname (if true) or IP address (if false).
	UseDatanodeHostname bool
	// DialFunc is used to connect to the datanodes. If nil, then
	// (&net.Dialer{}).DialContext is used.
	DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

	conn     net.Conn
	deadline time.Time
	stream   *blockWriteStream
	closed   bool
}

// SetDeadline sets the deadline for future Write, Flush, and Close calls. A
// zero value for t means those calls will not time out.
func (bw *BlockWriter) SetDeadline(t time.Time) error {
	bw.deadline = t
	if bw.conn != nil {
		return bw.conn.SetDeadline(t)
	}

	// Return the error at connection time.
	return nil
}

// Write implements io.Writer.
//
// Unlike BlockReader, BlockWriter currently has no ability to recover from
// write failures (timeouts, datanode failure, etc). Once it returns an error
// from Write or Close, it may be in an invalid state.
//
// This will hopefully be fixed in a future release.
func (bw *BlockWriter) Write(b []byte) (int, error) {
	var blockFull bool
	if bw.Offset >= bw.BlockSize {
		return 0, ErrEndOfBlock
	} else if (bw.Offset + int64(len(b))) > bw.BlockSize {
		blockFull = true
		b = b[:bw.BlockSize-bw.Offset]
	}

	if bw.stream == nil {
		err := bw.connectNext()
		// TODO: handle failures, set up recovery pipeline
		if err != nil {
			return 0, err
		}
	}

	// TODO: handle failures, set up recovery pipeline
	n, err := bw.stream.Write(b)
	bw.Offset += int64(n)
	if err == nil && blockFull {
		err = ErrEndOfBlock
	}

	return n, err
}

// Flush flushes any unwritten packets out to the datanode.
func (bw *BlockWriter) Flush() error {
	if bw.stream != nil {
		return bw.stream.flush(true)
	}

	return nil
}

// Close implements io.Closer. It flushes any unwritten packets out to the
// datanode, and sends a final packet indicating the end of the block. The
// block must still be finalized with the namenode.
func (bw *BlockWriter) Close() error {
	bw.closed = true
	if bw.conn != nil {
		defer bw.conn.Close()
	}

	if bw.stream != nil {
		// TODO: handle failures, set up recovery pipeline
		err := bw.stream.finish()
		if err != nil {
			return err
		}
	}

	return nil
}

func (bw *BlockWriter) connectNext() error {
	address := getDatanodeAddress(bw.currentPipeline()[0].GetId(), bw.UseDatanodeHostname)

	if bw.DialFunc == nil {
		bw.DialFunc = (&net.Dialer{}).DialContext
	}

	conn, err := bw.DialFunc(context.Background(), "tcp", address)
	if err != nil {
		return err
	}

	err = conn.SetDeadline(bw.deadline)
	if err != nil {
		return err
	}

	err = bw.writeBlockWriteRequest(conn)
	if err != nil {
		return err
	}

	resp, err := readBlockOpResponse(conn)
	if err != nil {
		return err
	} else if resp.GetStatus() != hdfs.Status_SUCCESS {
		return fmt.Errorf("write failed: %s (%s)", resp.GetStatus().String(), resp.GetMessage())
	}

	bw.conn = conn
	bw.stream = newBlockWriteStream(conn, bw.Offset)
	return nil
}

func (bw *BlockWriter) currentPipeline() []*hdfs.DatanodeInfoProto {
	// TODO: we need to be able to reconfigure the pipeline when a node fails.
	//
	// targets := make([]*hdfs.DatanodeInfoProto, 0, len(br.pipeline))
	// for _, loc := range s.block.GetLocs() {
	// 	addr := getDatanodeAddress(loc)
	// 	for _, pipelineAddr := range br.pipeline {
	// 		if ipAddr == addr {
	// 			append(targets, loc)
	// 		}
	// 	}
	// }
	//
	// return targets

	return bw.Block.GetLocs()
}

func (bw *BlockWriter) currentStage() hdfs.OpWriteBlockProto_BlockConstructionStage {
	// TODO: this should be PIPELINE_SETUP_STREAMING_RECOVERY or
	// PIPELINE_SETUP_APPEND_RECOVERY for recovery.
	if bw.Append {
		return hdfs.OpWriteBlockProto_PIPELINE_SETUP_APPEND
	}

	return hdfs.OpWriteBlockProto_PIPELINE_SETUP_CREATE
}

func (bw *BlockWriter) generationTimestamp() int64 {
	if bw.Append {
		return int64(bw.Block.B.GetGenerationStamp())
	}

	return 0
}

// writeBlockWriteRequest creates an OpWriteBlock message and submits it to the
// datanode. This occurs before any writing actually occurs, and is intended
// to synchronize the client with the datanode, returning an error if the
// submitted expected state differs from the actual state on the datanode.
//
// The field "MinBytesRcvd" below is used during append operation and should be
// the block's expected size. The field "MaxBytesRcvd" is used only in the case
// of PIPELINE_SETUP_STREAMING_RECOVERY.
//
// See: https://github.com/apache/hadoop/blob/6314843881b4c67d08215e60293f8b33242b9416/hadoop-hdfs-project/hadoop-hdfs/src/main/java/org/apache/hadoop/hdfs/server/datanode/BlockReceiver.java#L216
// And: https://github.com/apache/hadoop/blob/6314843881b4c67d08215e60293f8b33242b9416/hadoop-hdfs-project/hadoop-hdfs/src/main/java/org/apache/hadoop/hdfs/server/datanode/fsdataset/impl/FsDatasetImpl.java#L1462
func (bw *BlockWriter) writeBlockWriteRequest(w io.Writer) error {
	targets := bw.currentPipeline()[1:]

	op := &hdfs.OpWriteBlockProto{
		Header: &hdfs.ClientOperationHeaderProto{
			BaseHeader: &hdfs.BaseHeaderProto{
				Block: bw.Block.GetB(),
				Token: bw.Block.GetBlockToken(),
			},
			ClientName: proto.String(bw.ClientName),
		},
		Targets:               targets,
		Stage:                 bw.currentStage().Enum(),
		PipelineSize:          proto.Uint32(uint32(len(targets))),
		MinBytesRcvd:          proto.Uint64(bw.Block.GetB().GetNumBytes()),
		MaxBytesRcvd:          proto.Uint64(uint64(bw.Offset)),
		LatestGenerationStamp: proto.Uint64(uint64(bw.generationTimestamp())),
		RequestedChecksum: &hdfs.ChecksumProto{
			Type:             hdfs.ChecksumTypeProto_CHECKSUM_CRC32C.Enum(),
			BytesPerChecksum: proto.Uint32(outboundChunkSize),
		},
	}

	return writeBlockOpRequest(w, writeBlockOp, op)
}
