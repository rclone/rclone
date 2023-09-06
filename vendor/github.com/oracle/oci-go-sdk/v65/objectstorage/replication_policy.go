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

// ReplicationPolicy The details of a replication policy.
type ReplicationPolicy struct {

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
	Status ReplicationPolicyStatusEnum `mandatory:"true" json:"status"`

	// A human-readable description of the status.
	StatusMessage *string `mandatory:"true" json:"statusMessage"`
}

func (m ReplicationPolicy) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m ReplicationPolicy) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingReplicationPolicyStatusEnum(string(m.Status)); !ok && m.Status != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for Status: %s. Supported values are: %s.", m.Status, strings.Join(GetReplicationPolicyStatusEnumStringValues(), ",")))
	}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// ReplicationPolicyStatusEnum Enum with underlying type: string
type ReplicationPolicyStatusEnum string

// Set of constants representing the allowable values for ReplicationPolicyStatusEnum
const (
	ReplicationPolicyStatusActive      ReplicationPolicyStatusEnum = "ACTIVE"
	ReplicationPolicyStatusClientError ReplicationPolicyStatusEnum = "CLIENT_ERROR"
)

var mappingReplicationPolicyStatusEnum = map[string]ReplicationPolicyStatusEnum{
	"ACTIVE":       ReplicationPolicyStatusActive,
	"CLIENT_ERROR": ReplicationPolicyStatusClientError,
}

var mappingReplicationPolicyStatusEnumLowerCase = map[string]ReplicationPolicyStatusEnum{
	"active":       ReplicationPolicyStatusActive,
	"client_error": ReplicationPolicyStatusClientError,
}

// GetReplicationPolicyStatusEnumValues Enumerates the set of values for ReplicationPolicyStatusEnum
func GetReplicationPolicyStatusEnumValues() []ReplicationPolicyStatusEnum {
	values := make([]ReplicationPolicyStatusEnum, 0)
	for _, v := range mappingReplicationPolicyStatusEnum {
		values = append(values, v)
	}
	return values
}

// GetReplicationPolicyStatusEnumStringValues Enumerates the set of values in String for ReplicationPolicyStatusEnum
func GetReplicationPolicyStatusEnumStringValues() []string {
	return []string{
		"ACTIVE",
		"CLIENT_ERROR",
	}
}

// GetMappingReplicationPolicyStatusEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingReplicationPolicyStatusEnum(val string) (ReplicationPolicyStatusEnum, bool) {
	enum, ok := mappingReplicationPolicyStatusEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
