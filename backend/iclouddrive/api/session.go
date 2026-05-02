package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

const iCloudUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3.1 Safari/605.1.15"

// Session represents an iCloud session
type Session struct {
	SessionToken   string         `json:"session_token"`
	Scnt           string         `json:"scnt"`
	SessionID      string         `json:"session_id"`
	AccountCountry string         `json:"account_country"`
	TrustToken     string         `json:"trust_token"`
	ClientID       string         `json:"client_id"`
	AuthAttributes string         `json:"auth_attributes"`
	FrameID        string         `json:"frame_id"`
	Cookies        []*http.Cookie `json:"cookies"`
	AccountInfo    AccountInfo    `json:"account_info"`

	mu       sync.Mutex   `json:"-"` // protects session fields during concurrent Request calls
	srv      *rest.Client `json:"-"`
	needs2FA bool         `json:"-"` // set when SRP signin returns 409
}

// srpInitResponse is the server response from /auth/signin/init
type srpInitResponse struct {
	Iteration int    `json:"iteration"`
	Salt      string `json:"salt"`
	Protocol  string `json:"protocol"`
	B         string `json:"b"`
	C         string `json:"c"`
}

func cookieValueFingerprint(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:6])
}

func authStateBodySummary(body []byte) string {
	summary := []string{fmt.Sprintf("bytes=%d", len(body)), "hash=" + cookieValueFingerprint(string(body))}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return strings.Join(summary, " ")
	}
	keys := make([]string, 0, len(top))
	for key := range top {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	summary = append(summary, fmt.Sprintf("keys=%v", keys))
	if _, ok := top["phoneNumberVerification"]; ok {
		summary = append(summary, "wrapped=true")
	}
	return strings.Join(summary, " ")
}

func cookieDebugSummary(c *http.Cookie) string {
	parts := []string{c.Name}
	if c.Value == "" {
		parts = append(parts, "empty", "len=0")
	} else {
		parts = append(parts, "set", fmt.Sprintf("len=%d", len(c.Value)), "hash="+cookieValueFingerprint(c.Value))
	}
	if c.Path != "" {
		parts = append(parts, "path="+c.Path)
	}
	if c.Domain != "" {
		parts = append(parts, "domain="+c.Domain)
	}
	if c.MaxAge != 0 {
		parts = append(parts, fmt.Sprintf("maxAge=%d", c.MaxAge))
	}
	if !c.Expires.IsZero() {
		parts = append(parts, "expires")
	}
	return strings.Join(parts, ",")
}

func cookieDebugSummaries(cookies []*http.Cookie) []string {
	if len(cookies) == 0 {
		return nil
	}
	out := make([]string, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, cookieDebugSummary(c))
	}
	return out
}

func cookieJarDebugSummaries(cookies []*http.Cookie) []string {
	if len(cookies) == 0 {
		return nil
	}
	out := make([]string, 0, len(cookies))
	for _, c := range cookies {
		summary := c.Name
		if c.Value == "" {
			summary += ",empty,len=0"
		} else {
			summary += fmt.Sprintf(",len=%d,hash=%s", len(c.Value), cookieValueFingerprint(c.Value))
		}
		out = append(out, summary)
	}
	return out
}

func (s *Session) mergeCookies(cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	existing := make(map[string]int, len(s.Cookies))
	for i, c := range s.Cookies {
		existing[c.Name] = i
	}
	for _, c := range cookies {
		if c.Value == "" {
			if idx, ok := existing[c.Name]; ok {
				fs.Debugf(nil, "iclouddrive: deleting empty auth cookie: %s", cookieDebugSummary(c))
				s.Cookies = append(s.Cookies[:idx], s.Cookies[idx+1:]...)
				delete(existing, c.Name)
				for j := idx; j < len(s.Cookies); j++ {
					existing[s.Cookies[j].Name] = j
				}
			} else {
				fs.Debugf(nil, "iclouddrive: ignoring empty auth cookie tombstone for missing cookie: %s", cookieDebugSummary(c))
			}
			continue
		}
		if idx, ok := existing[c.Name]; ok {
			s.Cookies[idx] = c
		} else {
			s.Cookies = append(s.Cookies, c)
			existing[c.Name] = len(s.Cookies) - 1
		}
	}
}

