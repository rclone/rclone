/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2020 MinIO, Inc.
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
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/minio/minio-go/v6/pkg/s3utils"
)

// GetBucketEncryption - get default encryption configuration for a bucket.
func (c Client) GetBucketEncryption(bucketName string) (ServerSideEncryptionConfiguration, error) {
	return c.GetBucketEncryptionWithContext(context.Background(), bucketName)
}

// GetBucketEncryptionWithContext gets the default encryption configuration on an existing bucket with a context to control cancellations and timeouts.
func (c Client) GetBucketEncryptionWithContext(ctx context.Context, bucketName string) (ServerSideEncryptionConfiguration, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return ServerSideEncryptionConfiguration{}, err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("encryption", "")

	// Execute GET on bucket to get the default encryption configuration.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return ServerSideEncryptionConfiguration{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ServerSideEncryptionConfiguration{}, httpRespToErrorResponse(resp, bucketName, "")
	}

	bucketEncryptionBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ServerSideEncryptionConfiguration{}, err
	}

	encryptionConfig := ServerSideEncryptionConfiguration{}
	if err := xml.Unmarshal(bucketEncryptionBuf, &encryptionConfig); err != nil {
		return ServerSideEncryptionConfiguration{}, err
	}
	return encryptionConfig, nil
}
