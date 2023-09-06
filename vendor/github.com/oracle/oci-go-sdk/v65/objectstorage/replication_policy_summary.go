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

// ReplicationPolicySummary The summary of a replication policy.
type ReplicationPolicySummary struct {

	// The id of the replication policy.
	Id *string `mandatory:"true" json:"id"`

	// The name of the policy.
	Name *string `mandatory:"true" json:"name"`

	// The destination region to replicate to, for example "us-ashburn-1".
	DestinationRegionName *string `mandatory:"true" json:"destinationRegionName"`

	// The bucket to replicate to in the destination region. Replication policy creation does not automatically
	// create a destination bucket. Create the destination bucket before creating the policy.
	DestinationBucketName *string `mandatory:"true" json:"destinationBucketName"`

	// The date when the replication policy was created as per RFC 3339 (https://tools.ietf.org/html/rfc3339).
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// Changes made to the source bucket before this time has been replicated.
	TimeLastSync *common.SDKTime `mandatory:"true" json:"timeLastSync"`

	// The replication status of the policy. If the status is CLIENT_ERROR, once the user fixes the issue
	// described in the status message, the status will become ACTIVE.
	Status ReplicationPolicySummaryStatusEnum `mandatory:"true" json:"status"`

	// A human-readable description of the status.
	StatusMessage *string `mandatory:"true" json:"statusMessage"`
}

func (m ReplicationPolicySummary) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m ReplicationPolicySummary) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingReplicationPolicySummaryStatusEnum(string(m.Status)); !ok && m.Status != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for Status: %s. Supported values are: %s.", m.Status, strings.Join(GetReplicationPolicySummaryStatusEnumStringValues(), ",")))
	}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// ReplicationPolicySummaryStatusEnum Enum with underlying type: string
type ReplicationPolicySummaryStatusEnum string

// Set of constants representing the allowable values for ReplicationPolicySummaryStatusEnum
const (
	ReplicationPolicySummaryStatusActive      ReplicationPolicySummaryStatusEnum = "ACTIVE"
	ReplicationPolicySummaryStatusClientError ReplicationPolicySummaryStatusEnum = "CLIENT_ERROR"
)

var mappingReplicationPolicySummaryStatusEnum = map[string]ReplicationPolicySummaryStatusEnum{
	"ACTIVE":       ReplicationPolicySummaryStatusActive,
	"CLIENT_ERROR": ReplicationPolicySummaryStatusClientError,
}

var mappingReplicationPolicySummaryStatusEnumLowerCase = map[string]ReplicationPolicySummaryStatusEnum{
	"active":       ReplicationPolicySummaryStatusActive,
	"client_error": ReplicationPolicySummaryStatusClientError,
}

// GetReplicationPolicySummaryStatusEnumValues Enumerates the set of values for ReplicationPolicySummaryStatusEnum
func GetReplicationPolicySummaryStatusEnumValues() []ReplicationPolicySummaryStatusEnum {
	values := make([]ReplicationPolicySummaryStatusEnum, 0)
	for _, v := range mappingReplicationPolicySummaryStatusEnum {
		values = append(values, v)
	}
	return values
}

// GetReplicationPolicySummaryStatusEnumStringValues Enumerates the set of values in String for ReplicationPolicySummaryStatusEnum
func GetReplicationPolicySummaryStatusEnumStringValues() []string {
	return []string{
		"ACTIVE",
		"CLIENT_ERROR",
	}
}

// GetMappingReplicationPolicySummaryStatusEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingReplicationPolicySummaryStatusEnum(val string) (ReplicationPolicySummaryStatusEnum, bool) {
	enum, ok := mappingReplicationPolicySummaryStatusEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