// extractHeaders reads Apple session headers from an HTTP response into the session
// Does NOT acquire s.mu - caller is responsible for locking if needed
func (s *Session) extractHeaders(resp *http.Response) {
	s.mergeCookies(resp.Cookies())
	if val := resp.Header.Get("X-Apple-ID-Account-Country"); val != "" {
		s.AccountCountry = val
	}
	if val := resp.Header.Get("X-Apple-ID-Session-Id"); val != "" {
		s.SessionID = val
	}
	if val := resp.Header.Get("X-Apple-Session-Token"); val != "" {
		s.SessionToken = val
	}
	if val := resp.Header.Get("X-Apple-TwoSV-Trust-Token"); val != "" {
		s.TrustToken = val
	}
	if val := resp.Header.Get("scnt"); val != "" {
		s.Scnt = val
	}
	if val := resp.Header.Get("X-Apple-Auth-Attributes"); val != "" {
		s.AuthAttributes = val
	}
}

// Request makes a JSON API request and extracts session headers
func (s *Session) Request(ctx context.Context, opts rest.Opts, request any, response any) (*http.Response, error) {
	resp, err := s.srv.CallJSON(ctx, &opts, &request, &response)
	if err != nil {
		return resp, err
	}
	s.mu.Lock()
	s.extractHeaders(resp)
	s.mu.Unlock()
	return resp, nil
}

// Requires2FA returns true if the session requires 2FA
func (s *Session) Requires2FA() bool {
	if s.needs2FA {
		return true
	}
	return s.AccountInfo.DsInfo != nil && s.AccountInfo.DsInfo.HsaVersion == 2 && s.AccountInfo.HsaChallengeRequired
}

// SignIn performs SRP-based authentication against Apple's idmsa endpoint
func (s *Session) SignIn(ctx context.Context, appleID, password string) error {
	// Step 1: Initialize the auth session
	if err := s.authStart(ctx); err != nil {
		return fmt.Errorf("authStart: %w", err)
	}

	// Step 2: Federate (submit account name)
	if err := s.authFederate(ctx, appleID); err != nil {
		return fmt.Errorf("authFederate: %w", err)
	}

	// Step 3: SRP init - send client public value A, get salt + B
	client, err := newSRPClient()
	if err != nil {
		return fmt.Errorf("newSRPClient: %w", err)
	}
	aBase64 := base64.StdEncoding.EncodeToString(client.getABytes())

	initResp, err := s.authSRPInit(ctx, aBase64, appleID)
	if err != nil {
		return fmt.Errorf("authSRPInit: %w", err)
	}

	// Decode server values
	serverB, err := base64.StdEncoding.DecodeString(initResp.B)
	if err != nil {
		return fmt.Errorf("decode B: %w", err)
	}
	salt, err := base64.StdEncoding.DecodeString(initResp.Salt)
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}

	// Step 4: Derive password key and process the SRP challenge
	derivedKey, err := derivePassword(password, salt, initResp.Iteration, initResp.Protocol)
	if err != nil {
		return fmt.Errorf("derivePassword: %w", err)
	}
	if err := client.processChallenge([]byte(appleID), derivedKey, salt, serverB); err != nil {
		return fmt.Errorf("processChallenge: %w", err)
	}

	// Step 5: Complete - send M1, M2 proofs
	m1Base64 := base64.StdEncoding.EncodeToString(client.M1)
	m2Base64 := base64.StdEncoding.EncodeToString(client.M2)

	if err := s.authSRPComplete(ctx, appleID, m1Base64, m2Base64, initResp.C); err != nil {
		return fmt.Errorf("authSRPComplete: %w", err)
	}

	return nil
}

