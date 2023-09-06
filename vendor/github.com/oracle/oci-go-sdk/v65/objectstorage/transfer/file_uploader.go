// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// FileUploader is an interface to upload a file
type FileUploader interface {
	// split file into multiple parts and uploads them to blob storage, then merge
	UploadFileMultiparts(ctx context.Context, request UploadFileRequest) (response UploadResponse, err error)

	// uploads a file to blob storage via PutObject API
	UploadFilePutObject(ctx context.Context, request UploadFileRequest) (response UploadResponse, err error)

	// resume a file upload, use it when UploadFile failed
	ResumeUploadFile(ctx context.Context, uploadID string) (response UploadResponse, err error)
}

type fileUpload struct {
	uploadID          string
	manifest          *multipartManifest
	multipartUploader multipartUploader
	fileUploadReqs    map[string]UploadFileRequest // save user input to resume
}

func (fileUpload *fileUpload) UploadFileMultiparts(ctx context.Context, request UploadFileRequest) (response UploadResponse, err error) {
	file, err := os.Open(request.FilePath)
	defer file.Close()
	if err != nil {
		return
	}

	fi, err := file.Stat()
	if err != nil {
		return
	}

	fileSize := fi.Size()

	uploadID, err := fileUpload.multipartUploader.createMultipartUpload(ctx, request.UploadRequest)

	if err != nil {
		return
	}
	fileUpload.uploadID = uploadID

	if fileUpload.fileUploadReqs == nil {
		fileUpload.fileUploadReqs = make(map[string]UploadFileRequest)
	}

	if fileUpload.manifest == nil {
		fileUpload.manifest = &multipartManifest{parts: make(map[string]map[int]uploadPart)}
	}

	// save the request for later resume if needed
	fileUpload.fileUploadReqs[uploadID] = request

	// UploadFileMultiparts closes the done channel when it returns
	done := make(chan struct{})
	defer close(done)
	parts := fileUpload.manifest.splitFileToParts(done, *request.PartSize, request.EnableMultipartChecksumVerification, file, fileSize)
	response, err = fileUpload.startConcurrentUpload(ctx, done, parts, request)
	return
}

func (fileUpload *fileUpload) UploadFilePutObject(ctx context.Context, request UploadFileRequest) (UploadResponse, error) {
	response := UploadResponse{Type: SinglepartUpload}
	file, err := os.Open(request.FilePath)
	defer file.Close()
	if err != nil {
		return response, err
	}

	fi, err := file.Stat()
	if err != nil {
		return response, err
	}

	fileSize := int64(fi.Size())

	req := objectstorage.PutObjectRequest{
		NamespaceName:           request.NamespaceName,
		BucketName:              request.BucketName,
		ObjectName:              request.ObjectName,
		ContentLength:           common.Int64(fileSize),
		PutObjectBody:           file,
		OpcMeta:                 request.Metadata,
		IfMatch:                 request.IfMatch,
		IfNoneMatch:             request.IfNoneMatch,
		ContentType:             request.ContentType,
		ContentLanguage:         request.ContentLanguage,
		ContentEncoding:         request.ContentEncoding,
		ContentMD5:              request.ContentMD5,
		OpcClientRequestId:      request.OpcClientRequestID,
		RequestMetadata:         request.RequestMetadata,
		StorageTier:             request.StorageTier,
		OpcSseCustomerAlgorithm: request.OpcSseCustomerAlgorithm,
		OpcSseCustomerKey:       request.OpcSseCustomerKey,
		OpcSseCustomerKeySha256: request.OpcSseCustomerKeySha256,
		OpcSseKmsKeyId:          request.OpcSseKmsKeyId,
	}

	resp, err := request.ObjectStorageClient.PutObject(ctx, req)

	if err != nil {
		return response, err
	}

	// set the response
	response.SinglepartUploadResponse = &SinglepartUploadResponse{PutObjectResponse: resp}
	return response, nil
}

func (fileUpload *fileUpload) ResumeUploadFile(ctx context.Context, uploadID string) (UploadResponse, error) {
	response := UploadResponse{Type: MultipartUpload}
	if fileUpload.manifest == nil || fileUpload.manifest.parts == nil {
		err := errors.New("cannot resume upload file, please call UploadFileMultiparts first")
		return response, err
	}

	parts := fileUpload.manifest.parts[uploadID]

	failedParts := []uploadPart{}
	for _, failedPart := range parts {
		if failedPart.err != nil || failedPart.etag == nil {
			failedPart.err = nil // reset the previouse error to nil for resume
			failedParts = append(failedParts, failedPart)
		}
	}

	if len(failedParts) == 0 {
		err := errors.New("previous upload succeed, cannot resume")
		return response, err
	}

	failedPartsChannel := make(chan uploadPart, len(failedParts))
	go func() {
		// close the channel after splitFile returns
		defer func() {
			common.Debugln("closing parts channel from failedPartsChannel")
			close(failedPartsChannel)
		}()

		for _, failedPart := range failedParts {
			failedPartsChannel <- failedPart
		}
	}()

	// ResumeUploadFile closes the done channel when it returns
	done := make(chan struct{})
	defer close(done)

	response, err := fileUpload.startConcurrentUpload(ctx, done, failedPartsChannel, fileUpload.fileUploadReqs[uploadID])
	return response, err
}

func (fileUpload *fileUpload) startConcurrentUpload(ctx context.Context, done <-chan struct{}, parts <-chan uploadPart, request UploadFileRequest) (response UploadResponse, err error) {
	result := make(chan uploadPart)
	numUploads := *request.NumberOfGoroutines
	var wg sync.WaitGroup
	wg.Add(numUploads)

	// start fixed number of goroutines to upload parts
	for i := 0; i < numUploads; i++ {
		go func() {
			fileUpload.multipartUploader.uploadParts(ctx, done, parts, result, request.UploadRequest, fileUpload.uploadID)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	fileUpload.manifest.updateManifest(result, fileUpload.uploadID)
	// Calculate multipartMD5 once enabled multipart MD5 verification.
	multipartMD5 := fileUpload.manifest.getMultipartMD5Checksum(request.EnableMultipartChecksumVerification, fileUpload.uploadID)

	resp, err := fileUpload.multipartUploader.commit(ctx, request.UploadRequest, fileUpload.manifest.parts[fileUpload.uploadID], fileUpload.uploadID)

	if err != nil {
		common.Debugf("failed to commit with error: %v\n", err)
		return UploadResponse{
				Type: MultipartUpload,
				MultipartUploadResponse: &MultipartUploadResponse{
					isResumable: common.Bool(true), UploadID: common.String(fileUpload.uploadID)}},
			err
	}

	if multipartMD5 != nil && *request.EnableMultipartChecksumVerification && strings.Compare(*resp.OpcMultipartMd5, *multipartMD5) != 0 {
		err = fmt.Errorf("multipart base64 MD5 checksum verification failure, the sending opcMD5 is %s, the reveived is %s", *resp.OpcMultipartMd5, *multipartMD5)
		err = uploadManagerError{err: err}
		common.Debugf("MD5 checksum error: %v\n", err)
	}

	response = UploadResponse{
		Type: MultipartUpload,
		MultipartUploadResponse: &MultipartUploadResponse{
			CommitMultipartUploadResponse: resp, UploadID: common.String(fileUpload.uploadID)},
	}
	return
}
