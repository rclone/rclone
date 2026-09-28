package funambol

// SMS login via Telefónica's T3 identity provider, used by O2 Spain accounts
// that have no Funambol password.  It mirrors the O2 web client:
//
//	rclone                      O2 server            apiseg / T3
//	  |-- pkce/authorize -------->|  (keeps verifier)
//	  |<-- 302 authorize ---------|
//	  |-- authorize ---------------------------------->|
//	  |<-- 302 t3#…sessionID&sessionData --------------|
//	  |-- manageCredentialMobileO2 ------------------->|  sends the SMS
//	  |-- verifyCredentialMobileO2 (code) ------------>|
//	  |<-- redirectUri /sapi/login/oauth?code&state ---|
//	  |-- redirectUri ----------->|  (exchanges code, sets session cookies)
//
// The server holds the OAuth secrets, so rclone needs none: it only keeps
// one cookie jar from the first call to the last.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// t3API is the T3 credential API; a var so tests can point it elsewhere.
var t3API = "https://api.telefonica.es/t3/cus/segu/v5/seguCredentialO2s"

const (
	t3Host       = "t3.o2online.es"             // the only login page we know how to drive
	t3Origin     = "https://" + t3Host          // sent as Origin/Referer like the page does
	t3LoginApp   = "/loginAppO2"                // suffix of the COCO.idOrigen header
	oauthPath    = "/sapi/login/oauth"          // where T3 sends the authorization code
	pkcePath     = "/sapi/oauth/pkce/authorize" // starts the server-side PKCE flow
	configTmpSMS = "tmp_sms"                    // smsPending between the config steps
)

const (
	maxRedirectBody = 4 << 10 // redirect body drained so the connection is reused
	maxHops         = 5       // redirects followed to reach the T3 login page
)

// Spanish mobile numbers as T3 expects them: 9 digits, no country code.
const (
	spainCC      = "34"
	intlPrefix   = "00"
	mobileDigits = 9
)

// errCodeRejected marks a code T3 refused, which the user may enter again.
var errCodeRejected = errors.New("the code was not accepted")

// smsPending is the T3 session kept between sending and verifying the code.
type smsPending struct {
	SessionID   string `json:"session_id"`
	SessionData string `json:"session_data"`
	Consumer    string `json:"consumer"`
	Mobile      string `json:"mobile"`
	Cookies     string `json:"cookies"` // T3 API cookies, from dumpCookiesFor
	Session     string `json:"session"` // endpoint cookies holding the PKCE verifier
}

type smsSendRequest struct {
	Mobile      string `json:"mobile"`
	SessionID   string `json:"sessionID"`
	SessionData string `json:"sessionData"`
	ConsumerID  string `json:"consumerId"`
}

type smsSendResponse struct {
	NewSessionData string `json:"newSessionData"`
	NewSessionID   string `json:"newSessionID"`
}

// smsVerifyRequest mirrors the page, including "Mobile" being capitalised
// here but not in smsSendRequest.
type smsVerifyRequest struct {
	OTP         string `json:"otp"`
	SessionData string `json:"sessionData"`
	SessionID   string `json:"sessionID"`
	Mobile      string `json:"Mobile"`
}

type smsVerifyResponse struct {
	RedirectURI string `json:"redirectUri"`
}

// normalizePhone reduces a Spanish mobile number to the 9 digits T3 expects,
// e.g. "+34 612 34 56 78" -> "612345678".
func normalizePhone(s string) (string, error) {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}

	n := strings.TrimPrefix(b.String(), intlPrefix+spainCC)
	if len(n) == len(spainCC)+mobileDigits && strings.HasPrefix(n, spainCC) {
		n = n[len(spainCC):]
	}

	if len(n) != mobileDigits {
		return "", fmt.Errorf("SMS login needs user to be a Spanish mobile number, not %q", s)
	}
	return n, nil
}

// t3URL is the URL the jar keys the T3 API cookies on.
func t3URL() *url.URL {
	u, _ := url.Parse(t3API)
	return u
}

