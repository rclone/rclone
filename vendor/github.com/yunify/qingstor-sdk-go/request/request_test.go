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

package request

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yunify/qingstor-sdk-go/config"
	"github.com/yunify/qingstor-sdk-go/logger"
	"github.com/yunify/qingstor-sdk-go/request/data"
	"github.com/yunify/qingstor-sdk-go/request/errors"
)

type SomeActionProperties struct {
	A  *string `json:"a" name:"a"`
	B  *string `json:"b" name:"b"`
	CD *string `json:"c-d" name:"c-d"`
}

type SomeActionInput struct {
	Date            *time.Time `json:"Date" name:"Date" format:"RFC 822" location:"headers"`
	IfModifiedSince *time.Time `json:"If-Modified-Since" name:"If-Modified-Since" format:"RFC 822" location:"headers"`
	Range           *string    `json:"Range" name:"Range" location:"headers"`
	UploadID        *string    `json:"upload_id" name:"upload_id" location:"query"`
	Count           *int       `json:"count" name:"count" location:"elements"`
}

func (s *SomeActionInput) Validate() error {
	return nil
}

type SomeActionOutput struct {
	StatusCode *int `location:"statusCode"`
	Error      *errors.QingStorError
	RequestID  *string `location:"requestID"`
}

func String(v string) *string {
	return &v
}

func Int(v int) *int {
	return &v
}

func Time(v time.Time) *time.Time {
	return &v
}

func TestRequestSend(t *testing.T) {
	conf, err := config.New("ACCESS_KEY_ID", "SECRET_ACCESS_KEY")
	assert.Nil(t, err)
	logger.SetLevel("warn")

	operation := &data.Operation{
		Config: conf,
		Properties: &SomeActionProperties{
			A:  String("aaa"),
			B:  String("bbb"),
			CD: String("ccc-ddd"),
		},
		APIName:       "Some Action",
		RequestMethod: "GET",
		RequestURI:    "/<a>/<b>/<c-d>",
		StatusCodes: []int{
			200, // OK
			206, // Partial content
			304, // Not modified
			412, // Precondition failed
		},
	}

	output := &SomeActionOutput{}
	r, err := New(operation, &SomeActionInput{
		Date:            Time(time.Date(2016, 9, 1, 15, 30, 0, 0, time.UTC)),
		IfModifiedSince: Time(time.Date(2016, 9, 1, 15, 30, 0, 0, time.UTC)),
		Range:           String("100-"),
		UploadID:        String("0"),
		Count:           Int(23),
	}, output)
	assert.Nil(t, err)

	err = r.build()
	assert.Nil(t, err)

	err = r.sign()
	assert.Nil(t, err)

	assert.Equal(t, r.HTTPRequest.URL.String(), "https://qingstor.com:443/aaa/bbb/ccc-ddd?upload_id=0")
	assert.Equal(t, r.HTTPRequest.Header.Get("Range"), "100-")
	assert.Equal(t, r.HTTPRequest.Header.Get("If-Modified-Since"), "Thu, 01 Sep 2016 15:30:00 GMT")
	assert.Equal(t, r.HTTPRequest.Header.Get("Content-Length"), "12")
	assert.Equal(t, r.HTTPRequest.Header.Get("Authorization"), "QS ACCESS_KEY_ID:pA7G9qo4iQ6YHu7p4fX9Wcg4V9S6Mcgvz7p/0wEdz78=")

	httpResponse := &http.Response{Header: http.Header{}}
	httpResponse.StatusCode = 400
	httpResponse.Header.Set("Content-Type", "application/json")
	responseString := `{
	  "code": "bad_request",
	  "message": "Invalid argument(s) or invalid argument value(s)",
	  "request_id": "1e588695254aa08cf7a43f612e6ce14b",
	  "url": "http://docs.qingcloud.com/object_storage/api/object/get.html"
	}`
	httpResponse.Body = ioutil.NopCloser(bytes.NewReader([]byte(responseString)))
	assert.Nil(t, err)
	r.HTTPResponse = httpResponse

	err = r.unpack()
	assert.NotNil(t, err)

	switch e := err.(type) {
	case *errors.QingStorError:
		assert.Equal(t, "bad_request", e.Code)
		assert.Equal(t, "1e588695254aa08cf7a43f612e6ce14b", e.RequestID)
	}
}
