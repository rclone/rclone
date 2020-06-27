/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2017 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package minio

import (
	"context"
	"io"
	"net/http"

	"github.com/minio/minio-go/v6/pkg/encrypt"
)

// Core - Inherits Client and adds new methods to expose the low level S3 APIs.
type Core struct {
	*Client
}

// NewCore - Returns new initialized a Core client, this CoreClient should be
// only used under special conditions such as need to access lower primitives
// and being able to use them to write your own wrappers.
func NewCore(endpoint string, accessKeyID, secretAccessKey string, secure bool) (*Core, error) {
	var s3Client Core
	client, err := NewV4(endpoint, accessKeyID, secretAccessKey, secure)
	if err != nil {
		return nil, err
	}
	s3Client.Client = client
	return &s3Client, nil
}

// ListObjects - List all the objects at a prefix, optionally with marker and delimiter
// you can further filter the results.
func (c Core) ListObjects(bucket, prefix, marker, delimiter string, maxKeys int) (result ListBucketResult, err error) {
	return c.listObjectsQuery(bucket, prefix, marker, delimiter, maxKeys)
}

// ListObjectsV2 - Lists all the objects at a prefix, similar to ListObjects() but uses
// continuationToken instead of marker to support iteration over the results.
func (c Core) ListObjectsV2(bucketName, objectPrefix, continuationToken string, fetchOwner bool, delimiter string, maxkeys int, startAfter string) (ListBucketV2Result, error) {
	return c.listObjectsV2Query(bucketName, objectPrefix, continuationToken, fetchOwner, false, delimiter, maxkeys, startAfter)
}

// CopyObjectWithContext - copies an object from source object to destination object on server side.
func (c Core) CopyObjectWithContext(ctx context.Context, sourceBucket, sourceObject, destBucket, destObject string, metadata map[string]string) (ObjectInfo, error) {
	return c.copyObjectDo(ctx, sourceBucket, sourceObject, destBucket, destObject, metadata)
}

// CopyObject - copies an object from source object to destination object on server side.
func (c Core) CopyObject(sourceBucket, sourceObject, destBucket, destObject string, metadata map[string]string) (ObjectInfo, error) {
	return c.CopyObjectWithContext(context.Background(), sourceBucket, sourceObject, destBucket, destObject, metadata)
}

// CopyObjectPartWithContext - creates a part in a multipart upload by copying (a
// part of) an existing object.
func (c Core) CopyObjectPartWithContext(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string,
	partID int, startOffset, length int64, metadata map[string]string) (p CompletePart, err error) {

	return c.copyObjectPartDo(ctx, srcBucket, srcObject, destBucket, destObject, uploadID,
		partID, startOffset, length, metadata)
}

// CopyObjectPart - creates a part in a multipart upload by copying (a
// part of) an existing object.
func (c Core) CopyObjectPart(srcBucket, srcObject, destBucket, destObject string, uploadID string,
	partID int, startOffset, length int64, metadata map[string]string) (p CompletePart, err error) {

	return c.CopyObjectPartWithContext(context.Background(), srcBucket, srcObject, destBucket, destObject, uploadID,
		partID, startOffset, length, metadata)
}

// PutObjectWithContext - Upload object. Uploads using single PUT call.
func (c Core) PutObjectWithContext(ctx context.Context, bucket, object string, data io.Reader, size int64, md5Base64, sha256Hex string, opts PutObjectOptions) (ObjectInfo, error) {
	return c.putObjectDo(ctx, bucket, object, data, md5Base64, sha256Hex, size, opts)
}

// PutObject - Upload object. Uploads using single PUT call.
func (c Core) PutObject(bucket, object string, data io.Reader, size int64, md5Base64, sha256Hex string, opts PutObjectOptions) (ObjectInfo, error) {
	return c.PutObjectWithContext(context.Background(), bucket, object, data, size, md5Base64, sha256Hex, opts)
}

// NewMultipartUpload - Initiates new multipart upload and returns the new uploadID.
func (c Core) NewMultipartUpload(bucket, object string, opts PutObjectOptions) (uploadID string, err error) {
	result, err := c.initiateMultipartUpload(context.Background(), bucket, object, opts)
	return result.UploadID, err
}

// ListMultipartUploads - List incomplete uploads.
func (c Core) ListMultipartUploads(bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result ListMultipartUploadsResult, err error) {
	return c.listMultipartUploadsQuery(bucket, keyMarker, uploadIDMarker, prefix, delimiter, maxUploads)
}

