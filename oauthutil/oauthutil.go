package oauthutil

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const (
	// ConfigToken is the key used to store the token under
	ConfigToken = "token"

	// ConfigClientID is the config key used to store the client id
	ConfigClientID = "client_id"

	// ConfigClientSecret is the config key used to store the client secret
	ConfigClientSecret = "client_secret"

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
	tokenString, err := fs.ConfigFile.GetValue(string(name), ConfigToken)
	if err != nil {
		return nil, err
	}
	if tokenString == "" {
		return nil, fmt.Errorf("Empty token found - please run rclone config again")
	}
	token := new(oauth2.Token)
	err = json.Unmarshal([]byte(tokenString), token)
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
	old := fs.ConfigFile.MustValue(name, ConfigToken)
	if tokenString != old {
		fs.ConfigFile.SetValue(name, ConfigToken, tokenString)
		fs.SaveConfig()
		fs.Debug(name, "Saving new token in config file")
	}
	return nil
}

// tokenSource stores updated tokens in the config file
type tokenSource struct {
	Name        string
	TokenSource oauth2.TokenSource
	OldToken    oauth2.Token
}

// Token returns a token or an error.
// Token must be safe for concurrent use by multiple goroutines.
// The returned Token must not be modified.
//
// This saves the token in the config file if it has changed
func (ts *tokenSource) Token() (*oauth2.Token, error) {
	token, err := ts.TokenSource.Token()
	if err != nil {
		return nil, err
	}
	if *token != ts.OldToken {
		err = putToken(ts.Name, token)
		if err != nil {
			return nil, err
		}
	}
	return token, nil
}

// Check interface satisfied
var _ oauth2.TokenSource = (*tokenSource)(nil)

// Context returns a context with our HTTP Client baked in for oauth2
func Context() context.Context {
	return context.WithValue(nil, oauth2.HTTPClient, fs.Config.Client())
}

// overrideCredentials sets the ClientID and ClientSecret from the
// config file if they are not blank
func overrideCredentials(name string, config *oauth2.Config) {
	ClientID := fs.ConfigFile.MustValue(name, ConfigClientID)
	if ClientID != "" {
		config.ClientID = ClientID
	}
	ClientSecret := fs.ConfigFile.MustValue(name, ConfigClientSecret)
	if ClientSecret != "" {
		config.ClientSecret = ClientSecret
	}
}

// NewClient gets a token from the config file and configures a Client
// with it
func NewClient(name string, config *oauth2.Config) (*http.Client, error) {
	overrideCredentials(name, config)
	token, err := getToken(name)
	if err != nil {
		return nil, err
	}

	// Set our own http client in the context
	ctx := Context()

	// Wrap the TokenSource in our TokenSource which saves changed
	// tokens in the config file
	ts := &tokenSource{
		Name:        name,
		OldToken:    *token,
		TokenSource: config.TokenSource(ctx, token),
	}
	return oauth2.NewClient(ctx, ts), nil

}

// Config does the initial creation of the token
//
// It may run an internal webserver to receive the results
func Config(name string, config *oauth2.Config) error {
	overrideCredentials(name, config)
	// See if already have a token
	tokenString := fs.ConfigFile.MustValue(name, "token")
	if tokenString != "" {
		fmt.Printf("Already have a token - refresh?\n")
		if !fs.Confirm() {
			return nil
		}
	}

	// Detect whether we should use internal web server
	useWebServer := false
	switch config.RedirectURL {
	case RedirectURL, RedirectPublicURL:
		useWebServer = true
	case TitleBarRedirectURL:
		fmt.Printf("Use auto config?\n")
		fmt.Printf(" * Say Y if not sure\n")
		fmt.Printf(" * Say N if you are working on a remote or headless machine or Y didn't work\n")
		useWebServer = fs.Confirm()
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
			return fmt.Errorf("Failed to get code")
		}
	} else {
		// Read the code, and exchange it for a token.
		fmt.Printf("Enter verification code> ")
		authCode = fs.ReadLine()
	}
	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		return fmt.Errorf("Failed to get token: %v", err)
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
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "", 404)
		return
	})
	mux.HandleFunc("/auth", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, s.authURL, 307)
		return
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
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
		fmt.Fprintf(w, "<h1>Failed!</h1>\nNo code found.")
		http.Error(w, "", 500)
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
