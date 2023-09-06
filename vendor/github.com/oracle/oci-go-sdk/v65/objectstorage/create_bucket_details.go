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

// CreateBucketDetails To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type CreateBucketDetails struct {

	// The name of the bucket. Valid characters are uppercase or lowercase letters, numbers, hyphens, underscores, and periods.
	// Bucket names must be unique within an Object Storage namespace. Avoid entering confidential information.
	// example: Example: my-new-bucket1
	Name *string `mandatory:"true" json:"name"`

	// The ID of the compartment in which to create the bucket.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// Arbitrary string, up to 4KB, of keys and values for user-defined metadata.
	Metadata map[string]string `mandatory:"false" json:"metadata"`

	// The type of public access enabled on this bucket.
	// A bucket is set to `NoPublicAccess` by default, which only allows an authenticated caller to access the
	// bucket and its contents. When `ObjectRead` is enabled on the bucket, public access is allowed for the
	// `GetObject`, `HeadObject`, and `ListObjects` operations. When `ObjectReadWithoutList` is enabled on the bucket,
	// public access is allowed for the `GetObject` and `HeadObject` operations.
	PublicAccessType CreateBucketDetailsPublicAccessTypeEnum `mandatory:"false" json:"publicAccessType,omitempty"`

	// The type of storage tier of this bucket.
	// A bucket is set to 'Standard' tier by default, which means the bucket will be put in the standard storage tier.
	// When 'Archive' tier type is set explicitly, the bucket is put in the Archive Storage tier. The 'storageTier'
	// property is immutable after bucket is created.
	StorageTier CreateBucketDetailsStorageTierEnum `mandatory:"false" json:"storageTier,omitempty"`

	// Whether or not events are emitted for object state changes in this bucket. By default, `objectEventsEnabled` is
	// set to `false`. Set `objectEventsEnabled` to `true` to emit events for object state changes. For more information
	// about events, see Overview of Events (https://docs.cloud.oracle.com/Content/Events/Concepts/eventsoverview.htm).
	ObjectEventsEnabled *bool `mandatory:"false" json:"objectEventsEnabled"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no predefined name, type, or namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// Defined tags for this resource. Each key is predefined and scoped to a namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of a master encryption key used to call the Key
	// Management service to generate a data encryption key or to encrypt or decrypt a data encryption key.
	KmsKeyId *string `mandatory:"false" json:"kmsKeyId"`

	// Set the versioning status on the bucket. By default, a bucket is created with versioning `Disabled`. Use this option to enable versioning during bucket creation. Objects in a version enabled bucket are protected from overwrites and deletions. Previous versions of the same object will be available in the bucket.
	Versioning CreateBucketDetailsVersioningEnum `mandatory:"false" json:"versioning,omitempty"`

	// Set the auto tiering status on the bucket. By default, a bucket is created with auto tiering `Disabled`.
	// Use this option to enable auto tiering during bucket creation. Objects in a bucket with auto tiering set to
	// `InfrequentAccess` are transitioned automatically between the 'Standard' and 'InfrequentAccess'
	// tiers based on the access pattern of the objects.
	AutoTiering BucketAutoTieringEnum `mandatory:"false" json:"autoTiering,omitempty"`
}

func (m CreateBucketDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m CreateBucketDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingCreateBucketDetailsPublicAccessTypeEnum(string(m.PublicAccessType)); !ok && m.PublicAccessType != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for PublicAccessType: %s. Supported values are: %s.", m.PublicAccessType, strings.Join(GetCreateBucketDetailsPublicAccessTypeEnumStringValues(), ",")))
	}
	if _, ok := GetMappingCreateBucketDetailsStorageTierEnum(string(m.StorageTier)); !ok && m.StorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for StorageTier: %s. Supported values are: %s.", m.StorageTier, strings.Join(GetCreateBucketDetailsStorageTierEnumStringValues(), ",")))
	}
	if _, ok := GetMappingCreateBucketDetailsVersioningEnum(string(m.Versioning)); !ok && m.Versioning != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for Versioning: %s. Supported values are: %s.", m.Versioning, strings.Join(GetCreateBucketDetailsVersioningEnumStringValues(), ",")))
	}
	if _, ok := GetMappingBucketAutoTieringEnum(string(m.AutoTiering)); !ok && m.AutoTiering != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for AutoTiering: %s. Supported values are: %s.", m.AutoTiering, strings.Join(GetBucketAutoTieringEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// CreateBucketDetailsPublicAccessTypeEnum Enum with underlying type: string
type CreateBucketDetailsPublicAccessTypeEnum string

// Set of constants representing the allowable values for CreateBucketDetailsPublicAccessTypeEnum
const (
	CreateBucketDetailsPublicAccessTypeNopublicaccess        CreateBucketDetailsPublicAccessTypeEnum = "NoPublicAccess"
	CreateBucketDetailsPublicAccessTypeObjectread            CreateBucketDetailsPublicAccessTypeEnum = "ObjectRead"
	CreateBucketDetailsPublicAccessTypeObjectreadwithoutlist CreateBucketDetailsPublicAccessTypeEnum = "ObjectReadWithoutList"
)

var mappingCreateBucketDetailsPublicAccessTypeEnum = map[string]CreateBucketDetailsPublicAccessTypeEnum{
	"NoPublicAccess":        CreateBucketDetailsPublicAccessTypeNopublicaccess,
	"ObjectRead":            CreateBucketDetailsPublicAccessTypeObjectread,
	"ObjectReadWithoutList": CreateBucketDetailsPublicAccessTypeObjectreadwithoutlist,
}

var mappingCreateBucketDetailsPublicAccessTypeEnumLowerCase = map[string]CreateBucketDetailsPublicAccessTypeEnum{
	"nopublicaccess":        CreateBucketDetailsPublicAccessTypeNopublicaccess,
	"objectread":            CreateBucketDetailsPublicAccessTypeObjectread,
	"objectreadwithoutlist": CreateBucketDetailsPublicAccessTypeObjectreadwithoutlist,
}

// GetCreateBucketDetailsPublicAccessTypeEnumValues Enumerates the set of values for CreateBucketDetailsPublicAccessTypeEnum
func GetCreateBucketDetailsPublicAccessTypeEnumValues() []CreateBucketDetailsPublicAccessTypeEnum {
	values := make([]CreateBucketDetailsPublicAccessTypeEnum, 0)
	for _, v := range mappingCreateBucketDetailsPublicAccessTypeEnum {
		values = append(values, v)
	}
	return values
}

// GetCreateBucketDetailsPublicAccessTypeEnumStringValues Enumerates the set of values in String for CreateBucketDetailsPublicAccessTypeEnum
func GetCreateBucketDetailsPublicAccessTypeEnumStringValues() []string {
	return []string{
		"NoPublicAccess",
		"ObjectRead",
		"ObjectReadWithoutList",
	}
}

// GetMappingCreateBucketDetailsPublicAccessTypeEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingCreateBucketDetailsPublicAccessTypeEnum(val string) (CreateBucketDetailsPublicAccessTypeEnum, bool) {
	enum, ok := mappingCreateBucketDetailsPublicAccessTypeEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// CreateBucketDetailsStorageTierEnum Enum with underlying type: string
type CreateBucketDetailsStorageTierEnum string

// Set of constants representing the allowable values for CreateBucketDetailsStorageTierEnum
const (
	CreateBucketDetailsStorageTierStandard CreateBucketDetailsStorageTierEnum = "Standard"
	CreateBucketDetailsStorageTierArchive  CreateBucketDetailsStorageTierEnum = "Archive"
)

var mappingCreateBucketDetailsStorageTierEnum = map[string]CreateBucketDetailsStorageTierEnum{
	"Standard": CreateBucketDetailsStorageTierStandard,
	"Archive":  CreateBucketDetailsStorageTierArchive,
}

var mappingCreateBucketDetailsStorageTierEnumLowerCase = map[string]CreateBucketDetailsStorageTierEnum{
	"standard": CreateBucketDetailsStorageTierStandard,
	"archive":  CreateBucketDetailsStorageTierArchive,
}

// GetCreateBucketDetailsStorageTierEnumValues Enumerates the set of values for CreateBucketDetailsStorageTierEnum
func GetCreateBucketDetailsStorageTierEnumValues() []CreateBucketDetailsStorageTierEnum {
	values := make([]CreateBucketDetailsStorageTierEnum, 0)
	for _, v := range mappingCreateBucketDetailsStorageTierEnum {
		values = append(values, v)
	}
	return values
}

// GetCreateBucketDetailsStorageTierEnumStringValues Enumerates the set of values in String for CreateBucketDetailsStorageTierEnum
func GetCreateBucketDetailsStorageTierEnumStringValues() []string {
	return []string{
		"Standard",
		"Archive",
	}
}

// GetMappingCreateBucketDetailsStorageTierEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingCreateBucketDetailsStorageTierEnum(val string) (CreateBucketDetailsStorageTierEnum, bool) {
	enum, ok := mappingCreateBucketDetailsStorageTierEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// CreateBucketDetailsVersioningEnum Enum with underlying type: string
type CreateBucketDetailsVersioningEnum string

// Set of constants representing the allowable values for CreateBucketDetailsVersioningEnum
const (
	CreateBucketDetailsVersioningEnabled  CreateBucketDetailsVersioningEnum = "Enabled"
	CreateBucketDetailsVersioningDisabled CreateBucketDetailsVersioningEnum = "Disabled"
)

var mappingCreateBucketDetailsVersioningEnum = map[string]CreateBucketDetailsVersioningEnum{
	"Enabled":  CreateBucketDetailsVersioningEnabled,
	"Disabled": CreateBucketDetailsVersioningDisabled,
}

var mappingCreateBucketDetailsVersioningEnumLowerCase = map[string]CreateBucketDetailsVersioningEnum{
	"enabled":  CreateBucketDetailsVersioningEnabled,
	"disabled": CreateBucketDetailsVersioningDisabled,
}

// GetCreateBucketDetailsVersioningEnumValues Enumerates the set of values for CreateBucketDetailsVersioningEnum
func GetCreateBucketDetailsVersioningEnumValues() []CreateBucketDetailsVersioningEnum {
	values := make([]CreateBucketDetailsVersioningEnum, 0)
	for _, v := range mappingCreateBucketDetailsVersioningEnum {
		values = append(values, v)
	}
	return values
}

// GetCreateBucketDetailsVersioningEnumStringValues Enumerates the set of values in String for CreateBucketDetailsVersioningEnum
func GetCreateBucketDetailsVersioningEnumStringValues() []string {
	return []string{
		"Enabled",
		"Disabled",
	}
}

// GetMappingCreateBucketDetailsVersioningEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingCreateBucketDetailsVersioningEnum(val string) (CreateBucketDetailsVersioningEnum, bool) {
	enum, ok := mappingCreateBucketDetailsVersioningEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
