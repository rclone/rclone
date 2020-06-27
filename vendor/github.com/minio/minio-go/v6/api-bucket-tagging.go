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
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/minio/minio-go/v6/pkg/s3utils"
	"github.com/minio/minio-go/v6/pkg/tags"
)

// GetBucketTagging gets tagging configuration for a bucket.
func (c Client) GetBucketTagging(bucketName string) (*tags.Tags, error) {
	return c.GetBucketTaggingWithContext(context.Background(), bucketName)
}

// GetBucketTaggingWithContext gets tagging configuration for a bucket with a context to control cancellations and timeouts.
func (c Client) GetBucketTaggingWithContext(ctx context.Context, bucketName string) (*tags.Tags, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return nil, err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("tagging", "")

	// Execute GET on bucket to get tagging configuration.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp, bucketName, "")
	}

	defer io.Copy(ioutil.Discard, resp.Body)
	return tags.ParseBucketXML(resp.Body)
}

// SetBucketTagging sets tagging configuration for a bucket.
func (c Client) SetBucketTagging(bucketName string, tags *tags.Tags) error {
	return c.SetBucketTaggingWithContext(context.Background(), bucketName, tags)
}

// SetBucketTaggingWithContext sets tagging configuration for a bucket with a context to control cancellations and timeouts.
func (c Client) SetBucketTaggingWithContext(ctx context.Context, bucketName string, tags *tags.Tags) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	if tags == nil {
		return errors.New("nil tags passed")
	}

	buf, err := xml.Marshal(tags)
	if err != nil {
		return err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("tagging", "")

	// Content-length is mandatory to set a default encryption configuration
	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentBody:      bytes.NewReader(buf),
		contentLength:    int64(len(buf)),
		contentMD5Base64: sumMD5Base64(buf),
	}

	// Execute PUT on bucket to put tagging configuration.
	resp, err := c.executeMethod(ctx, http.MethodPut, reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp, bucketName, "")
	}
	return nil
}

// DeleteBucketTagging removes tagging configuration for a bucket.
func (c Client) DeleteBucketTagging(bucketName string) error {
	return c.DeleteBucketTaggingWithContext(context.Background(), bucketName)
}

// DeleteBucketTaggingWithContext removes tagging configuration for a bucket with a context to control cancellations and timeouts.
func (c Client) DeleteBucketTaggingWithContext(ctx context.Context, bucketName string) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("tagging", "")

	// Execute DELETE on bucket to remove tagging configuration.
	resp, err := c.executeMethod(ctx, http.MethodDelete, requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp, bucketName, "")
	}
	return nil
}
