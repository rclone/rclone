// +-------------------------------------------------------------------------
// | Copyright (C) 2016 Yunify, Inc.
// +-------------------------------------------------------------------------
// | Licensed under the Apache License, Version 2.0 (the "License");
// | you may not use this work except in compliance with the License.
// | You may obtain a copy of the License in the LICENSE file, or at:
// |
// | http://www.apache.org/licenses/LICENSE-2.0
// |
// | Unless required by applicable law or agreed to in writing, software
// | distributed under the License is distributed on an "AS IS" BASIS,
// | WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// | See the License for the specific language governing permissions and
// | limitations under the License.
// +-------------------------------------------------------------------------

package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yunify/qingstor-sdk-go/v3/config"
	"github.com/yunify/qingstor-sdk-go/v3/request"
	"github.com/yunify/qingstor-sdk-go/v3/request/data"
	"github.com/yunify/qingstor-sdk-go/v3/request/errors"
	"github.com/yunify/qingstor-sdk-go/v3/utils"
)

var _ fmt.State
var _ io.Reader
var _ http.Header
var _ strings.Reader
var _ time.Time
var _ config.Config
var _ utils.Conn

// AbortMultipartUpload does Abort multipart upload.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/abort_multipart_upload.html
func (s *Bucket) AbortMultipartUpload(objectKey string, input *AbortMultipartUploadInput) (*AbortMultipartUploadOutput, error) {
	r, x, err := s.AbortMultipartUploadRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// AbortMultipartUploadRequest creates request and output object of AbortMultipartUpload.
func (s *Bucket) AbortMultipartUploadRequest(objectKey string, input *AbortMultipartUploadInput) (*request.Request, *AbortMultipartUploadOutput, error) {

	if input == nil {
		input = &AbortMultipartUploadInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "Abort Multipart Upload",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			204, // Object multipart deleted
		},
	}

	x := &AbortMultipartUploadOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// AbortMultipartUploadInput presents input for AbortMultipartUpload.
type AbortMultipartUploadInput struct {
	// Object multipart upload ID
	UploadID *string `json:"upload_id" name:"upload_id" location:"query"` // Required

}

// Validate validates the input for AbortMultipartUpload.
func (v *AbortMultipartUploadInput) Validate() error {

	if v.UploadID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "UploadID",
			ParentName:    "AbortMultipartUploadInput",
		}
	}

	return nil
}

