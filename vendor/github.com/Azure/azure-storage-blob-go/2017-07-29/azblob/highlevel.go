package azblob

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
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
	// BlockSize specifies the block size to use; the default (and maximum size) is BlockBlobMaxStageBlockBytes.
	BlockSize int64

	// Progress is a function that is invoked periodically as bytes are sent to the BlockBlobURL.
	Progress pipeline.ProgressReceiver

	// BlobHTTPHeaders indicates the HTTP headers to be associated with the blob.
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

	// Validate parameters and set defaults
	if o.BlockSize < 0 || o.BlockSize > BlockBlobMaxUploadBlobBytes {
		panic(fmt.Sprintf("BlockSize option must be > 0 and <= %d", BlockBlobMaxUploadBlobBytes))
	}
	if o.BlockSize == 0 {
		o.BlockSize = BlockBlobMaxUploadBlobBytes // Default if unspecified
	}
	size := int64(len(b))

	if size <= BlockBlobMaxUploadBlobBytes {
		// If the size can fit in 1 Upload call, do it this way
		var body io.ReadSeeker = bytes.NewReader(b)
		if o.Progress != nil {
			body = pipeline.NewRequestBodyProgress(body, o.Progress)
		}
		return blockBlobURL.Upload(ctx, body, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions)
	}

	var numBlocks = uint16(((size - 1) / o.BlockSize) + 1)
	if numBlocks > BlockBlobMaxBlocks {
		panic(fmt.Sprintf("The buffer's size is too big or the BlockSize is too small; the number of blocks must be <= %d", BlockBlobMaxBlocks))
	}

	blockIDList := make([]string, numBlocks) // Base-64 encoded block IDs
	progress := int64(0)
	progressLock := &sync.Mutex{}

	err := doBatchTransfer(ctx, batchTransferOptions{
		operationName: "UploadBufferToBlockBlob",
		transferSize:  size,
		chunkSize:     o.BlockSize,
		parallelism:   o.Parallelism,
		operation: func(offset int64, count int64) error {
			// This function is called once per block.
			// It is passed this block's offset within the buffer and its count of bytes
			// Prepare to read the proper block/section of the buffer
			var body io.ReadSeeker = bytes.NewReader(b[offset : offset+count])
			blockNum := offset / o.BlockSize
			if o.Progress != nil {
				blockProgress := int64(0)
				body = pipeline.NewRequestBodyProgress(body,
					func(bytesTransferred int64) {
						diff := bytesTransferred - blockProgress
						blockProgress = bytesTransferred
						progressLock.Lock() // 1 goroutine at a time gets a progress report
						progress += diff
						o.Progress(progress)
						progressLock.Unlock()
					})
			}

			// Block IDs are unique values to avoid issue if 2+ clients are uploading blocks
			// at the same time causing PutBlockList to get a mix of blocks from all the clients.
			blockIDList[blockNum] = base64.StdEncoding.EncodeToString(newUUID().bytes())
			_, err := blockBlobURL.StageBlock(ctx, blockIDList[blockNum], body, o.AccessConditions.LeaseAccessConditions)
			return err
		},
	})
	if err != nil {
		return nil, err
	}
	// All put blocks were successful, call Put Block List to finalize the blob
	return blockBlobURL.CommitBlockList(ctx, blockIDList, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions)
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

///////////////////////////////////////////////////////////////////////////////


const BlobDefaultDownloadBlockSize = int64(4 * 1024 * 1024) // 4MB

// DownloadFromAzureFileOptions identifies options used by the DownloadAzureFileToBuffer and DownloadAzureFileToFile functions.
type DownloadFromBlobOptions struct {
	// BlockSize specifies the block size to use for each parallel download; the default size is BlobDefaultDownloadBlockSize.
	BlockSize int64

	// Progress is a function that is invoked periodically as bytes are received.
	Progress pipeline.ProgressReceiver

	// AccessConditions indicates the access conditions used when making HTTP GET requests against the blob.
	AccessConditions BlobAccessConditions

	// Parallelism indicates the maximum number of blocks to download in parallel (0=default)
	Parallelism uint16

	// RetryReaderOptionsPerBlock is used when downloading each block.
	RetryReaderOptionsPerBlock RetryReaderOptions
}

