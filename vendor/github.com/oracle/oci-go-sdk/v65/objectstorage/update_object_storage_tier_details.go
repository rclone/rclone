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

// UpdateObjectStorageTierDetails To change the storage tier of an object, we specify the object name and the desired
// storage tier in the body. Objects can be moved between Standard and InfrequentAccess
// tiers and from Standard or InfrequentAccess tier to Archive tier. If a version id is
// specified, only the specified version of the object is moved to a different storage
// tier, else the current version is used.
type UpdateObjectStorageTierDetails struct {

	// An object for which the storage tier needs to be changed.
	ObjectName *string `mandatory:"true" json:"objectName"`

	// The storage tier that the object should be moved to.
	StorageTier StorageTierEnum `mandatory:"true" json:"storageTier"`

	// The versionId of the object. Current object version is used by default.
	VersionId *string `mandatory:"false" json:"versionId"`
}

func (m UpdateObjectStorageTierDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m UpdateObjectStorageTierDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingStorageTierEnum(string(m.StorageTier)); !ok && m.StorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for StorageTier: %s. Supported values are: %s.", m.StorageTier, strings.Join(GetStorageTierEnumStringValues(), ",")))
	}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
