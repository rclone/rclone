package movistarcloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rclone/rclone/backend/movistarcloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fshttp"
)

const (
	baseMvcDomain            = "micloud.movistar.es"
	baseMobileConnectDomain  = "mobileconnect.telefonica.es"
	maxRedirects             = 5
	defaultPollingIntervalMs = 2000
	maxPollingAttempts       = 120 // 120 * 2s = 4 minutes max wait
	configPhoneNumber        = "phone_number"
	configAccessToken        = "access_token"
	configJSessionID         = "jsessionid"
)

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
	req.Header.Set("Cookie", cookies.header())
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

// verifyMobileConnectURL checks the URL points to the expected MobileConnect domain
func verifyMobileConnectURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Hostname() != baseMobileConnectDomain || u.Scheme != "https" {
		return fmt.Errorf("invalid authorization URL: %s", rawURL)
	}
	return nil
}

// extractJWTCorr extracts the "corr" field from a JWT header without verifying the signature
func extractJWTCorr(jwtString string) (string, error) {
	// The JWT has 3 parts separated by dots; the header is the first part
	parts := strings.SplitN(jwtString, ".", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid JWT format")
	}

	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(jwtString, jwt.MapClaims{})
	if err != nil {
		// If token parsing fails, try just decoding the header
		headerBytes, decErr := base64.RawURLEncoding.DecodeString(parts[0])
		if decErr != nil {
			return "", fmt.Errorf("failed to decode JWT header: %w", err)
		}
		var header map[string]interface{}
		if jsonErr := json.Unmarshal(headerBytes, &header); jsonErr != nil {
			return "", fmt.Errorf("failed to parse JWT header: %w", jsonErr)
		}
		corr, ok := header["corr"].(string)
		if !ok || corr == "" {
			return "", fmt.Errorf("JWT header missing corr field")
		}
		return corr, nil
	}

	// Try header first (where the TS code looks)
	corr, ok := token.Header["corr"].(string)
	if ok && corr != "" {
		return corr, nil
	}
	return "", fmt.Errorf("JWT header missing corr field")
}

// loginState holds intermediate state during the SMS login flow
type loginState struct {
	micloudCookies       cookieMap
	mobileConnectCookies cookieMap
	pollingURL           string
	pollingIntervalMs    int
	finishURL            string
}

// Regex patterns for extracting polling parameters and CSRF token from the HTML response.
// These match the JavaScript calls and form fields that the MobileConnect page uses.
var (
	pollingRe = regexp.MustCompile(`(?i)\bmanage_sba_polling\s*\(\s*['"](.+?)['"]\s*,\s*\{([^}]+)\}\s*,\s*\w+\s*,\s*(\d+)\s*\)`)
	csrfRe    = regexp.MustCompile(`(?is)<form\s+action\s*=\s*['"](.+?)['"]\s+method=['"]post['"].*?>.*?<input\b[^>]*?['"]csrfmiddlewaretoken['"][^>]*?value\s*=\s*['"](.+?)['"]`)
	jsKeyRe   = regexp.MustCompile(`(?m)(\w+)\s*:\s*['"]([^'"]+)['"]`)
)

// parseJSObject extracts key-value pairs from a JS-style object string like {key: 'value', ...}
func parseJSObject(s string) map[string]string {
	result := make(map[string]string)
	for _, match := range jsKeyRe.FindAllStringSubmatch(s, -1) {
		result[match[1]] = match[2]
	}
	return result
}