// AbortMultipartUploadOutput presents output for AbortMultipartUpload.
type AbortMultipartUploadOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// CompleteMultipartUpload does Complete multipart upload.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/complete_multipart_upload.html
func (s *Bucket) CompleteMultipartUpload(objectKey string, input *CompleteMultipartUploadInput) (*CompleteMultipartUploadOutput, error) {
	r, x, err := s.CompleteMultipartUploadRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// CompleteMultipartUploadRequest creates request and output object of CompleteMultipartUpload.
func (s *Bucket) CompleteMultipartUploadRequest(objectKey string, input *CompleteMultipartUploadInput) (*request.Request, *CompleteMultipartUploadOutput, error) {

	if input == nil {
		input = &CompleteMultipartUploadInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "Complete multipart upload",
		RequestMethod: "POST",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			201, // Object created
		},
	}

	x := &CompleteMultipartUploadOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// CompleteMultipartUploadInput presents input for CompleteMultipartUpload.
type CompleteMultipartUploadInput struct {
	// Object multipart upload ID
	UploadID *string `json:"upload_id" name:"upload_id" location:"query"` // Required

	// MD5sum of the object part
	ETag *string `json:"ETag,omitempty" name:"ETag" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSEncryptionCustomerKey *string `json:"X-QS-Encryption-Customer-Key,omitempty" name:"X-QS-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSEncryptionCustomerKeyMD5 *string `json:"X-QS-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Encryption-Customer-Key-MD5" location:"headers"`

	// Object parts
	ObjectParts []*ObjectPartType `json:"object_parts" name:"object_parts" location:"elements"` // Required

}

// Validate validates the input for CompleteMultipartUpload.
func (v *CompleteMultipartUploadInput) Validate() error {

	if v.UploadID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "UploadID",
			ParentName:    "CompleteMultipartUploadInput",
		}
	}

	if len(v.ObjectParts) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "ObjectParts",
			ParentName:    "CompleteMultipartUploadInput",
		}
	}

	if len(v.ObjectParts) > 0 {
		for _, property := range v.ObjectParts {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// CompleteMultipartUploadOutput presents output for CompleteMultipartUpload.
type CompleteMultipartUploadOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
}

// DeleteObject does Delete the object.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/delete.html
func (s *Bucket) DeleteObject(objectKey string) (*DeleteObjectOutput, error) {
	r, x, err := s.DeleteObjectRequest(objectKey)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// DeleteObjectRequest creates request and output object of DeleteObject.
func (s *Bucket) DeleteObjectRequest(objectKey string) (*request.Request, *DeleteObjectOutput, error) {

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Object",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			204, // Object deleted
		},
	}

	x := &DeleteObjectOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteObjectOutput presents output for DeleteObject.
type DeleteObjectOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// GetObject does Retrieve the object.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/get.html
func (s *Bucket) GetObject(objectKey string, input *GetObjectInput) (*GetObjectOutput, error) {
	r, x, err := s.GetObjectRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// GetObjectRequest creates request and output object of GetObject.
func (s *Bucket) GetObjectRequest(objectKey string, input *GetObjectInput) (*request.Request, *GetObjectOutput, error) {

	if input == nil {
		input = &GetObjectInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Object",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			200, // OK
			206, // Partial content
			304, // Not modified
			412, // Precondition failed
		},
	}

	x := &GetObjectOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetObjectInput presents input for GetObject.
type GetObjectInput struct {
	// Specified the Cache-Control response header
	ResponseCacheControl *string `json:"response-cache-control,omitempty" name:"response-cache-control" location:"query"`
	// Specified the Content-Disposition response header
	ResponseContentDisposition *string `json:"response-content-disposition,omitempty" name:"response-content-disposition" location:"query"`
	// Specified the Content-Encoding response header
	ResponseContentEncoding *string `json:"response-content-encoding,omitempty" name:"response-content-encoding" location:"query"`
	// Specified the Content-Language response header
	ResponseContentLanguage *string `json:"response-content-language,omitempty" name:"response-content-language" location:"query"`
	// Specified the Content-Type response header
	ResponseContentType *string `json:"response-content-type,omitempty" name:"response-content-type" location:"query"`
	// Specified the Expires response header
	ResponseExpires *string `json:"response-expires,omitempty" name:"response-expires" location:"query"`

	// Check whether the ETag matches
	IfMatch *string `json:"If-Match,omitempty" name:"If-Match" location:"headers"`
	// Check whether the object has been modified
	IfModifiedSince *time.Time `json:"If-Modified-Since,omitempty" name:"If-Modified-Since" format:"RFC 822" location:"headers"`
	// Check whether the ETag does not match
	IfNoneMatch *string `json:"If-None-Match,omitempty" name:"If-None-Match" location:"headers"`
	// Check whether the object has not been modified
	IfUnmodifiedSince *time.Time `json:"If-Unmodified-Since,omitempty" name:"If-Unmodified-Since" format:"RFC 822" location:"headers"`
	// Specified range of the object
	Range *string `json:"Range,omitempty" name:"Range" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSEncryptionCustomerKey *string `json:"X-QS-Encryption-Customer-Key,omitempty" name:"X-QS-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSEncryptionCustomerKeyMD5 *string `json:"X-QS-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Encryption-Customer-Key-MD5" location:"headers"`
}

// Validate validates the input for GetObject.
func (v *GetObjectInput) Validate() error {

	return nil
}

// GetObjectOutput presents output for GetObject.
type GetObjectOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// The response body
	Body io.ReadCloser `location:"body"`

	// The Cache-Control general-header field is used to specify directives for caching mechanisms in both requests and responses.
	CacheControl *string `json:"Cache-Control,omitempty" name:"Cache-Control" location:"headers"`
	// In a multipart/form-data body, the HTTP Content-Disposition general header is a header that can be used on the subpart of a multipart body to give information about the field it applies to.
	ContentDisposition *string `json:"Content-Disposition,omitempty" name:"Content-Disposition" location:"headers"`
	// The Content-Encoding entity header is used to compress the media-type.
	ContentEncoding *string `json:"Content-Encoding,omitempty" name:"Content-Encoding" location:"headers"`
	// The Content-Language entity header is used to describe the language(s) intended for the audience.
	ContentLanguage *string `json:"Content-Language,omitempty" name:"Content-Language" location:"headers"`
	// Object content length
	ContentLength *int64 `json:"Content-Length,omitempty" name:"Content-Length" location:"headers"`
	// Range of response data content
	ContentRange *string `json:"Content-Range,omitempty" name:"Content-Range" location:"headers"`
	// The Content-Type entity header is used to indicate the media type of the resource.
	ContentType *string `json:"Content-Type,omitempty" name:"Content-Type" location:"headers"`
	// MD5sum of the object
	ETag *string `json:"ETag,omitempty" name:"ETag" location:"headers"`
	// The Expires header contains the date/time after which the response is considered stale.
	Expires      *string    `json:"Expires,omitempty" name:"Expires" location:"headers"`
	LastModified *time.Time `json:"Last-Modified,omitempty" name:"Last-Modified" format:"RFC 822" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// User-defined metadata
	XQSMetaData *map[string]string `json:"X-QS-MetaData,omitempty" name:"X-QS-MetaData" location:"headers"`
	// Storage class of the object
	XQSStorageClass *string `json:"X-QS-Storage-Class,omitempty" name:"X-QS-Storage-Class" location:"headers"`
}

// Close will close the underlay body.
func (o *GetObjectOutput) Close() (err error) {
	if o.Body != nil {
		return o.Body.Close()
	}
	return
}

// HeadObject does Check whether the object exists and available.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/head.html
func (s *Bucket) HeadObject(objectKey string, input *HeadObjectInput) (*HeadObjectOutput, error) {
	r, x, err := s.HeadObjectRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// HeadObjectRequest creates request and output object of HeadObject.
func (s *Bucket) HeadObjectRequest(objectKey string, input *HeadObjectInput) (*request.Request, *HeadObjectOutput, error) {

	if input == nil {
		input = &HeadObjectInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "HEAD Object",
		RequestMethod: "HEAD",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &HeadObjectOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// HeadObjectInput presents input for HeadObject.
type HeadObjectInput struct {
	// Check whether the ETag matches
	IfMatch *string `json:"If-Match,omitempty" name:"If-Match" location:"headers"`
	// Check whether the object has been modified
	IfModifiedSince *time.Time `json:"If-Modified-Since,omitempty" name:"If-Modified-Since" format:"RFC 822" location:"headers"`
	// Check whether the ETag does not match
	IfNoneMatch *string `json:"If-None-Match,omitempty" name:"If-None-Match" location:"headers"`
	// Check whether the object has not been modified
	IfUnmodifiedSince *time.Time `json:"If-Unmodified-Since,omitempty" name:"If-Unmodified-Since" format:"RFC 822" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSEncryptionCustomerKey *string `json:"X-QS-Encryption-Customer-Key,omitempty" name:"X-QS-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSEncryptionCustomerKeyMD5 *string `json:"X-QS-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Encryption-Customer-Key-MD5" location:"headers"`
}

// Validate validates the input for HeadObject.
func (v *HeadObjectInput) Validate() error {

	return nil
}

// HeadObjectOutput presents output for HeadObject.
type HeadObjectOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Object content length
	ContentLength *int64 `json:"Content-Length,omitempty" name:"Content-Length" location:"headers"`
	// Object content type
	ContentType *string `json:"Content-Type,omitempty" name:"Content-Type" location:"headers"`
	// MD5sum of the object
	ETag         *string    `json:"ETag,omitempty" name:"ETag" location:"headers"`
	LastModified *time.Time `json:"Last-Modified,omitempty" name:"Last-Modified" format:"RFC 822" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// User-defined metadata
	XQSMetaData *map[string]string `json:"X-QS-MetaData,omitempty" name:"X-QS-MetaData" location:"headers"`
	// Storage class of the object
	XQSStorageClass *string `json:"X-QS-Storage-Class,omitempty" name:"X-QS-Storage-Class" location:"headers"`
}

// ImageProcess does Image process with the action on the object
// Documentation URL: https://docs.qingcloud.com/qingstor/data_process/image_process/index.html
func (s *Bucket) ImageProcess(objectKey string, input *ImageProcessInput) (*ImageProcessOutput, error) {
	r, x, err := s.ImageProcessRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// ImageProcessRequest creates request and output object of ImageProcess.
func (s *Bucket) ImageProcessRequest(objectKey string, input *ImageProcessInput) (*request.Request, *ImageProcessOutput, error) {

	if input == nil {
		input = &ImageProcessInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "Image Process",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>/<object-key>?image",
		StatusCodes: []int{
			200, // OK
			304, // Not modified
		},
	}

	x := &ImageProcessOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// ImageProcessInput presents input for ImageProcess.
type ImageProcessInput struct {
	// Image process action
	Action *string `json:"action" name:"action" location:"query"` // Required
	// Specified the Cache-Control response header
	ResponseCacheControl *string `json:"response-cache-control,omitempty" name:"response-cache-control" location:"query"`
	// Specified the Content-Disposition response header
	ResponseContentDisposition *string `json:"response-content-disposition,omitempty" name:"response-content-disposition" location:"query"`
	// Specified the Content-Encoding response header
	ResponseContentEncoding *string `json:"response-content-encoding,omitempty" name:"response-content-encoding" location:"query"`
	// Specified the Content-Language response header
	ResponseContentLanguage *string `json:"response-content-language,omitempty" name:"response-content-language" location:"query"`
	// Specified the Content-Type response header
	ResponseContentType *string `json:"response-content-type,omitempty" name:"response-content-type" location:"query"`
	// Specified the Expires response header
	ResponseExpires *string `json:"response-expires,omitempty" name:"response-expires" location:"query"`

	// Check whether the object has been modified
	IfModifiedSince *time.Time `json:"If-Modified-Since,omitempty" name:"If-Modified-Since" format:"RFC 822" location:"headers"`
}

// Validate validates the input for ImageProcess.
func (v *ImageProcessInput) Validate() error {

	if v.Action == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Action",
			ParentName:    "ImageProcessInput",
		}
	}

	return nil
}

// ImageProcessOutput presents output for ImageProcess.
type ImageProcessOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// The response body
	Body io.ReadCloser `location:"body"`

	// Object content length
	ContentLength *int64 `json:"Content-Length,omitempty" name:"Content-Length" location:"headers"`
}

