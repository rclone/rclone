/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2019 MinIO, Inc.
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
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v6/pkg/s3utils"
)

// objectRetention - object retention specified in
// https://docs.aws.amazon.com/AmazonS3/latest/API/Type_API_ObjectLockConfiguration.html
type objectRetention struct {
	XMLNS           string        `xml:"xmlns,attr,omitempty"`
	XMLName         xml.Name      `xml:"Retention"`
	Mode            RetentionMode `xml:"Mode"`
	RetainUntilDate time.Time     `type:"timestamp" timestampFormat:"iso8601" xml:"RetainUntilDate"`
}

func newObjectRetention(mode *RetentionMode, date *time.Time) (*objectRetention, error) {
	if mode == nil {
		return nil, fmt.Errorf("Mode not set")
	}

	if date == nil {
		return nil, fmt.Errorf("RetainUntilDate not set")
	}

	if !mode.IsValid() {
		return nil, fmt.Errorf("invalid retention mode `%v`", mode)
	}
	objectRetention := &objectRetention{
		Mode:            *mode,
		RetainUntilDate: *date,
	}
	return objectRetention, nil
}

// PutObjectRetentionOptions represents options specified by user for PutObject call
type PutObjectRetentionOptions struct {
	GovernanceBypass bool
	Mode             *RetentionMode
	RetainUntilDate  *time.Time
	VersionID        string
}

// PutObjectRetention : sets object retention for a given object and versionID.
func (c Client) PutObjectRetention(bucketName, objectName string, opts PutObjectRetentionOptions) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("retention", "")

	if opts.VersionID != "" {
		urlValues.Set("versionId", opts.VersionID)
	}

	retention, err := newObjectRetention(opts.Mode, opts.RetainUntilDate)
	if err != nil {
		return err
	}

	retentionData, err := xml.Marshal(retention)
	if err != nil {
		return err
	}

	// Build headers.
	headers := make(http.Header)

	if opts.GovernanceBypass {
		// Set the bypass goverenance retention header
		headers.Set(amzBypassGovernance, "true")
	}

	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		queryValues:      urlValues,
		contentBody:      bytes.NewReader(retentionData),
		contentLength:    int64(len(retentionData)),
		contentMD5Base64: sumMD5Base64(retentionData),
		contentSHA256Hex: sum256Hex(retentionData),
		customHeader:     headers,
	}

	// Execute PUT Object Retention.
	resp, err := c.executeMethod(context.Background(), "PUT", reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			return httpRespToErrorResponse(resp, bucketName, objectName)
		}
	}
	return nil
}

// GetObjectRetention gets retention of given object.
func (c Client) GetObjectRetention(bucketName, objectName, versionID string) (mode *RetentionMode, retainUntilDate *time.Time, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return nil, nil, err
	}

	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return nil, nil, err
	}
	urlValues := make(url.Values)
	urlValues.Set("retention", "")
	if versionID != "" {
		urlValues.Set("versionId", versionID)
	}
	// Execute GET on bucket to list objects.
	resp, err := c.executeMethod(context.Background(), "GET", requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, nil, httpRespToErrorResponse(resp, bucketName, objectName)
		}
	}
	retention := &objectRetention{}
	if err = xml.NewDecoder(resp.Body).Decode(retention); err != nil {
		return nil, nil, err
	}

	return &retention.Mode, &retention.RetainUntilDate, nil
}
