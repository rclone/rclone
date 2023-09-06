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
	"strings"
)

// StorageTierEnum Enum with underlying type: string
type StorageTierEnum string

// Set of constants representing the allowable values for StorageTierEnum
const (
	StorageTierStandard         StorageTierEnum = "Standard"
	StorageTierInfrequentAccess StorageTierEnum = "InfrequentAccess"
	StorageTierArchive          StorageTierEnum = "Archive"
)

var mappingStorageTierEnum = map[string]StorageTierEnum{
	"Standard":         StorageTierStandard,
	"InfrequentAccess": StorageTierInfrequentAccess,
	"Archive":          StorageTierArchive,
}

var mappingStorageTierEnumLowerCase = map[string]StorageTierEnum{
	"standard":         StorageTierStandard,
	"infrequentaccess": StorageTierInfrequentAccess,
	"archive":          StorageTierArchive,
}

// GetStorageTierEnumValues Enumerates the set of values for StorageTierEnum
func GetStorageTierEnumValues() []StorageTierEnum {
	values := make([]StorageTierEnum, 0)
	for _, v := range mappingStorageTierEnum {
		values = append(values, v)
	}
	return values
}

// GetStorageTierEnumStringValues Enumerates the set of values in String for StorageTierEnum
func GetStorageTierEnumStringValues() []string {
	return []string{
		"Standard",
		"InfrequentAccess",
		"Archive",
	}
}

// GetMappingStorageTierEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingStorageTierEnum(val string) (StorageTierEnum, bool) {
	enum, ok := mappingStorageTierEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
