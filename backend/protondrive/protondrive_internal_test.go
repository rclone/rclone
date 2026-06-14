package protondrive

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/assert"
)

var protonDriveAppVersionPattern = regexp.MustCompile(`(?i)^external-drive(-[a-z_]+)+@[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?-((stable|beta|RC|alpha)(([.-]?\d+)*)?)?([.-]?dev)?(\+.*)?$`)

func TestProtonDriveAppVersionFromRcloneVersion(t *testing.T) {
	testCases := []struct {
		name          string
		rcloneVersion string
		want          string
	}{
		{
			name:          "release",
			rcloneVersion: "v1.73.5",
			want:          "external-drive-rclone@1.73.5-stable",
		},
		{
			name:          "dev build",
			rcloneVersion: "v1.74.0-DEV",
			want:          "external-drive-rclone@1.74.0-dev",
		},
		{
			name:          "beta build with extra metadata",
			rcloneVersion: "v1.74.0-beta.9519.990f33f2a.fix-protondrive-sdk-2026",
			want:          "external-drive-rclone@1.74.0-beta.9519+990f33f2a.fix-protondrive-sdk-2026",
		},
		{
			name:          "beta build with unsanitized branch name",
			rcloneVersion: "v1.74.0-beta.9519.990f33f2a.fix/protondrive-sdk-2026",
			want:          "external-drive-rclone@1.74.0-beta.9519+990f33f2a.fix-protondrive-sdk-2026",
		},
		{
			name:          "invalid version falls back to stable",
			rcloneVersion: "not-a-version",
			want:          "external-drive-rclone@1.0.0-stable",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := protonDriveAppVersionFromRcloneVersion(testCase.rcloneVersion)

			if got != testCase.want {
				t.Fatalf("unexpected app version: got %q, want %q", got, testCase.want)
			}
			if !protonDriveAppVersionPattern.MatchString(got) {
				t.Fatalf("app version %q does not match Proton pattern", got)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	apiErr := func(status int, code proton.Code) error {
		return &proton.APIError{Status: status, Code: code, Message: "test"}
	}

	for _, tc := range []struct {
		name      string
		ctx       context.Context
		err       error
		wantRetry bool
	}{
		{"nil error", ctx, nil, false},
		{"cancelled context", cancelledCtx, errors.New("some error"), false},
		{"permanent validation error Code=200501 Status=422 (not retried)", ctx, apiErr(422, 200501), false},
		{"transient storage block error Code=200501 Status=500 (retried)", ctx, apiErr(500, 200501), true},
		{"server error Status=500", ctx, apiErr(500, 0), true},
		{"server error Status=502", ctx, apiErr(502, 0), true},
		{"server error Status=504", ctx, apiErr(504, 0), true},
		{"server error Status=503 (handled by SDK, not retried here)", ctx, apiErr(503, 0), false},
		{"rate limit Status=429 (handled by SDK, not retried here)", ctx, apiErr(429, 0), false},
		{"client error Status=400", ctx, apiErr(400, 0), false},
		{"client error Status=404", ctx, apiErr(404, 0), false},
		{"wrapped API error retried via errors.As", ctx, fmt.Errorf("wrapped: %w", &proton.APIError{Status: 500}), true},
		{"non-API error falls back to fserrors.ShouldRetry", ctx, errors.New("plain error"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotRetry, _ := shouldRetry(tc.ctx, tc.err)
			assert.Equal(t, tc.wantRetry, gotRetry)
		})
	}
}

func TestValidateProtonAuthMethod(t *testing.T) {
	assert.NoError(t, validateProtonAuthMethod(""))
	assert.NoError(t, validateProtonAuthMethod(authMethodPassword))
	assert.NoError(t, validateProtonAuthMethod(authMethodWeb))
	assert.Error(t, validateProtonAuthMethod("invalid"))
}

func TestNormalizeProtonAuthMethod(t *testing.T) {
	assert.Equal(t, authMethodPassword, normalizeProtonAuthMethod(""))
	assert.Equal(t, authMethodPassword, normalizeProtonAuthMethod(authMethodPassword))
	assert.Equal(t, authMethodWeb, normalizeProtonAuthMethod(authMethodWeb))
}

func TestProtonGenerateSignInURL(t *testing.T) {
	key, signInURL, err := protonGenerateSignInURL("ABCD1234")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, key, 32)

	parsed, err := url.Parse(signInURL)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "account.proton.me", parsed.Host)
	assert.Equal(t, "/desktop/login", parsed.Path)
	assert.Equal(t, "drive", parsed.Query().Get("app"))
	assert.Equal(t, "3", parsed.Query().Get("pv"))

	const payloadPrefix = "#payload="
	if !strings.HasPrefix(parsed.Fragment, strings.TrimPrefix(payloadPrefix, "#")) {
		t.Fatalf("missing payload fragment in sign-in URL: %q", signInURL)
	}

	payload, err := url.QueryUnescape(strings.TrimPrefix(parsed.RawFragment, "payload="))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(payload, ":")
	if len(parts) != 4 {
		t.Fatalf("unexpected sign-in payload parts: %q", payload)
	}
	assert.Equal(t, "0", parts[0])
	assert.Equal(t, "ABCD1234", parts[1])
	assert.Equal(t, protonWebAuthClientID, parts[3])

	decodedKey, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, key, decodedKey)
}

