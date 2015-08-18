package oauthutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ncw/rclone/fs"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// configKey is the key used to store the token under
const configKey = "token"

// TitleBarRedirectURL is the OAuth2 redirect URL to use when the authorization
// code should be returned in the title bar of the browser, with the page text
// prompting the user to copy the code and paste it in the application.
const TitleBarRedirectURL = "urn:ietf:wg:oauth:2.0:oob"

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
	tokenString, err := fs.ConfigFile.GetValue(string(name), configKey)
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
	old := fs.ConfigFile.MustValue(name, configKey)
	if tokenString != old {
		fs.ConfigFile.SetValue(name, configKey, tokenString)
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
		putToken(ts.Name, token)
	}
	return token, nil
}

// Check interface satisfied
var _ oauth2.TokenSource = (*tokenSource)(nil)

// Context returns a context with our HTTP Client baked in for oauth2
func Context() context.Context {
	return context.WithValue(nil, oauth2.HTTPClient, fs.Config.Client())
}

// NewClient gets a token from the config file and configures a Client
// with it
func NewClient(name string, config *oauth2.Config) (*http.Client, error) {
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
func Config(name string, config *oauth2.Config) error {
	// See if already have a token
	tokenString := fs.ConfigFile.MustValue(name, "token")
	if tokenString != "" {
		fmt.Printf("Already have a token - refresh?\n")
		if !fs.Confirm() {
			return nil
		}
	}

	// Generate a URL for the user to visit for authorization.
	authUrl := config.AuthCodeURL("state")
	fmt.Printf("Go to the following link in your browser\n")
	fmt.Printf("%s\n", authUrl)
	fmt.Printf("Log in, then type paste the token that is returned in the browser here\n")

	// Read the code, and exchange it for a token.
	fmt.Printf("Enter verification code> ")
	authCode := fs.ReadLine()
	token, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		return fmt.Errorf("Failed to get token: %v", err)
	}
	return putToken(name, token)
}
