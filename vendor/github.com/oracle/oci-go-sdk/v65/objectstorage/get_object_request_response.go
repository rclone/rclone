// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package objectstorage

import (
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"io"
	"net/http"
	"strings"
)

// GetObjectRequest wrapper for the GetObject operation
//
// # See also
//
// Click https://docs.cloud.oracle.com/en-us/iaas/tools/go-sdk-examples/latest/objectstorage/GetObject.go.html to see an example of how to use GetObjectRequest.
type GetObjectRequest struct {

	// The Object Storage namespace used for the request.
	NamespaceName *string `mandatory:"true" contributesTo:"path" name:"namespaceName"`

	// The name of the bucket. Avoid entering confidential information.
	// Example: `my-new-bucket1`
	BucketName *string `mandatory:"true" contributesTo:"path" name:"bucketName"`

	// The name of the object. Avoid entering confidential information.
	// Example: `test/object1.log`
	ObjectName *string `mandatory:"true" contributesTo:"path" name:"objectName"`

	// VersionId used to identify a particular version of the object
	VersionId *string `mandatory:"false" contributesTo:"query" name:"versionId"`

	// The entity tag (ETag) to match with the ETag of an existing resource. If the specified ETag matches the ETag of
	// the existing resource, GET and HEAD requests will return the resource and PUT and POST requests will upload
	// the resource.
	IfMatch *string `mandatory:"false" contributesTo:"header" name:"if-match"`

	// The entity tag (ETag) to avoid matching. Wildcards ('*') are not allowed. If the specified ETag does not
	// match the ETag of the existing resource, the request returns the expected response. If the ETag matches
	// the ETag of the existing resource, the request returns an HTTP 304 status without a response body.
	IfNoneMatch *string `mandatory:"false" contributesTo:"header" name:"if-none-match"`

	// The client request ID for tracing.
	OpcClientRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-client-request-id"`

	// Optional byte range to fetch, as described in RFC 7233 (https://tools.ietf.org/html/rfc7233#section-2.1).
	// Note that only a single range of bytes is supported.
	Range *string `mandatory:"false" contributesTo:"header" name:"range"`

	// The optional header that specifies "AES256" as the encryption algorithm. For more information, see
	// Using Your Own Keys for Server-Side Encryption (https://docs.cloud.oracle.com/Content/Object/Tasks/usingyourencryptionkeys.htm).
	OpcSseCustomerAlgorithm *string `mandatory:"false" contributesTo:"header" name:"opc-sse-customer-algorithm"`

	// The optional header that specifies the base64-encoded 256-bit encryption key to use to encrypt or
	// decrypt the data. For more information, see
	// Using Your Own Keys for Server-Side Encryption (https://docs.cloud.oracle.com/Content/Object/Tasks/usingyourencryptionkeys.htm).
	OpcSseCustomerKey *string `mandatory:"false" contributesTo:"header" name:"opc-sse-customer-key"`

	// The optional header that specifies the base64-encoded SHA256 hash of the encryption key. This
	// value is used to check the integrity of the encryption key. For more information, see
	// Using Your Own Keys for Server-Side Encryption (https://docs.cloud.oracle.com/Content/Object/Tasks/usingyourencryptionkeys.htm).
	OpcSseCustomerKeySha256 *string `mandatory:"false" contributesTo:"header" name:"opc-sse-customer-key-sha256"`

	// Specify this query parameter to override the value of the Content-Disposition response header in the GetObject response.
	HttpResponseContentDisposition *string `mandatory:"false" contributesTo:"query" name:"httpResponseContentDisposition"`

	// Specify this query parameter to override the Cache-Control response header in the GetObject response.
	HttpResponseCacheControl *string `mandatory:"false" contributesTo:"query" name:"httpResponseCacheControl"`

	// Specify this query parameter to override the Content-Type response header in the GetObject response.
	HttpResponseContentType *string `mandatory:"false" contributesTo:"query" name:"httpResponseContentType"`

	// Specify this query parameter to override the Content-Language response header in the GetObject response.
	HttpResponseContentLanguage *string `mandatory:"false" contributesTo:"query" name:"httpResponseContentLanguage"`

	// Specify this query parameter to override the Content-Encoding response header in the GetObject response.
	HttpResponseContentEncoding *string `mandatory:"false" contributesTo:"query" name:"httpResponseContentEncoding"`

	// Specify this query parameter to override the Expires response header in the GetObject response.
	HttpResponseExpires *string `mandatory:"false" contributesTo:"query" name:"httpResponseExpires"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request GetObjectRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request GetObjectRequest) HTTPRequest(method, path string, binaryRequestBody *common.OCIReadSeekCloser, extraHeaders map[string]string) (http.Request, error) {

	_, err := request.ValidateEnumValue()
	if err != nil {
		return http.Request{}, err
	}
	return common.MakeDefaultHTTPRequestWithTaggedStructAndExtraHeaders(method, path, request, extraHeaders)
}

// BinaryRequestBody implements the OCIRequest interface
func (request GetObjectRequest) BinaryRequestBody() (*common.OCIReadSeekCloser, bool) {

	return nil, false

}

// ReplaceMandatoryParamInPath replaces the mandatory parameter in the path with the value provided.
// Not all services are supporting this feature and this method will be a no-op for those services.
func (request GetObjectRequest) ReplaceMandatoryParamInPath(client *common.BaseClient, mandatoryParamMap map[string][]common.TemplateParamForPerRealmEndpoint) {
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
	if mandatoryParamMap["objectName"] != nil {
		templateParam := mandatoryParamMap["objectName"]
		for _, template := range templateParam {
			replacementParam := *request.ObjectName
			if template.EndsWithDot {
				replacementParam = replacementParam + "."
			}
			client.Host = strings.Replace(client.Host, template.Template, replacementParam, -1)
		}
	}
}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request GetObjectRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (request GetObjectRequest) ValidateEnumValue() (bool, error) {
	errMessage := []string{}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// GetObjectResponse wrapper for the GetObject operation
type GetObjectResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// The io.ReadCloser instance
	Content io.ReadCloser `presentIn:"body" encoding:"binary"`

	// Echoes back the value passed in the opc-client-request-id header, for use by clients when debugging.
	OpcClientRequestId *string `presentIn:"header" name:"opc-client-request-id"`

	// Unique Oracle-assigned identifier for the request. If you need to contact Oracle about a particular
	// request, provide this request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`

	// The entity tag (ETag) for the object.
	ETag *string `presentIn:"header" name:"etag"`

	// The user-defined metadata for the object.
	OpcMeta map[string]string `presentIn:"header-collection" prefix:"opc-meta-"`

	// The object size in bytes.
	ContentLength *int64 `presentIn:"header" name:"content-length"`

	// Content-Range header for range requests, per RFC 7233 (https://tools.ietf.org/html/rfc7233#section-4.2).
	ContentRange *string `presentIn:"header" name:"content-range"`

	// Content-MD5 header, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.15).
	// Unavailable for objects uploaded using multipart upload.
	ContentMd5 *string `presentIn:"header" name:"content-md5"`

	// Only applicable to objects uploaded using multipart upload.
	// Base-64 representation of the multipart object hash.
	// The multipart object hash is calculated by taking the MD5 hashes of the parts,
	// concatenating the binary representation of those hashes in order of their part numbers,
	// and then calculating the MD5 hash of the concatenated values.
	OpcMultipartMd5 *string `presentIn:"header" name:"opc-multipart-md5"`

	// Content-Type header, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.17).
	ContentType *string `presentIn:"header" name:"content-type"`

	// Content-Language header, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.12).
	ContentLanguage *string `presentIn:"header" name:"content-language"`

	// Content-Encoding header, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.11).
	ContentEncoding *string `presentIn:"header" name:"content-encoding"`

	// Cache-Control header, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.9).
	CacheControl *string `presentIn:"header" name:"cache-control"`

	// Content-Disposition header, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-19.5.1).
	ContentDisposition *string `presentIn:"header" name:"content-disposition"`

	// The object modification time, as described in RFC 2616 (https://tools.ietf.org/html/rfc2616#section-14.29).
	LastModified *common.SDKTime `presentIn:"header" name:"last-modified"`

	// The storage tier that the object is stored in.
	StorageTier GetObjectStorageTierEnum `presentIn:"header" name:"storage-tier"`

	// Archival state of an object. This field is set only for objects in Archive tier.
	ArchivalState GetObjectArchivalStateEnum `presentIn:"header" name:"archival-state"`

	// Time that the object is returned to the archived state. This field is only present for restored objects.
	TimeOfArchival *common.SDKTime `presentIn:"header" name:"time-of-archival"`

	// VersionId of the object
	VersionId *string `presentIn:"header" name:"version-id"`

	// The date and time after which the object is no longer cached by a browser, proxy, or other caching entity. See
	// RFC 2616 (https://tools.ietf.org/rfc/rfc2616#section-14.21).
	Expires *common.SDKTime `presentIn:"header" name:"expires"`

	// Flag to indicate whether or not the object was modified.  If this is true,
	// the getter for the object itself will return null.  Callers should check this
	// if they specified one of the request params that might result in a conditional
	// response (like 'if-match'/'if-none-match').
	IsNotModified bool
}

