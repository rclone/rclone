// Package raid3: compression helpers for write/read path (config mapping, compress, decompress).
package raid3

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

// ConfigToFooterCompression maps config string to footer compression bytes.
// Returns error if compression is not "none", "snappy", or "zstd".
func ConfigToFooterCompression(compression string) ([4]byte, error) {
	switch compression {
	case "none", "":
		return CompressionNone, nil
	case "snappy":
		return CompressionSnappy, nil
	case "zstd":
		return CompressionZstd, nil
	default:
		return [4]byte{}, fmt.Errorf("raid3: invalid compression %q: only none, snappy, and zstd are supported", compression)
	}
}

// errUnsupportedCompression is returned when the footer indicates an unsupported compression (e.g. LZ4).
var errUnsupportedCompression = errors.New("raid3: unsupported compression in object footer")

// zstdDefaultLevel is the default zstd encoder level (good balance of speed and ratio).
var zstdDefaultLevel = zstd.SpeedDefault

// newCompressingReader returns a reader that compresses data from src using the given algorithm.
// For "none" returns src unchanged. For "snappy" or "zstd" returns a reader that streams compressed output.
// The hasher (or other caller) remains the source of truth for uncompressed size and hashes.
func newCompressingReader(src io.Reader, algo string) (io.Reader, error) {
	if algo == "none" || algo == "" {
		return src, nil
	}
	pr, pw := io.Pipe()
	switch algo {
	case "snappy":
		go func() {
			sw := snappy.NewWriter(pw)
			_, err := io.Copy(sw, src)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if err := sw.Close(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()
		return pr, nil
	case "zstd":
		go func() {
			enc, err := zstd.NewWriter(pw, zstd.WithEncoderLevel(zstdDefaultLevel))
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_, err = io.Copy(enc, src)
			if err != nil {
				_ = enc.Close()
				_ = pw.CloseWithError(err)
				return
			}
			if err := enc.Close(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()
		return pr, nil
	default:
		_ = pw.Close()
		return nil, fmt.Errorf("raid3: unsupported compression algorithm %q", algo)
	}
}

// decompressReadCloser wraps a snappy.Reader and closes the underlying ReadCloser on Close.
type decompressReadCloser struct {
	*snappy.Reader
	src io.Closer
}

func (d *decompressReadCloser) Close() error {
	return d.src.Close()
}

// zstdDecompressReadCloser wraps a zstd Decoder and closes both the decoder and the source on Close.
type zstdDecompressReadCloser struct {
	dec *zstd.Decoder
	src io.Closer
}

func (z *zstdDecompressReadCloser) Read(p []byte) (n int, err error) {
	return z.dec.Read(p)
}

func (z *zstdDecompressReadCloser) Close() error {
	z.dec.Close()
	return z.src.Close()
}

// newDecompressingReadCloser returns a ReadCloser that decompresses data from rc using the footer's compression.
// If compression is CompressionNone, returns rc unchanged. If CompressionSnappy or CompressionZstd, wraps with the appropriate decoder.
// For any other value (e.g. LZ4) returns errUnsupportedCompression.
func newDecompressingReadCloser(rc io.ReadCloser, compression [4]byte) (io.ReadCloser, error) {
	if compression == CompressionNone {
		return rc, nil
	}
	if compression == CompressionSnappy {
		return &decompressReadCloser{Reader: snappy.NewReader(rc), src: rc}, nil
	}
	if compression == CompressionZstd {
		dec, err := zstd.NewReader(rc)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		return &zstdDecompressReadCloser{dec: dec, src: rc}, nil
	}
	_ = rc.Close()
	return nil, errUnsupportedCompression
}
