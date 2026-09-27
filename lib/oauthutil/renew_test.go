package oauthutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config/configmap"
)

func newTestTokenSource(t *testing.T) *TokenSource {
	t.Helper()
	tok, err := json.Marshal(map[string]any{
		"access_token":  "A",
		"token_type":    "bearer",
		"refresh_token": "R",
		"expiry":        time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, ts, err := NewClient(context.Background(), "repro", configmap.Simple{"token": string(tok)},
		&Config{TokenURL: "https://127.0.0.1:1/token"})
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// Shutdown before the renewOnExpiry goroutine calls OnExpiry must not panic.
func TestShutdownBeforeRenewerStarts(t *testing.T) {
	for i := 0; i < 1000; i++ {
		NewRenew("repro", newTestTokenSource(t), func() error { return nil }).Shutdown()
	}
}

// Shutdown must not race OnExpiry's write of expiryTimer.
func TestShutdownRacesOnExpiry(t *testing.T) {
	r := NewRenew("repro", newTestTokenSource(t), func() error { return nil })
	time.Sleep(50 * time.Millisecond)
	r.Shutdown()
}
