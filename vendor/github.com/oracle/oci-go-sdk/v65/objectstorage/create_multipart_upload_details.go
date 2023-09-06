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

// CreateMultipartUploadDetails To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type CreateMultipartUploadDetails struct {

	// The name of the object to which this multi-part upload is targeted. Avoid entering confidential information.
	// Example: test/object1.log
	Object *string `mandatory:"true" json:"object"`

	// The optional Content-Type header that defines the standard MIME type format of the object to upload.
	// Specifying values for this header has no effect on Object Storage behavior. Programs that read the object
	// determine what to do based on the value provided. For example, you could use this header to identify and
	// perform special operations on text only objects.
	ContentType *string `mandatory:"false" json:"contentType"`

	// The optional Content-Language header that defines the content language of the object to upload. Specifying
	// values for this header has no effect on Object Storage behavior. Programs that read the object determine what
	// to do based on the value provided. For example, you could use this header to identify and differentiate objects
	// based on a particular language.
	ContentLanguage *string `mandatory:"false" json:"contentLanguage"`

	// The optional Content-Encoding header that defines the content encodings that were applied to the object to
	// upload. Specifying values for this header has no effect on Object Storage behavior. Programs that read the
	// object determine what to do based on the value provided. For example, you could use this header to determine
	// what decoding mechanisms need to be applied to obtain the media-type specified by the Content-Type header of
	// the object.
	ContentEncoding *string `mandatory:"false" json:"contentEncoding"`

	// The optional Content-Disposition header that defines presentational information for the object to be
	// returned in GetObject and HeadObject responses. Specifying values for this header has no effect on Object
	// Storage behavior. Programs that read the object determine what to do based on the value provided.
	// For example, you could use this header to let users download objects with custom filenames in a browser.
	ContentDisposition *string `mandatory:"false" json:"contentDisposition"`

	// The optional Cache-Control header that defines the caching behavior value to be returned in GetObject and
	// HeadObject responses. Specifying values for this header has no effect on Object Storage behavior. Programs
	// that read the object determine what to do based on the value provided.
	// For example, you could use this header to identify objects that require caching restrictions.
	CacheControl *string `mandatory:"false" json:"cacheControl"`

	// The storage tier that the object should be stored in. If not specified, the object will be stored in
	// the same storage tier as the bucket.
	StorageTier StorageTierEnum `mandatory:"false" json:"storageTier,omitempty"`

	// Arbitrary string keys and values for the user-defined metadata for the object.
	// Keys must be in "opc-meta-*" format. Avoid entering confidential information.
	Metadata map[string]string `mandatory:"false" json:"metadata"`
}

func (m CreateMultipartUploadDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m CreateMultipartUploadDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingStorageTierEnum(string(m.StorageTier)); !ok && m.StorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for StorageTier: %s. Supported values are: %s.", m.StorageTier, strings.Join(GetStorageTierEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
