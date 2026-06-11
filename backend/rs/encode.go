package rs

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"time"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
)

// DefaultStripeFragmentSize is S: bytes per shard per RS stripe (fragment size).
// Erasure-coded systems commonly use 64KiB–1MiB unit sizes (e.g. Tahoe-LAFS default
// 128KiB segments; HDFS EC often 64KiB+). 256KiB balances memory (k·S per stripe),
// reconstruct work, and remote API round-trips.
const DefaultStripeFragmentSize = 256 * 1024

// BuildResult is the outcome of BuildRSShardsToWriters (content hashes and length).
type BuildResult struct {
	ContentLength int64
	Mtime         time.Time
	MD5           [16]byte
	SHA256        [32]byte
	StripeSize    uint32
	NumStripes    uint32
}

// NumStripesForContent returns the stripe count for a logical size, k, and fragment size S.
// For non-empty content it is ceil(ContentLength / (k*S)). Empty content yields 0.
func NumStripesForContent(contentLength int64, k, S int) int {
	if contentLength == 0 || k < 1 || S < 1 {
		return 0
	}
	capacity := int64(k) * int64(S)
	return int((contentLength + capacity - 1) / capacity)
}

func normalizeStripeFragmentSize(n int) int {
	if n <= 0 {
		return DefaultStripeFragmentSize
	}
	return n
}

// ShardPayloadByteLength returns parity-shard payload size (NumStripes × S) for the given logical size.
func ShardPayloadByteLength(contentLength int64, k, stripeFragmentSize int) int64 {
	S := normalizeStripeFragmentSize(stripeFragmentSize)
	return ParityShardPayloadLen(contentLength, k, S)
}

// ShardParticleFileSize returns the on-wire particle size for data shard 0 (largest data shard when uniform).
// Prefer ExpectedParticleSize per shard index for virtual-padding layout.
func ShardParticleFileSize(contentLength int64, k, stripeFragmentSize int, withFooter bool) int64 {
	return ExpectedParticleSize(contentLength, 0, k, 0, stripeFragmentSize, withFooter)
}

