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
	"fmt"
	"os"
	"reflect"
)

func checkError(err error) {
	if err != nil {
		fmt.Println(err.Error())
	}
}

func checkErrorForExit(err error, code ...int) {
	if err != nil {
		exitCode := 1
		if len(code) > 0 {
			exitCode = code[0]
		}
		fmt.Println(err.Error(), exitCode)
		os.Exit(exitCode)
	}
}

func checkEqual(value, shouldBe interface{}) error {
	if value == nil || shouldBe == nil {
		if value == shouldBe {
			return nil
		}
	} else {
		if reflect.DeepEqual(value, shouldBe) {
			return nil
		}
	}

	return fmt.Errorf("Value \"%v\" should be \"%v\"", value, shouldBe)
}
