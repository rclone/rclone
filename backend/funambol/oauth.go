package funambol

// OAuth login for O2 Spain, used when client_id is set and pass is empty.
// It sends the same SMS code as the SMS login (sms.go), but runs the OAuth
// flow itself with the client credentials of an O2 desktop app instead of
// letting the server run it.  This gets a refresh token, from which SAPI
// sessions are created again unattended when they expire:
//
//	rclone                      O2 server            apiseg / T3
//	  |-- authorize (PKCE) --------------------------->|
//	  |<-- 302 t3#…sessionID&sessionData --------------|
//	  |-- manageCredentialMobileO2 ------------------->|  sends the SMS
//	  |-- verifyCredentialMobileO2 (code) ------------>|
//	  |<-- redirects to clientoauth.html?code&state ---|  (not fetched)
//	  |-- token (code + verifier) -------------------->|
//	  |-- /sapi/login/oauth + token ->|  (sets session cookies)
//
// Every SAPI call carries the token in an "Authorization: oauth" header, and
// the server returns renewed tokens in the same header of its replies.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/funambol/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

// O2 Spain's OAuth endpoints; vars so tests can point them elsewhere.
var (
	o2AuthURL  = "https://apiseg.telefonica.es/openid/connect/auth/oauth/v2/o2/cus/authorize"
	o2TokenURL = "https://apiseg.telefonica.es/openid/connect/auth/oauth/v2/o2/cus/token"
)

const (
	clientOAuthPath      = "/ui/html/clientoauth.html" // callback the O2 desktop apps are registered with
	configOAuthCreds     = "oauth_credentials"         // last token envelope the server returned
	defaultOAuthPlatform = "windows"
)

// oauthEnvelope is the JSON carried, base64 encoded, in the Authorization
// header of SAPI requests and replies.  Data holds fields the server adds
// (expiresin, lastrefreshdate, ...), which are sent back as received.
type oauthEnvelope struct {
	Data map[string]any `json:"data"`
}

// usesOAuth reports whether the remote logs in with OAuth.
func (opt *Options) usesOAuth() bool {
	return opt.ClientID != "" && opt.Pass == ""
}

// oauthConfig returns the OAuth client configuration.
func (f *Fs) oauthConfig() (*oauth2.Config, error) {
	secret := f.opt.ClientSecret
	if secret != "" {
		var err error
		secret, err = obscure.Reveal(secret)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt client_secret: %w", err)
		}
	}
	return &oauth2.Config{
		ClientID:     f.opt.ClientID,
		ClientSecret: secret,
		Endpoint:     oauth2.Endpoint{AuthURL: o2AuthURL, TokenURL: o2TokenURL},
		RedirectURL:  f.opt.Endpoint + clientOAuthPath,
		Scopes:       []string{"openid"},
	}, nil
}

// oauthStart returns the authorization URL the SMS login starts from, and
// records its state and PKCE verifier in p.
func (f *Fs) oauthStart(p *smsPending) (string, error) {
	c, err := f.oauthConfig()
	if err != nil {
		return "", err
	}
	p.State, err = random.Password(128)
	if err != nil {
		return "", err
	}
	p.Verifier = oauth2.GenerateVerifier()
	// acr_values=2 selects the SMS login, as the desktop apps ask for it.
	return c.AuthCodeURL(p.State, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(p.Verifier),
		oauth2.SetAuthURLParam("acr_values", "2")), nil
}

// oauthFinish follows the redirect T3 returns after the code check to the
// OAuth callback, exchanges the code for a token and logs in with it.
func (f *Fs) oauthFinish(ctx context.Context, p *smsPending, redirect string) error {
	code, err := f.oauthCode(ctx, p, redirect)
	if err != nil {
		return err
	}
	c, err := f.oauthConfig()
	if err != nil {
		return err
	}
	token, err := c.Exchange(context.WithValue(ctx, oauth2.HTTPClient, f.client), code, oauth2.VerifierOption(p.Verifier))
	if err != nil {
		return fmt.Errorf("getting the OAuth token: %w", err)
	}
	if token.RefreshToken == "" {
		return errors.New("getting the OAuth token: no refresh token in reply - check client_id")
	}
	if err := f.putToken(token); err != nil {
		return err
	}
	f.m.Set(configOAuthCreds, "")
	f.clearSession()
	return f.oauthLogin(ctx)
}

// oauthCode follows redirects from rawURL until the OAuth callback and returns
// the code it carries.  The callback itself is not fetched: it is a web page
// for the desktop apps, which read the code from its URL.
func (f *Fs) oauthCode(ctx context.Context, p *smsPending, rawURL string) (string, error) {
	want, err := url.Parse(f.opt.Endpoint + clientOAuthPath)
	if err != nil {
		return "", err
	}
	for range maxHops {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", errors.New("unparsable OAuth redirect")
		}
		if u.Scheme == want.Scheme && u.Host == want.Host && u.Path == want.Path {
			q := u.Query()
			if q.Get("state") != p.State {
				return "", errors.New("OAuth state mismatch")
			}
			if e := q.Get("error"); e != "" {
				return "", fmt.Errorf("OAuth authorization refused: %s", e)
			}
			if q.Get("code") == "" {
				return "", errors.New("OAuth callback has no code")
			}
			return q.Get("code"), nil
		}
		next, err := f.get(ctx, rawURL)
		if err != nil {
			return "", err
		}
		if next == "" {
			return "", fmt.Errorf("OAuth redirects stopped before %s", want.Path)
		}
		rawURL = next
	}
	return "", fmt.Errorf("OAuth redirects did not reach %s", want.Path)
}