// authStart initializes the SRP auth session by hitting the authorize/signin endpoint
func (s *Session) authStart(ctx context.Context) error {
	if s.FrameID == "" {
		s.FrameID = strings.ToLower(uuid.New().String())
	}
	frameTag := "auth-" + s.FrameID

	params := url.Values{}
	params.Set("frame_id", frameTag)
	params.Set("language", "en_US")
	params.Set("skVersion", "7")
	params.Set("iframeId", frameTag)
	params.Set("client_id", s.ClientID)
	params.Set("redirect_uri", "https://www.icloud.com")
	params.Set("response_type", "code")
	params.Set("response_mode", "web_message")
	params.Set("state", frameTag)
	params.Set("authVersion", "latest")

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/authorize/signin",
		Parameters: params,
		ExtraHeaders: map[string]string{
			"Accept":     "*/*",
			"User-Agent": iCloudUserAgent,
		},
		RootURL:    authEndpoint,
		NoResponse: true,
	}

	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authStart: unexpected status %s", resp.Status)
	}

	s.extractHeaders(resp)
	return nil
}

// authFederate submits the account name to Apple's federate endpoint
func (s *Session) authFederate(ctx context.Context, accountName string) error {
	values := map[string]any{
		"accountName": accountName,
		"rememberMe":  true,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/federate",
		Parameters:   url.Values{"isRememberMeEnabled": {"true"}},
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		Body:         body,
		NoResponse:   true,
	}

	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	s.extractHeaders(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authFederate: unexpected status %s", resp.Status)
	}
	return nil
}

// authSRPInit sends the client's public value A to the server and retrieves
// the salt, server public value B, iteration count, protocol, and challenge
func (s *Session) authSRPInit(ctx context.Context, aBase64, accountName string) (*srpInitResponse, error) {
	values := map[string]any{
		"a":           aBase64,
		"accountName": accountName,
		"protocols":   []string{"s2k", "s2k_fo"},
	}
	body, err := IntoReader(values)
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/signin/init",
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		Body:         body,
	}

	var initResp srpInitResponse
	resp, err := s.srv.CallJSON(ctx, &opts, nil, &initResp)
	if err != nil {
		return nil, err
	}

	s.extractHeaders(resp)
	return &initResp, nil
}

// authSRPComplete sends the SRP proofs M1 and M2 to complete authentication
// Returns nil on success (200 or 409/2FA needed)
func (s *Session) authSRPComplete(ctx context.Context, accountName, m1Base64, m2Base64, c string) error {
	trustTokens := []string{}
	if s.TrustToken != "" {
		trustTokens = []string{s.TrustToken}
	}

	values := map[string]any{
		"accountName": accountName,
		"m1":          m1Base64,
		"m2":          m2Base64,
		"c":           c,
		"rememberMe":  true,
		"trustTokens": trustTokens,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/signin/complete",
		Parameters:   url.Values{"isRememberMeEnabled": {"true"}},
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		IgnoreStatus: true,
		Body:         body,
	}

	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return err
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	s.extractHeaders(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		fs.Debugf(nil, "iclouddrive: SRP sign in successful")
		return nil
	case http.StatusConflict:
		// 409 = 2FA required
		fs.Debugf(nil, "iclouddrive: SRP sign in requires 2FA, response: %s", respBody)
		s.needs2FA = true
		return nil
	case http.StatusPreconditionFailed:
		// 412 = non-2FA account needs repair/complete step
		fs.Debugf(nil, "iclouddrive: SRP sign in returned 412, attempting repair/complete")
		return s.authRepairComplete(ctx)
	case http.StatusForbidden:
		return fmt.Errorf("sign in failed: incorrect username or password")
	default:
		return fmt.Errorf("sign in failed: %s: %s", resp.Status, respBody)
	}
}

// authRepairComplete handles the repair flow for non-2FA accounts that return 412
func (s *Session) authRepairComplete(ctx context.Context) error {
	body, err := IntoReader(map[string]any{})
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/repair/complete",
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		IgnoreStatus: true,
		NoResponse:   true,
		Body:         body,
	}
	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return fmt.Errorf("repair/complete failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("repair/complete returned %s", resp.Status)
	}
	s.extractHeaders(resp)
	fs.Debugf(nil, "iclouddrive: repair/complete successful")
	return nil
}

