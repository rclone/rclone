/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2020 MinIO, Inc.
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

	"github.com/minio/minio-go/v6/pkg/s3utils"
)

// objectLegalHold - object legal hold specified in
// https://docs.aws.amazon.com/AmazonS3/latest/API/archive-RESTObjectPUTLegalHold.html
type objectLegalHold struct {
	XMLNS   string          `xml:"xmlns,attr,omitempty"`
	XMLName xml.Name        `xml:"LegalHold"`
	Status  LegalHoldStatus `xml:"Status,omitempty"`
}

// PutObjectLegalHoldOptions represents options specified by user for PutObjectLegalHold call
type PutObjectLegalHoldOptions struct {
	VersionID string
	Status    *LegalHoldStatus
}

// GetObjectLegalHoldOptions represents options specified by user for GetObjectLegalHold call
type GetObjectLegalHoldOptions struct {
	VersionID string
}

// LegalHoldStatus - object legal hold status.
type LegalHoldStatus string

const (
	// LegalHoldEnabled indicates legal hold is enabled
	LegalHoldEnabled LegalHoldStatus = "ON"

	// LegalHoldDisabled indicates legal hold is disabled
	LegalHoldDisabled LegalHoldStatus = "OFF"
)

func (r LegalHoldStatus) String() string {
	return string(r)
}

// IsValid - check whether this legal hold status is valid or not.
func (r LegalHoldStatus) IsValid() bool {
	return r == LegalHoldEnabled || r == LegalHoldDisabled
}

func newObjectLegalHold(status *LegalHoldStatus) (*objectLegalHold, error) {
	if status == nil {
		return nil, fmt.Errorf("Status not set")
	}
	if !status.IsValid() {
		return nil, fmt.Errorf("invalid legal hold status `%v`", status)
	}
	legalHold := &objectLegalHold{
		Status: *status,
	}
	return legalHold, nil
}

// PutObjectLegalHold : sets object legal hold for a given object and versionID.
func (c Client) PutObjectLegalHold(bucketName, objectName string, opts PutObjectLegalHoldOptions) error {
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
	urlValues.Set("legal-hold", "")

	if opts.VersionID != "" {
		urlValues.Set("versionId", opts.VersionID)
	}

	lh, err := newObjectLegalHold(opts.Status)
	if err != nil {
		return err
	}

	lhData, err := xml.Marshal(lh)
	if err != nil {
		return err
	}

	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		queryValues:      urlValues,
		contentBody:      bytes.NewReader(lhData),
		contentLength:    int64(len(lhData)),
		contentMD5Base64: sumMD5Base64(lhData),
		contentSHA256Hex: sum256Hex(lhData),
	}

	// Execute PUT Object Legal Hold.
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

// GetObjectLegalHold gets legal-hold status of given object.
func (c Client) GetObjectLegalHold(bucketName, objectName string, opts GetObjectLegalHoldOptions) (status *LegalHoldStatus, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return nil, err
	}

	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return nil, err
	}
	urlValues := make(url.Values)
	urlValues.Set("legal-hold", "")

	if opts.VersionID != "" {
		urlValues.Set("versionId", opts.VersionID)
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
		return nil, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, httpRespToErrorResponse(resp, bucketName, objectName)
		}
	}
	lh := &objectLegalHold{}
	if err = xml.NewDecoder(resp.Body).Decode(lh); err != nil {
		return nil, err
	}

	return &lh.Status, nil
}
