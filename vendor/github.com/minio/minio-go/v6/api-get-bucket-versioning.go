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

// BucketVersioningConfiguration is the versioning configuration structure
type BucketVersioningConfiguration struct {
	XMLName   xml.Name `xml:"VersioningConfiguration"`
	Status    string   `xml:"Status"`
	MfaDelete string   `xml:"MfaDelete,omitempty"`
}

// GetBucketVersioning - get versioning configuration for a bucket.
func (c Client) GetBucketVersioning(bucketName string) (BucketVersioningConfiguration, error) {
	return c.GetBucketVersioningWithContext(context.Background(), bucketName)
}

// GetBucketVersioningWithContext gets the versioning configuration on an existing bucket with a context to control cancellations and timeouts.
func (c Client) GetBucketVersioningWithContext(ctx context.Context, bucketName string) (BucketVersioningConfiguration, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return BucketVersioningConfiguration{}, err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("versioning", "")

	// Execute GET on bucket to get the versioning configuration.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return BucketVersioningConfiguration{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return BucketVersioningConfiguration{}, httpRespToErrorResponse(resp, bucketName, "")
	}

	bucketVersioningBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return BucketVersioningConfiguration{}, err
	}

	versioningConfig := BucketVersioningConfiguration{}
	if err := xml.Unmarshal(bucketVersioningBuf, &versioningConfig); err != nil {
		return BucketVersioningConfiguration{}, err
	}
	return versioningConfig, nil
}