// PutObjectPartWithContext - Upload an object part.
func (c Core) PutObjectPartWithContext(ctx context.Context, bucket, object, uploadID string, partID int, data io.Reader, size int64, md5Base64, sha256Hex string, sse encrypt.ServerSide) (ObjectPart, error) {
	return c.uploadPart(ctx, bucket, object, uploadID, data, partID, md5Base64, sha256Hex, size, sse)
}

// PutObjectPart - Upload an object part.
func (c Core) PutObjectPart(bucket, object, uploadID string, partID int, data io.Reader, size int64, md5Base64, sha256Hex string, sse encrypt.ServerSide) (ObjectPart, error) {
	return c.PutObjectPartWithContext(context.Background(), bucket, object, uploadID, partID, data, size, md5Base64, sha256Hex, sse)
}

// ListObjectParts - List uploaded parts of an incomplete upload.x
func (c Core) ListObjectParts(bucket, object, uploadID string, partNumberMarker int, maxParts int) (result ListObjectPartsResult, err error) {
	return c.listObjectPartsQuery(bucket, object, uploadID, partNumberMarker, maxParts)
}

// CompleteMultipartUploadWithContext - Concatenate uploaded parts and commit to an object.
func (c Core) CompleteMultipartUploadWithContext(ctx context.Context, bucket, object, uploadID string, parts []CompletePart) (string, error) {
	res, err := c.completeMultipartUpload(ctx, bucket, object, uploadID, completeMultipartUpload{
		Parts: parts,
	})
	return res.ETag, err
}

// CompleteMultipartUpload - Concatenate uploaded parts and commit to an object.
func (c Core) CompleteMultipartUpload(bucket, object, uploadID string, parts []CompletePart) (string, error) {
	return c.CompleteMultipartUploadWithContext(context.Background(), bucket, object, uploadID, parts)
}

// AbortMultipartUploadWithContext - Abort an incomplete upload.
func (c Core) AbortMultipartUploadWithContext(ctx context.Context, bucket, object, uploadID string) error {
	return c.abortMultipartUpload(ctx, bucket, object, uploadID)
}

// AbortMultipartUpload - Abort an incomplete upload.
func (c Core) AbortMultipartUpload(bucket, object, uploadID string) error {
	return c.AbortMultipartUploadWithContext(context.Background(), bucket, object, uploadID)
}

// GetBucketPolicy - fetches bucket access policy for a given bucket.
func (c Core) GetBucketPolicy(bucket string) (string, error) {
	return c.getBucketPolicy(bucket)
}

// PutBucketPolicy - applies a new bucket access policy for a given bucket.
func (c Core) PutBucketPolicy(bucket, bucketPolicy string) error {
	return c.PutBucketPolicyWithContext(context.Background(), bucket, bucketPolicy)
}

// PutBucketPolicyWithContext - applies a new bucket access policy for a given bucket with a context to control
// cancellations and timeouts.
func (c Core) PutBucketPolicyWithContext(ctx context.Context, bucket, bucketPolicy string) error {
	return c.putBucketPolicy(ctx, bucket, bucketPolicy)
}

// GetObjectWithContext is a lower level API implemented to support reading
// partial objects and also downloading objects with special conditions
// matching etag, modtime etc.
func (c Core) GetObjectWithContext(ctx context.Context, bucketName, objectName string, opts GetObjectOptions) (io.ReadCloser, ObjectInfo, http.Header, error) {
	return c.getObject(ctx, bucketName, objectName, opts)
}

// GetObject is a lower level API implemented to support reading
// partial objects and also downloading objects with special conditions
// matching etag, modtime etc.
func (c Core) GetObject(bucketName, objectName string, opts GetObjectOptions) (io.ReadCloser, ObjectInfo, http.Header, error) {
	return c.GetObjectWithContext(context.Background(), bucketName, objectName, opts)
}

// StatObjectWithContext is a lower level API implemented to support special
// conditions matching etag, modtime on a request.
func (c Core) StatObjectWithContext(ctx context.Context, bucketName, objectName string, opts StatObjectOptions) (ObjectInfo, error) {
	return c.statObject(ctx, bucketName, objectName, opts)
}

// StatObject is a lower level API implemented to support special
// conditions matching etag, modtime on a request.
func (c Core) StatObject(bucketName, objectName string, opts StatObjectOptions) (ObjectInfo, error) {
	return c.StatObjectWithContext(context.Background(), bucketName, objectName, opts)
}
