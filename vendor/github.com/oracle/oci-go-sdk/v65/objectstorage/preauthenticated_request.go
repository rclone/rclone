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

// PreauthenticatedRequest Pre-authenticated requests provide a way to let users access a bucket or an object without having their own credentials.
// When you create a pre-authenticated request, a unique URL is generated. Users in your organization, partners, or third
// parties can use this URL to access the targets identified in the pre-authenticated request.
// See Using Pre-Authenticated Requests (https://docs.cloud.oracle.com/Content/Object/Tasks/usingpreauthenticatedrequests.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized, talk to an
// administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type PreauthenticatedRequest struct {

	// The unique identifier to use when directly addressing the pre-authenticated request.
	Id *string `mandatory:"true" json:"id"`

	// The user-provided name of the pre-authenticated request.
	Name *string `mandatory:"true" json:"name"`

	// The URI to embed in the URL when using the pre-authenticated request.
	AccessUri *string `mandatory:"true" json:"accessUri"`

	// The operation that can be performed on this resource.
	AccessType PreauthenticatedRequestAccessTypeEnum `mandatory:"true" json:"accessType"`

	// The expiration date for the pre-authenticated request as per RFC 3339 (https://tools.ietf.org/html/rfc3339). After
	// this date the pre-authenticated request will no longer be valid.
	TimeExpires *common.SDKTime `mandatory:"true" json:"timeExpires"`

	// The date when the pre-authenticated request was created as per specification
	// RFC 3339 (https://tools.ietf.org/html/rfc3339).
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The name of the object that is being granted access to by the pre-authenticated request. Avoid entering confidential
	// information. The object name can be null and if so, the pre-authenticated request grants access to the entire bucket.
	// Example: test/object1.log
	ObjectName *string `mandatory:"false" json:"objectName"`

	// Specifies whether a list operation is allowed on a PAR with accessType "AnyObjectRead" or "AnyObjectReadWrite".
	// Deny: Prevents the user from performing a list operation.
	// ListObjects: Authorizes the user to perform a list operation.
	BucketListingAction PreauthenticatedRequestBucketListingActionEnum `mandatory:"false" json:"bucketListingAction,omitempty"`

	// The full Path for the object.
	FullPath *string `mandatory:"false" json:"fullPath"`
}

func (m PreauthenticatedRequest) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m PreauthenticatedRequest) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingPreauthenticatedRequestAccessTypeEnum(string(m.AccessType)); !ok && m.AccessType != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for AccessType: %s. Supported values are: %s.", m.AccessType, strings.Join(GetPreauthenticatedRequestAccessTypeEnumStringValues(), ",")))
	}

	if _, ok := GetMappingPreauthenticatedRequestBucketListingActionEnum(string(m.BucketListingAction)); !ok && m.BucketListingAction != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for BucketListingAction: %s. Supported values are: %s.", m.BucketListingAction, strings.Join(GetPreauthenticatedRequestBucketListingActionEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// PreauthenticatedRequestBucketListingActionEnum Enum with underlying type: string
type PreauthenticatedRequestBucketListingActionEnum string

// Set of constants representing the allowable values for PreauthenticatedRequestBucketListingActionEnum
const (
	PreauthenticatedRequestBucketListingActionDeny        PreauthenticatedRequestBucketListingActionEnum = "Deny"
	PreauthenticatedRequestBucketListingActionListobjects PreauthenticatedRequestBucketListingActionEnum = "ListObjects"
)

var mappingPreauthenticatedRequestBucketListingActionEnum = map[string]PreauthenticatedRequestBucketListingActionEnum{
	"Deny":        PreauthenticatedRequestBucketListingActionDeny,
	"ListObjects": PreauthenticatedRequestBucketListingActionListobjects,
}

var mappingPreauthenticatedRequestBucketListingActionEnumLowerCase = map[string]PreauthenticatedRequestBucketListingActionEnum{
	"deny":        PreauthenticatedRequestBucketListingActionDeny,
	"listobjects": PreauthenticatedRequestBucketListingActionListobjects,
}

// GetPreauthenticatedRequestBucketListingActionEnumValues Enumerates the set of values for PreauthenticatedRequestBucketListingActionEnum
func GetPreauthenticatedRequestBucketListingActionEnumValues() []PreauthenticatedRequestBucketListingActionEnum {
	values := make([]PreauthenticatedRequestBucketListingActionEnum, 0)
	for _, v := range mappingPreauthenticatedRequestBucketListingActionEnum {
		values = append(values, v)
	}
	return values
}