// Close will close the underlay body.
func (o *ImageProcessOutput) Close() (err error) {
	if o.Body != nil {
		return o.Body.Close()
	}
	return
}

// InitiateMultipartUpload does Initial multipart upload on the object.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/initiate_multipart_upload.html
func (s *Bucket) InitiateMultipartUpload(objectKey string, input *InitiateMultipartUploadInput) (*InitiateMultipartUploadOutput, error) {
	r, x, err := s.InitiateMultipartUploadRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// InitiateMultipartUploadRequest creates request and output object of InitiateMultipartUpload.
func (s *Bucket) InitiateMultipartUploadRequest(objectKey string, input *InitiateMultipartUploadInput) (*request.Request, *InitiateMultipartUploadOutput, error) {

	if input == nil {
		input = &InitiateMultipartUploadInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "Initiate Multipart Upload",
		RequestMethod: "POST",
		RequestURI:    "/<bucket-name>/<object-key>?uploads",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &InitiateMultipartUploadOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// InitiateMultipartUploadInput presents input for InitiateMultipartUpload.
type InitiateMultipartUploadInput struct {
	// Object content type
	ContentType *string `json:"Content-Type,omitempty" name:"Content-Type" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSEncryptionCustomerKey *string `json:"X-QS-Encryption-Customer-Key,omitempty" name:"X-QS-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSEncryptionCustomerKeyMD5 *string `json:"X-QS-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Encryption-Customer-Key-MD5" location:"headers"`
	// User-defined metadata
	XQSMetaData *map[string]string `json:"X-QS-MetaData,omitempty" name:"X-QS-MetaData" location:"headers"`
	// Specify the storage class for object
	// XQSStorageClass's available values: STANDARD, STANDARD_IA
	XQSStorageClass *string `json:"X-QS-Storage-Class,omitempty" name:"X-QS-Storage-Class" location:"headers"`
}

// Validate validates the input for InitiateMultipartUpload.
func (v *InitiateMultipartUploadInput) Validate() error {

	if v.XQSMetaData != nil {
		XQSMetaDataerr := utils.IsMetaDataValid(v.XQSMetaData)
		if XQSMetaDataerr != nil {
			return XQSMetaDataerr
		}
	}

	if v.XQSStorageClass != nil {
		xQSStorageClassValidValues := []string{"STANDARD", "STANDARD_IA"}
		xQSStorageClassParameterValue := fmt.Sprint(*v.XQSStorageClass)

		xQSStorageClassIsValid := false
		for _, value := range xQSStorageClassValidValues {
			if value == xQSStorageClassParameterValue {
				xQSStorageClassIsValid = true
			}
		}

		if !xQSStorageClassIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "XQSStorageClass",
				ParameterValue: xQSStorageClassParameterValue,
				AllowedValues:  xQSStorageClassValidValues,
			}
		}
	}

	return nil
}

// InitiateMultipartUploadOutput presents output for InitiateMultipartUpload.
type InitiateMultipartUploadOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Bucket name
	Bucket *string `json:"bucket,omitempty" name:"bucket" location:"elements"`
	// Object key
	Key *string `json:"key,omitempty" name:"key" location:"elements"`
	// Object multipart upload ID
	UploadID *string `json:"upload_id,omitempty" name:"upload_id" location:"elements"`

	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
}

