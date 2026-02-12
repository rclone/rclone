// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"
)

// EC footer constants (90-byte footer at tail of each particle)
const (
	FooterMagic   = "RCLONE/EC" // 9 bytes
	FooterVersion = 1
	FooterSize    = 90
)

// Algorithm names (4 bytes ASCII, null-padded)
var (
	AlgorithmR3 = [4]byte{'R', '3', 0, 0}
	AlgorithmRS = [4]byte{'R', 'S', 0, 0}
)

// Compression (4 bytes ASCII, null-padded)
var (
	CompressionNone   = [4]byte{0, 0, 0, 0}
	CompressionLZ4    = [4]byte{'L', 'Z', '4', ' '}
	CompressionSnappy = [4]byte{'s', 'z', ' ', ' '}
)

// Shard indices for RAID3 (0=even, 1=odd, 2=parity)
const (
	ShardEven   = 0
	ShardOdd    = 1
	ShardParity = 2
)

// Standard hashes for zero-length content (used in footer when no hashes provided).
var emptyFileMD5 [16]byte
var emptyFileSHA256 [32]byte

func init() {
	hex.Decode(emptyFileMD5[:], []byte("d41d8cd98f00b204e9800998ecf8427e"))
	hex.Decode(emptyFileSHA256[:], []byte("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
}

// Footer holds the 90-byte EC footer stored at the end of each particle.
// Layout: Magic 9, Version 2, ContentLength 8, MD5 16, SHA256 32, Mtime 8, Compression 4, Algorithm 4, DataShards 1, ParityShards 1, CurrentShard 1, Reserved 4.
type Footer struct {
	ContentLength int64
	MD5           [16]byte
	SHA256        [32]byte
	Mtime         int64
	Compression   [4]byte
	Algorithm     [4]byte
	DataShards    uint8
	ParityShards  uint8
	CurrentShard  uint8
	Reserved      [4]byte
}

// MarshalBinary encodes the footer to exactly FooterSize bytes (little-endian).
func (f *Footer) MarshalBinary() ([]byte, error) {
	b := make([]byte, FooterSize)
	copy(b[0:9], FooterMagic)
	binary.LittleEndian.PutUint16(b[9:11], FooterVersion)
	binary.LittleEndian.PutUint64(b[11:19], uint64(f.ContentLength))
	copy(b[19:35], f.MD5[:])
	copy(b[35:67], f.SHA256[:])
	binary.LittleEndian.PutUint64(b[67:75], uint64(f.Mtime))
	copy(b[75:79], f.Compression[:])
	copy(b[79:83], f.Algorithm[:])
	b[83] = f.DataShards
	b[84] = f.ParityShards
	b[85] = f.CurrentShard
	copy(b[86:90], f.Reserved[:])
	return b, nil
}

// ParseFooter parses a 90-byte buffer into a Footer. Returns error if len(buf) != 90 or magic/version mismatch.
func ParseFooter(buf []byte) (*Footer, error) {
	if len(buf) != FooterSize {
		return nil, errors.New("footer: buffer length must be 90")
	}
	if string(buf[0:9]) != FooterMagic {
		return nil, errors.New("footer: invalid magic")
	}
	version := binary.LittleEndian.Uint16(buf[9:11])
	if version != FooterVersion {
		return nil, errors.New("footer: unsupported version")
	}
	f := &Footer{}
	f.ContentLength = int64(binary.LittleEndian.Uint64(buf[11:19]))
	copy(f.MD5[:], buf[19:35])
	copy(f.SHA256[:], buf[35:67])
	f.Mtime = int64(binary.LittleEndian.Uint64(buf[67:75]))
	copy(f.Compression[:], buf[75:79])
	copy(f.Algorithm[:], buf[79:83])
	f.DataShards = buf[83]
	f.ParityShards = buf[84]
	f.CurrentShard = buf[85]
	copy(f.Reserved[:], buf[86:90])
	return f, nil
}

// FooterFromReconstructed builds a footer for RAID3 (data shards 2, parity 1, algorithm R3).
// For zero-length content with nil hashes, the standard empty-file MD5 and SHA256 are used.
func FooterFromReconstructed(contentLength int64, md5, sha256 []byte, mtime time.Time, compression [4]byte, currentShard int) *Footer {
	var md5Arr [16]byte
	var sha256Arr [32]byte
	if len(md5) >= 16 {
		copy(md5Arr[:], md5[:16])
	} else if contentLength == 0 {
		md5Arr = emptyFileMD5
	}
	if len(sha256) >= 32 {
		copy(sha256Arr[:], sha256[:32])
	} else if contentLength == 0 {
		sha256Arr = emptyFileSHA256
	}
	return &Footer{
		ContentLength: contentLength,
		MD5:           md5Arr,
		SHA256:         sha256Arr,
		Mtime:          mtime.Unix(),
		Compression:    compression,
		Algorithm:      AlgorithmR3,
		DataShards:     2,
		ParityShards:   1,
		CurrentShard:   uint8(currentShard),
		Reserved:       [4]byte{0, 0, 0, 0},
	}
}
