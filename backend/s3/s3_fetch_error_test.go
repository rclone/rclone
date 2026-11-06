package s3

import (
	"context"
	"errors"
	"net/http"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fetchResponseError(status int, code, detail string) error {
	h := make(http.Header)
	h.Set("fastly-object-storage-df-error", detail)
	return &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status, Header: h}},
			Err:      &smithy.GenericAPIError{Code: code, Message: "fetch failed"},
		},
		RequestID: "request-1",
	}
}

func TestDirectFetchFallbackStrictClassification(t *testing.T) {
	for _, tt := range []struct {
		name      string
		operation string
		status    int
		code      string
		detail    string
		fallback  bool
	}{
		{"expired URL source status (live response)", "UploadPart", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 400", true},
		{"invalid signature source status (live response)", "UploadPart", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 403", true},
		{"put source status", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503", true},
		{"lower source status boundary", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 100", true},
		{"upper source status boundary", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 599", true},
		{"unverified InvalidRequest code", "PutObject", 400, "InvalidRequest", "DirectFetchSourceStatus 403", false},
		{"wrong code", "PutObject", 400, "AccessDenied", "DirectFetchSourceStatus 403", false},
		{"wrong response status", "PutObject", 403, "DirectFetchSourceStatus", "DirectFetchSourceStatus 403", false},
		{"missing diagnostic header", "PutObject", 400, "DirectFetchSourceStatus", "", false},
		{"unknown diagnostic", "PutObject", 400, "DirectFetchSourceStatus", "unknown detail", false},
		{"below source status boundary", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 099", false},
		{"above source status boundary", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 600", false},
		{"out of range source status", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 999", false},
		{"diagnostic suffix", "PutObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503 extra", false},
		{"complete operation", "CompleteMultipartUpload", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503", false},
		{"head operation", "HeadObject", 400, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503", false},
		{"signature error", "PutObject", 400, "SignatureDoesNotMatch", "DirectFetchSourceStatus 503", false},
		{"bucket error", "PutObject", 400, "NoSuchBucket", "DirectFetchSourceStatus 503", false},
		{"precondition error", "PutObject", 412, "PreconditionFailed", "DirectFetchSourceStatus 503", false},
		{"rate limited", "PutObject", 429, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503", false},
		{"server error", "PutObject", 503, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503", false},
		{"unknown code", "PutObject", 400, "UnexpectedFailure", "DirectFetchSourceStatus 503", false},
		{"missing source length", "PutObject", 400, "DirectFetchMissingContentLength", "DirectFetchMissingContentLength", true},
		{"missing source length part", "UploadPart", 400, "DirectFetchMissingContentLength", "DirectFetchMissingContentLength", true},
		{"missing source length wrong status", "PutObject", 403, "DirectFetchMissingContentLength", "DirectFetchMissingContentLength", false},
		{"missing source length wrong code", "PutObject", 400, "InvalidRequest", "DirectFetchMissingContentLength", false},
		{"missing source length without diagnostic", "PutObject", 400, "DirectFetchMissingContentLength", "", false},
		{"missing source length mismatched diagnostic", "PutObject", 400, "DirectFetchMissingContentLength", "DirectFetchSourceStatus 503", false},
		{"missing source length diagnostic suffix", "PutObject", 400, "DirectFetchMissingContentLength", "DirectFetchMissingContentLength extra", false},
		{"missing source length complete operation", "CompleteMultipartUpload", 400, "DirectFetchMissingContentLength", "DirectFetchMissingContentLength", false},
		{"source not found (live response)", "PutObject", 404, "NoSuchKey", "", true},
		{"source not found part (live response)", "UploadPart", 404, "NoSuchKey", "", true},
		{"source not found wrong status", "PutObject", 400, "NoSuchKey", "", false},
		{"not found other code", "PutObject", 404, "NoSuchBucket", "", false},
		{"not found upload", "UploadPart", 404, "NoSuchUpload", "", false},
		{"not found complete operation", "CompleteMultipartUpload", 404, "NoSuchKey", "", false},
		{"not found head operation", "HeadObject", 404, "NoSuchKey", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cause := fetchResponseError(tt.status, tt.code, tt.detail)
			err := wrapDirectFetchError(tt.operation, cause)
			got := directFetchFallback(err)
			assert.Equal(t, tt.fallback, errors.Is(got, fs.ErrorCantCopy))
			var apiErr smithy.APIError
			require.ErrorAs(t, got, &apiErr)
			assert.Equal(t, tt.code, apiErr.ErrorCode())
			assert.ErrorIs(t, got, cause)
		})
	}
}

func TestDirectFetchFallbackPreservesNilCancellationAndMalformedErrors(t *testing.T) {
	assert.NoError(t, wrapDirectFetchError("PutObject", nil))
	assert.NoError(t, directFetchFallback(nil))
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		err := directFetchFallback(wrapDirectFetchError("PutObject", cause))
		assert.ErrorIs(t, err, cause)
		assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	}

	h := make(http.Header)
	h.Set(directFetchErrorHeader, "DirectFetchSourceStatus 503")
	malformed := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusBadRequest, Header: h}},
		Err:      errors.New("malformed XML response"),
	}
	got := directFetchFallback(wrapDirectFetchError("PutObject", malformed))
	assert.NotErrorIs(t, got, fs.ErrorCantCopy)
	assert.ErrorIs(t, got, malformed)
	var apiErr smithy.APIError
	assert.False(t, errors.As(got, &apiErr), "a malformed response must not acquire an API code")

	missingXML := fetchResponseError(http.StatusBadRequest, "", "DirectFetchSourceStatus 503")
	got = directFetchFallback(wrapDirectFetchError("PutObject", missingXML))
	assert.NotErrorIs(t, got, fs.ErrorCantCopy)
}

func TestDirectFetchErrorExtractsTypedSDKDiagnostics(t *testing.T) {
	cause := fetchResponseError(http.StatusBadRequest, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503")
	err := wrapDirectFetchError("UploadPart part 7", cause)
	var directErr *directFetchError
	require.ErrorAs(t, err, &directErr)
	assert.Equal(t, "UploadPart part 7", directErr.operation)
	assert.Equal(t, http.StatusBadRequest, directErr.status)
	assert.Equal(t, "DirectFetchSourceStatus", directErr.code)
	assert.Equal(t, "DirectFetchSourceStatus 503", directErr.detail)
	assert.Equal(t, "request-1", directErr.requestID)
	assert.ErrorIs(t, err, cause)
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "DirectFetchSourceStatus", apiErr.ErrorCode())
}

func TestDirectFetchErrorOmitsServerText(t *testing.T) {
	sourceURL := "https://source.example/file?X-Amz-Signature=secret"
	for _, tt := range []struct {
		name   string
		detail string
		want   string
	}{
		{"source status", "DirectFetchSourceStatus 403", `direct fetch UploadPart (HTTP 400, code "DirectFetchSourceStatus", request "request-1"): DirectFetchSourceStatus 403`},
		{"code only", "DirectFetchConfiguration", `direct fetch UploadPart (HTTP 400, code "DirectFetchSourceStatus", request "request-1"): DirectFetchConfiguration`},
		{"no detail", "", `direct fetch UploadPart (HTTP 400, code "DirectFetchSourceStatus", request "request-1")`},
		{"reflected URL", "source " + sourceURL, `direct fetch UploadPart (HTTP 400, code "DirectFetchSourceStatus", request "request-1"): detail omitted`},
		{"control characters", "DirectFetchSourceStatus 403\n", `direct fetch UploadPart (HTTP 400, code "DirectFetchSourceStatus", request "request-1"): detail omitted`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cause := fetchResponseError(http.StatusBadRequest, "DirectFetchSourceStatus", tt.detail)
			var apiErr *smithy.GenericAPIError
			require.ErrorAs(t, cause, &apiErr)
			apiErr.Message = "message " + sourceURL

			err := wrapDirectFetchError("UploadPart", cause)
			assert.Equal(t, tt.want, err.Error())
			assert.ErrorIs(t, err, cause)
		})
	}
}

func TestDirectFetchErrorWithoutResponseShowsCause(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	err := wrapDirectFetchError("PutObject", cause)
	assert.Equal(t, "direct fetch PutObject: dial tcp: connection refused", err.Error())
	assert.ErrorIs(t, err, cause)
}

func TestDirectFetchErrorCallUsesOnePacerAttemptAndKeepsCancellation(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	f := newDirectFetchTestFs(ctx, t, "Fastly", http.NotFoundHandler())
	calls := 0
	err := f.directFetchCall(ctx, "PutObject", func() error {
		calls++
		return fetchResponseError(http.StatusServiceUnavailable, "SlowDown", "")
	})
	assert.Equal(t, 1, calls)
	assert.Error(t, err)
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "SlowDown", apiErr.ErrorCode())

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	calls = 0
	err = f.directFetchCall(cancelledCtx, "PutObject", func() error {
		calls++
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	assert.Zero(t, calls)

	cancelledCtx, cancel = context.WithCancel(ctx)
	calls = 0
	err = f.directFetchCall(cancelledCtx, "PutObject", func() error {
		calls++
		cancel()
		return fetchResponseError(http.StatusBadRequest, "InvalidRequest", "DirectFetchSourceStatus 503")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	assert.Equal(t, 1, calls)
}
