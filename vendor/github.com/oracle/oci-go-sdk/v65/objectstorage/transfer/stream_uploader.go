// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package transfer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// StreamUploader is an interface for upload a stream
type StreamUploader interface {
	// uploads a stream to blob storage
	UploadStream(ctx context.Context, request UploadStreamRequest) (response UploadResponse, err error)
}

type streamUpload struct {
	uploadID          string
	manifest          *multipartManifest
	multipartUploader multipartUploader
	request           UploadStreamRequest
}

func (streamUpload *streamUpload) UploadStream(ctx context.Context, request UploadStreamRequest) (response UploadResponse, err error) {

	uploadID, err := streamUpload.multipartUploader.createMultipartUpload(ctx, request.UploadRequest)

	if err != nil {
		return UploadResponse{}, err
	}
	streamUpload.uploadID = uploadID

	if streamUpload.manifest == nil {
		streamUpload.manifest = &multipartManifest{parts: make(map[string]map[int]uploadPart)}
	}

	// UploadFileMultipart closes the done channel when it returns
	done := make(chan struct{})
	defer close(done)
	parts := streamUpload.manifest.splitStreamToParts(done, *request.PartSize, request.EnableMultipartChecksumVerification, request.StreamReader)

	return streamUpload.startConcurrentUpload(ctx, done, parts, request)
}

func (streamUpload *streamUpload) startConcurrentUpload(ctx context.Context, done <-chan struct{}, parts <-chan uploadPart, request UploadStreamRequest) (response UploadResponse, err error) {
	result := make(chan uploadPart)
	numUploads := *request.NumberOfGoroutines
	var wg sync.WaitGroup
	wg.Add(numUploads)

	// start fixed number of goroutines to upload parts
	for i := 0; i < numUploads; i++ {
		go func() {
			streamUpload.multipartUploader.uploadParts(ctx, done, parts, result, request.UploadRequest, streamUpload.uploadID)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	streamUpload.manifest.updateManifest(result, streamUpload.uploadID)
	// Calculate multipartMD5 once enabled multipart MD5 verification.
	multipartMD5 := streamUpload.manifest.getMultipartMD5Checksum(request.EnableMultipartChecksumVerification, streamUpload.uploadID)

	resp, err := streamUpload.multipartUploader.commit(ctx, request.UploadRequest, streamUpload.manifest.parts[streamUpload.uploadID], streamUpload.uploadID)

	if err != nil {
		common.Debugf("failed to commit with error: %v\n", err)
		return UploadResponse{
				Type:                    MultipartUpload,
				MultipartUploadResponse: &MultipartUploadResponse{UploadID: common.String(streamUpload.uploadID)}},
			err
	}
	if multipartMD5 != nil && *request.EnableMultipartChecksumVerification && strings.Compare(*resp.OpcMultipartMd5, *multipartMD5) != 0 {
		err = fmt.Errorf("multipart base64 MD5 checksum verification failure, the sending opcMD5 is %s, the reveived is %s", *resp.OpcMultipartMd5, *multipartMD5)
		err = uploadManagerError{err: err}
		common.Debugf("MD5 checksum error: %v\n", err)
	}

	response = UploadResponse{
		Type:                    MultipartUpload,
		MultipartUploadResponse: &MultipartUploadResponse{CommitMultipartUploadResponse: resp},
	}
	return
}
