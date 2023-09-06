// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

// Package transfer simplifies interaction with the Object Storage service by abstracting away the method used
// to upload objects.  Depending on the configuration parameters, UploadManager may choose to do a single
// put_object request, or break up the upload into multiple parts and utilize multi-part uploads.
//
// An advantage of using multi-part uploads is the ability to retry individual failed parts, as well as being
// able to upload parts in parallel to reduce upload time.
//
// To use this package, you must be authorized in an IAM policy. If you're not authorized, talk to an administrator.
package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// UploadManager is the interface that groups the upload methods
type UploadManager struct {
	FileUploader   FileUploader
	StreamUploader StreamUploader
}

var (
	errorInvalidStreamUploader = uploadManagerError{err: errors.New("streamUploader is required, use NewUploadManager for default implementation")}
	errorInvalidFileUploader   = uploadManagerError{err: errors.New("fileUploader is required, use NewUploadManager for default implementation")}
)

// NewUploadManager return a pointer to UploadManager
func NewUploadManager() *UploadManager {
	return &UploadManager{
		FileUploader:   &fileUpload{multipartUploader: &multipartUpload{}},
		StreamUploader: &streamUpload{multipartUploader: &multipartUpload{}},
	}
}

// UploadFile uploads an object to Object Storage. Depending on the options provided and the
// size of the object, the object may be uploaded in multiple parts or just an signle object.
func (uploadManager *UploadManager) UploadFile(ctx context.Context, request UploadFileRequest) (response UploadResponse, err error) {
	if err = request.validate(); err != nil {
		return
	}

	if err = request.initDefaultValues(); err != nil {
		return
	}

	if uploadManager.FileUploader == nil {
		err = errorInvalidFileUploader
		return
	}

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

	// parallel upload disabled by user or the file size smaller than or equal to partSize
	// use UploadFilePutObject
	if !*request.AllowMultipartUploads ||
		fileSize <= *request.PartSize {
		response, err = uploadManager.FileUploader.UploadFilePutObject(ctx, request)
		return
	}

	response, err = uploadManager.FileUploader.UploadFileMultiparts(ctx, request)
	return
}

// ResumeUploadFile resumes a multipart file upload.
func (uploadManager *UploadManager) ResumeUploadFile(ctx context.Context, uploadID string) (response UploadResponse, err error) {
	if len(strings.TrimSpace(uploadID)) == 0 {
		err = errors.New("uploadID is required to resume a multipart file upload")
		err = uploadManagerError{err: err}
		return
	}
	response, err = uploadManager.FileUploader.ResumeUploadFile(ctx, uploadID)
	return
}

// UploadStream uploads streaming data to Object Storage. If the stream is non-empty, this will always perform a
// multipart upload, splitting parts based on the part size (10 MiB if none specified). If the stream is empty,
// this will upload a single empty object to Object Storage.
// Stream uploads are not currently resumable.
func (uploadManager *UploadManager) UploadStream(ctx context.Context, request UploadStreamRequest) (response UploadResponse, err error) {
	if err = request.validate(); err != nil {
		return
	}

	if err = request.initDefaultValues(); err != nil {
		return
	}

	if uploadManager.StreamUploader == nil {
		err = errorInvalidStreamUploader
		return
	}
	//check if the stream is empty
	if isZeroLength(request.StreamReader) {
		return uploadEmptyStream(ctx, request)
	}

	response, err = uploadManager.StreamUploader.UploadStream(ctx, request)
	return
}

func isZeroLength(streamReader io.Reader) bool {
	switch v := streamReader.(type) {
	case *bytes.Buffer:
		return v.Len() == 0
	case *bytes.Reader:
		return v.Len() == 0
	case *strings.Reader:
		return v.Len() == 0
	case *os.File:
		fi, err := v.Stat()
		if err != nil {
			return false
		}
		return fi.Size() == 0
	default:
		return false
	}
}

func uploadEmptyStream(ctx context.Context, request UploadStreamRequest) (response UploadResponse, err error) {
	putObjReq := objectstorage.PutObjectRequest{
		NamespaceName:      request.UploadRequest.NamespaceName,
		BucketName:         request.UploadRequest.BucketName,
		ObjectName:         request.UploadRequest.ObjectName,
		ContentLength:      new(int64),
		PutObjectBody:      http.NoBody,
		OpcMeta:            request.UploadRequest.Metadata,
		IfMatch:            request.UploadRequest.IfMatch,
		IfNoneMatch:        request.UploadRequest.IfNoneMatch,
		ContentType:        request.UploadRequest.ContentType,
		ContentLanguage:    request.UploadRequest.ContentLanguage,
		ContentEncoding:    request.UploadRequest.ContentEncoding,
		ContentMD5:         request.UploadRequest.ContentMD5,
		OpcClientRequestId: request.UploadRequest.OpcClientRequestID,
		RequestMetadata:    request.UploadRequest.RequestMetadata,
	}
	putObjResp, err := request.UploadRequest.ObjectStorageClient.PutObject(ctx, putObjReq)
	spUploadResp := SinglepartUploadResponse{putObjResp}
	return UploadResponse{SinglepartUpload, &spUploadResp, nil}, err
}

func getUploadManagerRetryPolicy() *common.RetryPolicy {
	attempts := uint(3)
	retryOnAllNon200ResponseCodes := func(r common.OCIOperationResponse) bool {
		return !(r.Error == nil && 199 < r.Response.HTTPResponse().StatusCode && r.Response.HTTPResponse().StatusCode < 300)
	}

	policy := common.NewRetryPolicyWithOptions(
		// since this retries on ANY non-2xx response, we don't need special handling for eventual consistency
		common.ReplaceWithValuesFromRetryPolicy(common.DefaultRetryPolicyWithoutEventualConsistency()),
		common.WithMaximumNumberAttempts(attempts),
		common.WithShouldRetryOperation(retryOnAllNon200ResponseCodes),
	)

	return &policy
}

type uploadManagerError struct {
	err error
}

func (ume uploadManagerError) Error() string {
	return fmt.Sprintf("%s\nClient Version: %s, OS Version: %s/%s\nSee https://docs.oracle.com/iaas/Content/API/Concepts/sdk_troubleshooting.htm for common issues and steps to resolve them. If you need to contact support, or file a GitHub issue, please include this full error message.", ume.err, common.Version(), runtime.GOOS, runtime.Version())
}