// putToken saves token in the config.
func (f *Fs) putToken(token *oauth2.Token) error {
	f.auth.tokenMu.Lock()
	defer f.auth.tokenMu.Unlock()
	return oauthutil.PutToken(f.name, f.m, token, false)
}

// refreshToken gets a new token with the refresh token from the token
// endpoint.
func (f *Fs) refreshToken(ctx context.Context) error {
	token, err := oauthutil.GetToken(f.name, f.m)
	if err != nil {
		return err
	}
	c, err := f.oauthConfig()
	if err != nil {
		return err
	}
	// Without an access token the token source always refreshes.
	ts := c.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, f.client), &oauth2.Token{RefreshToken: token.RefreshToken})
	token, err = ts.Token()
	if err != nil {
		return err
	}
	return f.putToken(token)
}

// clearSession removes the SAPI session cookies from the jar.
func (f *Fs) clearSession() {
	f.client.Jar.SetCookies(f.jarURL(), []*http.Cookie{{Name: "JSESSIONID", MaxAge: -1}, {Name: "validationKey", MaxAge: -1}})
}

// oauthLogin creates a SAPI session with the saved token.  The server renews
// an expired token itself; only if it refuses the token is the refresh token
// sent to the token endpoint.  It returns errReconnect when the token can't be
// renewed.  The caller must hold auth.mu.
func (f *Fs) oauthLogin(ctx context.Context) error {
	if _, err := oauthutil.GetToken(f.name, f.m); err != nil {
		return errReconnect(f.name)
	}
	_, sent := f.oauthHeader()
	resp, err := f.sapiOAuthLogin(ctx)
	// The server answers a login with an expired access token with a redirect
	// that carries the renewed token, so log in again with it.
	if _, access := f.oauthHeader(); err != nil && access != sent {
		fs.Debugf(f, "OAuth login renewed the token, logging in again: %v", err)
		resp, err = f.sapiOAuthLogin(ctx)
	}
	if !isAuthFailure(resp, err) {
		return err
	}
	fs.Debugf(f, "OAuth login refused, refreshing the token: %v", err)
	if err := f.refreshToken(ctx); err != nil {
		fs.Debugf(f, "OAuth token refresh failed: %v", err)
		return errReconnect(f.name)
	}
	resp, err = f.sapiOAuthLogin(ctx)
	if isAuthFailure(resp, err) {
		fs.Debugf(f, "OAuth login refused after token refresh: %v", err)
		return errReconnect(f.name)
	}
	return err
}

// sapiOAuthLogin posts the login; the transport adds the token.
func (f *Fs) sapiOAuthLogin(ctx context.Context) (*http.Response, error) {
	opts := rest.Opts{
		Method:      "POST",
		Path:        oauthPath,
		Parameters:  url.Values{"action": {"login"}, "responsetime": {"true"}},
		ContentType: "application/x-www-form-urlencoded; charset=UTF-8",
		NoRedirect:  true,
	}
	var res otpValidateResponse
	var resp *http.Response
	err := f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &res)
		return shouldRetry(ctx, resp, err)
	})
	if err == nil {
		err = res.AsErr()
	}
	if err != nil {
		return resp, fmt.Errorf("OAuth login: %w", err)
	}
	if res.Data.JSessionID != "" {
		f.client.Jar.SetCookies(f.jarURL(), []*http.Cookie{{Name: "JSESSIONID", Value: res.Data.JSessionID}})
	}
	vk := res.Data.ValidationKey
	if resp.Header.Get("validationkey") != "" {
		vk = resp.Header.Get("validationkey")
	}
	f.setValidationKey(vk)
	if f.key() == "" {
		return resp, errors.New("OAuth login: no validation key in reply")
	}
	f.persistSession()
	return resp, nil
}

// isAuthFailure reports whether resp and err show that SAPI refused the
// session or the token.
func isAuthFailure(resp *http.Response, err error) bool {
	if err == nil {
		return false
	}
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		return true
	}
	var ae *api.Error
	return errors.As(err, &ae) && (isReauthCode(ae.Code) || ae.Code == "SEC-1003")
}

