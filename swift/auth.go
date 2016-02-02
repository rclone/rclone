package swift

import "github.com/ncw/swift"

// auth is an authenticator for swift
type auth struct {
	swift.Authenticator
	storageURL string
}

// newAuth creates a swift authenticator wrapper to override the
// StorageUrl method.
func newAuth(Authenticator swift.Authenticator, storageURL string) *auth {
	return &auth{
		Authenticator: Authenticator,
		storageURL:    storageURL,
	}
}

// The public storage URL - set Internal to true to read
// internal/service net URL
func (a *auth) StorageUrl(Internal bool) string {
	if a.storageURL != "" {
		return a.storageURL
	}
	return a.Authenticator.StorageUrl(Internal)
}

// Check the interfaces are satisfied
var _ swift.Authenticator = (*auth)(nil)