// downloadAzureFileToBuffer downloads an Azure file to a buffer with parallel.
func downloadBlobToBuffer(ctx context.Context, blobURL BlobURL, offset int64, count int64,
	ac BlobAccessConditions, b []byte, o DownloadFromBlobOptions,
	initialDownloadResponse *DownloadResponse) error {
	// Validate parameters, and set defaults.
	if o.BlockSize < 0 {
		panic("BlockSize option must be >= 0")
	}
	if o.BlockSize == 0 {
		o.BlockSize = BlobDefaultDownloadBlockSize
	}

	if offset < 0 {
		panic("offset option must be >= 0")
	}

	if count < 0 {
		panic("count option must be >= 0")
	}

	if count == CountToEnd { // If size not specified, calculate it
		if initialDownloadResponse != nil {
			count = initialDownloadResponse.ContentLength() - offset // if we have the length, use it
		} else {
			// If we don't have the length at all, get it
			dr, err := blobURL.Download(ctx, 0, CountToEnd, ac, false)
			if err != nil {
				return err
			}
			count = dr.ContentLength() - offset
		}
	}

	if int64(len(b)) < count {
		panic(fmt.Errorf("the buffer's size should be equal to or larger than the request count of bytes: %d", count))
	}

	// Prepare and do parallel download.
	progress := int64(0)
	progressLock := &sync.Mutex{}

	err := doBatchTransfer(ctx, batchTransferOptions{
		operationName: "downloadBlobToBuffer",
		transferSize:    count,
		chunkSize:     o.BlockSize,
		parallelism:   o.Parallelism,
		operation: func(chunkStart int64, count int64) error {
			dr, err := blobURL.Download(ctx, chunkStart+ offset, count, ac, false)
			body := dr.Body(o.RetryReaderOptionsPerBlock)
			if o.Progress != nil {
				rangeProgress := int64(0)
				body = pipeline.NewResponseBodyProgress(
					body,
					func(bytesTransferred int64) {
						diff := bytesTransferred - rangeProgress
						rangeProgress = bytesTransferred
						progressLock.Lock()
						progress += diff
						o.Progress(progress)
						progressLock.Unlock()
					})
			}
			_, err = io.ReadFull(body, b[chunkStart:chunkStart+count])
			body.Close()
			return err
		},
	})
	if err != nil {
		return err
	}
	return nil
}

// DownloadAzureFileToBuffer downloads an Azure file to a buffer with parallel.
// Offset and count are optional, pass 0 for both to download the entire blob.
func DownloadBlobToBuffer(ctx context.Context, blobURL BlobURL, offset int64, count int64,
	ac BlobAccessConditions, b []byte, o DownloadFromBlobOptions) error {
	return downloadBlobToBuffer(ctx, blobURL, offset, count, ac, b, o, nil)
}

// DownloadBlobToFile downloads an Azure file to a local file.
// The file would be truncated if the size doesn't match.
// Offset and count are optional, pass 0 for both to download the entire blob.
func DownloadBlobToFile(ctx context.Context, blobURL BlobURL, offset int64, count int64,
	ac BlobAccessConditions, file *os.File, o DownloadFromBlobOptions) error {
	// 1. Validate parameters.
	if file == nil {
		panic("file must not be nil")
	}

	// 2. Calculate the size of the destination file
	var size int64

	if count == CountToEnd {
		// Try to get Azure file's size
		props, err := blobURL.GetProperties(ctx, ac)
		if err != nil {
			return err
		}
		size = props.ContentLength() - offset
	} else {
		size = count
	}

	// 3. Compare and try to resize local file's size if it doesn't match Azure file's size.
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() != size {
		if err = file.Truncate(size); err != nil {
			return err
		}
	}

	if size > 0 {
		// 4. Set mmap and call DownloadAzureFileToBuffer.
		m, err := newMMF(file, true, 0, int(size))
		if err != nil {
			return err
		}
		defer m.unmap()
		return downloadBlobToBuffer(ctx, blobURL, offset, size, ac, m, o, nil)
	} else { // if the blob's size is 0, there is no need in downloading it
		return nil
	}
}


