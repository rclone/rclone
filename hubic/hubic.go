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
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "api_hubic_svWP970PvSWbw5G3PzrAqZ6X2uHeZBPI"
	rcloneEncryptedClientSecret = "leZKCcqy9movLhDWLVXX8cSLp_FzoiAPeEJOIOMRw1A5RuC4iLEPDYPWVF46adC_MVonnLdVEOTHVstfBOZ_lY4WNp8CK_YWlpRZ9diT5YI"
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
		ClientSecret: fs.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "hubic",
		Description: "Hubic",
		NewFs:       NewFs,
		Config: func(name string) {
			err := oauthutil.Config("hubic", name, oauthConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: fs.ConfigClientID,
			Help: "Hubic Client Id - leave blank normally.",
		}, {
			Name: fs.ConfigClientSecret,
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
	features    *fs.Features // optional features
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
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.Errorf("failed to get credentials: %s", resp.Status)
	}
	decoder := json.NewDecoder(resp.Body)
	var result credentials
	err = decoder.Decode(&result)
	if err != nil {
		return err
	}
	// fs.Debug(f, "Got credentials %+v", result)
	if result.Token == "" || result.Endpoint == "" || result.Expires == "" {
		return errors.New("couldn't read token, result and expired from credentials")
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
	client, _, err := oauthutil.NewClient(name, oauthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure Hubic")
	}

	f := &Fs{
		client: client,
	}

	// Make the swift Connection
	c := &swiftLib.Connection{
		Auth:           newAuth(f),
		ConnectTimeout: 10 * fs.Config.ConnectTimeout, // Use the timeouts in the transport
		Timeout:        10 * fs.Config.Timeout,        // Use the timeouts in the transport
		Transport:      fs.Config.Transport(),
	}
	err = c.Authenticate()
	if err != nil {
		return nil, errors.Wrap(err, "error authenticating swift connection")
	}

	// Make inner swift Fs from the connection
	swiftFs, err := swift.NewFsWithConnection(name, root, c)
	if err != nil && err != fs.ErrorIsFile {
		return nil, err
	}
	f.Fs = swiftFs
	f.features = f.Fs.Features().Wrap(f)
	return f, err
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = (*Fs)(nil)
	_ fs.UnWrapper = (*Fs)(nil)
)
