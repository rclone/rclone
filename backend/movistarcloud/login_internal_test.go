// Internal tests for the Movistar Cloud login flow
package movistarcloud

import "testing"

// TestGenerateCodeChallenge checks the S256 PKCE transformation against the
// test vector from RFC 7636 appendix B.
func TestGenerateCodeChallenge(t *testing.T) {
	const (
		verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	)
	if got := generateCodeChallenge(verifier); got != challenge {
		t.Errorf("generateCodeChallenge() = %q, want %q", got, challenge)
	}
}

func TestValidateT3URL(t *testing.T) {
	for _, test := range []struct {
		name            string
		url             string
		wantSessionID   string
		wantSessionData string
		wantErr         bool
	}{
		{
			name:            "valid",
			url:             "https://t3.movistar.es/#/accessUserPass?sessionID=abc123&sessionData=ZGF0YQ%3D%3D",
			wantSessionID:   "abc123",
			wantSessionData: "ZGF0YQ==",
		},
		{
			name:    "wrong host",
			url:     "https://evil.example.com/#/accessUserPass?sessionID=abc&sessionData=xyz",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			url:     "http://t3.movistar.es/#/accessUserPass?sessionID=abc&sessionData=xyz",
			wantErr: true,
		},
		{
			name:    "wrong route",
			url:     "https://t3.movistar.es/#/somethingElse?sessionID=abc&sessionData=xyz",
			wantErr: true,
		},
		{
			name:    "missing params",
			url:     "https://t3.movistar.es/#/accessUserPass?sessionID=abc",
			wantErr: true,
		},
		{
			name:    "no fragment",
			url:     "https://t3.movistar.es/accessUserPass?sessionID=abc&sessionData=xyz",
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sessionID, sessionData, err := validateT3URL(test.url)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sessionID != test.wantSessionID || sessionData != test.wantSessionData {
				t.Errorf("got (%q, %q), want (%q, %q)", sessionID, sessionData, test.wantSessionID, test.wantSessionData)
			}
		})
	}
}

func TestNormalizePhoneNumber(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"612345678", "612345678"},
		{"34612345678", "612345678"},
		{"+34612345678", "612345678"},
		{"0034612345678", "612345678"},
		{"+34 612 345 678", "612345678"},
		{"612-345-678", "612345678"},
		{"  612345678  ", "612345678"},
		{"", ""},
		{"abc", ""},
	} {
		t.Run(test.in, func(t *testing.T) {
			if got := normalizePhoneNumber(test.in); got != test.want {
				t.Errorf("normalizePhoneNumber(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestValidateMicloudOAuthURL(t *testing.T) {
	for _, test := range []struct {
		name      string
		url       string
		wantCode  string
		wantState string
		wantErr   bool
	}{
		{
			name:      "valid",
			url:       "https://micloud.movistar.es/ui/html/clientoauth.html?code=theCode&state=theState",
			wantCode:  "theCode",
			wantState: "theState",
		},
		{
			name:    "wrong host",
			url:     "https://evil.example.com/ui/html/clientoauth.html?code=theCode&state=theState",
			wantErr: true,
		},
		{
			name:    "missing code",
			url:     "https://micloud.movistar.es/ui/html/clientoauth.html?state=theState",
			wantErr: true,
		},
		{
			name:    "missing state",
			url:     "https://micloud.movistar.es/ui/html/clientoauth.html?code=theCode",
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, state, err := validateMicloudOAuthURL(test.url)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if code != test.wantCode || state != test.wantState {
				t.Errorf("got (%q, %q), want (%q, %q)", code, state, test.wantCode, test.wantState)
			}
		})
	}
}
