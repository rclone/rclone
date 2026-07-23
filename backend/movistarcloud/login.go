package movistarcloud

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/rclone/rclone/backend/movistarcloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fshttp"
)

const (
	micloudDomain = "micloud.movistar.es"
	t3Domain      = "t3.movistar.es"

	// OAuth 2.0 (Authorization Code + PKCE) endpoints and client credentials
	// used by the Movistar Cloud macOS application.
	oauthAuthURL      = "https://apiseg.telefonica.es/openid/connect/cus/col/auth/oauth/v2/authorize"
	oauthTokenURL     = "https://apiseg.telefonica.es/openid/connect/cus/col/auth/oauth/v2/token"
	oauthClientID     = "x"
	oauthClientSecret = "x"
	oauthRedirectURI  = "https://micloud.movistar.es/ui/html/clientoauth.html"

	// Telefónica credential API used to drive the SMS one-time-password flow
	// (this is what a browser visiting the OAuth login page would talk to).
	credentialBaseURL = "https://api.telefonica.es/t3/cus/segu/v6/loginCredentialLAs"
	credentialOrigin  = "MCLOUD_APP/loginApp"
	consumerID        = "MCLOUD_APP"

	// loginPlatform impersonates the native macOS app, which is granted
	// longer-lived sessions than the web client.
	loginPlatform = "macos"

	configPhoneNumber = "phone_number"
	configAccessToken = "access_token"
	configJSessionID  = "jsessionid"
	configDeviceID    = "device_id"
	// configLoginState temporarily holds the intermediate login state between
	// the "send OTP" and "verify OTP" config steps. It is cleared on success.
	configLoginState = "login_state"
)

// credentialHeaders are sent with every call to the Telefónica credential API.
var credentialHeaders = map[string]string{
	"Content-Type":  "application/json",
	"COCO.idOrigen": credentialOrigin,
}

// cookieMap tracks cookies across requests during the login flow
type cookieMap map[string]string

