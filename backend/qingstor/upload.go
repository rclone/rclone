// Upload object to QingStor

//go:build !plan9 && !js

package qingstor

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	qs "github.com/yunify/qingstor-sdk-go/v3/service"
)

const (
	// maxSinglePartSize = 1024 * 1024 * 1024 * 5 // The maximum allowed size when uploading a single object to QingStor
	// maxMultiPartSize = 1024 * 1024 * 1024 * 1 // The maximum allowed part size when uploading a part to QingStor
	minMultiPartSize = 1024 * 1024 * 4 // The minimum allowed part size when uploading a part to QingStor
	maxMultiParts    = 10000           // The maximum allowed number of parts in a multi-part upload
)

const (
	defaultUploadPartSize    = 1024 * 1024 * 64 // The default part size to buffer chunks of a payload into.
	defaultUploadConcurrency = 4                // the default number of goroutines to spin up when using multiPartUpload.
)

func readFillBuf(r io.Reader, b []byte) (offset int, err error) {
	for offset < len(b) && err == nil {
		var n int
		n, err = r.Read(b[offset:])
		offset += n
	}

	return offset, err
}

// uploadInput contains all input for upload requests to QingStor.
type uploadInput struct {
	body           io.Reader
	qsSvc          *qs.Service
	mimeType       string
	zone           string
	bucket         string
	key            string
	partSize       int64
	concurrency    int
	maxUploadParts int
}

// uploader internal structure to manage an upload to QingStor.
type uploader struct {
	cfg        *uploadInput
	totalSize  int64 // set to -1 if the size is not known
	readerPos  int64 // current reader position
	readerSize int64 // current reader content size
}

// newUploader creates a new Uploader instance to upload objects to QingStor.
func newUploader(in *uploadInput) *uploader {
	u := &uploader{
		cfg: in,
	}
	return u
}

// bucketInit initiate as bucket controller
func (u *uploader) bucketInit() (*qs.Bucket, error) {
	bucketInit, err := u.cfg.qsSvc.Bucket(u.cfg.bucket, u.cfg.zone)
	return bucketInit, err
}

// String converts uploader to a string
func (u *uploader) String() string {
	return fmt.Sprintf("QingStor bucket %s key %s", u.cfg.bucket, u.cfg.key)
}

// nextReader returns a seekable reader representing the next packet of data.
// This operation increases the shared u.readerPos counter, but note that it
// does not need to be wrapped in a mutex because nextReader is only called
// from the main thread.
func (u *uploader) nextReader() (io.ReadSeeker, int, error) {
	type readerAtSeeker interface {
		io.ReaderAt
		io.ReadSeeker
	}
	switch r := u.cfg.body.(type) {
	case readerAtSeeker:
		var err error
		n := u.cfg.partSize
		if u.totalSize >= 0 {
			bytesLeft := u.totalSize - u.readerPos

			if bytesLeft <= u.cfg.partSize {
				err = io.EOF
				n = bytesLeft
			}
		}
		reader := io.NewSectionReader(r, u.readerPos, n)
		u.readerPos += n
		u.readerSize = n
		return reader, int(n), err

	default:
		part := make([]byte, u.cfg.partSize)
		n, err := readFillBuf(r, part)
		u.readerPos += int64(n)
		u.readerSize = int64(n)
		return bytes.NewReader(part[0:n]), n, err
	}
}

// init will initialize all default options.
func (u *uploader) init() {
	if u.cfg.concurrency == 0 {
		u.cfg.concurrency = defaultUploadConcurrency
	}
	if u.cfg.partSize == 0 {
		u.cfg.partSize = defaultUploadPartSize
	}
	if u.cfg.maxUploadParts == 0 {
		u.cfg.maxUploadParts = maxMultiParts
	}
	// Try to get the total size for some optimizations
	u.totalSize = -1
	switch r := u.cfg.body.(type) {
	case io.Seeker:
		pos, _ := r.Seek(0, io.SeekCurrent)
		defer func() {
			_, _ = r.Seek(pos, io.SeekStart)
		}()

		n, err := r.Seek(0, io.SeekEnd)
		if err != nil {
			return
		}
		u.totalSize = n

		// Try to adjust partSize if it is too small and account for
		// integer division truncation.
		if u.totalSize/u.cfg.partSize >= u.cfg.partSize {
			// Add one to the part size to account for remainders
			// during the size calculation. e.g odd number of bytes.
			u.cfg.partSize = (u.totalSize / int64(u.cfg.maxUploadParts)) + 1
		}
	}
}

