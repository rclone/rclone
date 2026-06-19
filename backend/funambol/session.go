package funambol

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/rclone/rclone/backend/funambol/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

// persistentLoginCookie is the name of the long-lived "persistent login
// cookie" the service issues when rememberme=true.  Presenting it lets the
// backend mint fresh validation keys (via SEC-1003) without a new code.
const persistentLoginCookie = "PLC"

// browserUA is sent on the login calls.  The service's bot protection rejects
// a login from an unknown device id unless the request looks like a browser;
// presenting a browser User-Agent (plus Origin/Referer) lets a freshly
// generated device id register itself on first login, just like the web app.
const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// newDeviceID generates a fresh "web-<hex>" device id in the format the service
// expects.  A brand-new id registers itself during the first (two-factor)
// login as long as the login request carries browser-like headers.
func newDeviceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "web-" + hex.EncodeToString(b)
}

// configTmpSession holds the provisional cookies between the credentials step
// and the verification-code step of interactive configuration.
const configTmpSession = "tmp_session"

// deviceName is reported to the service as the device name (shows up in the
// account's "my devices" list).
const deviceName = "rclone"

// cookiePair is the minimal serialisable form of a cookie (the jar only exposes
// name and value).
type cookiePair struct {
	N string `json:"n"`
	V string `json:"v"`
}

// jarURL is the URL the cookie jar is keyed on.
func (f *Fs) jarURL() *url.URL {
	u, _ := url.Parse(f.opt.Endpoint)
	return u
}

// validationKeyFromJar returns the value of the validationKey cookie, which is
// exactly the key SAPI expects as the validationkey query parameter.
func (f *Fs) validationKeyFromJar() string {
	if f.client == nil || f.client.Jar == nil {
		return ""
	}
	for _, c := range f.client.Jar.Cookies(f.jarURL()) {
		if c.Name == "validationKey" {
			return c.Value
		}
	}
	return ""
}

// persistentLoginPresent reports whether the long-lived persistent login
// cookie is in the jar, i.e. the session can be refreshed without a new code.
func (f *Fs) persistentLoginPresent() bool {
	if f.client == nil || f.client.Jar == nil {
		return false
	}
	for _, c := range f.client.Jar.Cookies(f.jarURL()) {
		if c.Name == persistentLoginCookie && c.Value != "" {
			return true
		}
	}
	return false
}

// sapiErrorHandler decodes a SAPI error envelope from a non-2xx response into a
// typed *api.Error so callers can recognise codes such as SEC-1003 (and read
// the fresh validation key it carries in the data field).
func sapiErrorHandler(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err == nil {
		var s api.Status
		if jErr := json.Unmarshal(body, &s); jErr == nil && s.Err != nil && s.Err.Code != "" {
			return s.Err
		}
	}
	return fmt.Errorf("HTTP error %v (%v) returned body: %q", resp.StatusCode, resp.Status, body)
}

// loginHeaders are the extra headers the web client sends on the login/OTP
// calls.  X-deviceid is set as a default header on the client and the browser
// User-Agent comes from the client's transport; Origin/Referer complete the
// browser-like request the service's bot protection expects.
func (f *Fs) loginHeaders() map[string]string {
	return map[string]string{
		"X-devicename": deviceName,
		"Origin":       f.opt.Endpoint,
		"Referer":      f.opt.Endpoint + "/",
	}
}

// dumpCookies serialises the current session cookies to a base64 JSON blob.
func (f *Fs) dumpCookies() string {
	if f.client == nil || f.client.Jar == nil {
		return ""
	}
	var pairs []cookiePair
	for _, c := range f.client.Jar.Cookies(f.jarURL()) {
		pairs = append(pairs, cookiePair{c.Name, c.Value})
	}
	b, err := json.Marshal(pairs)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// restoreCookies loads cookies previously produced by dumpCookies into the jar.
func (f *Fs) restoreCookies(s string) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return
	}
	var pairs []cookiePair
	if err := json.Unmarshal(raw, &pairs); err != nil {
		return
	}
	cookies := make([]*http.Cookie, 0, len(pairs))
	for _, p := range pairs {
		cookies = append(cookies, &http.Cookie{Name: p.N, Value: p.V})
	}
	f.client.Jar.SetCookies(f.jarURL(), cookies)
}

// persistSession writes the current cookies back to the config so they survive
// across rclone invocations.
func (f *Fs) persistSession() {
	if f.m == nil {
		return
	}
	if s := f.dumpCookies(); s != "" {
		f.m.Set("cookies", s)
	}
}

