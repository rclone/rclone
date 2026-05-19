package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHeadersMergesCookies(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{{Name: "existing", Value: "old"}}

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "existing", Value: "new"}).String())
	resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "fresh", Value: "value"}).String())
	resp.Header.Set("X-Apple-Session-Token", "session-token")

	s.extractHeaders(resp)

	require.Len(t, s.Cookies, 2)
	assert.Equal(t, "new", s.Cookies[0].Value)
	assert.Equal(t, "fresh", s.Cookies[1].Name)
	assert.Equal(t, "session-token", s.SessionToken)
}

func TestExtractHeadersDeletesEmptyCookies(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{{Name: "X-APPLE-WEBAUTH-HSA-LOGIN", Value: "stale"}, {Name: "keep", Value: "value"}}

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "X-APPLE-WEBAUTH-HSA-LOGIN", Value: ""}).String())

	s.extractHeaders(resp)

	require.Len(t, s.Cookies, 1)
	assert.Equal(t, "keep", s.Cookies[0].Name)
	assert.Equal(t, "keep=value", s.GetCookieString())
}

func TestHasPCSCookiesRequiresDriveDocumentsCookie(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{
		{Name: "X-APPLE-WEBAUTH-PCS-Photos", Value: "photos"},
		{Name: "X-APPLE-WEBAUTH-PCS-Sharing", Value: "sharing"},
	}

	assert.False(t, s.hasPCSCookies())
	assert.Equal(t, []string{"X-APPLE-WEBAUTH-PCS-Documents"}, s.missingPCSCookies())
}

func TestHasPCSCookiesRequiresPhotosAndSharingCookies(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{{Name: "X-APPLE-WEBAUTH-PCS-Documents", Value: "documents"}}

	assert.False(t, s.hasPCSCookies())
	assert.Equal(t, []string{"X-APPLE-WEBAUTH-PCS-Photos", "X-APPLE-WEBAUTH-PCS-Sharing"}, s.missingPCSCookies())
}

func TestHasPCSCookiesIgnoresEmptyCookies(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{
		{Name: "X-APPLE-WEBAUTH-PCS-Documents", Value: "documents"},
		{Name: "X-APPLE-WEBAUTH-PCS-Photos", Value: "photos"},
		{Name: "X-APPLE-WEBAUTH-PCS-Sharing", Value: ""},
	}

	assert.False(t, s.hasPCSCookies())
	assert.Equal(t, []string{"X-APPLE-WEBAUTH-PCS-Sharing"}, s.missingPCSCookies())
}

func TestHasPCSCookiesAcceptsDocumentsPhotosAndSharing(t *testing.T) {
	s := NewSession()
	s.Cookies = []*http.Cookie{
		{Name: "X-APPLE-WEBAUTH-PCS-Documents", Value: "documents"},
		{Name: "X-APPLE-WEBAUTH-PCS-Photos", Value: "photos"},
		{Name: "X-APPLE-WEBAUTH-PCS-Sharing", Value: "sharing"},
	}

	assert.True(t, s.hasPCSCookies())
	assert.Empty(t, s.missingPCSCookies())
}
