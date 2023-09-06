package rpc

import (
	"errors"
	"io"

	hadoop "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_common"
	"google.golang.org/protobuf/proto"
)

var errUnexpectedSequenceNumber = errors.New("unexpected sequence number")

type transport interface {
	writeRequest(w io.Writer, method string, requestID int32, req proto.Message) error
	readResponse(r io.Reader, method string, requestID int32, resp proto.Message) error
}

// basicTransport implements plain RPC.
type basicTransport struct {
	// clientID is the client ID of this writer.
	clientID []byte
}

// writeRequest writes an RPC message.
//
// A request packet:
// +-----------------------------------------------------------+
// |  uint32 length of the next three parts                    |
// +-----------------------------------------------------------+
// |  varint length + RpcRequestHeaderProto                    |
// +-----------------------------------------------------------+
// |  varint length + RequestHeaderProto                       |
// +-----------------------------------------------------------+
// |  varint length + Request                                  |
// +-----------------------------------------------------------+
func (t *basicTransport) writeRequest(w io.Writer, method string, requestID int32, req proto.Message) error {
	rrh := newRPCRequestHeader(requestID, t.clientID)
	rh := newRequestHeader(method)

	reqBytes, err := makeRPCPacket(rrh, rh, req)
	if err != nil {
		return err
	}

	_, err = w.Write(reqBytes)
	return err
}

// ReadResponse reads a response message.
//
// A response from the namenode:
// +-----------------------------------------------------------+
// |  uint32 length of the next two parts                      |
// +-----------------------------------------------------------+
// |  varint length + RpcResponseHeaderProto                   |
// +-----------------------------------------------------------+
// |  varint length + Response                                 |
// +-----------------------------------------------------------+
func (t *basicTransport) readResponse(r io.Reader, method string, requestID int32, resp proto.Message) error {
	rrh := &hadoop.RpcResponseHeaderProto{}
	err := readRPCPacket(r, rrh, resp)
	if err != nil {
		return err
	} else if int32(rrh.GetCallId()) != requestID {
		return errUnexpectedSequenceNumber
	} else if rrh.GetStatus() != hadoop.RpcResponseHeaderProto_SUCCESS {
		return &NamenodeError{
			method:    method,
			message:   rrh.GetErrorMsg(),
			code:      int(rrh.GetErrorDetail()),
			exception: rrh.GetExceptionClassName(),
		}
	}

	return nil
}
