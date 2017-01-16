package oauthutil

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/net/context"
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

// getToken returns the token saved in the config file under
// section name.
func getToken(name string) (*oauth2.Token, error) {
	tokenString := fs.ConfigFileGet(name, fs.ConfigToken)
	if tokenString == "" {
		return nil, errors.New("empty token found - please run rclone config again")
	}
	token := new(oauth2.Token)
	err := json.Unmarshal([]byte(tokenString), token)
	if err != nil {
		return nil, err
	}
	// if has data then return it
	if token.AccessToken != "" && token.RefreshToken != "" {
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
	err = putToken(name, token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// putToken stores the token in the config file
//
// This saves the config file if it changes
func putToken(name string, token *oauth2.Token) error {
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return err
	}
	tokenString := string(tokenBytes)
	old := fs.ConfigFileGet(name, fs.ConfigToken)
	if tokenString != old {
		err = fs.ConfigSetValueAndSave(name, fs.ConfigToken, tokenString)
		if err != nil {
			fs.ErrorLog(nil, "Failed to save new token in config file: %v", err)
		} else {
			fs.Debug(name, "Saved new token in config file")
		}
	}
	return nil
}

// TokenSource stores updated tokens in the config file
type TokenSource struct {
	mu          sync.Mutex
	name        string
	tokenSource oauth2.TokenSource
	token       *oauth2.Token
	config      *oauth2.Config
	ctx         context.Context
	expiryTimer *time.Timer // signals whenever the token expires
}

// Token returns a token or an error.
// Token must be safe for concurrent use by multiple goroutines.
// The returned Token must not be modified.
//
// This saves the token in the config file if it has changed
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Make a new token source if required
	if ts.tokenSource == nil {
		ts.tokenSource = ts.config.TokenSource(ts.ctx, ts.token)
	}

	token, err := ts.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	changed := *token != *ts.token
	ts.token = token
	if changed {
		// Bump on the expiry timer if it is set
		if ts.expiryTimer != nil {
			ts.expiryTimer.Reset(ts.timeToExpiry())
		}
		err = putToken(ts.name, token)
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
func Context() context.Context {
	return context.WithValue(nil, oauth2.HTTPClient, fs.Config.Client())
}

// overrideCredentials sets the ClientID and ClientSecret from the
// config file if they are not blank.
// If any value is overridden, true is returned.
// the origConfig is copied
func overrideCredentials(name string, origConfig *oauth2.Config) (config *oauth2.Config, changed bool) {
	config = new(oauth2.Config)
	*config = *origConfig
	changed = false
	ClientID := fs.ConfigFileGet(name, fs.ConfigClientID)
	if ClientID != "" {
		config.ClientID = ClientID
		changed = true
	}
	ClientSecret := fs.ConfigFileGet(name, fs.ConfigClientSecret)
	if ClientSecret != "" {
		config.ClientSecret = ClientSecret
		changed = true
	}
	return config, changed
}

// NewClient gets a token from the config file and configures a Client
// with it.  It returns the client and a TokenSource which Invalidate may need to be called on
func NewClient(name string, config *oauth2.Config) (*http.Client, *TokenSource, error) {
	config, _ = overrideCredentials(name, config)
	token, err := getToken(name)
	if err != nil {
		return nil, nil, err
	}

	// Set our own http client in the context
	ctx := Context()

	// Wrap the TokenSource in our TokenSource which saves changed
	// tokens in the config file
	ts := &TokenSource{
		name:   name,
		token:  token,
		config: config,
		ctx:    ctx,
	}
	return oauth2.NewClient(ctx, ts), ts, nil

}

// Config does the initial creation of the token
//
// It may run an internal webserver to receive the results
func Config(id, name string, config *oauth2.Config) error {
	config, changed := overrideCredentials(name, config)
	automatic := fs.ConfigFileGet(name, fs.ConfigAutomatic) != ""

	// See if already have a token
	tokenString := fs.ConfigFileGet(name, "token")
	if tokenString != "" {
		fmt.Printf("Already have a token - refresh?\n")
		if !fs.Confirm() {
			return nil
		}
	}

	// Detect whether we should use internal web server
	useWebServer := false
	switch config.RedirectURL {
	case RedirectURL, RedirectPublicURL, RedirectLocalhostURL:
		useWebServer = true
		if automatic {
			break
		}
		fmt.Printf("Use auto config?\n")
		fmt.Printf(" * Say Y if not sure\n")
		fmt.Printf(" * Say N if you are working on a remote or headless machine\n")
		auto := fs.Confirm()
		if !auto {
			fmt.Printf("For this to work, you will need rclone available on a machine that has a web browser available.\n")
			fmt.Printf("Execute the following on your machine:\n")
			if changed {
				fmt.Printf("\trclone authorize %q %q %q\n", id, config.ClientID, config.ClientSecret)
			} else {
				fmt.Printf("\trclone authorize %q\n", id)
			}
			fmt.Println("Then paste the result below:")
			code := ""
			for code == "" {
				fmt.Printf("result> ")
				code = strings.TrimSpace(fs.ReadLine())
			}
			token := &oauth2.Token{}
			err := json.Unmarshal([]byte(code), token)
			if err != nil {
				return err
			}
			return putToken(name, token)
		}
	case TitleBarRedirectURL:
		useWebServer = automatic
		if !automatic {
			fmt.Printf("Use auto config?\n")
			fmt.Printf(" * Say Y if not sure\n")
			fmt.Printf(" * Say N if you are working on a remote or headless machine or Y didn't work\n")
			useWebServer = fs.Confirm()
		}
		if useWebServer {
			// copy the config and set to use the internal webserver
			configCopy := *config
			config = &configCopy
			config.RedirectURL = RedirectURL
		}
	}

	// Make random state
	stateBytes := make([]byte, 16)
	_, err := rand.Read(stateBytes)
	if err != nil {
		return err
	}
	state := fmt.Sprintf("%x", stateBytes)
	authURL := config.AuthCodeURL(state)

	// Prepare webserver
	server := authServer{
		state:       state,
		bindAddress: bindAddress,
		authURL:     authURL,
	}
	if useWebServer {
		server.code = make(chan string, 1)
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
		if authCode != "" {
			fmt.Printf("Got code\n")
		} else {
			return errors.New("failed to get code")
		}
	} else {
		// Read the code, and exchange it for a token.
		fmt.Printf("Enter verification code> ")
		authCode = fs.ReadLine()
	}
	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		return errors.Wrap(err, "failed to get token")
	}

	// Print code if we do automatic retrieval
	if automatic {
		result, err := json.Marshal(token)
		if err != nil {
			return errors.Wrap(err, "failed to marshal token")
		}
		fmt.Printf("Paste the following into your remote machine --->\n%s\n<---End paste", result)
	}
	return putToken(name, token)
}

// Local web server for collecting auth
type authServer struct {
	state       string
	listener    net.Listener
	bindAddress string
	code        chan string
	authURL     string
}

// startWebServer runs an internal web server to receive config details
func (s *authServer) Start() {
	fs.Debug(nil, "Starting auth server on %s", s.bindAddress)
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    s.bindAddress,
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(false)
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
		fs.Debug(nil, "Received request on auth server")
		code := req.FormValue("code")
		if code != "" {
			state := req.FormValue("state")
			if state != s.state {
				fs.Debug(nil, "State did not match: want %q got %q", s.state, state)
				fmt.Fprintf(w, "<h1>Failure</h1>\n<p>Auth state doesn't match</p>")
			} else {
				fs.Debug(nil, "Successfully got code")
				if s.code != nil {
					fmt.Fprintf(w, "<h1>Success</h1>\n<p>Go back to rclone to continue</p>")
					s.code <- code
				} else {
					fmt.Fprintf(w, "<h1>Success</h1>\n<p>Cut and paste this code into rclone: <code>%s</code></p>", code)
				}
			}
			return
		}
		fs.Debug(nil, "No code found on request")
		w.WriteHeader(500)
		fmt.Fprintf(w, "<h1>Failed!</h1>\nNo code found returned by remote server.")

	})

	var err error
	s.listener, err = net.Listen("tcp", s.bindAddress)
	if err != nil {
		log.Fatalf("Failed to start auth webserver: %v", err)
	}
	err = server.Serve(s.listener)
	fs.Debug(nil, "Closed auth server with error: %v", err)
}

func (s *authServer) Stop() {
	fs.Debug(nil, "Closing auth server")
	if s.code != nil {
		close(s.code)
		s.code = nil
	}
	_ = s.listener.Close()
}
