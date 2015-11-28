// Package hubic provides an interface to the Hubic object storage
// system.
package hubic

// This uses the normal swift mechanism to update the credentials and
// ignores the expires field returned by the Hubic API.  This may need
// to be revisted after some actual experience.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/ncw/rclone/swift"
	swiftLib "github.com/ncw/swift"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID     = "api_hubic_svWP970PvSWbw5G3PzrAqZ6X2uHeZBPI"
	rcloneClientSecret = "8MrG3pjWyJya4OnO9ZTS4emI+9fa1ouPgvfD2MbTzfDYvO/H5czFxsTXtcji4/Hz3snz8/CrzMzlxvP9//Ty/Q=="
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: []string{
			"credentials.r", // Read Openstack credentials
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://api.hubic.com/oauth/auth/",
			TokenURL: "https://api.hubic.com/oauth/token/",
		},
		ClientID:     rcloneClientID,
		ClientSecret: fs.Reveal(rcloneClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.Info{
		Name:  "hubic",
		NewFs: NewFs,
		Config: func(name string) {
			err := oauthutil.Config(name, oauthConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: oauthutil.ConfigClientID,
			Help: "Hubic Client Id - leave blank normally.",
		}, {
			Name: oauthutil.ConfigClientSecret,
			Help: "Hubic Client Secret - leave blank normally.",
		}},
	})
}

// credentials is the JSON returned from the Hubic API to read the
// OpenStack credentials
type credentials struct {
	Token    string `json:"token"`    // Openstack token
	Endpoint string `json:"endpoint"` // Openstack endpoint
	Expires  string `json:"expires"`  // Expires date - eg "2015-11-09T14:24:56+01:00"
}

// Fs represents a remote hubic
type Fs struct {
	fs.Fs                    // wrapped Fs
	client      *http.Client // client for oauth api
	credentials credentials  // returned from the Hubic API
	expires     time.Time    // time credentials expire
}

// Object describes a swift object
type Object struct {
	*swift.Object
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Object.String()
}

// ------------------------------------------------------------

// String converts this Fs to a string
func (f *Fs) String() string {
	if f.Fs == nil {
		return "Hubic"
	}
	return fmt.Sprintf("Hubic %s", f.Fs.String())
}

// getCredentials reads the OpenStack Credentials using the Hubic API
//
// The credentials are read into the Fs
func (f *Fs) getCredentials() (err error) {
	req, err := http.NewRequest("GET", "https://api.hubic.com/1.0/account/credentials", nil)
	if err != nil {
		return err
	}
	req.Header.Add("User-Agent", fs.UserAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Failed to get credentials: %s", resp.Status)
	}
	decoder := json.NewDecoder(resp.Body)
	var result credentials
	err = decoder.Decode(&result)
	if err != nil {
		return err
	}
	// fs.Debug(f, "Got credentials %+v", result)
	if result.Token == "" || result.Endpoint == "" || result.Expires == "" {
		return fmt.Errorf("Couldn't read token, result and expired from credentials")
	}
	f.credentials = result
	expires, err := time.Parse(time.RFC3339, result.Expires)
	if err != nil {
		return err
	}
	f.expires = expires
	fs.Debug(f, "Got swift credentials (expiry %v in %v)", f.expires, f.expires.Sub(time.Now()))
	return nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	client, err := oauthutil.NewClient(name, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to configure Hubic: %v", err)
	}

	f := &Fs{
		client: client,
	}

	// Make the swift Connection
	c := &swiftLib.Connection{
		Auth:           newAuth(f),
		UserAgent:      fs.UserAgent,
		ConnectTimeout: 10 * fs.Config.ConnectTimeout, // Use the timeouts in the transport
		Timeout:        10 * fs.Config.Timeout,        // Use the timeouts in the transport
		Transport:      fs.Config.Transport(),
	}
	err = c.Authenticate()
	if err != nil {
		return nil, fmt.Errorf("Error authenticating swift connection: %v", err)
	}

	// Make inner swift Fs from the connection
	swiftFs, err := swift.NewFsWithConnection(name, root, c)
	if err != nil {
		return nil, err
	}
	f.Fs = swiftFs
	return f, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	fPurge, ok := f.Fs.(fs.Purger)
	if !ok {
		return fs.ErrorCantPurge
	}
	return fPurge.Purge()
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	fCopy, ok := f.Fs.(fs.Copier)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	return fCopy.Copy(src, remote)
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = (*Fs)(nil)
	_ fs.Purger    = (*Fs)(nil)
	_ fs.Copier    = (*Fs)(nil)
	_ fs.UnWrapper = (*Fs)(nil)
)
