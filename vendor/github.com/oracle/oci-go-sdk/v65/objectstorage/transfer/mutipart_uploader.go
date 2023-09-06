// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package transfer

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// multipartUploader is an interface wrap the methods talk to object storage service
type multipartUploader interface {
	createMultipartUpload(ctx context.Context, request UploadRequest) (string, error)
	uploadParts(ctx context.Context, done <-chan struct{}, parts <-chan uploadPart, result chan<- uploadPart, request UploadRequest, uploadID string)
	uploadPart(ctx context.Context, request UploadRequest, part uploadPart, uploadID string) (objectstorage.UploadPartResponse, error)
	commit(ctx context.Context, request UploadRequest, parts map[int]uploadPart, uploadID string) (resp objectstorage.CommitMultipartUploadResponse, err error)
}

// multipartUpload implements multipartUploader interface
type multipartUpload struct{}

// createMultipartUpload creates a new multipart upload in Object Storage and return the uploadId
func (uploader *multipartUpload) createMultipartUpload(ctx context.Context, request UploadRequest) (string, error) {
	multipartUploadRequest := objectstorage.CreateMultipartUploadRequest{
		NamespaceName:      request.NamespaceName,
		BucketName:         request.BucketName,
		IfMatch:            request.IfMatch,
		IfNoneMatch:        request.IfNoneMatch,
		OpcClientRequestId: request.OpcClientRequestID,
	}

	multipartUploadRequest.Object = request.ObjectName
	multipartUploadRequest.ContentType = request.ContentType
	multipartUploadRequest.ContentEncoding = request.ContentEncoding
	multipartUploadRequest.ContentLanguage = request.ContentLanguage
	multipartUploadRequest.Metadata = request.Metadata
	multipartUploadRequest.OpcSseCustomerAlgorithm = request.OpcSseCustomerAlgorithm
	multipartUploadRequest.OpcSseCustomerKey = request.OpcSseCustomerKey
	multipartUploadRequest.OpcSseCustomerKeySha256 = request.OpcSseCustomerKeySha256
	multipartUploadRequest.OpcSseKmsKeyId = request.OpcSseKmsKeyId
	switch request.StorageTier {
	case objectstorage.PutObjectStorageTierStandard:
		multipartUploadRequest.StorageTier = objectstorage.StorageTierStandard
	case objectstorage.PutObjectStorageTierArchive:
		multipartUploadRequest.StorageTier = objectstorage.StorageTierArchive
	case objectstorage.PutObjectStorageTierInfrequentaccess:
		multipartUploadRequest.StorageTier = objectstorage.StorageTierInfrequentAccess
	}

	resp, err := request.ObjectStorageClient.CreateMultipartUpload(ctx, multipartUploadRequest)
	if err == nil {
		return *resp.UploadId, nil
	}
	return "", err
}

func (uploader *multipartUpload) uploadParts(ctx context.Context, done <-chan struct{}, parts <-chan uploadPart, result chan<- uploadPart, request UploadRequest, uploadID string) {
	// loop through the part from parts channel created by splitFileParts method
	for part := range parts {
		if part.err != nil {
			// ignore this part which contains error from split function
			result <- part
			return
		}

		resp, err := uploader.uploadPart(ctx, request, part, uploadID)
		if err != nil {
			common.Debugf("upload error %v\n", err)
			part.err = err
		} else {
			part.partBody = nil
		}
		part.etag = resp.ETag
		select {
		case result <- part:
			// Invoke the callBack after upload of each Part
			if nil != request.CallBack {
				uploadedPart := MultiPartUploadPart{
					PartNum:    part.partNum,
					TotalParts: part.totalParts,
					Offset:     part.offset,
					Hash:       part.hash,
					Err:        part.err,
					OpcMD5:     part.opcMD5}

				request.CallBack(uploadedPart)
			}
			common.Debugf("uploadParts resp %v, %v\n", part.partNum, resp.ETag)
		case <-done:
			common.Debugln("uploadParts received Done")
			return
		}
	}
}

// send request to upload part to object storage
func (uploader *multipartUpload) uploadPart(ctx context.Context, request UploadRequest, part uploadPart, uploadID string) (objectstorage.UploadPartResponse, error) {
	req := objectstorage.UploadPartRequest{
		NamespaceName:           request.NamespaceName,
		BucketName:              request.BucketName,
		ObjectName:              request.ObjectName,
		UploadId:                common.String(uploadID),
		UploadPartNum:           common.Int(part.partNum),
		UploadPartBody:          ioutil.NopCloser(bytes.NewReader(part.partBody)),
		ContentLength:           common.Int64(part.size),
		IfMatch:                 request.IfMatch,
		IfNoneMatch:             request.IfNoneMatch,
		OpcClientRequestId:      request.OpcClientRequestID,
		RequestMetadata:         request.RequestMetadata,
		ContentMD5:              part.opcMD5,
		OpcSseCustomerAlgorithm: request.OpcSseCustomerAlgorithm,
		OpcSseCustomerKey:       request.OpcSseCustomerKey,
		OpcSseCustomerKeySha256: request.OpcSseCustomerKeySha256,
		OpcSseKmsKeyId:          request.OpcSseKmsKeyId,
	}

	resp, err := request.ObjectStorageClient.UploadPart(ctx, req)

	return resp, err
}

// commits the multipart upload
func (uploader *multipartUpload) commit(ctx context.Context, request UploadRequest, parts map[int]uploadPart, uploadID string) (resp objectstorage.CommitMultipartUploadResponse, err error) {
	req := objectstorage.CommitMultipartUploadRequest{
		NamespaceName:      request.NamespaceName,
		BucketName:         request.BucketName,
		ObjectName:         request.ObjectName,
		UploadId:           common.String(uploadID),
		IfMatch:            request.IfMatch,
		IfNoneMatch:        request.IfNoneMatch,
		OpcClientRequestId: request.OpcClientRequestID,
		RequestMetadata:    request.RequestMetadata,
	}

	partsToCommit := []objectstorage.CommitMultipartUploadPartDetails{}

	for _, part := range parts {
		if part.etag != nil {
			detail := objectstorage.CommitMultipartUploadPartDetails{
				Etag:    part.etag,
				PartNum: common.Int(part.partNum),
			}

			// update the parts to commit
			partsToCommit = append(partsToCommit, detail)
		} else {
			// some parts failed, return error for resume
			common.Debugf("uploadPart has error: %v\n", part.err)
			err = part.err
			return
		}
	}

	req.PartsToCommit = partsToCommit
	resp, err = request.ObjectStorageClient.CommitMultipartUpload(ctx, req)
	return
}
