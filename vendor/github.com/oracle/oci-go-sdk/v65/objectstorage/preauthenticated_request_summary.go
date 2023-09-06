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

// PreauthenticatedRequestSummary Get summary information about pre-authenticated requests.
type PreauthenticatedRequestSummary struct {

	// The unique identifier to use when directly addressing the pre-authenticated request.
	Id *string `mandatory:"true" json:"id"`

	// The user-provided name of the pre-authenticated request.
	Name *string `mandatory:"true" json:"name"`

	// The operation that can be performed on this resource.
	AccessType PreauthenticatedRequestSummaryAccessTypeEnum `mandatory:"true" json:"accessType"`

	// The expiration date for the pre-authenticated request as per RFC 3339 (https://tools.ietf.org/html/rfc3339). After this date the pre-authenticated request will no longer be valid.
	TimeExpires *common.SDKTime `mandatory:"true" json:"timeExpires"`

	// The date when the pre-authenticated request was created as per RFC 3339 (https://tools.ietf.org/html/rfc3339).
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The name of object that is being granted access to by the pre-authenticated request. This can be null and if it is,
	// the pre-authenticated request grants access to the entire bucket.
	ObjectName *string `mandatory:"false" json:"objectName"`

	// Specifies whether a list operation is allowed on a PAR with accessType "AnyObjectRead" or "AnyObjectReadWrite".
	// Deny: Prevents the user from performing a list operation.
	// ListObjects: Authorizes the user to perform a list operation.
	BucketListingAction PreauthenticatedRequestBucketListingActionEnum `mandatory:"false" json:"bucketListingAction,omitempty"`
}

func (m PreauthenticatedRequestSummary) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m PreauthenticatedRequestSummary) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingPreauthenticatedRequestSummaryAccessTypeEnum(string(m.AccessType)); !ok && m.AccessType != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for AccessType: %s. Supported values are: %s.", m.AccessType, strings.Join(GetPreauthenticatedRequestSummaryAccessTypeEnumStringValues(), ",")))
	}

	if _, ok := GetMappingPreauthenticatedRequestBucketListingActionEnum(string(m.BucketListingAction)); !ok && m.BucketListingAction != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for BucketListingAction: %s. Supported values are: %s.", m.BucketListingAction, strings.Join(GetPreauthenticatedRequestBucketListingActionEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// PreauthenticatedRequestSummaryAccessTypeEnum Enum with underlying type: string
type PreauthenticatedRequestSummaryAccessTypeEnum string

// Set of constants representing the allowable values for PreauthenticatedRequestSummaryAccessTypeEnum
const (
	PreauthenticatedRequestSummaryAccessTypeObjectread         PreauthenticatedRequestSummaryAccessTypeEnum = "ObjectRead"
	PreauthenticatedRequestSummaryAccessTypeObjectwrite        PreauthenticatedRequestSummaryAccessTypeEnum = "ObjectWrite"
	PreauthenticatedRequestSummaryAccessTypeObjectreadwrite    PreauthenticatedRequestSummaryAccessTypeEnum = "ObjectReadWrite"
	PreauthenticatedRequestSummaryAccessTypeAnyobjectwrite     PreauthenticatedRequestSummaryAccessTypeEnum = "AnyObjectWrite"
	PreauthenticatedRequestSummaryAccessTypeAnyobjectread      PreauthenticatedRequestSummaryAccessTypeEnum = "AnyObjectRead"
	PreauthenticatedRequestSummaryAccessTypeAnyobjectreadwrite PreauthenticatedRequestSummaryAccessTypeEnum = "AnyObjectReadWrite"
)

var mappingPreauthenticatedRequestSummaryAccessTypeEnum = map[string]PreauthenticatedRequestSummaryAccessTypeEnum{
	"ObjectRead":         PreauthenticatedRequestSummaryAccessTypeObjectread,
	"ObjectWrite":        PreauthenticatedRequestSummaryAccessTypeObjectwrite,
	"ObjectReadWrite":    PreauthenticatedRequestSummaryAccessTypeObjectreadwrite,
	"AnyObjectWrite":     PreauthenticatedRequestSummaryAccessTypeAnyobjectwrite,
	"AnyObjectRead":      PreauthenticatedRequestSummaryAccessTypeAnyobjectread,
	"AnyObjectReadWrite": PreauthenticatedRequestSummaryAccessTypeAnyobjectreadwrite,
}

var mappingPreauthenticatedRequestSummaryAccessTypeEnumLowerCase = map[string]PreauthenticatedRequestSummaryAccessTypeEnum{
	"objectread":         PreauthenticatedRequestSummaryAccessTypeObjectread,
	"objectwrite":        PreauthenticatedRequestSummaryAccessTypeObjectwrite,
	"objectreadwrite":    PreauthenticatedRequestSummaryAccessTypeObjectreadwrite,
	"anyobjectwrite":     PreauthenticatedRequestSummaryAccessTypeAnyobjectwrite,
	"anyobjectread":      PreauthenticatedRequestSummaryAccessTypeAnyobjectread,
	"anyobjectreadwrite": PreauthenticatedRequestSummaryAccessTypeAnyobjectreadwrite,
}

// GetPreauthenticatedRequestSummaryAccessTypeEnumValues Enumerates the set of values for PreauthenticatedRequestSummaryAccessTypeEnum
func GetPreauthenticatedRequestSummaryAccessTypeEnumValues() []PreauthenticatedRequestSummaryAccessTypeEnum {
	values := make([]PreauthenticatedRequestSummaryAccessTypeEnum, 0)
	for _, v := range mappingPreauthenticatedRequestSummaryAccessTypeEnum {
		values = append(values, v)
	}
	return values
}

// GetPreauthenticatedRequestSummaryAccessTypeEnumStringValues Enumerates the set of values in String for PreauthenticatedRequestSummaryAccessTypeEnum
func GetPreauthenticatedRequestSummaryAccessTypeEnumStringValues() []string {
	return []string{
		"ObjectRead",
		"ObjectWrite",
		"ObjectReadWrite",
		"AnyObjectWrite",
		"AnyObjectRead",
		"AnyObjectReadWrite",
	}
}

// GetMappingPreauthenticatedRequestSummaryAccessTypeEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingPreauthenticatedRequestSummaryAccessTypeEnum(val string) (PreauthenticatedRequestSummaryAccessTypeEnum, bool) {
	enum, ok := mappingPreauthenticatedRequestSummaryAccessTypeEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
