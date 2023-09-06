// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Object Storage Service API
//
// Use Object Storage and Archive Storage APIs to manage buckets, objects, and related resources.
// For more information, see Overview of Object Storage (https://docs.cloud.oracle.com/Content/Object/Concepts/objectstorageoverview.htm) and
// Overview of Archive Storage (https://docs.cloud.oracle.com/Content/Archive/Concepts/archivestorageoverview.htm).
//

package objectstorage

import (
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"strings"
)

// MultipartUpload Multipart uploads provide efficient and resilient uploads, especially for large objects. Multipart uploads also accommodate
// objects that are too large for a single upload operation. With multipart uploads, individual parts of an object can be
// uploaded in parallel to reduce the amount of time you spend uploading. Multipart uploads can also minimize the impact
// of network failures by letting you retry a failed part upload instead of requiring you to retry an entire object upload.
// See Using Multipart Uploads (https://docs.cloud.oracle.com/Content/Object/Tasks/usingmultipartuploads.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type MultipartUpload struct {

	// The Object Storage namespace in which the in-progress multipart upload is stored.
	Namespace *string `mandatory:"true" json:"namespace"`

	// The bucket in which the in-progress multipart upload is stored.
	Bucket *string `mandatory:"true" json:"bucket"`

	// The object name of the in-progress multipart upload.
	Object *string `mandatory:"true" json:"object"`

	// The unique identifier for the in-progress multipart upload.
	UploadId *string `mandatory:"true" json:"uploadId"`

	// The date and time the upload was created, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.29).
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The storage tier that the object is stored in.
	StorageTier StorageTierEnum `mandatory:"false" json:"storageTier,omitempty"`
}

func (m MultipartUpload) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m MultipartUpload) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingStorageTierEnum(string(m.StorageTier)); !ok && m.StorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for StorageTier: %s. Supported values are: %s.", m.StorageTier, strings.Join(GetStorageTierEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
