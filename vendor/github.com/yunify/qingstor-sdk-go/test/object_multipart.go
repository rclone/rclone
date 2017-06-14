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
	"os"
	"os/exec"

	"github.com/DATA-DOG/godog"

	"fmt"
	"github.com/yunify/qingstor-sdk-go/request/errors"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

// ObjectMultipartFeatureContext provides feature context for object multipart.
func ObjectMultipartFeatureContext(s *godog.Suite) {
	s.Step(`^initiate multipart upload with key "(.{1,})"$`, initiateMultipartUploadWithKey)
	s.Step(`^initiate multipart upload status code is (\d+)$`, initiateMultipartUploadStatusCodeIs)

	s.Step(`^upload the first part with key "(.{1,})"$`, uploadTheFirstPartWithKey)
	s.Step(`^upload the first part status code is (\d+)$`, uploadTheFirstPartStatusCodeIs)
	s.Step(`^upload the second part with key "(.{1,})"$`, uploadTheSecondPartWithKey)
	s.Step(`^upload the second part status code is (\d+)$`, uploadTheSecondPartStatusCodeIs)
	s.Step(`^upload the third part with key "(.{1,})"$`, uploadTheThirdPartWithKey)
	s.Step(`^upload the third part status code is (\d+)$`, uploadTheThirdPartStatusCodeIs)

	s.Step(`^list multipart with key "(.{1,})"$`, listMultipartWithKey)
	s.Step(`^list multipart status code is (\d+)$`, listMultipartStatusCodeIs)
	s.Step(`^list multipart object parts count is (\d+)$`, listMultipartObjectPartsCountIs)

	s.Step(`^complete multipart upload with key "(.{1,})"$`, completeMultipartUploadWithKey)
	s.Step(`^complete multipart upload status code is (\d+)$`, completeMultipartUploadStatusCodeIs)

	s.Step(`^abort multipart upload with key "(.{1,})"$`, abortMultipartUploadWithKey)
	s.Step(`^abort multipart upload status code is (\d+)$`, abortMultipartUploadStatusCodeIs)

	s.Step(`^delete the multipart object with key "(.{1,})"$`, deleteTheMultipartObjectWithKey)
	s.Step(`^delete the multipart object status code is (\d+)$`, deleteTheMultipartObjectStatusCodeIs)
}

// --------------------------------------------------------------------------

var initiateMultipartUploadOutput *qs.InitiateMultipartUploadOutput

func initiateMultipartUploadWithKey(objectKey string) error {
	initiateMultipartUploadOutput, err = bucket.InitiateMultipartUpload(
		objectKey,
		&qs.InitiateMultipartUploadInput{
			ContentType: qs.String("text/plain"),
		},
	)
	return err
}

func initiateMultipartUploadStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(initiateMultipartUploadOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var uploadTheFirstPartOutput *qs.UploadMultipartOutput
var uploadTheSecondPartOutput *qs.UploadMultipartOutput
var uploadTheThirdPartOutput *qs.UploadMultipartOutput

func uploadTheFirstPartWithKey(objectKey string) error {
	_, err = exec.Command("dd", "if=/dev/zero", "of=/tmp/sdk_bin_part_0", "bs=1048576", "count=5").Output()
	if err != nil {
		return err
	}
	defer os.Remove("/tmp/sdk_bin_part_0")

	file, err := os.Open("/tmp/sdk_bin_part_0")
	if err != nil {
		return err
	}
	defer file.Close()

	uploadTheFirstPartOutput, err = bucket.UploadMultipart(
		objectKey,
		&qs.UploadMultipartInput{
			UploadID:   initiateMultipartUploadOutput.UploadID,
			PartNumber: qs.Int(0),
			Body:       file,
		},
	)
	return err
}

func uploadTheFirstPartStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(uploadTheFirstPartOutput.StatusCode), statusCode)
}

func uploadTheSecondPartWithKey(objectKey string) error {
	_, err = exec.Command("dd", "if=/dev/zero", "of=/tmp/sdk_bin_part_1", "bs=1048576", "count=4").Output()
	if err != nil {
		return err
	}
	defer os.Remove("/tmp/sdk_bin_part_1")

	file, err := os.Open("/tmp/sdk_bin_part_1")
	if err != nil {
		return err
	}
	defer file.Close()

	uploadTheSecondPartOutput, err = bucket.UploadMultipart(
		objectKey,
		&qs.UploadMultipartInput{
			UploadID:   initiateMultipartUploadOutput.UploadID,
			PartNumber: qs.Int(1),
			Body:       file,
		},
	)
	return err
}

func uploadTheSecondPartStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(uploadTheSecondPartOutput.StatusCode), statusCode)
}

func uploadTheThirdPartWithKey(objectKey string) error {
	_, err = exec.Command("dd", "if=/dev/zero", "of=/tmp/sdk_bin_part_2", "bs=1048576", "count=3").Output()
	if err != nil {
		return err
	}
	defer os.Remove("/tmp/sdk_bin_part_2")

	file, err := os.Open("/tmp/sdk_bin_part_2")
	if err != nil {
		return err
	}
	defer file.Close()

	uploadTheThirdPartOutput, err = bucket.UploadMultipart(
		objectKey,
		&qs.UploadMultipartInput{
			UploadID:   initiateMultipartUploadOutput.UploadID,
			PartNumber: qs.Int(2),
			Body:       file,
		},
	)
	return err
}

func uploadTheThirdPartStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(uploadTheThirdPartOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var listMultipartOutput *qs.ListMultipartOutput

func listMultipartWithKey(objectKey string) error {
	listMultipartOutput, err = bucket.ListMultipart(
		objectKey,
		&qs.ListMultipartInput{
			UploadID: initiateMultipartUploadOutput.UploadID,
		},
	)
	return err
}

func listMultipartStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(listMultipartOutput.StatusCode), statusCode)
}

func listMultipartObjectPartsCountIs(count int) error {
	return checkEqual(len(listMultipartOutput.ObjectParts), count)
}

// --------------------------------------------------------------------------

var completeMultipartUploadOutput *qs.CompleteMultipartUploadOutput

func completeMultipartUploadWithKey(objectKey string) error {
	completeMultipartUploadOutput, err = bucket.CompleteMultipartUpload(
		objectKey,
		&qs.CompleteMultipartUploadInput{
			UploadID:    initiateMultipartUploadOutput.UploadID,
			ETag:        qs.String(`"4072783b8efb99a9e5817067d68f61c6"`),
			ObjectParts: listMultipartOutput.ObjectParts,
		},
	)
	return err
}

func completeMultipartUploadStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(completeMultipartUploadOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

func abortMultipartUploadWithKey(objectKey string) error {
	_, err = bucket.AbortMultipartUpload(
		objectKey,
		&qs.AbortMultipartUploadInput{
			UploadID: initiateMultipartUploadOutput.UploadID,
		},
	)
	return nil
}

func abortMultipartUploadStatusCodeIs(statusCode int) error {
	switch e := err.(type) {
	case *errors.QingStorError:
		return checkEqual(e.StatusCode, statusCode)
	}

	return fmt.Errorf("abort multipart upload should get \"%d\"", statusCode)
}

// --------------------------------------------------------------------------

var deleteTheMultipartObjectOutput *qs.DeleteObjectOutput

func deleteTheMultipartObjectWithKey(objectKey string) error {
	deleteTheMultipartObjectOutput, err = bucket.DeleteObject(objectKey)
	return err
}

func deleteTheMultipartObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteTheMultipartObjectOutput.StatusCode), statusCode)
}
