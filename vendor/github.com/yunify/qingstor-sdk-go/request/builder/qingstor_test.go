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

package builder

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yunify/qingstor-sdk-go/config"
	"github.com/yunify/qingstor-sdk-go/request/data"
)

type ObjectSubServiceProperties struct {
	BucketName *string `json:"bucket-name" name:"bucket-name"`
	ObjectKey  *string `json:"object-key" name:"object-key"`
	Zone       *string `json:"zone" name:"zone"`
}
type GetObjectInput struct {
	IfMatch           *string    `json:"If-Match" name:"If-Match" location:"headers"`
	IfModifiedSince   *time.Time `json:"If-Modified-Since" name:"If-Modified-Since" format:"RFC 822" location:"headers"`
	IfNoneMatch       *string    `json:"If-None-Match" name:"If-None-Match" location:"headers"`
	IfUnmodifiedSince time.Time  `json:"If-Unmodified-Since" name:"If-Unmodified-Since" format:"RFC 822" location:"headers"`
	// Specified range of the Object
	Range *string `json:"Range" name:"Range" location:"headers"`
}

func (i *GetObjectInput) Validate() error {
	return nil
}

func TestQingStorBuilder_BuildHTTPRequest(t *testing.T) {
	conf, err := config.NewDefault()
	assert.Nil(t, err)
	conf.Host = "qingstor.dev"

	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.Nil(t, err)

	qsBuilder := &QingStorBuilder{}
	operation := &data.Operation{
		Config:      conf,
		APIName:     "GET Object",
		ServiceName: "QingStor",
		Properties: &ObjectSubServiceProperties{
			BucketName: String("test"),
			ObjectKey:  String("path/to/key.txt"),
			Zone:       String("beta"),
		},
		RequestMethod: "GET",
		RequestURI:    "/<bucket-name>/<object-key>",
		StatusCodes: []int{
			201,
		},
	}
	inputValue := reflect.ValueOf(&GetObjectInput{
		IfModifiedSince: Time(time.Date(2016, 9, 1, 15, 30, 0, 0, tz)),
		Range:           String("100-"),
	})
	httpRequest, err := qsBuilder.BuildHTTPRequest(operation, &inputValue)
	assert.Nil(t, err)
	assert.NotNil(t, httpRequest.Header.Get("Date"))
	assert.Equal(t, "0", httpRequest.Header.Get("Content-Length"))
	assert.Equal(t, "", httpRequest.Header.Get("If-Match"))
	assert.Equal(t, "Thu, 01 Sep 2016 07:30:00 GMT", httpRequest.Header.Get("If-Modified-Since"))
	assert.Equal(t, "100-", httpRequest.Header.Get("Range"))
	assert.Equal(t, "https://beta.qingstor.dev:443/test/path/to/key.txt", httpRequest.URL.String())
}
