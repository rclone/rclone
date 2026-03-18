package api

import (
	"fmt"
	"testing"
	"time"
)

// TestLinkValid tests the Link.Valid method for various scenarios
func TestLinkValid(t *testing.T) {
	tests := []struct {
		name     string
		link     *Link
		expected bool
		desc     string
	}{
		{
			name:     "nil link",
			link:     nil,
			expected: false,
			desc:     "nil link should be invalid",
		},
		{
			name:     "empty URL",
			link:     &Link{URL: ""},
			expected: false,
			desc:     "empty URL should be invalid",
		},
		{
			name: "valid URL with future expire parameter",
			link: &Link{
				URL: fmt.Sprintf("https://example.com/file?expire=%d", time.Now().Add(time.Hour).Unix()),
			},
			expected: true,
			desc:     "URL with future expire parameter should be valid",
		},
		{
			name: "expired URL with past expire parameter",
			link: &Link{
				URL: fmt.Sprintf("https://example.com/file?expire=%d", time.Now().Add(-time.Hour).Unix()),
			},
			expected: false,
			desc:     "URL with past expire parameter should be invalid",
		},
		{
			name: "URL expire parameter takes precedence over Expire field",
			link: &Link{
				URL:    fmt.Sprintf("https://example.com/file?expire=%d", time.Now().Add(time.Hour).Unix()),
				Expire: Time(time.Now().Add(-time.Hour)), // Fallback is expired
			},
			expected: true,
			desc:     "URL expire parameter should take precedence over Expire field",
		},
		{
			name: "URL expire parameter within 10 second buffer should be invalid",
			link: &Link{
				URL: fmt.Sprintf("https://example.com/file?expire=%d", time.Now().Add(5*time.Second).Unix()),
			},
			expected: false,
			desc:     "URL expire parameter within 10 second buffer should be invalid",
		},
		{
			name: "fallback to Expire field when no URL expire parameter",
			link: &Link{
				URL:    "https://example.com/file",
				Expire: Time(time.Now().Add(time.Hour)),
			},
			expected: true,
			desc:     "should fallback to Expire field when URL has no expire parameter",
		},
		{
			name: "fallback to Expire field when URL expire parameter is invalid",
			link: &Link{
				URL:    "https://example.com/file?expire=invalid",
				Expire: Time(time.Now().Add(time.Hour)),
			},
			expected: true,
			desc:     "should fallback to Expire field when URL expire parameter is unparsable",
		},
		{
			name: "invalid when both URL expire and Expire field are expired",
			link: &Link{
				URL:    fmt.Sprintf("https://example.com/file?expire=%d", time.Now().Add(-time.Hour).Unix()),
				Expire: Time(time.Now().Add(-time.Hour)),
			},
			expected: false,
			desc:     "should be invalid when both URL expire and Expire field are expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.link.Valid()
			if result != tt.expected {
				t.Errorf("Link.Valid() = %v, expected %v. %s", result, tt.expected, tt.desc)
			}
		})
	}
}
