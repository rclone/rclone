package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rclone/rclone/lib/rest"
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

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestValidateCodes_409Responses(t *testing.T) {
	tests := []struct {
		name       string
		isSMS      bool
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "2FA Success on 409 with Valid Code",
			isSMS:      false,
			statusCode: http.StatusConflict,
			body:       `{"securityCode":{"code":"123456","valid":true}}`,
			wantErr:    false,
		},
		{
			name:       "2FA Failure on 409 with Invalid Code",
			isSMS:      false,
			statusCode: http.StatusConflict,
			body:       `{"securityCode":{"code":"123456","valid":false}}`,
			wantErr:    true,
		},
		{
			name:       "SMS Success on 409 with Valid Code",
			isSMS:      true,
			statusCode: http.StatusConflict,
			body:       `{"securityCode":{"code":"123456","valid":true}}`,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			s.srv = rest.NewClient(&http.Client{
				Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
					path := req.URL.Path
					expectedPath := "/appleauth/auth/verify/trusteddevice/securitycode"
					if tt.isSMS {
						expectedPath = "/appleauth/auth/verify/phone/securitycode"
					}

					if path == expectedPath {
						return &http.Response{
							StatusCode: tt.statusCode,
							Status:     "409 Conflict",
							Body:       io.NopCloser(strings.NewReader(tt.body)),
							Header:     make(http.Header),
						}, nil
					}
					if path == "/appleauth/auth/2sv/trust" || path == "/setup/ws/1/accountLogin" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Status:     "200 OK",
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Header:     make(http.Header),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Status:     "400 Bad Request",
						Body:       io.NopCloser(strings.NewReader("")),
						Header:     make(http.Header),
					}, nil
				}),
			})

			var err error
			if tt.isSMS {
				err = s.ValidateSMSCode(context.Background(), "123456", 1, "sms")
			} else {
				err = s.Validate2FACode(context.Background(), "123456")
			}

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
