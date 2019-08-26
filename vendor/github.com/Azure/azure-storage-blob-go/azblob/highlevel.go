package azblob

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"

	"bytes"
	"os"
	"sync"
	"time"

	"errors"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// CommonResponse returns the headers common to all blob REST API responses.
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
	// Note that the progress reporting is not always increasing; it can go down when retrying a request.
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
	bufferSize := int64(len(b))
	if o.BlockSize == 0 {
		// If bufferSize > (BlockBlobMaxStageBlockBytes * BlockBlobMaxBlocks), then error
		if bufferSize > BlockBlobMaxStageBlockBytes*BlockBlobMaxBlocks {
			return nil, errors.New("buffer is too large to upload to a block blob")
		}
		// If bufferSize <= BlockBlobMaxUploadBlobBytes, then Upload should be used with just 1 I/O request
		if bufferSize <= BlockBlobMaxUploadBlobBytes {
			o.BlockSize = BlockBlobMaxUploadBlobBytes // Default if unspecified
		} else {
			o.BlockSize = bufferSize / BlockBlobMaxBlocks   // buffer / max blocks = block size to use all 50,000 blocks
			if o.BlockSize < BlobDefaultDownloadBlockSize { // If the block size is smaller than 4MB, round up to 4MB
				o.BlockSize = BlobDefaultDownloadBlockSize
			}
			// StageBlock will be called with blockSize blocks and a Parallelism of (BufferSize / BlockSize).
		}
	}

	if bufferSize <= BlockBlobMaxUploadBlobBytes {
		// If the size can fit in 1 Upload call, do it this way
		var body io.ReadSeeker = bytes.NewReader(b)
		if o.Progress != nil {
			body = pipeline.NewRequestBodyProgress(body, o.Progress)
		}
		return blockBlobURL.Upload(ctx, body, o.BlobHTTPHeaders, o.Metadata, o.AccessConditions)
	}

	var numBlocks = uint16(((bufferSize - 1) / o.BlockSize) + 1)

	blockIDList := make([]string, numBlocks) // Base-64 encoded block IDs
	progress := int64(0)
	progressLock := &sync.Mutex{}

	err := DoBatchTransfer(ctx, BatchTransferOptions{
		OperationName: "UploadBufferToBlockBlob",
		TransferSize:  bufferSize,
		ChunkSize:     o.BlockSize,
		Parallelism:   o.Parallelism,
		Operation: func(offset int64, count int64, ctx context.Context) error {
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
			_, err := blockBlobURL.StageBlock(ctx, blockIDList[blockNum], body, o.AccessConditions.LeaseAccessConditions, nil)
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

// DownloadFromBlobOptions identifies options used by the DownloadBlobToBuffer and DownloadBlobToFile functions.
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

// downloadBlobToBuffer downloads an Azure blob to a buffer with parallel.
func downloadBlobToBuffer(ctx context.Context, blobURL BlobURL, offset int64, count int64,
	b []byte, o DownloadFromBlobOptions, initialDownloadResponse *DownloadResponse) error {
	if o.BlockSize == 0 {
		o.BlockSize = BlobDefaultDownloadBlockSize
	}

	if count == CountToEnd { // If size not specified, calculate it
		if initialDownloadResponse != nil {
			count = initialDownloadResponse.ContentLength() - offset // if we have the length, use it
		} else {
			// If we don't have the length at all, get it
			dr, err := blobURL.Download(ctx, 0, CountToEnd, o.AccessConditions, false)
			if err != nil {
				return err
			}
			count = dr.ContentLength() - offset
		}
	}

	// Prepare and do parallel download.
	progress := int64(0)
	progressLock := &sync.Mutex{}

	err := DoBatchTransfer(ctx, BatchTransferOptions{
		OperationName: "downloadBlobToBuffer",
		TransferSize:  count,
		ChunkSize:     o.BlockSize,
		Parallelism:   o.Parallelism,
		Operation: func(chunkStart int64, count int64, ctx context.Context) error {
			dr, err := blobURL.Download(ctx, chunkStart+offset, count, o.AccessConditions, false)
			if err != nil {
				return err
			}
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

// DownloadBlobToBuffer downloads an Azure blob to a buffer with parallel.
// Offset and count are optional, pass 0 for both to download the entire blob.
func DownloadBlobToBuffer(ctx context.Context, blobURL BlobURL, offset int64, count int64,
	b []byte, o DownloadFromBlobOptions) error {
	return downloadBlobToBuffer(ctx, blobURL, offset, count, b, o, nil)
}

// DownloadBlobToFile downloads an Azure blob to a local file.
// The file would be truncated if the size doesn't match.
// Offset and count are optional, pass 0 for both to download the entire blob.
func DownloadBlobToFile(ctx context.Context, blobURL BlobURL, offset int64, count int64,
	file *os.File, o DownloadFromBlobOptions) error {
	// 1. Calculate the size of the destination file
	var size int64

	if count == CountToEnd {
		// Try to get Azure blob's size
		props, err := blobURL.GetProperties(ctx, o.AccessConditions)
		if err != nil {
			return err
		}
		size = props.ContentLength() - offset
	} else {
		size = count
	}

	// 2. Compare and try to resize local file's size if it doesn't match Azure blob's size.
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
		// 3. Set mmap and call downloadBlobToBuffer.
		m, err := newMMF(file, true, 0, int(size))
		if err != nil {
			return err
		}
		defer m.unmap()
		return downloadBlobToBuffer(ctx, blobURL, offset, size, m, o, nil)
	} else { // if the blob's size is 0, there is no need in downloading it
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////

// BatchTransferOptions identifies options used by DoBatchTransfer.
type BatchTransferOptions struct {
	TransferSize  int64
	ChunkSize     int64
	Parallelism   uint16
	Operation     func(offset int64, chunkSize int64, ctx context.Context) error
	OperationName string
}

// DoBatchTransfer helps to execute operations in a batch manner.
// Can be used by users to customize batch works (for other scenarios that the SDK does not provide)
func DoBatchTransfer(ctx context.Context, o BatchTransferOptions) error {
	if o.ChunkSize == 0 {
		return errors.New("ChunkSize cannot be 0")
	}

	// Prepare and do parallel operations.
	numChunks := uint16(((o.TransferSize - 1) / o.ChunkSize) + 1)
	operationChannel := make(chan func() error, o.Parallelism) // Create the channel that release 'Parallelism' goroutines concurrently
	operationResponseChannel := make(chan error, numChunks)    // Holds each response
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create the goroutines that process each operation (in parallel).
	if o.Parallelism == 0 {
		o.Parallelism = 5 // default Parallelism
	}
	for g := uint16(0); g < o.Parallelism; g++ {
		//grIndex := g
		go func() {
			for f := range operationChannel {
				err := f()
				operationResponseChannel <- err
			}
		}()
	}

	// Add each chunk's operation to the channel.
	for chunkNum := uint16(0); chunkNum < numChunks; chunkNum++ {
		curChunkSize := o.ChunkSize

		if chunkNum == numChunks-1 { // Last chunk
			curChunkSize = o.TransferSize - (int64(chunkNum) * o.ChunkSize) // Remove size of all transferred chunks from total
		}
		offset := int64(chunkNum) * o.ChunkSize

		operationChannel <- func() error {
			return o.Operation(offset, curChunkSize, ctx)
		}
	}
	close(operationChannel)

	// Wait for the operations to complete.
	var firstErr error = nil
	for chunkNum := uint16(0); chunkNum < numChunks; chunkNum++ {
		responseError := <-operationResponseChannel
		// record the first error (the original error which should cause the other chunks to fail with canceled context)
		if responseError != nil && firstErr == nil {
			cancel() // As soon as any operation fails, cancel all remaining operation calls
			firstErr = responseError
		}
	}
	return firstErr
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
	if err != nil {
		return nil, err
	}
	return result.(CommonResponse), nil
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
	if num == 0 {
		t.firstBlock = buffer

		// If whole payload fits in 1 block, don't stage it; End will upload it with 1 I/O operation
		// If the payload is exactly the same size as the buffer, there may be more content coming in.
		if len(buffer) < t.o.BufferSize {
			return nil
		}
	}
	// Else, upload a staged block...
	atomicMorphUint32(&t.maxBlockNum, func(startVal uint32) (val uint32, morphResult interface{}) {
		// Atomically remember (in t.numBlocks) the maximum block num we've ever seen
		if startVal < num {
			return num, nil
		}
		return startVal, nil
	})
	blockID := newUuidBlockID(t.blockIDPrefix).WithBlockNumber(num).ToBase64()
	_, err := t.b.StageBlock(ctx, blockID, bytes.NewReader(buffer), LeaseAccessConditions{}, nil)
	return err
}

func (t *uploadStreamToBlockBlobOptions) end(ctx context.Context) (interface{}, error) {
	// If the first block had the exact same size as the buffer
	// we would have staged it as a block thinking that there might be more data coming
	if t.maxBlockNum == 0 && len(t.firstBlock) != t.o.BufferSize {
		// If whole payload fits in 1 block (block #0), upload it with 1 I/O operation
		return t.b.Upload(ctx, bytes.NewReader(t.firstBlock),
			t.o.BlobHTTPHeaders, t.o.Metadata, t.o.AccessConditions)
	}
	// Multiple blocks staged, commit them all now
	blockID := newUuidBlockID(t.blockIDPrefix)
	blockIDs := make([]string, t.maxBlockNum+1)
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

type firstErr struct {
	lock       sync.Mutex
	finalError error
}

func (fe *firstErr) set(err error) {
	fe.lock.Lock()
	if fe.finalError == nil {
		fe.finalError = err
	}
	fe.lock.Unlock()
}

func (fe *firstErr) get() (err error) {
	fe.lock.Lock()
	err = fe.finalError
	fe.lock.Unlock()
	return
}

func uploadStream(ctx context.Context, reader io.Reader, o UploadStreamOptions, t iTransfer) (interface{}, error) {
	firstErr := firstErr{}
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
					// NOTE: finalErr could be assigned to multiple times here which is OK,
					// some error will be returned.
					firstErr.set(err)
					cancel()
				}
				incoming <- outgoingMsg.buffer // The goroutine reading from the stream can reuse this buffer now
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
		if err != nil { // Less than len(buffer) bytes were read
			buffer = buffer[:n] // Make slice match the # of read bytes
		}
		if len(buffer) > 0 {
			// Buffer not empty, upload it
			wg.Add(1) // We're posting a buffer to be sent
			outgoing <- OutgoingMsg{chunkNum: c, buffer: buffer}
		}
		if err != nil { // The reader is done, no more outgoing buffers
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				err = nil // This function does NOT return an error if io.ReadFull returns io.EOF or io.ErrUnexpectedEOF
			} else {
				firstErr.set(err)
			}
			break
		}
	}
	// NOTE: Don't close the incoming channel because the outgoing goroutines post buffers into it when they are done
	close(outgoing) // Make all the outgoing goroutines terminate when this channel is empty
	wg.Wait()       // Wait for all pending outgoing messages to complete
	err := firstErr.get()
	if err == nil {
		// If no error, after all blocks uploaded, commit them to the blob & return the result
		return t.end(ctx)
	}
	return nil, err
}
