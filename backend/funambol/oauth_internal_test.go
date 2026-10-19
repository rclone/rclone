package funambol

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClient    = "client-1"
	oauthCode     = "auth-code"
	oauthSession  = "authorize-session"
	oauthStaleKey = "vk-stale"
)

// oauthServer fakes the O2 server, the OAuth authorization server and the T3
// credential API on one server.
type oauthServer struct {
	srv          *httptest.Server
	state        string       // state of the last authorization request
	challenge    string       // PKCE challenge of the last authorization request
	refused      string       // access token the SAPI login refuses
	expired      string       // access token the SAPI login renews with a redirect
	logins       atomic.Int32 // SAPI logins
	refreshes    atomic.Int32 // refresh token grants
	clientID     string
	secret       string
	platform     string
	refreshToken string
}

// oauthHeaderData decodes the token envelope of an Authorization header.
func oauthHeaderData(t *testing.T, header string) map[string]any {
	encoded, ok := strings.CutPrefix(header, "oauth ")
	require.True(t, ok, "Authorization header %q", header)
	b, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	var env oauthEnvelope
	require.NoError(t, json.Unmarshal(b, &env))
	return env.Data
}

// renewedHeader is the Authorization reply header renewing access to
// access+"r".
func renewedHeader(access string) string {
	b, _ := json.Marshal(map[string]any{"data": map[string]any{
		"accesstoken": access + "r", "refreshtoken": "rt-renewed", "expiresin": 3600, "lastrefreshdate": 1,
	}})
	return "oauth " + base64.StdEncoding.EncodeToString(b)
}

func newOAuthServer(t *testing.T) *oauthServer {
	s := &oauthServer{clientID: testClient, platform: defaultOAuthPlatform, refreshToken: "rt1"}
	mux := http.NewServeMux()
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)

	oldT3, oldAuth, oldToken := t3API, o2AuthURL, o2TokenURL
	t3API, o2AuthURL, o2TokenURL = s.srv.URL+"/t3", s.srv.URL+"/authorize", s.srv.URL+"/token"
	t.Cleanup(func() { t3API, o2AuthURL, o2TokenURL = oldT3, oldAuth, oldToken })

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, s.clientID, q.Get("client_id"))
		assert.Equal(t, s.srv.URL+clientOAuthPath, q.Get("redirect_uri"))
		assert.Equal(t, "S256", q.Get("code_challenge_method"))
		assert.Equal(t, "2", q.Get("acr_values"))
		assert.Equal(t, "offline", q.Get("access_type"))
		s.state, s.challenge = q.Get("state"), q.Get("code_challenge")
		http.SetCookie(w, &http.Cookie{Name: "authsession", Value: oauthSession, Path: "/"})
		http.Redirect(w, r, "https://"+t3Host+"/acceso/#/accessUserPassO2?client_name="+t3Consumer+
			"&sessionID="+smsSession+"&sessionData="+smsData, http.StatusFound)
	})
	addT3Handlers(t, mux, func() string { return s.srv.URL + "/authorize/continue" })

	// Only the session that sent the SMS gets the code.
	mux.HandleFunc("/authorize/continue", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("authsession")
		if err != nil || c.Value != oauthSession {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, s.srv.URL+clientOAuthPath+"?code="+oauthCode+"&state="+s.state, http.StatusFound)
	})
	mux.HandleFunc(clientOAuthPath, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the OAuth callback must not be fetched")
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		clientID, secret, basic := r.BasicAuth()
		if !basic {
			clientID, secret = r.FormValue("client_id"), r.FormValue("client_secret")
		}
		assert.Equal(t, s.clientID, clientID)
		assert.True(t, secret == s.secret, "the token grant must use the application's matching secret")
		w.Header().Set("Content-Type", "application/json")
		switch r.FormValue("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
			assert.Equal(t, s.challenge, base64.RawURLEncoding.EncodeToString(sum[:]))
			assert.Equal(t, oauthCode, r.FormValue("code"))
			_, _ = io.WriteString(w, `{"access_token":"at1","refresh_token":"rt1","token_type":"Bearer","expires_in":3600}`)
		case "refresh_token":
			s.refreshes.Add(1)
			assert.Equal(t, s.refreshToken, r.FormValue("refresh_token"))
			_, _ = io.WriteString(w, `{"access_token":"at3","refresh_token":"rt3","token_type":"Bearer","expires_in":3600}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	mux.HandleFunc(oauthPath, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "login", r.URL.Query().Get("action"))
		data := oauthHeaderData(t, r.Header.Get("Authorization"))
		assert.Equal(t, s.platform, data["platform"])
		assert.Equal(t, smsPhone, data["msisdn"])
		access, _ := data["accesstoken"].(string)
		if access == s.refused {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"code":"SEC-1004"}}`)
			return
		}
		s.logins.Add(1)
		w.Header().Set("Authorization", renewedHeader(access))
		if access == s.expired {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"jsessionid":"js-1","validationkey":"`+testKey+`"}}`)
	})
	mux.HandleFunc("/sapi/media/folder/root", func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("Authorization"))
		if r.URL.Query().Get("validationkey") != testKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"code":"SEC-1004"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"folders":[{"id":1,"name":"root"}]}}`)
	})
	return s
}

// config returns a remote using OAuth, already logged in with token if set.
func (s *oauthServer) config(t *testing.T, token string) configmap.Simple {
	m := configmap.Simple{"user": "+34 612 34 56 78", "endpoint": s.srv.URL, "client_id": testClient}
	if token != "" {
		cookies, err := json.Marshal([]cookiePair{{"validationKey", oauthStaleKey}})
		require.NoError(t, err)
		m["token"] = token
		m["cookies"] = base64.StdEncoding.EncodeToString(cookies)
	}
	return m
}