// singlePartUpload upload a single object that contentLength less than "defaultUploadPartSize"
func (u *uploader) singlePartUpload(buf io.Reader, size int64) error {
	bucketInit, _ := u.bucketInit()

	req := qs.PutObjectInput{
		ContentLength: &size,
		ContentType:   &u.cfg.mimeType,
		Body:          buf,
	}

	_, err := bucketInit.PutObject(u.cfg.key, &req)
	if err == nil {
		fs.Debugf(u, "Upload single object finished")
	}
	return err
}

// Upload upload an object into QingStor
func (u *uploader) upload() error {
	u.init()

	if u.cfg.partSize < minMultiPartSize {
		return fmt.Errorf("part size must be at least %d bytes", minMultiPartSize)
	}

	// Do one read to determine if we have more than one part
	reader, _, err := u.nextReader()
	if err == io.EOF { // single part
		fs.Debugf(u, "Uploading as single part object to QingStor")
		return u.singlePartUpload(reader, u.readerPos)
	} else if err != nil {
		return fmt.Errorf("read upload data failed: %w", err)
	}

	fs.Debugf(u, "Uploading as multi-part object to QingStor")
	mu := multiUploader{uploader: u}
	return mu.multiPartUpload(reader)
}

// internal structure to manage a specific multipart upload to QingStor.
type multiUploader struct {
	*uploader
	wg          sync.WaitGroup
	mtx         sync.Mutex
	err         error
	uploadID    *string
	objectParts completedParts
	hashMd5     hash.Hash
}

// keeps track of a single chunk of data being sent to QingStor.
type chunk struct {
	buffer     io.ReadSeeker
	partNumber int
	size       int64
}

// completedParts is a wrapper to make parts sortable by their part number,
// since QingStor required this list to be sent in sorted order.
type completedParts []*qs.ObjectPartType

func (a completedParts) Len() int           { return len(a) }
func (a completedParts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a completedParts) Less(i, j int) bool { return *a[i].PartNumber < *a[j].PartNumber }

// String converts multiUploader to a string
func (mu *multiUploader) String() string {
	if uploadID := mu.uploadID; uploadID != nil {
		return fmt.Sprintf("QingStor bucket %s key %s uploadID %s", mu.cfg.bucket, mu.cfg.key, *uploadID)
	}
	return fmt.Sprintf("QingStor bucket %s key %s uploadID <nil>", mu.cfg.bucket, mu.cfg.key)
}

// getErr is a thread-safe getter for the error object
func (mu *multiUploader) getErr() error {
	mu.mtx.Lock()
	defer mu.mtx.Unlock()
	return mu.err
}

// setErr is a thread-safe setter for the error object
func (mu *multiUploader) setErr(e error) {
	mu.mtx.Lock()
	defer mu.mtx.Unlock()
	mu.err = e
}

// readChunk runs in worker goroutines to pull chunks off of the ch channel
// and send() them as UploadPart requests.
func (mu *multiUploader) readChunk(ch chan chunk) {
	defer mu.wg.Done()
	for {
		c, ok := <-ch
		if !ok {
			break
		}
		if mu.getErr() == nil {
			if err := mu.send(c); err != nil {
				mu.setErr(err)
			}
		}
	}
}

// initiate init a Multiple Object and obtain UploadID
func (mu *multiUploader) initiate() error {
	bucketInit, _ := mu.bucketInit()
	req := qs.InitiateMultipartUploadInput{
		ContentType: &mu.cfg.mimeType,
	}
	fs.Debugf(mu, "Initiating a multi-part upload")
	rsp, err := bucketInit.InitiateMultipartUpload(mu.cfg.key, &req)
	if err == nil {
		mu.uploadID = rsp.UploadID
		mu.hashMd5 = md5.New()
	}
	return err
}