func (response GetObjectResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response GetObjectResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// GetObjectStorageTierEnum Enum with underlying type: string
type GetObjectStorageTierEnum string

// Set of constants representing the allowable values for GetObjectStorageTierEnum
const (
	GetObjectStorageTierStandard         GetObjectStorageTierEnum = "Standard"
	GetObjectStorageTierInfrequentaccess GetObjectStorageTierEnum = "InfrequentAccess"
	GetObjectStorageTierArchive          GetObjectStorageTierEnum = "Archive"
)

var mappingGetObjectStorageTierEnum = map[string]GetObjectStorageTierEnum{
	"Standard":         GetObjectStorageTierStandard,
	"InfrequentAccess": GetObjectStorageTierInfrequentaccess,
	"Archive":          GetObjectStorageTierArchive,
}

var mappingGetObjectStorageTierEnumLowerCase = map[string]GetObjectStorageTierEnum{
	"standard":         GetObjectStorageTierStandard,
	"infrequentaccess": GetObjectStorageTierInfrequentaccess,
	"archive":          GetObjectStorageTierArchive,
}

// GetGetObjectStorageTierEnumValues Enumerates the set of values for GetObjectStorageTierEnum
func GetGetObjectStorageTierEnumValues() []GetObjectStorageTierEnum {
	values := make([]GetObjectStorageTierEnum, 0)
	for _, v := range mappingGetObjectStorageTierEnum {
		values = append(values, v)
	}
	return values
}

// GetGetObjectStorageTierEnumStringValues Enumerates the set of values in String for GetObjectStorageTierEnum
func GetGetObjectStorageTierEnumStringValues() []string {
	return []string{
		"Standard",
		"InfrequentAccess",
		"Archive",
	}
}

// GetMappingGetObjectStorageTierEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingGetObjectStorageTierEnum(val string) (GetObjectStorageTierEnum, bool) {
	enum, ok := mappingGetObjectStorageTierEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}

