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

// Bucket A bucket is a container for storing objects in a compartment within a namespace. A bucket is associated with a single compartment.
// The compartment has policies that indicate what actions a user can perform on a bucket and all the objects in the bucket. For more
// information, see Managing Buckets (https://docs.cloud.oracle.com/Content/Object/Tasks/managingbuckets.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type Bucket struct {

	// The Object Storage namespace in which the bucket resides.
	Namespace *string `mandatory:"true" json:"namespace"`

	// The name of the bucket. Avoid entering confidential information.
	// Example: my-new-bucket1
	Name *string `mandatory:"true" json:"name"`

	// The compartment ID in which the bucket is authorized.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// Arbitrary string keys and values for user-defined metadata.
	Metadata map[string]string `mandatory:"true" json:"metadata"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the user who created the bucket.
	CreatedBy *string `mandatory:"true" json:"createdBy"`

	// The date and time the bucket was created, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.29).
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The entity tag (ETag) for the bucket.
	Etag *string `mandatory:"true" json:"etag"`

	// The type of public access enabled on this bucket.
	// A bucket is set to `NoPublicAccess` by default, which only allows an authenticated caller to access the
	// bucket and its contents. When `ObjectRead` is enabled on the bucket, public access is allowed for the
	// `GetObject`, `HeadObject`, and `ListObjects` operations. When `ObjectReadWithoutList` is enabled on the
	// bucket, public access is allowed for the `GetObject` and `HeadObject` operations.
	PublicAccessType BucketPublicAccessTypeEnum `mandatory:"false" json:"publicAccessType,omitempty"`

	// The storage tier type assigned to the bucket. A bucket is set to `Standard` tier by default, which means
	// objects uploaded or copied to the bucket will be in the standard storage tier. When the `Archive` tier type
	// is set explicitly for a bucket, objects uploaded or copied to the bucket will be stored in archive storage.
	// The `storageTier` property is immutable after bucket is created.
	StorageTier BucketStorageTierEnum `mandatory:"false" json:"storageTier,omitempty"`

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

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of a master encryption key used to call the Key Management
	// service to generate a data encryption key or to encrypt or decrypt a data encryption key.
	KmsKeyId *string `mandatory:"false" json:"kmsKeyId"`

	// The entity tag (ETag) for the live object lifecycle policy on the bucket.
	ObjectLifecyclePolicyEtag *string `mandatory:"false" json:"objectLifecyclePolicyEtag"`

	// The approximate number of objects in the bucket. Count statistics are reported periodically. You will see a
	// lag between what is displayed and the actual object count.
	ApproximateCount *int64 `mandatory:"false" json:"approximateCount"`

	// The approximate total size in bytes of all objects in the bucket. Size statistics are reported periodically. You will
	// see a lag between what is displayed and the actual size of the bucket.
	ApproximateSize *int64 `mandatory:"false" json:"approximateSize"`

	// Whether or not this bucket is a replication source. By default, `replicationEnabled` is set to `false`. This will
	// be set to 'true' when you create a replication policy for the bucket.
	ReplicationEnabled *bool `mandatory:"false" json:"replicationEnabled"`

	// Whether or not this bucket is read only. By default, `isReadOnly` is set to `false`. This will
	// be set to 'true' when this bucket is configured as a destination in a replication policy.
	IsReadOnly *bool `mandatory:"false" json:"isReadOnly"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the bucket.
	Id *string `mandatory:"false" json:"id"`

	// The versioning status on the bucket. A bucket is created with versioning `Disabled` by default.
	// For versioning `Enabled`, objects are protected from overwrites and deletes, by maintaining their version history. When versioning is `Suspended`, the previous versions will still remain but new versions will no longer be created when overwitten or deleted.
	Versioning BucketVersioningEnum `mandatory:"false" json:"versioning,omitempty"`

	// The auto tiering status on the bucket. A bucket is created with auto tiering `Disabled` by default.
	// For auto tiering `InfrequentAccess`, objects are transitioned automatically between the 'Standard'
	// and 'InfrequentAccess' tiers based on the access pattern of the objects.
	AutoTiering BucketAutoTieringEnum `mandatory:"false" json:"autoTiering,omitempty"`
}

