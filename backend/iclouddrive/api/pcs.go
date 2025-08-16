// Adds the PCS (Private Cloud Storage) consent flow required when
// Advanced Data Protection (ADP) is enabled on the Apple ID.
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
	icloudWebBase = "https://www.icloud.com"
	setupWSBase   = "https://setup.icloud.com/setup/ws/1"
)

type webAccessState struct {
	// IsICDRSDisabled is true when ADP is enabled for the Apple ID
	IsICDRSDisabled bool `json:"isICDRSDisabled"`
	// IsDeviceConsentedForPCS is true when the trusted device has already consented to PCS
	IsDeviceConsentedForPCS bool `json:"isDeviceConsentedForPCS"`
}

type pcsResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Apple sometimes returns {}, sometimes a richer object. Treat 200 as “armed”,
// and only error if Apple provides an explicit error.
type enableResp struct {
	IsDeviceConsentNotificationSent bool   `json:"isDeviceConsentNotificationSent"`
	Success                         bool   `json:"success"`
	Error                           string `json:"error"`
	Message                         string `json:"message"`
}

// ---------- Cookie helpers ----------

func (c *Client) WebAuthToken() string {
	if c.Session == nil || c.Session.Cookies == nil {
		return ""
	}
	for _, ck := range c.Session.Cookies {
		if ck != nil && ck.Name == "X-APPLE-WEBAUTH-TOKEN" && ck.Value != "" {
			return ck.Value
		}
	}
	return ""
}

func (c *Client) WebAuthUser() string {
	if c.Session != nil && c.Session.Cookies != nil {
		for _, ck := range c.Session.Cookies {
			if ck != nil && ck.Name == "X-APPLE-WEBAUTH-USER" && ck.Value != "" {
				return ck.Value
			}
		}
	}
	return c.appleID
}

func (c *Client) CookieValue(name string) string {
	if c.Session == nil || c.Session.Cookies == nil {
		return ""
	}
	for _, ck := range c.Session.Cookies {
		if ck != nil && ck.Name == name && ck.Value != "" {
			return ck.Value
		}
	}
	return ""
}

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

// CookieHeaderFor builds a Cookie header with only cookies valid for the root URL's domain.
// It adds WEBAUTH/HSA cookies ONLY for setup.icloud.com where Apple expects them.
func (c *Client) CookieHeaderFor(root string) string {
	u, _ := url.Parse(root)
	domainCookies, _ := GetCookiesForDomain(u, c.Session.Cookies)

	var b strings.Builder
	for _, ck := range domainCookies {
		if ck == nil || ck.Name == "" || ck.Value == "" {
			continue
		}
		b.WriteString(ck.Name)
		b.WriteString("=")
		b.WriteString(ck.Value)
		b.WriteString(";")
	}
	cur := b.String()

	// Only for setup.icloud.com we force-include WEBAUTH/HSA
	if strings.Contains(u.Host, "setup.icloud.com") {
		if tok := c.WebAuthToken(); tok != "" && !strings.Contains(cur, "X-APPLE-WEBAUTH-TOKEN=") {
			b.WriteString(" X-APPLE-WEBAUTH-TOKEN=" + tok + ";")
		}
		if usr := c.WebAuthUser(); usr != "" && !strings.Contains(cur, "X-APPLE-WEBAUTH-USER=") {
			b.WriteString(" X-APPLE-WEBAUTH-USER=" + usr + ";")
		}
		if hsa := c.CookieValue("X-APPLE-WEBAUTH-HSA-LOGIN"); hsa != "" && !strings.Contains(cur, "X-APPLE-WEBAUTH-HSA-LOGIN=") {
			b.WriteString(" X-APPLE-WEBAUTH-HSA-LOGIN=" + hsa + ";")
		}
	}

	return b.String()
}

// ---------- Build number & setup params ----------

