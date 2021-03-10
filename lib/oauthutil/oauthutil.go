package oauthutil

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/random"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
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

	// AuthResponseTemplate is a template to handle the redirect URL for oauth requests
	AuthResponseTemplate = `<!DOCTYPE html>
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
	Name: config.ConfigClientID,
	Help: "OAuth Client Id\nLeave blank normally.",
}, {
	Name: config.ConfigClientSecret,
	Help: "OAuth Client Secret\nLeave blank normally.",
}, {
	Name:     config.ConfigToken,
	Help:     "OAuth Access Token as a JSON blob.",
	Advanced: true,
}, {
	Name:     config.ConfigAuthURL,
	Help:     "Auth server URL.\nLeave blank to use the provider defaults.",
	Advanced: true,
}, {
	Name:     config.ConfigTokenURL,
	Help:     "Token server url.\nLeave blank to use the provider defaults.",
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
		return nil, errors.Errorf("empty token found - please run \"rclone config reconnect %s:\"", name)
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

// If token has expired then first try re-reading it from the config
// file in case a concurrently running rclone has updated it already
func (ts *TokenSource) reReadToken() bool {
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
		return false
	}
	fs.Debugf(ts.name, "Loaded fresh token from config file")
	ts.token = newToken
	ts.tokenSource = nil // invalidate since we changed the token
	return true
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
		fs.Debugf(ts.name, "Token refresh failed try %d/%d: %v", i, maxTries, err)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't fetch token - maybe it has expired? - refresh with \"rclone config reconnect %s:\"", ts.name)
	}
	changed = changed || (*token != *ts.token)
	ts.token = token
	if changed {
		// Bump on the expiry timer if it is set
		if ts.expiryTimer != nil {
			ts.expiryTimer.Reset(ts.timeToExpiry())
		}
		err = PutToken(ts.name, ts.m, token, false)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't store token")
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
	return t.Expiry.Sub(time.Now())
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
	NoOffline    bool                    // If set then "access_type=offline" parameter is not passed
	CheckAuth    CheckAuthFn             // When the AuthResult is known the checkAuth function is called if set
	OAuth2Opts   []oauth2.AuthCodeOption // extra oauth2 options
	StateBlankOK bool                    // If set, state returned as "" is deemed to be OK
}

// Config does the initial creation of the token
//
// If opt is nil it will use the default Options
//
// It may run an internal webserver to receive the results
func Config(ctx context.Context, id, name string, m configmap.Mapper, oauthConfig *oauth2.Config, opt *Options) error {
	if opt == nil {
		opt = &Options{}
	}
	oauthConfig, changed := overrideCredentials(name, m, oauthConfig)
	authorizeOnlyValue, ok := m.Get(config.ConfigAuthorize)
	authorizeOnly := ok && authorizeOnlyValue != "" // set if being run by "rclone authorize"
	authorizeNoAutoBrowserValue, ok := m.Get(config.ConfigAuthNoBrowser)
	authorizeNoAutoBrowser := ok && authorizeNoAutoBrowserValue != ""

	// See if already have a token
	tokenString, ok := m.Get("token")
	if ok && tokenString != "" {
		fmt.Printf("Already have a token - refresh?\n")
		if !config.ConfirmWithConfig(ctx, m, "config_refresh_token", true) {
			return nil
		}
	}

	// Ask the user whether they are using a local machine
	isLocal := func() bool {
		fmt.Printf("Use auto config?\n")
		fmt.Printf(" * Say Y if not sure\n")
		fmt.Printf(" * Say N if you are working on a remote or headless machine\n")
		return config.ConfirmWithConfig(ctx, m, "config_is_local", true)
	}

	// Detect whether we should use internal web server
	useWebServer := false
	switch oauthConfig.RedirectURL {
	case TitleBarRedirectURL:
		useWebServer = authorizeOnly
		if !authorizeOnly {
			useWebServer = isLocal()
		}
		if useWebServer {
			// copy the config and set to use the internal webserver
			configCopy := *oauthConfig
			oauthConfig = &configCopy
			oauthConfig.RedirectURL = RedirectURL
		}
	default:
		if changed {
			fmt.Printf("Make sure your Redirect URL is set to %q in your custom config.\n", oauthConfig.RedirectURL)
		}
		useWebServer = true
		if authorizeOnly {
			break
		}
		if !isLocal() {
			fmt.Printf(`For this to work, you will need rclone available on a machine that has