// GetObjectArchivalStateEnum Enum with underlying type: string
type GetObjectArchivalStateEnum string

// Set of constants representing the allowable values for GetObjectArchivalStateEnum
const (
	GetObjectArchivalStateArchived  GetObjectArchivalStateEnum = "Archived"
	GetObjectArchivalStateRestoring GetObjectArchivalStateEnum = "Restoring"
	GetObjectArchivalStateRestored  GetObjectArchivalStateEnum = "Restored"
)

var mappingGetObjectArchivalStateEnum = map[string]GetObjectArchivalStateEnum{
	"Archived":  GetObjectArchivalStateArchived,
	"Restoring": GetObjectArchivalStateRestoring,
	"Restored":  GetObjectArchivalStateRestored,
}

var mappingGetObjectArchivalStateEnumLowerCase = map[string]GetObjectArchivalStateEnum{
	"archived":  GetObjectArchivalStateArchived,
	"restoring": GetObjectArchivalStateRestoring,
	"restored":  GetObjectArchivalStateRestored,
}

// GetGetObjectArchivalStateEnumValues Enumerates the set of values for GetObjectArchivalStateEnum
func GetGetObjectArchivalStateEnumValues() []GetObjectArchivalStateEnum {
	values := make([]GetObjectArchivalStateEnum, 0)
	for _, v := range mappingGetObjectArchivalStateEnum {
		values = append(values, v)
	}
	return values
}

// GetGetObjectArchivalStateEnumStringValues Enumerates the set of values in String for GetObjectArchivalStateEnum
func GetGetObjectArchivalStateEnumStringValues() []string {
	return []string{
		"Archived",
		"Restoring",
		"Restored",
	}
}

// GetMappingGetObjectArchivalStateEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingGetObjectArchivalStateEnum(val string) (GetObjectArchivalStateEnum, bool) {
	enum, ok := mappingGetObjectArchivalStateEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
