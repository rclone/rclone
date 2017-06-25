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
	"errors"
	"fmt"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/gherkin"

	qsErrors "github.com/yunify/qingstor-sdk-go/request/errors"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

// BucketFeatureContext provides feature context for bucket.
func BucketFeatureContext(s *godog.Suite) {
	s.Step(`^initialize the bucket$`, initializeTheBucket)
	s.Step(`^the bucket is initialized$`, theBucketIsInitialized)

	s.Step(`^put bucket$`, putBucketFake)
	s.Step(`^put bucket status code is (\d+)$`, putBucketStatusCodeIsFake)
	s.Step(`^put same bucket again$`, putSameBucketAgain)
	s.Step(`^put same bucket again status code is (\d+)$$`, putSameBucketAgainStatusCodeIs)

	s.Step(`^list objects$`, listObjects)
	s.Step(`^list objects status code is (\d+)$`, listObjectsStatusCodeIs)
	s.Step(`^list objects keys count is (\d+)$`, listObjectsKeysCountIs)

	s.Step(`^head bucket$`, headBucket)
	s.Step(`^head bucket status code is (\d+)$`, headBucketStatusCodeIs)

	s.Step(`^delete bucket$`, deleteBucketFake)
	s.Step(`^delete bucket status code is (\d+)$`, deleteBucketStatusCodeIsFake)

	s.Step(`^delete multiple objects:$`, deleteMultipleObjects)
	s.Step(`^delete multiple objects code is (\d+)$`, deleteMultipleObjectsCodeIs)

	s.Step(`^get bucket statistics$`, getBucketStatistics)
	s.Step(`^get bucket statistics status code is (\d+)$`, getBucketStatisticsStatusCodeIs)
	s.Step(`^get bucket statistics status is "([^"]*)"$`, getBucketStatisticsStatusIs)

	s.Step(`^an object created by initiate multipart upload$`, anObjectCreatedByInitiateMultipartUpload)
	s.Step(`^list multipart uploads$`, listMultipartUploads)
	s.Step(`^list multipart uploads count is (\d+)$`, listMultipartUploadsCountIs)
	s.Step(`^list multipart uploads with prefix$`, listMultipartUploadsWithPrefix)
	s.Step(`^list multipart uploads with prefix count is (\d+)$`, listMultipartUploadsWithPrefixCountIs)
}

// --------------------------------------------------------------------------

var bucket *qs.Bucket

func initializeTheBucket() error {
	bucket, err = qsService.Bucket(tc.BucketName, tc.Zone)
	return err
}

func theBucketIsInitialized() error {
	if bucket == nil {
		return errors.New("Bucket is not initialized")
	}
	return nil
}

// --------------------------------------------------------------------------

var putBucketOutput *qs.PutBucketOutput

func putBucket() error {
	putBucketOutput, err = bucket.Put()
	return err
}

func putBucketFake() error {
	return nil
}

func putBucketStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(putBucketOutput.StatusCode), statusCode)
}

func putBucketStatusCodeIsFake(_ int) error {
	return nil
}

// --------------------------------------------------------------------------

func putSameBucketAgain() error {
	_, err = bucket.Put()
	return nil
}

func putSameBucketAgainStatusCodeIs(statusCode int) error {
	switch e := err.(type) {
	case *qsErrors.QingStorError:
		return checkEqual(e.StatusCode, statusCode)
	}

	return fmt.Errorf("put same bucket again should get \"%d\"", statusCode)
}

// --------------------------------------------------------------------------

var listObjectsOutput *qs.ListObjectsOutput

func listObjects() error {
	listObjectsOutput, err = bucket.ListObjects(&qs.ListObjectsInput{
		Delimiter: qs.String("/"),
		Limit:     qs.Int(1000),
		Prefix:    qs.String("Test/"),
		Marker:    qs.String("Next"),
	})
	return err
}

func listObjectsStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(listObjectsOutput.StatusCode), statusCode)
}

func listObjectsKeysCountIs(count int) error {
	return checkEqual(len(listObjectsOutput.Keys), count)
}

// --------------------------------------------------------------------------

var headBucketOutput *qs.HeadBucketOutput

func headBucket() error {
	headBucketOutput, err = bucket.Head()
	return err
}

func headBucketStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(headBucketOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var deleteBucketOutput *qs.DeleteBucketOutput

func deleteBucket() error {
	deleteBucketOutput, err = bucket.Delete()
	return err
}

func deleteBucketFake() error {
	return nil
}

func deleteBucketStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteBucketOutput.StatusCode), statusCode)
}

func deleteBucketStatusCodeIsFake(_ int) error {
	return nil
}

// --------------------------------------------------------------------------

var deleteMultipleObjectsOutput *qs.DeleteMultipleObjectsOutput

func deleteMultipleObjects(requestJSON *gherkin.DocString) error {
	_, err := bucket.PutObject("object_0", nil)
	if err != nil {
		return err
	}
	_, err = bucket.PutObject("object_1", nil)
	if err != nil {
		return err
	}
	_, err = bucket.PutObject("object_2", nil)
	if err != nil {
		return err
	}

	deleteMultipleObjectsInput := &qs.DeleteMultipleObjectsInput{}
	err = json.Unmarshal([]byte(requestJSON.Content), deleteMultipleObjectsInput)
	if err != nil {
		return err
	}

	deleteMultipleObjectsOutput, err = bucket.DeleteMultipleObjects(
		&qs.DeleteMultipleObjectsInput{
			Objects: deleteMultipleObjectsInput.Objects,
			Quiet:   deleteMultipleObjectsInput.Quiet,
		},
	)
	return err
}

func deleteMultipleObjectsCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteMultipleObjectsOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var getBucketStatisticsOutput *qs.GetBucketStatisticsOutput

func getBucketStatistics() error {
	getBucketStatisticsOutput, err = bucket.GetStatistics()
	return err
}

func getBucketStatisticsStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(getBucketStatisticsOutput.StatusCode), statusCode)
}

func getBucketStatisticsStatusIs(status string) error {
	return checkEqual(qs.StringValue(getBucketStatisticsOutput.Status), status)
}

// --------------------------------------------------------------------------
var listMultipartUploadsOutputObjectKey = "list_multipart_uploads_object_key"
var listMultipartUploadsInitiateOutput *qs.InitiateMultipartUploadOutput
var listMultipartUploadsOutput *qs.ListMultipartUploadsOutput

func anObjectCreatedByInitiateMultipartUpload() error {
	listMultipartUploadsInitiateOutput, err = bucket.InitiateMultipartUpload(
		listMultipartUploadsOutputObjectKey, nil,
	)
	return err
}

func listMultipartUploads() error {
	listMultipartUploadsOutput, err = bucket.ListMultipartUploads(nil)
	return err
}

func listMultipartUploadsCountIs(count int) error {
	return checkEqual(len(listMultipartUploadsOutput.Uploads), count)
}

func listMultipartUploadsWithPrefix() error {
	listMultipartUploadsOutput, err = bucket.ListMultipartUploads(
		&qs.ListMultipartUploadsInput{
			Prefix: qs.String(listMultipartUploadsOutputObjectKey),
		},
	)
	return err
}

func listMultipartUploadsWithPrefixCountIs(count int) error {
	_, err = bucket.AbortMultipartUpload(
		listMultipartUploadsOutputObjectKey, &qs.AbortMultipartUploadInput{
			UploadID: listMultipartUploadsInitiateOutput.UploadID,
		},
	)
	if err != nil {
		return err
	}

	return checkEqual(len(listMultipartUploadsOutput.Uploads), count)
}
