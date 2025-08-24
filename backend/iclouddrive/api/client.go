// Package api provides functionality for interacting with the iCloud API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

const (
	baseEndpoint  = "https://www.icloud.com"
	homeEndpoint  = "https://www.icloud.com"
	setupEndpoint = "https://setup.icloud.com/setup/ws/1"
	authEndpoint  = "https://idmsa.apple.com/appleauth/auth"
)

type sessionSave func(*Session)

// Client defines the client configuration
type Client struct {
	appleID             string
	password            string
	srv                 *rest.Client
	Session             *Session
	sessionSaveCallback sessionSave

	// ADP/PCS support
	pcsMu          sync.Mutex
	pcsReady       map[string]bool
	pcsGroup       singleflight.Group
	webBuildNumber string

	drive *DriveService
}

// New creates a new Client instance with the provided Apple ID, password, trust token, cookies, and session save callback.
//
// Parameters:
// - appleID: the Apple ID of the user.
// - password: the password of the user.
// - trustToken: the trust token for the session.
// - clientID: the client id for the session.
// - cookies: the cookies for the session.
// - sessionSaveCallback: the callback function to save the session.
func New(appleID, password, trustToken string, clientID string, cookies []*http.Cookie, sessionSaveCallback sessionSave) (*Client, error) {
	icloud := &Client{
		appleID:             appleID,
		password:            password,
		srv:                 rest.NewClient(fshttp.NewClient(context.Background())),
		Session:             NewSession(),
		sessionSaveCallback: sessionSaveCallback,
	}

	icloud.Session.TrustToken = trustToken
	icloud.Session.Cookies = cookies
	icloud.Session.ClientID = clientID
	return icloud, nil
}

// DriveService returns the DriveService instance associated with the Client.
func (c *Client) DriveService(ctx context.Context) (*DriveService, error) {
	var err error
	if c.drive == nil {
		// Ensure ADP/PCS consent/cookies for iCloud Drive before use it
		if err := c.EnsurePCSForServiceOnce(ctx, "iclouddrive"); err != nil {
			return nil, fmt.Errorf("icloud: ADP/PCS consent for iCloud Drive failed: %w", err)
		}

		c.drive, err = NewDriveService(c)
		if err != nil {
			return nil, err
		}
	}
	return c.drive, nil
}

// EnsurePCSForServiceOnce ensures the PCS flow runs at most once concurrently per app.
// If the run fails, the next caller will retry.
func (c *Client) EnsurePCSForServiceOnce(ctx context.Context, app string) error {
	// Fast path: already ready?
	c.pcsMu.Lock()
	ready := c.pcsReady != nil && c.pcsReady[app]
	c.pcsMu.Unlock()
	if ready {
		return nil
	}

	// singleflight.Do executes the given function for a given key only once.
	// Callers will wait for the result. The context is passed to the function being executed.
	_, err, _ := c.pcsGroup.Do(app, func() (any, error) {
		return nil, c.EnsurePCSForService(ctx, app)
	})

	if err == nil {
		// Mark as ready only on success.
		c.pcsMu.Lock()
		if c.pcsReady == nil {
			c.pcsReady = make(map[string]bool)
		}
		c.pcsReady[app] = true
		c.pcsMu.Unlock()
	}

	return err
}

// Request makes a request and retries it if the session is invalid.
// This function is the main entry point for making requests to the iCloud
// API. If the initial request returns a 401 (Unauthorized), it will try to
// reauthenticate and retry the request.
func (c *Client) Request(ctx context.Context, opts rest.Opts, request any, response any) (resp *http.Response, err error) {
	resp, err = c.Session.Request(ctx, opts, request, response)
	if err != nil && resp != nil {
		// try to reauth
		if resp.StatusCode == 401 || resp.StatusCode == 421 {
			err = c.Authenticate(ctx)
			if err != nil {
				return nil, err
			}

			if c.Session.Requires2FA() {
				return nil, errors.New("trust token expired, please reauth")
			}
			return c.RequestNoReAuth(ctx, opts, request, response)
		}
	}
	return resp, err
}

// RequestNoReAuth makes a request without re-authenticating.
// This function is useful when you have a session that is already
// authenticated, but you need to make a request without triggering
// a re-authentication.
func (c *Client) RequestNoReAuth(ctx context.Context, opts rest.Opts, request any, response any) (resp *http.Response, err error) {
	// Make the request without re-authenticating
	resp, err = c.Session.Request(ctx, opts, request, response)
	return resp, err
}

// Authenticate authenticates the client with the iCloud API.
func (c *Client) Authenticate(ctx context.Context) error {
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

// RequestError holds info on a result state, icloud can return a 200 but the result is unknown
type RequestError struct {
	Status string
	Text   string
}

// Error satisfy the error interface.
func (e *RequestError) Error() string {
	return fmt.Sprintf("%s: %s", e.Text, e.Status)
}

func newRequestError(Status string, Text string) *RequestError {
	return &RequestError{
		Status: strings.ToLower(Status),
		Text:   Text,
	}
}

// newRequestErrorf makes a new error from sprintf parameters.
func newRequestErrorf(Status string, Text string, Parameters ...any) *RequestError {
	return newRequestError(strings.ToLower(Status), fmt.Sprintf(Text, Parameters...))
}
