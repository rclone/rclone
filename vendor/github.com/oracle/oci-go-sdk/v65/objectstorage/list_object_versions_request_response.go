// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package objectstorage

import (
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"net/http"
	"strings"
)

// ListObjectVersionsRequest wrapper for the ListObjectVersions operation
//
// # See also
//
// Click https://docs.cloud.oracle.com/en-us/iaas/tools/go-sdk-examples/latest/objectstorage/ListObjectVersions.go.html to see an example of how to use ListObjectVersionsRequest.
type ListObjectVersionsRequest struct {

	// The Object Storage namespace used for the request.
	NamespaceName *string `mandatory:"true" contributesTo:"path" name:"namespaceName"`

	// The name of the bucket. Avoid entering confidential information.
	// Example: `my-new-bucket1`
	BucketName *string `mandatory:"true" contributesTo:"path" name:"bucketName"`

	// The string to use for matching against the start of object names in a list query.
	Prefix *string `mandatory:"false" contributesTo:"query" name:"prefix"`

	// Object names returned by a list query must be greater or equal to this parameter.
	Start *string `mandatory:"false" contributesTo:"query" name:"start"`

	// Object names returned by a list query must be strictly less than this parameter.
	End *string `mandatory:"false" contributesTo:"query" name:"end"`

	// For list pagination. The maximum number of results per page, or items to return in a paginated
	// "List" call. For important details about how pagination works, see
	// List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	Limit *int `mandatory:"false" contributesTo:"query" name:"limit"`

	// When this parameter is set, only objects whose names do not contain the delimiter character
	// (after an optionally specified prefix) are returned in the objects key of the response body.
	// Scanned objects whose names contain the delimiter have the part of their name up to the first
	// occurrence of the delimiter (including the optional prefix) returned as a set of prefixes.
	// Note that only '/' is a supported delimiter character at this time.
	Delimiter *string `mandatory:"false" contributesTo:"query" name:"delimiter"`

	// Object summary by default includes only the 'name' field. Use this parameter to also
	// include 'size' (object size in bytes), 'etag', 'md5', 'timeCreated' (object creation date and time),
	// 'timeModified' (object modification date and time), 'storageTier' and 'archivalState' fields.
	// Specify the value of this parameter as a comma-separated, case-insensitive list of those field names.
	// For example 'name,etag,timeCreated,md5,timeModified,storageTier,archivalState'.
	Fields ListObjectVersionsFieldsEnum `mandatory:"false" contributesTo:"query" name:"fields" omitEmpty:"true"`

	// The client request ID for tracing.
	OpcClientRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-client-request-id"`

	// Object names returned by a list query must be greater than this parameter.
	StartAfter *string `mandatory:"false" contributesTo:"query" name:"startAfter"`

	// For list pagination. The value of the `opc-next-page` response header from the previous "List" call. For important
	// details about how pagination works, see List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	Page *string `mandatory:"false" contributesTo:"query" name:"page"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request ListObjectVersionsRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request ListObjectVersionsRequest) HTTPRequest(method, path string, binaryRequestBody *common.OCIReadSeekCloser, extraHeaders map[string]string) (http.Request, error) {

	_, err := request.ValidateEnumValue()
	if err != nil {
		return http.Request{}, err
	}
	return common.MakeDefaultHTTPRequestWithTaggedStructAndExtraHeaders(method, path, request, extraHeaders)
}

// BinaryRequestBody implements the OCIRequest interface
func (request ListObjectVersionsRequest) BinaryRequestBody() (*common.OCIReadSeekCloser, bool) {

	return nil, false

}

