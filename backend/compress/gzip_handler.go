package compress

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"

	"github.com/buengese/sgzip"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/hash"
)

// gzipModeHandler implements compressionModeHandler for gzip
type gzipModeHandler struct{}

// isCompressible checks the compression ratio of the provided data and returns true if the ratio exceeds
// the configured threshold
func (g *gzipModeHandler) isCompressible(r io.Reader, compressionMode int) (bool, error) {
	var b bytes.Buffer
	var n int64
	w, err := sgzip.NewWriterLevel(&b, sgzip.DefaultCompression)
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
func (g *gzipModeHandler) newObjectGetOriginalSize(meta *ObjectMetadata) (int64, error) {
	if meta.CompressionMetadataGzip == nil {
		return 0, errors.New("missing gzip metadata")
	}
	return meta.CompressionMetadataGzip.Size, nil
}

// openGetReadCloser opens a compressed object and returns a ReadCloser in the Open method
func (g *gzipModeHandler) openGetReadCloser(
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
		file, err = sgzip.NewReaderAt(cr, o.meta.CompressionMetadataGzip, offset)
	} else {
		file, err = sgzip.NewReader(cr)
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
func (g *gzipModeHandler) processFileNameGetFileExtension(compressionMode int) string {
	if compressionMode == Gzip {
		return gzFileExt
	}

	return ""
}

// putCompress compresses the input data and uploads it to the remote, returning the new object and its metadata
func (g *gzipModeHandler) putCompress(
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

	resultsGzip := make(chan compressionResult[sgzip.GzipMetadata])
	go func() {
		gz, err := sgzip.NewWriterLevel(pipeWriter, f.opt.CompressionLevel)
		if err != nil {
			resultsGzip <- compressionResult[sgzip.GzipMetadata]{err: err, meta: sgzip.GzipMetadata{}}
			close(resultsGzip)
			return
		}
		_, err = io.Copy(gz, in)
		gzErr := gz.Close()
		if gzErr != nil && err == nil {
			err = gzErr
		}
		closeErr := pipeWriter.Close()
		if closeErr != nil && err == nil {
			err = closeErr
		}
		resultsGzip <- compressionResult[sgzip.GzipMetadata]{err: err, meta: gz.MetaData()}
		close(resultsGzip)
	}()

	wrappedIn := wrap(bufio.NewReaderSize(pipeReader, bufferSize)) // Probably no longer needed as sgzip has it's own buffering

	// Find a hash the destination supports to compute a hash of
	// the compressed data.
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	var err error
	if ht != hash.None {
		// unwrap the accounting again
		wrappedIn, wrap = accounting.UnWrap(wrappedIn)
		hasher, err = hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, nil, err
		}
		// add the hasher and re-wrap the accounting
		wrappedIn = io.TeeReader(wrappedIn, hasher)
		wrappedIn = wrap(wrappedIn)
	}

	// Transfer the data
	o, err := f.rcat(ctx, makeDataName(src.Remote(), src.Size(), f.mode), io.NopCloser(wrappedIn), src.ModTime(ctx), options)
	if err != nil {
		if o != nil {
			if removeErr := o.Remove(ctx); removeErr != nil {
				fs.Errorf(o, "Failed to remove partially transferred object: %v", removeErr)
			}
		}
		return nil, nil, err
	}
	// Check whether we got an error during compression
	result := <-resultsGzip
	if result.err != nil {
		if o != nil {
			if removeErr := o.Remove(ctx); removeErr != nil {
				fs.Errorf(o, "Failed to remove partially compressed object: %v", removeErr)
			}
		}
		return nil, nil, result.err
	}

	// Generate metadata
	meta := g.newMetadata(result.meta.Size, f.mode, result.meta, hex.EncodeToString(metaHasher.Sum(nil)), mimeType)

	// Check the hashes of the compressed data if we were comparing them
	if ht != hash.None && hasher != nil {
		err = f.verifyObjectHash(ctx, o, hasher, ht)
		if err != nil {
			return nil, nil, err
		}
	}
	return o, meta, nil
}

// putUncompressGetNewMetadata returns metadata in the putUncompress method for a specific compression algorithm
func (g *gzipModeHandler) putUncompressGetNewMetadata(o fs.Object, mode int, md5 string, mimeType string, sum []byte) (fs.Object, *ObjectMetadata, error) {
	return o, g.newMetadata(o.Size(), mode, sgzip.GzipMetadata{}, hex.EncodeToString(sum), mimeType), nil
}

// This function generates a metadata object for sgzip.GzipMetadata or SzstdMetadata.
// Warning: This function panics if cmeta is not of the expected type.
func (g *gzipModeHandler) newMetadata(size int64, mode int, cmeta any, md5 string, mimeType string) *ObjectMetadata {
	meta, ok := cmeta.(sgzip.GzipMetadata)
	if !ok {
		panic("invalid cmeta type: expected sgzip.GzipMetadata")
	}

	objMeta := new(ObjectMetadata)
	objMeta.Size = size
	objMeta.Mode = mode
	objMeta.CompressionMetadataGzip = &meta
	objMeta.CompressionMetadataZstd = nil
	objMeta.MD5 = md5
	objMeta.MimeType = mimeType

	return objMeta
}
