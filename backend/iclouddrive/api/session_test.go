package api

import (
	"context"
	"encoding/base64"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
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

// fakeIdmsa stands in for Apple's idmsa and setup endpoints. It verifies the
// escrow proof as the real server would, so a wrong account name, password
// or session key fails escrow/complete.
type fakeIdmsa struct {
	t        *testing.T
	password string
	conflict http.Header // headers on the 409 from signin/complete and the securitycode endpoints

	mu    sync.Mutex
	calls []string
	salt  []byte
	b     *big.Int
	A     *big.Int
	B     *big.Int
}

const (
	fakeEscrowChallenge = "escrow-challenge"
	fakeEscrowToken     = "escrow-session-token"
)

func (f *fakeIdmsa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.URL.Path)
	switch r.URL.Path {
	case "/appleauth/auth/signin/complete",
		"/appleauth/auth/verify/trusteddevice/securitycode",
		"/appleauth/auth/verify/phone/securitycode":
		maps.Copy(w.Header(), f.conflict)
		w.WriteHeader(http.StatusConflict)
	case "/appleauth/auth/escrow/init":
		var req struct {
			A           string   `json:"a"`
			AccountName *string  `json:"accountName"`
			Protocols   []string `json:"protocols"`
		}
		if !assert.NoError(f.t, readJSONBody(r, &req)) || !assert.NotNil(f.t, req.AccountName) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		aBytes, err := base64.StdEncoding.DecodeString(req.A)
		if !assert.NoError(f.t, err) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(f.t, "", *req.AccountName)
		assert.Equal(f.t, []string{"s2k", "s2k_fo"}, req.Protocols)
		f.A = new(big.Int).SetBytes(aBytes)
		f.salt = []byte("fake-escrow-salt")
		f.b = big.NewInt(0x1234567)
		f.B = new(big.Int).Add(new(big.Int).Mul(getMultiplier(), f.verifier()), new(big.Int).Exp(srpG, f.b, srpN))
		f.B.Mod(f.B, srpN)
		writeJSON(f.t, w, srpInitResponse{
			Iteration: 1000,
			Salt:      base64.StdEncoding.EncodeToString(f.salt),
			Protocol:  "s2k",
			B:         base64.StdEncoding.EncodeToString(padToN(f.B)),
			C:         fakeEscrowChallenge,
		})
	case "/appleauth/auth/escrow/complete":
		var req struct {
			M1 string `json:"m1"`
			M2 string `json:"m2"`
			C  string `json:"c"`
			K  string `json:"k"`
		}
		if !assert.NoError(f.t, readJSONBody(r, &req)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		u := calculateU(f.A, f.B)
		S := new(big.Int).Mul(f.A, new(big.Int).Exp(f.verifier(), u, srpN))
		S.Exp(S.Mod(S, srpN), f.b, srpN)
		K := calculateK(padToN(S))
		M1 := calculateM1([]byte(""), f.salt, padToN(f.A), padToN(f.B), K)
		M2 := calculateM2(padToN(f.A), M1, K)
		if req.C != fakeEscrowChallenge ||
			req.M1 != base64.StdEncoding.EncodeToString(M1) ||
			req.M2 != base64.StdEncoding.EncodeToString(M2) ||
			req.K != base64.StdEncoding.EncodeToString(K) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("X-Apple-Session-Token", fakeEscrowToken)
		w.WriteHeader(http.StatusOK)
	case "/appleauth/auth/2sv/trust":
		w.Header().Set("X-Apple-TwoSV-Trust-Token", "new-trust-token")
		w.WriteHeader(http.StatusNoContent)
	case "/setup/ws/1/accountLogin":
		writeJSON(f.t, w, map[string]any{})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// verifier returns the SRP verifier for the fake account's password.
func (f *fakeIdmsa) verifier() *big.Int {
	key, err := derivePassword(f.password, f.salt, 1000, "s2k")
	assert.NoError(f.t, err)
	return new(big.Int).Exp(srpG, calculateX(f.salt, key), srpN)
}

func (f *fakeIdmsa) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// redirectTransport sends every request to the test server whatever its host.
type redirectTransport struct {
	host string
	base http.RoundTripper
}

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = rt.host
	return rt.base.RoundTrip(req)
}

// newFakeIdmsaSession returns a session whose requests all go to a fakeIdmsa
// that knows the account password "correct horse".
func newFakeIdmsaSession(t *testing.T, conflict http.Header) (*Session, *fakeIdmsa) {
	t.Helper()
	fake := &fakeIdmsa{t: t, password: "correct horse", conflict: conflict}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	s := NewSession()
	s.srv = rest.NewClient(&http.Client{Transport: redirectTransport{
		host: strings.TrimPrefix(server.URL, "http://"),
		base: server.Client().Transport,
	}})
	s.password = fake.password
	return s, fake
}

func escrowHeader() http.Header {
	h := make(http.Header)
	h.Set("X-Apple-EDP", "true")
	return h
}

func TestEscrowRequired(t *testing.T) {
	newResp := func(header string) *http.Response {
		h := make(http.Header)
		if header != "" {
			h.Set(header, "true")
		}
		return &http.Response{StatusCode: http.StatusConflict, Header: h}
	}
	assert.False(t, escrowRequired(nil))
	assert.True(t, escrowRequired(newResp("X-Apple-EDP")))
	assert.True(t, escrowRequired(newResp("X-Apple-PDP")))
	assert.False(t, escrowRequired(newResp("X-Apple-TwoSV-Trust-Eligible")))
	assert.False(t, escrowRequired(newResp("")))
}

func TestCompleteEscrow(t *testing.T) {
	s, fake := newFakeIdmsaSession(t, nil)
	require.NoError(t, s.completeEscrow(t.Context()))
	assert.Equal(t, fakeEscrowToken, s.SessionToken)
	assert.Equal(t, []string{"/appleauth/auth/escrow/init", "/appleauth/auth/escrow/complete"}, fake.paths())
}

func TestCompleteEscrowWrongPassword(t *testing.T) {
	s, _ := newFakeIdmsaSession(t, nil)
	s.password = "wrong password"
	require.Error(t, s.completeEscrow(t.Context()))
	assert.Empty(t, s.SessionToken)
}

func TestCompleteEscrowNeedsPassword(t *testing.T) {
	s, fake := newFakeIdmsaSession(t, nil)
	s.password = ""
	require.Error(t, s.completeEscrow(t.Context()))
	assert.Empty(t, fake.paths())
}

func TestSignInCompletesEscrowForAcceptedTrustToken(t *testing.T) {
	trustEligible := make(http.Header)
	trustEligible.Set("X-Apple-TwoSV-Trust-Eligible", "true")
	for _, test := range []struct {
		name       string
		trustToken string
		conflict   http.Header
		want2FA    bool
		wantPaths  []string
	}{
		{
			name:       "trust token accepted",
			trustToken: "trust-token",
			conflict:   escrowHeader(),
			want2FA:    false,
			wantPaths:  []string{"/appleauth/auth/signin/complete", "/appleauth/auth/escrow/init", "/appleauth/auth/escrow/complete"},
		},
		{
			name:       "no trust token sent",
			trustToken: "",
			conflict:   escrowHeader(),
			want2FA:    true,
			wantPaths:  []string{"/appleauth/auth/signin/complete"},
		},
		{
			name:       "2FA required",
			trustToken: "trust-token",
			conflict:   trustEligible,
			want2FA:    true,
			wantPaths:  []string{"/appleauth/auth/signin/complete"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, fake := newFakeIdmsaSession(t, test.conflict)
			s.TrustToken = test.trustToken
			require.NoError(t, s.authSRPComplete(t.Context(), "user@example.com", "m1", "m2", "c"))
			assert.Equal(t, test.want2FA, s.Requires2FA())
			assert.Equal(t, test.wantPaths, fake.paths())
		})
	}
}

func TestValidateCodeCompletesEscrow(t *testing.T) {
	for _, test := range []struct {
		name     string
		validate func(ctx context.Context, s *Session) error
		codePath string
	}{
		{
			name:     "trusted device",
			validate: func(ctx context.Context, s *Session) error { return s.Validate2FACode(ctx, "123456") },
			codePath: "/appleauth/auth/verify/trusteddevice/securitycode",
		},
		{
			name:     "sms",
			validate: func(ctx context.Context, s *Session) error { return s.ValidateSMSCode(ctx, "123456", 1, "sms") },
			codePath: "/appleauth/auth/verify/phone/securitycode",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			accepted := escrowHeader()
			accepted.Set("X-Apple-Session-Token", "code-session-token")
			s, fake := newFakeIdmsaSession(t, accepted)
			require.NoError(t, test.validate(t.Context(), s))
			assert.Equal(t, []string{
				test.codePath,
				"/appleauth/auth/escrow/init",
				"/appleauth/auth/escrow/complete",
				"/appleauth/auth/2sv/trust",
				"/setup/ws/1/accountLogin",
			}, fake.paths())
			assert.Equal(t, "new-trust-token", s.TrustToken)
		})
	}
}