///////////////////////////////////////////////////////////////////////////////

// BatchTransferOptions identifies options used by doBatchTransfer.
type batchTransferOptions struct {
	transferSize  int64
	chunkSize     int64
	parallelism   uint16
	operation     func(offset int64, chunkSize int64) error
	operationName string
}

// doBatchTransfer helps to execute operations in a batch manner.
func doBatchTransfer(ctx context.Context, o batchTransferOptions) error {
	// Prepare and do parallel operations.
	numChunks := uint16(((o.transferSize - 1) / o.chunkSize) + 1)
	operationChannel := make(chan func() error, o.parallelism) // Create the channel that release 'parallelism' goroutines concurrently
	operationResponseChannel := make(chan error, numChunks)    // Holds each response
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create the goroutines that process each operation (in parallel).
	if o.parallelism == 0 {
		o.parallelism = 5 // default parallelism
	}
	for g := uint16(0); g < o.parallelism; g++ {
		//grIndex := g
		go func() {
			for f := range operationChannel {
				//fmt.Printf("[%s] gr-%d start action\n", o.operationName, grIndex)
				err := f()
				operationResponseChannel <- err
				//fmt.Printf("[%s] gr-%d end action\n", o.operationName, grIndex)
			}
		}()
	}

	// Add each chunk's operation to the channel.
	for chunkNum := uint16(0); chunkNum < numChunks; chunkNum++ {
		curChunkSize := o.chunkSize

		if chunkNum == numChunks-1 { // Last chunk
			curChunkSize = o.transferSize - (int64(chunkNum) * o.chunkSize) // Remove size of all transferred chunks from total
		}
		offset := int64(chunkNum) * o.chunkSize

		operationChannel <- func() error {
			return o.operation(offset, curChunkSize)
		}
	}
	close(operationChannel)

	// Wait for the operations to complete.
	for chunkNum := uint16(0); chunkNum < numChunks; chunkNum++ {
		responseError := <-operationResponseChannel
		if responseError != nil {
			cancel()             // As soon as any operation fails, cancel all remaining operation calls
			return responseError // No need to process anymore responses
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////////////////////

type UploadStreamToBlockBlobOptions struct {
	BufferSize       int
	MaxBuffers       int
	BlobHTTPHeaders  BlobHTTPHeaders
	Metadata         Metadata
	AccessConditions BlobAccessConditions
}

func UploadStreamToBlockBlob(ctx context.Context, reader io.Reader, blockBlobURL BlockBlobURL,
	o UploadStreamToBlockBlobOptions) (CommonResponse, error) {
	result, err := uploadStream(ctx, reader,
		UploadStreamOptions{BufferSize: o.BufferSize, MaxBuffers: o.MaxBuffers},
		&uploadStreamToBlockBlobOptions{b: blockBlobURL, o: o, blockIDPrefix: newUUID()})
	return result.(CommonResponse), err
}

type uploadStreamToBlockBlobOptions struct {
	b             BlockBlobURL
	o             UploadStreamToBlockBlobOptions
	blockIDPrefix uuid   // UUID used with all blockIDs
	maxBlockNum   uint32 // defaults to 0
	firstBlock    []byte // Used only if maxBlockNum is 0
}

func (t *uploadStreamToBlockBlobOptions) start(ctx context.Context) (interface{}, error) {
	return nil, nil
}

func (t *uploadStreamToBlockBlobOptions) chunk(ctx context.Context, num uint32, buffer []byte) error {
	if num == 0 && len(buffer) < t.o.BufferSize {
		// If whole payload fits in 1 block, don't stage it; End will upload it with 1 I/O operation
		t.firstBlock = buffer
		return nil
	}
	// Else, upload a staged block...
	AtomicMorphUint32(&t.maxBlockNum, func(startVal uint32) (val uint32, morphResult interface{}) {
		// Atomically remember (in t.numBlocks) the maximum block num we've ever seen
		if startVal < num {
			return num, nil
		}
		return startVal, nil
	})
	blockID := newUuidBlockID(t.blockIDPrefix).WithBlockNumber(num).ToBase64()
	_, err := t.b.StageBlock(ctx, blockID, bytes.NewReader(buffer), LeaseAccessConditions{})
	return err
}

func (t *uploadStreamToBlockBlobOptions) end(ctx context.Context) (interface{}, error) {
	if t.maxBlockNum == 0 {
		// If whole payload fits in 1 block (block #0), upload it with 1 I/O operation
		return t.b.Upload(ctx, bytes.NewReader(t.firstBlock),
			t.o.BlobHTTPHeaders, t.o.Metadata, t.o.AccessConditions)
	}
	// Multiple blocks staged, commit them all now
	blockID := newUuidBlockID(t.blockIDPrefix)
	blockIDs := make([]string, t.maxBlockNum + 1)
	for bn := uint32(0); bn <= t.maxBlockNum; bn++ {
		blockIDs[bn] = blockID.WithBlockNumber(bn).ToBase64()
	}
	return t.b.CommitBlockList(ctx, blockIDs, t.o.BlobHTTPHeaders, t.o.Metadata, t.o.AccessConditions)
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type iTransfer interface {
	start(ctx context.Context) (interface{}, error)
	chunk(ctx context.Context, num uint32, buffer []byte) error
	end(ctx context.Context) (interface{}, error)
}

type UploadStreamOptions struct {
	MaxBuffers int
	BufferSize int
}

func uploadStream(ctx context.Context, reader io.Reader, o UploadStreamOptions, t iTransfer) (interface{}, error) {
	ctx, cancel := context.WithCancel(ctx) // New context so that any failure cancels everything
	defer cancel()
	wg := sync.WaitGroup{} // Used to know when all outgoing messages have finished processing
	type OutgoingMsg struct {
		chunkNum uint32
		buffer   []byte
	}

	// Create a channel to hold the buffers usable for incoming datsa
	incoming := make(chan []byte, o.MaxBuffers)
	outgoing := make(chan OutgoingMsg, o.MaxBuffers) // Channel holding outgoing buffers
	if result, err := t.start(ctx); err != nil {
		return result, err
	}

	numBuffers := 0 // The number of buffers & out going goroutines created so far
	injectBuffer := func() {
		// For each Buffer, create it and a goroutine to upload it
		incoming <- make([]byte, o.BufferSize) // Add the new buffer to the incoming channel so this goroutine can from the reader into it
		numBuffers++
		go func() {
			for outgoingMsg := range outgoing {
				// Upload the outgoing buffer
				err := t.chunk(ctx, outgoingMsg.chunkNum, outgoingMsg.buffer)
				wg.Done() // Indicate this buffer was sent
				if nil != err {
					cancel()
				}
				incoming <- outgoingMsg.buffer // The goroutine reading from the stream can use reuse this buffer now
			}
		}()
	}
	injectBuffer() // Create our 1st buffer & outgoing goroutine

	// This goroutine grabs a buffer, reads from the stream into the buffer,
	// and inserts the buffer into the outgoing channel to be uploaded
	for c := uint32(0); true; c++ { // Iterate once per chunk
		var buffer []byte
		if numBuffers < o.MaxBuffers {
			select {
			// We're not at max buffers, see if a previously-created buffer is available
			case buffer = <-incoming:
				break
			default:
				// No buffer available; inject a new buffer & go routine to process it
				injectBuffer()
				buffer = <-incoming // Grab the just-injected buffer
			}
		} else {
			// We are at max buffers, block until we get to reuse one
			buffer = <-incoming
		}
		n, err := io.ReadFull(reader, buffer)
		if err != nil {
			buffer = buffer[:n] // Make slice match the # of read bytes
		}
		if len(buffer) > 0 {
			// Buffer not empty, upload it
			wg.Add(1) // We're posting a buffer to be sent
			outgoing <- OutgoingMsg{chunkNum: c, buffer: buffer}
		}
		if err != nil { // The reader is done, no more outgoing buffers
			break
		}
	}
	// NOTE: Don't close the incoming channel because the outgoing goroutines post buffers into it when they are done
	close(outgoing) // Make all the outgoing goroutines terminate when this channel is empty
	wg.Wait()       // Wait for all pending outgoing messages to complete
	// After all blocks uploaded, commit them to the blob & return the result
	return t.end(ctx)
}
