package pcloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCallbackHostname checks that only hosts within pCloud's domain
// are accepted.
func TestCallbackHostname(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string // "" means an error is expected
	}{
		{"", defaultHostname},
		{"api.pcloud.com", "api.pcloud.com"},
		{"eapi.pcloud.com", "eapi.pcloud.com"},
		{"EAPI.pCloud.COM", "eapi.pcloud.com"},
		{"new-region.api.pcloud.com", "new-region.api.pcloud.com"},
		{"pcloud.com", ""},
		{".pcloud.com", ""},
		{"-.pcloud.com", ""},
		{"a-.pcloud.com", ""},
		{"a..pcloud.com", ""},
		{"api.pcloud.com.", ""},
		{"api.pclоud.com", ""}, // Cyrillic о
		{"127.0.0.1:18443", ""},
		{"evil.example.com", ""},
		{"api.pcloud.com.evil.example.com", ""},
		{"api.pcloud.com:443", ""},
		{"api.pcloud.com/path", ""},
		{"user@evil.example.com#.pcloud.com", ""},
		{"evil.example.com?.pcloud.com", ""},
	} {
		t.Run("["+test.in+"]", func(t *testing.T) {
			got, err := callbackHostname(test.in)
			if test.want == "" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.want, got)
		})
	}
}