// reloginOnError creates a new SAPI session with the OAuth token when resp and
// err show that the session sent with the validation key sent is gone.  It
// reports whether the caller should retry.
func (f *Fs) reloginOnError(ctx context.Context, resp *http.Response, err error, sent string) (bool, error) {
	if !f.opt.usesOAuth() || !isAuthFailure(resp, err) {
		return false, nil
	}
	// A refused download from another host says nothing about the session.
	if resp != nil && resp.Request != nil && !f.isSAPIURL(resp.Request.URL) {
		return false, nil
	}
	f.auth.mu.Lock()
	defer f.auth.mu.Unlock()
	// Another call has created a new session already.
	if f.key() != sent {
		return true, nil
	}
	fs.Debugf(f, "session expired, logging in again with the OAuth token: %v", err)
	f.clearSession()
	if err := f.oauthLogin(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// isSAPIURL reports whether u is a SAPI call on the endpoint.
func (f *Fs) isSAPIURL(u *url.URL) bool {
	base := f.jarURL()
	return u.Scheme == base.Scheme && strings.EqualFold(u.Host, base.Host) && strings.HasPrefix(u.Path, "/sapi/")
}

// oauthHeader returns the Authorization header that carries the saved token
// to SAPI and the access token in it, or "" if there is no token.
func (f *Fs) oauthHeader() (header, access string) {
	f.auth.tokenMu.Lock()
	defer f.auth.tokenMu.Unlock()
	token, err := oauthutil.GetToken(f.name, f.m)
	if err != nil {
		return "", ""
	}
	var env oauthEnvelope
	if raw, _ := f.m.Get(configOAuthCreds); raw != "" {
		_ = json.Unmarshal([]byte(raw), &env)
	}
	if env.Data == nil {
		env.Data = make(map[string]any)
	}
	// The server's lifetime fields describe the access token it sent.
	if env.Data["accesstoken"] != token.AccessToken {
		delete(env.Data, "expiresin")
		delete(env.Data, "lastrefreshdate")
	}
	env.Data["accesstoken"] = token.AccessToken
	env.Data["refreshtoken"] = token.RefreshToken
	env.Data["valid"] = token.Valid()
	env.Data["platform"] = f.opt.OAuthPlatform
	if f.opt.OAuthPlatform == "" {
		env.Data["platform"] = defaultOAuthPlatform
	}
	if mobile, err := normalizePhone(f.opt.User); err == nil {
		env.Data["msisdn"] = mobile
	}
	b, err := json.Marshal(env)
	if err != nil {
		return "", ""
	}
	return "oauth " + base64.StdEncoding.EncodeToString(b), token.AccessToken
}

// saveOAuthHeader saves the renewed token the server returns in the
// Authorization header of resp, for a request sent with the access token sent.
func (f *Fs) saveOAuthHeader(resp *http.Response, sent string) {
	header := resp.Header.Get("Authorization")
	if header == "" {
		return
	}
	scheme, encoded, _ := strings.Cut(header, " ")
	if !strings.EqualFold(scheme, "oauth") {
		fs.Debugf(f, "ignoring Authorization reply header with scheme %q", scheme)
		return
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	var env struct {
		Data struct {
			AccessToken  string      `json:"accesstoken"`
			RefreshToken string      `json:"refreshtoken"`
			ExpiresIn    json.Number `json:"expiresin"`
		} `json:"data"`
	}
	if err == nil {
		err = json.Unmarshal(b, &env)
	}
	if err != nil {
		fs.Debugf(f, "ignoring unparsable Authorization reply header: %v", err)
		return
	}

	f.auth.tokenMu.Lock()
	defer f.auth.tokenMu.Unlock()
	token, err := oauthutil.GetToken(f.name, f.m)
	// A token saved since the request was sent is newer than this reply.
	if err != nil || token.AccessToken != sent {
		return
	}
	if raw, _ := f.m.Get(configOAuthCreds); raw == string(b) {
		return
	}
	changed := env.Data.AccessToken != "" && env.Data.AccessToken != token.AccessToken
	if changed {
		token.AccessToken = env.Data.AccessToken
		token.Expiry = time.Time{}
	}
	if env.Data.RefreshToken != "" {
		token.RefreshToken = env.Data.RefreshToken
	}
	if s, err := env.Data.ExpiresIn.Int64(); err == nil && s > 0 && (changed || token.Expiry.IsZero()) {
		token.Expiry = time.Now().Add(time.Duration(s) * time.Second)
	}
	if err := oauthutil.PutToken(f.name, f.m, token, false); err != nil {
		fs.Errorf(f, "couldn't save the renewed OAuth token: %v", err)
		return
	}
	f.m.Set(configOAuthCreds, string(b))
}

// oauthTransport adds the OAuth token to every SAPI request and saves the
// renewed tokens the replies return, including download redirects.
type oauthTransport struct {
	f    *Fs
	base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.f.isSAPIURL(req.URL) {
		return t.base.RoundTrip(req)
	}
	header, sent := t.f.oauthHeader()
	if header == "" {
		return t.base.RoundTrip(req)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", header)
	resp, err := t.base.RoundTrip(req)
	if err == nil {
		t.f.saveOAuthHeader(resp, sent)
	}
	return resp, err
}

// useOAuthTransport makes the client send the OAuth token if the remote uses
// OAuth.
func (f *Fs) useOAuthTransport() {
	if f.opt.usesOAuth() {
		f.client.Transport = &oauthTransport{f: f, base: f.client.Transport}
	}
}
