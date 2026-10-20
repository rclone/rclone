package s3

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fetchResponseError(status int, code, detail string) error {
	h := make(http.Header)
	h.Set("fastly-object-storage-df-error", detail)
	h.Set("x-amz-request-id", "request-1")
	return &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status, Header: h}},
		Err:      &smithy.GenericAPIError{Code: code, Message: "fetch failed"},
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			cause := fetchResponseError(tt.status, tt.code, tt.detail)
			err := wrapDirectFetchError(tt.operation, "https://source.example/?token=secret", cause)
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
	assert.NoError(t, wrapDirectFetchError("PutObject", "https://source.example/?token=secret", nil))
	assert.NoError(t, directFetchFallback(nil))
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		err := directFetchFallback(wrapDirectFetchError("PutObject", "https://source.example/?token=secret", cause))
		assert.ErrorIs(t, err, cause)
		assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	}

	h := make(http.Header)
	h.Set(directFetchErrorHeader, "DirectFetchSourceStatus 503")
	malformed := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusBadRequest, Header: h}},
		Err:      errors.New("malformed XML response"),
	}
	got := directFetchFallback(wrapDirectFetchError("PutObject", "https://source.example/?token=secret", malformed))
	assert.NotErrorIs(t, got, fs.ErrorCantCopy)
	assert.ErrorIs(t, got, malformed)
	var apiErr smithy.APIError
	assert.False(t, errors.As(got, &apiErr), "a malformed response must not acquire an API code")

	missingXML := fetchResponseError(http.StatusBadRequest, "", "DirectFetchSourceStatus 503")
	got = directFetchFallback(wrapDirectFetchError("PutObject", "https://source.example/?token=secret", missingXML))
	assert.NotErrorIs(t, got, fs.ErrorCantCopy)
}

func TestDirectFetchErrorExtractsTypedSDKDiagnostics(t *testing.T) {
	cause := fetchResponseError(http.StatusBadRequest, "DirectFetchSourceStatus", "DirectFetchSourceStatus 503")
	err := wrapDirectFetchError("UploadPart part 7", "https://source.example/?token=secret", cause)
	var directErr *directFetchError
	require.ErrorAs(t, err, &directErr)
	assert.Equal(t, "UploadPart part 7", directErr.operation)
	assert.Equal(t, http.StatusBadRequest, directErr.status)
	assert.Equal(t, "DirectFetchSourceStatus", directErr.code)
	assert.Equal(t, "fetch failed", directErr.message)
	assert.Equal(t, "DirectFetchSourceStatus 503", directErr.detail)
	assert.Equal(t, "request-1", directErr.requestID)
	assert.ErrorIs(t, err, cause)
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "DirectFetchSourceStatus", apiErr.ErrorCode())
}

func TestDirectFetchErrorRedactsURLsCredentialsControlsAndLength(t *testing.T) {
	sourceURL := "https://source-user:userinfo-secret@source.example/path-secret/object?X-Amz-Credential=credential-secret%2Fscope&X-Amz-Signature=signature-secret%2Bvalue"
	escapedQuery := url.QueryEscape(sourceURL)
	escapedPath := url.PathEscape(sourceURL)
	xmlEscaped := strings.ReplaceAll(sourceURL, "&", "&amp;")
	message := strings.Join([]string{
		"source " + sourceURL,
		"query " + escapedQuery,
		"path " + escapedPath,
		"xml " + xmlEscaped,
		"other https://other-user:other-secret@other.example/private?token=other-query-secret",
		"separate signature-secret+value signature-secret%2Bvalue credential-secret/scope credential-secret%2Fscope",
		"controls\n\t\x00",
		strings.Repeat("界", 1100),
	}, " | ")
	h := make(http.Header)
	h.Set(directFetchErrorHeader, "detail "+sourceURL+" signature-secret%2Bvalue")
	h.Set("x-amz-request-id", "request-1")
	cause := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusBadRequest, Header: h}},
		Err:      &smithy.GenericAPIError{Code: "InvalidRequest", Message: message},
	}

	err := wrapDirectFetchError("UploadPart part 7", sourceURL, cause)
	text := err.Error()
	for _, secret := range []string{
		sourceURL, escapedQuery, escapedPath, xmlEscaped,
		"userinfo-secret", "path-secret", "credential-secret", "signature-secret",
		"other-secret", "other-query-secret",
	} {
		assert.NotContains(t, text, secret)
	}
	assert.NotContains(t, text, "\n")
	assert.NotContains(t, text, "\t")
	assert.NotContains(t, text, "\x00")
	assert.Contains(t, text, "UploadPart part 7")
	assert.Contains(t, text, "HTTP 400")
	assert.Contains(t, text, "InvalidRequest")
	assert.Contains(t, text, "request-1")
	assert.Contains(t, text, "…")
	assert.True(t, utf8.ValidString(text))
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Same(t, cause.Err, apiErr)
}