func (m Bucket) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m Bucket) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingBucketPublicAccessTypeEnum(string(m.PublicAccessType)); !ok && m.PublicAccessType != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for PublicAccessType: %s. Supported values are: %s.", m.PublicAccessType, strings.Join(GetBucketPublicAccessTypeEnumStringValues(), ",")))
	}
	if _, ok := GetMappingBucketStorageTierEnum(string(m.StorageTier)); !ok && m.StorageTier != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for StorageTier: %s. Supported values are: %s.", m.StorageTier, strings.Join(GetBucketStorageTierEnumStringValues(), ",")))
	}
	if _, ok := GetMappingBucketVersioningEnum(string(m.Versioning)); !ok && m.Versioning != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for Versioning: %s. Supported values are: %s.", m.Versioning, strings.Join(GetBucketVersioningEnumStringValues(), ",")))
	}
	if _, ok := GetMappingBucketAutoTieringEnum(string(m.AutoTiering)); !ok && m.AutoTiering != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for AutoTiering: %s. Supported values are: %s.", m.AutoTiering, strings.Join(GetBucketAutoTieringEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// BucketPublicAccessTypeEnum Enum with underlying type: string
type BucketPublicAccessTypeEnum string

// Set of constants representing the allowable values for BucketPublicAccessTypeEnum
const (
	BucketPublicAccessTypeNopublicaccess        BucketPublicAccessTypeEnum = "NoPublicAccess"
	BucketPublicAccessTypeObjectread            BucketPublicAccessTypeEnum = "ObjectRead"
	BucketPublicAccessTypeObjectreadwithoutlist BucketPublicAccessTypeEnum = "ObjectReadWithoutList"
)

var mappingBucketPublicAccessTypeEnum = map[string]BucketPublicAccessTypeEnum{
	"NoPublicAccess":        BucketPublicAccessTypeNopublicaccess,
	"ObjectRead":            BucketPublicAccessTypeObjectread,
	"ObjectReadWithoutList": BucketPublicAccessTypeObjectreadwithoutlist,
}

var mappingBucketPublicAccessTypeEnumLowerCase = map[string]BucketPublicAccessTypeEnum{
	"nopublicaccess":        BucketPublicAccessTypeNopublicaccess,
	"objectread":            BucketPublicAccessTypeObjectread,
	"objectreadwithoutlist": BucketPublicAccessTypeObjectreadwithoutlist,
}

// GetBucketPublicAccessTypeEnumValues Enumerates the set of values for BucketPublicAccessTypeEnum
func GetBucketPublicAccessTypeEnumValues() []BucketPublicAccessTypeEnum {
	values := make([]BucketPublicAccessTypeEnum, 0)
	for _, v := range mappingBucketPublicAccessTypeEnum {
		values = append(values, v)
	}
	return values
}

// GetBucketPublicAccessTypeEnumStringValues Enumerates the set of values in String for BucketPublicAccessTypeEnum
func GetBucketPublicAccessTypeEnumStringValues() []string {
	return []string{
		"NoPublicAccess",
		"ObjectRead",
		"ObjectReadWithoutList",
	}
}

// GetMappingBucketPublicAccessTypeEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingBucketPublicAccessTypeEnum(val string) (BucketPublicAccessTypeEnum, bool) {
	enum, ok := mappingBucketPublicAccessTypeEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// BucketStorageTierEnum Enum with underlying type: string
type BucketStorageTierEnum string

// Set of constants representing the allowable values for BucketStorageTierEnum
const (
	BucketStorageTierStandard BucketStorageTierEnum = "Standard"
	BucketStorageTierArchive  BucketStorageTierEnum = "Archive"
)

var mappingBucketStorageTierEnum = map[string]BucketStorageTierEnum{
	"Standard": BucketStorageTierStandard,
	"Archive":  BucketStorageTierArchive,
}

var mappingBucketStorageTierEnumLowerCase = map[string]BucketStorageTierEnum{
	"standard": BucketStorageTierStandard,
	"archive":  BucketStorageTierArchive,
}

