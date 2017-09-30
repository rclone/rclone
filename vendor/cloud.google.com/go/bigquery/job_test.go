// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bigquery

import (
	"testing"

	"cloud.google.com/go/internal/testutil"
	"golang.org/x/net/context"
	bq "google.golang.org/api/bigquery/v2"
)

func TestCreateJobRef(t *testing.T) {
	defer fixRandomJobID("RANDOM")()
	for _, test := range []struct {
		jobID          string
		addJobIDSuffix bool
		want           string
	}{
		{
			jobID:          "foo",
			addJobIDSuffix: false,
			want:           "foo",
		},
		{
			jobID:          "",
			addJobIDSuffix: false,
			want:           "RANDOM",
		},
		{
			jobID:          "",
			addJobIDSuffix: true, // irrelevant
			want:           "RANDOM",
		},
		{
			jobID:          "foo",
			addJobIDSuffix: true,
			want:           "foo-RANDOM",
		},
	} {
		jr := createJobRef(test.jobID, test.addJobIDSuffix, "projectID")
		got := jr.JobId
		if got != test.want {
			t.Errorf("%q, %t: got %q, want %q", test.jobID, test.addJobIDSuffix, got, test.want)
		}
	}
}

func fixRandomJobID(s string) func() {
	prev := randomJobIDFn
	randomJobIDFn = func() string { return s }
	return func() { randomJobIDFn = prev }
}

func checkJob(t *testing.T, i int, got, want *bq.Job) {
	if got.JobReference == nil {
		t.Errorf("#%d: empty job  reference", i)
		return
	}
	if got.JobReference.JobId == "" {
		t.Errorf("#%d: empty job ID", i)
		return
	}
	d := testutil.Diff(got, want)
	if d != "" {
		t.Errorf("#%d: (got=-, want=+) %s", i, d)
	}
}

type testService struct {
	*bq.Job

	service
}

func (s *testService) insertJob(ctx context.Context, projectID string, conf *insertJobConf) (*Job, error) {
	s.Job = conf.job
	return &Job{}, nil
}

func (s *testService) jobStatus(ctx context.Context, projectID, jobID string) (*JobStatus, error) {
	return &JobStatus{State: Done}, nil
}
