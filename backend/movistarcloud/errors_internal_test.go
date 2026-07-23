// Internal tests for Movistar Cloud error handling
package movistarcloud

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/movistarcloud/api"
)

// htmlLoginPage is a trimmed-down version of the full HTML login page that the
// Movistar Cloud web endpoints return on authentication failures.
const htmlLoginPage = `<!--?xml version="1.0" encoding="UTF-8"?--><!DOCTYPE html><html lang="en"><head><title>Movistar Cloud</title></head><body></body></html>`

func newResp(status int, contentType string) *http.Response {
	header := http.Header{}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{StatusCode: status, Header: header}
}

func TestSummarizeErrorBody(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{
			name:        "html 401 by content type",
			status:      http.StatusUnauthorized,
			contentType: "text/html; charset=utf-8",
			body:        "<html><body>login</body></html>",
			want:        "authentication failed, the session has likely expired - run \"rclone config reconnect <remote>:\" to log in again",
		},
		{
			name:   "html 401 by doctype sniff",
			status: http.StatusUnauthorized,
			body:   htmlLoginPage,
			want:   "authentication failed, the session has likely expired - run \"rclone config reconnect <remote>:\" to log in again",
		},
		{
			name:        "html 403",
			status:      http.StatusForbidden,
			contentType: "text/html",
			body:        "<html></html>",
			want:        "authentication failed, the session has likely expired - run \"rclone config reconnect <remote>:\" to log in again",
		},
		{
			name:        "html other status",
			status:      http.StatusInternalServerError,
			contentType: "text/html",
			body:        "<html><body>oops</body></html>",
			want:        "server returned an HTML page instead of a JSON response",
		},
		{
			name:   "short plain text",
			status: http.StatusBadRequest,
			body:   "  something went wrong  ",
			want:   "something went wrong",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := summarizeErrorBody(newResp(test.status, test.contentType), []byte(test.body))
			if got != test.want {
				t.Errorf("summarizeErrorBody() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSummarizeErrorBodyTruncates(t *testing.T) {
	body := strings.Repeat("a", 1000)
	got := summarizeErrorBody(newResp(http.StatusBadRequest, ""), []byte(body))
	if len([]rune(got)) != 257 { // 256 chars + ellipsis
		t.Errorf("expected truncated body of 257 runes, got %d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "\u2026") {
		t.Errorf("expected truncated body to end with an ellipsis, got %q", got)
	}
}

func TestParseDataEnvelope(t *testing.T) {
	t.Run("error envelope surfaces server error", func(t *testing.T) {
		// HTTP 200 body that signals failure via an "error" object instead of "data".
		body := `{"error":{"code":"PAPI-0000","message":"Unknown exception","parameters":[],"cause":""},"responsetime":1783966571162}`
		_, err := parseDataEnvelope(strings.NewReader(body))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		apiErr, ok := err.(*api.Error)
		if !ok {
			t.Fatalf("expected *api.Error, got %T: %v", err, err)
		}
		if apiErr.Code != "PAPI-0000" || apiErr.Message != "Unknown exception" {
			t.Errorf("unexpected error contents: %+v", apiErr)
		}
		if want := `Error "PAPI-0000": Unknown exception`; err.Error() != want {
			t.Errorf("Error() = %q, want %q", err.Error(), want)
		}
	})

	t.Run("valid data is returned", func(t *testing.T) {
		body := `{"data":{"authorizationurl":"https://example.com/auth"},"responsetime":1}`
		data, err := parseDataEnvelope(strings.NewReader(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(data), "authorizationurl") {
			t.Errorf("unexpected data payload: %s", data)
		}
	})

	t.Run("empty response reports a clear error", func(t *testing.T) {
		body := `{"responsetime":1}`
		_, err := parseDataEnvelope(strings.NewReader(body))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "empty response") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestAPIErrorFormat(t *testing.T) {
	for _, test := range []struct {
		name string
		err  api.Error
		want string
	}{
		{
			name: "no status (200 envelope error)",
			err:  api.Error{Code: "PAPI-0000", Message: "Unknown exception"},
			want: `Error "PAPI-0000": Unknown exception`,
		},
		{
			name: "with http status",
			err:  api.Error{Code: "401 Unauthorized", Status: 401, Message: "denied"},
			want: `Error "401 Unauthorized" (401): denied`,
		},
		{
			name: "with cause",
			err:  api.Error{Code: "PAPI-0001", Message: "boom", Cause: "timeout"},
			want: `Error "PAPI-0001": boom (cause: timeout)`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Errorf("Error() = %q, want %q", got, test.want)
			}
		})
	}
}
