package box

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/box/api"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/lib/jwtutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeJWTPart decodes one base64url encoded part of a JWT into a map
func decodeJWTPart(t *testing.T, part string) map[string]any {
	data, err := base64.RawURLEncoding.DecodeString(part)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

// Test the JWT assertion sent to the token endpoint is in the format Box expects
func TestJWTAssertion(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	boxConfig := &api.ConfigJSON{
		BoxAppSettings: api.AppSettings{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			AppAuth: api.AppAuth{
				PublicKeyID: "public-key-id",
			},
		},
		EnterpriseID: "enterprise-id",
	}

	var form map[string][]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.NoError(t, r.ParseForm())
		form = r.PostForm
		_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"bearer","expires_in":3600}`))
	}))
	defer ts.Close()

	claims, err := getClaims(boxConfig, "enterprise")
	require.NoError(t, err)
	m := configmap.Simple{}
	start := time.Now()
	err = jwtutil.Config("box", "test", ts.URL, *claims, getSigningHeaders(boxConfig), getQueryParams(boxConfig), privateKey, m, ts.Client())
	require.NoError(t, err)

	// Check the token was stored
	assert.Contains(t, m["token"], `"access_token":"access-token"`)

	// Check the form parameters
	assert.Equal(t, []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"}, form["grant_type"])
	assert.Equal(t, []string{"client-id"}, form["client_id"])
	assert.Equal(t, []string{"client-secret"}, form["client_secret"])
	require.Len(t, form["assertion"], 1)
	parts := strings.Split(form["assertion"][0], ".")
	require.Len(t, parts, 3)

	// Check the signature independently of the JWT library
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	require.NoError(t, rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, hash[:], signature))

	// Check the header
	assert.Equal(t, map[string]any{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "public-key-id",
	}, decodeJWTPart(t, parts[0]))

	// Check the claims - aud must be a string, not an array
	payload := decodeJWTPart(t, parts[1])
	exp, ok := payload["exp"].(float64)
	require.True(t, ok, "exp should be a number")
	assert.InDelta(t, float64(start.Add(45*time.Second).Unix()), exp, 5)
	jti, ok := payload["jti"].(string)
	require.True(t, ok, "jti should be a string")
	assert.Len(t, jti, 40)
	delete(payload, "exp")
	delete(payload, "jti")
	assert.Equal(t, map[string]any{
		"aud":          tokenURL,
		"iss":          "client-id",
		"sub":          "enterprise-id",
		"box_sub_type": "enterprise",
	}, payload)
}

// Test box_config_file has ~ and environment variables expanded
func TestGetBoxConfigShellExpand(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"enterpriseID":"enterprise-id"}`), 0600))
	t.Setenv("RCLONE_TEST_BOX_CONFIG_DIR", dir)
	m := configmap.Simple{
		"box_config_file": "$RCLONE_TEST_BOX_CONFIG_DIR/config.json",
	}
	boxConfig, err := getBoxConfig(m)
	require.NoError(t, err)
	assert.Equal(t, "enterprise-id", boxConfig.EnterpriseID)
}
