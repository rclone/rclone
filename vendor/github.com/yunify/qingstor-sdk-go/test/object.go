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
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/DATA-DOG/godog"

	"github.com/yunify/qingstor-sdk-go/request"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

// ObjectFeatureContext provides feature context for object.
func ObjectFeatureContext(s *godog.Suite) {
	s.Step(`^put object with key "(.{1,})"$`, putObjectWithKey)
	s.Step(`^put object status code is (\d+)$`, putObjectStatusCodeIs)

	s.Step(`^copy object with key "(.{1,})"$`, copyObjectWithKey)
	s.Step(`^copy object status code is (\d+)$`, copyObjectStatusCodeIs)

	s.Step(`^move object with key "(.{1,})"$`, moveObjectWithKey)
	s.Step(`^move object status code is (\d+)$`, moveObjectStatusCodeIs)

	s.Step(`^get object with key "(.{1,})"$`, getObjectWithKey)
	s.Step(`^get object status code is (\d+)$`, getObjectStatusCodeIs)
	s.Step(`^get object content length is (\d+)$`, getObjectContentLengthIs)

	s.Step(`^get object "(.{1,})" with content type "(.{1,})"$`, getObjectWithContentType)
	s.Step(`^get object content type is "(.{1,})"$`, getObjectContentTypeIs)

	s.Step(`^get object "(.{1,})" with query signature$`, getObjectWithQuerySignature)
	s.Step(`^get object with query signature content length is (\d+)$`, getObjectWithQuerySignatureContentLengthIs)

	s.Step(`^head object with key "(.{1,})"$`, headObjectWithKey)
	s.Step(`^head object status code is (\d+)$`, headObjectStatusCodeIs)

	s.Step(`^options object "(.{1,})" with method "([^"]*)" and origin "([^"]*)"$`, optionsObjectWithMethodAndOrigin)
	s.Step(`^options object status code is (\d+)$`, optionsObjectStatusCodeIs)

	s.Step(`^delete object with key "(.{1,})"$`, deleteObjectWithKey)
	s.Step(`^delete object status code is (\d+)$`, deleteObjectStatusCodeIs)
	s.Step(`^delete the move object with key "(.{1,})"$`, deleteTheMoveObjectWithKey)
	s.Step(`^delete the move object status code is (\d+)$`, deleteTheMoveObjectStatusCodeIs)
}

// --------------------------------------------------------------------------
var putObjectOutput *qs.PutObjectOutput

func putObjectWithKey(objectKey string) error {
	_, err = exec.Command("dd", "if=/dev/zero", "of=/tmp/sdk_bin", "bs=1024", "count=1").Output()
	if err != nil {
		return err
	}
	defer os.Remove("/tmp/sdk_bin")

	file, err := os.Open("/tmp/sdk_bin")
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	hashInBytes := hash.Sum(nil)[:16]
	md5String := hex.EncodeToString(hashInBytes)

	//file.Seek(0, io.SeekStart)
	file.Seek(0, 0)
	putObjectOutput, err = bucket.PutObject(objectKey, &qs.PutObjectInput{
		ContentType: qs.String("text/plain"),
		ContentMD5:  qs.String(md5String),
		Body:        file,
	})
	return err
}

func putObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(putObjectOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------
var copyObjectOutput *qs.PutObjectOutput

func copyObjectWithKey(objectKey string) error {
	copyObjectKey := fmt.Sprintf(`%s_copy`, objectKey)
	copyObjectOutput, err = bucket.PutObject(copyObjectKey, &qs.PutObjectInput{
		XQSCopySource: qs.String(fmt.Sprintf(`/%s/%s`, tc.BucketName, objectKey)),
	})
	return err
}

func copyObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(copyObjectOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------
var moveObjectOutput *qs.PutObjectOutput

func moveObjectWithKey(objectKey string) error {
	copyObjectKey := fmt.Sprintf(`%s_copy`, objectKey)
	moveObjectKey := fmt.Sprintf(`%s_move`, objectKey)
	moveObjectOutput, err = bucket.PutObject(moveObjectKey, &qs.PutObjectInput{
		XQSMoveSource: qs.String(fmt.Sprintf(`/%s/%s`, tc.BucketName, copyObjectKey)),
	})
	return err
}

func moveObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(moveObjectOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var getObjectOutput *qs.GetObjectOutput

func getObjectWithKey(objectKey string) error {
	getObjectOutput, err = bucket.GetObject(objectKey, nil)
	return err
}

func getObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(getObjectOutput.StatusCode), statusCode)
}

func getObjectContentLengthIs(length int) error {
	buffer := &bytes.Buffer{}
	buffer.ReadFrom(getObjectOutput.Body)
	getObjectOutput.Body.Close()
	return checkEqual(len(buffer.Bytes())*1024, length)
}

// --------------------------------------------------------------------------

var getObjectWithContentTypeRequest *request.Request

func getObjectWithContentType(objectKey, contentType string) error {
	getObjectWithContentTypeRequest, _, err = bucket.GetObjectRequest(
		objectKey,
		&qs.GetObjectInput{
			ResponseContentType: qs.String(contentType),
		},
	)
	if err != nil {
		return err
	}
	err = getObjectWithContentTypeRequest.Send()
	if err != nil {
		return err
	}
	return nil
}

func getObjectContentTypeIs(contentType string) error {
	return checkEqual(getObjectWithContentTypeRequest.HTTPResponse.Header.Get("Content-Type"), contentType)
}

// --------------------------------------------------------------------------

var getObjectWithQuerySignatureURL string

func getObjectWithQuerySignature(objectKey string) error {
	r, _, err := bucket.GetObjectRequest(objectKey, nil)
	if err != nil {
		return err
	}

	err = r.SignQuery(10)
	if err != nil {
		return err
	}

	getObjectWithQuerySignatureURL = r.HTTPRequest.URL.String()
	return nil
}

func getObjectWithQuerySignatureContentLengthIs(length int) error {
	out, err := http.Get(getObjectWithQuerySignatureURL)
	if err != nil {
		return err
	}
	buffer := &bytes.Buffer{}
	buffer.ReadFrom(out.Body)
	out.Body.Close()

	return checkEqual(len(buffer.Bytes())*1024, length)
}

// --------------------------------------------------------------------------

var headObjectOutput *qs.HeadObjectOutput

func headObjectWithKey(objectKey string) error {
	headObjectOutput, err = bucket.HeadObject(objectKey, nil)
	return err
}

func headObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(headObjectOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var optionsObjectOutput *qs.OptionsObjectOutput

func optionsObjectWithMethodAndOrigin(objectKey, method, origin string) error {
	optionsObjectOutput, err = bucket.OptionsObject(
		objectKey,
		&qs.OptionsObjectInput{
			AccessControlRequestMethod: qs.String(method),
			Origin: qs.String(origin),
		},
	)
	return err
}

func optionsObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(optionsObjectOutput.StatusCode), statusCode)
}

// --------------------------------------------------------------------------

var deleteObjectOutput *qs.DeleteObjectOutput
var deleteTheMoveObjectOutput *qs.DeleteObjectOutput

func deleteObjectWithKey(objectKey string) error {
	deleteObjectOutput, err = bucket.DeleteObject(objectKey)
	return err
}

func deleteObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteObjectOutput.StatusCode), statusCode)
}

func deleteTheMoveObjectWithKey(objectKey string) error {
	deleteTheMoveObjectOutput, err = bucket.DeleteObject(fmt.Sprintf(`%s_move`, objectKey))
	return err
}

func deleteTheMoveObjectStatusCodeIs(statusCode int) error {
	return checkEqual(qs.IntValue(deleteTheMoveObjectOutput.StatusCode), statusCode)
}
