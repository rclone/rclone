package rs

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"time"
)

// EC footer layout and algorithm identifiers for Reed-Solomon particles.
const (
	FooterMagic   = "RCLONE/EC"
	FooterVersion = 2
	FooterSize    = 98
)

// AlgorithmRS is the footer algorithm tag for Reed-Solomon encoding.
var (
	AlgorithmRS = [4]byte{'R', 'S', 0, 0}
)

// CompressionNone is the footer compression tag for uncompressed payloads.
var (
	CompressionNone = [4]byte{0, 0, 0, 0}
)

var emptyFileMD5 [16]byte
var emptyFileSHA256 [32]byte

func init() {
	_, _ = hex.Decode(emptyFileMD5[:], []byte("d41d8cd98f00b204e9800998ecf8427e"))
	_, _ = hex.Decode(emptyFileSHA256[:], []byte("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
}

// Footer holds metadata from a single RS/EC shard particle.
type Footer struct {
	ContentLength int64
	MD5           [16]byte
	SHA256        [32]byte
	Mtime         int64
	Compression   [4]byte
	NumBlocks     uint32
	Algorithm     [4]byte
	DataShards    uint8
	ParityShards  uint8
	CurrentShard  uint8
	StripeSize    uint32
	PayloadCRC32C uint32
}

// MarshalBinary encodes the footer to its on-disk layout.
func (f *Footer) MarshalBinary() ([]byte, error) {
	b := make([]byte, FooterSize)
	copy(b[0:9], FooterMagic)
	binary.LittleEndian.PutUint16(b[9:11], FooterVersion)
	binary.LittleEndian.PutUint64(b[11:19], uint64(f.ContentLength))
	copy(b[19:35], f.MD5[:])
	copy(b[35:67], f.SHA256[:])
	binary.LittleEndian.PutUint64(b[67:75], uint64(f.Mtime))
	copy(b[75:79], f.Compression[:])
	binary.LittleEndian.PutUint32(b[79:83], f.NumBlocks)
	copy(b[83:87], f.Algorithm[:])
	b[87] = f.DataShards
	b[88] = f.ParityShards
	b[89] = f.CurrentShard
	binary.LittleEndian.PutUint32(b[90:94], f.StripeSize)
	binary.LittleEndian.PutUint32(b[94:98], f.PayloadCRC32C)
	return b, nil
}

// ParseFooter decodes a footer from its binary layout.
func ParseFooter(buf []byte) (*Footer, error) {
	if len(buf) != FooterSize {
		return nil, fmt.Errorf("footer: buffer length must be %d", FooterSize)
	}
	if string(buf[0:9]) != FooterMagic {
		return nil, errors.New("footer: invalid magic")
	}
	if binary.LittleEndian.Uint16(buf[9:11]) != FooterVersion {
		return nil, errors.New("footer: unsupported version")
	}
	f := &Footer{}
	f.ContentLength = int64(binary.LittleEndian.Uint64(buf[11:19]))
	copy(f.MD5[:], buf[19:35])
	copy(f.SHA256[:], buf[35:67])
	f.Mtime = int64(binary.LittleEndian.Uint64(buf[67:75]))
	copy(f.Compression[:], buf[75:79])
	f.NumBlocks = binary.LittleEndian.Uint32(buf[79:83])
	copy(f.Algorithm[:], buf[83:87])
	f.DataShards = buf[87]
	f.ParityShards = buf[88]
	f.CurrentShard = buf[89]
	f.StripeSize = binary.LittleEndian.Uint32(buf[90:94])
	f.PayloadCRC32C = binary.LittleEndian.Uint32(buf[94:98])
	return f, nil
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func crc32cChecksum(data []byte) uint32 {
	return crc32.Checksum(data, crc32cTable)
}

// CRC32C returns the Castagnoli CRC32 of data (same as EC footer PayloadCRC32C).
func CRC32C(data []byte) uint32 {
	return crc32cChecksum(data)
}

// NewRSFooter builds a Footer for shard shardIndex with the given logical object metadata.
func NewRSFooter(contentLength int64, md5Sum, sha256Sum []byte, mtime time.Time, dataShards, parityShards, shardIndex int, stripeSize uint32, payloadCRC uint32) *Footer {
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
		Mtime:         mtime.Unix(),
		Compression:   CompressionNone,
		NumBlocks:     0,
		Algorithm:     AlgorithmRS,
		DataShards:    uint8(dataShards),
		ParityShards:  uint8(parityShards),
		CurrentShard:  uint8(shardIndex),
		StripeSize:    stripeSize,
		PayloadCRC32C: payloadCRC,
	}
}