func (c cookieMap) header() string {
	var parts []string
	for name, value := range c {
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func (c cookieMap) update(resp *http.Response) {
	for _, cookie := range resp.Cookies() {
		c[cookie.Name] = cookie.Value
	}
}

// fetchWithCookies makes an HTTP request with the given cookies and updates them from the response
func fetchWithCookies(ctx context.Context, client *http.Client, method, rawURL string, cookies cookieMap, body io.Reader, extraHeaders map[string]string, followRedirects bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if header := cookies.header(); header != "" {
		req.Header.Set("Cookie", header)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	if !followRedirects {
		client = &http.Client{
			Transport: client.Transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	cookies.update(resp)
	return resp, nil
}

// parseDataEnvelope decodes the standard Movistar Cloud JSON response envelope
// from r and returns the raw "data" payload.
//
// The API frequently replies with HTTP 200 while signalling failure through an
// "error" object in place of "data" (e.g. {"error":{"code":"PAPI-0000",...}}).
// Surface that as a proper error so callers report the server's message rather
// than tripping over an empty "data" field with a cryptic JSON parse error.
func parseDataEnvelope(r io.Reader) (json.RawMessage, error) {
	var wrapper api.GetResponse
	if err := json.NewDecoder(r).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if wrapper.Error != nil {
		return nil, wrapper.Error
	}
	if len(wrapper.Data) == 0 {
		return nil, errors.New("server returned an empty response")
	}
	return wrapper.Data, nil
}

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateState returns an opaque anti-CSRF value for the OAuth flow.
func generateState() string { return randomHex(16) }

// generateNonce returns an opaque OpenID Connect nonce.
func generateNonce() string { return randomHex(16) }

// generateDeviceID returns a random device identifier in the format the app uses.
func generateDeviceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "mox-" + base64.StdEncoding.EncodeToString(b)
}

// generateCodeChallenge returns the S256 PKCE challenge for a verifier.
func generateCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// normalizePhoneNumber strips separators and an optional Spanish country-code
// prefix ("+34", "0034" or "34"), leaving the bare 9-digit national number
// that the login API expects (e.g. "612345678").
func normalizePhoneNumber(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteByte(byte(r))
		}
	}
	n := digits.String()
	n = strings.TrimPrefix(n, "0034")
	// Only strip a leading "34" when it is clearly the country code in front of
	// a full 9-digit national number, to avoid mangling other inputs.
	if len(n) == 11 && strings.HasPrefix(n, "34") {
		n = n[2:]
	}
	return n
}

// pendingLogin is the intermediate state kept between sending and verifying the
// SMS one-time password. It must be serialisable so it can be stashed in the
// config file across the interactive OTP prompt.
type pendingLogin struct {
	CodeVerifier   string    `json:"code_verifier"`
	State          string    `json:"state"`
	NewSessionData string    `json:"new_session_data"`
	NewSessionID   string    `json:"new_session_id"`
	Cookies        cookieMap `json:"cookies"`
}

type manageCredentialRequest struct {
	ConsumerID  string `json:"consumerId"`
	Mobile      string `json:"Mobile"`
	SessionID   string `json:"sessionId"`
	SessionData string `json:"sessionData"`
}

type manageCredentialResponse struct {
	NewSessionData string `json:"newSessionData"`
	NewSessionID   string `json:"newSessionID"`
}

type verifyCredentialRequest struct {
	OTP         string `json:"otp"`
	SessionData string `json:"sessionData"`
	SessionID   string `json:"sessionID"`
	Mobile      string `json:"Mobile"`
}

type verifyCredentialResponse struct {
	RedirectURI string `json:"redirectUri"`
}

// validateT3URL parses the OAuth authorization redirect that points at the
// Telefónica login page and extracts the session handle from its SPA fragment,
// e.g. https://t3.movistar.es/#/accessUserPass?sessionID=...&sessionData=...
func validateT3URL(rawLocation string) (sessionID, sessionData string, err error) {
	u, err := url.Parse(rawLocation)
	if err != nil {
		return "", "", fmt.Errorf("invalid authorization redirect URL: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() != t3Domain {
		return "", "", fmt.Errorf("unexpected authorization redirect URL: %s", rawLocation)
	}

	hashIdx := strings.IndexByte(rawLocation, '#')
	if hashIdx < 0 {
		return "", "", fmt.Errorf("authorization redirect URL missing fragment: %s", rawLocation)
	}
	const routePrefix = "/accessUserPass?"
	fragment := rawLocation[hashIdx+1:]
	if !strings.HasPrefix(fragment, routePrefix) {
		return "", "", fmt.Errorf("unexpected authorization redirect route: %s", rawLocation)
	}

	values, err := url.ParseQuery(fragment[len(routePrefix):])
	if err != nil {
		return "", "", fmt.Errorf("failed to parse authorization redirect parameters: %w", err)
	}
	sessionID = values.Get("sessionID")
	sessionData = values.Get("sessionData")
	if sessionID == "" || sessionData == "" {
		return "", "", fmt.Errorf("authorization redirect missing session parameters: %s", rawLocation)
	}
	return sessionID, sessionData, nil
}

// validateMicloudOAuthURL parses the OAuth callback returned after a successful
// OTP verification and extracts the authorization code and state.
func validateMicloudOAuthURL(rawURL string) (code, state string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid OAuth redirect URL: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() != micloudDomain {
		return "", "", fmt.Errorf("unexpected OAuth redirect URL: %s", rawURL)
	}
	q := u.Query()
	code = q.Get("code")
	state = q.Get("state")
	if code == "" || state == "" {
		return "", "", fmt.Errorf("OAuth redirect missing code or state: %s", rawURL)
	}
	return code, state, nil
}

// initiateLogin starts the OAuth login flow and asks Movistar to send an SMS
// one-time password to the given phone number. The returned pendingLogin holds
// everything needed to finish the flow once the user provides the OTP.
func initiateLogin(ctx context.Context, client *http.Client, phoneNumber string) (pending *pendingLogin, err error) {
	codeVerifier := oauth2.GenerateVerifier()
	state := generateState()

	// Step 1: Hit the OAuth authorize endpoint. It responds with a redirect to
	// the Telefónica login page carrying an opaque session handle.
	authURL := oauthAuthURL + "?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {oauthClientID},
		"state":                 {state},
		"redirect_uri":          {oauthRedirectURI},
		"scope":                 {"openid"},
		"acr_values":            {"2"},
		"nonce":                 {generateNonce()},
		"code_challenge":        {generateCodeChallenge(codeVerifier)},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"},
		"lang":                  {"en"},
	}.Encode()

	resp, err := fetchWithCookies(ctx, client, "GET", authURL, cookieMap{}, nil, nil, false)
	if err != nil {
		return nil, fmt.Errorf("failed to start authorization: %w", err)
	}
	_ = resp.Body.Close()

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, errors.New("authorization endpoint did not return a redirect")
	}
	sessionID, sessionData, err := validateT3URL(location)
	if err != nil {
		return nil, err
	}

	// Step 2: Ask Telefónica to send the SMS OTP to the phone number.
	apiCookies := cookieMap{}
	manageBody, err := json.Marshal(manageCredentialRequest{
		ConsumerID:  consumerID,
		Mobile:      phoneNumber,
		SessionID:   sessionID,
		SessionData: sessionData,
	})
	if err != nil {
		return nil, err
	}
	resp, err = fetchWithCookies(ctx, client, "POST",
		credentialBaseURL+"/manageCredentialMobile",
		apiCookies,
		bytes.NewReader(manageBody),
		credentialHeaders,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to request OTP: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to request OTP: unexpected status %d", resp.StatusCode)
	}
	var manageResp manageCredentialResponse
	if err := json.NewDecoder(resp.Body).Decode(&manageResp); err != nil {
		return nil, fmt.Errorf("failed to parse OTP request response: %w", err)
	}
	if manageResp.NewSessionData == "" || manageResp.NewSessionID == "" {
		return nil, errors.New("OTP request response missing session data")
	}

	return &pendingLogin{
		CodeVerifier:   codeVerifier,
		State:          state,
		NewSessionData: manageResp.NewSessionData,
		NewSessionID:   manageResp.NewSessionID,
		Cookies:        apiCookies,
	}, nil
}

// completeLogin verifies the OTP, exchanges the resulting authorization code
// for OAuth tokens (PKCE) and trades those for a Movistar Cloud session.
func completeLogin(ctx context.Context, client *http.Client, pending *pendingLogin, phoneNumber, otp string) (session *api.Session, err error) {
	apiCookies := cookieMap{}
	for k, v := range pending.Cookies {
		apiCookies[k] = v
	}

	// Step 1: Verify the OTP. On success Telefónica returns the OAuth callback
	// URI containing the authorization code.
	verifyBody, err := json.Marshal(verifyCredentialRequest{
		OTP:         otp,
		SessionData: pending.NewSessionData,
		SessionID:   pending.NewSessionID,
		Mobile:      phoneNumber,
	})
	if err != nil {
		return nil, err
	}
	resp, err := fetchWithCookies(ctx, client, "POST",
		credentialBaseURL+"/verifyCredentialMobile",
		apiCookies,
		bytes.NewReader(verifyBody),
		credentialHeaders,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to verify OTP: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to verify OTP: unexpected status %d", resp.StatusCode)
	}
	var verifyResp verifyCredentialResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return nil, fmt.Errorf("failed to parse OTP verification response: %w", err)
	}
	if verifyResp.RedirectURI == "" {
		return nil, errors.New("OTP verification response missing redirect URI")
	}

	code, receivedState, err := validateMicloudOAuthURL(verifyResp.RedirectURI)
	if err != nil {
		return nil, err
	}
	if receivedState != pending.State {
		return nil, errors.New("state mismatch in OAuth redirect")
	}

	// Step 2: Exchange the authorization code for OAuth tokens. This is a
	// standard Authorization Code + PKCE exchange, so let golang.org/x/oauth2
	// build the request and parse the response.
	oauthConf := &oauth2.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		RedirectURL:  oauthRedirectURI,
		Scopes:       []string{"openid"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   oauthAuthURL,
			TokenURL:  oauthTokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, client)
	token, err := oauthConf.Exchange(tokenCtx, code, oauth2.VerifierOption(pending.CodeVerifier))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Step 3: Trade the OAuth token for a Movistar Cloud session. The token is
	// wrapped in the base64 JSON envelope the login endpoint expects.
	envelope, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"accesstoken":     token.AccessToken,
			"expiresin":       strconv.FormatInt(token.ExpiresIn, 10),
			"lastrefreshdate": 0,
			"platform":        loginPlatform,
			"refreshtoken":    token.RefreshToken,
		},
	})
	if err != nil {
		return nil, err
	}
	return refreshSession(ctx, client, &api.Session{
		AccessToken: base64.StdEncoding.EncodeToString(envelope),
	})
}