// initiateLogin starts the MobileConnect SMS authentication flow.
// It returns a loginState containing the cookies and URLs needed to
// poll for authentication and finish the flow.
func initiateLogin(ctx context.Context, client *http.Client, phoneNumber string) (state *loginState, err error) {
	micloudCookies := cookieMap{}

	// Step 1: Get initial JSESSIONID cookie
	resp, err := fetchWithCookies(ctx, client, "GET", "https://"+baseMvcDomain, micloudCookies, nil, nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial session: %w", err)
	}
	_ = resp.Body.Close()

	if _, ok := micloudCookies["JSESSIONID"]; !ok {
		return nil, fmt.Errorf("failed to get initial JSESSIONID cookie")
	}

	// Step 2: Start login process
	formData := "platform=web&msisdn=" + url.QueryEscape(phoneNumber)
	resp, err = fetchWithCookies(ctx, client, "POST",
		"https://"+baseMvcDomain+"/sapi/login/mobileconnect?action=start",
		micloudCookies,
		strings.NewReader(formData),
		map[string]string{
			"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
		},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start login: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	var wrapper api.GetResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}
	var loginResp api.LoginStartResponse
	if err := json.Unmarshal(wrapper.Data, &loginResp); err != nil {
		return nil, fmt.Errorf("failed to parse login response data: %w", err)
	}
	if loginResp.AuthorizationURL == "" {
		return nil, fmt.Errorf("no authorization URL in login response")
	}

	if err := verifyMobileConnectURL(loginResp.AuthorizationURL); err != nil {
		return nil, err
	}

	// Step 3: Follow MobileConnect redirects manually to collect cookies and corr value
	mobileConnectCookies := cookieMap{}
	currentURL := loginResp.AuthorizationURL
	var corr string

	for i := 0; i <= maxRedirects; i++ {
		resp, err = fetchWithCookies(ctx, client, "GET", currentURL, mobileConnectCookies, nil, nil, false)
		if err != nil {
			return nil, fmt.Errorf("failed during MobileConnect redirect: %w", err)
		}

		// Extract corr from JWT in URL
		u, _ := url.Parse(currentURL)
		if jwtString := u.Query().Get("jwt"); jwtString != "" {
			c, err := extractJWTCorr(jwtString)
			if err != nil {
				return nil, fmt.Errorf("failed to extract corr from JWT: %w", err)
			}
			if corr == "" {
				corr = c
			} else if c != corr {
				_ = resp.Body.Close()
				return nil, fmt.Errorf("mismatched corr value in JWT")
			}
		}

		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			break // Not a redirect, we've arrived
		}

		location := resp.Header.Get("Location")
		_ = resp.Body.Close()
		if location == "" {
			return nil, fmt.Errorf("redirect response missing Location header")
		}

		// Resolve relative URLs
		base, _ := url.Parse(currentURL)
		resolved, err := base.Parse(location)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve redirect URL: %w", err)
		}
		currentURL = resolved.String()

		if err := verifyMobileConnectURL(currentURL); err != nil {
			return nil, err
		}
	}

	if _, ok := mobileConnectCookies["connect.sid"]; !ok {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("failed to get connect.sid cookie from MobileConnect")
	}

	// Step 4: Parse the final HTML response for polling parameters and CSRF token
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read MobileConnect response: %w", err)
	}
	htmlStr := string(body)

	// Extract polling parameters
	pollingMatch := pollingRe.FindStringSubmatch(htmlStr)
	if pollingMatch == nil {
		return nil, fmt.Errorf("failed to extract polling parameters from MobileConnect response")
	}

	pollingURLPath := pollingMatch[1]
	paramsObj := parseJSObject(pollingMatch[2])
	pollingIntervalMs, err := strconv.Atoi(pollingMatch[3])
	if err != nil || pollingIntervalMs <= 0 {
		return nil, fmt.Errorf("invalid polling interval: %s", pollingMatch[4])
	}

	// Verify required parameters
	for _, key := range []string{"corr", "nonce", "trans"} {
		if paramsObj[key] == "" {
			return nil, fmt.Errorf("missing %s in polling parameters", key)
		}
	}

	if paramsObj["corr"] != corr {
		return nil, fmt.Errorf("mismatched corr in polling parameters")
	}

	// Build polling URL
	baseURL, _ := url.Parse(resp.Request.URL.String())
	pollingResolved, err := baseURL.Parse(pollingURLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve polling URL: %w", err)
	}
	q := pollingResolved.Query()
	q.Set("corr", paramsObj["corr"])
	q.Set("nonce", paramsObj["nonce"])
	q.Set("trans", paramsObj["trans"])
	pollingResolved.RawQuery = q.Encode()

	// Extract CSRF token and finish URL
	csrfMatch := csrfRe.FindStringSubmatch(htmlStr)
	if csrfMatch == nil {
		return nil, fmt.Errorf("failed to extract CSRF token from MobileConnect response")
	}

	finishURLPath := csrfMatch[1]
	csrfToken := csrfMatch[2]

	finishResolved, err := baseURL.Parse(finishURLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve finish URL: %w", err)
	}
	fq := finishResolved.Query()
	fq.Set("csrfmiddlewaretoken", csrfToken)
	fq.Set("corr", paramsObj["corr"])
	fq.Set("nonce", paramsObj["nonce"])
	fq.Set("trans", paramsObj["trans"])
	fq.Set("code", "")
	finishResolved.RawQuery = fq.Encode()

	return &loginState{
		micloudCookies:       micloudCookies,
		mobileConnectCookies: mobileConnectCookies,
		pollingURL:           pollingResolved.String(),
		pollingIntervalMs:    pollingIntervalMs,
		finishURL:            finishResolved.String(),
	}, nil
}

