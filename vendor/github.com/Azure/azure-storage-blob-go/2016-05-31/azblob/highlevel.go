package azblob

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"

	"bytes"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// CommonResponseHeaders returns the headers common to all blob REST API responses.
type CommonResponse interface {
	// ETag returns the value for header ETag.
	ETag() ETag

	// LastModified returns the value for header Last-Modified.
	LastModified() time.Time

	// RequestID returns the value for header x-ms-request-id.
	RequestID() string

	// Date returns the value for header Date.
	Date() time.Time

	// Version returns the value for header x-ms-version.
	Version() string

	// Response returns the raw HTTP response object.
	Response() *http.Response
}

// UploadToBlockBlobOptions identifies options used by the UploadBufferToBlockBlob and UploadFileToBlockBlob functions.
type UploadToBlockBlobOptions struct {
	// BlockSize specifies the block size to use; the default (and maximum size) is BlockBlobMaxPutBlockBytes.
	BlockSize uint64

	// Progress is a function that is invoked periodically as bytes are send in a PutBlock call to the BlockBlobURL.
	Progress pipeline.ProgressReceiver

	// BlobHTTPHeaders indicates the HTTP headers to be associated with the blob when PutBlockList is called.
	BlobHTTPHeaders BlobHTTPHeaders

	// Metadata indicates the metadata to be associated with the blob when PutBlockList is called.
	Metadata Metadata

	// AccessConditions indicates the access conditions for the block blob.
	AccessConditions BlobAccessConditions

	// Parallelism indicates the maximum number of blocks to upload in parallel (0=default)
	Parallelism uint16
}

