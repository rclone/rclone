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

// BucketACLFeatureContext provides feature context for bucket ACL.
func BucketACLFeatureContext(s *godog.Suite) {
	s.Step(`^put bucket ACL:$`, putBucketACL)
	s.Step(`^put bucket ACL status code is (\d+)$`, putBucketACLStatusCodeIs)

	s.Step(`^get bucket ACL$`, getBucketACL)
	s.Step(`^get bucket ACL status code is (\d+)$`, getBucketACLStatusCodeIs)
	s.Step(`^get bucket ACL should have grantee name "([^"]*)"$`, getBucketACLShouldHaveGranteeName)
}

// --------------------------------------------------------------------------

var putBucketACLOutput *qs.PutBucketACLOutput

func putBucketACL(ACLJSONText *gherkin.DocString) error {
	putBucketACLInput := &qs.PutBucketACLInput{}
	err = json.Unmarshal([]byte(ACLJSONText.Content), putBucketACLInput)
	if err != nil {
		return err
	}

	putBucketACLOutput, err = bucket.PutACL(putBucketACLInput)
	return err
}

func putBucketACLStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(putBucketACLOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var getBucketACLOutput *qs.GetBucketACLOutput

func getBucketACL() error {
	getBucketACLOutput, err = bucket.GetACL()
	return err
}

func getBucketACLStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(getBucketACLOutput.StatusCode), statusCode)
}

func getBucketACLShouldHaveGranteeName(name string) error {
	for _, ACL := range getBucketACLOutput.ACL {
		if qs.StringValue(ACL.Grantee.Name) == name {
			return nil
		}
	}

	return fmt.Errorf("Grantee name \"%s\" not found in bucket ACLs", name)
}
