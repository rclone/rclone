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
	"sync"

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
var putObjectOutputs []*qs.PutObjectOutput

func putObjectWithKey(objectKey string) error {
	_, err = exec.Command("dd", "if=/dev/zero", "of=/tmp/sdk_bin", "bs=1024", "count=1").Output()
	if err != nil {
		return err
	}
	defer os.Remove("/tmp/sdk_bin")

	errChan := make(chan error, tc.Concurrency)
	putObjectOutputs = make([]*qs.PutObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			file, err := os.Open("/tmp/sdk_bin")
			if err != nil {
				errChan <- err
				return
			}
			defer file.Close()

			hash := md5.New()
			_, err = io.Copy(hash, file)
			if err != nil {
				errChan <- err
				return
			}
			hashInBytes := hash.Sum(nil)[:16]
			md5String := hex.EncodeToString(hashInBytes)

			//file.Seek(0, io.SeekStart)
			file.Seek(0, 0)
			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			putObjectOutput, err := bucket.PutObject(
				fmt.Sprintf("%s-%d", objectKey, index),
				&qs.PutObjectInput{
					ContentType: qs.String("text/plain"),
					ContentMD5:  qs.String(md5String),
					Body:        file,
				},
			)
			if err != nil {
				errChan <- err
				return
			}
			putObjectOutputs[index] = putObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func putObjectStatusCodeIs(statusCode int) error {
	for _, output := range putObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------
var copyObjectOutputs []*qs.PutObjectOutput

func copyObjectWithKey(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	copyObjectOutputs = make([]*qs.PutObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			copyObjectOutput, err := bucket.PutObject(
				fmt.Sprintf("%s-%d-copy", objectKey, index),
				&qs.PutObjectInput{
					XQSCopySource: qs.String(
						fmt.Sprintf("/%s/%s-%d", tc.BucketName, objectKey, index),
					),
				})
			if err != nil {
				errChan <- err
				return
			}
			copyObjectOutputs[index] = copyObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func copyObjectStatusCodeIs(statusCode int) error {
	for _, output := range copyObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------
var moveObjectOutputs []*qs.PutObjectOutput

func moveObjectWithKey(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	moveObjectOutputs = make([]*qs.PutObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			moveObjectOutput, err := bucket.PutObject(
				fmt.Sprintf("%s-%d-move", objectKey, index),
				&qs.PutObjectInput{
					XQSMoveSource: qs.String(
						fmt.Sprintf(`/%s/%s-%d-copy`, tc.BucketName, objectKey, index),
					),
				})
			if err != nil {
				errChan <- err
				return
			}
			moveObjectOutputs[index] = moveObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func moveObjectStatusCodeIs(statusCode int) error {
	for _, output := range moveObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------

var getObjectOutputs []*qs.GetObjectOutput

func getObjectWithKey(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	getObjectOutputs = make([]*qs.GetObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			getObjectOutput, err := bucket.GetObject(
				fmt.Sprintf("%s-%d", objectKey, index), nil,
			)
			if err != nil {
				errChan <- err
				return
			}
			getObjectOutputs[index] = getObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func getObjectStatusCodeIs(statusCode int) error {
	for _, output := range getObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

func getObjectContentLengthIs(length int) error {
	buffer := &bytes.Buffer{}
	for _, output := range getObjectOutputs {
		buffer.Truncate(0)
		buffer.ReadFrom(output.Body)
		err = checkEqual(len(buffer.Bytes())*1024, length)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------

var getObjectWithContentTypeRequests []*request.Request

func getObjectWithContentType(objectKey, contentType string) error {
	errChan := make(chan error, tc.Concurrency)
	getObjectWithContentTypeRequests = make([]*request.Request, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			getObjectWithContentTypeRequest, _, err := bucket.GetObjectRequest(
				fmt.Sprintf("%s-%d", objectKey, index),
				&qs.GetObjectInput{
					ResponseContentType: qs.String(contentType),
				},
			)
			if err != nil {
				errChan <- err
				return
			}
			err = getObjectWithContentTypeRequest.Send()
			if err != nil {
				errChan <- err
				return
			}
			err = getObjectWithContentTypeRequest.Send()
			if err != nil {
				errChan <- err
				return
			}
			getObjectWithContentTypeRequests[index] = getObjectWithContentTypeRequest
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func getObjectContentTypeIs(contentType string) error {
	for _, r := range getObjectWithContentTypeRequests {
		err = checkEqual(r.HTTPResponse.Header.Get("Content-Type"), contentType)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------

var getObjectWithQuerySignatureURLs []string

func getObjectWithQuerySignature(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	getObjectWithQuerySignatureURLs = make([]string, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			r, _, err := bucket.GetObjectRequest(
				fmt.Sprintf("%s-%d", objectKey, index), nil,
			)
			if err != nil {
				errChan <- err
				return
			}
			err = r.Build()
			if err != nil {
				errChan <- err
				return
			}
			err = r.SignQuery(10)
			if err != nil {
				errChan <- err
				return
			}

			getObjectWithQuerySignatureURLs[index] = r.HTTPRequest.URL.String()
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func getObjectWithQuerySignatureContentLengthIs(length int) error {
	buffer := &bytes.Buffer{}
	for _, url := range getObjectWithQuerySignatureURLs {
		out, err := http.Get(url)
		if err != nil {
			return err
		}
		buffer.Truncate(0)
		buffer.ReadFrom(out.Body)
		out.Body.Close()
		err = checkEqual(len(buffer.Bytes())*1024, length)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------

var headObjectOutputs []*qs.HeadObjectOutput

func headObjectWithKey(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	headObjectOutputs = make([]*qs.HeadObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			headObjectOutput, err := bucket.HeadObject(
				fmt.Sprintf("%s-%d", objectKey, index), nil,
			)
			if err != nil {
				errChan <- err
				return
			}
			headObjectOutputs[index] = headObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func headObjectStatusCodeIs(statusCode int) error {
	for _, output := range headObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------

var optionsObjectOutputs []*qs.OptionsObjectOutput

func optionsObjectWithMethodAndOrigin(objectKey, method, origin string) error {
	errChan := make(chan error, tc.Concurrency)
	optionsObjectOutputs = make([]*qs.OptionsObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			optionsObjectOutput, err := bucket.OptionsObject(
				fmt.Sprintf("%s-%d", objectKey, index),
				&qs.OptionsObjectInput{
					AccessControlRequestMethod: qs.String(method),
					Origin: qs.String(origin),
				},
			)
			if err != nil {
				errChan <- err
				return
			}
			optionsObjectOutputs[index] = optionsObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func optionsObjectStatusCodeIs(statusCode int) error {
	for _, output := range optionsObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------

var deleteObjectOutputs []*qs.DeleteObjectOutput
var deleteTheMoveObjectOutputs []*qs.DeleteObjectOutput

func deleteObjectWithKey(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	deleteObjectOutputs = make([]*qs.DeleteObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			deleteObjectOutput, err := bucket.DeleteObject(
				fmt.Sprintf("%s-%d", objectKey, index),
			)
			if err != nil {
				errChan <- err
				return
			}
			deleteObjectOutputs[index] = deleteObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteObjectStatusCodeIs(statusCode int) error {
	for _, output := range deleteObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteTheMoveObjectWithKey(objectKey string) error {
	errChan := make(chan error, tc.Concurrency)
	deleteTheMoveObjectOutputs = make([]*qs.DeleteObjectOutput, tc.Concurrency)

	wg := sync.WaitGroup{}
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go func(index int, errChan chan<- error) {
			wg.Done()

			if len(objectKey) > 1000 {
				objectKey = objectKey[:1000]
			}
			deleteTheMoveObjectOutput, err := bucket.DeleteObject(
				fmt.Sprintf("%s-%d-move", objectKey, index),
			)
			if err != nil {
				errChan <- err
				return
			}
			deleteTheMoveObjectOutputs[index] = deleteTheMoveObjectOutput
			errChan <- nil
			return
		}(i, errChan)
	}
	wg.Wait()

	for i := 0; i < tc.Concurrency; i++ {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteTheMoveObjectStatusCodeIs(statusCode int) error {
	for _, output := range deleteTheMoveObjectOutputs {
		err = checkEqual(qs.IntValue(output.StatusCode), statusCode)
		if err != nil {
			return err
		}
	}
	return nil
}