// oauthURL is the callback T3 hands the code to; the jar keys the PKCE
// session cookie on it, which may be scoped below "/".
func (f *Fs) oauthURL() *url.URL {
	u := *f.jarURL()
	u.Path = oauthPath
	return &u
}

// get GETs rawURL without following redirects - following would drop the
// URL fragment T3 carries its session in - and returns the resolved
// Location, empty if the reply is not a redirect.  Errors quote only the
// host and path: the URL may carry an OAuth code.
func (f *Fs) get(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := rest.ClientWithNoRedirects(f.client).Do(req)
	if err != nil {
		return "", fmt.Errorf("%s%s: %w", req.URL.Host, req.URL.Path, err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxRedirectBody))
	_ = resp.Body.Close()
	f.keepPersistentLogin(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("%s%s: HTTP %d", req.URL.Host, req.URL.Path, resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", nil
	}

	next, err := resp.Request.URL.Parse(location)
	if err != nil {
		return "", err
	}
	return next.String(), nil
}

// keepPersistentLogin logs the cookies a login hop sets - attributes only,
// never values - and files the persistent login cookie under the endpoint
// root, so a PLC scoped to a narrower path or another host is still stored.
func (f *Fs) keepPersistentLogin(resp *http.Response) {
	u := resp.Request.URL
	fs.Debugf(nil, "funambol: %s%s: HTTP %d, redirect to %s", u.Host, u.Path, resp.StatusCode, redirectTarget(resp))
	for _, c := range resp.Cookies() {
		fs.Debugf(nil, "funambol:   sets cookie %s (domain %q, path %q, expires %v, max-age %d, empty %v)",
			c.Name, c.Domain, c.Path, c.Expires, c.MaxAge, c.Value == "")
		if c.Name == persistentLoginCookie && c.Value != "" {
			f.client.Jar.SetCookies(f.jarURL(), []*http.Cookie{{Name: c.Name, Value: c.Value}})
		}
	}
}

// redirectTarget is the host and path of resp's Location, without the query
// or fragment, which may carry an OAuth code or session.
func redirectTarget(resp *http.Response) string {
	next, err := resp.Location()
	if err != nil {
		return "none"
	}
	return next.Host + next.Path
}

// loginPage follows redirects from rawURL until the T3 login page.
func (f *Fs) loginPage(ctx context.Context, rawURL string) (string, error) {
	for range maxHops {
		next, err := f.get(ctx, rawURL)
		if err != nil {
			return "", err
		}

		if next == "" {
			return "", fmt.Errorf("login redirects stopped before %s", t3Host)
		}

		u, err := url.Parse(next)
		if err != nil {
			return "", err
		}

		if u.Host == t3Host {
			return next, nil
		}
		rawURL = next
	}
	return "", fmt.Errorf("login redirects did not reach %s", t3Host)
}

// parseT3 extracts the T3 session from the login page URL, e.g.
// https://t3.o2online.es/acceso/#/accessUserPassO2?client_name=…&sessionID=…&sessionData=…
func parseT3(rawURL string) (*smsPending, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if u.Host != t3Host {
		return nil, fmt.Errorf("unsupported identity provider %q", u.Host)
	}

	_, query, _ := strings.Cut(u.Fragment, "?")
	v, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	p := &smsPending{SessionID: v.Get("sessionID"), SessionData: v.Get("sessionData"), Consumer: v.Get("client_name")}
	if p.SessionID == "" || p.SessionData == "" || p.Consumer == "" {
		return nil, fmt.Errorf("login page URL lacks the T3 session: %s", rawURL)
	}
	return p, nil
}

// t3Call POSTs req to the T3 credential API method and decodes the reply.
// It is not retried: a repeat would send another SMS or reuse a spent code.
func (f *Fs) t3Call(ctx context.Context, method, consumer string, req, reply any) error {
	srv := rest.NewClient(f.client).SetRoot(t3API).SetErrorHandler(sapiErrorHandler)
	opts := rest.Opts{
		Method: "POST",
		Path:   "/" + method,
		ExtraHeaders: map[string]string{
			"*COCO.idOrigen": consumer + t3LoginApp, // exact case, as the page sends it
			"Origin":         t3Origin,
			"Referer":        t3Origin + "/",
		},
	}
	_, err := srv.CallJSON(ctx, &opts, req, reply)
	return err
}

// smsSend starts the OAuth flow and has T3 text a code to the user.  The
// returned smsPending carries what smsVerify needs, including the T3 API
// cookies; the endpoint's cookies stay in the jar.
func (f *Fs) smsSend(ctx context.Context) (*smsPending, error) {
	mobile, err := normalizePhone(f.opt.User)
	if err != nil {
		return nil, err
	}

	// The web client sends only platform and deviceid.  rememberme and
	// access_type are guesses to ask for a persistent login, which the
	// server does not otherwise give to SMS logins.
	start := f.opt.Endpoint + pkcePath + "?" + url.Values{
		"platform":    {"web"},
		"deviceid":    {f.opt.DeviceID},
		"rememberme":  {"true"},
		"access_type": {"offline"},
	}.Encode()
	page, err := f.loginPage(ctx, start)
	if err != nil {
		return nil, err
	}

	p, err := parseT3(page)
	if err != nil {
		return nil, err
	}

	p.Mobile = mobile
	req := smsSendRequest{Mobile: p.Mobile, SessionID: p.SessionID, SessionData: p.SessionData, ConsumerID: p.Consumer}
	var reply smsSendResponse
	if err := f.t3Call(ctx, "manageCredentialMobileO2", p.Consumer, &req, &reply); err != nil {
		return nil, fmt.Errorf("sending the SMS code: %w", err)
	}

	if reply.NewSessionID == "" || reply.NewSessionData == "" {
		return nil, errors.New("sending the SMS code: no session in reply")
	}
	p.SessionID, p.SessionData = reply.NewSessionID, reply.NewSessionData
	p.Cookies = f.dumpCookiesFor(t3URL())
	p.Session = f.dumpCookiesFor(f.oauthURL())
	return p, nil
}

// smsVerify checks the code with T3 and completes the login at the callback
// it returns, which leaves the session cookies in the jar.
func (f *Fs) smsVerify(ctx context.Context, p *smsPending, code string) error {
	f.restoreCookiesFor(t3URL(), p.Cookies)
	f.restoreCookiesFor(f.oauthURL(), p.Session)

	req := smsVerifyRequest{OTP: code, SessionData: p.SessionData, SessionID: p.SessionID, Mobile: p.Mobile}
	var reply smsVerifyResponse
	if err := f.t3Call(ctx, "verifyCredentialMobileO2", p.Consumer, &req, &reply); err != nil {
		return fmt.Errorf("%w: %v", errCodeRejected, err)
	}

	// Only hand the code to our own endpoint, over the same scheme.
	callback, err := url.Parse(reply.RedirectURI)
	if err != nil {
		return errors.New("unparsable callback")
	}

	want := f.oauthURL()
	if callback.Scheme != want.Scheme || callback.Host != want.Host || callback.Path != want.Path {
		return fmt.Errorf("unexpected callback %s://%s%s", callback.Scheme, callback.Host, callback.Path)
	}

	// The callback is a SAPI login, which gives the persistent login
	// cookie to a password login that asks with rememberme.
	q := callback.Query()
	q.Set("rememberme", "true")
	callback.RawQuery = q.Encode()

	if _, err := f.get(ctx, callback.String()); err != nil {
		return err
	}

	if f.validationKeyFromJar() == "" {
		return errors.New("login did not complete: no validation key")
	}

	// Without the persistent login cookie the session can't be renewed.
	if f.persistentLoginPresent() {
		fs.Debugf(nil, "funambol: SMS login persistent cookie present: true")
	} else {
		fs.Logf(nil, "funambol: SMS login gave no persistent login cookie, so the session will expire in a few hours")
	}
	return nil
}