func TestProtonParseForkKeyPassword(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	payload := map[string]string{"keyPassword": "salted-key-pass"}

	encryptedPayload := mustEncryptProtonForkPayload(t, key, []byte("123456789012"), payload)

	got, err := protonParseForkKeyPassword(key, encryptedPayload)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "salted-key-pass", got)
}

func TestProtonParseForkKeyPasswordErrors(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "not base64", payload: "not base64"},
		{name: "too short", payload: base64.StdEncoding.EncodeToString([]byte("short"))},
		{name: "missing keyPassword", payload: mustEncryptProtonForkPayload(t, key, []byte("123456789012"), map[string]string{"other": "value"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := protonParseForkKeyPassword(key, tc.payload)
			assert.Error(t, err)
		})
	}
}

func TestNormalizeSaltedKeyPass(t *testing.T) {
	base64Value := base64.StdEncoding.EncodeToString([]byte("already encoded"))

	assert.Equal(t, base64Value, normalizeSaltedKeyPass(base64Value))
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("raw key password")), normalizeSaltedKeyPass("raw key password"))
}

func TestProtonSessionForkHTTPFlow(t *testing.T) {
	const appVersion = "external-drive-rclone@1.75.0-dev"
	const userAgent = "rclone-test"

	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, appVersion, r.Header.Get("x-pm-appversion"))
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))

		requests = append(requests, r.URL.EscapedPath())
		switch r.URL.EscapedPath() {
		case "/auth/v4/sessions/forks":
			_, _ = w.Write([]byte(`{"Code":1000,"Selector":"selector/with/slash","UserCode":"USERCODE"}`))
		case "/auth/v4/sessions/forks/selector%2Fwith%2Fslash":
			_, _ = w.Write([]byte(`{"Code":1000,"Payload":"payload","UID":"uid","AccessToken":"access","RefreshToken":"refresh"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	withProtonDriveAPIBaseURL(t, server.URL)

	initResponse, err := protonSessionForkInit(context.Background(), server.Client(), appVersion, userAgent)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "selector/with/slash", initResponse.Selector)
	assert.Equal(t, "USERCODE", initResponse.UserCode)

	statusResponse, ready, err := protonSessionForkStatus(context.Background(), server.Client(), appVersion, userAgent, initResponse.Selector)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, ready)
	assert.Equal(t, "uid", statusResponse.UID)
	assert.Equal(t, "access", statusResponse.AccessToken)
	assert.Equal(t, "refresh", statusResponse.RefreshToken)
	assert.Equal(t, []string{"/auth/v4/sessions/forks", "/auth/v4/sessions/forks/selector%2Fwith%2Fslash"}, requests)
}

func TestProtonSessionForkStatusNotReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"Code":2001,"Error":"not ready"}`, http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	withProtonDriveAPIBaseURL(t, server.URL)

	response, ready, err := protonSessionForkStatus(context.Background(), server.Client(), "app-version", "user-agent", "selector")
	assert.NoError(t, err)
	assert.False(t, ready)
	assert.Nil(t, response)
}

func withProtonDriveAPIBaseURL(t *testing.T, value string) {
	t.Helper()

	old := protonDriveAPIBaseURL
	protonDriveAPIBaseURL = value
	t.Cleanup(func() {
		protonDriveAPIBaseURL = old
	})
}

func mustEncryptProtonForkPayload(t *testing.T, key, nonce []byte, payload map[string]string) string {
	t.Helper()

	plaintext, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}

	sealed := gcm.Seal(nil, nonce, plaintext, protonForkAAD)
	ciphertext := sealed[:len(sealed)-gcm.Overhead()]
	tag := sealed[len(sealed)-gcm.Overhead():]

	blob := append(append([]byte{}, nonce...), ciphertext...)
	blob = append(blob, tag...)
	return base64.StdEncoding.EncodeToString(blob)
}
