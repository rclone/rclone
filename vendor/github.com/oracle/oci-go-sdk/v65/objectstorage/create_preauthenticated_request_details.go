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

// CreatePreauthenticatedRequestDetails The representation of CreatePreauthenticatedRequestDetails
type CreatePreauthenticatedRequestDetails struct {

	// A user-specified name for the pre-authenticated request. Names can be helpful in managing pre-authenticated requests.
	// Avoid entering confidential information.
	Name *string `mandatory:"true" json:"name"`

	// The operation that can be performed on this resource.
	AccessType CreatePreauthenticatedRequestDetailsAccessTypeEnum `mandatory:"true" json:"accessType"`

	// The expiration date for the pre-authenticated request as per RFC 3339 (https://tools.ietf.org/html/rfc3339).
	// After this date the pre-authenticated request will no longer be valid.
	TimeExpires *common.SDKTime `mandatory:"true" json:"timeExpires"`

	// Specifies whether a list operation is allowed on a PAR with accessType "AnyObjectRead" or "AnyObjectReadWrite".
	// Deny: Prevents the user from performing a list operation.
	// ListObjects: Authorizes the user to perform a list operation.
	BucketListingAction PreauthenticatedRequestBucketListingActionEnum `mandatory:"false" json:"bucketListingAction,omitempty"`

	// The name of the object that is being granted access to by the pre-authenticated request. Avoid entering confidential
	// information. The object name can be null and if so, the pre-authenticated request grants access to the entire bucket
	// if the access type allows that. The object name can be a prefix as well, in that case pre-authenticated request
	// grants access to all the objects within the bucket starting with that prefix provided that we have the correct access type.
	ObjectName *string `mandatory:"false" json:"objectName"`
}

func (m CreatePreauthenticatedRequestDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m CreatePreauthenticatedRequestDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingCreatePreauthenticatedRequestDetailsAccessTypeEnum(string(m.AccessType)); !ok && m.AccessType != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for AccessType: %s. Supported values are: %s.", m.AccessType, strings.Join(GetCreatePreauthenticatedRequestDetailsAccessTypeEnumStringValues(), ",")))
	}

	if _, ok := GetMappingPreauthenticatedRequestBucketListingActionEnum(string(m.BucketListingAction)); !ok && m.BucketListingAction != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for BucketListingAction: %s. Supported values are: %s.", m.BucketListingAction, strings.Join(GetPreauthenticatedRequestBucketListingActionEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// CreatePreauthenticatedRequestDetailsAccessTypeEnum Enum with underlying type: string
type CreatePreauthenticatedRequestDetailsAccessTypeEnum string

// Set of constants representing the allowable values for CreatePreauthenticatedRequestDetailsAccessTypeEnum
const (
	CreatePreauthenticatedRequestDetailsAccessTypeObjectread         CreatePreauthenticatedRequestDetailsAccessTypeEnum = "ObjectRead"
	CreatePreauthenticatedRequestDetailsAccessTypeObjectwrite        CreatePreauthenticatedRequestDetailsAccessTypeEnum = "ObjectWrite"
	CreatePreauthenticatedRequestDetailsAccessTypeObjectreadwrite    CreatePreauthenticatedRequestDetailsAccessTypeEnum = "ObjectReadWrite"
	CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectwrite     CreatePreauthenticatedRequestDetailsAccessTypeEnum = "AnyObjectWrite"
	CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectread      CreatePreauthenticatedRequestDetailsAccessTypeEnum = "AnyObjectRead"
	CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectreadwrite CreatePreauthenticatedRequestDetailsAccessTypeEnum = "AnyObjectReadWrite"
)

var mappingCreatePreauthenticatedRequestDetailsAccessTypeEnum = map[string]CreatePreauthenticatedRequestDetailsAccessTypeEnum{
	"ObjectRead":         CreatePreauthenticatedRequestDetailsAccessTypeObjectread,
	"ObjectWrite":        CreatePreauthenticatedRequestDetailsAccessTypeObjectwrite,
	"ObjectReadWrite":    CreatePreauthenticatedRequestDetailsAccessTypeObjectreadwrite,
	"AnyObjectWrite":     CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectwrite,
	"AnyObjectRead":      CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectread,
	"AnyObjectReadWrite": CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectreadwrite,
}

var mappingCreatePreauthenticatedRequestDetailsAccessTypeEnumLowerCase = map[string]CreatePreauthenticatedRequestDetailsAccessTypeEnum{
	"objectread":         CreatePreauthenticatedRequestDetailsAccessTypeObjectread,
	"objectwrite":        CreatePreauthenticatedRequestDetailsAccessTypeObjectwrite,
	"objectreadwrite":    CreatePreauthenticatedRequestDetailsAccessTypeObjectreadwrite,
	"anyobjectwrite":     CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectwrite,
	"anyobjectread":      CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectread,
	"anyobjectreadwrite": CreatePreauthenticatedRequestDetailsAccessTypeAnyobjectreadwrite,
}

// GetCreatePreauthenticatedRequestDetailsAccessTypeEnumValues Enumerates the set of values for CreatePreauthenticatedRequestDetailsAccessTypeEnum
func GetCreatePreauthenticatedRequestDetailsAccessTypeEnumValues() []CreatePreauthenticatedRequestDetailsAccessTypeEnum {
	values := make([]CreatePreauthenticatedRequestDetailsAccessTypeEnum, 0)
	for _, v := range mappingCreatePreauthenticatedRequestDetailsAccessTypeEnum {
		values = append(values, v)
	}
	return values
}

// GetCreatePreauthenticatedRequestDetailsAccessTypeEnumStringValues Enumerates the set of values in String for CreatePreauthenticatedRequestDetailsAccessTypeEnum
func GetCreatePreauthenticatedRequestDetailsAccessTypeEnumStringValues() []string {
	return []string{
		"ObjectRead",
		"ObjectWrite",
		"ObjectReadWrite",
		"AnyObjectWrite",
		"AnyObjectRead",
		"AnyObjectReadWrite",
	}
}

// GetMappingCreatePreauthenticatedRequestDetailsAccessTypeEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingCreatePreauthenticatedRequestDetailsAccessTypeEnum(val string) (CreatePreauthenticatedRequestDetailsAccessTypeEnum, bool) {
	enum, ok := mappingCreatePreauthenticatedRequestDetailsAccessTypeEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
