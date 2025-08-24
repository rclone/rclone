// Package api handles the client-side interactions with Apple's iCloud APIs.
// This file specifically adds the Private Cloud Storage (PCS) consent flow,
// which is a necessary step when Advanced Data Protection (ADP) is enabled
// on the target Apple ID.
package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

const (
	// icloudWebBase is the base URL for iCloud's web interface.
	icloudWebBase = "https://www.icloud.com"

	// setupWSBase is the base URL for iCloud's setup web services API.
	setupWSBase = "https://setup.icloud.com/setup/ws/1"

	// pcsConsentTimeout is the maximum time to wait for the entire PCS consent process.
	pcsConsentTimeout = 5 * time.Minute

	// pcsPollingInterval is the interval at which to poll for consent status updates.
	pcsPollingInterval = 5 * time.Second
)

// WebAccessState represents the iCloud Advanced Data Protection status for an account.
type WebAccessState struct {
	// IsICDRSDisabled is true when Advanced Data Protection (ADP) is enabled.
	IsICDRSDisabled bool `json:"isICDRSDisabled"`
	// IsDeviceConsentedForPCS is true if the trusted device has already consented to PCS access.
	IsDeviceConsentedForPCS bool `json:"isDeviceConsentedForPCS"`
}

// PCSResponse represents the response from a requestPCS API call.
type PCSResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// EnableConsentResponse represents the response from enabling device consent.
// Apple's API can return an empty object on success, so we treat a 200 OK
// response as success unless an explicit error is provided.
type EnableConsentResponse struct {
	IsDeviceConsentNotificationSent bool   `json:"isDeviceConsentNotificationSent"`
	Success                         bool   `json:"success"`
	Error                           string `json:"error"`
	Message                         string `json:"message"`
}

// ----------------------------------------------------------------------------
// # Section: Cookie and Session Utilities
// ----------------------------------------------------------------------------

// WebAuthToken returns the value of the X-APPLE-WEBAUTH-TOKEN cookie from the session.
func (c *Client) WebAuthToken() string {
	return c.CookieValue("X-APPLE-WEBAUTH-TOKEN")
}

// WebAuthUser returns the value of the X-APPLE-WEBAUTH-USER cookie from the session.
// It falls back to the client's Apple ID if the cookie is not present.
func (c *Client) WebAuthUser() string {
	if v := c.CookieValue("X-APPLE-WEBAUTH-USER"); v != "" {
		return v
	}
	return c.appleID
}

