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

// Package service provides QingStor Service API (API Version 2016-01-06)
package service

import (
	"net/http"

	"github.com/yunify/qingstor-sdk-go/v3/config"
	"github.com/yunify/qingstor-sdk-go/v3/request"
	"github.com/yunify/qingstor-sdk-go/v3/request/data"
)

var _ http.Header

// Service QingStor provides low-cost and reliable online storage service with unlimited storage space, high read and write performance, high reliability and data safety, fine-grained access control, and easy to use API.
type Service struct {
	Config *config.Config
}

// Init initializes a new service.
func Init(c *config.Config) (*Service, error) {
	return &Service{Config: c}, nil
}

// ListBuckets does Retrieve the bucket list.
// Documentation URL: https://docs.qingcloud.com/qingstor/api/service/get.html
func (s *Service) ListBuckets(input *ListBucketsInput) (*ListBucketsOutput, error) {
	r, x, err := s.ListBucketsRequest(input)

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

// ListBucketsRequest creates request and output object of ListBuckets.
func (s *Service) ListBucketsRequest(input *ListBucketsInput) (*request.Request, *ListBucketsOutput, error) {

	if input == nil {
		input = &ListBucketsInput{}
	}

	o := &data.Operation{
		Config:        s.Config,
		APIName:       "Get Service",
		RequestMethod: "GET",
		RequestURI:    "/",
		StatusCodes: []int{
			200, // OK
		},
	}

	x := &ListBucketsOutput{}
	r, err := request.New(o, input, x)
	if err != nil {
		return nil, nil, err
	}

	return r, x, nil
}

// ListBucketsInput presents input for ListBuckets.
type ListBucketsInput struct {
	// Limits results to buckets that in the location
	Location *string `json:"Location,omitempty" name:"Location" location:"headers"`
}

// Validate validates the input for ListBuckets.
func (v *ListBucketsInput) Validate() error {

	return nil
}

// ListBucketsOutput presents output for ListBuckets.
type ListBucketsOutput struct {
	StatusCode *int `location:"statusCode"`

	RequestID *string `location:"requestID"`

	// Buckets information
	Buckets []*BucketType `json:"buckets,omitempty" name:"buckets" location:"elements"`
	// Bucket count
	Count *int `json:"count,omitempty" name:"count" location:"elements"`
}
