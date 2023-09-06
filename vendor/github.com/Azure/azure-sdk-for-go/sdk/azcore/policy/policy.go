//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package policy

import (
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/exported"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/tracing"
)

// Policy represents an extensibility point for the Pipeline that can mutate the specified
// Request and react to the received Response.
type Policy = exported.Policy

// Transporter represents an HTTP pipeline transport used to send HTTP requests and receive responses.
type Transporter = exported.Transporter

// Request is an abstraction over the creation of an HTTP request as it passes through the pipeline.
// Don't use this type directly, use runtime.NewRequest() instead.
type Request = exported.Request

// ClientOptions contains optional settings for a client's pipeline.
// All zero-value fields will be initialized with default values.
type ClientOptions struct {
	// APIVersion overrides the default version requested of the service. Set with caution as this package version has not been tested with arbitrary service versions.
	APIVersion string

	// Cloud specifies a cloud for the client. The default is Azure Public Cloud.
	Cloud cloud.Configuration

	// Logging configures the built-in logging policy.
	Logging LogOptions

	// Retry configures the built-in retry policy.
	Retry RetryOptions

	// Telemetry configures the built-in telemetry policy.
	Telemetry TelemetryOptions

	// TracingProvider configures the tracing provider.
	// It defaults to a no-op tracer.
	TracingProvider tracing.Provider

	// Transport sets the transport for HTTP requests.
	Transport Transporter

	// PerCallPolicies contains custom policies to inject into the pipeline.
	// Each policy is executed once per request.
	PerCallPolicies []Policy

	// PerRetryPolicies contains custom policies to inject into the pipeline.
	// Each policy is executed once per request, and for each retry of that request.
	PerRetryPolicies []Policy
}

// LogOptions configures the logging policy's behavior.
type LogOptions struct {
	// IncludeBody indicates if request and response bodies should be included in logging.
	// The default value is false.
	// NOTE: enabling this can lead to disclosure of sensitive information, use with care.
	IncludeBody bool

	// AllowedHeaders is the slice of headers to log with their values intact.
	// All headers not in the slice will have their values REDACTED.
	// Applies to request and response headers.
	AllowedHeaders []string

	// AllowedQueryParams is the slice of query parameters to log with their values intact.
	// All query parameters not in the slice will have their values REDACTED.
	AllowedQueryParams []string
}

// RetryOptions configures the retry policy's behavior.
// Zero-value fields will have their specified default values applied during use.
// This allows for modification of a subset of fields.
type RetryOptions struct {
	// MaxRetries specifies the maximum number of attempts a failed operation will be retried
	// before producing an error.
	// The default value is three.  A value less than zero means one try and no retries.
	MaxRetries int32

	// TryTimeout indicates the maximum time allowed for any single try of an HTTP request.
	// This is disabled by default.  Specify a value greater than zero to enable.
	// NOTE: Setting this to a small value might cause premature HTTP request time-outs.
	TryTimeout time.Duration

	// RetryDelay specifies the initial amount of delay to use before retrying an operation.
	// The value is used only if the HTTP response does not contain a Retry-After header.
	// The delay increases exponentially with each retry up to the maximum specified by MaxRetryDelay.
	// The default value is four seconds.  A value less than zero means no delay between retries.
	RetryDelay time.Duration

	// MaxRetryDelay specifies the maximum delay allowed before retrying an operation.
	// Typically the value is greater than or equal to the value specified in RetryDelay.
	// The default Value is 60 seconds.  A value less than zero means there is no cap.
	MaxRetryDelay time.Duration

	// StatusCodes specifies the HTTP status codes that indicate the operation should be retried.
	// A nil slice will use the following values.
	//   http.StatusRequestTimeout      408
	//   http.StatusTooManyRequests     429
	//   http.StatusInternalServerError 500
	//   http.StatusBadGateway          502
	//   http.StatusServiceUnavailable  503
	//   http.StatusGatewayTimeout      504
	// Specifying values will replace the default values.
	// Specifying an empty slice will disable retries for HTTP status codes.
	StatusCodes []int

	// ShouldRetry evaluates if the retry policy should retry the request.
	// When specified, the function overrides comparison against the list of
	// HTTP status codes and error checking within the retry policy. Context
	// and NonRetriable errors remain evaluated before calling ShouldRetry.
	// The *http.Response and error parameters are mutually exclusive, i.e.
	// if one is nil, the other is not nil.
	// A return value of true means the retry policy should retry.
	ShouldRetry func(*http.Response, error) bool
}

// TelemetryOptions configures the telemetry policy's behavior.
type TelemetryOptions struct {
	// ApplicationID is an application-specific identification string to add to the User-Agent.
	// It has a maximum length of 24 characters and must not contain any spaces.
	ApplicationID string

	// Disabled will prevent the addition of any telemetry data to the User-Agent.
	Disabled bool
}

// TokenRequestOptions contain specific parameter that may be used by credentials types when attempting to get a token.
type TokenRequestOptions = exported.TokenRequestOptions

// BearerTokenOptions configures the bearer token policy's behavior.
type BearerTokenOptions struct {
	// AuthorizationHandler allows SDK developers to run client-specific logic when BearerTokenPolicy must authorize a request.
	// When this field isn't set, the policy follows its default behavior of authorizing every request with a bearer token from
	// its given credential.
	AuthorizationHandler AuthorizationHandler
}

// AuthorizationHandler allows SDK developers to insert custom logic that runs when BearerTokenPolicy must authorize a request.
type AuthorizationHandler struct {
	// OnRequest is called each time the policy receives a request. Its func parameter authorizes the request with a token
	// from the policy's given credential. Implementations that need to perform I/O should use the Request's context,
	// available from Request.Raw().Context(). When OnRequest returns an error, the policy propagates that error and doesn't
	// send the request. When OnRequest is nil, the policy follows its default behavior, authorizing the request with a
	// token from its credential according to its configuration.
	OnRequest func(*Request, func(TokenRequestOptions) error) error

	// OnChallenge is called when the policy receives a 401 response, allowing the AuthorizationHandler to re-authorize the
	// request according to an authentication challenge (the Response's WWW-Authenticate header). OnChallenge is responsible
	// for parsing parameters from the challenge. Its func parameter will authorize the request with a token from the policy's
	// given credential. Implementations that need to perform I/O should use the Request's context, available from
	// Request.Raw().Context(). When OnChallenge returns nil, the policy will send the request again. When OnChallenge is nil,
	// the policy will return any 401 response to the client.
	OnChallenge func(*Request, *http.Response, func(TokenRequestOptions) error) error
}
