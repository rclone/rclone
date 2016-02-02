package swift

import "github.com/ncw/swift"

// auth is an authenticator for swift
type auth struct {
	swift.Authenticator
	f          *Fs
	storageUrl string
}

// newAuth creates a swift authenticator
func newAuth(f *Fs, s string) *auth {
	return &auth{
		f:          f,
		storageUrl: s,
	}
}

// The public storage URL - set Internal to true to read
// internal/service net URL
func (a *auth) StorageUrl(Internal bool) string {
	if a.storageUrl != "" {
		return a.storageUrl
	}
	return a.f.c.Auth.StorageUrl(Internal)
}

// Check the interfaces are satisfied
var _ swift.Authenticator = (*auth)(nil)