// getAuthOrigin returns the origin URL for auth requests
// Supports both global (idmsa.apple.com) and China (idmsa.apple.com.cn) endpoints
func getAuthOrigin() string {
	return strings.TrimSuffix(authEndpoint, "/appleauth/auth")
}

// getSRPAuthHeaders returns headers needed for SRP auth requests
func (s *Session) getSRPAuthHeaders() map[string]string {
	frameTag := "auth-" + s.FrameID
	authOrigin := getAuthOrigin()
	headers := map[string]string{
		"Accept":                           "application/json",
		"Content-Type":                     "application/json",
		"User-Agent":                       iCloudUserAgent,
		"Origin":                           authOrigin,
		"Referer":                          authOrigin + "/",
		"X-Apple-Widget-Key":               s.ClientID,
		"X-Apple-OAuth-Client-Id":          s.ClientID,
		"X-Apple-OAuth-Client-Type":        "firstPartyAuth",
		"X-Apple-OAuth-Redirect-URI":       "https://www.icloud.com",
		"X-Apple-OAuth-Require-Grant-Code": "true",
		"X-Apple-OAuth-Response-Mode":      "web_message",
		"X-Apple-OAuth-Response-Type":      "code",
		"X-Apple-OAuth-State":              frameTag,
		"X-Apple-Frame-Id":                 frameTag,
		"X-Requested-With":                 "XMLHttpRequest",
		"X-Apple-Mandate-Security-Upgrade": "0",
		"X-Apple-I-Require-UE":             "true",
		"X-Apple-I-FD-Client-Info":         `{"U":"` + iCloudUserAgent + `","L":"en-US","Z":"GMT-05:00","V":"1.1","F":""}`,
	}
	if s.AuthAttributes != "" {
		headers["X-Apple-Auth-Attributes"] = s.AuthAttributes
	}
	if s.Scnt != "" {
		headers["scnt"] = s.Scnt
	}
	if s.SessionID != "" {
		headers["X-Apple-ID-Session-Id"] = s.SessionID
	}
	return headers
}

// AuthWithToken authenticates the session
func (s *Session) AuthWithToken(ctx context.Context) error {
	values := map[string]any{
		"accountCountryCode": s.AccountCountry,
		"dsWebAuthToken":     s.SessionToken,
		"extended_login":     true,
		"trustToken":         s.TrustToken,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/accountLogin",
		ExtraHeaders: GetCommonHeaders(map[string]string{}),
		RootURL:      setupEndpoint,
		Body:         body,
	}

	resp, err := s.Request(ctx, opts, nil, &s.AccountInfo)
	if err != nil {
		return err
	}
	fs.Debugf(nil, "iclouddrive: accountLogin response cookies: %v", cookieDebugSummaries(resp.Cookies()))
	fs.Debugf(nil, "iclouddrive: session cookie jar after accountLogin: %v", cookieJarDebugSummaries(s.Cookies))

	// Acquire PCS cookies if Advanced Data Protection is enabled
	if ws := s.AccountInfo.Webservices["ckdatabasews"]; ws != nil && ws.PcsRequired {
		fs.Debugf(nil, "iclouddrive: ADP detected (pcsRequired=true)")
		if s.hasPCSCookies() {
			fs.Debugf(nil, "iclouddrive: PCS cookies already present, skipping acquisition")
		} else {
			if err := s.acquirePCSCookies(ctx); err != nil {
				return err
			}
		}
	} else {
		fs.Debugf(nil, "iclouddrive: no ADP (pcsRequired=false)")
	}

	return nil
}

// hasPCSCookies checks if the required PCS cookies for Photos are already present
func (s *Session) hasPCSCookies() bool {
	var hasPhotos, hasSharing bool
	for _, c := range s.Cookies {
		switch c.Name {
		case "X-APPLE-WEBAUTH-PCS-Photos":
			hasPhotos = true
		case "X-APPLE-WEBAUTH-PCS-Sharing":
			hasSharing = true
		}
	}
	return hasPhotos && hasSharing
}

