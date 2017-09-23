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
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"time"

	"github.com/DATA-DOG/godog"

	"github.com/yunify/qingstor-sdk-go/config"
	qsErrors "github.com/yunify/qingstor-sdk-go/request/errors"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

func main() {
	setUp()

	context := func(s *godog.Suite) {
		ServiceFeatureContext(s)
		BucketFeatureContext(s)
		BucketACLFeatureContext(s)
		BucketCORSFeatureContext(s)
		BucketPolicyFeatureContext(s)
		BucketExternalMirrorFeatureContext(s)
		ObjectFeatureContext(s)
		ObjectMultipartFeatureContext(s)
		ImageFeatureContext(s)
	}
	options := godog.Options{
		Format: "pretty",
		Paths:  []string{"./features"},
		Tags:   "",
	}
	status := godog.RunWithOptions("*", context, options)

	//tearDown()

	os.Exit(status)
}

func setUp() {
	loadTestConfig()
	loadConfig()
	initQingStorService()

	err := initializeTheBucket()
	checkErrorForExit(err)

	err = theBucketIsInitialized()
	checkErrorForExit(err)

	//err = putBucket()
	//checkError(err)

	//err = putBucketStatusCodeIs(201)
	//checkError(err)
}

func tearDown() {
	err := deleteBucket()
	checkError(err)

	err = deleteBucketStatusCodeIs(204)
	checkError(err)

	retries := 0
	for retries < tc.MaxRetries {
		deleteBucketOutput, err = bucket.Delete()
		checkError(err)
		if err != nil {
			switch e := err.(type) {
			case *qsErrors.QingStorError:
				if e.Code == "bucket_not_exists" {
					return
				}
			}
		}
		retries++
		time.Sleep(time.Second * time.Duration(tc.RetryWaitTime))
	}
}

var err error
var tc *testConfig
var c *config.Config
var qsService *qs.Service

type testConfig struct {
	Zone       string `json:"zone" yaml:"zone"`
	BucketName string `json:"bucket_name" yaml:"bucket_name"`

	RetryWaitTime int `json:"retry_wait_time" yaml:"retry_wait_time"`
	MaxRetries    int `json:"max_retries" yaml:"max_retries"`
}

func loadTestConfig() {
	if tc == nil {
		configYAML, err := ioutil.ReadFile("./test_config.yaml")
		checkErrorForExit(err)

		tc = &testConfig{}
		err = yaml.Unmarshal(configYAML, tc)
		checkErrorForExit(err)
	}
}

func loadConfig() {
	if c == nil {
		c, err = config.NewDefault()
		checkErrorForExit(err)

		err = c.LoadConfigFromFilePath("./config.yaml")
		checkErrorForExit(err)
	}
}

func initQingStorService() {
	if qsService == nil {
		qsService, err = qs.Init(c)
		checkErrorForExit(err)
	}
}
