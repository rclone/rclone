package rs

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"time"
)

// RS footer v1 layout for Reed-Solomon shard particles.
const (
	FooterVersion = 1
	// FooterSize is the trailing metadata size per shard particle (v1).
	FooterSize = 104

	footerOffVersion       = 8
	footerOffAlgorithm     = 12
	footerOffContentLength = 16
	footerOffMD5           = 24
	footerOffSHA256        = 40
	footerOffMtime         = 72
	footerOffStripeSize    = 80
	footerOffNumStripes    = 84
	footerOffPayloadCRC32C = 88
	footerOffDataShards    = 92
	footerOffParityShards  = 93
	footerOffCurrentShard  = 94
	footerOffReserved      = 95
	footerOffWriteID       = 96
)

// FooterMagic is the 8-byte particle footer identifier (RCLONE + RS).
var FooterMagic = [8]byte{'R', 'C', 'L', 'O', 'N', 'E', 'R', 'S'}

// AlgorithmSYMM is the footer algorithm tag for stripe-wise systematic RS encoding.
var AlgorithmSYMM = [4]byte{'S', 'Y', 'M', 'M'}

var emptyFileMD5 [16]byte
var emptyFileSHA256 [32]byte

func init() {
	_, _ = hex.Decode(emptyFileMD5[:], []byte("d41d8cd98f00b204e9800998ecf8427e"))
	_, _ = hex.Decode(emptyFileSHA256[:], []byte("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
}

// Footer holds metadata from a single RS shard particle.
type Footer struct {
	ContentLength int64
	MD5           [16]byte
	SHA256        [32]byte
	// Mtime is nanoseconds since the Unix epoch.
	Mtime        int64
	Algorithm    [4]byte
	DataShards   uint8
	ParityShards uint8
	CurrentShard uint8
	// StripeSize is the per-shard fragment size S in bytes (one RS stripe appends S bytes per shard).
	StripeSize    uint32
	PayloadCRC32C uint32
	// NumStripes is the number of RS stripes in the shard payload (parity: NumStripes*StripeSize bytes; data shards use virtual padding).
	NumStripes uint32
	// WriteID is a random nonce shared by every shard particle of one Put (guards against torn/mixed-write reads).
	WriteID uint64
}

// MarshalBinary encodes the footer to its on-disk layout.
func (f *Footer) MarshalBinary() ([]byte, error) {
	b := make([]byte, FooterSize)
	copy(b[0:8], FooterMagic[:])
	binary.LittleEndian.PutUint32(b[footerOffVersion:], FooterVersion)
	copy(b[footerOffAlgorithm:], f.Algorithm[:])
	binary.LittleEndian.PutUint64(b[footerOffContentLength:], uint64(f.ContentLength))
	copy(b[footerOffMD5:], f.MD5[:])
	copy(b[footerOffSHA256:], f.SHA256[:])
	binary.LittleEndian.PutUint64(b[footerOffMtime:], uint64(f.Mtime))
	binary.LittleEndian.PutUint32(b[footerOffStripeSize:], f.StripeSize)
	binary.LittleEndian.PutUint32(b[footerOffNumStripes:], f.NumStripes)
	binary.LittleEndian.PutUint32(b[footerOffPayloadCRC32C:], f.PayloadCRC32C)
	b[footerOffDataShards] = f.DataShards
	b[footerOffParityShards] = f.ParityShards
	b[footerOffCurrentShard] = f.CurrentShard
	b[footerOffReserved] = 0
	binary.LittleEndian.PutUint64(b[footerOffWriteID:], f.WriteID)
	return b, nil
}

// ParseFooter decodes a footer from its binary layout.
func ParseFooter(buf []byte) (*Footer, error) {
	if len(buf) != FooterSize {
		return nil, fmt.Errorf("footer: buffer length must be %d", FooterSize)
	}
	if !bytes.Equal(buf[0:8], FooterMagic[:]) {
		return nil, errors.New("footer: invalid magic")
	}
	ver := binary.LittleEndian.Uint32(buf[footerOffVersion:])
	if ver != FooterVersion {
		return nil, fmt.Errorf("footer: unsupported version %d (want %d)", ver, FooterVersion)
	}
	f := &Footer{}
	f.ContentLength = int64(binary.LittleEndian.Uint64(buf[footerOffContentLength:]))
	copy(f.MD5[:], buf[footerOffMD5:])
	copy(f.SHA256[:], buf[footerOffSHA256:])
	f.Mtime = int64(binary.LittleEndian.Uint64(buf[footerOffMtime:]))
	copy(f.Algorithm[:], buf[footerOffAlgorithm:])
	f.DataShards = buf[footerOffDataShards]
	f.ParityShards = buf[footerOffParityShards]
	f.CurrentShard = buf[footerOffCurrentShard]
	f.StripeSize = binary.LittleEndian.Uint32(buf[footerOffStripeSize:])
	f.PayloadCRC32C = binary.LittleEndian.Uint32(buf[footerOffPayloadCRC32C:])
	f.NumStripes = binary.LittleEndian.Uint32(buf[footerOffNumStripes:])
	f.WriteID = binary.LittleEndian.Uint64(buf[footerOffWriteID:])
	return f, nil
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func crc32cChecksum(data []byte) uint32 {
	return crc32.Checksum(data, crc32cTable)
}

// CRC32C returns the Castagnoli CRC32 of data (same as footer PayloadCRC32C).
func CRC32C(data []byte) uint32 {
	return crc32cChecksum(data)
}

// NewRSFooter builds a Footer for shard shardIndex with the given logical object metadata.
func NewRSFooter(contentLength int64, md5Sum, sha256Sum []byte, mtime time.Time, dataShards, parityShards, shardIndex int, stripeSize, numStripes uint32, payloadCRC uint32, writeID uint64) *Footer {
	var md5Arr [16]byte
	var sha256Arr [32]byte
	if len(md5Sum) >= 16 {
		copy(md5Arr[:], md5Sum[:16])
	} else if contentLength == 0 {
		md5Arr = emptyFileMD5
	}
	if len(sha256Sum) >= 32 {
		copy(sha256Arr[:], sha256Sum[:32])
	} else if contentLength == 0 {
		sha256Arr = emptyFileSHA256
	}
	return &Footer{
		ContentLength: contentLength,
		MD5:           md5Arr,
		SHA256:        sha256Arr,
		Mtime:         mtime.UnixNano(),
		Algorithm:     AlgorithmSYMM,
		DataShards:    uint8(dataShards),
		ParityShards:  uint8(parityShards),
		CurrentShard:  uint8(shardIndex),
		StripeSize:    stripeSize,
		NumStripes:    numStripes,
		PayloadCRC32C: payloadCRC,
		WriteID:       writeID,
	}
}