// encodeLogicalToShardWriters streams logical bytes from in, RS-encodes stripe-by-stripe into writers,
// and appends footers. Used by BuildRSShardsToWriters (spooling Put) and intended for use_spooling=false.
func encodeLogicalToShardWriters(ctx context.Context, in io.Reader, src fs.ObjectInfo, dataShards, parityShards int, stripeFragmentSize int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	if len(writers) != dataShards+parityShards {
		return nil, fmt.Errorf("rs: writers count mismatch: got %d want %d", len(writers), dataShards+parityShards)
	}
	S := normalizeStripeFragmentSize(stripeFragmentSize)
	k := dataShards
	declared := src.Size()

	stripeBuf := make([]byte, k*S)
	mtime := src.ModTime(ctx).Truncate(time.Second)

	md5h := md5.New()
	sha256h := sha256.New()
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("rs: create encoder: %w", err)
	}
	shards := make([][]byte, k+parityShards)
	crcH := make([]hash.Hash32, len(writers))
	for i := range crcH {
		crcH[i] = crc32.New(crc32cTable)
	}

	var contentLength int64
	stripeIndex := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		clear(stripeBuf)
		n, readErr := io.ReadFull(in, stripeBuf)
		if stripeIndex == 0 && n == 0 && (readErr == io.EOF || readErr == io.ErrUnexpectedEOF) {
			if declared >= 0 && declared != 0 {
				return nil, fmt.Errorf("incorrect upload size %d != %d", 0, declared)
			}
			return buildZeroLengthShards(ctx, src, dataShards, parityShards, writers, withFooter)
		}
		if readErr == io.ErrUnexpectedEOF {
			readErr = nil
		} else if readErr == io.EOF && n == 0 {
			break
		} else if readErr != nil {
			return nil, fmt.Errorf("rs: read source: %w", readErr)
		}
		if n == 0 {
			break
		}

		contentLength += int64(n)
		if declared >= 0 && contentLength > declared {
			return nil, fmt.Errorf("incorrect upload size %d != %d", contentLength, declared)
		}

		if _, err := md5h.Write(stripeBuf[:n]); err != nil {
			return nil, err
		}
		if _, err := sha256h.Write(stripeBuf[:n]); err != nil {
			return nil, err
		}

		dataPieces, err := enc.Split(stripeBuf)
		if err != nil {
			return nil, fmt.Errorf("rs: split stripe %d: %w", stripeIndex, err)
		}
		for i := range shards {
			shards[i] = nil
		}
		copy(shards, dataPieces)
		for i := dataShards; i < len(shards); i++ {
			shards[i] = make([]byte, len(shards[0]))
		}
		if err := enc.Encode(shards); err != nil {
			return nil, fmt.Errorf("rs: encode stripe %d: %w", stripeIndex, err)
		}
		stripeLogical := n
		for i := range writers {
			var frag []byte
			if i < k {
				flen := DataShardFragLen(i, k, S, stripeLogical)
				frag = shards[i][:flen]
			} else {
				frag = shards[i]
			}
			if _, err := writers[i].Write(frag); err != nil {
				return nil, fmt.Errorf("rs: write shard %d stripe %d: %w", i, stripeIndex, err)
			}
			_, _ = crcH[i].Write(frag)
		}

		stripeIndex++
	}

	if declared >= 0 && contentLength != declared {
		return nil, fmt.Errorf("incorrect upload size %d != %d", contentLength, declared)
	}

	numStripes := NumStripesForContent(contentLength, k, S)
	if numStripes != stripeIndex {
		return nil, fmt.Errorf("rs: internal error: stripe count %d != expected %d", stripeIndex, numStripes)
	}

	var md5Arr [16]byte
	var sha256Arr [32]byte
	copy(md5Arr[:], md5h.Sum(nil))
	copy(sha256Arr[:], sha256h.Sum(nil))

	stripeU32 := uint32(S)
	numStripesU32 := uint32(numStripes)
	for i := range writers {
		if withFooter {
			ft := NewRSFooter(contentLength, md5Arr[:], sha256Arr[:], mtime, dataShards, parityShards, i, stripeU32, numStripesU32, crcH[i].Sum32())
			fb, err := ft.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := writers[i].Write(fb); err != nil {
				return nil, fmt.Errorf("rs: write shard footer %d: %w", i, err)
			}
		}
	}

	return &BuildResult{
		ContentLength: contentLength,
		Mtime:         mtime,
		MD5:           md5Arr,
		SHA256:        sha256Arr,
		StripeSize:    stripeU32,
		NumStripes:    numStripesU32,
	}, nil
}

// buildZeroLengthShards writes empty logical files (no payload; footer only).
func buildZeroLengthShards(ctx context.Context, src fs.ObjectInfo, dataShards, parityShards int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	if len(writers) != dataShards+parityShards {
		return nil, fmt.Errorf("rs: writers count mismatch: got %d want %d", len(writers), dataShards+parityShards)
	}
	mtime := src.ModTime(ctx).Truncate(time.Second)
	for i := range writers {
		if withFooter {
			ft := NewRSFooter(0, emptyFileMD5[:], emptyFileSHA256[:], mtime, dataShards, parityShards, i, 0, 0, crc32cChecksum(nil))
			fb, err := ft.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := writers[i].Write(fb); err != nil {
				return nil, fmt.Errorf("rs: write shard footer %d: %w", i, err)
			}
		}
	}
	return &BuildResult{
		ContentLength: 0,
		Mtime:         mtime,
		MD5:           emptyFileMD5,
		SHA256:        emptyFileSHA256,
		StripeSize:    0,
		NumStripes:    0,
	}, nil
}

// BuildRSShardsToWriters reads object bytes from in, Reed-Solomon encodes in stripes into writers.
// stripeFragmentSize is S (bytes per shard per stripe); if <= 0, DefaultStripeFragmentSize is used.
// If withFooter is true, each writer receives payload plus EC footer (rclone particle format).
//
// When src.Size() >= 0, the total number of logical bytes read from in must equal src.Size()
// (same idea as chunker). Unknown size (src.Size() < 0) skips that check.
func BuildRSShardsToWriters(ctx context.Context, in io.Reader, src fs.ObjectInfo, dataShards, parityShards int, stripeFragmentSize int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return encodeLogicalToShardWriters(ctx, in, src, dataShards, parityShards, stripeFragmentSize, writers, withFooter)
}
