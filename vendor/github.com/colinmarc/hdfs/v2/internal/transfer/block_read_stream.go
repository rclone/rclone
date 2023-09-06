package transfer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

var errInvalidChecksum = errors.New("invalid checksum")

// blockReadStream implements io.Reader for reading a packet stream for a single
// block from a single datanode.
type blockReadStream struct {
	reader       io.Reader
	checksumTab  *crc32.Table
	chunkSize    int
	checksumSize int

	checksums bytes.Buffer
	chunk     bytes.Buffer

	packetLength int
	chunkIndex   int
	numChunks    int
	lastPacket   bool
}

func newBlockReadStream(reader io.Reader, chunkSize int, checksumTab *crc32.Table, checksumSize int) *blockReadStream {
	return &blockReadStream{
		reader:       reader,
		chunkSize:    chunkSize,
		checksumTab:  checksumTab,
		checksumSize: checksumSize,
	}
}

func (s *blockReadStream) Read(b []byte) (int, error) {
	if s.chunkIndex == s.numChunks {
		if s.lastPacket {
			return 0, io.EOF
		}

		err := s.startPacket()
		if err != nil {
			return 0, err
		}
	}

	remainingInPacket := (s.packetLength - (s.chunkIndex * s.chunkSize))

	// For small reads, we need to buffer a single chunk. If we did that
	// previously, read the rest of the buffer, so we're aligned back on a
	// chunk boundary.
	if s.chunk.Len() > 0 {
		n, _ := s.chunk.Read(b)
		return n, nil
	} else if len(b) < s.chunkSize {
		chunkSize := s.chunkSize
		if chunkSize > remainingInPacket {
			chunkSize = remainingInPacket
		}

		_, err := io.CopyN(&s.chunk, s.reader, int64(chunkSize))
		if err != nil {
			return 0, err
		}

		err = s.validateChecksum(s.chunk.Bytes())
		if err != nil {
			return 0, err
		}

		s.chunkIndex++
		n, _ := s.chunk.Read(b)
		return n, nil
	}

	// Always align reads to a chunk boundary. This makes the code much simpler,
	// and with readers that pick sane read sizes (like io.Copy), should be
	// efficient.
	var amountToRead int
	var chunksToRead int
	if len(b) > remainingInPacket {
		chunksToRead = s.numChunks - s.chunkIndex
		amountToRead = remainingInPacket
	} else {
		chunksToRead = len(b) / s.chunkSize
		amountToRead = chunksToRead * s.chunkSize
	}

	n, err := io.ReadFull(s.reader, b[:amountToRead])
	if err != nil {
		return n, err
	}

	// Validate the bytes we just read into b against the packet checksums.
	for i := 0; i < chunksToRead; i++ {
		chunkOff := i * s.chunkSize
		chunkEnd := chunkOff + s.chunkSize
		if chunkEnd >= n {
			chunkEnd = n
		}

		err := s.validateChecksum(b[chunkOff:chunkEnd])
		if err != nil {
			return n, err
		}

		s.chunkIndex++
	}

	// EOF would be returned by the next call to Read anyway, but it's nice to
	// return it here.
	if s.chunkIndex == s.numChunks && s.lastPacket {
		err = io.EOF
	}

	return n, err
}

func (s *blockReadStream) validateChecksum(b []byte) error {
	if s.checksumTab == nil {
		return nil
	}

	checksumOffset := 4 * s.chunkIndex
	checksumBytes := s.checksums.Bytes()[checksumOffset : checksumOffset+4]
	checksum := binary.BigEndian.Uint32(checksumBytes)

	crc := crc32.Checksum(b, s.checksumTab)
	if crc != checksum {
		return errInvalidChecksum
	}

	return nil
}

func (s *blockReadStream) startPacket() error {
	header, err := s.readPacketHeader()
	if err != nil {
		return err
	}

	dataLength := int(header.GetDataLen())
	numChunks := int(math.Ceil(float64(dataLength) / float64(s.chunkSize)))

	checksumsLength := numChunks * s.checksumSize
	s.checksums.Reset()
	s.checksums.Grow(checksumsLength)
	_, err = io.CopyN(&s.checksums, s.reader, int64(checksumsLength))
	if err != nil {
		return err
	}

	s.packetLength = dataLength
	s.numChunks = numChunks
	s.chunkIndex = 0
	s.lastPacket = header.GetLastPacketInBlock()

	return nil
}

func (s *blockReadStream) readPacketHeader() (*hdfs.PacketHeaderProto, error) {
	lengthBytes := make([]byte, 6)
	_, err := io.ReadFull(s.reader, lengthBytes)
	if err != nil {
		return nil, err
	}

	// We don't actually care about the total length.
	packetHeaderLength := binary.BigEndian.Uint16(lengthBytes[4:])
	packetHeaderBytes := make([]byte, packetHeaderLength)
	_, err = io.ReadFull(s.reader, packetHeaderBytes)
	if err != nil {
		return nil, err
	}

	packetHeader := &hdfs.PacketHeaderProto{}
	err = proto.Unmarshal(packetHeaderBytes, packetHeader)

	return packetHeader, nil
}
