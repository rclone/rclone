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

// RetentionMode - object retention mode.
type RetentionMode string

const (
	// Governance - governance mode.
	Governance RetentionMode = "GOVERNANCE"

	// Compliance - compliance mode.
	Compliance RetentionMode = "COMPLIANCE"
)

func (r RetentionMode) String() string {
	return string(r)
}

// IsValid - check whether this retention mode is valid or not.
func (r RetentionMode) IsValid() bool {
	return r == Governance || r == Compliance
}

// ValidityUnit - retention validity unit.
type ValidityUnit string

const (
	// Days - denotes no. of days.
	Days ValidityUnit = "DAYS"

	// Years - denotes no. of years.
	Years ValidityUnit = "YEARS"
)

func (unit ValidityUnit) String() string {
	return string(unit)
}

// IsValid - check whether this validity unit is valid or not.
func (unit ValidityUnit) isValid() bool {
	return unit == Days || unit == Years
}

// Retention - bucket level retention configuration.
type Retention struct {
	Mode     RetentionMode
	Validity time.Duration
}

func (r Retention) String() string {
	return fmt.Sprintf("{Mode:%v, Validity:%v}", r.Mode, r.Validity)
}

// IsEmpty - returns whether retention is empty or not.
func (r Retention) IsEmpty() bool {
	return r.Mode == "" || r.Validity == 0
}

// objectLockConfig - object lock configuration specified in
// https://docs.aws.amazon.com/AmazonS3/latest/API/Type_API_ObjectLockConfiguration.html
type objectLockConfig struct {
	XMLNS             string   `xml:"xmlns,attr,omitempty"`
	XMLName           xml.Name `xml:"ObjectLockConfiguration"`
	ObjectLockEnabled string   `xml:"ObjectLockEnabled"`
	Rule              *struct {
		DefaultRetention struct {
			Mode  RetentionMode `xml:"Mode"`
			Days  *uint         `xml:"Days"`
			Years *uint         `xml:"Years"`
		} `xml:"DefaultRetention"`
	} `xml:"Rule,omitempty"`
}

func newObjectLockConfig(mode *RetentionMode, validity *uint, unit *ValidityUnit) (*objectLockConfig, error) {
	config := &objectLockConfig{
		ObjectLockEnabled: "Enabled",
	}

	if mode != nil && validity != nil && unit != nil {
		if !mode.IsValid() {
			return nil, fmt.Errorf("invalid retention mode `%v`", mode)
		}

		if !unit.isValid() {
			return nil, fmt.Errorf("invalid validity unit `%v`", unit)
		}

		config.Rule = &struct {
			DefaultRetention struct {
				Mode  RetentionMode `xml:"Mode"`
				Days  *uint         `xml:"Days"`
				Years *uint         `xml:"Years"`
			} `xml:"DefaultRetention"`
		}{}

		config.Rule.DefaultRetention.Mode = *mode
		if *unit == Days {
			config.Rule.DefaultRetention.Days = validity
		} else {
			config.Rule.DefaultRetention.Years = validity
		}

		return config, nil
	}

	if mode == nil && validity == nil && unit == nil {
		return config, nil
	}

	return nil, fmt.Errorf("all of retention mode, validity and validity unit must be passed")
}

// SetBucketObjectLockConfig sets object lock configuration in given bucket. mode, validity and unit are either all set or all nil.
func (c Client) SetBucketObjectLockConfig(bucketName string, mode *RetentionMode, validity *uint, unit *ValidityUnit) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("object-lock", "")

	config, err := newObjectLockConfig(mode, validity, unit)
	if err != nil {
		return err
	}

	configData, err := xml.Marshal(config)
	if err != nil {
		return err
	}

	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentBody:      bytes.NewReader(configData),
		contentLength:    int64(len(configData)),
		contentMD5Base64: sumMD5Base64(configData),
		contentSHA256Hex: sum256Hex(configData),
	}

	// Execute PUT bucket object lock configuration.
	resp, err := c.executeMethod(context.Background(), "PUT", reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return httpRespToErrorResponse(resp, bucketName, "")
		}
	}
	return nil
}

// GetObjectLockConfig gets object lock configuration of given bucket.
func (c Client) GetObjectLockConfig(bucketName string) (objectLock string, mode *RetentionMode, validity *uint, unit *ValidityUnit, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return "", nil, nil, nil, err
	}

	urlValues := make(url.Values)
	urlValues.Set("object-lock", "")

	// Execute GET on bucket to list objects.
	resp, err := c.executeMethod(context.Background(), "GET", requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return "", nil, nil, nil, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return "", nil, nil, nil, httpRespToErrorResponse(resp, bucketName, "")
		}
	}
	config := &objectLockConfig{}
	if err = xml.NewDecoder(resp.Body).Decode(config); err != nil {
		return "", nil, nil, nil, err
	}

	if config.Rule != nil {
		mode = &config.Rule.DefaultRetention.Mode
		if config.Rule.DefaultRetention.Days != nil {
			validity = config.Rule.DefaultRetention.Days
			days := Days
			unit = &days
		} else {
			validity = config.Rule.DefaultRetention.Years
			years := Years
			unit = &years
		}
		return config.ObjectLockEnabled, mode, validity, unit, nil
	}
	return config.ObjectLockEnabled, nil, nil, nil, nil
}

// GetBucketObjectLockConfig gets object lock configuration of given bucket.
func (c Client) GetBucketObjectLockConfig(bucketName string) (mode *RetentionMode, validity *uint, unit *ValidityUnit, err error) {
	_, mode, validity, unit, err = c.GetObjectLockConfig(bucketName)
	return mode, validity, unit, err
}

// SetObjectLockConfig sets object lock configuration in given bucket. mode, validity and unit are either all set or all nil.
func (c Client) SetObjectLockConfig(bucketName string, mode *RetentionMode, validity *uint, unit *ValidityUnit) error {
	return c.SetBucketObjectLockConfig(bucketName, mode, validity, unit)
}
