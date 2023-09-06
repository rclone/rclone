// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/sony/gobreaker"
)

// ServiceError models all potential errors generated the service call
type ServiceError interface {
	// The http status code of the error
	GetHTTPStatusCode() int

	// The human-readable error string as sent by the service
	GetMessage() string

	// A short error code that defines the error, meant for programmatic parsing.
	// See https://docs.cloud.oracle.com/Content/API/References/apierrors.htm
	GetCode() string

	// Unique Oracle-assigned identifier for the request.
	// If you need to contact Oracle about a particular request, please provide the request ID.
	GetOpcRequestID() string
}

// ServiceErrorRichInfo models all potential errors generated the service call and contains rich info for debugging purpose
type ServiceErrorRichInfo interface {
	ServiceError
	// The service this service call is sending to
	GetTargetService() string

	// The API name this service call is sending to
	GetOperationName() string

	// The timestamp when this request is made
	GetTimestamp() SDKTime

	// The endpoint and the Http method of this service call
	GetRequestTarget() string

	// The client version, in this case the oci go sdk version
	GetClientVersion() string

	// The API reference doc link for this API, optional and maybe empty
	GetOperationReferenceLink() string

	// Troubleshooting doc link
	GetErrorTroubleshootingLink() string
}

// ServiceErrorLocalizationMessage models all potential errors generated the service call and has localized error message info
type ServiceErrorLocalizationMessage interface {
	ServiceErrorRichInfo
	// The original error message string as sent by the service
	GetOriginalMessage() string

	// The values to be substituted into the originalMessageTemplate, expressed as a string-to-string map.
	GetMessageArgument() map[string]string

	// Template in ICU MessageFormat for the human-readable error string in English, but without the values replaced
	GetOriginalMessageTemplate() string
}

type servicefailure struct {
	StatusCode              int
	Code                    string            `json:"code,omitempty"`
	Message                 string            `json:"message,omitempty"`
	OriginalMessage         string            `json:"originalMessage"`
	OriginalMessageTemplate string            `json:"originalMessageTemplate"`
	MessageArgument         map[string]string `json:"messageArguments"`
	OpcRequestID            string            `json:"opc-request-id"`
	// debugging information
	TargetService string  `json:"target-service"`
	OperationName string  `json:"operation-name"`
	Timestamp     SDKTime `json:"timestamp"`
	RequestTarget string  `json:"request-target"`
	ClientVersion string  `json:"client-version"`

	// troubleshooting guidance
	OperationReferenceLink   string `json:"operation-reference-link"`
	ErrorTroubleshootingLink string `json:"error-troubleshooting-link"`
}

func newServiceFailureFromResponse(response *http.Response) error {
	var err error
	var timestamp SDKTime
	t, err := tryParsingTimeWithValidFormatsForHeaders([]byte(response.Header.Get("Date")), "Date")

	if err != nil {
		timestamp = *now()
	} else {
		timestamp = sdkTimeFromTime(t)
	}

	se := servicefailure{
		StatusCode:    response.StatusCode,
		Code:          "BadErrorResponse",
		OpcRequestID:  response.Header.Get("opc-request-id"),
		Timestamp:     timestamp,
		ClientVersion: defaultSDKMarker + "/" + Version(),
		RequestTarget: fmt.Sprintf("%s %s", response.Request.Method, response.Request.URL),
	}

	//If there is an error consume the body, entirely
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		se.Message = fmt.Sprintf("The body of the response was not readable, due to :%s", err.Error())
		return se
	}

	err = json.Unmarshal(body, &se)
	if err != nil {
		Debugf("Error response could not be parsed due to: %s", err.Error())
		se.Message = fmt.Sprintf("Failed to parse json from response body due to: %s. With response body %s.", err.Error(), string(body[:]))
		return se
	}
	return se
}

// PostProcessServiceError process the service error after an error is raised and complete it with extra information
func PostProcessServiceError(err error, service string, method string, apiReferenceLink string) error {
	var serviceFailure servicefailure
	if _, ok := err.(servicefailure); !ok {
		return err
	}
	serviceFailure = err.(servicefailure)
	serviceFailure.OperationName = method
	serviceFailure.TargetService = service
	serviceFailure.ErrorTroubleshootingLink = fmt.Sprintf("https://docs.oracle.com/iaas/Content/API/References/apierrors.htm#apierrors_%v__%v_%s", serviceFailure.StatusCode, serviceFailure.StatusCode, strings.ToLower(serviceFailure.Code))
	serviceFailure.OperationReferenceLink = apiReferenceLink
	return serviceFailure
}

func (se servicefailure) Error() string {
	return fmt.Sprintf(`Error returned by %s Service. Http Status Code: %d. Error Code: %s. Opc request id: %s. Message: %s
Operation Name: %s
Timestamp: %s
Client Version: %s
Request Endpoint: %s
Troubleshooting Tips: See %s for more information about resolving this error.%s
To get more info on the failing request, you can set OCI_GO_SDK_DEBUG env var to info or higher level to log the request/response details.
If you are unable to resolve this %s issue, please contact Oracle support and provide them this full error message.`,
		se.TargetService, se.StatusCode, se.Code, se.OpcRequestID, se.Message, se.OperationName, se.Timestamp, se.ClientVersion, se.RequestTarget, se.ErrorTroubleshootingLink, se.getOperationReferenceMessage(), se.TargetService)
}

