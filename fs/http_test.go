//+build go1.7

package fs

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the "%p" reprentation of the thing passed in
func ptr(p interface{}) string {
	return fmt.Sprintf("%p", p)
}

func TestSetDefaults(t *testing.T) {
	old := http.DefaultTransport.(*http.Transport)
	new := new(http.Transport)
	setDefaults(new, old)
	// Can't use assert.Equal or reflect.DeepEqual for this as it has functions in
	// Check functions by comparing the "%p" representations of them
	assert.Equal(t, ptr(old.Proxy), ptr(new.Proxy), "when checking .Proxy")
	assert.Equal(t, ptr(old.DialContext), ptr(new.DialContext), "when checking .DialContext")
	// Check the other public fields
	assert.Equal(t, ptr(old.Dial), ptr(new.Dial), "when checking .Dial")
	assert.Equal(t, ptr(old.DialTLS), ptr(new.DialTLS), "when checking .DialTLS")
	assert.Equal(t, old.TLSClientConfig, new.TLSClientConfig, "when checking .TLSClientConfig")
	assert.Equal(t, old.TLSHandshakeTimeout, new.TLSHandshakeTimeout, "when checking .TLSHandshakeTimeout")
	assert.Equal(t, old.DisableKeepAlives, new.DisableKeepAlives, "when checking .DisableKeepAlives")
	assert.Equal(t, old.DisableCompression, new.DisableCompression, "when checking .DisableCompression")
	assert.Equal(t, old.MaxIdleConns, new.MaxIdleConns, "when checking .MaxIdleConns")
	assert.Equal(t, old.MaxIdleConnsPerHost, new.MaxIdleConnsPerHost, "when checking .MaxIdleConnsPerHost")
	assert.Equal(t, old.IdleConnTimeout, new.IdleConnTimeout, "when checking .IdleConnTimeout")
	assert.Equal(t, old.ResponseHeaderTimeout, new.ResponseHeaderTimeout, "when checking .ResponseHeaderTimeout")
	assert.Equal(t, old.ExpectContinueTimeout, new.ExpectContinueTimeout, "when checking .ExpectContinueTimeout")
	assert.Equal(t, old.TLSNextProto, new.TLSNextProto, "when checking .TLSNextProto")
	assert.Equal(t, old.MaxResponseHeaderBytes, new.MaxResponseHeaderBytes, "when checking .MaxResponseHeaderBytes")
}

func TestCleanAuth(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"floo", "floo"},
		{"Authorization: ", "Authorization: "},
		{"Authorization: \n", "Authorization: \n"},
		{"Authorization: A", "Authorization: X"},
		{"Authorization: A\n", "Authorization: X\n"},
		{"Authorization: AAAA", "Authorization: XXXX"},
		{"Authorization: AAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAAA", "Authorization: XXXX"},
		{"Authorization: AAAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAAAAAAA\nPotato: Help\n", "Authorization: XXXX\nPotato: Help\n"},
		{"Sausage: 1\nAuthorization: AAAAAAAAA\nPotato: Help\n", "Sausage: 1\nAuthorization: XXXX\nPotato: Help\n"},
	} {
		got := string(cleanAuth([]byte(test.in)))
		assert.Equal(t, test.want, got, test.in)
	}
}