// acquirePCSCookies requests PCS cookies for ADP-enabled accounts
// May require user approval on a trusted device (polls every 10s, max 5 min)
func (s *Session) acquirePCSCookies(ctx context.Context) error {
	fs.Logf(nil, "iclouddrive: Advanced Data Protection enabled, requesting PCS cookies")
	const maxAttempts = 30 // 30 * 10s = 5 minutes max
	for attempt := 0; attempt < maxAttempts; attempt++ {
		fs.Debugf(nil, "iclouddrive: requestPCS outgoing cookies: %v", cookieJarDebugSummaries(s.Cookies))
		values := map[string]any{
			"appName":               "photos",
			"derivedFromUserAction": true,
		}
		body, err := IntoReader(values)
		if err != nil {
			return fmt.Errorf("requestPCS: %w", err)
		}
		opts := rest.Opts{
			Method:       "POST",
			Path:         "/requestPCS",
			ExtraHeaders: s.GetHeaders(map[string]string{}),
			RootURL:      setupEndpoint,
		}
		opts.Body = body
		var pcsResp struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		resp, err := s.Request(ctx, opts, nil, &pcsResp)
		if err != nil {
			return fmt.Errorf("requestPCS: %w", err)
		}
		fs.Debugf(nil, "iclouddrive: requestPCS response cookies: %v", cookieDebugSummaries(resp.Cookies()))
		fs.Debugf(nil, "iclouddrive: requestPCS response: status=%q message=%q cookies=%d",
			pcsResp.Status, pcsResp.Message, len(resp.Cookies()))
		if pcsResp.Status == "success" {
			if !s.hasPCSCookies() {
				return fmt.Errorf("requestPCS: server returned success but PCS cookies missing")
			}
			fs.Logf(nil, "iclouddrive: PCS cookies acquired")
			return nil
		}
		// Device consent required - poll until approved
		fs.Logf(nil, "iclouddrive: waiting for device approval for PCS (%s)", pcsResp.Message)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return fmt.Errorf("requestPCS: timed out waiting for device approval after 5 minutes")
}

// RequestPushNotification explicitly requests a push notification to trusted devices
// Required for iOS 26.4+ where the SRP 409 response no longer auto-pushes
func (s *Session) RequestPushNotification(ctx context.Context) error {
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/verify/trusteddevice/securitycode",
		ExtraHeaders: s.GetAuthHeaders(map[string]string{}),
		RootURL:      authEndpoint,
		NoResponse:   true,
	}

	_, err := s.Request(ctx, opts, nil, nil)
	return err
}

// Validate2FACode validates the 2FA code from a trusted device push notification
func (s *Session) Validate2FACode(ctx context.Context, code string) error {
	values := map[string]any{"securityCode": map[string]string{"code": code}}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/verify/trusteddevice/securitycode",
		ExtraHeaders: s.GetAuthHeaders(map[string]string{}),
		RootURL:      authEndpoint,
		Body:         body,
		NoResponse:   true,
	}

	_, err = s.Request(ctx, opts, nil, nil)
	if err == nil {
		if err := s.TrustSession(ctx); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("validate2FACode failed: %w", err)
}

// TrustedPhoneNumber represents a phone number that can receive SMS verification codes
type TrustedPhoneNumber struct {
	ID                 int    `json:"id"`
	NumberWithDialCode string `json:"numberWithDialCode"`
	ObfuscatedNumber   string `json:"obfuscatedNumber"`
	PushMode           string `json:"pushMode"`
	NonFTEU            bool   `json:"nonFTEU"`
}

// AuthStateResponse is the response from GET /appleauth/auth after sign-in
// Some accounts return fields at top level, others nest them under phoneNumberVerification
type AuthStateResponse struct {
	TrustedPhoneNumbers     []TrustedPhoneNumber `json:"trustedPhoneNumbers"`
	TrustedPhoneNumber      *TrustedPhoneNumber  `json:"trustedPhoneNumber"`
	NoTrustedDevices        bool                 `json:"noTrustedDevices"`
	AuthenticationType      string               `json:"authenticationType"`
	Hsa2Account             bool                 `json:"hsa2Account"`
	PhoneNumberVerification *AuthStateResponse   `json:"phoneNumberVerification"`
}

