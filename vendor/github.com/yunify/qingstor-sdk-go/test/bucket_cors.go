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

package main

import (
	"encoding/json"
	"fmt"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/gherkin"

	qs "github.com/yunify/qingstor-sdk-go/service"
)

// BucketCORSFeatureContext provides feature context for bucket CORS.
func BucketCORSFeatureContext(s *godog.Suite) {
	s.Step(`^put bucket CORS:$`, putBucketCORS)
	s.Step(`^put bucket CORS status code is (\d+)$`, putBucketCORSStatusCodeIs)

	s.Step(`^get bucket CORS$`, getBucketCORS)
	s.Step(`^get bucket CORS status code is (\d+)$`, getBucketCORSStatusCodeIs)
	s.Step(`^get bucket CORS should have allowed origin "([^"]*)"$`, getBucketCORSShouldHaveAllowedOrigin)

	s.Step(`^delete bucket CORS`, deleteBucketCORS)
	s.Step(`^delete bucket CORS status code is (\d+)$`, deleteBucketCORSStatusCodeIs)
}

// --------------------------------------------------------------------------

var putBucketCORSOutput *qs.PutBucketCORSOutput

func putBucketCORS(CORSJSONText *gherkin.DocString) error {
	putBucketCORSInput := &qs.PutBucketCORSInput{}
	err = json.Unmarshal([]byte(CORSJSONText.Content), putBucketCORSInput)
	if err != nil {
		return err
	}

	putBucketCORSOutput, err = bucket.PutCORS(putBucketCORSInput)
	return err
}

func putBucketCORSStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(putBucketCORSOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var getBucketCORSOutput *qs.GetBucketCORSOutput

func getBucketCORS() error {
	getBucketCORSOutput, err = bucket.GetCORS()
	return err
}

func getBucketCORSStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(getBucketCORSOutput.StatusCode), statusCode)
}

func getBucketCORSShouldHaveAllowedOrigin(origin string) error {
	for _, CORSRule := range getBucketCORSOutput.CORSRules {
		if qs.StringValue(CORSRule.AllowedOrigin) == origin {
			return nil
		}
	}

	return fmt.Errorf("Allowed origin \"%s\" not found in bucket CORS rules", origin)
}

// --------------------------------------------------------------------------

var deleteBucketCORSOutput *qs.DeleteBucketCORSOutput

func deleteBucketCORS() error {
	deleteBucketCORSOutput, err = bucket.DeleteCORS()
	return err
}

func deleteBucketCORSStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteBucketCORSOutput.StatusCode), statusCode)
}