func savedToken(t *testing.T, m configmap.Mapper) (access, refresh string) {
	token, err := oauthutil.GetToken("test", m)
	require.NoError(t, err)
	return token.AccessToken, token.RefreshToken
}

// With client_id set, the SMS login gets an OAuth token and logs in with it.
func TestConfigOAuth(t *testing.T) {
	s := newOAuthServer(t)
	m := s.config(t, "")

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, "sms", out.State)

	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "sms", Result: smsCode})
	require.NoError(t, err)
	require.Nil(t, out)

	// The token the login renewed is the one saved.
	access, refresh := savedToken(t, m)
	assert.Equal(t, "at1r", access)
	assert.Equal(t, "rt-renewed", refresh)
	assert.NotEmpty(t, m[configOAuthCreds])
	assert.Empty(t, m[configTmpSMS])
	assert.True(t, strings.HasPrefix(m["device_id"], "fol-"), m["device_id"])

	f, err := newSessionFs(context.Background(), "test", m)
	require.NoError(t, err)
	f.restoreCookies(m["cookies"])
	assert.Equal(t, testKey, f.validationKeyFromJar())
}

func TestConfigOAuthWithBundledCredentials(t *testing.T) {
	s := newOAuthServer(t)
	s.clientID, s.platform = o2OAuthClientID, "macos"
	var err error
	s.secret, err = obscure.Reveal(o2OAuthEncryptedClientSecret)
	require.NoError(t, err)
	m := s.config(t, "")
	m["client_id"], m["client_secret"], m["oauth_platform"] = s.clientID, o2OAuthEncryptedClientSecret, s.platform

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, "sms", out.State)
	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "sms", Result: smsCode})
	require.NoError(t, err)
	require.Nil(t, out)
	access, refresh := savedToken(t, m)
	require.NotEmpty(t, refresh)

	// A new process restores the saved credentials after its SAPI session expires.
	s.refused, s.refreshToken = access, refresh
	cookies, err := json.Marshal([]cookiePair{{"validationKey", oauthStaleKey}})
	require.NoError(t, err)
	m["cookies"] = base64.StdEncoding.EncodeToString(cookies)
	_, err = NewFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.refreshes.Load())
	access, refresh = savedToken(t, m)
	assert.Equal(t, "at3r", access)
	assert.Equal(t, "rt-renewed", refresh)
}

// An expired session is created again from the token, without a new code.
func TestOAuthReloginOnExpiredSession(t *testing.T) {
	s := newOAuthServer(t)
	m := s.config(t, `{"access_token":"at1","refresh_token":"rt1"}`)

	_, err := NewFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.logins.Load())
	assert.Equal(t, int32(0), s.refreshes.Load())
	access, _ := savedToken(t, m)
	assert.Equal(t, "at1r", access)
}

// A token the server refuses is refreshed at the token endpoint.
func TestOAuthRefreshesRefusedToken(t *testing.T) {
	s := newOAuthServer(t)
	s.refused = "at1"
	m := s.config(t, `{"access_token":"at1","refresh_token":"rt1"}`)

	_, err := NewFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.refreshes.Load())
	access, _ := savedToken(t, m)
	assert.Equal(t, "at3r", access)
}

// A login that renews an expired token with a redirect is sent again with the
// renewed token.
func TestOAuthReloginAfterRedirectRenewsToken(t *testing.T) {
	s := newOAuthServer(t)
	s.expired = "at1"
	m := s.config(t, `{"access_token":"at1","refresh_token":"rt1"}`)

	_, err := NewFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	assert.Equal(t, int32(2), s.logins.Load())
	assert.Equal(t, int32(0), s.refreshes.Load())
	access, _ := savedToken(t, m)
	assert.Equal(t, "at1rr", access)
}

// A token that can't be renewed asks for "rclone config reconnect".
func TestOAuthReconnectWhenRefreshFails(t *testing.T) {
	s := newOAuthServer(t)
	s.refused = "at1"
	m := s.config(t, `{"access_token":"at1","refresh_token":"rt-revoked"}`)
	o2TokenURL = s.srv.URL + "/nowhere"

	_, err := NewFs(context.Background(), "test", "", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rclone config reconnect test:")
}

// A reply to a request sent with an older token must not undo a newer one.
func TestSaveOAuthHeaderKeepsNewerToken(t *testing.T) {
	s := newOAuthServer(t)
	m := s.config(t, `{"access_token":"at1","refresh_token":"rt1"}`)
	f, err := newSessionFs(context.Background(), "test", m)
	require.NoError(t, err)

	resp := &http.Response{Header: http.Header{"Authorization": {renewedHeader("old")}}}
	f.saveOAuthHeader(resp, "old")
	access, _ := savedToken(t, m)
	assert.Equal(t, "at1", access)

	f.saveOAuthHeader(resp, "at1")
	access, refresh := savedToken(t, m)
	assert.Equal(t, "oldr", access)
	assert.Equal(t, "rt-renewed", refresh)
}

// Without client_id the remote keeps the server-side SMS login.
func TestUsesOAuth(t *testing.T) {
	assert.False(t, (&Options{}).usesOAuth())
	assert.True(t, (&Options{ClientID: testClient}).usesOAuth())
	assert.False(t, (&Options{ClientID: testClient, Pass: "x"}).usesOAuth())
}