// refreshSession exchanges an access token for a fresh Movistar Cloud session
// (a new access token plus JSESSIONID) using the OAuth login endpoint.
//
// The Movistar Cloud access token doubles as a long-lived credential: posting
// it back to this endpoint mints a rotated token and session, which is how
// sessions are kept alive without a standard OAuth refresh_token grant.
func refreshSession(ctx context.Context, client *http.Client, session *api.Session) (newSession *api.Session, err error) {
	if session.AccessToken == "" {
		return nil, errors.New("cannot refresh session without an access token")
	}

	deviceID := session.DeviceID
	if deviceID == "" {
		deviceID = generateDeviceID()
	}

	headers := map[string]string{
		"Authorization": "oauth " + session.AccessToken,
		"X-deviceid":    deviceID,
	}
	cookies := cookieMap{}
	if session.JSessionID != "" {
		cookies["JSESSIONID"] = session.JSessionID
	}

	resp, err := fetchWithCookies(ctx, client, "POST",
		rootURL+"/sapi/login/oauth?action=login&responsetime=true",
		cookies,
		nil,
		headers,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh session: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("session refresh failed with status %d", resp.StatusCode)
	}

	data, err := parseDataEnvelope(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh session: %w", err)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		JSessionID  string `json:"jsessionid"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	if result.AccessToken == "" || result.JSessionID == "" {
		return nil, errors.New("refresh returned empty credentials")
	}
	if result.AccessToken == session.AccessToken {
		return nil, errors.New("refresh did not return a new access token")
	}
	if session.JSessionID != "" && result.JSessionID == session.JSessionID {
		return nil, errors.New("refresh did not return a new session id")
	}

	return &api.Session{
		AccessToken: result.AccessToken,
		JSessionID:  result.JSessionID,
		DeviceID:    deviceID,
	}, nil
}

