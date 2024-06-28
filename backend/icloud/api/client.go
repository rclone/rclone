// Package api provides functionality for interacting with the iCloud API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

const (
	baseEndpoint  = "https://www.icloud.com"
	homeEndpoint  = "https://www.icloud.com"
	setupEndpoint = "https://setup.icloud.com/setup/ws/1"
	authEndpoint  = "https://idmsa.apple.com/appleauth/auth"
	clientID      = "e9f98057fb916de2bbd755ef280d7257146a76e5118f27ab2e9a3d065c20c17e"
)

type sessionSave func(*Session)

// Client defines the client configuration
type Client struct {
	appleID             string
	password            string
	srv                 *rest.Client
	Session             *Session
	sessionSaveCallback sessionSave

	drive *DriveService
}

// New creates a new Client instance with the provided Apple ID, password, trust token, cookies, and session save callback.
//
// Parameters:
// - appleID: the Apple ID of the user.
// - password: the password of the user.
// - trustToken: the trust token for the session.
// - cookies: the cookies for the session.
// - sessionSaveCallback: the callback function to save the session.
func New(appleID, password, trustToken string, cookies []*http.Cookie, sessionSaveCallback sessionSave) (*Client, error) {
	icloud := &Client{
		appleID:             appleID,
		password:            password,
		srv:                 rest.NewClient(fshttp.NewClient(context.Background())),
		Session:             NewSession(),
		sessionSaveCallback: sessionSaveCallback,
	}

	icloud.Session.TrustToken = trustToken
	icloud.Session.Cookies = cookies
	return icloud, nil
}

// DriveService returns the DriveService instance associated with the Client.
func (c *Client) DriveService() (*DriveService, error) {
	if c.drive == nil {
		c.drive, _ = NewDriveService(c)
	}
	return c.drive, nil
}

// Request makes a request and retries it if the session is invalid.
//
// This function is the main entry point for making requests to the iCloud
// API. If the initial request returns a 401 (Unauthorized), it will try to
// reauthenticate and retry the request.
func (c *Client) Request(ctx context.Context, opts rest.Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	resp, err = c.Session.Request(ctx, opts, request, response)
	if err != nil {
		// try to reauth
		if resp.StatusCode == 401 && c.Session.TrustToken != "" {
			err = c.Authenticate(ctx)
			if err != nil {
				return nil, err
			}
			return c.RequestNoReAuth(ctx, opts, request, response)
		}
	}
	return resp, err
}

// RequestNoReAuth makes a request without re-authenticating.
//
// This function is useful when you have a session that is already
// authenticated, but you need to make a request without triggering
// a re-authentication.
func (c *Client) RequestNoReAuth(ctx context.Context, opts rest.Opts, request interface{}, response interface{}) (resp *http.Response, err error) {
	// Make the request without re-authenticating
	resp, err = c.Session.Request(ctx, opts, request, response)
	return resp, err
}

// Authenticate authenticates the client with the iCloud API.
func (c *Client) Authenticate(ctx context.Context) error {
	{

		if c.Session.Cookies != nil {
			if err := c.Session.ValidateSession(ctx); err == nil {
				fs.Debugf("icloud", "Valid session, no need to reauth")
				return nil
			}
			c.Session.Cookies = nil
		}

		fs.Debugf("icloud", "Authenticating as %s\n", c.appleID)
		err := c.Session.SignIn(ctx, c.appleID, c.password)

		if err == nil {
			err = c.Session.AuthWithToken(ctx)
			if err == nil && c.sessionSaveCallback != nil {
				c.sessionSaveCallback(c.Session)
			}
		}
		return err
	}
}

// SignIn signs in the client using the provided context and credentials.
func (c *Client) SignIn(ctx context.Context) error {
	return c.Session.SignIn(ctx, c.appleID, c.password)
}

// IntoReader marshals the provided values into a JSON encoded reader
func IntoReader(values any) (*bytes.Reader, error) {
	m, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(m), nil
}
