package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAccountLoginDomainRedirect(t *testing.T) {
	var hosts, origins []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/setup/ws/1/accountLogin", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusFound)
		_, _ = w.Write([]byte(`{"domainToUse":"iCloud.com.cn"}`))
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	s := NewSession()
	s.srv = rest.NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		origins = append(origins, req.Header.Get("Origin"))
		copied := req.Clone(req.Context())
		copied.URL.Scheme = serverURL.Scheme
		copied.URL.Host = serverURL.Host
		return http.DefaultTransport.RoundTrip(copied)
	})})
	require.ErrorIs(t, s.AuthWithToken(context.Background()), ErrDomainChanged)
	assert.Equal(t, []string{"setup.icloud.com"}, hosts)
	assert.Equal(t, []string{"https://www.icloud.com"}, origins)
	assert.Equal(t, "iCloud.com.cn", s.DomainToUse)
	assert.Empty(t, s.AccountInfo.Webservices)
}

func TestValidateCodeCanChangeDomain(t *testing.T) {
	for _, tc := range []struct {
		name, validationPath string
		validate             func(*Session) error
	}{
		{"trusted device", "/appleauth/auth/verify/trusteddevice/securitycode", func(s *Session) error {
			return s.Validate2FACode(context.Background(), "000000")
		}},
		{"SMS", "/appleauth/auth/verify/phone/securitycode", func(s *Session) error {
			return s.ValidateSMSCode(context.Background(), "000000", 1, "sms")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			s := NewSession()
			s.srv = rest.NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				status, body := http.StatusOK, ""
				if req.URL.Path == "/setup/ws/1/accountLogin" {
					status, body = http.StatusFound, `{"domainToUse":"iCloud.com.cn"}`
				}
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})})
			require.ErrorIs(t, tc.validate(s), ErrDomainChanged)
			assert.Equal(t, []string{tc.validationPath, "/appleauth/auth/2sv/trust", "/setup/ws/1/accountLogin"}, paths)
			assert.Equal(t, "iCloud.com.cn", s.DomainToUse)
		})
	}
}

func TestSessionEndpoints(t *testing.T) {
	s := NewSession()
	assert.Equal(t, endpoints{baseEndpoint, setupEndpoint, authEndpoint}, s.endpoints())
	assert.Equal(t, "https://www.icloud.com", s.getCommonHeaders(nil)["Origin"])
	assert.Equal(t, "https://idmsa.apple.com", s.getSRPAuthHeaders()["Origin"])
	require.NoError(t, s.SetDomainToUse("iCloud.com.cn"))
	assert.Equal(t, "https://setup.icloud.com.cn/setup/ws/1", s.endpoints().setup)
	assert.Equal(t, "https://idmsa.apple.com.cn", s.getSRPAuthHeaders()["Origin"])
	assert.Equal(t, "https://idmsa.apple.com.cn/", s.getSRPAuthHeaders()["Referer"])
	assert.Equal(t, "https://www.icloud.com.cn/", s.getCommonHeaders(nil)["Referer"])
	assert.Equal(t, "https://www.icloud.com.cn", s.getSRPAuthHeaders()["X-Apple-OAuth-Redirect-URI"])
	require.Error(t, s.SetDomainToUse("example.com"))
	assert.Equal(t, "iCloud.com.cn", s.DomainToUse)
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var restored Session
	require.NoError(t, json.Unmarshal(data, &restored))
	assert.Equal(t, s.endpoints(), restored.endpoints())
}

func TestAuthStartRedirectURI(t *testing.T) {
	for _, tc := range []struct{ domain, want string }{
		{"", "https://www.icloud.com"},
		{"iCloud.com.cn", "https://www.icloud.com.cn"},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			s := NewSession()
			if tc.domain != "" {
				require.NoError(t, s.SetDomainToUse(tc.domain))
			}
			s.srv = rest.NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, tc.want, req.URL.Query().Get("redirect_uri"))
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
			})})
			require.NoError(t, s.authStart(context.Background()))
		})
	}
}

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