// saveSession persists the session to the rclone config
func saveSession(m configmap.Mapper, session *api.Session) {
	m.Set(configAccessToken, session.AccessToken)
	m.Set(configJSessionID, session.JSessionID)
	if session.DeviceID != "" {
		m.Set(configDeviceID, session.DeviceID)
	}
}

// Config implements the multi-step interactive configuration for Movistar Cloud.
//
// State machine:
//
//	""    → read phone number, start OAuth login, send SMS OTP, prompt for it
//	"otp" → verify OTP, finish OAuth exchange, save the resulting session
func Config(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	rawPhone, _ := m.Get(configPhoneNumber)
	phoneNumber := normalizePhoneNumber(rawPhone)
	if phoneNumber == "" {
		return nil, errors.New("phone number not set - please set the phone_number option (e.g. 612345678)")
	}

	switch configIn.State {
	case "":
		client := fshttp.NewClient(ctx)
		pending, err := initiateLogin(ctx, client, phoneNumber)
		if err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}
		pendingJSON, err := json.Marshal(pending)
		if err != nil {
			return nil, fmt.Errorf("failed to store login state: %w", err)
		}
		m.Set(configLoginState, string(pendingJSON))
		fs.Logf(nil, "An OTP code has been sent via SMS to %s.", phoneNumber)
		return fs.ConfigInput("otp", "config_otp", "Enter the OTP code sent to your phone via SMS")

	case "otp":
		otp := strings.TrimSpace(configIn.Result)
		if otp == "" {
			return fs.ConfigInput("otp", "config_otp", "OTP code can't be blank - enter the code sent to your phone via SMS")
		}
		pendingJSON, ok := m.Get(configLoginState)
		if !ok || pendingJSON == "" {
			return nil, errors.New("login state lost - please run \"rclone config reconnect\" to try again")
		}
		var pending pendingLogin
		if err := json.Unmarshal([]byte(pendingJSON), &pending); err != nil {
			return nil, fmt.Errorf("failed to read login state: %w", err)
		}

		client := fshttp.NewClient(ctx)
		session, err := completeLogin(ctx, client, &pending, phoneNumber, otp)
		if err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}

		m.Set(configLoginState, "")
		saveSession(m, session)
		fs.Logf(nil, "Authentication successful!")
		return nil, nil
	}
	return nil, fmt.Errorf("unknown state %q", configIn.State)
}