// GetBucketStorageTierEnumValues Enumerates the set of values for BucketStorageTierEnum
func GetBucketStorageTierEnumValues() []BucketStorageTierEnum {
	values := make([]BucketStorageTierEnum, 0)
	for _, v := range mappingBucketStorageTierEnum {
		values = append(values, v)
	}
	return values
}

// GetBucketStorageTierEnumStringValues Enumerates the set of values in String for BucketStorageTierEnum
func GetBucketStorageTierEnumStringValues() []string {
	return []string{
		"Standard",
		"Archive",
	}
}

// GetMappingBucketStorageTierEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingBucketStorageTierEnum(val string) (BucketStorageTierEnum, bool) {
	enum, ok := mappingBucketStorageTierEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// BucketVersioningEnum Enum with underlying type: string
type BucketVersioningEnum string

// Set of constants representing the allowable values for BucketVersioningEnum
const (
	BucketVersioningEnabled   BucketVersioningEnum = "Enabled"
	BucketVersioningSuspended BucketVersioningEnum = "Suspended"
	BucketVersioningDisabled  BucketVersioningEnum = "Disabled"
)

var mappingBucketVersioningEnum = map[string]BucketVersioningEnum{
	"Enabled":   BucketVersioningEnabled,
	"Suspended": BucketVersioningSuspended,
	"Disabled":  BucketVersioningDisabled,
}

var mappingBucketVersioningEnumLowerCase = map[string]BucketVersioningEnum{
	"enabled":   BucketVersioningEnabled,
	"suspended": BucketVersioningSuspended,
	"disabled":  BucketVersioningDisabled,
}

// GetBucketVersioningEnumValues Enumerates the set of values for BucketVersioningEnum
func GetBucketVersioningEnumValues() []BucketVersioningEnum {
	values := make([]BucketVersioningEnum, 0)
	for _, v := range mappingBucketVersioningEnum {
		values = append(values, v)
	}
	return values
}

// GetBucketVersioningEnumStringValues Enumerates the set of values in String for BucketVersioningEnum
func GetBucketVersioningEnumStringValues() []string {
	return []string{
		"Enabled",
		"Suspended",
		"Disabled",
	}
}

// GetMappingBucketVersioningEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingBucketVersioningEnum(val string) (BucketVersioningEnum, bool) {
	enum, ok := mappingBucketVersioningEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// BucketAutoTieringEnum Enum with underlying type: string
type BucketAutoTieringEnum string

// Set of constants representing the allowable values for BucketAutoTieringEnum
const (
	BucketAutoTieringDisabled         BucketAutoTieringEnum = "Disabled"
	BucketAutoTieringInfrequentaccess BucketAutoTieringEnum = "InfrequentAccess"
)

var mappingBucketAutoTieringEnum = map[string]BucketAutoTieringEnum{
	"Disabled":         BucketAutoTieringDisabled,
	"InfrequentAccess": BucketAutoTieringInfrequentaccess,
}

var mappingBucketAutoTieringEnumLowerCase = map[string]BucketAutoTieringEnum{
	"disabled":         BucketAutoTieringDisabled,
	"infrequentaccess": BucketAutoTieringInfrequentaccess,
}

// GetBucketAutoTieringEnumValues Enumerates the set of values for BucketAutoTieringEnum
func GetBucketAutoTieringEnumValues() []BucketAutoTieringEnum {
	values := make([]BucketAutoTieringEnum, 0)
	for _, v := range mappingBucketAutoTieringEnum {
		values = append(values, v)
	}
	return values
}

// GetBucketAutoTieringEnumStringValues Enumerates the set of values in String for BucketAutoTieringEnum
func GetBucketAutoTieringEnumStringValues() []string {
	return []string{
		"Disabled",
		"InfrequentAccess",
	}
}

// GetMappingBucketAutoTieringEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingBucketAutoTieringEnum(val string) (BucketAutoTieringEnum, bool) {
	enum, ok := mappingBucketAutoTieringEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