// pollForAuthentication polls the MobileConnect endpoint until the user
// clicks the SMS link. Returns when authentication is confirmed.
func pollForAuthentication(ctx context.Context, client *http.Client, state *loginState) error {
	interval := time.Duration(state.pollingIntervalMs) * time.Millisecond
	for i := 0; i < maxPollingAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		resp, err := fetchWithCookies(ctx, client, "GET", state.pollingURL, state.mobileConnectCookies, nil, nil, true)
		if err != nil {
			return fmt.Errorf("polling failed: %w", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != 204 {
			return nil // Authentication confirmed
		}
	}
	return fmt.Errorf("authentication timed out after %d polling attempts", maxPollingAttempts)
}

// finishAuthentication completes the MobileConnect flow and returns a session
func finishAuthentication(ctx context.Context, client *http.Client, state *loginState) (session *api.Session, err error) {
	initialJSessionID := state.micloudCookies["JSESSIONID"]
	if initialJSessionID == "" {
		return nil, fmt.Errorf("initial JSESSIONID missing")
	}

	// Hit the finish URL to get the code and state
	resp, err := fetchWithCookies(ctx, client, "GET", state.finishURL, state.mobileConnectCookies, nil, nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to finish authentication: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	respURL := resp.Request.URL
	code := respURL.Query().Get("code")
	stateParam := respURL.Query().Get("state")
	if code == "" || stateParam == "" {
		return nil, fmt.Errorf("missing code or state in authentication response URL")
	}

	// Validate with Movistar Cloud
	validateBody, _ := json.Marshal(map[string]interface{}{
		"data": map[string]string{
			"code":  code,
			"state": stateParam,
		},
	})

	validateResp, err := fetchWithCookies(ctx, client, "POST",
		"https://"+baseMvcDomain+"/sapi/credential/mobileconnect?action=validate",
		state.micloudCookies,
		strings.NewReader(string(validateBody)),
		map[string]string{"Content-Type": "application/json"},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to validate authentication: %w", err)
	}
	defer fs.CheckClose(validateResp.Body, &err)

	if validateResp.StatusCode != 200 {
		return nil, fmt.Errorf("validation failed with status %d", validateResp.StatusCode)
	}

	var validateResult struct {
		AccessToken     string `json:"access_token"`
		ExpiresIn       string `json:"expires_in"`
		MSISDN          string `json:"msisdn"`
		LastRefreshDate int64  `json:"lastrefreshdate"`
		Platform        string `json:"platform"`
	}
	var validateWrapper api.GetResponse
	if err := json.NewDecoder(validateResp.Body).Decode(&validateWrapper); err != nil {
		return nil, fmt.Errorf("failed to parse validation response: %w", err)
	}
	if err := json.Unmarshal(validateWrapper.Data, &validateResult); err != nil {
		return nil, fmt.Errorf("failed to parse validation response data: %w", err)
	}

	// Build the OAuth token for the login endpoint
	tokenData := map[string]interface{}{
		"data": map[string]interface{}{
			"accesstoken":     validateResult.AccessToken,
			"expiresin":       validateResult.ExpiresIn,
			"lastrefreshdate": validateResult.LastRefreshDate,
			"msisdn":          validateResult.MSISDN,
			"platform":        validateResult.Platform,
			"refreshtoken":    "",
		},
	}
	tokenJSON, _ := json.Marshal(tokenData)
	authToken := "oauth " + base64.StdEncoding.EncodeToString(tokenJSON)

	// Final login request
	loginResp, err := fetchWithCookies(ctx, client, "POST",
		"https://"+baseMvcDomain+"/sapi/login?action=login&responsetime=true",
		cookieMap{}, // fresh cookies
		nil,
		map[string]string{"Authorization": authToken},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to complete login: %w", err)
	}
	defer fs.CheckClose(loginResp.Body, &err)

	if loginResp.StatusCode != 200 {
		return nil, fmt.Errorf("login failed with status %d", loginResp.StatusCode)
	}

	var loginResult struct {
		AccessToken string `json:"access_token"`
		JSessionID  string `json:"jsessionid"`
	}
	var loginWrapper api.GetResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginWrapper); err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}
	if err := json.Unmarshal(loginWrapper.Data, &loginResult); err != nil {
		return nil, fmt.Errorf("failed to parse login response data: %w", err)
	}

	if loginResult.JSessionID == initialJSessionID {
		return nil, fmt.Errorf("JSESSIONID not updated after authentication, login may have failed")
	}

	return &api.Session{
		AccessToken: loginResult.AccessToken,
		JSessionID:  loginResult.JSessionID,
	}, nil
}

