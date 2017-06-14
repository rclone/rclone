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

package unpacker

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yunify/qingstor-sdk-go/request/data"
)

func StringValue(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

func IntValue(v *int) int {
	if v != nil {
		return *v
	}
	return 0
}

func Int64Value(v *int64) int64 {
	if v != nil {
		return *v
	}
	return 0
}

func TimeValue(v *time.Time) time.Time {
	if v != nil {
		return *v
	}
	return time.Time{}
}

func TestBaseUnpacker_UnpackHTTPRequest(t *testing.T) {
	type FakeOutput struct {
		StatusCode *int

		A  *string `location:"elements" json:"a" name:"a"`
		B  *string `location:"elements" json:"b" name:"b"`
		CD *int    `location:"elements" json:"cd" name:"cd"`
		EF *int64  `location:"elements" json:"ef" name:"ef"`
	}

	httpResponse := &http.Response{Header: http.Header{}}
	httpResponse.StatusCode = 200
	httpResponse.Header.Set("Content-Type", "application/json")
	responseString := `{"a": "el_a", "b": "el_b", "cd": 1024, "ef": 2048}`
	httpResponse.Body = ioutil.NopCloser(bytes.NewReader([]byte(responseString)))

	output := &FakeOutput{}
	outputValue := reflect.ValueOf(output)
	unpacker := BaseUnpacker{}
	err := unpacker.UnpackHTTPRequest(&data.Operation{}, httpResponse, &outputValue)
	assert.Nil(t, err)
	assert.Equal(t, 200, IntValue(output.StatusCode))
	assert.Equal(t, "el_a", StringValue(output.A))
	assert.Equal(t, "el_b", StringValue(output.B))
	assert.Equal(t, 1024, IntValue(output.CD))
	assert.Equal(t, int64(2048), Int64Value(output.EF))
}
