// Common authentication between Google Drive and Google Cloud Storage
package googleauth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"code.google.com/p/goauth2/oauth"
	"github.com/ncw/rclone/fs"
)

// A token cache to save the token in the config file section named
type TokenCache string

// Get the token from the config file - returns an error if it isn't present
func (name TokenCache) Token() (*oauth.Token, error) {
	tokenString, err := fs.ConfigFile.GetValue(string(name), "token")
	if err != nil {
		return nil, err
	}
	if tokenString == "" {
		return nil, fmt.Errorf("Empty token found - please reconfigure")
	}
	token := new(oauth.Token)
	err = json.Unmarshal([]byte(tokenString), token)
	if err != nil {
		return nil, err
	}
	return token, nil

}

// Save the token to the config file
//
// This saves the config file if it changes
func (name TokenCache) PutToken(token *oauth.Token) error {
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return err
	}
	tokenString := string(tokenBytes)
	old := fs.ConfigFile.MustValue(string(name), "token")
	if tokenString != old {
		fs.ConfigFile.SetValue(string(name), "token", tokenString)
		fs.SaveConfig()
	}
	return nil
}

// Auth contains information to authenticate an app against google services
type Auth struct {
	Scope               string
	DefaultClientId     string
	DefaultClientSecret string
}

// Makes a new transport using authorisation from the config
//
// Doesn't have a token yet
func (auth *Auth) newTransport(name string) (*oauth.Transport, error) {
	clientId := fs.ConfigFile.MustValue(name, "client_id")
	if clientId == "" {
		clientId = auth.DefaultClientId
	}
	clientSecret := fs.ConfigFile.MustValue(name, "client_secret")
	if clientSecret == "" {
		clientSecret = auth.DefaultClientSecret
	}

	// Settings for authorization.
	var config = &oauth.Config{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Scope:        auth.Scope,
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		TokenCache:   TokenCache(name),
	}

	t := &oauth.Transport{
		Config:    config,
		Transport: http.DefaultTransport,
	}

	return t, nil
}

// Makes a new transport using authorisation from the config with token
func (auth *Auth) NewTransport(name string) (*oauth.Transport, error) {
	t, err := auth.newTransport(name)
	if err != nil {
		return nil, err
	}

	// Try to pull the token from the cache; if this fails, we need to get one.
	token, err := t.Config.TokenCache.Token()
	if err != nil {
		return nil, fmt.Errorf("Failed to get token: %s", err)
	}
	t.Token = token

	return t, nil
}

// Configuration helper - called after the user has put in the defaults
func (auth *Auth) Config(name string) {
	// See if already have a token
	tokenString := fs.ConfigFile.MustValue(name, "token")
	if tokenString != "" {
		fmt.Printf("Already have a token - refresh?\n")
		if !fs.Confirm() {
			return
		}
	}

	// Get a transport
	t, err := auth.newTransport(name)
	if err != nil {
		log.Fatalf("Couldn't make transport: %v", err)
	}

	// Generate a URL for the user to visit for authorization.
	authUrl := t.Config.AuthCodeURL("state")
	fmt.Printf("Go to the following link in your browser\n")
	fmt.Printf("%s\n", authUrl)
	fmt.Printf("Log in, then type paste the token that is returned in the browser here\n")

	// Read the code, and exchange it for a token.
	fmt.Printf("Enter verification code> ")
	authCode := fs.ReadLine()
	_, err = t.Exchange(authCode)
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}
}