// refreshSession refreshes an existing session using the access token
func refreshSession(ctx context.Context, client *http.Client, session *api.Session) (newSession *api.Session, err error) {
	if session.AccessToken == "" {
		return nil, fmt.Errorf("cannot refresh session without an access token")
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://"+baseMvcDomain+"/sapi/login?action=login&responsetime=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "oauth "+session.AccessToken)
	req.Header.Set("Cookie", "JSESSIONID="+session.JSessionID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh session: %w", err)
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("session refresh failed with status %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		JSessionID  string `json:"jsessionid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	return &api.Session{
		AccessToken: result.AccessToken,
		JSessionID:  result.JSessionID,
	}, nil
}

// doLogin runs the full SMS-based login flow.
// It blocks while waiting for the user to click the SMS link.
func doLogin(ctx context.Context, phoneNumber string) (*api.Session, error) {
	client := fshttp.NewClient(ctx)

	state, err := initiateLogin(ctx, client, phoneNumber)
	if err != nil {
		return nil, err
	}

	fs.Logf(nil, "SMS sent to %s. Please click the link in the message to authenticate...", phoneNumber)

	if err := pollForAuthentication(ctx, client, state); err != nil {
		return nil, err
	}

	return finishAuthentication(ctx, client, state)
}

// saveSession persists the session to the rclone config
func saveSession(m configmap.Mapper, session *api.Session) {
	m.Set(configAccessToken, session.AccessToken)
	m.Set(configJSessionID, session.JSessionID)
}

// Config implements the multi-step interactive configuration for Movistar Cloud.
//
// State machine:
//
//	"" → read phone number from config, initiate SMS login, poll, finish, save session
func Config(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	switch configIn.State {
	case "":
		phoneNumber, _ := m.Get(configPhoneNumber)
		if phoneNumber == "" {
			return nil, fmt.Errorf("phone number not set - please set the phone_number option")
		}

		session, err := doLogin(ctx, phoneNumber)
		if err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}

		saveSession(m, session)
		fs.Logf(nil, "Authentication successful!")
		return nil, nil
	}
	return nil, fmt.Errorf("unknown state %q", configIn.State)
}
