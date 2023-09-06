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

import "fmt"

// QingStorError stores information of an QingStor error response.
type QingStorError struct {
	StatusCode int

	Code         string `json:"code"`
	Message      string `json:"message"`
	RequestID    string `json:"request_id"`
	ReferenceURL string `json:"url"`
}

// Error returns the description of QingStor error response.
func (qse QingStorError) Error() string {
	return fmt.Sprintf(
		"QingStor Error: StatusCode \"%d\", Code \"%s\", Message \"%s\", Request ID \"%s\", Reference URL \"%s\"",
		qse.StatusCode, qse.Code, qse.Message, qse.RequestID, qse.ReferenceURL)
}