// ListMultipart does List object parts.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/list_multipart.html
func (s *Bucket) ListMultipart(objectKey string, input *ListMultipartInput) (*ListMultipartOutput, error) {
	r, x, err := s.ListMultipartRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// ListMultipartRequest creates request and output object of ListMultipart.
func (s *Bucket) ListMultipartRequest(objectKey string, input *ListMultipartInput) (*request.Request, *ListMultipartOutput, error) {

	if input == nil {
		input = &ListMultipartInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "List Multipart",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &ListMultipartOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// ListMultipartInput presents input for ListMultipart.
type ListMultipartInput struct {
	// Limit results count
	Limit *int `json:"limit,omitempty" name:"limit" location:"query"`
	// Object multipart upload part number
	PartNumberMarker *int `json:"part_number_marker,omitempty" name:"part_number_marker" location:"query"`
	// Object multipart upload ID
	UploadID *string `json:"upload_id" name:"upload_id" location:"query"` // Required

}

// Validate validates the input for ListMultipart.
func (v *ListMultipartInput) Validate() error {

	if v.UploadID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "UploadID",
			ParentName:    "ListMultipartInput",
		}
	}

	return nil
}

// ListMultipartOutput presents output for ListMultipart.
type ListMultipartOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Object multipart count
	Count *int `json:"count,omitempty" name:"count" location:"elements"`
	// Object parts
	ObjectParts []*ObjectPartType `json:"object_parts,omitempty" name:"object_parts" location:"elements"`
}