// UploadBufferToBlockBlob uploads a buffer in blocks to a block blob.
func UploadBufferToBlockBlob(ctx context.Context, b []byte,
	blockBlobURL BlockBlobURL, o UploadToBlockBlobOptions) (CommonResponse, error) {

	if o.BlockSize < 0 || o.BlockSize > BlockBlobMaxPutBlockBytes {
		panic(fmt.Sprintf("BlockSize option must be > 0 and <= %d", BlockBlobMaxPutBlockBytes))
	}
	if o.BlockSize == 0 {
		o.BlockSize = BlockBlobMaxPutBlockBytes // Default if unspecified
	}
	size := uint64(len(b))

	if size <= BlockBlobMaxPutBlobBytes {
		// If the size can fit in 1 Put Blob call, do it this way
		var body io.ReadSeeker = bytes.NewReader(b)
		if o.Progress != nil {
			body = pipeline.NewRequestBodyProgress(body, o.Progress)
		}
		return blockBlobURL.PutBlob(ctx, body, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions)
	}

	parallelism := o.Parallelism
	if parallelism == 0 {
		parallelism = 5 // default parallelism
	}

	var numBlocks uint16 = uint16(((size - 1) / o.BlockSize) + 1)
	if numBlocks > BlockBlobMaxBlocks {
		panic(fmt.Sprintf("The streamSize is too big or the BlockSize is too small; the number of blocks must be <= %d", BlockBlobMaxBlocks))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	blockIDList := make([]string, numBlocks) // Base 64 encoded block IDs
	blockSize := o.BlockSize

	putBlockChannel := make(chan func() (*BlockBlobsPutBlockResponse, error), parallelism) // Create the channel that release 'parallelism' goroutines concurrently
	putBlockResponseChannel := make(chan error, numBlocks)                                 // Holds each Put Block's response

	// Create the goroutines that process each Put Block (in parallel)
	for g := uint16(0); g < parallelism; g++ {
		go func() {
			for f := range putBlockChannel {
				_, err := f()
				putBlockResponseChannel <- err
			}
		}()
	}

	blobProgress := int64(0)
	progressLock := &sync.Mutex{}

	// Add each put block to the channel
	for blockNum := uint16(0); blockNum < numBlocks; blockNum++ {
		if blockNum == numBlocks-1 { // Last block
			blockSize = size - (uint64(blockNum) * o.BlockSize) // Remove size of all uploaded blocks from total
		}
		offset := uint64(blockNum) * o.BlockSize

		// Prepare to read the proper block/section of the buffer
		var body io.ReadSeeker = bytes.NewReader(b[offset : offset+blockSize])
		capturedBlockNum := blockNum
		if o.Progress != nil {
			blockProgress := int64(0)
			body = pipeline.NewRequestBodyProgress(body,
				func(bytesTransferred int64) {
					diff := bytesTransferred - blockProgress
					blockProgress = bytesTransferred
					progressLock.Lock()
					blobProgress += diff
					o.Progress(blobProgress)
					progressLock.Unlock()
				})
		}

		// Block IDs are unique values to avoid issue if 2+ clients are uploading blocks
		// at the same time causing PutBlockList to get a mix of blocks from all the clients.
		blockIDList[blockNum] = base64.StdEncoding.EncodeToString(newUUID().bytes())
		putBlockChannel <- func() (*BlockBlobsPutBlockResponse, error) {
			return blockBlobURL.PutBlock(ctx, blockIDList[capturedBlockNum], body, o.AccessConditions.LeaseAccessConditions)
		}
	}
	close(putBlockChannel)

	// Wait for the put blocks to complete
	for blockNum := uint16(0); blockNum < numBlocks; blockNum++ {
		responseError := <-putBlockResponseChannel
		if responseError != nil {
			cancel()                  // As soon as any Put Block fails, cancel all remaining Put Block calls
			return nil, responseError // No need to process anymore responses
		}
	}
	// All put blocks were successful, call Put Block List to finalize the blob
	return blockBlobURL.PutBlockList(ctx, blockIDList, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions)
}

// UploadFileToBlockBlob uploads a file in blocks to a block blob.
func UploadFileToBlockBlob(ctx context.Context, file *os.File,
	blockBlobURL BlockBlobURL, o UploadToBlockBlobOptions) (CommonResponse, error) {

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	m := mmf{} // Default to an empty slice; used for 0-size file
	if stat.Size() != 0 {
		m, err = newMMF(file, false, 0, int(stat.Size()))
		if err != nil {
			return nil, err
		}
		defer m.unmap()
	}
	return UploadBufferToBlockBlob(ctx, m, blockBlobURL, o)
}

// DownloadStreamOptions is used to configure a call to NewDownloadBlobToStream to download a large stream with intelligent retries.
type DownloadStreamOptions struct {
	// Range indicates the starting offset and count of bytes within the blob to download.
	Range BlobRange

	// AccessConditions indicates the BlobAccessConditions to use when accessing the blob.
	AccessConditions BlobAccessConditions
}

type retryStream struct {
	ctx      context.Context
	getBlob  func(ctx context.Context, blobRange BlobRange, ac BlobAccessConditions, rangeGetContentMD5 bool) (*GetResponse, error)
	o        DownloadStreamOptions
	response *http.Response
}

// NewDownloadStream creates a stream over a blob allowing you download the blob's contents.
// When network errors occur, the retry stream internally issues new HTTP GET requests for
// the remaining range of the blob's contents. The GetBlob argument identifies the function
// to invoke when the GetRetryStream needs to make an HTTP GET request as Read methods are called.
// The callback can wrap the response body (with progress reporting, for example) before returning.
func NewDownloadStream(ctx context.Context,
	getBlob func(ctx context.Context, blobRange BlobRange, ac BlobAccessConditions, rangeGetContentMD5 bool) (*GetResponse, error),
	o DownloadStreamOptions) io.ReadCloser {

	// BlobAccessConditions may already have an If-Match:etag header
	if getBlob == nil {
		panic("getBlob must not be nil")
	}
	return &retryStream{ctx: ctx, getBlob: getBlob, o: o, response: nil}
}

func (s *retryStream) Read(p []byte) (n int, err error) {
	for {
		if s.response != nil { // We working with a successful response
			n, err := s.response.Body.Read(p) // Read from the stream
			if err == nil || err == io.EOF {  // We successfully read data or end EOF
				s.o.Range.Offset += int64(n) // Increments the start offset in case we need to make a new HTTP request in the future
				if s.o.Range.Count != 0 {
					s.o.Range.Count -= int64(n) // Decrement the count in case we need to make a new HTTP request in the future
				}
				return n, err // Return the return to the caller
			}
			s.Close()
			s.response = nil // Something went wrong; our stream is no longer good
			if nerr, ok := err.(net.Error); ok {
				if !nerr.Timeout() && !nerr.Temporary() {
					return n, err // Not retryable
				}
			} else {
				return n, err // Not retryable, just return
			}
		}

		// We don't have a response stream to read from, try to get one
		response, err := s.getBlob(s.ctx, s.o.Range, s.o.AccessConditions, false)
		if err != nil {
			return 0, err
		}
		// Successful GET; this is the network stream we'll read from
		s.response = response.Response()

		// Ensure that future requests are from the same version of the source
		s.o.AccessConditions.IfMatch = response.ETag()

		// Loop around and try to read from this stream
	}
}

func (s *retryStream) Close() error {
	if s.response != nil && s.response.Body != nil {
		return s.response.Body.Close()
	}
	return nil
}
