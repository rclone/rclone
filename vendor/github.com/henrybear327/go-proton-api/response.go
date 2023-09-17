package proton

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

type Code int

const (
	SuccessCode               Code = 1000
	MultiCode                 Code = 1001
	InvalidValue              Code = 2001
	AFileOrFolderNameExist    Code = 2500
	ADraftExist               Code = 2500
	AppVersionMissingCode     Code = 5001
	AppVersionBadCode         Code = 5003
	UsernameInvalid           Code = 6003 // Deprecated, but still used.
	PasswordWrong             Code = 8002
	HumanVerificationRequired Code = 9001
	PaidPlanRequired          Code = 10004
	AuthRefreshTokenInvalid   Code = 10013
)

var (
	ErrFileNameExist   = errors.New("a file with that name already exists (Code=2500, Status=422)")
	ErrFolderNameExist = errors.New("a folder with that name already exists (Code=2500, Status=422)")
	ErrADraftExist     = errors.New("draft already exists on this revision (Code=2500, Status=409)")
)

// APIError represents an error returned by the API.
type APIError struct {
	// Status is the HTTP status code of the response that caused the error.
	Status int

	// Code is the error code returned by the API.
	Code Code

	// Message is the error message returned by the API.
	Message string `json:"Error"`

	// Details contains optional error details which are specific to each request.
	Details any
}

func (err APIError) Error() string {
	return fmt.Sprintf("%v (Code=%v, Status=%v)", err.Message, err.Code, err.Status)
}

func (err APIError) DetailsToString() string {
	if err.Details == nil {
		return ""
	}

	bytes, e := json.Marshal(err.Details)
	if e != nil {
		return fmt.Sprintf("Failed to generate json: %v", e)
	}

	return string(bytes)
}

// NetError represents a network error. It is returned when the API is unreachable.
type NetError struct {
	// Cause is the underlying error that caused the network error.
	Cause error

	// Message is an additional message that describes the network error.
	Message string
}

func newNetError(err error, message string) *NetError {
	return &NetError{Cause: err, Message: message}
}

func (err *NetError) Error() string {
	return fmt.Sprintf("%s: %v", err.Message, err.Cause)
}

func (err *NetError) Unwrap() error {
	return err.Cause
}

func (err *NetError) Is(target error) bool {
	_, ok := target.(*NetError)
	return ok
}

func catchAPIError(_ *resty.Client, res *resty.Response) error {
	if !res.IsError() {
		return nil
	}

	method := "NONE"
	route := "N/A"

	if res.Request != nil {
		method = res.Request.Method
		route = res.Request.URL
	}

	var err error

	if apiErr, ok := res.Error().(*APIError); ok {
		apiErr.Status = res.StatusCode()
		err = apiErr
	} else {
		statusCode := res.StatusCode()
		statusText := res.Status()

		// Catch error that may slip through when APIError deserialization routine fails for whichever reason.
		if statusCode >= 400 {
			err = &APIError{
				Status:  statusCode,
				Code:    0,
				Message: statusText,
			}
		} else {
			err = fmt.Errorf("%v", res.Status())
		}
	}

	return fmt.Errorf(
		"%v %s %s: %w",
		res.StatusCode(), method, route, err,
	)
}

func updateTime(_ *resty.Client, res *resty.Response) error {
	date, err := time.Parse(time.RFC1123, res.Header().Get("Date"))
	if err != nil {
		return err
	}

	crypto.UpdateTime(date.Unix())

	return nil
}

// nolint:gosec
func catchRetryAfter(_ *resty.Client, res *resty.Response) (time.Duration, error) {
	// 0 and no error means default behaviour which is exponential backoff with jitter.
	if res.StatusCode() != http.StatusTooManyRequests && res.StatusCode() != http.StatusServiceUnavailable {
		return 0, nil
	}

	// Parse the Retry-After header, or fallback to 10 seconds.
	after, err := strconv.Atoi(res.Header().Get("Retry-After"))
	if err != nil {
		after = 10
	}

	// Add some jitter to the delay.
	after += rand.Intn(10)

	logrus.WithFields(logrus.Fields{
		"pkg":    "go-proton-api",
		"status": res.StatusCode(),
		"url":    res.Request.URL,
		"method": res.Request.Method,
		"after":  after,
	}).Warn("Too many requests, retrying after delay")

	return time.Duration(after) * time.Second, nil
}

func catchTooManyRequests(res *resty.Response, _ error) bool {
	return res.StatusCode() == http.StatusTooManyRequests || res.StatusCode() == http.StatusServiceUnavailable
}

func catchDialError(res *resty.Response, err error) bool {
	return res.RawResponse == nil
}

func catchDropError(_ *resty.Response, err error) bool {
	if netErr := new(net.OpError); errors.As(err, &netErr) {
		return true
	}

	return false
}

// parseResponse should be used as post-processing of response when request is
// called with resty.SetDoNotParseResponse(off).
//
// In this case the resty is not processing request at all including http
// status check or APIerror parsing. Hence, the returned error would be nil
// even on non-200 reponsenses.
//
// This function also closes the response body.
func parseResponse(res *resty.Response, err error) (*resty.Response, error) {
	if err != nil || res.StatusCode() == 200 {
		return res, err
	}

	method := "NONE"
	route := "N/A"

	if res.Request != nil {
		method = res.Request.Method
		route = res.Request.URL
	}

	apiErr, ok := parseRawAPIError(res.RawBody())
	if !ok {
		apiErr = &APIError{
			Code:    0,
			Message: res.Status(),
		}
	}

	apiErr.Status = res.StatusCode()

	return res, fmt.Errorf(
		"%v %s %s: %w",
		res.StatusCode(), method, route, apiErr,
	)
}

func parseRawAPIError(rawResponse io.ReadCloser) (*APIError, bool) {
	apiErr := APIError{}
	defer rawResponse.Close()

	body, err := io.ReadAll(rawResponse)
	if err != nil {
		return &apiErr, false
	}

	if err := json.Unmarshal(body, &apiErr); err != nil {
		return &apiErr, false
	}

	return &apiErr, true
}
