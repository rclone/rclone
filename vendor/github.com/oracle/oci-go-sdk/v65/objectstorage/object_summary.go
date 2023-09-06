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

// ObjectSummary To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type ObjectSummary struct {

	// The name of the object. Avoid entering confidential information.
	// Example: test/object1.log
	Name *string `mandatory:"true" json:"name"`

	// Size of the object in bytes.
	Size *int64 `mandatory:"false" json:"size"`

	// Base64-encoded MD5 hash of the object data.
	Md5 *string `mandatory:"false" json:"md5"`

	// The date and time the object was created, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.29).
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`

	// The current entity tag (ETag) for the object.
	Etag *string `mandatory:"false" json:"etag"`

	// The storage tier that the object is stored in.
	StorageTier StorageTierEnum `mandatory:"false" json:"storageTier,omitempty"`

	// Archival state of an object. This field is set only for objects in Archive tier.
	ArchivalState ArchivalStateEnum `mandatory:"false" json:"archivalState,omitempty"`

	// The date and time the object was modified, as described in RFC 2616 (https://tools.ietf.org/rfc/rfc2616), section 14.29.
	TimeModified *common.SDKTime `mandatory:"false" json:"timeModified"`
}

func (m ObjectSummary) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m ObjectSummary) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingStorageTierEnum(string(m.StorageTier)); !ok && m.StorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for StorageTier: %s. Supported values are: %s.", m.StorageTier, strings.Join(GetStorageTierEnumStringValues(), ",")))
	}
	if _, ok := GetMappingArchivalStateEnum(string(m.ArchivalState)); !ok && m.ArchivalState != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for ArchivalState: %s. Supported values are: %s.", m.ArchivalState, strings.Join(GetArchivalStateEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
