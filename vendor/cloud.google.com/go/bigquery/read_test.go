// Copyright 2015 Google Inc. All Rights Reserved.
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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"cloud.google.com/go/internal/testutil"

	"golang.org/x/net/context"
	bq "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/iterator"
)

type readTabledataArgs struct {
	conf *readTableConf
	tok  string
}

// readServiceStub services read requests by returning data from an in-memory list of values.
type readServiceStub struct {
	// values and pageTokens are used as sources of data to return in response to calls to readTabledata or readQuery.
	values     [][][]Value       // contains pages / rows / columns.
	pageTokens map[string]string // maps incoming page token to returned page token.

	// arguments are recorded for later inspection.
	readTabledataCalls []readTabledataArgs

	service
}

func (s *readServiceStub) readValues(tok string) *readDataResult {
	result := &readDataResult{
		pageToken: s.pageTokens[tok],
		rows:      s.values[0],
	}
	s.values = s.values[1:]

	return result
}

func (s *readServiceStub) waitForQuery(ctx context.Context, projectID, jobID string) (Schema, error) {
	return nil, nil
}

func (s *readServiceStub) readTabledata(ctx context.Context, conf *readTableConf, token string) (*readDataResult, error) {
	s.readTabledataCalls = append(s.readTabledataCalls, readTabledataArgs{conf, token})
	return s.readValues(token), nil
}

func TestRead(t *testing.T) {
	// The data for the service stub to return is populated for each test case in the testCases for loop.
	ctx := context.Background()
	service := &readServiceStub{}
	c := &Client{
		projectID: "project-id",
		service:   service,
	}

	queryJob := &Job{
		projectID: "project-id",
		jobID:     "job-id",
		c:         c,
		isQuery:   true,
		destinationTable: &bq.TableReference{
			ProjectId: "project-id",
			DatasetId: "dataset-id",
			TableId:   "table-id",
		},
	}

	for _, readFunc := range []func() *RowIterator{
		func() *RowIterator {
			return c.Dataset("dataset-id").Table("table-id").Read(ctx)
		},
		func() *RowIterator {
			it, err := queryJob.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return it
		},
	} {
		testCases := []struct {
			data       [][][]Value
			pageTokens map[string]string
			want       [][]Value
		}{
			{
				data:       [][][]Value{{{1, 2}, {11, 12}}, {{30, 40}, {31, 41}}},
				pageTokens: map[string]string{"": "a", "a": ""},
				want:       [][]Value{{1, 2}, {11, 12}, {30, 40}, {31, 41}},
			},
			{
				data:       [][][]Value{{{1, 2}, {11, 12}}, {{30, 40}, {31, 41}}},
				pageTokens: map[string]string{"": ""}, // no more pages after first one.
				want:       [][]Value{{1, 2}, {11, 12}},
			},
		}
		for _, tc := range testCases {
			service.values = tc.data
			service.pageTokens = tc.pageTokens
			if got, ok := collectValues(t, readFunc()); ok {
				if !testutil.Equal(got, tc.want) {
					t.Errorf("reading: got:\n%v\nwant:\n%v", got, tc.want)
				}
			}
		}
	}
}

func collectValues(t *testing.T, it *RowIterator) ([][]Value, bool) {
	var got [][]Value
	for {
		var vals []Value
		err := it.Next(&vals)
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Errorf("err calling Next: %v", err)
			return nil, false
		}
		got = append(got, vals)
	}
	return got, true
}

func TestNoMoreValues(t *testing.T) {
	c := &Client{
		projectID: "project-id",
		service: &readServiceStub{
			values: [][][]Value{{{1, 2}, {11, 12}}},
		},
	}
	it := c.Dataset("dataset-id").Table("table-id").Read(context.Background())
	var vals []Value
	// We expect to retrieve two values and then fail on the next attempt.
	if err := it.Next(&vals); err != nil {
		t.Fatalf("Next: got: %v: want: nil", err)
	}
	if err := it.Next(&vals); err != nil {
		t.Fatalf("Next: got: %v: want: nil", err)
	}
	if err := it.Next(&vals); err != iterator.Done {
		t.Fatalf("Next: got: %v: want: iterator.Done", err)
	}
}

type errorReadService struct {
	service
}

var errBang = errors.New("bang!")

func (s *errorReadService) readTabledata(ctx context.Context, conf *readTableConf, token string) (*readDataResult, error) {
	return nil, errBang
}

func TestReadError(t *testing.T) {
	// test that service read errors are propagated back to the caller.
	c := &Client{
		projectID: "project-id",
		service:   &errorReadService{},
	}
	it := c.Dataset("dataset-id").Table("table-id").Read(context.Background())
	var vals []Value
	if err := it.Next(&vals); err != errBang {
		t.Fatalf("Get: got: %v: want: %v", err, errBang)
	}
}

func TestReadTabledataOptions(t *testing.T) {
	// test that read options are propagated.
	s := &readServiceStub{
		values: [][][]Value{{{1, 2}}},
	}
	c := &Client{
		projectID: "project-id",
		service:   s,
	}
	it := c.Dataset("dataset-id").Table("table-id").Read(context.Background())
	it.PageInfo().MaxSize = 5
	var vals []Value
	if err := it.Next(&vals); err != nil {
		t.Fatal(err)
	}
	want := []readTabledataArgs{{
		conf: &readTableConf{
			projectID: "project-id",
			datasetID: "dataset-id",
			tableID:   "table-id",
			paging: pagingConf{
				recordsPerRequest:    5,
				setRecordsPerRequest: true,
			},
		},
		tok: "",
	}}

	if !testutil.Equal(s.readTabledataCalls, want, cmp.AllowUnexported(readTabledataArgs{}, readTableConf{}, pagingConf{})) {
		t.Errorf("reading: got:\n%v\nwant:\n%v", s.readTabledataCalls, want)
	}
}

func TestReadQueryOptions(t *testing.T) {
	// test that read options are propagated.
	s := &readServiceStub{
		values: [][][]Value{{{1, 2}}},
	}
	queryJob := &Job{
		projectID: "project-id",
		jobID:     "job-id",
		c:         &Client{service: s},
		isQuery:   true,
		destinationTable: &bq.TableReference{
			ProjectId: "project-id",
			DatasetId: "dataset-id",
			TableId:   "table-id",
		},
	}
	it, err := queryJob.Read(context.Background())
	if err != nil {
		t.Fatalf("err calling Read: %v", err)
	}
	it.PageInfo().MaxSize = 5
	var vals []Value
	if err := it.Next(&vals); err != nil {
		t.Fatalf("Next: got: %v: want: nil", err)
	}

	want := []readTabledataArgs{{
		conf: &readTableConf{
			projectID: "project-id",
			datasetID: "dataset-id",
			tableID:   "table-id",
			paging: pagingConf{
				recordsPerRequest:    5,
				setRecordsPerRequest: true,
			},
		},
		tok: "",
	}}

	if !testutil.Equal(s.readTabledataCalls, want, cmp.AllowUnexported(readTabledataArgs{}, readTableConf{}, pagingConf{})) {
		t.Errorf("reading: got:\n%v\nwant:\n%v", s.readTabledataCalls, want)
	}
}