// ReplaceMandatoryParamInPath replaces the mandatory parameter in the path with the value provided.
// Not all services are supporting this feature and this method will be a no-op for those services.
func (request ListObjectVersionsRequest) ReplaceMandatoryParamInPath(client *common.BaseClient, mandatoryParamMap map[string][]common.TemplateParamForPerRealmEndpoint) {
	if mandatoryParamMap["namespaceName"] != nil {
		templateParam := mandatoryParamMap["namespaceName"]
		for _, template := range templateParam {
			replacementParam := *request.NamespaceName
			if template.EndsWithDot {
				replacementParam = replacementParam + "."
			}
			client.Host = strings.Replace(client.Host, template.Template, replacementParam, -1)
		}
	}
	if mandatoryParamMap["bucketName"] != nil {
		templateParam := mandatoryParamMap["bucketName"]
		for _, template := range templateParam {
			replacementParam := *request.BucketName
			if template.EndsWithDot {
				replacementParam = replacementParam + "."
			}
			client.Host = strings.Replace(client.Host, template.Template, replacementParam, -1)
		}
	}
}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request ListObjectVersionsRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (request ListObjectVersionsRequest) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if _, ok := GetMappingListObjectVersionsFieldsEnum(string(request.Fields)); !ok && request.Fields != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for Fields: %s. Supported values are: %s.", request.Fields, strings.Join(GetListObjectVersionsFieldsEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// ListObjectVersionsResponse wrapper for the ListObjectVersions operation
type ListObjectVersionsResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// A list of ObjectVersionCollection instances
	ObjectVersionCollection `presentIn:"body"`

	// Echoes back the value passed in the opc-client-request-id header, for use by clients when debugging.
	OpcClientRequestId *string `presentIn:"header" name:"opc-client-request-id"`

	// Unique Oracle-assigned identifier for the request. If you need to contact Oracle about a particular
	// request, provide this request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`

	// For paginating a list of object versions.
	// In the GET request, set the limit to the number of object versions that you want returned in the response.
	// If the `opc-next-page` header appears in the response, then this is a partial list and there are
	// additional object versions to get. Include the header's value as the `page` parameter in the subsequent
	// GET request to get the next batch of object versions and prefixes. Repeat this process to retrieve the entire list of
	// object versions and prefixes.
	// For more details about how pagination works, see List Pagination (https://docs.cloud.oracle.com/iaas/Content/API/Concepts/usingapi.htm#nine).
	OpcNextPage *string `presentIn:"header" name:"opc-next-page"`
}

func (response ListObjectVersionsResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response ListObjectVersionsResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// ListObjectVersionsFieldsEnum Enum with underlying type: string
type ListObjectVersionsFieldsEnum string

// Set of constants representing the allowable values for ListObjectVersionsFieldsEnum
const (
	ListObjectVersionsFieldsName          ListObjectVersionsFieldsEnum = "name"
	ListObjectVersionsFieldsSize          ListObjectVersionsFieldsEnum = "size"
	ListObjectVersionsFieldsEtag          ListObjectVersionsFieldsEnum = "etag"
	ListObjectVersionsFieldsTimecreated   ListObjectVersionsFieldsEnum = "timeCreated"
	ListObjectVersionsFieldsMd5           ListObjectVersionsFieldsEnum = "md5"
	ListObjectVersionsFieldsTimemodified  ListObjectVersionsFieldsEnum = "timeModified"
	ListObjectVersionsFieldsStoragetier   ListObjectVersionsFieldsEnum = "storageTier"
	ListObjectVersionsFieldsArchivalstate ListObjectVersionsFieldsEnum = "archivalState"
)

var mappingListObjectVersionsFieldsEnum = map[string]ListObjectVersionsFieldsEnum{
	"name":          ListObjectVersionsFieldsName,
	"size":          ListObjectVersionsFieldsSize,
	"etag":          ListObjectVersionsFieldsEtag,
	"timeCreated":   ListObjectVersionsFieldsTimecreated,
	"md5":           ListObjectVersionsFieldsMd5,
	"timeModified":  ListObjectVersionsFieldsTimemodified,
	"storageTier":   ListObjectVersionsFieldsStoragetier,
	"archivalState": ListObjectVersionsFieldsArchivalstate,
}

var mappingListObjectVersionsFieldsEnumLowerCase = map[string]ListObjectVersionsFieldsEnum{
	"name":          ListObjectVersionsFieldsName,
	"size":          ListObjectVersionsFieldsSize,
	"etag":          ListObjectVersionsFieldsEtag,
	"timecreated":   ListObjectVersionsFieldsTimecreated,
	"md5":           ListObjectVersionsFieldsMd5,
	"timemodified":  ListObjectVersionsFieldsTimemodified,
	"storagetier":   ListObjectVersionsFieldsStoragetier,
	"archivalstate": ListObjectVersionsFieldsArchivalstate,
}

// GetListObjectVersionsFieldsEnumValues Enumerates the set of values for ListObjectVersionsFieldsEnum
func GetListObjectVersionsFieldsEnumValues() []ListObjectVersionsFieldsEnum {
	values := make([]ListObjectVersionsFieldsEnum, 0)
	for _, v := range mappingListObjectVersionsFieldsEnum {
		values = append(values, v)
	}
	return values
}

// GetListObjectVersionsFieldsEnumStringValues Enumerates the set of values in String for ListObjectVersionsFieldsEnum
func GetListObjectVersionsFieldsEnumStringValues() []string {
	return []string{
		"name",
		"size",
		"etag",
		"timeCreated",
		"md5",
		"timeModified",
		"storageTier",
		"archivalState",
	}
}

// GetMappingListObjectVersionsFieldsEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingListObjectVersionsFieldsEnum(val string) (ListObjectVersionsFieldsEnum, bool) {
	enum, ok := mappingListObjectVersionsFieldsEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
