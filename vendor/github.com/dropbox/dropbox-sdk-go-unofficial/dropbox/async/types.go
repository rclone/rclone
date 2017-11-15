// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package async : has no documentation (yet)
package async

import (
	"encoding/json"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
)

// LaunchResultBase : Result returned by methods that launch an asynchronous
// job. A method who may either launch an asynchronous job, or complete the
// request synchronously, can use this union by extending it, and adding a
// 'complete' field with the type of the synchronous response. See
// `LaunchEmptyResult` for an example.
type LaunchResultBase struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
}

// Valid tag values for LaunchResultBase
const (
	LaunchResultBaseAsyncJobId = "async_job_id"
)

// UnmarshalJSON deserializes into a LaunchResultBase instance
func (u *LaunchResultBase) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		err = json.Unmarshal(body, &u.AsyncJobId)

		if err != nil {
			return err
		}
	}
	return nil
}

// LaunchEmptyResult : Result returned by methods that may either launch an
// asynchronous job or complete synchronously. Upon synchronous completion of
// the job, no additional information is returned.
type LaunchEmptyResult struct {
	dropbox.Tagged
	// AsyncJobId : This response indicates that the processing is asynchronous.
	// The string is an id that can be used to obtain the status of the
	// asynchronous job.
	AsyncJobId string `json:"async_job_id,omitempty"`
}

// Valid tag values for LaunchEmptyResult
const (
	LaunchEmptyResultAsyncJobId = "async_job_id"
	LaunchEmptyResultComplete   = "complete"
)

// UnmarshalJSON deserializes into a LaunchEmptyResult instance
func (u *LaunchEmptyResult) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
	}
	var w wrap
	var err error
	if err = json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "async_job_id":
		err = json.Unmarshal(body, &u.AsyncJobId)

		if err != nil {
			return err
		}
	}
	return nil
}

// PollArg : Arguments for methods that poll the status of an asynchronous job.
type PollArg struct {
	// AsyncJobId : Id of the asynchronous job. This is the value of a response
	// returned from the method that launched the job.
	AsyncJobId string `json:"async_job_id"`
}

// NewPollArg returns a new PollArg instance
func NewPollArg(AsyncJobId string) *PollArg {
	s := new(PollArg)
	s.AsyncJobId = AsyncJobId
	return s
}

// PollResultBase : Result returned by methods that poll for the status of an
// asynchronous job. Unions that extend this union should add a 'complete' field
// with a type of the information returned upon job completion. See
// `PollEmptyResult` for an example.
type PollResultBase struct {
	dropbox.Tagged
}

// Valid tag values for PollResultBase
const (
	PollResultBaseInProgress = "in_progress"
)

// PollEmptyResult : Result returned by methods that poll for the status of an
// asynchronous job. Upon completion of the job, no additional information is
// returned.
type PollEmptyResult struct {
	dropbox.Tagged
}

// Valid tag values for PollEmptyResult
const (
	PollEmptyResultInProgress = "in_progress"
	PollEmptyResultComplete   = "complete"
)

// PollError : Error returned by methods for polling the status of asynchronous
// job.
type PollError struct {
	dropbox.Tagged
}

// Valid tag values for PollError
const (
	PollErrorInvalidAsyncJobId = "invalid_async_job_id"
	PollErrorInternalError     = "internal_error"
	PollErrorOther             = "other"
)
