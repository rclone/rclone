package oauthutil

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fshttp"
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

	// AuthResponse is a template to handle the redirect URL for oauth requests
	AuthResponse = `<!DOCTYPE html>
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
Error: {{ .AuthError.Name }}<br>
{{ if .AuthError.Description }}Description: {{ .AuthError.Description }}<br>{{ end }}
{{ if .AuthError.Code }}Code: {{ .AuthError.Code }}<br>{{ end }}
{{ if .AuthError.HelpURL }}Look here for help: <a href="{{ .AuthError.HelpURL }}">{{ .AuthError.HelpURL }}</a><br>{{ end }}
{{ else }}
	{{ if .Code }}
Please copy this code into rclone:
{{ .Code }}
	{{ else }}
All done. Please go back to rclone.
	{{ end }}
{{ end }}
</pre>
</body>
</html>
`
)

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
		return nil, errors.New("empty token found - please run rclone config again")
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
		err = config.SetValueAndSave(name, config.ConfigToken, tokenString)
		if newSection && err != nil {
			fs.Debugf(name, "Added new token to config, still needs to be saved")
		} else if err != nil {
			fs.Errorf(nil, "Failed to save new token in config file: %v", err)
		} else {
			fs.Debugf(name, "Saved new token in config file")
		}
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
	tokenString, err := config.FileGetFresh(ts.name, config.ConfigToken)
	if err != nil {
		fs.Debugf(ts.name, "Failed to read token out of config file: %v", err)
		return false
	}
	newToken := new(oauth2.Token)
	err = json.Unmarshal([]byte(tokenString), newToken)
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
		return nil, err
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
			return nil, err
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
		return 3E9 * time.Second // ~95 years
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
func Context(client *http.Client) context.Context {
	return context.WithValue(context.Background(), oauth2.HTTPClient, client)
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
func NewClientWithBaseClient(name string, m configmap.Mapper, config *oauth2.Config, baseClient *http.Client) (*http.Client, *TokenSource, error) {
	config, _ = overrideCredentials(name, m, config)
	token, err := GetToken(name, m)
	if err != nil {
		return nil, nil, err
	}

	// Set our own http client in the context
	ctx := Context(baseClient)

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
func NewClient(name string, m configmap.Mapper, oauthConfig *oauth2.Config) (*http.Client, *TokenSource, error) {
	return NewClientWithBaseClient(name, m, oauthConfig, fshttp.NewClient(fs.Config))
}

// Config does the initial creation of the token
//
// It may run an internal webserver to receive the results
func Config(id, name string, m configmap.Mapper, config *oauth2.Config, opts ...oauth2.AuthCodeOption) error {
	return doConfig(id, name, m, nil, config, true, opts)
}

// ConfigNoOffline does the same as Config but does not pass the
// "access_type=offline" parameter.
func ConfigNoOffline(id, name string, m configmap.Mapper, config *oauth2.Config, opts ...oauth2.AuthCodeOption) error {
	return doConfig(id, name, m, nil, config, false, opts)
}

// ConfigErrorCheck does the same as Config, but allows the backend to pass a error handling function
// This function gets called with the request made to rclone as a parameter if no code was found
func ConfigErrorCheck(id, name string, m configmap.Mapper, errorHandler func(*http.Request) AuthError, config *oauth2.Config, opts ...oauth2.AuthCodeOption) error {
	return doConfig(id, name, m, errorHandler, config, true, opts)
}

func doConfig(id, name string, m configmap.Mapper, errorHandler func(*http.Request) AuthError, oauthConfig *oauth2.Config, offline bool, opts []oauth2.AuthCodeOption) error {
	oauthConfig, changed := overrideCredentials(name, m, oauthConfig)
	authorizeOnlyValue, ok := m.Get(config.ConfigAuthorize)
	authorizeOnly := ok && authorizeOnlyValue != "" // set if being run by "rclone authorize"

	// See if already have a token
	tokenString, ok := m.Get("token")
	if ok && tokenString != "" {
		fmt.Printf("Already have a token - refresh?\n")
		if !config.ConfirmWithConfig(m, "config_refresh_token", true) {
			return nil
		}
	}

	// Ask the user whether they are using a local machine
	isLocal := func() bool {
		fmt.Printf("Use auto config?\n")
		fmt.Printf(" * Say Y if not sure\n")
		fmt.Printf(" * Say N if you are working on a remote or headless machine\n")
		return config.ConfirmWithConfig(m, "config_is_local", true)
	}

	// Detect whether we should use internal web server
	useWebServer := false
	switch oauthConfig.RedirectURL {
	case RedirectURL, RedirectPublicURL, RedirectLocalhostURL:
		if changed {
			fmt.Printf("Make sure your Redirect URL is set to %q in your custom config.\n", oauthConfig.RedirectURL)
		}
		useWebServer = true
		if authorizeOnly {
			break
		}
		if !isLocal() {
			fmt.Printf("For this to work, you will need rclone available on a machine that has a web browser available.\n")
			fmt.Printf("Execute the following on your machine (same rclone version recommended) :\n")
			if changed {
				fmt.Printf("\trclone authorize %q %q %q\n", id, oauthConfig.ClientID, oauthConfig.ClientSecret)
			} else {
				fmt.Printf("\trclone authorize %q\n", id)
			}
			fmt.Println("Then paste the result below:")
			code := ""
			for code == "" {
				fmt.Printf("result> ")
				code = strings.TrimSpace(config.ReadLine())
			}
			token := &oauth2.Token{}
			err := json.Unmarshal([]byte(code), token)
			if err != nil {
				return err
			}
			return PutToken(name, m, token, true)
		}
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
	}

	// Make random state
	stateBytes := make([]byte, 16)
	_, err := rand.Read(stateBytes)
	if err != nil {
		return err
	}
	state := fmt.Sprintf("%x", stateBytes)
	if offline {
		opts = append(opts, oauth2.AccessTypeOffline)
	}
	authURL := oauthConfig.AuthCodeURL(state, opts...)

	// Prepare webserver
	server := authServer{
		state:        state,
		bindAddress:  bindAddress,
		authURL:      authURL,
		errorHandler: errorHandler,
	}
	if useWebServer {
		server.code = make(chan string, 1)
		server.err = make(chan error, 1)
		go server.Start()
		defer server.Stop()
		authURL = "http://" + bindAddress + "/auth"
	}

	// Generate a URL for the user to visit for authorization.
	_ = open.Start(authURL)
	fmt.Printf("If your browser doesn't open automatically go to the following link: %s\n", authURL)
	fmt.Printf("Log in and authorize rclone for access\n")

	var authCode string
	if useWebServer {
		// Read the code, and exchange it for a token.
		fmt.Printf("Waiting for code...\n")
		authCode = <-server.code
		authError := <-server.err
		if authCode != "" {
			fmt.Printf("Got code\n")
		} else {
			if authError != nil {
				return authError
			}
			return errors.New("failed to get code")
		}
	} else {
		// Read the code, and exchange it for a token.
		fmt.Printf("Enter verification code> ")
		authCode = config.ReadLine()
	}
	token, err := oauthConfig.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		return errors.Wrap(err, "failed to get token")
	}

	// Print code if we do automatic retrieval
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
	state        string
	listener     net.Listener
	bindAddress  string
	code         chan string
	err          chan error
	authURL      string
	server       *http.Server
	errorHandler func(*http.Request) AuthError
}

// AuthError gets returned by the backend's errorHandler function
type AuthError struct {
	Name        string
	Description string
	Code        string
	HelpURL     string
}

// AuthResponseData can fill the AuthResponse template
type AuthResponseData struct {
	OK   bool   // Failure or Success?
	Code string // code to paste into rclone config
	AuthError
}

// startWebServer runs an internal web server to receive config details
func (s *authServer) Start() {
	fs.Debugf(nil, "Starting auth server on %s", s.bindAddress)
	mux := http.NewServeMux()
	s.server = &http.Server{
		Addr:    s.bindAddress,
		Handler: mux,
	}
	s.server.SetKeepAlivesEnabled(false)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "", 404)
		return
	})
	mux.HandleFunc("/auth", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, s.authURL, http.StatusTemporaryRedirect)
		return
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fs.Debugf(nil, "Received request on auth server")
		code := req.FormValue("code")
		var err error
		var t = template.Must(template.New("authResponse").Parse(AuthResponse))
		resp := AuthResponseData{AuthError: AuthError{}}
		if code != "" {
			state := req.FormValue("state")
			if state != s.state {
				fs.Debugf(nil, "State did not match: want %q got %q", s.state, state)
				resp.OK = false
				resp.AuthError = AuthError{
					Name: "Auth State doesn't match",
				}
			} else {
				fs.Debugf(nil, "Successfully got code")
				resp.OK = true
				if s.code == nil {
					resp.Code = code
				}
			}
		} else {
			fs.Debugf(nil, "No code found on request")
			var authError AuthError
			if s.errorHandler == nil {
				authError = AuthError{
					Name:        "Auth Error",
					Description: "No code found returned by remote server.",
				}
			} else {
				authError = s.errorHandler(req)
			}
			err = fmt.Errorf("Error: %s\nCode: %s\nDescription: %s\nHelp: %s",
				authError.Name, authError.Code, authError.Description, authError.HelpURL)
			resp.OK = false
			resp.AuthError = authError
			w.WriteHeader(500)
		}
		if err := t.Execute(w, resp); err != nil {
			fs.Debugf(nil, "Could not execute template for web response.")
		}
		if s.code != nil {
			s.code <- code
			s.err <- err
		}
	})

	var err error
	s.listener, err = net.Listen("tcp", s.bindAddress)
	if err != nil {
		log.Fatalf("Failed to start auth webserver: %v", err)
	}
	err = s.server.Serve(s.listener)
	fs.Debugf(nil, "Closed auth server with error: %v", err)
}

func (s *authServer) Stop() {
	fs.Debugf(nil, "Closing auth server")
	if s.code != nil {
		close(s.code)
		s.code = nil
	}
	_ = s.listener.Close()

	// close the server
	_ = s.server.Close()
}
