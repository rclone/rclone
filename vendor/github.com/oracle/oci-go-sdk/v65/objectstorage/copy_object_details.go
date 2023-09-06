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

// CopyObjectDetails The parameters required by Object Storage to process a request to copy an object to another bucket.
// To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type CopyObjectDetails struct {

	// The name of the object to be copied.
	SourceObjectName *string `mandatory:"true" json:"sourceObjectName"`

	// The destination region the object will be copied to, for example "us-ashburn-1".
	DestinationRegion *string `mandatory:"true" json:"destinationRegion"`

	// The destination Object Storage namespace the object will be copied to.
	DestinationNamespace *string `mandatory:"true" json:"destinationNamespace"`

	// The destination bucket the object will be copied to.
	DestinationBucket *string `mandatory:"true" json:"destinationBucket"`

	// The name of the destination object resulting from the copy operation. Avoid entering confidential information.
	DestinationObjectName *string `mandatory:"true" json:"destinationObjectName"`

	// The entity tag (ETag) to match against that of the source object. Used to confirm that the source object
	// with a given name is the version of that object storing a specified ETag.
	SourceObjectIfMatchETag *string `mandatory:"false" json:"sourceObjectIfMatchETag"`

	// VersionId of the object to copy. If not provided then current version is copied by default.
	SourceVersionId *string `mandatory:"false" json:"sourceVersionId"`

	// The entity tag (ETag) to match against that of the destination object (an object intended to be overwritten).
	// Used to confirm that the destination object stored under a given name is the version of that object
	// storing a specified entity tag.
	DestinationObjectIfMatchETag *string `mandatory:"false" json:"destinationObjectIfMatchETag"`

	// The entity tag (ETag) to avoid matching. The only valid value is '*', which indicates that the request should fail
	// if the object already exists in the destination bucket.
	DestinationObjectIfNoneMatchETag *string `mandatory:"false" json:"destinationObjectIfNoneMatchETag"`

	// Arbitrary string keys and values for the user-defined metadata for the object. Keys must be in
	// "opc-meta-*" format. Avoid entering confidential information. Metadata key-value pairs entered
	// in this field are assigned to the destination object. If you enter no metadata values, the destination
	// object will inherit any existing metadata values associated with the source object.
	DestinationObjectMetadata map[string]string `mandatory:"false" json:"destinationObjectMetadata"`

	// The storage tier that the object should be stored in. If not specified, the object will be stored in
	// the same storage tier as the bucket.
	DestinationObjectStorageTier StorageTierEnum `mandatory:"false" json:"destinationObjectStorageTier,omitempty"`
}

func (m CopyObjectDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m CopyObjectDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingStorageTierEnum(string(m.DestinationObjectStorageTier)); !ok && m.DestinationObjectStorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for DestinationObjectStorageTier: %s. Supported values are: %s.", m.DestinationObjectStorageTier, strings.Join(GetStorageTierEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
