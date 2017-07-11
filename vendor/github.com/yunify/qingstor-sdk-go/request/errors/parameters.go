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

package errors

import (
	"fmt"
	"strings"
)

// ParameterRequiredError indicates that the required parameter is missing.
type ParameterRequiredError struct {
	ParameterName string
	ParentName    string
}

// Error returns the description of ParameterRequiredError.
func (e ParameterRequiredError) Error() string {
	return fmt.Sprintf(`"%s" is required in "%s"`, e.ParameterName, e.ParentName)
}

// ParameterValueNotAllowedError indicates that the parameter value is not allowed.
type ParameterValueNotAllowedError struct {
	ParameterName  string
	ParameterValue string
	AllowedValues  []string
}

// Error returns the description of ParameterValueNotAllowedError.
func (e ParameterValueNotAllowedError) Error() string {
	allowedValues := []string{}
	for _, value := range e.AllowedValues {
		allowedValues = append(allowedValues, "\""+value+"\"")
	}
	return fmt.Sprintf(
		`"%s" value "%s" is not allowed, should be one of %s`,
		e.ParameterName,
		e.ParameterValue,
		strings.Join(allowedValues, ", "))
}