// send upload a part into QingStor
func (mu *multiUploader) send(c chunk) error {
	bucketInit, _ := mu.bucketInit()
	req := qs.UploadMultipartInput{
		PartNumber:    &c.partNumber,
		UploadID:      mu.uploadID,
		ContentLength: &c.size,
		Body:          c.buffer,
	}
	fs.Debugf(mu, "Uploading a part to QingStor with partNumber %d and partSize %d", c.partNumber, c.size)
	_, err := bucketInit.UploadMultipart(mu.cfg.key, &req)
	if err != nil {
		return err
	}
	fs.Debugf(mu, "Done uploading part partNumber %d and partSize %d", c.partNumber, c.size)

	mu.mtx.Lock()
	defer mu.mtx.Unlock()

	_, _ = c.buffer.Seek(0, 0)
	_, _ = io.Copy(mu.hashMd5, c.buffer)

	parts := qs.ObjectPartType{PartNumber: &c.partNumber, Size: &c.size}
	mu.objectParts = append(mu.objectParts, &parts)
	return err
}

// complete complete a multipart upload
func (mu *multiUploader) complete() error {
	var err error
	if err = mu.getErr(); err != nil {
		return err
	}
	bucketInit, _ := mu.bucketInit()
	//if err = mu.list(); err != nil {
	//	return err
	//}
	//md5String := fmt.Sprintf("\"%s\"", hex.EncodeToString(mu.hashMd5.Sum(nil)))

	md5String := fmt.Sprintf("\"%x\"", mu.hashMd5.Sum(nil))
	sort.Sort(mu.objectParts)
	req := qs.CompleteMultipartUploadInput{
		UploadID:    mu.uploadID,
		ObjectParts: mu.objectParts,
		ETag:        &md5String,
	}
	fs.Debugf(mu, "Completing multi-part object")
	_, err = bucketInit.CompleteMultipartUpload(mu.cfg.key, &req)
	if err == nil {
		fs.Debugf(mu, "Complete multi-part finished")
	}
	return err
}

// abort abort a multipart upload
func (mu *multiUploader) abort() error {
	var err error
	bucketInit, _ := mu.bucketInit()

	if uploadID := mu.uploadID; uploadID != nil {
		req := qs.AbortMultipartUploadInput{
			UploadID: uploadID,
		}
		fs.Debugf(mu, "Aborting multi-part object %q", *uploadID)
		_, err = bucketInit.AbortMultipartUpload(mu.cfg.key, &req)
	}

	return err
}

// multiPartUpload upload a multiple object into QingStor
func (mu *multiUploader) multiPartUpload(firstBuf io.ReadSeeker) (err error) {
	// Initiate a multi-part upload
	if err = mu.initiate(); err != nil {
		return err
	}

	// Cancel the session if something went wrong
	defer atexit.OnError(&err, func() {
		fs.Debugf(mu, "Cancelling multipart upload: %v", err)
		cancelErr := mu.abort()
		if cancelErr != nil {
			fs.Logf(mu, "Failed to cancel multipart upload: %v", cancelErr)
		}
	})()

	ch := make(chan chunk, mu.cfg.concurrency)
	for i := 0; i < mu.cfg.concurrency; i++ {
		mu.wg.Add(1)
		go mu.readChunk(ch)
	}

	var partNumber int
	ch <- chunk{partNumber: partNumber, buffer: firstBuf, size: mu.readerSize}

	for mu.getErr() == nil {
		partNumber++
		// This upload exceeded maximum number of supported parts, error now.
		if partNumber > mu.cfg.maxUploadParts || partNumber > maxMultiParts {
			var msg string
			if partNumber > mu.cfg.maxUploadParts {
				msg = fmt.Sprintf("exceeded total allowed configured maxUploadParts (%d). "+
					"Adjust PartSize to fit in this limit", mu.cfg.maxUploadParts)
			} else {
				msg = fmt.Sprintf("exceeded total allowed QingStor limit maxUploadParts (%d). "+
					"Adjust PartSize to fit in this limit", maxMultiParts)
			}
			mu.setErr(errors.New(msg))
			break
		}

		var reader io.ReadSeeker
		var nextChunkLen int
		reader, nextChunkLen, err = mu.nextReader()
		if err != nil && err != io.EOF {
			// empty ch
			go func() {
				for range ch {
				}
			}()
			// Wait for all goroutines finish
			close(ch)
			mu.wg.Wait()
			return err
		}
		if nextChunkLen == 0 && partNumber > 0 {
			// No need to upload empty part, if file was empty to start
			// with empty single part would of been created and never
			// started multipart upload.
			break
		}
		num := partNumber
		ch <- chunk{partNumber: num, buffer: reader, size: mu.readerSize}
	}
	// Wait for all goroutines finish
	close(ch)
	mu.wg.Wait()
	// Complete Multipart Upload
	return mu.complete()
}