// OptionsObject does Check whether the object accepts a origin with method and header.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/options.html
func (s *Bucket) OptionsObject(objectKey string, input *OptionsObjectInput) (*OptionsObjectOutput, error) {
	r, x, err := s.OptionsObjectRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// OptionsObjectRequest creates request and output object of OptionsObject.
func (s *Bucket) OptionsObjectRequest(objectKey string, input *OptionsObjectInput) (*request.Request, *OptionsObjectOutput, error) {

	if input == nil {
		input = &OptionsObjectInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "OPTIONS Object",
		RequestMethod: "OPTIONS",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			200, // OK
			304, // Object not modified
			412, // Object precondition failed
		},
	}

	x := &OptionsObjectOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// OptionsObjectInput presents input for OptionsObject.
type OptionsObjectInput struct {
	// Request headers
	AccessControlRequestHeaders *string `json:"Access-Control-Request-Headers,omitempty" name:"Access-Control-Request-Headers" location:"headers"`
	// Request method
	AccessControlRequestMethod *string `json:"Access-Control-Request-Method" name:"Access-Control-Request-Method" location:"headers"` // Required
	// Request origin
	Origin *string `json:"Origin" name:"Origin" location:"headers"` // Required

}

// Validate validates the input for OptionsObject.
func (v *OptionsObjectInput) Validate() error {

	if v.AccessControlRequestMethod == nil {
		return errors.ParameterRequiredError{
			ParameterName: "AccessControlRequestMethod",
			ParentName:    "OptionsObjectInput",
		}
	}

	if v.Origin == nil {
		return errors.ParameterRequiredError{
			ParameterName: "Origin",
			ParentName:    "OptionsObjectInput",
		}
	}

	return nil
}

// OptionsObjectOutput presents output for OptionsObject.
type OptionsObjectOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Allowed headers
	AccessControlAllowHeaders *string `json:"Access-Control-Allow-Headers,omitempty" name:"Access-Control-Allow-Headers" location:"headers"`
	// Allowed methods
	AccessControlAllowMethods *string `json:"Access-Control-Allow-Methods,omitempty" name:"Access-Control-Allow-Methods" location:"headers"`
	// Allowed origin
	AccessControlAllowOrigin *string `json:"Access-Control-Allow-Origin,omitempty" name:"Access-Control-Allow-Origin" location:"headers"`
	// Expose headers
	AccessControlExposeHeaders *string `json:"Access-Control-Expose-Headers,omitempty" name:"Access-Control-Expose-Headers" location:"headers"`
	// Max age
	AccessControlMaxAge *string `json:"Access-Control-Max-Age,omitempty" name:"Access-Control-Max-Age" location:"headers"`
}

