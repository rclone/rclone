// Package oauthutil provides OAuth utilities.
package oauthutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/random"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

var (
	// templateString is the template used in the authorization webserver
	templateString string
)

const (
	// TitleBarRedirectURL is the OAuth2 redirect URL to use when the authorization
	// code should be returned in the title bar of the browser, with the page text
	// prompting the user to copy the code and paste it in the application.
	TitleBarRedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	// bindPort is the port that we bind the local webserver to
	bindPort = "53682"

	// bindAddress is binding for local webserver when active
	bindAddress = "127.0.0.1:" + bindPort

	// RedirectURL is redirect to local webserver when active
	RedirectURL = "http://" + bindAddress + "/"

	// RedirectPublicURL is redirect to local webserver when active with public name
	RedirectPublicURL = "http://localhost.rclone.org:" + bindPort + "/"

	// RedirectLocalhostURL is redirect to local webserver when active with localhost
	RedirectLocalhostURL = "http://localhost:" + bindPort + "/"

	// RedirectPublicSecureURL is a public https URL which
	// redirects to the local webserver
	RedirectPublicSecureURL = "https://oauth.rclone.org/"

	// DefaultAuthResponseTemplate is the default template used in the authorization webserver
	DefaultAuthResponseTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{ if .OK }}Success!{{ else }}Failure!{{ end }}</title>