// FetchWebBuildNumber tries to read BUILD_INFO.buildNumber from iCloud web (best-effort).
func (c *Client) FetchWebBuildNumber(ctx context.Context) string {
	req := &rest.Opts{Method: "GET", RootURL: icloudWebBase, Path: "/"}
	resp, err := c.srv.Call(ctx, req)
	if err != nil || resp == nil || resp.Body == nil {
		fs.Debugf("icloud", "[PCS] fetchWebBuildNumber: request failed: %v", err)
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	re := regexp.MustCompile(`BUILD_INFO\s*=\s*\{[^}]*buildNumber\s*:\s*"?([^"',}]+)"?`)
	if m := re.FindStringSubmatch(string(body)); len(m) >= 2 && m[1] != "" {
		return m[1]
	}
	fs.Debugf("icloud", "[PCS] fetchWebBuildNumber: could not extract build number (status %d)", resp.StatusCode)
	return ""
}

// DefaultSetupParams builds the common query params required by setup APIs.
func (c *Client) DefaultSetupParams(ctx context.Context) url.Values {
	p := url.Values{}

	// Cache build number to avoid repeated fetches
	if c.webBuildNumber == "" {
		c.webBuildNumber = c.FetchWebBuildNumber(ctx)
	}
	if c.webBuildNumber != "" {
		p.Set("clientBuildNumber", c.webBuildNumber)
	}

	if c.Session != nil && c.Session.ClientID != "" {
		p.Set("clientId", c.Session.ClientID)
	}
	if c.Session != nil && c.Session.AccountInfo.DsInfo != nil && c.Session.AccountInfo.DsInfo.Dsid != "" {
		p.Set("dsid", c.Session.AccountInfo.DsInfo.Dsid)
	}
	return p
}

// SetupHeaders builds headers for setup.icloud.com with proper cookies.
func (c *Client) SetupHeaders() (map[string]string, error) {
	// Throttle this noisy line to at most once per 10s
	if time.Since(c.lastAttemptHSA) > 10*time.Second {
		fs.Debugf("icloud", "[PCS] setupHeaders: tokenPresent=%v", c.WebAuthToken() != "")
		c.lastAttemptHSA = time.Now()
	}
	// Best-effort ensure WEBAUTH token
	if c.WebAuthToken() == "" {
		_ = c.RefreshWebAuth()
	}

	h := GetCommonHeaders(map[string]string{
		"Referer":          icloudWebBase + "/iclouddrive/",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           icloudWebBase,
	})
	if c.Session != nil && c.Session.SessionID != "" {
		h["X-Apple-ID-Session-Id"] = c.Session.SessionID
	}
	if c.Session != nil && c.Session.Scnt != "" {
		h["scnt"] = c.Session.Scnt
	}

	h["Cookie"] = c.CookieHeaderFor(setupWSBase)
	return h, nil
}

// RefreshWebAuth calls /refreshWebAuth to get X-APPLE-WEBAUTH-TOKEN via Set-Cookie.
func (c *Client) RefreshWebAuth() error {
	fs.Debugf("icloud", "[PCS] refreshWebAuth(): POST %s/refreshWebAuth", setupWSBase)
	opts := rest.Opts{
		Method:       "POST",
		RootURL:      setupWSBase,
		Path:         "/refreshWebAuth",
		ExtraHeaders: map[string]string{"Origin": icloudWebBase, "Referer": icloudWebBase + "/"},
	}
	resp, err := c.RequestNoReAuth(context.Background(), opts, nil, nil)
	if err != nil {
		fs.Debugf("icloud", "[PCS] refreshWebAuth warning: %v", err)
		return err
	}
	for _, ck := range resp.Cookies() {
		if ck != nil && ck.Name == "X-APPLE-WEBAUTH-TOKEN" && ck.Value != "" {
			c.AddOrReplaceCookie(ck)
		}
	}
	return nil
}

// ---------- Low-level helpers to talk to setup.icloud.com ----------

func (c *Client) SetupCallJSON(ctx context.Context, opts rest.Opts, req any, out any) (*http.Response, error) {
	h, err := c.SetupHeaders()
	if err != nil {
		return nil, err
	}
	opts.ExtraHeaders = h

	resp, err := c.RequestNoReAuth(ctx, opts, req, out)
	if err == nil && (resp == nil || resp.StatusCode != 421) {
		return resp, err
	}

	// 421 → bootstrap web cookies + refresh token and retry once
	fs.Debugf("icloud", "[PCS] setupCallJSON: got 421 → bootstrap web cookies + refreshWebAuth, then retry once")
	c.BootstrapWebCookies()
	_ = c.RefreshWebAuth()

	if h2, err2 := c.SetupHeaders(); err2 == nil {
		opts.ExtraHeaders = h2
	}
	return c.RequestNoReAuth(ctx, opts, req, out)
}

// BootstrapWebCookies visits iCloud web so they can set required cookies.
func (c *Client) BootstrapWebCookies() {
	commonHeaders := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
	}
	ctx := context.Background()
	// GET https://www.icloud.com/
	if resp, err := c.srv.Call(ctx, &rest.Opts{
		Method:       "GET",
		RootURL:      icloudWebBase,
		Path:         "/",
		ExtraHeaders: commonHeaders,
	}); err == nil {
		c.Session.CaptureCookies(resp)
	}
	// GET https://www.icloud.com/iclouddrive/
	if resp, err := c.srv.Call(ctx, &rest.Opts{
		Method:       "GET",
		RootURL:      icloudWebBase,
		Path:         "/iclouddrive/",
		ExtraHeaders: commonHeaders,
	}); err == nil {
		c.Session.CaptureCookies(resp)
	}
}

// ---------- PCS flow (ADP web consent) ----------

func (c *Client) CheckWebAccessState() (*webAccessState, error) {
	h, err := c.SetupHeaders()
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		RootURL:      setupWSBase,
		Path:         "/requestWebAccessState",
		ExtraHeaders: h,
		Parameters:   c.DefaultSetupParams(context.Background()),
	}
	var st webAccessState
	_, err = c.SetupCallJSON(context.Background(), opts, nil, &st)
	if err != nil {
		return nil, err
	}
	fs.Debugf("icloud", "[PCS] webAccessState: ADP=%v deviceConsented=%v", st.IsICDRSDisabled, st.IsDeviceConsentedForPCS)
	return &st, nil
}

