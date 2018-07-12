package azblob_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	chk "gopkg.in/check.v1"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/2017-07-29/azblob"
)

// For testing docs, see: https://labix.org/gocheck
// To test a specific test: go test -check.f MyTestSuite

type retryTestScenario int32

const (
	// Retry until success. Max reties hit. Operation time out prevents additional retries
	retryTestScenarioRetryUntilSuccess         retryTestScenario = 1
	retryTestScenarioRetryUntilOperationCancel retryTestScenario = 2
	retryTestScenarioRetryUntilMaxRetries      retryTestScenario = 3
)

func (s *aztestsSuite) TestRetryTestScenarioUntilSuccess(c *chk.C) {
	testRetryTestScenario(c, retryTestScenarioRetryUntilSuccess)
}

func (s *aztestsSuite) TestRetryTestScenarioUntilOperationCancel(c *chk.C) {
	testRetryTestScenario(c, retryTestScenarioRetryUntilOperationCancel)
}
func (s *aztestsSuite) TestRetryTestScenarioUntilMaxRetries(c *chk.C) {
	testRetryTestScenario(c, retryTestScenarioRetryUntilMaxRetries)
}
func newRetryTestPolicyFactory(c *chk.C, scenario retryTestScenario, maxRetries int32, cancel context.CancelFunc) *retryTestPolicyFactory {
	return &retryTestPolicyFactory{c: c, scenario: scenario, maxRetries: maxRetries, cancel: cancel}
}

type retryTestPolicyFactory struct {
	c          *chk.C
	scenario   retryTestScenario
	maxRetries int32
	cancel     context.CancelFunc
	try        int32
}

