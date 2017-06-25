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

// BucketPolicyFeatureContext provides feature context for bucket policy.
func BucketPolicyFeatureContext(s *godog.Suite) {
	s.Step(`^put bucket policy:$`, putBucketPolicy)
	s.Step(`^put bucket policy status code is (\d+)$`, putBucketPolicyStatusCodeIs)

	s.Step(`^get bucket policy$`, getBucketPolicy)
	s.Step(`^get bucket policy status code is (\d+)$`, getBucketPolicyStatusCodeIs)
	s.Step(`^get bucket policy should have Referer "([^"]*)"$`, getBucketPolicyShouldHaveReferer)

	s.Step(`^delete bucket policy$`, deleteBucketPolicy)
	s.Step(`^delete bucket policy status code is (\d+)$`, deleteBucketPolicyStatusCodeIs)
}

// --------------------------------------------------------------------------

var putBucketPolicyOutput *qs.PutBucketPolicyOutput

func putBucketPolicy(PolicyJSONText *gherkin.DocString) error {
	putBucketPolicyInput := &qs.PutBucketPolicyInput{}
	err = json.Unmarshal([]byte(PolicyJSONText.Content), putBucketPolicyInput)
	if err != nil {
		return err
	}

	if len(putBucketPolicyInput.Statement) == 1 {
		putBucketPolicyInput.Statement[0].Resource = qs.StringSlice([]string{tc.BucketName + "/*"})
	}

	putBucketPolicyOutput, err = bucket.PutPolicy(putBucketPolicyInput)
	return err
}

func putBucketPolicyStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(putBucketPolicyOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var getBucketPolicyOutput *qs.GetBucketPolicyOutput

func getBucketPolicy() error {
	getBucketPolicyOutput, err = bucket.GetPolicy()
	return err
}

func getBucketPolicyStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(getBucketPolicyOutput.StatusCode), statusCode)
}

func getBucketPolicyShouldHaveReferer(compare string) error {
	for _, statement := range getBucketPolicyOutput.Statement {
		if statement.Condition != nil &&
			statement.Condition.StringLike != nil {

			for _, referer := range statement.Condition.StringLike.Referer {
				if qs.StringValue(referer) == compare {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("Referer \"%s\" not found in bucket policy statement", compare)
}

// --------------------------------------------------------------------------

var deleteBucketPolicyOutput *qs.DeleteBucketPolicyOutput

func deleteBucketPolicy() error {
	deleteBucketPolicyOutput, err = bucket.DeletePolicy()
	return err
}

func deleteBucketPolicyStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteBucketPolicyOutput.StatusCode), statusCode)
}
