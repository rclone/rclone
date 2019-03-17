package swift

import (
	"net/http"
	"time"

	"github.com/ncw/swift"
)

// auth is an authenticator for swift.  It overrides the StorageUrl
// and AuthToken with fixed values.
type auth struct {
	parentAuth swift.Authenticator
	storageURL string
	authToken  string
}

// newAuth creates a swift authenticator wrapper to override the
// StorageUrl and AuthToken values.
//
// Note that parentAuth can be nil
func newAuth(parentAuth swift.Authenticator, storageURL string, authToken string) *auth {
	return &auth{
		parentAuth: parentAuth,
		storageURL: storageURL,
		authToken:  authToken,
	}
}

// Request creates an http.Request for the auth - return nil if not needed
func (a *auth) Request(c *swift.Connection) (*http.Request, error) {
	if a.parentAuth == nil {
		return nil, nil
	}
	return a.parentAuth.Request(c)
}

// Response parses the http.Response
func (a *auth) Response(resp *http.Response) error {
	if a.parentAuth == nil {
		return nil
	}
	return a.parentAuth.Response(resp)
}

// The public storage URL - set Internal to true to read
// internal/service net URL
func (a *auth) StorageUrl(Internal bool) string { // nolint
	if a.storageURL != "" {
		return a.storageURL
	}
	if a.parentAuth == nil {
		return ""
	}
	return a.parentAuth.StorageUrl(Internal)
}

// The access token
func (a *auth) Token() string {
	if a.authToken != "" {
		return a.authToken
	}
	if a.parentAuth == nil {
		return ""
	}
	return a.parentAuth.Token()
}

// Expires returns the time the token expires if known or Zero if not.
func (a *auth) Expires() (t time.Time) {
	if do, ok := a.parentAuth.(swift.Expireser); ok {
		t = do.Expires()
	}
	return t
}

// The CDN url if available
func (a *auth) CdnUrl() string { // nolint
	if a.parentAuth == nil {
		return ""
	}
	return a.parentAuth.CdnUrl()
}

// Check the interfaces are satisfied
var (
	_ swift.Authenticator = (*auth)(nil)
	_ swift.Expireser     = (*auth)(nil)
)
