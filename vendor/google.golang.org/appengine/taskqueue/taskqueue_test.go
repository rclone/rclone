// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package taskqueue

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/internal"
	"google.golang.org/appengine/internal/aetesting"
	pb "google.golang.org/appengine/internal/taskqueue"
)

func TestAddErrors(t *testing.T) {
	var tests = []struct {
		err, want error
		sameErr   bool // if true, should return err exactly
	}{
		{
			err: &internal.APIError{
				Service: "taskqueue",
				Code:    int32(pb.TaskQueueServiceError_TASK_ALREADY_EXISTS),
			},
			want: ErrTaskAlreadyAdded,
		},
		{
			err: &internal.APIError{
				Service: "taskqueue",
				Code:    int32(pb.TaskQueueServiceError_TOMBSTONED_TASK),
			},
			want: ErrTaskAlreadyAdded,
		},
		{
			err: &internal.APIError{
				Service: "taskqueue",
				Code:    int32(pb.TaskQueueServiceError_UNKNOWN_QUEUE),
			},
			want:    errors.New("not used"),
			sameErr: true,
		},
	}
	for _, tc := range tests {
		c := aetesting.FakeSingleContext(t, "taskqueue", "Add", func(req *pb.TaskQueueAddRequest, res *pb.TaskQueueAddResponse) error {
			// don't fill in any of the response
			return tc.err
		})
		task := &Task{Path: "/worker", Method: "PULL"}
		_, err := Add(c, task, "a-queue")
		want := tc.want
		if tc.sameErr {
			want = tc.err
		}
		if err != want {
			t.Errorf("Add with tc.err = %v, got %#v, want = %#v", tc.err, err, want)
		}
	}
}

func TestAddMulti(t *testing.T) {
	c := aetesting.FakeSingleContext(t, "taskqueue", "BulkAdd", func(req *pb.TaskQueueBulkAddRequest, res *pb.TaskQueueBulkAddResponse) error {
		res.Taskresult = []*pb.TaskQueueBulkAddResponse_TaskResult{
			{
				Result: pb.TaskQueueServiceError_OK.Enum(),
			},
			{
				Result: pb.TaskQueueServiceError_TASK_ALREADY_EXISTS.Enum(),
			},
			{
				Result: pb.TaskQueueServiceError_TOMBSTONED_TASK.Enum(),
			},
			{
				Result: pb.TaskQueueServiceError_INTERNAL_ERROR.Enum(),
			},
		}
		return nil
	})
	tasks := []*Task{
		{Path: "/worker", Method: "PULL"},
		{Path: "/worker", Method: "PULL"},
		{Path: "/worker", Method: "PULL"},
		{Path: "/worker", Method: "PULL"},
	}
	r, err := AddMulti(c, tasks, "a-queue")
	if len(r) != len(tasks) {
		t.Fatalf("AddMulti returned %d tasks, want %d", len(r), len(tasks))
	}
	want := appengine.MultiError{
		nil,
		ErrTaskAlreadyAdded,
		ErrTaskAlreadyAdded,
		&internal.APIError{
			Service: "taskqueue",
			Code:    int32(pb.TaskQueueServiceError_INTERNAL_ERROR),
		},
	}
	if !reflect.DeepEqual(err, want) {
		t.Errorf("AddMulti got %v, wanted %v", err, want)
	}
}

func TestAddWithEmptyPath(t *testing.T) {
	c := aetesting.FakeSingleContext(t, "taskqueue", "Add", func(req *pb.TaskQueueAddRequest, res *pb.TaskQueueAddResponse) error {
		if got, want := string(req.Url), "/_ah/queue/a-queue"; got != want {
			return fmt.Errorf("req.Url = %q; want %q", got, want)
		}
		return nil
	})
	if _, err := Add(c, &Task{}, "a-queue"); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

func TestParseRequestHeaders(t *testing.T) {
	tests := []struct {
		Header http.Header
		Want   RequestHeaders
	}{
		{
			Header: map[string][]string{
				"X-Appengine-Queuename":            []string{"foo"},
				"X-Appengine-Taskname":             []string{"bar"},
				"X-Appengine-Taskretrycount":       []string{"4294967297"}, // 2^32 + 1
				"X-Appengine-Taskexecutioncount":   []string{"4294967298"}, // 2^32 + 2
				"X-Appengine-Tasketa":              []string{"1500000000"},
				"X-Appengine-Taskpreviousresponse": []string{"404"},
				"X-Appengine-Taskretryreason":      []string{"baz"},
				"X-Appengine-Failfast":             []string{"yes"},
			},
			Want: RequestHeaders{
				QueueName:            "foo",
				TaskName:             "bar",
				TaskRetryCount:       4294967297,
				TaskExecutionCount:   4294967298,
				TaskETA:              time.Date(2017, time.July, 14, 2, 40, 0, 0, time.UTC),
				TaskPreviousResponse: 404,
				TaskRetryReason:      "baz",
				FailFast:             true,
			},
		},
		{
			Header: map[string][]string{},
			Want: RequestHeaders{
				QueueName:            "",
				TaskName:             "",
				TaskRetryCount:       0,
				TaskExecutionCount:   0,
				TaskETA:              time.Time{},
				TaskPreviousResponse: 0,
				TaskRetryReason:      "",
				FailFast:             false,
			},
		},
	}

	for idx, test := range tests {
		got := *ParseRequestHeaders(test.Header)
		if got.TaskETA.UnixNano() != test.Want.TaskETA.UnixNano() {
			t.Errorf("%d. ParseRequestHeaders got TaskETA %v, wanted %v", idx, got.TaskETA, test.Want.TaskETA)
		}
		got.TaskETA = time.Time{}
		test.Want.TaskETA = time.Time{}
		if !reflect.DeepEqual(got, test.Want) {
			t.Errorf("%d. ParseRequestHeaders got %v, wanted %v", idx, got, test.Want)
		}
	}
}
