package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHeadersMergesCookies(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{{Name: "existing", Value: "old"}}

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "existing", Value: "new"}).String())
	resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "fresh", Value: "value"}).String())
	resp.Header.Set("X-Apple-Session-Token", "session-token")

	s.extractHeaders(resp)

	require.Len(t, s.Cookies, 2)
	assert.Equal(t, "new", s.Cookies[0].Value)
	assert.Equal(t, "fresh", s.Cookies[1].Name)
	assert.Equal(t, "session-token", s.SessionToken)
}

func TestExtractHeadersDeletesEmptyCookies(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{{Name: "X-APPLE-WEBAUTH-HSA-LOGIN", Value: "stale"}, {Name: "keep", Value: "value"}}

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "X-APPLE-WEBAUTH-HSA-LOGIN", Value: ""}).String())

	s.extractHeaders(resp)

	require.Len(t, s.Cookies, 1)
	assert.Equal(t, "keep", s.Cookies[0].Name)
	assert.Equal(t, "keep=value", s.GetCookieString())
}

// TestVerifyAccepted covers the acceptance logic for the securitycode
// "enter" endpoints: a 409 must be treated as success when either the
// X-Apple-Session-Token header is present, or the JSON body reports
// "securityCode":{"valid":true}. Anything else on a 409, and any
// non-2xx/409 status, is a rejection.
func TestVerifyAccepted(t *testing.T) {
	newResp := func(status int, sessionToken string) *http.Response {
		h := make(http.Header)
		if sessionToken != "" {
			h.Set("X-Apple-Session-Token", sessionToken)
		}
		return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d", status), Header: h}
	}

	for _, test := range []struct {
		name   string
		resp   *http.Response
		body   string
		accept bool
	}{
		{
			name:   "204 success no body",
			resp:   newResp(204, ""),
			body:   "",
			accept: true,
		},
		{
			name:   "409 with valid securityCode and no session token",
			resp:   newResp(409, ""),
			body:   `{"success":0,"securityCode":{"code":"123456","valid":true},"phoneNumberVerification":{}}`,
			accept: true,
		},
		{
			name:   "409 with securityCode valid=false is rejected",
			resp:   newResp(409, ""),
			body:   `{"success":0,"securityCode":{"code":"123456","valid":false}}`,
			accept: false,
		},
		{
			name:   "409 with empty body is rejected",
			resp:   newResp(409, ""),
			body:   "",
			accept: false,
		},
		{
			name:   "409 with garbage body is rejected",
			resp:   newResp(409, ""),
			body:   "not json",
			accept: false,
		},
		{
			name:   "409 with X-Apple-Session-Token header (ADP) accepted despite empty body",
			resp:   newResp(409, "session-token-value"),
			body:   "",
			accept: true,
		},
		{
			name:   "400 with valid securityCode is still rejected (only 2xx/409 count)",
			resp:   newResp(400, ""),
			body:   `{"securityCode":{"valid":true}}`,
			accept: false,
		},
		{
			name:   "nil response is rejected",
			resp:   nil,
			body:   "",
			accept: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.accept, verifyAccepted(test.resp, []byte(test.body)))
		})
	}
}

// TestGetVerifyHeaders checks that the securitycode/2sv-trust header
// overrides are applied: Origin and Referer point at icloud.com instead of
// idmsa.apple.com, the two idmsa-sign-in-only headers are absent, and
// auth-correlation headers (scnt, session id) are preserved.
func TestGetVerifyHeaders(t *testing.T) {
	s := NewSession()
	s.ClientID = "test-client-id"
	s.Scnt = "test-scnt"
	s.SessionID = "test-session-id"
	s.AuthAttributes = "test-auth-attrs"

	headers := s.getVerifyHeaders(map[string]string{})

	assert.Equal(t, baseEndpoint, headers["Origin"])
	assert.Equal(t, baseEndpoint+"/", headers["Referer"])
	assert.NotContains(t, headers, "X-Apple-I-FD-Client-Info")
	assert.NotContains(t, headers, "X-Apple-Auth-Attributes")
	// auth-correlation headers must survive the override
	assert.Equal(t, "test-scnt", headers["scnt"])
	assert.Equal(t, "test-session-id", headers["X-Apple-ID-Session-Id"])

	// overwrite still wins
	overridden := s.getVerifyHeaders(map[string]string{"Origin": "https://example.com"})
	assert.Equal(t, "https://example.com", overridden["Origin"])
}