func (f *retryTestPolicyFactory) New(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.Policy {
	f.try = 0 // Reset this for each test
	return &retryTestPolicy{factory: f, next: next}
}

type retryTestPolicy struct {
	next    pipeline.Policy
	factory *retryTestPolicyFactory
}

type retryError struct {
	temporary, timeout bool
}

func (e *retryError) Temporary() bool { return e.temporary }
func (e *retryError) Timeout() bool   { return e.timeout }
func (e *retryError) Error() string {
	return fmt.Sprintf("Temporary=%t, Timeout=%t", e.Temporary(), e.Timeout())
}

type httpResponse struct {
	response *http.Response
}

func (r *httpResponse) Response() *http.Response { return r.response }

func (p *retryTestPolicy) Do(ctx context.Context, request pipeline.Request) (response pipeline.Response, err error) {
	c := p.factory.c
	p.factory.try++                                                   // Increment the try
	c.Assert(p.factory.try <= p.factory.maxRetries, chk.Equals, true) // Ensure # of tries < MaxRetries
	req := request.Request

	// Validate the expected pre-conditions for each try
	expectedHost := "PrimaryDC"
	if p.factory.try%2 == 0 {
		if p.factory.scenario != retryTestScenarioRetryUntilSuccess || p.factory.try <= 4 {
			expectedHost = "SecondaryDC"
		}
	}
	c.Assert(req.URL.Host, chk.Equals, expectedHost) // Ensure we got the expected primary/secondary DC

	// Ensure that any headers & query parameters this method adds (later) are removed/reset for each try
	c.Assert(req.Header.Get("TestHeader"), chk.Equals, "") // Ensure our "TestHeader" is not in the HTTP request
	values := req.URL.Query()
	c.Assert(len(values["TestQueryParam"]), chk.Equals, 0) // TestQueryParam shouldn't be in the HTTP request

	if seeker, ok := req.Body.(io.ReadSeeker); !ok {
		c.Fail() // Body must be an io.ReadSeeker
	} else {
		pos, err := seeker.Seek(0, io.SeekCurrent)
		c.Assert(err, chk.IsNil)            // Ensure that body was seekable
		c.Assert(pos, chk.Equals, int64(0)) // Ensure body seeked back to position 0
	}

	// Add a query param & header; these not be here on the next try
	values["TestQueryParam"] = []string{"TestQueryParamValue"}
	req.Header.Set("TestHeader", "TestValue") // Add a header this not exist with each try
	b := []byte{0}
	n, err := req.Body.Read(b)
	c.Assert(n, chk.Equals, 1) // Read failed

	switch p.factory.scenario {
	case retryTestScenarioRetryUntilSuccess:
		switch p.factory.try {
		case 1:
			if deadline, ok := ctx.Deadline(); ok {
				time.Sleep(time.Until(deadline) + time.Second) // Let the context timeout expire
			}
			err = ctx.Err()
		case 2:
			err = &retryError{temporary: true}
		case 3:
			err = &retryError{timeout: true}
		case 4:
			response = &httpResponse{response: &http.Response{StatusCode: http.StatusNotFound}}
		case 5:
			err = &retryError{temporary: true} // These attempts all fail but we're making sure we never see the secondary DC again
		case 6:
			response = &httpResponse{response: &http.Response{StatusCode: http.StatusOK}} // Stop retries with valid response
		default:
			c.Fail() // Retries should have stopped so we shouldn't get here
		}
	case retryTestScenarioRetryUntilOperationCancel:
		switch p.factory.try {
		case 1:
			p.factory.cancel()
			err = context.Canceled
		default:
			c.Fail() // Retries should have stopped so we shouldn't get here
		}
	case retryTestScenarioRetryUntilMaxRetries:
		err = &retryError{temporary: true} // Keep retrying until maxRetries is hit
	}
	return response, err // Return the response & err
}

func testRetryTestScenario(c *chk.C, scenario retryTestScenario) {
	u, _ := url.Parse("http://PrimaryDC")
	retryOptions := azblob.RetryOptions{
		Policy:                      azblob.RetryPolicyExponential,
		MaxTries:                    6,
		TryTimeout:                  2 * time.Second,
		RetryDelay:                  1 * time.Second,
		MaxRetryDelay:               4 * time.Second,
		RetryReadsFromSecondaryHost: "SecondaryDC",
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 64 /*2^MaxTries(6)*/ *retryOptions.TryTimeout)
	retrytestPolicyFactory := newRetryTestPolicyFactory(c, scenario, retryOptions.MaxTries, cancel)
	factories := [...]pipeline.Factory{
		azblob.NewRetryPolicyFactory(retryOptions),
		retrytestPolicyFactory,
	}
	p := pipeline.NewPipeline(factories[:], pipeline.Options{})
	request, err := pipeline.NewRequest(http.MethodGet, *u, strings.NewReader("TestData"))
	response, err := p.Do(ctx, nil, request)
	switch scenario {
	case retryTestScenarioRetryUntilSuccess:
		if err != nil || response == nil || response.Response() == nil || response.Response().StatusCode != http.StatusOK {
			c.Fail() // Operation didn't run to success
		}
	case retryTestScenarioRetryUntilMaxRetries:
		c.Assert(err, chk.NotNil)                                               // Ensure we ended with an error
		c.Assert(response, chk.IsNil)                                           // Ensure we ended without a valid response
		c.Assert(retrytestPolicyFactory.try, chk.Equals, retryOptions.MaxTries) // Ensure the operation ends with the exact right number of tries
	case retryTestScenarioRetryUntilOperationCancel:
		c.Assert(err, chk.Equals, context.Canceled)                                     // Ensure we ended due to cancellation
		c.Assert(response, chk.IsNil)                                                   // Ensure we ended without a valid response
		c.Assert(retrytestPolicyFactory.try <= retryOptions.MaxTries, chk.Equals, true) // Ensure we didn't end due to reaching max tries
	}
	cancel()
}

/*
   	Fail primary; retry should be on secondary URL - maybe do this twice
   	Fail secondary; and never see primary again

   	Make sure any mutations are lost on each retry
   	Make sure body is reset on each retry

   	Timeout a try; should retry (unless no more)
   	timeout an operation; should not retry
   	check timeout query param; should be try timeout

   	Return Temporary() = true; should retry (unless max)
   	Return Timeout() true; should retry (unless max)

   	Secondary try returns 404; no more tries against secondary

   	error where Temporary() and Timeout() return false; no retry
   	error where Temporary() & Timeout don't exist; no retry
    no error; no retry; return success, nil
*/
