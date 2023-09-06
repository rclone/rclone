// Package rpc implements some of the lower-level functionality required to
// communicate with the namenode and datanodes.
package rpc

import (
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"time"

	"google.golang.org/protobuf/proto"
)

var errInvalidResponse = errors.New("invalid response from namenode")

// Used for client ID generation, below.
const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func newClientID() []byte {
	id := make([]byte, 16)

	rand.Seed(time.Now().UTC().UnixNano())
	for i := range id {
		id[i] = chars[rand.Intn(len(chars))]
	}

	return id
}

func makePrefixedMessage(msg proto.Message) ([]byte, error) {
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	lengthBytes := make([]byte, 10)
	n := binary.PutUvarint(lengthBytes, uint64(len(msgBytes)))
	return append(lengthBytes[:n], msgBytes...), nil
}

func makeRPCPacket(msgs ...proto.Message) ([]byte, error) {
	packet := make([]byte, 4, 128)

	length := 0
	for _, msg := range msgs {
		b, err := makePrefixedMessage(msg)
		if err != nil {
			return nil, err
		}

		packet = append(packet, b...)
		length += len(b)
	}

	binary.BigEndian.PutUint32(packet, uint32(length))
	return packet, nil
}

func readRPCPacket(r io.Reader, msgs ...proto.Message) error {
	var packetLength uint32
	err := binary.Read(r, binary.BigEndian, &packetLength)
	if err != nil {
		return err
	}

	packet := make([]byte, packetLength)
	_, err = io.ReadFull(r, packet)
	if err != nil {
		return err
	}

	for _, msg := range msgs {
		// HDFS doesn't send all the response messages all the time (for example, if
		// the RpcResponseHeaderProto contains an error).
		if len(packet) == 0 {
			return nil
		}

		msgLength, n := binary.Uvarint(packet)
		if n <= 0 || msgLength > uint64(len(packet)) {
			return errInvalidResponse
		}

		packet = packet[n:]
		if msgLength != 0 {
			err = proto.Unmarshal(packet[:msgLength], msg)
			if err != nil {
				return err
			}

			packet = packet[msgLength:]
		}
	}

	if len(packet) > 0 {
		return errInvalidResponse
	}

	return nil
}
