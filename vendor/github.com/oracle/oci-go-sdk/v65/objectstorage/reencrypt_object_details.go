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

// ReencryptObjectDetails The details used to re-encrypt the data encryption keys associated with an object.
// You can only specify either a kmsKeyId or an sseCustomerKey in the request payload, not both.
// If the request payload is empty, the object is encrypted using the encryption key assigned to the
// bucket. The bucket encryption mechanism can either be a master encryption key managed by Oracle or the Vault service.
// - The sseCustomerKey field specifies the customer-provided encryption key (SSE-C) that will be used to re-encrypt the data encryption keys of the
//   object and its chunks.
// - The sourceSSECustomerKey field specifies information about the customer-provided encryption key that is currently
//   associated with the object source. Specify a value for the sourceSSECustomerKey only if the object
//   is encrypted with a customer-provided encryption key.
type ReencryptObjectDetails struct {

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the master encryption key used to call the Vault
	// service to re-encrypt the data encryption keys associated with the object and its chunks. If the kmsKeyId value is
	// empty, whether null or an empty string, the API will perform re-encryption by using the kmsKeyId associated with the
	// bucket or the master encryption key managed by Oracle, depending on the bucket encryption mechanism.
	KmsKeyId *string `mandatory:"false" json:"kmsKeyId"`

	SseCustomerKey *SseCustomerKeyDetails `mandatory:"false" json:"sseCustomerKey"`

	SourceSseCustomerKey *SseCustomerKeyDetails `mandatory:"false" json:"sourceSseCustomerKey"`
}

func (m ReencryptObjectDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m ReencryptObjectDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
