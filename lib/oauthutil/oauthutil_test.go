package oauthutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
)

// TestDeviceFlow exercises configSetupDevice end-to-end against a stub
// authorization server: first authorization_pending, then a token.
func TestDeviceFlow(t *testing.T) {
	var pollCount atomic.Int32
	var tokenCalls, deviceCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/devicecode", func(w http.ResponseWriter, r *http.Request) {
		deviceCalls.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("client_id"); got != "test-client" {
			t.Errorf("device request client_id = %q, want test-client", got)
		}
		if got := r.PostForm.Get("client_secret"); got != "" {
			t.Errorf("device request must not include client_secret, got %q", got)
		}
		if got := r.PostForm.Get("access_type"); got != "" {
			t.Errorf("device request must not include access_type (RFC 8628 has no such field), got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "DEV-123",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://example.test/device",
			"expires_in": 900,
			"interval": 1
		}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Errorf("token request grant_type = %q, want device_code grant", got)
		}
		if got := r.PostForm.Get("client_secret"); got != "" {
			t.Errorf("token request must not include client_secret, got %q", got)
		}
		if got := r.PostForm.Get("device_code"); got != "DEV-123" {
			t.Errorf("token request device_code = %q, want DEV-123", got)
		}
		if got := r.PostForm.Get("access_type"); got != "" {
			t.Errorf("token request must not include access_type (RFC 8628 has no such field), got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		// First call: still pending. Second call: succeed.
		if pollCount.Add(1) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"access_token": "AT-123",
			"refresh_token": "RT-456",
			"token_type": "Bearer",
			"expires_in": 3600
		}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &Config{
		ClientID:      "test-client",
		ClientSecret:  "should-not-be-sent",
		Scopes:        []string{"scope1"},
		DeviceAuthURL: srv.URL + "/devicecode",
		TokenURL:      srv.URL + "/token",
	}
	m := configmap.Simple{}

	if err := configSetupDevice(context.Background(), "remote", m, cfg, &Options{NoOffline: true}); err != nil {
		t.Fatalf("configSetupDevice: %v", err)
	}

	if deviceCalls.Load() != 1 {
		t.Errorf("device endpoint hit %d times, want 1", deviceCalls.Load())
	}
	if tokenCalls.Load() < 2 {
		t.Errorf("token endpoint hit %d times, want >=2 (one pending + one success)", tokenCalls.Load())
	}

	saved, ok := m.Get(config.ConfigToken)
	if !ok || saved == "" {
		t.Fatal("token was not saved to configmap")
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal([]byte(saved), &tok); err != nil {
		t.Fatalf("decode saved token: %v", err)
	}
	if tok.AccessToken != "AT-123" || tok.RefreshToken != "RT-456" {
		t.Errorf("saved token = %+v, want AT-123/RT-456", tok)
	}
}

// TestDeviceFlowExpired verifies a clean error when the device code expires.
func TestDeviceFlowExpired(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/devicecode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"device_code": "DEV-1",
			"user_code": "X",
			"verification_uri": "https://example.test/device",
			"expires_in": 900,
			"interval": 1
		}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"expired_token","error_description":"code expired"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &Config{
		ClientID:      "test-client",
		Scopes:        []string{"scope1"},
		DeviceAuthURL: srv.URL + "/devicecode",
		TokenURL:      srv.URL + "/token",
	}
	m := configmap.Simple{}

	err := configSetupDevice(context.Background(), "remote", m, cfg, &Options{NoOffline: true})
	if err == nil {
		t.Fatal("expected error on expired_token, got nil")
	}
	// The OAuth error code and Microsoft-style error_description must be
	// surfaced verbatim — without the misleading "rclone config reconnect"
	// suggestion that maybeWrapOAuthError used to add.
	if !strings.Contains(err.Error(), "expired_token") {
		t.Errorf("error %q does not contain OAuth error code", err.Error())
	}
	if !strings.Contains(err.Error(), "code expired") {
		t.Errorf("error %q does not contain server-supplied description", err.Error())
	}
	if strings.Contains(err.Error(), "config reconnect") {
		t.Errorf("error %q must not suggest 'rclone config reconnect' for an initial-auth failure", err.Error())
	}
	if _, ok := m.Get(config.ConfigToken); ok {
		t.Error("token should not be saved on failure")
	}
}

func TestDeviceFlowSupported(t *testing.T) {
	if deviceFlowSupported(&Config{}) {
		t.Error("empty Config must not report device-flow support")
	}
	if !deviceFlowSupported(&Config{DeviceAuthURL: "https://example.test/devicecode"}) {
		t.Error("Config with DeviceAuthURL must report device-flow support")
	}
}