// GetAuthState retrieves the current auth state including trusted phone numbers for SMS 2FA
func (s *Session) GetAuthState(ctx context.Context) (*AuthStateResponse, error) {
	opts := rest.Opts{
		Method:        "GET",
		Path:          "",
		ExtraHeaders:  s.GetAuthHeaders(map[string]string{}),
		RootURL:       authEndpoint,
		ContentLength: int64Ptr(0),
	}
	// Use srv.Call directly to capture the raw response body for debugging
	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return nil, fmt.Errorf("getAuthState: %w", err)
	}
	s.mu.Lock()
	s.extractHeaders(resp)
	s.mu.Unlock()
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("getAuthState read: %w", readErr)
	}
	fs.Debugf(nil, "iclouddrive: auth state response summary: %s", authStateBodySummary(body))
	var state AuthStateResponse
	if err := json.Unmarshal(body, &state); err != nil {
		fs.Debugf(nil, "iclouddrive: auth state parse failed: %s", authStateBodySummary(body))
		return nil, fmt.Errorf("getAuthState unmarshal: %w", err)
	}
	// Some accounts nest auth data under phoneNumberVerification
	if state.PhoneNumberVerification != nil && state.AuthenticationType == "" {
		fs.Debugf(nil, "iclouddrive: unwrapping phoneNumberVerification envelope")
		n := state.PhoneNumberVerification
		state.TrustedPhoneNumbers = n.TrustedPhoneNumbers
		state.TrustedPhoneNumber = n.TrustedPhoneNumber
		state.NoTrustedDevices = n.NoTrustedDevices
		state.AuthenticationType = n.AuthenticationType
		state.Hsa2Account = n.Hsa2Account
		state.PhoneNumberVerification = nil
	}
	fs.Debugf(nil, "iclouddrive: auth state: type=%s hsa2=%v noTrustedDevices=%v phones=%d phoneSingular=%v",
		state.AuthenticationType, state.Hsa2Account, state.NoTrustedDevices,
		len(state.TrustedPhoneNumbers), state.TrustedPhoneNumber != nil)
	// Fall back to singular trustedPhoneNumber when plural array is empty
	// Some accounts return only the singular form (SMS-only, no trusted devices)
	if len(state.TrustedPhoneNumbers) == 0 && state.TrustedPhoneNumber != nil {
		fs.Debugf(nil, "iclouddrive: using singular trustedPhoneNumber (id=%d) as fallback",
			state.TrustedPhoneNumber.ID)
		state.TrustedPhoneNumbers = []TrustedPhoneNumber{*state.TrustedPhoneNumber}
	}
	return &state, nil
}

// RequestSMSCode triggers SMS code delivery to a trusted phone number
func (s *Session) RequestSMSCode(ctx context.Context, phoneID int, mode string) error {
	values := map[string]any{
		"phoneNumber": map[string]any{"id": phoneID},
		"mode":        mode,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:       "PUT",
		Path:         "/verify/phone",
		ExtraHeaders: s.GetAuthHeaders(map[string]string{}),
		RootURL:      authEndpoint,
		Body:         body,
		NoResponse:   true,
	}
	_, err = s.Request(ctx, opts, nil, nil)
	if err != nil {
		return fmt.Errorf("requestSMSCode: %w", err)
	}
	return nil
}

