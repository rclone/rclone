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
	"net/http"

	"github.com/minio/minio-go/v6/pkg/s3utils"
)

// BucketExists verify if bucket exists and you have permission to access it.
func (c Client) BucketExists(bucketName string) (bool, error) {
	return c.BucketExistsWithContext(context.Background(), bucketName)
}

// BucketExistsWithContext verify if bucket exists and you have permission to access it. Allows for a Context to
// control cancellations and timeouts.
func (c Client) BucketExistsWithContext(ctx context.Context, bucketName string) (bool, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return false, err
	}

	// Execute HEAD on bucketName.
	resp, err := c.executeMethod(ctx, "HEAD", requestMetadata{
		bucketName:       bucketName,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		if ToErrorResponse(err).Code == "NoSuchBucket" {
			return false, nil
		}
		return false, err
	}
	if resp != nil {
		resperr := httpRespToErrorResponse(resp, bucketName, "")
		if ToErrorResponse(resperr).Code == "NoSuchBucket" {
			return false, nil
		}
		if resp.StatusCode != http.StatusOK {
			return false, httpRespToErrorResponse(resp, bucketName, "")
		}
	}
	return true, nil
}

// StatObject verifies if object exists and you have permission to access.
func (c Client) StatObject(bucketName, objectName string, opts StatObjectOptions) (ObjectInfo, error) {
	return c.StatObjectWithContext(context.Background(), bucketName, objectName, opts)
}

// StatObjectWithContext verifies if object exists and you have permission to access with a context to control
// cancellations and timeouts.
func (c Client) StatObjectWithContext(ctx context.Context, bucketName, objectName string, opts StatObjectOptions) (ObjectInfo, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return ObjectInfo{}, err
	}
	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return ObjectInfo{}, err
	}
	return c.statObject(ctx, bucketName, objectName, opts)
}

// Lower level API for statObject supporting pre-conditions and range headers.
func (c Client) statObject(ctx context.Context, bucketName, objectName string, opts StatObjectOptions) (ObjectInfo, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return ObjectInfo{}, err
	}
	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return ObjectInfo{}, err
	}

	// Execute HEAD on objectName.
	resp, err := c.executeMethod(ctx, "HEAD", requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		contentSHA256Hex: emptySHA256Hex,
		customHeader:     opts.Header(),
	})
	defer closeResponse(resp)
	if err != nil {
		return ObjectInfo{}, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			return ObjectInfo{}, httpRespToErrorResponse(resp, bucketName, objectName)
		}
	}

	return ToObjectInfo(bucketName, objectName, resp.Header)
}
