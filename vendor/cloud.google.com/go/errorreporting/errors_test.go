// Copyright 2016 Google Inc. All Rights Reserved.
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

package errorreporting

import (
	"errors"
	"strings"
	"testing"

	"cloud.google.com/go/internal/testutil"

	gax "github.com/googleapis/gax-go"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	erpb "google.golang.org/genproto/googleapis/devtools/clouderrorreporting/v1beta1"
)

type fakeReportErrorsClient struct {
	req    *erpb.ReportErrorEventRequest
	fail   bool
	doneCh chan struct{}
}

func (c *fakeReportErrorsClient) ReportErrorEvent(ctx context.Context, req *erpb.ReportErrorEventRequest, _ ...gax.CallOption) (*erpb.ReportErrorEventResponse, error) {
	defer func() {
		close(c.doneCh)
	}()
	if c.fail {
		return nil, errors.New("request failed")
	}
	c.req = req
	return &erpb.ReportErrorEventResponse{}, nil
}

func (c *fakeReportErrorsClient) Close() error {
	return nil
}

func newFakeReportErrorsClient() *fakeReportErrorsClient {
	c := &fakeReportErrorsClient{}
	c.doneCh = make(chan struct{})
	return c
}

func newTestClient(c *fakeReportErrorsClient) *Client {
	newClient = func(ctx context.Context, opts ...option.ClientOption) (client, error) {
		return c, nil
	}
	t, err := NewClient(context.Background(), testutil.ProjID(), Config{
		ServiceName:    "myservice",
		ServiceVersion: "v1.0",
	})
	if err != nil {
		panic(err)
	}
	return t
}

func commonChecks(t *testing.T, req *erpb.ReportErrorEventRequest, fn string) {
	if req.Event.ServiceContext.Service != "myservice" {
		t.Errorf("error report didn't contain service name")
	}
	if req.Event.ServiceContext.Version != "v1.0" {
		t.Errorf("error report didn't contain version name")
	}
	if !strings.Contains(req.Event.Message, "error") {
		t.Errorf("error report didn't contain message")
	}
	if !strings.Contains(req.Event.Message, fn) {
		t.Errorf("error report didn't contain stack trace")
	}
}

func TestReport(t *testing.T) {
	fc := newFakeReportErrorsClient()
	c := newTestClient(fc)
	c.Report(Entry{Error: errors.New("error")})

	<-fc.doneCh
	r := fc.req
	if r == nil {
		t.Fatalf("got no error report, expected one")
	}
	commonChecks(t, r, "errorreporting.TestReport")
}
func TestReportSync(t *testing.T) {
	ctx := context.Background()
	fc := newFakeReportErrorsClient()
	c := newTestClient(fc)
	if err := c.ReportSync(ctx, Entry{Error: errors.New("error")}); err != nil {
		t.Fatalf("cannot upload errors: %v", err)
	}

	<-fc.doneCh
	r := fc.req
	if r == nil {
		t.Fatalf("got no error report, expected one")
	}
	commonChecks(t, r, "errorreporting.TestReport")
}