</head>
<body>
<h1>{{ if .OK }}Success!{{ else }}Failure!{{ end }}</h1>
<hr>
<pre style="width: 750px; white-space: pre-wrap;">
{{ if eq .OK false }}
Error: {{ .Name }}<br>
{{ if .Description }}Description: {{ .Description }}<br>{{ end }}
{{ if .Code }}Code: {{ .Code }}<br>{{ end }}
{{ if .HelpURL }}Look here for help: <a href="{{ .HelpURL }}">{{ .HelpURL }}</a><br>{{ end }}
{{ else }}
All done. Please go back to rclone.
{{ end }}
</pre>
</body>
</html>
`
)

// SharedOptions are shared between backends the utilize an OAuth flow
var SharedOptions = []fs.Option{{
	Name:      config.ConfigClientID,
	Help:      "OAuth Client Id.\n\nLeave blank normally.",
	Sensitive: true,
}, {
	Name:      config.ConfigClientSecret,
	Help:      "OAuth Client Secret.\n\nLeave blank normally.",
	Sensitive: true,
}, {
	Name:      config.ConfigToken,
	Help:      "OAuth Access Token as a JSON blob.",
	Advanced:  true,
	Sensitive: true,
}, {
	Name:     config.ConfigAuthURL,
	Help:     "Auth server URL.\n\nLeave blank to use the provider defaults.",
	Advanced: true,
}, {
	Name:     config.ConfigTokenURL,
	Help:     "Token server url.\n\nLeave blank to use the provider defaults.",
	Advanced: true,
}}

// oldToken contains an end-user's tokens.
// This is the data you must store to persist authentication.
//
// From the original code.google.com/p/goauth2/oauth package - used
// for backwards compatibility in the rclone config file
type oldToken struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// GetToken returns the token saved in the config file under
// section name.
func GetToken(name string, m configmap.Mapper) (*oauth2.Token, error) {
	tokenString, ok := m.Get(config.ConfigToken)
	if !ok || tokenString == "" {
		return nil, fmt.Errorf("empty token found - please run \"rclone config reconnect %s:\"", name)
	}
	token := new(oauth2.Token)
	err := json.Unmarshal([]byte(tokenString), token)
	if err != nil {
		return nil, err
	}
	// if has data then return it
	if token.AccessToken != "" {
		return token, nil
	}
	// otherwise try parsing as oldToken
	oldtoken := new(oldToken)
	err = json.Unmarshal([]byte(tokenString), oldtoken)
	if err != nil {
		return nil, err
	}
	// Fill in result into new token
	token.AccessToken = oldtoken.AccessToken
	token.RefreshToken = oldtoken.RefreshToken
	token.Expiry = oldtoken.Expiry
	// Save new format in config file
	err = PutToken(name, m, token, false)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// PutToken stores the token in the config file
//
// This saves the config file if it changes
func PutToken(name string, m configmap.Mapper, token *oauth2.Token, newSection bool) error {
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return err
	}
	tokenString := string(tokenBytes)
	old, ok := m.Get(config.ConfigToken)
	if !ok || tokenString != old {
		m.Set(config.ConfigToken, tokenString)
		fs.Debugf(name, "Saved new token in config file")
	}
	return nil
}

// TokenSource stores updated tokens in the config file
type TokenSource struct {
	mu          sync.Mutex
	name        string
	m           configmap.Mapper
	tokenSource oauth2.TokenSource
	token       *oauth2.Token
	config      *oauth2.Config
	ctx         context.Context
	expiryTimer *time.Timer // signals whenever the token expires
}

// If token has expired then first try re-reading it (and its refresh token)
// from the config file in case a concurrently running rclone has updated them
// already.
// Returns whether either of the two tokens has been reread.
func (ts *TokenSource) reReadToken() (changed bool) {
	tokenString, found := ts.m.Get(config.ConfigToken)
	if !found || tokenString == "" {
		fs.Debugf(ts.name, "Failed to read token out of config file")
		return false
	}
	newToken := new(oauth2.Token)
	err := json.Unmarshal([]byte(tokenString), newToken)
	if err != nil {
		fs.Debugf(ts.name, "Failed to parse token out of config file: %v", err)
		return false
	}

	if !newToken.Valid() {
		fs.Debugf(ts.name, "Loaded invalid token from config file - ignoring")
	} else {
		fs.Debugf(ts.name, "Loaded fresh token from config file")
		changed = true
	}
	if newToken.RefreshToken != "" && newToken.RefreshToken != ts.token.RefreshToken {
		fs.Debugf(ts.name, "Loaded new refresh token from config file")
		changed = true
	}

	if changed {
		ts.token = newToken
		ts.tokenSource = nil // invalidate since we changed the token
	}
	return changed
}

type retrieveErrResponse struct {
	Error string `json:"error"`
}

// If err is nil or an error other than fatal OAuth errors, returns err itself.
// Otherwise returns a more user-friendly error.
func maybeWrapOAuthError(err error, remoteName string) (newErr error) {
	newErr = err
	if rErr, ok := err.(*oauth2.RetrieveError); ok {
		if rErr.Response.StatusCode == 400 || rErr.Response.StatusCode == 401 {
			fs.Debugf(remoteName, "got fatal oauth error: %v", rErr)
			var resp retrieveErrResponse
			if err = json.Unmarshal(rErr.Body, &resp); err != nil {
				newErr = fmt.Errorf("(can't decode error info) - try refreshing token with \"rclone config reconnect %s:\"", remoteName)
				return
			}
			var suggestion string
			switch resp.Error {
			case "invalid_client", "unauthorized_client", "unsupported_grant_type", "invalid_scope":
				suggestion = "if you're using your own client id/secret, make sure they're properly set up following the docs"
			case "invalid_grant":
				fallthrough
			default:
				suggestion = fmt.Sprintf("maybe token expired? - try refreshing with \"rclone config reconnect %s:\"", remoteName)
			}
			newErr = fmt.Errorf("%s: %s", resp.Error, suggestion)
		}
	}
	return
}

// Token returns a token or an error.
// Token must be safe for concurrent use by multiple goroutines.
// The returned Token must not be modified.
//
// This saves the token in the config file if it has changed
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	var (
		token   *oauth2.Token
		err     error
		changed = false
	)
	const maxTries = 5

	// Try getting the token a few times
	for i := 1; i <= maxTries; i++ {
		// Try reading the token from the config file in case it has
		// been updated by a concurrent rclone process
		if !ts.token.Valid() {
			if ts.reReadToken() {
				changed = true
			} else if ts.token.RefreshToken == "" {
				return nil, fserrors.FatalError(
					fmt.Errorf("token expired and there's no refresh token - manually refresh with \"rclone config reconnect %s:\"", ts.name),
				)
			}
		}

		// Make a new token source if required
		if ts.tokenSource == nil {
			ts.tokenSource = ts.config.TokenSource(ts.ctx, ts.token)
		}

		token, err = ts.tokenSource.Token()
		if err == nil {
			break
		}
		if newErr := maybeWrapOAuthError(err, ts.name); newErr != err {
			err = newErr // Fatal OAuth error
			break
		}
		fs.Debugf(ts.name, "Token refresh failed try %d/%d: %v", i, maxTries, err)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch token: %w", err)
	}
	changed = changed || token.AccessToken != ts.token.AccessToken || token.RefreshToken != ts.token.RefreshToken || token.Expiry != ts.token.Expiry
	ts.token = token
	if changed {
		// Bump on the expiry timer if it is set
		if ts.expiryTimer != nil {
			ts.expiryTimer.Reset(ts.timeToExpiry())
		}
		err = PutToken(ts.name, ts.m, token, false)
		if err != nil {
			return nil, fmt.Errorf("couldn't store token: %w", err)
		}
	}
	return token, nil
}

// Invalidate invalidates the token
func (ts *TokenSource) Invalidate() {
	ts.mu.Lock()
	ts.token.AccessToken = ""
	ts.mu.Unlock()
}

// Expire marks the token as expired
//
// This also marks the token in the config file as expired, if it is the same one
func (ts *TokenSource) Expire() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.token.Expiry = time.Now().Add(time.Hour * (-1)) // expire token
	t, err := GetToken(ts.name, ts.m)
	if err != nil {
		return err
	}
	if t.AccessToken == ts.token.AccessToken {
		err = PutToken(ts.name, ts.m, ts.token, false)
	}
	return err
}

// timeToExpiry returns how long until the token expires
//
// Call with the lock held
func (ts *TokenSource) timeToExpiry() time.Duration {
	t := ts.token
	if t == nil {
		return 0
	}
	if t.Expiry.IsZero() {
		return 3e9 * time.Second // ~95 years
	}
	return time.Until(t.Expiry)
}

// OnExpiry returns a channel which has the time written to it when
// the token expires.  Note that there is only one channel so if
// attaching multiple go routines it will only signal to one of them.
func (ts *TokenSource) OnExpiry() <-chan time.Time {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.expiryTimer == nil {
		ts.expiryTimer = time.NewTimer(ts.timeToExpiry())
	}
	return ts.expiryTimer.C
}

// Check interface satisfied
var _ oauth2.TokenSource = (*TokenSource)(nil)

// Context returns a context with our HTTP Client baked in for oauth2
func Context(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, client)
}

// overrideCredentials sets the ClientID and ClientSecret from the
// config file if they are not blank.
// If any value is overridden, true is returned.
// the origConfig is copied
func overrideCredentials(name string, m configmap.Mapper, origConfig *oauth2.Config) (newConfig *oauth2.Config, changed bool) {
	newConfig = new(oauth2.Config)
	*newConfig = *origConfig
	changed = false
	ClientID, ok := m.Get(config.ConfigClientID)
	if ok && ClientID != "" {
		newConfig.ClientID = ClientID
		// Clear out any existing client secret since the ID changed.
		// (otherwise it's impossible for a config to clear the secret)
		newConfig.ClientSecret = ""
		changed = true
	}
	ClientSecret, ok := m.Get(config.ConfigClientSecret)
	if ok && ClientSecret != "" {
		newConfig.ClientSecret = ClientSecret
		changed = true
	}
	AuthURL, ok := m.Get(config.ConfigAuthURL)
	if ok && AuthURL != "" {
		newConfig.Endpoint.AuthURL = AuthURL
		changed = true
	}
	TokenURL, ok := m.Get(config.ConfigTokenURL)
	if ok && TokenURL != "" {
		newConfig.Endpoint.TokenURL = TokenURL
		changed = true
	}
	return newConfig, changed
}

// NewClientWithBaseClient gets a token from the config file and
// configures a Client with it.  It returns the client and a
// TokenSource which Invalidate may need to be called on.  It uses the
// httpClient passed in as the base client.
func NewClientWithBaseClient(ctx context.Context, name string, m configmap.Mapper, config *oauth2.Config, baseClient *http.Client) (*http.Client, *TokenSource, error) {
	config, _ = overrideCredentials(name, m, config)
	token, err := GetToken(name, m)
	if err != nil {
		return nil, nil, err
	}

	// Set our own http client in the context
	ctx = Context(ctx, baseClient)

	// Wrap the TokenSource in our TokenSource which saves changed
	// tokens in the config file
	ts := &TokenSource{
		name:   name,
		m:      m,
		token:  token,
		config: config,
		ctx:    ctx,
	}
	return oauth2.NewClient(ctx, ts), ts, nil

}

// NewClient gets a token from the config file and configures a Client
// with it.  It returns the client and a TokenSource which Invalidate may need to be called on
func NewClient(ctx context.Context, name string, m configmap.Mapper, oauthConfig *oauth2.Config) (*http.Client, *TokenSource, error) {
	return NewClientWithBaseClient(ctx, name, m, oauthConfig, fshttp.NewClient(ctx))
}

// AuthResult is returned from the web server after authorization
// success or failure
type AuthResult struct {
	OK          bool // Failure or Success?
	Name        string
	Description string
	Code        string
	HelpURL     string
	Form        url.Values // the complete contents of the form
	Err         error      // any underlying error to report
}

// Error satisfies the error interface so AuthResult can be used as an error
func (ar *AuthResult) Error() string {
	status := "Error"
	if ar.OK {
		status = "OK"
	}
	return fmt.Sprintf("%s: %s\nCode: %q\nDescription: %s\nHelp: %s",
		status, ar.Name, ar.Code, ar.Description, ar.HelpURL)
}

// CheckAuthFn is called when a good Auth has been received
type CheckAuthFn func(*oauth2.Config, *AuthResult) error

// Options for the oauth config
type Options struct {
	OAuth2Config *oauth2.Config          // Basic config for oauth2
	NoOffline    bool                    // If set then "access_type=offline" parameter is not passed
	CheckAuth    CheckAuthFn             // When the AuthResult is known the checkAuth function is called if set
	OAuth2Opts   []oauth2.AuthCodeOption // extra oauth2 options
	StateBlankOK bool                    // If set, state returned as "" is deemed to be OK
}

// ConfigOut returns a config item suitable for the backend config
//
// state is the place to return the config to
// oAuth is the config to run the oauth with
func ConfigOut(state string, oAuth *Options) (*fs.ConfigOut, error) {
	return &fs.ConfigOut{
		State: state,
		OAuth: oAuth,
	}, nil
}

// ConfigOAuth does the oauth config specified in the config block
//
// This is called with a state which has pushed on it
//
//	state prefixed with "*oauth"
//	state for oauth to return to
//	state that returned the OAuth when we wish to recall it
//	value that returned the OAuth
func ConfigOAuth(ctx context.Context, name string, m configmap.Mapper, ri *fs.RegInfo, in fs.ConfigIn) (*fs.ConfigOut, error) {
	stateParams, state := fs.StatePop(in.State)

	// Make the next state
	newState := func(state string) string {
		return fs.StatePush(stateParams, state)
	}

	// Recall the Oauth state again by calling the Config with the same input again
	getOAuth := func() (opt *Options, err error) {
		tmpState, _ := fs.StatePop(stateParams)
		tmpState, State := fs.StatePop(tmpState)
		_, Result := fs.StatePop(tmpState)
		out, err := ri.Config(ctx, name, m, fs.ConfigIn{State: State, Result: Result})
		if err != nil {
			return nil, err
		}
		if out.OAuth == nil {
			return nil, errors.New("failed to recall OAuth state")
		}
		opt, ok := out.OAuth.(*Options)
		if !ok {
			return nil, fmt.Errorf("internal error: oauth failed: wrong type in config: %T", out.OAuth)
		}
		if opt.OAuth2Config == nil {
			return nil, errors.New("internal error: oauth failed: OAuth2Config not set")
		}
		return opt, nil
	}

	switch state {
	case "*oauth":
		// See if already have a token
		tokenString, ok := m.Get("token")
		if ok && tokenString != "" {
			return fs.ConfigConfirm(newState("*oauth-confirm"), true, "config_refresh_token", "Already have a token - refresh?")
		}
		return fs.ConfigGoto(newState("*oauth-confirm"))
	case "*oauth-confirm":
		if in.Result == "false" {
			return fs.ConfigGoto(newState("*oauth-done"))
		}
		return fs.ConfigConfirm(newState("*oauth-islocal"), true, "config_is_local", "Use web browser to automatically authenticate rclone with remote?\n * Say Y if the machine running rclone has a web browser you can use\n * Say N if running rclone on a (remote) machine without web browser access\nIf not sure try Y. If Y failed, try N.\n")
	case "*oauth-islocal":
		if in.Result == "true" {
			return fs.ConfigGoto(newState("*oauth-do"))
		}
		return fs.ConfigGoto(newState("*oauth-remote"))
	case "*oauth-remote":
		opt, err := getOAuth()
		if err != nil {
			return nil, err
		}
		if noWebserverNeeded(opt.OAuth2Config) {
			authURL, _, err := getAuthURL(name, m, opt.OAuth2Config, opt)
			if err != nil {
				return nil, err
			}
			return fs.ConfigInput(newState("*oauth-do"), "config_verification_code", fmt.Sprintf("Verification code\n\nGo to this URL, authenticate then paste the code here.\n\n%s\n", authURL))
		}
		var out strings.Builder
		fmt.Fprintf(&out, `For this to work, you will need rclone available on a machine that has
