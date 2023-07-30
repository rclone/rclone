//go:build !plan9 && !solaris && !js
// +build !plan9,!solaris,!js

package oracleobjectstorage

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"golang.org/x/sync/errgroup"
)

var warnStreamUpload sync.Once

func (o *Object) uploadMultipart(
	ctx context.Context,
	putReq *objectstorage.PutObjectRequest,
	in io.Reader,
	src fs.ObjectInfo) (err error) {
	uploadID, uploadedParts, err := o.createMultipartUpload(ctx, putReq)
	if err != nil {
		fs.Errorf(o, "failed to create multipart upload-id err: %v", err)
		return err
	}
	return o.uploadParts(ctx, putReq, in, src, uploadID, uploadedParts)
}

func (o *Object) createMultipartUpload(ctx context.Context, putReq *objectstorage.PutObjectRequest) (
	uploadID string, uploadedParts map[int]objectstorage.MultipartUploadPartSummary, err error) {
	bucketName, bucketPath := o.split()
	f := o.fs
	if f.opt.AttemptResumeUpload {
		fs.Debugf(o, "attempting to resume upload for %v (if any)", o.remote)
		resumeUploads, err := o.fs.findLatestMultipartUpload(ctx, bucketName, bucketPath)
		if err == nil && len(resumeUploads) > 0 {
			uploadID = *resumeUploads[0].UploadId
			uploadedParts, err = f.listMultipartUploadParts(ctx, bucketName, bucketPath, uploadID)
			if err == nil {
				fs.Debugf(o, "resuming with existing upload id: %v", uploadID)
				return uploadID, uploadedParts, err
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
		return "", nil, err
	}
	uploadID = *resp.UploadId
	fs.Debugf(o, "created new upload id: %v", uploadID)
	return uploadID, nil, err
}

func (o *Object) uploadParts(
	ctx context.Context,
	putReq *objectstorage.PutObjectRequest,
	in io.Reader,
	src fs.ObjectInfo,
	uploadID string,
	uploadedParts map[int]objectstorage.MultipartUploadPartSummary) (err error) {
	bucketName, bucketPath := o.split()
	f := o.fs

	// make concurrency machinery
	concurrency := f.opt.UploadConcurrency
	if concurrency < 1 {
		concurrency = 1
	}

	uploadParts := f.opt.MaxUploadParts
	if uploadParts < 1 {
		uploadParts = 1
	} else if uploadParts > maxUploadParts {
		uploadParts = maxUploadParts
	}

	// calculate size of parts
	partSize := f.opt.ChunkSize
	fileSize := src.Size()

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 5 MiB). With a maximum number of parts (10,000) this will be a file of
	// 48 GiB which seems like a not too unreasonable limit.
	if fileSize == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, fs.SizeSuffix(int64(partSize)*int64(uploadParts)))
		})
	} else {
		partSize = chunksize.Calculator(o, fileSize, uploadParts, f.opt.ChunkSize)
	}

	uploadCtx, cancel := context.WithCancel(ctx)
	defer atexit.OnError(&err, func() {
		cancel()
		if o.fs.opt.LeavePartsOnError {
			return
		}
		fs.Debugf(o, "Cancelling multipart upload")
		errCancel := o.fs.abortMultiPartUpload(
			context.Background(),
			bucketName,
			bucketPath,
			uploadID)
		if errCancel != nil {
			fs.Debugf(o, "Failed to cancel multipart upload: %v", errCancel)
		} else {
			fs.Debugf(o, "canceled and aborted multipart upload: %v", uploadID)
		}
	})()

	var (
		g, gCtx  = errgroup.WithContext(uploadCtx)
		finished = false
		partsMu  sync.Mutex // to protect parts
		parts    []*objectstorage.CommitMultipartUploadPartDetails
		off      int64
		md5sMu   sync.Mutex
		md5s     []byte
		tokens   = pacer.NewTokenDispenser(concurrency)
		memPool  = o.fs.getMemoryPool(int64(partSize))
	)

	addMd5 := func(md5binary *[md5.Size]byte, partNum int64) {
		md5sMu.Lock()
		defer md5sMu.Unlock()
		start := partNum * md5.Size
		end := start + md5.Size
		if extend := end - int64(len(md5s)); extend > 0 {
			md5s = append(md5s, make([]byte, extend)...)
		}
		copy(md5s[start:end], (*md5binary)[:])
	}

	for partNum := int64(1); !finished; partNum++ {
		// Get a block of memory from the pool and token which limits concurrency.
		tokens.Get()
		buf := memPool.Get()

		free := func() {
			// return the memory and token
			memPool.Put(buf)
			tokens.Put()
		}

		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in uploading all the other parts.
		if gCtx.Err() != nil {
			free()
			break
		}

		// Read the chunk
		var n int
		n, err = readers.ReadFill(in, buf) // this can never return 0, nil
		if err == io.EOF {
			if n == 0 && partNum != 1 { // end if no data and if not first chunk
				free()
				break
			}
			finished = true
		} else if err != nil {
			free()
			return fmt.Errorf("multipart upload failed to read source: %w", err)
		}
		buf = buf[:n]

		partNum := partNum
		fs.Debugf(o, "multipart upload starting chunk %d size %v offset %v/%v", partNum, fs.SizeSuffix(n), fs.SizeSuffix(off), fs.SizeSuffix(fileSize))
		off += int64(n)
		g.Go(func() (err error) {
			defer free()
			partLength := int64(len(buf))

			// create checksum of buffer for integrity checking
			md5sumBinary := md5.Sum(buf)
			addMd5(&md5sumBinary, partNum-1)
			md5sum := base64.StdEncoding.EncodeToString(md5sumBinary[:])
			if uploadedPart, ok := uploadedParts[int(partNum)]; ok {
				if md5sum == *uploadedPart.Md5 {
					fs.Debugf(o, "matched uploaded part found, part num %d, skipping part, md5=%v", partNum, md5sum)
					partsMu.Lock()
					parts = append(parts, &objectstorage.CommitMultipartUploadPartDetails{
						PartNum: uploadedPart.PartNumber,
						Etag:    uploadedPart.Etag,
					})
					partsMu.Unlock()
					return nil
				}
			}

			req := objectstorage.UploadPartRequest{
				NamespaceName: common.String(o.fs.opt.Namespace),
				BucketName:    common.String(bucketName),
				ObjectName:    common.String(bucketPath),
				UploadId:      common.String(uploadID),
				UploadPartNum: common.Int(int(partNum)),
				ContentLength: common.Int64(partLength),
				ContentMD5:    common.String(md5sum),
			}
			o.applyPartUploadOptions(putReq, &req)
			var resp objectstorage.UploadPartResponse
			err = f.pacer.Call(func() (bool, error) {
				req.UploadPartBody = io.NopCloser(bytes.NewReader(buf))
				resp, err = f.srv.UploadPart(gCtx, req)
				if err != nil {
					if partNum <= int64(concurrency) {
						return shouldRetry(gCtx, resp.HTTPResponse(), err)
					}
					// retry all chunks once have done the first batch
					return true, err
				}
				partsMu.Lock()
				parts = append(parts, &objectstorage.CommitMultipartUploadPartDetails{
					PartNum: common.Int(int(partNum)),
					Etag:    resp.ETag,
				})
				partsMu.Unlock()
				return false, nil
			})
			if err != nil {
				fs.Errorf(o, "multipart upload failed to upload part:%d err: %v", partNum, err)
				return fmt.Errorf("multipart upload failed to upload part: %w", err)
			}
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return err
	}

	// sort the completed parts by part number
	sort.Slice(parts, func(i, j int) bool {
		return *parts[i].PartNum < *parts[j].PartNum
	})

	var resp objectstorage.CommitMultipartUploadResponse
	resp, err = o.commitMultiPart(ctx, uploadID, parts)
	if err != nil {
		return err
	}
	fs.Debugf(o, "multipart upload %v committed.", uploadID)
	hashOfHashes := md5.Sum(md5s)
	wantMultipartMd5 := base64.StdEncoding.EncodeToString(hashOfHashes[:]) + "-" + strconv.Itoa(len(parts))
	gotMultipartMd5 := *resp.OpcMultipartMd5
	if wantMultipartMd5 != gotMultipartMd5 {
		fs.Errorf(o, "multipart upload corrupted: multipart md5 differ: expecting %s but got %s", wantMultipartMd5, gotMultipartMd5)
		return fmt.Errorf("multipart upload corrupted: md5 differ: expecting %s but got %s", wantMultipartMd5, gotMultipartMd5)
	}
	fs.Debugf(o, "multipart upload %v md5 matched: expecting %s and got %s", uploadID, wantMultipartMd5, gotMultipartMd5)
	return nil
}

// commits the multipart upload
func (o *Object) commitMultiPart(ctx context.Context, uploadID string, parts []*objectstorage.CommitMultipartUploadPartDetails) (resp objectstorage.CommitMultipartUploadResponse, err error) {
	bucketName, bucketPath := o.split()
	req := objectstorage.CommitMultipartUploadRequest{
		NamespaceName: common.String(o.fs.opt.Namespace),
		BucketName:    common.String(bucketName),
		ObjectName:    common.String(bucketPath),
		UploadId:      common.String(uploadID),
	}
	var partsToCommit []objectstorage.CommitMultipartUploadPartDetails
	for _, part := range parts {
		partsToCommit = append(partsToCommit, *part)
	}
	req.PartsToCommit = partsToCommit
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CommitMultipartUpload(ctx, req)
		// if multipart is corrupted, we will abort the uploadId
		if o.isMultiPartUploadCorrupted(err) {
			fs.Debugf(o, "multipart uploadId %v is corrupted, aborting...", uploadID)
			errCancel := o.fs.abortMultiPartUpload(
				context.Background(),
				bucketName,
				bucketPath,
				uploadID)
			if errCancel != nil {
				fs.Debugf(o, "Failed to abort multipart upload: %v, ignoring.", errCancel)
			} else {
				fs.Debugf(o, "aborted multipart upload: %v", uploadID)
			}
			return false, err
		}
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	return resp, err
}

func (o *Object) isMultiPartUploadCorrupted(err error) bool {
	if err == nil {
		return false
	}
	// Check if this ocierr object, and if it is multipart commit error
	if ociError, ok := err.(common.ServiceError); ok {
		// If it is a timeout then we want to retry that
		if ociError.GetCode() == "InvalidUploadPart" {
			return true
		}
	}
	return false
}