// PutObject does Upload the object.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/put.html
func (s *Bucket) PutObject(objectKey string, input *PutObjectInput) (*PutObjectOutput, error) {
	r, x, err := s.PutObjectRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// PutObjectRequest creates request and output object of PutObject.
func (s *Bucket) PutObjectRequest(objectKey string, input *PutObjectInput) (*request.Request, *PutObjectOutput, error) {

	if input == nil {
		input = &PutObjectInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Object",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			201, // Object created
		},
	}

	x := &PutObjectOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutObjectInput presents input for PutObject.
type PutObjectInput struct {
	// Object cache control
	CacheControl *string `json:"Cache-Control,omitempty" name:"Cache-Control" location:"headers"`
	// Object content encoding
	ContentEncoding *string `json:"Content-Encoding,omitempty" name:"Content-Encoding" location:"headers"`
	// Object content size
	ContentLength *int64 `json:"Content-Length" name:"Content-Length" location:"headers"` // Required
	// Object MD5sum
	ContentMD5 *string `json:"Content-MD5,omitempty" name:"Content-MD5" location:"headers"`
	// Object content type
	ContentType *string `json:"Content-Type,omitempty" name:"Content-Type" location:"headers"`
	// Used to indicate that particular server behaviors are required by the client
	Expect *string `json:"Expect,omitempty" name:"Expect" location:"headers"`
	// Copy source, format (/<bucket-name>/<object-key>)
	XQSCopySource *string `json:"X-QS-Copy-Source,omitempty" name:"X-QS-Copy-Source" location:"headers"`
	// Encryption algorithm of the object
	XQSCopySourceEncryptionCustomerAlgorithm *string `json:"X-QS-Copy-Source-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Copy-Source-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSCopySourceEncryptionCustomerKey *string `json:"X-QS-Copy-Source-Encryption-Customer-Key,omitempty" name:"X-QS-Copy-Source-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSCopySourceEncryptionCustomerKeyMD5 *string `json:"X-QS-Copy-Source-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Copy-Source-Encryption-Customer-Key-MD5" location:"headers"`
	// Check whether the copy source matches
	XQSCopySourceIfMatch *string `json:"X-QS-Copy-Source-If-Match,omitempty" name:"X-QS-Copy-Source-If-Match" location:"headers"`
	// Check whether the copy source has been modified
	XQSCopySourceIfModifiedSince *time.Time `json:"X-QS-Copy-Source-If-Modified-Since,omitempty" name:"X-QS-Copy-Source-If-Modified-Since" format:"RFC 822" location:"headers"`
	// Check whether the copy source does not match
	XQSCopySourceIfNoneMatch *string `json:"X-QS-Copy-Source-If-None-Match,omitempty" name:"X-QS-Copy-Source-If-None-Match" location:"headers"`
	// Check whether the copy source has not been modified
	XQSCopySourceIfUnmodifiedSince *time.Time `json:"X-QS-Copy-Source-If-Unmodified-Since,omitempty" name:"X-QS-Copy-Source-If-Unmodified-Since" format:"RFC 822" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSEncryptionCustomerKey *string `json:"X-QS-Encryption-Customer-Key,omitempty" name:"X-QS-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSEncryptionCustomerKeyMD5 *string `json:"X-QS-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Encryption-Customer-Key-MD5" location:"headers"`
	// Check whether fetch target object has not been modified
	XQSFetchIfUnmodifiedSince *time.Time `json:"X-QS-Fetch-If-Unmodified-Since,omitempty" name:"X-QS-Fetch-If-Unmodified-Since" format:"RFC 822" location:"headers"`
	// Fetch source, should be a valid url
	XQSFetchSource *string `json:"X-QS-Fetch-Source,omitempty" name:"X-QS-Fetch-Source" location:"headers"`
	// User-defined metadata
	XQSMetaData *map[string]string `json:"X-QS-MetaData,omitempty" name:"X-QS-MetaData" location:"headers"`
	// Move source, format (/<bucket-name>/<object-key>)
	XQSMoveSource *string `json:"X-QS-Move-Source,omitempty" name:"X-QS-Move-Source" location:"headers"`
	// Specify the storage class for object
	// XQSStorageClass's available values: STANDARD, STANDARD_IA
	XQSStorageClass *string `json:"X-QS-Storage-Class,omitempty" name:"X-QS-Storage-Class" location:"headers"`

	// The request body
	Body io.Reader `location:"body"`
}

// Validate validates the input for PutObject.
func (v *PutObjectInput) Validate() error {

	if v.XQSMetaData != nil {
		XQSMetaDataerr := utils.IsMetaDataValid(v.XQSMetaData)
		if XQSMetaDataerr != nil {
			return XQSMetaDataerr
		}
	}

	if v.XQSStorageClass != nil {
		xQSStorageClassValidValues := []string{"STANDARD", "STANDARD_IA"}
		xQSStorageClassParameterValue := fmt.Sprint(*v.XQSStorageClass)

		xQSStorageClassIsValid := false
		for _, value := range xQSStorageClassValidValues {
			if value == xQSStorageClassParameterValue {
				xQSStorageClassIsValid = true
			}
		}

		if !xQSStorageClassIsValid {
			return errors.ParameterValueNotAllowedError{
				ParameterName:  "XQSStorageClass",
				ParameterValue: xQSStorageClassParameterValue,
				AllowedValues:  xQSStorageClassValidValues,
			}
		}
	}

	return nil
}

// PutObjectOutput presents output for PutObject.
type PutObjectOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// MD5sum of the object
	ETag *string `json:"ETag,omitempty" name:"ETag" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
}

// UploadMultipart does Upload object multipart.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/object/multipart/upload_multipart.html
func (s *Bucket) UploadMultipart(objectKey string, input *UploadMultipartInput) (*UploadMultipartOutput, error) {
	r, x, err := s.UploadMultipartRequest(objectKey, input)

	if err != nil {
		return x, err
	}

	err = r.Send()
	if err != nil {
		return nil, err
	}

	requestID := r.HTTPResponse.Header.Get(http.CanonicalHeaderKey("X-QS-Request-ID"))
	x.RequestID = &requestID

	return x, err
}

// UploadMultipartRequest creates request and output object of UploadMultipart.
func (s *Bucket) UploadMultipartRequest(objectKey string, input *UploadMultipartInput) (*request.Request, *UploadMultipartOutput, error) {

	if input == nil {
		input = &UploadMultipartInput{}
	}

	properties := *s.Properties

	properties.ObjectKey = &objectKey

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "Upload Multipart",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			201, // Object multipart created
		},
	}

	x := &UploadMultipartOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// UploadMultipartInput presents input for UploadMultipart.
type UploadMultipartInput struct {
	// Object multipart upload part number
	PartNumber *int `json:"part_number" name:"part_number" default:"0" location:"query"` // Required
	// Object multipart upload ID
	UploadID *string `json:"upload_id" name:"upload_id" location:"query"` // Required

	// Object multipart content length
	ContentLength *int64 `json:"Content-Length,omitempty" name:"Content-Length" location:"headers"`
	// Object multipart content MD5sum
	ContentMD5 *string `json:"Content-MD5,omitempty" name:"Content-MD5" location:"headers"`
	// Specify range of the source object
	XQSCopyRange *string `json:"X-QS-Copy-Range,omitempty" name:"X-QS-Copy-Range" location:"headers"`
	// Copy source, format (/<bucket-name>/<object-key>)
	XQSCopySource *string `json:"X-QS-Copy-Source,omitempty" name:"X-QS-Copy-Source" location:"headers"`
	// Encryption algorithm of the object
	XQSCopySourceEncryptionCustomerAlgorithm *string `json:"X-QS-Copy-Source-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Copy-Source-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSCopySourceEncryptionCustomerKey *string `json:"X-QS-Copy-Source-Encryption-Customer-Key,omitempty" name:"X-QS-Copy-Source-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSCopySourceEncryptionCustomerKeyMD5 *string `json:"X-QS-Copy-Source-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Copy-Source-Encryption-Customer-Key-MD5" location:"headers"`
	// Check whether the Etag of copy source matches the specified value
	XQSCopySourceIfMatch *string `json:"X-QS-Copy-Source-If-Match,omitempty" name:"X-QS-Copy-Source-If-Match" location:"headers"`
	// Check whether the copy source has been modified since the specified date
	XQSCopySourceIfModifiedSince *time.Time `json:"X-QS-Copy-Source-If-Modified-Since,omitempty" name:"X-QS-Copy-Source-If-Modified-Since" format:"RFC 822" location:"headers"`
	// Check whether the Etag of copy source does not matches the specified value
	XQSCopySourceIfNoneMatch *string `json:"X-QS-Copy-Source-If-None-Match,omitempty" name:"X-QS-Copy-Source-If-None-Match" location:"headers"`
	// Check whether the copy source has not been unmodified since the specified date
	XQSCopySourceIfUnmodifiedSince *time.Time `json:"X-QS-Copy-Source-If-Unmodified-Since,omitempty" name:"X-QS-Copy-Source-If-Unmodified-Since" format:"RFC 822" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
	// Encryption key of the object
	XQSEncryptionCustomerKey *string `json:"X-QS-Encryption-Customer-Key,omitempty" name:"X-QS-Encryption-Customer-Key" location:"headers"`
	// MD5sum of encryption key
	XQSEncryptionCustomerKeyMD5 *string `json:"X-QS-Encryption-Customer-Key-MD5,omitempty" name:"X-QS-Encryption-Customer-Key-MD5" location:"headers"`

	// The request body
	Body io.Reader `location:"body"`
}

// Validate validates the input for UploadMultipart.
func (v *UploadMultipartInput) Validate() error {

	if v.PartNumber == nil {
		return errors.ParameterRequiredError{
			ParameterName: "PartNumber",
			ParentName:    "UploadMultipartInput",
		}
	}

	if v.UploadID == nil {
		return errors.ParameterRequiredError{
			ParameterName: "UploadID",
			ParentName:    "UploadMultipartInput",
		}
	}

	return nil
}

// UploadMultipartOutput presents output for UploadMultipart.
type UploadMultipartOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// MD5sum of the object
	ETag *string `json:"ETag,omitempty" name:"ETag" location:"headers"`
	// Range of response data content
	XQSContentCopyRange *string `json:"X-QS-Content-Copy-Range,omitempty" name:"X-QS-Content-Copy-Range" location:"headers"`
	// Encryption algorithm of the object
	XQSEncryptionCustomerAlgorithm *string `json:"X-QS-Encryption-Customer-Algorithm,omitempty" name:"X-QS-Encryption-Customer-Algorithm" location:"headers"`
}
