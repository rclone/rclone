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

// RenameObjectDetails To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type RenameObjectDetails struct {

	// The name of the source object to be renamed.
	SourceName *string `mandatory:"true" json:"sourceName"`

	// The new name of the source object. Avoid entering confidential information.
	NewName *string `mandatory:"true" json:"newName"`

	// The if-match entity tag (ETag) of the source object.
	SrcObjIfMatchETag *string `mandatory:"false" json:"srcObjIfMatchETag"`

	// The if-match entity tag (ETag) of the new object.
	NewObjIfMatchETag *string `mandatory:"false" json:"newObjIfMatchETag"`

	// The if-none-match entity tag (ETag) of the new object. The only valid value is '*', which indicates
	// request should fail if the new object already exists.
	NewObjIfNoneMatchETag *string `mandatory:"false" json:"newObjIfNoneMatchETag"`
}

func (m RenameObjectDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m RenameObjectDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