func (se servicefailure) getOperationReferenceMessage() string {
	if se.OperationReferenceLink == "" {
		return ""
	}
	return fmt.Sprintf("\nAlso see %s for details on this operation's requirements.", se.OperationReferenceLink)
}

func (se servicefailure) GetHTTPStatusCode() int {
	return se.StatusCode

}

func (se servicefailure) GetMessage() string {
	return se.Message
}

func (se servicefailure) GetOriginalMessage() string {
	return se.OriginalMessage
}

func (se servicefailure) GetOriginalMessageTemplate() string {
	return se.OriginalMessageTemplate
}

func (se servicefailure) GetMessageArgument() map[string]string {
	return se.MessageArgument
}

func (se servicefailure) GetCode() string {
	return se.Code
}

func (se servicefailure) GetOpcRequestID() string {
	return se.OpcRequestID
}

func (se servicefailure) GetTargetService() string {
	return se.TargetService
}

func (se servicefailure) GetOperationName() string {
	return se.OperationName
}

func (se servicefailure) GetTimestamp() SDKTime {
	return se.Timestamp
}

func (se servicefailure) GetRequestTarget() string {
	return se.RequestTarget
}

func (se servicefailure) GetClientVersion() string {
	return se.ClientVersion
}

func (se servicefailure) GetOperationReferenceLink() string {
	return se.OperationReferenceLink
}

func (se servicefailure) GetErrorTroubleshootingLink() string {
	return se.ErrorTroubleshootingLink
}

// IsServiceError returns false if the error is not service side, otherwise true
// additionally it returns an interface representing the ServiceError
func IsServiceError(err error) (failure ServiceError, ok bool) {
	failure, ok = err.(ServiceError)
	return
}

// IsServiceErrorRichInfo returns false if the error is not service side or is not containing rich info, otherwise true
// additionally it returns an interface representing the ServiceErrorRichInfo
func IsServiceErrorRichInfo(err error) (failure ServiceErrorRichInfo, ok bool) {
	failure, ok = err.(ServiceErrorRichInfo)
	return
}

// IsServiceErrorLocalizationMessage returns false if the error is not service side, otherwise true
// additionally it returns an interface representing the ServiceErrorOriginalMessage
func IsServiceErrorLocalizationMessage(err error) (failure ServiceErrorLocalizationMessage, ok bool) {
	failure, ok = err.(ServiceErrorLocalizationMessage)
	return
}

type deadlineExceededByBackoffError struct{}

func (deadlineExceededByBackoffError) Error() string {
	return "now() + computed backoff duration exceeds request deadline"
}

// DeadlineExceededByBackoff is the error returned by Call() when GetNextDuration() returns a time.Duration that would
// force the user to wait past the request deadline before re-issuing a request. This enables us to exit early, since
// we cannot succeed based on the configured retry policy.
var DeadlineExceededByBackoff error = deadlineExceededByBackoffError{}

// NonSeekableRequestRetryFailure is the error returned when the request is with binary request body, and is configured
// retry, but the request body is not retryable
type NonSeekableRequestRetryFailure struct {
	err error
}

func (ne NonSeekableRequestRetryFailure) Error() string {
	if ne.err == nil {
		return fmt.Sprintf("Unable to perform Retry on this request body type, which did not implement seek() interface")
	}
	return fmt.Sprintf("%s. Unable to perform Retry on this request body type, which did not implement seek() interface", ne.err.Error())
}

// IsNetworkError validates if an error is a net.Error and check if it's temporary or timeout
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	if r, ok := err.(net.Error); ok && (r.Temporary() || r.Timeout()) || strings.Contains(err.Error(), "net/http: HTTP/1.x transport connection broken") {
		return true
	}
	return false
}

// IsCircuitBreakerError validates if an error's text is Open state ErrOpenState or HalfOpen state ErrTooManyRequests
func IsCircuitBreakerError(err error) bool {
	if err == nil {
		return false
	}

	if err.Error() == gobreaker.ErrOpenState.Error() || err.Error() == gobreaker.ErrTooManyRequests.Error() {
		return true
	}
	return false
}

func getCircuitBreakerError(request *http.Request, err error, cbr *OciCircuitBreaker) error {
	cbErr := fmt.Errorf("%s, so this request was not sent to the %s service.\n\n The circuit breaker was opened because the %s service failed too many times recently. "+
		"Because the circuit breaker has been opened, requests within a %.2f second window of when the circuit breaker opened will not be sent to the %s service.\n\n"+
		"URL which circuit breaker prevented request to - %s \n Circuit Breaker Info \n Name - %s \n State - %s \n\n Errors from %s service which opened the circuit breaker:\n\n%s \n",
		err, cbr.Cbst.serviceName, cbr.Cbst.serviceName, cbr.Cbst.openStateWindow.Seconds(), cbr.Cbst.serviceName, request.URL.Host+request.URL.Path, cbr.Cbst.name, cbr.Cb.State().String(), cbr.Cbst.serviceName, cbr.GetHistory())
	return cbErr
}

// StatErrCode is a type which wraps error's statusCode and errorCode from service end
type StatErrCode struct {
	statusCode int
	errorCode  string
}
