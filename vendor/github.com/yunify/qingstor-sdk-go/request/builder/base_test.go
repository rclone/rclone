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
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yunify/qingstor-sdk-go/config"
	"github.com/yunify/qingstor-sdk-go/request/data"
)

type FakeProperties struct {
	A  *string `name:"a"`
	B  *string `name:"b"`
	CD *int    `name:"c-d"`
}
type FakeInput struct {
	ParamA    *string    `location:"query" name:"a"`
	ParamB    *string    `location:"query" name:"b"`
	ParamCD   *int       `location:"query" name:"c_d" default:"1024"`
	HeaderA   *string    `location:"headers" name:"A"`
	HeaderB   *time.Time `location:"headers" name:"B" format:"RFC 822"`
	HeaderCD  *int       `location:"headers" name:"C-D"`
	ElementA  *string    `location:"elements" name:"a"`
	ElementB  *string    `location:"elements" name:"b"`
	ElementCD *int64     `location:"elements" name:"cd"`
	Body      *string    `localtion:"body"`
}

func (i *FakeInput) Validate() error {
	return nil
}

func String(v string) *string {
	return &v
}

func Int(v int) *int {
	return &v
}

func Int64(v int64) *int64 {
	return &v
}

func Time(v time.Time) *time.Time {
	return &v
}

func TestBaseBuilder_BuildHTTPRequest(t *testing.T) {
	conf, err := config.NewDefault()
	assert.Nil(t, err)

	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.Nil(t, err)

	builder := BaseBuilder{}
	operation := &data.Operation{
		Config:      conf,
		APIName:     "This is API name",
		ServiceName: "Base",
		Properties: &FakeProperties{
			A:  String("property_a"),
			B:  String("property_b"),
			CD: Int(0),
		},
		RequestMethod: "GET",
		RequestURI:    "/hello/<a>/<c-d>/<b>/world",
		StatusCodes: []int{
			200,
			201,
		},
	}
	inputValue := reflect.ValueOf(&FakeInput{
		ParamA:    String("param_a"),
		ParamCD:   Int(1024),
		HeaderA:   String("header_a"),
		HeaderB:   Time(time.Date(2016, 9, 1, 15, 30, 0, 0, tz)),
		ElementA:  String("element_a"),
		ElementB:  String("element_b"),
		ElementCD: Int64(0),
		Body:      String("This is body string"),
	})
	httpRequest, err := builder.BuildHTTPRequest(operation, &inputValue)
	assert.Nil(t, err)
	assert.Equal(t, &map[string]string{
		"a":   "property_a",
		"b":   "property_b",
		"c-d": "0",
	}, builder.parsedProperties)
	assert.Equal(t, &map[string]string{
		"a":   "param_a",
		"c_d": "1024",
	}, builder.parsedQuery)
	assert.Equal(t, &map[string]string{
		"A":            "header_a",
		"B":            "Thu, 01 Sep 2016 07:30:00 GMT",
		"Content-Type": "application/json",
	}, builder.parsedHeaders)
	assert.NotNil(t, httpRequest.Header.Get("Date"))
	assert.Equal(t, "40", httpRequest.Header.Get("Content-Length"))

	buffer := &bytes.Buffer{}
	buffer.ReadFrom(httpRequest.Body)
	httpRequest.Body.Close()
	assert.Equal(t, "{\"a\":\"element_a\",\"b\":\"element_b\",\"cd\":0}", buffer.String())
}
