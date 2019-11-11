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

// Bucket presents bucket.
type Bucket struct {
	Config     *config.Config
	Properties *Properties
}

// Bucket initializes a new bucket.
func (s *Service) Bucket(bucketName string, zone string) (*Bucket, error) {
	zone = strings.ToLower(zone)
	properties := &Properties{
		BucketName: &bucketName,
		Zone:       &zone,
	}

	return &Bucket{Config: s.Config, Properties: properties}, nil
}

// Delete does Delete a bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/delete.html
func (s *Bucket) Delete() (*DeleteBucketOutput, error) {
	r, x, err := s.DeleteRequest()

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

// DeleteRequest creates request and output object of DeleteBucket.
func (s *Bucket) DeleteRequest() (*request.Request, *DeleteBucketOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Bucket",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>",
		StatusCodes: []int{
			204, // Bucket deleted
		},
	}

	x := &DeleteBucketOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteBucketOutput presents output for DeleteBucket.
type DeleteBucketOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// DeleteCORS does Delete CORS information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/cors/delete_cors.html
func (s *Bucket) DeleteCORS() (*DeleteBucketCORSOutput, error) {
	r, x, err := s.DeleteCORSRequest()

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

// DeleteCORSRequest creates request and output object of DeleteBucketCORS.
func (s *Bucket) DeleteCORSRequest() (*request.Request, *DeleteBucketCORSOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Bucket CORS",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>?cors",
		StatusCodes: []int{
			204, // OK
		},
	}

	x := &DeleteBucketCORSOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteBucketCORSOutput presents output for DeleteBucketCORS.
type DeleteBucketCORSOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// DeleteExternalMirror does Delete external mirror of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/external_mirror/delete_external_mirror.html
func (s *Bucket) DeleteExternalMirror() (*DeleteBucketExternalMirrorOutput, error) {
	r, x, err := s.DeleteExternalMirrorRequest()

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

// DeleteExternalMirrorRequest creates request and output object of DeleteBucketExternalMirror.
func (s *Bucket) DeleteExternalMirrorRequest() (*request.Request, *DeleteBucketExternalMirrorOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Bucket External Mirror",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>?mirror",
		StatusCodes: []int{
			204, // No content
		},
	}

	x := &DeleteBucketExternalMirrorOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteBucketExternalMirrorOutput presents output for DeleteBucketExternalMirror.
type DeleteBucketExternalMirrorOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// DeleteLifecycle does Delete Lifecycle information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/lifecycle/delete_lifecycle.html
func (s *Bucket) DeleteLifecycle() (*DeleteBucketLifecycleOutput, error) {
	r, x, err := s.DeleteLifecycleRequest()

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

// DeleteLifecycleRequest creates request and output object of DeleteBucketLifecycle.
func (s *Bucket) DeleteLifecycleRequest() (*request.Request, *DeleteBucketLifecycleOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Bucket Lifecycle",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>?lifecycle",
		StatusCodes: []int{
			204, // Lifecycle deleted
		},
	}

	x := &DeleteBucketLifecycleOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteBucketLifecycleOutput presents output for DeleteBucketLifecycle.
type DeleteBucketLifecycleOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// DeleteNotification does Delete Notification information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/notification/delete_notification.html
func (s *Bucket) DeleteNotification() (*DeleteBucketNotificationOutput, error) {
	r, x, err := s.DeleteNotificationRequest()

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

// DeleteNotificationRequest creates request and output object of DeleteBucketNotification.
func (s *Bucket) DeleteNotificationRequest() (*request.Request, *DeleteBucketNotificationOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Bucket Notification",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>?notification",
		StatusCodes: []int{
			204, // notification deleted
		},
	}

	x := &DeleteBucketNotificationOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteBucketNotificationOutput presents output for DeleteBucketNotification.
type DeleteBucketNotificationOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// DeletePolicy does Delete policy information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/policy/delete_policy.html
func (s *Bucket) DeletePolicy() (*DeleteBucketPolicyOutput, error) {
	r, x, err := s.DeletePolicyRequest()

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

// DeletePolicyRequest creates request and output object of DeleteBucketPolicy.
func (s *Bucket) DeletePolicyRequest() (*request.Request, *DeleteBucketPolicyOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "DELETE Bucket Policy",
		RequestMethod: "DELETE",
		RequestURI:    "/<bucket-name>?policy",
		StatusCodes: []int{
			204, // No content
		},
	}

	x := &DeleteBucketPolicyOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteBucketPolicyOutput presents output for DeleteBucketPolicy.
type DeleteBucketPolicyOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// DeleteMultipleObjects does Delete multiple objects from the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/delete_multiple.html
func (s *Bucket) DeleteMultipleObjects(input *DeleteMultipleObjectsInput) (*DeleteMultipleObjectsOutput, error) {
	r, x, err := s.DeleteMultipleObjectsRequest(input)

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

// DeleteMultipleObjectsRequest creates request and output object of DeleteMultipleObjects.
func (s *Bucket) DeleteMultipleObjectsRequest(input *DeleteMultipleObjectsInput) (*request.Request, *DeleteMultipleObjectsOutput, error) {

	if input == nil {
		input = &DeleteMultipleObjectsInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "Delete Multiple Objects",
		RequestMethod: "POST",
		RequestURI:    "/<bucket-name>?delete",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &DeleteMultipleObjectsOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// DeleteMultipleObjectsInput presents input for DeleteMultipleObjects.
type DeleteMultipleObjectsInput struct {

	// A list of keys to delete
	Objects []*KeyType `json:"objects" name:"objects" location:"elements"` // Required
	// Whether to return the list of deleted objects
	Quiet *bool `json:"quiet,omitempty" name:"quiet" location:"elements"`
}

// Validate validates the input for DeleteMultipleObjects.
func (v *DeleteMultipleObjectsInput) Validate() error {

	if len(v.Objects) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "Objects",
			ParentName:    "DeleteMultipleObjectsInput",
		}
	}

	if len(v.Objects) > 0 {
		for _, property := range v.Objects {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteMultipleObjectsOutput presents output for DeleteMultipleObjects.
type DeleteMultipleObjectsOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// List of deleted objects
	Deleted []*KeyType `json:"deleted,omitempty" name:"deleted" location:"elements"`
	// Error messages
	Errors []*KeyDeleteErrorType `json:"errors,omitempty" name:"errors" location:"elements"`
}

// GetACL does Get ACL information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/get_acl.html
func (s *Bucket) GetACL() (*GetBucketACLOutput, error) {
	r, x, err := s.GetACLRequest()

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

// GetACLRequest creates request and output object of GetBucketACL.
func (s *Bucket) GetACLRequest() (*request.Request, *GetBucketACLOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket ACL",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?acl",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketACLOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketACLOutput presents output for GetBucketACL.
type GetBucketACLOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Bucket ACL rules
	ACL []*ACLType `json:"acl,omitempty" name:"acl" location:"elements"`
	// Bucket owner
	Owner *OwnerType `json:"owner,omitempty" name:"owner" location:"elements"`
}

// GetCORS does Get CORS information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/cors/get_cors.html
func (s *Bucket) GetCORS() (*GetBucketCORSOutput, error) {
	r, x, err := s.GetCORSRequest()

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

// GetCORSRequest creates request and output object of GetBucketCORS.
func (s *Bucket) GetCORSRequest() (*request.Request, *GetBucketCORSOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket CORS",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?cors",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketCORSOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketCORSOutput presents output for GetBucketCORS.
type GetBucketCORSOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Bucket CORS rules
	CORSRules []*CORSRuleType `json:"cors_rules,omitempty" name:"cors_rules" location:"elements"`
}

// GetExternalMirror does Get external mirror of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/external_mirror/get_external_mirror.html
func (s *Bucket) GetExternalMirror() (*GetBucketExternalMirrorOutput, error) {
	r, x, err := s.GetExternalMirrorRequest()

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

// GetExternalMirrorRequest creates request and output object of GetBucketExternalMirror.
func (s *Bucket) GetExternalMirrorRequest() (*request.Request, *GetBucketExternalMirrorOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket External Mirror",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?mirror",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketExternalMirrorOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketExternalMirrorOutput presents output for GetBucketExternalMirror.
type GetBucketExternalMirrorOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Source site url
	SourceSite *string `json:"source_site,omitempty" name:"source_site" location:"elements"`
}

// GetLifecycle does Get Lifecycle information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/lifecycle/get_lifecycle.html
func (s *Bucket) GetLifecycle() (*GetBucketLifecycleOutput, error) {
	r, x, err := s.GetLifecycleRequest()

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

// GetLifecycleRequest creates request and output object of GetBucketLifecycle.
func (s *Bucket) GetLifecycleRequest() (*request.Request, *GetBucketLifecycleOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket Lifecycle",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?lifecycle",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketLifecycleOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketLifecycleOutput presents output for GetBucketLifecycle.
type GetBucketLifecycleOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Bucket Lifecycle rule
	Rule []*RuleType `json:"rule,omitempty" name:"rule" location:"elements"`
}

// GetNotification does Get Notification information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/notification/get_notification.html
func (s *Bucket) GetNotification() (*GetBucketNotificationOutput, error) {
	r, x, err := s.GetNotificationRequest()

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

// GetNotificationRequest creates request and output object of GetBucketNotification.
func (s *Bucket) GetNotificationRequest() (*request.Request, *GetBucketNotificationOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket Notification",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?notification",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketNotificationOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketNotificationOutput presents output for GetBucketNotification.
type GetBucketNotificationOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Bucket Notification
	Notifications []*NotificationType `json:"notifications,omitempty" name:"notifications" location:"elements"`
}

// GetPolicy does Get policy information of the bucket.
// Documentation URL: https://https://docs.qingcloud.com/qingstor/api/bucket/policy/get_policy.html
func (s *Bucket) GetPolicy() (*GetBucketPolicyOutput, error) {
	r, x, err := s.GetPolicyRequest()

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

// GetPolicyRequest creates request and output object of GetBucketPolicy.
func (s *Bucket) GetPolicyRequest() (*request.Request, *GetBucketPolicyOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket Policy",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?policy",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketPolicyOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketPolicyOutput presents output for GetBucketPolicy.
type GetBucketPolicyOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Bucket policy statement
	Statement []*StatementType `json:"statement,omitempty" name:"statement" location:"elements"`
}

// GetStatistics does Get statistics information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/get_stats.html
func (s *Bucket) GetStatistics() (*GetBucketStatisticsOutput, error) {
	r, x, err := s.GetStatisticsRequest()

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

// GetStatisticsRequest creates request and output object of GetBucketStatistics.
func (s *Bucket) GetStatisticsRequest() (*request.Request, *GetBucketStatisticsOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket Statistics",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?stats",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &GetBucketStatisticsOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// GetBucketStatisticsOutput presents output for GetBucketStatistics.
type GetBucketStatisticsOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Objects count in the bucket
	Count *int64 `json:"count,omitempty" name:"count" location:"elements"`
	// Bucket created time
	Created *time.Time `json:"created,omitempty" name:"created" format:"ISO 8601" location:"elements"`
	// QingCloud Zone ID
	Location *string `json:"location,omitempty" name:"location" location:"elements"`
	// Bucket name
	Name *string `json:"name,omitempty" name:"name" location:"elements"`
	// Bucket storage size
	Size *int64 `json:"size,omitempty" name:"size" location:"elements"`
	// Bucket status
	// Status's available values: active, suspended
	Status *string `json:"status,omitempty" name:"status" location:"elements"`
	// URL to access the bucket
	URL *string `json:"url,omitempty" name:"url" location:"elements"`
}

// Head does Check whether the bucket exists and available.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/head.html
func (s *Bucket) Head() (*HeadBucketOutput, error) {
	r, x, err := s.HeadRequest()

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

// HeadRequest creates request and output object of HeadBucket.
func (s *Bucket) HeadRequest() (*request.Request, *HeadBucketOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "HEAD Bucket",
		RequestMethod: "HEAD",
		RequestURI:    "/<bucket-name>",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &HeadBucketOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// HeadBucketOutput presents output for HeadBucket.
type HeadBucketOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// ListMultipartUploads does List multipart uploads in the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/list_multipart_uploads.html
func (s *Bucket) ListMultipartUploads(input *ListMultipartUploadsInput) (*ListMultipartUploadsOutput, error) {
	r, x, err := s.ListMultipartUploadsRequest(input)

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

// ListMultipartUploadsRequest creates request and output object of ListMultipartUploads.
func (s *Bucket) ListMultipartUploadsRequest(input *ListMultipartUploadsInput) (*request.Request, *ListMultipartUploadsOutput, error) {

	if input == nil {
		input = &ListMultipartUploadsInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "List Multipart Uploads",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>?uploads",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &ListMultipartUploadsOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// ListMultipartUploadsInput presents input for ListMultipartUploads.
type ListMultipartUploadsInput struct {
	// Put all keys that share a common prefix into a list
	Delimiter *string `json:"delimiter,omitempty" name:"delimiter" location:"query"`
	// Limit results returned from the first key after key_marker sorted by alphabetical order
	KeyMarker *string `json:"key_marker,omitempty" name:"key_marker" location:"query"`
	// Results count limit
	Limit *int `json:"limit,omitempty" name:"limit" location:"query"`
	// Limits results to keys that begin with the prefix
	Prefix *string `json:"prefix,omitempty" name:"prefix" location:"query"`
	// Limit results returned from the first uploading segment after upload_id_marker sorted by the time of upload_id
	UploadIDMarker *string `json:"upload_id_marker,omitempty" name:"upload_id_marker" location:"query"`
}

// Validate validates the input for ListMultipartUploads.
func (v *ListMultipartUploadsInput) Validate() error {

	return nil
}

// ListMultipartUploadsOutput presents output for ListMultipartUploads.
type ListMultipartUploadsOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Other object keys that share common prefixes
	CommonPrefixes []*string `json:"common_prefixes,omitempty" name:"common_prefixes" location:"elements"`
	// Delimiter that specified in request parameters
	Delimiter *string `json:"delimiter,omitempty" name:"delimiter" location:"elements"`
	// Indicate if these are more results in the next page
	HasMore *bool `json:"has_more,omitempty" name:"has_more" location:"elements"`
	// Limit that specified in request parameters
	Limit *int `json:"limit,omitempty" name:"limit" location:"elements"`
	// Marker that specified in request parameters
	Marker *string `json:"marker,omitempty" name:"marker" location:"elements"`
	// Bucket name
	Name *string `json:"name,omitempty" name:"name" location:"elements"`
	// The last key in uploads list
	NextKeyMarker *string `json:"next_key_marker,omitempty" name:"next_key_marker" location:"elements"`
	// The last upload_id in uploads list
	NextUploadIDMarker *string `json:"next_upload_id_marker,omitempty" name:"next_upload_id_marker" location:"elements"`
	// Prefix that specified in request parameters
	Prefix *string `json:"prefix,omitempty" name:"prefix" location:"elements"`
	// Multipart uploads
	Uploads []*UploadsType `json:"uploads,omitempty" name:"uploads" location:"elements"`
}

// ListObjects does Retrieve the object list in a bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/get.html
func (s *Bucket) ListObjects(input *ListObjectsInput) (*ListObjectsOutput, error) {
	r, x, err := s.ListObjectsRequest(input)

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

// ListObjectsRequest creates request and output object of ListObjects.
func (s *Bucket) ListObjectsRequest(input *ListObjectsInput) (*request.Request, *ListObjectsOutput, error) {

	if input == nil {
		input = &ListObjectsInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "GET Bucket (List Objects)",
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &ListObjectsOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// ListObjectsInput presents input for ListObjects.
type ListObjectsInput struct {
	// Put all keys that share a common prefix into a list
	Delimiter *string `json:"delimiter,omitempty" name:"delimiter" location:"query"`
	// Results count limit
	Limit *int `json:"limit,omitempty" name:"limit" location:"query"`
	// Limit results to keys that start at this marker
	Marker *string `json:"marker,omitempty" name:"marker" location:"query"`
	// Limits results to keys that begin with the prefix
	Prefix *string `json:"prefix,omitempty" name:"prefix" location:"query"`
}

// Validate validates the input for ListObjects.
func (v *ListObjectsInput) Validate() error {

	return nil
}

// ListObjectsOutput presents output for ListObjects.
type ListObjectsOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Other object keys that share common prefixes
	CommonPrefixes []*string `json:"common_prefixes,omitempty" name:"common_prefixes" location:"elements"`
	// Delimiter that specified in request parameters
	Delimiter *string `json:"delimiter,omitempty" name:"delimiter" location:"elements"`
	// Indicate if these are more results in the next page
	HasMore *bool `json:"has_more,omitempty" name:"has_more" location:"elements"`
	// Object keys
	Keys []*KeyType `json:"keys,omitempty" name:"keys" location:"elements"`
	// Limit that specified in request parameters
	Limit *int `json:"limit,omitempty" name:"limit" location:"elements"`
	// Marker that specified in request parameters
	Marker *string `json:"marker,omitempty" name:"marker" location:"elements"`
	// Bucket name
	Name *string `json:"name,omitempty" name:"name" location:"elements"`
	// The last key in keys list
	NextMarker *string `json:"next_marker,omitempty" name:"next_marker" location:"elements"`
	// Bucket owner
	Owner *OwnerType `json:"owner,omitempty" name:"owner" location:"elements"`
	// Prefix that specified in request parameters
	Prefix *string `json:"prefix,omitempty" name:"prefix" location:"elements"`
}

// Put does Create a new bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/put.html
func (s *Bucket) Put() (*PutBucketOutput, error) {
	r, x, err := s.PutRequest()

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

// PutRequest creates request and output object of PutBucket.
func (s *Bucket) PutRequest() (*request.Request, *PutBucketOutput, error) {

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>",
		StatusCodes: []int{
			201, // Bucket created
		},
	}

	x := &PutBucketOutput{}
	r, err := request.New(o, nil, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketOutput presents output for PutBucket.
type PutBucketOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// PutACL does Set ACL information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/put_acl.html
func (s *Bucket) PutACL(input *PutBucketACLInput) (*PutBucketACLOutput, error) {
	r, x, err := s.PutACLRequest(input)

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

// PutACLRequest creates request and output object of PutBucketACL.
func (s *Bucket) PutACLRequest(input *PutBucketACLInput) (*request.Request, *PutBucketACLOutput, error) {

	if input == nil {
		input = &PutBucketACLInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket ACL",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>?acl",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &PutBucketACLOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketACLInput presents input for PutBucketACL.
type PutBucketACLInput struct {
	// Bucket ACL rules
	ACL []*ACLType `json:"acl" name:"acl" location:"elements"` // Required

}

// Validate validates the input for PutBucketACL.
func (v *PutBucketACLInput) Validate() error {

	if len(v.ACL) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "ACL",
			ParentName:    "PutBucketACLInput",
		}
	}

	if len(v.ACL) > 0 {
		for _, property := range v.ACL {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// PutBucketACLOutput presents output for PutBucketACL.
type PutBucketACLOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// PutCORS does Set CORS information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/cors/put_cors.html
func (s *Bucket) PutCORS(input *PutBucketCORSInput) (*PutBucketCORSOutput, error) {
	r, x, err := s.PutCORSRequest(input)

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

// PutCORSRequest creates request and output object of PutBucketCORS.
func (s *Bucket) PutCORSRequest(input *PutBucketCORSInput) (*request.Request, *PutBucketCORSOutput, error) {

	if input == nil {
		input = &PutBucketCORSInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket CORS",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>?cors",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &PutBucketCORSOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketCORSInput presents input for PutBucketCORS.
type PutBucketCORSInput struct {
	// Bucket CORS rules
	CORSRules []*CORSRuleType `json:"cors_rules" name:"cors_rules" location:"elements"` // Required

}

// Validate validates the input for PutBucketCORS.
func (v *PutBucketCORSInput) Validate() error {

	if len(v.CORSRules) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "CORSRules",
			ParentName:    "PutBucketCORSInput",
		}
	}

	if len(v.CORSRules) > 0 {
		for _, property := range v.CORSRules {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// PutBucketCORSOutput presents output for PutBucketCORS.
type PutBucketCORSOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// PutExternalMirror does Set external mirror of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/external_mirror/put_external_mirror.html
func (s *Bucket) PutExternalMirror(input *PutBucketExternalMirrorInput) (*PutBucketExternalMirrorOutput, error) {
	r, x, err := s.PutExternalMirrorRequest(input)

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

// PutExternalMirrorRequest creates request and output object of PutBucketExternalMirror.
func (s *Bucket) PutExternalMirrorRequest(input *PutBucketExternalMirrorInput) (*request.Request, *PutBucketExternalMirrorOutput, error) {

	if input == nil {
		input = &PutBucketExternalMirrorInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket External Mirror",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>?mirror",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &PutBucketExternalMirrorOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketExternalMirrorInput presents input for PutBucketExternalMirror.
type PutBucketExternalMirrorInput struct {
	// Source site url
	SourceSite *string `json:"source_site" name:"source_site" location:"elements"` // Required

}

// Validate validates the input for PutBucketExternalMirror.
func (v *PutBucketExternalMirrorInput) Validate() error {

	if v.SourceSite == nil {
		return errors.ParameterRequiredError{
			ParameterName: "SourceSite",
			ParentName:    "PutBucketExternalMirrorInput",
		}
	}

	return nil
}

// PutBucketExternalMirrorOutput presents output for PutBucketExternalMirror.
type PutBucketExternalMirrorOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// PutLifecycle does Set Lifecycle information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/lifecycle/put_lifecycle.html
func (s *Bucket) PutLifecycle(input *PutBucketLifecycleInput) (*PutBucketLifecycleOutput, error) {
	r, x, err := s.PutLifecycleRequest(input)

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

// PutLifecycleRequest creates request and output object of PutBucketLifecycle.
func (s *Bucket) PutLifecycleRequest(input *PutBucketLifecycleInput) (*request.Request, *PutBucketLifecycleOutput, error) {

	if input == nil {
		input = &PutBucketLifecycleInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket Lifecycle",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>?lifecycle",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &PutBucketLifecycleOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketLifecycleInput presents input for PutBucketLifecycle.
type PutBucketLifecycleInput struct {
	// Bucket Lifecycle rule
	Rule []*RuleType `json:"rule" name:"rule" location:"elements"` // Required

}

// Validate validates the input for PutBucketLifecycle.
func (v *PutBucketLifecycleInput) Validate() error {

	if len(v.Rule) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "Rule",
			ParentName:    "PutBucketLifecycleInput",
		}
	}

	if len(v.Rule) > 0 {
		for _, property := range v.Rule {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// PutBucketLifecycleOutput presents output for PutBucketLifecycle.
type PutBucketLifecycleOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// PutNotification does Set Notification information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/notification/put_notification.html
func (s *Bucket) PutNotification(input *PutBucketNotificationInput) (*PutBucketNotificationOutput, error) {
	r, x, err := s.PutNotificationRequest(input)

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

// PutNotificationRequest creates request and output object of PutBucketNotification.
func (s *Bucket) PutNotificationRequest(input *PutBucketNotificationInput) (*request.Request, *PutBucketNotificationOutput, error) {

	if input == nil {
		input = &PutBucketNotificationInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket Notification",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>?notification",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &PutBucketNotificationOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketNotificationInput presents input for PutBucketNotification.
type PutBucketNotificationInput struct {
	// Bucket Notification
	Notifications []*NotificationType `json:"notifications" name:"notifications" location:"elements"` // Required

}

// Validate validates the input for PutBucketNotification.
func (v *PutBucketNotificationInput) Validate() error {

	if len(v.Notifications) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "Notifications",
			ParentName:    "PutBucketNotificationInput",
		}
	}

	if len(v.Notifications) > 0 {
		for _, property := range v.Notifications {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// PutBucketNotificationOutput presents output for PutBucketNotification.
type PutBucketNotificationOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}

// PutPolicy does Set policy information of the bucket.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/bucket/policy/put_policy.html
func (s *Bucket) PutPolicy(input *PutBucketPolicyInput) (*PutBucketPolicyOutput, error) {
	r, x, err := s.PutPolicyRequest(input)

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

// PutPolicyRequest creates request and output object of PutBucketPolicy.
func (s *Bucket) PutPolicyRequest(input *PutBucketPolicyInput) (*request.Request, *PutBucketPolicyOutput, error) {

	if input == nil {
		input = &PutBucketPolicyInput{}
	}

	properties := *s.Properties

	o := &data.Operation{
		Config:        s.Config,
		Properties:    &properties,
		APIName:       "PUT Bucket Policy",
		RequestMethod: "PUT",
		RequestURI:    "/<bucket-name>?policy",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &PutBucketPolicyOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// PutBucketPolicyInput presents input for PutBucketPolicy.
type PutBucketPolicyInput struct {
	// Bucket policy statement
	Statement []*StatementType `json:"statement" name:"statement" location:"elements"` // Required

}

// Validate validates the input for PutBucketPolicy.
func (v *PutBucketPolicyInput) Validate() error {

	if len(v.Statement) == 0 {
		return errors.ParameterRequiredError{
			ParameterName: "Statement",
			ParentName:    "PutBucketPolicyInput",
		}
	}

	if len(v.Statement) > 0 {
		for _, property := range v.Statement {
			if err := property.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// PutBucketPolicyOutput presents output for PutBucketPolicy.
type PutBucketPolicyOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`
}