// GetPreauthenticatedRequestBucketListingActionEnumStringValues Enumerates the set of values in String for PreauthenticatedRequestBucketListingActionEnum
func GetPreauthenticatedRequestBucketListingActionEnumStringValues() []string {
	return []string{
		"Deny",
		"ListObjects",
	}
}

// GetMappingPreauthenticatedRequestBucketListingActionEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingPreauthenticatedRequestBucketListingActionEnum(val string) (PreauthenticatedRequestBucketListingActionEnum, bool) {
	enum, ok := mappingPreauthenticatedRequestBucketListingActionEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// PreauthenticatedRequestAccessTypeEnum Enum with underlying type: string
type PreauthenticatedRequestAccessTypeEnum string

// Set of constants representing the allowable values for PreauthenticatedRequestAccessTypeEnum
const (
	PreauthenticatedRequestAccessTypeObjectread         PreauthenticatedRequestAccessTypeEnum = "ObjectRead"
	PreauthenticatedRequestAccessTypeObjectwrite        PreauthenticatedRequestAccessTypeEnum = "ObjectWrite"
	PreauthenticatedRequestAccessTypeObjectreadwrite    PreauthenticatedRequestAccessTypeEnum = "ObjectReadWrite"
	PreauthenticatedRequestAccessTypeAnyobjectwrite     PreauthenticatedRequestAccessTypeEnum = "AnyObjectWrite"
	PreauthenticatedRequestAccessTypeAnyobjectread      PreauthenticatedRequestAccessTypeEnum = "AnyObjectRead"
	PreauthenticatedRequestAccessTypeAnyobjectreadwrite PreauthenticatedRequestAccessTypeEnum = "AnyObjectReadWrite"
)

var mappingPreauthenticatedRequestAccessTypeEnum = map[string]PreauthenticatedRequestAccessTypeEnum{
	"ObjectRead":         PreauthenticatedRequestAccessTypeObjectread,
	"ObjectWrite":        PreauthenticatedRequestAccessTypeObjectwrite,
	"ObjectReadWrite":    PreauthenticatedRequestAccessTypeObjectreadwrite,
	"AnyObjectWrite":     PreauthenticatedRequestAccessTypeAnyobjectwrite,
	"AnyObjectRead":      PreauthenticatedRequestAccessTypeAnyobjectread,
	"AnyObjectReadWrite": PreauthenticatedRequestAccessTypeAnyobjectreadwrite,
}

var mappingPreauthenticatedRequestAccessTypeEnumLowerCase = map[string]PreauthenticatedRequestAccessTypeEnum{
	"objectread":         PreauthenticatedRequestAccessTypeObjectread,
	"objectwrite":        PreauthenticatedRequestAccessTypeObjectwrite,
	"objectreadwrite":    PreauthenticatedRequestAccessTypeObjectreadwrite,
	"anyobjectwrite":     PreauthenticatedRequestAccessTypeAnyobjectwrite,
	"anyobjectread":      PreauthenticatedRequestAccessTypeAnyobjectread,
	"anyobjectreadwrite": PreauthenticatedRequestAccessTypeAnyobjectreadwrite,
}

// GetPreauthenticatedRequestAccessTypeEnumValues Enumerates the set of values for PreauthenticatedRequestAccessTypeEnum
func GetPreauthenticatedRequestAccessTypeEnumValues() []PreauthenticatedRequestAccessTypeEnum {
	values := make([]PreauthenticatedRequestAccessTypeEnum, 0)
	for _, v := range mappingPreauthenticatedRequestAccessTypeEnum {
		values = append(values, v)
	}
	return values
}

// GetPreauthenticatedRequestAccessTypeEnumStringValues Enumerates the set of values in String for PreauthenticatedRequestAccessTypeEnum
func GetPreauthenticatedRequestAccessTypeEnumStringValues() []string {
	return []string{
		"ObjectRead",
		"ObjectWrite",
		"ObjectReadWrite",
		"AnyObjectWrite",
		"AnyObjectRead",
		"AnyObjectReadWrite",
	}
}

// GetMappingPreauthenticatedRequestAccessTypeEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingPreauthenticatedRequestAccessTypeEnum(val string) (PreauthenticatedRequestAccessTypeEnum, bool) {
	enum, ok := mappingPreauthenticatedRequestAccessTypeEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
