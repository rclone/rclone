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

// Duration The amount of time that objects in the bucket should be preserved for and which is calculated in relation to
// each object's Last-Modified timestamp. If duration is not present, then there is no time limit and the objects
// in the bucket will be preserved indefinitely.
type Duration struct {

	// The timeAmount is interpreted in units defined by the timeUnit parameter, and is calculated in relation
	// to each object's Last-Modified timestamp.
	TimeAmount *int64 `mandatory:"true" json:"timeAmount"`

	// The unit that should be used to interpret timeAmount.
	TimeUnit DurationTimeUnitEnum `mandatory:"true" json:"timeUnit"`
}

func (m Duration) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m Duration) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingDurationTimeUnitEnum(string(m.TimeUnit)); !ok && m.TimeUnit != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for TimeUnit: %s. Supported values are: %s.", m.TimeUnit, strings.Join(GetDurationTimeUnitEnumStringValues(), ",")))
	}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// DurationTimeUnitEnum Enum with underlying type: string
type DurationTimeUnitEnum string

// Set of constants representing the allowable values for DurationTimeUnitEnum
const (
	DurationTimeUnitYears DurationTimeUnitEnum = "YEARS"
	DurationTimeUnitDays  DurationTimeUnitEnum = "DAYS"
)

var mappingDurationTimeUnitEnum = map[string]DurationTimeUnitEnum{
	"YEARS": DurationTimeUnitYears,
	"DAYS":  DurationTimeUnitDays,
}

var mappingDurationTimeUnitEnumLowerCase = map[string]DurationTimeUnitEnum{
	"years": DurationTimeUnitYears,
	"days":  DurationTimeUnitDays,
}

// GetDurationTimeUnitEnumValues Enumerates the set of values for DurationTimeUnitEnum
func GetDurationTimeUnitEnumValues() []DurationTimeUnitEnum {
	values := make([]DurationTimeUnitEnum, 0)
	for _, v := range mappingDurationTimeUnitEnum {
		values = append(values, v)
	}
	return values
}

// GetDurationTimeUnitEnumStringValues Enumerates the set of values in String for DurationTimeUnitEnum
func GetDurationTimeUnitEnumStringValues() []string {
	return []string{
		"YEARS",
		"DAYS",
	}
}

// GetMappingDurationTimeUnitEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingDurationTimeUnitEnum(val string) (DurationTimeUnitEnum, bool) {
	enum, ok := mappingDurationTimeUnitEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
