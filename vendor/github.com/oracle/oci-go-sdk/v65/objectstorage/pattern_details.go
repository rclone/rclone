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

// PatternDetails Specifying inclusion and exclusion patterns.
type PatternDetails struct {

	// An array of glob patterns to match the object names to include. An empty array includes all objects in the
	// bucket. Exclusion patterns take precedence over inclusion patterns.
	// A Glob pattern is a sequence of characters to match text. Any character that appears in the pattern, other
	// than the special pattern characters described below, matches itself.
	//     Glob patterns must be between 1 and 1024 characters.
	//     The special pattern characters have the following meanings:
	//     \           Escapes the following character
	//     *           Matches any string of characters.
	//     ?           Matches any single character .
	//     [...]       Matches a group of characters. A group of characters can be:
	//                     A set of characters, for example: [Zafg9@]. This matches any character in the brackets.
	//                     A range of characters, for example: [a-z]. This matches any character in the range.
	//                         [a-f] is equivalent to [abcdef].
	//                         For character ranges only the CHARACTER-CHARACTER pattern is supported.
	//                             [ab-yz] is not valid
	//                             [a-mn-z] is not valid
	//                         Character ranges can not start with ^ or :
	//                         To include a '-' in the range, make it the first or last character.
	InclusionPatterns []string `mandatory:"false" json:"inclusionPatterns"`

	// An array of glob patterns to match the object names to exclude. An empty array is ignored. Exclusion
	// patterns take precedence over inclusion patterns.
	// A Glob pattern is a sequence of characters to match text. Any character that appears in the pattern, other
	// than the special pattern characters described below, matches itself.
	//     Glob patterns must be between 1 and 1024 characters.
	//     The special pattern characters have the following meanings:
	//     \           Escapes the following character
	//     *           Matches any string of characters.
	//     ?           Matches any single character .
	//     [...]       Matches a group of characters. A group of characters can be:
	//                     A set of characters, for example: [Zafg9@]. This matches any character in the brackets.
	//                     A range of characters, for example: [a-z]. This matches any character in the range.
	//                         [a-f] is equivalent to [abcdef].
	//                         For character ranges only the CHARACTER-CHARACTER pattern is supported.
	//                             [ab-yz] is not valid
	//                             [a-mn-z] is not valid
	//                         Character ranges can not start with ^ or :
	//                         To include a '-' in the range, make it the first or last character.
	ExclusionPatterns []string `mandatory:"false" json:"exclusionPatterns"`
}

func (m PatternDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m PatternDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