// ValidateSMSCode validates a 2FA code received via SMS
func (s *Session) ValidateSMSCode(ctx context.Context, code string, phoneID int, mode string) error {
	values := map[string]any{
		"securityCode": map[string]string{"code": code},
		"phoneNumber":  map[string]any{"id": phoneID},
		"mode":         mode,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/verify/phone/securitycode",
		ExtraHeaders: s.GetAuthHeaders(map[string]string{}),
		RootURL:      authEndpoint,
		Body:         body,
		NoResponse:   true,
	}
	_, err = s.Request(ctx, opts, nil, nil)
	if err == nil {
		if err := s.TrustSession(ctx); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("validateSMSCode: %w", err)
}

// TrustSession trusts the session
func (s *Session) TrustSession(ctx context.Context) error {
	opts := rest.Opts{
		Method:        "GET",
		Path:          "/2sv/trust",
		ExtraHeaders:  s.GetAuthHeaders(map[string]string{}),
		RootURL:       authEndpoint,
		NoResponse:    true,
		ContentLength: int64Ptr(0),
	}

	_, err := s.Request(ctx, opts, nil, nil)
	if err != nil {
		return fmt.Errorf("trustSession failed: %w", err)
	}

	return s.AuthWithToken(ctx)
}

// ValidateSession validates the session
func (s *Session) ValidateSession(ctx context.Context) error {
	opts := rest.Opts{
		Method:        "POST",
		Path:          "/validate",
		ExtraHeaders:  s.GetHeaders(map[string]string{}),
		RootURL:       setupEndpoint,
		ContentLength: int64Ptr(0),
	}
	_, err := s.Request(ctx, opts, nil, &s.AccountInfo)
	if err != nil {
		return fmt.Errorf("validateSession failed: %w", err)
	}

	s.needs2FA = false
	return nil
}

// GetAuthHeaders returns the authentication headers for the session
// Used for 2FA validation and trust requests to idmsa.apple.com
func (s *Session) GetAuthHeaders(overwrite map[string]string) map[string]string {
	headers := s.getSRPAuthHeaders()
	maps.Copy(headers, overwrite)
	return headers
}

// GetHeaders returns the authentication headers required for a request
func (s *Session) GetHeaders(overwrite map[string]string) map[string]string {
	headers := GetCommonHeaders(map[string]string{})
	headers["Cookie"] = s.GetCookieString()
	maps.Copy(headers, overwrite)
	return headers
}

// GetCookieString returns the cookie header string for the session
func (s *Session) GetCookieString() string {
	var b strings.Builder
	first := true
	for _, cookie := range s.Cookies {
		if cookie.Value == "" {
			continue
		}
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(cookie.Name)
		b.WriteByte('=')
		b.WriteString(cookie.Value)
	}
	return b.String()
}

// GetCommonHeaders generates common HTTP headers with optional overwrite
func GetCommonHeaders(overwrite map[string]string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Origin":       baseEndpoint,
		"Referer":      fmt.Sprintf("%s/", baseEndpoint),
		"User-Agent":   iCloudUserAgent,
	}
	maps.Copy(headers, overwrite)
	return headers
}

func int64Ptr(v int64) *int64 { return &v }

// NewSession creates a new Session instance with default values
func NewSession() *Session {
	session := &Session{
		FrameID: strings.ToLower(uuid.New().String()),
	}
	httpClient := fshttp.NewClient(context.Background())
	if tr, ok := httpClient.Transport.(*fshttp.Transport); ok {
		tr.SetRequestFilter(func(req *http.Request) {
			req.Header.Set("User-Agent", iCloudUserAgent)
		})
	}
	session.srv = rest.NewClient(httpClient).SetRoot(baseEndpoint)
	return session
}

// AccountInfo represents the subset of Apple's account response we actually use
// json.Unmarshal silently ignores extra fields in the response
type AccountInfo struct {
	DsInfo               *dsInfo                `json:"dsInfo"`
	Webservices          map[string]*webService `json:"webservices"`
	HsaChallengeRequired bool                   `json:"hsaChallengeRequired"`
}

// dsInfo holds the account metadata fields we read
type dsInfo struct {
	HsaVersion int `json:"hsaVersion"`
}

// webService represents a web service
type webService struct {
	PcsRequired bool   `json:"pcsRequired"`
	URL         string `json:"url"`
	UploadURL   string `json:"uploadUrl"`
	Status      string `json:"status"`
}