// loginGenerate performs the credentials step.  The web client posts the
// username and password as a form to /sapi/login/otp/generate together with the
// X-deviceid header; this authenticates and, when two-factor is enforced,
// emails (or SMSes) a one-time code.  rememberme=true requests the persistent
// login cookie so the session can be re-established later without a new code.
// Set-Cookie headers (the provisional session) are captured by the jar
// regardless of the two-factor outcome.
func (f *Fs) loginGenerate(ctx context.Context) error {
	form := url.Values{"login": {f.opt.User}, "password": {f.opt.Pass}, "rememberme": {"true"}}.Encode()
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/sapi/login/otp/generate",
		Body:         strings.NewReader(form),
		ContentType:  "application/x-www-form-urlencoded",
		ExtraHeaders: f.loginHeaders(),
		NoResponse:   true,
	}
	var resp *http.Response
	return f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
}

// otpRequest is the body of the OTP validation call.
type otpRequest struct {
	Data struct {
		OTP string `json:"otp"`
	} `json:"data"`
}

// otpValidateResponse is the validate reply; it may carry the validation key.
type otpValidateResponse struct {
	api.Status
	Data struct {
		ValidationKey string `json:"validationkey"`
		JSessionID    string `json:"jsessionid"`
	} `json:"data"`
}

// setValidationKey stores key as the validationKey cookie in the jar.
func (f *Fs) setValidationKey(key string) {
	if key == "" {
		return
	}
	f.client.Jar.SetCookies(f.jarURL(), []*http.Cookie{{Name: "validationKey", Value: key}})
}

// otpValidate submits the emailed/SMS code to complete the login and records
// the validation key (delivered as a response header or in the body).
func (f *Fs) otpValidate(ctx context.Context, code string) error {
	var req otpRequest
	req.Data.OTP = code
	opts := rest.Opts{Method: "POST", Path: "/sapi/login/otp", ExtraHeaders: f.loginHeaders()}
	var res otpValidateResponse
	var resp *http.Response
	err := f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.srv.CallJSON(ctx, &opts, &req, &res)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if err := res.AsErr(); err != nil {
		return err
	}
	vk := res.Data.ValidationKey
	if resp != nil && resp.Header.Get("validationkey") != "" {
		vk = resp.Header.Get("validationkey")
	}
	f.setValidationKey(vk)
	return nil
}

// newSessionFs builds a bare Fs (client, jar, srv, pacer) for the auth flow,
// without a dircache or root id.  It also makes sure a stable device id exists.
func newSessionFs(ctx context.Context, name string, m configmap.Mapper) (*Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.Pass != "" {
		clear, err := obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
		opt.Pass = clear
	}
	if opt.Endpoint == "" {
		opt.Endpoint = defaultEndpoint
	}
	opt.Endpoint = strings.TrimRight(opt.Endpoint, "/")

	// Generate and persist a device id on first use; a fresh one registers
	// itself during the login below thanks to the browser-like headers.
	if opt.DeviceID == "" {
		opt.DeviceID = newDeviceID()
		m.Set("device_id", opt.DeviceID)
	}

	// The login endpoints reject non-browser clients, so give this client a
	// browser User-Agent (the transport forces the UA, so it must come from
	// the context config rather than a per-request header).
	ctx, ci := fs.AddConfig(ctx)
	ci.UserAgent = browserUA

	client := fshttp.NewClient(ctx)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client.Jar = jar
	srv := rest.NewClient(client).SetRoot(opt.Endpoint)
	srv.SetErrorHandler(sapiErrorHandler)
	srv.SetHeader("X-deviceid", opt.DeviceID)
	return &Fs{
		name:   name,
		opt:    *opt,
		m:      m,
		client: client,
		srv:    srv,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		auth:   new(authState),
	}, nil
}

// Config drives interactive configuration, handling the emailed verification
// code (two-factor) and persisting the resulting session.
func Config(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	f, err := newSessionFs(ctx, name, m)
	if err != nil {
		return nil, err
	}
	if f.opt.User == "" || f.opt.Pass == "" {
		return fs.ConfigError("", "user and pass must be set before continuing")
	}

	switch configIn.State {
	case "":
		if err := f.loginGenerate(ctx); err != nil {
			return nil, fmt.Errorf("login failed - check your username and password: %w", err)
		}
		if f.validationKeyFromJar() != "" {
			// Provider didn't require a second factor.
			f.persistSession()
			return nil, nil
		}
		m.Set(configTmpSession, f.dumpCookies())
		return fs.ConfigInput("otp", "config_otp", "A verification code has been emailed to you.\nEnter the code")
	case "otp":
		code := strings.TrimSpace(configIn.Result)
		if code == "" {
			return fs.ConfigError("otp", "the verification code can't be empty")
		}
		if tmp, ok := m.Get(configTmpSession); ok && tmp != "" {
			f.restoreCookies(tmp)
		}
		if err := f.otpValidate(ctx, code); err != nil {
			return fs.ConfigError("otp", fmt.Sprintf("the code was not accepted (%v) - enter it again", err))
		}
		if f.validationKeyFromJar() == "" {
			return fs.ConfigError("", "login did not complete; please start again")
		}
		m.Set(configTmpSession, "")
		f.persistSession()
		return nil, nil
	}
	return nil, fmt.Errorf("unknown configuration state %q", configIn.State)
}