func (c *Client) EnableDeviceConsentForPCS(appName string) error {
	h, err := c.SetupHeaders()
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:       "POST",
		RootURL:      setupWSBase,
		Path:         "/enableDeviceConsentForPCS",
		ExtraHeaders: h,
		Parameters:   c.DefaultSetupParams(context.Background()),
	}
	body := map[string]any{"appName": appName}

	var out enableResp
	resp, err := c.SetupCallJSON(context.Background(), opts, body, &out)
	if err != nil {
		return err
	}

	// Don’t treat “success=false {}” as an error; only error on explicit error string.
	if out.Error != "" {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		fs.Debugf("icloud", "[PCS] enableDeviceConsentForPCS: http=%d error=%q message=%q", code, out.Error, out.Message)
		return fmt.Errorf("enableDeviceConsentForPCS failed: %s", out.Error)
	}
	if out.IsDeviceConsentNotificationSent {
		fs.Debugf("icloud", "[PCS] consent notification sent to trusted device")
	}
	return nil
}

func (c *Client) RequestPCS(appName string, derived bool) (*pcsResp, error) {
	h, err := c.SetupHeaders()
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		RootURL:      setupWSBase,
		Path:         "/requestPCS",
		ExtraHeaders: h,
		Parameters:   c.DefaultSetupParams(context.Background()),
	}
	body := map[string]any{
		"appName":               appName,
		"derivedFromUserAction": derived,
	}
	var out pcsResp
	_, err = c.SetupCallJSON(context.Background(), opts, body, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EnsurePCSForService(appName string) error {
	st, err := c.CheckWebAccessState()
	if err != nil {
		return err
	}

	if !st.IsICDRSDisabled {
		fs.Debugf("icloud", "[PCS] ADP not active: no PCS consent required")
		return nil
	}

	if !st.IsDeviceConsentedForPCS {
		fs.Infof("icloud", "[PCS] ADP active: requesting PCS consent from your trusted device…")
		fs.Infof("icloud", "[PCS] Approve the iCloud Drive web access request on a trusted device.")

		if err := c.EnableDeviceConsentForPCS(appName); err != nil {
			return fmt.Errorf("enableDeviceConsentForPCS: %w", err)
		}

		// Immediately “arm” the device by calling requestPCS once
		if resp, err := c.RequestPCS(appName, true); err == nil && resp.Message != "" {
			fs.Debugf("icloud", "[PCS] requestPCS: %s", resp.Message)
		}

		// Poll up to 5 minutes for device consent (checking state & nudging PCS)
		for i := 0; i < 60; i++ {
			time.Sleep(5 * time.Second)

			// Keep the arming alive
			if r, err := c.RequestPCS(appName, false); err == nil && r.Message != "" {
				fs.Debugf("icloud", "[PCS] requestPCS: %s", r.Message)
			}

			st, err = c.CheckWebAccessState()
			if err != nil {
				fs.Debugf("icloud", "[PCS] checkWebAccessState error during polling: %v", err)
				continue
			}
			fs.Debugf("icloud", "[PCS] state: ADP=%v, deviceConsented=%v", st.IsICDRSDisabled, st.IsDeviceConsentedForPCS)
			if st.IsDeviceConsentedForPCS {
				break
			}
			if (i+1)%6 == 0 {
				fs.Infof("icloud", "[PCS] Still waiting for approval on your trusted device…")
			}
		}
		if !st.IsDeviceConsentedForPCS {
			return fmt.Errorf("PCS consent not received: please approve on your trusted device and retry")
		}
	}

	// Device is consented → now wait until cookies are staged for the service.
	fs.Debugf("icloud", "[PCS] Requesting PCS for %q and waiting until ready…", appName)
	for attempt := 0; attempt < 60; attempt++ { // up to ~5 minutes
		resp, err := c.RequestPCS(appName, attempt == 0)
		if err != nil {
			fs.Debugf("icloud", "[PCS] requestPCS attempt %d/60 error: %v", attempt+1, err)
			time.Sleep(5 * time.Second)
			continue
		}
		if strings.EqualFold(resp.Status, "success") {
			fs.Debugf("icloud", "[PCS] PCS for %q obtained", appName)
			// Nice-to-have: persist updated cookies immediately if supported.
			if c.sessionSaveCallback != nil && c.Session != nil {
				c.sessionSaveCallback(c.Session)
			}
			return nil
		}
		if resp.Message != "" {
			// Typical messages: "Requested the device to upload cookies.", "Cookies not available yet on server."
			fs.Debugf("icloud", "[PCS] requestPCS: %s", resp.Message)
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("unable to obtain PCS for %s: timeout", appName)
}