a web browser available.

For more help and alternate methods see: https://rclone.org/remote_setup/

Execute the following on the machine with the web browser (same rclone
version recommended):

`)
		// Find the overridden options
		inM := ri.Options.NonDefault(m)
		delete(inM, fs.ConfigToken) // delete token as we are refreshing it
		for k, v := range inM {
			fs.Debugf(nil, "sending %s = %q", k, v)
		}
		// Encode them into a string
		mCopyString, err := inM.Encode()
		if err != nil {
			return nil, fmt.Errorf("oauthutil authorize encode: %w", err)
		}
		// Write what the user has to do
		if len(mCopyString) > 0 {
			fmt.Fprintf(&out, "\trclone authorize %q %q\n", ri.Name, mCopyString)
		} else {
			fmt.Fprintf(&out, "\trclone authorize %q\n", ri.Name)
		}
		fmt.Fprintln(&out, "\nThen paste the result.")
		return fs.ConfigInput(newState("*oauth-authorize"), "config_token", out.String())
	case "*oauth-authorize":
		// Read the updates to the config
		outM := configmap.Simple{}
		token := oauth2.Token{}
		code := in.Result
		newFormat := true
		err := outM.Decode(code)
		if err != nil {
			newFormat = false
			err = json.Unmarshal([]byte(code), &token)
		}
		if err != nil {
			return fs.ConfigError(newState("*oauth-authorize"), fmt.Sprintf("Couldn't decode response - try again (make sure you are using a matching version of rclone on both sides: %v\n", err))
		}
		// Save the config updates
		if newFormat {
			for k, v := range outM {
				m.Set(k, v)
				fs.Debugf(nil, "received %s = %q", k, v)
			}
		} else {
			m.Set(fs.ConfigToken, code)
		}
		return fs.ConfigGoto(newState("*oauth-done"))
	case "*oauth-do":
		// Make sure we can read the HTML template file if it was specified.
		configTemplateFile, _ := m.Get("config_template_file")
		configTemplateString, _ := m.Get("config_template")

		if configTemplateFile != "" {
			dat, err := os.ReadFile(configTemplateFile)

			if err != nil {
				return nil, fmt.Errorf("failed to read template file: %w", err)
			}

			templateString = string(dat)
		} else if configTemplateString != "" {
			templateString = configTemplateString
		} else {
			templateString = DefaultAuthResponseTemplate
		}
		code := in.Result
		opt, err := getOAuth()
		if err != nil {
			return nil, err
		}
		oauthConfig, changed := overrideCredentials(name, m, opt.OAuth2Config)
		if changed {
			fs.Logf(nil, "Make sure your Redirect URL is set to %q in your custom config.\n", oauthConfig.RedirectURL)
		}
		if code == "" {
			oauthConfig = fixRedirect(oauthConfig)
			code, err = configSetup(ctx, ri.Name, name, m, oauthConfig, opt)
			if err != nil {
				return nil, fmt.Errorf("config failed to refresh token: %w", err)
			}
		}
		err = configExchange(ctx, name, m, oauthConfig, code)
		if err != nil {
			return nil, err
		}
		return fs.ConfigGoto(newState("*oauth-done"))
	case "*oauth-done":
		// Return to the state indicated in the State stack
		_, returnState := fs.StatePop(stateParams)
		return fs.ConfigGoto(returnState)
	}
	return nil, fmt.Errorf("unknown internal oauth state %q", state)
}

func init() {
	// Set the function to avoid circular import
	fs.ConfigOAuth = ConfigOAuth
}

// Return true if can run without a webserver and just entering a code
func noWebserverNeeded(oauthConfig *oauth2.Config) bool {
	return oauthConfig.RedirectURL == TitleBarRedirectURL
}

// get the URL we need to send the user to
func getAuthURL(name string, m configmap.Mapper, oauthConfig *oauth2.Config, opt *Options) (authURL string, state string, err error) {
	oauthConfig, _ = overrideCredentials(name, m, oauthConfig)

	// Make random state
	state, err = random.Password(128)
	if err != nil {
		return "", "", err
	}

	// Generate oauth URL
	opts := opt.OAuth2Opts
	if !opt.NoOffline {
		opts = append(opts, oauth2.AccessTypeOffline)
	}
	authURL = oauthConfig.AuthCodeURL(state, opts...)
	return authURL, state, nil
}

// If TitleBarRedirect is set but we are doing a real oauth, then
// override our redirect URL
func fixRedirect(oauthConfig *oauth2.Config) *oauth2.Config {
	switch oauthConfig.RedirectURL {
	case TitleBarRedirectURL:
		// copy the config and set to use the internal webserver
		configCopy := *oauthConfig
		oauthConfig = &configCopy
		oauthConfig.RedirectURL = RedirectURL
	}
	return oauthConfig
}

// configSetup does the initial creation of the token
//
// If opt is nil it will use the default Options.
//
// It will run an internal webserver to receive the results
func configSetup(ctx context.Context, id, name string, m configmap.Mapper, oauthConfig *oauth2.Config, opt *Options) (string, error) {
	if opt == nil {
		opt = &Options{}
	}
	authorizeNoAutoBrowserValue, ok := m.Get(config.ConfigAuthNoBrowser)
	authorizeNoAutoBrowser := ok && authorizeNoAutoBrowserValue != ""

	authURL, state, err := getAuthURL(name, m, oauthConfig, opt)
	if err != nil {
		return "", err
	}

	// Prepare webserver
	server := newAuthServer(opt, bindAddress, state, authURL)
	err = server.Init()
	if err != nil {
		return "", fmt.Errorf("failed to start auth webserver: %w", err)
	}
	go server.Serve()
	defer server.Stop()
	authURL = "http://" + bindAddress + "/auth?state=" + state

	if !authorizeNoAutoBrowser {
		// Open the URL for the user to visit
		_ = open.Start(authURL)
		fs.Logf(nil, "If your browser doesn't open automatically go to the following link: %s\n", authURL)
	} else {
		fs.Logf(nil, "Please go to the following link: %s\n", authURL)
	}
	fs.Logf(nil, "Log in and authorize rclone for access\n")

	// Read the code via the webserver
	fs.Logf(nil, "Waiting for code...\n")
	auth := <-server.result
	if !auth.OK || auth.Code == "" {
		return "", auth
	}
	fs.Logf(nil, "Got code\n")
	if opt.CheckAuth != nil {
		err = opt.CheckAuth(oauthConfig, auth)
		if err != nil {
			return "", err
		}
	}
	return auth.Code, nil
}

// Exchange the code for a token
func configExchange(ctx context.Context, name string, m configmap.Mapper, oauthConfig *oauth2.Config, code string) error {
	ctx = Context(ctx, fshttp.NewClient(ctx))
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	return PutToken(name, m, token, true)
}

// Local web server for collecting auth
type authServer struct {
	opt         *Options
	state       string
	listener    net.Listener
	bindAddress string
	authURL     string
	server      *http.Server
	result      chan *AuthResult
}

// newAuthServer makes the webserver for collecting auth
func newAuthServer(opt *Options, bindAddress, state, authURL string) *authServer {
	return &authServer{
		opt:         opt,
		state:       state,
		bindAddress: bindAddress,
		authURL:     authURL, // http://host/auth redirects to here
		result:      make(chan *AuthResult, 1),
	}
}

// Receive the auth request
func (s *authServer) handleAuth(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		fs.Debugf(nil, "Ignoring %s request on auth server to %q", req.Method, req.URL.Path)
		http.NotFound(w, req)
		return
	}
	fs.Debugf(nil, "Received %s request on auth server to %q", req.Method, req.URL.Path)

	// Reply with the response to the user and to the channel
	reply := func(status int, res *AuthResult) {
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "text/html")
		var t = template.Must(template.New("authResponse").Parse(templateString))
		if err := t.Execute(w, res); err != nil {
			fs.Debugf(nil, "Could not execute template for web response.")
		}
		s.result <- res
	}

	// Parse the form parameters and save them
	err := req.ParseForm()
	if err != nil {
		reply(http.StatusBadRequest, &AuthResult{
			Name:        "Parse form error",
			Description: err.Error(),
		})
		return
	}

	// get code, error if empty
	code := req.Form.Get("code")
	if code == "" {
		reply(http.StatusBadRequest, &AuthResult{
			Name:        "Auth Error",
			Description: "No code returned by remote server",
		})
		return
	}

	// check state
	state := req.Form.Get("state")
	if state != s.state && !(state == "" && s.opt.StateBlankOK) {
		reply(http.StatusBadRequest, &AuthResult{
			Name:        "Auth state doesn't match",
			Description: fmt.Sprintf("Expecting %q got %q", s.state, state),
		})
		return
	}

	// code OK
	reply(http.StatusOK, &AuthResult{
		OK:   true,
		Code: code,
		Form: req.Form,
	})
}

// Init gets the internal web server ready to receive config details
func (s *authServer) Init() error {
	fs.Debugf(nil, "Starting auth server on %s", s.bindAddress)
	mux := http.NewServeMux()
	s.server = &http.Server{
		Addr:    s.bindAddress,
		Handler: mux,
	}
	s.server.SetKeepAlivesEnabled(false)

	mux.HandleFunc("/auth", func(w http.ResponseWriter, req *http.Request) {
		state := req.FormValue("state")
		if state != s.state {
			fs.Debugf(nil, "State did not match: want %q got %q", s.state, state)
			http.Error(w, "State did not match - please try again", http.StatusForbidden)
			return
		}
		fs.Debugf(nil, "Redirecting browser to: %s", s.authURL)
		http.Redirect(w, req, s.authURL, http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/", s.handleAuth)

	var err error
	s.listener, err = net.Listen("tcp", s.bindAddress)
	if err != nil {
		return err
	}
	return nil
}

// Serve the auth server, doesn't return
func (s *authServer) Serve() {
	err := s.server.Serve(s.listener)
	fs.Debugf(nil, "Closed auth server with error: %v", err)
}

// Stop the auth server by closing its socket
func (s *authServer) Stop() {
	fs.Debugf(nil, "Closing auth server")
	close(s.result)
	_ = s.listener.Close()

	// close the server
	_ = s.server.Close()
}