func TestDirectFetchErrorRedactsWholeReflectedURLsBeforeCredentialComponents(t *testing.T) {
	sourceURL := "https://source.example/file?token=first&signature=second%2bsecret"
	xmlEscaped := strings.ReplaceAll(sourceURL, "&", "&amp;")
	fullyEncoded := url.QueryEscape(sourceURL)
	message := strings.Join([]string{sourceURL, xmlEscaped, fullyEncoded}, " | ")
	h := make(http.Header)
	h.Set(directFetchErrorHeader, message)
	h.Set("x-amz-request-id", "request-1")
	cause := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusBadRequest, Header: h}},
		Err:      &smithy.GenericAPIError{Code: "InvalidRequest", Message: message},
	}

	text := wrapDirectFetchError("UploadPart part 7", sourceURL, cause).Error()
	for _, secret := range []string{sourceURL, xmlEscaped, fullyEncoded, "first", "second%2bsecret", "second%252bsecret"} {
		assert.NotContains(t, text, secret)
	}
	assert.Contains(t, text, "UploadPart part 7")
	assert.Contains(t, text, "InvalidRequest")
	assert.Contains(t, text, "request-1")
}

func TestSafeDirectFetchTextRedactsPercentEncodedWholeURLCaseVariations(t *testing.T) {
	sourceURL := "https://source.example/file?token=first&signature=second%2bsecret"
	for _, reflection := range []struct {
		name string
		text string
	}{
		{
			name: "exact lower-case query-escaped reflection",
			text: "https%3a%2f%2fsource.example%2ffile%3ftoken%3dfirst%26signature%3dsecond%252bsecret",
		},
		{
			name: "standard path-escaped reflection",
			text: "https:%2F%2Fsource.example%2Ffile%3Ftoken=first&signature=second%252bsecret",
		},
		{
			name: "lower-case path-escaped reflection",
			text: "https:%2f%2fsource.example%2ffile%3ftoken=first&signature=second%252bsecret",
		},
		{
			name: "mixed-case path-escaped reflection",
			text: "https:%2F%2fsource.example%2Ffile%3ftoken=first&signature=second%252Bsecret",
		},
	} {
		t.Run(reflection.name, func(t *testing.T) {
			for _, mode := range []struct {
				name                       string
				redactCredentialComponents bool
			}{
				{name: "structural"},
				{name: "message-detail", redactCredentialComponents: true},
			} {
				t.Run(mode.name, func(t *testing.T) {
					got := safeDirectFetchText("prefix "+reflection.text+" suffix", sourceURL, mode.redactCredentialComponents)
					assert.Equal(t, "prefix [redacted URL] suffix", got)
					assert.NotContains(t, got, "second%252")
				})
			}
		})
	}
}

func TestDirectFetchErrorShortCredentialDoesNotCorruptStructuralDiagnostics(t *testing.T) {
	sourceURL := "https://source.example/file?token=a"
	h := make(http.Header)
	h.Set(directFetchErrorHeader, "source "+sourceURL+" token a")
	h.Set("x-amz-request-id", "request-a")
	cause := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusBadRequest, Header: h}},
		Err:      &smithy.GenericAPIError{Code: "InvalidRequest", Message: "source " + sourceURL + " token a"},
	}

	err := wrapDirectFetchError("UploadPart part 7", sourceURL, cause)
	text := err.Error()
	assert.Contains(t, text, `direct fetch UploadPart part 7 (HTTP 400, code "InvalidRequest", request "request-a")`)
	assert.NotContains(t, text, sourceURL)
	assert.ErrorIs(t, err, cause)
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Same(t, cause.Err, apiErr)
}

func TestDirectFetchErrorCallUsesOnePacerAttemptAndKeepsCancellation(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	f := newDirectFetchTestFs(ctx, t, "Fastly", http.NotFoundHandler())
	sourceURL := "https://source.example/object?X-Amz-Signature=pacer-secret"
	calls := 0
	err := f.directFetchCall(ctx, "PutObject", sourceURL, func() error {
		calls++
		return fetchResponseError(http.StatusServiceUnavailable, "SlowDown", sourceURL)
	})
	assert.Equal(t, 1, calls)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), sourceURL)
	assert.NotContains(t, err.Error(), "pacer-secret")
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "SlowDown", apiErr.ErrorCode())

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	calls = 0
	err = f.directFetchCall(cancelledCtx, "PutObject", sourceURL, func() error {
		calls++
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	assert.Zero(t, calls)

	cancelledCtx, cancel = context.WithCancel(ctx)
	calls = 0
	err = f.directFetchCall(cancelledCtx, "PutObject", sourceURL, func() error {
		calls++
		cancel()
		return fetchResponseError(http.StatusBadRequest, "InvalidRequest", "DirectFetchSourceStatus 503")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, fs.ErrorCantCopy)
	assert.Equal(t, 1, calls)
}
