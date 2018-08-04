package fshttp

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
	newT := new(http.Transport)
	setDefaults(newT, old)
	// Can't use assert.Equal or reflect.DeepEqual for this as it has functions in
	// Check functions by comparing the "%p" representations of them
	assert.Equal(t, ptr(old.Proxy), ptr(newT.Proxy), "when checking .Proxy")
	assert.Equal(t, ptr(old.DialContext), ptr(newT.DialContext), "when checking .DialContext")
	// Check the other public fields
	assert.Equal(t, ptr(old.Dial), ptr(newT.Dial), "when checking .Dial")
	assert.Equal(t, ptr(old.DialTLS), ptr(newT.DialTLS), "when checking .DialTLS")
	assert.Equal(t, old.TLSClientConfig, newT.TLSClientConfig, "when checking .TLSClientConfig")
	assert.Equal(t, old.TLSHandshakeTimeout, newT.TLSHandshakeTimeout, "when checking .TLSHandshakeTimeout")
	assert.Equal(t, old.DisableKeepAlives, newT.DisableKeepAlives, "when checking .DisableKeepAlives")
	assert.Equal(t, old.DisableCompression, newT.DisableCompression, "when checking .DisableCompression")
	assert.Equal(t, old.MaxIdleConns, newT.MaxIdleConns, "when checking .MaxIdleConns")
	assert.Equal(t, old.MaxIdleConnsPerHost, newT.MaxIdleConnsPerHost, "when checking .MaxIdleConnsPerHost")
	assert.Equal(t, old.IdleConnTimeout, newT.IdleConnTimeout, "when checking .IdleConnTimeout")
	assert.Equal(t, old.ResponseHeaderTimeout, newT.ResponseHeaderTimeout, "when checking .ResponseHeaderTimeout")
	assert.Equal(t, old.ExpectContinueTimeout, newT.ExpectContinueTimeout, "when checking .ExpectContinueTimeout")
	assert.Equal(t, old.TLSNextProto, newT.TLSNextProto, "when checking .TLSNextProto")
	assert.Equal(t, old.MaxResponseHeaderBytes, newT.MaxResponseHeaderBytes, "when checking .MaxResponseHeaderBytes")
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
		got := string(cleanAuth([]byte(test.in), authBufs[0]))
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestCleanAuths(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"floo", "floo"},
		{"Authorization: AAAAAAAAA\nPotato: Help\n", "Authorization: XXXX\nPotato: Help\n"},
		{"X-Auth-Token: AAAAAAAAA\nPotato: Help\n", "X-Auth-Token: XXXX\nPotato: Help\n"},
		{"X-Auth-Token: AAAAAAAAA\nAuthorization: AAAAAAAAA\nPotato: Help\n", "X-Auth-Token: XXXX\nAuthorization: XXXX\nPotato: Help\n"},
	} {
		got := string(cleanAuths([]byte(test.in)))
		assert.Equal(t, test.want, got, test.in)
	}
}