a web browser available.

For more help and alternate methods see: https://rclone.org/remote_setup/

Execute the following on the machine with the web browser (same rclone
version recommended):

`)
			if changed {
				fmt.Printf("\trclone authorize %q -- %q %q\n", id, oauthConfig.ClientID, oauthConfig.ClientSecret)
			} else {
				fmt.Printf("\trclone authorize %q\n", id)
			}
			fmt.Println("\nThen paste the result below:")
			code := config.ReadNonEmptyLine("result> ")
			token := &oauth2.Token{}
			err := json.Unmarshal([]byte(code), token)
			if err != nil {
				return err
			}
			return PutToken(name, m, token, true)
		}
	}

	// Make random state
	state, err := random.Password(128)
	if err != nil {
		return err
	}

	// Generate oauth URL
	opts := opt.OAuth2Opts
	if !opt.NoOffline {
		opts = append(opts, oauth2.AccessTypeOffline)
	}
	authURL := oauthConfig.AuthCodeURL(state, opts...)

	// Prepare webserver if needed
	var server *authServer
	if useWebServer {
		server = newAuthServer(opt, bindAddress, state, authURL)
		err := server.Init()
		if err != nil {
			return errors.Wrap(err, "failed to start auth webserver")
		}
		go server.Serve()
		defer server.Stop()
		authURL = "http://" + bindAddress + "/auth?state=" + state
	}

	if !authorizeNoAutoBrowser && oauthConfig.RedirectURL != TitleBarRedirectURL {
		// Open the URL for the user to visit
		_ = open.Start(authURL)
		fmt.Printf("If your browser doesn't open automatically go to the following link: %s\n", authURL)
	} else {
		fmt.Printf("Please go to the following link: %s\n", authURL)
	}
	fmt.Printf("Log in and authorize rclone for access\n")

	// Read the code via the webserver or manually
	var auth *AuthResult
	if useWebServer {
		fmt.Printf("Waiting for code...\n")
		auth = <-server.result
		if !auth.OK || auth.Code == "" {
			return auth
		}
		fmt.Printf("Got code\n")
		if opt.CheckAuth != nil {
			err = opt.CheckAuth(oauthConfig, auth)
			if err != nil {
				return err
			}
		}
	} else {
		auth = &AuthResult{
			Code: config.ReadNonEmptyLine("Enter verification code> "),
		}
	}

	// Exchange the code for a token
	ctx = Context(ctx, fshttp.NewClient(ctx))
	token, err := oauthConfig.Exchange(ctx, auth.Code)
	if err != nil {
		return errors.Wrap(err, "failed to get token")
	}

	// Print code if we are doing a manual auth
	if authorizeOnly {
		result, err := json.Marshal(token)
		if err != nil {
			return errors.Wrap(err, "failed to marshal token")
		}
		fmt.Printf("Paste the following into your remote machine --->\n%s\n<---End paste\n", result)
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
	fs.Debugf(nil, "Received %s request on auth server to %q", req.Method, req.URL.Path)

	// Reply with the response to the user and to the channel
	reply := func(status int, res *AuthResult) {
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "text/html")
		var t = template.Must(template.New("authResponse").Parse(AuthResponseTemplate))
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

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "", http.StatusNotFound)
		return
	})
	mux.HandleFunc("/auth", func(w http.ResponseWriter, req *http.Request) {
		state := req.FormValue("state")
		if state != s.state {
			fs.Debugf(nil, "State did not match: want %q got %q", s.state, state)
			http.Error(w, "State did not match - please try again", http.StatusForbidden)
			return
		}
		fs.Debugf(nil, "Redirecting browser to: %s", s.authURL)
		http.Redirect(w, req, s.authURL, http.StatusTemporaryRedirect)
		return
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