// CookieValue retrieves the value of a cookie by its name from the client session.
// It returns an empty string if the cookie is not found.
func (c *Client) CookieValue(name string) string {
	if c.Session == nil || c.Session.Cookies == nil {
		return ""
	}
	for _, ck := range c.Session.Cookies {
		if ck != nil && ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

// AddOrReplaceCookie adds a new cookie or replaces an existing one in the client session.
func (c *Client) AddOrReplaceCookie(ck *http.Cookie) {
	if ck == nil || ck.Name == "" || c.Session == nil {
		return
	}
	if c.Session.Cookies == nil {
		c.Session.Cookies = []*http.Cookie{ck}
		return
	}
	for i, existing := range c.Session.Cookies {
		if existing != nil && existing.Name == ck.Name {
			c.Session.Cookies[i] = ck
			return
		}
	}
	c.Session.Cookies = append(c.Session.Cookies, ck)
}

// CookieHeaderFor builds a string of cookies suitable for a `Cookie` HTTP header,
// tailored for requests to the specified URL root.
func (c *Client) CookieHeaderFor(root string) string {
	u, _ := url.Parse(root)
	domainCookies, _ := GetCookiesForDomain(u, c.Session.Cookies)

	var b strings.Builder
	for _, ck := range domainCookies {
		if ck != nil && ck.Name != "" && ck.Value != "" {
			fmt.Fprintf(&b, "%s=%s; ", ck.Name, ck.Value)
		}
	}

	// For setup.icloud.com, Apple's API requires certain authentication
	// details to be passed directly in the Cookie header.
	if strings.Contains(u.Host, "setup.icloud.com") {
		if tok := c.WebAuthToken(); tok != "" {
			fmt.Fprintf(&b, "X-APPLE-WEBAUTH-TOKEN=%s; ", tok)
		}
		if usr := c.WebAuthUser(); usr != "" {
			fmt.Fprintf(&b, "X-APPLE-WEBAUTH-USER=%s; ", usr)
		}
		if hsa := c.CookieValue("X-APPLE-WEBAUTH-HSA-LOGIN"); hsa != "" {
			fmt.Fprintf(&b, "X-APPLE-WEBAUTH-HSA-LOGIN=%s; ", hsa)
		}
	}

	return strings.TrimRight(b.String(), "; ")
}

// ----------------------------------------------------------------------------
// # Section: iCloud Setup API Helpers
// ----------------------------------------------------------------------------

// FetchWebBuildNumber retrieves the current build number from the iCloud web interface.
// This is a best-effort call; it returns an empty string on failure.
func (c *Client) FetchWebBuildNumber(ctx context.Context) string {
	req := &rest.Opts{Method: "GET", RootURL: icloudWebBase, Path: "/"}
	resp, err := c.srv.Call(ctx, req)
	if err != nil {
		fs.Debugf("icloud", "Failed to fetch web build number: %v", err)
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fs.Debugf("icloud", "Failed to read response body for build number: %v", err)
		return ""
	}

	re := regexp.MustCompile(`BUILD_INFO\s*=\s*\{[^}]*buildNumber\s*:\s*"?([^"',}]+)"?`)
	if m := re.FindStringSubmatch(string(body)); len(m) >= 2 && m[1] != "" {
		c.webBuildNumber = m[1]
		fs.Debugf("icloud", "Discovered iCloud web build number: %s", c.webBuildNumber)
	}
	if c.webBuildNumber == "" {
		fs.Debugf("icloud", "Could not extract build number from iCloud homepage (status %d)", resp.StatusCode)
	}
	return c.webBuildNumber
}

// DefaultSetupParams constructs a set of default URL query parameters required
// for most `setup.icloud.com` API calls.
func (c *Client) DefaultSetupParams(ctx context.Context) url.Values {
	p := url.Values{}
	if c.webBuildNumber == "" {
		c.webBuildNumber = c.FetchWebBuildNumber(ctx)
	}
	if c.webBuildNumber != "" {
		p.Set("clientBuildNumber", c.webBuildNumber)
	}
	if c.Session != nil {
		if c.Session.ClientID != "" {
			p.Set("clientId", c.Session.ClientID)
		}
		if dsInfo := c.Session.AccountInfo.DsInfo; dsInfo != nil && dsInfo.Dsid != "" {
			p.Set("dsid", dsInfo.Dsid)
		}
	}
	return p
}

// SetupHeaders builds the common HTTP headers required for `setup.icloud.com` API calls.
func (c *Client) SetupHeaders() (map[string]string, error) {
	// Best-effort attempt to ensure the web auth token is fresh.
	if c.WebAuthToken() == "" {
		_ = c.RefreshWebAuth(context.Background())
	}

	h := GetCommonHeaders(map[string]string{
		"Referer":          icloudWebBase + "/iclouddrive/",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           icloudWebBase,
	})
	if c.Session != nil {
		if c.Session.SessionID != "" {
			h["X-Apple-ID-Session-Id"] = c.Session.SessionID
		}
		if c.Session.Scnt != "" {
			h["scnt"] = c.Session.Scnt
		}
	}
	h["Cookie"] = c.CookieHeaderFor(setupWSBase)
	return h, nil
}

// RefreshWebAuth attempts to refresh the X-APPLE-WEBAUTH-TOKEN by calling the
// `refreshWebAuth` endpoint. This is often needed to keep the session alive.
func (c *Client) RefreshWebAuth(ctx context.Context) error {
	fs.Debugf("icloud", "[PCS] Refreshing web authentication token")
	opts := rest.Opts{
		Method:       "POST",
		RootURL:      setupWSBase,
		Path:         "/refreshWebAuth",
		ExtraHeaders: map[string]string{"Origin": icloudWebBase, "Referer": icloudWebBase + "/"},
	}
	resp, err := c.RequestNoReAuth(ctx, opts, nil, nil)
	if err != nil {
		fs.Debugf("icloud", "[PCS] refreshWebAuth call failed: %v", err)
		return fmt.Errorf("refresh web auth failed: %w", err)
	}
	for _, ck := range resp.Cookies() {
		if ck != nil && ck.Name == "X-APPLE-WEBAUTH-TOKEN" && ck.Value != "" {
			c.AddOrReplaceCookie(ck)
			fs.Debugf("icloud", "[PCS] Successfully refreshed web auth token.")
		}
	}
	return nil
}

// BootstrapWebCookies "warms up" the session by visiting key iCloud web pages
// to ensure all necessary session cookies are set.
func (c *Client) BootstrapWebCookies(ctx context.Context) {
	fs.Debugf("icloud", "[PCS] Bootstrapping web cookies by visiting iCloud pages")
	commonHeaders := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
	}

	// Visit the root page
	if resp, err := c.srv.Call(ctx, &rest.Opts{
		Method: "GET", RootURL: icloudWebBase, Path: "/", ExtraHeaders: commonHeaders,
	}); err == nil {
		c.Session.CaptureCookies(resp)
	}

	// Visit the iCloud Drive page
	if resp, err := c.srv.Call(ctx, &rest.Opts{
		Method: "GET", RootURL: icloudWebBase, Path: "/iclouddrive/", ExtraHeaders: commonHeaders,
	}); err == nil {
		c.Session.CaptureCookies(resp)
	}
}

// SetupCallJSON executes a request to a `setup.icloud.com` endpoint,
// automatically handling JSON marshalling and unmarshalling. It includes retry
// logic for HTTP 421 (Misdirected Request) errors, which often indicate a stale
// session that can be fixed by refreshing cookies.
func (c *Client) SetupCallJSON(ctx context.Context, opts rest.Opts, req, out any) (*http.Response, error) {
	h, err := c.SetupHeaders()
	if err != nil {
		return nil, fmt.Errorf("failed to build setup headers: %w", err)
	}
	opts.ExtraHeaders = h

	resp, err := c.RequestNoReAuth(ctx, opts, req, out)

	// A 421 status suggests our session/cookies are out of date.
	// We attempt to fix this by bootstrapping cookies and refreshing the auth token.
	if resp != nil && resp.StatusCode == 421 {
		fs.Debugf("icloud", "[PCS] Received 421 status; bootstrapping web cookies and refreshing token")
		c.BootstrapWebCookies(ctx)
		if err2 := c.RefreshWebAuth(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to refresh web auth after 421 response: %w", err2)
		}

		// Rebuild headers with the new session state and retry the request.
		if h2, err2 := c.SetupHeaders(); err2 == nil {
			opts.ExtraHeaders = h2
		} else {
			return nil, fmt.Errorf("failed to rebuild setup headers after 421 response: %w", err2)
		}
		return c.RequestNoReAuth(ctx, opts, req, out)
	}

	return resp, err
}

// ----------------------------------------------------------------------------
// # Section: PCS Flow (Advanced Data Protection Web Consent)
// ----------------------------------------------------------------------------

// EnsurePCSForService orchestrates the entire Private Cloud Storage (PCS) consent
// flow, which is required when an Apple ID has Advanced Data Protection (ADP)
// enabled.
//
// This process involves:
// 1. Checking if ADP is active. If not, no action is needed.
// 2. If ADP is active, checking if the device has already been granted consent.
// 3. If not consented, sending a notification to a trusted Apple device.
// 4. Polling until the user approves the request on their trusted device.
// 5. Once consent is granted, polling until the necessary PCS cookies are available.
//
// The appName parameter specifies the service requiring access (e.g., "ICLOUD_DRIVE").
func (c *Client) EnsurePCSForService(ctx context.Context, appName string) error {
	st, err := c.CheckWebAccessState(ctx)
	if err != nil {
		return fmt.Errorf("failed to check web access state: %w", err)
	}

	if !st.IsICDRSDisabled {
		fs.Debugf("icloud", "[PCS] ADP not active; no PCS consent required.")
		return nil
	}

	fs.Infof("icloud", "[PCS] Advanced Data Protection is active. Attempting to obtain consent from your trusted device.")
	if !st.IsDeviceConsentedForPCS {
		fs.Infof("icloud", "[PCS] A request has been sent to your trusted Apple device(s). Please approve the request for web access to proceed.")
		if err := c.WaitForDeviceConsent(ctx, appName); err != nil {
			return err
		}
	}

	fs.Debugf("icloud", "[PCS] Device is consented. Now waiting for service cookies to be staged for %q...", appName)
	if err := c.WaitForPCSCookies(ctx, appName); err != nil {
		return err
	}

	// Persist the session, including the newly acquired PCS cookies.
	if c.sessionSaveCallback != nil && c.Session != nil {
		c.sessionSaveCallback(c.Session)
	}

	fs.Infof("icloud", "[PCS] Successfully obtained web access for %q.", appName)
	return nil
}

// CheckWebAccessState queries iCloud for the current ADP and device consent status.
func (c *Client) CheckWebAccessState(ctx context.Context) (*WebAccessState, error) {
	opts := rest.Opts{
		Method:     "POST",
		RootURL:    setupWSBase,
		Path:       "/requestWebAccessState",
		Parameters: c.DefaultSetupParams(ctx),
	}
	var st WebAccessState
	_, err := c.SetupCallJSON(ctx, opts, nil, &st)
	if err != nil {
		return nil, fmt.Errorf("requestWebAccessState failed: %w", err)
	}
	fs.Debugf("icloud", "[PCS] Web Access State: ADP Enabled=%v, Device Consented=%v", st.IsICDRSDisabled, st.IsDeviceConsentedForPCS)
	return &st, nil
}

// EnableDeviceConsentForPCS sends a notification to the user's trusted devices
// requesting consent for web access.
func (c *Client) EnableDeviceConsentForPCS(ctx context.Context, appName string) error {
	opts := rest.Opts{
		Method:     "POST",
		RootURL:    setupWSBase,
		Path:       "/enableDeviceConsentForPCS",
		Parameters: c.DefaultSetupParams(ctx),
	}
	body := map[string]any{"appName": appName}

	var out EnableConsentResponse
	resp, err := c.SetupCallJSON(ctx, opts, body, &out)
	if err != nil {
		return fmt.Errorf("enableDeviceConsentForPCS request failed: %w", err)
	}
	if out.Error != "" {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		fs.Debugf("icloud", "[PCS] enableDeviceConsentForPCS API error: http=%d error=%q message=%q", code, out.Error, out.Message)
		return fmt.Errorf("enableDeviceConsentForPCS returned an error: %s", out.Error)
	}
	if out.IsDeviceConsentNotificationSent {
		fs.Debugf("icloud", "[PCS] Consent notification sent to trusted device(s).")
	}
	return nil
}

// RequestPCS asks the server to provision the PCS cookies for the specified app.
// This is called repeatedly until the cookies are ready.
func (c *Client) RequestPCS(ctx context.Context, appName string, derivedFromUserAction bool) (*PCSResponse, error) {
	opts := rest.Opts{
		Method:     "POST",
		RootURL:    setupWSBase,
		Path:       "/requestPCS",
		Parameters: c.DefaultSetupParams(ctx),
	}
	body := map[string]any{
		"appName":               appName,
		"derivedFromUserAction": derivedFromUserAction,
	}
	var out PCSResponse
	_, err := c.SetupCallJSON(ctx, opts, body, &out)
	if err != nil {
		return nil, fmt.Errorf("requestPCS failed: %w", err)
	}
	return &out, nil
}

// WaitForDeviceConsent initiates the consent request and then polls until the
// user grants approval on a trusted device.
func (c *Client) WaitForDeviceConsent(ctx context.Context, appName string) error {
	ctx, cancel := context.WithTimeout(ctx, pcsConsentTimeout)
	defer cancel()

	if err := c.EnableDeviceConsentForPCS(ctx, appName); err != nil {
		return err
	}

	// Immediately "arm" the device by calling requestPCS once. This tells the
	// server we are ready and waiting for the user's approval.
	if resp, err := c.RequestPCS(ctx, appName, true); err == nil && resp.Message != "" {
		fs.Debugf("icloud", "[PCS] Initial requestPCS call status: %s", resp.Message)
	}

	ticker := time.NewTicker(pcsPollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for approval on your trusted device; please try again")
		case <-ticker.C:
			st, err := c.CheckWebAccessState(ctx)
			if err != nil {
				fs.Debugf("icloud", "[PCS] Error checking web access state during polling: %v", err)
				continue
			}
			if st.IsDeviceConsentedForPCS {
				fs.Infof("icloud", "[PCS] Approval received from trusted device.")
				return nil
			}
			// Periodically re-arm the request to ensure it doesn't expire.
			_, _ = c.RequestPCS(ctx, appName, false)
			fs.Infof("icloud", "[PCS] Still waiting for approval on your trusted device...")
		}
	}
}

// WaitForPCSCookies polls the `requestPCS` endpoint until the server confirms
// that the necessary cookies have been successfully staged and set.
func (c *Client) WaitForPCSCookies(ctx context.Context, appName string) error {
	ctx, cancel := context.WithTimeout(ctx, pcsConsentTimeout)
	defer cancel()

	ticker := time.NewTicker(pcsPollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out while waiting to obtain service cookies for %s", appName)
		case <-ticker.C:
			resp, err := c.RequestPCS(ctx, appName, false)
			if err != nil {
				fs.Debugf("icloud", "[PCS] requestPCS error during polling: %v", err)
				continue
			}
			if strings.EqualFold(resp.Status, "success") {
				return nil
			}
			if resp.Message != "" {
				fs.Debugf("icloud", "[PCS] Waiting for cookies, server status: %s", resp.Message)
			}
		}
	}
}
