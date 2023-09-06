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

// ObjectLifecycleRule To use any of the API operations, you must be authorized in an IAM policy. If you are not authorized,
// talk to an administrator. If you are an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type ObjectLifecycleRule struct {

	// The name of the lifecycle rule to be applied.
	Name *string `mandatory:"true" json:"name"`

	// The action of the object lifecycle policy rule.
	// Rules using the action 'ARCHIVE' move objects from Standard and InfrequentAccess storage tiers
	// into the Archive storage tier (https://docs.cloud.oracle.com/Content/Archive/Concepts/archivestorageoverview.htm).
	// Rules using the action 'INFREQUENT_ACCESS' move objects from Standard storage tier into the
	// Infrequent Access Storage tier. Objects that are already in InfrequentAccess tier or in Archive
	// tier are left untouched.
	// Rules using the action 'DELETE' permanently delete objects from buckets.
	// Rules using 'ABORT' abort the uncommitted multipart-uploads and permanently delete their parts from buckets.
	Action *string `mandatory:"true" json:"action"`

	// Specifies the age of objects to apply the rule to. The timeAmount is interpreted in units defined by the
	// timeUnit parameter, and is calculated in relation to each object's Last-Modified time.
	TimeAmount *int64 `mandatory:"true" json:"timeAmount"`

	// The unit that should be used to interpret timeAmount.  Days are defined as starting and ending at midnight UTC.
	// Years are defined as 365.2425 days long and likewise round up to the next midnight UTC.
	TimeUnit ObjectLifecycleRuleTimeUnitEnum `mandatory:"true" json:"timeUnit"`

	// A Boolean that determines whether this rule is currently enabled.
	IsEnabled *bool `mandatory:"true" json:"isEnabled"`

	// The target of the object lifecycle policy rule. The values of target can be either "objects",
	// "multipart-uploads" or "previous-object-versions".
	// This field when declared as "objects" is used to specify ARCHIVE, INFREQUENT_ACCESS
	// or DELETE rule for objects.
	// This field when declared as "previous-object-versions" is used to specify ARCHIVE,
	// INFREQUENT_ACCESS or DELETE rule for previous versions of existing objects.
	// This field when declared as "multipart-uploads" is used to specify the ABORT (only) rule for
	// uncommitted multipart-uploads.
	Target *string `mandatory:"false" json:"target"`

	ObjectNameFilter *ObjectNameFilter `mandatory:"false" json:"objectNameFilter"`
}

func (m ObjectLifecycleRule) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m ObjectLifecycleRule) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingObjectLifecycleRuleTimeUnitEnum(string(m.TimeUnit)); !ok && m.TimeUnit != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for TimeUnit: %s. Supported values are: %s.", m.TimeUnit, strings.Join(GetObjectLifecycleRuleTimeUnitEnumStringValues(), ",")))
	}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// ObjectLifecycleRuleTimeUnitEnum Enum with underlying type: string
type ObjectLifecycleRuleTimeUnitEnum string

// Set of constants representing the allowable values for ObjectLifecycleRuleTimeUnitEnum
const (
	ObjectLifecycleRuleTimeUnitDays  ObjectLifecycleRuleTimeUnitEnum = "DAYS"
	ObjectLifecycleRuleTimeUnitYears ObjectLifecycleRuleTimeUnitEnum = "YEARS"
)

var mappingObjectLifecycleRuleTimeUnitEnum = map[string]ObjectLifecycleRuleTimeUnitEnum{
	"DAYS":  ObjectLifecycleRuleTimeUnitDays,
	"YEARS": ObjectLifecycleRuleTimeUnitYears,
}

var mappingObjectLifecycleRuleTimeUnitEnumLowerCase = map[string]ObjectLifecycleRuleTimeUnitEnum{
	"days":  ObjectLifecycleRuleTimeUnitDays,
	"years": ObjectLifecycleRuleTimeUnitYears,
}

// GetObjectLifecycleRuleTimeUnitEnumValues Enumerates the set of values for ObjectLifecycleRuleTimeUnitEnum
func GetObjectLifecycleRuleTimeUnitEnumValues() []ObjectLifecycleRuleTimeUnitEnum {
	values := make([]ObjectLifecycleRuleTimeUnitEnum, 0)
	for _, v := range mappingObjectLifecycleRuleTimeUnitEnum {
		values = append(values, v)
	}
	return values
}

// GetObjectLifecycleRuleTimeUnitEnumStringValues Enumerates the set of values in String for ObjectLifecycleRuleTimeUnitEnum
func GetObjectLifecycleRuleTimeUnitEnumStringValues() []string {
	return []string{
		"DAYS",
		"YEARS",
	}
}

// GetMappingObjectLifecycleRuleTimeUnitEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingObjectLifecycleRuleTimeUnitEnum(val string) (ObjectLifecycleRuleTimeUnitEnum, bool) {
	enum, ok := mappingObjectLifecycleRuleTimeUnitEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
