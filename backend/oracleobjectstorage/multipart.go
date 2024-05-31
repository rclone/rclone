//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pool"
	"golang.org/x/net/http/httpguts"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/hash"
)

var warnStreamUpload sync.Once

// Info needed for an upload
type uploadInfo struct {
	req       *objectstorage.PutObjectRequest
	md5sumHex string
}

type objectChunkWriter struct {
	chunkSize       int64
	size            int64
	f               *Fs
	bucket          *string
	key             *string
	uploadID        *string
	partsToCommit   []objectstorage.CommitMultipartUploadPartDetails
	partsToCommitMu sync.Mutex
	existingParts   map[int]objectstorage.MultipartUploadPartSummary
	eTag            string
	md5sMu          sync.Mutex
	md5s            []byte
	ui              uploadInfo
	o               *Object
}

func (o *Object) uploadMultipart(ctx context.Context, src fs.ObjectInfo, in io.Reader, options ...fs.OpenOption) error {
	_, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	return err
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(
	ctx context.Context,
	remote string,
	src fs.ObjectInfo,
	options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}
	ui, err := o.prepareUpload(ctx, src, options)
	if err != nil {
		return info, nil, fmt.Errorf("failed to prepare upload: %w", err)
	}

	uploadParts := f.opt.MaxUploadParts
	if uploadParts < 1 {
		uploadParts = 1
	} else if uploadParts > maxUploadParts {
		uploadParts = maxUploadParts
	}
	size := src.Size()

	// calculate size of parts
	chunkSize := f.opt.ChunkSize

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 5 MiB). With a maximum number of parts (10,000) this will be a file of
	// 48 GiB which seems like a not too unreasonable limit.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, fs.SizeSuffix(int64(chunkSize)*int64(uploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(src, size, uploadParts, chunkSize)
	}

	uploadID, existingParts, err := o.createMultipartUpload(ctx, ui.req)
	if err != nil {
		return info, nil, fmt.Errorf("create multipart upload request failed: %w", err)
	}
	bucketName, bucketPath := o.split()
	chunkWriter := &objectChunkWriter{
		chunkSize:     int64(chunkSize),
		size:          size,
		f:             f,
		bucket:        &bucketName,
		key:           &bucketPath,
		uploadID:      &uploadID,
		existingParts: existingParts,
		ui:            ui,
		o:             o,
	}
	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(chunkSize),
		Concurrency:       o.fs.opt.UploadConcurrency,
		LeavePartsOnError: o.fs.opt.LeavePartsOnError,
	}
	fs.Debugf(o, "open chunk writer: started multipart upload: %v", uploadID)
	return info, chunkWriter, err
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (w *objectChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {
	if chunkNumber < 0 {
		err := fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
		return -1, err
	}
	// Only account after the checksum reads have been done
	if do, ok := reader.(pool.DelayAccountinger); ok {
		// To figure out this number, do a transfer and if the accounted size is 0 or a
		// multiple of what it should be, increase or decrease this number.
		do.DelayAccounting(2)
	}
	m := md5.New()
	currentChunkSize, err := io.Copy(m, reader)
	if err != nil {
		return -1, err
	}
	// If no data read, don't write the chunk
	if currentChunkSize == 0 {
		return 0, nil
	}
	md5sumBinary := m.Sum([]byte{})
	w.addMd5(&md5sumBinary, int64(chunkNumber))
	md5sum := base64.StdEncoding.EncodeToString(md5sumBinary)

	// Object storage requires 1 <= PartNumber <= 10000
	ossPartNumber := chunkNumber + 1
	if existing, ok := w.existingParts[ossPartNumber]; ok {
		if md5sum == *existing.Md5 {
			fs.Debugf(w.o, "matched uploaded part found, part num %d, skipping part, md5=%v", *existing.PartNumber, md5sum)
			w.addCompletedPart(existing.PartNumber, existing.Etag)
			return currentChunkSize, nil
		}
	}
	req := objectstorage.UploadPartRequest{
		NamespaceName: common.String(w.f.opt.Namespace),
		BucketName:    w.bucket,
		ObjectName:    w.key,
		UploadId:      w.uploadID,
		UploadPartNum: common.Int(ossPartNumber),
		ContentLength: common.Int64(currentChunkSize),
		ContentMD5:    common.String(md5sum),
	}
	w.o.applyPartUploadOptions(w.ui.req, &req)
	var resp objectstorage.UploadPartResponse
	err = w.f.pacer.Call(func() (bool, error) {
		// req.UploadPartBody = io.NopCloser(bytes.NewReader(buf))
		// rewind the reader on retry and after reading md5
		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		req.UploadPartBody = io.NopCloser(reader)
		resp, err = w.f.srv.UploadPart(ctx, req)
		if err != nil {
			if ossPartNumber <= 8 {
				return shouldRetry(ctx, resp.HTTPResponse(), err)
			}
			// retry all chunks once have done the first few
			return true, err
		}
		return false, err
	})
	if err != nil {
		fs.Errorf(w.o, "multipart upload failed to upload part:%d err: %v", ossPartNumber, err)
		return -1, fmt.Errorf("multipart upload failed to upload part: %w", err)
	}
	w.addCompletedPart(&ossPartNumber, resp.ETag)
	return currentChunkSize, err

}

// add a part number and etag to the completed parts
func (w *objectChunkWriter) addCompletedPart(partNum *int, eTag *string) {
	w.partsToCommitMu.Lock()
	defer w.partsToCommitMu.Unlock()
	w.partsToCommit = append(w.partsToCommit, objectstorage.CommitMultipartUploadPartDetails{
		PartNum: partNum,
		Etag:    eTag,
	})
}

func (w *objectChunkWriter) Close(ctx context.Context) (err error) {
	req := objectstorage.CommitMultipartUploadRequest{
		NamespaceName: common.String(w.f.opt.Namespace),
		BucketName:    w.bucket,
		ObjectName:    w.key,
		UploadId:      w.uploadID,
	}
	req.PartsToCommit = w.partsToCommit
	var resp objectstorage.CommitMultipartUploadResponse
	err = w.f.pacer.Call(func() (bool, error) {
		resp, err = w.f.srv.CommitMultipartUpload(ctx, req)
		// if multipart is corrupted, we will abort the uploadId
		if isMultiPartUploadCorrupted(err) {
			fs.Debugf(w.o, "multipart uploadId %v is corrupted, aborting...", *w.uploadID)
			_ = w.Abort(ctx)
			return false, err
		}
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	if err != nil {
		return err
	}
	w.eTag = *resp.ETag
	hashOfHashes := md5.Sum(w.md5s)
	wantMultipartMd5 := fmt.Sprintf("%s-%d", base64.StdEncoding.EncodeToString(hashOfHashes[:]), len(w.partsToCommit))
	gotMultipartMd5 := *resp.OpcMultipartMd5
	if wantMultipartMd5 != gotMultipartMd5 {
		fs.Errorf(w.o, "multipart upload corrupted: multipart md5 differ: expecting %s but got %s", wantMultipartMd5, gotMultipartMd5)
		return fmt.Errorf("multipart upload corrupted: md5 differ: expecting %s but got %s", wantMultipartMd5, gotMultipartMd5)
	}
	fs.Debugf(w.o, "multipart upload %v md5 matched: expecting %s and got %s", *w.uploadID, wantMultipartMd5, gotMultipartMd5)
	return nil
}

func isMultiPartUploadCorrupted(err error) bool {
	if err == nil {
		return false
	}
	// Check if this oci-err object, and if it is multipart commit error
	if ociError, ok := err.(common.ServiceError); ok {
		// If it is a timeout then we want to retry that
		if ociError.GetCode() == "InvalidUploadPart" {
			return true
		}
	}
	return false
}

func (w *objectChunkWriter) Abort(ctx context.Context) error {
	fs.Debugf(w.o, "Cancelling multipart upload")
	err := w.o.fs.abortMultiPartUpload(
		ctx,
		w.bucket,
		w.key,
		w.uploadID)
	if err != nil {
		fs.Debugf(w.o, "Failed to cancel multipart upload: %v", err)
	} else {
		fs.Debugf(w.o, "canceled and aborted multipart upload: %v", *w.uploadID)
	}
	return err
}

// addMd5 adds a binary md5 to the md5 calculated so far
func (w *objectChunkWriter) addMd5(md5binary *[]byte, chunkNumber int64) {
	w.md5sMu.Lock()
	defer w.md5sMu.Unlock()
	start := chunkNumber * md5.Size
	end := start + md5.Size
	if extend := end - int64(len(w.md5s)); extend > 0 {
		w.md5s = append(w.md5s, make([]byte, extend)...)
	}
	copy(w.md5s[start:end], (*md5binary))
}

func (o *Object) prepareUpload(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption) (ui uploadInfo, err error) {
	bucket, bucketPath := o.split()

	ui.req = &objectstorage.PutObjectRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucket),
		ObjectName:    common.String(bucketPath),
	}

	// Set the mtime in the metadata
	modTime := src.ModTime(ctx)
	// Fetch metadata if --metadata is in use
	meta, err := fs.GetMetadataOptions(ctx, o.fs, src, options)
	if err != nil {
		return ui, fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	ui.req.OpcMeta = make(map[string]string, len(meta)+2)
	// merge metadata into request and user metadata
	for k, v := range meta {
		pv := common.String(v)
		k = strings.ToLower(k)
		switch k {
		case "cache-control":
			ui.req.CacheControl = pv
		case "content-disposition":
			ui.req.ContentDisposition = pv
		case "content-encoding":
			ui.req.ContentEncoding = pv
		case "content-language":
			ui.req.ContentLanguage = pv
		case "content-type":
			ui.req.ContentType = pv
		case "tier":
			// ignore
		case "mtime":
			// mtime in meta overrides source ModTime
			metaModTime, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				fs.Debugf(o, "failed to parse metadata %s: %q: %v", k, v, err)
			} else {
				modTime = metaModTime
			}
		case "btime":
			// write as metadata since we can't set it
			ui.req.OpcMeta[k] = v
		default:
			ui.req.OpcMeta[k] = v
		}
	}

	// Set the mtime in the metadata
	ui.req.OpcMeta[metaMtime] = swift.TimeToFloatString(modTime)

	// read the md5sum if available
	// - for non-multipart
	//    - so we can add a ContentMD5
	//    - so we can add the md5sum in the metadata as metaMD5Hash if using SSE/SSE-C
	// - for multipart provided checksums aren't disabled
	//    - so we can add the md5sum in the metadata as metaMD5Hash
	size := src.Size()
	isMultipart := size < 0 || size >= int64(o.fs.opt.UploadCutoff)
	var md5sumBase64 string
	if !isMultipart || !o.fs.opt.DisableChecksum {
		ui.md5sumHex, err = src.Hash(ctx, hash.MD5)
		if err == nil && matchMd5.MatchString(ui.md5sumHex) {
			hashBytes, err := hex.DecodeString(ui.md5sumHex)
			if err == nil {
				md5sumBase64 = base64.StdEncoding.EncodeToString(hashBytes)
				if isMultipart && !o.fs.opt.DisableChecksum {
					// Set the md5sum as metadata on the object if
					// - a multipart upload
					// - the ETag is not an MD5, e.g. when using SSE/SSE-C
					// provided checksums aren't disabled
					ui.req.OpcMeta[metaMD5Hash] = md5sumBase64
				}
			}
		}
	}
	// Set the content type if it isn't set already
	if ui.req.ContentType == nil {
		ui.req.ContentType = common.String(fs.MimeType(ctx, src))
	}
	if size >= 0 {
		ui.req.ContentLength = common.Int64(size)
	}
	if md5sumBase64 != "" {
		ui.req.ContentMD5 = &md5sumBase64
	}
	o.applyPutOptions(ui.req, options...)
	useBYOKPutObject(o.fs, ui.req)
	if o.fs.opt.StorageTier != "" {
		storageTier, ok := objectstorage.GetMappingPutObjectStorageTierEnum(o.fs.opt.StorageTier)
		if !ok {
			return ui, fmt.Errorf("not a valid storage tier: %v", o.fs.opt.StorageTier)
		}
		ui.req.StorageTier = storageTier
	}
	// Check metadata keys and values are valid
	for key, value := range ui.req.OpcMeta {
		if !httpguts.ValidHeaderFieldName(key) {
			fs.Errorf(o, "Dropping invalid metadata key %q", key)
			delete(ui.req.OpcMeta, key)
		} else if value == "" {
			fs.Errorf(o, "Dropping nil metadata value for key %q", key)
			delete(ui.req.OpcMeta, key)
		} else if !httpguts.ValidHeaderFieldValue(value) {
			fs.Errorf(o, "Dropping invalid metadata value %q for key %q", value, key)
			delete(ui.req.OpcMeta, key)
		}
	}
	return ui, nil
}

func (o *Object) createMultipartUpload(ctx context.Context, putReq *objectstorage.PutObjectRequest) (
	uploadID string, existingParts map[int]objectstorage.MultipartUploadPartSummary, err error) {
	bucketName, bucketPath := o.split()
	err = o.fs.makeBucket(ctx, bucketName)
	if err != nil {
		fs.Errorf(o, "failed to create bucket: %v, err: %v", bucketName, err)
		return uploadID, existingParts, err
	}
	if o.fs.opt.AttemptResumeUpload {
		fs.Debugf(o, "attempting to resume upload for %v (if any)", o.remote)
		resumeUploads, err := o.fs.findLatestMultipartUpload(ctx, bucketName, bucketPath)
		if err == nil && len(resumeUploads) > 0 {
			uploadID = *resumeUploads[0].UploadId
			existingParts, err = o.fs.listMultipartUploadParts(ctx, bucketName, bucketPath, uploadID)
			if err == nil {
				fs.Debugf(o, "resuming with existing upload id: %v", uploadID)
				return uploadID, existingParts, err
			}
		}
	}
	req := objectstorage.CreateMultipartUploadRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucketName),
	}
	req.Object = common.String(bucketPath)
	if o.fs.opt.StorageTier != "" {
		storageTier, ok := objectstorage.GetMappingStorageTierEnum(o.fs.opt.StorageTier)
		if !ok {
			return "", nil, fmt.Errorf("not a valid storage tier: %v", o.fs.opt.StorageTier)
		}
		req.StorageTier = storageTier
	}
	o.applyMultipartUploadOptions(putReq, &req)

	var resp objectstorage.CreateMultipartUploadResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CreateMultipartUpload(ctx, req)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	if err != nil {
		return "", existingParts, err
	}
	existingParts = make(map[int]objectstorage.MultipartUploadPartSummary)
	uploadID = *resp.UploadId
	fs.Debugf(o, "created new upload id: %v", uploadID)
	return uploadID, existingParts, err
}
