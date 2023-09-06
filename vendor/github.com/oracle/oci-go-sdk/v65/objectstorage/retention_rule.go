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

// RetentionRule The details of a retention rule.
type RetentionRule struct {

	// Unique identifier for the retention rule.
	Id *string `mandatory:"true" json:"id"`

	// User specified name for the retention rule.
	DisplayName *string `mandatory:"true" json:"displayName"`

	// The entity tag (ETag) for the retention rule.
	Etag *string `mandatory:"true" json:"etag"`

	// The date and time that the retention rule was created as per RFC3339 (https://tools.ietf.org/html/rfc3339).
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The date and time that the retention rule was modified as per RFC3339 (https://tools.ietf.org/html/rfc3339).
	TimeModified *common.SDKTime `mandatory:"true" json:"timeModified"`

	Duration *Duration `mandatory:"false" json:"duration"`

	// The date and time as per RFC 3339 (https://tools.ietf.org/html/rfc3339) after which this rule becomes locked.
	// and can only be deleted by deleting the bucket.
	TimeRuleLocked *common.SDKTime `mandatory:"false" json:"timeRuleLocked"`
}

func (m RetentionRule) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m RetentionRule) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
