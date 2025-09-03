package compress

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/hash"
)

// zstdModeHandler implements compressionModeHandler for zstd
type zstdModeHandler struct{}

// isCompressible checks the compression ratio of the provided data and returns true if the ratio exceeds
// the configured threshold
func (z *zstdModeHandler) isCompressible(r io.Reader, compressionMode int) (bool, error) {
	var b bytes.Buffer
	var n int64
	w, err := NewWriterSzstd(&b, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return false, err
	}
	n, err = io.Copy(w, r)
	if err != nil {
		return false, err
	}
	err = w.Close()
	if err != nil {
		return false, err
	}
	ratio := float64(n) / float64(b.Len())
	return ratio > minCompressionRatio, nil
}

// newObjectGetOriginalSize returns the original file size from the metadata
func (z *zstdModeHandler) newObjectGetOriginalSize(meta *ObjectMetadata) (int64, error) {
	if meta.CompressionMetadataZstd == nil {
		return 0, errors.New("missing zstd metadata")
	}
	return meta.CompressionMetadataZstd.Size, nil
}

// openGetReadCloser opens a compressed object and returns a ReadCloser in the Open method
func (z *zstdModeHandler) openGetReadCloser(
	ctx context.Context,
	o *Object,
	offset int64,
	limit int64,
	cr chunkedreader.ChunkedReader,
	closer io.Closer,
	options ...fs.OpenOption,
) (rc io.ReadCloser, err error) {
	var file io.Reader

	if offset != 0 {
		file, err = NewReaderAtSzstd(cr, o.meta.CompressionMetadataZstd, offset)
	} else {
		file, err = zstd.NewReader(cr)
	}
	if err != nil {
		return nil, err
	}

	var fileReader io.Reader
	if limit != -1 {
		fileReader = io.LimitReader(file, limit)
	} else {
		fileReader = file
	}
	// Return a ReadCloser
	return ReadCloserWrapper{Reader: fileReader, Closer: closer}, nil
}

// processFileNameGetFileExtension returns the file extension for the given compression mode
func (z *zstdModeHandler) processFileNameGetFileExtension(compressionMode int) string {
	if compressionMode == Zstd {
		return zstdFileExt
	}

	return ""
}

// putCompress compresses the input data and uploads it to the remote, returning the new object and its metadata
func (z *zstdModeHandler) putCompress(
	ctx context.Context,
	f *Fs,
	in io.Reader,
	src fs.ObjectInfo,
	options []fs.OpenOption,
	mimeType string,
) (fs.Object, *ObjectMetadata, error) {
	// Unwrap reader accounting
	in, wrap := accounting.UnWrap(in)

	// Add the metadata hasher
	metaHasher := md5.New()
	in = io.TeeReader(in, metaHasher)

	// Compress the file
	pipeReader, pipeWriter := io.Pipe()

	resultsZstd := make(chan compressionResult[SzstdMetadata])
	go func() {
		writer, err := NewWriterSzstd(pipeWriter, zstd.WithEncoderLevel(zstd.EncoderLevel(f.opt.CompressionLevel)))
		if err != nil {
			resultsZstd <- compressionResult[SzstdMetadata]{err: err}
			close(resultsZstd)
			return
		}
		_, err = io.Copy(writer, in)
		if wErr := writer.Close(); wErr != nil && err == nil {
			err = wErr
		}
		if cErr := pipeWriter.Close(); cErr != nil && err == nil {
			err = cErr
		}

		resultsZstd <- compressionResult[SzstdMetadata]{err: err, meta: writer.GetMetadata()}
		close(resultsZstd)
	}()

	wrappedIn := wrap(bufio.NewReaderSize(pipeReader, bufferSize))

	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	var err error
	if ht != hash.None {
		wrappedIn, wrap = accounting.UnWrap(wrappedIn)
		hasher, err = hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, nil, err
		}
		wrappedIn = io.TeeReader(wrappedIn, hasher)
		wrappedIn = wrap(wrappedIn)
	}

	o, err := f.rcat(ctx, makeDataName(src.Remote(), src.Size(), f.mode), io.NopCloser(wrappedIn), src.ModTime(ctx), options)
	if err != nil {
		return nil, nil, err
	}

	result := <-resultsZstd
	if result.err != nil {
		if o != nil {
			_ = o.Remove(ctx)
		}
		return nil, nil, result.err
	}

	// Build metadata using uncompressed size for filename
	meta := z.newMetadata(result.meta.Size, f.mode, result.meta, hex.EncodeToString(metaHasher.Sum(nil)), mimeType)
	if ht != hash.None && hasher != nil {
		err = f.verifyObjectHash(ctx, o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}
	return o, meta, nil
}

// putUncompressGetNewMetadata returns metadata in the putUncompress method for a specific compression algorithm
func (z *zstdModeHandler) putUncompressGetNewMetadata(o fs.Object, mode int, md5 string, mimeType string, sum []byte) (fs.Object, *ObjectMetadata, error) {
	return o, z.newMetadata(o.Size(), mode, SzstdMetadata{}, hex.EncodeToString(sum), mimeType), nil
}

// This function generates a metadata object for sgzip.GzipMetadata or SzstdMetadata.
// Warning: This function panics if cmeta is not of the expected type.
func (z *zstdModeHandler) newMetadata(size int64, mode int, cmeta any, md5 string, mimeType string) *ObjectMetadata {
	meta, ok := cmeta.(SzstdMetadata)
	if !ok {
		panic("invalid cmeta type: expected SzstdMetadata")
	}

	objMeta := new(ObjectMetadata)
	objMeta.Size = size
	objMeta.Mode = mode
	objMeta.CompressionMetadataGzip = nil
	objMeta.CompressionMetadataZstd = &meta
	objMeta.MD5 = md5
	objMeta.MimeType = mimeType

	return objMeta
}
