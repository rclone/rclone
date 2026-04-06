// Package api provides functionality for interacting with the iCloud API
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

const (
	baseEndpoint  = "https://www.icloud.com"
	setupEndpoint = "https://setup.icloud.com/setup/ws/1"
	authEndpoint  = "https://idmsa.apple.com/appleauth/auth"
)

type sessionSave func(*Session)

// Client defines the client configuration
type Client struct {
	appleID             string
	password            string
	remoteName          string // rclone remote name, used for cache namespacing
	srv                 *rest.Client
	Session             *Session
	sessionSaveCallback sessionSave

	drive *DriveService
	mu    sync.Mutex // protects drive and Authenticate
}

// New creates a new iCloud API client and initializes its HTTP session
func New(appleID, password, trustToken string, clientID string, cookies []*http.Cookie, sessionSaveCallback sessionSave, remoteName string) (*Client, error) {
	icloud := &Client{
		appleID:             strings.ToLower(appleID), // Apple SRP requires lowercase in client-side proof
		password:            password,
		remoteName:          filepath.Base(remoteName),
		srv:                 rest.NewClient(fshttp.NewClient(context.Background())),
		Session:             NewSession(),
		sessionSaveCallback: sessionSaveCallback,
	}

	icloud.Session.TrustToken = trustToken
	icloud.Session.Cookies = cookies
	icloud.Session.ClientID = clientID
	return icloud, nil
}

// DriveService returns the DriveService instance, creating it on first call
func (c *Client) DriveService() (*DriveService, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.drive == nil {
		var err error
		c.drive, err = NewDriveService(c)
		if err != nil {
			return nil, err
		}
	}
	return c.drive, nil
}

// Request makes a request to the iCloud API, re-authenticating on 401/421
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
			return c.Session.Request(ctx, opts, request, response)
		}
	}
	return resp, err
}

// Authenticate authenticates the client, reusing existing session if valid
func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Skip /validate round-trip when saved session has cookies + service endpoints
	// Native client behavior: use cached session, reauth lazily on 401/421
	if c.Session.Cookies != nil && len(c.Session.AccountInfo.Webservices) > 0 {
		fs.Debugf(nil, "iclouddrive: reusing saved session")
		return nil
	}
	// Try loading cached service endpoints to avoid /validate round-trip (~5s)
	if c.Session.Cookies != nil && c.loadCachedWebservices() {
		fs.Debugf(nil, "iclouddrive: reusing session with cached endpoints")
		return nil
	}
	if c.Session.Cookies != nil {
		if err := c.Session.ValidateSession(ctx); err == nil {
			fs.Debugf(nil, "iclouddrive: valid session, no need to reauth")
			c.saveCachedWebservices()
			return nil
		}
		c.Session.Cookies = nil
	}

	fs.Debugf(nil, "iclouddrive: authenticating")
	err := c.Session.SignIn(ctx, c.appleID, c.password)
	if err != nil {
		return err
	}

	// If 2FA is required, skip AuthWithToken - caller must complete 2FA first
	if c.Session.Requires2FA() {
		return nil
	}

	err = c.Session.AuthWithToken(ctx)
	if err == nil {
		c.saveCachedWebservices()
		if c.sessionSaveCallback != nil {
			c.sessionSaveCallback(c.Session)
		}
	}
	return err
}

// loadCachedWebservices loads service endpoints from disk cache
func (c *Client) loadCachedWebservices() bool {
	data, err := os.ReadFile(filepath.Join(config.GetCacheDir(), cacheSubdir, c.remoteName, "webservices.json"))
	if err != nil {
		return false
	}
	var ws map[string]*webService
	if err := json.Unmarshal(data, &ws); err != nil {
		return false
	}
	if len(ws) == 0 {
		return false
	}
	c.Session.AccountInfo.Webservices = ws
	return true
}

// saveCachedWebservices persists service endpoints to disk
func (c *Client) saveCachedWebservices() {
	if len(c.Session.AccountInfo.Webservices) == 0 {
		return
	}
	saveJSONCache(filepath.Join(config.GetCacheDir(), cacheSubdir, c.remoteName), "webservices.json", c.Session.AccountInfo.Webservices)
}

// CacheDir returns the disk cache directory for this remote
func (c *Client) CacheDir() string {
	return filepath.Join(config.GetCacheDir(), cacheSubdir, c.remoteName)
}

// ClearCacheDir removes all disk cache files for a remote
func ClearCacheDir(remoteName string) {
	dir := filepath.Join(config.GetCacheDir(), cacheSubdir, filepath.Base(remoteName))
	if err := os.RemoveAll(dir); err != nil {
		fs.Debugf(nil, "iclouddrive: failed to clear cache: %v", err)
	}
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

// Error satisfies the error interface
func (e *RequestError) Error() string {
	return fmt.Sprintf("%s: %s", e.Text, e.Status)
}

func newRequestError(status string, text string) *RequestError {
	return &RequestError{
		Status: strings.ToLower(status),
		Text:   text,
	}
}

// newRequestErrorf makes a new error from sprintf parameters
func newRequestErrorf(status string, text string, params ...any) *RequestError {
	return newRequestError(status, fmt.Sprintf(text, params...))
}
